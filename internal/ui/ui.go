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
	"io"
	"io/fs"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/MehmetMHY/ch/internal/config"
	"github.com/MehmetMHY/ch/pkg/types"
	"github.com/ledongthuc/pdf"
	"github.com/lu4p/cat"
	"github.com/rwcarlsen/goexif/exif"
	"github.com/tealeg/xlsx/v3"
	"golang.org/x/net/html"
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

// runFzfSSHSafe executes fzf in a way that works correctly over SSH connections
// by keeping stderr attached to TTY for UI display and redirecting stdout to temp file
func (t *Terminal) runFzfSSHSafe(fzfArgs []string, inputText string) (string, error) {
	tempDir, err := config.GetTempDir()
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
	tempDir, err := config.GetTempDir()
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
	fmt.Println("ch - lightweight CLI for AI models")
	fmt.Println("")
	fmt.Println("usage:")
	fmt.Printf("  ch [-h] [-c] [--clear] [-a] [-d [dir]] [-p [platform]] [-m model] [-o platform|model] [-l file/url] [-w query] [-s url] [-e] [query]\n")
	fmt.Println("")
	fmt.Println("options:")
	fmt.Printf("  %-18s %s\n", "-h, --help", "show help and exit")
	fmt.Printf("  %-18s %s\n", "-c, --continue", "continue from latest session")
	fmt.Printf("  %-18s %s\n", "--clear", "clear all tmp files")
	fmt.Printf("  %-18s %s\n", "-a, --history", "search sessions")
	fmt.Printf("  %-18s %s\n", "-d [dir]", "generate codedump")
	fmt.Printf("  %-18s %s\n", "-p [platform]", "switch platform")
	fmt.Printf("  %-18s %s\n", "-m model", "specify model")
	fmt.Printf("  %-18s %s\n", "-o platform|model", "specify platform and model")
	fmt.Printf("  %-18s %s\n", "-l file/url", "load file or scrape URL")
	fmt.Printf("  %-18s %s\n", "-w query", "web search")
	fmt.Printf("  %-18s %s\n", "-s url", "scrape URL")
	fmt.Printf("  %-18s %s\n", "-e, --export", "export code blocks")
	fmt.Println("")
	fmt.Println("examples:")
	fmt.Println("  ch -p \"openai\" -m \"gpt-4.1\" \"goal of life\"")
	fmt.Println("  cat example.txt | ch \"what does this do?\"")
	fmt.Println("  ch \"what is AI?\"")
	fmt.Println("")

	// Dynamically generate platforms list
	var platforms []string
	platforms = append(platforms, "openai")
	for name := range t.config.Platforms {
		if name != "openai" {
			platforms = append(platforms, name)
		}
	}
	fmt.Println("available platforms:")
	fmt.Printf("  %s\n", strings.Join(platforms, ", "))
	fmt.Println("")
	fmt.Println("interactive mode:")
	fmt.Println("  run 'ch' for interactive. use '!h' for help.")
}

// RecordShellSession records the entire shell session and returns the content as a string.
func (t *Terminal) RecordShellSession() (string, error) {
	shell := os.Getenv("SHELL")
	if shell == "" {
		shell = "/bin/sh" // Fallback shell
	}

	// Get the application's temporary directory
	tempDir, err := config.GetTempDir()
	if err != nil {
		return "", fmt.Errorf("failed to get temp directory: %w", err)
	}

	// Create a temporary file to store the session recording
	tempFile, err := ioutil.TempFile(tempDir, "ch_shell_session_*.log")
	if err != nil {
		return "", fmt.Errorf("failed to create temporary file: %w", err)
	}
	defer os.Remove(tempFile.Name()) // Clean up the temp file

	t.PrintInfo(fmt.Sprintf("%s session started", shell))

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
		"--prompt=option: ", "--multi",
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

	// If >all is selected along with other items, ignore >all and process others
	if len(selectedItems) > 1 {
		for _, item := range selectedItems {
			if !strings.HasPrefix(item, ">all") {
				return t.processHelpSelection(item, options)
			}
		}
	}

	// If only >all is selected, show all options
	if len(selectedItems) == 1 && strings.HasPrefix(selectedItems[0], ">all") {
		for _, option := range options {
			if !strings.HasPrefix(option, ">all") && !strings.HasPrefix(option, ">state") {
				fmt.Printf("\033[93m%s\033[0m\n", option)
			}
		}
		return ""
	}

	// Process single selection
	return t.processHelpSelection(selectedItems[0], options)
}

