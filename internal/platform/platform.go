package platform

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/MehmetMHY/ch/pkg/types"
	"github.com/sashabaranov/go-openai"
)

// Manager handles AI platform operations
type Manager struct {
	client *openai.Client
	config *types.Config
}

// NewManager creates a new platform manager
func NewManager(config *types.Config) *Manager {
	return &Manager{
		config: config,
	}
}

// Initialize initializes the AI client for the current platform
func (m *Manager) Initialize() error {
	if m.config.CurrentPlatform == "openai" {
		apiKey := os.Getenv("OPENAI_API_KEY")
		if apiKey == "" {
			return fmt.Errorf("OPENAI_API_KEY environment variable is required for OpenAI platform")
		}
		m.client = openai.NewClient(apiKey)
		m.config.CurrentBaseURL = ""
		return nil
	}

	platform, exists := m.config.Platforms[m.config.CurrentPlatform]
	if !exists {
		return fmt.Errorf("platform %s not found", m.config.CurrentPlatform)
	}

	var apiKey string
	if platform.Name != "ollama" {
		apiKey = os.Getenv(platform.EnvName)
		if apiKey == "" {
			return fmt.Errorf("%s environment variable is required for %s", platform.EnvName, platform.Name)
		}
	}

	clientConfig := openai.DefaultConfig(apiKey)
	// Use CurrentBaseURL if set, otherwise use the first URL if multi-URL, otherwise use single URL
	baseURL := m.config.CurrentBaseURL
	if baseURL == "" {
		if platform.BaseURL.IsMulti() && len(platform.BaseURL.Multi) > 0 {
			baseURL = platform.BaseURL.Multi[0]
		} else {
			baseURL = platform.BaseURL.Single
		}
	}
	m.config.CurrentBaseURL = baseURL
	clientConfig.BaseURL = baseURL
	m.client = openai.NewClientWithConfig(clientConfig)

	return nil
}

// SendChatRequest sends a chat request to the current platform
func (m *Manager) SendChatRequest(messages []types.ChatMessage, model string, streamingCancel *func(), isStreaming *bool) (string, error) {
	var openaiMessages []openai.ChatCompletionMessage

	// Merge consecutive user messages to handle cases like file loading + follow-up question
	mergedMessages := m.mergeConsecutiveUserMessages(messages)

	for _, msg := range mergedMessages {
		openaiMessages = append(openaiMessages, openai.ChatCompletionMessage{
			Role:    msg.Role,
			Content: msg.Content,
		})
	}

	if m.IsReasoningModel(model) {
		return m.sendNonStreamingRequest(openaiMessages, model, streamingCancel, isStreaming)
	}

	return m.sendStreamingRequest(openaiMessages, model, streamingCancel, isStreaming)
}

// mergeConsecutiveUserMessages combines consecutive user messages into one
// This handles cases where file content is loaded as one message, then a question is asked
func (m *Manager) mergeConsecutiveUserMessages(messages []types.ChatMessage) []types.ChatMessage {
	if len(messages) <= 1 {
		return messages
	}

	var result []types.ChatMessage
	var lastUserContent []string

	for _, msg := range messages {
		if msg.Role == "user" {
			lastUserContent = append(lastUserContent, msg.Content)
		} else {
			// Non-user message: flush any accumulated user messages
			if len(lastUserContent) > 0 {
				result = append(result, types.ChatMessage{
					Role:    "user",
					Content: strings.Join(lastUserContent, "\n\n"),
				})
				lastUserContent = nil
			}
			result = append(result, msg)
		}
	}

	// Flush any remaining user messages
	if len(lastUserContent) > 0 {
		result = append(result, types.ChatMessage{
			Role:    "user",
			Content: strings.Join(lastUserContent, "\n\n"),
		})
	}

	return result
}

// modelWithTime holds a model name and its creation timestamp for sorting
type modelWithTime struct {
	name    string
	created int64
}

