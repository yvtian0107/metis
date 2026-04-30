package engine

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"gorm.io/gorm"

	"metis/internal/app"
)

// SLA status constants (mirror itsm package constants for engine-internal use).
const (
	slaOnTrack          = "on_track"
	slaBreachedResponse = "breached_response"
	slaBreachedResolve  = "breached_resolution"
)

var (
	ErrNotificationNoRecipients = errors.New("SLA notification has no resolved recipients")
	ErrNotificationNoEmail      = errors.New("SLA notification recipients have no email")
)

// escalationRuleModel is the engine-internal projection of itsm_escalation_rules.
type escalationRuleModel struct {
	ID           uint           `gorm:"primaryKey"`
	SLAID        uint           `gorm:"column:sla_id"`
	TriggerType  string         `gorm:"column:trigger_type"`
	Level        int            `gorm:"column:level"`
	WaitMinutes  int            `gorm:"column:wait_minutes"`
	ActionType   string         `gorm:"column:action_type"`
	TargetConfig string         `gorm:"column:target_config;type:text"`
	IsActive     bool           `gorm:"column:is_active"`
	DeletedAt    gorm.DeletedAt `gorm:"column:deleted_at;index"`
}

func (escalationRuleModel) TableName() string { return "itsm_escalation_rules" }

// SLAAssuranceConfigProvider exposes the configured SLA assurance post.
type SLAAssuranceConfigProvider interface {
	SLAAssuranceAgentID() uint
}

// HandleSLACheck is the cron task handler for itsm-sla-check.
// It scans active tickets with SLA deadlines, detects breaches,
// updates sla_status, and executes escalation rules.
func HandleSLACheck(db *gorm.DB, configProvider SLAAssuranceConfigProvider, executor app.AIDecisionExecutor, resolver *ParticipantResolver, notifier NotificationSender) func(ctx context.Context, payload json.RawMessage) error {
	return func(ctx context.Context, _ json.RawMessage) error {
		now := time.Now()

		// Find active tickets with SLA deadlines. Tickets already marked as response-breached
		// still need to be checked for resolution breach.
		var tickets []ticketModel
		err := db.Where("status IN ? AND sla_paused_at IS NULL AND "+
			"(sla_response_deadline IS NOT NULL OR sla_resolution_deadline IS NOT NULL)",
			[]string{
				TicketStatusSubmitted,
				TicketStatusWaitingHuman,
				TicketStatusApprovedDecisioning,
				TicketStatusRejectedDecisioning,
				TicketStatusDecisioning,
				TicketStatusExecutingAction,
			},
		).Find(&tickets).Error
		if err != nil {
			slog.Error("sla-check: failed to query tickets", "error", err)
			return err
		}

		if len(tickets) == 0 {
			return nil
		}

		slog.Info("sla-check: scanning tickets", "count", len(tickets))

		var ticketErrs []error
		for i := range tickets {
			t := &tickets[i]
			if err := checkTicketSLA(ctx, db, t, now, configProvider, executor, resolver, notifier); err != nil {
				wrapped := fmt.Errorf("ticket %d(%s): %w", t.ID, t.Code, err)
				slog.Error("sla-check: failed to check ticket", "ticketID", t.ID, "code", t.Code, "error", err)
				ticketErrs = append(ticketErrs, wrapped)
			}
		}

		return errors.Join(ticketErrs...)
	}
}

