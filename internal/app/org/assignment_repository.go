package org

import (
	"errors"

	"github.com/samber/do/v2"
	"gorm.io/gorm"

	"metis/internal/database"
)

type AssignmentRepo struct {
	db *database.DB
}

func NewAssignmentRepo(i do.Injector) (*AssignmentRepo, error) {
	db := do.MustInvoke[*database.DB](i)
	return &AssignmentRepo{db: db}, nil
}

func (r *AssignmentRepo) FindByID(id uint) (*UserPosition, error) {
	var up UserPosition
	if err := r.db.Preload("Department").Preload("Position").First(&up, id).Error; err != nil {
		return nil, err
	}
	return &up, nil
}

func (r *AssignmentRepo) FindByUserID(userID uint) ([]UserPosition, error) {
	var items []UserPosition
	if err := r.db.Where("user_id = ?", userID).
		Preload("Department").
		Preload("Position").
		Order("is_primary DESC, sort ASC, id ASC").
		Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

func (r *AssignmentRepo) FindByDepartmentID(deptID uint) ([]UserPosition, error) {
	var items []UserPosition
	if err := r.db.Where("department_id = ?", deptID).
		Preload("Position").
		Order("is_primary DESC, sort ASC, id ASC").
		Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

func (r *AssignmentRepo) AddPosition(up *UserPosition) error {
	return r.db.Create(up).Error
}

// AddPositionWithPrimary creates a new assignment, atomically handling primary status.
// If setPrimary is true, it demotes any existing primary for this user first.
// If autoSetPrimary is true and the user has no existing assignments, it sets this as primary.
func (r *AssignmentRepo) AddPositionWithPrimary(up *UserPosition, setPrimary, autoSetPrimary bool) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if setPrimary {
			if err := tx.Model(&UserPosition{}).
				Where("user_id = ?", up.UserID).
				Update("is_primary", false).Error; err != nil {
				return err
			}
			up.IsPrimary = true
		} else if autoSetPrimary {
			var count int64
			if err := tx.Model(&UserPosition{}).
				Where("user_id = ?", up.UserID).
				Count(&count).Error; err != nil {
				return err
			}
			if count == 0 {
				up.IsPrimary = true
			}
		}
		return tx.Create(up).Error
	})
}

func (r *AssignmentRepo) ExistsByUserAndDept(userID, deptID uint) (bool, error) {
	var count int64
	if err := r.db.Model(&UserPosition{}).
		Where("user_id = ? AND department_id = ?", userID, deptID).
		Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

func (r *AssignmentRepo) ExistsByUserDeptAndPosition(userID, deptID, posID uint) (bool, error) {
	var count int64
	if err := r.db.Model(&UserPosition{}).
		Where("user_id = ? AND department_id = ? AND position_id = ?", userID, deptID, posID).
		Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

func (r *AssignmentRepo) FindByUserAndDept(userID, deptID uint) ([]UserPosition, error) {
	var items []UserPosition
	if err := r.db.Where("user_id = ? AND department_id = ?", userID, deptID).
		Preload("Position").
		Order("is_primary DESC, sort ASC, id ASC").
		Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

// SetUserDeptPositions atomically replaces all positions for a user in a department.
func (r *AssignmentRepo) SetUserDeptPositions(userID, deptID uint, positionIDs []uint, primaryPositionID *uint) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		// 1. Get current assignments for this user+dept
		var current []UserPosition
		if err := tx.Where("user_id = ? AND department_id = ?", userID, deptID).
			Find(&current).Error; err != nil {
			return err
		}

		// 2. Build sets for diff
		currentSet := make(map[uint]UserPosition)
		for _, c := range current {
			currentSet[c.PositionID] = c
		}
		newSet := make(map[uint]bool)
		for _, pid := range positionIDs {
			newSet[pid] = true
		}

		// 3. Delete removed positions
		for pid, assignment := range currentSet {
			if !newSet[pid] {
				if err := tx.Delete(&assignment).Error; err != nil {
					return err
				}
			}
		}

		// 4. Add new positions
		for _, pid := range positionIDs {
			if _, exists := currentSet[pid]; !exists {
				up := &UserPosition{
					UserID:       userID,
					DepartmentID: deptID,
					PositionID:   pid,
				}
				if err := tx.Create(up).Error; err != nil {
					return err
				}
			}
		}

		// 5. Handle primary if specified
		if primaryPositionID != nil {
			// Clear all primary for this user
			if err := tx.Model(&UserPosition{}).
				Where("user_id = ?", userID).
				Update("is_primary", false).Error; err != nil {
				return err
			}
			// Set the target as primary
			if err := tx.Model(&UserPosition{}).
				Where("user_id = ? AND department_id = ? AND position_id = ?",
					userID, deptID, *primaryPositionID).
				Update("is_primary", true).Error; err != nil {
				return err
			}
		}

		return nil
	})
}

