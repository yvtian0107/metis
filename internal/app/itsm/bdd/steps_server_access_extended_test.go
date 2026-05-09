package bdd

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"slices"
	"strings"
	"time"
	"unsafe"

	"github.com/cucumber/godog"

	"metis/internal/app/itsm/domain"
	"metis/internal/app/itsm/engine"
	ticketpkg "metis/internal/app/itsm/ticket"
	org "metis/internal/app/org/domain"
	"metis/internal/database"
	"metis/internal/model"
)

func registerServerAccessExtendedSteps(sc *godog.ScenarioContext, bc *bddContext) {
	sc.Given(`^the following participants exist:$`, bc.givenParticipants)
	sc.Given(`^server access smart service is published$`, bc.givenServerAccessSmartServicePublished)
	sc.Given(`^requester "([^"]*)" has a server access ticket for "([^"]*)"$`, bc.givenServerAccessTicketCreated)
	sc.Given(`^all members of position "([^"]*)" are inactive$`, bc.givenAllMembersOfPositionInactive)
	sc.Given(`^position "([^"]*)" has no members$`, bc.givenPositionHasNoMembers)
	sc.Given(`^user "([^"]*)" also belongs to department "([^"]*)" and position "([^"]*)"$`, bc.givenUserAlsoBelongsToDepartmentAndPosition)

	sc.When(`^the smart engine makes the routing decision$`, bc.whenSmartEngineDecisionCycle)
	sc.When(`^a confirmed smart decision creates work for position "([^"]*)"$`, bc.whenConfirmedSmartDecisionCreatesWorkForPosition)
	sc.When(`^a confirmed smart decision completes the ticket$`, bc.whenDeterministicCompletePlan)
	sc.When(`^the current assignee claims and completes the work$`, bc.whenAssigneeClaimsAndProcesss)
	sc.When(`^the current assignee claims and rejects the work$`, bc.whenAssigneeRejects)
	sc.When(`^the smart engine continues after the current human activity$`, bc.whenSmartEngineDecisionCycleAgain)
	sc.When(`^user "([^"]*)" claims and completes the current work$`, bc.whenNamedUserClaimsAndCompletesCurrentWork)
	sc.When(`^user "([^"]*)" attempts to complete the latest finished work again$`, bc.whenNamedUserAttemptsToCompleteLatestFinishedWorkAgain)
	sc.When(`^admin "([^"]*)" reassigns the current work to "([^"]*)" because "([^"]*)"$`, bc.whenAdminReassignsCurrentWork)
	sc.When(`^admin "([^"]*)" cancels the in-progress ticket because "([^"]*)"$`, bc.whenAdminCancelsTicket)

	sc.Then(`^the current work is assigned to position "([^"]*)"$`, bc.thenCurrentProcessAssignedToPosition)
	sc.Then(`^user "([^"]*)" can claim the current work$`, bc.thenClaimShouldSucceed)
	sc.Then(`^user "([^"]*)" can process the current work$`, bc.thenProcessShouldSucceed)
	sc.Then(`^user "([^"]*)" cannot claim the current work$`, bc.thenClaimShouldFail)
	sc.Then(`^user "([^"]*)" cannot process the current work$`, bc.thenProcessShouldFail)
	sc.Then(`^ticket status is "([^"]*)"$`, bc.thenTicketStatusIs)
	sc.Then(`^ticket status is not "([^"]*)"$`, bc.thenTicketStatusIsNot)
	sc.Then(`^current activity status is "([^"]*)"$`, bc.thenCurrentActivityStatusIs)
	sc.Then(`^timeline contains event type "([^"]*)"$`, bc.thenTimelineContainsEventType)
	sc.Then(`^all activities and assignments are cancelled$`, bc.thenAllActivitiesAndAssignmentsCancelled)
	sc.Then(`^the current ticket assignee is "([^"]*)"$`, bc.thenTicketAssigneeIs)
	sc.Then(`^the server access form data persists all key fields$`, bc.thenServerAccessFormDataPersistsAllKeyFields)
	sc.Then(`^the server access timeline contains the core lifecycle$`, bc.thenServerAccessTimelineContainsCoreLifecycle)
	sc.Then(`^the latest activity exposes AI reasoning and confidence$`, bc.thenLatestActivityExposesAIReasoningAndConfidence)
	sc.Then(`^requester "([^"]*)" can see the ticket in my tickets$`, bc.thenRequesterCanSeeTicketInMyTickets)
	sc.Then(`^requester "([^"]*)" can still see the ticket in my tickets after completion$`, bc.thenRequesterCanStillSeeTicketInMyTicketsAfterCompletion)
	sc.Then(`^operator "([^"]*)" can see the ticket in pending approvals$`, bc.thenOperatorCanSeeTicketInPendingApprovals)
	sc.Then(`^operator "([^"]*)" can see the ticket in approval history$`, bc.thenOperatorCanSeeTicketInApprovalHistory)
	sc.Then(`^user "([^"]*)" cannot see the ticket in approval history$`, bc.thenUserCannotSeeTicketInApprovalHistory)
	sc.Then(`^the user who completed the current work can see the ticket in approval history$`, bc.thenLastCompletedUserCanSeeTicketInApprovalHistory)
	sc.Then(`^the ticket is visible in monitor with service, step, owner, waiting time, and SLA$`, bc.thenTicketVisibleInMonitorWithCoreFields)
	sc.Then(`^the last operation failed$`, bc.thenOperationFailed)
}

