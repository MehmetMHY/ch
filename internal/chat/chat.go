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

// runEditorWithFallback tries to run helix first, then falls back to vim/nano on failure
func (m *Manager) runEditorWithFallback(filePath string) error {
	editors := []string{m.state.Config.PreferredEditor, "vim", "nano"}

	for i, editor := range editors {
		// Check if the editor exists
		if _, err := exec.LookPath(editor); err != nil {
			continue
		}

		cmd := exec.Command(editor, filePath)
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout

		// For the first attempts, suppress stderr to avoid showing error messages
		// Only show stderr for the final attempt
		if i < len(editors)-1 {
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

	selected, err := terminal.FzfSelect(items, "Backtrack to: ")
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

		prompt := fmt.Sprintf("File %d/%d: ", i+1, len(matches))
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
	selectedItems, err := terminal.FzfMultiSelect(items, "Export entries (TAB=multi): ")
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

	// Get all files in current directory (including subdirectories) and add option to create new file
	allFiles, err := m.getAllFilesInCurrentDir()
	if err != nil {
		return "", fmt.Errorf("failed to get current directory files: %v", err)
	}

	// Create the file selection options - add [NEW FILE] at the top like the codedump [NONE] option
	var fileOptions []string
	fileOptions = append(fileOptions, "[NEW FILE]")
	if len(allFiles) > 0 {
		fileOptions = append(fileOptions, allFiles...)
	}

	selectedOption, err := terminal.FzfSelect(fileOptions, "Save to file: ")
	if err != nil {
		return "", fmt.Errorf("file selection failed: %v", err)
	}

	if selectedOption == "" {
		return "", fmt.Errorf("export cancelled")
	}

	var filename string
	if selectedOption == "[NEW FILE]" {
		// Generate filename options and let user select
		filenameOptions := m.generateFilenameOptions(editedContent)
		selectedFilename, err := terminal.FzfSelect(filenameOptions, "Export file: ")
		if err != nil {
			return "", fmt.Errorf("filename selection failed: %v", err)
		}
		if selectedFilename == "" {
			return "", fmt.Errorf("export cancelled")
		}
		filename = selectedFilename
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

	return fullPath, nil
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

// generateFilenameOptions creates many filename options with various extensions
func (m *Manager) generateFilenameOptions(content string) []string {
	var options []string
	currentDir, _ := os.Getwd()
	seenNames := make(map[string]bool)

	// First priority: add cha_<uuid>.txt as the very first suggestion
	chatID := uuid.New().String()
	firstOption := fmt.Sprintf("cha_%s.txt", chatID)
	fullPath := filepath.Join(currentDir, firstOption)
	if _, err := os.Stat(fullPath); os.IsNotExist(err) {
		options = append(options, firstOption)
		seenNames[firstOption] = true
	}

	// ALL possible text file extensions
	extensions := []string{
		// Programming languages
		".py", ".js", ".ts", ".jsx", ".tsx", ".go", ".rs", ".java", ".c", ".cpp", ".cc", ".cxx",
		".h", ".hpp", ".cs", ".php", ".rb", ".swift", ".kt", ".scala", ".pl", ".pm", ".r", ".m",
		".mm", ".lua", ".sh", ".bash", ".zsh", ".fish", ".ps1", ".bat", ".cmd", ".vb", ".fs",
		".clj", ".cljs", ".hs", ".elm", ".ex", ".exs", ".erl", ".hrl", ".dart", ".asm", ".s",
		".f", ".f90", ".f95", ".pas", ".pp", ".ada", ".adb", ".ads", ".d", ".nim", ".cr",
		".jl", ".ml", ".mli", ".ocaml", ".rkt", ".scm", ".lisp", ".cl", ".lsp", ".tcl", ".tk",

		// Web and markup
		".html", ".htm", ".xhtml", ".css", ".scss", ".sass", ".less", ".stylus", ".xml", ".xsl",
		".xslt", ".svg", ".jsp", ".asp", ".aspx", ".php3", ".php4", ".php5", ".phtml",

		// Data and config
		".json", ".yaml", ".yml", ".toml", ".ini", ".cfg", ".conf", ".config", ".properties",
		".env", ".editorconfig", ".gitignore", ".gitattributes", ".htaccess", ".robots",

		// Documentation
		".md", ".markdown", ".rst", ".txt", ".rtf", ".tex", ".latex", ".org", ".adoc", ".asciidoc",
		".wiki", ".textile", ".creole", ".pod", ".man", ".1", ".2", ".3", ".4", ".5", ".6", ".7", ".8",

		// Scripts and automation
		".dockerfile", ".makefile", ".cmake", ".mk", ".gradle", ".sbt", ".ant", ".maven", ".pom",
		".rake", ".gulpfile", ".gruntfile", ".webpack", ".rollup", ".vite", ".parcel",

		// Database and query
		".sql", ".mysql", ".pgsql", ".sqlite", ".mongodb", ".cql", ".cypher", ".sparql",

		// Logs and data
		".log", ".logs", ".csv", ".tsv", ".tab", ".dat", ".data", ".out", ".output", ".result",
		".report", ".summary", ".stats", ".metrics", ".trace", ".dump", ".backup", ".bak",

		// System and service
		".service", ".socket", ".timer", ".mount", ".automount", ".swap", ".target", ".path",
		".slice", ".scope", ".desktop", ".directory", ".theme", ".spec", ".repo", ".list",

		// Editor and IDE
		".vim", ".vimrc", ".nvim", ".emacs", ".el", ".elc", ".atom", ".sublime", ".vscode",
		".editorconfig", ".eslintrc", ".prettierrc", ".babelrc", ".tsconfig", ".jsconfig",

		// Build and packaging
		".lock", ".sum", ".mod", ".deps", ".requirements", ".pipfile", ".poetry", ".cargo",
		".npm", ".yarn", ".package", ".manifest", ".assembly", ".project", ".solution",

		// Templates and views
		".template", ".tmpl", ".tpl", ".mustache", ".handlebars", ".hbs", ".ejs", ".erb",
		".haml", ".slim", ".pug", ".jade", ".twig", ".blade", ".vue", ".svelte", ".angular",

		// Misc text formats
		".patch", ".diff", ".gitpatch", ".eml", ".msg", ".mbox", ".ics", ".vcf", ".ldif",
		".pem", ".crt", ".key", ".csr", ".p12", ".pfx", ".jks", ".keystore", ".truststore",
	}

	// Extract words from content
	words := m.extractWords(content)

	// Generate random extensions for edge cases (1-5 characters) - 2500 unique ones (10% of total)
	// Evenly distribute lengths: 500 each of 1,2,3,4,5 character extensions
	randomExtCount := 2500
	randomExtensions := make([]string, randomExtCount)

	extIndex := 0
	for length := 1; length <= 5; length++ {
		for count := 0; count < 500 && extIndex < randomExtCount; count++ {
			extChars := make([]byte, length)
			for j := range extChars {
				extChars[j] = byte('a' + rand.Intn(26)) // random lowercase letter
			}
			randomExt := "." + string(extChars)
			randomExtensions[extIndex] = randomExt
			extIndex++
		}
	}

	// Calculate distribution
	randomExtTargetCount := randomExtCount              // 2500 files with random extensions (10%)
	knownExtTargetCount := 20000 - randomExtTargetCount // 17500 files with known extensions (90%)

	// Combine known extensions with random ones for fallback
	allExtensions := append(extensions, randomExtensions...)

	// If we have enough words, generate word-based options
	if len(words) >= 5 {
		// Generate 90% from known extensions
		perKnownExt := knownExtTargetCount / len(extensions)
		for _, ext := range extensions {
			for i := 0; i < perKnownExt && len(options) < knownExtTargetCount; i++ {
				numWords := 3 + rand.Intn(3) // 3, 4, or 5 words
				selectedWords := m.pickRandomWords(words, numWords)
				filename := strings.Join(selectedWords, "_") + ext

				// Check if unique and doesn't exist
				if !seenNames[filename] {
					fullPath := filepath.Join(currentDir, filename)
					if _, err := os.Stat(fullPath); os.IsNotExist(err) {
						options = append(options, filename)
						seenNames[filename] = true
					}
				}
			}
		}

		// Generate 10% from random extensions (1 per extension)
		for _, ext := range randomExtensions {
			if len(options) >= 20000 {
				break
			}
			numWords := 3 + rand.Intn(3) // 3, 4, or 5 words
			selectedWords := m.pickRandomWords(words, numWords)
			filename := strings.Join(selectedWords, "_") + ext

			// Check if unique and doesn't exist
			if !seenNames[filename] {
				fullPath := filepath.Join(currentDir, filename)
				if _, err := os.Stat(fullPath); os.IsNotExist(err) {
					options = append(options, filename)
					seenNames[filename] = true
				}
			}
		}

		// Fill any remaining slots with ch_<uuid> options
		for len(options) < 20000 {
			ext := allExtensions[rand.Intn(len(allExtensions))]
			filename := fmt.Sprintf("ch_%s%s", uuid.New().String()[:8], ext)
			fullPath := filepath.Join(currentDir, filename)
			if _, err := os.Stat(fullPath); os.IsNotExist(err) {
				options = append(options, filename)
			}
		}
	} else {
		// Not enough words, distribute ch_<uuid> format across all extensions (90% known, 10% random)
		// First fill with known extensions
		perKnownExt := knownExtTargetCount / len(extensions)
		for _, ext := range extensions {
			for i := 0; i < perKnownExt && len(options) < knownExtTargetCount; i++ {
				filename := fmt.Sprintf("ch_%s%s", uuid.New().String()[:8], ext)
				fullPath := filepath.Join(currentDir, filename)
				if _, err := os.Stat(fullPath); os.IsNotExist(err) {
					options = append(options, filename)
				}
			}
		}

		// Then fill remaining with random extensions
		for len(options) < 20000 && len(options)-knownExtTargetCount < randomExtCount {
			ext := randomExtensions[len(options)-knownExtTargetCount]
			filename := fmt.Sprintf("ch_%s%s", uuid.New().String()[:8], ext)
			fullPath := filepath.Join(currentDir, filename)
			if _, err := os.Stat(fullPath); os.IsNotExist(err) {
				options = append(options, filename)
			}
		}
	}

	return options
}

// extractWords extracts meaningful words from content for filename generation
func (m *Manager) extractWords(content string) []string {
	// Remove code blocks, special characters, and split into words
	cleaned := regexp.MustCompile("```[\\s\\S]*?```").ReplaceAllString(content, "")
	cleaned = regexp.MustCompile("[^a-zA-Z0-9\\s]").ReplaceAllString(cleaned, " ")

	words := strings.Fields(strings.ToLower(cleaned))
	var validWords []string

	// Filter words: length 3-12, not common words
	commonWords := map[string]bool{
		"the": true, "and": true, "for": true, "are": true, "but": true,
		"not": true, "you": true, "all": true, "can": true, "had": true,
		"her": true, "was": true, "one": true, "our": true, "out": true,
		"day": true, "get": true, "has": true, "him": true, "his": true,
		"how": true, "man": true, "new": true, "now": true, "old": true,
		"see": true, "two": true, "way": true, "who": true, "boy": true,
		"did": true, "its": true, "let": true, "put": true, "say": true,
		"she": true, "too": true, "use": true, "with": true, "this": true,
	}

	for _, word := range words {
		if len(word) >= 3 && len(word) <= 12 && !commonWords[word] {
			validWords = append(validWords, word)
		}
	}

	// Remove duplicates
	seen := make(map[string]bool)
	var unique []string
	for _, word := range validWords {
		if !seen[word] {
			seen[word] = true
			unique = append(unique, word)
		}
	}

	return unique
}

// pickRandomWords selects n random words from the slice without duplicates
func (m *Manager) pickRandomWords(words []string, n int) []string {
	if len(words) == 0 {
		return []string{}
	}

	// Create a map to track used words and avoid duplicates
	used := make(map[string]bool)
	result := make([]string, 0, n)

	// Try to pick unique words first
	maxAttempts := n * 10 // Prevent infinite loops
	attempts := 0

	for len(result) < n && attempts < maxAttempts {
		word := words[rand.Intn(len(words))]
		if !used[word] {
			used[word] = true
			result = append(result, word)
		}
		attempts++
	}

	// If we couldn't find enough unique words, fill the remaining slots
	// by cycling through available words (still avoiding immediate repeats)
	for len(result) < n {
		for _, word := range words {
			if len(result) >= n {
				break
			}
			// Only add if it's not the same as the last word added
			if len(result) == 0 || result[len(result)-1] != word {
				result = append(result, word)
			}
		}
		// If we still can't fill it, break to avoid infinite loop
		if len(result) == 0 {
			break
		}
	}

	return result
}

// ListChatHistory provides an interactive view of the chat history with fzf
func (m *Manager) ListChatHistory(terminal *ui.Terminal) error {
	if len(m.state.ChatHistory) <= 1 {
		return fmt.Errorf("no chat history to display")
	}

	// Prepare chat history entries for fzf selection
	var items []string
	var entryContents []string

	// Add [ALL] option at the top
	items = append(items, "[ALL]")
	entryContents = append(entryContents, "") // Placeholder for ALL option

	for i, entry := range m.state.ChatHistory {
		if i == 0 {
			continue // Skip system prompt
		}

		// Handle loaded files (when User starts with specific patterns)
		if entry.User != "" {
			// Check if this is loaded content by looking for file patterns or "Loaded: " prefix
			if strings.Contains(entry.User, "File: ") || strings.Contains(entry.User, "Loaded: ") {
				lines := strings.Split(entry.User, "\n")
				for _, line := range lines {
					if strings.HasPrefix(line, "File: ") {
						filePath := strings.TrimPrefix(line, "File: ")
						preview := filePath
						if len(preview) > 80 {
							preview = preview[:80] + "..."
						}
						items = append(items, fmt.Sprintf("[FILE] %d: %s", i, preview))
						entryContents = append(entryContents, strings.TrimPrefix(line, "File: "))
					} else if strings.HasPrefix(line, "Loaded: ") {
						loadedContent := strings.TrimPrefix(line, "Loaded: ")
						// Split by comma and create separate entries for each file
						files := strings.Split(loadedContent, ", ")
						for _, file := range files {
							file = strings.TrimSpace(file)
							if file != "" {
								preview := file
								if len(preview) > 80 {
									preview = preview[:80] + "..."
								}
								items = append(items, fmt.Sprintf("[FILE] %d: %s", i, preview))
								entryContents = append(entryContents, file)
							}
						}
					}
				}
			} else {
				// Regular user message
				userPreview := strings.Split(entry.User, "\n")[0]
				if len(userPreview) > 80 {
					userPreview = userPreview[:80] + "..."
				}
				items = append(items, fmt.Sprintf("[USER] %d: %s", i, userPreview))
				entryContents = append(entryContents, entry.User)
			}
		}

		if entry.Bot != "" {
			// Bot response
			botPreview := strings.Split(entry.Bot, "\n")[0]
			if len(botPreview) > 80 {
				botPreview = botPreview[:80] + "..."
			}
			items = append(items, fmt.Sprintf("[BOT] %d: %s", i, botPreview))
			entryContents = append(entryContents, entry.Bot)
		}
	}

	if len(items) <= 1 { // Only [ALL] option
		return fmt.Errorf("no chat history entries to display")
	}

	// Use exact matching fzf for selection
	selectedItems, err := terminal.FzfMultiSelectExact(items, "Entries: ")
	if err != nil {
		return fmt.Errorf("selection cancelled or failed: %v", err)
	}

	if len(selectedItems) == 0 {
		return fmt.Errorf("no entries selected")
	}

	// Check if [ALL] was selected
	showAll := false
	for _, item := range selectedItems {
		if strings.HasPrefix(item, "[ALL]") {
			showAll = true
			break
		}
	}

	if showAll {
		// Show all entries
		var lastPlatform, lastModel string
		for i, entry := range m.state.ChatHistory {
			if i == 0 {
				continue // Skip system prompt
			}

			// Only print platform/model info if it changed
			if entry.Platform != lastPlatform || entry.Model != lastModel {
				fmt.Printf("\033[91mUSING: %s/%s\033[0m\n", entry.Platform, entry.Model)
				lastPlatform = entry.Platform
				lastModel = entry.Model
			}

			if entry.User != "" {
				// Check if this is loaded content
				if strings.Contains(entry.User, "File: ") || strings.Contains(entry.User, "Loaded: ") {
					lines := strings.Split(entry.User, "\n")
					for _, line := range lines {
						if strings.HasPrefix(line, "File: ") {
							fmt.Printf("\033[93mFILE:\033[0m %s\n", strings.TrimPrefix(line, "File: "))
						} else if strings.HasPrefix(line, "Loaded: ") {
							loadedContent := strings.TrimPrefix(line, "Loaded: ")
							// Split by comma and display each file on separate line
							files := strings.Split(loadedContent, ", ")
							for _, file := range files {
								file = strings.TrimSpace(file)
								if file != "" {
									fmt.Printf("\033[93mFILE:\033[0m %s\n", file)
								}
							}
						} else if strings.TrimSpace(line) != "" {
							fmt.Printf("%s\n", line)
						}
					}
				} else {
					fmt.Printf("\033[94mUSER:\033[0m %s\n", entry.User)
				}
			}

			if entry.Bot != "" {
				// Check if this is a platform/model switch message
				if (strings.Contains(entry.Bot, "Switched from") && strings.Contains(entry.Bot, "to")) || strings.Contains(entry.Bot, "Switched model from") {
					// For model switches, show the old model info
					if strings.Contains(entry.Bot, "Switched model from") {
						// Extract old model from message like "Switched model from gpt-4o-mini to gpt-4.1-mini"
						parts := strings.Split(entry.Bot, " from ")
						if len(parts) > 1 {
							modelPart := strings.Split(parts[1], " to ")
							if len(modelPart) > 0 {
								oldModel := strings.TrimSpace(modelPart[0])
								fmt.Printf("\033[91mCHANGED: %s/%s\033[0m\n", entry.Platform, oldModel)
							}
						}
					}
					// Skip printing the bot message for switches
				} else {
					fmt.Printf("\033[92mBOT:\033[0m %s\n", entry.Bot)
				}
			}
		}
	} else {
		// Show selected entries
		for _, selectedItem := range selectedItems {
			// Find the corresponding content and print it without the label
			for i, item := range items {
				if item == selectedItem && i > 0 { // Skip [ALL] option
					content := entryContents[i]
					if content != "" {
						fmt.Printf("\033[93m%s\033[0m\n", content)
					}
					break
				}
			}
		}
	}

	return nil
}

// getAllFilesInCurrentDir returns all files in the current directory and subdirectories
func (m *Manager) getAllFilesInCurrentDir() ([]string, error) {
	currentDir, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("failed to get current directory: %v", err)
	}

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

		// Skip hidden files and directories (starting with .)
		if strings.HasPrefix(filepath.Base(relPath), ".") {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
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