// parseTimestamp attempts to extract a Unix timestamp (in seconds) from a value.
// Handles: epoch seconds, milliseconds, nanoseconds (float64), and ISO 8601 strings.
// Returns 0 if the value cannot be parsed.
func parseTimestamp(val interface{}) int64 {
	switch v := val.(type) {
	case float64:
		epoch := int64(v)
		if epoch <= 0 {
			return 0
		}
		// Nanoseconds (>= 1e18, e.g. 1686588896000000000)
		if epoch >= 1e18 {
			return epoch / 1e9
		}
		// Microseconds (>= 1e15)
		if epoch >= 1e15 {
			return epoch / 1e6
		}
		// Milliseconds (>= 1e12, e.g. 1686588896000)
		if epoch >= 1e12 {
			return epoch / 1e3
		}
		// Seconds
		return epoch
	case string:
		// Try parsing as a numeric string (e.g., "1686588896" or "1.686e9")
		if numVal, err := strconv.ParseFloat(v, 64); err == nil && numVal > 0 {
			return parseTimestamp(numVal)
		}
		// Try common ISO 8601 formats
		for _, layout := range []string{
			time.RFC3339Nano,
			time.RFC3339,
			"2006-01-02T15:04:05Z",
			"2006-01-02",
		} {
			if t, err := time.Parse(layout, v); err == nil {
				return t.Unix()
			}
		}
		return 0
	default:
		return 0
	}
}

// sortModelsByTime sorts models by created timestamp descending (newest first),
// falling back to alphabetical sort when no timestamps are available
func sortModelsByTime(models []modelWithTime) []string {
	hasTimestamps := false
	for _, m := range models {
		if m.created > 0 {
			hasTimestamps = true
			break
		}
	}

	if hasTimestamps {
		sort.Slice(models, func(i, j int) bool {
			if models[i].created != models[j].created {
				return models[i].created > models[j].created
			}
			return models[i].name < models[j].name
		})
	} else {
		sort.Slice(models, func(i, j int) bool {
			return models[i].name < models[j].name
		})
	}

	names := make([]string, len(models))
	for i, m := range models {
		names[i] = m.name
	}
	return names
}

// sortModelsGroupedByPlatform sorts "platform|model" entries alphabetically by platform,
// then within each platform group by timestamp descending (newest first).
// Falls back to alphabetical within a group when no timestamps are available.
func sortModelsGroupedByPlatform(models []modelWithTime) []string {
	// Group models by platform prefix
	type group struct {
		platform string
		models   []modelWithTime
	}
	groupMap := make(map[string][]modelWithTime)
	for _, m := range models {
		parts := strings.SplitN(m.name, "|", 2)
		plat := ""
		if len(parts) == 2 {
			plat = parts[0]
		}
		groupMap[plat] = append(groupMap[plat], m)
	}

	// Get sorted platform names
	var platformNames []string
	for p := range groupMap {
		platformNames = append(platformNames, p)
	}
	sort.Strings(platformNames)

	// Sort within each group, then concatenate
	var result []string
	for _, p := range platformNames {
		result = append(result, sortModelsByTime(groupMap[p])...)
	}
	return result
}

// ListModels returns available models for the current platform
func (m *Manager) ListModels() ([]string, error) {
	if m.config.CurrentPlatform == "openai" {
		models, err := m.client.ListModels(context.Background())
		if err != nil {
			return nil, err
		}

		var modelsWithTime []modelWithTime
		for _, model := range models.Models {
			modelsWithTime = append(modelsWithTime, modelWithTime{
				name:    model.ID,
				created: model.CreatedAt,
			})
		}
		return sortModelsByTime(modelsWithTime), nil
	}

	platform := m.config.Platforms[m.config.CurrentPlatform]
	models, err := m.fetchPlatformModelsWithTime(platform)
	if err != nil {
		return nil, err
	}
	return sortModelsByTime(models), nil
}

