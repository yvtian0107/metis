package bdd

// steps_vpn_classic_test.go — step definitions for the VPN classic engine BDD scenarios.

import (
	"context"
	"encoding/json"
	"fmt"
	. "metis/internal/app/itsm/domain"
	"time"

	"github.com/cucumber/godog"

	"metis/internal/app/itsm/engine"
)

// registerClassicSteps registers all classic engine step definitions.
func registerClassicSteps(sc *godog.ScenarioContext, bc *bddContext) {
	sc.Given(`^已基于协作规范发布 VPN 开通服务（经典引擎）$`, bc.givenClassicServicePublished)
	sc.When(`^"([^"]*)" 提交 VPN 申请，访问原因为 "([^"]*)"$`, bc.whenSubmitVPNRequest)
	sc.Then(`^当前活动类型为 "([^"]*)"$`, bc.thenCurrentActivityTypeIs)
	sc.Then(`^当前活动分配给 "([^"]*)" 所属的 ([^/]+)/([^"]+)$`, bc.thenCurrentActivityAssignedTo)
	sc.Then(`^当前活动未分配给 "([^"]*)"$`, bc.thenCurrentActivityNotAssignedTo)
	sc.When(`^"([^"]*)" 认领并处理完成当前工单$`, bc.whenClaimAndProcess)
}

// givenClassicServicePublished generates a VPN workflow via LLM and publishes a classic service definition.
func (bc *bddContext) givenClassicServicePublished() error {
	return publishVPNService(bc, bc.llmCfg)
}