func (bc *bddContext) givenAllMembersOfPositionInactive(positionCode string) error {
	pos, ok := bc.positions[positionCode]
	if !ok {
		return fmt.Errorf("position %q not found in context", positionCode)
	}

	var assignments []struct {
		UserID uint
	}
	if err := bc.db.Table("user_positions").Where("position_id = ?", pos.ID).Select("user_id").Scan(&assignments).Error; err != nil {
		return fmt.Errorf("load users for position %q: %w", positionCode, err)
	}
	if len(assignments) == 0 {
		return fmt.Errorf("position %q has no members to deactivate", positionCode)
	}

	ids := make([]uint, 0, len(assignments))
	for _, row := range assignments {
		ids = append(ids, row.UserID)
		if user, ok := bc.usersByName[userNameByID(bc.usersByName, row.UserID)]; ok {
			user.IsActive = false
		}
	}
	return bc.db.Table("users").Where("id IN ?", ids).Update("is_active", false).Error
}

func (bc *bddContext) givenPositionHasNoMembers(positionCode string) error {
	pos, ok := bc.positions[positionCode]
	if !ok {
		return fmt.Errorf("position %q not found in context", positionCode)
	}
	return bc.db.Table("user_positions").Where("position_id = ?", pos.ID).Delete(nil).Error
}

func (bc *bddContext) givenUserAlsoBelongsToDepartmentAndPosition(username, departmentCode, positionCode string) error {
	user, ok := bc.usersByName[username]
	if !ok {
		return fmt.Errorf("user %q not found", username)
	}
	return bc.attachUserToOrgPosition(user.ID, departmentCode, positionCode)
}

func (bc *bddContext) attachUserToOrgPosition(userID uint, departmentCode, positionCode string) error {
	dept, ok := bc.departments[departmentCode]
	if !ok {
		dept = &org.Department{Code: departmentCode, Name: departmentCode, IsActive: true}
		if err := bc.db.Where("code = ?", departmentCode).FirstOrCreate(dept).Error; err != nil {
			return fmt.Errorf("create department %q: %w", departmentCode, err)
		}
		bc.departments[departmentCode] = dept
	}

	pos, ok := bc.positions[positionCode]
	if !ok {
		pos = &org.Position{Code: positionCode, Name: positionCode, IsActive: true}
		if err := bc.db.Where("code = ?", positionCode).FirstOrCreate(pos).Error; err != nil {
			return fmt.Errorf("create position %q: %w", positionCode, err)
		}
		bc.positions[positionCode] = pos
	}

	up := &org.UserPosition{
		UserID:       userID,
		DepartmentID: dept.ID,
		PositionID:   pos.ID,
		IsPrimary:    false,
	}
	if err := bc.db.Where("user_id = ? AND department_id = ? AND position_id = ?", userID, dept.ID, pos.ID).
		FirstOrCreate(up).Error; err != nil {
		return fmt.Errorf("create user_position for user %d: %w", userID, err)
	}
	return nil
}

func (bc *bddContext) whenAdminReassignsCurrentWork(adminUsername, newAssigneeUsername, reason string) error {
	admin, ok := bc.usersByName[adminUsername]
	if !ok {
		return fmt.Errorf("admin user %q not found", adminUsername)
	}
	newAssignee, ok := bc.usersByName[newAssigneeUsername]
	if !ok {
		return fmt.Errorf("new assignee %q not found", newAssigneeUsername)
	}
	activity, err := bc.getCurrentActivity()
	if err != nil {
		return err
	}

	svc := newBDDTicketService(bc)
	updated, err := svc.OverrideReassign(bc.ticket.ID, activity.ID, newAssignee.ID, reason, admin.ID)
	if err != nil {
		bc.lastErr = err
		return err
	}
	bc.ticket = updated
	return nil
}

