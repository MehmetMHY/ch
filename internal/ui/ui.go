package ui

import (
	"fmt"
	"os"
	"os/exec"
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
	fmt.Println("Simple Go Chat Client")
	fmt.Println("\nUsage:")
	fmt.Println("  ./cha-go                              # Interactive mode")
	fmt.Println("  ./cha-go [query]                      # Direct query mode")
	fmt.Println("  ./cha-go -h                           # Show this help")
	fmt.Println("  ./cha-go -p [platform]                # Switch platform")
	fmt.Println("  ./cha-go -m [model]                   # Specify model")
	fmt.Println("  ./cha-go -p [platform] -m [model] [query]  # Full command")
	fmt.Println("\nExamples:")
	fmt.Println("  ./cha-go -p groq what is AI?")
	fmt.Println("  ./cha-go -p groq -m llama3 what is the goal of life")
	fmt.Println("  ./cha-go -m gpt-4o explain quantum computing")
	fmt.Println("\nAvailable platforms:")
	fmt.Println("  - openai (default)")
	for name := range t.config.Platforms {
		fmt.Printf("  - %s\n", name)
	}
	fmt.Println("\nInteractive commands:")
	fmt.Printf("  %s - Exit\n", t.config.ExitKey)
	fmt.Printf("  %s - Switch models\n", t.config.ModelSwitch)
	fmt.Printf("  %s - Terminal input mode\n", t.config.TerminalInput)
	fmt.Printf("  %s - Clear chat history\n", t.config.ClearHistory)
	fmt.Printf("  %s - Export chat to file\n", t.config.ExportChat)
	fmt.Printf("  %s - Show help\n", t.config.HelpKey)
	fmt.Println("  !p - Switch platforms (interactive)")
	fmt.Println("  !p [platform] - Switch to specific platform")
	fmt.Println("  !w [query] - Web search using SearXNG (requires SearXNG running on localhost:8080)")
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
