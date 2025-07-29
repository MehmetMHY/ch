package types

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
	BaseURL string            `json:"base_url"`
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
	OpenAIAPIKey    string
	DefaultModel    string
	CurrentModel    string
	SystemPrompt    string
	ExitKey         string
	ModelSwitch     string
	EditorInput     string
	ClearHistory    string
	HelpKey         string
	ExportChat      string
	Backtrack       string
	SaveHistory     string
	LoadFiles       string
	LoadFilesAdv    string
	AnswerSearch    string
	PlatformSwitch  string
	CodeDump        string
	ShellOption     string
	LoadHistory     string
	EditorAlias     string
	MultiLine       string
	PreferredEditor string
	CurrentPlatform string
	Platforms       map[string]Platform
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

// AppState holds the application's runtime state
type AppState struct {
	Config             *Config
	Messages           []ChatMessage
	ChatHistory        []ChatHistory
	IsStreaming        bool
	StreamingCancel    func()
	IsExecutingCommand bool
	CommandCancel      func()
}

// ClientInitializer interface for creating AI clients
type ClientInitializer interface {
	Initialize(config *Config) error
	SendChatRequest(messages []ChatMessage, model string) (string, error)
	ListModels() ([]string, error)
}
