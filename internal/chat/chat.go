package chat

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math/rand"
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

// getTempDir returns the application's temporary directory, creating it if it doesn't exist
func (m *Manager) getTempDir() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home directory: %w", err)
	}

	tempDir := filepath.Join(homeDir, ".ch", "tmp")

	// Create the directory if it doesn't exist
	if err := os.MkdirAll(tempDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create temp directory: %w", err)
	}

	return tempDir, nil
}

// getWorkingEditor tries the preferred editor, falls back to vim, then nano
func (m *Manager) getWorkingEditor(testFile string) string {
	editors := []string{m.state.Config.PreferredEditor, "vim", "nano"}

	for _, editor := range editors {
		// Check if the editor binary exists
		if _, err := exec.LookPath(editor); err != nil {
			continue
		}

		// Special handling for helix - try it but with a quick fallback if it fails
		if editor == "hx" {
			// Test if helix can actually run by trying it with a very brief command
			// If this fails, we'll fall back to vim immediately
			testCmd := exec.Command(editor, "--help")
			if err := testCmd.Run(); err != nil {
				continue // Skip helix and try vim
			}
			// If help works, let's try the real thing but be ready to catch panics
		}

		return editor
	}

	// Final fallback
	return "nano"
}

// runEditorWithFallback tries to run the user's preferred editor, then falls back to common editors.
func (m *Manager) runEditorWithFallback(filePath string) error {
	var editors []string
	if envEditor := os.Getenv("EDITOR"); envEditor != "" {
		editors = append(editors, envEditor)
	}
	editors = append(editors, m.state.Config.PreferredEditor, "vim", "nano")

	// Remove duplicates
	seen := make(map[string]bool)
	uniqueEditors := []string{}
	for _, editor := range editors {
		if editor != "" && !seen[editor] {
			uniqueEditors = append(uniqueEditors, editor)
			seen[editor] = true
		}
	}

	for i, editor := range uniqueEditors {
		// Check if the editor exists
		if _, err := exec.LookPath(editor); err != nil {
			continue
		}

		cmd := exec.Command(editor, filePath)
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout

		// For the first attempts, suppress stderr to avoid showing error messages
		// Only show stderr for the final attempt
		if i < len(uniqueEditors)-1 {
			cmd.Stderr = nil // Suppress error messages for fallback attempts
		} else {
			cmd.Stderr = os.Stderr // Show errors for final attempt
		}

		if err := cmd.Run(); err != nil {
			// If this editor failed, try the next one
			continue
		}

		// Success!
		return nil
	}

	return fmt.Errorf("no working editor found")
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

	// Track newly created file for smart prioritization
	m.AddRecentlyCreatedFile(fullPath)

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

	// Track newly created file for smart prioritization
	m.AddRecentlyCreatedFile(fullPath)

	return fullPath, nil
}

// BacktrackHistory allows the user to select a previous message to revert to.
// It returns the number of messages that were backtracked.
func (m *Manager) BacktrackHistory(terminal *ui.Terminal) (int, error) {
	if len(m.state.ChatHistory) <= 1 {
		return 0, fmt.Errorf("no history to backtrack")
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

	selected, err := terminal.FzfSelect(items, "backtrack to: ")
	if err != nil {
		return 0, fmt.Errorf("fzf selection failed: %v", err)
	}

	if selected == "" {
		return 0, nil // User cancelled selection
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
		if entry.User != "" {
			m.state.Messages = append(m.state.Messages, types.ChatMessage{Role: "user", Content: entry.User})
		}
		if entry.Bot != "" {
			m.state.Messages = append(m.state.Messages, types.ChatMessage{Role: "assistant", Content: entry.Bot})
		}
	}

	return backtrackedCount, nil
}

// HandleTerminalInput handles terminal input mode
func (m *Manager) HandleTerminalInput() (string, error) {
	tmpDir, err := m.getTempDir()
	if err != nil {
		return "", fmt.Errorf("failed to get temp directory: %v", err)
	}

	tmpFile, err := ioutil.TempFile(tmpDir, "ch-*.txt")
	if err != nil {
		return "", fmt.Errorf("error creating temp file: %v", err)
	}
	tmpFilePath := tmpFile.Name()
	tmpFile.Close()

	defer os.Remove(tmpFilePath)

	// Try to run the editor with automatic fallback
	err = m.runEditorWithFallback(tmpFilePath)
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

// GetChatHistory returns the current chat history
func (m *Manager) GetChatHistory() []types.ChatHistory {
	return m.state.ChatHistory
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
func (m *Manager) ExportCodeBlocks(terminal *ui.Terminal) ([]string, error) {
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
		code := match[2]

		// Generate filename options and let user select
		filenameOptions := m.generateFilenameOptions(code)

		prompt := fmt.Sprintf("file %d/%d: ", i+1, len(matches))
		selectedFilename, err := terminal.FzfSelect(filenameOptions, prompt)
		if err != nil {
			return filePaths, fmt.Errorf("filename selection failed: %v", err)
		}

		if selectedFilename == "" {
			// User cancelled - skip this file
			continue
		}

		filename := selectedFilename

		fullPath := filepath.Join(currentDir, filename)

		// Write code to file
		err = os.WriteFile(fullPath, []byte(code), 0644)
		if err != nil {
			return filePaths, fmt.Errorf("failed to write file %s: %v", filename, err)
		}

		// Track newly created file for smart prioritization
		m.AddRecentlyCreatedFile(fullPath)

		filePaths = append(filePaths, fullPath)
	}

	return filePaths, nil
}

// ExportChatInteractive allows user to select chat entries via fzf, edit in text editor, and save
func (m *Manager) ExportChatInteractive(terminal *ui.Terminal) (string, error) {
	if len(m.state.ChatHistory) <= 1 {
		return "", fmt.Errorf("no chat history to export")
	}

	// Ask for edit mode
	editMode, err := terminal.FzfSelect([]string{"auto export", "manual export"}, "select export mode: ")
	if err != nil {
		return "", fmt.Errorf("selection cancelled or failed: %v", err)
	}

	if editMode == "auto export" {
		return m.ExportChatAuto(terminal)
	}

	if editMode == "" {
		return "", nil // User cancelled
	}

	// Prepare chat entries for fzf selection (newest to oldest)
	var items []string
	var chatEntries []types.ChatHistory

	// Iterate in reverse order (newest to oldest)
	for i := len(m.state.ChatHistory) - 1; i >= 1; i-- {
		entry := m.state.ChatHistory[i]

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

	// Add >all option at the top of the list
	fzfOptions := append([]string{">all"}, items...)

	// Use fzf for selection
	selectedItems, err := terminal.FzfMultiSelect(fzfOptions, "export entries (tab=multi): ")
	if err != nil {
		return "", fmt.Errorf("selection cancelled or failed: %v", err)
	}

	if len(selectedItems) == 0 {
		return "", fmt.Errorf("no entries selected")
	}

	// Check if >all was selected
	var selectedEntries []types.ChatHistory
	allSelected := false
	for _, item := range selectedItems {
		if strings.HasPrefix(item, ">all") {
			allSelected = true
			break
		}
	}

	if allSelected {
		// Select all entries
		selectedEntries = chatEntries
	} else {
		// Extract selected chat entries
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

			// Check if this is a file loading entry and try to get the actual content
			if strings.HasPrefix(entry.User, "Loaded: ") {
				actualContent := m.getLoadedContentForHistoryEntry(entry)
				if actualContent != "" {
					// Clean up excessive newlines from loaded content
					cleanedContent := m.cleanupLoadedContent(actualContent)
					contentBuilder.WriteString(cleanedContent)
				} else {
					contentBuilder.WriteString(entry.User)
				}
			} else {
				contentBuilder.WriteString(entry.User)
			}

			contentBuilder.WriteString("\n\n")
		}

		if entry.Bot != "" {
			contentBuilder.WriteString("ASSISTANT:\n")
			contentBuilder.WriteString(entry.Bot)
			contentBuilder.WriteString("\n")
		}
	}

	// Open in text editor for modification (add trailing newline for easier editing)
	editedContent, err := m.openInEditor(contentBuilder.String() + "\n")
	if err != nil {
		return "", fmt.Errorf("error opening editor: %v", err)
	}

	if strings.TrimSpace(editedContent) == "" {
		return "", fmt.Errorf("no content to save")
	}

	// Get all files in current directory (including subdirectories)
	allFiles, err := m.getAllFilesInCurrentDir()
	if err != nil {
		return "", fmt.Errorf("failed to get current directory files: %v", err)
	}

	// Extract loaded files from chat history to prioritize them
	loadedFiles := m.extractLoadedFilesFromHistory()

	// Generate new filename options
	newFileOptions := m.generateFilenameOptions(editedContent)

	// Create unified list of new and existing files, prioritizing .txt
	unifiedOptions := m.createUnifiedFileOptions(".txt", newFileOptions, allFiles, loadedFiles, m.state.RecentlyCreatedFiles)

	selectedOption, err := terminal.FzfSelect(unifiedOptions, "save to file: ")
	if err != nil {
		return "", fmt.Errorf("file selection failed: %v", err)
	}
	if selectedOption == "" {
		return "", fmt.Errorf("export cancelled")
	}

	var filename string
	if strings.HasPrefix(selectedOption, "[w] ") {
		filename = strings.TrimPrefix(selectedOption, "[w] ")
	} else {
		filename = selectedOption
	}

	// Save to file
	currentDir, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("failed to get current directory: %v", err)
	}

	fullPath := filepath.Join(currentDir, filename)
	err = os.WriteFile(fullPath, []byte(editedContent), 0644)
	if err != nil {
		return "", fmt.Errorf("failed to write file: %v", err)
	}

	// Track newly created file for smart prioritization
	m.AddRecentlyCreatedFile(fullPath)

	return "", nil
}

// ExportChatAuto allows user to automatically extract and save code blocks from chat history
func (m *Manager) ExportChatAuto(terminal *ui.Terminal) (string, error) {
	// Step 1: Get all chat entries
	var selectedEntries []types.ChatHistory

	// Iterate in reverse order (newest to oldest)
	for i := len(m.state.ChatHistory) - 1; i >= 1; i-- {
		entry := m.state.ChatHistory[i]
		if entry.User != "" || entry.Bot != "" {
			selectedEntries = append(selectedEntries, entry)
		}
	}

	if len(selectedEntries) == 0 {
		return "", fmt.Errorf("no chat entries to export")
	}

	// Step 2: Extract content
	type ExtractedSnippet struct {
		Content  string
		Language string
	}
	var snippets []ExtractedSnippet
	codeBlockRegex := regexp.MustCompile("(?s)```([a-zA-Z0-9]*)\n(.*?)\n```")

	for _, entry := range selectedEntries {
		if entry.Bot != "" {
			matches := codeBlockRegex.FindAllStringSubmatch(entry.Bot, -1)
			for _, match := range matches {
				language := match[1]
				if language == "" {
					language = "text"
				}
				content := match[2]
				snippets = append(snippets, ExtractedSnippet{Content: content, Language: language})
			}
		}
	}

	if len(snippets) == 0 {
		return "", fmt.Errorf("no code blocks found in selected chat entries")
	}

	// Step 3: Select snippets with fzf
	var snippetOptions []string
	for i, snippet := range snippets {
		preview := strings.Split(snippet.Content, "\n")[0]
		if len(preview) > 80 {
			preview = preview[:80] + "..."
		}
		snippetOptions = append(snippetOptions, fmt.Sprintf("[%d] (%s) %s", i+1, snippet.Language, preview))
	}

	selectedSnippetItems, err := terminal.FzfMultiSelect(snippetOptions, "select snippets to save (tab=multi): ")
	if err != nil {
		return "", fmt.Errorf("snippet selection failed: %v", err)
	}
	if len(selectedSnippetItems) == 0 {
		return "", fmt.Errorf("no snippets selected")
	}

	var selectedSnippets []ExtractedSnippet
	for _, item := range selectedSnippetItems {
		var index int
		if _, err := fmt.Sscanf(item, "[%d]", &index); err == nil && index > 0 && index <= len(snippets) {
			selectedSnippets = append(selectedSnippets, snippets[index-1])
		}
	}

	if len(selectedSnippets) == 0 {
		return "", fmt.Errorf("no valid snippets selected")
	}

	// Step 4 & 5: Get filename and save files for each snippet
	var savedFiles []string
	currentDir, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("failed to get current directory: %v", err)
	}

	// Pre-fetch all files once to avoid doing it in the loop
	allFiles, err := m.getAllFilesInCurrentDir()
	if err != nil {
		return "", fmt.Errorf("failed to get current directory files: %v", err)
	}
	loadedFiles := m.extractLoadedFilesFromHistory()
	recentlyCreatedFiles := m.state.RecentlyCreatedFiles

	for i := 0; i < len(selectedSnippets); i++ {
		snippet := selectedSnippets[i]
		ext := m.getLanguageExtension(snippet.Language)
		language := snippet.Language
		if language == "" || ext == ".txt" {
			language = "?"
		}
		prompt := fmt.Sprintf("[%s %d] save to file: ", language, i+1)

		// Generate new filename options
		newFileOptions := m.generatePrioritizedFilenameOptions(snippet.Content, ext)

		// Create unified list of new and existing files
		unifiedOptions := m.createUnifiedFileOptions(ext, newFileOptions, allFiles, loadedFiles, recentlyCreatedFiles)

		selectedOption, err := terminal.FzfSelect(unifiedOptions, prompt)
		if err != nil {
			return "", fmt.Errorf("file selection failed: %v", err)
		}
		if selectedOption == "" {
			continue // User cancelled, skip to next snippet
		}

		var filename string
		if strings.HasPrefix(selectedOption, "[w] ") {
			filename = strings.TrimPrefix(selectedOption, "[w] ")
		} else {
			filename = selectedOption
		}

		fullPath := filepath.Join(currentDir, filename)
		err = os.WriteFile(fullPath, []byte(snippet.Content), 0644)
		if err != nil {
			return "", fmt.Errorf("failed to write file %s: %v", filename, err)
		}
		m.AddRecentlyCreatedFile(fullPath)
		savedFiles = append(savedFiles, fullPath)
	}

	return "", nil
}

