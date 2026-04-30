package resolver

import (
	"fmt"
	"metis/internal/app/org/domain"
	"slices"
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
	noFilters := username == "" && deptCode == "" && positionCode == ""

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

	// No-filter org context is used by prompt builders. Return the org vocabulary
	// only; users are intentionally excluded to avoid unnecessary personal data.
	if noFilters {
		var depts []domain.Department
		deptQ := r.db.Order("sort ASC, id ASC")
		if !includeInactive {
			deptQ = deptQ.Where("is_active = ?", true)
		}
		if err := deptQ.Limit(50).Find(&depts).Error; err != nil {
			return nil, fmt.Errorf("query departments: %w", err)
		}
		for _, d := range depts {
			cd := app.OrgContextDepartment{
				ID:       d.ID,
				Code:     d.Code,
				Name:     d.Name,
				IsActive: d.IsActive,
			}
			if d.ParentID != nil {
				var parent domain.Department
				if err := r.db.Select("code").First(&parent, *d.ParentID).Error; err == nil {
					cd.ParentCode = parent.Code
				}
			}
			result.Departments = append(result.Departments, cd)
		}

		var positions []domain.Position
		posQ := r.db.Order("sort ASC, id ASC")
		if !includeInactive {
			posQ = posQ.Where("is_active = ?", true)
		}
		if err := posQ.Limit(50).Find(&positions).Error; err != nil {
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

// SearchOrgStructure searches active department and position vocabulary by
// Chinese name or code. Results are bounded and never include user details.
func (r *OrgResolverImpl) SearchOrgStructure(query string, kinds []string, limit int) (*app.OrgStructureSearchResult, error) {
	limit = normalizeOrgStructureLimit(limit)
	query = strings.TrimSpace(query)
	result := &app.OrgStructureSearchResult{
		Departments: []app.OrgContextDepartment{},
		Positions:   []app.OrgContextPosition{},
	}
	if query == "" {
		result.Summary = "查询词为空"
		return result, nil
	}

	if orgStructureKindAllowed(kinds, "department") {
		departments, err := r.searchDepartments(query, limit)
		if err != nil {
			return nil, err
		}
		result.Departments = departments
	}
	if orgStructureKindAllowed(kinds, "position") {
		positions, err := r.searchPositions(query, limit)
		if err != nil {
			return nil, err
		}
		result.Positions = positions
	}
	result.Summary = fmt.Sprintf("找到 %d 个部门、%d 个岗位", len(result.Departments), len(result.Positions))
	return result, nil
}

// ResolveOrgParticipant maps natural department/position hints to participant
// configuration candidates. It returns aggregate candidate counts only.
func (r *OrgResolverImpl) ResolveOrgParticipant(departmentHint, positionHint string, limit int) (*app.OrgParticipantResolveResult, error) {
	limit = normalizeOrgStructureLimit(limit)
	departmentHint = strings.TrimSpace(departmentHint)
	positionHint = strings.TrimSpace(positionHint)
	result := &app.OrgParticipantResolveResult{Candidates: []app.OrgParticipantCandidate{}}
	if departmentHint == "" && positionHint == "" {
		result.Summary = "部门和岗位提示均为空"
		return result, nil
	}

	departments := []app.OrgContextDepartment{}
	if departmentHint != "" {
		found, err := r.searchDepartments(departmentHint, limit)
		if err != nil {
			return nil, err
		}
		departments = found
	}
	positions := []app.OrgContextPosition{}
	if positionHint != "" {
		found, err := r.searchPositions(positionHint, limit)
		if err != nil {
			return nil, err
		}
		positions = found
	}

	switch {
	case departmentHint != "" && positionHint != "":
		for _, dept := range departments {
			for _, pos := range positions {
				count, err := r.countActiveUsersForParticipant(dept.ID, pos.ID)
				if err != nil {
					return nil, err
				}
				allowed, err := r.isDepartmentPositionAllowed(dept.ID, pos.ID)
				if err != nil {
					return nil, err
				}
				if !allowed && count == 0 {
					continue
				}
				result.Candidates = append(result.Candidates, app.OrgParticipantCandidate{
					Type:           "position_department",
					DepartmentCode: dept.Code,
					DepartmentName: dept.Name,
					PositionCode:   pos.Code,
					PositionName:   pos.Name,
					CandidateCount: count,
				})
				if len(result.Candidates) >= limit {
					break
				}
			}
			if len(result.Candidates) >= limit {
				break
			}
		}
	case departmentHint != "":
		for _, dept := range departments {
			count, err := r.countActiveUsersForParticipant(dept.ID, 0)
			if err != nil {
				return nil, err
			}
			result.Candidates = append(result.Candidates, app.OrgParticipantCandidate{
				Type:           "department",
				DepartmentCode: dept.Code,
				DepartmentName: dept.Name,
				CandidateCount: count,
			})
		}
	case positionHint != "":
		for _, pos := range positions {
			count, err := r.countActiveUsersForParticipant(0, pos.ID)
			if err != nil {
				return nil, err
			}
			result.Candidates = append(result.Candidates, app.OrgParticipantCandidate{
				Type:           "position",
				PositionCode:   pos.Code,
				PositionName:   pos.Name,
				CandidateCount: count,
			})
		}
	}

	result.Summary = fmt.Sprintf("找到 %d 个参与人配置候选", len(result.Candidates))
	return result, nil
}

func normalizeOrgStructureLimit(limit int) int {
	if limit <= 0 || limit > 10 {
		return 10
	}
	return limit
}

func orgStructureKindAllowed(kinds []string, kind string) bool {
	if len(kinds) == 0 {
		return true
	}
	return slices.Contains(kinds, kind)
}

func (r *OrgResolverImpl) searchDepartments(query string, limit int) ([]app.OrgContextDepartment, error) {
	var departments []domain.Department
	like := "%" + query + "%"
	if err := r.db.Where("is_active = ?", true).
		Where("(name LIKE ? OR code LIKE ?)", like, like).
		Order("sort ASC, id ASC").
		Limit(limit).
		Find(&departments).Error; err != nil {
		return nil, fmt.Errorf("search departments: %w", err)
	}
	result := make([]app.OrgContextDepartment, 0, len(departments))
	for _, d := range departments {
		item := app.OrgContextDepartment{
			ID:       d.ID,
			Code:     d.Code,
			Name:     d.Name,
			IsActive: d.IsActive,
		}
		if d.ParentID != nil {
			var parent domain.Department
			if err := r.db.Select("code").First(&parent, *d.ParentID).Error; err == nil {
				item.ParentCode = parent.Code
			}
		}
		result = append(result, item)
	}
	return result, nil
}

func (r *OrgResolverImpl) searchPositions(query string, limit int) ([]app.OrgContextPosition, error) {
	var positions []domain.Position
	like := "%" + query + "%"
	if err := r.db.Where("is_active = ?", true).
		Where("(name LIKE ? OR code LIKE ?)", like, like).
		Order("sort ASC, id ASC").
		Limit(limit).
		Find(&positions).Error; err != nil {
		return nil, fmt.Errorf("search positions: %w", err)
	}
	result := make([]app.OrgContextPosition, 0, len(positions))
	for _, p := range positions {
		result = append(result, app.OrgContextPosition{
			ID:       p.ID,
			Code:     p.Code,
			Name:     p.Name,
			IsActive: p.IsActive,
		})
	}
	return result, nil
}

func (r *OrgResolverImpl) isDepartmentPositionAllowed(deptID, posID uint) (bool, error) {
	var count int64
	if err := r.db.Model(&domain.DepartmentPosition{}).
		Where("department_id = ? AND position_id = ?", deptID, posID).
		Count(&count).Error; err != nil {
		return false, fmt.Errorf("check department position: %w", err)
	}
	return count > 0, nil
}

func (r *OrgResolverImpl) countActiveUsersForParticipant(deptID, posID uint) (int64, error) {
	q := r.db.Table("user_positions").
		Joins("JOIN users ON users.id = user_positions.user_id").
		Where("user_positions.deleted_at IS NULL AND users.is_active = ?", true)
	if deptID > 0 {
		q = q.Where("user_positions.department_id = ?", deptID)
	}
	if posID > 0 {
		q = q.Where("user_positions.position_id = ?", posID)
	}
	var count int64
	if err := q.Count(&count).Error; err != nil {
		return 0, fmt.Errorf("count active users: %w", err)
	}
	return count, nil
}

// --- Participant resolution ---

func (r *OrgResolverImpl) FindUsersByPositionCode(posCode string) ([]uint, error) {
	var userIDs []uint
	err := r.db.Table("user_positions").
		Joins("JOIN positions ON positions.id = user_positions.position_id").
		Joins("JOIN users ON users.id = user_positions.user_id").
		Where("positions.code = ? AND user_positions.deleted_at IS NULL AND users.is_active = ?", posCode, true).
		Pluck("DISTINCT users.id", &userIDs).Error
	return userIDs, err
}

func (r *OrgResolverImpl) FindUsersByDepartmentCode(deptCode string) ([]uint, error) {
	var userIDs []uint
	err := r.db.Table("user_positions").
		Joins("JOIN departments ON departments.id = user_positions.department_id").
		Joins("JOIN users ON users.id = user_positions.user_id").
		Where("departments.code = ? AND user_positions.deleted_at IS NULL AND users.is_active = ?", deptCode, true).
		Pluck("DISTINCT users.id", &userIDs).Error
	return userIDs, err
}

func (r *OrgResolverImpl) FindUsersByPositionAndDepartment(posCode, deptCode string) ([]uint, error) {
	var userIDs []uint
	err := r.db.Table("user_positions").
		Joins("JOIN positions ON positions.id = user_positions.position_id").
		Joins("JOIN departments ON departments.id = user_positions.department_id").
		Joins("JOIN users ON users.id = user_positions.user_id").
		Where("positions.code = ? AND departments.code = ? AND user_positions.deleted_at IS NULL AND users.is_active = ?", posCode, deptCode, true).
		Pluck("DISTINCT users.id", &userIDs).Error
	return userIDs, err
}

func (r *OrgResolverImpl) FindUsersByPositionID(positionID uint) ([]uint, error) {
	var userIDs []uint
	err := r.db.Table("user_positions").
		Joins("JOIN users ON users.id = user_positions.user_id").
		Where("user_positions.position_id = ? AND user_positions.deleted_at IS NULL AND users.is_active = ?", positionID, true).
		Pluck("DISTINCT users.id", &userIDs).Error
	return userIDs, err
}

func (r *OrgResolverImpl) FindUsersByDepartmentID(departmentID uint) ([]uint, error) {
	var userIDs []uint
	err := r.db.Table("user_positions").
		Joins("JOIN users ON users.id = user_positions.user_id").
		Where("user_positions.department_id = ? AND user_positions.deleted_at IS NULL AND users.is_active = ?", departmentID, true).
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