// checkTicketSLA checks a single ticket for SLA breaches and executes escalation.
func checkTicketSLA(ctx context.Context, db *gorm.DB, t *ticketModel, now time.Time, configProvider SLAAssuranceConfigProvider, executor app.AIDecisionExecutor, resolver *ParticipantResolver, notifier NotificationSender) error {
	responseBreached := t.SLAResponseDeadline != nil && now.After(*t.SLAResponseDeadline)
	resolutionBreached := t.SLAResolutionDeadline != nil && now.After(*t.SLAResolutionDeadline)

	if resolutionBreached {
		if t.SLAStatus != slaBreachedResolve {
			if err := recordSLABreach(db, t, slaBreachedResolve, "sla_resolution_breached", "SLA 解决时限已超时"); err != nil {
				return err
			}
			t.SLAStatus = slaBreachedResolve
			slog.Warn("sla-check: resolution SLA breached", "ticketID", t.ID, "code", t.Code,
				"deadline", t.SLAResolutionDeadline.Format(time.RFC3339))
		}
	} else if responseBreached {
		if t.SLAStatus != slaBreachedResponse {
			if err := recordSLABreach(db, t, slaBreachedResponse, "sla_response_breached", "SLA 响应时限已超时"); err != nil {
				return err
			}
			t.SLAStatus = slaBreachedResponse
			slog.Warn("sla-check: response SLA breached", "ticketID", t.ID, "code", t.Code,
				"deadline", t.SLAResponseDeadline.Format(time.RFC3339))
		}
	}

	if responseBreached {
		if err := executeEscalation(ctx, db, t, "response_timeout", now, configProvider, executor, resolver, notifier); err != nil {
			return err
		}
	}
	if resolutionBreached {
		if err := executeEscalation(ctx, db, t, "resolution_timeout", now, configProvider, executor, resolver, notifier); err != nil {
			return err
		}
	}
	return nil
}

// executeEscalation loads and executes matching escalation rules for a ticket's SLA breach.
func executeEscalation(ctx context.Context, db *gorm.DB, t *ticketModel, triggerType string, now time.Time, configProvider SLAAssuranceConfigProvider, executor app.AIDecisionExecutor, resolver *ParticipantResolver, notifier NotificationSender) error {
	rules, err := loadTicketEscalationRules(db, t.ID, triggerType)
	if err != nil {
		return err
	}
	if len(rules) == 0 {
		slog.Warn("sla-check: ticket has no SLA template, skipping escalation", "ticketID", t.ID)
		return nil
	}

	var deadline *time.Time
	if triggerType == "response_timeout" {
		deadline = t.SLAResponseDeadline
	} else {
		deadline = t.SLAResolutionDeadline
	}
	if deadline == nil {
		return nil
	}

	for _, rule := range rules {
		// Check if enough time has passed since breach for this escalation level
		triggerTime := deadline.Add(time.Duration(rule.WaitMinutes) * time.Minute)
		if now.Before(triggerTime) {
			continue // not yet time for this escalation level
		}
		if escalationAlreadyRecorded(db, t.ID, rule.ID) {
			continue
		}
		if t.EngineType == "classic" {
			reasoning := "系统计时器已判定 SLA 超时，经典引擎按已配置升级规则直接执行动作。"
			if err := executeEscalationAction(ctx, db, t, &rule, triggerType, 0, "系统计时器", reasoning, resolver, notifier); err != nil {
				slog.Warn("sla-check: classic escalation failed", "ticketID", t.ID, "ruleID", rule.ID, "error", err)
				recordEscalationPending(db, t, &rule, triggerType, 0, "系统计时器", fmt.Sprintf("经典引擎 SLA 升级动作执行失败：%v", err))
			}
			continue
		}

		agentID := uint(0)
		if configProvider != nil {
			agentID = configProvider.SLAAssuranceAgentID()
		}
		if agentID == 0 {
			recordEscalationPending(db, t, &rule, triggerType, 0, "", "系统计时器已判定 SLA 超时，但 SLA 保障岗未绑定智能体，未自动触发升级动作。")
			continue
		}
		if executor == nil {
			recordEscalationPending(db, t, &rule, triggerType, agentID, "SLA 保障岗", "SLA 保障岗已配置，但 AI 执行器不可用，升级动作待人工处理。")
			continue
		}
		agentName := loadAgentName(db, agentID)
		if err := runSLAAssuranceAgent(ctx, db, t, &rule, triggerType, agentID, agentName, executor, resolver, notifier); err != nil {
			slog.Warn("sla-check: SLA assurance agent failed", "ticketID", t.ID, "ruleID", rule.ID, "error", err)
			recordEscalationPending(db, t, &rule, triggerType, agentID, agentName, fmt.Sprintf("SLA 保障岗运行失败，升级动作待人工处理：%v", err))
		}
	}
	return nil
}

