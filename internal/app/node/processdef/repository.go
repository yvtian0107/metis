package processdef

import (
	"github.com/samber/do/v2"
	"metis/internal/app/node/domain"

	"metis/internal/database"
)

type ProcessDefRepo struct {
	db *database.DB
}

func NewProcessDefRepo(i do.Injector) (*ProcessDefRepo, error) {
	db := do.MustInvoke[*database.DB](i)
	return &ProcessDefRepo{db: db}, nil
}

func (r *ProcessDefRepo) Create(pd *domain.ProcessDef) error {
	return r.db.Create(pd).Error
}

func (r *ProcessDefRepo) FindByID(id uint) (*domain.ProcessDef, error) {
	var pd domain.ProcessDef
	if err := r.db.First(&pd, id).Error; err != nil {
		return nil, err
	}
	return &pd, nil
}

func (r *ProcessDefRepo) FindByName(name string) (*domain.ProcessDef, error) {
	var pd domain.ProcessDef
	if err := r.db.Where("name = ?", name).First(&pd).Error; err != nil {
		return nil, err
	}
	return &pd, nil
}

type ProcessDefListParams struct {
	Keyword  string
	Page     int
	PageSize int
}

func (r *ProcessDefRepo) List(params ProcessDefListParams) ([]domain.ProcessDef, int64, error) {
	if params.Page < 1 {
		params.Page = 1
	}
	if params.PageSize < 1 {
		params.PageSize = 20
	}

	base := r.db.Model(&domain.ProcessDef{})

	if params.Keyword != "" {
		like := "%" + params.Keyword + "%"
		base = base.Where("name LIKE ? OR display_name LIKE ?", like, like)
	}

	var total int64
	if err := base.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var items []domain.ProcessDef
	offset := (params.Page - 1) * params.PageSize
	if err := base.Offset(offset).Limit(params.PageSize).
		Order("created_at DESC").
		Find(&items).Error; err != nil {
		return nil, 0, err
	}

	return items, total, nil
}

func (r *ProcessDefRepo) Update(id uint, updates map[string]any) error {
	return r.db.Model(&domain.ProcessDef{}).Where("id = ?", id).Updates(updates).Error
}

func (r *ProcessDefRepo) Delete(id uint) error {
	return r.db.Delete(&domain.ProcessDef{}, id).Error
}