// createUnifiedFileOptions creates a single list of filename options for fzf,
// interleaving suggested new files and existing files for better prioritization.
func (m *Manager) createUnifiedFileOptions(priorityExt string, suggestedFilenames, allFiles, loadedFiles, recentlyCreated []string) []string {
	var options []string
	seen := make(map[string]bool)

	// --- Step 1: Separate suggested files ---
	var suggestedMatching, suggestedOther []string
	for _, file := range suggestedFilenames {
		// The suggested filenames are already prioritized, just need to split them
		if strings.HasSuffix(file, priorityExt) {
			suggestedMatching = append(suggestedMatching, file)
		} else {
			suggestedOther = append(suggestedOther, file)
		}
	}

	// --- Step 2: Build prioritized lists of existing files ---
	var existingMatching, existingOther []string
	seenExisting := make(map[string]bool)

	add := func(file string, list *[]string) {
		if !seenExisting[file] && !strings.HasSuffix(file, "/") {
			*list = append(*list, file)
			seenExisting[file] = true
		}
	}

	// Get all non-directory files
	var allNonDirFiles []string
	for _, file := range allFiles {
		if !strings.HasSuffix(file, "/") {
			allNonDirFiles = append(allNonDirFiles, file)
		}
	}

	// Prioritize matching files
	for _, file := range recentlyCreated {
		if filepath.Ext(file) == priorityExt {
			add(file, &existingMatching)
		}
	}
	for _, file := range loadedFiles {
		if filepath.Ext(file) == priorityExt {
			add(file, &existingMatching)
		}
	}
	for _, file := range allNonDirFiles {
		if filepath.Ext(file) == priorityExt {
			add(file, &existingMatching)
		}
	}

	// Prioritize other files
	for _, file := range recentlyCreated {
		if filepath.Ext(file) != priorityExt {
			add(file, &existingOther)
		}
	}
	for _, file := range loadedFiles {
		if filepath.Ext(file) != priorityExt {
			add(file, &existingOther)
		}
	}
	for _, file := range allNonDirFiles {
		if filepath.Ext(file) != priorityExt {
			add(file, &existingOther)
		}
	}

	// --- Step 3: Assemble the final list ---
	addToOptions := func(file string, isWrite bool) {
		if seen[file] {
			return
		}
		if isWrite {
			options = append(options, "[w] "+file)
		} else {
			options = append(options, file)
		}
		seen[file] = true
	}

	// Add in order: suggested matching, existing matching, suggested other, existing other
	for _, file := range suggestedMatching {
		addToOptions(file, false)
	}
	for _, file := range existingMatching {
		addToOptions(file, true)
	}
	for _, file := range suggestedOther {
		addToOptions(file, false)
	}
	for _, file := range existingOther {
		addToOptions(file, true)
	}

	return options
}

// generatePrioritizedFilenameOptions creates filename options, prioritizing the correct extension.
func (m *Manager) generatePrioritizedFilenameOptions(content, priorityExt string) []string {
	allOptions := m.generateFilenameOptions(content)
	var prioritizedOptions, otherOptions []string

	for _, opt := range allOptions {
		if strings.HasSuffix(opt, priorityExt) {
			prioritizedOptions = append(prioritizedOptions, opt)
		} else {
			otherOptions = append(otherOptions, opt)
		}
	}

	return append(prioritizedOptions, otherOptions...)
}

