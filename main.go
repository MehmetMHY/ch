package main

import (
	"bufio"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"
	"time"

	"github.com/google/uuid"
	"github.com/sashabaranov/go-openai"
)

type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ChatHistory struct {
	Time int64  `json:"time"`
	User string `json:"user"`
	Bot  string `json:"bot"`
}

type Platform struct {
	Name      string            `json:"name"`
	BaseURL   string            `json:"base_url"`
	EnvName   string            `json:"env_name"`
	Models    PlatformModels    `json:"models"`
	Headers   map[string]string `json:"headers"`
}

type PlatformModels struct {
	URL         string `json:"url"`
	JSONPath    string `json:"json_name_path"`
	Headers     map[string]string `json:"headers"`
}

type Config struct {
	OpenAIAPIKey      string
	DefaultModel      string
	CurrentModel      string
	SystemPrompt      string
	ExitKey           string
	ModelSwitch       string
	TerminalInput     string
	ClearHistory      string
	HelpKey           string
	ExportChat        string
	PreferredEditor   string
	CurrentPlatform   string
	Platforms         map[string]Platform
}

var (
	config      Config
	client      *openai.Client
	messages    []ChatMessage
	chatHistory []ChatHistory
	streamingCancel context.CancelFunc
	isStreaming     bool
)

func init() {
	config = Config{
		OpenAIAPIKey:    os.Getenv("OPENAI_API_KEY"),
		DefaultModel:    "gpt-4o-mini",
		CurrentModel:    "gpt-4o-mini",
		SystemPrompt:    "You are a helpful assistant powered by Cha who provides concise, clear, and accurate answers. Be brief, but ensure the response fully addresses the question without leaving out important details. Always return any code or file output in a Markdown code fence, with syntax ```<language or filetype>\n...``` so it can be parsed automatically. Only do this when needed, no need to do this for responses just code segments and/or when directly asked to do so from the user.",
		ExitKey:         "!q",
		ModelSwitch:     "!sm",
		TerminalInput:   "!t",
		ClearHistory:    "!c",
		HelpKey:         "!h",
		ExportChat:      "!e",
		PreferredEditor: "hx",
		CurrentPlatform: "openai",
		Platforms: map[string]Platform{
			"groq": {
				Name:    "groq",
				BaseURL: "https://api.groq.com/openai/v1",
				EnvName: "GROQ_API_KEY",
				Models: PlatformModels{
					URL:      "https://api.groq.com/openai/v1/models",
					JSONPath: "data.id",
				},
			},
			"deepseek": {
				Name:    "deepseek",
				BaseURL: "https://api.deepseek.com",
				EnvName: "DEEP_SEEK_API_KEY",
				Models: PlatformModels{
					URL:      "https://api.deepseek.com/models",
					JSONPath: "data.id",
				},
			},
			"anthropic": {
				Name:    "anthropic",
				BaseURL: "https://api.anthropic.com/v1/",
				EnvName: "ANTHROPIC_API_KEY",
				Models: PlatformModels{
					URL:      "https://api.anthropic.com/v1/models",
					JSONPath: "data.id",
				},
			},
			"xai": {
				Name:    "xai",
				BaseURL: "https://api.x.ai/v1",
				EnvName: "XAI_API_KEY",
				Models: PlatformModels{
					URL:      "https://api.x.ai/v1/models",
					JSONPath: "data.id",
				},
			},
		},
	}

	messages = []ChatMessage{
		{Role: "system", Content: config.SystemPrompt},
	}
	chatHistory = []ChatHistory{
		{Time: time.Now().Unix(), User: config.SystemPrompt, Bot: ""},
	}
}