func (r *AssignmentRepo) RemovePosition(assignmentID, userID uint) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		var up UserPosition
		if err := tx.Where("id = ? AND user_id = ?", assignmentID, userID).First(&up).Error; err != nil {
			return err
		}
		if err := tx.Delete(&up).Error; err != nil {
			return err
		}
		// If removed was primary, auto-promote next
		if up.IsPrimary {
			var next UserPosition
			if err := tx.Where("user_id = ?", userID).Order("sort ASC, id ASC").First(&next).Error; err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					return nil // no more assignments
				}
				return err
			}
			return tx.Model(&next).Update("is_primary", true).Error
		}
		return nil
	})
}

// UpdatePositionWithPrimary updates assignment fields, atomically handling isPrimary changes.
// If setPrimary is true, it demotes existing primary before setting this one.
func (r *AssignmentRepo) UpdatePositionWithPrimary(assignmentID, userID uint, fields map[string]any, setPrimary bool) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if setPrimary {
			if err := tx.Model(&UserPosition{}).
				Where("user_id = ?", userID).
				Update("is_primary", false).Error; err != nil {
				return err
			}
			fields["is_primary"] = true
		}
		result := tx.Model(&UserPosition{}).
			Where("id = ? AND user_id = ?", assignmentID, userID).
			Updates(fields)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return gorm.ErrRecordNotFound
		}
		return nil
	})
}

func (r *AssignmentRepo) SetPrimary(userID uint, assignmentID uint) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		// Verify target assignment exists
		var target UserPosition
		if err := tx.Where("id = ? AND user_id = ?", assignmentID, userID).First(&target).Error; err != nil {
			return err
		}
		if err := tx.Model(&UserPosition{}).
			Where("user_id = ?", userID).
			Update("is_primary", false).Error; err != nil {
			return err
		}
		return tx.Model(&UserPosition{}).
			Where("id = ?", assignmentID).
			Update("is_primary", true).Error
	})
}

func (r *AssignmentRepo) DeleteByID(id uint) error {
	return r.db.Delete(&UserPosition{}, id).Error
}

func (r *AssignmentRepo) ListUsersByDepartment(deptID uint, keyword string, page, pageSize int) ([]AssignmentItem, int64, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}

	base := r.db.Table("user_positions").
		Select("user_positions.id as assignment_id, user_positions.user_id, users.username, users.email, users.avatar, user_positions.department_id, user_positions.position_id, positions.name as position_name, user_positions.is_primary, user_positions.created_at").
		Joins("LEFT JOIN users ON users.id = user_positions.user_id").
		Joins("LEFT JOIN positions ON positions.id = user_positions.position_id").
		Where("user_positions.department_id = ? AND user_positions.deleted_at IS NULL", deptID)

	if keyword != "" {
		like := "%" + keyword + "%"
		base = base.Where("(users.username LIKE ? OR users.email LIKE ?)", like, like)
	}

	// Count distinct users for pagination
	countQuery := r.db.Table("user_positions").
		Joins("LEFT JOIN users ON users.id = user_positions.user_id").
		Where("user_positions.department_id = ? AND user_positions.deleted_at IS NULL", deptID)
	if keyword != "" {
		like := "%" + keyword + "%"
		countQuery = countQuery.Where("(users.username LIKE ? OR users.email LIKE ?)", like, like)
	}
	var total int64
	if err := countQuery.Distinct("user_positions.user_id").Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// Get distinct user IDs for this page
	offset := (page - 1) * pageSize
	var userIDs []uint
	uidQuery := r.db.Table("user_positions").
		Select("DISTINCT user_positions.user_id").
		Joins("LEFT JOIN users ON users.id = user_positions.user_id").
		Where("user_positions.department_id = ? AND user_positions.deleted_at IS NULL", deptID)
	if keyword != "" {
		like := "%" + keyword + "%"
		uidQuery = uidQuery.Where("(users.username LIKE ? OR users.email LIKE ?)", like, like)
	}
	if err := uidQuery.Order("user_positions.user_id ASC").Offset(offset).Limit(pageSize).Pluck("user_positions.user_id", &userIDs).Error; err != nil {
		return nil, 0, err
	}
	if len(userIDs) == 0 {
		return nil, total, nil
	}

	// Fetch all assignment items for these users in this department
	var items []AssignmentItem
	if err := base.Where("user_positions.user_id IN ?", userIDs).
		Order("user_positions.user_id ASC, user_positions.is_primary DESC, user_positions.sort ASC, user_positions.id ASC").
		Find(&items).Error; err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

