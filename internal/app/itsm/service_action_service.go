package itsm

import (
	"errors"

	"github.com/samber/do/v2"
	"gorm.io/gorm"
)

var (
	ErrServiceActionNotFound = errors.New("service action not found")
	ErrActionCodeExists      = errors.New("action code already exists in this service")
)

type ServiceActionService struct {
	repo        *ServiceActionRepo
	serviceDefs *ServiceDefService
}

func NewServiceActionService(i do.Injector) (*ServiceActionService, error) {
	repo := do.MustInvoke[*ServiceActionRepo](i)
	serviceDefs := do.MustInvoke[*ServiceDefService](i)
	return &ServiceActionService{repo: repo, serviceDefs: serviceDefs}, nil
}

func (s *ServiceActionService) Create(action *ServiceAction) (*ServiceAction, error) {
	if _, err := s.repo.FindByServiceAndCode(action.ServiceID, action.Code); err == nil {
		return nil, ErrActionCodeExists
	}
	action.IsActive = true
	if err := s.repo.Create(action); err != nil {
		return nil, err
	}
	if s.serviceDefs != nil {
		if err := s.serviceDefs.RefreshPublishHealthCheckIfPresent(action.ServiceID); err != nil {
			return nil, err
		}
	}
	return s.repo.FindByID(action.ID)
}

func (s *ServiceActionService) Get(id uint) (*ServiceAction, error) {
	a, err := s.repo.FindByID(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrServiceActionNotFound
		}
		return nil, err
	}
	return a, nil
}

func (s *ServiceActionService) Update(id uint, updates map[string]any) (*ServiceAction, error) {
	existing, err := s.repo.FindByID(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrServiceActionNotFound
		}
		return nil, err
	}
	if code, ok := updates["code"].(string); ok && code != existing.Code {
		if _, err := s.repo.FindByServiceAndCode(existing.ServiceID, code); err == nil {
			return nil, ErrActionCodeExists
		}
	}
	if err := s.repo.Update(id, updates); err != nil {
		return nil, err
	}
	if s.serviceDefs != nil {
		if err := s.serviceDefs.RefreshPublishHealthCheckIfPresent(existing.ServiceID); err != nil {
			return nil, err
		}
	}
	return s.repo.FindByID(id)
}

func (s *ServiceActionService) Delete(id uint) error {
	existing, err := s.repo.FindByID(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrServiceActionNotFound
		}
		return err
	}
	if err := s.repo.Delete(id); err != nil {
		return err
	}
	if s.serviceDefs != nil {
		return s.serviceDefs.RefreshPublishHealthCheckIfPresent(existing.ServiceID)
	}
	return nil
}

func (s *ServiceActionService) ListByService(serviceID uint) ([]ServiceAction, error) {
	return s.repo.ListByService(serviceID)
}
