package bdd

// steps_countersign_test.go — step definitions for the multi-role countersign BDD scenarios.

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	. "metis/internal/app/itsm/domain"
	"time"

	"github.com/cucumber/godog"

	"metis/internal/app/itsm/engine"
)

// registerCountersignSteps registers all countersign BDD step definitions.
func registerCountersignSteps(sc *godog.ScenarioContext, bc *bddContext) {
	sc.Given(`^已定义多角色并行处理协作规范$`, bc.givenCountersignCollaborationSpec)
	sc.Given(`^已基于协作规范发布多角色并行处理服务（智能引擎）$`, bc.givenCountersignSmartServicePublished)
	sc.Given(`^"([^"]*)" 已创建并行处理工单，场景为 "([^"]*)"$`, bc.givenCountersignTicketCreated)

	sc.When(`^并行处理组中岗位 "([^"]*)" 的处理人认领并处理完成$`, bc.whenCountersignRoleProcesss)

	sc.Then(`^应存在一个并行处理活动组，包含 (\d+) 个并行活动$`, bc.thenParallelGroupExists)
	sc.Then(`^并行处理组仍有未完成活动，不应触发下一步$`, bc.thenParallelGroupNotConverged)
	sc.Then(`^并行处理组全部完成，应触发下一轮决策$`, bc.thenParallelGroupConverged)
	sc.Then(`^不应存在分配给岗位 "([^"]*)" 的待处理活动$`, bc.thenNoActivityForPosition)
}

// --- Given steps ---

func (bc *bddContext) givenCountersignCollaborationSpec() error {
	bc.collaborationSpec = countersignCollaborationSpec
	return nil
}

func (bc *bddContext) givenCountersignSmartServicePublished() error {
	return publishCountersignSmartService(bc)
}

func (bc *bddContext) givenCountersignTicketCreated(username, caseKey string) error {
	user, ok := bc.usersByName[username]
	if !ok {
		return fmt.Errorf("user %q not found in context", username)
	}

	payload, ok := countersignCasePayloads[caseKey]
	if !ok {
		return fmt.Errorf("unknown countersign case key %q", caseKey)
	}

	formJSON, _ := json.Marshal(payload.FormData)

	ticket := &Ticket{
		Code:         fmt.Sprintf("CS-%s-%d", caseKey, time.Now().UnixNano()),
		Title:        payload.Summary,
		ServiceID:    bc.service.ID,
		EngineType:   "smart",
		Status:       "pending",
		PriorityID:   bc.priority.ID,
		RequesterID:  user.ID,
		FormData:     JSONField(formJSON),
		WorkflowJSON: bc.service.WorkflowJSON,
	}
	if err := bc.db.Create(ticket).Error; err != nil {
		return fmt.Errorf("create ticket: %w", err)
	}
	bc.ticket = ticket
	return nil
}

// --- When steps ---

// whenCountersignRoleProcesss finds the parallel activity assigned to a specific position code
// and completes it via SmartEngine.Progress().
func (bc *bddContext) whenCountersignRoleProcesss(positionCode string) error {
	if bc.ticket == nil {
		return fmt.Errorf("no ticket in context")
	}

	// Find the parallel activity assigned to this position.
	var activities []TicketActivity
	bc.db.Where("ticket_id = ? AND activity_group_id != '' AND status IN ?",
		bc.ticket.ID, []string{"pending", "in_progress"}).
		Find(&activities)

	if len(activities) == 0 {
		return fmt.Errorf("no pending parallel activities found for ticket %d", bc.ticket.ID)
	}

	// Find the activity with an assignment matching the position code.
	var targetActivity *TicketActivity
	var targetAssignment TicketAssignment
	orgSvc := &testOrgService{db: bc.db}

	for i := range activities {
		var assignment TicketAssignment
		if err := bc.db.Where("activity_id = ?", activities[i].ID).First(&assignment).Error; err != nil {
			continue
		}

		// Match 1: direct PositionID match.
		if assignment.PositionID != nil {
			for code, pos := range bc.positions {
				if pos.ID == *assignment.PositionID && code == positionCode {
					targetActivity = &activities[i]
					targetAssignment = assignment
					break
				}
			}
		}

		// Match 2: assigned user belongs to the expected position (LLM sometimes
		// uses participant_id directly instead of position_department).
		if targetActivity == nil {
			var userID uint
			if assignment.AssigneeID != nil {
				userID = *assignment.AssigneeID
			} else if assignment.UserID != nil {
				userID = *assignment.UserID
			}
			if userID > 0 {
				for _, dept := range bc.departments {
					userIDs, _ := orgSvc.FindUsersByPositionAndDepartment(positionCode, dept.Code)
					for _, uid := range userIDs {
						if uid == userID {
							targetActivity = &activities[i]
							targetAssignment = assignment
							break
						}
					}
					if targetActivity != nil {
						break
					}
				}
			}
		}

		if targetActivity != nil {
			break
		}
	}

	if targetActivity == nil {
		// Diagnostic: log what assignments look like.
		for i := range activities {
			var assignment TicketAssignment
			if err := bc.db.Where("activity_id = ?", activities[i].ID).First(&assignment).Error; err != nil {
				log.Printf("[countersign-diag] activity %d (group=%s): no assignment: %v", activities[i].ID, activities[i].ActivityGroupID, err)
				continue
			}
			log.Printf("[countersign-diag] activity %d: participantType=%s userID=%v positionID=%v deptID=%v assigneeID=%v",
				activities[i].ID, assignment.ParticipantType, assignment.UserID, assignment.PositionID, assignment.DepartmentID, assignment.AssigneeID)
		}
		return fmt.Errorf("no parallel activity found for position %q", positionCode)
	}

	// Determine the operator (the user assigned to this position).
	var operatorID uint
	if targetAssignment.AssigneeID != nil {
		operatorID = *targetAssignment.AssigneeID
	} else if targetAssignment.UserID != nil {
		operatorID = *targetAssignment.UserID
	} else {
		// Resolve from org.
		orgSvc := &testOrgService{db: bc.db}
		if targetAssignment.PositionID != nil && targetAssignment.DepartmentID != nil {
			for pCode, p := range bc.positions {
				if p.ID == *targetAssignment.PositionID && pCode == positionCode {
					for dCode, d := range bc.departments {
						if d.ID == *targetAssignment.DepartmentID {
							userIDs, _ := orgSvc.FindUsersByPositionAndDepartment(pCode, dCode)
							if len(userIDs) > 0 {
								operatorID = userIDs[0]
							}
							break
						}
					}
					break
				}
			}
		}
	}

	if operatorID == 0 {
		return fmt.Errorf("could not determine operator for position %q", positionCode)
	}

	// Claim.
	bc.db.Model(&TicketAssignment{}).
		Where("activity_id = ?", targetActivity.ID).
		Updates(map[string]any{"assignee_id": operatorID, "status": "claimed"})

	// Progress — this triggers the convergence check in SmartEngine.Progress().
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	err := bc.smartEngine.Progress(ctx, bc.db, engine.ProgressParams{
		TicketID:   bc.ticket.ID,
		ActivityID: targetActivity.ID,
		Outcome:    "completed",
		OperatorID: operatorID,
	})
	if err != nil {
		bc.lastErr = err
		log.Printf("countersign progress for %s: %v", positionCode, err)
	}

	bc.db.First(bc.ticket, bc.ticket.ID)
	return nil
}

