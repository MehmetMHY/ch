package ui

import (
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/MehmetMHY/ch/pkg/types"
	"github.com/google/uuid"
)

func isImagePath(path string) bool {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".png", ".jpg", ".jpeg", ".gif", ".webp", ".bmp", ".tiff", ".tif":
		return true
	}
	return false
}

// LoadChatContent captures native images and also extracts OCR text when sending is off.
// Print-only utilities deliberately continue using LoadFileContent.
func (t *Terminal) LoadChatContent(selections []string, imagesOff bool) (types.LoadedContent, error) {
	return t.loadChatContent(selections, imagesOff, t.extractTextFromImageBytes)
}

func (t *Terminal) loadChatContent(selections []string, imagesOff bool, extractText func([]byte) (string, error)) (types.LoadedContent, error) {
	var result types.LoadedContent
	var text strings.Builder
	var totalBytes int64
	loadFile := func(path string) error {
		if !isImagePath(path) {
			content, err := t.loadTextFile(path)
			if err != nil {
				return err
			}
			text.WriteString(content)
			return nil
		}
		maxImageBytes, maxTotalBytes, err := types.ResolveImageLimits(t.config)
		if err != nil {
			return err
		}
		file, err := os.Open(path) // #nosec G304 -- Explicitly selected image input.
		if err != nil {
			return err
		}
		defer file.Close()
		readLimit := maxImageBytes
		if remaining := maxTotalBytes - totalBytes; remaining < readLimit {
			readLimit = remaining
		}
		data, err := io.ReadAll(io.LimitReader(file, readLimit+1))
		if err != nil {
			return err
		}
		if int64(len(data)) > maxTotalBytes-totalBytes {
			return fmt.Errorf("images exceed the %d MiB total limit (image_max_total_size_mb)", maxTotalBytes/types.MiB)
		}
		img := types.ImageAttachment{ID: uuid.NewString(), Name: filepath.Base(path), MIMEType: http.DetectContentType(data), Data: data}
		if err := img.Validate(maxImageBytes); err != nil {
			return err
		}
		totalBytes += int64(len(data))
		result.Images = append(result.Images, img)
		text.WriteString(fmt.Sprintf("File: %s [image]\n\n", path))
		if imagesOff {
			extracted, err := extractText(data)
			if err != nil {
				result.Warnings = append(result.Warnings, fmt.Sprintf("OCR unavailable for %s: %v; native image retained", img.Name, err))
			} else if extracted = strings.TrimSpace(extracted); extracted != "" {
				text.WriteString("text extraction (OCR):\n" + extracted + "\n\n")
			}
		}
		return nil
	}
	for _, selection := range selections {
		if t.IsURL(selection) {
			content, err := t.LoadFileContent([]string{selection})
			if err != nil {
				return types.LoadedContent{}, err
			}
			text.WriteString(content)
			continue
		}
		info, err := os.Stat(selection)
		if err != nil {
			return types.LoadedContent{}, err
		}
		if !info.IsDir() {
			if err := loadFile(selection); err != nil {
				return types.LoadedContent{}, fmt.Errorf("loading %s: %w", selection, err)
			}
			continue
		}
		err = filepath.WalkDir(selection, func(path string, entry fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if entry.IsDir() {
				return nil
			}
			if err := loadFile(path); err != nil && isImagePath(path) {
				return err
			}
			return nil // Preserve directory loading's skip behavior for non-text files.
		})
		if err != nil {
			return types.LoadedContent{}, err
		}
	}
	result.Text = text.String()
	return result, nil
}
