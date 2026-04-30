package sla

import (
	"github.com/samber/do/v2"
	. "metis/internal/app/itsm/domain"

	"metis/internal/database"
)

type PriorityRepo struct {
	db *database.DB
}

func NewPriorityRepo(i do.Injector) (*PriorityRepo, error) {
	db := do.MustInvoke[*database.DB](i)
	return &PriorityRepo{db: db}, nil
}

func (r *PriorityRepo) Create(p *Priority) error {
	return r.db.Create(p).Error
}

func (r *PriorityRepo) FindByID(id uint) (*Priority, error) {
	var p Priority
	if err := r.db.First(&p, id).Error; err != nil {
		return nil, err
	}
	return &p, nil
}

func (r *PriorityRepo) FindByCode(code string) (*Priority, error) {
	var p Priority
	if err := r.db.Where("code = ?", code).First(&p).Error; err != nil {
		return nil, err
	}
	return &p, nil
}

func (r *PriorityRepo) FindActiveByCode(code string) (*Priority, error) {
	var p Priority
	if err := r.db.Where("code = ? AND is_active = ?", code, true).First(&p).Error; err != nil {
		return nil, err
	}
	return &p, nil
}

func (r *PriorityRepo) Update(id uint, updates map[string]any) error {
	return r.db.Model(&Priority{}).Where("id = ?", id).Updates(updates).Error
}

func (r *PriorityRepo) Delete(id uint) error {
	return r.db.Delete(&Priority{}, id).Error
}

func (r *PriorityRepo) FindDefaultActive() (*Priority, error) {
	var p Priority
	if err := r.db.Where("is_active = ?", true).Order("value ASC").First(&p).Error; err != nil {
		return nil, err
	}
	return &p, nil
}

func (r *PriorityRepo) ListAll() ([]Priority, error) {
	var items []Priority
	if err := r.db.Order("value ASC, id ASC").Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}
