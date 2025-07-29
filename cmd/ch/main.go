package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/MehmetMHY/ch/internal/chat"
	"github.com/MehmetMHY/ch/internal/config"
	"github.com/MehmetMHY/ch/internal/platform"
	"github.com/MehmetMHY/ch/internal/ui"
	"github.com/MehmetMHY/ch/pkg/types"
	"github.com/chzyer/readline"
	"github.com/google/uuid"
)

func main() {
	// initialize application state
	state := config.InitializeAppState()

	// initialize components
	terminal := ui.NewTerminal(state.Config)
	chatManager := chat.NewManager(state)
	platformManager := platform.NewManager(state.Config)

	// parse command line arguments
	var (
		helpFlag       = flag.Bool("h", false, "Show help")
		codedumpFlag   = flag.String("d", "", "Generate codedump file (optionally specify directory path)")
		platformFlag   = flag.String("p", "", "Switch platform (leave empty for interactive selection)")
		modelFlag      = flag.String("m", "", "Specify model to use")
		exportCodeFlag = flag.Bool("e", false, "Export code blocks from the last response")
	)

	flag.Parse()
	remainingArgs := flag.Args()

	// handle help flag
	if *helpFlag {
		terminal.ShowHelp()
		return
	}

	// handle codedump flag
	if flag.Lookup("d").Value.String() != flag.Lookup("d").DefValue {
		targetDir := *codedumpFlag
		if targetDir == "" {
			targetDir = "."
		}

		// Validate directory
		if !isValidCodedumpDir(targetDir) {
			if targetDir != "." {
				terminal.PrintError("Invalid directory path or permission denied")
				return
			}
		}

		codedump, err := terminal.CodeDumpFromDirForCLI(targetDir)
		if err != nil {
			// Check if user cancelled (Ctrl-C/Ctrl-D during fzf)
			if strings.Contains(err.Error(), "user cancelled") {
				return // Exit silently without creating file
			}
			terminal.PrintError(fmt.Sprintf("Error generating codedump: %v", err))
			return
		}

		filename := fmt.Sprintf("code_dump_%s.txt", uuid.New().String())
		err = os.WriteFile(filename, []byte(codedump), 0644)
		if err != nil {
			terminal.PrintError(fmt.Sprintf("Error writing codedump file: %v", err))
			return
		}

		fmt.Println(filename)
		return
	}

	// handle export code flag
	if *exportCodeFlag {
		err := handleExportCodeBlocks(chatManager, terminal)
		if err != nil {
			terminal.PrintError(fmt.Sprintf("Error exporting code blocks: %v", err))
		}
		return
	}

	// handle platform switching
	if *platformFlag != "" {
		result, err := platformManager.SelectPlatform(*platformFlag, *modelFlag, terminal.FzfSelect)
		if err != nil {
			terminal.PrintError(fmt.Sprintf("%v", err))
			return
		}
		if result != nil {
			chatManager.SetCurrentPlatform(result["platform_name"].(string))
			chatManager.SetCurrentModel(result["picked_model"].(string))
			terminal.PrintPlatformSwitch(result["platform_name"].(string), result["picked_model"].(string))
		}
	}

	// handle model switching
	if *modelFlag != "" && *platformFlag == "" {
		chatManager.SetCurrentModel(*modelFlag)
	}

	// initialize platform client
	err := platformManager.Initialize()
	if err != nil {
		terminal.PrintError(fmt.Sprintf("Failed to initialize client: %v", err))
		return
	}

	// set up signal handling
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		for range sigChan {
			if state.IsStreaming && state.StreamingCancel != nil {
				fmt.Print("\r\033[K")
				state.StreamingCancel()
			} else if state.IsExecutingCommand && state.CommandCancel != nil {
				fmt.Print("\r\033[K")
				state.CommandCancel()
			} else {
				os.Exit(0)
			}
		}
	}()

	// handle direct query mode
	if len(remainingArgs) > 0 {
		query := strings.Join(remainingArgs, " ")
		err := processDirectQuery(query, chatManager, platformManager, terminal, state, *exportCodeFlag)
		if err != nil {
			terminal.PrintError(fmt.Sprintf("%v", err))
		}
		return
	}

	// interactive mode
	runInteractiveMode(chatManager, platformManager, terminal, state)
}

