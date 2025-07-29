package config

import (
	"os"
	"time"

	"github.com/MehmetMHY/ch/pkg/types"
)

// DefaultConfig returns the default configuration
func DefaultConfig() *types.Config {
	// Get default platform from environment variable, fallback to "openai"
	defaultPlatform := os.Getenv("CH_DEFAULT_PLATFORM")
	if defaultPlatform == "" {
		defaultPlatform = "openai"
	}

	// Get default model from environment variable, fallback to "gpt-4o-mini"
	defaultModel := os.Getenv("CH_DEFAULT_MODEL")
	if defaultModel == "" {
		defaultModel = "gpt-4o-mini"
	}

	return &types.Config{
		OpenAIAPIKey:    os.Getenv("OPENAI_API_KEY"),
		DefaultModel:    defaultModel,
		CurrentModel:    defaultModel,
		SystemPrompt:    "You are a helpful assistant powered by Cha who provides concise, clear, and accurate answers. Be brief, but ensure the response fully addresses the question without leaving out important details. Always return any code or file output in a Markdown code fence, with syntax ```<language or filetype>\n...``` so it can be parsed automatically. Only do this when needed, no need to do this for responses just code segments and/or when directly asked to do so from the user.",
		ExitKey:         "!q",
		ModelSwitch:     "!m",
		EditorInput:     "!t",
		ClearHistory:    "!c",
		HelpKey:         "!h",
		ExportChat:      "!e",
		Backtrack:       "!b",
		SaveHistory:     "!w",
		LoadFiles:       "!l",
		LoadFilesAdv:    "!f",
		AnswerSearch:    "!a",
		PlatformSwitch:  "!p",
		CodeDump:        "!d",
		ShellRecord:     "!x",
		ShellOption:     "!x",
		LoadHistory:     "!r",
		EditorAlias:     "!v",
		MultiLine:       "\\",
		PreferredEditor: "hx",
		CurrentPlatform: defaultPlatform,
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
		},
	}
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
		IsStreaming:        false,
		StreamingCancel:    nil,
		IsExecutingCommand: false,
		CommandCancel:      nil,
	}
}
