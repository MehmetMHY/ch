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
	if userConfig.WebSearch != "" {
		defaultConfig.WebSearch = userConfig.WebSearch
	}
	if userConfig.NumSearchResults != 0 {
		defaultConfig.NumSearchResults = userConfig.NumSearchResults
	}
	if userConfig.SearchCountry != "" {
		defaultConfig.SearchCountry = userConfig.SearchCountry
	}
	if userConfig.SearchLang != "" {
		defaultConfig.SearchLang = userConfig.SearchLang
	}
	if userConfig.ScrapeURL != "" {
		defaultConfig.ScrapeURL = userConfig.ScrapeURL
	}
	if userConfig.CopyToClipboard != "" {
		defaultConfig.CopyToClipboard = userConfig.CopyToClipboard
	}
	if userConfig.QuickCopyLatest != "" {
		defaultConfig.QuickCopyLatest = userConfig.QuickCopyLatest
	}
	if userConfig.LoadFiles != "" {
		defaultConfig.LoadFiles = userConfig.LoadFiles
	}
	if userConfig.AnswerSearch != "" {
		defaultConfig.AnswerSearch = userConfig.AnswerSearch
	}
	if userConfig.PlatformSwitch != "" {
		defaultConfig.PlatformSwitch = userConfig.PlatformSwitch
	}
	if userConfig.AllModels != "" {
		defaultConfig.AllModels = userConfig.AllModels
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
	if userConfig.MultiLine != "" {
		defaultConfig.MultiLine = userConfig.MultiLine
	}
	if userConfig.PreferredEditor != "" {
		defaultConfig.PreferredEditor = userConfig.PreferredEditor
	}
	if userConfig.CurrentPlatform != "" {
		defaultConfig.CurrentPlatform = userConfig.CurrentPlatform
	}
	// Handle ShowSearchResults - only override if user config has it explicitly set
	// We check if other config fields are set to know if this is an actual config file vs empty
	if userConfig.DefaultModel != "" || userConfig.CurrentPlatform != "" || userConfig.SystemPrompt != "" || userConfig.ShowSearchResults {
		defaultConfig.ShowSearchResults = userConfig.ShowSearchResults
	}
	// Handle MuteNotifications similarly
	if userConfig.DefaultModel != "" || userConfig.CurrentPlatform != "" || userConfig.SystemPrompt != "" || userConfig.MuteNotifications {
		defaultConfig.MuteNotifications = userConfig.MuteNotifications
	}

	// EnableSessionSave doesn't use omitempty, so we can safely use the userConfig value
	// This allows users to explicitly disable it by setting it to false
	// Only override if the config file has at least one other field set (to distinguish from empty config)
	if userConfig.DefaultModel != "" || userConfig.CurrentPlatform != "" || userConfig.SystemPrompt != "" {
		defaultConfig.EnableSessionSave = userConfig.EnableSessionSave
	}
	if userConfig.SaveAllSessions {
		defaultConfig.SaveAllSessions = userConfig.SaveAllSessions
	}

	// Merge ShallowLoadDirs if provided
	if userConfig.ShallowLoadDirs != nil {
		defaultConfig.ShallowLoadDirs = userConfig.ShallowLoadDirs
	}

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
	// Get home directory for default shallow load dirs
	homeDir, _ := os.UserHomeDir()
	// Include common parent directories that are typically large and high up in the filesystem
	shallowDirs := []string{
		"/",              // root directory
		"/Users/",        // macOS user home parent
		"/home/",         // Linux/Unix user home parent
		"/usr/",          // Unix system resources
		"/var/",          // Unix variable data
		"/opt/",          // Optional software packages
		"/Library/",      // macOS system library
		"/System/",       // macOS system files
		"/mnt/",          // mount points for external drives/network shares
		"/media/",        // removable media mount points (Linux)
		"/Applications/", // macOS applications folder
		"/tmp/",          // temporary files directory
	}
	if homeDir != "" {
		shallowDirs = append(shallowDirs, homeDir)
	}

	// Start with hardcoded defaults
	defaultConfig := &types.Config{
		OpenAIAPIKey:      "", // API keys are fetched per-platform in Initialize()
		DefaultModel:      "gpt-4.1-mini",
		CurrentModel:      "gpt-4.1-mini",
		SystemPrompt:      "You are a helpful assistant powered by Ch who provides concise, clear, and accurate answers. Be brief, but ensure the response fully addresses the question without leaving out important details. But still, do NOT go crazy long with your response if you DON'T HAVE TO. Always return any code or file output in a Markdown code fence, with syntax ```<language or filetype>\n...``` so it can be parsed automatically. Only do this when needed, no need to do this for responses just code segments and/or when directly asked to do so from the user.",
		ExitKey:           "!q",
		ModelSwitch:       "!m",
		EditorInput:       "!t",
		ClearHistory:      "!c",
		HelpKey:           "!h",
		ExportChat:        "!e",
		Backtrack:         "!b",
		WebSearch:         "!w",
		ShowSearchResults: true,
		NumSearchResults:  5,
		SearchCountry:     "us",
		SearchLang:        "en",
		ScrapeURL:         "!s",
		CopyToClipboard:   "!y",
		QuickCopyLatest:   "CC",
		LoadFiles:         "!l",
		AnswerSearch:      "!a",
		PlatformSwitch:    "!p",
		AllModels:         "!o",
		CodeDump:          "!d",
		ShellRecord:       "!x",
		ShellOption:       "!x",
		MultiLine:         "\\",
		PreferredEditor:   "vim",
		CurrentPlatform:   "openai",
		MuteNotifications: false,
		EnableSessionSave: true,
		ShallowLoadDirs:   shallowDirs,
		Platforms: map[string]types.Platform{
			"groq": {
				Name:    "groq",
				BaseURL: types.BaseURLValue{Single: "https://api.groq.com/openai/v1"},
				EnvName: "GROQ_API_KEY",
				Models: types.PlatformModels{
					URL:      "https://api.groq.com/openai/v1/models",
					JSONPath: "data.id",
				},
			},
			"deepseek": {
				Name:    "deepseek",
				BaseURL: types.BaseURLValue{Single: "https://api.deepseek.com"},
				EnvName: "DEEP_SEEK_API_KEY",
				Models: types.PlatformModels{
					URL:      "https://api.deepseek.com/models",
					JSONPath: "data.id",
				},
			},
			"anthropic": {
				Name:    "anthropic",
				BaseURL: types.BaseURLValue{Single: "https://api.anthropic.com/v1/"},
				EnvName: "ANTHROPIC_API_KEY",
				Models: types.PlatformModels{
					URL:      "https://api.anthropic.com/v1/models",
					JSONPath: "data.id",
				},
			},
			"xai": {
				Name:    "xai",
				BaseURL: types.BaseURLValue{Single: "https://api.x.ai/v1"},
				EnvName: "XAI_API_KEY",
				Models: types.PlatformModels{
					URL:      "https://api.x.ai/v1/models",
					JSONPath: "data.id",
				},
			},
			"ollama": {
				Name:    "ollama",
				BaseURL: types.BaseURLValue{Single: "http://localhost:11434/v1"},
				EnvName: "ollama",
				Models: types.PlatformModels{
					URL:      "http://localhost:11434/api/tags",
					JSONPath: "models.name",
				},
			},
			"together": {
				Name:    "together",
				BaseURL: types.BaseURLValue{Single: "https://api.together.xyz/v1"},
				EnvName: "TOGETHER_API_KEY",
				Models: types.PlatformModels{
					URL:      "https://api.together.xyz/v1/models",
					JSONPath: "id",
				},
			},
			"google": {
				Name:    "google",
				BaseURL: types.BaseURLValue{Single: "https://generativelanguage.googleapis.com/v1beta/openai/"},
				EnvName: "GEMINI_API_KEY",
				Models: types.PlatformModels{
					URL:      "https://generativelanguage.googleapis.com/v1beta/models",
					JSONPath: "models.name",
				},
			},
			"mistral": {
				Name:    "mistral",
				BaseURL: types.BaseURLValue{Single: "https://api.mistral.ai/v1"},
				EnvName: "MISTRAL_API_KEY",
				Models: types.PlatformModels{
					URL:      "https://api.mistral.ai/v1/models",
					JSONPath: "data.id",
				},
			},
			"amazon": {
				Name: "amazon",
				BaseURL: types.BaseURLValue{
					Multi: []string{
						"https://bedrock-runtime.us-west-2.amazonaws.com/openai/v1",
						"https://bedrock-runtime.us-east-1.amazonaws.com/openai/v1",
						"https://bedrock-runtime.us-east-2.amazonaws.com/openai/v1",
						"https://bedrock-runtime.ap-northeast-1.amazonaws.com/openai/v1",
						"https://bedrock-runtime.ap-northeast-2.amazonaws.com/openai/v1",
						"https://bedrock-runtime.ap-northeast-3.amazonaws.com/openai/v1",
						"https://bedrock-runtime.ap-south-1.amazonaws.com/openai/v1",
						"https://bedrock-runtime.ap-south-2.amazonaws.com/openai/v1",
						"https://bedrock-runtime.ap-southeast-1.amazonaws.com/openai/v1",
						"https://bedrock-runtime.ap-southeast-2.amazonaws.com/openai/v1",
						"https://bedrock-runtime.ca-central-1.amazonaws.com/openai/v1",
						"https://bedrock-runtime.eu-central-1.amazonaws.com/openai/v1",
						"https://bedrock-runtime.eu-central-2.amazonaws.com/openai/v1",
						"https://bedrock-runtime.eu-north-1.amazonaws.com/openai/v1",
						"https://bedrock-runtime.eu-south-1.amazonaws.com/openai/v1",
						"https://bedrock-runtime.eu-south-2.amazonaws.com/openai/v1",
						"https://bedrock-runtime.eu-west-1.amazonaws.com/openai/v1",
						"https://bedrock-runtime.eu-west-2.amazonaws.com/openai/v1",
						"https://bedrock-runtime.eu-west-3.amazonaws.com/openai/v1",
						"https://bedrock-runtime.sa-east-1.amazonaws.com/openai/v1",
						"https://bedrock-runtime.us-gov-east-1.amazonaws.com/openai/v1",
						"https://bedrock-runtime.us-gov-west-1.amazonaws.com/openai/v1",
					},
				},
				EnvName: "AWS_BEDROCK_API_KEY",
				Models: types.PlatformModels{
					URL:      "https://bedrock.us-west-2.amazonaws.com/foundation-models",
					JSONPath: "modelSummaries.modelId",
				},
			},
		},
	}

	// Load user config from config.json and merge with defaults
	userConfig, err := loadConfigFromFile()
	if err == nil {
		defaultConfig = mergeConfigs(defaultConfig, userConfig)
	}

	// Override with environment variables, giving them higher precedence
	if platformEnv := os.Getenv("CH_DEFAULT_PLATFORM"); platformEnv != "" {
		defaultConfig.CurrentPlatform = platformEnv
	}
	if modelEnv := os.Getenv("CH_DEFAULT_MODEL"); modelEnv != "" {
		defaultConfig.CurrentModel = modelEnv
		defaultConfig.DefaultModel = modelEnv
	}

	return defaultConfig
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