func (t *Terminal) processHelpSelection(selected string, options []string) string {
	if strings.HasPrefix(selected, ">state") {
		return ">state"
	}
	// If line contains >, just print it in yellow and return
	if strings.HasPrefix(selected, ">") {
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

// getCommandList returns the list of help commands
func (t *Terminal) getCommandList() []string {
	return []string{
		fmt.Sprintf("%s - exit interface", t.config.ExitKey),
		fmt.Sprintf("%s - help page", t.config.HelpKey),
		fmt.Sprintf("%s - clear chat history", t.config.ClearHistory),
		fmt.Sprintf("%s - backtrack messages", t.config.Backtrack),
		fmt.Sprintf("%s - select from all models", t.config.AllModels),
		fmt.Sprintf("%s - switch models", t.config.ModelSwitch),
		fmt.Sprintf("%s - switch platforms", t.config.PlatformSwitch),
		fmt.Sprintf("%s - record shell session", t.config.ShellRecord),
		fmt.Sprintf("%s - generate codedump", t.config.CodeDump),
		fmt.Sprintf("%s - export chat(s)", t.config.ExportChat),
		fmt.Sprintf("%s - add to clipboard", t.config.CopyToClipboard),
		fmt.Sprintf("%s - multi-line input mode", t.config.MultiLine),
		fmt.Sprintf("%s [buff] - text editor mode", t.config.EditorInput),
		fmt.Sprintf("%s [dir] - load files/dirs", t.config.LoadFiles),
		fmt.Sprintf("%s [url] - scrape URL(s)", t.config.ScrapeURL),
		fmt.Sprintf("%s [query] - web search", t.config.WebSearch),
		fmt.Sprintf("%s [exact] - search sessions", t.config.AnswerSearch),
		"ctrl+c - clear prompt input",
		"ctrl+d - exit completely",
	}
}

// getInteractiveHelpOptions returns a slice of strings containing the help information for fzf selection.
func (t *Terminal) getInteractiveHelpOptions() []string {
	options := []string{
		">all - show all help options",
		">state - show current state",
	}
	options = append(options, t.getCommandList()...)
	return options
}

// ShowLoadingAnimation displays a loading animation
func (t *Terminal) ShowLoadingAnimation(message string, done chan bool) {
	if t.config.IsPipedOutput {
		<-done // Just wait for done signal, don't show animation
		return
	}
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
	if t.config.IsPipedOutput {
		fmt.Printf("%s\n", message)
	} else {
		fmt.Printf("\033[92m%s\033[0m\n", message)
	}
}

// PrintError prints an error message
func (t *Terminal) PrintError(message string) {
	if t.config.IsPipedOutput {
		fmt.Fprintf(os.Stderr, "%s\n", message)
	} else {
		fmt.Printf("\033[91m%s\033[0m\n", message)
	}
}

// PrintInfo prints an informational message
func (t *Terminal) PrintInfo(message string) {
	if t.config.IsPipedOutput {
		return // Suppress info messages when piped
	}
	fmt.Printf("\033[93m%s\033[0m\n", message)
}

// PrintModelSwitch prints model switch confirmation
func (t *Terminal) PrintModelSwitch(model string) {
	if t.config.IsPipedOutput {
		return // Suppress when piped
	}
	fmt.Printf("\033[96m%s\033[0m \033[95m%s\033[0m\n", t.config.CurrentPlatform, model)
}

// PrintPlatformSwitch prints platform switch confirmation
func (t *Terminal) PrintPlatformSwitch(platform, model string) {
	if t.config.IsPipedOutput {
		return // Suppress when piped
	}
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
	case ".docx", ".odt", ".rtf":
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

// loadDOCX extracts text content from DOCX, ODT, and RTF files
func (t *Terminal) loadDOCX(filePath string) (string, error) {
	text, err := cat.File(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to extract text from document: %w", err)
	}
	return text, nil
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
	content.WriteString(fmt.Sprintf("image analysis for: %s\n\n", filepath.Base(filePath)))

	// Get basic file info
	fileInfo, err := file.Stat()
	if err == nil {
		content.WriteString(fmt.Sprintf("file size: %d bytes (%.2f KB)\n", fileInfo.Size(), float64(fileInfo.Size())/1024.0))
		content.WriteString(fmt.Sprintf("modified: %s\n", fileInfo.ModTime().Format("2006-01-02 15:04:05")))
	}

	// Reset file pointer
	file.Seek(0, 0)

	// Decode image to get basic properties
	img, format, err := image.Decode(file)
	if err != nil {
		return "", fmt.Errorf("failed to decode image: %w", err)
	}

	bounds := img.Bounds()
	content.WriteString(fmt.Sprintf("format: %s\n", strings.ToUpper(format)))
	content.WriteString(fmt.Sprintf("dimensions: %dx%d pixels\n", bounds.Dx(), bounds.Dy()))

	// Reset file pointer for EXIF reading
	file.Seek(0, 0)

	// Try to extract EXIF metadata
	exifData, err := exif.Decode(file)
	if err == nil {
		content.WriteString("\nEXIF metadata:\n")

		// Common EXIF tags to extract
		exifTags := []struct {
			name string
			tag  exif.FieldName
		}{
			{"camera make", exif.Make},
			{"camera model", exif.Model},
			{"date time", exif.DateTime},
			{"date time original", exif.DateTimeOriginal},
			{"date time digitized", exif.DateTimeDigitized},
			{"software", exif.Software},
			{"artist", exif.Artist},
			{"copyright", exif.Copyright},
			{"image description", exif.ImageDescription},
			{"user comment", exif.UserComment},
			{"orientation", exif.Orientation},
			{"x resolution", exif.XResolution},
			{"y resolution", exif.YResolution},
			{"resolution unit", exif.ResolutionUnit},
			{"flash", exif.Flash},
			{"focal length", exif.FocalLength},
			{"exposure time", exif.ExposureTime},
			{"f number", exif.FNumber},
			{"iso", exif.ISOSpeedRatings},
			{"white balance", exif.WhiteBalance},
			{"gps latitude", exif.GPSLatitude},
			{"gps longitude", exif.GPSLongitude},
			{"gps altitude", exif.GPSAltitude},
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
							content.WriteString(fmt.Sprintf("  gps coordinates: %.6f, %.6f\n", latDeg, lonDeg))
						}
					}
				}
			}
		}
	} else {
		content.WriteString("\nno EXIF metadata found or failed to read EXIF data\n")
	}

	// Color analysis - sample some pixels to get dominant colors
	content.WriteString(fmt.Sprintf("\nimage properties:\n"))
	content.WriteString(fmt.Sprintf("  color mode: %T\n", img.ColorModel()))
	content.WriteString(fmt.Sprintf("  aspect ratio: %.2f:1\n", float64(bounds.Dx())/float64(bounds.Dy())))

	megapixels := float64(bounds.Dx()*bounds.Dy()) / 1000000.0
	if megapixels > 1.0 {
		content.WriteString(fmt.Sprintf("  megapixels: %.1f MP\n", megapixels))
	} else {
		content.WriteString(fmt.Sprintf("  resolution: %.0f K pixels\n", megapixels*1000))
	}

	// Extract text using OCR
	content.WriteString("\n" + strings.Repeat("=", 50) + "\n")
	content.WriteString("text extraction (OCR):\n")
	content.WriteString(strings.Repeat("=", 50) + "\n\n")

	extractedText, err := t.extractTextFromImage(filePath)
	if err != nil {
		content.WriteString(fmt.Sprintf("OCR error: %v\n", err))
	} else if strings.TrimSpace(extractedText) == "" {
		content.WriteString("no text detected in the image.\n")
	} else {
		content.WriteString("extracted text:\n")
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

	// Convert targetDir to absolute path for comparison
	absTargetDir, err := filepath.Abs(targetDir)
	if err != nil {
		return nil, fmt.Errorf("failed to get absolute path: %v", err)
	}

	// Check if this directory should be loaded shallowly
	isShallow := config.IsShallowLoadDir(t.config, absTargetDir)

	err = filepath.WalkDir(targetDir, func(path string, d fs.DirEntry, err error) error {
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
	fzfOptions := append([]string{">none"}, allFiles...)

	// Use fzf to let user exclude files/directories
	excludedItems, err := t.FzfMultiSelect(fzfOptions, "exclude from dump (tab=multi): ")
	if err != nil {
		return "", fmt.Errorf("failed to get exclusions: %v", err)
	}

	// Filter out the NONE option if selected
	var filteredExclusions []string
	for _, item := range excludedItems {
		if !strings.HasPrefix(item, ">none") {
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
	fzfOptions := append([]string{">none"}, allFiles...)

	// Use CLI-specific fzf that detects cancellation
	excludedItems, err := t.FzfMultiSelectForCLI(fzfOptions, "exclude from dump (tab=multi): ")
	if err != nil {
		return "", fmt.Errorf("failed to get exclusions: %v", err)
	}

	// Filter out the NONE option if selected
	var filteredExclusions []string
	for _, item := range excludedItems {
		if !strings.HasPrefix(item, ">none") {
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
		".pdf": true, ".docx": true, ".odt": true, ".rtf": true, ".xlsx": true, ".csv": true,
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

// generateCodeDumpFromDir creates the final codedump string from a specific directory
func (t *Terminal) generateCodeDumpFromDir(files []string, sourceDir string) (string, error) {
	var result strings.Builder

	result.WriteString("=== Code Dump ===\n\n")
	result.WriteString(fmt.Sprintf("generated from directory: %s\n", sourceDir))
	result.WriteString(fmt.Sprintf("total files: %d\n\n", len(files)))

	for _, file := range files {
		// Build full path for reading
		fullPath := filepath.Join(sourceDir, file)

		// Check if this is a supported file type that we can process
		ext := strings.ToLower(filepath.Ext(file))
		// Image files are excluded from codedump, only document types are processed as special files.
		supportedTypes := []string{".pdf", ".docx", ".odt", ".rtf", ".xlsx", ".csv"}
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

// scrapeURL scrapes content from a single URL (with loading animation)
func (t *Terminal) scrapeURL(urlStr string) (string, error) {
	// Show loading animation
	done := make(chan bool)
	go t.ShowLoadingAnimation("Scraping...", done)

	content, err := t.scrapeURLInternal(urlStr)

	// Stop loading animation
	done <- true

	return content, err
}

// scrapeURLInternal scrapes content from a single URL without animation
func (t *Terminal) scrapeURLInternal(urlStr string) (string, error) {
	// Clean any shell escapes from the URL
	cleanedURL := t.cleanURL(urlStr)

	var result strings.Builder
	result.WriteString(fmt.Sprintf("=== %s ===\n\n", cleanedURL))

	var scrapeErr error
	if t.isYouTubeURL(cleanedURL) {
		// YouTube scraping with yt-dlp
		content, err := t.scrapeYouTube(cleanedURL)
		if err != nil {
			scrapeErr = fmt.Errorf("failed to scrape YouTube URL: %w", err)
		} else {
			result.WriteString(content)
		}
	} else {
		// Regular web scraping with curl + lynx
		content, err := t.scrapeWeb(cleanedURL)
		if err != nil {
			scrapeErr = fmt.Errorf("failed to scrape URL: %w", err)
		} else {
			result.WriteString(content)
		}
	}

	if scrapeErr != nil {
		return "", scrapeErr
	}

	result.WriteString("\n")
	return result.String(), nil
}

// scrapeWeb scrapes regular web pages using native Go http and html parsing.
func (t *Terminal) scrapeWeb(urlStr string) (string, error) {
	client := &http.Client{
		Timeout: 30 * time.Second,
	}
	req, err := http.NewRequest("GET", urlStr, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}
	// Set a user-agent to mimic a browser, as some sites block default Go user-agent
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36")

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to fetch URL: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("failed to fetch URL: status code %d", resp.StatusCode)
	}

	return t.textContentFromHTML(resp.Body)
}

// textContentFromHTML extracts readable text from an HTML document body.
func (t *Terminal) textContentFromHTML(body io.Reader) (string, error) {
	doc, err := html.Parse(body)
	if err != nil {
		return "", fmt.Errorf("failed to parse HTML: %w", err)
	}

	var sb strings.Builder
	var traverse func(*html.Node)
	traverse = func(n *html.Node) {
		// Skip unwanted tags entirely
		if n.Type == html.ElementNode {
			switch n.Data {
			case "script", "style", "nav", "header", "footer", "aside":
				return
			}
		}

		if n.Type == html.TextNode {
			// Add text content, cleaning up whitespace
			trimmed := strings.TrimSpace(n.Data)
			if len(trimmed) > 0 {
				sb.WriteString(trimmed)
				sb.WriteString(" ") // Add space after text nodes
			}
		}

		for c := n.FirstChild; c != nil; c = c.NextSibling {
			traverse(c)
		}

		// Add a newline after block-level elements for better readability
		if n.Type == html.ElementNode {
			switch n.Data {
			case "p", "div", "h1", "h2", "h3", "h4", "h5", "h6", "li", "br", "tr":
				sb.WriteString("\n")
			}
		}
	}

	traverse(doc)

	// Post-process the text to clean up excessive whitespace and newlines
	text := sb.String()
	text = regexp.MustCompile(`\s+`).ReplaceAllString(text, " ")
	text = regexp.MustCompile(`(\s*\n\s*)+`).ReplaceAllString(text, "\n")

	return strings.TrimSpace(text), nil
}

// scrapeYouTube scrapes YouTube videos using yt-dlp
func (t *Terminal) scrapeYouTube(urlStr string) (string, error) {
	var result strings.Builder

	// Get metadata
	result.WriteString("--- Metadata ---\n")
	metadataCmd := exec.Command("yt-dlp", "-j", urlStr)
	metadataOutput, err := metadataCmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to get YouTube metadata: %w", err)
	}

	// Parse key fields from JSON (simple parsing without jq)
	metadata := string(metadataOutput)
	result.WriteString(t.parseYouTubeMetadata(metadata))

	// Get subtitles
	result.WriteString("\n--- Subtitles ---\n")

	tempDir, err := config.GetTempDir()
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
		result.WriteString(fmt.Sprintf("title: %s\n", title))
	}
	if duration := extractField("duration"); duration != "" {
		result.WriteString(fmt.Sprintf("duration: %s seconds\n", duration))
	}
	if viewCount := extractField("view_count"); viewCount != "" {
		result.WriteString(fmt.Sprintf("view count: %s\n", viewCount))
	}
	if uploader := extractField("uploader"); uploader != "" {
		result.WriteString(fmt.Sprintf("uploader: %s\n", uploader))
	}
	if uploadDate := extractField("upload_date"); uploadDate != "" {
		result.WriteString(fmt.Sprintf("upload date: %s\n", uploadDate))
	}
	if description := extractField("description"); description != "" {
		// Truncate description if too long
		if len(description) > 500 {
			description = description[:500] + "..."
		}
		result.WriteString(fmt.Sprintf("description: %s\n", description))
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

		// Show loading animation for each URL
		done := make(chan bool)
		go t.ShowLoadingAnimation("Scraping...", done)

		content, err := t.scrapeURLInternal(urlStr)

		// Stop loading animation
		done <- true

		if err != nil {
			result.WriteString(fmt.Sprintf("Error scraping %s: %v\n", urlStr, err))
			continue
		}

		result.WriteString(content)
	}

	return result.String(), nil
}

// WebSearch performs a web search using the Brave Search API
func (t *Terminal) WebSearch(query string) (string, error) {
	apiKey := os.Getenv("BRAVE_API_KEY")
	if apiKey == "" {
		return "", fmt.Errorf("the BRAVE_API_KEY environment variable is not set")
	}

	// Show loading animation
	done := make(chan bool)
	go t.ShowLoadingAnimation("Searching...", done)

	req, err := http.NewRequest("GET", "https://api.search.brave.com/res/v1/web/search", nil)
	if err != nil {
		done <- true
		return "", fmt.Errorf("failed to create search request: %w", err)
	}

	q := req.URL.Query()
	q.Add("q", query)
	q.Add("count", fmt.Sprintf("%d", t.config.NumSearchResults))
	q.Add("country", t.config.SearchCountry)
	q.Add("search_lang", t.config.SearchLang)
	req.URL.RawQuery = q.Encode()

	req.Header.Set("X-Subscription-Token", apiKey)

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		done <- true
		return "", fmt.Errorf("failed to perform search: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		done <- true
		return "", fmt.Errorf("search request failed with status: %s", resp.Status)
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		done <- true
		return "", fmt.Errorf("failed to read search response: %w", err)
	}

	var braveResult BraveSearchResult
	if err := json.Unmarshal(body, &braveResult); err != nil {
		done <- true
		return "", fmt.Errorf("failed to parse search results: %w", err)
	}

	done <- true

	if len(braveResult.Web.Results) == 0 {
		noResultsMsg := fmt.Sprintf("No search results found for: %s\n", query)
		if t.config.ShowSearchResults {
			fmt.Print(noResultsMsg)
		}
		return noResultsMsg, nil
	}

	formatted := t.formatBraveSearchResults(braveResult.Web.Results, query)

	if t.config.ShowSearchResults {
		fmt.Print(formatted)
	}

	return formatted, nil
}

// BraveSearchResult represents the top-level structure of the Brave Search API response
type BraveSearchResult struct {
	Web struct {
		Results []BraveWebResult `json:"results"`
	} `json:"web"`
}

// BraveWebResult represents a single web result from the Brave Search API
type BraveWebResult struct {
	Title       string `json:"title"`
	URL         string `json:"url"`
	Description string `json:"description"`
}

// formatBraveSearchResults formats the search results from Brave API
func (t *Terminal) formatBraveSearchResults(results []BraveWebResult, query string) string {
	var result strings.Builder

	if len(results) == 0 {
		result.WriteString(fmt.Sprintf("no search results found for: %s\n", query))
		return result.String()
	}

	for i, searchResult := range results {
		if searchResult.Title != "" && searchResult.URL != "" {
			if t.config.IsPipedOutput {
				result.WriteString(fmt.Sprintf("%d) %s\n", i+1, searchResult.Title))
				result.WriteString(fmt.Sprintf("%s\n", searchResult.URL))
				if searchResult.Description != "" {
					result.WriteString(fmt.Sprintf("%s\n", searchResult.Description))
				}
			} else {
				result.WriteString(fmt.Sprintf("\033[93m%d) \033[93m%s\033[0m\n", i+1, searchResult.Title))
				result.WriteString(fmt.Sprintf("\033[95m%s\033[0m\n", searchResult.URL))
				if searchResult.Description != "" {
					result.WriteString(fmt.Sprintf("\033[92m%s\033[0m\n", searchResult.Description))
				}
			}
		}
	}

	return result.String()
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

	// Ask for copy mode
	copyMode, err := t.FzfSelect([]string{"auto copy", "manual copy"}, "select copy mode: ")
	if err != nil {
		return fmt.Errorf("selection cancelled or failed: %v", err)
	}

	if copyMode == "auto copy" {
		return t.copyResponsesAuto(chatHistory)
	}

	if copyMode == "" {
		return nil // User cancelled
	}

	// Manual copy mode - proceed with original logic
	return t.copyResponsesManual(chatHistory)
}

// copyResponsesAuto automatically extracts code blocks from selected chat entries
func (t *Terminal) copyResponsesAuto(chatHistory []types.ChatHistory) error {
	// Create list of chat entries for fzf selection (same format as !e)
	var items []string
	var chatEntries []types.ChatHistory

	// Iterate in reverse order (newest to oldest)
	for i := len(chatHistory) - 1; i >= 1; i-- {
		entry := chatHistory[i]
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
		return fmt.Errorf("no chat entries available")
	}

	// Use fzf for selection
	selectedItems, err := t.FzfMultiSelect(items, "select entries to copy (tab=multi): ")
	if err != nil {
		return fmt.Errorf("selection failed: %w", err)
	}

	if len(selectedItems) == 0 {
		t.PrintInfo("no entries selected")
		return nil
	}

	// Parse selected indices and extract code blocks
	type ExtractedSnippet struct {
		Content  string
		Language string
	}
	var snippets []ExtractedSnippet
	codeBlockRegex := regexp.MustCompile("(?s)```([a-zA-Z0-9]*)\n(.*?)\n```")

	for _, item := range selectedItems {
		var index int
		parts := strings.SplitN(item, ":", 2)
		if len(parts) < 2 {
			continue
		}
		_, err := fmt.Sscanf(parts[0], "%d", &index)
		if err != nil {
			continue
		}

		// Find the entry
		for _, entry := range chatEntries {
			if entry.Time == chatHistory[index].Time && entry.Bot != "" {
				matches := codeBlockRegex.FindAllStringSubmatch(entry.Bot, -1)
				for _, match := range matches {
					language := match[1]
					if language == "" {
						language = "text"
					}
					content := match[2]
					snippets = append(snippets, ExtractedSnippet{Content: content, Language: language})
				}
				break
			}
		}
	}

	if len(snippets) == 0 {
		return fmt.Errorf("no code blocks found in selected entries")
	}

	// Combine snippets
	var combinedContent strings.Builder
	for i, snippet := range snippets {
		if i > 0 {
			combinedContent.WriteString("\n\n")
		}
		combinedContent.WriteString(snippet.Content)
	}

	finalContent := combinedContent.String()

	// Copy to clipboard
	err = t.CopyToClipboard(finalContent)
	if err != nil {
		return fmt.Errorf("failed to copy to clipboard: %w", err)
	}

	blockWord := "code block"
	if len(snippets) > 1 {
		blockWord = "code blocks"
	}
	fmt.Printf("\033[93madded %d %s to clipboard\033[0m\n", len(snippets), blockWord)
	return nil
}

// copyResponsesManual allows user to manually select responses to copy
func (t *Terminal) copyResponsesManual(chatHistory []types.ChatHistory) error {
	// Create list of responses for fzf selection (same format as !e)
	var responseOptions []string
	var responseMap = make(map[string]types.ChatHistory)

	// Iterate in reverse order (newest to oldest)
	for i := len(chatHistory) - 1; i >= 1; i-- {
		entry := chatHistory[i]
		if entry.User != "" || entry.Bot != "" {
			// Create preview for fzf
			userPreview := strings.Split(entry.User, "\n")[0]
			if len(userPreview) > 60 {
				userPreview = userPreview[:60] + "..."
			}

			timestamp := time.Unix(entry.Time, 0).Format("2006-01-02 15:04:05")
			optionText := fmt.Sprintf("%d: %s - %s", i, timestamp, userPreview)
			responseOptions = append(responseOptions, optionText)
			responseMap[optionText] = entry
		}
	}

	if len(responseOptions) == 0 {
		return fmt.Errorf("no responses found in chat history")
	}

	// Use fzf for multi-selection
	selected, err := t.FzfMultiSelect(responseOptions, "select responses to copy (tab=multi): ")
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

	responseWord := "response"
	if len(selected) > 1 {
		responseWord = "responses"
	}
	fmt.Printf("\033[93madded %d %s to clipboard\033[0m\n", len(selected), responseWord)
	return nil
}

// openEditorWithContent opens the preferred editor with given content and returns the edited result
func (t *Terminal) openEditorWithContent(content string) (string, error) {
	// Create temporary file
	tempDir, err := config.GetTempDir()
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

	// Open editor with fallback
	if err := t.runEditorWithFallback(tempFile.Name()); err != nil {
		return "", fmt.Errorf("editor command failed: %w", err)
	}

	// Read edited content
	editedBytes, err := ioutil.ReadFile(tempFile.Name())
	if err != nil {
		return "", fmt.Errorf("failed to read edited content: %w", err)
	}

	return string(editedBytes), nil
}

// runEditorWithFallback tries to run the user's preferred editor, then falls back to common editors.
func (t *Terminal) runEditorWithFallback(filePath string) error {
	return RunEditorWithFallback(t.config, filePath)
}

// ExtractURLsFromText extracts all URLs from a given text using regex
func (t *Terminal) ExtractURLsFromText(text string) []string {
	// URL regex pattern
	urlRegex := regexp.MustCompile(`https?://[^\s<>"{}|\\^` + "`" + `\[\]]+`)
	matches := urlRegex.FindAllString(text, -1)

	// Remove duplicates
	seen := make(map[string]bool)
	var uniqueURLs []string
	for _, url := range matches {
		if url != "" && !seen[url] {
			uniqueURLs = append(uniqueURLs, url)
			seen[url] = true
		}
	}

	return uniqueURLs
}

// ExtractURLsFromChatHistory scans the entire chat history and extracts all unique URLs
func (t *Terminal) ExtractURLsFromChatHistory(chatHistory []types.ChatHistory) []string {
	var allURLs []string
	seen := make(map[string]bool)

	for _, entry := range chatHistory {
		// Extract URLs from user messages
		userURLs := t.ExtractURLsFromText(entry.User)
		for _, url := range userURLs {
			if !seen[url] {
				allURLs = append(allURLs, url)
				seen[url] = true
			}
		}

		// Extract URLs from bot messages
		botURLs := t.ExtractURLsFromText(entry.Bot)
		for _, url := range botURLs {
			if !seen[url] {
				allURLs = append(allURLs, url)
				seen[url] = true
			}
		}
	}

	return allURLs
}

// ExtractURLsFromMessages scans all chat messages and extracts all unique URLs
func (t *Terminal) ExtractURLsFromMessages(messages []types.ChatMessage) []string {
	var allURLs []string
	seen := make(map[string]bool)

	for _, message := range messages {
		// Extract URLs from message content
		urls := t.ExtractURLsFromText(message.Content)
		for _, url := range urls {
			if !seen[url] {
				allURLs = append(allURLs, url)
				seen[url] = true
			}
		}
	}

	return allURLs
}

// ExtractSentencesFromText extracts all sentences from a given text
func (t *Terminal) ExtractSentencesFromText(text string) []string {
	// Split by sentence terminators
	sentenceRegex := regexp.MustCompile(`[.!?]+\s+`)
	rawSentences := sentenceRegex.Split(text, -1)

	var sentences []string
	for _, sentence := range rawSentences {
		// Strip whitespace (like Python's strip())
		cleaned := strings.TrimSpace(sentence)

		// Skip empty or whitespace-only lines
		if cleaned == "" {
			continue
		}

		// Only keep sentences that are at least 10 characters and not too long
		if len(cleaned) >= 10 && len(cleaned) <= 200 {
			sentences = append(sentences, cleaned)
		}
	}

	return sentences
}

// ExtractSentencesFromChatHistory scans the entire chat history and extracts all unique sentences
func (t *Terminal) ExtractSentencesFromChatHistory(chatHistory []types.ChatHistory, messages []types.ChatMessage) []string {
	var allSentences []string
	seen := make(map[string]bool)

	// Extract from chat history
	for _, entry := range chatHistory {
		// Extract from user messages
		userSentences := t.ExtractSentencesFromText(entry.User)
		for _, sentence := range userSentences {
			if !seen[sentence] {
				allSentences = append(allSentences, sentence)
				seen[sentence] = true
			}
		}

		// Extract from bot messages
		botSentences := t.ExtractSentencesFromText(entry.Bot)
		for _, sentence := range botSentences {
			if !seen[sentence] {
				allSentences = append(allSentences, sentence)
				seen[sentence] = true
			}
		}
	}

	// Extract from messages
	for _, message := range messages {
		sentences := t.ExtractSentencesFromText(message.Content)
		for _, sentence := range sentences {
			if !seen[sentence] {
				allSentences = append(allSentences, sentence)
				seen[sentence] = true
			}
		}
	}

	return allSentences
}
