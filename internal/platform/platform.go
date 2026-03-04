package platform

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/MehmetMHY/ch/pkg/types"
	"github.com/sashabaranov/go-openai"
)

// Manager handles AI platform operations
type Manager struct {
	client *openai.Client
	config *types.Config
}

// NewManager creates a new platform manager
func NewManager(config *types.Config) *Manager {
	return &Manager{
		config: config,
	}
}

// Initialize initializes the AI client for the current platform
func (m *Manager) Initialize() error {
	if m.config.CurrentPlatform == "openai" {
		apiKey := os.Getenv("OPENAI_API_KEY")
		if apiKey == "" {
			return fmt.Errorf("OPENAI_API_KEY environment variable is required for OpenAI platform")
		}
		m.client = openai.NewClient(apiKey)
		m.config.CurrentBaseURL = ""
		return nil
	}

	platform, exists := m.config.Platforms[m.config.CurrentPlatform]
	if !exists {
		return fmt.Errorf("platform %s not found", m.config.CurrentPlatform)
	}

	var apiKey string
	if platform.Name != "ollama" {
		apiKey = os.Getenv(platform.EnvName)
		if apiKey == "" {
			return fmt.Errorf("%s environment variable is required for %s", platform.EnvName, platform.Name)
		}
	}

	clientConfig := openai.DefaultConfig(apiKey)
	// Use CurrentBaseURL if set, otherwise use the first URL if multi-URL, otherwise use single URL
	baseURL := m.config.CurrentBaseURL
	if baseURL == "" {
		if platform.BaseURL.IsMulti() && len(platform.BaseURL.Multi) > 0 {
			baseURL = platform.BaseURL.Multi[0]
		} else {
			baseURL = platform.BaseURL.Single
		}
	}
	m.config.CurrentBaseURL = baseURL
	clientConfig.BaseURL = baseURL
	m.client = openai.NewClientWithConfig(clientConfig)

	return nil
}

// SendChatRequest sends a chat request to the current platform
func (m *Manager) SendChatRequest(messages []types.ChatMessage, model string, streamingCancel *func(), isStreaming *bool) (string, error) {
	var openaiMessages []openai.ChatCompletionMessage

	// Merge consecutive user messages to handle cases like file loading + follow-up question
	mergedMessages := m.mergeConsecutiveUserMessages(messages)

	for _, msg := range mergedMessages {
		openaiMessages = append(openaiMessages, openai.ChatCompletionMessage{
			Role:    msg.Role,
			Content: msg.Content,
		})
	}

	if m.IsReasoningModel(model) {
		return m.sendNonStreamingRequest(openaiMessages, model, streamingCancel, isStreaming)
	}

	return m.sendStreamingRequest(openaiMessages, model, streamingCancel, isStreaming)
}

// mergeConsecutiveUserMessages combines consecutive user messages into one
// This handles cases where file content is loaded as one message, then a question is asked
func (m *Manager) mergeConsecutiveUserMessages(messages []types.ChatMessage) []types.ChatMessage {
	if len(messages) <= 1 {
		return messages
	}

	var result []types.ChatMessage
	var lastUserContent []string

	for _, msg := range messages {
		if msg.Role == "user" {
			lastUserContent = append(lastUserContent, msg.Content)
		} else {
			// Non-user message: flush any accumulated user messages
			if len(lastUserContent) > 0 {
				result = append(result, types.ChatMessage{
					Role:    "user",
					Content: strings.Join(lastUserContent, "\n\n"),
				})
				lastUserContent = nil
			}
			result = append(result, msg)
		}
	}

	// Flush any remaining user messages
	if len(lastUserContent) > 0 {
		result = append(result, types.ChatMessage{
			Role:    "user",
			Content: strings.Join(lastUserContent, "\n\n"),
		})
	}

	return result
}