// SelectPlatform handles platform selection and model selection
func (m *Manager) SelectPlatform(platformKey, modelName string, fzfSelector func([]string, string) (string, error)) (map[string]interface{}, error) {
	platformChanged := false
	if platformKey == "" {
		var platforms []string
		platforms = append(platforms, "openai")
		for name := range m.config.Platforms {
			platforms = append(platforms, name)
		}
		sort.Strings(platforms)

		selected, err := fzfSelector(platforms, "platform: ")
		if err != nil {
			return nil, err
		}

		if selected == "" {
			return nil, fmt.Errorf("no platform selected")
		}

		platformKey = selected
		platformChanged = true
	}

	if platformKey == "openai" {
		finalModel := modelName
		if platformChanged || finalModel == "" {
			apiKey := os.Getenv("OPENAI_API_KEY")
			if apiKey == "" {
				return nil, fmt.Errorf("OPENAI_API_KEY environment variable is required for OpenAI platform")
			}
			client := openai.NewClient(apiKey)
			models, err := client.ListModels(context.Background())
			if err != nil {
				return nil, err
			}

			var modelsWithTime []modelWithTime
			for _, model := range models.Models {
				modelsWithTime = append(modelsWithTime, modelWithTime{
					name:    model.ID,
					created: model.CreatedAt,
				})
			}
			modelNames := sortModelsByTime(modelsWithTime)

			selected, err := fzfSelector(modelNames, "model: ")
			if err != nil {
				return nil, err
			}

			if selected == "" {
				return nil, fmt.Errorf("no model selected")
			}
			finalModel = selected
		}

		return map[string]interface{}{
			"platform_name": "openai",
			"picked_model":  finalModel,
			"base_url":      "",
			"env_name":      "OPENAI_API_KEY",
		}, nil
	}

	platform, exists := m.config.Platforms[platformKey]
	if !exists {
		return nil, fmt.Errorf("platform %s not supported", platformKey)
	}

	// Handle multi-URL platforms (e.g., Amazon Bedrock with multiple regions)
	selectedURL := platform.BaseURL.Single
	if platform.BaseURL.IsMulti() {
		selected, err := fzfSelector(platform.BaseURL.Multi, "region: ")
		if err != nil {
			return nil, err
		}

		if selected == "" {
			return nil, fmt.Errorf("no region selected")
		}

		selectedURL = selected
	}

	finalModel := modelName
	var modelsList []string

	if finalModel == "" || platformChanged {
		modelsWithTime, err := m.fetchPlatformModelsWithTime(platform)
		if err != nil {
			return nil, fmt.Errorf("failed to retrieve models: %v", err)
		}

		if len(modelsWithTime) == 0 {
			return nil, fmt.Errorf("no models found or returned in unexpected format")
		}
		modelsList = sortModelsByTime(modelsWithTime)

		selected, err := fzfSelector(modelsList, "model: ")
		if err != nil {
			return nil, err
		}

		if selected == "" {
			return nil, fmt.Errorf("no model selected")
		}

		finalModel = selected
	}

	return map[string]interface{}{
		"platform_name": platformKey,
		"picked_model":  finalModel,
		"base_url":      selectedURL,
		"env_name":      platform.EnvName,
		"models":        modelsList,
	}, nil
}

// FetchAllModelsAsync fetches all models from all platforms asynchronously
// Returns a list of models formatted as "platform|model_name" sorted by newest first
// Only fetches from platforms where API keys are defined and not empty
func (m *Manager) FetchAllModelsAsync() ([]string, error) {
	var wg sync.WaitGroup
	results := make(chan modelWithTime)
	done := make(chan bool)
	var models []modelWithTime
	var mu sync.Mutex

	// Create a list of all platforms to fetch from
	platformsToFetch := []struct {
		name     string
		platform types.Platform
	}{
		{"openai", types.Platform{}}, // OpenAI is a special case
	}

	// Add other platforms from config
	for name, platform := range m.config.Platforms {
		platformsToFetch = append(platformsToFetch, struct {
			name     string
			platform types.Platform
		}{name, platform})
	}

	// Goroutine to collect results
	go func() {
		for model := range results {
			mu.Lock()
			models = append(models, model)
			mu.Unlock()
		}
		done <- true
	}()

	// Fetch models from each platform concurrently
	for _, p := range platformsToFetch {
		platformName := p.name
		platformConfig := p.platform

		wg.Add(1)
		go func(name string, config types.Platform) {
			defer wg.Done()

			// Special handling for OpenAI
			if name == "openai" {
				apiKey := os.Getenv("OPENAI_API_KEY")
				if apiKey == "" {
					return // Skip if API key is not set
				}

				client := openai.NewClient(apiKey)
				ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
				defer cancel()

				modelList, err := client.ListModels(ctx)
				if err != nil {
					return // Silently ignore errors
				}

				for _, model := range modelList.Models {
					platformNameFormatted := strings.ReplaceAll(name, " ", "-")
					results <- modelWithTime{
						name:    fmt.Sprintf("%s|%s", platformNameFormatted, model.ID),
						created: model.CreatedAt,
					}
				}
				return
			}

			// Check if API key is defined and not empty
			apiKey := os.Getenv(platformConfig.EnvName)
			if apiKey == "" && platformConfig.Name != "ollama" {
				return // Skip if API key is not set
			}

			// Fetch models from this platform
			modelList, err := m.fetchPlatformModelsWithTime(platformConfig)
			if err != nil {
				return // Silently ignore errors
			}

			for _, model := range modelList {
				platformNameFormatted := strings.ReplaceAll(name, " ", "-")
				results <- modelWithTime{
					name:    fmt.Sprintf("%s|%s", platformNameFormatted, model.name),
					created: model.created,
				}
			}
		}(platformName, platformConfig)
	}

	// Close results channel when all goroutines are done
	go func() {
		wg.Wait()
		close(results)
	}()

	// Wait for collector to finish
	<-done

	if len(models) == 0 {
		return nil, fmt.Errorf("no models found from any platform")
	}

	return sortModelsGroupedByPlatform(models), nil
}