func (bc *bddContext) whenConfirmedSmartDecisionCreatesWorkForPosition(positionCode string) error {
	if bc.ticket == nil {
		return fmt.Errorf("no ticket in context")
	}

	if err := bc.db.Model(&domain.Ticket{}).Where("id = ?", bc.ticket.ID).
		Update("status", "in_progress").Error; err != nil {
		return fmt.Errorf("update ticket status: %w", err)
	}

	plan := map[string]any{
		"next_step_type": "process",
		"activities": []map[string]any{
			{
				"type":            "process",
				"participant_type": "position_department",
				"position_code":   positionCode,
				"department_code": "it",
				"instructions":    "Handle the server access request",
			},
		},
		"reasoning":  fmt.Sprintf("Route the server access work to %s based on the confirmed decision.", positionCode),
		"confidence": 0.91,
	}

	rawPlan, err := json.Marshal(plan)
	if err != nil {
		return fmt.Errorf("marshal decision plan: %w", err)
	}

	var typedPlan struct {
		NextStepType string `json:"next_step_type"`
		Activities   []struct {
			Type            string `json:"type"`
			ParticipantType string `json:"participant_type"`
			PositionCode    string `json:"position_code"`
			DepartmentCode  string `json:"department_code"`
			Instructions    string `json:"instructions"`
		} `json:"activities"`
		Reasoning  string  `json:"reasoning"`
		Confidence float64 `json:"confidence"`
	}
	if err := json.Unmarshal(rawPlan, &typedPlan); err != nil {
		return fmt.Errorf("unmarshal typed decision plan: %w", err)
	}

	planForEngine := &engine.DecisionPlan{
		NextStepType: typedPlan.NextStepType,
		Reasoning:    typedPlan.Reasoning,
		Confidence:   typedPlan.Confidence,
	}
	for _, activity := range typedPlan.Activities {
		planForEngine.Activities = append(planForEngine.Activities, engine.DecisionActivity{
			Type:            activity.Type,
			ParticipantType: activity.ParticipantType,
			PositionCode:    activity.PositionCode,
			DepartmentCode:  activity.DepartmentCode,
			Instructions:    activity.Instructions,
		})
	}

	if err := bc.smartEngine.ExecuteDecisionPlan(bc.db, bc.ticket.ID, planForEngine); err != nil {
		bc.lastErr = err
		return fmt.Errorf("execute decision plan: %w", err)
	}
	return bc.db.First(bc.ticket, bc.ticket.ID).Error
}

func (bc *bddContext) whenAdminCancelsTicket(adminUsername, reason string) error {
	admin, ok := bc.usersByName[adminUsername]
	if !ok {
		return fmt.Errorf("admin user %q not found", adminUsername)
	}
	svc := newBDDTicketService(bc)
	updated, err := svc.Cancel(bc.ticket.ID, reason, admin.ID)
	if err != nil {
		bc.lastErr = err
		return err
	}
	bc.ticket = updated
	return nil
}

func (bc *bddContext) whenNamedUserClaimsAndCompletesCurrentWork(username string) error {
	return bc.progressCurrentActivityAsUser(username, "completed", "")
}

func (bc *bddContext) whenNamedUserAttemptsToCompleteLatestFinishedWorkAgain(username string) error {
	user, ok := bc.usersByName[username]
	if !ok {
		return fmt.Errorf("user %q not found", username)
	}
	if bc.ticket == nil {
		return fmt.Errorf("no ticket in context")
	}

	var activity domain.TicketActivity
	if err := bc.db.Where("ticket_id = ? AND status IN ?", bc.ticket.ID, []string{"completed", "approved"}).
		Order("finished_at DESC, id DESC").First(&activity).Error; err != nil {
		return fmt.Errorf("load latest finished activity: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	err := bc.smartEngine.Progress(ctx, bc.db, engine.ProgressParams{
		TicketID:   bc.ticket.ID,
		ActivityID: activity.ID,
		Outcome:    "completed",
		OperatorID: user.ID,
	})
	bc.lastErr = err
	return nil
}

