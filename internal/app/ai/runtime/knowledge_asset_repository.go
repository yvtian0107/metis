package runtime

import (
	"fmt"

	"github.com/samber/do/v2"
	"gorm.io/gorm"

	"metis/internal/database"
)

// KnowledgeAssetRepo provides GORM persistence for KnowledgeAsset.
type KnowledgeAssetRepo struct {
	db *database.DB
}

// AssetListParams holds query parameters for listing assets.
type AssetListParams struct {
	Category string // kb | kg | "" (all)
	Type     string // specific type or "" (all)
	Status   string // specific status or "" (all)
	Keyword  string
	Page     int
	PageSize int
}

func NewKnowledgeAssetRepo(i do.Injector) (*KnowledgeAssetRepo, error) {
	db := do.MustInvoke[*database.DB](i)
	return &KnowledgeAssetRepo{db: db}, nil
}

func (r *KnowledgeAssetRepo) Create(asset *KnowledgeAsset) error {
	return r.db.Create(asset).Error
}

func (r *KnowledgeAssetRepo) FindByID(id uint) (*KnowledgeAsset, error) {
	var asset KnowledgeAsset
	if err := r.db.First(&asset, id).Error; err != nil {
		return nil, err
	}
	return &asset, nil
}

func (r *KnowledgeAssetRepo) List(params AssetListParams) ([]KnowledgeAsset, int64, error) {
	if params.Page < 1 {
		params.Page = 1
	}
	if params.PageSize < 1 {
		params.PageSize = 20
	}

	q := r.db.Model(&KnowledgeAsset{})

	if params.Category != "" {
		q = q.Where("category = ?", params.Category)
	}
	if params.Type != "" {
		q = q.Where("type = ?", params.Type)
	}
	if params.Status != "" {
		q = q.Where("status = ?", params.Status)
	}
	if params.Keyword != "" {
		like := "%" + params.Keyword + "%"
		q = q.Where("name LIKE ? OR description LIKE ?", like, like)
	}

	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var assets []KnowledgeAsset
	offset := (params.Page - 1) * params.PageSize
	if err := q.Order("id DESC").Offset(offset).Limit(params.PageSize).Find(&assets).Error; err != nil {
		return nil, 0, err
	}

	return assets, total, nil
}

func (r *KnowledgeAssetRepo) Update(asset *KnowledgeAsset) error {
	return r.db.Save(asset).Error
}

func (r *KnowledgeAssetRepo) Delete(id uint) error {
	return r.db.Delete(&KnowledgeAsset{}, id).Error
}

func (r *KnowledgeAssetRepo) UpdateSourceCount(id uint) error {
	return r.db.Exec(`
		UPDATE ai_knowledge_assets SET
			source_count = (SELECT COUNT(*) FROM ai_knowledge_asset_sources WHERE asset_id = ?)
		WHERE id = ?
	`, id, id).Error
}

func (r *KnowledgeAssetRepo) UpdateStatus(id uint, status string) error {
	return r.db.Model(&KnowledgeAsset{}).Where("id = ?", id).
		Update("status", status).Error
}

func (r *KnowledgeAssetRepo) FindByIDs(ids []uint) ([]KnowledgeAsset, error) {
	var assets []KnowledgeAsset
	if len(ids) == 0 {
		return assets, nil
	}
	if err := r.db.Where("id IN ?", ids).Find(&assets).Error; err != nil {
		return nil, err
	}
	return assets, nil
}

func (r *KnowledgeAssetRepo) GormDB() *gorm.DB {
	return r.db.DB
}

// ----- Asset-Source association -----

// AddSources associates sources with an asset. Duplicates are silently ignored.
func (r *KnowledgeAssetRepo) AddSources(assetID uint, sourceIDs []uint) error {
	if len(sourceIDs) == 0 {
		return nil
	}
	for _, sid := range sourceIDs {
		assoc := KnowledgeAssetSource{AssetID: assetID, SourceID: sid}
		result := r.db.Where("asset_id = ? AND source_id = ?", assetID, sid).
			FirstOrCreate(&assoc)
		if result.Error != nil {
			return fmt.Errorf("add source %d to asset %d: %w", sid, assetID, result.Error)
		}
	}
	return nil
}

// RemoveSource removes a source association from an asset.
func (r *KnowledgeAssetRepo) RemoveSource(assetID, sourceID uint) error {
	return r.db.Where("asset_id = ? AND source_id = ?", assetID, sourceID).
		Delete(&KnowledgeAssetSource{}).Error
}

// RemoveAllSources removes all source associations for an asset.
func (r *KnowledgeAssetRepo) RemoveAllSources(assetID uint) error {
	return r.db.Where("asset_id = ?", assetID).
		Delete(&KnowledgeAssetSource{}).Error
}

// ListSourceIDs returns all source IDs associated with an asset.
func (r *KnowledgeAssetRepo) ListSourceIDs(assetID uint) ([]uint, error) {
	var ids []uint
	err := r.db.Model(&KnowledgeAssetSource{}).
		Where("asset_id = ?", assetID).Pluck("source_id", &ids).Error
	return ids, err
}

// ListAssetIDsBySource returns all asset IDs that reference a given source.
func (r *KnowledgeAssetRepo) ListAssetIDsBySource(sourceID uint) ([]uint, error) {
	var ids []uint
	err := r.db.Model(&KnowledgeAssetSource{}).
		Where("source_id = ?", sourceID).Pluck("asset_id", &ids).Error
	return ids, err
}

// CountSourceRefs counts how many assets reference a given source.
func (r *KnowledgeAssetRepo) CountSourceRefs(sourceID uint) (int64, error) {
	var count int64
	err := r.db.Model(&KnowledgeAssetSource{}).
		Where("source_id = ?", sourceID).Count(&count).Error
	return count, err
}
