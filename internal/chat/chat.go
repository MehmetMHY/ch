package chat

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/MehmetMHY/ch/internal/ui"
	"github.com/MehmetMHY/ch/pkg/types"
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

// ExportFullHistory exports the entire chat history to a JSON file.
func (m *Manager) ExportFullHistory() (string, error) {
	if len(m.state.ChatHistory) <= 1 {
		return "", fmt.Errorf("no chat history to export")
	}

	chatID := uuid.New().String()
	filename := fmt.Sprintf("ch_%s.json", chatID)

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

// ExportLastResponse saves the last bot response to a text file.
func (m *Manager) ExportLastResponse() (string, error) {
	if len(m.state.ChatHistory) <= 1 {
		return "", fmt.Errorf("no chat history to save")
	}

	lastEntry := m.state.ChatHistory[len(m.state.ChatHistory)-1]
	if lastEntry.Bot == "" {
		return "", fmt.Errorf("no response to save")
	}

	filename := fmt.Sprintf("ch_response_%d.txt", time.Now().Unix())

	currentDir, err := os.Getwd()
	if err != nil {
		return "", err
	}

	fullPath := filepath.Join(currentDir, filename)

	err = os.WriteFile(fullPath, []byte(lastEntry.Bot), 0644)
	if err != nil {
		return "", err
	}

	return fullPath, nil
}

// BacktrackHistory allows the user to select a previous message to revert to.
// It returns the number of messages that were backtracked.
func (m *Manager) BacktrackHistory() (int, error) {
	if len(m.state.ChatHistory) <= 1 {
		return 0, fmt.Errorf("No history to backtrack")
	}

	var items []string
	for i, entry := range m.state.ChatHistory {
		if i == 0 {
			continue // Skip system prompt
		}
		preview := strings.Split(entry.User, "\n")[0]
		if len(preview) > 80 {
			preview = preview[:80] + "..."
		}
		items = append(items, fmt.Sprintf("%d: %s - %s", i, time.Unix(entry.Time, 0).Format("2006-01-02 15:04:05"), preview))
	}

	// Reverse items to show most recent first
	for i, j := 0, len(items)-1; i < j; i, j = i+1, j-1 {
		items[i], items[j] = items[j], items[i]
	}

	cmd := exec.Command("fzf", "--reverse", "--height=40%", "--border", "--prompt=Select a message to backtrack to: ")
	cmd.Stdin = strings.NewReader(strings.Join(items, "\n"))

	out, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 130 {
			return 0, nil // fzf selection cancelled by user
		}
		return 0, fmt.Errorf("fzf selection failed: %v", err)
	}

	selected := strings.TrimSpace(string(out))
	if selected == "" {
		return 0, fmt.Errorf("no message selected")
	}

	parts := strings.Split(selected, ":")
	if len(parts) < 1 {
		return 0, fmt.Errorf("invalid selection format")
	}

	index := 0
	_, err = fmt.Sscanf(parts[0], "%d", &index)
	if err != nil {
		return 0, fmt.Errorf("could not parse index: %v", err)
	}

	if index <= 0 || index >= len(m.state.ChatHistory) {
		return 0, fmt.Errorf("invalid index selected")
	}

	originalHistoryCount := len(m.state.ChatHistory)
	m.state.ChatHistory = m.state.ChatHistory[:index+1]
	backtrackedCount := originalHistoryCount - len(m.state.ChatHistory)

	m.state.Messages = []types.ChatMessage{
		{Role: "system", Content: m.state.Config.SystemPrompt},
	}
	for _, entry := range m.state.ChatHistory[1:] {
		m.state.Messages = append(m.state.Messages, types.ChatMessage{Role: "user", Content: entry.User})
		m.state.Messages = append(m.state.Messages, types.ChatMessage{Role: "assistant", Content: entry.Bot})
	}

	return backtrackedCount, nil
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

	tmpFile, err := ioutil.TempFile(tmpDir, "ch-*.txt")
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

// ExportCodeBlocks extracts and saves all code blocks from the last bot response
func (m *Manager) ExportCodeBlocks() ([]string, error) {
	if len(m.state.ChatHistory) <= 1 {
		return nil, fmt.Errorf("no chat history available")
	}

	lastEntry := m.state.ChatHistory[len(m.state.ChatHistory)-1]
	if lastEntry.Bot == "" {
		return nil, fmt.Errorf("no bot response to export from")
	}

	// Regex to find markdown code blocks with optional language specification
	codeBlockRegex := regexp.MustCompile("(?s)```([a-zA-Z0-9]*)\n(.*?)\n```")
	matches := codeBlockRegex.FindAllStringSubmatch(lastEntry.Bot, -1)

	if len(matches) == 0 {
		return nil, fmt.Errorf("no code blocks found in the last response")
	}

	var filePaths []string
	currentDir, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("failed to get current directory: %v", err)
	}

	for i, match := range matches {
		language := match[1]
		code := match[2]

		// Generate filename based on language or use generic extension
		var extension string
		switch strings.ToLower(language) {
		case "python", "py":
			extension = ".py"
		case "go", "golang":
			extension = ".go"
		case "javascript", "js":
			extension = ".js"
		case "typescript", "ts":
			extension = ".ts"
		case "html":
			extension = ".html"
		case "css":
			extension = ".css"
		case "json":
			extension = ".json"
		case "xml":
			extension = ".xml"
		case "yaml", "yml":
			extension = ".yml"
		case "bash", "sh", "shell":
			extension = ".sh"
		case "sql":
			extension = ".sql"
		case "c":
			extension = ".c"
		case "cpp", "c++":
			extension = ".cpp"
		case "java":
			extension = ".java"
		case "rust", "rs":
			extension = ".rs"
		case "php":
			extension = ".php"
		case "ruby", "rb":
			extension = ".rb"
		case "swift":
			extension = ".swift"
		case "kotlin", "kt":
			extension = ".kt"
		case "scala":
			extension = ".scala"
		case "r":
			extension = ".r"
		case "matlab", "m":
			extension = ".m"
		case "perl", "pl":
			extension = ".pl"
		case "lua":
			extension = ".lua"
		case "dockerfile", "docker":
			extension = ".dockerfile"
		case "makefile", "make":
			extension = ".makefile"
		case "toml":
			extension = ".toml"
		case "ini":
			extension = ".ini"
		case "conf", "config":
			extension = ".conf"
		case "md", "markdown":
			extension = ".md"
		case "txt", "text":
			extension = ".txt"
		default:
			if language != "" {
				extension = "." + language
			} else {
				extension = ".txt"
			}
		}

		// Generate unique filename
		baseID := uuid.New().String()[:8]
		var filename string
		if len(matches) == 1 {
			filename = fmt.Sprintf("export_%s%s", baseID, extension)
		} else {
			filename = fmt.Sprintf("export_%s_%d%s", baseID, i+1, extension)
		}

		fullPath := filepath.Join(currentDir, filename)

		// Write code to file
		err = os.WriteFile(fullPath, []byte(code), 0644)
		if err != nil {
			return filePaths, fmt.Errorf("failed to write file %s: %v", filename, err)
		}

		filePaths = append(filePaths, fullPath)
	}

	return filePaths, nil
}

