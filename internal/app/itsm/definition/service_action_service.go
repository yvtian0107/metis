package definition

import (
	"errors"
	. "metis/internal/app/itsm/domain"

	"github.com/samber/do/v2"
	"gorm.io/gorm"
)

var (
	ErrServiceActionNotFound = errors.New("service action not found")
	ErrActionCodeExists      = errors.New("action code already exists in this service")
	ErrInvalidActionConfig   = errors.New("invalid action config")
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
	if err := s.ensureServiceExists(action.ServiceID); err != nil {
		return nil, err
	}
	if _, err := s.repo.FindByServiceAndCode(action.ServiceID, action.Code); err == nil {
		return nil, ErrActionCodeExists
	}
	config, err := NormalizeServiceActionConfig(action.ActionType, action.ConfigJSON)
	if err != nil {
		return nil, errors.Join(ErrInvalidActionConfig, err)
	}
	action.ConfigJSON = config
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

func (s *ServiceActionService) GetByService(serviceID, id uint) (*ServiceAction, error) {
	if err := s.ensureServiceExists(serviceID); err != nil {
		return nil, err
	}
	a, err := s.repo.FindByServiceAndID(serviceID, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrServiceActionNotFound
		}
		return nil, err
	}
	return a, nil
}

func (s *ServiceActionService) Update(serviceID, id uint, updates map[string]any) (*ServiceAction, error) {
	if err := s.ensureServiceExists(serviceID); err != nil {
		return nil, err
	}
	existing, err := s.repo.FindByServiceAndID(serviceID, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrServiceActionNotFound
		}
		return nil, err
	}
	if code, ok := updates["code"].(string); ok && code != existing.Code {
		if _, err := s.repo.FindByServiceAndCode(serviceID, code); err == nil {
			return nil, ErrActionCodeExists
		}
	}
	actionType := existing.ActionType
	if v, ok := updates["action_type"].(string); ok {
		actionType = v
	}
	configJSON := existing.ConfigJSON
	if v, ok := updates["config_json"].(JSONField); ok {
		configJSON = v
	}
	if _, actionTypeChanged := updates["action_type"]; actionTypeChanged {
		config, err := NormalizeServiceActionConfig(actionType, configJSON)
		if err != nil {
			return nil, errors.Join(ErrInvalidActionConfig, err)
		}
		updates["config_json"] = config
	} else if _, configChanged := updates["config_json"]; configChanged {
		config, err := NormalizeServiceActionConfig(actionType, configJSON)
		if err != nil {
			return nil, errors.Join(ErrInvalidActionConfig, err)
		}
		updates["config_json"] = config
	}
	if err := s.repo.UpdateByService(serviceID, id, updates); err != nil {
		return nil, err
	}
	if s.serviceDefs != nil {
		if err := s.serviceDefs.RefreshPublishHealthCheckIfPresent(existing.ServiceID); err != nil {
			return nil, err
		}
	}
	return s.repo.FindByID(id)
}

func (s *ServiceActionService) Delete(serviceID, id uint) error {
	if err := s.ensureServiceExists(serviceID); err != nil {
		return err
	}
	existing, err := s.repo.FindByServiceAndID(serviceID, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrServiceActionNotFound
		}
		return err
	}
	if err := s.repo.DeleteByService(serviceID, id); err != nil {
		return err
	}
	if s.serviceDefs != nil {
		return s.serviceDefs.RefreshPublishHealthCheckIfPresent(existing.ServiceID)
	}
	return nil
}

func (s *ServiceActionService) ListByService(serviceID uint) ([]ServiceAction, error) {
	if err := s.ensureServiceExists(serviceID); err != nil {
		return nil, err
	}
	return s.repo.ListByService(serviceID)
}

func (s *ServiceActionService) ensureServiceExists(serviceID uint) error {
	if s.serviceDefs == nil {
		return nil
	}
	if _, err := s.serviceDefs.Get(serviceID); err != nil {
		return err
	}
	return nil
}
