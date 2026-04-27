package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"
)

var (
	ErrNoEncryptionKey = errors.New("neither LICENSE_KEY_SECRET nor JWT_SECRET is set, cannot encrypt private key")
)

func normalizeLicenseFileToken(productName string) string {
	trimmed := strings.TrimSpace(productName)
	if trimmed == "" {
		return "LICENSE1"
	}

	var builder strings.Builder
	for _, r := range strings.ToUpper(trimmed) {
		switch {
		case r >= 'A' && r <= 'Z':
			builder.WriteRune(r)
		case r >= '0' && r <= '9':
			builder.WriteRune(r)
		}
		if builder.Len() >= 24 {
			break
		}
	}

	token := builder.String()
	if token != "" {
		return token + "1"
	}

	hash := sha256.Sum256([]byte(trimmed))
	return fmt.Sprintf("LIC%X1", hash[:4])
}

func buildLicenseFilePrefix(productName string) string {
	return normalizeLicenseFileToken(productName) + "."
}

func decodeLicenseFilePayload(encoded string) ([]byte, error) {
	data, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil {
		return nil, fmt.Errorf("decode license file: %w", err)
	}
	return data, nil
}

// GenerateKeyPair generates a new Ed25519 key pair and returns
// (publicKeyBase64, encryptedPrivateKeyBase64, error).
func GenerateKeyPair(encKey []byte) (string, string, error) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return "", "", fmt.Errorf("generate ed25519 key: %w", err)
	}

	pubB64 := base64.StdEncoding.EncodeToString(pub)
	privB64 := base64.StdEncoding.EncodeToString(priv)

	encrypted, err := encryptAESGCM([]byte(privB64), encKey)
	if err != nil {
		return "", "", fmt.Errorf("encrypt private key: %w", err)
	}

	return pubB64, base64.StdEncoding.EncodeToString(encrypted), nil
}

// GetEncryptionKey returns the 32-byte AES key from jwtSecret.
func GetEncryptionKey(jwtSecret []byte) ([]byte, error) {
	if len(jwtSecret) > 0 {
		h := sha256.Sum256(jwtSecret)
		return h[:], nil
	}
	return nil, ErrNoEncryptionKey
}

// GetEncryptionKeyWithFallback returns the encryption key.
// The licenseKeySecret (from metis.yaml) takes priority over jwtSecret.
func GetEncryptionKeyWithFallback(licenseKeySecret, jwtSecret []byte) ([]byte, error) {
	if len(licenseKeySecret) > 0 {
		h := sha256.Sum256(licenseKeySecret)
		return h[:], nil
	}
	return GetEncryptionKey(jwtSecret)
}

func GenerateLicenseKey() (string, error) {
	b := make([]byte, 24)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate license key: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func encryptAESGCM(plaintext, key []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}
	return aead.Seal(nonce, nonce, plaintext, nil), nil
}

func decryptAESGCM(ciphertext, key []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonceSize := aead.NonceSize()
	if len(ciphertext) < nonceSize {
		return nil, errors.New("ciphertext too short")
	}
	nonce, ct := ciphertext[:nonceSize], ciphertext[nonceSize:]
	return aead.Open(nil, nonce, ct, nil)
}

// DeriveLicenseFileKey derives a 32-byte AES key from the license file token and customer's registration code.
func DeriveLicenseFileKey(registrationCode string, fileToken string) []byte {
	h := sha256.Sum256([]byte(fileToken + ":" + registrationCode))
	return h[:]
}

// DeriveLicenseFileKeyV2 derives a 32-byte AES key using fileToken, licenseKey, and registrationCode.
func DeriveLicenseFileKeyV2(registrationCode string, fileToken string, licenseKey string) []byte {
	h := sha256.Sum256([]byte(fileToken + ":" + licenseKey + ":" + registrationCode))
	return h[:]
}

// EncryptLicenseFile encrypts the full .lic JSON payload into a single base64url string (v1 single-key).
func EncryptLicenseFile(plaintext []byte, registrationCode string, productName string) (string, error) {
	return EncryptLicenseFileV2(plaintext, registrationCode, productName, "")
}

// EncryptLicenseFileV2 encrypts the .lic JSON payload using dual-key derivation.
// If licenseKey is empty, it falls back to v1 single-key derivation.
func EncryptLicenseFileV2(plaintext []byte, registrationCode string, productName string, licenseKey string) (string, error) {
	prefix := buildLicenseFilePrefix(productName)
	fileToken := strings.TrimSuffix(prefix, ".")
	var key []byte
	if licenseKey != "" {
		key = DeriveLicenseFileKeyV2(registrationCode, fileToken, licenseKey)
	} else {
		key = DeriveLicenseFileKey(registrationCode, fileToken)
	}
	encrypted, err := encryptAESGCM(plaintext, key)
	if err != nil {
		return "", fmt.Errorf("encrypt license file: %w", err)
	}
	return prefix + base64.RawURLEncoding.EncodeToString(encrypted), nil
}

