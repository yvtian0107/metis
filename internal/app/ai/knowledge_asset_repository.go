package ai

import (
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
	var assets []KnowledgeAsset
	var total int64

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

	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	page := params.Page
	if page < 1 {
		page = 1
	}
	pageSize := params.PageSize
	if pageSize < 1 {
		pageSize = 20
	}

	if err := q.Order("id DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&assets).Error; err != nil {
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
	subQuery := r.db.Model(&KnowledgeAssetSource{}).Where("asset_id = ?", id).Select("COUNT(*)")
	return r.db.Model(&KnowledgeAsset{}).Where("id = ?", id).
		Update("source_count", subQuery).Error
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

func (r *KnowledgeAssetRepo) DB() *gorm.DB {
	return r.db.DB
}

// ----- Asset-Source association -----

// AddSources associates sources with an asset.
func (r *KnowledgeAssetRepo) AddSources(assetID uint, sourceIDs []uint) error {
	if len(sourceIDs) == 0 {
		return nil
	}
	assocs := make([]KnowledgeAssetSource, len(sourceIDs))
	for i, sid := range sourceIDs {
		assocs[i] = KnowledgeAssetSource{AssetID: assetID, SourceID: sid}
	}
	return r.db.Create(&assocs).Error
}

// RemoveSource removes a source association from an asset.
func (r *KnowledgeAssetRepo) RemoveSource(assetID, sourceID uint) error {
	return r.db.Where("asset_id = ? AND source_id = ?", assetID, sourceID).
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
