package ui

import (
	"bufio"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io/fs"
	"io/ioutil"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/MehmetMHY/ch/pkg/types"
	"github.com/ledongthuc/pdf"
	"github.com/nguyenthenguyen/docx"
	"github.com/otiai10/gosseract/v2"
	"github.com/rwcarlsen/goexif/exif"
	"github.com/tealeg/xlsx/v3"
)

// Terminal handles terminal-related operations
type Terminal struct {
	config *types.Config
}

// NewTerminal creates a new terminal handler
func NewTerminal(config *types.Config) *Terminal {
	return &Terminal{
		config: config,
	}
}

// getTempDir returns the application's temporary directory, creating it if it doesn't exist
func (t *Terminal) getTempDir() (string, error) {
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

// runFzfSSHSafe executes fzf in a way that works correctly over SSH connections
// by keeping stderr attached to TTY for UI display and redirecting stdout to temp file
func (t *Terminal) runFzfSSHSafe(fzfArgs []string, inputText string) (string, error) {
	tempDir, err := t.getTempDir()
	if err != nil {
		return "", fmt.Errorf("failed to get temp directory: %w", err)
	}

	outputFile := filepath.Join(tempDir, fmt.Sprintf("fzf-output-%d", time.Now().UnixNano()))

	// Build command that redirects only stdout to temp file, keeping stderr on TTY
	// Escape each fzf argument for shell safety
	var escapedArgs []string
	for _, arg := range fzfArgs {
		// Simple shell escaping - wrap each arg in single quotes and escape any single quotes
		escapedArg := "'" + strings.ReplaceAll(arg, "'", "'\"'\"'") + "'"
		escapedArgs = append(escapedArgs, escapedArg)
	}

	// Escape the output file path for shell safety
	escapedOutput := "'" + strings.ReplaceAll(outputFile, "'", "'\"'\"'") + "'"
	cmdStr := fmt.Sprintf("fzf %s > %s", strings.Join(escapedArgs, " "), escapedOutput)

	cmd := exec.Command("sh", "-c", cmdStr)
	cmd.Stdin = strings.NewReader(inputText)
	// stdout goes to file (via shell redirection)
	// stderr stays attached to TTY for UI display
	cmd.Stderr = os.Stderr

	err = cmd.Run()

	// Clean up temp file
	defer os.Remove(outputFile)

	// Handle cancellation (exit code 130 or 1)
	if exitErr, ok := err.(*exec.ExitError); ok {
		if exitErr.ExitCode() == 130 || exitErr.ExitCode() == 1 {
			return "", nil // User cancelled
		}
		return "", fmt.Errorf("fzf failed: %w", err)
	} else if err != nil {
		return "", fmt.Errorf("fzf execution failed: %w", err)
	}

	// Read the selection from temp file
	content, err := ioutil.ReadFile(outputFile)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil // No selection made
		}
		return "", fmt.Errorf("failed to read fzf output: %w", err)
	}

	return strings.TrimSpace(string(content)), nil
}

// runFzfSSHSafeWithQuery executes fzf with --print-query in a SSH-safe way
func (t *Terminal) runFzfSSHSafeWithQuery(fzfArgs []string, inputText string) ([]string, error) {
	tempDir, err := t.getTempDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get temp directory: %w", err)
	}

	outputFile := filepath.Join(tempDir, fmt.Sprintf("fzf-query-output-%d", time.Now().UnixNano()))

	// Build command that redirects only stdout to temp file, keeping stderr on TTY
	// Escape each fzf argument for shell safety
	var escapedArgs []string
	for _, arg := range fzfArgs {
		// Simple shell escaping - wrap each arg in single quotes and escape any single quotes
		escapedArg := "'" + strings.ReplaceAll(arg, "'", "'\"'\"'") + "'"
		escapedArgs = append(escapedArgs, escapedArg)
	}

	// Escape the output file path for shell safety
	escapedOutput := "'" + strings.ReplaceAll(outputFile, "'", "'\"'\"'") + "'"
	cmdStr := fmt.Sprintf("fzf %s > %s", strings.Join(escapedArgs, " "), escapedOutput)

	cmd := exec.Command("sh", "-c", cmdStr)
	cmd.Stdin = strings.NewReader(inputText)
	cmd.Stderr = os.Stderr

	err = cmd.Run()

	// Clean up temp file
	defer os.Remove(outputFile)

	// Handle cancellation (exit code 130 or 1)
	if exitErr, ok := err.(*exec.ExitError); ok {
		if exitErr.ExitCode() == 130 || exitErr.ExitCode() == 1 {
			return []string{}, nil // User cancelled
		}
		return nil, fmt.Errorf("fzf failed: %w", err)
	} else if err != nil {
		return nil, fmt.Errorf("fzf execution failed: %w", err)
	}

	// Read the output from temp file
	content, err := ioutil.ReadFile(outputFile)
	if err != nil {
		if os.IsNotExist(err) {
			return []string{}, nil // No output
		}
		return nil, fmt.Errorf("failed to read fzf output: %w", err)
	}

	if len(content) == 0 {
		return []string{}, nil
	}

	lines := strings.Split(strings.TrimRight(string(content), "\n"), "\n")
	return lines, nil
}

// IsTerminal checks if the input is from a terminal
func (t *Terminal) IsTerminal() bool {
	fileInfo, _ := os.Stdin.Stat()
	return (fileInfo.Mode() & os.ModeCharDevice) != 0
}

// ShowHelp displays the help information
func (t *Terminal) ShowHelp() {
	fmt.Println("Ch - A lightweight CLI tool for AI model interaction")
	fmt.Println("")
	fmt.Println("Usage:")
	fmt.Printf("  ch [-h] [-d [DIRECTORY]] [-p [PLATFORM]] [-m MODEL] [-l FILE/URL] [-e] [query]\n")
	fmt.Println("")
	fmt.Println("Options:")
	fmt.Printf("  %-18s %s\n", "-h, --help", "show this help message and exit")
	fmt.Printf("  %-18s %s\n", "-d [DIRECTORY]", "generate codedump file (optionally specify directory path)")
	fmt.Printf("  %-18s %s\n", "-p [PLATFORM]", "switch platform (leave empty for interactive selection)")
	fmt.Printf("  %-18s %s\n", "-m MODEL", "specify model to use")
	fmt.Printf("  %-18s %s\n", "-l FILE/URL", "load and display file content or scrape URL")
	fmt.Printf("  %-18s %s\n", "-e, --export", "export code blocks from the last response")
	fmt.Println("")
	fmt.Println("Examples:")
	fmt.Println("  ch \"What is artificial intelligence?\"")
	fmt.Println("  ch -p groq -m llama3-8b-8192 \"Explain quantum computing\"")
	fmt.Println("  ch -l document.pdf")
	fmt.Println("  ch -d /path/to/project")
	fmt.Println("  cat main.py | ch \"What does this code do?\"")
	fmt.Println("")

	// Dynamically generate platforms list
	var platforms []string
	platforms = append(platforms, "openai (default)")
	for name := range t.config.Platforms {
		if name != "openai" {
			platforms = append(platforms, name)
		}
	}
	fmt.Println("Available Platforms:")
	fmt.Printf("  %s\n", strings.Join(platforms, ", "))
	fmt.Println("")
	fmt.Println("Interactive Mode:")
	fmt.Println("  Run 'ch' without arguments to start. Use '!h' for a full list of commands.")
}

