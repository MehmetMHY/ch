package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/MehmetMHY/ch/internal/chat"
	"github.com/MehmetMHY/ch/internal/config"
	"github.com/MehmetMHY/ch/internal/platform"
	"github.com/MehmetMHY/ch/internal/ui"
	"github.com/MehmetMHY/ch/pkg/types"
	"github.com/chzyer/readline"
	"github.com/google/uuid"
	"github.com/tiktoken-go/tokenizer"
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
		tokenFlag      = flag.String("t", "", "Count tokens in file")
		loadFileFlag   = flag.String("l", "", "Load and display file content (supports text, PDF, DOCX, XLSX, CSV)")
	)
	flag.StringVar(tokenFlag, "token", "", "Count tokens in file")

	flag.Parse()
	remainingArgs := flag.Args()

	// Check if input is being piped
	var pipedInput string
	stat, _ := os.Stdin.Stat()
	if (stat.Mode() & os.ModeCharDevice) == 0 {
		// Input is being piped
		pipedBytes, err := io.ReadAll(os.Stdin)
		if err == nil && len(pipedBytes) > 0 {
			pipedInput = string(pipedBytes)
		}
	}

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
			terminal.PrintError(fmt.Sprintf("error generating codedump: %v", err))
			return
		}

		filename := fmt.Sprintf("code_dump_%s.txt", uuid.New().String())
		err = os.WriteFile(filename, []byte(codedump), 0644)
		if err != nil {
			terminal.PrintError(fmt.Sprintf("error writing codedump file: %v", err))
			return
		}

		fmt.Println(filename)
		return
	}

	// handle export code flag
	if *exportCodeFlag {
		err := handleExportCodeBlocks(chatManager, terminal)
		if err != nil {
			terminal.PrintError(fmt.Sprintf("error exporting code blocks: %v", err))
		}
		return
	}

	// handle load file flag
	if *loadFileFlag != "" {
		err := handleLoadFile(*loadFileFlag, terminal)
		if err != nil {
			terminal.PrintError(fmt.Sprintf("error loading file: %v", err))
		}
		return
	}

	// handle token counting flag
	if *tokenFlag != "" {
		err := handleTokenCount(*tokenFlag, *modelFlag, terminal, state)
		if err != nil {
			terminal.PrintError(fmt.Sprintf("error counting tokens: %v", err))
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
		terminal.PrintError(fmt.Sprintf("failed to initialize client: %v", err))
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

	// handle direct query mode (with piped input support)
	if len(remainingArgs) > 0 || pipedInput != "" {
		var query string

		// Build the query from piped input and/or arguments
		if pipedInput != "" && len(remainingArgs) > 0 {
			// Both piped input and arguments: combine them
			// Format: "piped content" + " " + "arguments"
			query = strings.TrimSpace(pipedInput) + " " + strings.Join(remainingArgs, " ")
		} else if pipedInput != "" {
			// Only piped input
			query = strings.TrimSpace(pipedInput)
		} else {
			// Only arguments (backward compatible)
			query = strings.Join(remainingArgs, " ")
		}

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
		filePaths, exportErr := chatManager.ExportCodeBlocks(terminal)
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
		Prompt:          "\033[94mUser: \033[0m",
		InterruptPrompt: "", // Don't show ^C when Ctrl+C is pressed
		EOFPrompt:       "exit",
	})
	if err != nil {
		panic(err)
	}
	defer rl.Close()

	for {
		line, err := rl.Readline()
		if err != nil {
			if err == readline.ErrInterrupt {
				// Ctrl+C pressed - clear input and continue
				fmt.Printf("\033[93mPress Ctrl+D to exit\033[0m\n")
				continue
			}
			// io.EOF (Ctrl+D) or other errors - exit
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
	return handleSpecialCommandsInternal(input, chatManager, platformManager, terminal, state, false)
}

func handleSpecialCommandsInternal(input string, chatManager *chat.Manager, platformManager *platform.Manager, terminal *ui.Terminal, state *types.AppState, fromHelp bool) bool {
	config := state.Config

	switch {
	case input == config.ExitKey:
		os.Exit(0)
		return true

	case input == config.HelpKey || input == "help":
		selectedCommand := terminal.ShowHelpFzf()
		if selectedCommand != "" {
			// Recursively handle the selected command
			return handleSpecialCommandsInternal(selectedCommand, chatManager, platformManager, terminal, state, true)
		}
		return true

	case input == config.ClearHistory:
		chatManager.ClearHistory()
		terminal.PrintInfo("history cleared")
		return true

	case input == config.ModelSwitch:
		models, err := platformManager.ListModels()
		if err != nil {
			terminal.PrintError(fmt.Sprintf("error fetching models: %v", err))
			return true
		}

		selectedModel, err := terminal.FzfSelect(models, "Model: ")
		if err != nil {
			terminal.PrintError(fmt.Sprintf("error selecting model: %v", err))
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
				terminal.PrintError(fmt.Sprintf("error initializing client: %v", err))
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
				terminal.PrintError(fmt.Sprintf("error initializing client: %v", err))
			} else {
				terminal.PrintPlatformSwitch(result["platform_name"].(string), result["picked_model"].(string))
				chatManager.AddToHistory(fmt.Sprintf("%s %s", config.PlatformSwitch, platformName), fmt.Sprintf("Switched from %s/%s to %s/%s", oldPlatform, oldModel, result["platform_name"].(string), result["picked_model"].(string)))
			}
		}
		return true

	case input == config.LoadFiles:
		return handleFileLoad(chatManager, terminal, state, "")

	case strings.HasPrefix(input, config.LoadFiles+" "):
		dirPath := strings.TrimSpace(strings.TrimPrefix(input, config.LoadFiles+" "))
		return handleFileLoad(chatManager, terminal, state, dirPath)

	case input == config.CodeDump:
		return handleCodeDump(chatManager, terminal, state)

	case input == config.ShellRecord:
		return handleShellRecord(chatManager, terminal, state)

	case strings.HasPrefix(input, config.ShellRecord+" "):
		command := strings.TrimPrefix(input, config.ShellRecord+" ")
		return handleShellCommand(command, chatManager, terminal, state)

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
			terminal.PrintError(fmt.Sprintf("error exporting chat: %v", err))
		}
		return true

	case input == config.Backtrack:
		backtrackedCount, err := chatManager.BacktrackHistory(terminal)
		if err != nil {
			terminal.PrintError(err.Error())
		} else {
			terminal.PrintInfo(fmt.Sprintf("backtracked by %d", backtrackedCount))
		}
		return true

	case input == config.ListHistory:
		err := handleListHistory(chatManager, terminal, state)
		if err != nil {
			terminal.PrintError(fmt.Sprintf("error listing history: %v", err))
		}
		return true

	case input == config.ScrapeURL:
		if fromHelp {
			fmt.Printf("\033[93m%s <url1> [url2] ... - scrape content from URLs\033[0m\n", config.ScrapeURL)
		} else {
			terminal.PrintError("no URLs provided")
		}
		return true

	case strings.HasPrefix(input, config.ScrapeURL+" "):
		urls := strings.Fields(strings.TrimPrefix(input, config.ScrapeURL+" "))
		return handleScrapeURLs(urls, chatManager, terminal, state)

	case input == config.WebSearch:
		if fromHelp {
			fmt.Printf("\033[93m%s <query> - search web using ddgr\033[0m\n", config.WebSearch)
		} else {
			terminal.PrintError("no search query provided")
		}
		return true

	case strings.HasPrefix(input, config.WebSearch+" "):
		query := strings.TrimPrefix(input, config.WebSearch+" ")
		return handleWebSearch(query, chatManager, terminal, state)

	case input == config.CopyToClipboard:
		if fromHelp {
			fmt.Printf("\033[93m%s - copy selected responses to clipboard\033[0m\n", config.CopyToClipboard)
		} else {
			err := terminal.CopyResponsesInteractive(chatManager.GetChatHistory())
			if err != nil {
				terminal.PrintError(fmt.Sprintf("error copying to clipboard: %v", err))
			}
		}
		return true

	case input == config.MultiLine:
		var lines []string
		terminal.PrintInfo("multi-line mode (end with '\\' on a new line)")

		// Create a new readline instance for multi-line input
		multiLineRl, err := readline.NewEx(&readline.Config{
			Prompt:      "... ",
			HistoryFile: "/dev/null", // Disable history for multi-line
		})
		if err != nil {
			terminal.PrintError(fmt.Sprintf("error creating multi-line input: %v", err))
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

func handleFileLoad(chatManager *chat.Manager, terminal *ui.Terminal, state *types.AppState, dirPath string) bool {
	var files []string
	var err error

	if dirPath == "" {
		// Use current directory
		files, err = terminal.GetCurrentDirFilesRecursive()
		if err != nil {
			terminal.PrintError(fmt.Sprintf("error reading current directory: %v", err))
			return true
		}
		if len(files) == 0 {
			terminal.PrintError("no files found in current directory")
			return true
		}
	} else {
		// Use specified directory
		if _, err := os.Stat(dirPath); os.IsNotExist(err) {
			terminal.PrintError(fmt.Sprintf("directory does not exist: %s", dirPath))
			return true
		}
		files, err = terminal.GetDirFilesRecursive(dirPath)
		if err != nil {
			terminal.PrintError(fmt.Sprintf("error reading directory %s: %v", dirPath, err))
			return true
		}
		if len(files) == 0 {
			terminal.PrintError(fmt.Sprintf("no files found in directory: %s", dirPath))
			return true
		}
	}

	selections, err := terminal.FzfMultiSelect(files, "Files: ")
	if err != nil {
		terminal.PrintError(fmt.Sprintf("error selecting files: %v", err))
		return true
	}

	if len(selections) == 0 {
		return true
	}

	// Resolve full paths if using custom directory
	var fullPaths []string
	if dirPath != "" {
		for _, selection := range selections {
			fullPaths = append(fullPaths, filepath.Join(dirPath, selection))
		}
	} else {
		fullPaths = selections
	}

	content, err := terminal.LoadFileContent(fullPaths)
	if err != nil {
		terminal.PrintError(fmt.Sprintf("error loading content: %v", err))
		return true
	}

	if content != "" {
		chatManager.AddUserMessage(content)
		if dirPath != "" {
			historySummary := fmt.Sprintf("Loaded from %s: %s", dirPath, strings.Join(selections, ", "))
			chatManager.AddToHistory(historySummary, "")
		} else {
			historySummary := fmt.Sprintf("Loaded: %s", strings.Join(selections, ", "))
			chatManager.AddToHistory(historySummary, "")
		}
	}

	return true
}

func handleCodeDump(chatManager *chat.Manager, terminal *ui.Terminal, state *types.AppState) bool {
	codedump, err := terminal.CodeDump()
	if err != nil {
		terminal.PrintError(fmt.Sprintf("error generating codedump: %v", err))
		return true
	}

	if codedump != "" {
		chatManager.AddUserMessage(codedump)
		chatManager.AddToHistory("Codedump loaded", "")
	}

	return true
}

func handleShellRecord(chatManager *chat.Manager, terminal *ui.Terminal, state *types.AppState) bool {
	sessionContent, err := terminal.RecordShellSession()
	if err != nil {
		terminal.PrintError(fmt.Sprintf("error recording shell session: %v", err))
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
		chatManager.AddToHistory("Shell session loaded", "")
	} else {
		terminal.PrintInfo("no activity recorded in shell session")
	}

	return true
}

func handleShellCommand(command string, chatManager *chat.Manager, terminal *ui.Terminal, state *types.AppState) bool {
	if command == "" {
		terminal.PrintError("no command specified")
		return true
	}

	// Execute the command with live streaming
	cmd := exec.Command("sh", "-c", command)

	// Create pipes for stdout and stderr
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		terminal.PrintError(fmt.Sprintf("failed to create stdout pipe: %v", err))
		return true
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		terminal.PrintError(fmt.Sprintf("failed to create stderr pipe: %v", err))
		return true
	}

	// Start the command
	if err := cmd.Start(); err != nil {
		terminal.PrintError(fmt.Sprintf("failed to start command: %v", err))
		return true
	}

	// Set up signal handling for command interruption
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt)
	defer signal.Stop(sigChan)

	// Capture output while streaming it live
	var outputBuffer strings.Builder

	// Read stdout and stderr concurrently
	done := make(chan bool, 2)

	go func() {
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			line := scanner.Text()
			fmt.Println(line)
			outputBuffer.WriteString(line + "\n")
		}
		done <- true
	}()

	go func() {
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			line := scanner.Text()
			fmt.Println(line)
			outputBuffer.WriteString(line + "\n")
		}
		done <- true
	}()

	// Wait for either command completion or interruption
	cmdDone := make(chan error, 1)
	go func() {
		cmdDone <- cmd.Wait()
	}()

	var cmdErr error
	select {
	case <-sigChan:
		// Interrupt received - kill the command
		if err := cmd.Process.Kill(); err != nil {
			terminal.PrintError(fmt.Sprintf("failed to kill command: %v", err))
		}
		fmt.Println("\nCommand interrupted")

		// Wait for goroutines to finish reading any remaining output
		go func() {
			<-done
			<-done
		}()

		cmdErr = fmt.Errorf("command interrupted by user")

	case cmdErr = <-cmdDone:
		// Command completed normally
		// Wait for both goroutines to finish
		<-done
		<-done
	}

	outputStr := outputBuffer.String()

	var result string
	if cmdErr != nil {
		result = fmt.Sprintf("Command: %s\nError: %v\nOutput:\n%s", command, cmdErr, outputStr)
	} else {
		result = fmt.Sprintf("Command: %s\nOutput:\n%s", command, outputStr)
	}

	// Format the content for the chat context (uses full output)
	formattedContent := fmt.Sprintf("The user executed the following command and here is the output:\n\n---\n%s\n---", result)

	chatManager.AddUserMessage(formattedContent)
	chatManager.AddToHistory(fmt.Sprintf("!x %s", command), "Command executed and output added to context")

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
	filePaths, err := chatManager.ExportCodeBlocks(terminal)
	if err != nil {
		return err
	}

	if len(filePaths) == 0 {
		terminal.PrintInfo("no code blocks found in the last response")
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

func handleListHistory(chatManager *chat.Manager, terminal *ui.Terminal, state *types.AppState) error {
	return chatManager.ListChatHistory(terminal)
}

func handleTokenCount(filePath string, model string, terminal *ui.Terminal, state *types.AppState) error {
	// Check if file exists
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return fmt.Errorf("file does not exist: %s", filePath)
	}

	// Read file content
	content, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("error reading file: %v", err)
	}

	// Determine model for tokenization
	targetModel := model
	if targetModel == "" {
		targetModel = state.Config.CurrentModel
		if targetModel == "" {
			targetModel = state.Config.DefaultModel
		}
	}

	// Map model names to tokenizer encodings
	var encoding tokenizer.Encoding
	switch {
	case strings.Contains(strings.ToLower(targetModel), "gpt-4"):
		encoding = tokenizer.Cl100kBase
	case strings.Contains(strings.ToLower(targetModel), "gpt-3.5"):
		encoding = tokenizer.Cl100kBase
	case strings.Contains(strings.ToLower(targetModel), "gpt-2"):
		encoding = tokenizer.R50kBase
	case strings.Contains(strings.ToLower(targetModel), "claude"):
		encoding = tokenizer.Cl100kBase // Use cl100k_base as approximation for Claude
	default:
		encoding = tokenizer.Cl100kBase // Default to cl100k_base
	}

	// Get tokenizer
	enc, err := tokenizer.Get(encoding)
	if err != nil {
		return fmt.Errorf("error getting tokenizer: %v", err)
	}

	// Encode and count tokens
	tokens, _, err := enc.Encode(string(content))
	if err != nil {
		return fmt.Errorf("error encoding text: %v", err)
	}

	// Print results with colors matching the project's style
	fmt.Printf("\033[96m%s\033[0m %s\n", "File:", filePath)
	fmt.Printf("\033[96m%s\033[0m \033[95m%s\033[0m\n", "Model:", targetModel)
	fmt.Printf("\033[96m%s\033[0m \033[91m%d\033[0m\n", "Tokens:", len(tokens))

	return nil
}

