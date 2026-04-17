package engine

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"gorm.io/gorm"
)

// SLA status constants (mirror itsm package constants for engine-internal use).
const (
	slaOnTrack          = "on_track"
	slaBreachedResponse = "breached_response"
	slaBreachedResolve  = "breached_resolution"
)

// escalationRuleModel is the engine-internal projection of itsm_escalation_rules.
type escalationRuleModel struct {
	ID           uint   `gorm:"primaryKey"`
	SLAID        uint   `gorm:"column:sla_id"`
	TriggerType  string `gorm:"column:trigger_type"`
	Level        int    `gorm:"column:level"`
	WaitMinutes  int    `gorm:"column:wait_minutes"`
	ActionType   string `gorm:"column:action_type"`
	TargetConfig string `gorm:"column:target_config;type:text"`
	IsActive     bool   `gorm:"column:is_active"`
}

func (escalationRuleModel) TableName() string { return "itsm_escalation_rules" }

// HandleSLACheck is the cron task handler for itsm-sla-check.
// It scans active tickets with SLA deadlines, detects breaches,
// updates sla_status, and executes escalation rules.
func HandleSLACheck(db *gorm.DB) func(ctx context.Context, payload json.RawMessage) error {
	return func(ctx context.Context, _ json.RawMessage) error {
		now := time.Now()

		// Find tickets that are in_progress, not paused, have SLA deadlines, and not in terminal SLA state
		var tickets []ticketModel
		err := db.Where("status IN ? AND sla_paused_at IS NULL AND sla_status = ? AND "+
			"(sla_response_deadline IS NOT NULL OR sla_resolution_deadline IS NOT NULL)",
			[]string{"pending", "in_progress", "waiting_approval", "waiting_action"},
			slaOnTrack,
		).Find(&tickets).Error
		if err != nil {
			slog.Error("sla-check: failed to query tickets", "error", err)
			return err
		}

		if len(tickets) == 0 {
			return nil
		}

		slog.Info("sla-check: scanning tickets", "count", len(tickets))

		for i := range tickets {
			t := &tickets[i]
			checkTicketSLA(db, t, now)
		}

		return nil
	}
}

// checkTicketSLA checks a single ticket for SLA breaches and executes escalation.
func checkTicketSLA(db *gorm.DB, t *ticketModel, now time.Time) {
	// Check response deadline first (higher severity)
	if t.SLAResponseDeadline != nil && now.After(*t.SLAResponseDeadline) {
		if t.SLAStatus != slaBreachedResponse && t.SLAStatus != slaBreachedResolve {
			db.Model(&ticketModel{}).Where("id = ?", t.ID).Update("sla_status", slaBreachedResponse)
			slog.Warn("sla-check: response SLA breached", "ticketID", t.ID, "code", t.Code,
				"deadline", t.SLAResponseDeadline.Format(time.RFC3339))
			executeEscalation(db, t, "response_timeout", now)
		}
		return
	}

	// Check resolution deadline
	if t.SLAResolutionDeadline != nil && now.After(*t.SLAResolutionDeadline) {
		if t.SLAStatus != slaBreachedResolve {
			db.Model(&ticketModel{}).Where("id = ?", t.ID).Update("sla_status", slaBreachedResolve)
			slog.Warn("sla-check: resolution SLA breached", "ticketID", t.ID, "code", t.Code,
				"deadline", t.SLAResolutionDeadline.Format(time.RFC3339))
			executeEscalation(db, t, "resolution_timeout", now)
		}
	}
}

// executeEscalation loads and executes matching escalation rules for a ticket's SLA breach.
func executeEscalation(db *gorm.DB, t *ticketModel, triggerType string, now time.Time) {
	// Load escalation rules for the ticket's priority SLA
	// Link: priority → sla_template → escalation_rules
	// For now, we load rules where trigger_type matches and wait_minutes has elapsed since the deadline
	var rules []escalationRuleModel
	err := db.Where("trigger_type = ? AND is_active = ?", triggerType, true).
		Order("level ASC").
		Find(&rules).Error
	if err != nil {
		slog.Error("sla-check: failed to load escalation rules", "error", err, "ticketID", t.ID)
		return
	}

	var deadline *time.Time
	if triggerType == "response_timeout" {
		deadline = t.SLAResponseDeadline
	} else {
		deadline = t.SLAResolutionDeadline
	}
	if deadline == nil {
		return
	}

	for _, rule := range rules {
		// Check if enough time has passed since breach for this escalation level
		triggerTime := deadline.Add(time.Duration(rule.WaitMinutes) * time.Minute)
		if now.Before(triggerTime) {
			continue // not yet time for this escalation level
		}

		executeEscalationAction(db, t, &rule)
	}
}

// escalationTargetConfig holds parsed target_config JSON for escalation rules.
type escalationTargetConfig struct {
	UserIDs    []uint `json:"user_ids,omitempty"`
	PriorityID *uint  `json:"priority_id,omitempty"`
}

// executeEscalationAction executes a single escalation rule action.
func executeEscalationAction(db *gorm.DB, t *ticketModel, rule *escalationRuleModel) {
	var config escalationTargetConfig
	if rule.TargetConfig != "" {
		json.Unmarshal([]byte(rule.TargetConfig), &config)
	}

	switch rule.ActionType {
	case "notify":
		// Record timeline event for notification
		slog.Info("sla-check: escalation notify", "ticketID", t.ID, "level", rule.Level, "targetUsers", config.UserIDs)
		db.Create(&timelineModel{
			TicketID:   t.ID,
			OperatorID: 0,
			EventType:  "sla_escalation",
			Message:    "SLA 升级通知已触发",
		})

	case "reassign":
		if len(config.UserIDs) > 0 {
			newAssignee := config.UserIDs[0]
			db.Model(&ticketModel{}).Where("id = ?", t.ID).Update("assignee_id", newAssignee)
			slog.Info("sla-check: escalation reassign", "ticketID", t.ID, "level", rule.Level, "newAssignee", newAssignee)
			db.Create(&timelineModel{
				TicketID:   t.ID,
				OperatorID: 0,
				EventType:  "sla_escalation",
				Message:    "SLA 升级：工单已转派",
			})
		}

	case "escalate_priority":
		if config.PriorityID != nil {
			db.Model(&ticketModel{}).Where("id = ?", t.ID).Update("priority_id", *config.PriorityID)
			slog.Info("sla-check: escalation priority", "ticketID", t.ID, "level", rule.Level, "newPriority", *config.PriorityID)
			db.Create(&timelineModel{
				TicketID:   t.ID,
				OperatorID: 0,
				EventType:  "sla_escalation",
				Message:    "SLA 升级：工单优先级已提升",
			})
		}

	default:
		slog.Warn("sla-check: unknown escalation action", "actionType", rule.ActionType, "ticketID", t.ID)
	}
}