// ExportChatInteractive allows user to select chat entries via fzf, edit in text editor, and save
func (m *Manager) ExportChatInteractive(terminal *ui.Terminal) (string, error) {
	if len(m.state.ChatHistory) <= 1 {
		return "", fmt.Errorf("no chat history to export")
	}

	// Prepare chat entries for fzf selection
	var items []string
	var chatEntries []types.ChatHistory

	for i, entry := range m.state.ChatHistory {
		if i == 0 {
			continue // Skip system prompt
		}

		if entry.User != "" || entry.Bot != "" {
			// Create preview for fzf
			userPreview := strings.Split(entry.User, "\n")[0]
			if len(userPreview) > 60 {
				userPreview = userPreview[:60] + "..."
			}

			timestamp := time.Unix(entry.Time, 0).Format("2006-01-02 15:04:05")
			items = append(items, fmt.Sprintf("%d: %s - %s", i, timestamp, userPreview))
			chatEntries = append(chatEntries, entry)
		}
	}

	if len(items) == 0 {
		return "", fmt.Errorf("no chat entries to export")
	}

	// Use fzf for selection
	selectedItems, err := terminal.FzfMultiSelect(items, "Select chat entries to export (TAB for multiple): ")
	if err != nil {
		return "", fmt.Errorf("selection cancelled or failed: %v", err)
	}

	if len(selectedItems) == 0 {
		return "", fmt.Errorf("no entries selected")
	}

	// Extract selected chat entries
	var selectedEntries []types.ChatHistory
	for _, selectedItem := range selectedItems {
		// Parse the index from the selected item
		parts := strings.Split(selectedItem, ":")
		if len(parts) < 1 {
			continue
		}

		var index int
		_, err := fmt.Sscanf(parts[0], "%d", &index)
		if err != nil {
			continue
		}

		// Find the matching entry
		for i, entry := range m.state.ChatHistory {
			if i == index {
				selectedEntries = append(selectedEntries, entry)
				break
			}
		}
	}

	if len(selectedEntries) == 0 {
		return "", fmt.Errorf("no valid entries found")
	}

	// Build content for editing
	var contentBuilder strings.Builder
	for i, entry := range selectedEntries {
		if i > 0 {
			contentBuilder.WriteString("\n\n" + strings.Repeat("=", 50) + "\n\n")
		}

		timestamp := time.Unix(entry.Time, 0).Format("2006-01-02 15:04:05")
		contentBuilder.WriteString(fmt.Sprintf("Entry %d - %s - %s/%s\n\n", i+1, timestamp, entry.Platform, entry.Model))

		if entry.User != "" {
			contentBuilder.WriteString("USER:\n")
			contentBuilder.WriteString(entry.User)
			contentBuilder.WriteString("\n\n")
		}

		if entry.Bot != "" {
			contentBuilder.WriteString("ASSISTANT:\n")
			contentBuilder.WriteString(entry.Bot)
			contentBuilder.WriteString("\n")
		}
	}

	// Open in text editor for modification
	editedContent, err := m.openInEditor(contentBuilder.String())
	if err != nil {
		return "", fmt.Errorf("error opening editor: %v", err)
	}

	if strings.TrimSpace(editedContent) == "" {
		return "", fmt.Errorf("no content to save")
	}

	// Save to file
	filename := fmt.Sprintf("ch_export_%d.txt", time.Now().Unix())
	currentDir, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("failed to get current directory: %v", err)
	}

	fullPath := filepath.Join(currentDir, filename)
	err = os.WriteFile(fullPath, []byte(editedContent), 0644)
	if err != nil {
		return "", fmt.Errorf("failed to write file: %v", err)
	}

	return fullPath, nil
}