// ListModels returns available models for the current platform
func (m *Manager) ListModels() ([]string, error) {
	if m.config.CurrentPlatform == "openai" {
		models, err := m.client.ListModels(context.Background())
		if err != nil {
			return nil, err
		}

		var modelNames []string
		for _, model := range models.Models {
			modelNames = append(modelNames, model.ID)
		}
		sort.Strings(modelNames)
		return modelNames, nil
	}

	platform := m.config.Platforms[m.config.CurrentPlatform]
	models, err := m.fetchPlatformModels(platform)
	if err != nil {
		return nil, err
	}
	sort.Strings(models)
	return models, nil
}

// SelectPlatform handles platform selection and model selection
func (m *Manager) SelectPlatform(platformKey, modelName string, fzfSelector func([]string, string) (string, error)) (map[string]interface{}, error) {
	platformChanged := false
	if platformKey == "" {
		var platforms []string
		platforms = append(platforms, "openai")
		for name := range m.config.Platforms {
			platforms = append(platforms, name)
		}
		sort.Strings(platforms)

		selected, err := fzfSelector(platforms, "platform: ")
		if err != nil {
			return nil, err
		}

		if selected == "" {
			return nil, fmt.Errorf("no platform selected")
		}

		platformKey = selected
		platformChanged = true
	}

	if platformKey == "openai" {
		finalModel := modelName
		if platformChanged || finalModel == "" {
			apiKey := os.Getenv("OPENAI_API_KEY")
			if apiKey == "" {
				return nil, fmt.Errorf("OPENAI_API_KEY environment variable is required for OpenAI platform")
			}
			client := openai.NewClient(apiKey)
			models, err := client.ListModels(context.Background())
			if err != nil {
				return nil, err
			}

			var modelNames []string
			for _, model := range models.Models {
				modelNames = append(modelNames, model.ID)
			}
			sort.Strings(modelNames)

			selected, err := fzfSelector(modelNames, "model: ")
			if err != nil {
				return nil, err
			}

			if selected == "" {
				return nil, fmt.Errorf("no model selected")
			}
			finalModel = selected
		}

		return map[string]interface{}{
			"platform_name": "openai",
			"picked_model":  finalModel,
			"base_url":      "",
			"env_name":      "OPENAI_API_KEY",
		}, nil
	}

	platform, exists := m.config.Platforms[platformKey]
	if !exists {
		return nil, fmt.Errorf("platform %s not supported", platformKey)
	}

	// Handle multi-URL platforms (e.g., Amazon Bedrock with multiple regions)
	selectedURL := platform.BaseURL.Single
	if platform.BaseURL.IsMulti() {
		selected, err := fzfSelector(platform.BaseURL.Multi, "region: ")
		if err != nil {
			return nil, err
		}

		if selected == "" {
			return nil, fmt.Errorf("no region selected")
		}

		selectedURL = selected
	}

	finalModel := modelName
	var modelsList []string

	if finalModel == "" || platformChanged {
		var err error
		modelsList, err = m.fetchPlatformModels(platform)
		if err != nil {
			return nil, fmt.Errorf("failed to retrieve models: %v", err)
		}

		if len(modelsList) == 0 {
			return nil, fmt.Errorf("no models found or returned in unexpected format")
		}
		sort.Strings(modelsList)

		selected, err := fzfSelector(modelsList, "model: ")
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
		"base_url":      selectedURL,
		"env_name":      platform.EnvName,
		"models":        modelsList,
	}, nil
}

