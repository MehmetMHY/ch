package chat

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/MehmetMHY/ch/pkg/types"
)

// ---- EffectiveUserContent ----

func TestEffectiveUserContent(t *testing.T) {
	tests := []struct {
		name  string
		entry types.ChatHistory
		want  string
	}{
		{
			name:  "Only User is set",
			entry: types.ChatHistory{User: "What is Go?", Context: ""},
			want:  "What is Go?",
		},
		{
			name:  "Context overrides User",
			entry: types.ChatHistory{User: "What is Go?", Context: "Full file content... What is Go?"},
			want:  "Full file content... What is Go?",
		},
		{
			name:  "Both empty",
			entry: types.ChatHistory{User: "", Context: ""},
			want:  "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := EffectiveUserContent(tt.entry); got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

// ---- Message & history operations ----

func TestManager_MessageAndHistoryOperations(t *testing.T) {
	cfg := &types.Config{
		SystemPrompt:    "System Prompt",
		CurrentPlatform: "openai",
		CurrentModel:    "gpt-4o",
	}
	state := &types.AppState{
		Config:      cfg,
		Messages:    []types.ChatMessage{{Role: "system", Content: cfg.SystemPrompt}},
		ChatHistory: []types.ChatHistory{{User: cfg.SystemPrompt}},
	}
	m := NewManager(state)

	// AddUserMessage
	m.AddUserMessage("Hello assistant")
	if len(state.Messages) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(state.Messages))
	}
	if state.Messages[1].Role != "user" || state.Messages[1].Content != "Hello assistant" {
		t.Errorf("unexpected user message: %v", state.Messages[1])
	}

	// AddAssistantMessage
	m.AddAssistantMessage("Hello user")
	if len(state.Messages) != 3 {
		t.Fatalf("expected 3 messages, got %d", len(state.Messages))
	}
	if state.Messages[2].Role != "assistant" || state.Messages[2].Content != "Hello user" {
		t.Errorf("unexpected assistant message: %v", state.Messages[2])
	}

	// RemoveLastUserMessage removes the very last message (regardless of role)
	if len(state.Messages) != 3 {
		t.Fatalf("expected 3 messages before removal, got %d", len(state.Messages))
	}
	m.RemoveLastUserMessage()
	if len(state.Messages) != 2 {
		t.Errorf("expected 2 messages after removal, got %d", len(state.Messages))
	}
	if state.Messages[1].Role != "user" || state.Messages[1].Content != "Hello assistant" {
		t.Errorf("RemoveLastUserMessage should only pop the final message, got %v", state.Messages)
	}

	// RemoveLastUserMessage on empty slice should not panic
	state.Messages = []types.ChatMessage{}
	m.RemoveLastUserMessage()
	if len(state.Messages) != 0 {
		t.Errorf("expected 0 messages, got %d", len(state.Messages))
	}

	// Restore and test AddToHistory
	state.Messages = []types.ChatMessage{{Role: "system", Content: cfg.SystemPrompt}}
	m.AddToHistory("User prompt", "Bot reply")
	if len(state.ChatHistory) != 2 {
		t.Fatalf("expected 2 history entries, got %d", len(state.ChatHistory))
	}
	if state.ChatHistory[1].User != "User prompt" || state.ChatHistory[1].Bot != "Bot reply" {
		t.Errorf("unexpected history entry: %v", state.ChatHistory[1])
	}
	if state.ChatHistory[1].Platform != cfg.CurrentPlatform {
		t.Errorf("expected platform %q, got %q", cfg.CurrentPlatform, state.ChatHistory[1].Platform)
	}
	if state.ChatHistory[1].Model != cfg.CurrentModel {
		t.Errorf("expected model %q, got %q", cfg.CurrentModel, state.ChatHistory[1].Model)
	}

	// AddToHistoryWithContext
	m.AddToHistoryWithContext("User prompt 2", "Bot reply 2", "Detailed context")
	if len(state.ChatHistory) != 3 {
		t.Fatalf("expected 3 history entries, got %d", len(state.ChatHistory))
	}
	if state.ChatHistory[2].Context != "Detailed context" {
		t.Errorf("expected Context 'Detailed context', got %q", state.ChatHistory[2].Context)
	}

	// ClearHistory
	m.ClearHistory()
	if len(state.Messages) != 1 || state.Messages[0].Role != "system" {
		t.Errorf("expected only system message after clear, got %v", state.Messages)
	}
	if len(state.ChatHistory) != 1 || state.ChatHistory[0].User != cfg.SystemPrompt {
		t.Errorf("expected only system history entry after clear, got %v", state.ChatHistory)
	}
}

// ---- GetMessages / GetChatHistory / GetCurrentModel / SetCurrentModel / GetCurrentPlatform / SetCurrentPlatform ----

func TestManager_Accessors(t *testing.T) {
	cfg := &types.Config{CurrentModel: "gpt-4o", CurrentPlatform: "openai"}
	state := &types.AppState{
		Config:      cfg,
		Messages:    []types.ChatMessage{{Role: "system", Content: "prompt"}},
		ChatHistory: []types.ChatHistory{},
	}
	m := NewManager(state)

	if got := m.GetCurrentModel(); got != "gpt-4o" {
		t.Errorf("GetCurrentModel() = %q, want %q", got, "gpt-4o")
	}
	m.SetCurrentModel("gpt-5")
	if got := m.GetCurrentModel(); got != "gpt-5" {
		t.Errorf("SetCurrentModel: got %q, want %q", got, "gpt-5")
	}

	if got := m.GetCurrentPlatform(); got != "openai" {
		t.Errorf("GetCurrentPlatform() = %q, want %q", got, "openai")
	}
	m.SetCurrentPlatform("groq")
	if got := m.GetCurrentPlatform(); got != "groq" {
		t.Errorf("SetCurrentPlatform: got %q, want %q", got, "groq")
	}

	msgs := m.GetMessages()
	if len(msgs) != 1 {
		t.Errorf("GetMessages() len = %d, want 1", len(msgs))
	}
	hist := m.GetChatHistory()
	if len(hist) != 0 {
		t.Errorf("GetChatHistory() len = %d, want 0", len(hist))
	}
}

// ---- RestoreSessionState ----

func TestManager_RestoreSessionState(t *testing.T) {
	cfg := &types.Config{
		SystemPrompt:    "System",
		CurrentPlatform: "openai",
		CurrentModel:    "gpt-4o",
	}
	state := &types.AppState{
		Config:      cfg,
		Messages:    []types.ChatMessage{{Role: "system", Content: cfg.SystemPrompt}},
		ChatHistory: []types.ChatHistory{},
	}
	m := NewManager(state)

	session := &types.SessionFile{
		Platform: "groq",
		Model:    "llama3",
		BaseURL:  "https://api.groq.com/openai/v1",
		ChatHistory: []types.ChatHistory{
			{User: cfg.SystemPrompt, Bot: ""},                     // system entry (index 0, skipped)
			{User: "Hello", Bot: "Hi there", Context: ""},         // normal exchange
			{User: "", Bot: "Pure bot reply", Context: "ctx val"}, // context-only user
		},
	}
	m.RestoreSessionState(session)

	if state.Config.CurrentPlatform != "groq" {
		t.Errorf("expected platform 'groq', got %q", state.Config.CurrentPlatform)
	}
	if state.Config.CurrentModel != "llama3" {
		t.Errorf("expected model 'llama3', got %q", state.Config.CurrentModel)
	}
	if state.Config.CurrentBaseURL != "https://api.groq.com/openai/v1" {
		t.Errorf("unexpected BaseURL %q", state.Config.CurrentBaseURL)
	}

	// Messages must start with the system message, followed by user/bot pairs.
	// Entry 0 (system) is skipped, entry 1 contributes user+bot, entry 2 contributes user (via context)+bot.
	if len(state.Messages) < 1 || state.Messages[0].Role != "system" {
		t.Fatalf("first message must be system, got %v", state.Messages)
	}

	wantMessages := []types.ChatMessage{
		{Role: "system", Content: cfg.SystemPrompt},
		{Role: "user", Content: "Hello"},
		{Role: "assistant", Content: "Hi there"},
		{Role: "user", Content: "ctx val"},
		{Role: "assistant", Content: "Pure bot reply"},
	}
	if !reflect.DeepEqual(state.Messages, wantMessages) {
		t.Errorf("restored messages = %+v, want %+v", state.Messages, wantMessages)
	}
}

// ---- SaveSessionState / LoadLatestSessionState ----

func TestManager_SaveAndLoadSession_Latest(t *testing.T) {
	tempHome := t.TempDir()
	t.Setenv("HOME", tempHome)
	t.Setenv("USERPROFILE", tempHome)

	cfg := &types.Config{
		CurrentPlatform:   "openai",
		CurrentModel:      "gpt-4o",
		SystemPrompt:      "Sys",
		EnableSessionSave: true,
		SaveAllSessions:   false,
	}
	history := []types.ChatHistory{
		{User: "Sys", Bot: ""},
		{User: "Hello?", Bot: "Hi!", Time: 1000},
	}
	state := &types.AppState{
		Config:      cfg,
		Messages:    []types.ChatMessage{{Role: "system", Content: "Sys"}},
		ChatHistory: history,
	}
	m := NewManager(state)

	if err := m.SaveSessionState(); err != nil {
		t.Fatalf("SaveSessionState() error: %v", err)
	}

	loaded, err := m.LoadLatestSessionState()
	if err != nil {
		t.Fatalf("LoadLatestSessionState() error: %v", err)
	}

	if loaded.Platform != "openai" {
		t.Errorf("expected platform 'openai', got %q", loaded.Platform)
	}
	if loaded.Model != "gpt-4o" {
		t.Errorf("expected model 'gpt-4o', got %q", loaded.Model)
	}
	if len(loaded.ChatHistory) != 2 {
		t.Errorf("expected 2 history entries, got %d", len(loaded.ChatHistory))
	}
	if loaded.ChatHistory[1].User != "Hello?" || loaded.ChatHistory[1].Bot != "Hi!" {
		t.Errorf("unexpected history entry: %+v", loaded.ChatHistory[1])
	}
}

func TestManager_LoadLatestSessionState_Missing(t *testing.T) {
	tempHome := t.TempDir()
	t.Setenv("HOME", tempHome)
	t.Setenv("USERPROFILE", tempHome)

	cfg := &types.Config{SaveAllSessions: false}
	state := &types.AppState{Config: cfg}
	m := NewManager(state)

	_, err := m.LoadLatestSessionState()
	if err == nil {
		t.Error("expected error when no session file exists, got nil")
	}
}

func TestManager_SaveAndLoadSession_AllSessions(t *testing.T) {
	tempHome := t.TempDir()
	t.Setenv("HOME", tempHome)
	t.Setenv("USERPROFILE", tempHome)

	cfg := &types.Config{
		CurrentPlatform: "anthropic",
		CurrentModel:    "claude-3",
		SystemPrompt:    "S",
		SaveAllSessions: true,
	}
	history := []types.ChatHistory{
		{User: "S", Bot: "", Time: 1},
		{User: "Q", Bot: "A", Time: 2},
	}
	state := &types.AppState{
		Config:      cfg,
		ChatHistory: history,
	}
	m := NewManager(state)

	if err := m.SaveSessionState(); err != nil {
		t.Fatalf("SaveSessionState() error: %v", err)
	}

	loaded, err := m.LoadLatestSessionState()
	if err != nil {
		t.Fatalf("LoadLatestSessionState() error: %v", err)
	}
	if loaded.Model != "claude-3" {
		t.Errorf("expected model 'claude-3', got %q", loaded.Model)
	}
}

// ---- LoadCustomHistoryFile ----

func TestManager_LoadCustomHistoryFile(t *testing.T) {
	tmpDir := t.TempDir()

	session := types.SessionFile{
		Timestamp: 9999,
		Platform:  "groq",
		Model:     "llama3",
		ChatHistory: []types.ChatHistory{
			{User: "Hi", Bot: "Hello"},
		},
	}
	data, _ := json.MarshalIndent(session, "", "  ")
	filePath := filepath.Join(tmpDir, "my_session.json")
	if err := os.WriteFile(filePath, data, 0644); err != nil {
		t.Fatalf("could not write test session file: %v", err)
	}

	cfg := &types.Config{}
	state := &types.AppState{Config: cfg}
	m := NewManager(state)

	loaded, err := m.LoadCustomHistoryFile(filePath)
	if err != nil {
		t.Fatalf("LoadCustomHistoryFile() error: %v", err)
	}
	if loaded.Platform != "groq" {
		t.Errorf("expected platform 'groq', got %q", loaded.Platform)
	}
	if loaded.SourceFile != filePath {
		t.Errorf("expected SourceFile %q, got %q", filePath, loaded.SourceFile)
	}

	// Non-existent file
	_, err = m.LoadCustomHistoryFile(filepath.Join(tmpDir, "nonexistent.json"))
	if err == nil {
		t.Error("expected error for non-existent file, got nil")
	}

	// Corrupt JSON
	corruptPath := filepath.Join(tmpDir, "corrupt.json")
	os.WriteFile(corruptPath, []byte("not json"), 0644)
	_, err = m.LoadCustomHistoryFile(corruptPath)
	if err == nil {
		t.Error("expected error for corrupt JSON, got nil")
	}
}

// ---- AddRecentlyCreatedFile ----

func TestManager_AddRecentlyCreatedFile(t *testing.T) {
	t.Chdir(t.TempDir())

	cfg := &types.Config{}
	state := &types.AppState{
		Config:               cfg,
		RecentlyCreatedFiles: []string{},
	}
	m := NewManager(state)

	// Add 12 files - list must be capped at the 10 most recent entries.
	for i := 0; i < 12; i++ {
		m.AddRecentlyCreatedFile(filepath.Join("exports", strings.Repeat("x", i+1)+".txt"))
	}
	want := []string{
		filepath.Join("exports", "xxxxxxxxxxxx.txt"),
		filepath.Join("exports", "xxxxxxxxxxx.txt"),
		filepath.Join("exports", "xxxxxxxxxx.txt"),
		filepath.Join("exports", "xxxxxxxxx.txt"),
		filepath.Join("exports", "xxxxxxxx.txt"),
		filepath.Join("exports", "xxxxxxx.txt"),
		filepath.Join("exports", "xxxxxx.txt"),
		filepath.Join("exports", "xxxxx.txt"),
		filepath.Join("exports", "xxxx.txt"),
		filepath.Join("exports", "xxx.txt"),
	}
	if !reflect.DeepEqual(state.RecentlyCreatedFiles, want) {
		t.Errorf("RecentlyCreatedFiles = %v, want %v", state.RecentlyCreatedFiles, want)
	}

	// Re-adding an existing entry moves it to the front without growing the list.
	duplicate := state.RecentlyCreatedFiles[3]
	before := len(state.RecentlyCreatedFiles)
	m.AddRecentlyCreatedFile(duplicate)
	if len(state.RecentlyCreatedFiles) != before {
		t.Errorf("duplicate addition changed list length; before=%d after=%d", before, len(state.RecentlyCreatedFiles))
	}
	if state.RecentlyCreatedFiles[0] != duplicate {
		t.Errorf("duplicate should move to front, got %v", state.RecentlyCreatedFiles)
	}
}

// ---- getLanguageExtension ----

func TestManager_GetLanguageExtension(t *testing.T) {
	m := NewManager(&types.AppState{Config: &types.Config{}})

	tests := []struct {
		lang string
		want string
	}{
		{"python", ".py"},
		{"py", ".py"},
		{"javascript", ".js"},
		{"js", ".js"},
		{"typescript", ".ts"},
		{"ts", ".ts"},
		{"go", ".go"},
		{"java", ".java"},
		{"c", ".c"},
		{"cpp", ".cpp"},
		{"c++", ".cpp"},
		{"csharp", ".cs"},
		{"cs", ".cs"},
		{"ruby", ".rb"},
		{"rb", ".rb"},
		{"php", ".php"},
		{"swift", ".swift"},
		{"kotlin", ".kt"},
		{"rust", ".rs"},
		{"rs", ".rs"},
		{"html", ".html"},
		{"css", ".css"},
		{"json", ".json"},
		{"yaml", ".yaml"},
		{"yml", ".yaml"},
		{"markdown", ".md"},
		{"md", ".md"},
		{"shell", ".sh"},
		{"sh", ".sh"},
		{"bash", ".sh"},
		{"sql", ".sql"},
		{"dockerfile", ".Dockerfile"},
		{"makefile", ".Makefile"},
		// Case insensitivity
		{"Python", ".py"},
		{"GO", ".go"},
		// Unknown -> .txt
		{"brainfuck", ".txt"},
		{"", ".txt"},
	}

	for _, tt := range tests {
		t.Run(tt.lang, func(t *testing.T) {
			if got := m.getLanguageExtension(tt.lang); got != tt.want {
				t.Errorf("getLanguageExtension(%q) = %q, want %q", tt.lang, got, tt.want)
			}
		})
	}
}

// ---- sanitizeAIFilename ----

func TestSanitizeAIFilename(t *testing.T) {
	tests := []struct {
		raw  string
		want string
	}{
		{"hello_world", "hello_world"},
		{"Hello World", "hello_world"},
		{"  leading trailing  ", "leading_trailing"},
		{"api-request-handler", "api_request_handler"},
		{"parse_json", "parse_json"},
		{"  __double__underscore__  ", "double_underscore"},
		{"MiXeD-CaSe_Name", "mixed_case_name"},
		{"", ""},
		{"!!!@@@$$$", ""},
		{"a" + strings.Repeat("b", 50), "a" + strings.Repeat("b", 39)}, // capped at 40
		{"trailing_underscore_", "trailing_underscore"},
		{"123numeric456", "123numeric456"},
		// Hyphens and spaces become underscores
		{"foo-bar baz", "foo_bar_baz"},
		// Multiple consecutive separators collapse to one underscore
		{"foo---bar", "foo_bar"},
	}
	for _, tt := range tests {
		t.Run(tt.raw, func(t *testing.T) {
			if got := sanitizeAIFilename(tt.raw); got != tt.want {
				t.Errorf("sanitizeAIFilename(%q) = %q, want %q", tt.raw, got, tt.want)
			}
		})
	}
}

// ---- parseAIFilenameOutput ----

func TestParseAIFilenameOutput(t *testing.T) {
	tests := []struct {
		name     string
		response string
		maxCount int
		want     []string
	}{
		{
			name:     "fenced code block with text tag",
			response: "```text\nhello_world\napi_handler\nparse_json\n```",
			maxCount: 5,
			want:     []string{"hello_world.txt", "api_handler.txt", "parse_json.txt"},
		},
		{
			name:     "fenced code block with other tag",
			response: "```\nfoo_bar\nbaz_qux\n```",
			maxCount: 5,
			want:     []string{"foo_bar.txt", "baz_qux.txt"},
		},
		{
			name:     "maxCount limits output",
			response: "```text\na\nb\nc\nd\ne\nf\n```",
			maxCount: 3,
			want:     []string{"a.txt", "b.txt", "c.txt"},
		},
		{
			name:     "deduplication",
			response: "```text\nfoo\nfoo\nbar\n```",
			maxCount: 10,
			want:     []string{"foo.txt", "bar.txt"},
		},
		{
			name:     "empty / all-invalid lines",
			response: "```text\n!!!\n@@@\n```",
			maxCount: 5,
			want:     nil,
		},
		{
			name:     "no fenced block falls back to raw lines",
			response: "my_file\nanother_file",
			maxCount: 5,
			want:     []string{"my_file.txt", "another_file.txt"},
		},
		{
			// Opening fence with no closing fence: the closing-fence search
			// returns -1 so body is NOT trimmed and the whole raw response
			// (including the "```text" tag line) is parsed line by line. The tag
			// line sanitizes to "text". This pins that fallback behavior.
			name:     "unclosed fence keeps whole response including tag line",
			response: "```text\nfoo_bar\nbaz_qux",
			maxCount: 5,
			want:     []string{"text.txt", "foo_bar.txt", "baz_qux.txt"},
		},
		{
			// Leading prose before the fence is ignored; only fenced content is used.
			name:     "prose before fenced block is ignored",
			response: "Here are some names:\n```text\nonly_this\n```",
			maxCount: 5,
			want:     []string{"only_this.txt"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseAIFilenameOutput(tt.response, tt.maxCount)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("parseAIFilenameOutput() = %v, want %v", got, tt.want)
			}
			// All returned names must end with .txt
			for _, g := range got {
				if !strings.HasSuffix(g, ".txt") {
					t.Errorf("output %q does not end with .txt", g)
				}
			}
			if len(got) > tt.maxCount {
				t.Errorf("returned %d names, want at most %d", len(got), tt.maxCount)
			}
		})
	}
}

