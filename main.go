package main

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"time"

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

type Config struct {
	OpenAIAPIKey    string
	DefaultModel    string
	CurrentModel    string
	SystemPrompt    string
	ExitKey         string
	ModelSwitch     string
	TerminalInput   string
	ClearHistory    string
	HelpKey         string
	PreferredEditor string
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
		PreferredEditor: "hx",
	}

	if config.OpenAIAPIKey == "" {
		log.Fatal("OPENAI_API_KEY environment variable is required")
	}

	client = openai.NewClient(config.OpenAIAPIKey)
	messages = []ChatMessage{
		{Role: "system", Content: config.SystemPrompt},
	}
	chatHistory = []ChatHistory{
		{Time: time.Now().Unix(), User: config.SystemPrompt, Bot: ""},
	}
}

func main() {
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
		
		// Send to OpenAI
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

func printTitle() {
	fmt.Printf("\033[93mChatting with OpenAI Model: %s - Commands: '%s' to exit, '%s' to switch models, '%s' for terminal input mode, '%s' to clear chat history, '%s' for help\033[0m\n", 
		config.CurrentModel, config.ExitKey, config.ModelSwitch, config.TerminalInput, config.ClearHistory, config.HelpKey)
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
		
	case input == config.TerminalInput:
		terminalInput()
		return true
		
	default:
		return false
	}
}

func selectModel() {
	models, err := client.ListModels(context.Background())
	if err != nil {
		fmt.Printf("\033[91mError fetching models: %v\033[0m\n", err)
		return
	}
	
	// Get all models
	var allModels []string
	for _, model := range models.Models {
		allModels = append(allModels, model.ID)
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