package token

import (
	"crypto/rand"
	"encoding/hex"
	"strings"

	"golang.org/x/crypto/bcrypt"
)

const tokenPrefix = "itk_"

// GenerateIntegrationToken creates a new token in itk_<32-byte-hex> format.
// Returns the raw token (display once), bcrypt hash, and 8-char prefix for DB lookup.
func GenerateIntegrationToken() (raw string, hash string, prefix string, err error) {
	b := make([]byte, 32)
	if _, err = rand.Read(b); err != nil {
		return "", "", "", err
	}
	raw = tokenPrefix + hex.EncodeToString(b)
	prefix = raw[:8] // "itk_" + first 4 hex chars = 8 chars

	hashBytes, err := bcrypt.GenerateFromPassword([]byte(raw), bcrypt.DefaultCost)
	if err != nil {
		return "", "", "", err
	}
	hash = string(hashBytes)
	return raw, hash, prefix, nil
}

// ValidateIntegrationToken checks a raw token against a bcrypt hash.
func ValidateIntegrationToken(raw, hash string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(raw)) == nil
}

// ExtractTokenPrefix returns the prefix portion of a raw token for DB lookup.
func ExtractTokenPrefix(raw string) string {
	if len(raw) >= 8 && strings.HasPrefix(raw, tokenPrefix) {
		return raw[:8]
	}
	return ""
}
