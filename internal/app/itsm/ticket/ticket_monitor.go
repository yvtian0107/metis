package ticket

import (
	. "metis/internal/app/itsm/domain"
	"slices"
	"time"

	"metis/internal/app/itsm/engine"
)

type ticketMonitorFact struct {
	Activity              *TicketActivity
	AssignmentCount       int
	ResolvableAssignments int
	OwnerName             string
	ActionFailed          bool
}

func (s *TicketService) loadMonitorFacts(tickets []Ticket) (map[uint]ticketMonitorFact, error) {
	facts := make(map[uint]ticketMonitorFact, len(tickets))
	activityIDs := map[uint]struct{}{}
	for i := range tickets {
		t := &tickets[i]
		facts[t.ID] = ticketMonitorFact{}
		if t.CurrentActivityID != nil {
			activityIDs[*t.CurrentActivityID] = struct{}{}
		}
	}
	if len(activityIDs) == 0 {
		return facts, nil
	}

	var activities []TicketActivity
	if err := s.ticketRepo.DB().Where("id IN ?", keysOf(activityIDs)).Find(&activities).Error; err != nil {
		return nil, err
	}
	activityByID := map[uint]*TicketActivity{}
	for i := range activities {
		activityByID[activities[i].ID] = &activities[i]
	}

	assignments, err := s.loadAssignmentDisplays(activityIDs, 0)
	if err != nil {
		return nil, err
	}

	var failedRows []struct {
		TicketID   uint
		ActivityID uint
	}
	if err := s.ticketRepo.DB().Model(&TicketActionExecution{}).
		Where("activity_id IN ? AND status = ?", keysOf(activityIDs), "failed").
		Select("ticket_id", "activity_id").
		Scan(&failedRows).Error; err != nil {
		return nil, err
	}
	failedByActivity := map[uint]bool{}
	for _, row := range failedRows {
		failedByActivity[row.ActivityID] = true
	}

	var assignmentRows []TicketAssignment
	if err := s.ticketRepo.DB().Where("activity_id IN ? AND status = ?", keysOf(activityIDs), AssignmentPending).Find(&assignmentRows).Error; err != nil {
		return nil, err
	}
	assignmentCount := map[uint]int{}
	resolvableCount := map[uint]int{}
	for _, a := range assignmentRows {
		assignmentCount[a.ActivityID]++
		if a.AssigneeID != nil || a.UserID != nil || a.PositionID != nil || a.DepartmentID != nil {
			resolvableCount[a.ActivityID]++
		}
	}

	for i := range tickets {
		t := &tickets[i]
		fact := facts[t.ID]
		if t.CurrentActivityID != nil {
			if activity := activityByID[*t.CurrentActivityID]; activity != nil {
				fact.Activity = activity
				fact.ActionFailed = failedByActivity[activity.ID]
				fact.AssignmentCount = assignmentCount[activity.ID]
				fact.ResolvableAssignments = resolvableCount[activity.ID]
				if assignment, ok := assignments[activity.ID]; ok {
					fact.OwnerName = assignment.OwnerName
				}
			}
		}
		facts[t.ID] = fact
	}
	return facts, nil
}

func (s *TicketService) populateMonitorItem(item *TicketMonitorItem, ticket *Ticket, fact ticketMonitorFact, now time.Time) {
	if fact.Activity != nil {
		item.CurrentActivityName = fact.Activity.Name
		item.CurrentActivityType = fact.Activity.ActivityType
		startedAt := activityStartedAt(fact.Activity)
		item.CurrentActivityStartedAt = &startedAt
		item.WaitingMinutes = elapsedMinutes(now, startedAt)
		if item.NextStepSummary == "" {
			item.NextStepSummary = fact.Activity.Name
		}
		if item.CurrentOwnerName == "" && fact.OwnerName != "" {
			item.CurrentOwnerName = fact.OwnerName
		}
		if item.CurrentOwnerName == "" && ticket.AssigneeID != nil {
			item.CurrentOwnerName = item.AssigneeName
		}
	} else if isActiveTicket(ticket.Status) {
		item.WaitingMinutes = elapsedMinutes(now, ticket.UpdatedAt)
	}

	if !isActiveTicket(ticket.Status) {
		item.RiskLevel = "normal"
		item.Stuck = false
		return
	}

	reasons := EvaluateTicketMonitorRules(ticket, fact, now)
	item.MonitorReasons = reasons
	item.StuckReasons = monitorReasonMessages(reasons)
	if monitorReasonsContainSeverity(reasons, "blocked") {
		item.RiskLevel = "blocked"
		item.Stuck = true
		return
	}
	if monitorReasonsContainSeverity(reasons, "risk") {
		item.RiskLevel = "risk"
		item.Stuck = false
		return
	}
	item.RiskLevel = "normal"
	item.Stuck = false
	item.StuckReasons = nil
}