// openInEditor opens content in the user's preferred text editor and returns the edited content
func (m *Manager) openInEditor(content string) (string, error) {
	tmpDir, err := m.getTempDir()
	if err != nil {
		return "", fmt.Errorf("failed to get temp directory: %v", err)
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

	// Open in editor with automatic fallback
	err = m.runEditorWithFallback(tmpFilePath)
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

// getLanguageExtension maps language identifiers to file extensions
func (m *Manager) getLanguageExtension(language string) string {
	langMap := map[string]string{
		"python": "py", "py": "py",
		"javascript": "js", "js": "js",
		"typescript": "ts", "ts": "ts",
		"go":   "go",
		"java": "java",
		"c":    "c",
		"cpp":  "cpp", "c++": "cpp",
		"csharp": "cs", "cs": "cs",
		"ruby": "rb", "rb": "rb",
		"php":    "php",
		"swift":  "swift",
		"kotlin": "kt",
		"rust":   "rs", "rs": "rs",
		"html": "html",
		"css":  "css",
		"json": "json",
		"yaml": "yaml", "yml": "yaml",
		"markdown": "md", "md": "md",
		"shell": "sh", "sh": "sh", "bash": "sh",
		"sql":        "sql",
		"dockerfile": "Dockerfile",
		"makefile":   "Makefile",
	}
	if ext, ok := langMap[strings.ToLower(language)]; ok {
		return "." + ext
	}
	return ".txt" // Default extension
}

// generateFilenameOptions creates filename options with ch_<hash>.ext format
func (m *Manager) generateFilenameOptions(content string) []string {
	var options []string
	currentDir, _ := os.Getwd()

	// All file extensions from dataset (txt first as requested, then alphabetically)
	extensions := []string{
		// Most requested first
		".txt",
		// All extensions from your dataset
		".vbproj.webinfo",
		".jai",
		".jshintignore",
		".fy",
		".sed",
		".cue",
		".rvt",
		".delete",
		".edge",
		".default",
		".qdl",
		".dk",
		".cgi",
		".H",
		".mv",
		".WTC",
		".r2",
		".10.html",
		".php_OLD",
		".ilb",
		".emf",
		".leo",
		".lgt",
		".ig",
		".psm1",
		".06.html",
		".sj",
		".tfstate.backup",
		".c-objdump",
		".vhf",
		".server",
		".sta",
		".imba",
		".meltem",
		".sublime-mousemap",
		".min_",
		".pkl",
		".old.1",
		".removed",
		".ogv",
		".eot",
		".yrl",
		".metakeys",
		".raw",
		".html.pdf",
		".pkb",
		".admx",
		".seestyle",
		".dcr",
		".lkml",
		".80.html",
		".pyt",
		".mdown",
		".dyalog",
		".27.html",
		".st",
		".aspl",
		".geo",
		".awstats",
		".fgs",
		".lfe",
		".3p",
		".phpl",
		".rake",
		".safe",
		".ts",
		".odm",
		".hht",
		".vark",
		".zsh-theme",
		".dsc",
		".bal",
		".prisma",
		".all",
		".ext",
		".asp.asp",
		".svelte",
		".zap",
		".sgm",
		".epl",
		".png,bmp",
		".new.html",
		".sit",
		".5_mod_for_host",
		".css",
		".XMLHTTP",
		".erb.deface",
		".bc",
		".flr",
		".dox",
		".talon",
		".lvproj",
		".gs",
		".NDM",
		".gpd",
		".numsc",
		".mkdir",
		".mp3.html",
		".ejs.t",
		".cocomore.txt",
		".error-log",
		".ron",
		".wpt",
		".mly",
		".zshenv",
		".mov",
		".01.jpg",
		".rec",
		".vimrc",
		".wsgi",
		".pdf.",
		".bcp",
		".patch",
		".app",
		".html,,",
		".html.inc",
		".PL",
		".com.html",
		".all.hawaii",
		".flake8",
		".jspf",
		".axaml",
		".watchr",
		".pop_formata_viewer",
		".soy",
		".sh-session",
		".mat",
		".dwd",
		".cfswf",
		".prt",
		".bycategory",
		".ph",
		".Controls",
		".mqh",
		".fsti",
		".ho",
		".1-all-languages",
		".wgetrc",
		".tg",
		".phtml",
		".dzn",
		".eio",
		".zimpl",
		".eml",
		".2.pdf",
		".sublime-settings",
		".asax",
		".lsl",
		".pic",
		".gmx",
		".kit",
		".mrc",
		".iac.",
		".car",
		".aww",
		".vcproj",
		".rtx",
		".dfm",
		".idc",
		".ini",
		".xht",
		".store",
		".30.html",
		".lyx",
		".22.html",
		".xdl",
		".05",
		".mht",
		".overpassql",
		".parrot",
		".bin",
		".oliver",
		".env.example",
		".ds",
		".xslt",
		".htm8",
		".ru.html",
		".02.html",
		".pl6",
		".dircolors",
		".l",
		".stats",
		".4DProject",
		".dwt",
		".html_old",
		".hbk",
		".env.testing",
		".jpg.jpg",
		".cocci",
		".boot",
		".2-english",
		".sqf",
		".clang-format",
		".ksy",
		".wpd",
		".3.php",
		".jsm",
		".xmi",
		".msp",
		".html.hl",
		".ida",
		".fcgi",
		".S",
		".htm3",
		".pages-tef",
		".idr",
		".mld",
		".hats",
		".tst",
		".jp1",
		".dwl",
		".php~",
		".owen",
		".lex",
		".css.LCK",
		".snip",
		".X-MAGNIFIER_var_DE",
		".html",
		".Sponsors",
		".pod",
		".hta",
		".pbxproj",
		".biminifinder",
		".old.old",
		".plain",
		".adp",
		".mreply.rc",
		".grunit",
		".docx",
		".25.html",
		".htaccess",
		".GIF",
		".control",
		".opendir",
		".get-meta-tags",
		".tb",
		".mellel",
		".bmp.php",
		".sha384",
		".editorconfig",
		".lektorproject",
		".aj",
		".rno",
		".bdp",
		".tk",
		".sod",
		".pep",
		".mysqli",
		".textile",
		".newsletter",
		".wat",
		".urs",
		".ua",
		".rtfd",
		".chs",
		".wci",
		".printable",
		".controller",
		".wihtm",
		".tnl",
		".mobile",
		".env.prod",
		".eliomi",
		".sru",
		".raku",
		".hh",
		".jpe",
		".svh",
		".touch",
		".zlogout",
		".hoon",
		".exclude",
		".mp4",
		".7-english",
		".info",
		".asp.html",
		".mll",
		".sbk",
		".ned",
		".pod6",
		".arr",
		".raml",
		".pfx",
		".ipynb",
		".arj",
		".mbt",
		".cr",
		".lin",
		".pm",
		".coveragerc",
		".php.mno",
		".hxml",
		".muse",
		".proto",
		".de.html",
		".targets",
		".bs",
		".bak",
		".ps1",
		".uguide",
		".ll",
		".xwp",
		".webapp",
		".asx",
		".23",
		".ICO",
		".var",
		".php1",
		".topojson",
		".ado",
		".druby",
		".xsp-config",
		".cmp",
		".conf",
		".slim",
		".hts",
		".print.",
		".SWF",
		".ASC.",
		"._coffee",
		".qpf",
		".1-rc1",
		".erb",
		".50.html",
		".rei",
		".cabal",
		".tmvx",
		".hocon",
		".fir",
		".lid",
		".dtx",
		".swift",
		".pwd",
		".X-PCONF_var_DE",
		".org.zip",
		".image",
		".36",
		".aim",
		".Run.AdCode",
		".pwr",
		".h++",
		".sdv",
		".livecodescript",
		".files",
		".list.includes",
		".XML",
		".as",
		".test",
		".04",
		".idq",
		".3",
		".markdown",
		".EXE",
		".mso",
		".xml.asp",
		".sagews",
		".clang-tidy",
		".PAGE",
		".wbk",
		".xojo_toolbar",
		".geojson",
		".egov",
		".class",
		".61.html",
		".qll",
		".nim.cfg",
		".webm",
		".pdb",
		".gb",
		".pri",
		".Appraisal",
		".tfrproj",
		".php}",
		".env",
		".las",
		".com-redirect",
		".e",
		".json5",
		".contact",
		".4.html",
		".pas",
		".jps",
		".gemspec",
		".latest",
		".ctp",
		".c3",
		".abw",
		".qs",
		".2ms2",
		".ccs",
		".24stable",
		".fan",
		".array-key-exists",
		".rest",
		".tpt",
		".tar.bz2",
		".sarif",
		".seg",
		".html.printable",
		".pwn",
		".ignore",
		".cob",
		".vshader",
		".gmd",
		".rdf",
		".srv",
		".old.php",
		".au",
		".fichiers",
		".pyw",
		".barnes",
		".nanorc",
		".listevents",
		".en.php",
		".roma",
		".unternehmen",
		".72",
		".send",
		".wpa",
		".m3u",
		".dm",
		".tmv",
		".es.jsp",
		".eslintrc.json",
		".51",
		".tvpi",
		".lvclass",
		".lasso8",
		".gmap",
		".loop",
		".eex",
		".sha224",
		".rmiss",
		".fountain",
		".0.xml",
		".01.html",
		".clue",
		".rbtbar",
		".muf",
		".icl",
		".sce",
		".rst",
		".online",
		".lagda",
		".hm",
		".HXX",
		".apib",
		".stat",
		".PcbDoc",
		".kak",
		".anim",
		".CSS",
		".posting.prep",
		".mdoc",
		".natvis",
		".cirru",
		".sts.php",
		".ai",
		".daisy",
		".metadesc",
		".make",
		".scm",
		".X-GIFTREG_var_DE",
		".ct",
		".asax.cs",
		".photo",
		".csr",
		".jp",
		".sec.cfm",
		".cook",
		".mojo",
		".javascript",
		".jq",
		".1.php",
		".sla",
		".mask",
		".rbuistate",
		".obyx",
		".djvu",
		".content",
		".gql",
		".incl",
		".gsh",
		".X-RMA",
		".cfm",
		".65.html",
		".jte",
		".mdwn",
		".bbclass",
		".graph",
		".numpy",
		".xql",
		".notes",
		".JPG",
		".rm",
		".vb",
		".ins",
		".de.jsp",
		".xsx",
		".js.php",
		".wp6",
		".4dm",
		".toc",
		".pub",
		".kicad_pcb",
		".agc",
		".cqs",
		".a\u200bsp",
		".wlk",
		".opeico",
		".ser",
		".lvlib",
		".sbt",
		".fsx",
		".query",
		".sort",
		".tf",
		".oxh",
		".h\u200btml",
		".PSD",
		".jinja",
		".rtf",
		".googlebook",
		".1-en",
		".bwd",
		".ox",
		".prg",
		".bison",
		".sdg",
		".tmdx",
		".rs.in",
		".action2",
		".darcspatch",
		".xs",
		".rmvb",
		".unity",
		".003.jpg",
		".WMV",
		".pages",
		".wlua",
		".vbs",
		".cl",
		".0.jpg",
		".mi",
		".cz",
		".html.sav",
		".xhtm",
		".vn",
		".coq",
		".off",
		".ML",
		".stylus",
		".razor",
		".rbw",
		".grp",
		".jss",
		".FRK",
		".ibuysss.info",
		".cy",
		".orig.html",
		".axi.erb",
		".Engineer",
		".psw",
		".pl.html",
		".circom",
		".ne",
		".37",
		".AVI",
		".pat",
		".templates",
		".ndoc",
		".coffeekup",
		".comments",
		".nvimrc",
		".part",
		".SEG",
		".clj",
		".8.html",
		".faces",
		".typ",
		".wxi",
		".enu",
		".com.ar",
		".stml",
		"._order",
		".start",
		".hlr",
		".sid",
		".sha1",
		".sfv",
		".cast",
		".wisp",
		".jpg.xml",
		".11",
		".filters",
		".PHP",
		".x10",
		".vssscc",
		".exec",
		".deleted",
		".tm_properties",
		".mvn",
		".yml",
		".cobol",
		".tmux.conf",
		".njk",
		".arc",
		".docm",
		".iuml",
		".rss",
		".latex",
		".lxfml",
		".kicad_sch",
		".nearley",
		".odo",
		".tem",
		".html.html",
		".c++",
		".pvj",
		".dyl",
		".mdpolicy",
		".md5.txt",
		".lang",
		".fsockopen",
		".fnl",
		".pvk",
		".exe",
		".lean",
		".s7",
		".glslf",
		".p4",
		".mail",
		".22",
		".imprimer",
		".br",
		".de.txt",
		".psql",
		".zip,",
		".ik",
		".ackrc",
		".hzp",
		".zpl",
		".prw\n\n",
		".2004.html",
		".indt",
		".X-AFFILIATE_var_DE",
		".out",
		".xsql",
		".6pm",
		".workbook",
		".gco",
		".osg",
		".so",
		".mailsignature",
		".code",
		".eur",
		".download",
		".md2",
		".jpg",
		".engine",
		".tla",
		".asciidoc",
		".search.htm",
		".gts",
		".nginxconf",
		".qmd",
		".pass",
		".sss",
		".xbplate",
		".LN3",
		".gscript",
		".jbuilder",
		".ftl",
		".fdt",
		".qhelp",
		".9",
		".PNG",
		".loc",
		".npmignore",
		".pyp",
		".vala",
		".soh",
		".sublime-project",
		".php4",
		".top",
		".old.html",
		".maxproj",
		".mtl",
		".04.html",
		".bak2",
		".4",
		".sdc",
		".yara",
		".for",
		".show",
		".asxp",
		".conll",
		".pbi",
		".bzl",
		".tea",
		".27",
		".markdownlintignore",
		".gtp",
		".tmp",
		".ahk",
		".mcmeta",
		".filemtime",
		".SQL",
		".envrc",
		".ninja",
		".asddls",
		".old.asp",
		".hokkaido",
		".ily",
		".x68",
		".gsp",
		".json.example",
		".libsonnet",
		".gsx",
		".html.images",
		".a.html",
		".j",
		".insert",
		".sub",
		".tar",
		".sage",
		".save",
		".flex",
		".props",
		".Dsr",
		".iced",
		".mirah",
		".32",
		".gd",
		".ol",
		".fwdn",
		".zeek",
		".fth",
		".enn",
		".yp",
		".ok",
		".fbl",
		".nwctxt",
		".bones",
		".nim",
		".meta",
		".form_jhtml",
		".html\u200e",
		".09",
		".astro",
		".iconv",
		".mts",
		".dpc",
		".admin",
		".wplus",
		".qasm",
		".ecl",
		".wsdl",
		".portal",
		".dogpl",
		".g.",
		".cwl",
		".pp",
		".csslintrc",
		".luau",
		".less",
		".code-snippets",
		".8xp",
		".dof",
		".factor-boot-rc",
		".rbuild",
		".NSF",
		".js.aspx",
		".txt.",
		".access",
		".listing",
		".sublime-syntax",
		".cur",
		".py",
		".nwm",
		".dotsettings",
		".ml",
		".ft",
		".eleventyignore",
		".mmd",
		".soc",
		".bas",
		".back",
		".submit",
		".hs-boot",
		".justfile",
		".chem",
		".cpp",
		".xcam.at",
		".pvx",
		".srdf",
		".fish",
		".Html",
		".jcl",
		".embed",
		".dnn",
		".boo",
		".d-objdump",
		".xi",
		".hqx",
		".templ",
		".sys",
		".htmlc",
		".rdb",
		".scaml",
		".ttml",
		".fla",
		".avi",
		".wws",
		".ztml",
		".CFM",
		".lol",
		".reek",
		".mjml",
		".yang",
		".sog",
		".ec",
		".unsubscribe",
		".hwp",
		".ord",
		".hhi",
		".main",
		".dx",
		".rails",
		".js.gz",
		".shellcheckrc",
		".rs",
		".idl",
		".x-php",
		".1",
		".safariextz",
		".media",
		".clw",
		".rar",
		".ag.php",
		".befunge",
		".env.sample",
		".hack",
		".urdf",
		".ink",
		".shtm",
		".php_cs",
		".JS",
		".tmpl",
		".mysql-connect",
		".file-get-contents",
		".rviz",
		".cson",
		".CGI",
		".bdy",
		".cpy",
		".brx",
		".pgt",
		".Org.master",
		".x3d",
		".gitconfig",
		".lst",
		".fst",
		".corp.footer",
		".mo",
		".xst",
		".dropbox",
		".intr",
		".tif",
		".tscn",
		".luacheckrc",
		".pix",
		".handlebars",
		".gbl",
		".rpy",
		".rbi",
		".htm,",
		".flush",
		".php.sample",
		".xm",
		".X-OFFERS",
		".rvf",
		".apex",
		".pogo",
		".regex",
		".xacro",
		".rst.txt",
		".jlqm",
		".dir_colors",
		".swf.html",
		".PPT",
		".bash_functions",
		".PLD",
		".1.htm",
		".scrbl",
		".ans",
		".odd",
		".opml.config",
		".tex",
		".css.aspx",
		".c++-objdump",
		".xib",
		".xqm",
		".inl",
		".sthlp",
		".fon",
		".frg",
		".tid",
		".net.html",
		".yaml-tmlanguage",
		".itermcolors",
		".adoc",
		".dxp",
		".atomignore",
		".work",
		".hz",
		".lock",
		".env.production",
		".qbs",
		".ytdl",
		".dpatch",
		".mkfile",
		".stan",
		".mkiv",
		".ltr",
		".91",
		".postcss",
		".tvj",
		".gslides",
		".xy",
		".service",
		".tpp",
		".udo",
		".07",
		".arcconfig",
		".Old",
		".jsps",
		".du",
		".ndproj",
		".conf.html",
		".SchDoc",
		".app.src",
		".gsc",
		".dta",
		".bqn",
		".sql.gz",
		".i",
		".txt",
		".browse",
		".pch",
		".gdb",
		".hgignore",
		".pnp",
		".product_details",
		".key",
		".vmdk",
		".slnx",
		".ugmart.ug",
		".pbj",
		".min",
		".link",
		".latte",
		".yasnippet",
		".dic",
		".development",
		".btm",
		".joe",
		".htm.",
		".omfl",
		".aw",
		".thrift",
		".flf",
		".2.zip",
		".jison",
		".prawn",
		".cairo",
		".makefile",
		".factor-rc",
		".orc",
		".logtalk",
		".webdoc",
		".bash_profile",
		".X-GIFTREG",
		".08",
		".to",
		".CorelProject",
		".SIM",
		".hxx",
		".classpath",
		".rchit",
		".X-PCONF",
		".rc",
		".ufo",
		".htmlfeed",
		".sdi",
		".es",
		".feature",
		".19",
		".lck",
		".3-rc1",
		".95.html",
		".HRC",
		".2009.pdf",
		".eclxml",
		".mxt",
		".pov",
		".seo",
		".frm",
		".woa",
		".sch",
		".ly",
		".asp.LCK",
		".ps1xml",
		".pxd",
		".array-rand",
		".matah",
		".wbp",
		".jav",
		".DOCX",
		".highland",
		".pryrc",
		".htm_",
		".com.ua",
		".rkt",
		".tar.gz",
		".ltxd",
		".scp",
		".browserslistrc",
		".roff",
		".png.php",
		".fread",
		".htmlu",
		".srvl",
		".h",
		".ori",
		".bat",
		".types",
		".MAXIMIZE",
		".psd",
		".him",
		".file",
		".fsi",
		".13",
		".se",
		".keyword",
		".livemd",
		".mq4",
		".swig",
		".ejs",
		".pdf.html",
		".plugins",
		".au3",
		".vhdl",
		".sea",
		".mg",
		".gdbinit",
		".hc",
		".maxhelp",
		".mp2",
		".eslintrc",
		".gif",
		".html.old",
		".hp",
		".BMP",
		".tesc",
		".ampl",
		".63.html",
		".slint",
		".sample",
		".bk",
		".fds",
		".rspec",
		".comments.",
		".cps",
		".eclass",
		".board.asd",
		".zshrc",
		".bb",
		".P.",
		".0.html",
		".pu",
		".webarchive",
		".rss_jobs",
		".trade",
		".fut",
		".sublime-workspace",
		".29",
		".scriv",
		".http",
		".fsh",
		".gutschein",
		".sha3",
		".tt2",
		".tftpl",
		".gif.php",
		".man",
		".zh.html",
		".rhtm",
		".NET:",
		".ston",
		".wast",
		".hawaii",
		".htm7",
		".lha",
		".phppar",
		".php.static",
		".cbx",
		".bibtex",
		".xpml",
		".DnnWebService",
		".jpeg",
		".aspxx",
		".ijs",
		".gnuplot",
		".6-all-languages",
		".pre",
		".xojo_code",
		".lic",
		".asl",
		".bf",
		".pad",
		".q",
		".03",
		".fancybox",
		".array-merge",
		".search.",
		".als",
		".ibf",
		".30",
		".html.LCK",
		".cljs.hl",
		".odif",
		".forum",
		".jake",
		".Config",
		".por",
		".cgis",
		".tga",
		".xml.old",
		".nfo",
		".simplecov",
		".nasm",
		".wgsl",
		".biz",
		".11.html",
		".php5",
		".euc",
		".7",
		".lua",
		".gvy",
		".3pm",
		".manager",
		".scrivx",
		".pddl",
		".wsf",
		".seam",
		".Skins",
		".ck",
		".tcsh",
		".zep",
		".webidl",
		".pde",
		".RAW",
		".print-frame",
		".henry",
		".asps",
		".viw",
		".DOC",
		".cxx",
		".swi",
		".serv",
		".TEST",
		".wp4",
		".tern-config",
		".lyt",
		".ccproj",
		".opa",
		".kdelnk",
		".14.html",
		".nse",
		".gbs",
		".asp.bak",
		".Templates",
		".cmake",
		".mwp",
		".6.html",
		".haml.deface",
		".swf",
		".X-AOM",
		".etf",
		".ixi",
		".xy3",
		".brs",
		".kk",
		".search",
		".array-map",
		".Zif",
		".bsv",
		".msd",
		".gitattributes",
		".20.html",
		".pyc",
		".pot",
		".diff",
		".elm",
		".ajax",
		".mustache",
		".lignee",
		".rss_cars",
		".11-pr1",
		".shtml",
		".pro",
		".jinja2",
		".inc.asp",
		".1.pdf",
		".zlogin",
		".getimagesize",
		".kes",
		".exs",
		".include",
		".tmp.php",
		".php_files",
		".css.gz",
		".pdfx",
		".ipl",
		".squery",
		".12.html",
		".template",
		".cljs",
		".scala",
		".84",
		".Main",
		".qxd",
		".ppt",
		".X-FCOMP",
		".load",
		".E",
		".tcc",
		".mvc",
		".upgrade",
		".cds",
		".clp",
		".mc",
		".rd",
		".98.html",
		".030-i486",
		".12.pdf",
		".skin",
		".Publish",
		".MOV",
		".bdr",
		".zone",
		".cfg",
		".heex",
		".pascal",
		".js.LCK",
		".sqlproj",
		".cl2",
		".sdd",
		".gbp",
		".p8",
		".iil",
		".mspec",
		".wml",
		".aidl",
		".sl",
		".confirm.email",
		".fil",
		".pwdp",
		".rktd",
		".mw",
		".com.crt",
		".nimrod",
		".pdf.php",
		".webalizer",
		".gpx",
		".kojo",
		".cjsx",
		".zsh",
		".bicep",
		".epj",
		".tlp",
		".8a",
		".b",
		".pasm",
		".gov",
		".env.template",
		".tech",
		".ops",
		".zzs",
		".fadein.template",
		".jshintrc",
		".hrc",
		".dws",
		".jsh",
		".gjam",
		".process",
		".cls.php",
		".dgs",
		".glf",
		".us",
		".stm",
		".itml",
		".sda",
		".vho",
		".24.html",
		".copf",
		".htm.html",
		".ihlp",
		".6.edu",
		".phpp",
		".0-pl1",
		".dbm",
		".golo",
		".mediawiki",
		".filereader",
		".plantuml",
		".Org.vssscc",
		".liquid",
		".gradle.kts",
		".scripts",
		".en.jsp",
		".gawk",
		".cats",
		".popup.pop_3D_viewer",
		".WAV",
		".VMS",
		".150.html",
		".yul",
		".html}",
		".94",
		".wp7",
		".asn",
		".antlers.xml",
		".mathematica",
		".description",
		".jsonld",
		".pug",
		".Jpeg",
		".eb",
		".xul",
		".dfy",
		".gjs",
		".session",
		".E.",
		".vssettings",
		".cnc",
		".printer",
		".fdr",
		".jsa",
		".Css",
		".ISO",
		".parse-url",
		".cfml",
		".gv",
		".env.staging",
		".cxx-objdump",
		".hmtl",
		".html.start",
		".sol",
		".200.html",
		".gray",
		".iso",
		".ascx.vb",
		".gdshader",
		".sass",
		".webc",
		".phdo",
		".chm",
		"._ls",
		".D.",
		".xrb",
		".c++objdump",
		".chat",
		".pps",
		".apacheconf",
		".veo",
		".imagejpeg",
		".nproj",
		".edn",
		".5",
		".ehtml",
		".guy",
		".smali",
		".now",
		".ssjs",
		".hs",
		".baut",
		".wgx",
		".extract",
		".chord",
		".html1",
		".xlsx",
		".issues",
		".isl",
		".jslib",
		".wiki",
		".chpl",
		".sec",
		".psc1",
		".mdb",
		".7z",
		".original",
		".sdb",
		".dell",
		".wg",
		".htmls",
		".btxt",
		".htmlpar",
		".sym",
		".in-array",
		".1m",
		".podsl",
		".vbproj",
		".en",
		".0-rc1",
		".litcoffee",
		".htm.d",
		".sso",
		".print.jsp",
		".sats",
		".vercelignore",
		".matlab",
		".00",
		".prettierignore",
		".config",
		".axi",
		".cfg.php",
		".xsjs",
		".hid",
		".text",
		".coffeelintignore",
		".1-pt_BR",
		".run",
		".gemrc",
		".gitignore",
		".1-english",
		".opensearch",
		".pyde",
		".remove",
		".vdf",
		".ditamap",
		".crt",
		".gif.count",
		".quid",
		".asp",
		".inputrc",
		".SideMenu",
		".GetMapImage",
		".ocr",
		".kt",
		".phpt",
		".dockerignore",
		".bbcolors",
		".int",
		".tab-",
		".jslintrc",
		".3qt",
		".mysql-select-db",
		".test.cgi",
		".smt",
		".ps",
		".1st",
		".lark",
		".bbx",
		".owl",
		".mu",
		".reb",
		".lbi",
		".peggy",
		".xrl",
		".flv",
		".fluid",
		".static",
		".snap",
		".sw3",
		".storefront",
		".hic",
		".moo",
		".mmk",
		".prefab",
		".ics",
		".py3",
		".svn",
		".sls",
		".pyx",
		".sra",
		".sf",
		".X-SURVEY",
		".rakumod",
		".ms",
		".common",
		".epc",
		".svc",
		".nxg",
		".rdoc_options",
		".eps",
		".ani",
		".casino",
		".old1",
		".fnc",
		".iol",
		".dfti",
		".be",
		".old2",
		".pdd",
		".Eus",
		".rnw",
		".html.none",
		".cws",
		".frag",
		".cp",
		".gradle",
		".qtgp",
		".qpqd",
		".mhtml",
		".propfinder",
		".t",
		".dtex",
		".view",
		".trck",
		".require",
		".secure",
		".cscfg",
		".MP3",
		".subscribe",
		".uplc",
		".adm",
		".rsp",
		".sav",
		".ascx.resx",
		".ipp",
		".gif_var_DE",
		".mpeg",
		".clangd",
		".enfinity",
		".ghtml",
		".rft",
		".fdx",
		".ASPX",
		".java",
		".c",
		".csd",
		".txi",
		".cljscm",
		".ep",
		".vpdoc",
		".outcontrol",
		".found",
		".nded-pga-emial",
		".resume",
		".os",
		".vs",
		".tl",
		".sh",
		".6",
		".urd",
		".Set",
		".X-FCOMP_var_DE",
		".ha",
		".plf",
		".hqf",
		".php.LCK",
		".m2",
		".prc",
		".lib.php",
		".html[",
		".gltf",
		".xpy",
		".log.0",
		".dwf",
		".thompson",
		".flaskenv",
		".ficheros",
		".letter",
		".jsproj",
		".xproj",
		".blog",
		".tlv",
		".cat",
		".ug",
		".40.html",
		".zil",
		".lnk42",
		".emlx",
		".numpyw",
		".INFO",
		".bst",
		".io",
		".jpg[",
		".gshader",
		".zip",
		".war",
		".nfm",
		".tab",
		".curlrc",
		".bash_logout",
		".idf",
		".qml",
		".wlt",
		".mod",
		".2.php",
		".dylan",
		".sxg",
		"._js",
		".vsh",
		".res",
		".older",
		".osm",
		".bz2",
		".sitemap",
		".lynkx",
		".latexmkrc",
		".rbmnu",
		".sendtoafriendform",
		".svg",
		".email",
		".thanks",
		".bro",
		".php,",
		".nas",
		".doh",
		".neon",
		".old",
		".factor",
		".verify",
		".axs",
		".txl",
		".mud",
		".p",
		".dal",
		".-",
		".pxi",
		".dir",
		".profile",
		".sublime-menu",
		".tpl",
		".syntax",
		".jso",
		".asp2",
		".hlsli",
		".cnf",
		".unlink",
		".1.html",
		".dproj",
		".50",
		".ini.default",
		".3-pl1",
		".sha2",
		".mt",
		".3.asp",
		".array-values",
		".vto",
		".htm~",
		".nycrc",
		".rmd",
		". T.",
		".mysql",
		".scd",
		".daniel",
		".xconf",
		".tsx",
		".20",
		".sgl",
		".y",
		".xsl",
		".tpc",
		".Includes",
		".L.jpg",
		".html.",
		".gpt",
		".detail",
		".joseph",
		".ficken.cx",
		".graphql",
		".midi",
		".nbp",
		".f4v",
		".print.shtml",
		".sitemap.xml",
		".aspx,",
		".pwi",
		".btr",
		".34",
		".hpp",
		".dvi",
		".rlib",
		".xgi",
		".pwdpl",
		".p\u200bhp",
		".vert",
		".plsql",
		".jscsrc",
		".strpos",
		".ccp",
		".v2.php",
		".nawk",
		".hip",
		".mp3",
		".ar",
		".wbmp",
		".ott",
		".frk",
		".vscodeignore",
		".myt",
		".rq",
		".trace",
		".Direct",
		".tiff",
		".JUSTFILE",
		".cfm.cfm",
		".23.html",
		".vh",
		".apsx",
		".-safety-fear",
		".sph",
		".colorbox-min.js",
		".pae",
		".bbs",
		".axs.erb",
		".easignore",
		".rg",
		".redirect",
		".jspa",
		".asp_files",
		".0",
		".43",
		".LOG",
		".TextGrid",
		".fopen",
		".oui",
		".m",
		".apl",
		".05.html",
		".readme",
		".mk.rabattlp",
		".awt",
		".plist",
		".haml",
		".launch",
		".Aspx",
		".xsh",
		".boc",
		".offline",
		".wmv",
		".njs",
		".xojo_window",
		".html7",
		".bash_aliases",
		".wsc",
		".yacc",
		".faucetdepot",
		".nth",
		".rexx",
		".xyw",
		".require-once",
		".licx",
		".fft",
		".regexp",
		".X-MAGNIFIER",
		".csc",
		".sfx",
		".hlean",
		".sps",
		".php2",
		".cppm",
		".TXT",
		".snakefile",
		".bkp",
		".AdCode",
		".ob2",
		".artnet.",
		".readfile",
		".apf",
		".html.orig",
		".tres",
		".s",
		".mkv",
		".vx",
		".swp",
		".lmi",
		".coffee",
		".rtd",
		".wpl",
		".plx",
		".06",
		".nb",
		".xbdoc",
		".yardopts",
		".command",
		".0.zip",
		".capnp",
		".ipf",
		".mss",
		".inc.php",
		".pg",
		".rzk",
		".file-put-contents",
		".dhall",
		".htmlq",
		".es.html",
		".xlt",
		".gyp",
		".pytb",
		".vm",
		".go",
		".A",
		".form",
		".vor",
		".calendar",
		".jhtm",
		".html,",
		".16",
		".mlir",
		".snippets",
		".nimble",
		".Gif",
		".caddyfile",
		".xy.php",
		".re",
		".jisonlex",
		".zcml",
		".php.bak",
		".htm",
		".7.html",
		".gms",
		".array-keys",
		".5.html",
		".p7s",
		".project",
		".webmanifest",
		".cnt",
		".wireless.action",
		".cfm.bak",
		".login",
		".stl",
		".DESC.",
		".hdl",
		".zmpl",
		".inv",
		".eh",
		".frx",
		".swd",
		".lsp",
		".com.old",
		".nomad",
		".qbl",
		".ipspot",
		".sp1",
		".vxlpub",
		".cjs",
		".wixproj",
		".master",
		".sdw",
		".gap",
		".scxml",
		".scr",
		".o",
		".ged",
		".just",
		".red",
		".join",
		".gthr",
		".phpx",
		".pac",
		".sma",
		".125.html",
		".etx",
		".rbbas",
		".inc.php3",
		".ini.bak",
		".hxsl",
		".smi",
		".license",
		".php_",
		".proj",
		".eslintignore",
		".xls",
		".cron",
		".mml",
		".images",
		".forms",
		".imp",
		".mcw",
		".resi",
		".pcss",
		".gpg",
		".suarez",
		".alhtm",
		".LassoApp",
		".ogg",
		".php3",
		".vhd",
		".BAK",
		".1x",
		".history",
		".strings",
		".links",
		".29.html",
		".buyadspace",
		".html.htm",
		".api",
		".jade",
		".toit",
		".i3",
		".vhost",
		".sublime_metrics",
		".jsonnet",
		".pornoizlee.tk",
		".corp",
		".tps",
		".wav",
		".epsi",
		".pyi",
		".sty",
		".csproj.user",
		".65",
		".fn",
		".ttf",
		".mwd",
		".dev",
		".chdir",
		".kicad_sym",
		".groovy",
		".dsl",
		".htm.bak",
		".net-print.htm",
		".zs",
		".zdat",
		".new.php",
		".pbt",
		".bhtml",
		".eco",
		".pho",
		".mel",
		".vy",
		".bsl",
		".ctl",
		".mzn",
		".html_",
		".uc",
		".09.html",
		".escript",
		".read",
		".hml",
		".members",
		".pgp",
		".nc",
		".xml.php",
		".mao",
		".wvx",
		".xtend",
		".viper",
		".bbappend",
		".mpd",
		".yy",
		".cs",
		".nlogo",
		".price",
		".ys",
		".emacs",
		".lnk",
		".sphp3",
		".pls",
		".g4",
		".watchmanconfig",
		".sha512",
		".htm.rc",
		".grace",
		".10",
		".ssf",
		".it.html",
		".31",
		".smil",
		".auk",
		".perl",
		".working",
		".tdf",
		".bck",
		".ht",
		".suo",
		".clar",
		".1.x",
		".dist",
		".62.html",
		".swg",
		".email.shtml",
		".napravlenie_DESC",
		".src",
		".ronn",
		".aspy",
		".jsb",
		".duby",
		".php.old",
		".webproj",
		".mspx",
		".ed",
		".ods",
		".0b",
		".xspec",
		".taf",
		".scandir",
		".zmodel",
		".lib",
		".wps.rtf",
		".bu",
		".xpdf",
		".vrx",
		".jsf",
		".35",
		".ebnf",
		".jnlp",
		".acgi",
		".php.txt",
		".Z",
		".bib",
		".external",
		".iss",
		".pike",
		".exp",
		".sparql",
		".das",
		".dart",
		".mjs",
		".cppobjdump",
		".ice",
		".fpp",
		".asf",
		".info.html",
		".pml",
		".apt",
		".tcl",
		".te",
		".dontcopy",
		".aj_",
		".ini.php",
		".framework",
		".sitx",
		".mms",
		".asm",
		".tsc",
		".vspscc",
		".glyphs",
		".bok",
		".sitemap.",
		".kml",
		".24",
		".weechatlog",
		".p6m",
		".14",
		".jscad",
		".gpn",
		".abap",
		".srch",
		".smf",
		".scw",
		"._docx",
		".Commerce",
		".vcf",
		".n",
		".ihtml",
		".aspx",
		".setup",
		".mumps",
		".scad",
		".tt",
		".vapi",
		".js.asp",
		".Rprofile",
		".tact",
		".dockerfile",
		".xpi",
		".elv",
		".sent-",
		".rabl",
		".MPG",
		".XLS",
		".archiv",
		".no",
		".par",
		".oxo",
		".dita",
		".skcard",
		".font",
		".bml",
		".fsproj",
		".rpgle",
		".md",
		".gto",
		".inactive",
		".shen",
		".sdoc",
		".errors",
		".cginc",
		".lnt",
		".Zip",
		".gdoc",
		".jsfl",
		".guiaweb.tk",
		".JPEG",
		".gi",
		".ebuild",
		".forth",
		".ks",
		".zhtml",
		".dpk",
		".servlet",
		".jsx",
		".purs",
		".mk",
		".sgf",
		".gpp",
		".twig",
		".textclipping",
		".www",
		".polar",
		".console",
		".all-contributorsrc",
		".containerfile",
		".nsf",
		".,",
		".wireless",
		".dlm",
		".dump",
		".od",
		".ra",
		".sublime-build",
		".exc",
		".pl",
		".sv",
		".jpg.html",
		".gnu",
		".csi",
		".emberscript",
		".dig",
		".X-OFFERS_var_DE",
		".randomhouse",
		".COM",
		".cdr",
		".vsprintf",
		".xsjslib",
		".categorias",
		".c8rc",
		".prw",
		".forget.pass",
		".tfstate",
		".rvmrc",
		".smk",
		".ccxml",
		".utf8",
		".har",
		".mv4",
		".sail",
		".htm5",
		".categories",
		".Doc",
		".banan.se",
		".lookml",
		".pd",
		".unauth",
		".xyp",
		".yyp",
		".monkey2",
		".tls",
		".nvmrc",
		".ect",
		".rsc",
		".15",
		".5.i",
		".note",
		".psd1",
		".me",
		".cn",
		".omgrofl",
		".gz",
		".hsc",
		".ascx.cs",
		".ur",
		".db",
		".wren",
		".imprimer-cadre",
		".1_stable",
		".18",
		".ftlh",
		".pvm",
		".i7x",
		".jhtml",
		".4th",
		".edit",
		".local.cfm",
		".30-i486",
		".metadata",
		".gif         ",
		".Master",
		".63",
		".mkdown",
		".cshrc",
		".ivy",
		".mbizgroup",
		".buckconfig",
		".ls",
		".mac",
		".yaml.sed",
		".sln",
		".htn",
		".js",
		".pan",
		".asp_",
		".wxs",
		".gsd",
		".hl",
		".bdsproj",
		".inc",
		"._doc",
		".X-FANCYCAT_var_DE",
		".sha256sum",
		".crx",
		".jsonl",
		".rockspec",
		".bash_history",
		".temp",
		".swf.swf",
		".reg",
		".a5w",
		".txt.gz",
		".layer",
		".flypage",
		".location.href",
		".-bouncing",
		".srw",
		".htmll",
		".es6",
		".apj",
		".scss",
		".outbound",
		".xsp.metadata",
		".toml.example",
		".66",
		".obj",
		".pir",
		".MacOS",
		".sld",
		".php.htm",
		".2.js",
		".gform",
		".emulecollection",
		".rjs",
		".X-RMA_var_DE",
		".luf",
		".frt",
		".gaml",
		".thumb.jpg",
		".plot",
		".tmac",
		".tcl.in",
		".pegjs",
		".wl",
		".01",
		".2a",
		".90",
		".texty",
		".Xml",
		".13.html",
		".w",
		".flac",
		".children",
		".Admin",
		".jbf",
		".riot",
		".dwg",
		".004.jpg",
		".ligo",
		".xhtml",
		".req",
		".lnx",
		".catalog",
		".csdef",
		".jsp.old",
		".mm",
		".action",
		".prhtm",
		".sha256",
		".vnt",
		".cs2",
		".lasso",
		".cc",
		".sidebar",
		".fancypack",
		".jis",
		".md4",
		".film",
		".axd",
		".moon",
		".puml",
		".cyp",
		".monkey",
		".master.cs",
		".ent",
		".r",
		".Jpg",
		".prefs",
		".thtml",
		".dotm",
		".adml",
		".mli",
		".prl",
		".33",
		".epub",
		".csv",
		".nasl",
		".cmd",
		".yap",
		".tese",
		".plb",
		".z",
		".gtable",
		".vmb",
		".odt",
		".phtm",
		".fxml",
		".actions",
		".registration",
		".set",
		".prep",
		".met",
		".ce",
		".fodt",
		".95",
		".3gp",
		".di",
		".htm.old",
		".faq",
		".xproc",
		".IDL",
		".2b",
		".gf",
		".HTM",
		".asd",
		".X-AOM_var_DE",
		".pht",
		".fpl",
		".fr.jsp",
		".ofl",
		".DLL",
		".old.2",
		".ASP",
		".get",
		".sp",
		".metal",
		".nut",
		".lp",
		".htmla",
		".lbx",
		".tracker.ashx",
		".hrl",
		".eliom",
		".vbproj.vspscc",
		".ini.sample",
		".wri",
		".1in",
		".xlf",
		".homepage",
		".bmp",
		".ptnx",
		".kicad_wks",
		".pfa",
		".jelly",
		".xpl",
		".rss.php",
		".bylocation",
		".pdf.pdf",
		".cshtml",
		".kv",
		".ebay",
		".skins",
		".reds",
		".asp1",
		".pdpcmd",
		".sob",
		".url",
		".qc",
		".92",
		".cw",
		".texi",
		".mint",
		".applescript",
		".bzrignore",
		".env.ci",
		".75.html",
		".conllu",
		".docz",
		".htm.htm",
		".krl",
		".ma",
		".xsd",
		".nuspec",
		".readme_var_DE",
		".don",
		".parse.errors",
		".custom",
		".count",
		".implode",
		".add.php",
		".thor",
		".psb",
		".mligo",
		".co",
		".step",
		".cts",
		".html5",
		".wma",
		".mkvi",
		".ruby",
		".cljx",
		".ch",
		". php",
		".docxml",
		".sublime-keymap",
		".bxt",
		".plc",
		".mysql-query",
		".85",
		".26.html",
		".NT2",
		".yar",
		".charset",
		".avsc",
		".png",
		".vtt",
		".dcl",
		".odp",
		".assets",
		".crc32",
		".theme",
		".cil",
		".en.htm",
		".stylelintignore",
		".napravlenie_ASC",
		".html4",
		".kwd",
		".j2",
		".last",
		".cylc",
		".scalafmt.conf",
		".aux",
		".bpl",
		".mp",
		".PS",
		".fp",
		".properties",
		".hbs",
		".shim",
		".8",
		".mkd",
		".psp",
		".resultados",
		".env.dev",
		".php.backup",
		".dotx",
		".min.js",
		".mak",
		".bicepparam",
		".php.html",
		".scenic",
		".ijm",
		".alt",
		".coverfinder",
		".htm.LCK",
		".lwp",
		".idx",
		".prev",
		".sql",
		".wax",
		".DES",
		".spc",
		".ld",
		".iframe_filtros",
		".kmz",
		".sml",
		".angelscript",
		".disabled",
		".cu",
		".txx",
		".rb",
		".gdns",
		".pmc",
		".imprimir",
		".friend",
		".gclient",
		".dct",
		".mxml",
		".em",
		".wtt",
		".mysql.txt",
		".ticket.submit",
		".calca",
		".ksh",
		".data",
		".xliff",
		".err",
		".html.txt",
		".csv.php",
		".podspec",
		".cs.pp",
		".stw",
		".advsearch",
		".lis",
		".po",
		".itcl",
		".cql",
		".sublime-theme",
		".diz",
		".map",
		".wp5",
		".ssi",
		".ashx",
		".web",
		".ml4",
		".zml",
		".vbhtml",
		".sdm",
		".ocx",
		".trigger",
		".pluginspec",
		".msg",
		".pfb",
		".xmp",
		".uot",
		".mkdn",
		".wtx",
		".flt",
		".pdf",
		".m4v",
		".page",
		".xojo_script",
		".gitkeep",
		".m4",
		".u3i",
		".emacs.desktop",
		".metadata.js",
		".php.original",
		".spacemacs",
		".texinfo",
		".vorteil",
		".copy",
		".master.vb",
		".dot",
		".ttl",
		".shop",
		".upc",
		".minid",
		".srt",
		".xqy",
		".flux",
		".hy",
		".abbrev_defs",
		".r3",
		".styl",
		".abnf",
		".imgbotconfig",
		".pt",
		".rdoc",
		".oxygene",
		".mps",
		".xbm",
		".site",
		".wikitext",
		".prev_next",
		".act",
		".03.html",
		".[file",
		".com_Backup_",
		".cart",
		".pazderski.us",
		".nsi",
		".nginx",
		".ram",
		".ql",
		".kdl",
		".exe,",
		".roc",
		".ws",
		".nit",
		".wm",
		".preg-match",
		".vxml",
		".hdb",
		".ui",
		".aspp",
		".rxml",
		".groupproj",
		".w3x",
		".session-start",
		".21",
		".in",
		".wxl",
		".m4a",
		".m3",
		".ditaval",
		".ring",
		".builder",
		".s.html",
		".lue",
		".zprofile",
		".bashrc",
		".orig",
		".fmt",
		".pony",
		".28.html",
		".oz",
		".opml",
		".Web",
		".kicad_mod",
		".sxw",
		".rbx",
		".cbl",
		".KB",
		".4-all-languages",
		".pb",
		".creole",
		".jl",
		".3m",
		".asy",
		".vue",
		".build",
		".thy",
		".ptr",
		".smt2",
		".zig",
		".sema",
		".zrtf",
		".htmlprint",
		".ad.php",
		".git",
		".davis",
		".browser",
		".sexp",
		".nodemonignore",
		".nr",
		".00.html",
		".ss",
		".2",
		".tmux",
		".sig",
		".ort",
		".Php",
		".55.html",
		".textsearch",
		".rsx",
		".bfhtm",
		".vcl",
		".ro",
		".fr.html",
		".45.html",
		".wit",
		".OutJob",
		".nsh",
		".job",
		".18.html",
		".maninfo",
		".boom",
		".21.html",
		".rzn",
		".kshrc",
		".self",
		".axml",
		".cms",
		".ihmtl",
		".ltx",
		".yxx",
		".htmlhintrc",
		".esp",
		".htlm",
		".ecr",
		".sublime-commands",
		".7.js",
		".com,",
		".OLD",
		".pbtxt",
		".1c",
		".2.tmp",
		".gnus",
		".csproj.webinfo",
		".jspre",
		".csp",
		".num",
		".ant",
		".sh.in",
		".at.html",
		".sfproj",
		".move",
		".phps",
		".nikon",
		".sci",
		".doc",
		".gtl",
		".xzap",
		".htpasswd",
		".env.local",
		".hb",
		".ris",
		".jtd",
		".cuh",
		".fp7",
		".07.html",
		".dxf",
		".mno",
		".pig",
		".Org.sln",
		".vsixmanifest",
		".mdx",
		".kql",
		".dll",
		".jrtf",
		".artdeco",
		".yml.mysql",
		".backup",
		".sis",
		".js2",
		".rl",
		".terminal",
		".wp",
		".pmod",
		".z3",
		".religo",
		".resx",
		".sms",
		".jarvis",
		".php.php",
		".old.htm",
		".jpg]",
		".12",
		".glslv",
		".25",
		".UNX",
		".uof",
		".xojo_menu",
		".rabattlp",
		".section",
		".scalafix.conf",
		".geom",
		".vba",
		".il",
		".resource",
		".tgz",
		".fdxt",
		".pem",
		".99",
		".bz",
		".tag",
		".2.swf",
		".grxml",
		".gitmodules",
		".brd",
		".ebp",
		".xspf",
		".afm",
		".linq",
		".ceylon",
		".trelby",
		".spec",
		".p7b",
		".html_var_DE",
		".csx",
		".01-10",
		".cssd",
		".jflex",
		".nwp",
		".HTML",
		".15.html",
		".footer",
		".smithy",
		".A.",
		".cookie.js",
		".plt",
		".gtpl",
		".code-workspace",
		".ispc",
		".MK",
		".JSON-tmLanguage",
		".xml.gz",
		".kutxa.net-en",
		".gn",
		".nims",
		".tm",
		".per",
		".csh",
		".com.htm",
		".viewpage__10",
		".ahkl",
		".bmx",
		".lpr",
		".yaml",
		".new",
		".psgi",
		".art",
		".coffee.md",
		".nf",
		".sht",
		".workflow",
		".index",
		".core",
		".robot",
		".mako",
		".open",
		".doc.doc",
		".graphqls",
		".gbo",
		".md5",
		".ste",
		".opencl",
		".gsite",
		".tmd",
		".fx",
		".access.login",
		".srs",
		".auto-changelog",
		".kts",
		".gni",
		".noon",
		".htc",
		".bash",
		".205.html",
		".dyn",
		".support",
		".swf.LCK",
		".gleam",
		".ktm",
		".jtp",
		".jar",
		".dhtml",
		".env.development",
		".hold",
		".vwr",
		".dll.config",
		".award",
		".aug",
		".3.html",
		".Asp",
		".settings",
		".gbr",
		".nix",
		".pd_lua",
		".MPEG",
		".tml",
		".webinfo",
		".tsp",
		".ascx",
		".pptx",
		".env.test",
		".rbfrm",
		".mc_id",
		".gml",
		".vstemplate",
		".lng",
		".mid",
		".news",
		".avdl",
		".sublime_session",
		".asc",
		".href",
		".g",
		".restrictor.log",
		".ap",
		".rpt",
		".pm6",
		".08-2009",
		".google",
		".lisp",
		".jsonc",
		".html]",
		".sla.gz",
		".xojo_report",
		".pks",
		".ash",
		".lds",
		".nqp",
		".ico",
		".log",
		".c.html",
		".prolog",
		".ZIP",
		".cake",
		".mobi",
		".jpf",
		".log2",
		".edgeql",
		".tpl.php",
		".31.html",
		".xq",
		".4DForm",
		".new.htm",
		".v",
		".volt",
		".itk",
		".vcxproj",
		".geo.xml",
		".7-pl1",
		".h.in",
		".mysql-result",
		".gcode",
		".lslp",
		".phphp",
		".inc.html",
		".bowerrc",
		".26",
		".sfw",
		".nss",
		".exrc",
		".Static",
		".utxt",
		".carbon",
		".unx",
		".ALT",
		".PrjPCB",
		".cnm",
		".knt",
		".ux",
		".INI",
		".recherche",
		".popup",
		".TTF",
		".0.pdf",
		".xsp",
		".scc",
		".2.html",
		".sc",
		".fs",
		".wpw",
		".vw",
		".cmake.in",
		".17",
		".objdump",
		".clixml",
		".9.html",
		".sam",
		".bsp",
		".rec.html",
		".cypher",
		".msi",
		".sublime-macro",
		".X-AFFILIATE",
		".css.php",
		".asn1",
		".xml",
		".tfvars",
		".zw",
		".3x",
		".sisx",
		".ascii",
		".edu",
		".zig.zon",
		".NET",
		".local",
		".2-rc1",
		".shproj",
		".f",
		".64",
		".php.inc",
		".kid",
		".sublime-snippet",
		".home",
		".gypi",
		".gst",
		".mtml",
		".glade",
		".jad",
		".ex",
		".js.erb",
		".error",
		".buscar",
		".dmg",
		".90.html",
		".dbml",
		".rest.txt",
		".mk.gutschein",
		".wn",
		".xhtml5",
		".xc",
		".CPP",
		".mkii",
		".json",
		".tpb",
		".at",
		".5.php",
		".pmo",
		".nodos",
		".ak",
		".asp.old",
		".do",
		".torrent",
		".list",
		".esdl",
		".rbxs",
		".odin",
		".srf",
		".awp",
		".nu",
		".malesextoys.us",
		".babelrc",
		".wps",
		".rbres",
		".rktl",
		".0--DUP.htm",
		".dmb",
		".god",
		".pact",
		"._._order",
		".vcs",
		".demo",
		".ooc",
		".none",
		".ixx",
		".R",
		".rnh",
		".sas",
		".eit",
		".jsp",
		".cpt",
		".as\u200bp",
		".cx",
		".aquery",
		".wmf",
		".beta",
		".BTM",
		".rsh",
		".blade",
		".story",
		".tool",
		".gvimrc",
		".com.php",
		".asset",
		".opal",
		".jnp",
		".application",
		".dpj",
		".pld",
		".desktop",
		".toml",
		".prettierrc",
		".T.A",
		".appodeal",
		".pck",
		".captcha",
		".vhs",
		".sw",
		".LOG.new",
		".csl",
		".xdc",
		".dpr",
		".htm2",
		".split",
		".janet",
		".maxpat",
		".d",
		".7_0_A",
		".fshader",
		".cer",
		".vim",
		".ni",
		".marko",
		".pgsql",
		".wsd",
		".soe",
		".decls",
		".fun",
		".45",
		".aspg",
		".sgt",
		".sco",
		".lzh",
		".tu",
		".awk",
		".asax.vb",
		".tim",
		".gko",
		".cfx",
		".cls",
		".shader",
		".p3p",
		".kokuken",
		".4gl",
		".cproject",
		".dms",
		".filesize",
		".tac",
		".iml",
		".01-L.jpg",
		".pimx",
		".dsp",
		".ads",
		".sieve",
		".thm",
		".Services",
		".m3u8",
		".hu",
		".system",
		".html,404",
		".L.",
		".lasso9",
		".aspx_files",
		".eye",
		".user",
		".html_files",
		".pylintrc",
		".gmi",
		".psc",
		".staging",
		".cvsignore",
		".cljc",
		".erl",
		".se.php",
		".nl.html",
		".jspx",
		".cache",
		".desktop.in",
		".mgi",
		".ical",
		".csshandler.ashx",
		".trg",
		".rebol",
		".snippet",
		".preview",
		".js,",
		".udf",
		".upd",
		".hcl",
		".Justfile",
		".tern-project",
		".LCK",
		".C.",
		".tsv",
		".bdsgroup",
		".html.bak",
		".garcia",
		".old3",
		".crwl",
		".asmx",
		".sqlite",
		".results",
		".rss_homes",
		".ase",
		".d2",
		".mcfunction",
		".rego",
		".8xp.txt",
		".viminfo",
		".19.html",
		".x",
		".video",
		".w3m",
		".mcr",
		".depproj",
		".html.eex",
		".gdnlib",
		".data_",
		".babelignore",
		".contrib",
		".txt.php",
		".htx",
		".08.html",
		".ftn",
		".paul",
		".devcontainer.json",
		".Acquisition",
		".leex",
		".mbox",
		".d2w",
		".dats",
		".rbs",
		".php",
		".update",
		".mata",
		".jnl",
		".lhs",
		".php.",
		".vtl",
		".02",
		".csproj",
		".ldml",
		".simplexml-load-file",
		".grt",
		".ddl",
		".xquery",
		".lp2",
		".gdshaderinc",
		".3in",
		".uno",
		".storyboard",
		".del",
		".sublime-completions",
		".php_old",
		".en.html",
		".sds",
		".a",
		".vhi",
		".djs",
		".hx",
		".textproto",
		".asa",
		".rpm",
		".INC",
		".dlg",
		".ny",
		".inf",
		".MLD",
		".pop_3D_viewer",
		".zzq",
		".njx",
		".al",
		".atom",
		".hlsl",
		".ncl",
		".txt.txt",
		".eng",
		".asax.resx",
		".Hxx",
		".pkgproj",
		".wdl",
		".rex",
		".mawk",
		".sjs",
		".gtmpl",
		".zip.php",
		".php\u200e",
		".pmk",
		".agda",
		".awm",
		".bna",
		".fdml",
		".X-FANCYCAT",
		".Cfm",
		".btd",
		".click",
		".aty",
		".16.html",
		".mmap",
		".cod",
		".hql",
		".inc.js",
		".net-en",
		".praat",
		".whiley",
		".sbl",
		".print",
		".include-once",
		".includes",
		".dxb",
		".bean",
		".Email",
		".mag",
		".spin",
		".gp",
		".nz",
		".sqlrpgle",
		".mermaid",
		".rhtml",
		".100.html",
		".curry",
		".jst",
		".p6",
		".xaml",
		".swcrc",
		".sfd",
		".TFM",
		".ih",
		".el",
		".eq",
		".mq5",
		".5-pl1",
		".bi",
		".PDF",
		".star",
		".cat.php",
		".bats",
		".touch.action",
		".cpp-objdump",
		".Pdf",
		".fxh",
		".9C",
		".ngloss",
		".ino",
		".bak.php",
		".com_files",
		".jsd",
		".eam.fs",
		".glsl",
		".irclog",
		".dat",
		".svx",
		".vhw",
		".builds",
		".dtd",
		".1a",
		".mpl",
		".npmrc",
		".lidr",
		".slang",
		".tpl.html",
		".6pl",
		".vht",
		".home.test",
		".pprx",
		".pgsql.txt",
		".act.php",
		".r.",
		".mir",
		".ihya",
		".CXX",
		".irbrc",
		".PRJ",
		".imprimir-marco",
		".scpt",
		".xslx",
		".gpb",
		".apk",
		".mpg",
		".openbsd",
		".txt.html",
		".W32",
		".bloonset",
		".xpm",
		".img",
		".xml.dist",
		".mysql-pconnect",
		".partfinder",
		".mell",
		".p6l",
		".divx",
	}

	// Generate base hash once
	baseHash := m.generateHashFromContent(content, 5)

	// Generate options for each extension
	for _, ext := range extensions {
		filename := m.generateUniqueFilename(currentDir, baseHash, ext, content)
		options = append(options, filename)
	}

	return options
}

// generateUniqueFilename creates a unique filename by handling collisions
func (m *Manager) generateUniqueFilename(currentDir, baseHash, ext, content string) string {
	filename := fmt.Sprintf("ch_%s%s", baseHash, ext)
	fullPath := filepath.Join(currentDir, filename)

	// If file doesn't exist, return the original
	if _, err := os.Stat(fullPath); os.IsNotExist(err) {
		return filename
	}

	// Check if any files exist with the same base pattern (ch_<baseHash>)
	pattern := fmt.Sprintf("ch_%s", baseHash)
	if m.hasFilesWithPattern(currentDir, pattern) {
		// Try different substrings of content-based hash
		for offset := 1; offset <= 10; offset++ {
			newHash := m.generateHashFromContentWithOffset(content, 5, offset)
			filename = fmt.Sprintf("ch_%s%s", newHash, ext)
			fullPath = filepath.Join(currentDir, filename)

			if _, err := os.Stat(fullPath); os.IsNotExist(err) {
				return filename
			}
		}

		// If still colliding, add a numeric suffix
		for counter := 1; counter <= 999; counter++ {
			filename = fmt.Sprintf("ch_%s_%d%s", baseHash, counter, ext)
			fullPath = filepath.Join(currentDir, filename)

			if _, err := os.Stat(fullPath); os.IsNotExist(err) {
				return filename
			}
		}
	}

	return filename
}

// hasFilesWithPattern checks if any files exist with the given pattern
func (m *Manager) hasFilesWithPattern(currentDir, pattern string) bool {
	files, err := os.ReadDir(currentDir)
	if err != nil {
		return false
	}

	for _, file := range files {
		if !file.IsDir() && strings.HasPrefix(file.Name(), pattern) {
			return true
		}
	}
	return false
}

// generateHashFromContent creates a random hash using characters from the content
func (m *Manager) generateHashFromContent(content string, length int) string {
	return m.generateHashFromContentWithOffset(content, length, 0)
}

// generateHashFromContentWithOffset creates a hash with an offset for collision avoidance
func (m *Manager) generateHashFromContentWithOffset(content string, length, offset int) string {
	// Extract alphanumeric characters from content
	var charset []rune
	for _, char := range content {
		if (char >= 'a' && char <= 'z') || (char >= 'A' && char <= 'Z') || (char >= '0' && char <= '9') {
			charset = append(charset, char)
		}
	}

	// Fallback to default charset if content has no alphanumeric characters
	if len(charset) == 0 {
		charset = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789")
	}

	// Use content + offset as seed for more variation
	seed := int64(len(content) + offset)
	for i, char := range content {
		if i < 100 { // Only use first 100 chars to avoid overflow
			seed += int64(char) * int64(i+offset+1)
		}
	}
	rand.Seed(seed)

	// Generate hash
	hash := make([]rune, length)
	for i := range hash {
		hash[i] = charset[rand.Intn(len(charset))]
	}
	return string(hash)
}

// getAllFilesInCurrentDir returns all files in the current directory and subdirectories
func (m *Manager) getAllFilesInCurrentDir() ([]string, error) {
	currentDir, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("failed to get current directory: %v", err)
	}

	// Get absolute path for shallow check
	absCurrentDir, err := filepath.Abs(currentDir)
	if err != nil {
		return nil, fmt.Errorf("failed to get absolute path: %v", err)
	}

	// Check if this directory should be loaded shallowly
	isShallow := m.isShallowLoadDir(absCurrentDir)

	var files []string

	err = filepath.WalkDir(currentDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil // Skip files we can't access
		}

		// Skip the root directory itself
		if path == currentDir {
			return nil
		}

		// Get relative path from current directory
		relPath, err := filepath.Rel(currentDir, path)
		if err != nil {
			return nil // Skip if we can't get relative path
		}

		// For shallow directories, only include direct children (depth 1)
		if isShallow {
			depth := strings.Count(relPath, string(filepath.Separator))
			if d.IsDir() && depth > 0 {
				// Skip subdirectories beyond depth 1
				return filepath.SkipDir
			}
			if !d.IsDir() && depth > 0 {
				// Skip files beyond depth 1
				return nil
			}
		}

		// Skip version control directories but allow other hidden files/directories for export
		if d.IsDir() && (filepath.Base(relPath) == ".git" || filepath.Base(relPath) == ".svn" || filepath.Base(relPath) == ".hg") {
			return filepath.SkipDir
		}

		// Only include files, not directories
		if !d.IsDir() {
			files = append(files, relPath)
		}

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to walk directory: %v", err)
	}

	return files, nil
}

