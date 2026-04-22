package tools

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"

	"gorm.io/gorm"

	"metis/internal/app"
	"metis/internal/app/itsm/engine"
	"metis/internal/app/itsm/form"
)

// Operator is the concrete ServiceDeskOperator implementation.
// It bridges the ITSM tool handlers to the actual ITSM business services.
type Operator struct {
	db            *gorm.DB
	resolver      *engine.ParticipantResolver
	orgResolver   app.OrgResolver // nil when Org App is not installed
	withdrawFunc  func(ticketID uint, reason string, operatorID uint) error
	ticketCreator TicketCreator // nil-safe: falls back to error if not set
	matcher       ServiceMatcher
}

// NewOperator creates a new ServiceDeskOperator.
func NewOperator(db *gorm.DB, resolver *engine.ParticipantResolver, orgResolver app.OrgResolver, withdrawFunc func(uint, string, uint) error, ticketCreator TicketCreator, matcher ServiceMatcher) *Operator {
	return &Operator{db: db, resolver: resolver, orgResolver: orgResolver, withdrawFunc: withdrawFunc, ticketCreator: ticketCreator, matcher: matcher}
}

// MatchServices delegates service matching to the configured matcher.
func (o *Operator) MatchServices(ctx context.Context, query string) ([]ServiceMatch, MatchDecision, error) {
	if o.matcher == nil {
		return nil, MatchDecision{}, fmt.Errorf("service matcher is not configured")
	}
	return o.matcher.MatchServices(ctx, query)
}

// LoadService loads a service's full detail including form fields, actions, and routing hints.
func (o *Operator) LoadService(serviceID uint) (*ServiceDetail, error) {
	type svcRow struct {
		ID                uint
		Name              string
		CollaborationSpec string
		IntakeFormSchema  string
		WorkflowJSON      string
	}
	var svc svcRow
	if err := o.db.Table("itsm_service_definitions").
		Where("id = ? AND deleted_at IS NULL", serviceID).
		First(&svc).Error; err != nil {
		return nil, fmt.Errorf("service not found: %w", err)
	}

	detail := &ServiceDetail{
		ServiceID:         svc.ID,
		Name:              svc.Name,
		CollaborationSpec: svc.CollaborationSpec,
	}

	// Load form fields from inline intake form schema.
	if svc.IntakeFormSchema != "" {
		detail.FormFields = parseFormFields(svc.IntakeFormSchema)
	}

	// Load actions.
	type actionRow struct {
		ID   uint
		Code string
		Name string
	}
	var actions []actionRow
	o.db.Table("itsm_service_actions").
		Where("service_id = ? AND is_active = ? AND deleted_at IS NULL", serviceID, true).
		Select("id, code, name").
		Order("id ASC").
		Find(&actions)
	for _, a := range actions {
		detail.Actions = append(detail.Actions, ActionInfo{ID: a.ID, Code: a.Code, Name: a.Name})
	}

	// Extract routing field hint from workflow_json.
	if svc.WorkflowJSON != "" {
		detail.RoutingFieldHint = extractRoutingHint(svc.WorkflowJSON)
	}

	// Compute fields hash.
	detail.FieldsHash = computeFieldsHash(detail.FormFields)

	return detail, nil
}

// CreateTicket creates an ITSM ticket via the TicketService, ensuring full lifecycle
// processing (SLA, engine start, timeline) identical to UI-created tickets.
func (o *Operator) CreateTicket(userID uint, serviceID uint, summary string, formData map[string]any, sessionID uint) (*TicketResult, error) {
	if o.ticketCreator == nil {
		return nil, fmt.Errorf("ticket creation is not available")
	}

	result, err := o.ticketCreator.CreateFromAgent(context.Background(), AgentTicketRequest{
		UserID:    userID,
		ServiceID: serviceID,
		Summary:   summary,
		FormData:  formData,
		SessionID: sessionID,
	})
	if err != nil {
		return nil, fmt.Errorf("create ticket: %w", err)
	}

	return &TicketResult{
		TicketID:   result.TicketID,
		TicketCode: result.TicketCode,
		Status:     result.Status,
	}, nil
}

