package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/MehmetMHY/ch/internal/chat"
	"github.com/MehmetMHY/ch/internal/config"
	"github.com/MehmetMHY/ch/internal/platform"
	"github.com/MehmetMHY/ch/internal/search"
	"github.com/MehmetMHY/ch/internal/ui"
	"github.com/MehmetMHY/ch/pkg/types"
	"github.com/chzyer/readline"
)

func main() {
	// initialize application state
	state := config.InitializeAppState()

	// initialize components
	terminal := ui.NewTerminal(state.Config)
	chatManager := chat.NewManager(state)
	platformManager := platform.NewManager(state.Config)
	searchClient := search.NewSearXNGClient("")

	// parse command line arguments
	var (
		helpFlag     = flag.Bool("h", false, "Show help")
		platformFlag = flag.String("p", "", "Switch platform (leave empty for interactive selection)")
		modelFlag    = flag.String("m", "", "Specify model to use")
	)

	flag.Parse()
	remainingArgs := flag.Args()

	// handle help flag
	if *helpFlag {
		terminal.ShowHelp()
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
		err := processDirectQuery(query, chatManager, platformManager, searchClient, terminal, state)
		if err != nil {
			terminal.PrintError(fmt.Sprintf("%v", err))
		}
		return
	}

	// interactive mode
	runInteractiveMode(chatManager, platformManager, searchClient, terminal, state)
}

func processDirectQuery(query string, chatManager *chat.Manager, platformManager *platform.Manager, searchClient *search.SearXNGClient, terminal *ui.Terminal, state *types.AppState) error {
	if handleSpecialCommands(query, chatManager, platformManager, searchClient, terminal, state) {
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
	return nil
}

func runInteractiveMode(chatManager *chat.Manager, platformManager *platform.Manager, searchClient *search.SearXNGClient, terminal *ui.Terminal, state *types.AppState) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		terminal.PrintError(fmt.Sprintf("Error getting home directory: %v", err))
		return
	}
	historyFile := filepath.Join(homeDir, ".ch_history")

	rl, err := readline.NewEx(&readline.Config{
		Prompt:      "\033[94mUser: \033[0m",
		HistoryFile: historyFile,
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

		if handleSpecialCommands(input, chatManager, platformManager, searchClient, terminal, state) {
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

func handleSpecialCommands(input string, chatManager *chat.Manager, platformManager *platform.Manager, searchClient *search.SearXNGClient, terminal *ui.Terminal, state *types.AppState) bool {
	config := state.Config

	switch {
	case input == config.ExitKey:
		fmt.Println("Goodbye!")
		os.Exit(0)
		return true

	case input == config.HelpKey || input == "help":
		terminal.PrintTitle()
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

	case input == "!p":
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
				chatManager.AddToHistory("!p", fmt.Sprintf("Switched from %s/%s to %s/%s", oldPlatform, oldModel, result["platform_name"].(string), result["picked_model"].(string)))
			}
		}
		return true

	case strings.HasPrefix(input, "!p "):
		platformName := strings.TrimPrefix(input, "!p ")
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
				chatManager.AddToHistory(fmt.Sprintf("!p %s", platformName), fmt.Sprintf("Switched from %s/%s to %s/%s", oldPlatform, oldModel, result["platform_name"].(string), result["picked_model"].(string)))
			}
		}
		return true

	case strings.HasPrefix(input, "!w "):
		return handleWebSearch(input, chatManager, platformManager, searchClient, terminal, state)

	case input == "!w":
		terminal.PrintError("Please provide a search query after !w")
		return true

	case input == "!l":
		return handleFileLoad(chatManager, terminal, state)

	// Temp: (2025-07-09) For handling 'cha -ocr' integration.
	case input == config.LoadFileOCR:
		return handleFileLoadOCR(chatManager, terminal, state)

	case input == config.TerminalInput:
		userInput, err := chatManager.HandleTerminalInput()
		if err != nil {
			terminal.PrintError(fmt.Sprintf("%v", err))
			return true
		}

		fmt.Printf("\033[94m> %s\033[0m\n", strings.ReplaceAll(userInput, "\n", "\n> "))

		chatManager.AddUserMessage(userInput)

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
		chatManager.AddToHistory(userInput, response)
		return true

	case input == config.ExportChat:
		filePath, err := chatManager.ExportHistory()
		if err != nil {
			terminal.PrintError(fmt.Sprintf("Error exporting chat: %v", err))
		} else {
			fmt.Println(filePath)
		}
		return true

	default:
		return false
	}
}

func handleWebSearch(input string, chatManager *chat.Manager, platformManager *platform.Manager, searchClient *search.SearXNGClient, terminal *ui.Terminal, state *types.AppState) bool {
	searchQuery := strings.TrimPrefix(input, "!w ")
	if strings.TrimSpace(searchQuery) == "" {
		terminal.PrintError("Please provide a search query after !w")
		return true
	}

	searchResults, err := searchClient.Search(searchQuery)
	if err != nil {
		terminal.PrintError(fmt.Sprintf("%v", err))
		return true
	}

	searchContext := searchClient.FormatResults(searchResults, searchQuery)

	chatManager.AddUserMessage(searchContext)

	response, err := platformManager.SendChatRequest(chatManager.GetMessages(), chatManager.GetCurrentModel(), &state.StreamingCancel, &state.IsStreaming)
	if err != nil {
		if err.Error() == "request was interrupted" {
			chatManager.RemoveLastUserMessage()
			return true
		}
		terminal.PrintError(fmt.Sprintf("Error generating response: %v", err))
		return true
	}

	chatManager.AddAssistantMessage(response)
	chatManager.AddToHistory(fmt.Sprintf("!w %s", searchQuery), response)

	return true
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
		userPrompt := fmt.Sprintf("!l [Loaded %d file(s)/directory(s)]:\n%s", len(selections), content)
		chatManager.AddUserMessage(content)
		chatManager.AddToHistory(userPrompt, "")
	}

	return true
}

// Temp: (2025-07-09) For handling 'cha -ocr' integration.
func handleFileLoadOCR(chatManager *chat.Manager, terminal *ui.Terminal, state *types.AppState) bool {
	files, err := terminal.GetCurrentDirFilesOnly()
	if err != nil {
		terminal.PrintError(fmt.Sprintf("Error reading directory: %v", err))
		return true
	}

	selection, err := terminal.FzfSelectOrQuery(files, "Select files/dirs or enter a URL: ")
	if err != nil {
		terminal.PrintError(fmt.Sprintf("Error during selection: %v", err))
		return true
	}

	if selection == "" {
		return true
	}

	content, err := terminal.LoadFileContentOCR(selection, state)
	if err != nil {
		if err.Error() != "command cancelled" {
			terminal.PrintError(fmt.Sprintf("Error loading content: %v", err))
		}
		return true
	}

	if content != "" {
		userPrompt := fmt.Sprintf("!s [Loaded from %s]:\n%s", selection, content)
		chatManager.AddUserMessage(content)
		chatManager.AddToHistory(userPrompt, "")
	}

	return true
}
