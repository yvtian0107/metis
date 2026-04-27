package runtime

import (
	"errors"
	"fmt"
	"log/slog"

	"github.com/samber/do/v2"

	"metis/internal/scheduler"
)

// KnowledgeSourceService handles business logic for the independent source pool.
type KnowledgeSourceService struct {
	sourceRepo *KnowledgeSourceRepo
	assetRepo  *KnowledgeAssetRepo
	engine     *scheduler.Engine
}

func NewKnowledgeSourceService(i do.Injector) (*KnowledgeSourceService, error) {
	return &KnowledgeSourceService{
		sourceRepo: do.MustInvoke[*KnowledgeSourceRepo](i),
		assetRepo:  do.MustInvoke[*KnowledgeAssetRepo](i),
		engine:     do.MustInvoke[*scheduler.Engine](i),
	}, nil
}

// CreateFileSource creates a source from an uploaded file with content already extracted.
// For markdown/text the content is ready immediately; for other formats extraction is async.
func (s *KnowledgeSourceService) CreateFileSource(title, format, fileName string, byteSize int64, content string) (*KnowledgeSource, error) {
	extractStatus := ExtractStatusPending
	if format == SourceFormatMarkdown || format == SourceFormatText {
		extractStatus = ExtractStatusCompleted
	}

	src := &KnowledgeSource{
		Title:         title,
		Format:        format,
		FileName:      fileName,
		ByteSize:      byteSize,
		Content:       content,
		ExtractStatus: extractStatus,
	}
	if extractStatus == ExtractStatusCompleted && content != "" {
		src.ContentHash = hashContent(content)
	}

	if err := s.sourceRepo.Create(src); err != nil {
		return nil, fmt.Errorf("create file source: %w", err)
	}

	// Enqueue async extraction for non-text formats
	if extractStatus == ExtractStatusPending {
		if err := s.enqueueExtract(src.ID); err != nil {
			slog.Error("failed to enqueue source extract", "source_id", src.ID, "error", err)
		}
	}

	return src, nil
}

// CreateURLSource creates a source from a URL with optional crawl settings.
func (s *KnowledgeSourceService) CreateURLSource(title, sourceURL string, crawlDepth int, urlPattern string, crawlEnabled bool, crawlSchedule string) (*KnowledgeSource, error) {
	if title == "" {
		title = sourceURL
	}

	src := &KnowledgeSource{
		Title:         title,
		Format:        SourceFormatURL,
		SourceURL:     sourceURL,
		CrawlDepth:    crawlDepth,
		URLPattern:    urlPattern,
		CrawlEnabled:  crawlEnabled,
		CrawlSchedule: crawlSchedule,
		ExtractStatus: ExtractStatusPending,
	}
	if err := s.sourceRepo.Create(src); err != nil {
		return nil, fmt.Errorf("create url source: %w", err)
	}

	if err := s.enqueueExtract(src.ID); err != nil {
		slog.Error("failed to enqueue source extract", "source_id", src.ID, "error", err)
	}
	return src, nil
}

// CreateTextSource creates a source with inline text content (immediately ready).
func (s *KnowledgeSourceService) CreateTextSource(title, content string) (*KnowledgeSource, error) {
	src := &KnowledgeSource{
		Title:         title,
		Format:        SourceFormatText,
		Content:       content,
		ByteSize:      int64(len(content)),
		ExtractStatus: ExtractStatusCompleted,
		ContentHash:   hashContent(content),
	}
	if err := s.sourceRepo.Create(src); err != nil {
		return nil, fmt.Errorf("create text source: %w", err)
	}
	return src, nil
}

// Get returns a source by ID.
func (s *KnowledgeSourceService) Get(id uint) (*KnowledgeSource, error) {
	src, err := s.sourceRepo.FindByID(id)
	if err != nil {
		return nil, ErrSourceNotFound
	}
	return src, nil
}

// List returns paginated sources with filters.
func (s *KnowledgeSourceService) List(params SourceListParams) ([]KnowledgeSource, int64, error) {
	return s.sourceRepo.List(params)
}

// Delete removes a source if it is not referenced by any asset.
// Returns a list of referencing asset IDs on conflict.
func (s *KnowledgeSourceService) Delete(id uint) ([]uint, error) {
	// Check references
	refIDs, err := s.assetRepo.ListAssetIDsBySource(id)
	if err != nil {
		return nil, fmt.Errorf("check source refs: %w", err)
	}
	if len(refIDs) > 0 {
		return refIDs, errors.New("source is referenced by knowledge assets")
	}

	if err := s.sourceRepo.Delete(id); err != nil {
		return nil, fmt.Errorf("delete source: %w", err)
	}
	return nil, nil
}

// GetReferencingAssets returns the assets that reference the given source.
func (s *KnowledgeSourceService) GetReferencingAssets(sourceID uint) ([]KnowledgeAsset, error) {
	assetIDs, err := s.assetRepo.ListAssetIDsBySource(sourceID)
	if err != nil {
		return nil, fmt.Errorf("list asset refs: %w", err)
	}
	if len(assetIDs) == 0 {
		return nil, nil
	}
	return s.assetRepo.FindByIDs(assetIDs)
}

// GetRefCount returns the number of assets referencing a source.
func (s *KnowledgeSourceService) GetRefCount(sourceID uint) (int64, error) {
	return s.assetRepo.CountSourceRefs(sourceID)
}

// MarkAssetsStale marks all assets referencing this source as stale (needs rebuild).
func (s *KnowledgeSourceService) MarkAssetsStale(sourceID uint) error {
	assetIDs, err := s.assetRepo.ListAssetIDsBySource(sourceID)
	if err != nil {
		return fmt.Errorf("list asset refs: %w", err)
	}
	for _, aid := range assetIDs {
		if err := s.assetRepo.UpdateStatus(aid, AssetStatusStale); err != nil {
			slog.Error("failed to mark asset stale", "asset_id", aid, "error", err)
		}
	}
	return nil
}

func (s *KnowledgeSourceService) enqueueExtract(sourceID uint) error {
	return s.engine.Enqueue("ai-source-extract",
		[]byte(fmt.Sprintf(`{"sourceId":%d}`, sourceID)),
	)
}