// FetchAllModelsAsync fetches all models from all platforms asynchronously
// Returns a list of models formatted as "platform|model_name"
// Only fetches from platforms where API keys are defined and not empty
func (m *Manager) FetchAllModelsAsync() ([]string, error) {
	var wg sync.WaitGroup
	results := make(chan string)
	done := make(chan bool)
	var models []string
	var mu sync.Mutex

	// Create a list of all platforms to fetch from
	platformsToFetch := []struct {
		name     string
		platform types.Platform
	}{
		{"openai", types.Platform{}}, // OpenAI is a special case
	}

	// Add other platforms from config
	for name, platform := range m.config.Platforms {
		platformsToFetch = append(platformsToFetch, struct {
			name     string
			platform types.Platform
		}{name, platform})
	}

	// Goroutine to collect results
	go func() {
		for model := range results {
			mu.Lock()
			models = append(models, model)
			mu.Unlock()
		}
		done <- true
	}()

	// Fetch models from each platform concurrently
	for _, p := range platformsToFetch {
		platformName := p.name
		platformConfig := p.platform

		wg.Add(1)
		go func(name string, config types.Platform) {
			defer wg.Done()

			// Special handling for OpenAI
			if name == "openai" {
				apiKey := os.Getenv("OPENAI_API_KEY")
				if apiKey == "" {
					return // Skip if API key is not set
				}

				client := openai.NewClient(apiKey)
				ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
				defer cancel()

				modelList, err := client.ListModels(ctx)
				if err != nil {
					return // Silently ignore errors
				}

				for _, model := range modelList.Models {
					platformNameFormatted := strings.ReplaceAll(name, " ", "-")
					results <- fmt.Sprintf("%s|%s", platformNameFormatted, model.ID)
				}
				return
			}

			// Check if API key is defined and not empty
			apiKey := os.Getenv(platformConfig.EnvName)
			if apiKey == "" && platformConfig.Name != "ollama" {
				return // Skip if API key is not set
			}

			// Fetch models from this platform
			modelList, err := m.fetchPlatformModels(platformConfig)
			if err != nil {
				return // Silently ignore errors
			}

			for _, model := range modelList {
				platformNameFormatted := strings.ReplaceAll(name, " ", "-")
				results <- fmt.Sprintf("%s|%s", platformNameFormatted, model)
			}
		}(platformName, platformConfig)
	}

	// Close results channel when all goroutines are done
	go func() {
		wg.Wait()
		close(results)
	}()

	// Wait for collector to finish
	<-done

	if len(models) == 0 {
		return nil, fmt.Errorf("no models found from any platform")
	}

	return models, nil
}

// isSlowModel checks if the model matches any user-configured slow model patterns.
// Patterns are configured via "slow_model_patterns" in ~/.ch/config.json.
// By default, no models are considered slow (empty list).
func (m *Manager) isSlowModel(modelName string) bool {
	for _, pattern := range m.config.SlowModelPatterns {
		matched, _ := regexp.MatchString(pattern, modelName)
		if matched {
			return true
		}
	}
	return false
}

// IsReasoningModel checks if the model is a reasoning model (like o1, o2, etc.)
func (m *Manager) IsReasoningModel(modelName string) bool {
	return m.isSlowModel(modelName)
}

func (m *Manager) sendNonStreamingRequest(openaiMessages []openai.ChatCompletionMessage, model string, streamingCancel *func(), isStreaming *bool) (string, error) {
	req := openai.ChatCompletionRequest{
		Model:    model,
		Messages: openaiMessages,
		Stream:   false,
	}

	ctx, cancel := context.WithCancel(context.Background())
	*isStreaming = true
	*streamingCancel = cancel

	resp, err := m.client.CreateChatCompletion(ctx, req)

	*isStreaming = false
	*streamingCancel = nil

	if err != nil {
		if ctx.Err() == context.Canceled {
			return "", fmt.Errorf("request was interrupted")
		}
		return "", err
	}

	if len(resp.Choices) > 0 {
		fullResponse := resp.Choices[0].Message.Content
		return fullResponse, nil
	}

	return "", fmt.Errorf("no response content")
}

