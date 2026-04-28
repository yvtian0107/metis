package tools

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"strings"

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
		EngineType        string
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
		EngineType:        svc.EngineType,
		CollaborationSpec: svc.CollaborationSpec,
	}

	// Load form fields from inline intake form schema.
	if svc.IntakeFormSchema != "" {
		detail.FormFields = parseFormFields(svc.IntakeFormSchema)
		var schema any
		if err := json.Unmarshal([]byte(svc.IntakeFormSchema), &schema); err == nil {
			detail.FormSchema = schema
		}
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
		if hint := extractRoutingHint(svc.WorkflowJSON); routingHintSupported(detail.FormFields, hint) {
			detail.RoutingFieldHint = hint
		}
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

// SubmitConfirmedDraft creates an ITSM ticket from a user-confirmed service desk
// draft. It carries the draft identity so TicketService can enforce idempotency
// and preserve the human confirmation boundary.
func (o *Operator) SubmitConfirmedDraft(userID uint, serviceID uint, summary string, formData map[string]any, sessionID uint, draftVersion int, fieldsHash string, requestHash string) (*TicketResult, error) {
	if o.ticketCreator == nil {
		return nil, fmt.Errorf("ticket creation is not available")
	}

	result, err := o.ticketCreator.CreateFromAgent(context.Background(), AgentTicketRequest{
		UserID:       userID,
		ServiceID:    serviceID,
		Summary:      summary,
		FormData:     formData,
		SessionID:    sessionID,
		DraftVersion: draftVersion,
		FieldsHash:   fieldsHash,
		RequestHash:  requestHash,
	})
	if err != nil {
		return nil, fmt.Errorf("submit confirmed draft: %w", err)
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
		Where("status NOT IN ?", []string{"completed", "rejected", "withdrawn", "cancelled", "failed"})
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
		if r.Status != "completed" && r.Status != "rejected" && r.Status != "withdrawn" && r.Status != "cancelled" && r.Status != "failed" {
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
// Uses engine.ParticipantResolver for consistent validation with the runtime engine.
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
		return &ParticipantValidation{OK: true}, nil
	}

	def, err := engine.ParseWorkflowDef(json.RawMessage(svc.WorkflowJSON))
	if err != nil {
		return &ParticipantValidation{OK: true}, nil // Can't parse, skip.
	}

	for _, node := range def.Nodes {
		if node.Type != "process" && node.Type != "form" {
			continue
		}
		nd, err := engine.ParseNodeData(node.Data)
		if err != nil || len(nd.Participants) == 0 {
			continue
		}
		nodeLabel := nd.Label
		if nodeLabel == "" {
			nodeLabel = node.ID
		}

		for _, p := range nd.Participants {
			// Skip requester/requester_manager — can't validate before ticket exists
			if p.Type == "requester" || p.Type == "requester_manager" {
				continue
			}

			if o.resolver == nil {
				continue
			}
			userIDs, err := o.resolver.Resolve(o.db, 0, p)
			if err != nil {
				// orgResolver nil errors mean the org module isn't installed — skip
				if strings.Contains(err.Error(), "安装组织架构模块") {
					continue
				}
				return &ParticipantValidation{
					OK:            false,
					FailureReason: err.Error(),
					NodeLabel:     nodeLabel,
					Guidance:      "请联系管理员检查参与人配置",
				}, nil
			}
			if len(userIDs) == 0 {
				reason := fmt.Sprintf("节点[%s]的参与人（%s）无可用人员", nodeLabel, p.Type)
				if p.PositionCode != "" {
					reason = fmt.Sprintf("岗位[%s]", p.PositionCode)
					if p.DepartmentCode != "" {
						reason += fmt.Sprintf("+部门[%s]", p.DepartmentCode)
					}
					reason += " 下无可用人员"
				}
				return &ParticipantValidation{
					OK:            false,
					FailureReason: reason,
					NodeLabel:     nodeLabel,
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
			Key:          f.Key,
			Label:        f.Label,
			Type:         f.Type,
			Description:  f.Description,
			Placeholder:  f.Placeholder,
			DefaultValue: f.DefaultValue,
			Required:     f.Required,
			Validation:   f.Validation,
			Props:        f.Props,
		}
		// Check validation rules for "required".
		for _, v := range f.Validation {
			if v.Rule == "required" {
				ff.Required = true
			}
		}
		for _, opt := range f.Options {
			if opt.Label != "" || opt.Value != nil {
				ff.Options = append(ff.Options, FormOption{
					Label: opt.Label,
					Value: fmt.Sprintf("%v", opt.Value),
				})
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

func routingHintSupported(fields []FormField, hint *RoutingFieldHint) bool {
	if hint == nil || hint.FieldKey == "" || len(hint.OptionRouteMap) == 0 {
		return false
	}
	for _, field := range fields {
		if field.Key != hint.FieldKey {
			continue
		}
		switch field.Type {
		case "select", "radio", "multi_select", "checkbox":
			return true
		default:
			return false
		}
	}
	return false
}

func extractRoutingHint(workflowJSON string) *RoutingFieldHint {
	var workflow struct {
		Nodes []struct {
			ID    string `json:"id"`
			Label string `json:"label"`
			Data  struct {
				Label string `json:"label"`
			} `json:"data"`
		} `json:"nodes"`
		Edges []struct {
			Target string `json:"target"`
			Data   struct {
				Condition struct {
					Field string `json:"field"`
					Value any    `json:"value"`
				} `json:"condition"`
			} `json:"data"`
		} `json:"edges"`
	}
	if err := json.Unmarshal([]byte(workflowJSON), &workflow); err != nil {
		return nil
	}

	nodeLabels := make(map[string]string, len(workflow.Nodes))
	for _, node := range workflow.Nodes {
		label := node.Data.Label
		if label == "" {
			label = node.Label
		}
		nodeLabels[node.ID] = label
	}

	var fieldKey string
	routeMap := make(map[string]string)
	for _, edge := range workflow.Edges {
		condition := edge.Data.Condition
		if condition.Field == "" {
			continue
		}
		normalizedField := normalizeRoutingFieldKey(condition.Field)
		if normalizedField == "" {
			continue
		}
		if fieldKey == "" {
			fieldKey = normalizedField
		}
		if fieldKey != normalizedField {
			continue
		}
		routeLabel := nodeLabels[edge.Target]
		for _, value := range routingConditionValues(condition.Value) {
			if value == "" {
				continue
			}
			if routeLabel == "" {
				routeLabel = value
			}
			routeMap[value] = routeLabel
		}
	}
	if fieldKey == "" || len(routeMap) == 0 {
		return nil
	}
	return &RoutingFieldHint{FieldKey: fieldKey, OptionRouteMap: routeMap}
}

func normalizeRoutingFieldKey(field string) string {
	field = strings.TrimSpace(field)
	field = strings.TrimPrefix(field, "form.")
	return field
}

func routingConditionValues(raw any) []string {
	switch value := raw.(type) {
	case string:
		return []string{value}
	case []any:
		values := make([]string, 0, len(value))
		for _, item := range value {
			values = append(values, strings.TrimSpace(fmt.Sprintf("%v", item)))
		}
		return values
	default:
		if raw == nil {
			return nil
		}
		return []string{strings.TrimSpace(fmt.Sprintf("%v", raw))}
	}
}
