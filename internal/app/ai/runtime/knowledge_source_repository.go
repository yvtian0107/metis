package runtime

import (
	"github.com/samber/do/v2"
	"gorm.io/gorm"

	"metis/internal/database"
)

// KnowledgeSourceRepo provides GORM persistence for the independent source pool.
type KnowledgeSourceRepo struct {
	db *database.DB
}

// SourceListParams holds query parameters for listing sources.
type SourceListParams struct {
	Format        string // pdf, url, markdown, …
	ExtractStatus string // pending, completed, error
	Keyword       string
	Page          int
	PageSize      int
}

func NewKnowledgeSourceRepo(i do.Injector) (*KnowledgeSourceRepo, error) {
	db := do.MustInvoke[*database.DB](i)
	return &KnowledgeSourceRepo{db: db}, nil
}

func (r *KnowledgeSourceRepo) Create(source *KnowledgeSource) error {
	return r.db.Create(source).Error
}

func (r *KnowledgeSourceRepo) FindByID(id uint) (*KnowledgeSource, error) {
	var src KnowledgeSource
	if err := r.db.First(&src, id).Error; err != nil {
		return nil, err
	}
	return &src, nil
}

func (r *KnowledgeSourceRepo) FindByIDs(ids []uint) ([]KnowledgeSource, error) {
	var sources []KnowledgeSource
	if len(ids) == 0 {
		return sources, nil
	}
	if err := r.db.Where("id IN ?", ids).Find(&sources).Error; err != nil {
		return nil, err
	}
	return sources, nil
}

func (r *KnowledgeSourceRepo) List(params SourceListParams) ([]KnowledgeSource, int64, error) {
	if params.Page < 1 {
		params.Page = 1
	}
	if params.PageSize < 1 {
		params.PageSize = 20
	}

	q := r.db.Model(&KnowledgeSource{})

	if params.Format != "" {
		q = q.Where("format = ?", params.Format)
	}
	if params.ExtractStatus != "" {
		q = q.Where("extract_status = ?", params.ExtractStatus)
	}
	if params.Keyword != "" {
		like := "%" + params.Keyword + "%"
		q = q.Where("title LIKE ? OR source_url LIKE ? OR file_name LIKE ?", like, like, like)
	}

	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var sources []KnowledgeSource
	offset := (params.Page - 1) * params.PageSize
	if err := q.Order("id DESC").Offset(offset).Limit(params.PageSize).Find(&sources).Error; err != nil {
		return nil, 0, err
	}
	return sources, total, nil
}

func (r *KnowledgeSourceRepo) Update(source *KnowledgeSource) error {
	return r.db.Save(source).Error
}

func (r *KnowledgeSourceRepo) Delete(id uint) error {
	return r.db.Delete(&KnowledgeSource{}, id).Error
}

// UpdateExtractStatus atomically updates the extract status and optionally the error message.
func (r *KnowledgeSourceRepo) UpdateExtractStatus(id uint, status, errMsg string) error {
	return r.db.Model(&KnowledgeSource{}).Where("id = ?", id).
		Updates(map[string]any{
			"extract_status": status,
			"error_message":  errMsg,
		}).Error
}

// FindCrawlEnabledSources returns all URL sources with crawl_enabled=true.
func (r *KnowledgeSourceRepo) FindCrawlEnabledSources() ([]KnowledgeSource, error) {
	var sources []KnowledgeSource
	err := r.db.Where("format = ? AND crawl_enabled = ?", SourceFormatURL, true).Find(&sources).Error
	return sources, err
}

// FindByContentHash finds sources with the given content hash (for dedup).
func (r *KnowledgeSourceRepo) FindByContentHash(hash string) ([]KnowledgeSource, error) {
	var sources []KnowledgeSource
	err := r.db.Where("content_hash = ?", hash).Find(&sources).Error
	return sources, err
}

// GormDB exposes the underlying *gorm.DB for advanced queries.
func (r *KnowledgeSourceRepo) GormDB() *gorm.DB {
	return r.db.DB
}

// --- Legacy methods (used by compile/extract services during migration) ---

// FindCompletedByKbID returns completed sources for a legacy KnowledgeBase.
// Uses raw SQL against the still-existing kb_id column in the database.
// Will be removed when compile service migrates to KnowledgeAsset + KnowledgeEngine.
func (r *KnowledgeSourceRepo) FindCompletedByKbID(kbID uint) ([]KnowledgeSource, error) {
	var sources []KnowledgeSource
	err := r.db.Where("kb_id = ? AND extract_status = ?", kbID, ExtractStatusCompleted).Find(&sources).Error
	return sources, err
}

// FindByKbID returns all sources for a legacy KnowledgeBase.
func (r *KnowledgeSourceRepo) FindByKbID(kbID uint) ([]KnowledgeSource, error) {
	var sources []KnowledgeSource
	err := r.db.Where("kb_id = ?", kbID).Find(&sources).Error
	return sources, err
}

// DeleteByKbID deletes all sources for a legacy KnowledgeBase.
func (r *KnowledgeSourceRepo) DeleteByKbID(kbID uint) error {
	return r.db.Where("kb_id = ?", kbID).Delete(&KnowledgeSource{}).Error
}

// FindChildIDs returns IDs of child sources (by parent_id).
func (r *KnowledgeSourceRepo) FindChildIDs(parentID uint) ([]uint, error) {
	var ids []uint
	err := r.db.Model(&KnowledgeSource{}).Where("parent_id = ?", parentID).Pluck("id", &ids).Error
	return ids, err
}

// DeleteByParentID deletes all child sources of a parent.
func (r *KnowledgeSourceRepo) DeleteByParentID(parentID uint) error {
	return r.db.Where("parent_id = ?", parentID).Delete(&KnowledgeSource{}).Error
}