// ListMyTickets returns the user's non-terminal tickets.
func (o *Operator) ListMyTickets(userID uint, status string) ([]TicketSummary, error) {
	type row struct {
		ID        uint
		Code      string
		Title     string
		Status    string
		ServiceID uint
		CreatedAt string
	}

	query := o.db.Table("itsm_tickets").
		Where("requester_id = ? AND deleted_at IS NULL", userID).
		Where("status NOT IN ?", []string{"completed", "cancelled", "failed"})
	if status != "" {
		query = query.Where("status = ?", status)
	}

	var rows []row
	if err := query.Select("id, code, title, status, service_id, created_at").
		Order("created_at DESC").Limit(20).Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("list tickets: %w", err)
	}

	// Batch load service names.
	serviceNames := make(map[uint]string)
	var serviceIDs []uint
	for _, r := range rows {
		serviceIDs = append(serviceIDs, r.ServiceID)
	}
	if len(serviceIDs) > 0 {
		type sn struct {
			ID   uint
			Name string
		}
		var names []sn
		o.db.Table("itsm_service_definitions").Where("id IN ?", serviceIDs).
			Select("id, name").Find(&names)
		for _, n := range names {
			serviceNames[n.ID] = n.Name
		}
	}

	var summaries []TicketSummary
	for _, r := range rows {
		// can_withdraw: non-terminal + no claimed assignments
		canWithdraw := false
		if r.Status != "completed" && r.Status != "cancelled" && r.Status != "failed" {
			var claimedCount int64
			o.db.Table("itsm_ticket_assignments").
				Where("ticket_id = ? AND claimed_at IS NOT NULL", r.ID).
				Count(&claimedCount)
			canWithdraw = claimedCount == 0
		}

		summaries = append(summaries, TicketSummary{
			TicketID:    r.ID,
			TicketCode:  r.Code,
			Summary:     r.Title,
			Status:      r.Status,
			ServiceName: serviceNames[r.ServiceID],
			CreatedAt:   r.CreatedAt,
			CanWithdraw: canWithdraw,
		})
	}

	return summaries, nil
}

// WithdrawTicket cancels a ticket if the requester owns it and it hasn't been claimed.
func (o *Operator) WithdrawTicket(userID uint, ticketCode string, reason string) error {
	// Resolve ticket_code → ticket_id.
	type ticketRow struct {
		ID uint
	}
	var t ticketRow
	if err := o.db.Table("itsm_tickets").
		Where("code = ? AND deleted_at IS NULL", ticketCode).
		Select("id").First(&t).Error; err != nil {
		return fmt.Errorf("工单不存在: %s", ticketCode)
	}

	return o.withdrawFunc(t.ID, reason, userID)
}

