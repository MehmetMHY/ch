package ui

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/MehmetMHY/ch/pkg/types"
)

// RunEditorWithFallback tries to run the user's preferred editor, then falls back to common editors.
func RunEditorWithFallback(cfg *types.Config, filePath string) error {
	var editors []string
	if envEditor := os.Getenv("EDITOR"); envEditor != "" {
		editors = append(editors, envEditor)
	}
	editors = append(editors, cfg.PreferredEditor, "vim", "nano")

	// Remove duplicates
	seen := make(map[string]bool)
	uniqueEditors := []string{}
	for _, editor := range editors {
		if editor != "" && !seen[editor] {
			uniqueEditors = append(uniqueEditors, editor)
			seen[editor] = true
		}
	}

	for i, editor := range uniqueEditors {
		// Check if the editor exists
		if _, err := exec.LookPath(editor); err != nil {
			continue
		}

		cmd := exec.Command(editor, filePath)
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout

		// For the first attempts, suppress stderr to avoid showing error messages
		// Only show stderr for the final attempt
		if i < len(uniqueEditors)-1 {
			cmd.Stderr = nil // Suppress error messages for fallback attempts
		} else {
			cmd.Stderr = os.Stderr // Show errors for final attempt
		}

		if err := cmd.Run(); err != nil {
			// If this editor failed, try the next one
			continue
		}

		// Success!
		return nil
	}

	return fmt.Errorf("no working editor found")
}
