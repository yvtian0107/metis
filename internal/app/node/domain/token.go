package domain

import (
	"crypto/rand"
	"encoding/hex"
	"strings"

	"golang.org/x/crypto/bcrypt"
)

const tokenPrefix = "mtk_"

// GenerateNodeToken creates a new node token in mtk_<32-byte-hex> format.
// Returns the raw token (to display once) and its bcrypt hash.
func GenerateNodeToken() (raw string, hash string, prefix string, err error) {
	b := make([]byte, 32)
	if _, err = rand.Read(b); err != nil {
		return "", "", "", err
	}
	raw = tokenPrefix + hex.EncodeToString(b)
	prefix = raw[:12] // "mtk_" + first 8 hex chars

	hashBytes, err := bcrypt.GenerateFromPassword([]byte(raw), bcrypt.DefaultCost)
	if err != nil {
		return "", "", "", err
	}
	hash = string(hashBytes)
	return raw, hash, prefix, nil
}

// ValidateNodeToken checks a raw token against a bcrypt hash.
func ValidateNodeToken(raw, hash string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(raw)) == nil
}

// ExtractTokenPrefix returns the prefix portion of a raw token for DB lookup.
func ExtractTokenPrefix(raw string) string {
	if len(raw) >= 12 && strings.HasPrefix(raw, tokenPrefix) {
		return raw[:12]
	}
	return ""
}