// openInEditor opens content in the user's preferred text editor and returns the edited content
func (m *Manager) openInEditor(content string) (string, error) {
	tmpDir := "/tmp"
	if _, err := os.Stat(tmpDir); os.IsNotExist(err) {
		err = os.MkdirAll(tmpDir, 0755)
		if err != nil {
			return "", fmt.Errorf("error creating tmp directory: %v", err)
		}
	}

	tmpFile, err := ioutil.TempFile(tmpDir, "ch-export-*.txt")
	if err != nil {
		return "", fmt.Errorf("error creating temp file: %v", err)
	}
	tmpFilePath := tmpFile.Name()

	// Write initial content
	_, err = tmpFile.WriteString(content)
	if err != nil {
		tmpFile.Close()
		os.Remove(tmpFilePath)
		return "", fmt.Errorf("error writing to temp file: %v", err)
	}
	tmpFile.Close()

	defer os.Remove(tmpFilePath)

	// Open in editor
	cmd := exec.Command(m.state.Config.PreferredEditor, tmpFilePath)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	err = cmd.Run()
	if err != nil {
		return "", fmt.Errorf("error running editor: %v", err)
	}

	// Read edited content
	editedContent, err := ioutil.ReadFile(tmpFilePath)
	if err != nil {
		return "", fmt.Errorf("error reading edited file: %v", err)
	}

	return string(editedContent), nil
}