// RecordShellSession records the entire shell session and returns the content as a string.
func (t *Terminal) RecordShellSession() (string, error) {
	shell := os.Getenv("SHELL")
	if shell == "" {
		shell = "/bin/sh" // Fallback shell
	}

	// Get the application's temporary directory
	tempDir, err := t.getTempDir()
	if err != nil {
		return "", fmt.Errorf("failed to get temp directory: %w", err)
	}

	// Create a temporary file to store the session recording
	tempFile, err := ioutil.TempFile(tempDir, "ch_shell_session_*.log")
	if err != nil {
		return "", fmt.Errorf("failed to create temporary file: %w", err)
	}
	defer os.Remove(tempFile.Name()) // Clean up the temp file

	t.PrintInfo(fmt.Sprintf("starting shell session in %s (CTRL-D to exit)", shell))

	// Use the 'script' command to record the session with cross-platform compatibility
	// The issue is that different Unix systems expect different syntax:
	// - Linux (util-linux): script [options] [file]
	// - macOS/BSD: script [options] [file] [command]
	//
	// We'll use the simpler approach that works across platforms
	var cmd *exec.Cmd

	// Use script without specifying a command - this works on all platforms
	// Set the SHELL environment variable to ensure script uses the correct shell
	cmd = exec.Command("script", "-q", tempFile.Name())
	cmd.Env = append(os.Environ(), "SHELL="+shell)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		// If -q flag doesn't work, try without it (some old versions don't support -q)
		if _, ok := err.(*exec.ExitError); ok {
			cmd = exec.Command("script", tempFile.Name())
			cmd.Env = append(os.Environ(), "SHELL="+shell)
			cmd.Stdin = os.Stdin
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr

			if err := cmd.Run(); err != nil {
				// An exit error is expected when the shell exits, so we don't treat it as a fatal error
				if _, ok := err.(*exec.ExitError); !ok {
					return "", fmt.Errorf("failed to run shell session: %w", err)
				}
			}
		} else {
			return "", fmt.Errorf("failed to run shell session: %w", err)
		}
	}

	t.PrintInfo("shell session ended")

	// Read the recorded content from the temporary file
	content, err := ioutil.ReadFile(tempFile.Name())
	if err != nil {
		return "", fmt.Errorf("failed to read session recording: %w", err)
	}

	return string(content), nil
}

// ShowHelpFzf displays the help information using fzf for interactive selection.
// Returns the selected command if it should be executed, empty string otherwise.
func (t *Terminal) ShowHelpFzf() string {
	options := t.getInteractiveHelpOptions()

	fzfArgs := []string{
		"--reverse", "--height=40%", "--border",
		"--prompt=Option: ", "--multi",
	}
	inputText := strings.Join(options, "\n")

	output, err := t.runFzfSSHSafe(fzfArgs, inputText)
	if err != nil {
		t.PrintError(fmt.Sprintf("%v", err))
		return ""
	}

	if output == "" {
		return ""
	}

	selectedItems := strings.Split(output, "\n")

	// If [ALL] is selected along with other items, ignore [ALL] and process others
	if len(selectedItems) > 1 {
		for _, item := range selectedItems {
			if !strings.HasPrefix(item, "[ALL]") {
				return t.processHelpSelection(item, options)
			}
		}
	}

	// If only [ALL] is selected, show all options
	if len(selectedItems) == 1 && strings.HasPrefix(selectedItems[0], "[ALL]") {
		for _, option := range options {
			if !strings.HasPrefix(option, "[ALL]") {
				fmt.Printf("\033[93m%s\033[0m\n", option)
			}
		}
		return ""
	}

	// Process single selection
	return t.processHelpSelection(selectedItems[0], options)
}

func (t *Terminal) processHelpSelection(selected string, options []string) string {
	if strings.HasPrefix(selected, "[STATE]") {
		return "[STATE]"
	}
	// If line contains [ or ], just print it in yellow and return
	if strings.Contains(selected, "[") || strings.Contains(selected, "]") {
		fmt.Printf("\033[93m%s\033[0m\n", selected)
		return ""
	}

	// Extract the command (everything before the first space or dash)
	parts := strings.Fields(selected)
	if len(parts) > 0 {
		command := parts[0]
		// Handle commands that start with ! or single character commands like 'l'
		if strings.HasPrefix(command, "!") || len(command) == 1 {
			return command
		}
	}

	return ""
}

// getInteractiveHelpOptions returns a slice of strings containing the help information.
func (t *Terminal) getInteractiveHelpOptions() []string {
	options := []string{
		"[ALL] - show all help options",
		"[STATE] - show current state",
		fmt.Sprintf("%s - exit interface", t.config.ExitKey),
		fmt.Sprintf("%s - switch models", t.config.ModelSwitch),
		fmt.Sprintf("%s - text editor input mode", t.config.EditorInput),
		fmt.Sprintf("%s - clear chat history", t.config.ClearHistory),
		fmt.Sprintf("%s - backtrack to a previous message", t.config.Backtrack),
		fmt.Sprintf("%s - help page", t.config.HelpKey),
		fmt.Sprintf("%s - switch platforms (interactive)", t.config.PlatformSwitch),
		fmt.Sprintf("%s [dir] - load files/dirs from current or specified directory", t.config.LoadFiles),
		fmt.Sprintf("%s - generate codedump (all text files with fzf exclusion)", t.config.CodeDump),
		fmt.Sprintf("%s - export selected chat entries to a file", t.config.ExportChat),
		fmt.Sprintf("%s - record a shell session and use it as context", t.config.ShellRecord),
		fmt.Sprintf("%s <url1> [url2] ... - scrape content from URLs", t.config.ScrapeURL),
		fmt.Sprintf("%s <query> - search web using ddgr", t.config.WebSearch),
		fmt.Sprintf("%s - copy selected responses to clipboard", t.config.CopyToClipboard),
		fmt.Sprintf("%s - multi-line input mode (end with '\\' on a new line)", t.config.MultiLine),
		"Ctrl+C - clear current prompt input",
		"Ctrl+D - exit interface",
	}

	return options
}

// PrintTitle displays the current session information
func (t *Terminal) PrintTitle() {
	fmt.Printf("\033[93m%s - exit interface\033[0m\n", t.config.ExitKey)
	fmt.Printf("\033[93m%s - switch models\033[0m\n", t.config.ModelSwitch)
	fmt.Printf("\033[93m%s - switch platforms\033[0m\n", t.config.PlatformSwitch)
	fmt.Printf("\033[93m%s - text editor input\033[0m\n", t.config.EditorInput)
	fmt.Printf("\033[93m%s - clear history\033[0m\n", t.config.ClearHistory)
	fmt.Printf("\033[93m%s - backtrack\033[0m\n", t.config.Backtrack)
	fmt.Printf("\033[93m%s - help page\033[0m\n", t.config.HelpKey)
	fmt.Printf("\033[93m%s - load files/dirs\033[0m\n", t.config.LoadFiles)
	fmt.Printf("\033[93m%s - generate codedump\033[0m\n", t.config.CodeDump)
	fmt.Printf("\033[93m%s - export chat\033[0m\n", t.config.ExportChat)
	fmt.Printf("\033[93m%s - record shell session\033[0m\n", t.config.ShellRecord)
	fmt.Printf("\033[93m%s - multi-line input\033[0m\n", t.config.MultiLine)
	fmt.Printf("\033[93mCtrl+C - clear prompt input\033[0m\n")
	fmt.Printf("\033[93mCtrl+D - exit interface\033[0m\n")
}

// ShowLoadingAnimation displays a loading animation
func (t *Terminal) ShowLoadingAnimation(message string, done chan bool) {
	chars := []string{"⣾", "⣽", "⣻", "⢿", "⡿", "⣟", "⣯", "⣷", "⠁", "⠂", "⠄", "⡀", "⢀", "⠠", "⠐", "⠈"}
	i := 0
	for {
		select {
		case <-done:
			fmt.Print("\r\033[K")
			return
		default:
			fmt.Printf("\r\033[93m%s %s\033[0m", chars[i], message)
			i = (i + 1) % len(chars)
			time.Sleep(100 * time.Millisecond)
		}
	}
}