// ValidateParticipants checks if workflow participants can be resolved.
func (o *Operator) ValidateParticipants(serviceID uint, formData map[string]any) (*ParticipantValidation, error) {
	// Load workflow JSON.
	var svc struct {
		WorkflowJSON string
	}
	if err := o.db.Table("itsm_service_definitions").
		Where("id = ? AND deleted_at IS NULL", serviceID).
		Select("workflow_json").First(&svc).Error; err != nil {
		return nil, fmt.Errorf("service not found: %w", err)
	}

	if svc.WorkflowJSON == "" {
		// No workflow defined — skip validation.
		return &ParticipantValidation{OK: true}, nil
	}

	// Parse workflow nodes.
	var workflow struct {
		Nodes []struct {
			ID    string `json:"id"`
			Type  string `json:"type"`
			Label string `json:"label"`
			Data  struct {
				ParticipantType string `json:"participantType"`
				PositionCode    string `json:"positionCode"`
				DepartmentCode  string `json:"departmentCode"`
				UserID          *uint  `json:"userId"`
			} `json:"data"`
		} `json:"nodes"`
	}
	if err := json.Unmarshal([]byte(svc.WorkflowJSON), &workflow); err != nil {
		return &ParticipantValidation{OK: true}, nil // Can't parse, skip.
	}

	// Check each approve/process node.
	for _, node := range workflow.Nodes {
		if node.Type != "approve" && node.Type != "process" {
			continue
		}
		d := node.Data
		if d.ParticipantType == "user" && d.UserID != nil {
			// Direct user assignment — check user exists and is active.
			var count int64
			o.db.Table("users").Where("id = ? AND is_active = ?", *d.UserID, true).Count(&count)
			if count == 0 {
				return &ParticipantValidation{
					OK:            false,
					FailureReason: fmt.Sprintf("指定用户(ID=%d)不存在或已停用", *d.UserID),
					NodeLabel:     node.Label,
					Guidance:      "请联系管理员检查用户状态",
				}, nil
			}
		}
		if d.ParticipantType == "position" && d.PositionCode != "" {
			if o.orgResolver == nil {
				// Org App not installed — skip position validation.
				continue
			}
			// Position-based — check if any active user holds the position.
			var userIDs []uint
			var err error
			if d.DepartmentCode != "" {
				userIDs, err = o.orgResolver.FindUsersByPositionAndDepartment(d.PositionCode, d.DepartmentCode)
			} else {
				userIDs, err = o.orgResolver.FindUsersByPositionCode(d.PositionCode)
			}
			if err != nil {
				return nil, fmt.Errorf("validate participants: %w", err)
			}
			if len(userIDs) == 0 {
				reason := fmt.Sprintf("岗位[%s]", d.PositionCode)
				if d.DepartmentCode != "" {
					reason += fmt.Sprintf("+部门[%s]", d.DepartmentCode)
				}
				reason += " 下无可用人员"
				return &ParticipantValidation{
					OK:            false,
					FailureReason: reason,
					NodeLabel:     node.Label,
					Guidance:      "请联系 IT 管理员补充人员配置后再提单",
				}, nil
			}
		}
	}

	return &ParticipantValidation{OK: true}, nil
}

// --- helpers ---

func parseFormFields(schemaJSON string) []FormField {
	var schema form.FormSchema
	if err := json.Unmarshal([]byte(schemaJSON), &schema); err != nil {
		return nil
	}

	var fields []FormField
	for _, f := range schema.Fields {
		ff := FormField{
			Key:      f.Key,
			Label:    f.Label,
			Type:     f.Type,
			Required: f.Required,
		}
		// Check validation rules for "required".
		for _, v := range f.Validation {
			if v.Rule == "required" {
				ff.Required = true
			}
		}
		// Extract options for select/radio fields.
		for _, opt := range f.Options {
			if label, ok := opt.Label, true; ok {
				ff.Options = append(ff.Options, label)
			}
		}
		fields = append(fields, ff)
	}

	return fields
}

func computeFieldsHash(fields []FormField) string {
	b, _ := json.Marshal(fields)
	h := sha256.Sum256(b)
	return fmt.Sprintf("%x", h[:8])
}

func extractRoutingHint(workflowJSON string) *RoutingFieldHint {
	var workflow struct {
		Nodes []struct {
			Type string `json:"type"`
			Data struct {
				Conditions []struct {
					Field string `json:"field"`
					Value string `json:"value"`
					Label string `json:"label"`
				} `json:"conditions"`
			} `json:"data"`
		} `json:"nodes"`
	}
	if err := json.Unmarshal([]byte(workflowJSON), &workflow); err != nil {
		return nil
	}

	for _, node := range workflow.Nodes {
		if node.Type != "exclusive_gateway" {
			continue
		}
		if len(node.Data.Conditions) == 0 {
			continue
		}

		fieldKey := node.Data.Conditions[0].Field
		if fieldKey == "" {
			continue
		}

		routeMap := make(map[string]string)
		for _, c := range node.Data.Conditions {
			if c.Value != "" {
				label := c.Label
				if label == "" {
					label = c.Value
				}
				routeMap[c.Value] = label
			}
		}

		if len(routeMap) > 0 {
			return &RoutingFieldHint{
				FieldKey:       fieldKey,
				OptionRouteMap: routeMap,
			}
		}
	}

	return nil
}
