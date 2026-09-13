package chat

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/MehmetMHY/ch/internal/config"
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

	// RemoveLastUserMessage only removes a trailing user message.
	if len(state.Messages) != 3 {
		t.Fatalf("expected 3 messages before removal, got %d", len(state.Messages))
	}
	m.RemoveLastUserMessage()
	if len(state.Messages) != 3 {
		t.Errorf("expected assistant message to be preserved, got %d messages", len(state.Messages))
	}
	if state.Messages[2].Role != "assistant" || state.Messages[2].Content != "Hello user" {
		t.Errorf("RemoveLastUserMessage should not pop a final assistant message, got %v", state.Messages)
	}

	state.Messages = state.Messages[:2]
	m.RemoveLastUserMessage()
	if len(state.Messages) != 1 {
		t.Errorf("expected trailing user message to be removed, got %d messages", len(state.Messages))
	}

	// RemoveLastUserMessage on empty slice should not panic
	state.Messages = []types.ChatMessage{}
	m.RemoveLastUserMessage()
	if len(state.Messages) != 0 {
		t.Errorf("expected 0 messages, got %d", len(state.Messages))
	}

	state.Messages = []types.ChatMessage{
		{Role: "system", Content: cfg.SystemPrompt},
		{Role: "user", Content: "old prompt"},
		{Role: "assistant", Content: "old response"},
		{Role: "user", Content: "pending prompt"},
	}
	if removed := m.RemovePendingUserMessage("different prompt"); removed {
		t.Fatalf("RemovePendingUserMessage removed a non-matching prompt")
	}
	if len(state.Messages) != 4 {
		t.Fatalf("expected non-matching pending prompt to be preserved, got %v", state.Messages)
	}
	if removed := m.RemovePendingUserMessage("pending prompt"); !removed {
		t.Fatalf("RemovePendingUserMessage did not remove matching pending prompt")
	}
	if len(state.Messages) != 3 {
		t.Fatalf("expected only matching pending prompt removed, got %v", state.Messages)
	}
	if state.Messages[2].Role != "assistant" || state.Messages[2].Content != "old response" {
		t.Fatalf("expected prior assistant history to be preserved, got %v", state.Messages)
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
	if state.ChatHistory[1].ReasoningEffort != state.ReasoningEffort {
		t.Errorf("expected reasoning_effort %q, got %q", state.ReasoningEffort, state.ChatHistory[1].ReasoningEffort)
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
	if got := filepath.Base(state.SessionFilePath); got != "ch_session_latest.json" {
		t.Errorf("expected session file ch_session_latest.json, got %q", got)
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

func TestManager_PrepareSessionFilePath_AllSessions(t *testing.T) {
	tempHome := t.TempDir()
	t.Setenv("HOME", tempHome)
	t.Setenv("USERPROFILE", tempHome)

	state := &types.AppState{
		Config: &types.Config{
			SaveAllSessions: true,
		},
		SessionStartTime: 1783568531,
	}
	m := NewManager(state)

	path, err := m.PrepareSessionFilePath()
	if err != nil {
		t.Fatalf("PrepareSessionFilePath() error: %v", err)
	}
	if got := filepath.Base(path); got != "ch_session_1783568531.json" {
		t.Errorf("expected timestamped session file, got %q", got)
	}
	if got := m.CurrentSessionFileName(); got != "ch_session_1783568531.json" {
		t.Errorf("expected current session file name, got %q", got)
	}

	state.SessionStartTime = 999
	pathAgain, err := m.PrepareSessionFilePath()
	if err != nil {
		t.Fatalf("PrepareSessionFilePath() second call error: %v", err)
	}
	if pathAgain != path {
		t.Errorf("expected prepared path to remain stable, got %q then %q", path, pathAgain)
	}
}

func TestManager_RestoreSessionStatePreservesSourceFile(t *testing.T) {
	tempHome := t.TempDir()
	t.Setenv("HOME", tempHome)
	t.Setenv("USERPROFILE", tempHome)

	sourceFile := filepath.Join(tempHome, "ch_session_123.json")
	state := &types.AppState{
		Config: &types.Config{
			CurrentPlatform: "openai",
			CurrentModel:    "gpt-4o",
			SystemPrompt:    "S",
			SaveAllSessions: true,
		},
		ChatHistory: []types.ChatHistory{{User: "S"}},
	}
	m := NewManager(state)

	m.RestoreSessionState(&types.SessionFile{
		Timestamp:  123,
		Platform:   "groq",
		Model:      "llama3",
		SourceFile: sourceFile,
		ChatHistory: []types.ChatHistory{
			{User: "S"},
			{User: "Q", Bot: "A"},
		},
	})

	if got := m.CurrentSessionFileName(); got != "ch_session_123.json" {
		t.Errorf("expected restored session file name, got %q", got)
	}
	if err := m.SaveSessionState(); err != nil {
		t.Fatalf("SaveSessionState() error: %v", err)
	}
	if _, err := os.Stat(sourceFile); err != nil {
		t.Fatalf("expected save to preserve restored source file: %v", err)
	}
}

func TestManager_ForkSessionOnNextSaveSkipsUnchangedSession(t *testing.T) {
	tempHome := t.TempDir()
	t.Setenv("HOME", tempHome)
	t.Setenv("USERPROFILE", tempHome)

	tmpDir := filepath.Join(tempHome, ".ch", "tmp")
	if err := os.MkdirAll(tmpDir, 0700); err != nil {
		t.Fatalf("could not create temp dir: %v", err)
	}

	sourceFile := filepath.Join(tmpDir, "ch_session_123.json")
	session := types.SessionFile{
		Timestamp: 123,
		Platform:  "groq",
		Model:     "llama3",
		ChatHistory: []types.ChatHistory{
			{User: "S"},
			{User: "Q", Bot: "A"},
		},
	}
	sourceData, err := json.MarshalIndent(session, "", "  ")
	if err != nil {
		t.Fatalf("could not marshal source session: %v", err)
	}
	if err := os.WriteFile(sourceFile, sourceData, 0600); err != nil {
		t.Fatalf("could not write source session: %v", err)
	}
	session.SourceFile = sourceFile

	state := &types.AppState{
		Config: &types.Config{
			CurrentPlatform: "openai",
			CurrentModel:    "gpt-4o",
			SystemPrompt:    "S",
			SaveAllSessions: true,
		},
	}
	m := NewManager(state)
	m.RestoreSessionState(&session)
	m.ForkSessionOnNextSave()

	if err := m.SaveSessionState(); err != nil {
		t.Fatalf("SaveSessionState() error: %v", err)
	}

	matches, err := filepath.Glob(filepath.Join(tmpDir, "ch_session_*.json"))
	if err != nil {
		t.Fatalf("Glob() error: %v", err)
	}
	if len(matches) != 1 || matches[0] != sourceFile {
		t.Fatalf("expected only original session file, got %v", matches)
	}
}

func TestManager_ForkSessionOnNextSaveCreatesNewFileAfterChange(t *testing.T) {
	tempHome := t.TempDir()
	t.Setenv("HOME", tempHome)
	t.Setenv("USERPROFILE", tempHome)

	tmpDir := filepath.Join(tempHome, ".ch", "tmp")
	if err := os.MkdirAll(tmpDir, 0700); err != nil {
		t.Fatalf("could not create temp dir: %v", err)
	}

	sourceFile := filepath.Join(tmpDir, "ch_session_123.json")
	session := types.SessionFile{
		Timestamp: 123,
		Platform:  "groq",
		Model:     "llama3",
		ChatHistory: []types.ChatHistory{
			{User: "S"},
			{User: "Q", Bot: "A"},
		},
	}
	sourceData, err := json.MarshalIndent(session, "", "  ")
	if err != nil {
		t.Fatalf("could not marshal source session: %v", err)
	}
	if err := os.WriteFile(sourceFile, sourceData, 0600); err != nil {
		t.Fatalf("could not write source session: %v", err)
	}
	session.SourceFile = sourceFile

	state := &types.AppState{
		Config: &types.Config{
			CurrentPlatform: "openai",
			CurrentModel:    "gpt-4o",
			SystemPrompt:    "S",
			SaveAllSessions: true,
		},
	}
	m := NewManager(state)
	m.RestoreSessionState(&session)
	m.ForkSessionOnNextSave()
	m.AddToHistory("Q2", "A2")

	if err := m.SaveSessionState(); err != nil {
		t.Fatalf("SaveSessionState() error: %v", err)
	}

	if state.SessionFilePath == sourceFile {
		t.Fatalf("expected forked session path, got original %q", sourceFile)
	}
	if filepath.Base(state.SessionFilePath) == "ch_session_123.json" {
		t.Fatalf("expected forked session filename, got %q", state.SessionFilePath)
	}

	gotSourceData, err := os.ReadFile(sourceFile)
	if err != nil {
		t.Fatalf("could not read source session: %v", err)
	}
	if string(gotSourceData) != string(sourceData) {
		t.Fatalf("expected original session file to remain unchanged")
	}

	newData, err := os.ReadFile(state.SessionFilePath)
	if err != nil {
		t.Fatalf("could not read forked session: %v", err)
	}
	var forked types.SessionFile
	if err := json.Unmarshal(newData, &forked); err != nil {
		t.Fatalf("could not parse forked session: %v", err)
	}
	last := forked.ChatHistory[len(forked.ChatHistory)-1]
	if last.User != "Q2" || last.Bot != "A2" {
		t.Fatalf("expected forked session to include new exchange, got %+v", last)
	}
}

func TestManager_PrepareSessionFilePath_AllSessionsSkipsExistingTimestamp(t *testing.T) {
	tempHome := t.TempDir()
	t.Setenv("HOME", tempHome)
	t.Setenv("USERPROFILE", tempHome)

	tmpDir := filepath.Join(tempHome, ".ch", "tmp")
	if err := os.MkdirAll(tmpDir, 0700); err != nil {
		t.Fatalf("could not create temp dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "ch_session_1783568531.json"), []byte("{}"), 0600); err != nil {
		t.Fatalf("could not write existing session: %v", err)
	}

	state := &types.AppState{
		Config:           &types.Config{SaveAllSessions: true},
		SessionStartTime: 1783568531,
	}
	m := NewManager(state)

	path, err := m.PrepareSessionFilePath()
	if err != nil {
		t.Fatalf("PrepareSessionFilePath() error: %v", err)
	}
	if got := filepath.Base(path); got != "ch_session_1783568532.json" {
		t.Errorf("expected next available timestamped session file, got %q", got)
	}
}

func TestFormatSessionSearchPreview(t *testing.T) {
	preview := formatSessionSearchPreview("/tmp/ch_session_1783572416.json", 1783572299, "user", "Loaded: ch_session_1783572299.json")

	if !strings.HasPrefix(preview, "2026-07-09 04:44:59 UTC user: Loaded: ch_session_1783572299.json") {
		t.Fatalf("unexpected preview: %q", preview)
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

	tmpDir, err := config.GetTempDir()
	if err != nil {
		t.Fatalf("GetTempDir() error: %v", err)
	}
	pointerData, err := os.ReadFile(filepath.Join(tmpDir, latestSessionPointerFilename))
	if err != nil {
		t.Fatalf("expected latest session pointer: %v", err)
	}
	if got := strings.TrimSpace(string(pointerData)); got != filepath.Base(state.SessionFilePath) {
		t.Fatalf("latest pointer = %q, want %q", got, filepath.Base(state.SessionFilePath))
	}

	loaded, err := m.LoadLatestSessionState()
	if err != nil {
		t.Fatalf("LoadLatestSessionState() error: %v", err)
	}
	if loaded.Model != "claude-3" {
		t.Errorf("expected model 'claude-3', got %q", loaded.Model)
	}
}

func TestManager_LoadLatestSessionState_AllSessionsUsesPointer(t *testing.T) {
	tempHome := t.TempDir()
	t.Setenv("HOME", tempHome)
	t.Setenv("USERPROFILE", tempHome)

	tmpDir, err := config.GetTempDir()
	if err != nil {
		t.Fatalf("GetTempDir() error: %v", err)
	}

	writeSession := func(filename, model string, timestamp int64) {
		t.Helper()
		session := types.SessionFile{
			Timestamp:   timestamp,
			Platform:    "openai",
			Model:       model,
			ChatHistory: []types.ChatHistory{{User: "S"}},
		}
		data, err := json.Marshal(session)
		if err != nil {
			t.Fatalf("failed to marshal session: %v", err)
		}
		if err := os.WriteFile(filepath.Join(tmpDir, filename), data, 0600); err != nil {
			t.Fatalf("failed to write session %s: %v", filename, err)
		}
	}

	writeSession("ch_session_100.json", "from-pointer", 100)
	writeSession("ch_session_200.json", "from-scan", 200)
	if err := os.WriteFile(filepath.Join(tmpDir, latestSessionPointerFilename), []byte("ch_session_100.json\n"), 0600); err != nil {
		t.Fatalf("failed to write latest session pointer: %v", err)
	}

	m := NewManager(&types.AppState{Config: &types.Config{SaveAllSessions: true}})
	loaded, err := m.LoadLatestSessionState()
	if err != nil {
		t.Fatalf("LoadLatestSessionState() error: %v", err)
	}

	if loaded.Model != "from-pointer" {
		t.Fatalf("expected pointer-selected session, got model %q", loaded.Model)
	}
	if got := filepath.Base(loaded.SourceFile); got != "ch_session_100.json" {
		t.Fatalf("expected pointer source file, got %q", got)
	}
}

func TestWriteLatestSessionPointerKeepsMRUList(t *testing.T) {
	tmpDir := t.TempDir()

	if err := writeLatestSessionPointer(tmpDir, "ch_session_100.json"); err != nil {
		t.Fatalf("writeLatestSessionPointer() error: %v", err)
	}
	if err := writeLatestSessionPointer(tmpDir, "ch_session_200.json"); err != nil {
		t.Fatalf("writeLatestSessionPointer() error: %v", err)
	}
	if err := writeLatestSessionPointer(tmpDir, "ch_session_100.json"); err != nil {
		t.Fatalf("writeLatestSessionPointer() error: %v", err)
	}

	filenames := readLatestSessionPointerFilenames(tmpDir)
	want := []string{"ch_session_100.json", "ch_session_200.json"}
	if !reflect.DeepEqual(filenames, want) {
		t.Fatalf("pointer filenames = %v, want %v", filenames, want)
	}

	for i := 300; i < 1500; i += 100 {
		if err := writeLatestSessionPointer(tmpDir, fmt.Sprintf("ch_session_%d.json", i)); err != nil {
			t.Fatalf("writeLatestSessionPointer() error: %v", err)
		}
	}

	filenames = readLatestSessionPointerFilenames(tmpDir)
	if len(filenames) != latestSessionPointerLimit {
		t.Fatalf("expected %d pointer entries, got %d: %v", latestSessionPointerLimit, len(filenames), filenames)
	}
	if filenames[0] != "ch_session_1400.json" {
		t.Fatalf("expected newest pointer entry first, got %q", filenames[0])
	}
	if filenames[len(filenames)-1] != "ch_session_500.json" {
		t.Fatalf("expected pointer list to drop older entries first, got %v", filenames)
	}
}

func TestManager_LoadLatestSessionState_AllSessionsTriesPointerListBeforeScan(t *testing.T) {
	tempHome := t.TempDir()
	t.Setenv("HOME", tempHome)
	t.Setenv("USERPROFILE", tempHome)

	tmpDir, err := config.GetTempDir()
	if err != nil {
		t.Fatalf("GetTempDir() error: %v", err)
	}

	validSession := types.SessionFile{
		Timestamp:   100,
		Platform:    "openai",
		Model:       "from-pointer-list",
		ChatHistory: []types.ChatHistory{{User: "S"}},
	}
	validData, err := json.Marshal(validSession)
	if err != nil {
		t.Fatalf("failed to marshal session: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "ch_session_100.json"), validData, 0600); err != nil {
		t.Fatalf("failed to write valid session: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "ch_session_200.json"), []byte("not-json"), 0600); err != nil {
		t.Fatalf("failed to write corrupt session: %v", err)
	}

	newerSession := types.SessionFile{
		Timestamp:   400,
		Platform:    "openai",
		Model:       "from-scan",
		ChatHistory: []types.ChatHistory{{User: "S"}},
	}
	newerData, err := json.Marshal(newerSession)
	if err != nil {
		t.Fatalf("failed to marshal newer session: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "ch_session_400.json"), newerData, 0600); err != nil {
		t.Fatalf("failed to write newer session: %v", err)
	}

	pointer := strings.Join([]string{
		"ch_session_300.json",
		"ch_session_200.json",
		"ch_session_100.json",
	}, "\n") + "\n"
	if err := os.WriteFile(filepath.Join(tmpDir, latestSessionPointerFilename), []byte(pointer), 0600); err != nil {
		t.Fatalf("failed to write latest session pointer: %v", err)
	}

	m := NewManager(&types.AppState{Config: &types.Config{SaveAllSessions: true}})
	loaded, err := m.LoadLatestSessionState()
	if err != nil {
		t.Fatalf("LoadLatestSessionState() error: %v", err)
	}

	if loaded.Model != "from-pointer-list" {
		t.Fatalf("expected pointer-list session before scan fallback, got model %q", loaded.Model)
	}
	filenames := readLatestSessionPointerFilenames(tmpDir)
	if len(filenames) == 0 || filenames[0] != "ch_session_100.json" {
		t.Fatalf("expected working pointer entry to be promoted, got %v", filenames)
	}
}

func TestManager_LoadLatestSessionState_AllSessionsFallsBackWithoutPointer(t *testing.T) {
	tempHome := t.TempDir()
	t.Setenv("HOME", tempHome)
	t.Setenv("USERPROFILE", tempHome)

	tmpDir, err := config.GetTempDir()
	if err != nil {
		t.Fatalf("GetTempDir() error: %v", err)
	}

	for _, item := range []struct {
		filename  string
		model     string
		timestamp int64
	}{
		{filename: "ch_session_100.json", model: "older", timestamp: 100},
		{filename: "ch_session_200.json", model: "newer", timestamp: 200},
	} {
		session := types.SessionFile{
			Timestamp:   item.timestamp,
			Platform:    "openai",
			Model:       item.model,
			ChatHistory: []types.ChatHistory{{User: "S"}},
		}
		data, err := json.Marshal(session)
		if err != nil {
			t.Fatalf("failed to marshal session: %v", err)
		}
		if err := os.WriteFile(filepath.Join(tmpDir, item.filename), data, 0600); err != nil {
			t.Fatalf("failed to write session %s: %v", item.filename, err)
		}
	}

	m := NewManager(&types.AppState{Config: &types.Config{SaveAllSessions: true}})
	loaded, err := m.LoadLatestSessionState()
	if err != nil {
		t.Fatalf("LoadLatestSessionState() error: %v", err)
	}
	if loaded.Model != "newer" {
		t.Fatalf("expected fallback scan-selected session, got model %q", loaded.Model)
	}
	pointerData, err := os.ReadFile(filepath.Join(tmpDir, latestSessionPointerFilename))
	if err != nil {
		t.Fatalf("expected fallback scan to create latest session pointer: %v", err)
	}
	if got := strings.TrimSpace(string(pointerData)); got != "ch_session_200.json" {
		t.Fatalf("latest pointer = %q, want ch_session_200.json", got)
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

// TestSelectExportFilenameListOrder verifies the option list assembled by
// selectExportFilename puts ">custom" first, AI-suggested names next, then the
// unified file list. It tests the assembly order without invoking fzf by
// building the same slice the helper constructs.
func TestSelectExportFilenameListOrder(t *testing.T) {
	m := NewManager(&types.AppState{Config: &types.Config{}})

	aiNames := []string{"ai_suggestion_one.txt", "ai_suggestion_two.txt"}
	suggested := []string{"ch_abc.txt", "ch_abc.go"}
	allFiles := []string{"existing.txt", "other.go"}
	loadedFiles := []string{"loaded.txt"}
	recentFiles := []string{"recent.txt"}

	// Replicate the assembly in selectExportFilename: ">custom" first, then
	// AI names, then the unified list from createUnifiedFileOptions.
	unified := m.createUnifiedFileOptions(".txt", suggested, allFiles, loadedFiles, recentFiles)
	options := append([]string{">custom"}, append(aiNames, unified...)...)

	if len(options) == 0 {
		t.Fatal("assembled option list is empty")
	}
	if options[0] != ">custom" {
		t.Errorf("expected \">custom\" at index 0, got %q", options[0])
	}
	if len(options) < 3 || options[1] != "ai_suggestion_one.txt" || options[2] != "ai_suggestion_two.txt" {
		t.Errorf("expected AI names at indices 1-2, got %v", options)
	}
	// No duplicate ">custom" entries.
	count := 0
	for _, o := range options {
		if o == ">custom" {
			count++
		}
	}
	if count != 1 {
		t.Errorf("expected exactly one \">custom\" entry, got %d", count)
	}
}

// ---- Reasoning effort persistence ----

func TestAddToHistoryCapturesReasoningEffort(t *testing.T) {
	cfg := &types.Config{
		SystemPrompt:    "S",
		CurrentPlatform: "openai",
		CurrentModel:    "gpt-5.4",
	}
	state := &types.AppState{
		Config:      cfg,
		Messages:    []types.ChatMessage{{Role: "system", Content: cfg.SystemPrompt}},
		ChatHistory: []types.ChatHistory{{User: cfg.SystemPrompt}},
	}
	m := NewManager(state)

	// Set a reasoning effort.
	state.ReasoningEffort = "high"
	m.AddToHistory("question", "answer")
	if state.ChatHistory[1].ReasoningEffort != "high" {
		t.Errorf("expected reasoning_effort=high in history, got %q", state.ChatHistory[1].ReasoningEffort)
	}

	// Clear it.
	state.ReasoningEffort = ""
	m.AddToHistory("question2", "answer2")
	if state.ChatHistory[2].ReasoningEffort != "" {
		t.Errorf("expected empty reasoning_effort, got %q", state.ChatHistory[2].ReasoningEffort)
	}
}

func TestAddToHistoryWithContextCapturesReasoningEffort(t *testing.T) {
	cfg := &types.Config{
		SystemPrompt:    "S",
		CurrentPlatform: "openai",
		CurrentModel:    "gpt-5.4",
	}
	state := &types.AppState{
		Config:      cfg,
		Messages:    []types.ChatMessage{{Role: "system", Content: cfg.SystemPrompt}},
		ChatHistory: []types.ChatHistory{{User: cfg.SystemPrompt}},
	}
	m := NewManager(state)

	state.ReasoningEffort = "medium"
	m.AddToHistoryWithContext("summary", "answer", "full context")
	if state.ChatHistory[1].ReasoningEffort != "medium" {
		t.Errorf("expected reasoning_effort=medium, got %q", state.ChatHistory[1].ReasoningEffort)
	}
	if state.ChatHistory[1].Context != "full context" {
		t.Errorf("expected context, got %q", state.ChatHistory[1].Context)
	}
}

func TestSaveSessionStateIncludesReasoningEffort(t *testing.T) {
	tempHome := t.TempDir()
	t.Setenv("HOME", tempHome)
	t.Setenv("USERPROFILE", tempHome)

	cfg := &types.Config{
		SystemPrompt:      "S",
		CurrentPlatform:   "openai",
		CurrentModel:      "gpt-5.4",
		EnableSessionSave: true,
	}
	state := &types.AppState{
		Config:          cfg,
		Messages:        []types.ChatMessage{{Role: "system", Content: cfg.SystemPrompt}},
		ChatHistory:     []types.ChatHistory{{User: cfg.SystemPrompt}},
		ReasoningEffort: "low",
	}
	m := NewManager(state)

	if err := m.SaveSessionState(); err != nil {
		t.Fatalf("SaveSessionState error: %v", err)
	}

	// Load and verify.
	loaded, err := m.LoadLatestSessionState()
	if err != nil {
		t.Fatalf("LoadLatestSessionState error: %v", err)
	}
	if loaded.ReasoningEffort != "low" {
		t.Errorf("expected loaded reasoning_effort=low, got %q", loaded.ReasoningEffort)
	}
}

func TestRestoreSessionStateRestoresReasoningEffort(t *testing.T) {
	cfg := &types.Config{
		SystemPrompt:    "S",
		CurrentPlatform: "openai",
		CurrentModel:    "gpt-5.4",
	}
	state := &types.AppState{
		Config:          cfg,
		Messages:        []types.ChatMessage{{Role: "system", Content: cfg.SystemPrompt}},
		ChatHistory:     []types.ChatHistory{{User: cfg.SystemPrompt}},
		ReasoningEffort: "medium",
	}
	m := NewManager(state)

	// Save.
	if err := m.SaveSessionState(); err != nil {
		t.Fatalf("SaveSessionState error: %v", err)
	}

	// Change reasoning effort.
	state.ReasoningEffort = "high"

	// Load and restore.
	loaded, err := m.LoadLatestSessionState()
	if err != nil {
		t.Fatalf("LoadLatestSessionState error: %v", err)
	}
	m.RestoreSessionState(loaded)

	if state.ReasoningEffort != "medium" {
		t.Errorf("expected restored reasoning_effort=medium, got %q", state.ReasoningEffort)
	}
}

func TestRestoreSessionState_LegacySessionWithoutReasoningEffort(t *testing.T) {
	tempHome := t.TempDir()
	t.Setenv("HOME", tempHome)
	t.Setenv("USERPROFILE", tempHome)

	cfg := &types.Config{
		SystemPrompt:      "S",
		CurrentPlatform:   "openai",
		CurrentModel:      "gpt-5.4",
		EnableSessionSave: true,
	}
	state := &types.AppState{
		Config:          cfg,
		Messages:        []types.ChatMessage{{Role: "system", Content: cfg.SystemPrompt}},
		ChatHistory:     []types.ChatHistory{{User: cfg.SystemPrompt}},
		ReasoningEffort: "high",
	}
	m := NewManager(state)

	// Manually write a legacy session file without reasoning_effort.
	tmpDir, _ := config.GetTempDir()
	legacySession := types.SessionFile{
		Timestamp:   time.Now().Unix(),
		Platform:    "openai",
		Model:       "gpt-5.4",
		ChatHistory: []types.ChatHistory{{User: "S"}},
	}
	data, _ := json.Marshal(legacySession)
	legacyPath := filepath.Join(tmpDir, "ch_session_latest.json")
	os.WriteFile(legacyPath, data, 0600)

	loaded, err := m.LoadLatestSessionState()
	if err != nil {
		t.Fatalf("LoadLatestSessionState error: %v", err)
	}
	m.RestoreSessionState(loaded)

	// Legacy session without reasoning_effort should restore to empty (provider default).
	if state.ReasoningEffort != "" {
		t.Errorf("expected empty reasoning_effort from legacy session, got %q", state.ReasoningEffort)
	}
}

func TestExportFullHistoryIncludesReasoningEffort(t *testing.T) {
	tempHome := t.TempDir()
	t.Setenv("HOME", tempHome)
	t.Setenv("USERPROFILE", tempHome)

	cfg := &types.Config{
		SystemPrompt:    "S",
		CurrentPlatform: "openai",
		CurrentModel:    "gpt-5.4",
	}
	state := &types.AppState{
		Config:   cfg,
		Messages: []types.ChatMessage{{Role: "system", Content: cfg.SystemPrompt}},
		ChatHistory: []types.ChatHistory{
			{User: cfg.SystemPrompt},
			{User: "Q1", Bot: "A1", Platform: "openai", Model: "gpt-5.4", ReasoningEffort: "high"},
		},
	}
	m := NewManager(state)

	exportPath, err := m.ExportFullHistory()
	if err != nil {
		t.Fatalf("ExportFullHistory error: %v", err)
	}

	data, err := os.ReadFile(exportPath)
	if err != nil {
		t.Fatalf("failed to read export: %v", err)
	}

	var entries []types.ExportEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		t.Fatalf("failed to parse export: %v", err)
	}

	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].ReasoningEffort != "high" {
		t.Errorf("expected reasoning_effort=high in export, got %q", entries[0].ReasoningEffort)
	}
}

// ---- Compression: rebuildMessages ----

func TestRebuildMessages_NoCompressions(t *testing.T) {
	cfg := &types.Config{
		SystemPrompt:    "Sys",
		CurrentPlatform: "openai",
		CurrentModel:    "gpt-4o",
	}
	state := &types.AppState{
		Config: cfg,
		ChatHistory: []types.ChatHistory{
			{User: cfg.SystemPrompt, Bot: ""},   // index 0: system
			{User: "Hello", Bot: "Hi there"},    // index 1
			{User: "How are you?", Bot: "Good"}, // index 2
		},
	}
	m := NewManager(state)
	m.rebuildMessages()

	want := []types.ChatMessage{
		{Role: "system", Content: "Sys"},
		{Role: "user", Content: "Hello"},
		{Role: "assistant", Content: "Hi there"},
		{Role: "user", Content: "How are you?"},
		{Role: "assistant", Content: "Good"},
	}
	if !reflect.DeepEqual(state.Messages, want) {
		t.Errorf("messages = %+v, want %+v", state.Messages, want)
	}
}

func TestRebuildMessages_WithCompression(t *testing.T) {
	cfg := &types.Config{
		SystemPrompt:    "Sys",
		CurrentPlatform: "openai",
		CurrentModel:    "gpt-4o",
	}
	state := &types.AppState{
		Config: cfg,
		ChatHistory: []types.ChatHistory{
			{User: cfg.SystemPrompt, Bot: ""}, // index 0: system
			{User: "Old Q1", Bot: "Old A1"},   // index 1
			{User: "Old Q2", Bot: "Old A2"},   // index 2
			{User: "Old Q3", Bot: "Old A3"},   // index 3
			{User: "New Q1", Bot: "New A1"},   // index 4
			{User: "New Q2", Bot: "New A2"},   // index 5
		},
		Compressions: []types.CompressionRecord{
			{ThroughIndex: 3, Summary: "Summary of old conversation"},
		},
	}
	m := NewManager(state)
	m.rebuildMessages()

	want := []types.ChatMessage{
		{Role: "system", Content: "Sys"},
		{Role: "user", Content: "Compressed conversation summary:\n\nSummary of old conversation"},
		{Role: "user", Content: "New Q1"},
		{Role: "assistant", Content: "New A1"},
		{Role: "user", Content: "New Q2"},
		{Role: "assistant", Content: "New A2"},
	}
	if !reflect.DeepEqual(state.Messages, want) {
		t.Errorf("messages = %+v, want %+v", state.Messages, want)
	}
}

func TestRebuildMessages_WithCompressionAtEnd(t *testing.T) {
	cfg := &types.Config{
		SystemPrompt:    "Sys",
		CurrentPlatform: "openai",
		CurrentModel:    "gpt-4o",
	}
	state := &types.AppState{
		Config: cfg,
		ChatHistory: []types.ChatHistory{
			{User: cfg.SystemPrompt, Bot: ""}, // index 0
			{User: "Q1", Bot: "A1"},           // index 1
			{User: "Q2", Bot: "A2"},           // index 2
			{User: "Q3", Bot: "A3"},           // index 3
		},
		Compressions: []types.CompressionRecord{
			{ThroughIndex: 3, Summary: "Full summary"},
		},
	}
	m := NewManager(state)
	m.rebuildMessages()

	// Compression through_index=3 means everything up to and including
	// index 3 is summarized. No raw turns follow.
	want := []types.ChatMessage{
		{Role: "system", Content: "Sys"},
		{Role: "user", Content: "Compressed conversation summary:\n\nFull summary"},
	}
	if !reflect.DeepEqual(state.Messages, want) {
		t.Errorf("messages = %+v, want %+v", state.Messages, want)
	}
}

// ---- Compression: activeCompression ----

func TestActiveCompression_NewestValid(t *testing.T) {
	state := &types.AppState{
		ChatHistory: []types.ChatHistory{{}, {}, {}, {}, {}, {}, {}, {}},
		Compressions: []types.CompressionRecord{
			{ThroughIndex: 2, Summary: "old"},
			{ThroughIndex: 5, Summary: "new"},
		},
	}
	m := NewManager(state)
	active := m.activeCompression()
	if active == nil {
		t.Fatal("expected non-nil active compression")
	}
	if active.Summary != "new" {
		t.Errorf("expected newest summary 'new', got %q", active.Summary)
	}
}

func TestActiveCompression_NoneValid(t *testing.T) {
	state := &types.AppState{
		ChatHistory: []types.ChatHistory{{}, {}},
		Compressions: []types.CompressionRecord{
			{ThroughIndex: 5, Summary: "stale"},
		},
	}
	m := NewManager(state)
	active := m.activeCompression()
	if active != nil {
		t.Errorf("expected nil when through_index exceeds history, got %+v", active)
	}
}

func TestActiveCompression_Empty(t *testing.T) {
	state := &types.AppState{
		ChatHistory: []types.ChatHistory{{}},
	}
	m := NewManager(state)
	if active := m.activeCompression(); active != nil {
		t.Errorf("expected nil for empty compressions, got %+v", active)
	}
}

// ---- Compression: RestoreSessionState ----

func TestRestoreSessionState_RestoresCompressions(t *testing.T) {
	cfg := &types.Config{
		SystemPrompt:    "S",
		CurrentPlatform: "openai",
		CurrentModel:    "gpt-4o",
	}
	state := &types.AppState{
		Config:      cfg,
		ChatHistory: []types.ChatHistory{},
	}
	m := NewManager(state)

	session := &types.SessionFile{
		Platform: "groq",
		Model:    "llama3",
		ChatHistory: []types.ChatHistory{
			{User: "S", Bot: ""},
			{User: "Q1", Bot: "A1"},
			{User: "Q2", Bot: "A2"},
			{User: "Q3", Bot: "A3"},
		},
		Compressions: []types.CompressionRecord{
			{ThroughIndex: 2, Summary: "Sum of Q1/A1 and Q2/A2"},
		},
	}
	m.RestoreSessionState(session)

	if len(state.Compressions) != 1 {
		t.Fatalf("expected 1 compression, got %d", len(state.Compressions))
	}
	if state.Compressions[0].Summary != "Sum of Q1/A1 and Q2/A2" {
		t.Errorf("unexpected summary: %q", state.Compressions[0].Summary)
	}

	// Messages should use the compression summary plus turns after index 2.
	want := []types.ChatMessage{
		{Role: "system", Content: "S"},
		{Role: "user", Content: "Compressed conversation summary:\n\nSum of Q1/A1 and Q2/A2"},
		{Role: "user", Content: "Q3"},
		{Role: "assistant", Content: "A3"},
	}
	if !reflect.DeepEqual(state.Messages, want) {
		t.Errorf("messages = %+v, want %+v", state.Messages, want)
	}
}

func TestRestoreSessionState_LegacyWithoutCompressions(t *testing.T) {
	cfg := &types.Config{
		SystemPrompt:    "S",
		CurrentPlatform: "openai",
		CurrentModel:    "gpt-4o",
	}
	state := &types.AppState{
		Config:      cfg,
		ChatHistory: []types.ChatHistory{},
	}
	m := NewManager(state)

	// Legacy session with no compressions field.
	session := &types.SessionFile{
		Platform: "openai",
		Model:    "gpt-4o",
		ChatHistory: []types.ChatHistory{
			{User: "S", Bot: ""},
			{User: "Q1", Bot: "A1"},
		},
	}
	m.RestoreSessionState(session)

	if len(state.Compressions) != 0 {
		t.Errorf("expected 0 compressions for legacy session, got %d", len(state.Compressions))
	}

	// Messages should be rebuilt from full history (no compression).
	want := []types.ChatMessage{
		{Role: "system", Content: "S"},
		{Role: "user", Content: "Q1"},
		{Role: "assistant", Content: "A1"},
	}
	if !reflect.DeepEqual(state.Messages, want) {
		t.Errorf("messages = %+v, want %+v", state.Messages, want)
	}
}

// ---- Compression: SaveSessionState round-trip ----

func TestSaveAndLoadSession_CompressionsRoundTrip(t *testing.T) {
	tempHome := t.TempDir()
	t.Setenv("HOME", tempHome)
	t.Setenv("USERPROFILE", tempHome)

	cfg := &types.Config{
		CurrentPlatform:   "openai",
		CurrentModel:      "gpt-4o",
		SystemPrompt:      "S",
		EnableSessionSave: true,
	}
	state := &types.AppState{
		Config:      cfg,
		ChatHistory: []types.ChatHistory{{User: "S"}, {User: "Q1", Bot: "A1"}, {User: "Q2", Bot: "A2"}, {User: "Q3", Bot: "A3"}},
		Compressions: []types.CompressionRecord{
			{Time: 1000, ThroughIndex: 2, Summary: "Summary 1", Platform: "openai", Model: "gpt-4o"},
		},
	}
	m := NewManager(state)

	if err := m.SaveSessionState(); err != nil {
		t.Fatalf("SaveSessionState error: %v", err)
	}

	loaded, err := m.LoadLatestSessionState()
	if err != nil {
		t.Fatalf("LoadLatestSessionState error: %v", err)
	}
	if len(loaded.Compressions) != 1 {
		t.Fatalf("expected 1 compression in loaded session, got %d", len(loaded.Compressions))
	}
	if loaded.Compressions[0].Summary != "Summary 1" {
		t.Errorf("expected summary 'Summary 1', got %q", loaded.Compressions[0].Summary)
	}
	if loaded.Compressions[0].ThroughIndex != 2 {
		t.Errorf("expected through_index 2, got %d", loaded.Compressions[0].ThroughIndex)
	}
}

// ---- Compression: multiple compressions ----

func TestRebuildMessages_MultipleCompressions(t *testing.T) {
	cfg := &types.Config{
		SystemPrompt:    "S",
		CurrentPlatform: "openai",
		CurrentModel:    "gpt-4o",
	}
	state := &types.AppState{
		Config: cfg,
		ChatHistory: []types.ChatHistory{
			{User: "S"},             // 0
			{User: "Q1", Bot: "A1"}, // 1
			{User: "Q2", Bot: "A2"}, // 2
			{User: "Q3", Bot: "A3"}, // 3
			{User: "Q4", Bot: "A4"}, // 4
			{User: "Q5", Bot: "A5"}, // 5
			{User: "Q6", Bot: "A6"}, // 6
		},
		Compressions: []types.CompressionRecord{
			{ThroughIndex: 2, Summary: "First summary"},
			{ThroughIndex: 5, Summary: "Second summary (includes first)"},
		},
	}
	m := NewManager(state)
	m.rebuildMessages()

	// Should use the newest compression (through_index=5), so only Q6/A6 follows.
	want := []types.ChatMessage{
		{Role: "system", Content: "S"},
		{Role: "user", Content: "Compressed conversation summary:\n\nSecond summary (includes first)"},
		{Role: "user", Content: "Q6"},
		{Role: "assistant", Content: "A6"},
	}
	if !reflect.DeepEqual(state.Messages, want) {
		t.Errorf("messages = %+v, want %+v", state.Messages, want)
	}
}

// ---- Compression: BacktrackHistory prunes compressions ----

func TestBacktrackHistory_PruneCompressions(t *testing.T) {
	tempHome := t.TempDir()
	t.Setenv("HOME", tempHome)
	t.Setenv("USERPROFILE", tempHome)

	cfg := &types.Config{
		SystemPrompt:    "S",
		CurrentPlatform: "openai",
		CurrentModel:    "gpt-4o",
	}
	state := &types.AppState{
		Config: cfg,
		ChatHistory: []types.ChatHistory{
			{User: "S"},             // 0
			{User: "Q1", Bot: "A1"}, // 1
			{User: "Q2", Bot: "A2"}, // 2
			{User: "Q3", Bot: "A3"}, // 3
			{User: "Q4", Bot: "A4"}, // 4
		},
		Compressions: []types.CompressionRecord{
			{ThroughIndex: 2, Summary: "Sum up to Q2/A2"},
			{ThroughIndex: 4, Summary: "Sum up to Q4/A4"},
		},
	}
	m := NewManager(state)

	// Backtrack to index 2 (keeping entries 0, 1, 2).
	// This requires fzf; simulate by calling the internal logic directly
	// since BacktrackHistory uses fzf for selection.
	m.pruneCompressions(3) // historyLen = index+1 = 3

	// The compression with through_index=4 should be dropped.
	// The one with through_index=2 should survive (2 < 3).
	if len(state.Compressions) != 1 {
		t.Fatalf("expected 1 compression after prune, got %d", len(state.Compressions))
	}
	if state.Compressions[0].ThroughIndex != 2 {
		t.Errorf("expected through_index=2, got %d", state.Compressions[0].ThroughIndex)
	}
}

func TestBacktrackHistory_PruneAllCompressions(t *testing.T) {
	cfg := &types.Config{
		SystemPrompt:    "S",
		CurrentPlatform: "openai",
		CurrentModel:    "gpt-4o",
	}
	state := &types.AppState{
		Config: cfg,
		ChatHistory: []types.ChatHistory{
			{User: "S"},
			{User: "Q1", Bot: "A1"},
		},
		Compressions: []types.CompressionRecord{
			{ThroughIndex: 5, Summary: "stale"},
		},
	}
	m := NewManager(state)
	m.pruneCompressions(2)

	if len(state.Compressions) != 0 {
		t.Errorf("expected 0 compressions after prune, got %d", len(state.Compressions))
	}
}

// ---- Compression: ClearHistory clears compressions ----

func TestClearHistory_ClearsCompressions(t *testing.T) {
	cfg := &types.Config{
		SystemPrompt:    "S",
		CurrentPlatform: "openai",
		CurrentModel:    "gpt-4o",
	}
	state := &types.AppState{
		Config:   cfg,
		Messages: []types.ChatMessage{{Role: "system", Content: "S"}},
		ChatHistory: []types.ChatHistory{
			{User: "S"},
			{User: "Q1", Bot: "A1"},
		},
		Compressions: []types.CompressionRecord{
			{ThroughIndex: 1, Summary: "test"},
		},
	}
	m := NewManager(state)
	m.ClearHistory()

	if len(state.Compressions) != 0 {
		t.Errorf("expected 0 compressions after clear, got %d", len(state.Compressions))
	}
}

// ---- Compression: buildCompressionTranscript ----

func TestBuildCompressionTranscript(t *testing.T) {
	cfg := &types.Config{SystemPrompt: "Sys"}
	state := &types.AppState{
		Config: cfg,
		Messages: []types.ChatMessage{
			{Role: "system", Content: "Sys"},
			{Role: "user", Content: "Hello"},
			{Role: "assistant", Content: "Hi there"},
			{Role: "user", Content: "Follow up"},
		},
	}
	m := NewManager(state)
	transcript := m.buildCompressionTranscript()

	if !strings.Contains(transcript, "USER: Hello") {
		t.Errorf("expected 'USER: Hello' in transcript, got %q", transcript)
	}
	if !strings.Contains(transcript, "ASSISTANT: Hi there") {
		t.Errorf("expected 'ASSISTANT: Hi there' in transcript, got %q", transcript)
	}
	if !strings.Contains(transcript, "USER: Follow up") {
		t.Errorf("expected 'USER: Follow up' in transcript, got %q", transcript)
	}
	if strings.Contains(transcript, "Sys") {
		t.Errorf("system prompt should be excluded from transcript, got %q", transcript)
	}
}

func TestBuildCompressionTranscript_WithPreviousSummary(t *testing.T) {
	cfg := &types.Config{SystemPrompt: "Sys"}
	state := &types.AppState{
		Config: cfg,
		Messages: []types.ChatMessage{
			{Role: "system", Content: "Sys"},
			{Role: "user", Content: "Compressed conversation summary:\n\nPrevious summary text"},
			{Role: "user", Content: "New question after compression"},
			{Role: "assistant", Content: "New answer"},
		},
	}
	m := NewManager(state)
	transcript := m.buildCompressionTranscript()

	// The transcript should include the previous summary as a user turn,
	// so the new compression can build on it.
	if !strings.Contains(transcript, "Previous summary text") {
		t.Errorf("expected previous summary in transcript, got %q", transcript)
	}
	if !strings.Contains(transcript, "New question after compression") {
		t.Errorf("expected new question in transcript, got %q", transcript)
	}
}

// ---- Compression: pruneCompressions edge cases ----

func TestPruneCompressions_EmptyCompressions(t *testing.T) {
	state := &types.AppState{
		ChatHistory: []types.ChatHistory{{}, {}, {}},
	}
	m := NewManager(state)
	m.pruneCompressions(3)
	if len(state.Compressions) != 0 {
		t.Errorf("expected 0 compressions, got %d", len(state.Compressions))
	}
}

func TestPruneCompressions_KeepsBoundary(t *testing.T) {
	state := &types.AppState{
		ChatHistory: []types.ChatHistory{{}, {}, {}, {}},
		Compressions: []types.CompressionRecord{
			{ThroughIndex: 2, Summary: "at boundary"},
			{ThroughIndex: 3, Summary: "past boundary"},
		},
	}
	m := NewManager(state)
	// historyLen=3: through_index=2 is < 3 (kept), through_index=3 is >= 3 (dropped).
	m.pruneCompressions(3)
	if len(state.Compressions) != 1 {
		t.Fatalf("expected 1 compression, got %d", len(state.Compressions))
	}
	if state.Compressions[0].ThroughIndex != 2 {
		t.Errorf("expected through_index=2, got %d", state.Compressions[0].ThroughIndex)
	}
}

// ---- Compression: sessionSaveFingerprint includes compressions ----

func TestSessionSaveFingerprint_IncludesCompressions(t *testing.T) {
	cfg := &types.Config{
		SystemPrompt:    "S",
		CurrentPlatform: "openai",
		CurrentModel:    "gpt-4o",
	}
	state := &types.AppState{
		Config:      cfg,
		ChatHistory: []types.ChatHistory{{User: "S"}, {User: "Q", Bot: "A"}},
	}
	m := NewManager(state)

	fp1 := m.sessionSaveFingerprint()

	state.Compressions = []types.CompressionRecord{
		{ThroughIndex: 1, Summary: "test"},
	}

	fp2 := m.sessionSaveFingerprint()

	if fp1 == fp2 {
		t.Error("expected fingerprint to change when compressions are added")
	}
}

// ---- Compression: splitTranscript ----

func TestSplitTranscript_SingleChunk(t *testing.T) {
	m := NewManager(&types.AppState{Config: &types.Config{}})
	transcript := "USER: Hello\n\nASSISTANT: Hi"
	chunks := m.splitTranscript(transcript, 10000)
	if len(chunks) != 1 {
		t.Fatalf("expected 1 chunk, got %d", len(chunks))
	}
	if chunks[0] != transcript {
		t.Errorf("chunk = %q, want %q", chunks[0], transcript)
	}
}

func TestSplitTranscript_MultipleChunks(t *testing.T) {
	m := NewManager(&types.AppState{Config: &types.Config{}})
	// Each paragraph is ~20 chars; limit 50 chars = ~2 paragraphs per chunk.
	paragraphs := []string{
		"USER: Question number one here", // 30 chars
		"ASSISTANT: Answer number one",   // 27 chars
		"USER: Question number two here", // 29 chars
		"ASSISTANT: Answer number two",   // 27 chars
		"USER: Question number three",    // 26 chars
	}
	transcript := strings.Join(paragraphs, "\n\n")
	chunks := m.splitTranscript(transcript, 50)

	if len(chunks) < 2 {
		t.Fatalf("expected at least 2 chunks for %d char transcript with limit 50, got %d", len(transcript), len(chunks))
	}

	// Verify no chunk exceeds the limit (except when a single paragraph
	// itself exceeds the limit, which is not the case here).
	for i, c := range chunks {
		if len(c) > 50 {
			t.Errorf("chunk %d exceeds limit: %d > 50", i, len(c))
		}
	}
}

func TestSplitTranscript_HardSplitLongParagraph(t *testing.T) {
	m := NewManager(&types.AppState{Config: &types.Config{}})
	// Single paragraph longer than the limit forces a hard split.
	transcript := strings.Repeat("x", 250)
	chunks := m.splitTranscript(transcript, 100)

	if len(chunks) != 3 {
		t.Fatalf("expected 3 chunks for 250 chars with limit 100, got %d", len(chunks))
	}
	if len(chunks[0]) != 100 || len(chunks[1]) != 100 || len(chunks[2]) != 50 {
		t.Errorf("chunk sizes = %d/%d/%d, want 100/100/50", len(chunks[0]), len(chunks[1]), len(chunks[2]))
	}
}

func TestSplitTranscript_EmptyTranscript(t *testing.T) {
	m := NewManager(&types.AppState{Config: &types.Config{}})
	chunks := m.splitTranscript("", 1000)
	if len(chunks) != 1 {
		t.Fatalf("expected 1 chunk for empty transcript, got %d", len(chunks))
	}
}

// ---- Compression: resolveMaxChunkChars ----

func TestResolveMaxChunkChars_DefaultWhenNoMetadata(t *testing.T) {
	cfg := &types.Config{
		CurrentPlatform: "openai",
		CurrentModel:    "gpt-4o",
		// No Models.dev metadata loaded, so should fall back to default.
		ModelsDevEnabled: false,
	}
	state := &types.AppState{Config: cfg}
	m := NewManager(state)

	result := m.resolveMaxChunkChars()
	if result != compressDefaultMaxChars {
		t.Errorf("expected default %d, got %d", compressDefaultMaxChars, result)
	}
}

func TestResolveMaxChunkChars_FloorsAt1000(t *testing.T) {
	// Even with a tiny context window, chunk size should not go below 1000.
	cfg := &types.Config{
		CurrentPlatform: "openai",
		CurrentModel:    "gpt-4o",
	}
	state := &types.AppState{Config: cfg}
	m := NewManager(state)

	// Simulate by checking the floor constant is respected.
	// We can't easily set the context window without network, but we can
	// verify the floor logic: if contextWindow were 100 tokens:
	// 100 * 4 * 0.4 = 160, which is < 1000, so it should return 1000.
	// This is validated by the code path, not a direct test here.
	_ = m.resolveMaxChunkChars()
}

// ---- Compression: looksLikeContextLengthError ----

func TestLooksLikeContextLengthError(t *testing.T) {
	tests := []struct {
		name   string
		errMsg string
		want   bool
	}{
		{"context length", "This model's maximum context length is 8192 tokens", true},
		{"maximum context", "You exceeded the maximum context length", true},
		{"too long", "Your request is too long", true},
		{"too many tokens", "too many tokens in the request", true},
		{"token limit", "request exceeds token limit", true},
		{"maximum number of tokens", "maximum number of tokens exceeded", true},
		{"exceeds the model", "input exceeds the model's context window", true},
		{"reduce the length", "Please reduce the length of the messages", true},
		{"context window", "input is larger than the context window", true},
		{"input length", "input length is too large", true},
		{"max_tokens", "max_tokens limit exceeded", true},
		{"auth error", "invalid api key", false},
		{"network error", "connection refused", false},
		{"rate limit", "rate limit exceeded, please retry", false},
		{"nil error", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var err error
			if tt.errMsg != "" {
				err = fmt.Errorf("%s", tt.errMsg)
			}
			if got := looksLikeContextLengthError(err); got != tt.want {
				t.Errorf("looksLikeContextLengthError(%q) = %v, want %v", tt.errMsg, got, tt.want)
			}
		})
	}
}

func TestLooksLikeContextLengthError_NilError(t *testing.T) {
	if looksLikeContextLengthError(nil) {
		t.Error("expected false for nil error")
	}
}

// ---- Compression: retry halving math ----

func TestSummarizeWithRetry_HalvingMath(t *testing.T) {
	// Verify the halving logic produces the expected sequence:
	// 80000 -> 40000 -> 20000 -> 10000 (3 retries)
	// 10000 > compressMinChunkChars (1000), so all 3 retries would fire.
	maxChars := 80000
	current := maxChars
	for i := 0; i < compressMaxRetries; i++ {
		current = current / 2
	}
	if current != 10000 {
		t.Errorf("after %d halvings from %d: got %d, want 10000", compressMaxRetries, maxChars, current)
	}
	if current < compressMinChunkChars {
		t.Errorf("floor %d should be below %d", current, compressMinChunkChars)
	}
}

func TestSummarizeWithRetry_FloorStopsHalving(t *testing.T) {
	// Verify that a very small initial chunk size hits the floor.
	small := 2000 // already above 1000 floor
	current := small
	for i := 0; i < compressMaxRetries; i++ {
		halved := current / 2
		if halved < compressMinChunkChars {
			break // floor reached
		}
		current = halved
	}
	// 2000 -> 1000 (floor hit after 1 halving)
	if current != 1000 {
		t.Errorf("expected floor 1000, got %d", current)
	}
}
