package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/MehmetMHY/ch/pkg/types"
)

// ---- mergeConfigs ----

func TestMergeConfigs_BasicOverrides(t *testing.T) {
	def := &types.Config{
		DefaultModel:    "gpt-5.4-mini",
		CurrentModel:    "gpt-5.4-mini",
		CurrentBaseURL:  "https://default.example/v1",
		SystemPrompt:    "Default Prompt",
		ExitKey:         "!q",
		CurrentPlatform: "openai",
		Platforms:       map[string]types.Platform{},
	}
	user := &types.Config{
		DefaultModel:    "custom-model",
		CurrentBaseURL:  "https://custom.example/v1",
		SystemPrompt:    "Custom Prompt",
		CurrentPlatform: "groq",
	}

	merged := mergeConfigs(def, user)

	if merged.DefaultModel != "custom-model" {
		t.Errorf("DefaultModel: got %q, want %q", merged.DefaultModel, "custom-model")
	}
	// CurrentModel should follow DefaultModel when not explicitly set
	if merged.CurrentModel != "custom-model" {
		t.Errorf("CurrentModel: got %q, want %q", merged.CurrentModel, "custom-model")
	}
	if merged.SystemPrompt != "Custom Prompt" {
		t.Errorf("SystemPrompt: got %q, want %q", merged.SystemPrompt, "Custom Prompt")
	}
	if merged.CurrentPlatform != "groq" {
		t.Errorf("CurrentPlatform: got %q, want %q", merged.CurrentPlatform, "groq")
	}
	if merged.CurrentBaseURL != "https://custom.example/v1" {
		t.Errorf("CurrentBaseURL: got %q, want %q", merged.CurrentBaseURL, "https://custom.example/v1")
	}
	// ExitKey from default must not be wiped
	if merged.ExitKey != "!q" {
		t.Errorf("ExitKey should be preserved from default, got %q", merged.ExitKey)
	}
}

func TestMergeConfigs_CurrentModelExplicit(t *testing.T) {
	def := &types.Config{
		DefaultModel: "gpt-5.4-mini",
		CurrentModel: "gpt-5.4-mini",
		Platforms:    map[string]types.Platform{},
	}
	user := &types.Config{
		DefaultModel: "base-model",
		CurrentModel: "override-model",
		// CurrentPlatform is set so that bool-flag merging triggers
		CurrentPlatform: "openai",
	}
	merged := mergeConfigs(def, user)
	if merged.CurrentModel != "override-model" {
		t.Errorf("CurrentModel: got %q, want %q", merged.CurrentModel, "override-model")
	}
}

func TestMergeConfigs_SlowModelPatterns(t *testing.T) {
	def := &types.Config{
		Platforms: map[string]types.Platform{},
	}
	user := &types.Config{
		CurrentPlatform:   "openai",
		SlowModelPatterns: []string{`^o\d+`, `^gpt-5$`},
	}
	merged := mergeConfigs(def, user)
	if len(merged.SlowModelPatterns) != 2 {
		t.Fatalf("SlowModelPatterns len: got %d, want 2", len(merged.SlowModelPatterns))
	}
	if merged.SlowModelPatterns[0] != `^o\d+` {
		t.Errorf("SlowModelPatterns[0]: got %q, want %q", merged.SlowModelPatterns[0], `^o\d+`)
	}
}

func TestMergeConfigs_AINameFields(t *testing.T) {
	def := &types.Config{
		AINameEnable:         false,
		AINameCharThreshold:  500,
		AINameCount:          8,
		AINameTimeoutSeconds: 15,
		AINamePrompt:         "default prompt",
		Platforms:            map[string]types.Platform{},
	}
	user := &types.Config{
		CurrentPlatform:      "openai",
		AINameEnable:         true,
		AINameCharThreshold:  200,
		AINameCount:          5,
		AINameTimeoutSeconds: 30,
		AINamePrompt:         "custom prompt",
	}
	merged := mergeConfigs(def, user)
	if !merged.AINameEnable {
		t.Error("AINameEnable should be true")
	}
	if merged.AINameCharThreshold != 200 {
		t.Errorf("AINameCharThreshold: got %d, want 200", merged.AINameCharThreshold)
	}
	if merged.AINameCount != 5 {
		t.Errorf("AINameCount: got %d, want 5", merged.AINameCount)
	}
	if merged.AINameTimeoutSeconds != 30 {
		t.Errorf("AINameTimeoutSeconds: got %d, want 30", merged.AINameTimeoutSeconds)
	}
	if merged.AINamePrompt != "custom prompt" {
		t.Errorf("AINamePrompt: got %q, want %q", merged.AINamePrompt, "custom prompt")
	}
}

