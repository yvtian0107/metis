package service

import (
	"errors"

	"github.com/samber/do/v2"
	"gorm.io/gorm"

	"metis/internal/model"
	"metis/internal/repository"
)

var (
	ErrRoleNotFound    = errors.New("error.role.not_found")
	ErrRoleCodeExists  = errors.New("error.role.code_exists")
	ErrSystemRole      = errors.New("error.role.system_role")
	ErrSystemRoleDel   = errors.New("error.role.system_role_delete")
	ErrRoleHasUsers    = errors.New("error.role.has_users")
)

type RoleService struct {
	roleRepo    *repository.RoleRepo
	casbinSvc   *CasbinService
}

func NewRole(i do.Injector) (*RoleService, error) {
	roleRepo := do.MustInvoke[*repository.RoleRepo](i)
	casbinSvc := do.MustInvoke[*CasbinService](i)
	return &RoleService{
		roleRepo:  roleRepo,
		casbinSvc: casbinSvc,
	}, nil
}

func (s *RoleService) List(page, pageSize int) ([]model.Role, int64, error) {
	return s.roleRepo.List(page, pageSize)
}

func (s *RoleService) GetByID(id uint) (*model.Role, error) {
	role, err := s.roleRepo.FindByID(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrRoleNotFound
		}
		return nil, err
	}
	return role, nil
}

func (s *RoleService) Create(name, code, description string, sort int) (*model.Role, error) {
	exists, err := s.roleRepo.ExistsByCode(code)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, ErrRoleCodeExists
	}

	role := &model.Role{
		Name:        name,
		Code:        code,
		Description: description,
		Sort:        sort,
	}
	if err := s.roleRepo.Create(role); err != nil {
		return nil, err
	}
	return role, nil
}

type UpdateRoleParams struct {
	Name        *string
	Code        *string
	Description *string
	Sort        *int
}

func (s *RoleService) Update(id uint, params UpdateRoleParams) (*model.Role, error) {
	role, err := s.roleRepo.FindByID(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrRoleNotFound
		}
		return nil, err
	}

	// Cannot change system role code
	if params.Code != nil && role.IsSystem && *params.Code != role.Code {
		return nil, ErrSystemRole
	}

	oldCode := role.Code

	if params.Name != nil {
		role.Name = *params.Name
	}
	if params.Code != nil && *params.Code != role.Code {
		exists, err := s.roleRepo.ExistsByCode(*params.Code)
		if err != nil {
			return nil, err
		}
		if exists {
			return nil, ErrRoleCodeExists
		}
		role.Code = *params.Code
	}
	if params.Description != nil {
		role.Description = *params.Description
	}
	if params.Sort != nil {
		role.Sort = *params.Sort
	}

	if err := s.roleRepo.Update(role); err != nil {
		return nil, err
	}

	// If code changed, migrate Casbin policies
	if role.Code != oldCode {
		policies := s.casbinSvc.GetPoliciesForRole(oldCode)
		var newPolicies [][]string
		for _, p := range policies {
			newPolicies = append(newPolicies, []string{role.Code, p[1], p[2]})
		}
		_ = s.casbinSvc.SetPoliciesForRole(oldCode, nil)
		_ = s.casbinSvc.SetPoliciesForRole(role.Code, newPolicies)
	}

	return role, nil
}

func (s *RoleService) Delete(id uint) error {
	role, err := s.roleRepo.FindByID(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrRoleNotFound
		}
		return err
	}

	if role.IsSystem {
		return ErrSystemRoleDel
	}

	count, err := s.roleRepo.CountUsersByRoleID(id)
	if err != nil {
		return err
	}
	if count > 0 {
		return ErrRoleHasUsers
	}

	if err := s.roleRepo.Delete(id); err != nil {
		return err
	}

	// Clean up Casbin policies
	_ = s.casbinSvc.SetPoliciesForRole(role.Code, nil)

	return nil
}
