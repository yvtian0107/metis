package org

import (
	"errors"

	"github.com/samber/do/v2"
	"gorm.io/gorm"
)

var (
	ErrAssignmentNotFound       = errors.New("assignment not found")
	ErrAlreadyAssigned          = errors.New("user already assigned to this department")
	ErrPositionAlreadyAssigned  = errors.New("user already has this position in this department")
	ErrDepartmentInactive       = errors.New("department is inactive")
	ErrPositionInactive         = errors.New("position is inactive")
	ErrPositionNotAllowedInDept = errors.New("position is not allowed in this department")
)

type AssignmentService struct {
	repo     *AssignmentRepo
	deptRepo *DepartmentRepo
	posRepo  *PositionRepo
}

func NewAssignmentService(i do.Injector) (*AssignmentService, error) {
	repo := do.MustInvoke[*AssignmentRepo](i)
	deptRepo := do.MustInvoke[*DepartmentRepo](i)
	posRepo := do.MustInvoke[*PositionRepo](i)
	return &AssignmentService{repo: repo, deptRepo: deptRepo, posRepo: posRepo}, nil
}

func (s *AssignmentService) GetUserPositions(userID uint) ([]UserPositionResponse, error) {
	items, err := s.repo.FindByUserID(userID)
	if err != nil {
		return nil, err
	}
	result := make([]UserPositionResponse, 0, len(items))
	for _, item := range items {
		resp := UserPositionResponse{
			ID:           item.ID,
			UserID:       item.UserID,
			DepartmentID: item.DepartmentID,
			PositionID:   item.PositionID,
			IsPrimary:    item.IsPrimary,
			Sort:         item.Sort,
		}
		if item.Department.ID > 0 {
			r := item.Department.ToResponse()
			resp.Department = &r
		}
		if item.Position.ID > 0 {
			r := item.Position.ToResponse()
			resp.Position = &r
		}
		result = append(result, resp)
	}
	return result, nil
}

func (s *AssignmentService) AddUserPosition(userID, deptID, posID uint, isPrimary bool) (*UserPosition, error) {
	// Validate department exists and is active
	dept, err := s.deptRepo.FindByID(deptID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrDepartmentNotFound
		}
		return nil, err
	}
	if !dept.IsActive {
		return nil, ErrDepartmentInactive
	}

	// Validate position exists and is active
	pos, err := s.posRepo.FindByID(posID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrPositionNotFound
		}
		return nil, err
	}
	if !pos.IsActive {
		return nil, ErrPositionInactive
	}

	// Validate position is allowed in this department
	allowed, err := s.deptRepo.IsPositionAllowed(deptID, posID)
	if err != nil {
		return nil, err
	}
	if !allowed {
		return nil, ErrPositionNotAllowedInDept
	}

	exists, err := s.repo.ExistsByUserDeptAndPosition(userID, deptID, posID)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, ErrPositionAlreadyAssigned
	}

	up := &UserPosition{
		UserID:       userID,
		DepartmentID: deptID,
		PositionID:   posID,
	}
	if err := s.repo.AddPositionWithPrimary(up, isPrimary, !isPrimary); err != nil {
		return nil, err
	}
	return s.repo.FindByID(up.ID)
}

func (s *AssignmentService) RemoveUserPosition(userID, assignmentID uint) error {
	err := s.repo.RemovePosition(assignmentID, userID)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ErrAssignmentNotFound
	}
	return err
}

func (s *AssignmentService) UpdateUserPosition(userID, assignmentID uint, positionID *uint, isPrimary *bool) error {
	fields := map[string]any{}
	if positionID != nil {
		// Validate position is allowed in the assignment's department
		assignment, err := s.repo.FindByID(assignmentID)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrAssignmentNotFound
			}
			return err
		}
		allowed, err := s.deptRepo.IsPositionAllowed(assignment.DepartmentID, *positionID)
		if err != nil {
			return err
		}
		if !allowed {
			return ErrPositionNotAllowedInDept
		}
		fields["position_id"] = *positionID
	}
	setPrimary := isPrimary != nil && *isPrimary
	if len(fields) == 0 && !setPrimary {
		return nil
	}
	err := s.repo.UpdatePositionWithPrimary(assignmentID, userID, fields, setPrimary)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ErrAssignmentNotFound
	}
	return err
}

func (s *AssignmentService) SetPrimary(userID uint, assignmentID uint) error {
	err := s.repo.SetPrimary(userID, assignmentID)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ErrAssignmentNotFound
	}
	return err
}

func (s *AssignmentService) ListDepartmentMembers(deptID uint, keyword string, page, pageSize int) ([]MemberWithPositions, int64, error) {
	items, total, err := s.repo.ListUsersByDepartment(deptID, keyword, page, pageSize)
	if err != nil {
		return nil, 0, err
	}
	return GroupAssignmentsByUser(items), total, nil
}

func (s *AssignmentService) SetUserDeptPositions(userID, deptID uint, positionIDs []uint, primaryPositionID *uint) error {
	// Validate department
	dept, err := s.deptRepo.FindByID(deptID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrDepartmentNotFound
		}
		return err
	}
	if !dept.IsActive {
		return ErrDepartmentInactive
	}

	// Validate each position
	for _, posID := range positionIDs {
		pos, err := s.posRepo.FindByID(posID)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrPositionNotFound
			}
			return err
		}
		if !pos.IsActive {
			return ErrPositionInactive
		}
		allowed, err := s.deptRepo.IsPositionAllowed(deptID, posID)
		if err != nil {
			return err
		}
		if !allowed {
			return ErrPositionNotAllowedInDept
		}
	}

	// Validate primaryPositionID is in positionIDs
	if primaryPositionID != nil {
		found := false
		for _, pid := range positionIDs {
			if pid == *primaryPositionID {
				found = true
				break
			}
		}
		if !found {
			primaryPositionID = nil
		}
	}

	return s.repo.SetUserDeptPositions(userID, deptID, positionIDs, primaryPositionID)
}

// Scope helpers for future data permission isolation

func (s *AssignmentService) GetUserDepartmentIDs(userID uint) ([]uint, error) {
	return s.repo.GetUserDepartmentIDs(userID)
}

func (s *AssignmentService) GetUserDepartmentScope(userID uint) ([]uint, error) {
	deptIDs, err := s.repo.GetUserDepartmentIDs(userID)
	if err != nil {
		return nil, err
	}
	if len(deptIDs) == 0 {
		return nil, nil
	}

	// Single query: load all active departments' (id, parent_id)
	allDepts, err := s.deptRepo.ListAllIDsWithParent(true)
	if err != nil {
		return nil, err
	}

	// Build parent → children map in memory
	children := make(map[uint][]uint)
	for _, d := range allDepts {
		pid := uint(0)
		if d.ParentID != nil {
			pid = *d.ParentID
		}
		children[pid] = append(children[pid], d.ID)
	}

	// BFS in memory
	scope := make(map[uint]struct{})
	queue := make([]uint, len(deptIDs))
	copy(queue, deptIDs)
	for _, id := range deptIDs {
		scope[id] = struct{}{}
	}
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		for _, childID := range children[current] {
			if _, ok := scope[childID]; !ok {
				scope[childID] = struct{}{}
				queue = append(queue, childID)
			}
		}
	}

	result := make([]uint, 0, len(scope))
	for id := range scope {
		result = append(result, id)
	}
	return result, nil
}
