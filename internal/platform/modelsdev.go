package platform

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/MehmetMHY/ch/internal/config"
	"github.com/MehmetMHY/ch/pkg/types"
)

// ModelsDevURL is the fixed, read-only metadata endpoint.
const ModelsDevURL = "https://models.dev/api.json"

// ModelsDevMaxBytes caps the response body to avoid unbounded memory use.
const ModelsDevMaxBytes = 32 * 1024 * 1024 // 32 MiB

// ModelsDevFetchTimeout is the HTTP timeout for metadata refreshes.
const ModelsDevFetchTimeout = 30 * time.Second

// modelsDevCacheFilename is the cache file name under ~/.ch/cache/.
const modelsDevCacheFilename = "models_dev.json"

// modelsDevMetaFilename stores fetch metadata (timestamp + ETag).
const modelsDevMetaFilename = "models_dev.meta.json"

// ModelsDevProvider is a single provider entry from api.json.
type ModelsDevProvider struct {
	ID     string                    `json:"id"`
	Name   string                    `json:"name"`
	Models map[string]ModelsDevModel `json:"models"`
}

// ModelsDevModel is a provider-served model record from api.json.
// Only fields Ch trusts for capability decisions are parsed.
type ModelsDevModel struct {
	ID               string            `json:"id"`
	Name             string            `json:"name"`
	Reasoning        bool              `json:"reasoning"`
	ReasoningOptions []ModelsDevOption `json:"reasoning_options"`
	Cost             *ModelsDevCost    `json:"cost,omitempty"`
}

// ModelsDevOption describes a reasoning control option.
type ModelsDevOption struct {
	Type   string   `json:"type"`
	Values []string `json:"values,omitempty"`
	Min    int      `json:"min,omitempty"`
}

// ModelsDevCost holds provider-specific pricing per million tokens.
// Retained in cache for future cost estimation; not displayed yet.
type ModelsDevCost struct {
	Input      float64 `json:"input,omitempty"`
	Output     float64 `json:"output,omitempty"`
	Reasoning  float64 `json:"reasoning,omitempty"`
	CacheRead  float64 `json:"cache_read,omitempty"`
	CacheWrite float64 `json:"cache_write,omitempty"`
}

// ModelsDevCatalog is the parsed api.json payload.
type ModelsDevCatalog map[string]ModelsDevProvider

// modelsDevMeta is the cache metadata sidecar.
type modelsDevMeta struct {
	FetchedAt int64  `json:"fetched_at"`
	ETag      string `json:"etag,omitempty"`
}

// CapabilityResult describes the resolved reasoning-effort capability
// for a specific platform + model.
type CapabilityResult struct {
	// Status: "supported", "unsupported", or "unknown".
	Status string
	// AllowedValues are the exact effort values advertised for this model.
	// Only populated when Status == "supported".
	AllowedValues []string
	// HasToggle is true when the model supports a reasoning toggle but
	// not adjustable effort. The model still reasons but effort is not
	// user-controllable.
	HasToggle bool
}

// Capability status constants.
const (
	CapStatusSupported   = "supported"
	CapStatusUnsupported = "unsupported"
	CapStatusUnknown     = "unknown"
)

// modelsDevClient manages the catalog cache and lookups.
type modelsDevClient struct {
	mu          sync.Mutex
	cachePath   string
	metaPath    string
	catalog     ModelsDevCatalog
	catalogTime time.Time
	loaded      bool
}

var (
	modelsDevOnce   sync.Once
	modelsDevShared *modelsDevClient
)

// getModelsDevClient returns the singleton client, initializing cache paths.
func getModelsDevClient() *modelsDevClient {
	modelsDevOnce.Do(func() {
		cacheDir, err := config.GetCacheDir()
		if err != nil {
			// Without a cache dir we can still attempt in-memory fetches.
			cacheDir = ""
		}
		modelsDevShared = &modelsDevClient{
			cachePath: filepath.Join(cacheDir, modelsDevCacheFilename),
			metaPath:  filepath.Join(cacheDir, modelsDevMetaFilename),
		}
	})
	return modelsDevShared
}

