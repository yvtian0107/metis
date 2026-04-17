package tools

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"strings"

	"gorm.io/gorm"

	"metis/internal/app/itsm/engine"
	"metis/internal/app/itsm/form"
)

// Operator is the concrete ServiceDeskOperator implementation.
// It bridges the ITSM tool handlers to the actual ITSM business services.
type Operator struct {
	db           *gorm.DB
	resolver     *engine.ParticipantResolver
	withdrawFunc func(ticketID uint, reason string, operatorID uint) error
}

// NewOperator creates a new ServiceDeskOperator.
func NewOperator(db *gorm.DB, resolver *engine.ParticipantResolver, withdrawFunc func(uint, string, uint) error) *Operator {
	return &Operator{db: db, resolver: resolver, withdrawFunc: withdrawFunc}
}

// MatchServices searches active ServiceDefinitions by keyword scoring.
func (o *Operator) MatchServices(query string) ([]ServiceMatch, error) {
	type svcRow struct {
		ID          uint
		Name        string
		Description string
		CatalogID   uint
	}
	var rows []svcRow
	if err := o.db.Table("itsm_service_definitions").
		Where("is_active = ? AND deleted_at IS NULL", true).
		Select("id, name, description, catalog_id").
		Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("query services: %w", err)
	}

	queryLower := strings.ToLower(query)
	keywords := tokenize(queryLower)

	var matches []ServiceMatch
	for _, r := range rows {
		score := computeScore(queryLower, keywords, r.Name, r.Description)
		if score <= 0 {
			continue
		}
		reason := matchReason(queryLower, keywords, r.Name, r.Description)
		catalogPath := o.buildCatalogPath(r.CatalogID)

		matches = append(matches, ServiceMatch{
			ID:          r.ID,
			Name:        r.Name,
			CatalogPath: catalogPath,
			Description: truncate(r.Description, 100),
			Score:       score,
			Reason:      reason,
		})
	}

	// Sort by score descending, take top 3.
	sortMatches(matches)
	if len(matches) > 3 {
		matches = matches[:3]
	}

	return matches, nil
}