func loadTicketEscalationRules(db *gorm.DB, ticketID uint, triggerType string) ([]escalationRuleModel, error) {
	var row struct {
		ServiceVersionID    *uint
		SLAID               *uint
		EscalationRulesJSON string
	}
	if err := db.Table("itsm_tickets").
		Select("itsm_tickets.service_version_id, itsm_service_definition_versions.sla_id, itsm_service_definition_versions.escalation_rules_json").
		Joins("JOIN itsm_service_definition_versions ON itsm_service_definition_versions.id = itsm_tickets.service_version_id").
		Where("itsm_tickets.id = ?", ticketID).
		Scan(&row).Error; err != nil {
		return nil, err
	}
	if row.ServiceVersionID == nil || row.SLAID == nil || *row.SLAID == 0 || strings.TrimSpace(row.EscalationRulesJSON) == "" {
		return nil, nil
	}
	var snapshots []struct {
		ID           uint            `json:"id"`
		SLAID        uint            `json:"slaId"`
		TriggerType  string          `json:"triggerType"`
		Level        int             `json:"level"`
		WaitMinutes  int             `json:"waitMinutes"`
		ActionType   string          `json:"actionType"`
		TargetConfig json.RawMessage `json:"targetConfig"`
		IsActive     bool            `json:"isActive"`
	}
	if err := json.Unmarshal([]byte(row.EscalationRulesJSON), &snapshots); err != nil {
		return nil, err
	}
	rules := make([]escalationRuleModel, 0, len(snapshots))
	for _, snapshot := range snapshots {
		if !snapshot.IsActive || snapshot.TriggerType != triggerType {
			continue
		}
		rules = append(rules, escalationRuleModel{
			ID:           snapshot.ID,
			SLAID:        snapshot.SLAID,
			TriggerType:  snapshot.TriggerType,
			Level:        snapshot.Level,
			WaitMinutes:  snapshot.WaitMinutes,
			ActionType:   snapshot.ActionType,
			TargetConfig: string(snapshot.TargetConfig),
			IsActive:     snapshot.IsActive,
		})
	}
	return rules, nil
}

func loadAgentName(db *gorm.DB, agentID uint) string {
	var row struct{ Name string }
	if err := db.Table("ai_agents").Where("id = ? AND is_active = ?", agentID, true).Select("name").First(&row).Error; err != nil {
		return "SLA 保障岗"
	}
	return row.Name
}

func escalationAlreadyRecorded(db *gorm.DB, ticketID, ruleID uint) bool {
	var count int64
	pattern := "%\"rule_id\":" + strconv.FormatUint(uint64(ruleID), 10) + "%"
	db.Table("itsm_ticket_timelines").
		Where("ticket_id = ? AND event_type IN ? AND details LIKE ?",
			ticketID,
			[]string{"sla_escalation", "sla_assurance_pending"},
			pattern,
		).Count(&count)
	return count > 0
}

// escalationTargetConfig holds parsed target_config JSON for escalation rules.
type escalationTargetConfig struct {
	Recipients         []Participant `json:"recipients,omitempty"`
	AssigneeCandidates []Participant `json:"assigneeCandidates,omitempty"`
	ChannelID          uint          `json:"channelId,omitempty"`
	SubjectTemplate    string        `json:"subjectTemplate,omitempty"`
	BodyTemplate       string        `json:"bodyTemplate,omitempty"`
	PriorityID         uint          `json:"priorityId,omitempty"`
}