// --- Then steps ---

func (bc *bddContext) thenParallelGroupExists(expectedCount int) error {
	// Find activities with non-empty activity_group_id for this ticket.
	var activities []TicketActivity
	bc.db.Where("ticket_id = ? AND activity_group_id != ''", bc.ticket.ID).
		Find(&activities)

	if len(activities) == 0 {
		return fmt.Errorf("no parallel activities found for ticket %d", bc.ticket.ID)
	}

	// All should share the same group ID.
	groupID := activities[0].ActivityGroupID
	for _, a := range activities {
		if a.ActivityGroupID != groupID {
			return fmt.Errorf("activities have different group IDs: %q vs %q", groupID, a.ActivityGroupID)
		}
	}

	if len(activities) != expectedCount {
		return fmt.Errorf("expected %d parallel activities, got %d", expectedCount, len(activities))
	}

	return nil
}

func (bc *bddContext) thenParallelGroupNotConverged() error {
	// Check that there are still incomplete parallel activities.
	var pendingCount int64
	bc.db.Model(&TicketActivity{}).
		Where("ticket_id = ? AND activity_group_id != '' AND status NOT IN ?",
			bc.ticket.ID, []string{"completed", "cancelled"}).
		Count(&pendingCount)

	if pendingCount == 0 {
		return fmt.Errorf("expected pending parallel activities but all are completed")
	}

	// Verify no new non-parallel activities were created (no premature next step).
	var nonParallelPending int64
	bc.db.Model(&TicketActivity{}).
		Where("ticket_id = ? AND (activity_group_id = '' OR activity_group_id IS NULL) AND status IN ?",
			bc.ticket.ID, []string{"pending", "in_progress"}).
		Count(&nonParallelPending)

	if nonParallelPending > 0 {
		return fmt.Errorf("found %d non-parallel pending activities — premature next step triggered", nonParallelPending)
	}

	return nil
}

func (bc *bddContext) thenParallelGroupConverged() error {
	// All parallel activities should be completed.
	var pendingCount int64
	bc.db.Model(&TicketActivity{}).
		Where("ticket_id = ? AND activity_group_id != '' AND status NOT IN ?",
			bc.ticket.ID, []string{"completed", "cancelled"}).
		Count(&pendingCount)

	if pendingCount > 0 {
		return fmt.Errorf("expected all parallel activities completed, but %d still pending", pendingCount)
	}

	return nil
}

func (bc *bddContext) thenNoActivityForPosition(positionCode string) error {
	// Find position ID.
	pos, ok := bc.positions[positionCode]
	if !ok {
		return fmt.Errorf("position %q not in context", positionCode)
	}

	// Check no pending assignment for this position exists outside the parallel group.
	var count int64
	bc.db.Model(&TicketAssignment{}).
		Joins("JOIN itsm_ticket_activities ON itsm_ticket_activities.id = itsm_ticket_assignments.activity_id").
		Where("itsm_ticket_assignments.ticket_id = ? AND itsm_ticket_assignments.position_id = ? AND itsm_ticket_activities.status IN ? AND (itsm_ticket_activities.activity_group_id = '' OR itsm_ticket_activities.activity_group_id IS NULL)",
			bc.ticket.ID, pos.ID, []string{"pending", "in_progress"}).
		Count(&count)

	if count > 0 {
		return fmt.Errorf("found %d pending activities assigned to position %q — premature creation", count, positionCode)
	}

	return nil
}
