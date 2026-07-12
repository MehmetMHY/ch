package chat

import (
	"crypto/rand"
	"math/big"
)

// GenerateHashFromContent creates a random hash using characters from the content
func GenerateHashFromContent(content string, length int) string {
	return GenerateHashFromContentWithOffset(content, length, 0)
}

// GenerateHashFromContentWithOffset creates a hash with an offset for collision avoidance
func GenerateHashFromContentWithOffset(content string, length, offset int) string {
	// Extract alphanumeric characters from content
	var charset []rune
	for _, char := range content {
		if (char >= 'a' && char <= 'z') || (char >= 'A' && char <= 'Z') || (char >= '0' && char <= '9') {
			charset = append(charset, char)
		}
	}

	// Fallback to default charset if content has no alphanumeric characters
	if len(charset) == 0 {
		charset = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789")
	}

	_ = offset

	// Generate hash
	hash := make([]rune, length)
	for i := range hash {
		idx, err := rand.Int(rand.Reader, big.NewInt(int64(len(charset))))
		if err != nil {
			return ""
		}
		hash[i] = charset[idx.Int64()]
	}

	return string(hash)
}
