package sla

import (
	"github.com/samber/do/v2"
	. "metis/internal/app/itsm/domain"

	"metis/internal/database"
)

type SLATemplateRepo struct {
	db *database.DB
}

func NewSLATemplateRepo(i do.Injector) (*SLATemplateRepo, error) {
	db := do.MustInvoke[*database.DB](i)
	return &SLATemplateRepo{db: db}, nil
}

func (r *SLATemplateRepo) Create(s *SLATemplate) error {
	return r.db.Create(s).Error
}

func (r *SLATemplateRepo) FindByID(id uint) (*SLATemplate, error) {
	var s SLATemplate
	if err := r.db.First(&s, id).Error; err != nil {
		return nil, err
	}
	return &s, nil
}

func (r *SLATemplateRepo) FindByCode(code string) (*SLATemplate, error) {
	var s SLATemplate
	if err := r.db.Where("code = ?", code).First(&s).Error; err != nil {
		return nil, err
	}
	return &s, nil
}

func (r *SLATemplateRepo) Update(id uint, updates map[string]any) error {
	return r.db.Model(&SLATemplate{}).Where("id = ?", id).Updates(updates).Error
}

func (r *SLATemplateRepo) Delete(id uint) error {
	return r.db.Delete(&SLATemplate{}, id).Error
}

func (r *SLATemplateRepo) ListAll() ([]SLATemplate, error) {
	var items []SLATemplate
	if err := r.db.Order("id ASC").Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}