// catalogForConfig returns the catalog, refreshing from the network if the
// cache is stale and metadata is enabled. It never panics; on any failure
// it returns the last-known-good cache or nil.
func (c *modelsDevClient) catalogForConfig(cfg *types.Config) ModelsDevCatalog {
	c.mu.Lock()
	defer c.mu.Unlock()

	// If metadata is disabled, only use an existing in-memory or on-disk cache.
	if !cfg.ModelsDevEnabled {
		if c.loaded {
			return c.catalog
		}
		c.catalog = c.loadCacheFromDisk()
		c.loaded = true
		c.catalogTime = time.Now()
		return c.catalog
	}

	// Determine refresh threshold.
	refreshHours := cfg.ModelsDevRefreshHours
	if refreshHours <= 0 {
		refreshHours = 24
	}
	threshold := time.Duration(refreshHours) * time.Hour

	// Load from disk if not yet loaded.
	if !c.loaded {
		c.catalog = c.loadCacheFromDisk()
		c.loaded = true
		meta := c.loadMetaFromDisk()
		c.catalogTime = time.Unix(meta.FetchedAt, 0)
	}

	// If cache is fresh, use it.
	if c.catalog != nil && time.Since(c.catalogTime) < threshold {
		return c.catalog
	}

	// Cache is stale or absent: try a network refresh.
	fetched, etag, ok := c.fetchFromNetwork()
	if ok {
		c.catalog = fetched
		c.catalogTime = time.Now()
		c.persistCache(fetched, etag)
		return c.catalog
	}

	// Network failed: use whatever cache we have (even if stale).
	return c.catalog
}

// loadCacheFromDisk reads and parses the cached catalog file.
func (c *modelsDevClient) loadCacheFromDisk() ModelsDevCatalog {
	if c.cachePath == "" {
		return nil
	}
	data, err := os.ReadFile(c.cachePath) // #nosec G304 -- Cache path is under ~/.ch/cache/.
	if err != nil {
		return nil
	}
	var catalog ModelsDevCatalog
	if err := json.Unmarshal(data, &catalog); err != nil {
		return nil
	}
	return catalog
}

// loadMetaFromDisk reads the cache metadata sidecar.
func (c *modelsDevClient) loadMetaFromDisk() modelsDevMeta {
	if c.metaPath == "" {
		return modelsDevMeta{}
	}
	data, err := os.ReadFile(c.metaPath) // #nosec G304 -- Meta path is under ~/.ch/cache/.
	if err != nil {
		return modelsDevMeta{}
	}
	var meta modelsDevMeta
	if err := json.Unmarshal(data, &meta); err != nil {
		return modelsDevMeta{}
	}
	return meta
}

// fetchFromNetwork downloads and parses api.json. Returns the catalog,
// an ETag (if supplied), and ok=false on any error.
func (c *modelsDevClient) fetchFromNetwork() (ModelsDevCatalog, string, bool) {
	client := &http.Client{Timeout: ModelsDevFetchTimeout}

	req, err := http.NewRequest("GET", ModelsDevURL, nil)
	if err != nil {
		return nil, "", false
	}

	// Send ETag for conditional refresh if we have one.
	if meta := c.loadMetaFromDisk(); meta.ETag != "" {
		req.Header.Set("If-None-Match", meta.ETag)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, "", false
	}
	defer resp.Body.Close()

	// 304 Not Modified: existing cache is still valid.
	if resp.StatusCode == http.StatusNotModified {
		return c.loadCacheFromDisk(), c.loadMetaFromDisk().ETag, true
	}

	if resp.StatusCode != http.StatusOK {
		return nil, "", false
	}

	// Read with a size cap.
	limited := io.LimitReader(resp.Body, ModelsDevMaxBytes)
	data, err := io.ReadAll(limited)
	if err != nil {
		return nil, "", false
	}

	var catalog ModelsDevCatalog
	if err := json.Unmarshal(data, &catalog); err != nil {
		return nil, "", false
	}

	etag := resp.Header.Get("ETag")
	return catalog, etag, true
}

// persistCache atomically writes the catalog and metadata to disk.
func (c *modelsDevClient) persistCache(catalog ModelsDevCatalog, etag string) {
	if c.cachePath == "" {
		return
	}

	// Write catalog atomically.
	data, err := json.Marshal(catalog)
	if err != nil {
		return
	}
	tmpPath := c.cachePath + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0600); err != nil {
		return
	}
	if err := os.Rename(tmpPath, c.cachePath); err != nil {
		_ = os.Remove(tmpPath)
		return
	}

	// Write metadata sidecar.
	meta := modelsDevMeta{
		FetchedAt: time.Now().Unix(),
		ETag:      etag,
	}
	metaData, _ := json.Marshal(meta)
	tmpMeta := c.metaPath + ".tmp"
	if err := os.WriteFile(tmpMeta, metaData, 0600); err != nil {
		return
	}
	_ = os.Rename(tmpMeta, c.metaPath)
}

// resolveModelsDevProvider maps a Ch platform key to the models.dev provider ID.
func resolveModelsDevProvider(platformKey string, cfg *types.Config) string {
	if platformKey == "openai" {
		return "openai"
	}
	if p, ok := cfg.Platforms[platformKey]; ok {
		if p.ModelsDevProvider != "" {
			return p.ModelsDevProvider
		}
		return platformKey
	}
	return platformKey
}

