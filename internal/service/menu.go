package service

import (
	"errors"

	"github.com/samber/do/v2"
	"gorm.io/gorm"

	"metis/internal/model"
	"metis/internal/repository"
)

var (
	ErrMenuNotFound    = errors.New("error.menu.not_found")
	ErrMenuHasChildren = errors.New("error.menu.has_children")
)

type MenuService struct {
	menuRepo  *repository.MenuRepo
	casbinSvc *CasbinService
}

func NewMenu(i do.Injector) (*MenuService, error) {
	menuRepo := do.MustInvoke[*repository.MenuRepo](i)
	casbinSvc := do.MustInvoke[*CasbinService](i)
	return &MenuService{
		menuRepo:  menuRepo,
		casbinSvc: casbinSvc,
	}, nil
}

func (s *MenuService) GetTree() ([]model.Menu, error) {
	return s.menuRepo.GetTree()
}

// GetUserTree returns the menu tree filtered by the user's role permissions.
func (s *MenuService) GetUserTree(roleCode string) ([]model.Menu, error) {
	all, err := s.menuRepo.FindAll()
	if err != nil {
		return nil, err
	}

	// Get all permissions for this role
	policies := s.casbinSvc.GetPoliciesForRole(roleCode)
	permSet := make(map[string]bool)
	for _, p := range policies {
		permSet[p[1]] = true // p[1] is the obj (permission identifier)
	}

	// For admin, include all menus
	if roleCode == model.RoleAdmin {
		return buildUserTree(all, nil, nil), nil
	}

	// Filter menus: include if user has the permission or any descendant has it
	return buildUserTree(all, nil, permSet), nil
}

// buildUserTree builds a tree, filtering by permissions. If permSet is nil, include all.
func buildUserTree(all []model.Menu, parentID *uint, permSet map[string]bool) []model.Menu {
	var children []model.Menu
	for _, m := range all {
		if !ptrEqualMenu(m.ParentID, parentID) {
			continue
		}

		subtree := buildUserTree(all, &m.ID, permSet)

		if permSet == nil {
			// No filtering, include all
			m.Children = subtree
			children = append(children, m)
			continue
		}

		// Include if this menu's permission is allowed, or if any subtree child exists
		hasPermission := m.Permission != "" && permSet[m.Permission]
		if hasPermission || len(subtree) > 0 {
			m.Children = subtree
			children = append(children, m)
		}
	}
	return children
}

func ptrEqualMenu(a *uint, b *uint) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return *a == *b
}

// GetUserPermissions returns all permission strings for a user's role.
func (s *MenuService) GetUserPermissions(roleCode string) []string {
	policies := s.casbinSvc.GetPoliciesForRole(roleCode)
	var perms []string
	seen := make(map[string]bool)
	for _, p := range policies {
		obj := p[1]
		// Only include menu permissions (not API paths)
		if len(obj) > 0 && obj[0] != '/' && !seen[obj] {
			perms = append(perms, obj)
			seen[obj] = true
		}
	}
	return perms
}

func (s *MenuService) Create(menu *model.Menu) error {
	return s.menuRepo.Create(menu)
}

func (s *MenuService) Update(id uint, updates map[string]any) (*model.Menu, error) {
	menu, err := s.menuRepo.FindByID(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrMenuNotFound
		}
		return nil, err
	}

	if v, ok := updates["name"]; ok {
		menu.Name = v.(string)
	}
	if v, ok := updates["type"]; ok {
		menu.Type = model.MenuType(v.(string))
	}
	if v, ok := updates["path"]; ok {
		menu.Path = v.(string)
	}
	if v, ok := updates["icon"]; ok {
		menu.Icon = v.(string)
	}
	if v, ok := updates["permission"]; ok {
		menu.Permission = v.(string)
	}
	if v, ok := updates["sort"]; ok {
		menu.Sort = int(v.(float64))
	}
	if v, ok := updates["isHidden"]; ok {
		menu.IsHidden = v.(bool)
	}
	if v, ok := updates["parentId"]; ok {
		if v == nil {
			menu.ParentID = nil
		} else {
			pid := uint(v.(float64))
			menu.ParentID = &pid
		}
	}

	if err := s.menuRepo.Update(menu); err != nil {
		return nil, err
	}
	return menu, nil
}

func (s *MenuService) ReorderMenus(items []repository.SortItem) error {
	return s.menuRepo.UpdateSorts(items)
}

func (s *MenuService) Delete(id uint) error {
	hasChildren, err := s.menuRepo.HasChildren(id)
	if err != nil {
		return err
	}
	if hasChildren {
		return ErrMenuHasChildren
	}

	if err := s.menuRepo.Delete(id); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrMenuNotFound
		}
		return err
	}
	return nil
}
