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
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
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
)

func init() {
	config = Config{
		OpenAIAPIKey:    os.Getenv("OPENAI_API_KEY"),
		DefaultModel:    "gpt-4o-mini",
		CurrentModel:    "gpt-4o-mini",
		SystemPrompt:    "You are a helpful assistant powered by a simple Go chat client. Be concise, clear, and accurate.",
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
			fmt.Printf("\033[91mError: %v\033[0m\n", err)
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

	// Get remaining arguments (direct query)
	if len(remainingArgs) > 0 {
		// Non-interactive mode - process direct query
		query := strings.Join(remainingArgs, " ")
		err := processDirectQuery(query)
		if err != nil {
			fmt.Printf("\033[91mError: %v\033[0m\n", err)
		}
		return
	}

	// Interactive mode
	scanner := bufio.NewScanner(os.Stdin)
	
	for {
		fmt.Print("\033[94mUser: \033[0m")
		if !scanner.Scan() {
			break
		}
		
		input := strings.TrimSpace(scanner.Text())
		
		if input == "" {
			continue
		}
		
		// Handle special commands
		if handleSpecialCommands(input) {
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
			fmt.Printf("\033[91mError: %v\033[0m\n", err)
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
	// Add user message to history
	messages = append(messages, ChatMessage{Role: "user", Content: query})
	
	// Send request
	response, err := sendChatRequest(query)
	if err != nil {
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
			fmt.Printf("\033[91mError: %v\033[0m\n", err)
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
			fmt.Printf("\033[91mError: %v\033[0m\n", err)
		} else if result != nil {
			config.CurrentPlatform = result["platform_name"].(string)
			config.CurrentModel = result["picked_model"].(string)
			err = initializeClient()
			if err != nil {
				fmt.Printf("\033[91mError initializing client: %v\033[0m\n", err)
			}
		}
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
		fmt.Printf("\033[91mError: %v\033[0m\n", err)
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
		
		ctx := context.Background()
		resp, err := client.CreateChatCompletion(ctx, req)
		
		// Stop loading animation
		done <- true
		
		if err != nil {
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
		
		ctx := context.Background()
		stream, err := client.CreateChatCompletionStream(ctx, req)
		if err != nil {
			return "", err
		}
		defer stream.Close()
		
		var response strings.Builder
		
		for {
			completion, err := stream.Recv()
			if err != nil {
				if err == io.EOF {
					break
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