func runSLAAssuranceAgent(ctx context.Context, db *gorm.DB, t *ticketModel, rule *escalationRuleModel, triggerType string, agentID uint, agentName string, executor app.AIDecisionExecutor, resolver *ParticipantResolver, notifier NotificationSender) error {
	triggered := false
	toolHandler := func(name string, args json.RawMessage) (json.RawMessage, error) {
		switch name {
		case "sla.risk_queue":
			return marshalSLAToolResult([]map[string]any{slaRiskQueueItem(t, rule, triggerType)})
		case "sla.ticket_context":
			if err := validateSLAToolTicket(args, t.ID); err != nil {
				return nil, err
			}
			return marshalSLAToolResult(loadSLATicketContext(db, t.ID))
		case "sla.escalation_rules":
			if err := validateSLAToolTicket(args, t.ID); err != nil {
				return nil, err
			}
			return marshalSLAToolResult([]map[string]any{slaRulePayload(rule)})
		case "sla.trigger_escalation":
			var req struct {
				TicketID  uint   `json:"ticket_id"`
				RuleID    uint   `json:"rule_id"`
				Reasoning string `json:"reasoning"`
			}
			if err := json.Unmarshal(args, &req); err != nil {
				return nil, fmt.Errorf("invalid trigger args: %w", err)
			}
			if req.TicketID != t.ID || req.RuleID != rule.ID {
				return nil, fmt.Errorf("只允许触发当前候选工单和已命中规则")
			}
			reasoning := req.Reasoning
			if reasoning == "" {
				reasoning = "SLA 保障岗确认规则已命中，按已配置升级动作执行。"
			}
			if err := executeEscalationAction(ctx, db, t, rule, triggerType, agentID, agentName, reasoning, resolver, notifier); err != nil {
				return nil, err
			}
			triggered = true
			return marshalSLAToolResult(map[string]any{"ok": true, "triggered": true, "rule_id": rule.ID})
		case "sla.write_timeline":
			var req struct {
				TicketID  uint   `json:"ticket_id"`
				Message   string `json:"message"`
				Reasoning string `json:"reasoning"`
			}
			if err := json.Unmarshal(args, &req); err != nil {
				return nil, fmt.Errorf("invalid timeline args: %w", err)
			}
			if req.TicketID != t.ID {
				return nil, fmt.Errorf("只允许写入当前候选工单时间线")
			}
			if req.Message == "" {
				req.Message = "SLA 保障岗已记录处理观察"
			}
			db.Create(&timelineModel{
				TicketID:   t.ID,
				OperatorID: 0,
				EventType:  "sla_assurance_note",
				Message:    req.Message,
				Details:    escalationTimelineDetails(rule, triggerType, agentID, agentName),
				Reasoning:  req.Reasoning,
			})
			return marshalSLAToolResult(map[string]any{"ok": true})
		default:
			return nil, fmt.Errorf("未知 SLA 工具: %s", name)
		}
	}

	resp, err := executor.Execute(ctx, agentID, app.AIDecisionRequest{
		SystemPrompt: buildSLAAssuranceSystemPrompt(),
		UserMessage:  buildSLAAssuranceUserMessage(t, rule, triggerType),
		Tools:        slaAssuranceToolDefs(),
		ToolHandler:  toolHandler,
		MaxTurns:     8,
	})
	if err != nil {
		return err
	}
	if !triggered {
		content := ""
		if resp != nil {
			content = resp.Content
		}
		return fmt.Errorf("SLA 保障岗未触发升级动作: %s", trimForTimeline(content, 240))
	}
	return nil
}