func (s *TicketService) accumulateMonitorSummary(summary *TicketMonitorSummary, ticket *Ticket, item *TicketMonitorItem, now time.Time) {
	if isActiveTicket(ticket.Status) {
		summary.ActiveTotal++
		if ticket.EngineType == "smart" {
			summary.SmartActiveTotal++
		} else {
			summary.ClassicActiveTotal++
		}
		switch item.RiskLevel {
		case "blocked":
			summary.StuckTotal++
		case "risk":
			summary.RiskTotal++
		}
		if monitorItemHasMetric(item, "ai_incident_total") {
			summary.AIIncidentTotal++
		}
		if monitorItemHasMetric(item, "sla_risk_total") {
			summary.SLARiskTotal++
		}
	}
	if ticket.Status == TicketStatusCompleted && ticket.FinishedAt != nil && sameLocalDay(*ticket.FinishedAt, now) {
		summary.CompletedTodayTotal++
	}
}

func monitorHasSLARisk(ticket *Ticket, now time.Time) bool {
	reasons := EvaluateTicketMonitorRules(ticket, ticketMonitorFact{}, now)
	return slices.ContainsFunc(reasons, func(reason TicketMonitorReason) bool {
		return reason.MetricCode == "sla_risk_total"
	})
}

func EvaluateTicketMonitorRules(ticket *Ticket, fact ticketMonitorFact, now time.Time) []TicketMonitorReason {
	if ticket == nil || !isActiveTicket(ticket.Status) {
		return nil
	}
	reasons := make([]TicketMonitorReason, 0)
	addReason := func(metricCode, ruleCode, severity, message string, evidence map[string]any) {
		if slices.ContainsFunc(reasons, func(existing TicketMonitorReason) bool { return existing.RuleCode == ruleCode }) {
			return
		}
		if evidence == nil {
			evidence = map[string]any{}
		}
		evidence["observed_at"] = now.Format(time.RFC3339)
		reasons = append(reasons, TicketMonitorReason{
			MetricCode: metricCode,
			RuleCode:   ruleCode,
			Severity:   severity,
			Message:    message,
			Evidence:   evidence,
		})
	}

	if ticket.EngineType == "smart" && ticket.AIFailureCount >= engine.MaxAIFailureCount {
		addReason("ai_incident_total", "ai_circuit_breaker", "blocked", "AI 连续失败，等待人工接管", map[string]any{
			"engine_type":         ticket.EngineType,
			"ai_failure_count":    float64(ticket.AIFailureCount),
			"threshold_count":     float64(engine.MaxAIFailureCount),
			"ticket_status":       ticket.Status,
			"current_activity_id": monitorOptionalUint(ticket.CurrentActivityID),
		})
	}
	if ticket.CurrentActivityID == nil && now.Sub(ticket.UpdatedAt) >= monitorNoActivityBlockAfter {
		addReason("blocked_total", "no_current_activity", "blocked", "活跃工单超过 5 分钟没有当前活动", map[string]any{
			"current_activity_id": "nil",
			"ticket_updated_at":   ticket.UpdatedAt.Format(time.RFC3339),
			"waiting_minutes":     float64(elapsedMinutes(now, ticket.UpdatedAt)),
			"threshold_minutes":   float64(monitorNoActivityBlockAfter / time.Minute),
		})
	}
	if fact.Activity != nil && isActiveActivity(fact.Activity.Status) {
		startedAt := activityStartedAt(fact.Activity)
		if engine.IsHumanNode(fact.Activity.ActivityType) {
			switch {
			case fact.AssignmentCount == 0:
				addReason("blocked_total", "human_assignment_missing", "blocked", "当前人工节点没有处理人", map[string]any{
					"activity_id":                 float64(fact.Activity.ID),
					"activity_type":               fact.Activity.ActivityType,
					"assignment_count":            float64(fact.AssignmentCount),
					"resolvable_assignment_count": float64(fact.ResolvableAssignments),
				})
			case fact.ResolvableAssignments == 0:
				addReason("blocked_total", "human_assignment_unresolvable", "blocked", "当前人工节点没有可解析处理人", map[string]any{
					"activity_id":                 float64(fact.Activity.ID),
					"activity_type":               fact.Activity.ActivityType,
					"assignment_count":            float64(fact.AssignmentCount),
					"resolvable_assignment_count": float64(fact.ResolvableAssignments),
				})
			case now.Sub(startedAt) >= monitorHumanWaitRiskAfter:
				addReason("risk_total", "human_waiting_too_long", "risk", "人工节点等待超过 60 分钟", map[string]any{
					"activity_id":         float64(fact.Activity.ID),
					"activity_type":       fact.Activity.ActivityType,
					"activity_started_at": startedAt.Format(time.RFC3339),
					"waiting_minutes":     float64(elapsedMinutes(now, startedAt)),
					"threshold_minutes":   float64(monitorHumanWaitRiskAfter / time.Minute),
				})
			}
		}
		if fact.Activity.ActivityType == engine.NodeAction {
			if fact.ActionFailed {
				addReason("blocked_total", "action_execution_failed", "blocked", "自动化动作执行失败", map[string]any{
					"activity_id":   float64(fact.Activity.ID),
					"activity_type": fact.Activity.ActivityType,
					"action_failed": true,
				})
			} else if now.Sub(startedAt) >= monitorActionRiskAfter {
				addReason("risk_total", "action_running_too_long", "risk", "自动化动作运行超过 15 分钟", map[string]any{
					"activity_id":         float64(fact.Activity.ID),
					"activity_type":       fact.Activity.ActivityType,
					"activity_started_at": startedAt.Format(time.RFC3339),
					"waiting_minutes":     float64(elapsedMinutes(now, startedAt)),
					"threshold_minutes":   float64(monitorActionRiskAfter / time.Minute),
				})
			}
		}
	}

	switch ticket.SLAStatus {
	case SLAStatusBreachedResponse:
		addReason("sla_risk_total", "sla_response_breached", "blocked", "响应 SLA 已超时", map[string]any{
			"sla_status":     ticket.SLAStatus,
			"deadline_field": "sla_response_deadline",
		})
	case SLAStatusBreachedResolve:
		addReason("sla_risk_total", "sla_resolution_breached", "blocked", "解决 SLA 已超时", map[string]any{
			"sla_status":     ticket.SLAStatus,
			"deadline_field": "sla_resolution_deadline",
		})
	}
	if ticket.SLAResponseDeadline != nil {
		switch {
		case !ticket.SLAResponseDeadline.After(now):
			addReason("sla_risk_total", "sla_response_breached", "blocked", "响应 SLA 已超时", map[string]any{
				"sla_status":        ticket.SLAStatus,
				"deadline_field":    "sla_response_deadline",
				"deadline":          ticket.SLAResponseDeadline.Format(time.RFC3339),
				"remaining_minutes": float64(int(ticket.SLAResponseDeadline.Sub(now).Minutes())),
			})
		case ticket.SLAResponseDeadline.Sub(now) <= monitorSLADueRiskBefore:
			addReason("sla_risk_total", "sla_response_due_soon", "risk", "响应 SLA 距离截止小于 30 分钟", map[string]any{
				"sla_status":        ticket.SLAStatus,
				"deadline_field":    "sla_response_deadline",
				"deadline":          ticket.SLAResponseDeadline.Format(time.RFC3339),
				"remaining_minutes": float64(int(ticket.SLAResponseDeadline.Sub(now).Minutes())),
				"threshold_minutes": float64(monitorSLADueRiskBefore / time.Minute),
			})
		}
	}
	if ticket.SLAResolutionDeadline != nil {
		switch {
		case !ticket.SLAResolutionDeadline.After(now):
			addReason("sla_risk_total", "sla_resolution_breached", "blocked", "解决 SLA 已超时", map[string]any{
				"sla_status":        ticket.SLAStatus,
				"deadline_field":    "sla_resolution_deadline",
				"deadline":          ticket.SLAResolutionDeadline.Format(time.RFC3339),
				"remaining_minutes": float64(int(ticket.SLAResolutionDeadline.Sub(now).Minutes())),
			})
		case ticket.SLAResolutionDeadline.Sub(now) <= monitorSLADueRiskBefore:
			addReason("sla_risk_total", "sla_resolution_due_soon", "risk", "解决 SLA 距离截止小于 30 分钟", map[string]any{
				"sla_status":        ticket.SLAStatus,
				"deadline_field":    "sla_resolution_deadline",
				"deadline":          ticket.SLAResolutionDeadline.Format(time.RFC3339),
				"remaining_minutes": float64(int(ticket.SLAResolutionDeadline.Sub(now).Minutes())),
				"threshold_minutes": float64(monitorSLADueRiskBefore / time.Minute),
			})
		}
	}
	return reasons
}