// handleLoadFile loads and displays file content or scrapes URL
func handleLoadFile(filePath string, terminal *ui.Terminal) error {
	// Check if it's a URL
	if terminal.IsURL(filePath) {
		// Use the same loading logic as !l command for URLs
		content, err := terminal.LoadFileContent([]string{filePath})
		if err != nil {
			return fmt.Errorf("failed to scrape URL: %w", err)
		}
		fmt.Print(content)
		return nil
	}

	// Check if file exists
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return fmt.Errorf("file does not exist: %s", filePath)
	}

	// Use the same loading logic as !l command
	content, err := terminal.LoadFileContent([]string{filePath})
	if err != nil {
		return fmt.Errorf("failed to load file: %w", err)
	}

	// Print the content directly to stdout
	fmt.Print(content)
	return nil
}

// handleScrapeURLs handles the !s command for scraping URLs
func handleScrapeURLs(urls []string, chatManager *chat.Manager, terminal *ui.Terminal, state *types.AppState) bool {
	if len(urls) == 0 {
		terminal.PrintError("no URLs provided")
		return true
	}

	content, err := terminal.ScrapeURLs(urls)
	if err != nil {
		terminal.PrintError(fmt.Sprintf("error scraping URLs: %v", err))
		return true
	}

	if content != "" {
		chatManager.AddUserMessage(content)
		historySummary := fmt.Sprintf("Scraped: %s", strings.Join(urls, ", "))
		chatManager.AddToHistory(historySummary, "")
	}

	return true
}

// handleWebSearch handles the !w command for web search
func handleWebSearch(query string, chatManager *chat.Manager, terminal *ui.Terminal, state *types.AppState) bool {
	if query == "" {
		terminal.PrintError("no search query provided")
		return true
	}

	content, err := terminal.WebSearch(query)
	if err != nil {
		terminal.PrintError(fmt.Sprintf("error searching: %v", err))
		return true
	}

	if content != "" {
		chatManager.AddUserMessage(content)
		historySummary := fmt.Sprintf("Web search: %s", query)
		chatManager.AddToHistory(historySummary, "")
	}

	return true
}