// executeEscalationAction executes a single escalation rule action.
func executeEscalationAction(ctx context.Context, db *gorm.DB, t *ticketModel, rule *escalationRuleModel, triggerType string, agentID uint, agentName string, reasoning string, resolver *ParticipantResolver, notifier NotificationSender) error {
	var config escalationTargetConfig
	if rule.TargetConfig != "" {
		if err := json.Unmarshal([]byte(rule.TargetConfig), &config); err != nil {
			return fmt.Errorf("invalid SLA escalation target config: %w", err)
		}
	}

	switch rule.ActionType {
	case "notify":
		return executeNotifyEscalation(ctx, db, t, rule, triggerType, agentID, agentName, reasoning, config, resolver, notifier)

	case "reassign":
		candidateIDs, warnings := resolveConfiguredParticipants(db, resolver, t.ID, config.AssigneeCandidates)
		if len(candidateIDs) == 0 {
			recordEscalationResult(db, t, rule, triggerType, agentID, agentName, "SLA 升级：转派目标解析为空", reasoning, map[string]any{
				"warnings": warnings,
			})
			return nil
		}
		newAssignee := candidateIDs[0]
		if t.CurrentActivityID == nil || *t.CurrentActivityID == 0 {
			recordEscalationResult(db, t, rule, triggerType, agentID, agentName, "SLA 升级转派失败：工单没有当前活动", reasoning, map[string]any{
				"candidate_user_ids": candidateIDs,
				"selected_user_id":   newAssignee,
				"warnings":           warnings,
			})
			return nil
		}
		if err := reassignCurrentActivity(db, t, newAssignee); err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				recordEscalationResult(db, t, rule, triggerType, agentID, agentName, "SLA 升级转派失败：当前任务无可改派处理人", reasoning, map[string]any{
					"activity_id":        *t.CurrentActivityID,
					"candidate_user_ids": candidateIDs,
					"selected_user_id":   newAssignee,
					"warnings":           warnings,
				})
				return nil
			}
			return err
		}
		slog.Info("sla-check: escalation reassign", "ticketID", t.ID, "level", rule.Level, "newAssignee", newAssignee)
		recordEscalationResult(db, t, rule, triggerType, agentID, agentName, "SLA 升级：工单已转派", reasoning, map[string]any{
			"candidate_user_ids": candidateIDs,
			"selected_user_id":   newAssignee,
			"warnings":           warnings,
		})
		return nil

	case "escalate_priority":
		if config.PriorityID == 0 {
			recordEscalationResult(db, t, rule, triggerType, agentID, agentName, "SLA 升级：目标优先级未配置", reasoning, nil)
			return nil
		}
		var priority struct {
			ID   uint
			Name string
			Code string
		}
		if err := db.Table("itsm_priorities").
			Where("id = ? AND is_active = ?", config.PriorityID, true).
			Select("id, name, code").
			First(&priority).Error; err != nil {
			recordEscalationResult(db, t, rule, triggerType, agentID, agentName, "SLA 升级：目标优先级不存在或已停用", reasoning, map[string]any{"priority_id": config.PriorityID})
			return nil
		}
		if err := db.Model(&ticketModel{}).Where("id = ?", t.ID).Update("priority_id", config.PriorityID).Error; err != nil {
			return err
		}
		slog.Info("sla-check: escalation priority", "ticketID", t.ID, "level", rule.Level, "newPriority", config.PriorityID)
		recordEscalationResult(db, t, rule, triggerType, agentID, agentName, "SLA 升级：工单优先级已提升", reasoning, map[string]any{
			"priority_id":   priority.ID,
			"priority_name": priority.Name,
			"priority_code": priority.Code,
		})
		return nil

	default:
		slog.Warn("sla-check: unknown escalation action", "actionType", rule.ActionType, "ticketID", t.ID)
		return fmt.Errorf("未知 SLA 升级动作: %s", rule.ActionType)
	}
}

func reassignCurrentActivity(db *gorm.DB, t *ticketModel, newAssignee uint) error {
	return db.Transaction(func(tx *gorm.DB) error {
		updates := map[string]any{
			"user_id":          newAssignee,
			"assignee_id":      newAssignee,
			"participant_type": "user",
			"position_id":      nil,
			"department_id":    nil,
		}
		result := tx.Model(&assignmentModel{}).
			Where("ticket_id = ? AND activity_id = ? AND status = ? AND is_current = ?",
				t.ID, *t.CurrentActivityID, "pending", true).
			Updates(updates)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return gorm.ErrRecordNotFound
		}
		return tx.Model(&ticketModel{}).Where("id = ?", t.ID).Update("assignee_id", newAssignee).Error
	})
}

func recordEscalationPending(db *gorm.DB, t *ticketModel, rule *escalationRuleModel, triggerType string, agentID uint, agentName string, reasoning string) {
	db.Create(&timelineModel{
		TicketID:   t.ID,
		OperatorID: 0,
		EventType:  "sla_assurance_pending",
		Message:    "SLA 保障岗未上岗，升级动作待人工处理",
		Details:    escalationTimelineDetails(rule, triggerType, agentID, agentName),
		Reasoning:  reasoning,
	})
}