// FzfSelect provides a fuzzy finder interface for selection
func (t *Terminal) FzfSelect(items []string, prompt string) (string, error) {
	fzfArgs := []string{"--reverse", "--height=40%", "--border", "--prompt=" + prompt}
	inputText := strings.Join(items, "\n")

	return t.runFzfSSHSafe(fzfArgs, inputText)
}

// FzfMultiSelect provides a fuzzy finder interface for multiple selections
func (t *Terminal) FzfMultiSelect(items []string, prompt string) ([]string, error) {
	fzfArgs := []string{"--reverse", "--height=40%", "--border", "--prompt=" + prompt, "--multi", "--bind=tab:toggle+down"}
	inputText := strings.Join(items, "\n")

	result, err := t.runFzfSSHSafe(fzfArgs, inputText)
	if err != nil {
		return nil, err
	}

	if result == "" {
		return []string{}, nil
	}

	return strings.Split(result, "\n"), nil
}

// FzfMultiSelectExact provides an exact matching fuzzy finder interface for multiple selections
func (t *Terminal) FzfMultiSelectExact(items []string, prompt string) ([]string, error) {
	fzfArgs := []string{"--reverse", "--height=40%", "--border", "--prompt=" + prompt, "--multi", "--bind=tab:toggle+down", "--exact"}
	inputText := strings.Join(items, "\n")

	result, err := t.runFzfSSHSafe(fzfArgs, inputText)
	if err != nil {
		return nil, err
	}

	if result == "" {
		return []string{}, nil
	}

	return strings.Split(result, "\n"), nil
}

// FzfMultiSelectForCLI provides a fuzzy finder interface for multiple selections with cancellation detection
func (t *Terminal) FzfMultiSelectForCLI(items []string, prompt string) ([]string, error) {
	fzfArgs := []string{"--reverse", "--height=40%", "--border", "--prompt=" + prompt, "--multi", "--bind=tab:toggle+down"}
	inputText := strings.Join(items, "\n")

	result, err := t.runFzfSSHSafe(fzfArgs, inputText)
	if err != nil {
		return nil, err
	}

	// For CLI, treat empty result as cancellation
	if result == "" {
		return nil, fmt.Errorf("user cancelled")
	}

	return strings.Split(result, "\n"), nil
}

// FzfSelectOrQuery provides a fuzzy finder interface that allows for selection or custom query input.
func (t *Terminal) FzfSelectOrQuery(items []string, prompt string) (string, error) {
	fzfArgs := []string{"--reverse", "--height=40%", "--border", "--prompt=" + prompt, "--print-query"}
	inputText := strings.Join(items, "\n")

	lines, err := t.runFzfSSHSafeWithQuery(fzfArgs, inputText)
	if err != nil {
		return "", err
	}

	if len(lines) == 0 {
		return "", nil
	}

	// If fzf returns a selection, it's on the second line.
	if len(lines) > 1 && lines[1] != "" {
		return lines[1], nil
	}

	// If there's no selection, the query is on the first line.
	// This handles the case where the user types a URL and hits enter.
	if lines[0] != "" {
		return lines[0], nil
	}

	return "", nil
}

// PrintSuccess prints a success message
func (t *Terminal) PrintSuccess(message string) {
	fmt.Printf("\033[92m%s\033[0m\n", message)
}

// PrintError prints an error message
func (t *Terminal) PrintError(message string) {
	fmt.Printf("\033[91m%s\033[0m\n", message)
}

// PrintInfo prints an informational message
func (t *Terminal) PrintInfo(message string) {
	fmt.Printf("\033[93m%s\033[0m\n", message)
}

// PrintModelSwitch prints model switch confirmation
func (t *Terminal) PrintModelSwitch(model string) {
	fmt.Printf("\033[96m%s\033[0m \033[95m%s\033[0m\n", t.config.CurrentPlatform, model)
}

// PrintPlatformSwitch prints platform switch confirmation
func (t *Terminal) PrintPlatformSwitch(platform, model string) {
	fmt.Printf("\033[96m%s\033[0m \033[95m%s\033[0m\n", platform, model)
}

// LoadFileContent loads and returns content from selected files/directories or URLs
func (t *Terminal) LoadFileContent(selections []string) (string, error) {
	var contentBuilder strings.Builder

	for _, selection := range selections {
		if selection == "" {
			continue
		}

		// Check if this is a URL
		if t.isURL(selection) {
			urlContent, err := t.scrapeURL(selection)
			if err != nil {
				contentBuilder.WriteString(fmt.Sprintf("Error scraping %s: %v\n", selection, err))
				continue
			}
			contentBuilder.WriteString(urlContent)
			continue
		}

		// Handle files and directories
		info, err := os.Stat(selection)
		if err != nil {
			continue
		}

		if info.IsDir() {
			dirContent, err := t.loadDirectoryContent(selection)
			if err != nil {
				continue
			}
			contentBuilder.WriteString(dirContent)
		} else {
			fileContent, err := t.loadTextFile(selection)
			if err != nil {
				continue
			}
			contentBuilder.WriteString(fileContent)
		}
	}

	return contentBuilder.String(), nil
}

// loadTextFile loads content from various file types (text, PDF, DOCX, XLSX, CSV, images)
func (t *Terminal) loadTextFile(filePath string) (string, error) {
	ext := strings.ToLower(filepath.Ext(filePath))

	var content string
	var err error

	switch ext {
	case ".pdf":
		content, err = t.loadPDF(filePath)
	case ".docx":
		content, err = t.loadDOCX(filePath)
	case ".xlsx":
		content, err = t.loadXLSX(filePath)
	case ".csv":
		content, err = t.loadCSV(filePath)
	case ".jpg", ".jpeg", ".png", ".gif", ".bmp", ".tiff", ".tif", ".webp":
		content, err = t.loadImage(filePath)
	default:
		// Handle regular text files
		fileContent, readErr := ioutil.ReadFile(filePath)
		if readErr != nil {
			return "", readErr
		}

		// Check if file is likely a text file by examining content
		if !t.isTextFile(fileContent) {
			return "", fmt.Errorf("file is not a supported file type")
		}

		content = string(fileContent)
	}

	if err != nil {
		return "", err
	}

	var result strings.Builder
	result.WriteString(fmt.Sprintf("File: %s\n", filePath))
	result.WriteString(content)
	result.WriteString("\n\n")

	return result.String(), nil
}

// loadDirectoryContent loads content from all text files in a directory
func (t *Terminal) loadDirectoryContent(dirPath string) (string, error) {
	var result strings.Builder

	err := filepath.WalkDir(dirPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}

		if d.IsDir() {
			return nil
		}

		fileContent, err := t.loadTextFile(path)
		if err != nil {
			return nil
		}

		result.WriteString(fileContent)
		return nil
	})

	if err != nil {
		return "", err
	}

	return result.String(), nil
}

// loadPDF extracts text content from PDF files
func (t *Terminal) loadPDF(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to open PDF file: %w", err)
	}
	defer file.Close()

	stat, err := file.Stat()
	if err != nil {
		return "", fmt.Errorf("failed to get file stats: %w", err)
	}

	reader, err := pdf.NewReader(file, stat.Size())
	if err != nil {
		return "", fmt.Errorf("failed to create PDF reader: %w", err)
	}

	var content strings.Builder
	numPages := reader.NumPage()

	for i := 1; i <= numPages; i++ {
		page := reader.Page(i)

		text, err := page.GetPlainText(nil)
		if err != nil {
			continue // Skip pages with text extraction errors
		}

		content.WriteString(text)
		content.WriteString("\n")
	}

	return content.String(), nil
}

// loadDOCX extracts text content from DOCX files
func (t *Terminal) loadDOCX(filePath string) (string, error) {
	doc, err := docx.ReadDocxFile(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to open DOCX file: %w", err)
	}
	defer doc.Close()

	docx := doc.Editable()
	return docx.GetContent(), nil
}

