package chat

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/MehmetMHY/cha-go/pkg/types"
	"github.com/google/uuid"
)

// Manager handles chat operations
type Manager struct {
	state *types.AppState
}

// NewManager creates a new chat manager
func NewManager(state *types.AppState) *Manager {
	return &Manager{
		state: state,
	}
}

// AddUserMessage adds a user message to the chat
func (m *Manager) AddUserMessage(content string) {
	m.state.Messages = append(m.state.Messages, types.ChatMessage{
		Role:    "user",
		Content: content,
	})
}

// AddAssistantMessage adds an assistant message to the chat
func (m *Manager) AddAssistantMessage(content string) {
	m.state.Messages = append(m.state.Messages, types.ChatMessage{
		Role:    "assistant",
		Content: content,
	})
}

// AddToHistory adds an entry to the chat history
func (m *Manager) AddToHistory(user, bot string) {
	m.state.ChatHistory = append(m.state.ChatHistory, types.ChatHistory{
		Time:     time.Now().Unix(),
		User:     user,
		Bot:      bot,
		Platform: m.state.Config.CurrentPlatform,
		Model:    m.state.Config.CurrentModel,
	})
}

// RemoveLastUserMessage removes the last user message (for interrupted requests)
func (m *Manager) RemoveLastUserMessage() {
	if len(m.state.Messages) > 0 {
		m.state.Messages = m.state.Messages[:len(m.state.Messages)-1]
	}
}

// ClearHistory clears the chat history
func (m *Manager) ClearHistory() {
	m.state.Messages = []types.ChatMessage{
		{Role: "system", Content: m.state.Config.SystemPrompt},
	}
	m.state.ChatHistory = []types.ChatHistory{
		{Time: time.Now().Unix(), User: m.state.Config.SystemPrompt, Bot: "", Platform: m.state.Config.CurrentPlatform, Model: m.state.Config.CurrentModel},
	}
}

// ExportHistory exports chat history to a JSON file
func (m *Manager) ExportHistory() (string, error) {
	if len(m.state.ChatHistory) <= 1 {
		return "", fmt.Errorf("no chat history to export")
	}

	chatID := uuid.New().String()
	filename := fmt.Sprintf("cha_go_%s.json", chatID)

	currentDir, err := os.Getwd()
	if err != nil {
		return "", err
	}

	fullPath := filepath.Join(currentDir, filename)

	var entries []types.ExportEntry
	for _, entry := range m.state.ChatHistory[1:] {
		if entry.User != "" || entry.Bot != "" {
			entries = append(entries, types.ExportEntry{
				Platform:    entry.Platform,
				ModelName:   entry.Model,
				UserPrompt:  entry.User,
				BotResponse: entry.Bot,
				Timestamp:   entry.Time,
			})
		}
	}

	jsonData, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal JSON: %v", err)
	}

	err = os.WriteFile(fullPath, jsonData, 0644)
	if err != nil {
		return "", err
	}

	return fullPath, nil
}

// HandleTerminalInput handles terminal input mode
func (m *Manager) HandleTerminalInput() (string, error) {
	tmpDir := "/tmp"
	if _, err := os.Stat(tmpDir); os.IsNotExist(err) {
		err = os.MkdirAll(tmpDir, 0755)
		if err != nil {
			return "", fmt.Errorf("error creating tmp directory: %v", err)
		}
	}

	tmpFile, err := ioutil.TempFile(tmpDir, "cha-go-*.txt")
	if err != nil {
		return "", fmt.Errorf("error creating temp file: %v", err)
	}
	tmpFilePath := tmpFile.Name()
	tmpFile.Close()

	defer os.Remove(tmpFilePath)

	cmd := exec.Command(m.state.Config.PreferredEditor, tmpFilePath)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	err = cmd.Run()
	if err != nil {
		return "", fmt.Errorf("error running editor: %v", err)
	}

	content, err := ioutil.ReadFile(tmpFilePath)
	if err != nil {
		return "", fmt.Errorf("error reading temp file: %v", err)
	}

	input := strings.TrimSpace(string(content))
	if input == "" {
		return "", fmt.Errorf("no input provided")
	}

	return input, nil
}

// GetMessages returns the current messages
func (m *Manager) GetMessages() []types.ChatMessage {
	return m.state.Messages
}

// GetCurrentModel returns the current model
func (m *Manager) GetCurrentModel() string {
	return m.state.Config.CurrentModel
}

// SetCurrentModel sets the current model
func (m *Manager) SetCurrentModel(model string) {
	m.state.Config.CurrentModel = model
}

// GetCurrentPlatform returns the current platform
func (m *Manager) GetCurrentPlatform() string {
	return m.state.Config.CurrentPlatform
}

// SetCurrentPlatform sets the current platform
func (m *Manager) SetCurrentPlatform(platform string) {
	m.state.Config.CurrentPlatform = platform
}
