package sla

import (
	"errors"
	. "metis/internal/app/itsm/domain"

	"github.com/samber/do/v2"
	"gorm.io/gorm"

	"metis/internal/database"
)

var (
	ErrSLATemplateNotFound = errors.New("SLA template not found")
	ErrSLACodeExists       = errors.New("SLA code already exists")
	ErrSLATemplateInUse    = errors.New("SLA template is referenced by active services")
)

type SLATemplateService struct {
	repo *SLATemplateRepo
	db   *database.DB
}

func NewSLATemplateService(i do.Injector) (*SLATemplateService, error) {
	repo := do.MustInvoke[*SLATemplateRepo](i)
	db := do.MustInvoke[*database.DB](i)
	return &SLATemplateService{repo: repo, db: db}, nil
}

func (s *SLATemplateService) Create(sla *SLATemplate) (*SLATemplate, error) {
	if _, err := s.repo.FindByCode(sla.Code); err == nil {
		return nil, ErrSLACodeExists
	}
	sla.IsActive = true
	if err := s.repo.Create(sla); err != nil {
		return nil, err
	}
	return s.repo.FindByID(sla.ID)
}

func (s *SLATemplateService) Get(id uint) (*SLATemplate, error) {
	sla, err := s.repo.FindByID(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrSLATemplateNotFound
		}
		return nil, err
	}
	return sla, nil
}

func (s *SLATemplateService) Update(id uint, updates map[string]any) (*SLATemplate, error) {
	existing, err := s.repo.FindByID(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrSLATemplateNotFound
		}
		return nil, err
	}
	if code, ok := updates["code"].(string); ok && code != existing.Code {
		if _, err := s.repo.FindByCode(code); err == nil {
			return nil, ErrSLACodeExists
		}
	}
	if isActive, ok := updates["is_active"].(bool); ok && !isActive && existing.IsActive {
		if err := s.ensureNotReferencedByActiveService(id); err != nil {
			return nil, err
		}
	}
	if err := s.repo.Update(id, updates); err != nil {
		return nil, err
	}
	return s.repo.FindByID(id)
}

func (s *SLATemplateService) Delete(id uint) error {
	if _, err := s.repo.FindByID(id); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrSLATemplateNotFound
		}
		return err
	}
	if err := s.ensureNotReferencedByActiveService(id); err != nil {
		return err
	}
	return s.repo.Delete(id)
}

func (s *SLATemplateService) ListAll() ([]SLATemplate, error) {
	return s.repo.ListAll()
}

func (s *SLATemplateService) ensureNotReferencedByActiveService(id uint) error {
	var count int64
	if err := s.db.Model(&ServiceDefinition{}).
		Where("sla_id = ? AND is_active = ?", id, true).
		Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return ErrSLATemplateInUse
	}
	return nil
}
