package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/MehmetMHY/ch/pkg/types"
)

func TestGetTempDir(t *testing.T) {
	tempHome := t.TempDir()
	t.Setenv("HOME", tempHome)
	t.Setenv("USERPROFILE", tempHome)

	expectedDir := filepath.Join(tempHome, ".ch", "tmp")

	got, err := GetTempDir()
	if err != nil {
		t.Fatalf("GetTempDir() returned error: %v", err)
	}

	if got != expectedDir {
		t.Errorf("GetTempDir() = %v, want %v", got, expectedDir)
	}

	// Verify the directory actually exists on disk
	fi, err := os.Stat(expectedDir)
	if err != nil {
		t.Fatalf("expected directory to be created, but got err: %v", err)
	}
	if !fi.IsDir() {
		t.Errorf("expected %v to be a directory", expectedDir)
	}
}

func TestIsShallowLoadDir(t *testing.T) {
	tempHome := t.TempDir()
	t.Setenv("HOME", tempHome)
	t.Setenv("USERPROFILE", tempHome)

	cfg := &types.Config{
		ShallowLoadDirs: []string{
			"/var/log",
			"~/my_large_dir",
			"", // Should be ignored safely
		},
	}

	tests := []struct {
		name    string
		dirPath string
		want    bool
	}{
		{
			name:    "Exact match with standard dir",
			dirPath: "/var/log",
			want:    true,
		},
		{
			name:    "Match with non-cleaned path",
			dirPath: "/var/log/../log",
			want:    true,
		},
		{
			name:    "Tilde expansion path direct match",
			dirPath: filepath.Join(tempHome, "my_large_dir"),
			want:    true,
		},
		{
			name:    "Tilde expansion path non-cleaned match",
			dirPath: filepath.Join(tempHome, "my_large_dir", "sub_dir", ".."),
			want:    true,
		},
		{
			name:    "Non-matching directory",
			dirPath: "/var/log/other",
			want:    false,
		},
		{
			name:    "Relative path non-matching",
			dirPath: "./other",
			want:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsShallowLoadDir(cfg, tt.dirPath)
			if got != tt.want {
				t.Errorf("IsShallowLoadDir() = %v, want %v", got, tt.want)
			}
		})
	}
}
