package engine

import (
	"fmt"
	"time"

	"gorm.io/gorm"
)

func (e *ClassicEngine) completeActivity(tx *gorm.DB, activity *activityModel, params ProgressParams, now time.Time) (bool, error) {
	updates := map[string]any{
		"status":             ActivityCompleted,
		"transition_outcome": params.Outcome,
		"finished_at":        now,
	}
	if params.Opinion != "" {
		updates["decision_reasoning"] = params.Opinion
	}
	if len(params.Result) > 0 {
		updates["form_data"] = string(params.Result)
	}
	if err := tx.Model(&activityModel{}).Where("id = ?", params.ActivityID).Updates(updates).Error; err != nil {
		return false, err
	}
	return true, nil
}

func (e *ClassicEngine) assignParticipants(tx *gorm.DB, ticketID, activityID, _ uint, participants []Participant) error {
	if len(participants) == 0 {
		e.recordTimeline(tx, ticketID, &activityID, 0, "warning", "节点未配置参与人，等待管理员手动指派")
		return nil
	}

	for i, p := range participants {
		userIDs, err := e.resolver.Resolve(tx, ticketID, p)
		if err != nil {
			e.recordTimeline(tx, ticketID, &activityID, 0, "warning", fmt.Sprintf("参与人解析失败: %v", err))
			continue
		}
		if len(userIDs) == 0 {
			e.recordTimeline(tx, ticketID, &activityID, 0, "warning", fmt.Sprintf("参与人解析结果为空: type=%s value=%s", p.Type, p.Value))
			continue
		}

		for _, uid := range userIDs {
			assignment := &assignmentModel{
				TicketID:        ticketID,
				ActivityID:      activityID,
				ParticipantType: p.Type,
				AssigneeID:      &uid,
				Status:          "pending",
				Sequence:        i,
				IsCurrent:       i == 0,
			}
			if p.Type == "user" || p.Type == "requester" {
				assignment.UserID = &uid
			}
			if err := tx.Create(assignment).Error; err != nil {
				return err
			}
		}

		if i == 0 {
			tx.Model(&ticketModel{}).Where("id = ?", ticketID).Update("assignee_id", userIDs[0])
		}
	}

	return nil
}

func (e *ClassicEngine) recordTimeline(tx *gorm.DB, ticketID uint, activityID *uint, operatorID uint, eventType, message string) error {
	return e.recordTimelineWithReasoning(tx, ticketID, activityID, operatorID, eventType, message, "")
}

func (e *ClassicEngine) recordTimelineWithReasoning(tx *gorm.DB, ticketID uint, activityID *uint, operatorID uint, eventType, message, reasoning string) error {
	tl := &timelineModel{
		TicketID:   ticketID,
		ActivityID: activityID,
		OperatorID: operatorID,
		EventType:  eventType,
		Message:    message,
		Reasoning:  reasoning,
	}
	return tx.Create(tl).Error
}
