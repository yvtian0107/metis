package licensee

import (
	"errors"
	"fmt"
	"metis/internal/app/license/domain"

	"github.com/samber/do/v2"
	"gorm.io/gorm"
)

var (
	ErrLicenseeNotFound      = errors.New("error.license.licensee_not_found")
	ErrLicenseeNameExists    = errors.New("error.license.licensee_name_exists")
	ErrLicenseeCodeCollision = errors.New("error.license.licensee_code_collision")
	ErrLicenseeInvalidStatus = errors.New("error.license.licensee_invalid_status")
)

type LicenseeService struct {
	repo *LicenseeRepo
}

func NewLicenseeService(i do.Injector) (*LicenseeService, error) {
	return &LicenseeService{
		repo: do.MustInvoke[*LicenseeRepo](i),
	}, nil
}

type CreateLicenseeParams struct {
	Name  string
	Notes string
}

func (s *LicenseeService) CreateLicensee(params CreateLicenseeParams) (*domain.Licensee, error) {
	exists, err := s.repo.ExistsByName(params.Name)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, ErrLicenseeNameExists
	}

	// Generate unique code with retry
	var code string
	for i := 0; i < 3; i++ {
		c, err := domain.GenerateLicenseeCode()
		if err != nil {
			return nil, fmt.Errorf("generate licensee code: %w", err)
		}
		dup, err := s.repo.ExistsByCode(c)
		if err != nil {
			return nil, err
		}
		if !dup {
			code = c
			break
		}
	}
	if code == "" {
		return nil, ErrLicenseeCodeCollision
	}

	licensee := &domain.Licensee{
		Name:   params.Name,
		Code:   code,
		Notes:  params.Notes,
		Status: domain.LicenseeStatusActive,
	}
	if err := s.repo.Create(licensee); err != nil {
		return nil, err
	}
	return licensee, nil
}

func (s *LicenseeService) GetLicensee(id uint) (*domain.Licensee, error) {
	l, err := s.repo.FindByID(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrLicenseeNotFound
		}
		return nil, err
	}
	return l, nil
}

func (s *LicenseeService) ListLicensees(params LicenseeListParams) ([]domain.Licensee, int64, error) {
	return s.repo.List(params)
}

type UpdateLicenseeParams struct {
	Name  *string
	Notes *string
}

func (s *LicenseeService) UpdateLicensee(id uint, params UpdateLicenseeParams) (*domain.Licensee, error) {
	l, err := s.repo.FindByID(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrLicenseeNotFound
		}
		return nil, err
	}

	if params.Name != nil && *params.Name != l.Name {
		exists, err := s.repo.ExistsByName(*params.Name, id)
		if err != nil {
			return nil, err
		}
		if exists {
			return nil, ErrLicenseeNameExists
		}
		l.Name = *params.Name
	}
	if params.Notes != nil {
		l.Notes = *params.Notes
	}

	if err := s.repo.Update(l); err != nil {
		return nil, err
	}
	return l, nil
}

func (s *LicenseeService) UpdateLicenseeStatus(id uint, newStatus string) error {
	l, err := s.repo.FindByID(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrLicenseeNotFound
		}
		return err
	}

	if l.Status == newStatus {
		return fmt.Errorf("%w: status is already %s", ErrLicenseeInvalidStatus, newStatus)
	}

	// Validate transition: active <-> archived only
	valid := (l.Status == domain.LicenseeStatusActive && newStatus == domain.LicenseeStatusArchived) ||
		(l.Status == domain.LicenseeStatusArchived && newStatus == domain.LicenseeStatusActive)
	if !valid {
		return fmt.Errorf("%w: cannot transition from %s to %s", ErrLicenseeInvalidStatus, l.Status, newStatus)
	}

	return s.repo.UpdateStatus(id, newStatus)
}
