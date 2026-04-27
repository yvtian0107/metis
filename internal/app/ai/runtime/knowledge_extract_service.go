package runtime

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	htmltomarkdown "github.com/JohannesKaufmann/html-to-markdown/v2"
	"github.com/robfig/cron/v3"
	"github.com/samber/do/v2"

	"metis/internal/scheduler"
)

type KnowledgeExtractService struct {
	sourceRepo *KnowledgeSourceRepo
	assetRepo  *KnowledgeAssetRepo
	engine     *scheduler.Engine
}

func NewKnowledgeExtractService(i do.Injector) (*KnowledgeExtractService, error) {
	return &KnowledgeExtractService{
		sourceRepo: do.MustInvoke[*KnowledgeSourceRepo](i),
		assetRepo:  do.MustInvoke[*KnowledgeAssetRepo](i),
		engine:     do.MustInvoke[*scheduler.Engine](i),
	}, nil
}

type extractPayload struct {
	SourceID uint `json:"sourceId"`
}

func (s *KnowledgeExtractService) HandleExtract(ctx context.Context, payload json.RawMessage) error {
	var p extractPayload
	if err := json.Unmarshal(payload, &p); err != nil {
		return fmt.Errorf("unmarshal payload: %w", err)
	}

	src, err := s.sourceRepo.FindByID(p.SourceID)
	if err != nil {
		return fmt.Errorf("find source %d: %w", p.SourceID, err)
	}

	var content string
	var extractErr error

	switch src.Format {
	case SourceFormatMarkdown, SourceFormatText:
		// Already extracted at upload time
		return nil
	case SourceFormatURL:
		content, extractErr = s.extractURL(ctx, src)
	case SourceFormatPDF:
		content, extractErr = s.extractPDF(src)
	case SourceFormatDocx:
		content, extractErr = s.extractDocx(src)
	case SourceFormatXlsx:
		content, extractErr = s.extractXlsx(src)
	case SourceFormatPptx:
		content, extractErr = s.extractPptx(src)
	default:
		extractErr = fmt.Errorf("unsupported format: %s", src.Format)
	}

	if extractErr != nil {
		src.ExtractStatus = ExtractStatusError
		src.ErrorMessage = extractErr.Error()
		if err := s.sourceRepo.Update(src); err != nil {
			slog.Error("failed to update source status after extract error", "source_id", src.ID, "error", err)
		}
		return extractErr
	}

	src.Content = content
	src.ContentHash = hashContent(content)
	src.ExtractStatus = ExtractStatusCompleted
	src.ErrorMessage = ""
	if err := s.sourceRepo.Update(src); err != nil {
		return err
	}

	// Update source counts and trigger auto-build for all referencing assets
	s.notifyReferencingAssets(src.ID)

	return nil
}