func main() {
	// Parse command line arguments
	var (
		helpFlag     = flag.Bool("h", false, "Show help")
		platformFlag = flag.String("p", "", "Switch platform (leave empty for interactive selection)")
		modelFlag    = flag.String("m", "", "Specify model to use")
	)
	
	flag.Parse()
	
	// Get remaining arguments after flags (these will be the query)
	remainingArgs := flag.Args()

	// Handle help flag
	if *helpFlag {
		showHelp()
		return
	}

	// Handle platform switching (silent for command-line usage)
	if *platformFlag != "" {
		result, err := autoSelectPlatform(*platformFlag, *modelFlag)
		if err != nil {
			fmt.Printf("\033[91m%v\033[0m\n", err)
			return
		}
		if result != nil {
			config.CurrentPlatform = result["platform_name"].(string)
			config.CurrentModel = result["picked_model"].(string)
		}
	}

	// Handle model switching (only if platform wasn't switched, as platform switching handles model too)
	if *modelFlag != "" && *platformFlag == "" {
		config.CurrentModel = *modelFlag
	}

	// Initialize client
	err := initializeClient()
	if err != nil {
		log.Fatalf("Failed to initialize client: %v", err)
	}

	// Set up signal handling for graceful interruption during streaming
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		for range sigChan {
			if isStreaming && streamingCancel != nil {
				fmt.Print("\r\033[K") // Clear current line
				streamingCancel()
			} else {
				// Allow Ctrl+C to exit when not streaming
				os.Exit(0)
			}
		}
	}()

	// Get remaining arguments (direct query)
	if len(remainingArgs) > 0 {
		// Non-interactive mode - process direct query
		query := strings.Join(remainingArgs, " ")
		err := processDirectQuery(query)
		if err != nil {
			fmt.Printf("\033[91m%v\033[0m\n", err)
		}
		return
	}

	// Interactive mode
	scanner := bufio.NewScanner(os.Stdin)
	
	for {
		// Check if stdin is from a pipe/redirect and we've processed everything
		if !isTerminal() && !scanner.Scan() {
			break
		}
		
		// Only print prompt if we're in an actual terminal
		if isTerminal() {
			fmt.Print("\033[94mUser: \033[0m")
		}
		
		// Read input if we haven't already
		var input string
		if isTerminal() {
			if !scanner.Scan() {
				break
			}
			input = strings.TrimSpace(scanner.Text())
		} else {
			input = strings.TrimSpace(scanner.Text())
		}
		
		if input == "" {
			continue
		}
		
		// Handle special commands
		if handleSpecialCommands(input) {
			// If not in terminal (piped input), exit after handling special commands
			if !isTerminal() {
				break
			}
			continue
		}
		
		// Add user message to history
		messages = append(messages, ChatMessage{Role: "user", Content: input})
		
		// Create chat history entry
		historyEntry := ChatHistory{
			Time: time.Now().Unix(),
			User: input,
			Bot:  "",
		}
		
		// Send to current platform
		response, err := sendChatRequest(input)
		if err != nil {
			if err.Error() == "request was interrupted" {
				// Don't add the user message to history if the request was interrupted
				messages = messages[:len(messages)-1]
				continue
			}
			fmt.Printf("\033[91m%v\033[0m\n", err)
			continue
		}
		
		// Add assistant response to messages and history
		messages = append(messages, ChatMessage{Role: "assistant", Content: response})
		historyEntry.Bot = response
		chatHistory = append(chatHistory, historyEntry)
	}
}

func showHelp() {
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
	for name := range config.Platforms {
		fmt.Printf("  - %s\n", name)
	}
	fmt.Println("\nInteractive commands:")
	fmt.Printf("  %s - Exit\n", config.ExitKey)
	fmt.Printf("  %s - Switch models\n", config.ModelSwitch)
	fmt.Printf("  %s - Terminal input mode\n", config.TerminalInput)
	fmt.Printf("  %s - Clear chat history\n", config.ClearHistory)
	fmt.Printf("  %s - Export chat to file\n", config.ExportChat)
	fmt.Printf("  %s - Show help\n", config.HelpKey)
	fmt.Println("  !p - Switch platforms (interactive)")
	fmt.Println("  !p [platform] - Switch to specific platform")
	fmt.Println("  !w [query] - Web search using SearXNG (requires SearXNG running on localhost:8080)")
}

func printTitle() {
	fmt.Printf("\033[93mChatting with %s Model: %s\033[0m\n", strings.ToUpper(config.CurrentPlatform), config.CurrentModel)
	fmt.Printf("\033[93mCommands:\033[0m\n")
	fmt.Printf("\033[93m  • %s - Exit\033[0m\n", config.ExitKey)
	fmt.Printf("\033[93m  • %s - Switch models\033[0m\n", config.ModelSwitch)
	fmt.Printf("\033[93m  • !p - Switch platforms\033[0m\n")
	fmt.Printf("\033[93m  • %s - Terminal input\033[0m\n", config.TerminalInput)
	fmt.Printf("\033[93m  • %s - Clear history\033[0m\n", config.ClearHistory)
	fmt.Printf("\033[93m  • %s - Export chat\033[0m\n", config.ExportChat)
	fmt.Printf("\033[93m  • %s - Help\033[0m\n", config.HelpKey)
	fmt.Printf("\033[93m  • !w [query] - Web search\033[0m\n")
}

