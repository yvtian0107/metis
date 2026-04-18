package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"regexp"
	"strings"

	"metis/internal/llm"
)

// --- Long Document Types ---

type sourceChunk struct {
	Index   int
	Title   string // section/chapter title if available
	Content string
}

type scanConcept struct {
	Title    string   `json:"title"`
	Summary  string   `json:"summary"`
	Keywords []string `json:"keywords"`
}

type scanResult struct {
	Concepts []scanConcept `json:"concepts"`
}

type globalConcept struct {
	Title        string
	Summary      string // best summary (longest)
	ChunkIndices []int
}

// --- Chunking ---

var headerPattern = regexp.MustCompile(`(?m)^#{1,3}\s+.+$`)

// chunkSource splits content by natural boundaries: section headers > paragraphs > fixed length.
func chunkSource(content string, maxSize int) []sourceChunk {
	if len(content) <= maxSize {
		return []sourceChunk{{Index: 0, Title: "", Content: content}}
	}

	// Try splitting by headers first
	chunks := chunkByHeaders(content, maxSize)
	if len(chunks) > 1 {
		return chunks
	}

	// Fallback: split by double-newline paragraphs
	chunks = chunkByParagraphs(content, maxSize)
	if len(chunks) > 1 {
		return chunks
	}

	// Last resort: fixed-length split
	return chunkByFixedLength(content, maxSize)
}

func chunkByHeaders(content string, maxSize int) []sourceChunk {
	locs := headerPattern.FindAllStringIndex(content, -1)
	if len(locs) == 0 {
		return nil
	}

	var chunks []sourceChunk
	var currentTitle string
	var currentStart int

	for i, loc := range locs {
		// If this isn't the first header and the accumulated content exceeds maxSize, flush
		if i > 0 {
			section := content[currentStart:loc[0]]
			if len(section) > maxSize {
				// This section alone is too big, split it further
				subChunks := chunkByParagraphs(section, maxSize)
				for _, sc := range subChunks {
					sc.Index = len(chunks)
					if sc.Title == "" {
						sc.Title = currentTitle
					}
					chunks = append(chunks, sc)
				}
			} else if len(strings.TrimSpace(section)) > 0 {
				chunks = append(chunks, sourceChunk{
					Index:   len(chunks),
					Title:   currentTitle,
					Content: section,
				})
			}
			currentStart = loc[0]
		}
		currentTitle = strings.TrimSpace(content[loc[0]:loc[1]])
	}

	// Don't forget the last section
	if currentStart < len(content) {
		lastSection := content[currentStart:]
		if len(lastSection) > maxSize {
			subChunks := chunkByParagraphs(lastSection, maxSize)
			for _, sc := range subChunks {
				sc.Index = len(chunks)
				if sc.Title == "" {
					sc.Title = currentTitle
				}
				chunks = append(chunks, sc)
			}
		} else if len(strings.TrimSpace(lastSection)) > 0 {
			chunks = append(chunks, sourceChunk{
				Index:   len(chunks),
				Title:   currentTitle,
				Content: lastSection,
			})
		}
	}

	return chunks
}

func chunkByParagraphs(content string, maxSize int) []sourceChunk {
	paragraphs := strings.Split(content, "\n\n")
	var chunks []sourceChunk
	var current strings.Builder

	for _, para := range paragraphs {
		para = strings.TrimSpace(para)
		if para == "" {
			continue
		}
		if current.Len()+len(para)+2 > maxSize && current.Len() > 0 {
			chunks = append(chunks, sourceChunk{
				Index:   len(chunks),
				Content: current.String(),
			})
			current.Reset()
		}
		if current.Len() > 0 {
			current.WriteString("\n\n")
		}
		current.WriteString(para)
	}
	if current.Len() > 0 {
		chunks = append(chunks, sourceChunk{
			Index:   len(chunks),
			Content: current.String(),
		})
	}

	return chunks
}

func chunkByFixedLength(content string, maxSize int) []sourceChunk {
	var chunks []sourceChunk
	for i := 0; i < len(content); i += maxSize {
		end := i + maxSize
		if end > len(content) {
			end = len(content)
		}
		chunks = append(chunks, sourceChunk{
			Index:   len(chunks),
			Content: content[i:end],
		})
	}
	return chunks
}

// --- Scan Phase ---

