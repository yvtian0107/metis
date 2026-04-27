package sla

import (
	"errors"
	. "metis/internal/app/itsm/domain"

	"github.com/samber/do/v2"
	"gorm.io/gorm"
)

var (
	ErrPriorityNotFound   = errors.New("priority not found")
	ErrPriorityCodeExists = errors.New("priority code already exists")
)

type PriorityService struct {
	repo *PriorityRepo
}

func NewPriorityService(i do.Injector) (*PriorityService, error) {
	repo := do.MustInvoke[*PriorityRepo](i)
	return &PriorityService{repo: repo}, nil
}

func (s *PriorityService) Create(p *Priority) (*Priority, error) {
	if _, err := s.repo.FindByCode(p.Code); err == nil {
		return nil, ErrPriorityCodeExists
	}
	p.IsActive = true
	if err := s.repo.Create(p); err != nil {
		return nil, err
	}
	return s.repo.FindByID(p.ID)
}

func (s *PriorityService) Get(id uint) (*Priority, error) {
	p, err := s.repo.FindByID(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrPriorityNotFound
		}
		return nil, err
	}
	return p, nil
}

func (s *PriorityService) Update(id uint, updates map[string]any) (*Priority, error) {
	existing, err := s.repo.FindByID(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrPriorityNotFound
		}
		return nil, err
	}
	if code, ok := updates["code"].(string); ok && code != existing.Code {
		if _, err := s.repo.FindByCode(code); err == nil {
			return nil, ErrPriorityCodeExists
		}
	}
	if err := s.repo.Update(id, updates); err != nil {
		return nil, err
	}
	return s.repo.FindByID(id)
}

func (s *PriorityService) Delete(id uint) error {
	if _, err := s.repo.FindByID(id); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrPriorityNotFound
		}
		return err
	}
	return s.repo.Delete(id)
}

func (s *PriorityService) ListAll() ([]Priority, error) {
	return s.repo.ListAll()
}
