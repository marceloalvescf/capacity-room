package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
)

// NewToken returns 32 random bytes as base64url without padding.
func NewToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("read random bytes: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// Digest is the SHA-256 of the UTF-8 bytes of token.
func Digest(token string) []byte {
	sum := sha256.Sum256([]byte(token))
	return sum[:]
}