// isSlowModel checks if the model matches any user-configured slow model patterns.
// Patterns are configured via "slow_model_patterns" in ~/.ch/config.json.
// By default, no models are considered slow (empty list).
func (m *Manager) isSlowModel(modelName string) bool {
	for _, pattern := range m.config.SlowModelPatterns {
		matched, _ := regexp.MatchString(pattern, modelName)
		if matched {
			return true
		}
	}
	return false
}

// IsReasoningModel checks if the model is a reasoning model (like o1, o2, etc.)
func (m *Manager) IsReasoningModel(modelName string) bool {
	return m.isSlowModel(modelName)
}

func (m *Manager) sendNonStreamingRequest(openaiMessages []openai.ChatCompletionMessage, model string, streamingCancel *func(), isStreaming *bool) (string, error) {
	req := openai.ChatCompletionRequest{
		Model:    model,
		Messages: openaiMessages,
		Stream:   false,
	}

	ctx, cancel := context.WithCancel(context.Background())
	*isStreaming = true
	*streamingCancel = cancel

	resp, err := m.client.CreateChatCompletion(ctx, req)

	*isStreaming = false
	*streamingCancel = nil

	if err != nil {
		if ctx.Err() == context.Canceled {
			return "", fmt.Errorf("request was interrupted")
		}
		return "", err
	}

	if len(resp.Choices) > 0 {
		fullResponse := resp.Choices[0].Message.Content
		return fullResponse, nil
	}

	return "", fmt.Errorf("no response content")
}

func (m *Manager) sendStreamingRequest(openaiMessages []openai.ChatCompletionMessage, model string, streamingCancel *func(), isStreaming *bool) (string, error) {
	req := openai.ChatCompletionRequest{
		Model:    model,
		Messages: openaiMessages,
		Stream:   true,
	}

	ctx, cancel := context.WithCancel(context.Background())
	*isStreaming = true
	*streamingCancel = cancel

	stream, err := m.client.CreateChatCompletionStream(ctx, req)
	if err != nil {
		*isStreaming = false
		*streamingCancel = nil
		return "", err
	}
	defer func() {
		stream.Close()
		*isStreaming = false
		*streamingCancel = nil
	}()

	type streamChunk struct {
		Choices []struct {
			Delta struct {
				Content          string `json:"content"`
				ReasoningContent string `json:"reasoning_content"`
				Reasoning        string `json:"reasoning"`
			} `json:"delta"`
		} `json:"choices"`
	}

	var response strings.Builder
	wasReasoning := false
	lastReasoningEndsWithNewline := false
	insideThinkTag := false
	justExitedThinkTag := false

	for {
		rawBytes, err := stream.RecvRaw()
		if err != nil {
			if err == io.EOF {
				break
			}
			if ctx.Err() == context.Canceled {
				return response.String(), nil
			}
			return "", err
		}

		var chunk streamChunk
		if err := json.Unmarshal(rawBytes, &chunk); err != nil || len(chunk.Choices) == 0 {
			continue
		}

		delta := chunk.Choices[0].Delta
		reasoning := delta.Reasoning + delta.ReasoningContent

		if reasoning != "" {
			wasReasoning = true
			lastReasoningEndsWithNewline = strings.HasSuffix(reasoning, "\n")
			if m.config.ShowThinking {
				if m.config.IsPipedOutput {
					fmt.Print(reasoning)
				} else {
					fmt.Print("\033[90m" + reasoning + "\033[0m")
				}
			}
			response.WriteString(reasoning)
		}

		if delta.Content != "" {
			if wasReasoning && !lastReasoningEndsWithNewline && m.config.ShowThinking {
				fmt.Println()
			}
			wasReasoning = false

			if strings.Contains(delta.Content, "<think>") {
				insideThinkTag = true
			}

			if justExitedThinkTag {
				delta.Content = strings.TrimLeft(delta.Content, "\n\r ")
				if delta.Content == "" {
					continue
				}
				justExitedThinkTag = false
			}

			if insideThinkTag && !m.config.ShowThinking {
				// Skip displaying think-tagged content
			} else if m.config.IsPipedOutput {
				fmt.Print(delta.Content)
			} else if insideThinkTag {
				fmt.Print("\033[90m" + delta.Content + "\033[0m")
			} else {
				fmt.Print("\033[92m" + delta.Content + "\033[0m")
			}

			if strings.Contains(delta.Content, "</think>") {
				insideThinkTag = false
				if !m.config.ShowThinking {
					justExitedThinkTag = true
				}
			}

			response.WriteString(delta.Content)
		}
	}

	fmt.Println()
	return response.String(), nil
}

