package resolver

import (
	"fmt"
	"metis/internal/app/org/domain"
	"strings"

	"gorm.io/gorm"

	"metis/internal/app"
	"metis/internal/app/org/assignment"
	"metis/internal/model"
)

// OrgResolverImpl implements app.OrgResolver backed by Org App services and repos.
type OrgResolverImpl struct {
	svc  *assignment.AssignmentService
	repo *assignment.AssignmentRepo
	db   *gorm.DB
}

func NewOrgResolver(svc *assignment.AssignmentService, repo *assignment.AssignmentRepo, db *gorm.DB) *OrgResolverImpl {
	return &OrgResolverImpl{svc: svc, repo: repo, db: db}
}

// --- DataScope ---

func (r *OrgResolverImpl) GetUserDeptScope(userID uint, includeSubDepts bool) ([]uint, error) {
	if includeSubDepts {
		return r.svc.GetUserDepartmentScope(userID)
	}
	return r.svc.GetUserDepartmentIDs(userID)
}

// --- ITSM participant matching ---

func (r *OrgResolverImpl) GetUserPositionIDs(userID uint) ([]uint, error) {
	return r.repo.GetUserPositionIDs(userID)
}

func (r *OrgResolverImpl) GetUserDepartmentIDs(userID uint) ([]uint, error) {
	return r.repo.GetUserDepartmentIDs(userID)
}

// --- AI tools ---

func (r *OrgResolverImpl) GetUserPositions(userID uint) ([]app.OrgPosition, error) {
	items, err := r.repo.FindByUserID(userID)
	if err != nil {
		return nil, err
	}
	var result []app.OrgPosition
	for _, item := range items {
		if item.Position.ID > 0 {
			result = append(result, app.OrgPosition{
				ID:        item.Position.ID,
				Code:      item.Position.Code,
				Name:      item.Position.Name,
				IsPrimary: item.IsPrimary,
			})
		}
	}
	return result, nil
}

func (r *OrgResolverImpl) GetUserDepartment(userID uint) (*app.OrgDepartment, error) {
	primary, err := r.repo.GetUserPrimaryPosition(userID)
	if err != nil {
		return nil, err
	}
	if primary == nil || primary.Department.ID == 0 {
		return nil, nil
	}
	return &app.OrgDepartment{
		ID:   primary.Department.ID,
		Code: primary.Department.Code,
		Name: primary.Department.Name,
	}, nil
}

func (r *OrgResolverImpl) QueryContext(username, deptCode, positionCode string, includeInactive bool) (*app.OrgContextResult, error) {
	result := &app.OrgContextResult{
		Users:       []app.OrgContextUser{},
		Departments: []app.OrgContextDepartment{},
		Positions:   []app.OrgContextPosition{},
	}

	// Query by username
	if username != "" {
		var users []model.User
		q := r.db.Where("username LIKE ?", "%"+username+"%")
		if !includeInactive {
			q = q.Where("is_active = ?", true)
		}
		if err := q.Limit(20).Find(&users).Error; err != nil {
			return nil, fmt.Errorf("query users: %w", err)
		}
		for _, u := range users {
			cu := app.OrgContextUser{
				ID:       u.ID,
				Username: u.Username,
				Email:    u.Email,
				IsActive: u.IsActive,
			}
			// Enrich with org data
			if dept, err := r.GetUserDepartment(u.ID); err == nil && dept != nil {
				cu.Department = dept
			}
			if positions, err := r.GetUserPositions(u.ID); err == nil {
				cu.Positions = positions
			}
			result.Users = append(result.Users, cu)
		}
	}

	// Query by department code
	if deptCode != "" {
		var depts []domain.Department
		q := r.db.Where("code LIKE ?", "%"+deptCode+"%")
		if !includeInactive {
			q = q.Where("is_active = ?", true)
		}
		if err := q.Limit(20).Find(&depts).Error; err != nil {
			return nil, fmt.Errorf("query departments: %w", err)
		}
		for _, d := range depts {
			cd := app.OrgContextDepartment{
				ID:       d.ID,
				Code:     d.Code,
				Name:     d.Name,
				IsActive: d.IsActive,
			}
			// Resolve parent code
			if d.ParentID != nil {
				var parent domain.Department
				if err := r.db.Select("code").First(&parent, *d.ParentID).Error; err == nil {
					cd.ParentCode = parent.Code
				}
			}
			result.Departments = append(result.Departments, cd)
		}
	}

	// Query by position code
	if positionCode != "" {
		var positions []domain.Position
		q := r.db.Where("code LIKE ?", "%"+positionCode+"%")
		if !includeInactive {
			q = q.Where("is_active = ?", true)
		}
		if err := q.Limit(20).Find(&positions).Error; err != nil {
			return nil, fmt.Errorf("query positions: %w", err)
		}
		for _, p := range positions {
			result.Positions = append(result.Positions, app.OrgContextPosition{
				ID:       p.ID,
				Code:     p.Code,
				Name:     p.Name,
				IsActive: p.IsActive,
			})
		}
	}

	// Build summary
	var parts []string
	if len(result.Users) > 0 {
		parts = append(parts, fmt.Sprintf("找到 %d 个用户", len(result.Users)))
	}
	if len(result.Departments) > 0 {
		parts = append(parts, fmt.Sprintf("%d 个部门", len(result.Departments)))
	}
	if len(result.Positions) > 0 {
		parts = append(parts, fmt.Sprintf("%d 个岗位", len(result.Positions)))
	}
	if len(parts) == 0 {
		result.Summary = "未找到匹配的组织信息"
	} else {
		result.Summary = strings.Join(parts, "，")
	}

	return result, nil
}

