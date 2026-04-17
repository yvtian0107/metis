package org

import (
	"errors"

	"github.com/samber/do/v2"
	"gorm.io/gorm"
)

var (
	ErrPositionNotFound   = errors.New("position not found")
	ErrPositionCodeExists = errors.New("position code already exists")
	ErrPositionInUse      = errors.New("position is in use")
)

type PositionService struct {
	repo *PositionRepo
}

func NewPositionService(i do.Injector) (*PositionService, error) {
	repo := do.MustInvoke[*PositionRepo](i)
	return &PositionService{repo: repo}, nil
}

func (s *PositionService) Create(name, code string, description string) (*Position, error) {
	if _, err := s.repo.FindByCode(code); err == nil {
		return nil, ErrPositionCodeExists
	}

	pos := &Position{
		Name:        name,
		Code:        code,
		Description: description,
		IsActive:    true,
	}
	if err := s.repo.Create(pos); err != nil {
		return nil, err
	}
	return s.repo.FindByID(pos.ID)
}

func (s *PositionService) Get(id uint) (*Position, error) {
	pos, err := s.repo.FindByID(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrPositionNotFound
		}
		return nil, err
	}
	return pos, nil
}

func (s *PositionService) List(params PositionListParams) ([]Position, int64, error) {
	return s.repo.List(params)
}

func (s *PositionService) ListActive() ([]Position, error) {
	return s.repo.ListActive()
}

func (s *PositionService) Update(id uint, name, code *string, description *string, isActive *bool) (*Position, error) {
	pos, err := s.repo.FindByID(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrPositionNotFound
		}
		return nil, err
	}

	updates := map[string]any{}
	if name != nil {
		updates["name"] = *name
	}
	if code != nil {
		if existing, err := s.repo.FindByCode(*code); err == nil && existing.ID != id {
			return nil, ErrPositionCodeExists
		}
		updates["code"] = *code
	}
	if description != nil {
		updates["description"] = *description
	}
	if isActive != nil {
		updates["is_active"] = *isActive
	}

	if len(updates) > 0 {
		if err := s.repo.Update(id, updates); err != nil {
			return nil, err
		}
		pos, _ = s.repo.FindByID(id)
	}
	return pos, nil
}

func (s *PositionService) Delete(id uint) error {
	if _, err := s.repo.FindByID(id); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrPositionNotFound
		}
		return err
	}

	inUse, err := s.repo.InUse(id)
	if err != nil {
		return err
	}
	if inUse {
		return ErrPositionInUse
	}

	return s.repo.Delete(id)
}