// fetchPlatformModelsWithTime fetches models with their creation timestamps
func (m *Manager) fetchPlatformModelsWithTime(platform types.Platform) ([]modelWithTime, error) {
	httpClient := &http.Client{Timeout: 10 * time.Second}

	apiKey := os.Getenv(platform.EnvName)
	if apiKey == "" && platform.Name != "ollama" {
		return nil, fmt.Errorf("%s environment variable not set", platform.EnvName)
	}

	// Handle Google's special URL with API key in query parameter
	url := platform.Models.URL
	if platform.Name == "google" {
		url = strings.Replace(url, "https://generativelanguage.googleapis.com/v1beta/models", "https://generativelanguage.googleapis.com/v1beta/models?key="+apiKey, 1)
	}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	if platform.Name == "anthropic" {
		req.Header.Set("x-api-key", apiKey)
		req.Header.Set("anthropic-version", "2023-06-01")
	} else if platform.Name != "ollama" && platform.Name != "google" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var jsonData interface{}
	err = json.Unmarshal(body, &jsonData)
	if err != nil {
		return nil, err
	}

	return m.extractModelsWithTimeFromJSON(jsonData, platform.Models.JSONPath)
}

// extractModelsWithTimeFromJSON extracts model names and their creation timestamps from JSON.
// Tries timestamp fields in priority order: "created", "created_at", "modified_at".
// Handles epoch numbers (seconds/ms/ns) and ISO 8601 strings.
func (m *Manager) extractModelsWithTimeFromJSON(data interface{}, jsonPath string) ([]modelWithTime, error) {
	parts := strings.Split(jsonPath, ".")

	current := data

	for i, part := range parts[:len(parts)-1] {
		if dataMap, ok := current.(map[string]interface{}); ok {
			if val, exists := dataMap[part]; exists {
				current = val
			} else {
				return nil, fmt.Errorf("path part %s not found", part)
			}
		} else {
			return nil, fmt.Errorf("expected object at part %d", i)
		}
	}

	fieldName := parts[len(parts)-1]
	timestampFields := []string{"created", "created_at", "modified_at"}
	var models []modelWithTime

	if dataArray, ok := current.([]interface{}); ok {
		for _, item := range dataArray {
			if itemMap, ok := item.(map[string]interface{}); ok {
				if modelName, exists := itemMap[fieldName]; exists {
					if nameStr, ok := modelName.(string); ok {
						var created int64
						for _, field := range timestampFields {
							if val, exists := itemMap[field]; exists {
								if ts := parseTimestamp(val); ts > 0 {
									created = ts
									break
								}
							}
						}
						models = append(models, modelWithTime{
							name:    nameStr,
							created: created,
						})
					}
				}
			}
		}
	}

	return models, nil
}