// ---- cleanupLoadedContent ----

func TestManager_CleanupLoadedContent(t *testing.T) {
	m := NewManager(&types.AppState{Config: &types.Config{}})

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "excessive trailing newlines stripped",
			input: "hello\n\n\n\n\n",
			want:  "hello",
		},
		{
			name:  "more than 2 consecutive empty lines collapsed",
			input: "a\n\n\n\n\nb",
			want:  "a\n\n\nb",
		},
		{
			name:  "2 consecutive empty lines preserved",
			input: "a\n\n\nb",
			want:  "a\n\n\nb",
		},
		{
			name:  "no empty lines unchanged",
			input: "line1\nline2\nline3",
			want:  "line1\nline2\nline3",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := m.cleanupLoadedContent(tt.input)
			if got != tt.want {
				t.Errorf("cleanupLoadedContent() = %q, want %q", got, tt.want)
			}
		})
	}
}

// ---- createUnifiedFileOptions ----

func TestManager_CreateUnifiedFileOptions(t *testing.T) {
	m := NewManager(&types.AppState{Config: &types.Config{}})

	suggested := []string{"ch_abc.txt", "ch_abc.go"}
	allFiles := []string{"existing.txt", "other.go", "README.md"}
	loadedFiles := []string{"loaded.txt"}
	recentFiles := []string{"recent.txt"}

	opts := m.createUnifiedFileOptions(".txt", suggested, allFiles, loadedFiles, recentFiles)

	if len(opts) == 0 {
		t.Fatal("createUnifiedFileOptions returned empty list")
	}

	want := []string{
		"ch_abc.txt",
		"[w] recent.txt",
		"[w] loaded.txt",
		"[w] existing.txt",
		"ch_abc.go",
		"[w] other.go",
		"[w] README.md",
	}
	if !reflect.DeepEqual(opts, want) {
		t.Errorf("createUnifiedFileOptions() = %v, want %v", opts, want)
	}
}
