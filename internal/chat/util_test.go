package chat

import (
	"regexp"
	"strings"
	"testing"
)

func TestGenerateHashFromContent(t *testing.T) {
	content := "abc123!!!"
	length := 64

	hash := GenerateHashFromContent(content, length)

	if len(hash) != length {
		t.Errorf("expected hash length of %d, got %d", length, len(hash))
	}

	for _, char := range hash {
		if !strings.ContainsRune("abc123", char) {
			t.Errorf("hash %q contains %q, want only alphanumeric characters from content", hash, char)
		}
	}
}

func TestGenerateHashFromContent_ZeroLength(t *testing.T) {
	if got := GenerateHashFromContent("content", 0); got != "" {
		t.Errorf("GenerateHashFromContent() = %q, want empty string for zero length", got)
	}
}

func TestGenerateHashFromContent_CharsetFallback(t *testing.T) {
	// Completely non-alphanumeric content triggers fallback charset
	content := "!!!$$$@@@+++"
	length := 16

	hash := GenerateHashFromContent(content, length)

	if len(hash) != length {
		t.Errorf("expected hash length of %d, got %d", length, len(hash))
	}

	// Verify it still generated a valid alphanumeric hash from the fallback charset
	pattern := `^[a-zA-Z0-9]+$`
	re := regexp.MustCompile(pattern)
	if !re.MatchString(hash) {
		t.Errorf("expected hash %q to only contain alphanumeric characters from the fallback charset", hash)
	}
}

func TestGenerateHashFromContentWithOffset(t *testing.T) {
	content := "Some message"
	length := 8

	hash1 := GenerateHashFromContentWithOffset(content, length, 1)
	hash2 := GenerateHashFromContentWithOffset(content, length, 2)

	if len(hash1) != length {
		t.Errorf("expected hash1 length %d, got %d", length, len(hash1))
	}
	if len(hash2) != length {
		t.Errorf("expected hash2 length %d, got %d", length, len(hash2))
	}
	for _, hash := range []string{hash1, hash2} {
		for _, char := range hash {
			if !strings.ContainsRune("Somemessage", char) {
				t.Errorf("hash %q contains %q, want only alphanumeric characters from content", hash, char)
			}
		}
	}
}
