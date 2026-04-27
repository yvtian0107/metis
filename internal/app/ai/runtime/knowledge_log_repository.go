package runtime

import (
	"github.com/samber/do/v2"

	"metis/internal/database"
)

// KnowledgeLogRepo provides GORM persistence for KnowledgeLog.
type KnowledgeLogRepo struct {
	db *database.DB
}

func NewKnowledgeLogRepo(i do.Injector) (*KnowledgeLogRepo, error) {
	db := do.MustInvoke[*database.DB](i)
	return &KnowledgeLogRepo{db: db}, nil
}

func (r *KnowledgeLogRepo) Create(log *KnowledgeLog) error {
	return r.db.Create(log).Error
}

func (r *KnowledgeLogRepo) FindByAssetID(assetID uint, limit int) ([]KnowledgeLog, error) {
	if limit <= 0 {
		limit = 50
	}
	var logs []KnowledgeLog
	err := r.db.Where("asset_id = ?", assetID).Order("id DESC").Limit(limit).Find(&logs).Error
	return logs, err
}

func (r *KnowledgeLogRepo) DeleteByAssetID(assetID uint) error {
	return r.db.Where("asset_id = ?", assetID).Delete(&KnowledgeLog{}).Error
}
