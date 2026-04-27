package department

import (
	"github.com/samber/do/v2"
	"gorm.io/gorm"
	"metis/internal/app/org/domain"

	"metis/internal/database"
)

type DepartmentRepo struct {
	db *database.DB
}

func NewDepartmentRepo(i do.Injector) (*DepartmentRepo, error) {
	db := do.MustInvoke[*database.DB](i)
	return &DepartmentRepo{db: db}, nil
}

func (r *DepartmentRepo) Create(dept *domain.Department) error {
	return r.db.Create(dept).Error
}

func (r *DepartmentRepo) FindByID(id uint) (*domain.Department, error) {
	var dept domain.Department
	if err := r.db.First(&dept, id).Error; err != nil {
		return nil, err
	}
	return &dept, nil
}

func (r *DepartmentRepo) FindByCode(code string) (*domain.Department, error) {
	var dept domain.Department
	if err := r.db.Where("code = ?", code).First(&dept).Error; err != nil {
		return nil, err
	}
	return &dept, nil
}

func (r *DepartmentRepo) Update(id uint, updates map[string]any) error {
	return r.db.Model(&domain.Department{}).Where("id = ?", id).Updates(updates).Error
}

func (r *DepartmentRepo) Delete(id uint) error {
	return r.db.Delete(&domain.Department{}, id).Error
}

func (r *DepartmentRepo) ListAll() ([]domain.Department, error) {
	var depts []domain.Department
	if err := r.db.Order("sort ASC, id ASC").Find(&depts).Error; err != nil {
		return nil, err
	}
	return depts, nil
}

func (r *DepartmentRepo) ListActive() ([]domain.Department, error) {
	var depts []domain.Department
	if err := r.db.Where("is_active = ?", true).Order("sort ASC, id ASC").Find(&depts).Error; err != nil {
		return nil, err
	}
	return depts, nil
}

func (r *DepartmentRepo) HasChildren(parentID uint) (bool, error) {
	var count int64
	if err := r.db.Model(&domain.Department{}).Where("parent_id = ?", parentID).Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

func (r *DepartmentRepo) HasMembers(deptID uint) (bool, error) {
	var count int64
	if err := r.db.Model(&domain.UserPosition{}).Where("department_id = ?", deptID).Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

func (r *DepartmentRepo) CountMembersByDepartments() (map[uint]int, error) {
	var rows []struct {
		DepartmentID uint
		Count        int
	}
	if err := r.db.Model(&domain.UserPosition{}).
		Select("department_id, COUNT(*) as count").
		Group("department_id").
		Scan(&rows).Error; err != nil {
		return nil, err
	}
	result := make(map[uint]int, len(rows))
	for _, row := range rows {
		result[row.DepartmentID] = row.Count
	}
	return result, nil
}

type IDParent struct {
	ID       uint
	ParentID *uint
}

func (r *DepartmentRepo) ListAllIDsWithParent(activeOnly bool) ([]IDParent, error) {
	query := r.db.Model(&domain.Department{}).Select("id, parent_id")
	if activeOnly {
		query = query.Where("is_active = ?", true)
	}
	var items []IDParent
	if err := query.Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

// GetAllowedPositions returns the positions allowed for a department.
func (r *DepartmentRepo) GetAllowedPositions(deptID uint) ([]domain.Position, error) {
	var positions []domain.Position
	err := r.db.
		Joins("JOIN department_positions dp ON dp.position_id = positions.id").
		Where("dp.department_id = ? AND dp.deleted_at IS NULL", deptID).
		Order("positions.sort ASC, positions.id ASC").
		Find(&positions).Error
	return positions, err
}

func (r *DepartmentRepo) CountMembersByPositions(deptID uint, positionIDs []uint) (map[uint]int, error) {
	counts := make(map[uint]int, len(positionIDs))
	if len(positionIDs) == 0 {
		return counts, nil
	}
	var rows []struct {
		PositionID uint
		Count      int
	}
	if err := r.db.Model(&domain.UserPosition{}).
		Select("position_id, COUNT(DISTINCT user_id) AS count").
		Where("department_id = ? AND position_id IN ?", deptID, positionIDs).
		Group("position_id").
		Scan(&rows).Error; err != nil {
		return nil, err
	}
	for _, row := range rows {
		counts[row.PositionID] = row.Count
	}
	return counts, nil
}

// SetAllowedPositions replaces all allowed positions for a department in a transaction.
func (r *DepartmentRepo) SetAllowedPositions(deptID uint, positionIDs []uint) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Unscoped().Where("department_id = ?", deptID).Delete(&domain.DepartmentPosition{}).Error; err != nil {
			return err
		}
		for _, posID := range positionIDs {
			dp := domain.DepartmentPosition{DepartmentID: deptID, PositionID: posID}
			if err := tx.Create(&dp).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

// IsPositionAllowed returns true if the position is allowed in the department,
// or if the department has no allowed positions configured (no restriction).
func (r *DepartmentRepo) IsPositionAllowed(deptID, positionID uint) (bool, error) {
	var count int64
	if err := r.db.Model(&domain.DepartmentPosition{}).Where("department_id = ?", deptID).Count(&count).Error; err != nil {
		return false, err
	}
	if count == 0 {
		return true, nil // no restriction
	}
	var match int64
	if err := r.db.Model(&domain.DepartmentPosition{}).
		Where("department_id = ? AND position_id = ?", deptID, positionID).
		Count(&match).Error; err != nil {
		return false, err
	}
	return match > 0, nil
}

// ManagerInfo holds the manager username for a department (used in Tree queries).
type ManagerInfo struct {
	DepartmentID uint
	ManagerName  string
}

// ListManagerNames returns manager usernames for all departments via LEFT JOIN.
func (r *DepartmentRepo) ListManagerNames() (map[uint]string, error) {
	var results []ManagerInfo
	err := r.db.
		Table("departments").
		Select("departments.id as department_id, users.username as manager_name").
		Joins("LEFT JOIN users ON departments.manager_id = users.id").
		Where("departments.manager_id IS NOT NULL AND departments.deleted_at IS NULL").
		Find(&results).Error
	if err != nil {
		return nil, err
	}
	m := make(map[uint]string, len(results))
	for _, r := range results {
		m[r.DepartmentID] = r.ManagerName
	}
	return m, nil
}