// loadXLSX extracts text content from XLSX files
func (t *Terminal) loadXLSX(filePath string) (string, error) {
	workbook, err := xlsx.OpenFile(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to open XLSX file: %w", err)
	}

	var content strings.Builder

	for _, sheet := range workbook.Sheets {
		content.WriteString(fmt.Sprintf("=== Sheet: %s ===\n", sheet.Name))

		err := sheet.ForEachRow(func(row *xlsx.Row) error {
			var rowData []string
			err := row.ForEachCell(func(cell *xlsx.Cell) error {
				text := cell.String()
				rowData = append(rowData, text)
				return nil
			})
			if err != nil {
				return err
			}

			// Only add non-empty rows
			if len(strings.TrimSpace(strings.Join(rowData, ""))) > 0 {
				content.WriteString(fmt.Sprintf("%s\n", strings.Join(rowData, " | ")))
			}
			return nil
		})

		if err != nil {
			return "", fmt.Errorf("failed to read sheet %s: %w", sheet.Name, err)
		}
		content.WriteString("\n")
	}

	return content.String(), nil
}

// loadCSV extracts text content from CSV files
func (t *Terminal) loadCSV(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to open CSV file: %w", err)
	}
	defer file.Close()

	reader := csv.NewReader(file)
	records, err := reader.ReadAll()
	if err != nil {
		return "", fmt.Errorf("failed to read CSV file: %w", err)
	}

	var content strings.Builder

	for rowIndex, record := range records {
		content.WriteString(fmt.Sprintf("Row %d: %s\n", rowIndex+1, strings.Join(record, " | ")))
	}

	return content.String(), nil
}

// loadImage loads and extracts metadata and basic information from image files
func (t *Terminal) loadImage(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to open image file: %w", err)
	}
	defer file.Close()

	var content strings.Builder
	content.WriteString(fmt.Sprintf("Image Analysis for: %s\n\n", filepath.Base(filePath)))

	// Get basic file info
	fileInfo, err := file.Stat()
	if err == nil {
		content.WriteString(fmt.Sprintf("File Size: %d bytes (%.2f KB)\n", fileInfo.Size(), float64(fileInfo.Size())/1024.0))
		content.WriteString(fmt.Sprintf("Modified: %s\n", fileInfo.ModTime().Format("2006-01-02 15:04:05")))
	}

	// Reset file pointer
	file.Seek(0, 0)

	// Decode image to get basic properties
	img, format, err := image.Decode(file)
	if err != nil {
		return "", fmt.Errorf("failed to decode image: %w", err)
	}

	bounds := img.Bounds()
	content.WriteString(fmt.Sprintf("Format: %s\n", strings.ToUpper(format)))
	content.WriteString(fmt.Sprintf("Dimensions: %dx%d pixels\n", bounds.Dx(), bounds.Dy()))

	// Reset file pointer for EXIF reading
	file.Seek(0, 0)

	// Try to extract EXIF metadata
	exifData, err := exif.Decode(file)
	if err == nil {
		content.WriteString("\nEXIF Metadata:\n")

		// Common EXIF tags to extract
		exifTags := []struct {
			name string
			tag  exif.FieldName
		}{
			{"Camera Make", exif.Make},
			{"Camera Model", exif.Model},
			{"DateTime", exif.DateTime},
			{"DateTimeOriginal", exif.DateTimeOriginal},
			{"DateTimeDigitized", exif.DateTimeDigitized},
			{"Software", exif.Software},
			{"Artist", exif.Artist},
			{"Copyright", exif.Copyright},
			{"ImageDescription", exif.ImageDescription},
			{"UserComment", exif.UserComment},
			{"Orientation", exif.Orientation},
			{"XResolution", exif.XResolution},
			{"YResolution", exif.YResolution},
			{"ResolutionUnit", exif.ResolutionUnit},
			{"Flash", exif.Flash},
			{"FocalLength", exif.FocalLength},
			{"ExposureTime", exif.ExposureTime},
			{"FNumber", exif.FNumber},
			{"ISO", exif.ISOSpeedRatings},
			{"WhiteBalance", exif.WhiteBalance},
			{"GPS Latitude", exif.GPSLatitude},
			{"GPS Longitude", exif.GPSLongitude},
			{"GPS Altitude", exif.GPSAltitude},
		}

		for _, tagInfo := range exifTags {
			if tag, err := exifData.Get(tagInfo.tag); err == nil {
				value := strings.TrimSpace(tag.String())
				if value != "" && value != "0" && value != "0/1" {
					content.WriteString(fmt.Sprintf("  %s: %s\n", tagInfo.name, value))
				}
			}
		}

		// Try to get GPS coordinates in a more readable format
		if lat, err := exifData.Get(exif.GPSLatitude); err == nil {
			if latRef, err := exifData.Get(exif.GPSLatitudeRef); err == nil {
				if lon, err := exifData.Get(exif.GPSLongitude); err == nil {
					if lonRef, err := exifData.Get(exif.GPSLongitudeRef); err == nil {
						latDeg := convertDMSToDecimal(lat.String())
						lonDeg := convertDMSToDecimal(lon.String())
						if latRef.String() == "S" {
							latDeg = -latDeg
						}
						if lonRef.String() == "W" {
							lonDeg = -lonDeg
						}
						if latDeg != 0 || lonDeg != 0 {
							content.WriteString(fmt.Sprintf("  GPS Coordinates: %.6f, %.6f\n", latDeg, lonDeg))
						}
					}
				}
			}
		}
	} else {
		content.WriteString("\nNo EXIF metadata found or failed to read EXIF data\n")
	}

	// Color analysis - sample some pixels to get dominant colors
	content.WriteString(fmt.Sprintf("\nImage Properties:\n"))
	content.WriteString(fmt.Sprintf("  Color Mode: %T\n", img.ColorModel()))
	content.WriteString(fmt.Sprintf("  Aspect Ratio: %.2f:1\n", float64(bounds.Dx())/float64(bounds.Dy())))

	megapixels := float64(bounds.Dx()*bounds.Dy()) / 1000000.0
	if megapixels > 1.0 {
		content.WriteString(fmt.Sprintf("  Megapixels: %.1f MP\n", megapixels))
	} else {
		content.WriteString(fmt.Sprintf("  Resolution: %.0f K pixels\n", megapixels*1000))
	}

	// Extract text using OCR
	content.WriteString("\n" + strings.Repeat("=", 50) + "\n")
	content.WriteString("TEXT EXTRACTION (OCR):\n")
	content.WriteString(strings.Repeat("=", 50) + "\n\n")

	extractedText, err := t.extractTextFromImage(filePath)
	if err != nil {
		content.WriteString(fmt.Sprintf("OCR Error: %v\n", err))
	} else if strings.TrimSpace(extractedText) == "" {
		content.WriteString("No text detected in the image.\n")
	} else {
		content.WriteString("Extracted Text:\n")
		content.WriteString(strings.Repeat("-", 30) + "\n")
		content.WriteString(extractedText)
		content.WriteString("\n" + strings.Repeat("-", 30) + "\n")
	}

	return content.String(), nil
}

// convertDMSToDecimal converts degrees-minutes-seconds format to decimal degrees
func convertDMSToDecimal(dms string) float64 {
	// Parse DMS format like "40/1,2/1,3/1" or similar
	parts := strings.Split(dms, ",")
	if len(parts) < 3 {
		return 0
	}

	var degrees, minutes, seconds float64

	if d := parseFraction(strings.TrimSpace(parts[0])); d >= 0 {
		degrees = d
	}
	if m := parseFraction(strings.TrimSpace(parts[1])); m >= 0 {
		minutes = m
	}
	if s := parseFraction(strings.TrimSpace(parts[2])); s >= 0 {
		seconds = s
	}

	return degrees + minutes/60.0 + seconds/3600.0
}