// isShallowLoadDir checks if a directory should be loaded shallowly (only 1 level deep)
func (m *Manager) isShallowLoadDir(dirPath string) bool {
	// Normalize the directory path
	absPath, err := filepath.Abs(dirPath)
	if err != nil {
		return false
	}
	absPath = filepath.Clean(absPath)

	// Check against each shallow load directory
	for _, shallowDir := range m.state.Config.ShallowLoadDirs {
		if shallowDir == "" {
			continue
		}

		// Expand ~ to home directory
		if strings.HasPrefix(shallowDir, "~") {
			homeDir, err := os.UserHomeDir()
			if err != nil {
				continue
			}
			shallowDir = filepath.Join(homeDir, shallowDir[1:])
		}

		// Normalize shallow directory path
		absShallowDir, err := filepath.Abs(shallowDir)
		if err != nil {
			continue
		}
		absShallowDir = filepath.Clean(absShallowDir)

		// Check for exact match
		if absPath == absShallowDir {
			return true
		}
	}

	return false
}

// AddRecentlyCreatedFile adds a file to the recently created files list
// Keeps the list limited to the last 10 files for performance
func (m *Manager) AddRecentlyCreatedFile(filePath string) {
	// Convert to relative path if in current directory
	if currentDir, err := os.Getwd(); err == nil {
		if rel, err := filepath.Rel(currentDir, filePath); err == nil && !strings.HasPrefix(rel, "..") {
			filePath = rel
		}
	}

	// Remove duplicates and add to front
	var updatedFiles []string
	updatedFiles = append(updatedFiles, filePath)

	for _, existing := range m.state.RecentlyCreatedFiles {
		if existing != filePath && len(updatedFiles) < 10 {
			updatedFiles = append(updatedFiles, existing)
		}
	}

	m.state.RecentlyCreatedFiles = updatedFiles
}

