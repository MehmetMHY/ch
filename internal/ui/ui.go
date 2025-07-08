package ui

import (
	"fmt"
	"io/fs"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/MehmetMHY/cha-go/pkg/types"
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

// IsTerminal checks if the input is from a terminal
func (t *Terminal) IsTerminal() bool {
	fileInfo, _ := os.Stdin.Stat()
	return (fileInfo.Mode() & os.ModeCharDevice) != 0
}

// ShowHelp displays the help information
func (t *Terminal) ShowHelp() {
	fmt.Println("Cha-Go")
	fmt.Println("\nUsage:")
	fmt.Println("  ./cha-go")
	fmt.Println("  ./cha-go [query]")
	fmt.Println("  ./cha-go -h")
	fmt.Println("  ./cha-go -p [platform]")
	fmt.Println("  ./cha-go -m [model]")
	fmt.Println("  ./cha-go -p [platform] -m [model] [query]")
	fmt.Println("\nExamples:")
	fmt.Println("  ./cha-go -p groq what is AI?")
	fmt.Println("  ./cha-go -p groq -m llama3 what is the goal of life")
	fmt.Println("  ./cha-go -m gpt-4o explain quantum computing")
	fmt.Println("\nAvailable Platforms:")
	fmt.Println("  - openai (default)")
	for name := range t.config.Platforms {
		fmt.Printf("  - %s\n", name)
	}
	fmt.Println("\nInteractive Commands:")
	fmt.Printf("  %s - Exit\n", t.config.ExitKey)
	fmt.Printf("  %s - Switch models\n", t.config.ModelSwitch)
	fmt.Printf("  %s - Terminal input mode\n", t.config.TerminalInput)
	fmt.Printf("  %s - Clear chat history\n", t.config.ClearHistory)
	fmt.Printf("  %s - Export chat to file\n", t.config.ExportChat)
	fmt.Printf("  %s - Show help\n", t.config.HelpKey)
	fmt.Println("  !p - Switch platforms (interactive)")
	fmt.Println("  !p [platform] - Switch to specific platform")
	fmt.Println("  !w [query] - Web search using SearXNG (running on localhost:8080)")
	fmt.Println("  !l - Load files/dirs from current dir")
}

// PrintTitle displays the current session information
func (t *Terminal) PrintTitle() {
	fmt.Printf("\033[93mChatting with %s Model: %s\033[0m\n", strings.ToUpper(t.config.CurrentPlatform), t.config.CurrentModel)
	fmt.Printf("\033[93mCommands:\033[0m\n")
	fmt.Printf("\033[93m  • %s - Exit\033[0m\n", t.config.ExitKey)
	fmt.Printf("\033[93m  • %s - Switch models\033[0m\n", t.config.ModelSwitch)
	fmt.Printf("\033[93m  • !p - Switch platforms\033[0m\n")
	fmt.Printf("\033[93m  • %s - Terminal input\033[0m\n", t.config.TerminalInput)
	fmt.Printf("\033[93m  • %s - Clear history\033[0m\n", t.config.ClearHistory)
	fmt.Printf("\033[93m  • %s - Export chat\033[0m\n", t.config.ExportChat)
	fmt.Printf("\033[93m  • %s - Help\033[0m\n", t.config.HelpKey)
	fmt.Printf("\033[93m  • !w [query] - Web search\033[0m\n")
	fmt.Printf("\033[93m  • !l - Load files/directories\033[0m\n")
}

// ShowLoadingAnimation displays a loading animation
func (t *Terminal) ShowLoadingAnimation(message string, done chan bool) {
	chars := []string{"⣾", "⣽", "⣻", "⢿", "⡿", "⣟", "⣯", "⣷"}
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
	cmd := exec.Command("fzf", "--reverse", "--height=40%", "--border", "--prompt="+prompt)
	cmd.Stdin = strings.NewReader(strings.Join(items, "\n"))

	output, err := cmd.Output()
	if err != nil {
		return "", err
	}

	return strings.TrimSpace(string(output)), nil
}

// FzfMultiSelect provides a fuzzy finder interface for multiple selections
func (t *Terminal) FzfMultiSelect(items []string, prompt string) ([]string, error) {
	cmd := exec.Command("fzf", "--reverse", "--height=40%", "--border", "--prompt="+prompt, "--multi", "--bind=tab:select+down")
	cmd.Stdin = strings.NewReader(strings.Join(items, "\n"))

	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	result := strings.TrimSpace(string(output))
	if result == "" {
		return []string{}, nil
	}

	return strings.Split(result, "\n"), nil
}

// PrintPrompt displays the user prompt
func (t *Terminal) PrintPrompt() {
	if t.IsTerminal() {
		fmt.Print("\033[94mUser: \033[0m")
	}
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
	fmt.Printf("\033[95mSwitched to model: %s\033[0m\n", model)
}

// LoadFileContent loads and returns content from selected files/directories
func (t *Terminal) LoadFileContent(selections []string) (string, error) {
	var contentBuilder strings.Builder

	for _, selection := range selections {
		if selection == "" {
			continue
		}

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

// loadTextFile loads content from a text file
func (t *Terminal) loadTextFile(filePath string) (string, error) {
	content, err := ioutil.ReadFile(filePath)
	if err != nil {
		return "", err
	}

	// Check if file is likely a text file by examining content
	if !t.isTextFile(content) {
		return "", fmt.Errorf("file is not a text file")
	}

	var result strings.Builder
	result.WriteString(fmt.Sprintf("--- File: %s ---\n", filePath))
	result.WriteString(string(content))
	result.WriteString("\n--- End of file ---\n\n")

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
