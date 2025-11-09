package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/MehmetMHY/ch/pkg/types"
)

// GetTempDir returns the application's temporary directory, creating it if it doesn't exist
func GetTempDir() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home directory: %w", err)
	}

	tempDir := filepath.Join(homeDir, ".ch", "tmp")

	// Create the directory if it doesn't exist
	if err := os.MkdirAll(tempDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create temp directory: %w", err)
	}

	return tempDir, nil
}

// IsShallowLoadDir checks if a directory should be loaded shallowly (only 1 level deep)
func IsShallowLoadDir(cfg *types.Config, dirPath string) bool {
	// Normalize the directory path
	absPath, err := filepath.Abs(dirPath)
	if err != nil {
		return false
	}
	absPath = filepath.Clean(absPath)

	// Check against each shallow load directory
	for _, shallowDir := range cfg.ShallowLoadDirs {
		if shallowDir == "" {
			continue
		}

		// Expand ~ to home directory
		if strings.HasPrefix(shallowDir, "~") {
			homeDir, err := os.UserHomeDir()
			if err != nil {
				continue
			}
			shallowDir = filepath.Join(homeDir, shallowDir[1:])
		}

		// Normalize shallow directory path
		absShallowDir, err := filepath.Abs(shallowDir)
		if err != nil {
			continue
		}
		absShallowDir = filepath.Clean(absShallowDir)

		// Check for exact match
		if absPath == absShallowDir {
			return true
		}
	}

	return false
}
