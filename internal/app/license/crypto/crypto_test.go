package crypto

import (
	"encoding/json"
	"testing"
)

func TestCanonicalize(t *testing.T) {
	tests := []struct {
		name     string
		input    any
		expected string
	}{
		{
			name:     "empty object",
			input:    map[string]any{},
			expected: "{}",
		},
		{
			name:     "simple key sorting",
			input:    map[string]any{"b": 1, "a": 2},
			expected: `{"a":2,"b":1}`,
		},
		{
			name:     "nested map sorting",
			input:    map[string]any{"z": map[string]any{"c": 1, "a": 2}, "a": 3},
			expected: `{"a":3,"z":{"a":2,"c":1}}`,
		},
		{
			name:     "array preserves order",
			input:    []any{3, 1, 2},
			expected: "[3,1,2]",
		},
		{
			name:     "mixed types",
			input:    map[string]any{"str": "hello", "num": 42, "bool": true, "null": nil},
			expected: `{"bool":true,"null":null,"num":42,"str":"hello"}`,
		},
		{
			name:     "nested array in map",
			input:    map[string]any{"items": []any{map[string]any{"b": 1, "a": 2}}},
			expected: `{"items":[{"a":2,"b":1}]}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Canonicalize(tt.input)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.expected {
				t.Errorf("Canonicalize() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func FuzzCanonicalizeDeterminism(f *testing.F) {
	f.Add([]byte(`{"b":1,"a":2}`))
	f.Add([]byte(`[3,1,2]`))
	f.Add([]byte(`{"nested":{"z":1,"a":2}}`))
	f.Add([]byte(`{}`))
	f.Add([]byte(`null`))

	f.Fuzz(func(t *testing.T, data []byte) {
		var v any
		if err := json.Unmarshal(data, &v); err != nil {
			t.Skip()
		}

		s1, err := Canonicalize(v)
		if err != nil {
			t.Fatalf("first canonicalize failed: %v", err)
		}
		s2, err := Canonicalize(v)
		if err != nil {
			t.Fatalf("second canonicalize failed: %v", err)
		}
		if s1 != s2 {
			t.Fatalf("non-deterministic: %q vs %q", s1, s2)
		}
	})
}

func TestEncryptDecryptLicenseFile(t *testing.T) {
	tests := []struct {
		name        string
		plaintext   []byte
		regCode     string
		productName string
	}{
		{
			name:        "basic round-trip",
			plaintext:   []byte(`{"activationCode":"abc","publicKey":"def"}`),
			regCode:     "RG-12345678",
			productName: "Metis Enterprise",
		},
		{
			name:        "empty product name",
			plaintext:   []byte(`{}`),
			regCode:     "RG-test",
			productName: "",
		},
		{
			name:        "unicode product name",
			plaintext:   []byte(`{"key":"值"}`),
			regCode:     "RG-unicode",
			productName: "产品名称 123",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			encrypted, err := EncryptLicenseFile(tt.plaintext, tt.regCode, tt.productName)
			if err != nil {
				t.Fatalf("EncryptLicenseFile error: %v", err)
			}

			decrypted, err := DecryptLicenseFile(encrypted, tt.regCode)
			if err != nil {
				t.Fatalf("DecryptLicenseFile error: %v", err)
			}

			if string(decrypted) != string(tt.plaintext) {
				t.Errorf("decrypted = %q, want %q", decrypted, tt.plaintext)
			}
		})
	}
}

func FuzzEncryptDecryptRoundTrip(f *testing.F) {
	f.Add([]byte(`{"activationCode":"abc"}`), "RG-12345678", "Metis")
	f.Add([]byte(`{}`), "RG-test", "")

	f.Fuzz(func(t *testing.T, plaintext []byte, regCode string, productName string) {
		if len(regCode) == 0 {
			t.Skip()
		}
		encrypted, err := EncryptLicenseFile(plaintext, regCode, productName)
		if err != nil {
			t.Fatalf("encrypt failed: %v", err)
		}
		decrypted, err := DecryptLicenseFile(encrypted, regCode)
		if err != nil {
			t.Fatalf("decrypt failed: %v", err)
		}
		if string(decrypted) != string(plaintext) {
			t.Fatalf("round-trip failed: %q vs %q", decrypted, plaintext)
		}
	})
}

func TestDecryptLicenseFile_InvalidFormat(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"empty", ""},
		{"no dot", "abcdef"},
		{"dot at start", ".abcdef"},
		{"invalid base64", "TOKEN.not-base64!!!"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := DecryptLicenseFile(tt.input, "RG-test")
			if err == nil {
				t.Error("expected error for invalid input")
			}
		})
	}
}

func TestCanonicalizeDeterminism(t *testing.T) {
	input := map[string]any{
		"z": []any{
			map[string]any{"d": 4, "c": 3},
			map[string]any{"b": 2, "a": 1},
		},
		"a": map[string]any{"y": 25, "x": 24},
	}

	first, err := Canonicalize(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for i := 0; i < 10; i++ {
		got, err := Canonicalize(input)
		if err != nil {
			t.Fatalf("unexpected error on iteration %d: %v", i, err)
		}
		if got != first {
			t.Fatalf("non-deterministic output at iteration %d: %q vs %q", i, got, first)
		}
	}
}