func executeNotifyEscalation(ctx context.Context, db *gorm.DB, t *ticketModel, rule *escalationRuleModel, triggerType string, agentID uint, agentName string, reasoning string, config escalationTargetConfig, resolver *ParticipantResolver, notifier NotificationSender) error {
	recipientIDs, warnings := resolveConfiguredParticipants(db, resolver, t.ID, config.Recipients)
	if len(recipientIDs) == 0 {
		recordEscalationResult(db, t, rule, triggerType, agentID, agentName, "SLA 升级通知未发送：未解析到接收人", reasoning, map[string]any{
			"warnings": warnings,
		})
		return nil
	}
	if notifier == nil {
		recordEscalationResult(db, t, rule, triggerType, agentID, agentName, "SLA 升级通知未发送：消息通道不可用", reasoning, map[string]any{
			"recipient_user_ids": recipientIDs,
			"channel_id":         config.ChannelID,
			"warnings":           warnings,
		})
		return nil
	}

	subject := renderSLANotificationTemplate(config.SubjectTemplate, t)
	if strings.TrimSpace(subject) == "" {
		subject = renderSLANotificationTemplate("SLA 升级通知：{{ticket.code}}", t)
	}
	body := renderSLANotificationTemplate(config.BodyTemplate, t)
	if strings.TrimSpace(body) == "" {
		body = renderSLANotificationTemplate("工单 {{ticket.code}} 已触发 SLA 升级规则，请及时处理。", t)
	}

	err := notifier.Send(ctx, config.ChannelID, subject, body, recipientIDs)
	extra := map[string]any{
		"recipient_user_ids": recipientIDs,
		"channel_id":         config.ChannelID,
		"warnings":           warnings,
	}
	if err != nil {
		extra["send_error"] = err.Error()
		recordEscalationResult(db, t, rule, triggerType, agentID, agentName, "SLA 升级通知发送失败", reasoning, extra)
		return nil
	}
	slog.Info("sla-check: escalation notify sent", "ticketID", t.ID, "level", rule.Level, "channelID", config.ChannelID, "recipientUsers", recipientIDs)
	recordEscalationResult(db, t, rule, triggerType, agentID, agentName, "SLA 升级通知已发送", reasoning, extra)
	return nil
}

func resolveConfiguredParticipants(db *gorm.DB, resolver *ParticipantResolver, ticketID uint, participants []Participant) ([]uint, []string) {
	if len(participants) == 0 {
		return nil, []string{"未配置参与者表达式"}
	}
	if resolver == nil {
		return nil, []string{"参与者解析器不可用"}
	}
	userIDs := make([]uint, 0)
	seen := map[uint]struct{}{}
	warnings := make([]string, 0)
	for _, participant := range participants {
		ids, err := resolver.Resolve(db, ticketID, participant)
		if err != nil {
			warnings = append(warnings, err.Error())
			continue
		}
		if len(ids) == 0 {
			warnings = append(warnings, fmt.Sprintf("参与者解析结果为空: type=%s value=%s", participant.Type, participant.Value))
			continue
		}
		for _, id := range ids {
			if _, ok := seen[id]; ok {
				continue
			}
			seen[id] = struct{}{}
			userIDs = append(userIDs, id)
		}
	}
	return userIDs, warnings
}

func renderSLANotificationTemplate(template string, t *ticketModel) string {
	result := template
	replacements := map[string]string{
		"{{ticket.id}}":          strconv.FormatUint(uint64(t.ID), 10),
		"{{ticket.code}}":        t.Code,
		"{{ticket.title}}":       t.Title,
		"{{ticket.status}}":      t.Status,
		"{{ticket.priority_id}}": strconv.FormatUint(uint64(t.PriorityID), 10),
		"{{ticket.sla_status}}":  t.SLAStatus,
	}
	for key, value := range replacements {
		result = strings.ReplaceAll(result, key, value)
	}
	return result
}

func recordSLABreach(db *gorm.DB, t *ticketModel, status string, eventType string, message string) error {
	deadline := ""
	if eventType == "sla_response_breached" && t.SLAResponseDeadline != nil {
		deadline = t.SLAResponseDeadline.Format(time.RFC3339)
	}
	if eventType == "sla_resolution_breached" && t.SLAResolutionDeadline != nil {
		deadline = t.SLAResolutionDeadline.Format(time.RFC3339)
	}
	details, err := json.Marshal(map[string]any{
		"ticket_id":           t.ID,
		"ticket_code":         t.Code,
		"previous_sla_status": t.SLAStatus,
		"sla_status":          status,
		"deadline":            deadline,
	})
	if err != nil {
		return err
	}
	return db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&ticketModel{}).Where("id = ?", t.ID).Update("sla_status", status).Error; err != nil {
			return err
		}
		return tx.Create(&timelineModel{
			TicketID:   t.ID,
			OperatorID: 0,
			EventType:  eventType,
			Message:    message,
			Details:    string(details),
		}).Error
	})
}