func (bc *bddContext) progressCurrentActivityAsUser(username, outcome, opinion string) error {
	user, ok := bc.usersByName[username]
	if !ok {
		return fmt.Errorf("user %q not found", username)
	}
	activity, err := bc.getCurrentActivity()
	if err != nil {
		return err
	}

	if err := bc.db.Model(&domain.TicketAssignment{}).
		Where("activity_id = ?", activity.ID).
		Updates(map[string]any{"assignee_id": user.ID, "status": "claimed"}).Error; err != nil {
		return fmt.Errorf("claim assignment for %q: %w", username, err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	err = bc.smartEngine.Progress(ctx, bc.db, engine.ProgressParams{
		TicketID:   bc.ticket.ID,
		ActivityID: activity.ID,
		Outcome:    outcome,
		Opinion:    opinion,
		OperatorID: user.ID,
	})
	if err != nil {
		bc.lastErr = err
		return err
	}
	bc.lastCompletedUserID = user.ID
	return bc.db.First(bc.ticket, bc.ticket.ID).Error
}

func (bc *bddContext) thenClaimShouldSucceed(username string) error {
	if err := bc.thenClaimShouldFail(username); err != nil {
		return nil
	}
	return fmt.Errorf("expected user %q to be eligible to claim current work, but claim eligibility check failed", username)
}

func (bc *bddContext) thenProcessShouldSucceed(username string) error {
	return bc.thenClaimShouldSucceed(username)
}

func (bc *bddContext) thenServerAccessFormDataPersistsAllKeyFields() error {
	if bc.ticket == nil {
		return fmt.Errorf("no ticket in context")
	}
	if err := bc.db.First(bc.ticket, bc.ticket.ID).Error; err != nil {
		return fmt.Errorf("refresh ticket: %w", err)
	}

	var formData map[string]any
	if err := json.Unmarshal([]byte(bc.ticket.FormData), &formData); err != nil {
		return fmt.Errorf("unmarshal form data: %w", err)
	}

	for _, key := range []string{"target_host", "access_account", "source_ip", "access_window", "access_reason"} {
		value, ok := formData[key]
		if !ok || strings.TrimSpace(fmt.Sprintf("%v", value)) == "" {
			return fmt.Errorf("expected formData[%q] to be persisted, got %v", key, value)
		}
	}
	return nil
}

func (bc *bddContext) thenServerAccessTimelineContainsCoreLifecycle() error {
	if bc.ticket == nil {
		return fmt.Errorf("no ticket in context")
	}

	requiredEventTypes := []string{"ticket_created", "ai_decision_executed", "workflow_completed"}
	for _, eventType := range requiredEventTypes {
		if err := bc.thenTimelineContainsEventType(eventType); err != nil {
			return err
		}
	}

	var assignmentCount int64
	if err := bc.db.Model(&domain.TicketAssignment{}).Where("ticket_id = ?", bc.ticket.ID).Count(&assignmentCount).Error; err != nil {
		return err
	}
	if assignmentCount == 0 {
		return fmt.Errorf("expected at least one assignment for ticket %d", bc.ticket.ID)
	}

	var completedHumanCount int64
	if err := bc.db.Model(&domain.TicketActivity{}).
		Where("ticket_id = ? AND activity_type IN ? AND status IN ?", bc.ticket.ID,
			[]string{"approve", "form", "process"},
			[]string{"completed", "approved"}).
		Count(&completedHumanCount).Error; err != nil {
		return err
	}
	if completedHumanCount == 0 {
		return fmt.Errorf("expected at least one completed human activity for ticket %d", bc.ticket.ID)
	}
	return nil
}

func (bc *bddContext) thenLatestActivityExposesAIReasoningAndConfidence() error {
	if err := bc.thenActivityContainsAIReasoning(); err != nil {
		return err
	}
	activity, err := bc.getLatestActivity()
	if err != nil {
		return err
	}
	if activity.AIConfidence < 0 || activity.AIConfidence > 1 {
		return fmt.Errorf("expected AI confidence in [0,1], got %f", activity.AIConfidence)
	}

	var timeline domain.TicketTimeline
	if err := bc.db.Where("ticket_id = ? AND event_type = ?", bc.ticket.ID, "ai_decision_executed").
		Order("id DESC").First(&timeline).Error; err != nil {
		return fmt.Errorf("load ai decision timeline: %w", err)
	}
	if strings.TrimSpace(timeline.Reasoning) == "" {
		return fmt.Errorf("expected ai_decision_executed timeline reasoning to be present")
	}
	return nil
}

func (bc *bddContext) thenRequesterCanSeeTicketInMyTickets(username string) error {
	user, ok := bc.usersByName[username]
	if !ok {
		return fmt.Errorf("requester %q not found", username)
	}

	statusGroups := []string{"active", ""}
	for _, group := range statusGroups {
		if response, found, err := bc.findTicketInMine(user.ID, group); err != nil {
			return fmt.Errorf("list my tickets for status %q: %w", normalizedStatusGroup(group), err)
		} else if found {
			if strings.TrimSpace(response.Status) == "" {
				return fmt.Errorf("ticket %d in my tickets missing status for status group %q", bc.ticket.ID, normalizedStatusGroup(group))
			}
			if strings.TrimSpace(response.Title) == "" {
				return fmt.Errorf("ticket %d in my tickets missing title for status group %q", bc.ticket.ID, normalizedStatusGroup(group))
			}
			return nil
		}
	}
	return fmt.Errorf("ticket %d not found in requester %q my tickets", bc.ticket.ID, username)
}

func (bc *bddContext) thenRequesterCanStillSeeTicketInMyTicketsAfterCompletion(username string) error {
	user, ok := bc.usersByName[username]
	if !ok {
		return fmt.Errorf("requester %q not found", username)
	}

	if _, found, err := bc.findTicketInMine(user.ID, "active"); err != nil {
		return fmt.Errorf("list my tickets for status %q: %w", "active", err)
	} else if found {
		return fmt.Errorf("completed ticket %d should not remain in active my tickets tab", bc.ticket.ID)
	}

	for _, group := range []string{"terminal", ""} {
		response, found, err := bc.findTicketInMine(user.ID, group)
		if err != nil {
			return fmt.Errorf("list my tickets for status %q: %w", normalizedStatusGroup(group), err)
		}
		if !found {
			return fmt.Errorf("ticket %d not found in requester %q my tickets status group %q", bc.ticket.ID, username, normalizedStatusGroup(group))
		}
		if strings.TrimSpace(response.Status) == "" {
			return fmt.Errorf("ticket %d missing status in my tickets status group %q", bc.ticket.ID, normalizedStatusGroup(group))
		}
		if !slices.Contains([]string{"completed", "rejected", "withdrawn", "cancelled", "failed"}, response.Status) {
			return fmt.Errorf("ticket %d in my tickets status group %q expected terminal status, got %q", bc.ticket.ID, normalizedStatusGroup(group), response.Status)
		}
		if strings.TrimSpace(response.Title) == "" || strings.TrimSpace(response.ServiceName) == "" {
			return fmt.Errorf("ticket %d missing detail fields in my tickets status group %q", bc.ticket.ID, normalizedStatusGroup(group))
		}
	}
	return nil
}

func (bc *bddContext) thenOperatorCanSeeTicketInPendingApprovals(username string) error {
	user, ok := bc.usersByName[username]
	if !ok {
		return fmt.Errorf("operator %q not found", username)
	}
	repo := newBDDTicketRepo(bc)
	items, _, err := repo.ListPendingApprovals(ticketpkg.TicketApprovalListParams{Page: 1, PageSize: 20}, user.ID, nil, nil)
	if err != nil {
		return fmt.Errorf("list pending approvals: %w", err)
	}
	for _, item := range items {
		if item.ID == bc.ticket.ID {
			return nil
		}
	}
	return fmt.Errorf("ticket %d not found in pending approvals for %q", bc.ticket.ID, username)
}

func (bc *bddContext) thenOperatorCanSeeTicketInApprovalHistory(username string) error {
	user, ok := bc.usersByName[username]
	if !ok {
		return fmt.Errorf("operator %q not found", username)
	}
	response, found, err := bc.findTicketInApprovalHistory(user.ID)
	if err != nil {
		return fmt.Errorf("list approval history: %w", err)
	}
	if !found {
		return fmt.Errorf("ticket %d not found in approval history for %q", bc.ticket.ID, username)
	}
	if strings.TrimSpace(response.Status) == "" || strings.TrimSpace(response.Title) == "" || strings.TrimSpace(response.ServiceName) == "" {
		return fmt.Errorf("ticket %d missing approval history detail fields for %q", bc.ticket.ID, username)
	}
	return nil
}

func (bc *bddContext) thenUserCannotSeeTicketInApprovalHistory(username string) error {
	user, ok := bc.usersByName[username]
	if !ok {
		return fmt.Errorf("user %q not found", username)
	}
	_, found, err := bc.findTicketInApprovalHistory(user.ID)
	if err != nil {
		return fmt.Errorf("list approval history: %w", err)
	}
	if found {
		return fmt.Errorf("ticket %d unexpectedly found in approval history for %q", bc.ticket.ID, username)
	}
	return nil
}

func (bc *bddContext) thenLastCompletedUserCanSeeTicketInApprovalHistory() error {
	if bc.lastCompletedUserID == 0 {
		return fmt.Errorf("no completed operator recorded in scenario context")
	}
	username := userNameByID(bc.usersByName, bc.lastCompletedUserID)
	if strings.TrimSpace(username) == "" {
		return fmt.Errorf("completed operator user %d not found in scenario users", bc.lastCompletedUserID)
	}
	return bc.thenOperatorCanSeeTicketInApprovalHistory(username)
}

func (bc *bddContext) findTicketInMine(requesterID uint, status string) (domain.TicketResponse, bool, error) {
	svc := newBDDTicketService(bc)
	items, _, err := svc.Mine(requesterID, "", status, nil, nil, 1, 20)
	if err != nil {
		return domain.TicketResponse{}, false, err
	}
	responses, err := svc.BuildResponses(items, requesterID)
	if err != nil {
		return domain.TicketResponse{}, false, err
	}
	for _, item := range responses {
		if item.ID == bc.ticket.ID {
			return item, true, nil
		}
	}
	return domain.TicketResponse{}, false, nil
}

func (bc *bddContext) findTicketInApprovalHistory(operatorID uint) (domain.TicketResponse, bool, error) {
	svc := newBDDTicketService(bc)
	items, _, err := svc.ApprovalHistory(operatorID, "", 1, 20)
	if err != nil {
		return domain.TicketResponse{}, false, err
	}
	responses, err := svc.BuildResponses(items, operatorID)
	if err != nil {
		return domain.TicketResponse{}, false, err
	}
	for _, item := range responses {
		if item.ID == bc.ticket.ID {
			return item, true, nil
		}
	}
	return domain.TicketResponse{}, false, nil
}

func normalizedStatusGroup(status string) string {
	if strings.TrimSpace(status) == "" {
		return "all"
	}
	return status
}

func (bc *bddContext) thenTicketVisibleInMonitorWithCoreFields() error {
	svc := newBDDTicketService(bc)
	resp, err := svc.Monitor(ticketpkg.TicketMonitorParams{Page: 1, PageSize: 20}, bc.ticket.RequesterID)
	if err != nil {
		return fmt.Errorf("monitor query: %w", err)
	}
	for _, item := range resp.Items {
		if item.ID != bc.ticket.ID {
			continue
		}
		if strings.TrimSpace(item.ServiceName) == "" {
			return fmt.Errorf("monitor item missing service name")
		}
		if strings.TrimSpace(item.CurrentActivityName) == "" {
			return fmt.Errorf("monitor item missing current activity name")
		}
		if strings.TrimSpace(item.CurrentOwnerName) == "" {
			return fmt.Errorf("monitor item missing current owner name")
		}
		if item.WaitingMinutes < 0 {
			return fmt.Errorf("monitor item waiting minutes invalid: %d", item.WaitingMinutes)
		}
		if strings.TrimSpace(item.SLAStatus) == "" {
			return fmt.Errorf("monitor item missing SLA status")
		}
		return nil
	}
	return fmt.Errorf("ticket %d not found in monitor response", bc.ticket.ID)
}

func newBDDTicketRepo(bc *bddContext) *ticketpkg.TicketRepo {
	repo := &ticketpkg.TicketRepo{}
	setUnexportedField(repo, "db", &database.DB{DB: bc.db})
	return repo
}

func newBDDTicketService(bc *bddContext) *ticketpkg.TicketService {
	svc := &ticketpkg.TicketService{}
	setUnexportedField(svc, "ticketRepo", newBDDTicketRepo(bc))
	return svc
}

func setUnexportedField(target any, fieldName string, value any) {
	rv := reflect.ValueOf(target)
	if rv.Kind() != reflect.Ptr || rv.IsNil() {
		panic("target must be a non-nil pointer")
	}
	field := rv.Elem().FieldByName(fieldName)
	if !field.IsValid() {
		panic(fmt.Sprintf("field %q not found", fieldName))
	}
	reflect.NewAt(field.Type(), unsafe.Pointer(field.UnsafeAddr())).Elem().Set(reflect.ValueOf(value))
}

func userNameByID(users map[string]*model.User, userID uint) string {
	for username, user := range users {
		if user.ID == userID {
			return username
		}
	}
	return ""
}