func TestMergeConfigs_ShowSearchResultsAndMuteNotifications(t *testing.T) {
	def := &types.Config{
		ShowSearchResults: true,
		MuteNotifications: false,
		Platforms:         map[string]types.Platform{},
	}
	// User config with a real field set (CurrentPlatform) so bool-flag merge triggers
	user := &types.Config{
		CurrentPlatform:   "openai",
		ShowSearchResults: false,
		MuteNotifications: true,
	}
	merged := mergeConfigs(def, user)
	if merged.ShowSearchResults {
		t.Error("ShowSearchResults should be false after merge")
	}
	if !merged.MuteNotifications {
		t.Error("MuteNotifications should be true after merge")
	}
}

func TestMergeConfigs_EnableSessionSaveAndShowThinking(t *testing.T) {
	def := &types.Config{
		EnableSessionSave: false,
		ShowThinking:      true,
		Platforms:         map[string]types.Platform{},
	}
	user := &types.Config{
		CurrentPlatform:   "openai",
		EnableSessionSave: true,
		ShowThinking:      false,
	}
	merged := mergeConfigs(def, user)
	if !merged.EnableSessionSave {
		t.Error("EnableSessionSave should be true")
	}
	if merged.ShowThinking {
		t.Error("ShowThinking should be false")
	}
}

func TestMergeConfigs_PlatformMerge(t *testing.T) {
	existing := types.Platform{Name: "groq"}
	def := &types.Config{
		Platforms: map[string]types.Platform{"groq": existing},
	}
	user := &types.Config{
		CurrentPlatform: "openai",
		Platforms: map[string]types.Platform{
			"custom": {Name: "custom"},
		},
	}
	merged := mergeConfigs(def, user)
	if _, ok := merged.Platforms["groq"]; !ok {
		t.Error("existing platform 'groq' should be preserved")
	}
	if _, ok := merged.Platforms["custom"]; !ok {
		t.Error("user platform 'custom' should be added")
	}
}

func TestMergeConfigs_ShallowLoadDirsOverride(t *testing.T) {
	def := &types.Config{
		ShallowLoadDirs: []string{"/var/", "/tmp/"},
		Platforms:       map[string]types.Platform{},
	}
	user := &types.Config{
		CurrentPlatform: "openai",
		ShallowLoadDirs: []string{"/custom/dir"},
	}
	merged := mergeConfigs(def, user)
	if len(merged.ShallowLoadDirs) != 1 || merged.ShallowLoadDirs[0] != "/custom/dir" {
		t.Errorf("ShallowLoadDirs: got %v, want [/custom/dir]", merged.ShallowLoadDirs)
	}
}

func TestMergeConfigs_EmptyUserConfig(t *testing.T) {
	// An empty user config must not wipe defaults
	def := &types.Config{
		DefaultModel:      "gpt-5.4-mini",
		CurrentModel:      "gpt-5.4-mini",
		SystemPrompt:      "Helpful assistant",
		ShowSearchResults: true,
		ShowThinking:      true,
		EnableSessionSave: true,
		NumSearchResults:  5,
		Platforms:         map[string]types.Platform{},
	}
	user := &types.Config{}
	merged := mergeConfigs(def, user)
	if merged.DefaultModel != "gpt-5.4-mini" {
		t.Errorf("DefaultModel should be preserved, got %q", merged.DefaultModel)
	}
	if merged.NumSearchResults != 5 {
		t.Errorf("NumSearchResults should be preserved, got %d", merged.NumSearchResults)
	}
	if !merged.ShowSearchResults {
		t.Error("ShowSearchResults should be preserved")
	}
	if !merged.ShowThinking {
		t.Error("ShowThinking should be preserved")
	}
	if !merged.EnableSessionSave {
		t.Error("EnableSessionSave should be preserved")
	}
}

