package ticket

import (
	. "metis/internal/app/itsm/domain"
	"slices"
	"time"

	"metis/internal/app/itsm/engine"
)

type ticketMonitorFact struct {
	ActivityFacts         []ticketMonitorActivityFact
	Activity              *TicketActivity
	AssignmentCount       int
	ResolvableAssignments int
	ResolvableUserCount   int
	OwnerName             string
	ActionFailed          bool
}

type ticketMonitorActivityFact struct {
	Activity              TicketActivity
	AssignmentCount       int
	ResolvableAssignments int
	ResolvableUserCount   int
	OwnerName             string
	ActionFailed          bool
}

func (s *TicketService) loadMonitorFacts(tickets []Ticket) (map[uint]ticketMonitorFact, error) {
	facts := make(map[uint]ticketMonitorFact, len(tickets))
	ticketIDs := map[uint]struct{}{}
	currentActivityIDs := map[uint]uint{}
	for i := range tickets {
		t := &tickets[i]
		facts[t.ID] = ticketMonitorFact{}
		ticketIDs[t.ID] = struct{}{}
		if t.CurrentActivityID != nil {
			currentActivityIDs[t.ID] = *t.CurrentActivityID
		}
	}
	if len(ticketIDs) == 0 {
		return facts, nil
	}

	var activities []TicketActivity
	if err := s.ticketRepo.DB().
		Where("ticket_id IN ? AND status IN ?", keysOf(ticketIDs), []string{engine.ActivityPending, engine.ActivityInProgress}).
		Order("ticket_id ASC, id ASC").
		Find(&activities).Error; err != nil {
		return nil, err
	}
	activityIDs := map[uint]struct{}{}
	for i := range activities {
		activityIDs[activities[i].ID] = struct{}{}
	}
	if len(activityIDs) == 0 {
		return facts, nil
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
	resolvableUserCount := map[uint]int{}
	resolvableUsers, err := s.countResolvableAssignmentUsers(assignmentRows)
	if err != nil {
		return nil, err
	}
	for _, a := range assignmentRows {
		assignmentCount[a.ActivityID]++
		if resolvableUsers[a.ID] > 0 {
			resolvableCount[a.ActivityID]++
			resolvableUserCount[a.ActivityID] += resolvableUsers[a.ID]
		}
	}

	for i := range activities {
		activity := activities[i]
		activityFact := ticketMonitorActivityFact{
			Activity:              activity,
			ActionFailed:          failedByActivity[activity.ID],
			AssignmentCount:       assignmentCount[activity.ID],
			ResolvableAssignments: resolvableCount[activity.ID],
			ResolvableUserCount:   resolvableUserCount[activity.ID],
		}
		if assignment, ok := assignments[activity.ID]; ok {
			activityFact.OwnerName = assignment.OwnerName
		}
		fact := facts[activity.TicketID]
		fact.ActivityFacts = append(fact.ActivityFacts, activityFact)
		if fact.Activity == nil || currentActivityIDs[activity.TicketID] == activity.ID {
			fact.Activity = &activity
			fact.ActionFailed = activityFact.ActionFailed
			fact.AssignmentCount = activityFact.AssignmentCount
			fact.ResolvableAssignments = activityFact.ResolvableAssignments
			fact.ResolvableUserCount = activityFact.ResolvableUserCount
			fact.OwnerName = activityFact.OwnerName
		}
		facts[activity.TicketID] = fact
	}
	return facts, nil
}

func (s *TicketService) countResolvableAssignmentUsers(assignments []TicketAssignment) (map[uint]int, error) {
	result := make(map[uint]int, len(assignments))
	if len(assignments) == 0 {
		return result, nil
	}
	for _, assignment := range assignments {
		userIDs := map[uint]struct{}{}
		if assignment.AssigneeID != nil {
			userIDs[*assignment.AssigneeID] = struct{}{}
		}
		if assignment.UserID != nil {
			userIDs[*assignment.UserID] = struct{}{}
		}
		if ids := keysOf(userIDs); len(ids) > 0 {
			var count int64
			if err := s.ticketRepo.DB().Table("users").
				Where("id IN ? AND is_active = ? AND deleted_at IS NULL", ids, true).
				Count(&count).Error; err != nil {
				return nil, err
			}
			result[assignment.ID] += int(count)
		}
		orgCount, err := s.countResolvableOrgAssignmentUsers(assignment)
		if err != nil {
			return nil, err
		}
		result[assignment.ID] += orgCount
	}
	return result, nil
}

func (s *TicketService) countResolvableOrgAssignmentUsers(assignment TicketAssignment) (int, error) {
	db := s.ticketRepo.DB()
	if !db.Migrator().HasTable("user_positions") {
		return 0, nil
	}
	query := db.Table("user_positions").
		Joins("JOIN users ON users.id = user_positions.user_id").
		Where("user_positions.deleted_at IS NULL AND users.deleted_at IS NULL AND users.is_active = ?", true)
	switch {
	case assignment.PositionID != nil && assignment.DepartmentID != nil:
		query = query.Where("user_positions.position_id = ? AND user_positions.department_id = ?", *assignment.PositionID, *assignment.DepartmentID)
	case assignment.PositionID != nil:
		query = query.Where("user_positions.position_id = ?", *assignment.PositionID)
	case assignment.DepartmentID != nil:
		query = query.Where("user_positions.department_id = ?", *assignment.DepartmentID)
	default:
		return 0, nil
	}
	var count int64
	if err := query.Distinct("user_positions.user_id").Count(&count).Error; err != nil {
		return 0, err
	}
	return int(count), nil
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
	activityFacts := fact.ActivityFacts
	if len(activityFacts) == 0 && fact.Activity != nil {
		activityFacts = []ticketMonitorActivityFact{{
			Activity:              *fact.Activity,
			AssignmentCount:       fact.AssignmentCount,
			ResolvableAssignments: fact.ResolvableAssignments,
			ResolvableUserCount:   fact.ResolvableUserCount,
			OwnerName:             fact.OwnerName,
			ActionFailed:          fact.ActionFailed,
		}}
	}
	if len(activityFacts) == 0 && ticket.CurrentActivityID == nil && now.Sub(ticket.UpdatedAt) >= monitorNoActivityBlockAfter {
		addReason("blocked_total", "no_current_activity", "blocked", "活跃工单超过 5 分钟没有当前活动", map[string]any{
			"current_activity_id": "nil",
			"ticket_updated_at":   ticket.UpdatedAt.Format(time.RFC3339),
			"waiting_minutes":     float64(elapsedMinutes(now, ticket.UpdatedAt)),
			"threshold_minutes":   float64(monitorNoActivityBlockAfter / time.Minute),
		})
	}
	for _, activityFact := range activityFacts {
		activity := activityFact.Activity
		if !isActiveActivity(activity.Status) {
			continue
		}
		startedAt := activityStartedAt(&activity)
		if engine.IsHumanNode(activity.ActivityType) {
			switch {
			case activityFact.AssignmentCount == 0:
				addReason("blocked_total", "human_assignment_missing", "blocked", "当前人工节点没有处理人", map[string]any{
					"activity_id":                 float64(activity.ID),
					"activity_type":               activity.ActivityType,
					"assignment_count":            float64(activityFact.AssignmentCount),
					"resolvable_assignment_count": float64(activityFact.ResolvableAssignments),
					"resolvable_user_count":       float64(activityFact.ResolvableUserCount),
				})
			case activityFact.ResolvableAssignments == 0:
				addReason("blocked_total", "human_assignment_unresolvable", "blocked", "当前人工节点没有可解析处理人", map[string]any{
					"activity_id":                 float64(activity.ID),
					"activity_type":               activity.ActivityType,
					"assignment_count":            float64(activityFact.AssignmentCount),
					"resolvable_assignment_count": float64(activityFact.ResolvableAssignments),
					"resolvable_user_count":       float64(activityFact.ResolvableUserCount),
				})
			case now.Sub(startedAt) >= monitorHumanWaitRiskAfter:
				addReason("risk_total", "human_waiting_too_long", "risk", "人工节点等待超过 60 分钟", map[string]any{
					"activity_id":         float64(activity.ID),
					"activity_type":       activity.ActivityType,
					"activity_started_at": startedAt.Format(time.RFC3339),
					"waiting_minutes":     float64(elapsedMinutes(now, startedAt)),
					"threshold_minutes":   float64(monitorHumanWaitRiskAfter / time.Minute),
				})
			}
		}
		if activity.ActivityType == engine.NodeAction {
			if activityFact.ActionFailed {
				addReason("blocked_total", "action_execution_failed", "blocked", "自动化动作执行失败", map[string]any{
					"activity_id":   float64(activity.ID),
					"activity_type": activity.ActivityType,
					"action_failed": true,
				})
			} else if now.Sub(startedAt) >= monitorActionRiskAfter {
				addReason("risk_total", "action_running_too_long", "risk", "自动化动作运行超过 15 分钟", map[string]any{
					"activity_id":         float64(activity.ID),
					"activity_type":       activity.ActivityType,
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
	if pageSize > 100 {
		pageSize = 100
	}
	return page, pageSize
}
