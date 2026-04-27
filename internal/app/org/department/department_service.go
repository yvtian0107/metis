package department

import (
	"errors"
	"metis/internal/app/org/domain"

	"github.com/samber/do/v2"
	"gorm.io/gorm"
)

var (
	ErrDepartmentNotFound    = errors.New("department not found")
	ErrDepartmentCodeExists  = errors.New("department code already exists")
	ErrDepartmentHasChildren = errors.New("department has sub-departments")
	ErrDepartmentHasMembers  = errors.New("department has members")
)

type DepartmentService struct {
	repo *DepartmentRepo
}

func NewDepartmentService(i do.Injector) (*DepartmentService, error) {
	repo := do.MustInvoke[*DepartmentRepo](i)
	return &DepartmentService{repo: repo}, nil
}

func (s *DepartmentService) Create(name, code string, parentID, managerID *uint, sort int, description string) (*domain.Department, error) {
	if _, err := s.repo.FindByCode(code); err == nil {
		return nil, ErrDepartmentCodeExists
	}

	dept := &domain.Department{
		Name:        name,
		Code:        code,
		ParentID:    parentID,
		ManagerID:   managerID,
		Sort:        sort,
		Description: description,
		IsActive:    true,
	}
	if err := s.repo.Create(dept); err != nil {
		return nil, err
	}
	return s.repo.FindByID(dept.ID)
}

func (s *DepartmentService) Get(id uint) (*domain.Department, error) {
	dept, err := s.repo.FindByID(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrDepartmentNotFound
		}
		return nil, err
	}
	return dept, nil
}

func (s *DepartmentService) ListAll() ([]domain.Department, error) {
	return s.repo.ListAll()
}

func (s *DepartmentService) Tree() ([]DepartmentTreeNode, error) {
	depts, err := s.repo.ListAll()
	if err != nil {
		return nil, err
	}
	counts, err := s.repo.CountMembersByDepartments()
	if err != nil {
		return nil, err
	}
	managers, err := s.repo.ListManagerNames()
	if err != nil {
		return nil, err
	}
	return buildDepartmentTree(depts, counts, managers), nil
}

type UpdateDepartmentInput struct {
	Name        *string `json:"name"`
	Code        *string `json:"code"`
	ParentID    *uint   `json:"parentId"`
	ManagerID   *uint   `json:"managerId"`
	Sort        *int    `json:"sort"`
	Description *string `json:"description"`
	IsActive    *bool   `json:"isActive"`
}

func (s *DepartmentService) Update(id uint, input UpdateDepartmentInput) (*domain.Department, error) {
	dept, err := s.repo.FindByID(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrDepartmentNotFound
		}
		return nil, err
	}

	updates := map[string]any{}
	if input.Name != nil {
		updates["name"] = *input.Name
	}
	if input.Code != nil {
		if existing, err := s.repo.FindByCode(*input.Code); err == nil && existing.ID != id {
			return nil, ErrDepartmentCodeExists
		}
		updates["code"] = *input.Code
	}
	if input.ParentID != nil {
		updates["parent_id"] = *input.ParentID
	}
	if input.ManagerID != nil {
		updates["manager_id"] = *input.ManagerID
	}
	if input.Sort != nil {
		updates["sort"] = *input.Sort
	}
	if input.Description != nil {
		updates["description"] = *input.Description
	}
	if input.IsActive != nil {
		updates["is_active"] = *input.IsActive
	}

	if len(updates) > 0 {
		if err := s.repo.Update(id, updates); err != nil {
			return nil, err
		}
		dept, _ = s.repo.FindByID(id)
	}
	return dept, nil
}

func (s *DepartmentService) GetAllowedPositions(deptID uint) ([]domain.PositionResponse, error) {
	if _, err := s.Get(deptID); err != nil {
		return nil, err
	}
	positions, err := s.repo.GetAllowedPositions(deptID)
	if err != nil {
		return nil, err
	}
	ids := make([]uint, 0, len(positions))
	for _, p := range positions {
		ids = append(ids, p.ID)
	}
	memberCounts, err := s.repo.CountMembersByPositions(deptID, ids)
	if err != nil {
		return nil, err
	}
	result := make([]domain.PositionResponse, len(positions))
	for i, p := range positions {
		resp := p.ToResponse()
		resp.MemberCount = memberCounts[p.ID]
		result[i] = resp
	}
	return result, nil
}

func (s *DepartmentService) SetAllowedPositions(deptID uint, positionIDs []uint) error {
	if _, err := s.Get(deptID); err != nil {
		return err
	}
	return s.repo.SetAllowedPositions(deptID, positionIDs)
}

func (s *DepartmentService) Delete(id uint) error {
	if _, err := s.repo.FindByID(id); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrDepartmentNotFound
		}
		return err
	}

	hasChildren, err := s.repo.HasChildren(id)
	if err != nil {
		return err
	}
	if hasChildren {
		return ErrDepartmentHasChildren
	}

	hasMembers, err := s.repo.HasMembers(id)
	if err != nil {
		return err
	}
	if hasMembers {
		return ErrDepartmentHasMembers
	}

	return s.repo.Delete(id)
}

// Tree helpers

type DepartmentTreeNode struct {
	domain.DepartmentResponse
	ManagerName string               `json:"managerName"`
	MemberCount int                  `json:"memberCount"`
	Children    []DepartmentTreeNode `json:"children,omitempty"`
}

func buildDepartmentTree(depts []domain.Department, counts map[uint]int, managers map[uint]string) []DepartmentTreeNode {
	byParent := make(map[uint][]domain.Department)
	for _, d := range depts {
		pid := uint(0)
		if d.ParentID != nil {
			pid = *d.ParentID
		}
		byParent[pid] = append(byParent[pid], d)
	}

	var build func(parentID uint) []DepartmentTreeNode
	build = func(parentID uint) []DepartmentTreeNode {
		items := byParent[parentID]
		if len(items) == 0 {
			return nil
		}
		result := make([]DepartmentTreeNode, 0, len(items))
		for _, d := range items {
			result = append(result, DepartmentTreeNode{
				DepartmentResponse: d.ToResponse(),
				ManagerName:        managers[d.ID],
				MemberCount:        counts[d.ID],
				Children:           build(d.ID),
			})
		}
		return result
	}

	return build(0)
}