func processDirectQuery(query string, chatManager *chat.Manager, platformManager *platform.Manager, terminal *ui.Terminal, state *types.AppState, exportCode bool) error {
	if handleSpecialCommands(query, chatManager, platformManager, terminal, state) {
		return nil
	}

	chatManager.AddUserMessage(query)

	response, err := platformManager.SendChatRequest(chatManager.GetMessages(), chatManager.GetCurrentModel(), &state.StreamingCancel, &state.IsStreaming)
	if err != nil {
		if err.Error() == "request was interrupted" {
			chatManager.RemoveLastUserMessage()
			return nil
		}
		return err
	}

	chatManager.AddAssistantMessage(response)
	chatManager.AddToHistory(query, response)

	// Export code blocks if -e flag was used
	if exportCode {
		filePaths, exportErr := chatManager.ExportCodeBlocks()
		if exportErr != nil {
			terminal.PrintError(fmt.Sprintf("Error exporting code blocks: %v", exportErr))
		} else if len(filePaths) > 0 {
			for _, filePath := range filePaths {
				fmt.Println(filePath)
			}
		}
	}

	return nil
}

func runInteractiveMode(chatManager *chat.Manager, platformManager *platform.Manager, terminal *ui.Terminal, state *types.AppState) {
	rl, err := readline.NewEx(&readline.Config{
		Prompt: "\033[94mUser: \033[0m",
	})
	if err != nil {
		panic(err)
	}
	defer rl.Close()

	for {
		line, err := rl.Readline()
		if err != nil { // io.EOF, readline.ErrInterrupt
			break
		}
		input := strings.TrimSpace(line)

		if input == "" {
			continue
		}

		if handleSpecialCommands(input, chatManager, platformManager, terminal, state) {
			continue
		}

		chatManager.AddUserMessage(input)

		// Start loading animation for non-streaming models
		var loadingDone chan bool
		if platformManager.IsReasoningModel(chatManager.GetCurrentModel()) {
			loadingDone = make(chan bool)
			go terminal.ShowLoadingAnimation("Thinking", loadingDone)
		}

		response, err := platformManager.SendChatRequest(chatManager.GetMessages(), chatManager.GetCurrentModel(), &state.StreamingCancel, &state.IsStreaming)

		// Stop loading animation if it was started
		if loadingDone != nil {
			loadingDone <- true
		}

		if err != nil {
			if err.Error() == "request was interrupted" {
				chatManager.RemoveLastUserMessage()
				continue
			}
			terminal.PrintError(fmt.Sprintf("%v", err))
			continue
		}

		// Print response for non-streaming models
		if platformManager.IsReasoningModel(chatManager.GetCurrentModel()) {
			fmt.Printf("\033[92m%s\033[0m\n", response)
		}

		chatManager.AddAssistantMessage(response)
		chatManager.AddToHistory(input, response)
	}
}

