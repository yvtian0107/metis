package ai

import (
	"errors"
	"fmt"
	"log/slog"

	"github.com/samber/do/v2"
	"gorm.io/gorm"
)

// KnowledgeSource2Service handles business logic for the independent source pool.
type KnowledgeSource2Service struct {
	repo      *KnowledgeSource2Repo
	assetRepo *KnowledgeAssetRepo
}

func NewKnowledgeSource2Service(i do.Injector) (*KnowledgeSource2Service, error) {
	return &KnowledgeSource2Service{
		repo:      do.MustInvoke[*KnowledgeSource2Repo](i),
		assetRepo: do.MustInvoke[*KnowledgeAssetRepo](i),
	}, nil
}

// Create stores a new source. The source starts with ExtractStatus = pending
// for non-text formats.
func (s *KnowledgeSource2Service) Create(src *KnowledgeSource2) error {
	if src.Format == SourceFormatMarkdown || src.Format == SourceFormatText {
		src.ExtractStatus = ExtractStatusCompleted
	} else if src.ExtractStatus == "" {
		src.ExtractStatus = ExtractStatusPending
	}
	return s.repo.Create(src)
}

// Get retrieves a single source by ID.
func (s *KnowledgeSource2Service) Get(id uint) (*KnowledgeSource2, error) {
	src, err := s.repo.FindByID(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrSourceNotFound2
		}
		return nil, err
	}
	return src, nil
}

// Delete removes a source and its children. Returns an error if the source
// is still referenced by any knowledge asset.
func (s *KnowledgeSource2Service) Delete(id uint) error {
	// Check references
	refCount, err := s.assetRepo.CountSourceRefs(id)
	if err != nil {
		return fmt.Errorf("check source refs: %w", err)
	}
	if refCount > 0 {
		return fmt.Errorf("source is referenced by %d knowledge asset(s), remove associations first", refCount)
	}

	// Collect child IDs for cascading delete
	childIDs, err := s.repo.FindChildIDs(id)
	if err != nil {
		return fmt.Errorf("find child sources: %w", err)
	}

	// Delete children first
	if len(childIDs) > 0 {
		// Check children are not referenced either
		for _, cid := range childIDs {
			cref, _ := s.assetRepo.CountSourceRefs(cid)
			if cref > 0 {
				return fmt.Errorf("child source %d is referenced by %d asset(s)", cid, cref)
			}
		}
		if err := s.repo.DeleteByParentID(id); err != nil {
			return fmt.Errorf("delete child sources: %w", err)
		}
		slog.Info("deleted child sources", "parentId", id, "count", len(childIDs))
	}

	// Delete the source itself
	return s.repo.Delete(id)
}

// List returns paginated sources with optional filters.
func (s *KnowledgeSource2Service) List(params Source2ListParams) ([]KnowledgeSource2, int64, error) {
	return s.repo.List(params)
}

// GetReferencingAssets returns the list of assets that reference a given source.
func (s *KnowledgeSource2Service) GetReferencingAssets(sourceID uint) ([]KnowledgeAsset, error) {
	assetIDs, err := s.assetRepo.ListAssetIDsBySource(sourceID)
	if err != nil {
		return nil, err
	}
	if len(assetIDs) == 0 {
		return nil, nil
	}
	return s.assetRepo.FindByIDs(assetIDs)
}

// MarkStaleAssets marks all assets referencing a source as "stale" (needs rebuild).
func (s *KnowledgeSource2Service) MarkStaleAssets(sourceID uint) error {
	assetIDs, err := s.assetRepo.ListAssetIDsBySource(sourceID)
	if err != nil {
		return err
	}
	for _, aid := range assetIDs {
		if err := s.assetRepo.UpdateStatus(aid, AssetStatusStale); err != nil {
			slog.Warn("failed to mark asset stale", "assetId", aid, "err", err)
		}
	}
	return nil
}
