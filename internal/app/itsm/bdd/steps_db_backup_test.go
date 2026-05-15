package bdd

// steps_db_backup_test.go — step definitions for the DB backup whitelist BDD scenarios.

import (
	"context"
	"encoding/json"
	"fmt"
	app "metis/internal/app"
	ai "metis/internal/app/ai/runtime"
	. "metis/internal/app/itsm/domain"
	"metis/internal/app/itsm/engine"
	"strings"
	"time"

	"github.com/cucumber/godog"
)

// registerDbBackupSteps registers all DB backup whitelist step definitions.
func registerDbBackupSteps(sc *godog.ScenarioContext, bc *bddContext) {
	sc.Given(`^已定义数据库备份白名单临时放行协作规范$`, bc.givenDbBackupCollaborationSpec)
	sc.Given(`^已基于协作规范发布数据库备份白名单放行服务（智能引擎）$`, bc.givenDbBackupSmartServicePublished)
	sc.Given(`^已基于静态工作流发布数据库备份白名单放行服务（智能引擎）$`, bc.givenDbBackupStaticSmartServicePublished)
	sc.Given(`^放行接收端临时失败$`, bc.givenApplyReceiverTemporarilyFails)
	sc.Given(`^"([^"]*)" 已创建数据库备份白名单放行工单，场景为 "([^"]*)"$`, bc.givenDbBackupTicketCreated)
	sc.Given(`^"([^"]*)" 已创建数据库备份白名单放行工单 "([^"]*)"，场景为 "([^"]*)"$`, bc.givenDbBackupTicketCreatedWithAlias)
	sc.Given(`^数据库备份白名单流程图包含预检和放行动作节点$`, bc.thenDbBackupWorkflowContainsActionNodes)

	sc.Then(`^预检动作已为当前工单触发$`, bc.thenPrecheckActionTriggered)
	sc.Then(`^预检动作未为当前工单触发$`, bc.thenPrecheckActionNotTriggered)
	sc.Then(`^预检动作请求包含完整风险上下文$`, bc.thenPrecheckPayloadContainsRiskContext)
	sc.Then(`^放行动作已为当前工单触发$`, bc.thenApplyActionTriggered)
	sc.Then(`^放行动作请求包含完整放行上下文$`, bc.thenApplyPayloadContainsReleaseContext)
	sc.Then(`^放行动作未为当前工单触发$`, bc.thenApplyActionNotTriggered)
	sc.Then(`^放行动作执行失败$`, bc.thenApplyActionFailed)
	sc.Then(`^放行动作成功记录数为 (\d+)$`, bc.thenApplyActionSuccessCountIs)
	sc.Then(`^放行动作失败记录数为 (\d+)$`, bc.thenApplyActionFailureCountIs)
	sc.Then(`^放行动作失败记录数至少为 (\d+)$`, bc.thenApplyActionFailureCountAtLeast)
	sc.Then(`^当前工单未完成且未履约$`, bc.thenTicketNotCompletedNorFulfilled)
	sc.Then(`^工单 "([^"]*)" 的动作记录与工单 "([^"]*)" 完全隔离$`, bc.thenActionRecordsIsolated)
	sc.Then(`^记录当前工单活动数与动作请求数$`, bc.thenMarkCurrentActivityAndActionRequestCounts)
	sc.Then(`^当前工单活动数与动作请求数保持不变$`, bc.thenCurrentActivityAndActionRequestCountsUnchanged)
	sc.Then(`^数据库备份白名单流程图包含预检和放行动作节点$`, bc.thenDbBackupWorkflowContainsActionNodes)

	sc.When(`^放行接收端恢复成功$`, bc.whenApplyReceiverRecovers)

	// DBW-204 steps
	sc.Given(`^预检接收端临时失败$`, bc.givenPrecheckReceiverTemporarilyFails)
	sc.Then(`^预检动作执行失败$`, bc.thenPrecheckActionFailed)

	// DBW-509 steps
	sc.Then(`^放行动作失败记录包含完整故障信息$`, bc.thenApplyActionFailureRecordContainsDetail)

	// TICK-00109 回归 steps
	sc.When(`^管理员将当前活动指派给 "([^"]*)"$`, bc.whenAdminAssignsCurrentActivityTo)
	sc.When(`^被指派人认领并处理完成当前活动$`, bc.whenAssignedUserCompletesCurrentActivity)

	// DBW-407 steps
	sc.Then(`^当前工单所有活动均已取消$`, bc.thenAllCurrentTicketActivitiesCancelled)
}

