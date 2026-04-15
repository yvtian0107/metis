package docparse

import (
	"archive/zip"
	"encoding/xml"
	"fmt"
	"strings"
)

// docxDocument represents the simplified structure of word/document.xml.
type docxDocument struct {
	Body docxBody `xml:"body"`
}

type docxBody struct {
	Paragraphs []docxParagraph `xml:"p"`
}

type docxParagraph struct {
	Runs []docxRun `xml:"r"`
}

type docxRun struct {
	Text []docxText `xml:"t"`
}

type docxText struct {
	Value string `xml:",chardata"`
}

func parseDOCX(filePath string) (string, error) {
	r, err := zip.OpenReader(filePath)
	if err != nil {
		return "", fmt.Errorf("open docx: %w", err)
	}
	defer r.Close()

	for _, f := range r.File {
		if f.Name == "word/document.xml" {
			rc, err := f.Open()
			if err != nil {
				return "", fmt.Errorf("open document.xml: %w", err)
			}
			defer rc.Close()

			var doc docxDocument
			if err := xml.NewDecoder(rc).Decode(&doc); err != nil {
				return "", fmt.Errorf("decode document.xml: %w", err)
			}

			var paragraphs []string
			for _, p := range doc.Body.Paragraphs {
				var line strings.Builder
				for _, run := range p.Runs {
					for _, t := range run.Text {
						line.WriteString(t.Value)
					}
				}
				if s := strings.TrimSpace(line.String()); s != "" {
					paragraphs = append(paragraphs, s)
				}
			}
			return strings.Join(paragraphs, "\n"), nil
		}
	}

	return "", fmt.Errorf("word/document.xml not found in docx")
}