func (s *KnowledgeExtractService) extractURL(ctx context.Context, src *KnowledgeSource) (string, error) {
	client := &http.Client{Timeout: 30 * time.Second}
	req, err := http.NewRequestWithContext(ctx, "GET", src.SourceURL, nil)
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("User-Agent", "Metis-Knowledge-Crawler/1.0")

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("fetch URL: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("HTTP %d from %s", resp.StatusCode, src.SourceURL)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 10*1024*1024)) // 10MB limit
	if err != nil {
		return "", fmt.Errorf("read body: %w", err)
	}

	contentType := resp.Header.Get("Content-Type")
	var content string
	if strings.Contains(contentType, "text/html") {
		content = simpleHTMLToMarkdown(string(body))
	} else {
		content = string(body)
	}

	// Handle crawl depth
	if src.CrawlDepth > 0 {
		s.crawlChildPages(ctx, src, string(body))
	}

	return content, nil
}

func (s *KnowledgeExtractService) crawlChildPages(ctx context.Context, parent *KnowledgeSource, htmlBody string) {
	baseURL, err := url.Parse(parent.SourceURL)
	if err != nil {
		return
	}

	links := extractLinks(htmlBody, baseURL)
	for _, link := range links {
		// Filter by url_pattern if set
		if parent.URLPattern != "" && !matchURLPattern(link, parent.URLPattern) {
			continue
		}

		child := &KnowledgeSource{
			ParentID:      &parent.ID,
			Title:         link,
			Format:        SourceFormatURL,
			SourceURL:     link,
			CrawlDepth:    parent.CrawlDepth - 1,
			URLPattern:    parent.URLPattern,
			ExtractStatus: ExtractStatusPending,
		}
		if err := s.sourceRepo.Create(child); err != nil {
			slog.Error("failed to create child source", "url", link, "error", err)
			continue
		}

		if err := s.engine.Enqueue("ai-source-extract", json.RawMessage(
			fmt.Sprintf(`{"sourceId":%d}`, child.ID),
		)); err != nil {
			slog.Error("failed to enqueue child source extract", "source_id", child.ID, "error", err)
		}
	}
}

// extractPDF extracts text from a PDF. Placeholder — needs a pure Go PDF library.
func (s *KnowledgeExtractService) extractPDF(src *KnowledgeSource) (string, error) {
	// TODO: Integrate a pure Go PDF text extraction library (e.g., ledongthuc/pdf)
	return "", fmt.Errorf("PDF extraction not yet implemented — upload as Markdown instead")
}

// extractDocx extracts text from a .docx file. Placeholder.
func (s *KnowledgeExtractService) extractDocx(src *KnowledgeSource) (string, error) {
	// TODO: Integrate a Go DOCX parser
	return "", fmt.Errorf("DOCX extraction not yet implemented — upload as Markdown instead")
}

// extractXlsx extracts text from an .xlsx file. Placeholder.
func (s *KnowledgeExtractService) extractXlsx(src *KnowledgeSource) (string, error) {
	// TODO: Integrate excelize for XLSX parsing
	return "", fmt.Errorf("XLSX extraction not yet implemented — upload as Markdown instead")
}

// extractPptx extracts text from a .pptx file. Placeholder.
func (s *KnowledgeExtractService) extractPptx(src *KnowledgeSource) (string, error) {
	// TODO: Integrate a Go PPTX parser
	return "", fmt.Errorf("PPTX extraction not yet implemented — upload as Markdown instead")
}

// EnqueueExtract enqueues a source extraction task.
func (s *KnowledgeExtractService) EnqueueExtract(sourceID uint) error {
	return s.engine.Enqueue("ai-source-extract", json.RawMessage(
		fmt.Sprintf(`{"sourceId":%d}`, sourceID),
	))
}

// notifyReferencingAssets updates source counts and triggers auto-build for
// all assets that reference the given source.
func (s *KnowledgeExtractService) notifyReferencingAssets(sourceID uint) {
	assetIDs, err := s.assetRepo.ListAssetIDsBySource(sourceID)
	if err != nil {
		slog.Error("failed to find referencing assets", "source_id", sourceID, "error", err)
		return
	}
	for _, assetID := range assetIDs {
		if err := s.assetRepo.UpdateSourceCount(assetID); err != nil {
			slog.Error("failed to update asset source count", "asset_id", assetID, "error", err)
		}
		asset, err := s.assetRepo.FindByID(assetID)
		if err != nil {
			continue
		}
		if asset.AutoBuild {
			if err := s.enqueueCompile(assetID, false); err != nil {
				slog.Error("failed to enqueue auto-compile", "asset_id", assetID, "error", err)
			}
		}
	}
}

// enqueueCompile enqueues a knowledge compile task using a typed payload.
func (s *KnowledgeExtractService) enqueueCompile(kbID uint, recompile bool) error {
	payload := compilePayload{KbID: kbID, Recompile: recompile}
	b, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	return s.engine.Enqueue("ai-knowledge-compile", json.RawMessage(b))
}

func (s *KnowledgeExtractService) TaskDefs() []scheduler.TaskDef {
	return []scheduler.TaskDef{
		{
			Name:        "ai-source-extract",
			Type:        scheduler.TypeAsync,
			Description: "Extract text content from knowledge sources (files/URLs)",
			Timeout:     120 * time.Second,
			MaxRetries:  3,
			Handler:     s.HandleExtract,
		},
		{
			Name:        "ai-knowledge-crawl",
			Type:        scheduler.TypeScheduled,
			CronExpr:    "*/5 * * * *", // every 5 minutes, checks per-source schedules
			Description: "Check and re-crawl URL sources with crawl enabled",
			Timeout:     600 * time.Second,
			MaxRetries:  1,
			Handler:     s.HandleCrawl,
		},
	}
}

// HandleCrawl checks all crawl-enabled URL sources and re-crawls those whose cron schedule is due.
func (s *KnowledgeExtractService) HandleCrawl(ctx context.Context, _ json.RawMessage) error {
	sources, err := s.sourceRepo.FindCrawlEnabledSources()
	if err != nil {
		return fmt.Errorf("find crawl-enabled sources: %w", err)
	}

	parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)
	now := time.Now()
	var affectedSources []uint

	for _, src := range sources {
		if src.CrawlSchedule == "" {
			continue
		}

		sched, err := parser.Parse(src.CrawlSchedule)
		if err != nil {
			slog.Error("crawl: invalid cron schedule", "source_id", src.ID, "schedule", src.CrawlSchedule, "error", err)
			continue
		}

		// Determine if this source is due for crawl
		lastCrawl := src.CreatedAt
		if src.LastCrawledAt != nil {
			lastCrawl = *src.LastCrawledAt
		}
		if sched.Next(lastCrawl).After(now) {
			continue // not due yet
		}

		slog.Info("crawl: re-crawling source", "source_id", src.ID, "url", src.SourceURL)

		oldHash := src.ContentHash
		content, extractErr := s.extractURL(ctx, &src)
		if extractErr != nil {
			slog.Error("crawl: extract failed", "source_id", src.ID, "error", extractErr)
			crawlNow := time.Now()
			src.LastCrawledAt = &crawlNow
			if err := s.sourceRepo.Update(&src); err != nil {
				slog.Error("crawl: update source failed after extract error", "source_id", src.ID, "error", err)
			}
			continue
		}

		crawlNow := time.Now()
		src.LastCrawledAt = &crawlNow

		newHash := hashContent(content)
		if newHash != oldHash {
			src.Content = content
			src.ContentHash = newHash
			src.ExtractStatus = ExtractStatusCompleted
			src.ErrorMessage = ""
			slog.Info("crawl: content changed", "source_id", src.ID)
			affectedSources = append(affectedSources, src.ID)
		}

		if err := s.sourceRepo.Update(&src); err != nil {
			slog.Error("crawl: update source failed", "source_id", src.ID, "error", err)
		}
	}

	// Notify referencing assets for all affected sources
	for _, srcID := range affectedSources {
		s.notifyReferencingAssets(srcID)
	}

	return nil
}

