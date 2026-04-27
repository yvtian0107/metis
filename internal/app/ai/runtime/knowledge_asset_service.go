package runtime

import (
	"errors"
	"fmt"

	"github.com/samber/do/v2"
)

// KnowledgeAssetService provides shared business logic for knowledge assets
// (both KB and KG categories). Category-specific operations (build, search)
// are delegated to the registered KnowledgeEngine.
type KnowledgeAssetService struct {
	assetRepo  *KnowledgeAssetRepo
	sourceRepo *KnowledgeSourceRepo
	logRepo    *KnowledgeLogRepo
}

func NewKnowledgeAssetService(i do.Injector) (*KnowledgeAssetService, error) {
	return &KnowledgeAssetService{
		assetRepo:  do.MustInvoke[*KnowledgeAssetRepo](i),
		sourceRepo: do.MustInvoke[*KnowledgeSourceRepo](i),
		logRepo:    do.MustInvoke[*KnowledgeLogRepo](i),
	}, nil
}

// Create creates a new knowledge asset. Type is immutable after creation.
func (s *KnowledgeAssetService) Create(asset *KnowledgeAsset) error {
	if asset.Category == "" || asset.Type == "" {
		return errors.New("category and type are required")
	}
	if asset.Status == "" {
		asset.Status = AssetStatusIdle
	}
	return s.assetRepo.Create(asset)
}

// Get returns an asset by ID.
func (s *KnowledgeAssetService) Get(id uint) (*KnowledgeAsset, error) {
	asset, err := s.assetRepo.FindByID(id)
	if err != nil {
		return nil, ErrAssetNotFound
	}
	return asset, nil
}

// GetByCategory returns an asset by ID and validates its category.
func (s *KnowledgeAssetService) GetByCategory(id uint, category string) (*KnowledgeAsset, error) {
	asset, err := s.assetRepo.FindByID(id)
	if err != nil {
		return nil, ErrAssetNotFound
	}
	if asset.Category != category {
		return nil, fmt.Errorf("asset %d is not a %s", id, category)
	}
	return asset, nil
}

// List returns paginated assets with filters.
func (s *KnowledgeAssetService) List(params AssetListParams) ([]KnowledgeAsset, int64, error) {
	return s.assetRepo.List(params)
}

// Update updates an asset. Category and Type are immutable.
func (s *KnowledgeAssetService) Update(id uint, name, description string, config any, compileModelID, embeddingProviderID *uint, embeddingModelID string, autoBuild bool) (*KnowledgeAsset, error) {
	asset, err := s.assetRepo.FindByID(id)
	if err != nil {
		return nil, ErrAssetNotFound
	}
	asset.Name = name
	asset.Description = description
	asset.CompileModelID = compileModelID
	asset.EmbeddingProviderID = embeddingProviderID
	asset.EmbeddingModelID = embeddingModelID
	asset.AutoBuild = autoBuild
	if config != nil {
		if err := asset.SetConfig(config); err != nil {
			return nil, fmt.Errorf("set config: %w", err)
		}
	}
	if err := s.assetRepo.Update(asset); err != nil {
		return nil, err
	}
	return asset, nil
}

// Delete deletes an asset and its source associations and logs.
func (s *KnowledgeAssetService) Delete(id uint) error {
	if _, err := s.assetRepo.FindByID(id); err != nil {
		return ErrAssetNotFound
	}
	// Remove source associations
	if err := s.assetRepo.RemoveAllSources(id); err != nil {
		return fmt.Errorf("remove source associations: %w", err)
	}
	// Remove logs
	if err := s.logRepo.DeleteByAssetID(id); err != nil {
		return fmt.Errorf("remove logs: %w", err)
	}
	return s.assetRepo.Delete(id)
}

// --- Source association management ---

// AddSources associates sources with an asset and updates source count.
func (s *KnowledgeAssetService) AddSources(assetID uint, sourceIDs []uint) error {
	if _, err := s.assetRepo.FindByID(assetID); err != nil {
		return ErrAssetNotFound
	}
	if err := s.assetRepo.AddSources(assetID, sourceIDs); err != nil {
		return err
	}
	return s.assetRepo.UpdateSourceCount(assetID)
}

// RemoveSource removes a source association and updates source count.
func (s *KnowledgeAssetService) RemoveSource(assetID, sourceID uint) error {
	if err := s.assetRepo.RemoveSource(assetID, sourceID); err != nil {
		return err
	}
	return s.assetRepo.UpdateSourceCount(assetID)
}

// ListSources returns all sources associated with an asset.
func (s *KnowledgeAssetService) ListSources(assetID uint) ([]KnowledgeSource, error) {
	ids, err := s.assetRepo.ListSourceIDs(assetID)
	if err != nil {
		return nil, err
	}
	if len(ids) == 0 {
		return nil, nil
	}
	return s.sourceRepo.FindByIDs(ids)
}

// ListCompletedSources returns completed (extracted) sources for an asset.
func (s *KnowledgeAssetService) ListCompletedSources(assetID uint) ([]KnowledgeSource, error) {
	sources, err := s.ListSources(assetID)
	if err != nil {
		return nil, err
	}
	var completed []KnowledgeSource
	for _, src := range sources {
		if src.ExtractStatus == ExtractStatusCompleted {
			completed = append(completed, src)
		}
	}
	return completed, nil
}

// ListLogs returns build/compile logs for an asset.
func (s *KnowledgeAssetService) ListLogs(assetID uint, limit int) ([]KnowledgeLog, error) {
	return s.logRepo.FindByAssetID(assetID, limit)
}
