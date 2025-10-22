package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"math/rand"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

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

	// detect if stdout is being piped
	stdoutStat, _ := os.Stdout.Stat()
	if (stdoutStat.Mode() & os.ModeCharDevice) == 0 {
		state.Config.IsPipedOutput = true
	}

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
		allModelsFlag  = flag.String("o", "", "Specify platform and model (format: platform|model)")
		exportCodeFlag = flag.Bool("e", false, "Export code blocks from the last response")
		tokenFlag      = flag.String("t", "", "Count tokens in file")
		loadFileFlag   = flag.String("l", "", "Load and display file content (supports text, PDF, DOCX, XLSX, CSV)")
		webSearchFlag  = flag.String("w", "", "Perform a web search and print the results")
		scrapeURLFlag  = flag.String("s", "", "Scrape a URL and print the content")
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

	// handle web search flag
	if *webSearchFlag != "" {
		results, err := terminal.WebSearch(*webSearchFlag)
		if err != nil {
			terminal.PrintError(fmt.Sprintf("error during web search: %v", err))
			return
		}
		fmt.Println(results)
		return
	}

	// handle scrape URL flag
	if *scrapeURLFlag != "" {
		content, err := terminal.ScrapeURLs([]string{*scrapeURLFlag})
		if err != nil {
			terminal.PrintError(fmt.Sprintf("error scraping URL: %v", err))
			return
		}
		fmt.Println(strings.TrimSpace(content))
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
				terminal.PrintError("invalid directory path or permission denied")
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

		currentDir, err := os.Getwd()
		if err != nil {
			terminal.PrintError(fmt.Sprintf("error getting current directory: %v", err))
			return
		}
		filename := generateUniqueCodeDumpFilename(currentDir, codedump)
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

	// Handle -o flag (platform|model format)
	if *allModelsFlag != "" {
		parts := strings.Split(*allModelsFlag, "|")
		if len(parts) != 2 {
			terminal.PrintError("invalid -o format: use platform|model (e.g., openai|gpt-4)")
			return
		}

		platformName := strings.TrimSpace(parts[0])
		modelName := strings.TrimSpace(parts[1])

		if platformName == "" || modelName == "" {
			terminal.PrintError("invalid -o format: platform and model cannot be empty")
			return
		}

		// Validate platform exists
		if platformName != "openai" {
			if _, exists := state.Config.Platforms[platformName]; !exists {
				terminal.PrintError(fmt.Sprintf("platform '%s' not found", platformName))
				return
			}
		}

		*platformFlag = platformName
		*modelFlag = modelName
	}

	// Set platform and model based on precedence: flags > env vars > config file
	finalPlatform := state.Config.CurrentPlatform
	finalModel := state.Config.CurrentModel

	// Environment variables
	if p := os.Getenv("CH_DEFAULT_PLATFORM"); p != "" {
		finalPlatform = p
	}
	if m := os.Getenv("CH_DEFAULT_MODEL"); m != "" {
		finalModel = m
	}

	// Command-line flags override everything
	if *platformFlag != "" {
		finalPlatform = *platformFlag
	}
	if *modelFlag != "" {
		finalModel = *modelFlag
	}

	// Apply the final platform and model
	if finalPlatform != state.Config.CurrentPlatform || finalModel != state.Config.CurrentModel {
		// If the platform was changed via flag/env, we may need to select a model for it
		if *platformFlag != "" {
			result, err := platformManager.SelectPlatform(finalPlatform, finalModel, terminal.FzfSelect)
			if err != nil {
				terminal.PrintError(fmt.Sprintf("%v", err))
				return
			}
			if result != nil {
				chatManager.SetCurrentPlatform(result["platform_name"].(string))
				chatManager.SetCurrentModel(result["picked_model"].(string))
			}
		} else {
			chatManager.SetCurrentPlatform(finalPlatform)
			chatManager.SetCurrentModel(finalModel)
		}
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
			terminal.PrintError(fmt.Sprintf("error exporting code blocks: %v", exportErr))
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
		Prompt:          "\033[94muser: \033[0m",
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
				if !state.Config.MuteNotifications {
					fmt.Printf("\033[93mpress ctrl+d to exit\033[0m\n")
				}
				continue
			}
			// io.EOF (Ctrl+D) or other errors - exit
			break
		}
		input := strings.TrimSpace(line)

		if input == "" {
			continue
		}

		// Check if input ends with backslash for automatic multi-line continuation
		if strings.HasSuffix(input, state.Config.MultiLine) && input != state.Config.MultiLine {
			// Remove trailing backslash from the first line
			input = strings.TrimSuffix(input, state.Config.MultiLine)
			input = strings.TrimRight(input, " \t")

			var lines []string
			if input != "" {
				lines = append(lines, input)
			}

			// Create a new readline instance for multi-line input
			multiLineRl, err := readline.NewEx(&readline.Config{
				Prompt:      "... ",
				HistoryFile: "/dev/null", // Disable history for multi-line
			})
			if err != nil {
				terminal.PrintError(fmt.Sprintf("error creating multi-line input: %v", err))
				continue
			}

			multiLineActive := true
			for multiLineActive {
				line, err := multiLineRl.Readline()
				if err != nil {
					if err == readline.ErrInterrupt || err == io.EOF {
						multiLineRl.Close()
						multiLineActive = false
						break
					}
					break
				}

				// Check if line ends with backslash
				if strings.HasSuffix(line, state.Config.MultiLine) {
					// Remove trailing backslash and continue to next line
					line = strings.TrimSuffix(line, state.Config.MultiLine)
					line = strings.TrimRight(line, " \t")
					lines = append(lines, line)
				} else {
					// No backslash at end - this is the final line
					lines = append(lines, line)
					multiLineActive = false
					break
				}
			}

			multiLineRl.Close()

			input = strings.Join(lines, "\n")
			if strings.TrimSpace(input) == "" {
				continue
			}
		}

		if handleSpecialCommands(input, chatManager, platformManager, terminal, state) {
			continue
		}

		chatManager.AddUserMessage(input)

		// Start loading animation for non-streaming models
		var loadingDone chan bool
		if platformManager.IsReasoningModel(chatManager.GetCurrentModel()) {
			loadingDone = make(chan bool)
			go terminal.ShowLoadingAnimation("thinking", loadingDone)
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
			if state.Config.IsPipedOutput {
				fmt.Printf("%s\n", response)
			} else {
				fmt.Printf("\033[92m%s\033[0m\n", response)
			}
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
		if selectedCommand == ">state" {
			err := handleShowState(chatManager, terminal, state)
			if err != nil {
				terminal.PrintError(fmt.Sprintf("error showing state: %v", err))
			}
			return true
		}
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

		selectedModel, err := terminal.FzfSelect(models, "model: ")
		if err != nil {
			terminal.PrintError(fmt.Sprintf("error selecting model: %v", err))
			return true
		}

		if selectedModel != "" {
			chatManager.SetCurrentModel(selectedModel)
			if !config.MuteNotifications {
				terminal.PrintModelSwitch(selectedModel)
			}
		}
		return true

	case strings.HasPrefix(input, config.ModelSwitch+" "):
		modelName := strings.TrimPrefix(input, config.ModelSwitch+" ")
		chatManager.SetCurrentModel(modelName)
		if !config.MuteNotifications {
			terminal.PrintModelSwitch(modelName)
		}
		return true

	case input == config.PlatformSwitch:
		result, err := platformManager.SelectPlatform("", "", terminal.FzfSelect)
		if err != nil {
			terminal.PrintError(fmt.Sprintf("%v", err))
		} else if result != nil {
			chatManager.SetCurrentPlatform(result["platform_name"].(string))
			chatManager.SetCurrentModel(result["picked_model"].(string))
			config.CurrentBaseURL = result["base_url"].(string)
			err = platformManager.Initialize()
			if err != nil {
				terminal.PrintError(fmt.Sprintf("error initializing client: %v", err))
			} else {
				if !config.MuteNotifications {
					terminal.PrintPlatformSwitch(result["platform_name"].(string), result["picked_model"].(string))
				}
			}
		}
		return true

	case strings.HasPrefix(input, config.PlatformSwitch+" "):
		platformName := strings.TrimPrefix(input, config.PlatformSwitch+" ")
		result, err := platformManager.SelectPlatform(platformName, "", terminal.FzfSelect)
		if err != nil {
			terminal.PrintError(fmt.Sprintf("%v", err))
		} else if result != nil {
			chatManager.SetCurrentPlatform(result["platform_name"].(string))
			chatManager.SetCurrentModel(result["picked_model"].(string))
			config.CurrentBaseURL = result["base_url"].(string)
			err = platformManager.Initialize()
			if err != nil {
				terminal.PrintError(fmt.Sprintf("error initializing client: %v", err))
			} else {
				if !config.MuteNotifications {
					terminal.PrintPlatformSwitch(result["platform_name"].(string), result["picked_model"].(string))
				}
			}
		}
		return true

	case input == config.AllModels:
		return handleAllModels(chatManager, platformManager, terminal, state)

	case input == config.LoadFiles:
		return handleFileLoad(chatManager, terminal, state, "")

	case strings.HasPrefix(input, config.LoadFiles+" "):
		dirPath := strings.TrimSpace(strings.TrimPrefix(input, config.LoadFiles+" "))
		return handleFileLoad(chatManager, terminal, state, dirPath)

	case input == config.CodeDump:
		return handleCodeDump(chatManager, terminal, state)

	case input == config.ShellRecord:
		if fromHelp {
			fmt.Printf("\033[93m%s - record shell session\033[0m\n", config.ShellRecord)
			return true
		}
		return handleShellRecord(chatManager, terminal, state)

	case strings.HasPrefix(input, config.ShellRecord+" "):
		command := strings.TrimPrefix(input, config.ShellRecord+" ")
		if fromHelp {
			fmt.Printf("\033[93m%s [command] - record shell session\033[0m\n", config.ShellRecord)
			return true
		}
		return handleShellCommand(command, chatManager, terminal, state)

	case strings.HasPrefix(input, config.EditorInput+" "):
		arg := strings.TrimSpace(strings.TrimPrefix(input, config.EditorInput+" "))

		if fromHelp {
			fmt.Printf("\033[93m%s [buff] - text editor mode\033[0m\n", config.EditorInput)
			return true
		}

		if arg == "buff" {
			// Buffer mode: load content into memory without sending to model
			userInput, err := chatManager.HandleTerminalInput()
			if err != nil {
				terminal.PrintError(fmt.Sprintf("%v", err))
				return true
			}

			fmt.Printf("\033[94m> %s\033[0m\n", strings.ReplaceAll(userInput, "\n", "\n> "))

			chatManager.AddUserMessage(userInput)
			chatManager.AddToHistory("Text editor buffer loaded", "")
			return true
		}

		// Unknown argument
		terminal.PrintError(fmt.Sprintf("unknown argument '%s'. Use '%s' or '%s buff'", arg, config.EditorInput, config.EditorInput))
		return true

	case input == config.EditorInput:
		if fromHelp {
			fmt.Printf("\033[93m%s [buff] - text editor mode\033[0m\n", config.EditorInput)
			return true
		}

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

	case input == config.ScrapeURL:
		if fromHelp {
			fmt.Printf("\033[93m%s [url] - scrape URL(s)\033[0m\n", config.ScrapeURL)
			return true
		}

		// Extract all URLs from both chat history and messages
		historyURLs := terminal.ExtractURLsFromChatHistory(chatManager.GetChatHistory())
		messageURLs := terminal.ExtractURLsFromMessages(chatManager.GetMessages())

		// Combine and deduplicate URLs
		urlMap := make(map[string]bool)
		for _, url := range historyURLs {
			urlMap[url] = true
		}
		for _, url := range messageURLs {
			urlMap[url] = true
		}

		var allURLs []string
		for url := range urlMap {
			allURLs = append(allURLs, url)
		}

		if len(allURLs) == 0 {
			terminal.PrintError("no URLs found in chat history")
			return true
		}

		// Let user select URLs using fzf with multi-select (tab key)
		selectedURLs, err := terminal.FzfMultiSelect(allURLs, "select urls to scrape (tab=multi): ")
		if err != nil {
			terminal.PrintError(fmt.Sprintf("error selecting URLs: %v", err))
			return true
		}

		if len(selectedURLs) == 0 {
			return true
		}

		// Scrape the selected URLs
		return handleScrapeURLs(selectedURLs, chatManager, terminal, state)

	case strings.HasPrefix(input, config.ScrapeURL+" "):
		urls := strings.Fields(strings.TrimPrefix(input, config.ScrapeURL+" "))
		return handleScrapeURLs(urls, chatManager, terminal, state)

	case input == config.WebSearch:
		if fromHelp {
			fmt.Printf("\033[93m%s [query] - web search\033[0m\n", config.WebSearch)
			return true
		}

		// Extract all sentences from chat history
		allSentences := terminal.ExtractSentencesFromChatHistory(chatManager.GetChatHistory(), chatManager.GetMessages())

		if len(allSentences) == 0 {
			terminal.PrintError("no sentences found in chat history")
			return true
		}

		// Let user select a sentence using fzf
		selectedSentence, err := terminal.FzfSelect(allSentences, "select sentence to search: ")
		if err != nil {
			terminal.PrintError(fmt.Sprintf("error selecting sentence: %v", err))
			return true
		}

		if selectedSentence == "" {
			return true
		}

		// Use the selected sentence as the search query
		return handleWebSearch(selectedSentence, chatManager, terminal, state)

	case strings.HasPrefix(input, config.WebSearch+" "):
		query := strings.TrimPrefix(input, config.WebSearch+" ")
		return handleWebSearch(query, chatManager, terminal, state)

	case input == config.CopyToClipboard:
		err := terminal.CopyResponsesInteractive(chatManager.GetChatHistory())
		if err != nil {
			terminal.PrintError(fmt.Sprintf("error copying to clipboard: %v", err))
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
					return true // Exit silently
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
			if state.Config.IsPipedOutput {
				fmt.Printf("%s\n", response)
			} else {
				fmt.Printf("\033[92m%s\033[0m\n", response)
			}
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
	var targetPath string

	if dirPath == "" {
		// Use current directory
		targetPath, _ = os.Getwd()
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
		targetPath = dirPath
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

	// Check if this is a shallow load directory and inform the user
	if isShallowLoadDir(targetPath, state.Config) {
		terminal.PrintInfo("shallow loading")
	}

	selections, err := terminal.FzfMultiSelect(files, "files: ")
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
		fmt.Println("\ncommand interrupted")

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

func handleShowState(chatManager *chat.Manager, terminal *ui.Terminal, state *types.AppState) error {
	// Get current time
	currentDate := time.Now().Format("2006-01-02")
	currentTime := time.Now().Format("15:04:05 MST")

	// Get platform and model
	platform := chatManager.GetCurrentPlatform()
	model := chatManager.GetCurrentModel()

	// Get chat history and count
	chatHistory := chatManager.GetChatHistory()
	chatCount := len(chatHistory) - 1 // Subtract system prompt

	// Calculate total token count
	var totalContent string
	for _, entry := range chatHistory {
		totalContent += entry.User + " " + entry.Bot + " "
	}

	encoding := tokenizer.Cl100kBase
	enc, err := tokenizer.Get(encoding)
	if err != nil {
		return fmt.Errorf("error getting tokenizer: %v", err)
	}

	tokens, _, err := enc.Encode(totalContent)
	if err != nil {
		return fmt.Errorf("error encoding text: %v", err)
	}
	tokenCount := len(tokens)

	// Print the state
	combinedDateTime := currentDate + " " + currentTime
	if state.Config.IsPipedOutput {
		fmt.Printf("%s %s\n", "date:", combinedDateTime)
		fmt.Printf("%s %s\n", "platform:", platform)
		fmt.Printf("%s %s\n", "model:", model)
		fmt.Printf("%s %d\n", "chats:", chatCount)
		fmt.Printf("%s %d\n", "tokens:", tokenCount)
	} else {
		fmt.Printf("\033[96m%s\033[0m \033[93m%s\033[0m\n", "date:", combinedDateTime)
		fmt.Printf("\033[96m%s\033[0m \033[95m%s\033[0m\n", "platform:", platform)
		fmt.Printf("\033[96m%s\033[0m \033[95m%s\033[0m\n", "model:", model)
		fmt.Printf("\033[96m%s\033[0m \033[92m%d\033[0m\n", "chats:", chatCount)
		fmt.Printf("\033[96m%s\033[0m \033[91m%d\033[0m\n", "tokens:", tokenCount)
	}

	return nil
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
	if state.Config.IsPipedOutput {
		fmt.Printf("%s %s\n", "file:", filePath)
		fmt.Printf("%s %s\n", "model:", targetModel)
		fmt.Printf("%s %d\n", "tokens:", len(tokens))
	} else {
		fmt.Printf("\033[96m%s\033[0m %s\n", "file:", filePath)
		fmt.Printf("\033[96m%s\033[0m \033[95m%s\033[0m\n", "model:", targetModel)
		fmt.Printf("\033[96m%s\033[0m \033[91m%d\033[0m\n", "tokens:", len(tokens))
	}

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

// generateHashFromContent creates a short hash using characters from the content
func generateHashFromContent(content string, length int) string {
	return generateHashFromContentWithOffset(content, length, 0)
}

// generateHashFromContentWithOffset creates a hash with an offset for collision avoidance
func generateHashFromContentWithOffset(content string, length, offset int) string {
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

// generateUniqueCodeDumpFilename generates a unique filename for code dump with collision detection
func generateUniqueCodeDumpFilename(currentDir, content string) string {
	baseHash := generateHashFromContent(content, 8)
	filename := fmt.Sprintf("code_dump_%s.txt", baseHash)
	fullPath := filepath.Join(currentDir, filename)

	// Check if file exists, if not return it
	if _, err := os.Stat(fullPath); os.IsNotExist(err) {
		return filename
	}

	// If file exists, try with different offsets
	for offset := 1; offset <= 10; offset++ {
		newHash := generateHashFromContentWithOffset(content, 8, offset)
		filename = fmt.Sprintf("code_dump_%s.txt", newHash)
		fullPath = filepath.Join(currentDir, filename)

		if _, err := os.Stat(fullPath); os.IsNotExist(err) {
			return filename
		}
	}

	// If still colliding, add a numeric suffix
	for counter := 1; counter <= 999; counter++ {
		filename = fmt.Sprintf("code_dump_%s_%03d.txt", baseHash, counter)
		fullPath = filepath.Join(currentDir, filename)

		if _, err := os.Stat(fullPath); os.IsNotExist(err) {
			return filename
		}
	}

	// Fallback to original UUID if everything fails
	return fmt.Sprintf("code_dump_%s.txt", uuid.New().String())
}

// handleAllModels handles the !o command for selecting from all available models
func handleAllModels(chatManager *chat.Manager, platformManager *platform.Manager, terminal *ui.Terminal, state *types.AppState) bool {
	// Create channels for async operation
	type modelResult struct {
		models []string
		err    error
	}
	resultChan := make(chan modelResult)

	// Start fetching models in a goroutine
	go func() {
		models, err := platformManager.FetchAllModelsAsync()
		resultChan <- modelResult{models, err}
	}()

	// Show loading animation
	done := make(chan bool)
	go terminal.ShowLoadingAnimation("fetching models", done)

	// Wait for models to be fetched
	result := <-resultChan
	done <- true // Stop animation

	if result.err != nil {
		terminal.PrintError(fmt.Sprintf("%v", result.err))
		return true
	}

	if len(result.models) == 0 {
		terminal.PrintError("no models found")
		return true
	}

	models := result.models

	// Create a map to store platform and model info indexed by display string
	type modelInfo struct {
		platform string
		model    string
	}
	modelMap := make(map[string]modelInfo)

	for _, m := range models {
		parts := strings.SplitN(m, "] ", 2)
		if len(parts) == 2 {
			platform := strings.TrimPrefix(parts[0], "[")
			modelName := parts[1]
			modelMap[m] = modelInfo{platform, modelName}
		}
	}

	selectedModel, err := terminal.FzfSelect(models, "model: ")
	if err != nil {
		terminal.PrintError(fmt.Sprintf("error selecting model: %v", err))
		return true
	}

	if selectedModel == "" {
		return true
	}

	// Look up the platform and model from the map
	info, exists := modelMap[selectedModel]
	if !exists {
		terminal.PrintError("invalid model selection")
		return true
	}

	platformName := info.platform
	modelName := info.model

	// Store current platform to detect if it changed
	currentPlatform := state.Config.CurrentPlatform

	// Update the current platform and model in config
	state.Config.CurrentPlatform = platformName
	state.Config.CurrentModel = modelName

	// Update chatManager with the new values
	chatManager.SetCurrentPlatform(platformName)
	chatManager.SetCurrentModel(modelName)

	// If platform changed, reinitialize the client
	if platformName != currentPlatform {
		err := platformManager.Initialize()
		if err != nil {
			terminal.PrintError(fmt.Sprintf("error initializing client: %v", err))
			// Restore previous state on error
			state.Config.CurrentPlatform = currentPlatform
			return true
		}
	}

	if !state.Config.MuteNotifications {
		terminal.PrintModelSwitch(modelName)
	}

	return true
}

// isShallowLoadDir checks if a directory is in the shallow load list
func isShallowLoadDir(dirPath string, cfg *types.Config) bool {
	// Normalize the directory path
	absPath, err := filepath.Abs(dirPath)
	if err != nil {
		return false
	}
	absPath = filepath.Clean(absPath)

	// Check against each shallow load directory
	for _, shallowDir := range cfg.ShallowLoadDirs {
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