func handleSpecialCommands(input string, chatManager *chat.Manager, platformManager *platform.Manager, terminal *ui.Terminal, state *types.AppState) bool {
	config := state.Config

	switch {
	case input == config.ExitKey:
		os.Exit(0)
		return true

	case input == config.HelpKey || input == "help":
		selectedCommand := terminal.ShowHelpFzf()
		if selectedCommand != "" {
			// Recursively handle the selected command
			return handleSpecialCommands(selectedCommand, chatManager, platformManager, terminal, state)
		}
		return true

	case input == config.ClearHistory:
		chatManager.ClearHistory()
		terminal.PrintInfo("Chat history cleared.")
		return true

	case input == config.ModelSwitch:
		models, err := platformManager.ListModels()
		if err != nil {
			terminal.PrintError(fmt.Sprintf("Error fetching models: %v", err))
			return true
		}

		selectedModel, err := terminal.FzfSelect(models, "Select a model: ")
		if err != nil {
			terminal.PrintError(fmt.Sprintf("Error selecting model: %v", err))
			return true
		}

		if selectedModel != "" {
			oldModel := chatManager.GetCurrentModel()
			chatManager.SetCurrentModel(selectedModel)
			terminal.PrintModelSwitch(selectedModel)
			chatManager.AddToHistory(config.ModelSwitch, fmt.Sprintf("Switched model from %s to %s", oldModel, selectedModel))
		}
		return true

	case strings.HasPrefix(input, config.ModelSwitch+" "):
		modelName := strings.TrimPrefix(input, config.ModelSwitch+" ")
		oldModel := chatManager.GetCurrentModel()
		chatManager.SetCurrentModel(modelName)
		terminal.PrintModelSwitch(modelName)
		chatManager.AddToHistory(fmt.Sprintf("%s %s", config.ModelSwitch, modelName), fmt.Sprintf("Switched model from %s to %s", oldModel, modelName))
		return true

	case input == config.PlatformSwitch:
		result, err := platformManager.SelectPlatform("", "", terminal.FzfSelect)
		if err != nil {
			terminal.PrintError(fmt.Sprintf("%v", err))
		} else if result != nil {
			oldPlatform := chatManager.GetCurrentPlatform()
			oldModel := chatManager.GetCurrentModel()
			chatManager.SetCurrentPlatform(result["platform_name"].(string))
			chatManager.SetCurrentModel(result["picked_model"].(string))
			err = platformManager.Initialize()
			if err != nil {
				terminal.PrintError(fmt.Sprintf("Error initializing client: %v", err))
			} else {
				terminal.PrintPlatformSwitch(result["platform_name"].(string), result["picked_model"].(string))
				chatManager.AddToHistory(config.PlatformSwitch, fmt.Sprintf("Switched from %s/%s to %s/%s", oldPlatform, oldModel, result["platform_name"].(string), result["picked_model"].(string)))
			}
		}
		return true

	case strings.HasPrefix(input, config.PlatformSwitch+" "):
		platformName := strings.TrimPrefix(input, config.PlatformSwitch+" ")
		result, err := platformManager.SelectPlatform(platformName, "", terminal.FzfSelect)
		if err != nil {
			terminal.PrintError(fmt.Sprintf("%v", err))
		} else if result != nil {
			oldPlatform := chatManager.GetCurrentPlatform()
			oldModel := chatManager.GetCurrentModel()
			chatManager.SetCurrentPlatform(result["platform_name"].(string))
			chatManager.SetCurrentModel(result["picked_model"].(string))
			err = platformManager.Initialize()
			if err != nil {
				terminal.PrintError(fmt.Sprintf("Error initializing client: %v", err))
			} else {
				terminal.PrintPlatformSwitch(result["platform_name"].(string), result["picked_model"].(string))
				chatManager.AddToHistory(fmt.Sprintf("%s %s", config.PlatformSwitch, platformName), fmt.Sprintf("Switched from %s/%s to %s/%s", oldPlatform, oldModel, result["platform_name"].(string), result["picked_model"].(string)))
			}
		}
		return true

	case input == config.LoadFiles:
		return handleFileLoad(chatManager, terminal, state)

	case input == config.CodeDump:
		return handleCodeDump(chatManager, terminal, state)

	case input == config.ShellRecord:
		return handleShellRecord(chatManager, terminal, state)

	case input == config.EditorInput:
		userInput, err := chatManager.HandleTerminalInput()
		if err != nil {
			terminal.PrintError(fmt.Sprintf("%v", err))
			return true
		}

		fmt.Printf("\033[94m> %s\033[0m\n", strings.ReplaceAll(userInput, "\n", "\n> "))

		chatManager.AddUserMessage(userInput)

		var loadingDone chan bool
		if platformManager.IsReasoningModel(chatManager.GetCurrentModel()) {
			loadingDone = make(chan bool)
			go terminal.ShowLoadingAnimation("Thinking", loadingDone)
		}

		response, err := platformManager.SendChatRequest(chatManager.GetMessages(), chatManager.GetCurrentModel(), &state.StreamingCancel, &state.IsStreaming)

		if loadingDone != nil {
			loadingDone <- true
		}

		if err != nil {
			if err.Error() == "request was interrupted" {
				chatManager.RemoveLastUserMessage()
				return true
			}
			terminal.PrintError(fmt.Sprintf("%v", err))
			return true
		}

		if platformManager.IsReasoningModel(chatManager.GetCurrentModel()) {
			fmt.Printf("\033[92m%s\033[0m\n", response)
		}

		chatManager.AddAssistantMessage(response)
		chatManager.AddToHistory(userInput, response)
		return true

	case input == config.ExportChat:
		err := handleExportChatInteractive(chatManager, terminal, state)
		if err != nil {
			terminal.PrintError(fmt.Sprintf("Error exporting chat: %v", err))
		}
		return true

	case input == config.Backtrack:
		backtrackedCount, err := chatManager.BacktrackHistory()
		if err != nil {
			terminal.PrintError(fmt.Sprintf("Error backtracking: %v", err))
		} else {
			terminal.PrintInfo(fmt.Sprintf("Backtracked by %d.", backtrackedCount))
		}
		return true

	case input == config.MultiLine:
		var lines []string
		terminal.PrintInfo("Multi-line mode (end with '\\' on a new line).")

		// Create a new readline instance for multi-line input
		multiLineRl, err := readline.NewEx(&readline.Config{
			Prompt:      "... ",
			HistoryFile: "/dev/null", // Disable history for multi-line
		})
		if err != nil {
			terminal.PrintError(fmt.Sprintf("Error creating multi-line input: %v", err))
			return true
		}
		defer multiLineRl.Close()

		for {
			line, err := multiLineRl.Readline()
			if err != nil {
				if err == readline.ErrInterrupt || err == io.EOF {
					fmt.Println() // Move to the next line for a clean prompt
					return true   // Exit silently
				}
				break // Exit for other errors
			}
			if line == config.MultiLine {
				break
			}
			lines = append(lines, line)
		}

		fullInput := strings.Join(lines, "\n")
		if strings.TrimSpace(fullInput) == "" {
			return true
		}

		chatManager.AddUserMessage(fullInput)

		// Start loading animation for non-streaming models
		var loadingDone chan bool
		if platformManager.IsReasoningModel(chatManager.GetCurrentModel()) {
			loadingDone = make(chan bool)
			go terminal.ShowLoadingAnimation("Thinking", loadingDone)
		}

		response, err := platformManager.SendChatRequest(chatManager.GetMessages(), chatManager.GetCurrentModel(), &state.StreamingCancel, &state.IsStreaming)

		// Stop loading animation if it was started
		if loadingDone != nil {
			loadingDone <- true
		}

		if err != nil {
			if err.Error() == "request was interrupted" {
				chatManager.RemoveLastUserMessage()
				return true
			}
			terminal.PrintError(fmt.Sprintf("%v", err))
			return true
		}

		// Print response for non-streaming models
		if platformManager.IsReasoningModel(chatManager.GetCurrentModel()) {
			fmt.Printf("\033[92m%s\033[0m\n", response)
		}

		chatManager.AddAssistantMessage(response)
		chatManager.AddToHistory(fullInput, response)
		return true

	default:
		return false
	}
}

