package position

import (
	"errors"
	"metis/internal/app/org/domain"

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

func (s *PositionService) Create(name, code string, description string) (*domain.Position, error) {
	if _, err := s.repo.FindByCode(code); err == nil {
		return nil, ErrPositionCodeExists
	}

	pos := &domain.Position{
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

func (s *PositionService) Get(id uint) (*domain.Position, error) {
	pos, err := s.repo.FindByID(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrPositionNotFound
		}
		return nil, err
	}
	return pos, nil
}

func (s *PositionService) List(params PositionListParams) ([]domain.Position, int64, error) {
	return s.repo.List(params)
}

func (s *PositionService) ListWithUsage(params PositionListParams) ([]domain.PositionResponse, int64, error) {
	items, total, err := s.repo.List(params)
	if err != nil {
		return nil, 0, err
	}
	ids := make([]uint, 0, len(items))
	for _, item := range items {
		ids = append(ids, item.ID)
	}
	usage, err := s.repo.UsageByPositionIDs(ids)
	if err != nil {
		return nil, 0, err
	}
	result := make([]domain.PositionResponse, len(items))
	for i, item := range items {
		resp := item.ToResponse()
		if u, ok := usage[item.ID]; ok {
			resp.DepartmentCount = u.DepartmentCount
			resp.MemberCount = u.MemberCount
			resp.Departments = u.Departments
		}
		result[i] = resp
	}
	return result, total, nil
}

func (s *PositionService) ListActive() ([]domain.Position, error) {
	return s.repo.ListActive()
}

func (s *PositionService) Update(id uint, name, code *string, description *string, isActive *bool) (*domain.Position, error) {
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