// parseFraction parses a fraction string like "40/1" to a float64
func parseFraction(fraction string) float64 {
	parts := strings.Split(fraction, "/")
	if len(parts) != 2 {
		if f, err := strconv.ParseFloat(fraction, 64); err == nil {
			return f
		}
		return -1
	}

	numerator, err1 := strconv.ParseFloat(parts[0], 64)
	denominator, err2 := strconv.ParseFloat(parts[1], 64)

	if err1 != nil || err2 != nil || denominator == 0 {
		return -1
	}

	return numerator / denominator
}

// extractTextFromImage uses Tesseract OCR to extract text from an image
func (t *Terminal) extractTextFromImage(filePath string) (string, error) {
	client := gosseract.NewClient()
	defer client.Close()

	// Set language (default to English, but could be made configurable)
	err := client.SetLanguage("eng")
	if err != nil {
		// If English fails, try without setting language
		client = gosseract.NewClient()
		defer client.Close()
	}

	// Set image source
	err = client.SetImage(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to set image source: %w", err)
	}

	// Configure OCR settings for better accuracy
	client.SetVariable("tessedit_char_whitelist", "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789.,!?@#$%^&*()_+-={}[]|\\:;\"'<>/~` ")

	// Extract text
	text, err := client.Text()
	if err != nil {
		return "", fmt.Errorf("OCR extraction failed: %w", err)
	}

	// Clean up the extracted text
	cleanedText := strings.TrimSpace(text)

	// Remove excessive whitespace and normalize line breaks
	lines := strings.Split(cleanedText, "\n")
	var cleanedLines []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			cleanedLines = append(cleanedLines, line)
		}
	}

	return strings.Join(cleanedLines, "\n"), nil
}

// isTextFile checks if content is likely from a text file
func (t *Terminal) isTextFile(content []byte) bool {
	if len(content) == 0 {
		return true
	}

	// Check for null bytes (binary files usually contain them)
	for _, b := range content {
		if b == 0 {
			return false
		}
	}

	// Check if most bytes are printable ASCII or common UTF-8
	printableCount := 0
	for _, b := range content {
		if (b >= 32 && b <= 126) || b == 9 || b == 10 || b == 13 || b >= 128 {
			printableCount++
		}
	}

	// If more than 95% of bytes are printable, consider it text
	return float64(printableCount)/float64(len(content)) > 0.95
}

// GetCurrentDirFiles returns files and directories in the current directory
func (t *Terminal) GetCurrentDirFiles() ([]string, error) {
	pwd, err := os.Getwd()
	if err != nil {
		return nil, err
	}

	entries, err := ioutil.ReadDir(pwd)
	if err != nil {
		return nil, err
	}

	var items []string
	for _, entry := range entries {
		if entry.IsDir() {
			items = append(items, entry.Name()+"/")
		} else {
			items = append(items, entry.Name())
		}
	}

	return items, nil
}

// GetCurrentDirFilesRecursive returns all files and directories in the current directory and subdirectories
func (t *Terminal) GetCurrentDirFilesRecursive() ([]string, error) {
	currentDir, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("failed to get current directory: %v", err)
	}
	return t.GetDirFilesRecursive(currentDir)
}

// GetDirFilesRecursive returns all files and directories in the specified directory and subdirectories
func (t *Terminal) GetDirFilesRecursive(targetDir string) ([]string, error) {
	var items []string

	err := filepath.WalkDir(targetDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // Skip files we can't access
		}

		// Skip the root directory itself
		if path == targetDir {
			return nil
		}

		// Get relative path from target directory
		relPath, err := filepath.Rel(targetDir, path)
		if err != nil {
			return nil // Skip if we can't get relative path
		}

		// Skip certain system directories but allow hidden files
		if d.IsDir() && (filepath.Base(relPath) == ".git" || filepath.Base(relPath) == ".svn" || filepath.Base(relPath) == ".hg") {
			return filepath.SkipDir
		}

		// Add directories with trailing slash and files as-is
		if d.IsDir() {
			items = append(items, relPath+"/")
		} else {
			items = append(items, relPath)
		}

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to walk directory: %v", err)
	}

	return items, nil
}

// GetCurrentDirFilesOnly returns non-directory files in the current directory
func (t *Terminal) GetCurrentDirFilesOnly() ([]string, error) {
	pwd, err := os.Getwd()
	if err != nil {
		return nil, err
	}

	entries, err := ioutil.ReadDir(pwd)
	if err != nil {
		return nil, err
	}

	var items []string
	for _, entry := range entries {
		if !entry.IsDir() {
			items = append(items, entry.Name())
		}
	}

	return items, nil
}

// CodeDump generates a comprehensive code dump of all text files in the current directory
func (t *Terminal) CodeDump() (string, error) {
	pwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("failed to get current directory: %v", err)
	}

	return t.CodeDumpFromDir(pwd)
}

// CodeDumpFromDir generates a comprehensive code dump of all text files in the specified directory
func (t *Terminal) CodeDumpFromDir(targetDir string) (string, error) {
	// Convert to absolute path
	absDir, err := filepath.Abs(targetDir)
	if err != nil {
		return "", fmt.Errorf("failed to get absolute path: %v", err)
	}

	// Discover all files while respecting .gitignore
	allFiles, err := t.discoverFiles(absDir)
	if err != nil {
		return "", fmt.Errorf("failed to discover files: %v", err)
	}

	if len(allFiles) == 0 {
		return "", fmt.Errorf("no text files found in directory")
	}

	// Add NONE option at the top of the list
	fzfOptions := append([]string{"[NONE]"}, allFiles...)

	// Use fzf to let user exclude files/directories
	excludedItems, err := t.FzfMultiSelect(fzfOptions, "Exclude from dump (TAB=multi): ")
	if err != nil {
		return "", fmt.Errorf("failed to get exclusions: %v", err)
	}

	// Filter out the NONE option if selected
	var filteredExclusions []string
	for _, item := range excludedItems {
		if !strings.HasPrefix(item, "[NONE") {
			filteredExclusions = append(filteredExclusions, item)
		}
	}
	excludedItems = filteredExclusions

	// Filter out excluded items
	includedFiles := t.filterExcludedFiles(allFiles, excludedItems)

	if len(includedFiles) == 0 {
		return "", fmt.Errorf("no files remaining after exclusions")
	}

	// Generate the codedump string
	return t.generateCodeDumpFromDir(includedFiles, absDir)
}

// CodeDumpFromDirForCLI generates a comprehensive code dump for CLI usage with cancellation detection
func (t *Terminal) CodeDumpFromDirForCLI(targetDir string) (string, error) {
	// Convert to absolute path
	absDir, err := filepath.Abs(targetDir)
	if err != nil {
		return "", fmt.Errorf("failed to get absolute path: %v", err)
	}

	// Discover all files while respecting .gitignore
	allFiles, err := t.discoverFiles(absDir)
	if err != nil {
		return "", fmt.Errorf("failed to discover files: %v", err)
	}

	if len(allFiles) == 0 {
		return "", fmt.Errorf("no text files found in directory")
	}

	// Add NONE option at the top of the list
	fzfOptions := append([]string{"[NONE]"}, allFiles...)

	// Use CLI-specific fzf that detects cancellation
	excludedItems, err := t.FzfMultiSelectForCLI(fzfOptions, "Exclude from dump (TAB=multi): ")
	if err != nil {
		return "", fmt.Errorf("failed to get exclusions: %v", err)
	}

	// Filter out the NONE option if selected
	var filteredExclusions []string
	for _, item := range excludedItems {
		if !strings.HasPrefix(item, "[NONE") {
			filteredExclusions = append(filteredExclusions, item)
		}
	}
	excludedItems = filteredExclusions

	// Filter out excluded items
	includedFiles := t.filterExcludedFiles(allFiles, excludedItems)

	if len(includedFiles) == 0 {
		return "", fmt.Errorf("no files remaining after exclusions")
	}

	// Generate the codedump string
	return t.generateCodeDumpFromDir(includedFiles, absDir)
}