func (m *Manager) sendStreamingRequest(openaiMessages []openai.ChatCompletionMessage, model string, streamingCancel *func(), isStreaming *bool) (string, error) {
	req := openai.ChatCompletionRequest{
		Model:    model,
		Messages: openaiMessages,
		Stream:   true,
	}

	ctx, cancel := context.WithCancel(context.Background())
	*isStreaming = true
	*streamingCancel = cancel

	stream, err := m.client.CreateChatCompletionStream(ctx, req)
	if err != nil {
		*isStreaming = false
		*streamingCancel = nil
		return "", err
	}
	defer func() {
		stream.Close()
		*isStreaming = false
		*streamingCancel = nil
	}()

	type streamChunk struct {
		Choices []struct {
			Delta struct {
				Content          string `json:"content"`
				ReasoningContent string `json:"reasoning_content"`
				Reasoning        string `json:"reasoning"`
			} `json:"delta"`
		} `json:"choices"`
	}

	var response strings.Builder
	wasReasoning := false
	lastReasoningEndsWithNewline := false
	insideThinkTag := false
	justExitedThinkTag := false

	for {
		rawBytes, err := stream.RecvRaw()
		if err != nil {
			if err == io.EOF {
				break
			}
			if ctx.Err() == context.Canceled {
				return response.String(), nil
			}
			return "", err
		}

		var chunk streamChunk
		if err := json.Unmarshal(rawBytes, &chunk); err != nil || len(chunk.Choices) == 0 {
			continue
		}

		delta := chunk.Choices[0].Delta
		reasoning := delta.Reasoning + delta.ReasoningContent

		if reasoning != "" {
			wasReasoning = true
			lastReasoningEndsWithNewline = strings.HasSuffix(reasoning, "\n")
			if m.config.ShowThinking {
				if m.config.IsPipedOutput {
					fmt.Print(reasoning)
				} else {
					fmt.Print("\033[90m" + reasoning + "\033[0m")
				}
			}
			response.WriteString(reasoning)
		}

		if delta.Content != "" {
			if wasReasoning && !lastReasoningEndsWithNewline && m.config.ShowThinking {
				fmt.Println()
			}
			wasReasoning = false

			if strings.Contains(delta.Content, "<think>") {
				insideThinkTag = true
			}

			if justExitedThinkTag {
				delta.Content = strings.TrimLeft(delta.Content, "\n\r ")
				if delta.Content == "" {
					continue
				}
				justExitedThinkTag = false
			}

			if insideThinkTag && !m.config.ShowThinking {
				// Skip displaying think-tagged content
			} else if m.config.IsPipedOutput {
				fmt.Print(delta.Content)
			} else if insideThinkTag {
				fmt.Print("\033[90m" + delta.Content + "\033[0m")
			} else {
				fmt.Print("\033[92m" + delta.Content + "\033[0m")
			}

			if strings.Contains(delta.Content, "</think>") {
				insideThinkTag = false
				if !m.config.ShowThinking {
					justExitedThinkTag = true
				}
			}

			response.WriteString(delta.Content)
		}
	}

	fmt.Println()
	return response.String(), nil
}

func (m *Manager) fetchPlatformModels(platform types.Platform) ([]string, error) {
	httpClient := &http.Client{Timeout: 10 * time.Second}

	apiKey := os.Getenv(platform.EnvName)
	if apiKey == "" && platform.Name != "ollama" {
		return nil, fmt.Errorf("%s environment variable not set", platform.EnvName)
	}

	// Handle Google's special URL with API key in query parameter
	url := platform.Models.URL
	if platform.Name == "google" {
		url = strings.Replace(url, "https://generativelanguage.googleapis.com/v1beta/models", "https://generativelanguage.googleapis.com/v1beta/models?key="+apiKey, 1)
	}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	if platform.Name == "anthropic" {
		req.Header.Set("x-api-key", apiKey)
		req.Header.Set("anthropic-version", "2023-06-01")
	} else if platform.Name != "ollama" && platform.Name != "google" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var jsonData interface{}
	err = json.Unmarshal(body, &jsonData)
	if err != nil {
		return nil, err
	}

	return m.extractModelsFromJSON(jsonData, platform.Models.JSONPath)
}

func (m *Manager) extractModelsFromJSON(data interface{}, jsonPath string) ([]string, error) {
	parts := strings.Split(jsonPath, ".")

	current := data

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
