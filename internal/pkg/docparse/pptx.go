package docparse

import (
	"archive/zip"
	"encoding/xml"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
)

// pptxSlide represents the simplified structure of a PPTX slide XML.
type pptxSlide struct {
	CSld pptxCSld `xml:"cSld"`
}

type pptxCSld struct {
	SpTree pptxSpTree `xml:"spTree"`
}

type pptxSpTree struct {
	Shapes []pptxShape `xml:"sp"`
}

type pptxShape struct {
	TxBody *pptxTxBody `xml:"txBody"`
}

type pptxTxBody struct {
	Paragraphs []pptxParagraph `xml:"p"`
}

type pptxParagraph struct {
	Runs []pptxRun `xml:"r"`
}

type pptxRun struct {
	Text string `xml:"t"`
}

func parsePPTX(filePath string) (string, error) {
	r, err := zip.OpenReader(filePath)
	if err != nil {
		return "", fmt.Errorf("open pptx: %w", err)
	}
	defer r.Close()

	// Collect slide files in order
	var slideFiles []*zip.File
	for _, f := range r.File {
		dir := filepath.Dir(f.Name)
		base := filepath.Base(f.Name)
		if dir == "ppt/slides" && strings.HasPrefix(base, "slide") && strings.HasSuffix(base, ".xml") {
			slideFiles = append(slideFiles, f)
		}
	}

	sort.Slice(slideFiles, func(i, j int) bool {
		return slideFiles[i].Name < slideFiles[j].Name
	})

	var slides []string
	for _, sf := range slideFiles {
		text, err := extractSlideText(sf)
		if err != nil {
			continue
		}
		if s := strings.TrimSpace(text); s != "" {
			slides = append(slides, s)
		}
	}

	return strings.Join(slides, "\n\n"), nil
}

func extractSlideText(f *zip.File) (string, error) {
	rc, err := f.Open()
	if err != nil {
		return "", err
	}
	defer rc.Close()

	var slide pptxSlide
	if err := xml.NewDecoder(rc).Decode(&slide); err != nil {
		return "", err
	}

	var lines []string
	for _, sp := range slide.CSld.SpTree.Shapes {
		if sp.TxBody == nil {
			continue
		}
		for _, p := range sp.TxBody.Paragraphs {
			var sb strings.Builder
			for _, run := range p.Runs {
				sb.WriteString(run.Text)
			}
			if s := strings.TrimSpace(sb.String()); s != "" {
				lines = append(lines, s)
			}
		}
	}

	return strings.Join(lines, "\n"), nil
}