// discoverFiles finds all text files in the directory, respecting .gitignore
func (t *Terminal) discoverFiles(rootDir string) ([]string, error) {
	var allFiles []string
	var allDirs []string
	gitignorePatterns := t.loadGitignorePatterns(rootDir)

	err := filepath.WalkDir(rootDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // Skip files we can't access
		}

		relPath, err := filepath.Rel(rootDir, path)
		if err != nil {
			return nil
		}

		// Skip the root directory itself
		if relPath == "." {
			return nil
		}

		// Check if this path should be ignored
		if t.shouldIgnore(relPath, gitignorePatterns) {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		if d.IsDir() {
			allDirs = append(allDirs, relPath+"/")
		} else {
			// Check if it's a text file
			if t.isTextFileByPath(path) {
				allFiles = append(allFiles, relPath)
			}
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	// Combine directories and files for the selection list
	var combined []string
	combined = append(combined, allDirs...)
	combined = append(combined, allFiles...)

	return combined, nil
}

// loadGitignorePatterns loads patterns from .gitignore file if it exists
func (t *Terminal) loadGitignorePatterns(rootDir string) []string {
	// Always ignore .git directory by default
	patterns := []string{".git/"}

	gitignorePath := filepath.Join(rootDir, ".gitignore")
	file, err := os.Open(gitignorePath)
	if err != nil {
		return patterns // No .gitignore file, return with default patterns
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line != "" && !strings.HasPrefix(line, "#") {
			patterns = append(patterns, line)
		}
	}

	return patterns
}

// shouldIgnore checks if a path should be ignored based on gitignore patterns
func (t *Terminal) shouldIgnore(path string, patterns []string) bool {
	for _, pattern := range patterns {
		if t.matchesPattern(path, pattern) {
			return true
		}
	}
	return false
}

// matchesPattern checks if a path matches a gitignore pattern (simplified)
func (t *Terminal) matchesPattern(path, pattern string) bool {
	// Remove leading slash if present
	pattern = strings.TrimPrefix(pattern, "/")

	// Handle directory patterns (ending with /)
	if strings.HasSuffix(pattern, "/") {
		dirPattern := strings.TrimSuffix(pattern, "/")
		// Check if the path starts with the directory pattern
		return strings.HasPrefix(path, dirPattern+"/") || path == dirPattern
	}

	// Handle wildcard patterns
	if strings.Contains(pattern, "*") {
		matched, _ := filepath.Match(pattern, path)
		if matched {
			return true
		}
		// Also check if any parent directory matches
		parts := strings.Split(path, "/")
		for i := range parts {
			partialPath := strings.Join(parts[:i+1], "/")
			if matched, _ := filepath.Match(pattern, partialPath); matched {
				return true
			}
		}
		return false
	}

	// Exact match or prefix match for directories
	return path == pattern || strings.HasPrefix(path, pattern+"/")
}

// isTextFileByPath checks if a file is supported based on its path and content
func (t *Terminal) isTextFileByPath(filePath string) bool {
	// Check file extension first
	ext := strings.ToLower(filepath.Ext(filePath))
	supportedExtensions := map[string]bool{
		// Text files
		".txt": true, ".md": true, ".go": true, ".py": true, ".js": true,
		".ts": true, ".jsx": true, ".tsx": true, ".html": true, ".css": true,
		".scss": true, ".sass": true, ".json": true, ".xml": true, ".yaml": true,
		".yml": true, ".toml": true, ".ini": true, ".cfg": true, ".conf": true,
		".sh": true, ".bash": true, ".zsh": true, ".fish": true, ".ps1": true,
		".bat": true, ".cmd": true, ".dockerfile": true, ".makefile": true,
		".c": true, ".cpp": true, ".cc": true, ".cxx": true, ".h": true,
		".hpp": true, ".java": true, ".kt": true, ".scala": true, ".rb": true,
		".php": true, ".pl": true, ".pm": true, ".r": true, ".sql": true,
		".vim": true, ".lua": true, ".rs": true, ".swift": true, ".m": true,
		".mm": true, ".cs": true, ".vb": true, ".fs": true, ".clj": true,
		// Document files
		".pdf": true, ".docx": true, ".xlsx": true, ".csv": true,
		// Image files
		".jpg": true, ".jpeg": true, ".png": true, ".gif": true, ".bmp": true, ".tiff": true, ".tif": true, ".webp": true,
		".hs": true, ".elm": true, ".ex": true, ".exs": true, ".erl": true,
		".hrl": true, ".dart": true, ".gradle": true, ".sbt": true,
		".build": true, ".cmake": true, ".mk": true, ".am": true, ".in": true,
		".ac": true, ".m4": true, ".spec": true, ".desktop": true, ".service": true,
		".log": true, ".tsv": true, ".properties": true, ".env": true,
	}

	if supportedExtensions[ext] {
		return true
	}

	// For files without extension or unknown extensions, check if it's a text file
	if ext == "" {
		content, err := ioutil.ReadFile(filePath)
		if err != nil {
			return false
		}
		return t.isTextFile(content)
	}

	return false
}

// filterExcludedFiles removes excluded files and directories from the list
func (t *Terminal) filterExcludedFiles(allFiles, excludedItems []string) []string {
	excludedSet := make(map[string]bool)
	var excludedDirs []string

	// Process excluded items
	for _, item := range excludedItems {
		excludedSet[item] = true
		if strings.HasSuffix(item, "/") {
			excludedDirs = append(excludedDirs, strings.TrimSuffix(item, "/"))
		}
	}

	var includedFiles []string
	for _, file := range allFiles {
		// Skip if file is directly excluded
		if excludedSet[file] {
			continue
		}

		// Skip if file is a directory (we only want files in the final list)
		if strings.HasSuffix(file, "/") {
			continue
		}

		// Skip if file is in an excluded directory
		excluded := false
		for _, excludedDir := range excludedDirs {
			if strings.HasPrefix(file, excludedDir+"/") {
				excluded = true
				break
			}
		}

		if !excluded {
			includedFiles = append(includedFiles, file)
		}
	}

	return includedFiles
}

// generateCodeDump creates the final codedump string
func (t *Terminal) generateCodeDump(files []string) (string, error) {
	pwd, _ := os.Getwd()
	return t.generateCodeDumpFromDir(files, pwd)
}

// generateCodeDumpFromDir creates the final codedump string from a specific directory
func (t *Terminal) generateCodeDumpFromDir(files []string, sourceDir string) (string, error) {
	var result strings.Builder

	result.WriteString("=== CODE DUMP ===\n\n")
	result.WriteString(fmt.Sprintf("Generated from directory: %s\n", sourceDir))
	result.WriteString(fmt.Sprintf("Total files: %d\n\n", len(files)))

	for _, file := range files {
		// Build full path for reading
		fullPath := filepath.Join(sourceDir, file)

		// Check if this is a supported file type that we can process
		ext := strings.ToLower(filepath.Ext(file))
		// Image files are excluded from codedump, only document types are processed as special files.
		supportedTypes := []string{".pdf", ".docx", ".xlsx", ".csv"}
		isSpecialFile := false
		for _, supportedExt := range supportedTypes {
			if ext == supportedExt {
				isSpecialFile = true
				break
			}
		}

		var content string
		if isSpecialFile {
			// Use loadTextFile for special file types (PDFs, images, etc.)
			fileContent, err := t.loadTextFile(fullPath)
			if err != nil {
				result.WriteString(fmt.Sprintf("=== FILE: %s ===\nError processing file: %v\n\n", file, err))
				continue
			}
			content = fileContent
		} else {
			// Use regular file reading for text files
			fileBytes, err := ioutil.ReadFile(fullPath)
			if err != nil {
				result.WriteString(fmt.Sprintf("=== FILE: %s ===\nError reading file: %v\n\n", file, err))
				continue
			}

			// Double-check if it's a text file
			if !t.isTextFile(fileBytes) {
				continue
			}

			content = fmt.Sprintf("File: %s\n%s", file, string(fileBytes))
		}

		result.WriteString(fmt.Sprintf("=== FILE: %s ===\n", file))
		result.WriteString(content)
		result.WriteString("\n\n")
	}

	result.WriteString("=== END CODE DUMP ===")
	return result.String(), nil
}

// isURL checks if a string is a valid URL
func (t *Terminal) isURL(str string) bool {
	u, err := url.Parse(str)
	return err == nil && u.Scheme != "" && u.Host != ""
}

// IsURL is a public method to check if a string is a valid URL
func (t *Terminal) IsURL(str string) bool {
	return t.isURL(str)
}

// isYouTubeURL checks if a URL is a YouTube URL
func (t *Terminal) isYouTubeURL(urlStr string) bool {
	u, err := url.Parse(urlStr)
	if err != nil {
		return false
	}
	host := strings.ToLower(u.Host)
	return strings.Contains(host, "youtube.com") ||
		strings.Contains(host, "youtu.be") ||
		strings.Contains(host, "m.youtube.com") ||
		strings.Contains(host, "www.youtube.com") ||
		strings.Contains(host, "youtube-nocookie.com")
}

// cleanURL removes backslash escapes from URLs that shells sometimes add
func (t *Terminal) cleanURL(urlStr string) string {
	// Remove backslash escapes from common shell-escaped characters in URLs
	cleaned := strings.ReplaceAll(urlStr, `\?`, `?`)
	cleaned = strings.ReplaceAll(cleaned, `\=`, `=`)
	cleaned = strings.ReplaceAll(cleaned, `\&`, `&`)
	return cleaned
}

// scrapeURL scrapes content from a single URL
func (t *Terminal) scrapeURL(urlStr string) (string, error) {
	// Clean any shell escapes from the URL
	cleanedURL := t.cleanURL(urlStr)

	var result strings.Builder
	result.WriteString(fmt.Sprintf("=== %s ===\n\n", cleanedURL))

	if t.isYouTubeURL(cleanedURL) {
		// YouTube scraping with yt-dlp
		content, err := t.scrapeYouTube(cleanedURL)
		if err != nil {
			return "", fmt.Errorf("failed to scrape YouTube URL: %w", err)
		}
		result.WriteString(content)
	} else {
		// Regular web scraping with curl + lynx
		content, err := t.scrapeWeb(cleanedURL)
		if err != nil {
			return "", fmt.Errorf("failed to scrape URL: %w", err)
		}
		result.WriteString(content)
	}

	result.WriteString("\n")
	return result.String(), nil
}

// scrapeWeb scrapes regular web pages using curl and lynx
func (t *Terminal) scrapeWeb(urlStr string) (string, error) {
	// Use curl to fetch the content and pipe to lynx for text extraction
	cmd := exec.Command("sh", "-c", fmt.Sprintf("curl -s %s | lynx -dump -stdin", urlStr))
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to scrape web content: %w", err)
	}
	return string(output), nil
}

// scrapeYouTube scrapes YouTube videos using yt-dlp
func (t *Terminal) scrapeYouTube(urlStr string) (string, error) {
	var result strings.Builder

	// Get metadata
	result.WriteString("--- METADATA ---\n")
	metadataCmd := exec.Command("yt-dlp", "-j", urlStr)
	metadataOutput, err := metadataCmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to get YouTube metadata: %w", err)
	}

	// Parse key fields from JSON (simple parsing without jq)
	metadata := string(metadataOutput)
	result.WriteString(t.parseYouTubeMetadata(metadata))

	// Get subtitles
	result.WriteString("\n--- SUBTITLES ---\n")

	tempDir, err := t.getTempDir()
	if err != nil {
		return "", fmt.Errorf("failed to get temp directory: %w", err)
	}

	baseName := filepath.Join(tempDir, fmt.Sprintf("yt_%d", time.Now().UnixNano()))

	// Download subtitles
	subtitleCmd := exec.Command("yt-dlp", "--quiet", "--skip-download",
		"--write-auto-subs", "--sub-lang", "en", "--sub-format", "srt",
		"-o", baseName+".%(ext)s", urlStr)

	err = subtitleCmd.Run()
	if err == nil {
		// Find the .srt file
		pattern := baseName + "*.srt"
		matches, _ := filepath.Glob(pattern)
		if len(matches) > 0 {
			srtContent, readErr := ioutil.ReadFile(matches[0])
			if readErr == nil {
				result.WriteString(string(srtContent))
			}
			// Clean up temp files
			for _, match := range matches {
				os.Remove(match)
			}
		}
	}

	return result.String(), nil
}