func initializeClient() error {
	if config.CurrentPlatform == "openai" {
		if config.OpenAIAPIKey == "" {
			return fmt.Errorf("OPENAI_API_KEY environment variable is required")
		}
		client = openai.NewClient(config.OpenAIAPIKey)
		return nil
	}

	platform, exists := config.Platforms[config.CurrentPlatform]
	if !exists {
		return fmt.Errorf("platform %s not found", config.CurrentPlatform)
	}

	apiKey := os.Getenv(platform.EnvName)
	if apiKey == "" {
		return fmt.Errorf("%s environment variable is required for %s", platform.EnvName, platform.Name)
	}

	clientConfig := openai.DefaultConfig(apiKey)
	clientConfig.BaseURL = platform.BaseURL
	client = openai.NewClientWithConfig(clientConfig)
	
	return nil
}


func processDirectQuery(query string) error {
	// Handle special commands first
	if handleSpecialCommands(query) {
		return nil // Exit cleanly after handling special commands
	}
	
	// Add user message to history
	messages = append(messages, ChatMessage{Role: "user", Content: query})
	
	// Send request
	response, err := sendChatRequest(query)
	if err != nil {
		if err.Error() == "request was interrupted" {
			// Don't add the user message to history if the request was interrupted
			messages = messages[:len(messages)-1]
			return nil
		}
		return err
	}
	
	// Add response to history
	messages = append(messages, ChatMessage{Role: "assistant", Content: response})
	
	return nil
}

func handleSpecialCommands(input string) bool {
	switch {
	case input == config.ExitKey:
		fmt.Println("Goodbye!")
		os.Exit(0)
		return true
		
	case input == config.HelpKey || input == "help":
		printTitle()
		return true
		
	case input == config.ClearHistory:
		messages = []ChatMessage{
			{Role: "system", Content: config.SystemPrompt},
		}
		chatHistory = []ChatHistory{
			{Time: time.Now().Unix(), User: config.SystemPrompt, Bot: ""},
		}
		fmt.Println("\033[93mChat history cleared.\033[0m")
		return true
		
	case input == config.ModelSwitch:
		selectModel()
		return true
		
	case strings.HasPrefix(input, config.ModelSwitch+" "):
		modelName := strings.TrimPrefix(input, config.ModelSwitch+" ")
		config.CurrentModel = modelName
		fmt.Printf("\033[95mSwitched to model: %s\033[0m\n", config.CurrentModel)
		return true
		
	case input == "!p":
		result, err := autoSelectPlatform("", "")
		if err != nil {
			fmt.Printf("\033[91m%v\033[0m\n", err)
		} else if result != nil {
			config.CurrentPlatform = result["platform_name"].(string)
			config.CurrentModel = result["picked_model"].(string)
			err = initializeClient()
			if err != nil {
				fmt.Printf("\033[91mError initializing client: %v\033[0m\n", err)
			}
		}
		return true
		
	case strings.HasPrefix(input, "!p "):
		platformName := strings.TrimPrefix(input, "!p ")
		result, err := autoSelectPlatform(platformName, "")
		if err != nil {
			fmt.Printf("\033[91m%v\033[0m\n", err)
		} else if result != nil {
			config.CurrentPlatform = result["platform_name"].(string)
			config.CurrentModel = result["picked_model"].(string)
			err = initializeClient()
			if err != nil {
				fmt.Printf("\033[91mError initializing client: %v\033[0m\n", err)
			}
		}
		return true
		
	case strings.HasPrefix(input, "!w "):
		searchQuery := strings.TrimPrefix(input, "!w ")
		if strings.TrimSpace(searchQuery) == "" {
			fmt.Printf("\033[91mPlease provide a search query after !w\033[0m\n")
			return true
		}
		
		// Perform SearXNG search
		searchResults, err := performSearXNGSearch(searchQuery)
		if err != nil {
			fmt.Printf("\033[91m%v\033[0m\n", err)
			return true
		}
		
		// Create context for the AI model with search results
		searchContext := formatSearchResults(searchResults, searchQuery)
		
		// Add search context to messages
		messages = append(messages, ChatMessage{Role: "user", Content: searchContext})
		
		// Create chat history entry
		historyEntry := ChatHistory{
			Time: time.Now().Unix(),
			User: fmt.Sprintf("!w %s", searchQuery),
			Bot:  "",
		}
		
		// Send to AI model
		response, err := sendChatRequest(searchContext)
		if err != nil {
			if err.Error() == "request was interrupted" {
				// Don't add the search query to history if the request was interrupted
				messages = messages[:len(messages)-1]
				return true
			}
			fmt.Printf("\033[91mError generating response: %v\033[0m\n", err)
			return true
		}
		
		// Add response to messages and history
		messages = append(messages, ChatMessage{Role: "assistant", Content: response})
		historyEntry.Bot = response
		chatHistory = append(chatHistory, historyEntry)
		
		return true
		
	case input == "!w":
		fmt.Printf("\033[91mPlease provide a search query after !w\033[0m\n")
		return true
		
	case input == config.TerminalInput:
		terminalInput()
		return true
		
	case input == config.ExportChat:
		exportChatHistory()
		return true
		
	default:
		return false
	}
}

