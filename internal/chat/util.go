package chat

import (
	"math/rand"
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

	// Use content + offset as seed for more variation
	seed := int64(len(content) + offset)
	for i, char := range content {
		if i < 100 { // Only use first 100 chars to avoid overflow
			seed += int64(char) * int64(i+offset+1)
		}
	}
	rand.Seed(seed)

	// Generate hash
	hash := make([]rune, length)
	for i := range hash {
		hash[i] = charset[rand.Intn(len(charset))]
	}

	return string(hash)
}