// DecryptLicenseFile decrypts a base64url-encoded .lic string using the registration code (v1 single-key).
func DecryptLicenseFile(ciphertextBase64URL string, registrationCode string) ([]byte, error) {
	return DecryptLicenseFileV2(ciphertextBase64URL, registrationCode, "")
}

// DecryptLicenseFileV2 decrypts a .lic string using dual-key derivation.
// If licenseKey is empty, it falls back to v1 single-key derivation.
func DecryptLicenseFileV2(ciphertextBase64URL string, registrationCode string, licenseKey string) ([]byte, error) {
	original := strings.TrimSpace(ciphertextBase64URL)
	if original == "" {
		return nil, errors.New("license file is empty")
	}

	dot := strings.IndexRune(original, '.')
	if dot <= 0 {
		return nil, errors.New("invalid license file format")
	}

	fileToken := original[:dot]
	encoded := original[dot+1:]
	data, err := decodeLicenseFilePayload(encoded)
	if err != nil {
		return nil, err
	}
	var key []byte
	if licenseKey != "" {
		key = DeriveLicenseFileKeyV2(registrationCode, fileToken, licenseKey)
	} else {
		key = DeriveLicenseFileKey(registrationCode, fileToken)
	}
	plaintext, err := decryptAESGCM(data, key)
	if err != nil {
		return nil, fmt.Errorf("decrypt license file: %w", err)
	}
	return plaintext, nil
}

// Canonicalize produces a deterministic JSON string by recursively sorting object keys.
func Canonicalize(v any) (string, error) {
	data, err := json.Marshal(v)
	if err != nil {
		return "", fmt.Errorf("canonicalize marshal: %w", err)
	}
	var raw any
	if err := json.Unmarshal(data, &raw); err != nil {
		return "", fmt.Errorf("canonicalize unmarshal: %w", err)
	}
	return canonicalizeValue(raw), nil
}

func canonicalizeValue(v any) string {
	switch val := v.(type) {
	case map[string]any:
		keys := make([]string, 0, len(val))
		for k := range val {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		result := "{"
		for i, k := range keys {
			if i > 0 {
				result += ","
			}
			kb, _ := json.Marshal(k)
			result += string(kb) + ":" + canonicalizeValue(val[k])
		}
		return result + "}"
	case []any:
		result := "["
		for i, item := range val {
			if i > 0 {
				result += ","
			}
			result += canonicalizeValue(item)
		}
		return result + "]"
	default:
		b, _ := json.Marshal(v)
		return string(b)
	}
}

// SignLicense signs a canonicalized payload with the decrypted Ed25519 private key.
// Returns a base64url-encoded signature (no padding).
func SignLicense(payload map[string]any, encryptedPrivateKey string, encKey []byte) (string, error) {
	// Decrypt the private key
	encBytes, err := base64.StdEncoding.DecodeString(encryptedPrivateKey)
	if err != nil {
		return "", fmt.Errorf("decode encrypted private key: %w", err)
	}
	privB64Bytes, err := decryptAESGCM(encBytes, encKey)
	if err != nil {
		return "", fmt.Errorf("decrypt private key: %w", err)
	}
	privBytes, err := base64.StdEncoding.DecodeString(string(privB64Bytes))
	if err != nil {
		return "", fmt.Errorf("decode private key base64: %w", err)
	}

	// Canonicalize payload
	canonical, err := Canonicalize(payload)
	if err != nil {
		return "", err
	}

	// Sign
	sig := ed25519.Sign(ed25519.PrivateKey(privBytes), []byte(canonical))
	return base64.RawURLEncoding.EncodeToString(sig), nil
}

// VerifyLicenseSignature verifies an Ed25519 signature against a canonicalized payload.
func VerifyLicenseSignature(payload map[string]any, signatureBase64url string, publicKeyBase64 string) (bool, error) {
	pubBytes, err := base64.StdEncoding.DecodeString(publicKeyBase64)
	if err != nil {
		return false, fmt.Errorf("decode public key: %w", err)
	}
	sigBytes, err := base64.RawURLEncoding.DecodeString(signatureBase64url)
	if err != nil {
		return false, fmt.Errorf("decode signature: %w", err)
	}
	canonical, err := Canonicalize(payload)
	if err != nil {
		return false, err
	}
	return ed25519.Verify(ed25519.PublicKey(pubBytes), []byte(canonical), sigBytes), nil
}

// GenerateActivationCode combines payload + signature into a base64url-encoded JSON string.
func GenerateActivationCode(payload map[string]any, signature string) (string, error) {
	full := make(map[string]any, len(payload)+1)
	for k, v := range payload {
		full[k] = v
	}
	full["sig"] = signature
	data, err := json.Marshal(full)
	if err != nil {
		return "", fmt.Errorf("marshal activation code: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(data), nil
}

// DecodeActivationCode decodes a base64url-encoded activation code back to a map.
func DecodeActivationCode(code string) (map[string]any, error) {
	data, err := base64.RawURLEncoding.DecodeString(code)
	if err != nil {
		return nil, fmt.Errorf("decode activation code: %w", err)
	}
	var result map[string]any
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("unmarshal activation code: %w", err)
	}
	return result, nil
}
