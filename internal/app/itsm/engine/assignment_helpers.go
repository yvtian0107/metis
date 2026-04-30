package engine

import (
	"fmt"
	"time"

	"gorm.io/gorm"
)

func (r *ParticipantResolver) userOrgIDs(userID uint) (positionIDs []uint, departmentIDs []uint) {
	if r == nil || r.orgResolver == nil || userID == 0 {
		return nil, nil
	}
	if ids, err := r.orgResolver.GetUserPositionIDs(userID); err == nil {
		positionIDs = ids
	}
	if ids, err := r.orgResolver.GetUserDepartmentIDs(userID); err == nil {
		departmentIDs = ids
	}
	return positionIDs, departmentIDs
}

func assignmentOperatorCondition(db *gorm.DB, alias string, userID uint, positionIDs []uint, departmentIDs []uint) *gorm.DB {
	col := func(name string) string {
		return fmt.Sprintf("%s.%s", alias, name)
	}

	cond := db.Where(fmt.Sprintf("%s = ? OR %s = ?", col("user_id"), col("assignee_id")), userID, userID)
	if len(positionIDs) > 0 && len(departmentIDs) > 0 {
		cond = cond.Or(
			db.Where(
				fmt.Sprintf("%s = ? AND %s IN ? AND %s IN ?", col("participant_type"), col("position_id"), col("department_id")),
				"position_department", positionIDs, departmentIDs,
			),
		)
		cond = cond.Or(
			db.Where(
				fmt.Sprintf("COALESCE(%s, '') = '' AND %s IN ? AND %s IN ?", col("participant_type"), col("position_id"), col("department_id")),
				positionIDs, departmentIDs,
			),
		)
	}
	if len(positionIDs) > 0 {
		cond = cond.Or(
			db.Where(
				fmt.Sprintf("%s = ? AND %s IN ?", col("participant_type"), col("position_id")),
				"position", positionIDs,
			),
		)
		cond = cond.Or(
			db.Where(
				fmt.Sprintf("COALESCE(%s, '') = '' AND %s IN ? AND %s IS NULL", col("participant_type"), col("position_id"), col("department_id")),
				positionIDs,
			),
		)
	}
	if len(departmentIDs) > 0 {
		cond = cond.Or(
			db.Where(
				fmt.Sprintf("%s = ? AND %s IN ?", col("participant_type"), col("department_id")),
				"department", departmentIDs,
			),
		)
		cond = cond.Or(
			db.Where(
				fmt.Sprintf("COALESCE(%s, '') = '' AND %s IN ? AND %s IS NULL", col("participant_type"), col("department_id"), col("position_id")),
				departmentIDs,
			),
		)
	}
	return cond
}

func completePendingAssignment(tx *gorm.DB, resolver *ParticipantResolver, activityID uint, operatorID uint, outcome string, now time.Time, positionIDs []uint, departmentIDs []uint, orgScopeReady bool) (*assignmentModel, bool, error) {
	if operatorID == 0 {
		return nil, false, nil
	}

	if !orgScopeReady {
		positionIDs, departmentIDs = resolver.userOrgIDs(operatorID)
	}
	query := tx.Model(&assignmentModel{}).
		Where("activity_id = ? AND status IN ?", activityID, []string{"pending", "claimed"}).
		Where(assignmentOperatorCondition(tx, "itsm_ticket_assignments", operatorID, positionIDs, departmentIDs))

	status := HumanActivityResultStatus(outcome)
	result := query.Updates(map[string]any{
		"assignee_id": operatorID,
		"status":      status,
		"is_current":  false,
		"finished_at": now,
	})
	if result.Error != nil {
		return nil, false, result.Error
	}
	if result.RowsAffected == 0 {
		var assignmentCount int64
		if err := tx.Model(&assignmentModel{}).Where("activity_id = ?", activityID).Count(&assignmentCount).Error; err != nil {
			return nil, false, err
		}
		if assignmentCount == 0 {
			return nil, false, nil
		}
		return nil, false, ErrNoActiveAssignment
	}

	var completed assignmentModel
	err := tx.Where("activity_id = ? AND assignee_id = ? AND status = ?", activityID, operatorID, status).
		Order("id DESC").
		First(&completed).Error
	if err != nil {
		return nil, true, nil
	}
	return &completed, true, nil
}

func activityBecameInactive(tx *gorm.DB, activityID uint) bool {
	var row struct {
		Status string
	}
	if err := tx.Model(&activityModel{}).Select("status").Where("id = ?", activityID).First(&row).Error; err != nil {
		return false
	}
	return row.Status != ActivityPending && row.Status != ActivityInProgress
}