func selectModel() {
	var allModels []string
	var err error
	
	if config.CurrentPlatform == "openai" {
		models, modelErr := client.ListModels(context.Background())
		if modelErr != nil {
			fmt.Printf("\033[91mError fetching models: %v\033[0m\n", modelErr)
			return
		}
		
		for _, model := range models.Models {
			allModels = append(allModels, model.ID)
		}
	} else {
		// Fetch models from third-party platform
		platform := config.Platforms[config.CurrentPlatform]
		allModels, err = fetchPlatformModels(platform)
		if err != nil {
			fmt.Printf("\033[91mError fetching models: %v\033[0m\n", err)
			return
		}
	}
	
	if len(allModels) == 0 {
		fmt.Println("\033[91mNo models found\033[0m")
		return
	}
	
	// Use fzf for model selection
	selectedModel, err := fzfSelect(allModels, "Select a model: ")
	if err != nil {
		fmt.Printf("\033[91mError selecting model: %v\033[0m\n", err)
		return
	}
	
	if selectedModel != "" {
		config.CurrentModel = selectedModel
		fmt.Printf("\033[95mSwitched to model: %s\033[0m\n", config.CurrentModel)
	}
}

func fetchPlatformModels(platform Platform) ([]string, error) {
	// Create HTTP client
	httpClient := &http.Client{Timeout: 10 * time.Second}
	
	// Create request
	req, err := http.NewRequest("GET", platform.Models.URL, nil)
	if err != nil {
		return nil, err
	}
	
	// Add authentication header
	apiKey := os.Getenv(platform.EnvName)
	if apiKey == "" {
		return nil, fmt.Errorf("%s environment variable not set", platform.EnvName)
	}
	
	if platform.Name == "anthropic" {
		req.Header.Set("x-api-key", apiKey)
		req.Header.Set("anthropic-version", "2023-06-01")
	} else {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}
	req.Header.Set("Content-Type", "application/json")
	
	// Make request
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	
	// Parse response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	
	var jsonData interface{}
	err = json.Unmarshal(body, &jsonData)
	if err != nil {
		return nil, err
	}
	
	// Extract model names based on JSON path
	return extractModelsFromJSON(jsonData, platform.Models.JSONPath)
}

func extractModelsFromJSON(data interface{}, jsonPath string) ([]string, error) {
	parts := strings.Split(jsonPath, ".")
	
	current := data
	
	// Navigate to the data array
	for i, part := range parts[:len(parts)-1] {
		if dataMap, ok := current.(map[string]interface{}); ok {
			if val, exists := dataMap[part]; exists {
				current = val
			} else {
				return nil, fmt.Errorf("path part %s not found", part)
			}
		} else {
			return nil, fmt.Errorf("expected object at part %d", i)
		}
	}
	
	// Extract the final field from each item
	fieldName := parts[len(parts)-1]
	var models []string
	
	if dataArray, ok := current.([]interface{}); ok {
		for _, item := range dataArray {
			if itemMap, ok := item.(map[string]interface{}); ok {
				if modelName, exists := itemMap[fieldName]; exists {
					if nameStr, ok := modelName.(string); ok {
						models = append(models, nameStr)
					}
				}
			}
		}
	}
	
	return models, nil
}