// extractLoadedFilesFromHistory extracts all files that have been loaded from the chat history
// Returns them in reverse chronological order (most recently loaded first)
func (m *Manager) extractLoadedFilesFromHistory() []string {
	var loadedFiles []string
	seen := make(map[string]bool)

	// Go through chat history in reverse order to prioritize more recent files
	for i := len(m.state.ChatHistory) - 1; i >= 0; i-- {
		entry := m.state.ChatHistory[i]

		if entry.User != "" {
			// Check for loaded content patterns
			if strings.Contains(entry.User, "File: ") || strings.Contains(entry.User, "Loaded: ") {
				lines := strings.Split(entry.User, "\n")
				for _, line := range lines {
					if strings.HasPrefix(line, "File: ") {
						filePath := strings.TrimPrefix(line, "File: ")
						filePath = strings.TrimSpace(filePath)
						if filePath != "" && !seen[filePath] {
							// Check if file still exists in current directory
							if m.fileExistsInCurrentDir(filePath) {
								loadedFiles = append(loadedFiles, filePath)
								seen[filePath] = true
							}
						}
					} else if strings.HasPrefix(line, "Loaded: ") {
						loadedContent := strings.TrimPrefix(line, "Loaded: ")
						// Split by comma and process each file
						files := strings.Split(loadedContent, ", ")
						for _, file := range files {
							file = strings.TrimSpace(file)
							if file != "" && !seen[file] {
								// Check if file still exists in current directory
								if m.fileExistsInCurrentDir(file) {
									loadedFiles = append(loadedFiles, file)
									seen[file] = true
								}
							}
						}
					}
				}
			}
		}
	}

	return loadedFiles
}

