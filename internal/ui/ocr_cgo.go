//go:build cgo

package ui

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/otiai10/gosseract/v2"
)

// extractTextFromImage uses Tesseract OCR to extract text from an image
func (t *Terminal) extractTextFromImage(filePath string) (string, error) {
	// Check if tesseract is installed
	if _, err := exec.LookPath("tesseract"); err != nil {
		return "", fmt.Errorf("tesseract OCR is not installed. Please install it to enable image-to-text extraction")
	}

	client := gosseract.NewClient()
	defer client.Close()

	// Set language (default to English, but could be made configurable)
	err := client.SetLanguage("eng")
	if err != nil {
		// If English fails, try without setting language
		client = gosseract.NewClient()
		defer client.Close()
	}

	// Set image source
	err = client.SetImage(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to set image source: %w", err)
	}

	// Configure OCR settings for better accuracy
	client.SetVariable("tessedit_char_whitelist", "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789.,!?@#$%^&*()_+-={}[]|\\:;\"'<>/~` ")

	// Extract text
	text, err := client.Text()
	if err != nil {
		return "", fmt.Errorf("OCR extraction failed: %w", err)
	}

	// Clean up the extracted text
	cleanedText := strings.TrimSpace(text)

	// Remove excessive whitespace and normalize line breaks
	lines := strings.Split(cleanedText, "\n")
	var cleanedLines []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			cleanedLines = append(cleanedLines, line)
		}
	}

	return strings.Join(cleanedLines, "\n"), nil
}