// parseYouTubeMetadata extracts key metadata fields from JSON response
func (t *Terminal) parseYouTubeMetadata(jsonStr string) string {
	var result strings.Builder

	// Simple regex-based extraction of key fields
	extractField := func(field string) string {
		pattern := fmt.Sprintf(`"%s":\s*"([^"]*)"`, field)
		re := regexp.MustCompile(pattern)
		matches := re.FindStringSubmatch(jsonStr)
		if len(matches) > 1 {
			return matches[1]
		}

		// Try numeric fields
		pattern = fmt.Sprintf(`"%s":\s*(\d+)`, field)
		re = regexp.MustCompile(pattern)
		matches = re.FindStringSubmatch(jsonStr)
		if len(matches) > 1 {
			return matches[1]
		}

		return ""
	}

	if title := extractField("title"); title != "" {
		result.WriteString(fmt.Sprintf("Title: %s\n", title))
	}
	if duration := extractField("duration"); duration != "" {
		result.WriteString(fmt.Sprintf("Duration: %s seconds\n", duration))
	}
	if viewCount := extractField("view_count"); viewCount != "" {
		result.WriteString(fmt.Sprintf("View count: %s\n", viewCount))
	}
	if uploader := extractField("uploader"); uploader != "" {
		result.WriteString(fmt.Sprintf("Uploader: %s\n", uploader))
	}
	if uploadDate := extractField("upload_date"); uploadDate != "" {
		result.WriteString(fmt.Sprintf("Upload date: %s\n", uploadDate))
	}
	if description := extractField("description"); description != "" {
		// Truncate description if too long
		if len(description) > 500 {
			description = description[:500] + "..."
		}
		result.WriteString(fmt.Sprintf("Description: %s\n", description))
	}

	return result.String()
}

// ScrapeURLs scrapes content from multiple URLs
func (t *Terminal) ScrapeURLs(urls []string) (string, error) {
	var result strings.Builder

	for _, urlStr := range urls {
		if urlStr == "" {
			continue
		}

		content, err := t.scrapeURL(urlStr)
		if err != nil {
			result.WriteString(fmt.Sprintf("Error scraping %s: %v\n", urlStr, err))
			continue
		}

		result.WriteString(content)
	}

	return result.String(), nil
}

// WebSearch performs a web search using ddgr
func (t *Terminal) WebSearch(query string) (string, error) {
	_, err := exec.LookPath("ddgr")
	if err != nil {
		return "", fmt.Errorf("ddgr is not installed. Please install it to use web search")
	}

	numResults := fmt.Sprintf("%d", t.config.NumSearchResults)
	cmd := exec.Command("ddgr", "--json", "--num", numResults, query)
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("search failed: %w", err)
	}

	searchResults := string(output)
	if strings.TrimSpace(searchResults) == "" || searchResults == "[]" {
		noResultsMsg := fmt.Sprintf("No search results found for: %s\n", query)
		if t.config.ShowSearchResults {
			fmt.Print(noResultsMsg)
		}
		return noResultsMsg, nil
	}

	formatted := t.parseSearchResults(searchResults, query)

	if t.config.ShowSearchResults {
		fmt.Print(formatted)
	}

	return formatted, nil
}

// SearchResult represents a single search result
type SearchResult struct {
	Title    string `json:"title"`
	URL      string `json:"url"`
	Abstract string `json:"abstract"`
}

