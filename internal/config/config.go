package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"github.com/MehmetMHY/ch/pkg/types"
)

// loadConfigFromFile loads configuration from config.json in ~/.ch/ directory
func loadConfigFromFile() (*types.Config, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}

	chDir := filepath.Join(homeDir, ".ch")
	configPath := filepath.Join(chDir, "config.json")

	// Create ~/.ch directory if it doesn't exist
	if err := os.MkdirAll(chDir, 0755); err != nil {
		return nil, err
	}

	// Return empty config if file doesn't exist
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return &types.Config{}, nil
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, err
	}

	var config types.Config
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, err
	}

	return &config, nil
}

// mergeConfigs merges user config with default config, user config takes precedence
func mergeConfigs(defaultConfig, userConfig *types.Config) *types.Config {
	if userConfig.DefaultModel != "" {
		defaultConfig.DefaultModel = userConfig.DefaultModel
		// If current_model isn't explicitly set in user config, use the default_model
		if userConfig.CurrentModel == "" {
			defaultConfig.CurrentModel = userConfig.DefaultModel
		}
	}
	if userConfig.CurrentModel != "" {
		defaultConfig.CurrentModel = userConfig.CurrentModel
	}
	if userConfig.SystemPrompt != "" {
		defaultConfig.SystemPrompt = userConfig.SystemPrompt
	}
	if userConfig.ExitKey != "" {
		defaultConfig.ExitKey = userConfig.ExitKey
	}
	if userConfig.ModelSwitch != "" {
		defaultConfig.ModelSwitch = userConfig.ModelSwitch
	}
	if userConfig.EditorInput != "" {
		defaultConfig.EditorInput = userConfig.EditorInput
	}
	if userConfig.ClearHistory != "" {
		defaultConfig.ClearHistory = userConfig.ClearHistory
	}
	if userConfig.HelpKey != "" {
		defaultConfig.HelpKey = userConfig.HelpKey
	}
	if userConfig.ExportChat != "" {
		defaultConfig.ExportChat = userConfig.ExportChat
	}
	if userConfig.Backtrack != "" {
		defaultConfig.Backtrack = userConfig.Backtrack
	}
	if userConfig.SaveHistory != "" {
		defaultConfig.SaveHistory = userConfig.SaveHistory
	}
	if userConfig.WebSearch != "" {
		defaultConfig.WebSearch = userConfig.WebSearch
	}
	if userConfig.NumSearchResults != 0 {
		defaultConfig.NumSearchResults = userConfig.NumSearchResults
	}
	if userConfig.ScrapeURL != "" {
		defaultConfig.ScrapeURL = userConfig.ScrapeURL
	}
	if userConfig.CopyToClipboard != "" {
		defaultConfig.CopyToClipboard = userConfig.CopyToClipboard
	}
	if userConfig.LoadFiles != "" {
		defaultConfig.LoadFiles = userConfig.LoadFiles
	}
	if userConfig.LoadFilesAdv != "" {
		defaultConfig.LoadFilesAdv = userConfig.LoadFilesAdv
	}
	if userConfig.AnswerSearch != "" {
		defaultConfig.AnswerSearch = userConfig.AnswerSearch
	}
	if userConfig.PlatformSwitch != "" {
		defaultConfig.PlatformSwitch = userConfig.PlatformSwitch
	}
	if userConfig.CodeDump != "" {
		defaultConfig.CodeDump = userConfig.CodeDump
	}
	if userConfig.ShellRecord != "" {
		defaultConfig.ShellRecord = userConfig.ShellRecord
	}
	if userConfig.ShellOption != "" {
		defaultConfig.ShellOption = userConfig.ShellOption
	}
	if userConfig.LoadHistory != "" {
		defaultConfig.LoadHistory = userConfig.LoadHistory
	}
	if userConfig.EditorAlias != "" {
		defaultConfig.EditorAlias = userConfig.EditorAlias
	}
	if userConfig.MultiLine != "" {
		defaultConfig.MultiLine = userConfig.MultiLine
	}
	if userConfig.ShowState != "" {
		defaultConfig.ShowState = userConfig.ShowState
	}
	if userConfig.PreferredEditor != "" {
		defaultConfig.PreferredEditor = userConfig.PreferredEditor
	}
	if userConfig.CurrentPlatform != "" {
		defaultConfig.CurrentPlatform = userConfig.CurrentPlatform
	}
	// Note: ShowSearchResults is a bool, so we need to check if it was explicitly set
	// In JSON, omitempty will skip false values, but we can't distinguish between
	// unset and explicitly false. For now, we'll always use the default unless true is set.
	defaultConfig.ShowSearchResults = defaultConfig.ShowSearchResults || userConfig.ShowSearchResults

	// Merge platforms if provided
	if userConfig.Platforms != nil {
		for name, platform := range userConfig.Platforms {
			defaultConfig.Platforms[name] = platform
		}
	}

	return defaultConfig
}

