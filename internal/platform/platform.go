package platform

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"strings"
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
		if m.config.OpenAIAPIKey == "" {
			return fmt.Errorf("OPENAI_API_KEY environment variable is required")
		}
		m.client = openai.NewClient(m.config.OpenAIAPIKey)
		return nil
	}

	platform, exists := m.config.Platforms[m.config.CurrentPlatform]
	if !exists {
		return fmt.Errorf("platform %s not found", m.config.CurrentPlatform)
	}

	apiKey := os.Getenv(platform.EnvName)
	if apiKey == "" {
		return fmt.Errorf("%s environment variable is required for %s", platform.EnvName, platform.Name)
	}

	clientConfig := openai.DefaultConfig(apiKey)
	clientConfig.BaseURL = platform.BaseURL
	m.client = openai.NewClientWithConfig(clientConfig)

	return nil
}

// SendChatRequest sends a chat request to the current platform
func (m *Manager) SendChatRequest(messages []types.ChatMessage, model string, streamingCancel *func(), isStreaming *bool) (string, error) {
	var openaiMessages []openai.ChatCompletionMessage
	for _, msg := range messages {
		openaiMessages = append(openaiMessages, openai.ChatCompletionMessage{
			Role:    msg.Role,
			Content: msg.Content,
		})
	}

	if m.isReasoningModel(model) {
		return m.sendNonStreamingRequest(openaiMessages, model, streamingCancel, isStreaming)
	}

	return m.sendStreamingRequest(openaiMessages, model, streamingCancel, isStreaming)
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
		return modelNames, nil
	}

	platform := m.config.Platforms[m.config.CurrentPlatform]
	return m.fetchPlatformModels(platform)
}

// SelectPlatform handles platform selection and model selection
func (m *Manager) SelectPlatform(platformKey, modelName string, fzfSelector func([]string, string) (string, error)) (map[string]interface{}, error) {
	if platformKey == "" {
		var platforms []string
		platforms = append(platforms, "openai")
		for name := range m.config.Platforms {
			platforms = append(platforms, name)
		}

		selected, err := fzfSelector(platforms, "Select a platform: ")
		if err != nil {
			return nil, err
		}

		if selected == "" {
			return nil, fmt.Errorf("no platform selected")
		}

		platformKey = selected
	}

	if platformKey == "openai" {
		finalModel := modelName
		if finalModel == "" {
			finalModel = "gpt-4o-mini"
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

	finalModel := modelName
	var modelsList []string

	if finalModel == "" {
		var err error
		modelsList, err = m.fetchPlatformModels(platform)
		if err != nil {
			return nil, fmt.Errorf("failed to retrieve models: %v", err)
		}

		if len(modelsList) == 0 {
			return nil, fmt.Errorf("no models found or returned in unexpected format")
		}

		selected, err := fzfSelector(modelsList, "Select a model: ")
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

func (m *Manager) isReasoningModel(modelName string) bool {
	matched, _ := regexp.MatchString(`^o\d+`, modelName)
	return matched
}

// IsReasoningModel checks if the model is a reasoning model (like o1, o2, etc.)
func (m *Manager) IsReasoningModel(modelName string) bool {
	return m.isReasoningModel(modelName)
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

	var response strings.Builder

	for {
		completion, err := stream.Recv()
		if err != nil {
			if err == io.EOF {
				break
			}
			if ctx.Err() == context.Canceled {
				return response.String(), nil
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

	fmt.Println()
	return response.String(), nil
}

func (m *Manager) fetchPlatformModels(platform types.Platform) ([]string, error) {
	httpClient := &http.Client{Timeout: 10 * time.Second}

	req, err := http.NewRequest("GET", platform.Models.URL, nil)
	if err != nil {
		return nil, err
	}

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