// scanChunks calls LLM in parallel to extract lightweight concept lists from each chunk.
func (s *KnowledgeCompileService) scanChunks(ctx context.Context, llmClient llm.Client, modelID string, chunks []sourceChunk) ([]scanResult, error) {
	results := make([]scanResult, len(chunks))
	errors := make([]error, len(chunks))

	// Process sequentially for simplicity (can be parallelized later)
	for i, chunk := range chunks {
		prompt := chunk.Content
		if chunk.Title != "" {
			prompt = fmt.Sprintf("## %s\n\n%s", chunk.Title, chunk.Content)
		}

		resp, err := llmClient.Chat(ctx, llm.ChatRequest{
			Model:     modelID,
			MaxTokens: 8192,
			Messages: []llm.Message{
				{Role: llm.RoleSystem, Content: scanSystemPrompt},
				{Role: llm.RoleUser, Content: prompt},
			},
		})
		if err != nil {
			errors[i] = err
			slog.Warn("scan chunk failed", "chunk_index", i, "error", err)
			continue
		}

		jsonStr := llm.ExtractJSON(resp.Content)
		var sr scanResult
		if err := json.Unmarshal([]byte(jsonStr), &sr); err != nil {
			errors[i] = fmt.Errorf("parse scan result: %w", err)
			slog.Warn("parse scan result failed", "chunk_index", i, "error", err)
			continue
		}
		results[i] = sr
	}

	// Check if all failed
	allFailed := true
	for _, e := range errors {
		if e == nil {
			allFailed = false
			break
		}
	}
	if allFailed {
		return nil, fmt.Errorf("all %d chunks failed in scan phase", len(chunks))
	}

	return results, nil
}

// --- Merge Phase ---

// mergeScannedConcepts deduplicates concepts by exact title and builds concept→chunk mapping.
func mergeScannedConcepts(scanResults []scanResult, chunks []sourceChunk) []globalConcept {
	conceptMap := make(map[string]*globalConcept) // title → concept
	var order []string                            // preserve insertion order

	for i, sr := range scanResults {
		for _, c := range sr.Concepts {
			title := strings.TrimSpace(c.Title)
			if title == "" {
				continue
			}
			if existing, ok := conceptMap[title]; ok {
				existing.ChunkIndices = append(existing.ChunkIndices, i)
				// Keep the longer summary
				if len(c.Summary) > len(existing.Summary) {
					existing.Summary = c.Summary
				}
			} else {
				conceptMap[title] = &globalConcept{
					Title:        title,
					Summary:      c.Summary,
					ChunkIndices: []int{i},
				}
				order = append(order, title)
			}
		}
	}

	result := make([]globalConcept, 0, len(order))
	for _, title := range order {
		result = append(result, *conceptMap[title])
	}
	return result
}

// --- Gather Phase ---

// gatherEvidence collects relevant paragraphs from original chunks for a concept.
func gatherEvidence(concept globalConcept, chunks []sourceChunk, maxSize int) string {
	type scoredParagraph struct {
		text    string
		density float64
	}

	titleLower := strings.ToLower(concept.Title)
	titleWords := strings.Fields(titleLower)
	var paragraphs []scoredParagraph

	for _, idx := range concept.ChunkIndices {
		if idx >= len(chunks) {
			continue
		}
		chunk := chunks[idx]
		// Split chunk into paragraphs
		paras := strings.Split(chunk.Content, "\n\n")
		for _, para := range paras {
			para = strings.TrimSpace(para)
			if para == "" {
				continue
			}
			// Calculate mention density
			paraLower := strings.ToLower(para)
			matchCount := 0
			for _, w := range titleWords {
				matchCount += strings.Count(paraLower, w)
			}
			density := float64(matchCount) / float64(len(strings.Fields(para))+1)
			paragraphs = append(paragraphs, scoredParagraph{text: para, density: density})
		}
	}

	// Sort by density (highest first) — simple insertion sort is fine for typical sizes
	for i := 1; i < len(paragraphs); i++ {
		for j := i; j > 0 && paragraphs[j].density > paragraphs[j-1].density; j-- {
			paragraphs[j], paragraphs[j-1] = paragraphs[j-1], paragraphs[j]
		}
	}

	// Collect paragraphs within budget
	var result strings.Builder
	var snippets []string
	for _, p := range paragraphs {
		if result.Len()+len(p.text)+2 <= maxSize {
			if result.Len() > 0 {
				result.WriteString("\n\n")
			}
			result.WriteString(p.text)
		} else if len(snippets) < 5 {
			// Create snippet: first 2 + last 2 sentences
			snippet := makeSnippet(p.text)
			if snippet != "" {
				snippets = append(snippets, snippet)
			}
		}
	}

	if len(snippets) > 0 {
		result.WriteString("\n\n--- Additional context (snippets) ---\n\n")
		for _, s := range snippets {
			result.WriteString(s)
			result.WriteString("\n\n")
		}
	}

	return result.String()
}