// DefaultConfig returns the default configuration merged with user config from config.json
func DefaultConfig() *types.Config {
	// Get default platform from environment variable, fallback to default platform
	defaultPlatform := os.Getenv("CH_DEFAULT_PLATFORM")
	if defaultPlatform == "" {
		defaultPlatform = "openai"
	}

	// Get default model from environment variable, fallback to hardcoded default model
	defaultModel := os.Getenv("CH_DEFAULT_MODEL")
	if defaultModel == "" {
		defaultModel = "gpt-4.1-mini"
	}

	defaultConfig := &types.Config{
		OpenAIAPIKey:      "", // API keys are fetched per-platform in Initialize()
		DefaultModel:      defaultModel,
		CurrentModel:      defaultModel,
		SystemPrompt:      "You are a helpful assistant powered by Ch who provides concise, clear, and accurate answers. Be brief, but ensure the response fully addresses the question without leaving out important details. Always return any code or file output in a Markdown code fence, with syntax ```<language or filetype>\n...``` so it can be parsed automatically. Only do this when needed, no need to do this for responses just code segments and/or when directly asked to do so from the user.",
		ExitKey:           "!q",
		ModelSwitch:       "!m",
		EditorInput:       "!t",
		ClearHistory:      "!c",
		HelpKey:           "!h",
		ExportChat:        "!e",
		Backtrack:         "!b",
		SaveHistory:       "!z",
		WebSearch:         "!w",
		ShowSearchResults: false,
		NumSearchResults:  5,
		ScrapeURL:         "!s",
		CopyToClipboard:   "!y",
		LoadFiles:         "!l",
		LoadFilesAdv:      "!f",
		AnswerSearch:      "!a",
		PlatformSwitch:    "!p",
		CodeDump:          "!d",
		ShellRecord:       "!x",
		ShellOption:       "!x",
		LoadHistory:       "!r",
		EditorAlias:       "!v",
		MultiLine:         "\\",
		ShowState:         "!i",
		PreferredEditor:   "hx",
		CurrentPlatform:   defaultPlatform,
		Platforms: map[string]types.Platform{
			"groq": {
				Name:    "groq",
				BaseURL: "https://api.groq.com/openai/v1",
				EnvName: "GROQ_API_KEY",
				Models: types.PlatformModels{
					URL:      "https://api.groq.com/openai/v1/models",
					JSONPath: "data.id",
				},
			},
			"deepseek": {
				Name:    "deepseek",
				BaseURL: "https://api.deepseek.com",
				EnvName: "DEEP_SEEK_API_KEY",
				Models: types.PlatformModels{
					URL:      "https://api.deepseek.com/models",
					JSONPath: "data.id",
				},
			},
			"anthropic": {
				Name:    "anthropic",
				BaseURL: "https://api.anthropic.com/v1/",
				EnvName: "ANTHROPIC_API_KEY",
				Models: types.PlatformModels{
					URL:      "https://api.anthropic.com/v1/models",
					JSONPath: "data.id",
				},
			},
			"xai": {
				Name:    "xai",
				BaseURL: "https://api.x.ai/v1",
				EnvName: "XAI_API_KEY",
				Models: types.PlatformModels{
					URL:      "https://api.x.ai/v1/models",
					JSONPath: "data.id",
				},
			},
			"ollama": {
				Name:    "ollama",
				BaseURL: "http://localhost:11434/v1",
				EnvName: "ollama",
				Models: types.PlatformModels{
					URL:      "http://localhost:11434/api/tags",
					JSONPath: "models.name",
				},
			},
			"together": {
				Name:    "together",
				BaseURL: "https://api.together.xyz/v1",
				EnvName: "TOGETHER_API_KEY",
				Models: types.PlatformModels{
					URL:      "https://api.together.xyz/v1/models",
					JSONPath: "id",
				},
			},
			"google": {
				Name:    "google",
				BaseURL: "https://generativelanguage.googleapis.com/v1beta/openai/",
				EnvName: "GEMINI_API_KEY",
				Models: types.PlatformModels{
					URL:      "https://generativelanguage.googleapis.com/v1beta/models",
					JSONPath: "models.name",
				},
			},
			"mistral": {
				Name:    "mistral",
				BaseURL: "https://api.mistral.ai/v1",
				EnvName: "MISTRAL_API_KEY",
				Models: types.PlatformModels{
					URL:      "https://api.mistral.ai/v1/models",
					JSONPath: "data.id",
				},
			},
		},
	}

	// Load user config from config.json and merge with defaults
	userConfig, err := loadConfigFromFile()
	if err != nil {
		// If we can't load user config, just return defaults
		// In a production app you might want to log this error
		return defaultConfig
	}

	return mergeConfigs(defaultConfig, userConfig)
}

// InitializeAppState creates and returns initial application state
func InitializeAppState() *types.AppState {
	config := DefaultConfig()

	return &types.AppState{
		Config: config,
		Messages: []types.ChatMessage{
			{Role: "system", Content: config.SystemPrompt},
		},
		ChatHistory: []types.ChatHistory{
			{Time: time.Now().Unix(), User: config.SystemPrompt, Bot: ""},
		},
		RecentlyCreatedFiles: []string{},
		IsStreaming:          false,
		StreamingCancel:      nil,
		IsExecutingCommand:   false,
		CommandCancel:        nil,
	}
}