// normalizeModelIDForLookup converts a model ID to the form used by models.dev
// keys. Currently this only strips the generic "models/" prefix that Google's
// native model-list endpoint prepends. The outbound request model value is
// never modified.
func normalizeModelIDForLookup(modelID string) string {
	if strings.HasPrefix(modelID, "models/") {
		return strings.TrimPrefix(modelID, "models/")
	}
	return modelID
}

// ResolveCapability determines whether a platform+model supports adjustable
// reasoning effort via the root-level chat-completions reasoning_effort field.
//
// Returns:
//   - CapStatusSupported with AllowedValues when the model advertises effort options.
//   - CapStatusUnsupported when the model is known but does not support effort control.
//   - CapStatusUnknown when no metadata is available (caller may allow unverified overrides).
func ResolveCapability(platformKey, modelID string, cfg *types.Config) CapabilityResult {
	client := getModelsDevClient()
	catalog := client.catalogForConfig(cfg)

	if catalog == nil {
		return CapabilityResult{Status: CapStatusUnknown}
	}

	providerID := resolveModelsDevProvider(platformKey, cfg)
	provider, ok := catalog[providerID]
	if !ok {
		return CapabilityResult{Status: CapStatusUnknown}
	}

	// Try the raw model ID first.
	model, found := provider.Models[modelID]
	if !found {
		// Try normalized alias (e.g. "models/gemini-3.7-flash" -> "gemini-3.7-flash").
		normalized := normalizeModelIDForLookup(modelID)
		if normalized != modelID {
			model, found = provider.Models[normalized]
		}
	}
	if !found {
		return CapabilityResult{Status: CapStatusUnknown}
	}

	// Model is known. Check for effort-type reasoning options.
	if !model.Reasoning {
		return CapabilityResult{Status: CapStatusUnsupported}
	}

	hasToggle := false
	for _, opt := range model.ReasoningOptions {
		if opt.Type == "effort" && len(opt.Values) > 0 {
			return CapabilityResult{
				Status:        CapStatusSupported,
				AllowedValues: opt.Values,
			}
		}
		if opt.Type == "toggle" {
			hasToggle = true
		}
	}

	// Model supports reasoning but no adjustable effort control.
	return CapabilityResult{
		Status:    CapStatusUnsupported,
		HasToggle: hasToggle,
	}
}

// IsEffortValueValid checks whether a reasoning effort value is in the
// provided allowed-values list. "default" is never in the list
// (it is a Ch-specific sentinel that means "omit the parameter").
func IsEffortValueValid(effort string, allowed []string) bool {
	for _, v := range allowed {
		if v == effort {
			return true
		}
	}
	return false
}

// ProviderDefaultLabel is the fzf display string for "omit the parameter".
const ProviderDefaultLabel = "default (omit)"

// ProviderDefaultCLIValue is the CLI/interactive value that clears the effort.
const ProviderDefaultCLIValue = "default"

// IsProviderDefault returns true if the value is the sentinel that means
// "omit the reasoning_effort parameter".
func IsProviderDefault(effort string) bool {
	return effort == "" || effort == ProviderDefaultCLIValue
}

// NormalizeEffort converts the CLI sentinel "default" to the
// internal empty string. Other values are returned as-is.
func NormalizeEffort(effort string) string {
	if effort == ProviderDefaultCLIValue {
		return ""
	}
	return effort
}

// BuildEffortOptions constructs the fzf option list for !r.
// When metadata is available, only advertised values are shown.
// When metadata is unavailable, generic unverified values are shown.
func BuildEffortOptions(cap CapabilityResult) []string {
	options := []string{ProviderDefaultLabel}

	if cap.Status == CapStatusSupported {
		options = append(options, cap.AllowedValues...)
		return options
	}

	// Unknown metadata: show generic unverified values so the user can
	// explicitly override. These are the common effort levels across
	// providers that accept root-level reasoning_effort.
	if cap.Status == CapStatusUnknown {
		options = append(options, "none", "minimal", "low", "medium", "high", "xhigh", "max")
		return options
	}

	// Unsupported: only provider default is available.
	return options
}

// LabelToEffort converts a selected fzf label back to the internal effort value.
// The ProviderDefaultLabel maps to "" (omit). All other labels map to themselves.
func LabelToEffort(label string) string {
	if label == ProviderDefaultLabel {
		return ""
	}
	return label
}

// EffortToLabel converts an internal effort value to a display label.
func EffortToLabel(effort string) string {
	if effort == "" {
		return ProviderDefaultLabel
	}
	return effort
}

// DescribeEffortState returns a human-readable description of the current
// reasoning effort state for >state output.
func DescribeEffortState(effort string, cap CapabilityResult) string {
	if effort == "" {
		if cap.Status == CapStatusUnsupported {
			return "default (unsupported by current model)"
		}
		return "default"
	}

	if cap.Status == CapStatusUnknown {
		return fmt.Sprintf("%s (unverified, metadata unavailable)", effort)
	}
	return effort
}
