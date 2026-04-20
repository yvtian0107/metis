package ai

import (
	"github.com/samber/do/v2"
	"gorm.io/gorm"

	"metis/internal/database"
)

type ModelRepo struct {
	db *database.DB
}

func NewModelRepo(i do.Injector) (*ModelRepo, error) {
	return &ModelRepo{db: do.MustInvoke[*database.DB](i)}, nil
}

func (r *ModelRepo) Create(m *AIModel) error {
	return r.db.Create(m).Error
}

func (r *ModelRepo) FindByID(id uint) (*AIModel, error) {
	var m AIModel
	if err := r.db.Preload("Provider").First(&m, id).Error; err != nil {
		return nil, err
	}
	return &m, nil
}

type ModelListParams struct {
	Keyword    string
	Type       string
	ProviderID uint
	Page       int
	PageSize   int
}

func (r *ModelRepo) List(params ModelListParams) ([]AIModel, int64, error) {
	if params.Page < 1 {
		params.Page = 1
	}
	if params.PageSize < 1 {
		params.PageSize = 20
	}

	query := r.db.Model(&AIModel{})
	if params.Keyword != "" {
		like := "%" + params.Keyword + "%"
		query = query.Where("display_name LIKE ? OR model_id LIKE ?", like, like)
	}
	if params.Type != "" {
		query = query.Where("type = ?", params.Type)
	}
	if params.ProviderID > 0 {
		query = query.Where("provider_id = ?", params.ProviderID)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var models []AIModel
	offset := (params.Page - 1) * params.PageSize
	if err := query.Preload("Provider").
		Offset(offset).Limit(params.PageSize).
		Order("type ASC, created_at DESC").
		Find(&models).Error; err != nil {
		return nil, 0, err
	}
	return models, total, nil
}

func (r *ModelRepo) Update(m *AIModel) error {
	return r.db.Save(m).Error
}

func (r *ModelRepo) Delete(id uint) error {
	return r.db.Delete(&AIModel{}, id).Error
}

func (r *ModelRepo) ClearDefaultByType(modelType string) error {
	return r.db.Model(&AIModel{}).
		Where("type = ? AND is_default = ?", modelType, true).
		Update("is_default", false).Error
}

func (r *ModelRepo) FindByModelIDAndProvider(modelID string, providerID uint) (*AIModel, error) {
	var m AIModel
	if err := r.db.Where("model_id = ? AND provider_id = ?", modelID, providerID).First(&m).Error; err != nil {
		return nil, err
	}
	return &m, nil
}

func (r *ModelRepo) FindDefaultByType(modelType string) (*AIModel, error) {
	var m AIModel
	if err := r.db.Preload("Provider").
		Where("type = ? AND is_default = ?", modelType, true).
		First(&m).Error; err != nil {
		return nil, err
	}
	return &m, nil
}

func (r *ModelRepo) ListByProviderID(providerID uint) ([]AIModel, error) {
	var models []AIModel
	if err := r.db.Where("provider_id = ?", providerID).Find(&models).Error; err != nil {
		return nil, err
	}
	return models, nil
}

func (r *ModelRepo) TypeCountsForProviders(providerIDs []uint) (map[uint]map[string]int, error) {
	if len(providerIDs) == 0 {
		return map[uint]map[string]int{}, nil
	}

	type row struct {
		ProviderID uint
		Type       string
		Count      int
	}

	var rows []row
	if err := r.db.Model(&AIModel{}).
		Select("provider_id, type, COUNT(*) as count").
		Where("provider_id IN ?", providerIDs).
		Group("provider_id, type").
		Scan(&rows).Error; err != nil {
		return nil, err
	}

	counts := make(map[uint]map[string]int, len(providerIDs))
	for _, id := range providerIDs {
		counts[id] = map[string]int{}
	}
	for _, row := range rows {
		if counts[row.ProviderID] == nil {
			counts[row.ProviderID] = map[string]int{}
		}
		counts[row.ProviderID][row.Type] = row.Count
	}

	return counts, nil
}

func (r *ModelRepo) DB() *gorm.DB {
	return r.db.DB
}