func recordEscalationResult(db *gorm.DB, t *ticketModel, rule *escalationRuleModel, triggerType string, agentID uint, agentName string, message string, reasoning string, extra map[string]any) {
	db.Create(&timelineModel{
		TicketID:   t.ID,
		OperatorID: 0,
		EventType:  "sla_escalation",
		Message:    message,
		Details:    escalationTimelineDetails(rule, triggerType, agentID, agentName, extra),
		Reasoning:  reasoning,
	})
}

func escalationTimelineDetails(rule *escalationRuleModel, triggerType string, agentID uint, agentName string, extras ...map[string]any) string {
	payload := map[string]any{
		"rule_id":      rule.ID,
		"sla_id":       rule.SLAID,
		"trigger_type": triggerType,
		"level":        rule.Level,
		"action_type":  rule.ActionType,
		"agent_id":     agentID,
		"agent_name":   agentName,
	}
	for _, extra := range extras {
		for k, v := range extra {
			payload[k] = v
		}
	}
	raw, _ := json.Marshal(payload)
	return string(raw)
}

func slaAssuranceToolDefs() []app.AIToolDef {
	return []app.AIToolDef{
		{
			Name:        "sla.risk_queue",
			Description: "读取当前 SLA 风险/超时候选队列。当前执行上下文只返回系统计时器已确认命中规则的候选。",
			Parameters:  jsonSchema(`{"type":"object","properties":{"status":{"type":"string"}}}`),
		},
		{
			Name:        "sla.ticket_context",
			Description: "读取当前候选工单上下文，包括服务、状态、SLA 状态、截止时间、当前处理人。",
			Parameters:  jsonSchema(`{"type":"object","properties":{"ticket_id":{"type":"integer"}},"required":["ticket_id"]}`),
		},
		{
			Name:        "sla.escalation_rules",
			Description: "读取当前候选工单已命中的 SLA 升级规则。",
			Parameters:  jsonSchema(`{"type":"object","properties":{"ticket_id":{"type":"integer"},"trigger_type":{"type":"string"}},"required":["ticket_id"]}`),
		},
		{
			Name:        "sla.trigger_escalation",
			Description: "在规则允许范围内触发升级动作。只能传入当前候选工单和已命中规则，并必须给出 reasoning。",
			Parameters:  jsonSchema(`{"type":"object","properties":{"ticket_id":{"type":"integer"},"rule_id":{"type":"integer"},"reasoning":{"type":"string"}},"required":["ticket_id","rule_id","reasoning"]}`),
		},
		{
			Name:        "sla.write_timeline",
			Description: "写入 SLA 保障岗观察或处理备注。触发升级动作时仍必须调用 sla.trigger_escalation。",
			Parameters:  jsonSchema(`{"type":"object","properties":{"ticket_id":{"type":"integer"},"message":{"type":"string"},"reasoning":{"type":"string"}},"required":["ticket_id","message"]}`),
		},
	}
}

func buildSLAAssuranceSystemPrompt() string {
	return `你是 ITSM 的 SLA 保障岗。
系统计时器是 SLA 风险和超时事实的唯一来源，你不能主观判定是否超时。
你的职责是读取当前候选工单、读取已命中的升级规则，并在规则允许范围内触发通知、改派、提优先级等动作。
必须先理解工单上下文和升级规则；如果规则已命中，应调用 sla.trigger_escalation，并在 reasoning 中写明触发规则、动作类型、目标对象和判断依据。
不得触发当前候选以外的工单或规则。`
}