// --- Participant resolution ---

func (r *OrgResolverImpl) FindUsersByPositionCode(posCode string) ([]uint, error) {
	var userIDs []uint
	err := r.db.Table("user_positions").
		Joins("JOIN positions ON positions.id = user_positions.position_id").
		Joins("JOIN users ON users.id = user_positions.user_id").
		Where("positions.code = ? AND users.is_active = ?", posCode, true).
		Pluck("DISTINCT users.id", &userIDs).Error
	return userIDs, err
}

func (r *OrgResolverImpl) FindUsersByDepartmentCode(deptCode string) ([]uint, error) {
	var userIDs []uint
	err := r.db.Table("user_positions").
		Joins("JOIN departments ON departments.id = user_positions.department_id").
		Joins("JOIN users ON users.id = user_positions.user_id").
		Where("departments.code = ? AND users.is_active = ?", deptCode, true).
		Pluck("DISTINCT users.id", &userIDs).Error
	return userIDs, err
}

func (r *OrgResolverImpl) FindUsersByPositionAndDepartment(posCode, deptCode string) ([]uint, error) {
	var userIDs []uint
	err := r.db.Table("user_positions").
		Joins("JOIN positions ON positions.id = user_positions.position_id").
		Joins("JOIN departments ON departments.id = user_positions.department_id").
		Joins("JOIN users ON users.id = user_positions.user_id").
		Where("positions.code = ? AND departments.code = ? AND users.is_active = ?", posCode, deptCode, true).
		Pluck("DISTINCT users.id", &userIDs).Error
	return userIDs, err
}

func (r *OrgResolverImpl) FindUsersByPositionID(positionID uint) ([]uint, error) {
	var userIDs []uint
	err := r.db.Table("user_positions").
		Joins("JOIN users ON users.id = user_positions.user_id").
		Where("user_positions.position_id = ? AND users.is_active = ?", positionID, true).
		Pluck("DISTINCT users.id", &userIDs).Error
	return userIDs, err
}

func (r *OrgResolverImpl) FindUsersByDepartmentID(departmentID uint) ([]uint, error) {
	var userIDs []uint
	err := r.db.Table("user_positions").
		Joins("JOIN users ON users.id = user_positions.user_id").
		Where("user_positions.department_id = ? AND users.is_active = ?", departmentID, true).
		Pluck("DISTINCT users.id", &userIDs).Error
	return userIDs, err
}

func (r *OrgResolverImpl) FindManagerByUserID(userID uint) (uint, error) {
	var user struct {
		ManagerID *uint
	}
	if err := r.db.Table("users").Where("id = ?", userID).Select("manager_id").First(&user).Error; err != nil {
		return 0, fmt.Errorf("user %d not found: %w", userID, err)
	}
	if user.ManagerID == nil {
		return 0, nil
	}
	return *user.ManagerID, nil
}