// fileExistsInCurrentDir checks if a file exists in the current directory structure
func (m *Manager) fileExistsInCurrentDir(filePath string) bool {
	currentDir, err := os.Getwd()
	if err != nil {
		return false
	}

	// Try both as relative path and full path from current directory
	fullPath := filepath.Join(currentDir, filePath)
	if _, err := os.Stat(fullPath); err == nil {
		return true
	}

	// Also try as absolute path if it starts with current directory
	if filepath.IsAbs(filePath) && strings.HasPrefix(filePath, currentDir) {
		if _, err := os.Stat(filePath); err == nil {
			return true
		}
	}

	return false
}

// getLoadedContentForHistoryEntry attempts to retrieve the actual loaded file content
// for a history entry that contains "Loaded: ..." by matching it with the corresponding message
func (m *Manager) getLoadedContentForHistoryEntry(historyEntry types.ChatHistory) string {
	// Find the corresponding message in the chat messages that contains the actual content
	// The loaded content should be in a message that was added around the same time

	// Look for user messages that contain "File: " patterns (actual loaded content)
	for _, message := range m.state.Messages {
		if message.Role == "user" && (strings.Contains(message.Content, "File: ") || strings.Contains(message.Content, "\nFile: ")) {
			// Check if this message content contains files mentioned in the history entry
			if m.messageContainsLoadedFiles(message.Content, historyEntry.User) {
				return message.Content
			}
		}
	}

	return "" // Return empty if we can't find the actual content
}