func exportChatHistory() {
	if len(chatHistory) <= 1 {
		return // No chat history to export
	}
	
	// Generate unique filename with UUID
	chatID := uuid.New().String()
	filename := fmt.Sprintf("cha_go_%s.txt", chatID)
	
	// Get current working directory
	currentDir, err := os.Getwd()
	if err != nil {
		return
	}
	
	fullPath := filepath.Join(currentDir, filename)
	
	// Create file content
	var content strings.Builder
	content.WriteString(fmt.Sprintf("Cha Go Chat Export\n"))
	content.WriteString(fmt.Sprintf("Platform: %s\n", config.CurrentPlatform))
	content.WriteString(fmt.Sprintf("Model: %s\n", config.CurrentModel))
	content.WriteString(fmt.Sprintf("Exported: %s\n", time.Now().Format("2006-01-02 15:04:05")))
	content.WriteString(strings.Repeat("=", 50) + "\n\n")
	
	// Add chat messages (skip the first system message)
	for _, entry := range chatHistory[1:] {
		if entry.User != "" {
			content.WriteString(fmt.Sprintf("User: %s\n\n", entry.User))
		}
		if entry.Bot != "" {
			content.WriteString(fmt.Sprintf("Assistant: %s\n\n", entry.Bot))
			content.WriteString(strings.Repeat("-", 30) + "\n\n")
		}
	}
	
	// Write to file
	err = os.WriteFile(fullPath, []byte(content.String()), 0644)
	if err != nil {
		return
	}
	
	// Print only the full file path
	fmt.Println(fullPath)
}


func autoSelectPlatform(platformKey, modelName string) (map[string]interface{}, error) {
	// If platform not specified, show platform selector
	if platformKey == "" {
		var platforms []string
		platforms = append(platforms, "openai")
		for name := range config.Platforms {
			platforms = append(platforms, name)
		}
		
		selected, err := fzfSelect(platforms, "Select a platform: ")
		if err != nil {
			return nil, err
		}
		
		if selected == "" {
			return nil, fmt.Errorf("no platform selected")
		}
		
		platformKey = selected
	}
	
	// Handle OpenAI with default model
	if platformKey == "openai" {
		finalModel := modelName
		if finalModel == "" {
			finalModel = "gpt-4o-mini" // Only OpenAI gets a default
		}
		
		return map[string]interface{}{
			"platform_name": "openai",
			"picked_model":  finalModel,
			"base_url":      "",
			"env_name":      "OPENAI_API_KEY",
		}, nil
	}
	
	// Handle third-party platforms
	platform, exists := config.Platforms[platformKey]
	if !exists {
		return nil, fmt.Errorf("platform %s not supported", platformKey)
	}
	
	finalModel := modelName
	var modelsList []string
	
	// If no model specified, user MUST select one
	if finalModel == "" {
		var err error
		modelsList, err = fetchPlatformModels(platform)
		if err != nil {
			return nil, fmt.Errorf("failed to retrieve models: %v", err)
		}
		
		if len(modelsList) == 0 {
			return nil, fmt.Errorf("no models found or returned in unexpected format")
		}
		
		selected, err := fzfSelect(modelsList, "Select a model: ")
		if err != nil {
			return nil, err
		}
		
		if selected == "" {
			return nil, fmt.Errorf("no model selected")
		}
		
		finalModel = selected
	}
	
	return map[string]interface{}{
		"platform_name": platformKey,
		"picked_model":  finalModel,
		"base_url":      platform.BaseURL,
		"env_name":      platform.EnvName,
		"models":        modelsList,
	}, nil
}

func fzfSelect(items []string, prompt string) (string, error) {
	cmd := exec.Command("fzf", "--reverse", "--height=40%", "--border", "--prompt="+prompt)
	cmd.Stdin = strings.NewReader(strings.Join(items, "\n"))
	
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	
	return strings.TrimSpace(string(output)), nil
}

func isReasoningModel(modelName string) bool {
	matched, _ := regexp.MatchString(`^o\d+`, modelName)
	return matched
}

func showLoadingAnimation(message string, done chan bool) {
	chars := []string{"⣾", "⣽", "⣻", "⢿", "⡿", "⣟", "⣯", "⣷"}
	i := 0
	for {
		select {
		case <-done:
			fmt.Print("\r\033[K") // Clear the line
			return
		default:
			fmt.Printf("\r\033[93m%s %s\033[0m", chars[i], message)
			i = (i + 1) % len(chars)
			time.Sleep(100 * time.Millisecond)
		}
	}
}

