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

	var blocked, risk []string
	if ticket.EngineType == "smart" && ticket.AIFailureCount >= engine.MaxAIFailureCount {
		blocked = append(blocked, "AI 连续失败，等待人工接管")
	}
	if ticket.CurrentActivityID == nil && now.Sub(ticket.UpdatedAt) >= monitorNoActivityBlockAfter {
		blocked = append(blocked, "活跃工单超过 5 分钟没有当前活动")
	}
	if fact.Activity != nil && isActiveActivity(fact.Activity.Status) {
		if engine.IsHumanNode(fact.Activity.ActivityType) {
			switch {
			case fact.AssignmentCount == 0:
				blocked = append(blocked, "当前人工节点没有处理人")
			case fact.ResolvableAssignments == 0:
				blocked = append(blocked, "当前人工节点没有可解析处理人")
			case now.Sub(activityStartedAt(fact.Activity)) >= monitorHumanWaitRiskAfter:
				risk = append(risk, "人工节点等待超过 60 分钟")
			}
		}
		if fact.Activity.ActivityType == engine.NodeAction {
			if fact.ActionFailed {
				blocked = append(blocked, "自动化动作执行失败")
			} else if now.Sub(activityStartedAt(fact.Activity)) >= monitorActionRiskAfter {
				risk = append(risk, "自动化动作运行超过 15 分钟")
			}
		}
	}
	slaBlocked, slaRisk := monitorSLAReasons(ticket, now)
	blocked = append(blocked, slaBlocked...)
	risk = append(risk, slaRisk...)

	if len(blocked) > 0 {
		item.RiskLevel = "blocked"
		item.Stuck = true
		item.StuckReasons = blocked
		return
	}
	if len(risk) > 0 {
		item.RiskLevel = "risk"
		item.Stuck = false
		item.StuckReasons = risk
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
		if ticket.EngineType == "smart" && (ticket.AIFailureCount >= engine.MaxAIFailureCount || item.SmartState == "ai_disabled") {
			summary.AIIncidentTotal++
		}
		if monitorHasSLARisk(ticket, now) {
			summary.SLARiskTotal++
		}
	}
	if ticket.Status == TicketStatusCompleted && ticket.FinishedAt != nil && sameLocalDay(*ticket.FinishedAt, now) {
		summary.CompletedTodayTotal++
	}
}

func monitorHasSLARisk(ticket *Ticket, now time.Time) bool {
	blocked, risk := monitorSLAReasons(ticket, now)
	return len(blocked) > 0 || len(risk) > 0
}

func monitorSLAReasons(ticket *Ticket, now time.Time) ([]string, []string) {
	var blocked, risk []string
	addBlocked := func(reason string) {
		if !slices.Contains(blocked, reason) {
			blocked = append(blocked, reason)
		}
	}
	addRisk := func(reason string) {
		if !slices.Contains(risk, reason) {
			risk = append(risk, reason)
		}
	}

	switch ticket.SLAStatus {
	case SLAStatusBreachedResponse:
		addBlocked("响应 SLA 已超时")
	case SLAStatusBreachedResolve:
		addBlocked("解决 SLA 已超时")
	}
	if ticket.SLAResponseDeadline != nil {
		switch {
		case !ticket.SLAResponseDeadline.After(now):
			addBlocked("响应 SLA 已超时")
		case ticket.SLAResponseDeadline.Sub(now) <= monitorSLADueRiskBefore:
			addRisk("响应 SLA 距离截止小于 30 分钟")
		}
	}
	if ticket.SLAResolutionDeadline != nil {
		switch {
		case !ticket.SLAResolutionDeadline.After(now):
			addBlocked("解决 SLA 已超时")
		case ticket.SLAResolutionDeadline.Sub(now) <= monitorSLADueRiskBefore:
			addRisk("解决 SLA 距离截止小于 30 分钟")
		}
	}
	return blocked, risk
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
