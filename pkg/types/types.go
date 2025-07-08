package types

// ChatMessage represents a single chat message
type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// ChatHistory represents a chat exchange entry
type ChatHistory struct {
	Time int64  `json:"time"`
	User string `json:"user"`
	Bot  string `json:"bot"`
}

// Platform represents an AI platform configuration
type Platform struct {
	Name      string            `json:"name"`
	BaseURL   string            `json:"base_url"`
	EnvName   string            `json:"env_name"`
	Models    PlatformModels    `json:"models"`
	Headers   map[string]string `json:"headers"`
}

// PlatformModels contains model endpoint configuration
type PlatformModels struct {
	URL      string            `json:"url"`
	JSONPath string            `json:"json_name_path"`
	Headers  map[string]string `json:"headers"`
}

// Config holds application configuration
type Config struct {
	OpenAIAPIKey      string
	DefaultModel      string
	CurrentModel      string
	SystemPrompt      string
	ExitKey           string
	ModelSwitch       string
	TerminalInput     string
	ClearHistory      string
	HelpKey           string
	ExportChat        string
	PreferredEditor   string
	CurrentPlatform   string
	Platforms         map[string]Platform
}

// SearXNGResponse represents the JSON response from SearXNG API
type SearXNGResponse struct {
	Query           string            `json:"query"`
	NumberOfResults int               `json:"number_of_results"`
	Results         []SearXNGResult   `json:"results"`
	Infoboxes       []interface{}     `json:"infoboxes"`
	Suggestions     []string          `json:"suggestions"`
	Answers         []interface{}     `json:"answers"`
	Corrections     []interface{}     `json:"corrections"`
	Unresponsive    []interface{}     `json:"unresponsive_engines"`
}

// SearXNGResult represents a single search result
type SearXNGResult struct {
	URL       string   `json:"url"`
	Title     string   `json:"title"`
	Content   string   `json:"content"`
	Engine    string   `json:"engine"`
	ParsedURL []string `json:"parsed_url"`
	Template  string   `json:"template"`
	Engines   []string `json:"engines"`
	Positions []int    `json:"positions"`
	Score     float64  `json:"score"`
	Category  string   `json:"category"`
}

// AppState holds the application's runtime state
type AppState struct {
	Config          *Config
	Messages        []ChatMessage
	ChatHistory     []ChatHistory
	IsStreaming     bool
	StreamingCancel func()
}

// ClientInitializer interface for creating AI clients
type ClientInitializer interface {
	Initialize(config *Config) error
	SendChatRequest(messages []ChatMessage, model string) (string, error)
	ListModels() ([]string, error)
}