// messageContainsLoadedFiles checks if a message content contains files mentioned in a "Loaded: ..." history entry
func (m *Manager) messageContainsLoadedFiles(messageContent, historyEntry string) bool {
	if !strings.HasPrefix(historyEntry, "Loaded: ") {
		return false
	}

	// Extract file list from "Loaded: file1, file2, ..."
	loadedFilesList := strings.TrimPrefix(historyEntry, "Loaded: ")
	files := strings.Split(loadedFilesList, ", ")

	// Check if the message content contains references to these files
	matchCount := 0
	for _, file := range files {
		file = strings.TrimSpace(file)
		if file != "" {
			// Look for "File: <filename>" pattern in the message content
			filePattern := "File: " + file
			if strings.Contains(messageContent, filePattern) {
				matchCount++
			}
		}
	}

	// Consider it a match if we find references to at least half of the loaded files
	// (to handle cases where some files might have been deleted or renamed)
	return len(files) > 0 && matchCount >= (len(files)+1)/2
}

// cleanupLoadedContent removes excessive newlines from loaded file content for cleaner exports
func (m *Manager) cleanupLoadedContent(content string) string {
	// Remove excessive trailing newlines that loadTextFile adds
	content = strings.TrimRight(content, "\n")

	// Replace multiple consecutive newlines (more than 2) with just 2 newlines
	// This preserves intentional spacing while removing excessive gaps
	lines := strings.Split(content, "\n")
	var cleanedLines []string
	emptyLineCount := 0

	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			emptyLineCount++
			// Allow max 2 consecutive empty lines
			if emptyLineCount <= 2 {
				cleanedLines = append(cleanedLines, line)
			}
		} else {
			emptyLineCount = 0
			cleanedLines = append(cleanedLines, line)
		}
	}

	return strings.Join(cleanedLines, "\n")
}
