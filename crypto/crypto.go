package crypto

import (
	"crypto/rand"
	"encoding/base64"
)

// GenerateRandomBytes generates random bytes of the specified length
func GenerateRandomBytes(length int) ([]byte, error) {
	b := make([]byte, length)
	_, err := rand.Read(b)
	if err != nil {
		return nil, err
	}
	return b, nil
}

// GenerateRandomString generates a random base64-encoded string
func GenerateRandomString(length int) (string, error) {
	bytes, err := GenerateRandomBytes(length)
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(bytes), nil
}

// HashPassword hashes a password using a secure hashing algorithm
func HashPassword(password string) (string, error) {
	// Implementation will use bcrypt or similar
	// This is a placeholder
	return "hashed_" + password, nil
}

// CheckPasswordHash verifies a password against a hash
func CheckPasswordHash(password, hash string) bool {
	// Implementation will verify bcrypt hash
	// This is a placeholder
	return "hashed_" + password == hash
}