func buildSLAAssuranceUserMessage(t *ticketModel, rule *escalationRuleModel, triggerType string) string {
	payload := map[string]any{
		"ticket_id":    t.ID,
		"ticket_code":  t.Code,
		"title":        t.Title,
		"trigger_type": triggerType,
		"matched_rule": slaRulePayload(rule),
		"instruction":  "请严格按顺序处理：先调用 sla.risk_queue 确认候选队列，再调用 sla.ticket_context 读取当前工单上下文，然后调用 sla.escalation_rules 读取已命中规则；规则允许时必须调用 sla.trigger_escalation 触发升级动作。",
	}
	raw, _ := json.Marshal(payload)
	return string(raw)
}

func validateSLAToolTicket(args json.RawMessage, ticketID uint) error {
	var req struct {
		TicketID uint `json:"ticket_id"`
	}
	if len(args) > 0 {
		if err := json.Unmarshal(args, &req); err != nil {
			return fmt.Errorf("invalid ticket args: %w", err)
		}
	}
	if req.TicketID != 0 && req.TicketID != ticketID {
		return fmt.Errorf("只允许读取当前候选工单")
	}
	return nil
}

func marshalSLAToolResult(v any) (json.RawMessage, error) {
	raw, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	return raw, nil
}

func jsonSchema(raw string) any {
	var schema any
	_ = json.Unmarshal([]byte(raw), &schema)
	return schema
}

func slaRiskQueueItem(t *ticketModel, rule *escalationRuleModel, triggerType string) map[string]any {
	return map[string]any{
		"ticket_id":    t.ID,
		"ticket_code":  t.Code,
		"title":        t.Title,
		"sla_status":   t.SLAStatus,
		"trigger_type": triggerType,
		"rule_id":      rule.ID,
		"action_type":  rule.ActionType,
	}
}

func slaRulePayload(rule *escalationRuleModel) map[string]any {
	var target any
	if rule.TargetConfig != "" {
		_ = json.Unmarshal([]byte(rule.TargetConfig), &target)
	}
	return map[string]any{
		"rule_id":       rule.ID,
		"sla_id":        rule.SLAID,
		"trigger_type":  rule.TriggerType,
		"level":         rule.Level,
		"wait_minutes":  rule.WaitMinutes,
		"action_type":   rule.ActionType,
		"target_config": target,
	}
}

func loadSLATicketContext(db *gorm.DB, ticketID uint) map[string]any {
	var row struct {
		ID                    uint
		Code                  string
		Title                 string
		Status                string
		SLAStatus             string
		SLAResponseDeadline   *time.Time
		SLAResolutionDeadline *time.Time
		ServiceName           string
		RequesterName         string
		AssigneeName          string
		PriorityName          string
	}
	_ = db.Table("itsm_tickets").
		Select(`itsm_tickets.id, itsm_tickets.code, itsm_tickets.title, itsm_tickets.status,
			itsm_tickets.sla_status, itsm_tickets.sla_response_deadline, itsm_tickets.sla_resolution_deadline,
			itsm_service_definitions.name AS service_name,
			requesters.username AS requester_name,
			assignees.username AS assignee_name,
			itsm_priorities.name AS priority_name`).
		Joins("LEFT JOIN itsm_service_definitions ON itsm_service_definitions.id = itsm_tickets.service_id").
		Joins("LEFT JOIN users AS requesters ON requesters.id = itsm_tickets.requester_id").
		Joins("LEFT JOIN users AS assignees ON assignees.id = itsm_tickets.assignee_id").
		Joins("LEFT JOIN itsm_priorities ON itsm_priorities.id = itsm_tickets.priority_id").
		Where("itsm_tickets.id = ?", ticketID).
		Scan(&row).Error
	return map[string]any{
		"ticket_id":               row.ID,
		"ticket_code":             row.Code,
		"title":                   row.Title,
		"status":                  row.Status,
		"sla_status":              row.SLAStatus,
		"sla_response_deadline":   row.SLAResponseDeadline,
		"sla_resolution_deadline": row.SLAResolutionDeadline,
		"service_name":            row.ServiceName,
		"requester_name":          row.RequesterName,
		"assignee_name":           row.AssigneeName,
		"priority_name":           row.PriorityName,
	}
}

func trimForTimeline(s string, max int) string {
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}
	return string(runes[:max])
}