// GroupAssignmentsByUser groups flat AssignmentItem rows into MemberWithPositions.
func GroupAssignmentsByUser(items []AssignmentItem) []MemberWithPositions {
	if len(items) == 0 {
		return nil
	}
	orderMap := make(map[uint]int)     // userId → first seen index
	memberMap := make(map[uint]*MemberWithPositions)
	for _, item := range items {
		if _, exists := memberMap[item.UserID]; !exists {
			orderMap[item.UserID] = len(orderMap)
			memberMap[item.UserID] = &MemberWithPositions{
				UserID:       item.UserID,
				Username:     item.Username,
				Email:        item.Email,
				Avatar:       item.Avatar,
				DepartmentID: item.DepartmentID,
				CreatedAt:    item.CreatedAt,
			}
		}
		m := memberMap[item.UserID]
		m.Positions = append(m.Positions, MemberPositionItem{
			AssignmentID: item.AssignmentID,
			PositionID:   item.PositionID,
			PositionName: item.PositionName,
			IsPrimary:    item.IsPrimary,
		})
		if item.CreatedAt.Before(m.CreatedAt) {
			m.CreatedAt = item.CreatedAt
		}
	}
	result := make([]MemberWithPositions, len(memberMap))
	for uid, m := range memberMap {
		result[orderMap[uid]] = *m
	}
	return result
}

func (r *AssignmentRepo) CountByDepartments() (map[uint]int, error) {
	type countRow struct {
		DepartmentID uint
		Count        int
	}
	var rows []countRow
	if err := r.db.Model(&UserPosition{}).
		Select("department_id, COUNT(*) as count").
		Group("department_id").
		Find(&rows).Error; err != nil {
		return nil, err
	}
	result := make(map[uint]int, len(rows))
	for _, row := range rows {
		result[row.DepartmentID] = row.Count
	}
	return result, nil
}

func (r *AssignmentRepo) GetUserDepartmentIDs(userID uint) ([]uint, error) {
	var ids []uint
	if err := r.db.Model(&UserPosition{}).
		Where("user_id = ?", userID).
		Distinct().
		Pluck("department_id", &ids).Error; err != nil {
		return nil, err
	}
	return ids, nil
}

func (r *AssignmentRepo) GetUserPositionIDs(userID uint) ([]uint, error) {
	var ids []uint
	if err := r.db.Model(&UserPosition{}).
		Where("user_id = ?", userID).
		Distinct().
		Pluck("position_id", &ids).Error; err != nil {
		return nil, err
	}
	return ids, nil
}

func (r *AssignmentRepo) GetSubDepartmentIDs(parentIDs []uint, activeOnly bool) ([]uint, error) {
	if len(parentIDs) == 0 {
		return nil, nil
	}
	query := r.db.Model(&Department{}).Where("parent_id IN ?", parentIDs)
	if activeOnly {
		query = query.Where("is_active = ?", true)
	}
	var ids []uint
	if err := query.Pluck("id", &ids).Error; err != nil {
		return nil, err
	}
	return ids, nil
}

func (r *AssignmentRepo) GetUserPrimaryPosition(userID uint) (*UserPosition, error) {
	var up UserPosition
	if err := r.db.Where("user_id = ? AND is_primary = ?", userID, true).
		Preload("Department").
		Preload("Position").
		First(&up).Error; err != nil {
		return nil, err
	}
	return &up, nil
}
