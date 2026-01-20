//go:build !cgo

package ui

import "fmt"

// extractTextFromImage provides a stub when CGO is disabled (e.g., on Android)
func (t *Terminal) extractTextFromImage(filePath string) (string, error) {
	return "", fmt.Errorf("OCR (Tesseract) is not available on this platform. Image-to-text extraction is disabled")
}
