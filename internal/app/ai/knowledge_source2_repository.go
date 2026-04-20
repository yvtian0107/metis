package ai

import (
	"github.com/samber/do/v2"

	"metis/internal/database"
)

// KnowledgeSource2Repo provides GORM persistence for the independent source pool.
type KnowledgeSource2Repo struct {
	db *database.DB
}

// Source2ListParams holds query parameters for listing sources.
type Source2ListParams struct {
	Keyword  string
	Format   string // filter by format, "" = all
	Status   string // filter by extract_status, "" = all
	Page     int
	PageSize int
}

func NewKnowledgeSource2Repo(i do.Injector) (*KnowledgeSource2Repo, error) {
	return &KnowledgeSource2Repo{db: do.MustInvoke[*database.DB](i)}, nil
}

func (r *KnowledgeSource2Repo) Create(s *KnowledgeSource2) error {
	return r.db.Create(s).Error
}

func (r *KnowledgeSource2Repo) FindByID(id uint) (*KnowledgeSource2, error) {
	var s KnowledgeSource2
	if err := r.db.First(&s, id).Error; err != nil {
		return nil, err
	}
	return &s, nil
}

func (r *KnowledgeSource2Repo) FindByIDs(ids []uint) ([]KnowledgeSource2, error) {
	var sources []KnowledgeSource2
	if len(ids) == 0 {
		return sources, nil
	}
	if err := r.db.Where("id IN ?", ids).Find(&sources).Error; err != nil {
		return nil, err
	}
	return sources, nil
}

func (r *KnowledgeSource2Repo) List(params Source2ListParams) ([]KnowledgeSource2, int64, error) {
	if params.Page < 1 {
		params.Page = 1
	}
	if params.PageSize < 1 {
		params.PageSize = 20
	}

	q := r.db.Model(&KnowledgeSource2{})

	if params.Format != "" {
		q = q.Where("format = ?", params.Format)
	}
	if params.Status != "" {
		q = q.Where("extract_status = ?", params.Status)
	}
	if params.Keyword != "" {
		like := "%" + params.Keyword + "%"
		q = q.Where("title LIKE ? OR file_name LIKE ?", like, like)
	}

	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var sources []KnowledgeSource2
	offset := (params.Page - 1) * params.PageSize
	if err := q.Order("id DESC").Offset(offset).Limit(params.PageSize).Find(&sources).Error; err != nil {
		return nil, 0, err
	}

	return sources, total, nil
}

func (r *KnowledgeSource2Repo) Update(s *KnowledgeSource2) error {
	return r.db.Save(s).Error
}

func (r *KnowledgeSource2Repo) Delete(id uint) error {
	return r.db.Delete(&KnowledgeSource2{}, id).Error
}

// DeleteByParentID deletes child sources (crawled sub-pages).
func (r *KnowledgeSource2Repo) DeleteByParentID(parentID uint) error {
	return r.db.Where("parent_id = ?", parentID).Delete(&KnowledgeSource2{}).Error
}

// FindChildIDs returns IDs of child sources (crawled sub-pages).
func (r *KnowledgeSource2Repo) FindChildIDs(parentID uint) ([]uint, error) {
	var ids []uint
	err := r.db.Model(&KnowledgeSource2{}).
		Where("parent_id = ?", parentID).Pluck("id", &ids).Error
	return ids, err
}

// FindCompletedByIDs returns sources with extract_status=completed among given IDs.
func (r *KnowledgeSource2Repo) FindCompletedByIDs(ids []uint) ([]KnowledgeSource2, error) {
	var sources []KnowledgeSource2
	if len(ids) == 0 {
		return sources, nil
	}
	if err := r.db.Where("id IN ? AND extract_status = ?", ids, ExtractStatusCompleted).
		Find(&sources).Error; err != nil {
		return nil, err
	}
	return sources, nil
}

// FindCrawlEnabledSources returns all top-level sources with crawl enabled.
func (r *KnowledgeSource2Repo) FindCrawlEnabledSources() ([]KnowledgeSource2, error) {
	var sources []KnowledgeSource2
	err := r.db.Where("crawl_enabled = ? AND parent_id IS NULL", true).
		Find(&sources).Error
	return sources, err
}
