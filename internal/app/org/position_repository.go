package org

import (
	"github.com/samber/do/v2"

	"metis/internal/database"
)

type PositionRepo struct {
	db *database.DB
}

func NewPositionRepo(i do.Injector) (*PositionRepo, error) {
	db := do.MustInvoke[*database.DB](i)
	return &PositionRepo{db: db}, nil
}

func (r *PositionRepo) Create(pos *Position) error {
	return r.db.Create(pos).Error
}

func (r *PositionRepo) FindByID(id uint) (*Position, error) {
	var pos Position
	if err := r.db.First(&pos, id).Error; err != nil {
		return nil, err
	}
	return &pos, nil
}

func (r *PositionRepo) FindByCode(code string) (*Position, error) {
	var pos Position
	if err := r.db.Where("code = ?", code).First(&pos).Error; err != nil {
		return nil, err
	}
	return &pos, nil
}

func (r *PositionRepo) Update(id uint, updates map[string]any) error {
	return r.db.Model(&Position{}).Where("id = ?", id).Updates(updates).Error
}

func (r *PositionRepo) Delete(id uint) error {
	return r.db.Delete(&Position{}, id).Error
}

type PositionListParams struct {
	Keyword  string
	Page     int
	PageSize int
}

func (r *PositionRepo) List(params PositionListParams) ([]Position, int64, error) {
	if params.Page < 1 {
		params.Page = 1
	}

	base := r.db.Model(&Position{})
	if params.Keyword != "" {
		like := "%" + params.Keyword + "%"
		base = base.Where("name LIKE ? OR code LIKE ?", like, like)
	}

	var total int64
	if err := base.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var items []Position
	query := base.Order("sort ASC, id ASC")
	if params.PageSize > 0 {
		offset := (params.Page - 1) * params.PageSize
		query = query.Offset(offset).Limit(params.PageSize)
	}
	// pageSize=0 means return all (no pagination)
	if err := query.Find(&items).Error; err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

func (r *PositionRepo) ListActive() ([]Position, error) {
	var items []Position
	if err := r.db.Where("is_active = ?", true).Order("sort ASC, id ASC").Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

func (r *PositionRepo) InUse(id uint) (bool, error) {
	var count int64
	if err := r.db.Model(&UserPosition{}).Where("position_id = ?", id).Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}