// --- Given steps ---

func (bc *bddContext) givenDbBackupCollaborationSpec() error {
	bc.collaborationSpec = dbBackupCollaborationSpec
	return nil
}

func (bc *bddContext) givenDbBackupSmartServicePublished() error {
	return publishDbBackupSmartService(bc)
}

func (bc *bddContext) givenApplyReceiverTemporarilyFails() error {
	if bc.actionReceiver == nil {
		return fmt.Errorf("action receiver not initialized")
	}
	bc.actionReceiver.SetResponder("/apply", func(ActionRecord) (int, string) {
		return 500, `{"status":"error","message":"temporary apply failure"}`
	})
	return nil
}

func (bc *bddContext) givenDbBackupTicketCreated(username, caseKey string) error {
	user, ok := bc.usersByName[username]
	if !ok {
		return fmt.Errorf("user %q not found in context", username)
	}

	payload, ok := dbBackupCasePayloads[caseKey]
	if !ok {
		return fmt.Errorf("unknown case key %q", caseKey)
	}

	formJSON, _ := json.Marshal(payload.FormData)

	ticket := &Ticket{
		Code:         fmt.Sprintf("DB-%s-%d", caseKey, time.Now().UnixNano()),
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

func (bc *bddContext) givenDbBackupTicketCreatedWithAlias(username, alias, caseKey string) error {
	user, ok := bc.usersByName[username]
	if !ok {
		return fmt.Errorf("user %q not found in context", username)
	}

	payload, ok := dbBackupCasePayloads[caseKey]
	if !ok {
		return fmt.Errorf("unknown case key %q", caseKey)
	}

	formJSON, _ := json.Marshal(payload.FormData)

	ticket := &Ticket{
		Code:         fmt.Sprintf("DB-%s-%d", alias, time.Now().UnixNano()),
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
	bc.tickets[alias] = ticket
	bc.ticket = ticket // also set as current ticket
	return nil
}

// --- Then steps ---

func (bc *bddContext) thenPrecheckActionTriggered() error {
	return bc.assertActionTriggered("db_backup_whitelist_precheck", "/precheck")
}

func (bc *bddContext) thenPrecheckActionNotTriggered() error {
	return bc.assertActionNotTriggered("db_backup_whitelist_precheck", "/precheck")
}

func (bc *bddContext) thenPrecheckPayloadContainsRiskContext() error {
	return bc.assertLatestActionPayloadContains("db_backup_whitelist_precheck", "/precheck", []string{
		"ticket_code", "database_name", "source_ip", "whitelist_window", "access_reason",
	})
}

func (bc *bddContext) thenApplyActionTriggered() error {
	return bc.assertActionTriggered("db_backup_whitelist_apply", "/apply")
}

func (bc *bddContext) thenApplyPayloadContainsReleaseContext() error {
	return bc.assertLatestActionPayloadContains("db_backup_whitelist_apply", "/apply", []string{
		"ticket_code", "database_name", "source_ip", "whitelist_window",
	})
}

func (bc *bddContext) thenApplyActionNotTriggered() error {
	return bc.assertActionNotTriggered("db_backup_whitelist_apply", "/apply")
}

func (bc *bddContext) thenApplyActionFailed() error {
	return bc.assertActionExecutionCountAtLeast("db_backup_whitelist_apply", "failed", 1)
}

func (bc *bddContext) thenApplyActionSuccessCountIs(want int) error {
	return bc.assertActionExecutionCount("db_backup_whitelist_apply", "success", want)
}

func (bc *bddContext) thenApplyActionFailureCountIs(want int) error {
	return bc.assertActionExecutionCount("db_backup_whitelist_apply", "failed", want)
}

func (bc *bddContext) thenApplyActionFailureCountAtLeast(want int) error {
	return bc.assertActionExecutionCountAtLeast("db_backup_whitelist_apply", "failed", want)
}

func (bc *bddContext) thenTicketNotCompletedNorFulfilled() error {
	if bc.ticket == nil {
		return fmt.Errorf("no ticket in context")
	}
	if err := bc.db.First(bc.ticket, bc.ticket.ID).Error; err != nil {
		return fmt.Errorf("refresh ticket: %w", err)
	}
	if bc.ticket.Status == "completed" {
		return fmt.Errorf("ticket status is completed, expected non-completed")
	}
	if bc.ticket.Outcome == "fulfilled" {
		return fmt.Errorf("ticket outcome is fulfilled, expected non-fulfilled")
	}
	return nil
}

func (bc *bddContext) thenMarkCurrentActivityAndActionRequestCounts() error {
	if bc.ticket == nil {
		return fmt.Errorf("no ticket in context")
	}
	if err := bc.db.Model(&TicketActivity{}).Where("ticket_id = ?", bc.ticket.ID).Count(&bc.activityCountMark).Error; err != nil {
		return fmt.Errorf("count activities: %w", err)
	}
	bc.actionRequestMark = bc.actionRequestCountForCurrentTicket()
	return nil
}

func (bc *bddContext) thenCurrentActivityAndActionRequestCountsUnchanged() error {
	if bc.ticket == nil {
		return fmt.Errorf("no ticket in context")
	}
	var gotActivities int64
	if err := bc.db.Model(&TicketActivity{}).Where("ticket_id = ?", bc.ticket.ID).Count(&gotActivities).Error; err != nil {
		return fmt.Errorf("count activities: %w", err)
	}
	if gotActivities != bc.activityCountMark {
		return fmt.Errorf("activity count changed: got %d, want %d", gotActivities, bc.activityCountMark)
	}
	gotRequests := bc.actionRequestCountForCurrentTicket()
	if gotRequests != bc.actionRequestMark {
		return fmt.Errorf("action request count changed: got %d, want %d", gotRequests, bc.actionRequestMark)
	}
	return nil
}

func (bc *bddContext) whenApplyReceiverRecovers() error {
	if bc.actionReceiver == nil {
		return fmt.Errorf("action receiver not initialized")
	}
	bc.actionReceiver.SetResponder("/apply", func(ActionRecord) (int, string) {
		return 200, `{"status":"ok"}`
	})
	return nil
}

func (bc *bddContext) thenDbBackupWorkflowContainsActionNodes() error {
	if bc.service == nil {
		return fmt.Errorf("service not initialized")
	}
	precheckAction, ok := bc.serviceActions["db_backup_whitelist_precheck"]
	if !ok {
		return fmt.Errorf("precheck service action not found in context")
	}
	applyAction, ok := bc.serviceActions["db_backup_whitelist_apply"]
	if !ok {
		return fmt.Errorf("apply service action not found in context")
	}

	var wf struct {
		Nodes []struct {
			ID   string `json:"id"`
			Type string `json:"type"`
			Data struct {
				Label    string `json:"label"`
				ActionID uint   `json:"action_id"`
			} `json:"data"`
		} `json:"nodes"`
		Edges []struct {
			Source string `json:"source"`
			Target string `json:"target"`
			Data   struct {
				Outcome string `json:"outcome"`
			} `json:"data"`
		} `json:"edges"`
	}
	if err := json.Unmarshal([]byte(bc.service.WorkflowJSON), &wf); err != nil {
		return fmt.Errorf("parse workflow json: %w", err)
	}

	var precheckNodeID, applyNodeID string
	for _, node := range wf.Nodes {
		if node.Type != "action" {
			continue
		}
		switch node.Data.ActionID {
		case precheckAction.ID:
			if !strings.Contains(node.Data.Label, "预检") {
				return fmt.Errorf("precheck action node label %q does not describe precheck", node.Data.Label)
			}
			precheckNodeID = node.ID
		case applyAction.ID:
			if !strings.Contains(node.Data.Label, "放行") {
				return fmt.Errorf("apply action node label %q does not describe release", node.Data.Label)
			}
			applyNodeID = node.ID
		}
	}
	if precheckNodeID == "" {
		return fmt.Errorf("workflow does not contain precheck action node bound to action_id %d", precheckAction.ID)
	}
	if applyNodeID == "" {
		return fmt.Errorf("workflow does not contain apply action node bound to action_id %d", applyAction.ID)
	}
	for _, edge := range wf.Edges {
		if edge.Source == "db_process" && edge.Data.Outcome == "rejected" && edge.Target == applyNodeID {
			return fmt.Errorf("workflow rejected edge must not enter apply action node %q", applyNodeID)
		}
	}
	return nil
}

func (bc *bddContext) assertActionTriggered(actionCode, receiverPath string) error {
	if bc.ticket == nil {
		return fmt.Errorf("no ticket in context")
	}

	action, ok := bc.serviceActions[actionCode]
	if !ok {
		return fmt.Errorf("service action %q not found in context", actionCode)
	}

	// Check TicketActionExecution record exists with status=success.
	var count int64
	if err := bc.db.Model(&TicketActionExecution{}).
		Where("ticket_id = ? AND service_action_id = ? AND status = ?", bc.ticket.ID, action.ID, "success").
		Count(&count).Error; err != nil {
		return fmt.Errorf("query action execution: %w", err)
	}
	if count == 0 {
		return fmt.Errorf("no successful execution found for action %q (id=%d) on ticket %d", actionCode, action.ID, bc.ticket.ID)
	}

	// Check LocalActionReceiver received the request.
	if bc.actionReceiver == nil {
		return fmt.Errorf("action receiver not initialized")
	}
	records := bc.actionReceiver.RecordsByPath(receiverPath)
	// Find records matching this ticket's code in the body.
	var matched bool
	for _, rec := range records {
		if strings.Contains(rec.Body, bc.ticket.Code) {
			matched = true
			break
		}
	}
	if !matched {
		return fmt.Errorf("action receiver path %q has no request matching ticket code %q (total records: %d)",
			receiverPath, bc.ticket.Code, len(records))
	}

	return nil
}

func (bc *bddContext) assertActionNotTriggered(actionCode, receiverPath string) error {
	if bc.ticket == nil {
		return fmt.Errorf("no ticket in context")
	}

	action, ok := bc.serviceActions[actionCode]
	if !ok {
		return fmt.Errorf("service action %q not found in context", actionCode)
	}

	// Check no TicketActionExecution record exists.
	var count int64
	if err := bc.db.Model(&TicketActionExecution{}).
		Where("ticket_id = ? AND service_action_id = ?", bc.ticket.ID, action.ID).
		Count(&count).Error; err != nil {
		return fmt.Errorf("query action execution: %w", err)
	}
	if count > 0 {
		return fmt.Errorf("found %d execution records for action %q on ticket %d, expected 0", count, actionCode, bc.ticket.ID)
	}

	// Check LocalActionReceiver has no request matching this ticket.
	if bc.actionReceiver != nil {
		records := bc.actionReceiver.RecordsByPath(receiverPath)
		for _, rec := range records {
			if strings.Contains(rec.Body, bc.ticket.Code) {
				return fmt.Errorf("action receiver path %q has request matching ticket code %q, expected none",
					receiverPath, bc.ticket.Code)
			}
		}
	}

	return nil
}

func (bc *bddContext) assertLatestActionPayloadContains(actionCode, receiverPath string, keys []string) error {
	if bc.ticket == nil {
		return fmt.Errorf("no ticket in context")
	}
	action, ok := bc.serviceActions[actionCode]
	if !ok {
		return fmt.Errorf("service action %q not found in context", actionCode)
	}

	var exec TicketActionExecution
	if err := bc.db.Where("ticket_id = ? AND service_action_id = ?", bc.ticket.ID, action.ID).
		Order("id DESC").First(&exec).Error; err != nil {
		return fmt.Errorf("query latest execution for action %q: %w", actionCode, err)
	}
	if err := bc.assertPayloadKeysMatchTicket(string(exec.RequestPayload), keys); err != nil {
		return fmt.Errorf("execution request payload mismatch: %w", err)
	}

	var matchedReceiverBody string
	for _, rec := range bc.actionReceiver.RecordsByPath(receiverPath) {
		if strings.Contains(rec.Body, bc.ticket.Code) {
			matchedReceiverBody = rec.Body
		}
	}
	if matchedReceiverBody == "" {
		return fmt.Errorf("receiver path %q has no request for ticket code %q", receiverPath, bc.ticket.Code)
	}
	if err := bc.assertPayloadKeysMatchTicket(matchedReceiverBody, keys); err != nil {
		return fmt.Errorf("receiver request payload mismatch: %w", err)
	}
	return nil
}

func (bc *bddContext) assertPayloadKeysMatchTicket(raw string, keys []string) error {
	var payload map[string]any
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		return fmt.Errorf("parse payload JSON: %w; raw=%s", err, raw)
	}
	formData := map[string]any{}
	if len(bc.ticket.FormData) > 0 {
		if err := json.Unmarshal([]byte(bc.ticket.FormData), &formData); err != nil {
			return fmt.Errorf("parse ticket form_data: %w", err)
		}
	}

	for _, key := range keys {
		got, exists := payload[key]
		if !exists {
			return fmt.Errorf("payload missing key %q: %v", key, payload)
		}
		gotText := strings.TrimSpace(fmt.Sprint(got))
		if gotText == "" {
			return fmt.Errorf("payload key %q is empty: %v", key, payload)
		}
		if strings.Contains(gotText, "{{ticket.form_data.") {
			return fmt.Errorf("payload key %q contains unresolved template value %q", key, gotText)
		}

		wantText := ""
		switch key {
		case "ticket_code":
			wantText = bc.ticket.Code
		default:
			want, ok := formData[key]
			if !ok {
				return fmt.Errorf("ticket form_data missing expected key %q", key)
			}
			wantText = strings.TrimSpace(fmt.Sprint(want))
		}
		if gotText != wantText {
			return fmt.Errorf("payload key %q = %q, want %q", key, gotText, wantText)
		}
	}
	return nil
}

func (bc *bddContext) assertActionExecutionCount(actionCode, status string, want int) error {
	if bc.ticket == nil {
		return fmt.Errorf("no ticket in context")
	}
	action, ok := bc.serviceActions[actionCode]
	if !ok {
		return fmt.Errorf("service action %q not found in context", actionCode)
	}
	var count int64
	query := bc.db.Model(&TicketActionExecution{}).
		Where("ticket_id = ? AND service_action_id = ?", bc.ticket.ID, action.ID)
	if status != "" {
		query = query.Where("status = ?", status)
	}
	if err := query.Count(&count).Error; err != nil {
		return fmt.Errorf("query action execution count: %w", err)
	}
	if count != int64(want) {
		return fmt.Errorf("action %q status %q count = %d, want %d", actionCode, status, count, want)
	}
	return nil
}

func (bc *bddContext) assertActionExecutionCountAtLeast(actionCode, status string, want int) error {
	if bc.ticket == nil {
		return fmt.Errorf("no ticket in context")
	}
	action, ok := bc.serviceActions[actionCode]
	if !ok {
		return fmt.Errorf("service action %q not found in context", actionCode)
	}
	var count int64
	query := bc.db.Model(&TicketActionExecution{}).
		Where("ticket_id = ? AND service_action_id = ?", bc.ticket.ID, action.ID)
	if status != "" {
		query = query.Where("status = ?", status)
	}
	if err := query.Count(&count).Error; err != nil {
		return fmt.Errorf("query action execution count: %w", err)
	}
	if count < int64(want) {
		return fmt.Errorf("action %q status %q count = %d, want at least %d", actionCode, status, count, want)
	}
	return nil
}

func (bc *bddContext) actionRequestCountForCurrentTicket() int {
	if bc.actionReceiver == nil || bc.ticket == nil {
		return 0
	}
	count := 0
	for _, rec := range bc.actionReceiver.Records() {
		if strings.Contains(rec.Body, bc.ticket.Code) {
			count++
		}
	}
	return count
}

func (bc *bddContext) thenActionRecordsIsolated(ticketRefA, ticketRefB string) error {
	ticketA, ok := bc.tickets[ticketRefA]
	if !ok {
		return fmt.Errorf("ticket alias %q not found in context", ticketRefA)
	}
	ticketB, ok := bc.tickets[ticketRefB]
	if !ok {
		return fmt.Errorf("ticket alias %q not found in context", ticketRefB)
	}

	// Check A's execution records don't contain B's ticket_id.
	var countAinB int64
	bc.db.Model(&TicketActionExecution{}).
		Where("ticket_id = ?", ticketA.ID).
		Count(&countAinB)
	if countAinB == 0 {
		return fmt.Errorf("ticket %q has no action execution records", ticketRefA)
	}

	var countBinA int64
	bc.db.Model(&TicketActionExecution{}).
		Where("ticket_id = ?", ticketB.ID).
		Count(&countBinA)
	if countBinA == 0 {
		return fmt.Errorf("ticket %q has no action execution records", ticketRefB)
	}

	// Verify no cross-contamination: A's records should only have A's ticket_id.
	var crossCount int64
	bc.db.Model(&TicketActionExecution{}).
		Where("ticket_id = ? AND ticket_id = ?", ticketA.ID, ticketB.ID).
		Count(&crossCount)
	// This query always returns 0 since ticketA.ID != ticketB.ID (unless they're the same ticket).

	// Check receiver records: A's code should not appear in B's execution payloads and vice versa.
	var execsA []TicketActionExecution
	bc.db.Where("ticket_id = ?", ticketA.ID).Find(&execsA)
	for _, e := range execsA {
		if strings.Contains(string(e.RequestPayload), ticketB.Code) {
			return fmt.Errorf("ticket %q's action execution contains ticket %q's code in request payload",
				ticketRefA, ticketRefB)
		}
	}

	var execsB []TicketActionExecution
	bc.db.Where("ticket_id = ?", ticketB.ID).Find(&execsB)
	for _, e := range execsB {
		if strings.Contains(string(e.RequestPayload), ticketA.Code) {
			return fmt.Errorf("ticket %q's action execution contains ticket %q's code in request payload",
				ticketRefB, ticketRefA)
		}
	}

	return nil
}

// --- DBW-204: Precheck failure steps ---

func (bc *bddContext) givenPrecheckReceiverTemporarilyFails() error {
	if bc.actionReceiver == nil {
		return fmt.Errorf("action receiver not initialized")
	}
	bc.actionReceiver.SetResponder("/precheck", func(ActionRecord) (int, string) {
		return 500, `{"status":"error","message":"temporary precheck failure"}`
	})
	return nil
}

func (bc *bddContext) thenPrecheckActionFailed() error {
	return bc.assertActionExecutionCountAtLeast("db_backup_whitelist_precheck", "failed", 1)
}

// --- DBW-509: Failure traceability step ---

func (bc *bddContext) thenApplyActionFailureRecordContainsDetail() error {
	if bc.ticket == nil {
		return fmt.Errorf("no ticket in context")
	}
	action, ok := bc.serviceActions["db_backup_whitelist_apply"]
	if !ok {
		return fmt.Errorf("service action %q not found in context", "db_backup_whitelist_apply")
	}
	var exec TicketActionExecution
	if err := bc.db.Where("ticket_id = ? AND service_action_id = ? AND status = ?",
		bc.ticket.ID, action.ID, "failed").
		Order("id DESC").First(&exec).Error; err != nil {
		return fmt.Errorf("no failed execution record for apply action: %w", err)
	}
	if strings.TrimSpace(exec.FailureReason) == "" {
		return fmt.Errorf("failure_reason is empty; expected non-empty (e.g. 'HTTP 500')")
	}
	if len(exec.ResponsePayload) == 0 || string(exec.ResponsePayload) == "null" || string(exec.ResponsePayload) == "" {
		return fmt.Errorf("response_payload is empty; failed action must persist response body for traceability")
	}
	return nil
}

// --- TICK-00109 regression steps ---

// whenAdminAssignsCurrentActivityTo simulates admin Assign (指派), which transfers ownership
// of the current pending activity to the named user while preserving position/department context.
func (bc *bddContext) whenAdminAssignsCurrentActivityTo(username string) error {
	assignee, ok := bc.usersByName[username]
	if !ok {
		return fmt.Errorf("user %q not found in context", username)
	}
	if bc.ticket == nil {
		return fmt.Errorf("no ticket in context")
	}
	svc := newBDDTicketService(bc)
	updated, err := svc.Assign(bc.ticket.ID, assignee.ID, 0)
	if err != nil {
		bc.lastErr = err
		return fmt.Errorf("admin assign current activity to %q: %w", username, err)
	}
	bc.ticket = updated
	return nil
}

// whenAssignedUserCompletesCurrentActivity has the recently assigned user claim and complete
// the current activity. Used together with whenAdminAssignsCurrentActivityTo.
func (bc *bddContext) whenAssignedUserCompletesCurrentActivity() error {
	if bc.ticket == nil {
		return fmt.Errorf("no ticket in context")
	}
	activity, err := bc.getCurrentActivity()
	if err != nil {
		return fmt.Errorf("get current activity: %w", err)
	}

	// Find the assignee from the current assignment.
	var assignment TicketAssignment
	if err := bc.db.Where("activity_id = ? AND is_current = true", activity.ID).
		First(&assignment).Error; err != nil {
		return fmt.Errorf("load current assignment for activity %d: %w", activity.ID, err)
	}
	if assignment.AssigneeID == nil || *assignment.AssigneeID == 0 {
		return fmt.Errorf("current assignment for activity %d has no assignee_id", activity.ID)
	}
	assigneeID := *assignment.AssigneeID

	// Mark as claimed.
	if err := bc.db.Model(&TicketAssignment{}).
		Where("id = ?", assignment.ID).
		Updates(map[string]any{"status": "claimed"}).Error; err != nil {
		return fmt.Errorf("claim assignment: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := bc.smartEngine.Progress(ctx, bc.db, engine.ProgressParams{
		TicketID:   bc.ticket.ID,
		ActivityID: activity.ID,
		Outcome:    "completed",
		OperatorID: assigneeID,
	}); err != nil {
		bc.lastErr = err
		return fmt.Errorf("progress current activity as assignee %d: %w", assigneeID, err)
	}
	bc.lastCompletedUserID = assigneeID
	return bc.db.First(bc.ticket, bc.ticket.ID).Error
}

// --- DBW-407: Cancel step ---

func (bc *bddContext) thenAllCurrentTicketActivitiesCancelled() error {
	if bc.ticket == nil {
		return fmt.Errorf("no ticket in context")
	}
	var activeCount int64
	if err := bc.db.Model(&TicketActivity{}).
		Where("ticket_id = ? AND status = ?", bc.ticket.ID, "active").
		Count(&activeCount).Error; err != nil {
		return fmt.Errorf("count active activities: %w", err)
	}
	if activeCount > 0 {
		return fmt.Errorf("expected all activities cancelled, but %d are still active", activeCount)
	}
	var pendingCount int64
	if err := bc.db.Model(&TicketActivity{}).
		Where("ticket_id = ? AND status = ?", bc.ticket.ID, "pending").
		Count(&pendingCount).Error; err != nil {
		return fmt.Errorf("count pending activities: %w", err)
	}
	if pendingCount > 0 {
		return fmt.Errorf("expected all activities cancelled, but %d are still pending", pendingCount)
	}
	return nil
}

// --- Static-workflow service publish (no LLM) ---

// givenDbBackupStaticSmartServicePublished publishes the db backup whitelist service
// using a hardcoded workflow JSON so that deterministic BDD tests run without LLM.
// It performs the same setup as publishDbBackupSmartService but skips the LLM call.
func (bc *bddContext) givenDbBackupStaticSmartServicePublished() error {
	// 0. Initialize LocalActionReceiver.
	if bc.actionReceiver == nil {
		bc.actionReceiver = NewLocalActionReceiver()
	}

	// 1. ServiceCatalog
	catalog := &ServiceCatalog{
		Name:     "数据库服务（确定性）",
		Code:     "db-services-det",
		IsActive: true,
	}
	if err := bc.db.Create(catalog).Error; err != nil {
		return fmt.Errorf("create service catalog: %w", err)
	}

	// 2. Priority
	priority := &Priority{
		Name:     "紧急",
		Code:     "urgent-db-det",
		Value:    2,
		Color:    "#f5222d",
		IsActive: true,
	}
	if err := bc.db.Create(priority).Error; err != nil {
		return fmt.Errorf("create priority: %w", err)
	}
	bc.priority = priority

	// 3. Agent
	agent := &ai.Agent{
		Name:         "流程决策智能体（确定性）",
		Type:         "assistant",
		IsActive:     true,
		Visibility:   "private",
		Strategy:     "react",
		SystemPrompt: decisionAgentSystemPrompt,
		MaxTokens:    2048,
		MaxTurns:     1,
		CreatedBy:    1,
	}
	if err := bc.db.Create(agent).Error; err != nil {
		return fmt.Errorf("create agent: %w", err)
	}
	bc.db.Model(agent).Update("temperature", 0)

	// 4. ServiceDefinition (placeholder workflow; replaced after actions are created)
	svc := &ServiceDefinition{
		Name:              "数据库备份白名单临时放行（确定性）",
		Code:              "db-backup-whitelist-det",
		CatalogID:         catalog.ID,
		EngineType:        "smart",
		WorkflowJSON:      JSONField(`{"nodes":[],"edges":[]}`),
		CollaborationSpec: dbBackupCollaborationSpec,
		IntakeFormSchema:  JSONField(dbBackupFormSchema),
		AgentID:           &agent.ID,
		IsActive:          true,
	}
	if err := bc.db.Create(svc).Error; err != nil {
		return fmt.Errorf("create service definition: %w", err)
	}
	bc.service = svc

	// 5. Create service actions
	precheckConfig, _ := json.Marshal(engine.ActionConfig{
		URL:     bc.actionReceiver.URL("/precheck"),
		Method:  "POST",
		Body:    `{"ticket_code":"{{ticket.code}}","database_name":"{{ticket.form_data.database_name}}","source_ip":"{{ticket.form_data.source_ip}}","whitelist_window":"{{ticket.form_data.whitelist_window}}","access_reason":"{{ticket.form_data.access_reason}}"}`,
		Timeout: 10,
		Retries: 0,
	})
	precheckAction := &ServiceAction{
		Name:        "数据库备份白名单预检",
		Code:        "db_backup_whitelist_precheck",
		Description: "验证数据库备份白名单放行参数的合法性",
		ActionType:  "http",
		ConfigJSON:  JSONField(precheckConfig),
		ServiceID:   svc.ID,
		IsActive:    true,
	}
	if err := bc.db.Create(precheckAction).Error; err != nil {
		return fmt.Errorf("create precheck action: %w", err)
	}
	bc.serviceActions["db_backup_whitelist_precheck"] = precheckAction

	applyConfig, _ := json.Marshal(engine.ActionConfig{
		URL:     bc.actionReceiver.URL("/apply"),
		Method:  "POST",
		Body:    `{"ticket_code":"{{ticket.code}}","database_name":"{{ticket.form_data.database_name}}","source_ip":"{{ticket.form_data.source_ip}}","whitelist_window":"{{ticket.form_data.whitelist_window}}"}`,
		Timeout: 10,
		Retries: 0,
	})
	applyAction := &ServiceAction{
		Name:        "数据库备份白名单放行",
		Code:        "db_backup_whitelist_apply",
		Description: "执行数据库备份白名单放行配置",
		ActionType:  "http",
		ConfigJSON:  JSONField(applyConfig),
		ServiceID:   svc.ID,
		IsActive:    true,
	}
	if err := bc.db.Create(applyAction).Error; err != nil {
		return fmt.Errorf("create apply action: %w", err)
	}
	bc.serviceActions["db_backup_whitelist_apply"] = applyAction

	// 6. Build static workflow JSON with real action_id values.
	staticWorkflow, err := json.Marshal(map[string]any{
		"nodes": []map[string]any{
			{"id": "start", "type": "start", "data": map[string]any{"label": "开始", "nodeType": "start"}},
			{"id": "action_precheck", "type": "action", "data": map[string]any{"label": "备份白名单预检", "nodeType": "action", "action_id": precheckAction.ID}},
			{"id": "db_process", "type": "process", "data": map[string]any{
				"label":    "数据库管理员处理",
				"nodeType": "process",
				"participants": []map[string]any{
					{"type": "position_department", "position_code": "db_admin", "department_code": "it"},
				},
			}},
			{"id": "action_apply", "type": "action", "data": map[string]any{"label": "执行备份白名单放行", "nodeType": "action", "action_id": applyAction.ID}},
			{"id": "end", "type": "end", "data": map[string]any{"label": "结束", "nodeType": "end"}},
			{"id": "end_rejected", "type": "end", "data": map[string]any{"label": "驳回结束", "nodeType": "end"}},
		},
		"edges": []map[string]any{
			{"id": "e1", "source": "start", "target": "action_precheck"},
			{"id": "e2", "source": "action_precheck", "target": "db_process"},
			{"id": "e3", "source": "db_process", "target": "action_apply", "data": map[string]any{"outcome": "completed"}},
			{"id": "e4", "source": "db_process", "target": "end_rejected", "data": map[string]any{"outcome": "rejected"}},
			{"id": "e5", "source": "action_apply", "target": "end"},
		},
	})
	if err != nil {
		return fmt.Errorf("marshal static workflow: %w", err)
	}
	if err := bc.db.Model(svc).Update("workflow_json", JSONField(staticWorkflow)).Error; err != nil {
		return fmt.Errorf("update workflow_json: %w", err)
	}
	svc.WorkflowJSON = JSONField(staticWorkflow)

	// 7. Re-wire engines with syncActionSubmitter.
	orgSvc := &testOrgService{db: bc.db}
	resolver := engine.NewParticipantResolver(orgSvc)
	bc.engine = engine.NewClassicEngine(resolver, &noopSubmitter{}, nil)
	submitter := &syncActionSubmitter{db: bc.db, classicEngine: bc.engine}
	executor := &dbBackupGuardStubExecutor{}
	userProvider := &testUserProvider{db: bc.db}
	bc.smartEngine = engine.NewSmartEngine(executor, nil, userProvider, resolver, submitter, &bddConfigProvider{bc: bc})
	bc.smartEngine.SetActionExecutor(engine.NewActionExecutor(bc.db))

	return nil
}

// dbBackupGuardStubExecutor is a deterministic executor that returns a minimal dummy plan.
// The db_backup whitelist guard will override this plan; no LLM call is made.
type dbBackupGuardStubExecutor struct{}

func (e *dbBackupGuardStubExecutor) Execute(_ context.Context, _ uint, req app.AIDecisionRequest) (*app.AIDecisionResponse, error) {
	// Return minimal JSON so parseDecisionPlan succeeds; the guard overrides it.
	content := `{"next_step_type":"process","activities":[],"reasoning":"db-backup-guard-stub","confidence":0.9,"execution_mode":"single"}`
	return &app.AIDecisionResponse{Content: content, Turns: 1}, nil
}

var _ app.AIDecisionExecutor = (*dbBackupGuardStubExecutor)(nil)
