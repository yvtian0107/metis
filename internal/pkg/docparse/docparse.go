// Package docparse provides document text extraction for common file formats.
// It supports TXT, MD, PDF, DOCX, XLSX, and PPTX with pure Go implementations
// (no CGO required).
package docparse

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Parse extracts plain text from the given file path.
// It dispatches to the appropriate parser based on file extension.
func Parse(filePath string) (string, error) {
	ext := strings.ToLower(filepath.Ext(filePath))

	switch ext {
	case ".txt", ".md", ".markdown":
		return parseText(filePath)
	case ".pdf":
		return parsePDF(filePath)
	case ".docx":
		return parseDOCX(filePath)
	case ".xlsx":
		return parseXLSX(filePath)
	case ".pptx":
		return parsePPTX(filePath)
	default:
		return "", fmt.Errorf("unsupported file format: %s", ext)
	}
}

// SupportedExtensions returns the list of file extensions this package can parse.
func SupportedExtensions() []string {
	return []string{".txt", ".md", ".markdown", ".pdf", ".docx", ".xlsx", ".pptx"}
}

// IsSupportedMIME checks whether the given MIME type is supported.
func IsSupportedMIME(mime string) bool {
	supported := map[string]bool{
		"text/plain":                                                        true,
		"text/markdown":                                                     true,
		"application/pdf":                                                   true,
		"application/vnd.openxmlformats-officedocument.wordprocessingml.document":   true,
		"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet":         true,
		"application/vnd.openxmlformats-officedocument.presentationml.presentation": true,
	}
	return supported[mime]
}

func parseText(filePath string) (string, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return "", fmt.Errorf("read file: %w", err)
	}
	return string(data), nil
}
