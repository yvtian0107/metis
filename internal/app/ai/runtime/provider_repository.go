package runtime

import (
	"strings"

	"github.com/samber/do/v2"
	"gorm.io/gorm"

	"metis/internal/database"
)

// escapeLike escapes SQL LIKE wildcards so user input is treated literally.
func escapeLike(s string) string {
	s = strings.ReplaceAll(s, "\\", "\\\\")
	s = strings.ReplaceAll(s, "%", "\\%")
	s = strings.ReplaceAll(s, "_", "\\_")
	return s
}

type ProviderRepo struct {
	db *database.DB
}

func NewProviderRepo(i do.Injector) (*ProviderRepo, error) {
	return &ProviderRepo{db: do.MustInvoke[*database.DB](i)}, nil
}

func (r *ProviderRepo) Create(p *Provider) error {
	return r.db.Create(p).Error
}

func (r *ProviderRepo) FindByID(id uint) (*Provider, error) {
	var p Provider
	if err := r.db.First(&p, id).Error; err != nil {
		return nil, err
	}
	return &p, nil
}

type ProviderListParams struct {
	Keyword  string
	Page     int
	PageSize int
}

func (r *ProviderRepo) List(params ProviderListParams) ([]Provider, int64, error) {
	if params.Page < 1 {
		params.Page = 1
	}
	if params.PageSize < 1 {
		params.PageSize = 20
	}

	query := r.db.Model(&Provider{})
	if params.Keyword != "" {
		like := "%" + escapeLike(params.Keyword) + "%"
		query = query.Where("name LIKE ?", like)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var providers []Provider
	offset := (params.Page - 1) * params.PageSize
	if err := query.Offset(offset).Limit(params.PageSize).Order("created_at DESC").Find(&providers).Error; err != nil {
		return nil, 0, err
	}
	return providers, total, nil
}

func (r *ProviderRepo) Update(p *Provider) error {
	return r.db.Save(p).Error
}

func (r *ProviderRepo) Delete(id uint) error {
	return r.db.Delete(&Provider{}, id).Error
}

func (r *ProviderRepo) DeleteWithModels(id uint) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("provider_id = ?", id).Delete(&AIModel{}).Error; err != nil {
			return err
		}
		return tx.Delete(&Provider{}, id).Error
	})
}

func (r *ProviderRepo) CountModelsByProviderID(providerID uint) (int64, error) {
	var count int64
	if err := r.db.Model(&AIModel{}).Where("provider_id = ?", providerID).Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}

func (r *ProviderRepo) ModelCountsForProviders(providerIDs []uint) (map[uint]int, error) {
	if len(providerIDs) == 0 {
		return map[uint]int{}, nil
	}
	type countResult struct {
		ProviderID uint `gorm:"column:provider_id"`
		Count      int  `gorm:"column:cnt"`
	}
	var counts []countResult
	if err := r.db.Model(&AIModel{}).
		Select("provider_id, COUNT(*) as cnt").
		Where("provider_id IN ?", providerIDs).
		Group("provider_id").
		Find(&counts).Error; err != nil {
		return nil, err
	}
	m := make(map[uint]int, len(counts))
	for _, c := range counts {
		m[c.ProviderID] = c.Count
	}
	return m, nil
}

func (r *ProviderRepo) UpdateStatus(id uint, status string) error {
	updates := map[string]any{"status": status}
	if status == ProviderStatusActive {
		updates["health_checked_at"] = gorm.Expr("CURRENT_TIMESTAMP")
	}
	return r.db.Model(&Provider{}).Where("id = ?", id).Updates(updates).Error
}