func monitorOptionalUint(value *uint) any {
	if value == nil {
		return "nil"
	}
	return float64(*value)
}

func monitorReasonMessages(reasons []TicketMonitorReason) []string {
	if len(reasons) == 0 {
		return nil
	}
	messages := make([]string, 0, len(reasons))
	for _, reason := range reasons {
		messages = append(messages, reason.Message)
	}
	return messages
}

func monitorReasonsContainSeverity(reasons []TicketMonitorReason, severity string) bool {
	return slices.ContainsFunc(reasons, func(reason TicketMonitorReason) bool {
		return reason.Severity == severity
	})
}

func monitorItemHasMetric(item *TicketMonitorItem, metricCode string) bool {
	return slices.ContainsFunc(item.MonitorReasons, func(reason TicketMonitorReason) bool {
		return reason.MetricCode == metricCode
	})
}

func isActiveTicket(status string) bool {
	return IsActiveTicketStatus(status)
}

func isActiveActivity(status string) bool {
	return status == engine.ActivityPending || status == engine.ActivityInProgress
}

func activityStartedAt(activity *TicketActivity) time.Time {
	if activity.StartedAt != nil {
		return *activity.StartedAt
	}
	return activity.CreatedAt
}

func elapsedMinutes(now time.Time, from time.Time) int {
	if from.IsZero() || now.Before(from) {
		return 0
	}
	return int(now.Sub(from).Minutes())
}

