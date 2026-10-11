package chat

import (
	"fmt"
	"strings"

	"github.com/MehmetMHY/ch/pkg/types"
)

// AddLoadedContent records a context load atomically, including image references.
func (m *Manager) AddLoadedContent(summary string, loaded types.LoadedContent) error {
	var maxImageBytes, maxTotalBytes int64
	if len(loaded.Images) != 0 {
		var err error
		maxImageBytes, maxTotalBytes, err = types.ResolveImageLimits(m.state.Config)
		if err != nil {
			return err
		}
	}
	var totalBytes int64
	for _, img := range m.state.Images {
		totalBytes += int64(len(img.Data))
	}
	var ids []string
	for _, img := range loaded.Images {
		if err := img.Validate(maxImageBytes); err != nil {
			return err
		}
		if int64(len(img.Data)) > maxTotalBytes-totalBytes {
			return fmt.Errorf("images exceed the %d MiB session limit (image_max_total_size_mb); backtrack, clear history, or increase the limit", maxTotalBytes/types.MiB)
		}
		totalBytes += int64(len(img.Data))
		ids = append(ids, img.ID)
	}
	if loaded.Text == "" && len(ids) == 0 {
		return nil
	}
	m.state.Images = append(m.state.Images, loaded.Images...)
	m.AddUserMessage(loaded.Text)
	m.state.Messages[len(m.state.Messages)-1].ImageIDs = ids
	m.AddToHistoryWithContext(summary, "", loaded.Text)
	m.state.ChatHistory[len(m.state.ChatHistory)-1].ImageIDs = ids
	return nil
}

func (m *Manager) resolveMessageImages() []types.ChatMessage {
	images := make(map[string]types.ImageAttachment, len(m.state.Images))
	for _, img := range m.state.Images {
		images[img.ID] = img
	}
	result := make([]types.ChatMessage, len(m.state.Messages))
	for i, msg := range m.state.Messages {
		result[i] = msg
		result[i].Images = nil
		var excluded []string
		for _, id := range msg.ImageIDs {
			img, ok := images[id]
			if !ok {
				excluded = append(excluded, id+" (unavailable)")
			} else if m.state.ImagesOff || img.Disabled {
				excluded = append(excluded, img.Name)
			} else {
				result[i].Images = append(result[i].Images, img)
			}
		}
		if len(excluded) != 0 {
			result[i].Content += "\nImages not sent in this request: " + strings.Join(excluded, ", ") + "."
		}
	}
	return result
}

// pruneImages releases only attachments no longer referenced by retained history.
func (m *Manager) pruneImages() {
	used := make(map[string]bool)
	for _, entry := range m.state.ChatHistory {
		for _, id := range entry.ImageIDs {
			used[id] = true
		}
	}
	var kept []types.ImageAttachment
	for _, img := range m.state.Images {
		if used[img.ID] {
			kept = append(kept, img)
		}
	}
	m.state.Images = kept
}