func terminalInput() {
	// Create tmp directory if it doesn't exist
	tmpDir := "/tmp"
	if _, err := os.Stat(tmpDir); os.IsNotExist(err) {
		err = os.MkdirAll(tmpDir, 0755)
		if err != nil {
			fmt.Printf("\033[91mError creating tmp directory: %v\033[0m\n", err)
			return
		}
	}
	
	// Create temporary file
	tmpFile, err := ioutil.TempFile(tmpDir, "cha-go-*.txt")
	if err != nil {
		fmt.Printf("\033[91mError creating temp file: %v\033[0m\n", err)
		return
	}
	tmpFilePath := tmpFile.Name()
	tmpFile.Close()
	
	// Clean up temp file when done
	defer os.Remove(tmpFilePath)
	
	
	// Open editor with temp file
	cmd := exec.Command(config.PreferredEditor, tmpFilePath)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	
	err = cmd.Run()
	if err != nil {
		fmt.Printf("\033[91mError running editor: %v\033[0m\n", err)
		return
	}
	
	// Read content from temp file
	content, err := ioutil.ReadFile(tmpFilePath)
	if err != nil {
		fmt.Printf("\033[91mError reading temp file: %v\033[0m\n", err)
		return
	}
	
	input := strings.TrimSpace(string(content))
	if input == "" {
		fmt.Println("\033[91mNo input provided\033[0m")
		return
	}
	
	fmt.Printf("\033[94m> %s\033[0m\n", strings.ReplaceAll(input, "\n", "\n> "))
	
	// Add to messages and process
	messages = append(messages, ChatMessage{Role: "user", Content: input})
	
	historyEntry := ChatHistory{
		Time: time.Now().Unix(),
		User: input,
		Bot:  "",
	}
	
	response, err := sendChatRequest(input)
	if err != nil {
		if err.Error() == "request was interrupted" {
			// Don't add the user message to history if the request was interrupted
			messages = messages[:len(messages)-1]
			return
		}
		fmt.Printf("\033[91m%v\033[0m\n", err)
		return
	}
	
	messages = append(messages, ChatMessage{Role: "assistant", Content: response})
	historyEntry.Bot = response
	chatHistory = append(chatHistory, historyEntry)
}

func sendChatRequest(userInput string) (string, error) {
	// Convert our messages to OpenAI format
	var openaiMessages []openai.ChatCompletionMessage
	for _, msg := range messages {
		openaiMessages = append(openaiMessages, openai.ChatCompletionMessage{
			Role:    msg.Role,
			Content: msg.Content,
		})
	}
	
	// Check if this is a reasoning model (o1, o3, etc.)
	if isReasoningModel(config.CurrentModel) {
		// For reasoning models, use non-streaming with loading animation
		req := openai.ChatCompletionRequest{
			Model:    config.CurrentModel,
			Messages: openaiMessages,
			Stream:   false,
		}
		
		// Start loading animation
		done := make(chan bool)
		go showLoadingAnimation("Thinking", done)
		
		// Create context for cancellation
		ctx, cancel := context.WithCancel(context.Background())
		isStreaming = true
		streamingCancel = cancel
		
		resp, err := client.CreateChatCompletion(ctx, req)
		
		// Stop loading animation and reset streaming state
		isStreaming = false
		streamingCancel = nil
		done <- true
		
		if err != nil {
			if ctx.Err() == context.Canceled {
				return "", fmt.Errorf("request was interrupted")
			}
			return "", err
		}
		
		if len(resp.Choices) > 0 {
			fullResponse := resp.Choices[0].Message.Content
			fmt.Printf("\033[92m%s\033[0m\n", fullResponse)
			return fullResponse, nil
		}
		
		return "", fmt.Errorf("no response content")
	} else {
		// For regular models, use streaming
		req := openai.ChatCompletionRequest{
			Model:    config.CurrentModel,
			Messages: openaiMessages,
			Stream:   true,
		}
		
		// Create context for cancellation
		ctx, cancel := context.WithCancel(context.Background())
		isStreaming = true
		streamingCancel = cancel
		
		stream, err := client.CreateChatCompletionStream(ctx, req)
		if err != nil {
			isStreaming = false
			streamingCancel = nil
			return "", err
		}
		defer func() {
			stream.Close()
			isStreaming = false
			streamingCancel = nil
		}()
		
		var response strings.Builder
		
		for {
			completion, err := stream.Recv()
			if err != nil {
				if err == io.EOF {
					break
				}
				if ctx.Err() == context.Canceled {
					return response.String(), nil // Return partial response on cancellation
				}
				return "", err
			}
			
			if len(completion.Choices) > 0 {
				delta := completion.Choices[0].Delta.Content
				if delta != "" {
					fmt.Print("\033[92m" + delta + "\033[0m")
					response.WriteString(delta)
				}
			}
		}
		
		fmt.Println() // New line after streaming
		return response.String(), nil
	}
}