func handleFileLoad(chatManager *chat.Manager, terminal *ui.Terminal, state *types.AppState) bool {
	files, err := terminal.GetCurrentDirFiles()
	if err != nil {
		terminal.PrintError(fmt.Sprintf("Error reading directory: %v", err))
		return true
	}

	if len(files) == 0 {
		terminal.PrintError("No files found in current directory")
		return true
	}

	selections, err := terminal.FzfMultiSelect(files, "Select files/directories: ")
	if err != nil {
		terminal.PrintError(fmt.Sprintf("Error selecting files: %v", err))
		return true
	}

	if len(selections) == 0 {
		return true
	}

	content, err := terminal.LoadFileContent(selections)
	if err != nil {
		terminal.PrintError(fmt.Sprintf("Error loading content: %v", err))
		return true
	}

	if content != "" {
		chatManager.AddUserMessage(content)
	}

	return true
}

func handleCodeDump(chatManager *chat.Manager, terminal *ui.Terminal, state *types.AppState) bool {
	codedump, err := terminal.CodeDump()
	if err != nil {
		terminal.PrintError(fmt.Sprintf("Error generating codedump: %v", err))
		return true
	}

	if codedump != "" {
		chatManager.AddUserMessage(codedump)
	}

	return true
}

func handleShellRecord(chatManager *chat.Manager, terminal *ui.Terminal, state *types.AppState) bool {
	sessionContent, err := terminal.RecordShellSession()
	if err != nil {
		terminal.PrintError(fmt.Sprintf("Error recording shell session: %v", err))
		return true
	}

	if strings.TrimSpace(sessionContent) != "" {
		// Sanitize the content slightly to make it cleaner for the model
		// This removes the "Script started/done" lines.
		lines := strings.Split(sessionContent, "\n")
		var cleanedLines []string
		for _, line := range lines {
			if !strings.HasPrefix(line, "Script started on") && !strings.HasPrefix(line, "Script done on") {
				cleanedLines = append(cleanedLines, line)
			}
		}
		cleanedContent := strings.Join(cleanedLines, "\n")

		// Wrap the content for clarity
		formattedContent := fmt.Sprintf("The user ran the following shell session and here is the output:\n\n---\n%s\n---", cleanedContent)

		chatManager.AddUserMessage(formattedContent)
	} else {
		terminal.PrintInfo("No activity recorded in shell session.")
	}

	return true
}

func isValidCodedumpDir(dirPath string) bool {
	// Never allow root directory
	if dirPath == "/" {
		return false
	}

	// Check if path exists and is a directory
	info, err := os.Stat(dirPath)
	if err != nil {
		return false
	}

	return info.IsDir()
}

func handleExportCodeBlocks(chatManager *chat.Manager, terminal *ui.Terminal) error {
	filePaths, err := chatManager.ExportCodeBlocks()
	if err != nil {
		return err
	}

	if len(filePaths) == 0 {
		terminal.PrintInfo("No code blocks found in the last response")
		return nil
	}

	for _, filePath := range filePaths {
		fmt.Println(filePath)
	}

	return nil
}

func handleExportChatInteractive(chatManager *chat.Manager, terminal *ui.Terminal, state *types.AppState) error {
	filePath, err := chatManager.ExportChatInteractive(terminal)
	if err != nil {
		return err
	}

	if filePath != "" {
		fmt.Println(filePath)
	}

	return nil
}