// LoadService loads a service's full detail including form fields, actions, and routing hints.
func (o *Operator) LoadService(serviceID uint) (*ServiceDetail, error) {
	type svcRow struct {
		ID                uint
		Name              string
		CollaborationSpec string
		FormID            *uint
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

	// Load form fields.
	if svc.FormID != nil && *svc.FormID > 0 {
		var fd struct {
			Schema string
		}
		if err := o.db.Table("itsm_form_definitions").
			Where("id = ?", *svc.FormID).
			Select("schema").First(&fd).Error; err == nil && fd.Schema != "" {
			detail.FormFields = parseFormFields(fd.Schema)
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
		detail.RoutingFieldHint = extractRoutingHint(svc.WorkflowJSON)
	}

	// Compute fields hash.
	detail.FieldsHash = computeFieldsHash(detail.FormFields)

	return detail, nil
}

// CreateTicket creates an ITSM ticket via the database.
func (o *Operator) CreateTicket(userID uint, serviceID uint, summary string, formData map[string]any, sessionID uint) (*TicketResult, error) {
	// Get next ticket code.
	code, err := o.nextTicketCode()
	if err != nil {
		return nil, fmt.Errorf("generate ticket code: %w", err)
	}

	formJSON, _ := json.Marshal(formData)

	// Look up default priority.
	var priorityID uint
	o.db.Table("itsm_priorities").Where("is_active = ? AND deleted_at IS NULL", true).
		Order("value ASC").Select("id").Limit(1).Row().Scan(&priorityID)
	if priorityID == 0 {
		priorityID = 1
	}

	// Copy engine type and workflow from service.
	var svc struct {
		EngineType   string
		WorkflowJSON string
	}
	o.db.Table("itsm_service_definitions").Where("id = ?", serviceID).
		Select("engine_type, workflow_json").First(&svc)

	ticket := map[string]any{
		"code":             code,
		"title":            summary,
		"description":      summary,
		"service_id":       serviceID,
		"engine_type":      svc.EngineType,
		"status":           "pending",
		"priority_id":      priorityID,
		"requester_id":     userID,
		"source":           "agent",
		"agent_session_id": sessionID,
		"form_data":        string(formJSON),
		"workflow_json":    svc.WorkflowJSON,
		"sla_status":       "on_track",
	}

	result := o.db.Table("itsm_tickets").Create(ticket)
	if result.Error != nil {
		return nil, fmt.Errorf("create ticket: %w", result.Error)
	}

	// Read back the created ticket ID.
	var created struct{ ID uint }
	o.db.Table("itsm_tickets").Where("code = ?", code).Select("id").First(&created)

	return &TicketResult{
		TicketID:   created.ID,
		TicketCode: code,
		Status:     "pending",
	}, nil
}

// ListMyTickets returns the user's non-terminal tickets.
func (o *Operator) ListMyTickets(userID uint, status string) ([]TicketSummary, error) {
	type row struct {
		ID          uint
		Code        string
		Title       string
		Status      string
		ServiceID   uint
		CreatedAt   string
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
			ID       string `json:"id"`
			Type     string `json:"type"`
			Label    string `json:"label"`
			Data     struct {
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
			// Position-based — check if any active user holds the position.
			var count int64
			q := o.db.Table("user_positions").
				Joins("JOIN positions ON positions.id = user_positions.position_id").
				Joins("JOIN users ON users.id = user_positions.user_id").
				Where("positions.code = ? AND users.is_active = ?", d.PositionCode, true)
			if d.DepartmentCode != "" {
				q = q.Joins("JOIN departments ON departments.id = user_positions.department_id").
					Where("departments.code = ?", d.DepartmentCode)
			}
			q.Count(&count)
			if count == 0 {
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

func (o *Operator) buildCatalogPath(catalogID uint) string {
	var parts []string
	currentID := catalogID
	for i := 0; i < 5; i++ { // max depth
		type cat struct {
			Name     string
			ParentID *uint
		}
		var c cat
		if err := o.db.Table("itsm_service_catalogs").
			Where("id = ?", currentID).
			Select("name, parent_id").First(&c).Error; err != nil {
			break
		}
		parts = append([]string{c.Name}, parts...)
		if c.ParentID == nil || *c.ParentID == 0 {
			break
		}
		currentID = *c.ParentID
	}
	return strings.Join(parts, "/")
}

func (o *Operator) nextTicketCode() (string, error) {
	// Use a simple counter approach: find max code and increment.
	var maxCode string
	o.db.Table("itsm_tickets").Select("code").Order("id DESC").Limit(1).Row().Scan(&maxCode)

	if maxCode == "" {
		return "ITSM-000001", nil
	}

	// Parse numeric part.
	parts := strings.Split(maxCode, "-")
	if len(parts) < 2 {
		return "ITSM-000001", nil
	}
	var num int
	fmt.Sscanf(parts[len(parts)-1], "%d", &num)
	return fmt.Sprintf("ITSM-%06d", num+1), nil
}

func tokenize(s string) []string {
	var tokens []string
	for _, sep := range []string{" ", "，", ",", "、", "·", "/", "\\", "（", "）", "(", ")"} {
		s = strings.ReplaceAll(s, sep, " ")
	}
	for _, t := range strings.Fields(s) {
		t = strings.TrimSpace(t)
		if len(t) > 0 {
			tokens = append(tokens, strings.ToLower(t))
		}
	}
	return tokens
}

func computeScore(queryLower string, keywords []string, name, desc string) float64 {
	nameLower := strings.ToLower(name)
	descLower := strings.ToLower(desc)

	score := 0.0

	// Exact name contains query.
	if strings.Contains(nameLower, queryLower) {
		score += 0.5
	}

	// Keyword matches in name.
	for _, kw := range keywords {
		if strings.Contains(nameLower, kw) {
			score += 0.3
		}
		if strings.Contains(descLower, kw) {
			score += 0.1
		}
	}

	// Cap at 1.0.
	if score > 1.0 {
		score = 1.0
	}
	return score
}

func matchReason(queryLower string, keywords []string, name, desc string) string {
	nameLower := strings.ToLower(name)
	if strings.Contains(nameLower, queryLower) {
		return "名称关键词匹配"
	}
	for _, kw := range keywords {
		if strings.Contains(nameLower, kw) {
			return fmt.Sprintf("名称包含关键词「%s」", kw)
		}
	}
	return "描述关键词匹配"
}

func sortMatches(matches []ServiceMatch) {
	for i := 0; i < len(matches); i++ {
		for j := i + 1; j < len(matches); j++ {
			if matches[j].Score > matches[i].Score {
				matches[i], matches[j] = matches[j], matches[i]
			}
		}
	}
}

func truncate(s string, max int) string {
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}
	return string(runes[:max]) + "..."
}

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