func TestMergeConfigs_BoolOnlyUserConfigOverridesExplicitFlags(t *testing.T) {
	def := &types.Config{
		EnableSessionSave: true,
		ShowThinking:      true,
		Platforms:         map[string]types.Platform{},
	}
	user := &types.Config{
		EnableSessionSave:  false,
		ShowThinking:       false,
		ExplicitBoolFields: map[string]bool{"enable_session_save": true, "show_thinking": true},
	}
	merged := mergeConfigs(def, user)
	if merged.EnableSessionSave {
		t.Error("EnableSessionSave should be false when explicitly configured")
	}
	if merged.ShowThinking {
		t.Error("ShowThinking should be false when explicitly configured")
	}
}

// ShowSearchResults has its own escape hatch in the gate (|| userConfig.ShowSearchResults),
// so setting it true should take effect even without an identifying string field.
func TestMergeConfigs_ShowSearchResultsTrueSelfTriggers(t *testing.T) {
	def := &types.Config{
		ShowSearchResults: false,
		Platforms:         map[string]types.Platform{},
	}
	user := &types.Config{
		ShowSearchResults: true,
	}
	merged := mergeConfigs(def, user)
	if !merged.ShowSearchResults {
		t.Error("ShowSearchResults=true should self-trigger its own merge")
	}
}

func TestMergeConfigs_ShallowLoadDirsCanBeCleared(t *testing.T) {
	def := &types.Config{
		ShallowLoadDirs: []string{"/", "/home/"},
		Platforms:       map[string]types.Platform{},
	}
	user := &types.Config{
		ShallowLoadDirs: []string{},
	}
	merged := mergeConfigs(def, user)
	if merged.ShallowLoadDirs == nil {
		t.Fatal("ShallowLoadDirs should be an empty slice, not nil")
	}
	if len(merged.ShallowLoadDirs) != 0 {
		t.Errorf("ShallowLoadDirs should be cleared, got %v", merged.ShallowLoadDirs)
	}
}

// ---- DefaultConfig / env overrides ----

func TestDefaultConfig_FileOverride(t *testing.T) {
	tempHome := t.TempDir()
	t.Setenv("HOME", tempHome)
	t.Setenv("USERPROFILE", tempHome)
	t.Setenv("CH_DEFAULT_PLATFORM", "")
	t.Setenv("CH_DEFAULT_MODEL", "")

	chDir := filepath.Join(tempHome, ".ch")
	if err := os.MkdirAll(chDir, 0755); err != nil {
		t.Fatalf("failed to create mock .ch dir: %v", err)
	}

	mockUserConfig := types.Config{
		DefaultModel:    "grok-4-fast-non-reasoning",
		CurrentPlatform: "xai",
		SystemPrompt:    "Override Prompt",
	}
	data, _ := json.Marshal(mockUserConfig)
	if err := os.WriteFile(filepath.Join(chDir, "config.json"), data, 0644); err != nil {
		t.Fatalf("failed to write mock config.json: %v", err)
	}

	cfg := DefaultConfig()
	if cfg.DefaultModel != "grok-4-fast-non-reasoning" {
		t.Errorf("DefaultModel: got %q, want %q", cfg.DefaultModel, "grok-4-fast-non-reasoning")
	}
	if cfg.CurrentPlatform != "xai" {
		t.Errorf("CurrentPlatform: got %q, want %q", cfg.CurrentPlatform, "xai")
	}
	if cfg.SystemPrompt != "Override Prompt" {
		t.Errorf("SystemPrompt: got %q, want %q", cfg.SystemPrompt, "Override Prompt")
	}
}

func TestDefaultConfig_EnvOverrides(t *testing.T) {
	tempHome := t.TempDir()
	t.Setenv("HOME", tempHome)
	t.Setenv("USERPROFILE", tempHome)
	t.Setenv("CH_DEFAULT_PLATFORM", "ollama")
	t.Setenv("CH_DEFAULT_MODEL", "llama3")

	cfg := DefaultConfig()
	if cfg.CurrentPlatform != "ollama" {
		t.Errorf("CurrentPlatform: got %q, want %q", cfg.CurrentPlatform, "ollama")
	}
	if cfg.DefaultModel != "llama3" {
		t.Errorf("DefaultModel: got %q, want %q", cfg.DefaultModel, "llama3")
	}
	if cfg.CurrentModel != "llama3" {
		t.Errorf("CurrentModel: got %q, want %q", cfg.CurrentModel, "llama3")
	}
}