// whenSubmitVPNRequest creates a ticket in the DB, starts the classic engine,
// and auto-submits the initial form activity to advance to the process step.
//
// requestKind is mapped to the gateway routing field:
//   - "network_support" → first condition branch (network admin)
//   - "external_collaboration" → second condition branch (security admin)
func (bc *bddContext) whenSubmitVPNRequest(username, requestKind string) error {
	user, ok := bc.usersByName[username]
	if !ok {
		return fmt.Errorf("user %q not found in context", username)
	}

	// Parse the generated workflow to build form data that matches gateway conditions.
	formData, err := buildFormDataFromWorkflow(json.RawMessage(bc.service.WorkflowJSON), requestKind)
	if err != nil {
		return fmt.Errorf("build form data from workflow: %w", err)
	}
	formJSON, _ := json.Marshal(formData)

	ticket := &Ticket{
		Code:         fmt.Sprintf("VPN-%d", time.Now().UnixNano()),
		Title:        fmt.Sprintf("VPN开通申请 - %s", requestKind),
		ServiceID:    bc.service.ID,
		EngineType:   "classic",
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

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	err = bc.engine.Start(ctx, bc.db, engine.StartParams{
		TicketID:      ticket.ID,
		WorkflowJSON:  json.RawMessage(bc.service.WorkflowJSON),
		RequesterID:   user.ID,
		StartFormData: string(formJSON),
	})
	if err != nil {
		bc.lastErr = err
		return fmt.Errorf("classic engine start: %w", err)
	}

	// Refresh ticket from DB.
	if err := bc.db.First(bc.ticket, bc.ticket.ID).Error; err != nil {
		return fmt.Errorf("refresh ticket: %w", err)
	}

	// Auto-submit the form activity if the current activity is a form.
	activity, err := bc.getCurrentActivity()
	if err != nil {
		return nil // No pending activity is fine (engine may have auto-advanced)
	}
	if activity.ActivityType == "form" {
		err = bc.engine.Progress(ctx, bc.db, engine.ProgressParams{
			TicketID:   ticket.ID,
			ActivityID: activity.ID,
			Outcome:    "submitted",
			Result:     formJSON,
			OperatorID: user.ID,
		})
		if err != nil {
			bc.lastErr = err
			return fmt.Errorf("auto-submit form: %w", err)
		}
		// Refresh ticket.
		bc.db.First(bc.ticket, bc.ticket.ID)
	}

	return nil
}

// buildFormDataFromWorkflow parses the LLM-generated workflow to find the exclusive
// gateway condition field and builds form data that will route to the correct branch.
//
// requestKind selects which branch:
//   - "network_support" → first condition edge's first value
//   - "external_collaboration" → second condition edge's first value
func buildFormDataFromWorkflow(workflowJSON json.RawMessage, requestKind string) (map[string]any, error) {
	var wf struct {
		Edges []struct {
			Source string `json:"source"`
			Data   struct {
				Condition *struct {
					Field string `json:"field"`
					Value any    `json:"value"`
				} `json:"condition,omitempty"`
			} `json:"data"`
		} `json:"edges"`
		Nodes []struct {
			ID   string `json:"id"`
			Type string `json:"type"`
		} `json:"nodes"`
	}
	if err := json.Unmarshal(workflowJSON, &wf); err != nil {
		return nil, fmt.Errorf("parse workflow: %w", err)
	}

	// Find the exclusive gateway node.
	var gatewayID string
	for _, n := range wf.Nodes {
		if n.Type == "exclusive" {
			gatewayID = n.ID
			break
		}
	}
	if gatewayID == "" {
		return nil, fmt.Errorf("no exclusive gateway found in workflow")
	}

	// Collect condition edges from the gateway, preserving order.
	type condEdge struct {
		field string
		value any
	}
	var condEdges []condEdge
	for _, e := range wf.Edges {
		if e.Source == gatewayID && e.Data.Condition != nil {
			condEdges = append(condEdges, condEdge{
				field: e.Data.Condition.Field,
				value: e.Data.Condition.Value,
			})
		}
	}
	// Select branch: first edge for network_support, second for external_collaboration.
	var selected condEdge
	switch requestKind {
	case "network_support":
		if len(condEdges) < 1 {
			return nil, fmt.Errorf("expected at least 1 condition edge from gateway, got %d", len(condEdges))
		}
		selected = condEdges[0]
	case "external_collaboration":
		if len(condEdges) < 2 {
			return nil, fmt.Errorf("expected at least 2 condition edges from gateway, got %d", len(condEdges))
		}
		selected = condEdges[1]
	default:
		return nil, fmt.Errorf("unknown request kind %q", requestKind)
	}

	// Extract the form field name (strip "form." prefix).
	fieldName := selected.field
	if len(fieldName) > 5 && fieldName[:5] == "form." {
		fieldName = fieldName[5:]
	}

	// Pick the first value from the condition's value list.
	var formValue string
	switch v := selected.value.(type) {
	case []any:
		if len(v) > 0 {
			formValue = fmt.Sprintf("%v", v[0])
		}
	case string:
		formValue = v
	default:
		formValue = fmt.Sprintf("%v", v)
	}

	return map[string]any{
		fieldName: formValue,
	}, nil
}

// thenCurrentActivityTypeIs asserts the current activity's type.
func (bc *bddContext) thenCurrentActivityTypeIs(expected string) error {
	activity, err := bc.getCurrentActivity()
	if err != nil {
		return err
	}
	if activity.ActivityType != expected {
		return fmt.Errorf("expected activity type %q, got %q", expected, activity.ActivityType)
	}
	return nil
}

// thenCurrentActivityAssignedTo asserts the current activity is assigned to a user
// who belongs to the specified position/department combination.
func (bc *bddContext) thenCurrentActivityAssignedTo(username, deptCode, posCode string) error {
	user, ok := bc.usersByName[username]
	if !ok {
		return fmt.Errorf("user %q not found in context", username)
	}

	activity, err := bc.getCurrentActivity()
	if err != nil {
		return err
	}

	var assignments []TicketAssignment
	if err := bc.db.Where("activity_id = ?", activity.ID).Find(&assignments).Error; err != nil {
		return fmt.Errorf("query assignments: %w", err)
	}
	if len(assignments) == 0 {
		return fmt.Errorf("no assignments found for activity %d", activity.ID)
	}

	// Verify user belongs to the expected position+department.
	orgSvc := &testOrgService{db: bc.db}
	userIDs, err := orgSvc.FindUsersByPositionAndDepartment(posCode, deptCode)
	if err != nil {
		return fmt.Errorf("resolve users for %s/%s: %w", deptCode, posCode, err)
	}
	userInPosition := false
	for _, uid := range userIDs {
		if uid == user.ID {
			userInPosition = true
			break
		}
	}
	if !userInPosition {
		return fmt.Errorf("user %q (ID=%d) is not in position %s/%s", username, user.ID, deptCode, posCode)
	}

	// Check that user is among the assignees (directly or via position/department).
	for _, a := range assignments {
		// Classic engine assigns directly via AssigneeID.
		if a.AssigneeID != nil && *a.AssigneeID == user.ID {
			return nil
		}
		// Also check via PositionID/DepartmentID if set.
		if a.PositionID != nil && a.DepartmentID != nil {
			pos, posOK := bc.positions[posCode]
			dept, deptOK := bc.departments[deptCode]
			if posOK && deptOK && *a.PositionID == pos.ID && *a.DepartmentID == dept.ID {
				return nil
			}
		}
	}

	return fmt.Errorf("user %q not found in assignments for activity %d", username, activity.ID)
}

// thenCurrentActivityNotAssignedTo asserts the current activity is NOT assigned to a specific user.
func (bc *bddContext) thenCurrentActivityNotAssignedTo(username string) error {
	user, ok := bc.usersByName[username]
	if !ok {
		return fmt.Errorf("user %q not found in context", username)
	}

	activity, err := bc.getCurrentActivity()
	if err != nil {
		return err
	}

	var assignments []TicketAssignment
	if err := bc.db.Where("activity_id = ?", activity.ID).Find(&assignments).Error; err != nil {
		return fmt.Errorf("query assignments: %w", err)
	}

	// Check: user must not be directly assigned.
	for _, a := range assignments {
		if a.AssigneeID != nil && *a.AssigneeID == user.ID {
			return fmt.Errorf("activity %d is directly assigned to user %q", activity.ID, username)
		}
	}

	return nil
}

// whenClaimAndProcess finds the current activity, claims it for the user, and progresses with "completed".
func (bc *bddContext) whenClaimAndProcess(username string) error {
	user, ok := bc.usersByName[username]
	if !ok {
		return fmt.Errorf("user %q not found in context", username)
	}

	activity, err := bc.getCurrentActivity()
	if err != nil {
		return err
	}

	// Claim: update assignment.
	if err := bc.db.Model(&TicketAssignment{}).
		Where("activity_id = ?", activity.ID).
		Updates(map[string]any{"assignee_id": user.ID, "status": "claimed"}).Error; err != nil {
		return fmt.Errorf("claim assignment: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	err = bc.engine.Progress(ctx, bc.db, engine.ProgressParams{
		TicketID:   bc.ticket.ID,
		ActivityID: activity.ID,
		Outcome:    "approved",
		OperatorID: user.ID,
	})
	if err != nil {
		bc.lastErr = err
		return fmt.Errorf("classic engine progress: %w", err)
	}

	// Refresh ticket.
	return bc.db.First(bc.ticket, bc.ticket.ID).Error
}

// getCurrentActivity returns the latest non-completed activity for the current ticket.
func (bc *bddContext) getCurrentActivity() (*TicketActivity, error) {
	if bc.ticket == nil {
		return nil, fmt.Errorf("no ticket in context")
	}

	var activity TicketActivity
	err := bc.db.Where("ticket_id = ? AND status IN ?", bc.ticket.ID, []string{"pending", "in_progress", "pending"}).
		Order("id DESC").First(&activity).Error
	if err != nil {
		return nil, fmt.Errorf("no active activity found for ticket %d: %w", bc.ticket.ID, err)
	}
	return &activity, nil
}