// parseSearchResults parses JSON search results and formats them
func (t *Terminal) parseSearchResults(jsonStr, query string) string {
	var result strings.Builder

	// Try to parse as proper JSON array
	var searchResults []SearchResult
	err := json.Unmarshal([]byte(jsonStr), &searchResults)
	if err != nil {
		// Fallback to line-by-line parsing if JSON parsing fails
		return t.parseSearchResultsLegacy(jsonStr, query)
	}

	if len(searchResults) == 0 {
		result.WriteString(fmt.Sprintf("No search results found for: %s\n", query))
		return result.String()
	}

	// Format each result
	for i, searchResult := range searchResults {
		if searchResult.Title != "" && searchResult.URL != "" {
			result.WriteString(fmt.Sprintf("\033[93m%d) \033[93m%s\033[0m\n", i+1, searchResult.Title))
			result.WriteString(fmt.Sprintf("\033[95m%s\033[0m\n", searchResult.URL))
			if searchResult.Abstract != "" {
				result.WriteString(fmt.Sprintf("\033[92m%s\033[0m\n", searchResult.Abstract))
			}
			if i < len(searchResults)-1 {
				result.WriteString("\n")
			}
		}
	}

	return result.String()
}

// parseSearchResultsLegacy handles line-by-line parsing as fallback
func (t *Terminal) parseSearchResultsLegacy(jsonStr, query string) string {
	var result strings.Builder

	// Simple JSON parsing to extract results
	lines := strings.Split(jsonStr, "\n")
	resultNumber := 1
	validResults := []string{}

	// First pass: collect all valid results
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || line == "[" || line == "]" {
			continue
		}

		// Remove trailing comma
		line = strings.TrimSuffix(line, ",")

		// Extract title, URL, and abstract using regex
		title := t.extractJSONField(line, "title")
		url := t.extractJSONField(line, "url")
		abstract := t.extractJSONField(line, "abstract")

		// Only add if we have at least title and URL
		if title != "" && url != "" {
			validResults = append(validResults, fmt.Sprintf("\033[93m%d) \033[93m%s\033[0m\n\033[95m%s\033[0m\n", resultNumber, title, url))
			if abstract != "" {
				validResults[len(validResults)-1] += fmt.Sprintf("\033[92m%s\033[0m\n", abstract)
			}
			resultNumber++
		}
	}

	// Format results with proper spacing
	for i, validResult := range validResults {
		result.WriteString(validResult)
		if i < len(validResults)-1 {
			result.WriteString("\n")
		}
	}

	return result.String()
}

// extractJSONField extracts a field value from a JSON string
func (t *Terminal) extractJSONField(jsonStr, field string) string {
	pattern := fmt.Sprintf(`"%s":\s*"([^"]*)"`, field)
	re := regexp.MustCompile(pattern)
	matches := re.FindStringSubmatch(jsonStr)
	if len(matches) > 1 {
		// Unescape JSON string
		value := matches[1]
		value = strings.ReplaceAll(value, `\"`, `"`)
		value = strings.ReplaceAll(value, `\\`, `\`)
		return value
	}
	return ""
}

// CopyToClipboard copies content to the system clipboard with cross-platform support
func (t *Terminal) CopyToClipboard(content string) error {
	var cmd *exec.Cmd

	// Detect platform and use appropriate clipboard command
	if _, err := exec.LookPath("pbcopy"); err == nil {
		// macOS
		cmd = exec.Command("pbcopy")
	} else if _, err := exec.LookPath("xclip"); err == nil {
		// Linux with xclip
		cmd = exec.Command("xclip", "-selection", "clipboard")
	} else if _, err := exec.LookPath("xsel"); err == nil {
		// Linux with xsel
		cmd = exec.Command("xsel", "--clipboard", "--input")
	} else if _, err := exec.LookPath("wl-copy"); err == nil {
		// Wayland (Linux)
		cmd = exec.Command("wl-copy")
	} else if _, err := exec.LookPath("termux-clipboard-set"); err == nil {
		// Android/Termux
		cmd = exec.Command("termux-clipboard-set")
	} else if _, err := exec.LookPath("clip"); err == nil {
		// Windows (WSL or Git Bash)
		cmd = exec.Command("clip")
	} else {
		return fmt.Errorf("no clipboard utility found. Please install: pbcopy (macOS), xclip/xsel (Linux), wl-copy (Wayland), or termux-clipboard-set (Android)")
	}

	cmd.Stdin = strings.NewReader(content)
	return cmd.Run()
}

// CopyResponsesInteractive allows user to select and copy chat responses to clipboard
func (t *Terminal) CopyResponsesInteractive(chatHistory []types.ChatHistory) error {
	if len(chatHistory) == 0 {
		return fmt.Errorf("no chat history available")
	}

	// Create list of responses for fzf selection
	var responseOptions []string
	var responseMap = make(map[string]types.ChatHistory)

	for i, entry := range chatHistory {
		if entry.Bot != "" {
			// Create display text with truncated response
			displayText := entry.Bot
			if len(displayText) > 100 {
				displayText = displayText[:97] + "..."
			}
			// Replace newlines with spaces for better fzf display
			displayText = strings.ReplaceAll(displayText, "\n", " ")

			optionText := fmt.Sprintf("[%d] %s", i+1, displayText)
			responseOptions = append(responseOptions, optionText)
			responseMap[optionText] = entry
		}
	}

	if len(responseOptions) == 0 {
		return fmt.Errorf("no responses found in chat history")
	}

	// Use fzf for multi-selection
	selected, err := t.FzfMultiSelect(responseOptions, "Select responses to copy (TAB=multi): ")
	if err != nil {
		return fmt.Errorf("selection failed: %w", err)
	}

	if len(selected) == 0 {
		t.PrintInfo("no responses selected")
		return nil
	}

	// Combine selected responses
	var combinedContent strings.Builder
	for i, selection := range selected {
		if entry, exists := responseMap[selection]; exists {
			if i > 0 {
				combinedContent.WriteString("\n\n---\n\n")
			}
			combinedContent.WriteString(entry.Bot)
		}
	}

	finalContent := combinedContent.String()

	// Open editor for final editing
	editedContent, err := t.openEditorWithContent(finalContent)
	if err != nil {
		return fmt.Errorf("editor failed: %w", err)
	}

	// Copy to clipboard
	err = t.CopyToClipboard(editedContent)
	if err != nil {
		return fmt.Errorf("failed to copy to clipboard: %w", err)
	}

	fmt.Printf("\033[93madded %d response(s) to clipboard\033[0m\n", len(selected))
	return nil
}

// openEditorWithContent opens the preferred editor with given content and returns the edited result
func (t *Terminal) openEditorWithContent(content string) (string, error) {
	// Create temporary file
	tempDir, err := t.getTempDir()
	if err != nil {
		return "", fmt.Errorf("failed to get temp directory: %w", err)
	}

	tempFile, err := ioutil.TempFile(tempDir, "ch_clipboard_*.md")
	if err != nil {
		return "", fmt.Errorf("failed to create temp file: %w", err)
	}
	defer os.Remove(tempFile.Name())

	// Write content to temp file
	if _, err := tempFile.WriteString(content); err != nil {
		tempFile.Close()
		return "", fmt.Errorf("failed to write to temp file: %w", err)
	}
	tempFile.Close()

	// Determine editor command
	editor := t.config.PreferredEditor
	if envEditor := os.Getenv("EDITOR"); envEditor != "" {
		editor = envEditor
	}

	// Open editor
	cmd := exec.Command(editor, tempFile.Name())
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("editor command failed: %w", err)
	}

	// Read edited content
	editedBytes, err := ioutil.ReadFile(tempFile.Name())
	if err != nil {
		return "", fmt.Errorf("failed to read edited content: %w", err)
	}

	return string(editedBytes), nil
}
