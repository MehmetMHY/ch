package platform

import (
	"encoding/base64"
	"fmt"

	"github.com/MehmetMHY/ch/pkg/types"
	openai "github.com/sashabaranov/go-openai"
)

// SupportsImageTransport describes Ch's current chat-completions transport,
// independently of the selected model's vision capabilities.
func SupportsImageTransport(cfg *types.Config) bool {
	if p, ok := cfg.Platforms[cfg.CurrentPlatform]; ok && p.ImageInput != nil {
		return *p.ImageInput
	}
	switch cfg.CurrentPlatform {
	case "openai", "openrouter", "groq", "anthropic", "xai", "ollama", "together", "google", "mistral":
		return true
	}
	return false
}

func (m *Manager) imageRequestError(messages []types.ChatMessage, err error) error {
	if err == nil || err.Error() == "request was interrupted" {
		return err
	}
	for _, msg := range messages {
		if len(msg.Images) != 0 {
			return fmt.Errorf("%w (images remain loaded; use %s off to retry without images, or switch models)", err, m.config.ImageControl)
		}
	}
	return err
}

func (m *Manager) buildChatMessages(messages []types.ChatMessage, model string) ([]openai.ChatCompletionMessage, error) {
	var result []openai.ChatCompletionMessage
	checkedCapability := false
	var maxImageBytes, maxTotalBytes, totalBytes int64
	for _, msg := range m.mergeConsecutiveUserMessages(messages) {
		out := openai.ChatCompletionMessage{Role: msg.Role, Content: msg.Content}
		if len(msg.Images) != 0 {
			if !checkedCapability {
				var err error
				maxImageBytes, maxTotalBytes, err = types.ResolveImageLimits(m.config)
				if err != nil {
					return nil, err
				}
				if !SupportsImageTransport(m.config) {
					return nil, fmt.Errorf("image input is not supported by Ch's %s endpoint; use %s off or switch platforms", m.config.CurrentPlatform, m.config.ImageControl)
				}
				if ResolveImageCapability(m.config.CurrentPlatform, model, m.config) == CapStatusUnsupported {
					return nil, fmt.Errorf("model %s does not support image input; use %s off or switch models", model, m.config.ImageControl)
				}
				checkedCapability = true
			}
			out.Content = ""
			if msg.Content != "" {
				out.MultiContent = append(out.MultiContent, openai.ChatMessagePart{Type: openai.ChatMessagePartTypeText, Text: msg.Content})
			}
			for _, img := range msg.Images {
				if err := img.Validate(maxImageBytes); err != nil {
					return nil, err
				}
				if int64(len(img.Data)) > maxTotalBytes-totalBytes {
					return nil, fmt.Errorf("images exceed the %d MiB request limit (image_max_total_size_mb); disable images with %s or increase the limit", maxTotalBytes/types.MiB, m.config.ImageControl)
				}
				totalBytes += int64(len(img.Data))
				out.MultiContent = append(out.MultiContent, openai.ChatMessagePart{
					Type:     openai.ChatMessagePartTypeImageURL,
					ImageURL: &openai.ChatMessageImageURL{URL: "data:" + img.MIMEType + ";base64," + base64.StdEncoding.EncodeToString(img.Data)},
				})
			}
		}
		result = append(result, out)
	}
	return result, nil
}