func sameLocalDay(a time.Time, b time.Time) bool {
	ay, am, ad := a.Local().Date()
	by, bm, bd := b.Local().Date()
	return ay == by && am == bm && ad == bd
}

func monitorRiskMatches(filter string, riskLevel string) bool {
	switch filter {
	case "", "all":
		return true
	case "stuck":
		return riskLevel == "blocked" || riskLevel == "risk"
	default:
		return filter == riskLevel
	}
}

func monitorMetricMatches(metricCode string, ticket *Ticket, item *TicketMonitorItem, now time.Time) bool {
	switch metricCode {
	case "", "all":
		return true
	case "active_total":
		return isActiveTicket(ticket.Status)
	case "blocked_total", "stuck_total":
		return isActiveTicket(ticket.Status) && item.RiskLevel == "blocked"
	case "risk_total":
		return isActiveTicket(ticket.Status) && item.RiskLevel == "risk"
	case "sla_risk_total":
		return isActiveTicket(ticket.Status) && monitorItemHasMetric(item, "sla_risk_total")
	case "ai_incident_total":
		return isActiveTicket(ticket.Status) && monitorItemHasMetric(item, "ai_incident_total")
	case "completed_today_total":
		return ticket.Status == TicketStatusCompleted && ticket.FinishedAt != nil && sameLocalDay(*ticket.FinishedAt, now)
	case "smart_active_total":
		return isActiveTicket(ticket.Status) && ticket.EngineType == "smart"
	case "classic_active_total":
		return isActiveTicket(ticket.Status) && ticket.EngineType != "smart"
	default:
		return true
	}
}

func monitorRiskRank(riskLevel string) int {
	switch riskLevel {
	case "blocked":
		return 0
	case "risk":
		return 1
	default:
		return 2
	}
}

func normalizePage(page int, pageSize int) (int, int) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	if pageSize > 200 {
		pageSize = 200
	}
	return page, pageSize
}