// SearXNG API response structures
type SearXNGResponse struct {
	Query         string            `json:"query"`
	NumberOfResults int             `json:"number_of_results"`
	Results       []SearXNGResult   `json:"results"`
	Infoboxes     []interface{}     `json:"infoboxes"`
	Suggestions   []string          `json:"suggestions"`
	Answers       []interface{}     `json:"answers"`
	Corrections   []interface{}     `json:"corrections"`
	Unresponsive  []interface{}     `json:"unresponsive_engines"`
}

type SearXNGResult struct {
	URL         string `json:"url"`
	Title       string `json:"title"`
	Content     string `json:"content"`
	Engine      string `json:"engine"`
	ParsedURL   []string `json:"parsed_url"`
	Template    string `json:"template"`
	Engines     []string `json:"engines"`
	Positions   []int    `json:"positions"`
	Score       float64  `json:"score"`
	Category    string   `json:"category"`
}

func performSearXNGSearch(query string) ([]SearXNGResult, error) {
	// Use localhost:8080 as the default SearXNG instance
	apiURL := "http://localhost:8080/search"
	
	// Create HTTP client with timeout
	client := &http.Client{Timeout: 10 * time.Second}
	
	// Build parameters for the search
	params := url.Values{}
	params.Add("q", query)
	params.Add("format", "json")
	
	fullURL := fmt.Sprintf("%s?%s", apiURL, params.Encode())
	
	// Create request with proper headers
	req, err := http.NewRequest("GET", fullURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %v", err)
	}
	
	// Add headers that SearXNG expects
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	
	// Make the search request
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("SearXNG is not running or not accessible at localhost:8080")
	}
	defer resp.Body.Close()
	
	if resp.StatusCode == 403 {
		return nil, fmt.Errorf("SearXNG is blocking API requests. Please check your SearXNG configuration")
	}
	
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("SearXNG search failed with status: %d", resp.StatusCode)
	}
	
	var searchResponse SearXNGResponse
	err = json.NewDecoder(resp.Body).Decode(&searchResponse)
	if err != nil {
		return nil, fmt.Errorf("failed to parse SearXNG response: %v", err)
	}
	
	return searchResponse.Results, nil
}

func formatSearchResults(results []SearXNGResult, query string) string {
	if len(results) == 0 {
		return fmt.Sprintf("I searched for '%s' but didn't find any results. Please try a different query.", query)
	}
	
	var context strings.Builder
	context.WriteString(fmt.Sprintf("I searched for '%s' and found the following results. Please provide a comprehensive answer based on these sources using IEEE citation format:\n\n", query))
	
	// Include up to 8 results to avoid token limits
	maxResults := 8
	if len(results) > maxResults {
		results = results[:maxResults]
	}
	
	for i, result := range results {
		context.WriteString(fmt.Sprintf("[%d] %s\n", i+1, result.Title))
		context.WriteString(fmt.Sprintf("URL: %s\n", result.URL))
		
		// Clean and truncate content to avoid token limits
		content := strings.TrimSpace(result.Content)
		if len(content) > 300 {
			content = content[:300] + "..."
		}
		context.WriteString(fmt.Sprintf("Content: %s\n\n", content))
	}
	
	context.WriteString("Please provide a comprehensive answer based on these search results. Use IEEE citation format with citations like [1], [2], etc., and include a References section at the end listing all sources with their URLs in the format:\n\nReferences:\n[1] Title, URL\n[2] Title, URL\netc.")
	
	return context.String()
}

func isTerminal() bool {
	fileInfo, _ := os.Stdin.Stat()
	return (fileInfo.Mode() & os.ModeCharDevice) != 0
}
