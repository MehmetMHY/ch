package types

import "encoding/json"

// BaseURLValue can be either a string or a list of strings
type BaseURLValue struct {
	Single string
	Multi  []string
}

// UnmarshalJSON handles both string and []string for BaseURL
func (b *BaseURLValue) UnmarshalJSON(data []byte) error {
	var str string
	if err := json.Unmarshal(data, &str); err == nil {
		b.Single = str
		b.Multi = nil
		return nil
	}

	var arr []string
	if err := json.Unmarshal(data, &arr); err == nil {
		b.Single = ""
		b.Multi = arr
		return nil
	}

	return json.Unmarshal(data, &str)
}

// IsMulti returns true if BaseURL has multiple values
func (b *BaseURLValue) IsMulti() bool {
	return len(b.Multi) > 0
}

// GetURLs returns the URLs as a slice
func (b *BaseURLValue) GetURLs() []string {
	if b.IsMulti() {
		return b.Multi
	}
	if b.Single != "" {
		return []string{b.Single}
	}
	return []string{}
}

// ChatMessage represents a single chat message
type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// ChatHistory represents a chat exchange entry
type ChatHistory struct {
	Time     int64  `json:"time"`
	User     string `json:"user"`
	Bot      string `json:"bot"`
	Platform string `json:"platform"`
	Model    string `json:"model"`
}

// Platform represents an AI platform configuration
type Platform struct {
	Name    string            `json:"name"`
	BaseURL BaseURLValue      `json:"base_url"`
	EnvName string            `json:"env_name"`
	Models  PlatformModels    `json:"models"`
	Headers map[string]string `json:"headers"`
}

// PlatformModels contains model endpoint configuration
type PlatformModels struct {
	URL      string            `json:"url"`
	JSONPath string            `json:"json_name_path"`
	Headers  map[string]string `json:"headers"`
}

// Config holds application configuration
type Config struct {
	OpenAIAPIKey      string              `json:"openai_api_key,omitempty"`
	DefaultModel      string              `json:"default_model,omitempty"`
	CurrentModel      string              `json:"current_model,omitempty"`
	CurrentBaseURL    string              `json:"current_base_url,omitempty"`
	SystemPrompt      string              `json:"system_prompt,omitempty"`
	ExitKey           string              `json:"exit_key,omitempty"`
	ModelSwitch       string              `json:"model_switch,omitempty"`
	EditorInput       string              `json:"editor_input,omitempty"`
	ClearHistory      string              `json:"clear_history,omitempty"`
	HelpKey           string              `json:"help_key,omitempty"`
	ExportChat        string              `json:"export_chat,omitempty"`
	Backtrack         string              `json:"backtrack,omitempty"`
	SaveHistory       string              `json:"save_history,omitempty"`
	WebSearch         string              `json:"web_search,omitempty"`
	ShowSearchResults bool                `json:"show_search_results,omitempty"`
	NumSearchResults  int                 `json:"num_search_results,omitempty"`
	SearchCountry     string              `json:"search_country,omitempty"`
	SearchLang        string              `json:"search_lang,omitempty"`
	ScrapeURL         string              `json:"scrape_url,omitempty"`
	CopyToClipboard   string              `json:"copy_to_clipboard,omitempty"`
	LoadFiles         string              `json:"load_files,omitempty"`
	LoadFilesAdv      string              `json:"load_files_adv,omitempty"`
	AnswerSearch      string              `json:"answer_search,omitempty"`
	PlatformSwitch    string              `json:"platform_switch,omitempty"`
	CodeDump          string              `json:"code_dump,omitempty"`
	ShellRecord       string              `json:"shell_record,omitempty"`
	ShellOption       string              `json:"shell_option,omitempty"`
	LoadHistory       string              `json:"load_history,omitempty"`
	EditorAlias       string              `json:"editor_alias,omitempty"`
	MultiLine         string              `json:"multi_line,omitempty"`
	PreferredEditor   string              `json:"preferred_editor,omitempty"`
	CurrentPlatform   string              `json:"current_platform,omitempty"`
	AllModels         string              `json:"all_models,omitempty"`
	MuteNotifications bool                `json:"mute_notifications,omitempty"`
	EnableSessionSave bool                `json:"enable_session_save"`
	ShallowLoadDirs   []string            `json:"shallow_load_dirs,omitempty"`
	IsPipedOutput     bool                `json:"-"` // Runtime detection, not from config file
	Platforms         map[string]Platform `json:"platforms,omitempty"`
}

// ExportEntry represents a single entry in the JSON export
type ExportEntry struct {
	Platform    string `json:"platform"`
	ModelName   string `json:"model_name"`
	UserPrompt  string `json:"user_prompt"`
	BotResponse string `json:"bot_response"`
	Timestamp   int64  `json:"timestamp"`
}

// ChatExport represents the complete JSON export structure
type ChatExport struct {
	ExportedAt int64         `json:"exported_at"`
	Entries    []ExportEntry `json:"entries"`
}

// SessionFile represents a persistent session state saved to disk
type SessionFile struct {
	Timestamp   int64         `json:"timestamp"`
	Platform    string        `json:"platform"`
	Model       string        `json:"model"`
	BaseURL     string        `json:"base_url"`
	Messages    []ChatMessage `json:"messages"`
	ChatHistory []ChatHistory `json:"chat_history"`
}

// AppState holds the application's runtime state
type AppState struct {
	Config               *Config
	Messages             []ChatMessage
	ChatHistory          []ChatHistory
	RecentlyCreatedFiles []string
	IsStreaming          bool
	StreamingCancel      func()
	IsExecutingCommand   bool
	CommandCancel        func()
}

// ClientInitializer interface for creating AI clients
type ClientInitializer interface {
	Initialize(config *Config) error
	SendChatRequest(messages []ChatMessage, model string) (string, error)
	ListModels() ([]string, error)
}