// makeSnippet returns the first 2 and last 2 sentences of a paragraph.
func makeSnippet(text string) string {
	sentences := splitSentences(text)
	if len(sentences) <= 4 {
		return text
	}
	return strings.Join(sentences[:2], " ") + " [...] " + strings.Join(sentences[len(sentences)-2:], " ")
}

var sentenceSplitter = regexp.MustCompile(`[.!?。！？]\s+`)

func splitSentences(text string) []string {
	indices := sentenceSplitter.FindAllStringIndex(text, -1)
	if len(indices) == 0 {
		return []string{text}
	}

	var sentences []string
	prev := 0
	for _, idx := range indices {
		s := strings.TrimSpace(text[prev:idx[1]])
		if s != "" {
			sentences = append(sentences, s)
		}
		prev = idx[1]
	}
	// Last part
	if prev < len(text) {
		s := strings.TrimSpace(text[prev:])
		if s != "" {
			sentences = append(sentences, s)
		}
	}
	return sentences
}

// --- Write Phase ---

// writeConceptArticles calls LLM to produce full articles from evidence bundles.
func (s *KnowledgeCompileService) writeConceptArticles(ctx context.Context, llmClient llm.Client, modelID string, concepts []globalConcept, evidences map[string]string, cfg CompileConfig) []mapNodeOutput {
	systemPrompt := fmt.Sprintf(mapSystemPrompt, cfg.MinContentLength, cfg.TargetContentLength)

	var results []mapNodeOutput

	for _, concept := range concepts {
		evidence, ok := evidences[concept.Title]
		if !ok || len(evidence) == 0 {
			continue
		}

		prompt := fmt.Sprintf("## Concept to write about: %s\n\nSummary: %s\n\n## Source material:\n\n%s", concept.Title, concept.Summary, evidence)

		resp, err := llmClient.Chat(ctx, llm.ChatRequest{
			Model:     modelID,
			MaxTokens: 16384,
			Messages: []llm.Message{
				{Role: llm.RoleSystem, Content: systemPrompt},
				{Role: llm.RoleUser, Content: prompt},
			},
		})
		if err != nil {
			slog.Warn("write concept article failed", "concept", concept.Title, "error", err)
			continue
		}

		jsonStr := llm.ExtractJSON(resp.Content)
		var mr mapResult
		if err := json.Unmarshal([]byte(jsonStr), &mr); err != nil {
			slog.Warn("parse write result failed", "concept", concept.Title, "error", err)
			continue
		}

		results = append(results, mr.Nodes...)
	}

	return results
}

// --- Integration ---

// mapSourceLongDoc handles sources that exceed maxChunkSize using the three-phase pipeline.
func (s *KnowledgeCompileService) mapSourceLongDoc(ctx context.Context, llmClient llm.Client, modelID string, src KnowledgeSource, cfg CompileConfig) mapSourceResult {
	maxSize := cfg.MaxChunkSize
	if maxSize <= 0 {
		maxSize = 12000
	}

	slog.Info("knowledge compile: using long-doc pipeline", "source", src.Title, "content_len", len(src.Content), "max_chunk", maxSize)

	// Phase 1: CHUNK
	chunks := chunkSource(src.Content, maxSize)
	slog.Info("knowledge compile: chunked source", "source", src.Title, "chunks", len(chunks))

	// Phase 2: SCAN
	scanResults, err := s.scanChunks(ctx, llmClient, modelID, chunks)
	if err != nil {
		return mapSourceResult{SourceTitle: src.Title, SourceID: src.ID, Error: fmt.Errorf("scan phase: %w", err)}
	}

	// Phase 3: MERGE
	concepts := mergeScannedConcepts(scanResults, chunks)
	slog.Info("knowledge compile: merged concepts", "source", src.Title, "concepts", len(concepts))

	if len(concepts) == 0 {
		return mapSourceResult{SourceTitle: src.Title, SourceID: src.ID, Error: fmt.Errorf("no concepts found in long document")}
	}

	// Phase 4: GATHER
	evidences := make(map[string]string)
	for _, c := range concepts {
		evidences[c.Title] = gatherEvidence(c, chunks, maxSize)
	}

	// Phase 5: WRITE
	nodes := s.writeConceptArticles(ctx, llmClient, modelID, concepts, evidences, cfg)

	return mapSourceResult{
		SourceTitle: src.Title,
		SourceID:    src.ID,
		Nodes:       nodes,
	}
}