func TestDefaultConfig_BoolOnlyConfigFileOverrides(t *testing.T) {
	tempHome := t.TempDir()
	t.Setenv("HOME", tempHome)
	t.Setenv("USERPROFILE", tempHome)
	t.Setenv("CH_DEFAULT_PLATFORM", "")
	t.Setenv("CH_DEFAULT_MODEL", "")

	chDir := filepath.Join(tempHome, ".ch")
	if err := os.MkdirAll(chDir, 0755); err != nil {
		t.Fatalf("failed to create mock .ch dir: %v", err)
	}

	data := []byte(`{"enable_session_save":true,"save_all_sessions":true,"show_thinking":false,"show_search_results":false}`)
	if err := os.WriteFile(filepath.Join(chDir, "config.json"), data, 0644); err != nil {
		t.Fatalf("failed to write mock config.json: %v", err)
	}

	cfg := DefaultConfig()
	if !cfg.EnableSessionSave {
		t.Error("EnableSessionSave should be true from bool-only config")
	}
	if !cfg.SaveAllSessions {
		t.Error("SaveAllSessions should be true from bool-only config")
	}
	if cfg.ShowThinking {
		t.Error("ShowThinking should be false from bool-only config")
	}
	if cfg.ShowSearchResults {
		t.Error("ShowSearchResults should be false from bool-only config")
	}
}

func TestDefaultConfig_NoConfigFile(t *testing.T) {
	tempHome := t.TempDir()
	t.Setenv("HOME", tempHome)
	t.Setenv("USERPROFILE", tempHome)
	t.Setenv("CH_DEFAULT_PLATFORM", "")
	t.Setenv("CH_DEFAULT_MODEL", "")

	// No config.json exists – should get pure defaults
	cfg := DefaultConfig()
	if cfg.DefaultModel == "" {
		t.Error("DefaultModel should not be empty with pure defaults")
	}
	if cfg.ExitKey != "!q" {
		t.Errorf("ExitKey: got %q, want %q", cfg.ExitKey, "!q")
	}
	if cfg.NumSearchResults <= 0 {
		t.Errorf("NumSearchResults should be positive, got %d", cfg.NumSearchResults)
	}
	// All builtin platforms must be present
	for _, p := range []string{"groq", "openrouter", "deepseek", "anthropic", "xai", "ollama", "together", "google", "mistral", "amazon"} {
		if _, ok := cfg.Platforms[p]; !ok {
			t.Errorf("platform %q missing from default config", p)
		}
	}
}

func TestDefaultConfig_CorruptConfigFile(t *testing.T) {
	tempHome := t.TempDir()
	t.Setenv("HOME", tempHome)
	t.Setenv("USERPROFILE", tempHome)
	t.Setenv("CH_DEFAULT_PLATFORM", "")
	t.Setenv("CH_DEFAULT_MODEL", "")

	chDir := filepath.Join(tempHome, ".ch")
	os.MkdirAll(chDir, 0755)
	os.WriteFile(filepath.Join(chDir, "config.json"), []byte("not json {{"), 0644)

	// Should fall back to pure defaults without panicking
	cfg := DefaultConfig()
	if cfg == nil {
		t.Fatal("expected non-nil config even with corrupt file")
	}
	if cfg.ExitKey != "!q" {
		t.Errorf("ExitKey should be default '!q', got %q", cfg.ExitKey)
	}
}

// ---- InitializeAppState ----

func TestInitializeAppState(t *testing.T) {
	tempHome := t.TempDir()
	t.Setenv("HOME", tempHome)
	t.Setenv("USERPROFILE", tempHome)
	t.Setenv("CH_DEFAULT_PLATFORM", "")
	t.Setenv("CH_DEFAULT_MODEL", "")

	state := InitializeAppState()
	if state == nil {
		t.Fatal("expected non-nil AppState")
	}
	if len(state.Messages) != 1 || state.Messages[0].Role != "system" {
		t.Errorf("expected single system message, got %v", state.Messages)
	}
	if len(state.ChatHistory) != 1 || state.ChatHistory[0].User != state.Config.SystemPrompt {
		t.Errorf("expected single system history entry, got %v", state.ChatHistory)
	}
	if state.IsStreaming {
		t.Error("IsStreaming should default to false")
	}
	if state.IsExecutingCommand {
		t.Error("IsExecutingCommand should default to false")
	}
}
