package catalog

import (
	"errors"
	. "metis/internal/app/itsm/domain"

	"github.com/samber/do/v2"
	"gorm.io/gorm"
)

var (
	ErrCatalogNotFound      = errors.New("service catalog not found")
	ErrCatalogHasChildren   = errors.New("catalog has sub-categories, cannot delete")
	ErrCatalogHasServices   = errors.New("catalog has services, cannot delete")
	ErrCatalogTooDeep       = errors.New("service catalog supports at most two levels")
	ErrCatalogCodeExists    = errors.New("catalog code already exists")
	ErrCatalogInvalidParent = errors.New("invalid catalog parent")
)

type CatalogService struct {
	repo *CatalogRepo
}

type CatalogServiceCounts struct {
	Total           int64          `json:"total"`
	ByCatalogID     map[uint]int64 `json:"byCatalogId"`
	ByRootCatalogID map[uint]int64 `json:"byRootCatalogId"`
}

func NewCatalogService(i do.Injector) (*CatalogService, error) {
	repo := do.MustInvoke[*CatalogRepo](i)
	return &CatalogService{repo: repo}, nil
}

func (s *CatalogService) Create(name, code, description, icon string, parentID *uint, sortOrder int) (*ServiceCatalog, error) {
	if _, err := s.repo.FindByCode(code); err == nil {
		return nil, ErrCatalogCodeExists
	}
	if parentID != nil {
		if err := s.validateParentChange(0, *parentID); err != nil {
			return nil, err
		}
	}

	catalog := &ServiceCatalog{
		Name:        name,
		Code:        code,
		Description: description,
		Icon:        icon,
		ParentID:    parentID,
		SortOrder:   sortOrder,
		IsActive:    true,
	}
	if err := s.repo.Create(catalog); err != nil {
		return nil, err
	}
	return s.repo.FindByID(catalog.ID)
}

func (s *CatalogService) Get(id uint) (*ServiceCatalog, error) {
	c, err := s.repo.FindByID(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrCatalogNotFound
		}
		return nil, err
	}
	return c, nil
}

func (s *CatalogService) Update(id uint, updates map[string]any) (*ServiceCatalog, error) {
	existing, err := s.repo.FindByID(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrCatalogNotFound
		}
		return nil, err
	}
	if code, ok := updates["code"].(string); ok && code != existing.Code {
		if _, err := s.repo.FindByCode(code); err == nil {
			return nil, ErrCatalogCodeExists
		}
	}
	if parentID, ok := updates["parent_id"].(uint); ok {
		if err := s.validateParentChange(id, parentID); err != nil {
			return nil, err
		}
	}
	if err := s.repo.Update(id, updates); err != nil {
		if IsSQLiteUniqueError(err) {
			return nil, ErrCatalogCodeExists
		}
		return nil, err
	}
	return s.repo.FindByID(id)
}

func (s *CatalogService) Delete(id uint) error {
	if _, err := s.repo.FindByID(id); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrCatalogNotFound
		}
		return err
	}

	has, err := s.repo.HasChildren(id)
	if err != nil {
		return err
	}
	if has {
		return ErrCatalogHasChildren
	}

	hasSvc, err := s.repo.HasServices(id)
	if err != nil {
		return err
	}
	if hasSvc {
		return ErrCatalogHasServices
	}

	return s.repo.Delete(id)
}

// Tree returns the full catalog tree structure.
func (s *CatalogService) Tree() ([]ServiceCatalogResponse, error) {
	all, err := s.repo.FindAll()
	if err != nil {
		return nil, err
	}
	return buildTree(all, nil), nil
}

func (s *CatalogService) ServiceCounts() (*CatalogServiceCounts, error) {
	catalogs, err := s.repo.FindAll()
	if err != nil {
		return nil, err
	}
	directCounts, total, err := s.repo.ServiceCountsByCatalog()
	if err != nil {
		return nil, err
	}

	counts := &CatalogServiceCounts{
		Total:           total,
		ByCatalogID:     make(map[uint]int64, len(catalogs)),
		ByRootCatalogID: make(map[uint]int64, len(catalogs)),
	}
	catalogByID := make(map[uint]ServiceCatalog, len(catalogs))
	for _, catalog := range catalogs {
		catalogByID[catalog.ID] = catalog
		counts.ByCatalogID[catalog.ID] = directCounts[catalog.ID]
		if catalog.ParentID == nil {
			counts.ByRootCatalogID[catalog.ID] = 0
		}
	}
	for catalogID, directCount := range directCounts {
		catalog, ok := catalogByID[catalogID]
		if !ok {
			continue
		}
		rootID := catalog.ID
		if catalog.ParentID != nil {
			rootID = *catalog.ParentID
		}
		counts.ByRootCatalogID[rootID] += directCount
	}
	return counts, nil
}

func buildTree(catalogs []ServiceCatalog, parentID *uint) []ServiceCatalogResponse {
	var result []ServiceCatalogResponse
	for _, c := range catalogs {
		if ptrEq(c.ParentID, parentID) {
			resp := c.ToResponse()
			resp.Children = buildTree(catalogs, &c.ID)
			result = append(result, resp)
		}
	}
	return result
}

func ptrEq(a, b *uint) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return *a == *b
}

func (s *CatalogService) validateParentChange(id, parentID uint) error {
	if id != 0 && id == parentID {
		return ErrCatalogInvalidParent
	}
	parent, err := s.repo.FindByID(parentID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrCatalogNotFound
		}
		return err
	}
	if id == 0 {
		if parent.ParentID != nil {
			return ErrCatalogTooDeep
		}
		return nil
	}
	all, err := s.repo.FindAll()
	if err != nil {
		return err
	}
	if isDescendantCatalog(all, parentID, id) {
		return ErrCatalogInvalidParent
	}
	if parent.ParentID != nil {
		return ErrCatalogTooDeep
	}
	return nil
}

func isDescendantCatalog(catalogs []ServiceCatalog, candidateID, ancestorID uint) bool {
	current := candidateID
	for {
		found := false
		for _, c := range catalogs {
			if c.ID != current {
				continue
			}
			found = true
			if c.ParentID == nil {
				return false
			}
			if *c.ParentID == ancestorID {
				return true
			}
			current = *c.ParentID
			break
		}
		if !found {
			return false
		}
	}
}