// --- Utilities ---

func hashContent(content string) string {
	h := sha256.Sum256([]byte(content))
	return hex.EncodeToString(h[:])
}

// simpleHTMLToMarkdown converts HTML to clean Markdown using html-to-markdown.
func simpleHTMLToMarkdown(html string) string {
	md, err := htmltomarkdown.ConvertString(html)
	if err != nil {
		slog.Warn("html-to-markdown conversion failed, falling back to tag stripping", "error", err)
		return stripHTMLTags(html)
	}
	return md
}

// stripHTMLTags is a minimal fallback: removes all HTML tags and cleans whitespace.
func stripHTMLTags(html string) string {
	var result strings.Builder
	inTag := false
	for _, r := range html {
		switch {
		case r == '<':
			inTag = true
		case r == '>':
			inTag = false
		case !inTag:
			result.WriteRune(r)
		}
	}
	lines := strings.Split(result.String(), "\n")
	var cleaned []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			cleaned = append(cleaned, line)
		}
	}
	return strings.Join(cleaned, "\n")
}

// extractLinks extracts same-domain absolute URLs from HTML.
func extractLinks(html string, base *url.URL) []string {
	var links []string
	seen := make(map[string]bool)

	lower := strings.ToLower(html)
	idx := 0
	for {
		pos := strings.Index(lower[idx:], "href=\"")
		if pos == -1 {
			break
		}
		start := idx + pos + 6
		end := strings.Index(html[start:], "\"")
		if end == -1 {
			break
		}
		href := html[start : start+end]
		idx = start + end

		parsed, err := url.Parse(href)
		if err != nil {
			continue
		}
		resolved := base.ResolveReference(parsed)

		// Same domain only
		if resolved.Host != base.Host {
			continue
		}
		// Skip anchors and non-http
		if resolved.Scheme != "http" && resolved.Scheme != "https" {
			continue
		}

		link := resolved.String()
		if !seen[link] {
			seen[link] = true
			links = append(links, link)
		}
	}
	return links
}

// matchURLPattern checks if a URL matches a simple glob pattern.
func matchURLPattern(urlStr, pattern string) bool {
	if pattern == "" {
		return true
	}
	// Simple prefix match: "docs.example.com/guide/*" matches URLs starting with that prefix
	pattern = strings.TrimSuffix(pattern, "*")
	return strings.Contains(urlStr, pattern)
}
