package tools

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"metis/internal/app"
	"metis/internal/app/itsm/form"
)

// ToolHandler handles execution of a single tool call.
type ToolHandler func(ctx context.Context, userID uint, args json.RawMessage) (json.RawMessage, error)

// ---------------------------------------------------------------------------
// Session state
// ---------------------------------------------------------------------------

// ServiceDeskState represents the multi-turn conversation state for the service desk flow.
type ServiceDeskState struct {
	Stage                 string         `json:"stage"` // idle|candidates_ready|service_selected|service_loaded|awaiting_confirmation|confirmed|submitted
	CandidateServiceIDs   []uint         `json:"candidate_service_ids,omitempty"`
	TopMatchServiceID     uint           `json:"top_match_service_id,omitempty"`
	ConfirmedServiceID    uint           `json:"confirmed_service_id,omitempty"`
	ConfirmationRequired  bool           `json:"confirmation_required"`
	LoadedServiceID       uint           `json:"loaded_service_id,omitempty"`
	DraftSummary          string         `json:"draft_summary,omitempty"`
	DraftFormData         map[string]any `json:"draft_form_data,omitempty"`
	RequestText           string         `json:"request_text,omitempty"`
	PrefillFormData       map[string]any `json:"prefill_form_data,omitempty"`
	DraftVersion          int            `json:"draft_version"`
	ConfirmedDraftVersion int            `json:"confirmed_draft_version"`
	FieldsHash            string         `json:"fields_hash,omitempty"`
	MissingFields         []string       `json:"missing_fields,omitempty"`
	AskedFields           []string       `json:"asked_fields,omitempty"`
	MinDecisionReady      bool           `json:"min_decision_ready"`
}

// validTransitions defines the allowed stage transitions.
// "idle" is always a valid target (reset via itsm.new_request).
var validTransitions = map[string][]string{
	"idle":                  {"candidates_ready"},
	"candidates_ready":      {"candidates_ready", "service_selected", "service_loaded"},
	"service_selected":      {"candidates_ready", "service_loaded"},
	"service_loaded":        {"candidates_ready", "awaiting_confirmation"},
	"awaiting_confirmation": {"candidates_ready", "confirmed", "submitted", "awaiting_confirmation"},
	"confirmed":             {"candidates_ready", "submitted"},
	"submitted":             {"candidates_ready"},
}

// TransitionTo validates and performs a stage transition.
// Transition to "idle" is always allowed (reset).
func (s *ServiceDeskState) TransitionTo(next string) error {
	if next == "idle" {
		s.Stage = "idle"
		return nil
	}
	allowed, ok := validTransitions[s.Stage]
	if !ok {
		return fmt.Errorf("invalid transition from %q to %q: current stage unknown", s.Stage, next)
	}
	for _, a := range allowed {
		if a == next {
			s.Stage = next
			return nil
		}
	}
	return fmt.Errorf("invalid transition from %q to %q", s.Stage, next)
}

// StateStore reads/writes session state for a given session.
type StateStore interface {
	GetState(sessionID uint) (*ServiceDeskState, error)
	SaveState(sessionID uint, state *ServiceDeskState) error
}

// ---------------------------------------------------------------------------
// TicketCreator — full-lifecycle ticket creation
// ---------------------------------------------------------------------------

// TicketCreator creates tickets with full service processing (SLA, engine start, timeline).
// Implemented by TicketService to ensure agent-created tickets receive identical processing
// to UI-created tickets.
type TicketCreator interface {
	CreateFromAgent(ctx context.Context, req AgentTicketRequest) (*AgentTicketResult, error)
}

// AgentTicketRequest holds the parameters for creating a ticket from an AI agent session.
type AgentTicketRequest struct {
	UserID       uint
	ServiceID    uint
	Summary      string
	FormData     map[string]any
	SessionID    uint
	DraftVersion int
	FieldsHash   string
	RequestHash  string
}

// AgentTicketResult holds the outcome of an agent-created ticket.
type AgentTicketResult struct {
	TicketID   uint   `json:"ticket_id"`
	TicketCode string `json:"ticket_code"`
	Status     string `json:"status"`
}

// ---------------------------------------------------------------------------
// ServiceDeskOperator — business logic interface
// ---------------------------------------------------------------------------

// ServiceDeskOperator provides the business logic for service desk tools.
type ServiceDeskOperator interface {
	MatchServices(ctx context.Context, query string) ([]ServiceMatch, MatchDecision, error)
	LoadService(serviceID uint) (*ServiceDetail, error)
	CreateTicket(userID uint, serviceID uint, summary string, formData map[string]any, sessionID uint) (*TicketResult, error)
	SubmitConfirmedDraft(userID uint, serviceID uint, summary string, formData map[string]any, sessionID uint, draftVersion int, fieldsHash string, requestHash string) (*TicketResult, error)
	ListMyTickets(userID uint, status string) ([]TicketSummary, error)
	WithdrawTicket(userID uint, ticketCode string, reason string) error
	ValidateParticipants(serviceID uint, formData map[string]any) (*ParticipantValidation, error)
}

type MatchDecisionKind string

const (
	MatchDecisionSelectService     MatchDecisionKind = "select_service"
	MatchDecisionNeedClarification MatchDecisionKind = "need_clarification"
	MatchDecisionNoMatch           MatchDecisionKind = "no_match"
)

type MatchDecision struct {
	Kind                  MatchDecisionKind `json:"kind"`
	SelectedServiceID     uint              `json:"selected_service_id,omitempty"`
	ClarificationQuestion string            `json:"clarification_question,omitempty"`
}

type ServiceMatcher interface {
	MatchServices(ctx context.Context, query string) ([]ServiceMatch, MatchDecision, error)
}

// ServiceMatch is a single search result from MatchServices.
type ServiceMatch struct {
	ID          uint    `json:"id"`
	Name        string  `json:"name"`
	CatalogPath string  `json:"catalog_path"`
	Description string  `json:"description"`
	Score       float64 `json:"score"`
	Reason      string  `json:"reason"`
}

// ServiceDetail is the full definition of a service returned by LoadService.
type ServiceDetail struct {
	ServiceID          uint              `json:"service_id"`
	RequestedID        uint              `json:"requested_service_id,omitempty"`
	ResolvedFrom       string            `json:"resolved_from,omitempty"`
	Name               string            `json:"name"`
	EngineType         string            `json:"engine_type"`
	CollaborationSpec  string            `json:"collaboration_spec"`
	FormSchema         any               `json:"form_schema,omitempty"`
	FormFields         []FormField       `json:"form_fields"`
	Actions            []ActionInfo      `json:"actions"`
	RoutingFieldHint   *RoutingFieldHint `json:"routing_field_hint,omitempty"`
	PrefillSuggestions map[string]any    `json:"prefill_suggestions,omitempty"`
	FieldCollection    *FieldCollection  `json:"field_collection,omitempty"`
	FieldsHash         string            `json:"fields_hash"`
}

// FormField describes one field on a service request form.
type FormField struct {
	Key          string                `json:"key"`
	Label        string                `json:"label"`
	Type         string                `json:"type"`
	Description  string                `json:"description,omitempty"`
	Placeholder  string                `json:"placeholder,omitempty"`
	DefaultValue any                   `json:"defaultValue,omitempty"`
	Required     bool                  `json:"required"`
	Validation   []form.ValidationRule `json:"validation,omitempty"`
	Options      []FormOption          `json:"options,omitempty"`
	Props        map[string]any        `json:"props,omitempty"`
}

// FormOption describes one selectable option on a service request form.
type FormOption struct {
	Label string `json:"label"`
	Value string `json:"value"`
}

// FieldCollection summarizes the field-filling state for an AI service desk turn.
type FieldCollection struct {
	RequiredFields        []FieldCollectionItem `json:"required_fields"`
	PrefilledFields       []FieldCollectionItem `json:"prefilled_fields"`
	MissingRequiredFields []FieldCollectionItem `json:"missing_required_fields"`
	RoutingFieldKey       string                `json:"routing_field_key,omitempty"`
	ReadyForDraft         bool                  `json:"ready_for_draft"`
	NextRequiredTool      string                `json:"next_required_tool,omitempty"`
	RecommendedNextStep   string                `json:"recommended_next_step"`
}

// FieldCollectionItem describes one form field in a field collection summary.
type FieldCollectionItem struct {
	Key      string `json:"key"`
	Label    string `json:"label"`
	Type     string `json:"type"`
	Required bool   `json:"required"`
	Value    any    `json:"value,omitempty"`
	Source   string `json:"source,omitempty"`
}

// ActionInfo is a permitted action on a service.
type ActionInfo struct {
	ID   uint   `json:"id"`
	Code string `json:"code"`
	Name string `json:"name"`
}

// RoutingFieldHint tells the agent which form field drives routing.
type RoutingFieldHint struct {
	FieldKey       string            `json:"field_key"`
	OptionRouteMap map[string]string `json:"option_route_map"`
}

// TicketResult is the outcome of creating a ticket.
type TicketResult struct {
	TicketID   uint   `json:"ticket_id"`
	TicketCode string `json:"ticket_code"`
	Status     string `json:"status"`
}

// TicketSummary is a brief view of a ticket for listing.
type TicketSummary struct {
	TicketID    uint   `json:"ticket_id"`
	TicketCode  string `json:"ticket_code"`
	Summary     string `json:"summary"`
	Status      string `json:"status"`
	ServiceName string `json:"service_name"`
	CreatedAt   string `json:"created_at"`
	CanWithdraw bool   `json:"can_withdraw"`
}

// ParticipantValidation is the result of validating participants for a service.
type ParticipantValidation struct {
	OK            bool   `json:"ok"`
	FailureReason string `json:"failure_reason,omitempty"`
	NodeLabel     string `json:"node_label,omitempty"`
	Guidance      string `json:"guidance,omitempty"`
}

type DraftWarning struct {
	Type           string          `json:"type"`
	Field          string          `json:"field"`
	Message        string          `json:"message"`
	ResolvedValues []ResolvedValue `json:"resolved_values,omitempty"`
}

type ResolvedValue struct {
	Value string `json:"value"`
	Route string `json:"route"`
}

type DraftSubmitRequest struct {
	DraftVersion int            `json:"draftVersion"`
	Summary      string         `json:"summary"`
	FormData     map[string]any `json:"formData"`
}

type DraftSubmitResult struct {
	OK            bool                  `json:"ok"`
	TicketID      uint                  `json:"ticketId,omitempty"`
	TicketCode    string                `json:"ticketCode,omitempty"`
	Status        string                `json:"status,omitempty"`
	Message       string                `json:"message,omitempty"`
	FailureReason string                `json:"failureReason,omitempty"`
	NodeLabel     string                `json:"nodeLabel,omitempty"`
	Guidance      string                `json:"guidance,omitempty"`
	Warnings      []DraftWarning        `json:"warnings,omitempty"`
	MissingFields []FieldCollectionItem `json:"missingRequiredFields,omitempty"`
	State         *ServiceDeskState     `json:"state,omitempty"`
	Surface       map[string]any        `json:"surface,omitempty"`
}

func SubmitDraft(op ServiceDeskOperator, store StateStore, sessionID uint, userID uint, req DraftSubmitRequest) (*DraftSubmitResult, error) {
	state, err := store.GetState(sessionID)
	if err != nil {
		return nil, fmt.Errorf("get state: %w", err)
	}
	if state == nil || state.Stage != "awaiting_confirmation" {
		return nil, fmt.Errorf("当前阶段不允许提交草稿，请先完成草稿整理")
	}
	if state.LoadedServiceID == 0 {
		return nil, fmt.Errorf("请先加载服务")
	}
	if req.DraftVersion == 0 || req.DraftVersion != state.DraftVersion {
		return nil, fmt.Errorf("草稿已变更（当前版本 %d，提交版本 %d），请重新确认表单", state.DraftVersion, req.DraftVersion)
	}

	detail, err := op.LoadService(state.LoadedServiceID)
	if err != nil {
		return nil, fmt.Errorf("load service for submit: %w", err)
	}
	if detail.EngineType != "smart" {
		return nil, fmt.Errorf("仅支持 Agentic 服务提交")
	}
	if detail.FieldsHash != state.FieldsHash {
		return nil, fmt.Errorf("服务表单字段已变更，请重新整理草稿")
	}

	formData := normalizeFormDataKeys(req.FormData, detail.FormFields)
	if len(formData) == 0 && len(state.DraftFormData) > 0 {
		formData = normalizeFormDataKeys(state.DraftFormData, detail.FormFields)
	}
	formData = mergePrefillFormData(formData, state.PrefillFormData)

	summary := strings.TrimSpace(req.Summary)
	if summary == "" {
		summary = state.DraftSummary
	}
	warnings, missingRequired, blocking := validateDraftData(detail, formData)
	if blocking {
		return &DraftSubmitResult{
			OK:            false,
			Message:       "表单还有必填项或无效值，请补充后再提交。",
			Warnings:      warnings,
			MissingFields: missingRequired,
			State:         state,
		}, nil
	}

	validation, err := op.ValidateParticipants(state.LoadedServiceID, formData)
	if err != nil {
		return nil, fmt.Errorf("validate participants: %w", err)
	}
	if validation != nil && !validation.OK {
		return &DraftSubmitResult{
			OK:            false,
			Message:       "参与者预检失败，工单未创建。",
			FailureReason: validation.FailureReason,
			NodeLabel:     validation.NodeLabel,
			Guidance:      validation.Guidance,
			Warnings:      warnings,
			State:         state,
		}, nil
	}

	state.DraftSummary = summary
	state.DraftFormData = formData
	state.ConfirmedDraftVersion = state.DraftVersion

	ticket, err := op.SubmitConfirmedDraft(userID, state.LoadedServiceID, summary, formData, sessionID, state.DraftVersion, state.FieldsHash, requestHash(formData))
	if err != nil {
		return nil, fmt.Errorf("create ticket: %w", err)
	}

	resetState := defaultState()
	if err := store.SaveState(sessionID, resetState); err != nil {
		return nil, fmt.Errorf("save reset state: %w", err)
	}

	surfacePayload := map[string]any{
		"status":     "submitted",
		"serviceId":  detail.ServiceID,
		"title":      detail.Name,
		"summary":    summary,
		"values":     formData,
		"ticketId":   ticket.TicketID,
		"ticketCode": ticket.TicketCode,
		"message":    "工单已提交",
	}
	surface := map[string]any{
		"surfaceId":   fmt.Sprintf("itsm-draft-form-submitted-%d", ticket.TicketID),
		"surfaceType": "itsm.draft_form",
		"payload":     surfacePayload,
	}
	return &DraftSubmitResult{
		OK:         true,
		TicketID:   ticket.TicketID,
		TicketCode: ticket.TicketCode,
		Status:     ticket.Status,
		Message:    "工单已提交",
		Warnings:   warnings,
		State:      resetState,
		Surface:    surface,
	}, nil
}

// ---------------------------------------------------------------------------
// Registry
// ---------------------------------------------------------------------------

// Registry maps tool names to handlers.
type Registry struct {
	handlers map[string]ToolHandler
}

// NewRegistry creates a tool handler registry backed by the given operator and state store.
func NewRegistry(op ServiceDeskOperator, store StateStore) *Registry {
	r := &Registry{handlers: make(map[string]ToolHandler)}

	r.handlers["itsm.service_match"] = serviceMatchHandler(op, store)
	r.handlers["itsm.service_confirm"] = serviceConfirmHandler(store)
	r.handlers["itsm.service_load"] = serviceLoadHandler(op, store)
	r.handlers["itsm.current_request_context"] = currentRequestContextHandler(store)
	r.handlers["itsm.new_request"] = newRequestHandler(store)
	r.handlers["itsm.draft_prepare"] = draftPrepareHandler(op, store)
	r.handlers["itsm.draft_confirm"] = draftConfirmHandler(op, store)
	r.handlers["itsm.validate_participants"] = validateParticipantsHandler(op, store)
	r.handlers["itsm.ticket_create"] = ticketCreateHandler(op, store)
	r.handlers["itsm.my_tickets"] = myTicketsHandler(op)
	r.handlers["itsm.ticket_withdraw"] = ticketWithdrawHandler(op)

	return r
}

func currentRequestContextHandler(store StateStore) ToolHandler {
	return func(ctx context.Context, userID uint, args json.RawMessage) (json.RawMessage, error) {
		sid := sessionID(ctx)
		state, err := store.GetState(sid)
		if err != nil {
			return nil, fmt.Errorf("get state: %w", err)
		}
		if state == nil {
			state = defaultState()
		}
		return mustMarshal(map[string]any{
			"stage":                   state.Stage,
			"candidate_service_ids":   state.CandidateServiceIDs,
			"top_match_service_id":    state.TopMatchServiceID,
			"confirmed_service_id":    state.ConfirmedServiceID,
			"confirmation_required":   state.ConfirmationRequired,
			"loaded_service_id":       state.LoadedServiceID,
			"request_text":            state.RequestText,
			"prefill_form_data":       state.PrefillFormData,
			"draft_summary":           state.DraftSummary,
			"draft_form_data":         state.DraftFormData,
			"draft_version":           state.DraftVersion,
			"confirmed_draft_version": state.ConfirmedDraftVersion,
			"missing_fields":          state.MissingFields,
			"asked_fields":            state.AskedFields,
			"min_decision_ready":      state.MinDecisionReady,
			"next_expected_action":    NextExpectedAction(state),
		}), nil
	}
}

// Execute runs a tool by name.
func (r *Registry) Execute(ctx context.Context, toolName string, userID uint, args json.RawMessage) (json.RawMessage, error) {
	h, ok := r.handlers[toolName]
	if !ok {
		return nil, fmt.Errorf("unknown ITSM tool: %s", toolName)
	}
	return h(ctx, userID, args)
}

// HasTool checks if a tool is registered.
func (r *Registry) HasTool(name string) bool {
	_, ok := r.handlers[name]
	return ok
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func sessionID(ctx context.Context) uint {
	id, _ := ctx.Value(app.SessionIDKey).(uint)
	return id
}

func mustMarshal(v any) json.RawMessage {
	b, _ := json.Marshal(v)
	return b
}

func defaultState() *ServiceDeskState {
	return &ServiceDeskState{Stage: "idle"}
}

func syncConversationProgress(state *ServiceDeskState, missingRequired []FieldCollectionItem) {
	if state == nil {
		return
	}
	missingKeys := make([]string, 0, len(missingRequired))
	missingSet := make(map[string]struct{}, len(missingRequired))
	for _, item := range missingRequired {
		key := strings.TrimSpace(item.Key)
		if key == "" {
			continue
		}
		if _, exists := missingSet[key]; exists {
			continue
		}
		missingSet[key] = struct{}{}
		missingKeys = append(missingKeys, key)
	}

	asked := make([]string, 0, len(state.AskedFields)+len(missingKeys))
	seen := make(map[string]struct{}, len(state.AskedFields)+len(missingKeys))
	for _, key := range state.AskedFields {
		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}
		if _, stillMissing := missingSet[key]; !stillMissing {
			continue
		}
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		asked = append(asked, key)
	}
	for _, key := range missingKeys {
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		asked = append(asked, key)
	}

	state.MissingFields = missingKeys
	state.AskedFields = asked
	state.MinDecisionReady = len(missingKeys) == 0
}

func requestHash(data map[string]any) string {
	b, _ := json.Marshal(data)
	sum := sha256.Sum256(b)
	return fmt.Sprintf("%x", sum[:])
}

// NextExpectedAction returns the next service desk tool/action implied by state.
func NextExpectedAction(state *ServiceDeskState) string {
	if state == nil {
		return "itsm.service_match"
	}
	switch state.Stage {
	case "idle":
		return "itsm.service_match"
	case "candidates_ready":
		if state.ConfirmationRequired && state.ConfirmedServiceID == 0 {
			return "itsm.service_confirm"
		}
		if state.ConfirmedServiceID > 0 || state.TopMatchServiceID > 0 {
			return "itsm.service_load"
		}
	case "service_selected":
		return "itsm.service_load"
	case "service_loaded":
		return "itsm.draft_prepare"
	case "awaiting_confirmation":
		return "itsm.draft_confirm"
	case "confirmed":
		return "itsm.validate_participants"
	}
	return ""
}

var emailPattern = regexp.MustCompile(`[A-Za-z0-9._%+\-]+@[A-Za-z0-9.\-]+\.[A-Za-z]{2,}`)

func hashFormData(data map[string]any) string {
	// Build deterministic JSON by sorting map keys
	keys := make([]string, 0, len(data))
	for k := range data {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	sorted := make([]map[string]any, 0, len(keys))
	for _, k := range keys {
		sorted = append(sorted, map[string]any{"k": k, "v": data[k]})
	}
	b, _ := json.Marshal(sorted)
	h := sha256.Sum256(b)
	return fmt.Sprintf("%x", h[:8])
}

func resolveServiceID(state *ServiceDeskState, requested uint) (uint, string, error) {
	if requested == 0 {
		if state != nil && state.LoadedServiceID > 0 {
			return state.LoadedServiceID, "loaded_service", nil
		}
		return 0, "", fmt.Errorf("service_id is required")
	}
	if state == nil {
		return requested, "service_id", nil
	}
	if requested == state.LoadedServiceID || requested == state.ConfirmedServiceID {
		return requested, "service_id", nil
	}
	for _, cid := range state.CandidateServiceIDs {
		if cid == requested {
			return requested, "service_id", nil
		}
	}
	if requested >= 1 && int(requested) <= len(state.CandidateServiceIDs) {
		return state.CandidateServiceIDs[requested-1], "candidate_index", nil
	}
	if len(state.CandidateServiceIDs) > 0 && state.LoadedServiceID == 0 {
		return 0, "", fmt.Errorf("service_id %d 不在候选列表中，也不是合法候选序号", requested)
	}
	if state.LoadedServiceID > 0 {
		return state.LoadedServiceID, "loaded_service", nil
	}
	return requested, "service_id", nil
}

func normalizeFormDataKeys(data map[string]any, fields []FormField) map[string]any {
	if len(data) == 0 {
		return map[string]any{}
	}
	byLabel := make(map[string]string, len(fields))
	for _, f := range fields {
		if f.Label != "" {
			byLabel[strings.TrimSpace(f.Label)] = f.Key
		}
	}
	normalized := make(map[string]any, len(data))
	for key, val := range data {
		canonicalKey := strings.TrimSpace(key)
		if mapped, ok := byLabel[canonicalKey]; ok {
			canonicalKey = mapped
		}
		normalized[canonicalKey] = val
	}
	return normalized
}

func mergePrefillFormData(data map[string]any, prefill map[string]any) map[string]any {
	if len(data) == 0 {
		data = map[string]any{}
	}
	if len(prefill) == 0 {
		return data
	}
	merged := make(map[string]any, len(prefill)+len(data))
	for key, val := range prefill {
		if val != nil && strings.TrimSpace(fmt.Sprintf("%v", val)) != "" {
			merged[key] = val
		}
	}
	for key, val := range data {
		if val != nil && strings.TrimSpace(fmt.Sprintf("%v", val)) != "" {
			merged[key] = val
		}
	}
	return merged
}

func validateDraftData(detail *ServiceDetail, formData map[string]any) ([]DraftWarning, []FieldCollectionItem, bool) {
	var warnings []DraftWarning
	var missingRequired []FieldCollectionItem
	blocking := false
	missingKeys := map[string]struct{}{}
	warnedFields := map[string]struct{}{}

	for _, f := range detail.FormFields {
		raw, ok := formData[f.Key]
		item := FieldCollectionItem{
			Key:      f.Key,
			Label:    f.Label,
			Type:     f.Type,
			Required: f.Required,
		}
		if f.Required && (!ok || isDraftEmptyValue(raw)) {
			blocking = true
			missingKeys[f.Key] = struct{}{}
			missingRequired = append(missingRequired, item)
			warnings = append(warnings, DraftWarning{
				Type:    "missing_required",
				Field:   f.Key,
				Message: fmt.Sprintf("缺少必填字段：%s", f.Label),
			})
			continue
		}
		value := ""
		if s, ok := raw.(string); ok {
			value = strings.TrimSpace(s)
		} else if raw != nil {
			value = strings.TrimSpace(fmt.Sprintf("%v", raw))
		}
		if value != "" && isEmailSemanticField(f) && !isEmailValue(value) {
			blocking = true
			missingKeys[f.Key] = struct{}{}
			missingRequired = append(missingRequired, item)
			warnings = append(warnings, DraftWarning{
				Type:    "invalid_email",
				Field:   f.Key,
				Message: fmt.Sprintf("%s 需要完整邮箱地址，不能用用户名代替邮箱", f.Label),
			})
			continue
		}
		if value != "" && isTimeSemanticField(f) && hasAmbiguousRelativeTime(value) {
			blocking = true
			warnedFields[f.Key] = struct{}{}
			missingRequired = append(missingRequired, item)
			warnings = append(warnings, DraftWarning{
				Type:    "ambiguous_time",
				Field:   f.Key,
				Message: fmt.Sprintf("%s 包含相对日期和宽泛时段，但缺少具体时分，请继续追问具体时间。", f.Label),
			})
			continue
		}
		if value == "" || (f.Type != form.FieldSelect && f.Type != form.FieldRadio) {
			continue
		}

		optionRoutes := map[string]string{}
		if detail.RoutingFieldHint != nil && detail.RoutingFieldHint.FieldKey == f.Key {
			optionRoutes = detail.RoutingFieldHint.OptionRouteMap
		}

		allowed := make(map[string]struct{}, len(f.Options))
		for _, opt := range f.Options {
			if opt.Value != "" {
				allowed[opt.Value] = struct{}{}
				continue
			}
			if opt.Label != "" {
				allowed[opt.Label] = struct{}{}
			}
		}

		values := []string{value}
		if strings.Contains(value, ",") || strings.Contains(value, "，") {
			values = strings.FieldsFunc(value, func(r rune) bool {
				return r == ',' || r == '，'
			})
			resolvedValues := make([]ResolvedValue, 0, len(values))
			for _, item := range values {
				item = strings.TrimSpace(item)
				if item == "" {
					continue
				}
				if route := optionRoutes[item]; route != "" {
					resolvedValues = append(resolvedValues, ResolvedValue{Value: item, Route: route})
				}
			}
			warnings = append(warnings, DraftWarning{
				Type:           "multivalue_on_single_field",
				Field:          f.Key,
				Message:        fmt.Sprintf("%s 是单选字段，但草稿中包含多个值，请确认最终选择。", f.Label),
				ResolvedValues: resolvedValues,
			})
			blocking = true
			warnedFields[f.Key] = struct{}{}
		}

		for _, item := range values {
			item = strings.TrimSpace(item)
			if item == "" || len(allowed) == 0 {
				continue
			}
			if _, ok := allowed[item]; !ok {
				blocking = true
				warnedFields[f.Key] = struct{}{}
				warnings = append(warnings, DraftWarning{
					Type:    "invalid_option",
					Field:   f.Key,
					Message: fmt.Sprintf("%s 的值 %q 不在可选项中", f.Label, item),
				})
			}
		}
	}

	schema := schemaFromToolFields(detail.FormFields)
	for _, err := range form.ValidateFormData(schema, formData) {
		if _, alreadyMissing := missingKeys[err.Field]; alreadyMissing {
			continue
		}
		if _, alreadyWarned := warnedFields[err.Field]; alreadyWarned {
			continue
		}
		blocking = true
		warningType := "invalid_field_value"
		if fieldByKey(detail.FormFields, err.Field).Key != "" && strings.Contains(err.Message, "不在可选项中") {
			warningType = "invalid_option"
		}
		warnings = append(warnings, DraftWarning{
			Type:    warningType,
			Field:   err.Field,
			Message: err.Message,
		})
	}
	return warnings, missingRequired, blocking
}

func isTimeSemanticField(f FormField) bool {
	if f.Type == form.FieldDate || f.Type == form.FieldDatetime || f.Type == form.FieldDateRange {
		return true
	}
	text := strings.ToLower(strings.Join([]string{f.Key, f.Label, f.Description}, " "))
	return strings.Contains(text, "time") ||
		strings.Contains(text, "date") ||
		strings.Contains(text, "时间") ||
		strings.Contains(text, "时段") ||
		strings.Contains(text, "窗口") ||
		strings.Contains(text, "生效")
}

func canonicalizeTimeSemanticFields(detail *ServiceDetail, summary string, formData map[string]any) map[string]any {
	if len(formData) == 0 {
		return formData
	}
	canonical := make(map[string]any, len(formData))
	for key, val := range formData {
		canonical[key] = val
	}

	sources := []string{summary}
	for _, raw := range formData {
		if raw != nil {
			sources = append(sources, strings.TrimSpace(fmt.Sprintf("%v", raw)))
		}
	}
	for _, f := range detail.FormFields {
		if !isTimeSemanticField(f) || !isDraftEmptyValue(canonical[f.Key]) {
			continue
		}
		for _, source := range sources {
			if value := extractLabeledAbsoluteTimeValue(source, f); value != "" {
				canonical[f.Key] = value
				break
			}
		}
	}
	return canonical
}

func extractLabeledAbsoluteTimeValue(source string, f FormField) string {
	if source == "" {
		return ""
	}
	absoluteDateTime := `\d{4}-\d{2}-\d{2}\s+\d{2}:\d{2}:\d{2}`
	rangeTail := `(?:\s*(?:~|到|至)\s*` + absoluteDateTime + `)?`
	valuePattern := `(` + absoluteDateTime + rangeTail + `)`
	for _, token := range []string{f.Label, f.Key, "访问时段", "时间窗口", "执行窗口", "生效时间"} {
		if token == "" {
			continue
		}
		re := regexp.MustCompile(regexp.QuoteMeta(token) + `\s*(?:[:：=为是])?\s*` + valuePattern)
		if match := re.FindStringSubmatch(source); len(match) > 1 {
			return strings.TrimSpace(match[1])
		}
	}
	return ""
}

func validateDraftTimeSource(detail *ServiceDetail, summary string, formData map[string]any) ([]DraftWarning, []FieldCollectionItem, bool) {
	sources := []string{summary}
	for _, raw := range formData {
		if raw != nil {
			sources = append(sources, strings.TrimSpace(fmt.Sprintf("%v", raw)))
		}
	}

	for _, f := range detail.FormFields {
		if !isTimeSemanticField(f) {
			continue
		}
		for _, source := range sources {
			if !sourceMentionsTimeField(source, f) || !hasAmbiguousRelativeTime(source) {
				continue
			}
			item := FieldCollectionItem{
				Key:      f.Key,
				Label:    f.Label,
				Type:     f.Type,
				Required: f.Required,
				Source:   "time_semantics",
			}
			return []DraftWarning{{
				Type:    "ambiguous_time",
				Field:   f.Key,
				Message: fmt.Sprintf("%s 只有相对日期和宽泛时段，缺少具体时分，请继续追问具体时间。", f.Label),
			}}, []FieldCollectionItem{item}, true
		}
	}
	return nil, nil, false
}

func sourceMentionsTimeField(source string, f FormField) bool {
	if source == "" {
		return false
	}
	for _, token := range []string{f.Key, f.Label, "访问时段", "时间窗口", "执行窗口", "生效时间", "时间"} {
		if token != "" && strings.Contains(source, token) {
			return true
		}
	}
	return false
}

func hasAmbiguousRelativeTime(value string) bool {
	text := strings.TrimSpace(value)
	if text == "" {
		return false
	}
	ambiguousPeriod := regexp.MustCompile(`(今天|明天|后天)?(上午|早上|中午|下午|晚上|晚间|夜间|凌晨)`)
	if !ambiguousPeriod.MatchString(text) {
		return false
	}
	explicitClock := regexp.MustCompile(`\d{1,2}\s*[:：]\s*\d{1,2}|\d{1,2}\s*(点|时)(\s*\d{1,2}\s*分)?`)
	return !explicitClock.MatchString(text)
}

func isDraftEmptyValue(val any) bool {
	if val == nil {
		return true
	}
	switch v := val.(type) {
	case string:
		return strings.TrimSpace(v) == ""
	case []string:
		return len(v) == 0
	case []any:
		return len(v) == 0
	case map[string]any:
		if len(v) == 0 {
			return true
		}
		for _, item := range v {
			if !isDraftEmptyValue(item) {
				return false
			}
		}
		return true
	default:
		return false
	}
}

func schemaFromToolFields(fields []FormField) form.FormSchema {
	schema := form.FormSchema{Version: 1, Fields: make([]form.FormField, 0, len(fields))}
	for _, f := range fields {
		options := make([]form.FieldOption, 0, len(f.Options))
		for _, opt := range f.Options {
			options = append(options, form.FieldOption{Label: opt.Label, Value: opt.Value})
		}
		schema.Fields = append(schema.Fields, form.FormField{
			Key:          f.Key,
			Type:         f.Type,
			Label:        f.Label,
			Placeholder:  f.Placeholder,
			Description:  f.Description,
			DefaultValue: f.DefaultValue,
			Required:     f.Required,
			Validation:   f.Validation,
			Options:      options,
			Props:        f.Props,
		})
	}
	return schema
}

func fieldByKey(fields []FormField, key string) FormField {
	for _, f := range fields {
		if f.Key == key {
			return f
		}
	}
	return FormField{}
}

func isEmailSemanticField(field FormField) bool {
	if strings.EqualFold(field.Type, "email") {
		return true
	}
	semantic := strings.ToLower(field.Key + " " + field.Label + " " + field.Description + " " + field.Placeholder)
	return strings.Contains(semantic, "email") || strings.Contains(semantic, "邮箱")
}

func isEmailValue(value string) bool {
	return emailPattern.FindString(value) == value
}

func buildPrefillSuggestions(requestText string, fields []FormField) map[string]any {
	requestText = strings.TrimSpace(requestText)
	if requestText == "" || len(fields) == 0 {
		return nil
	}

	email := emailPattern.FindString(requestText)
	purpose := extractPurposeText(requestText)
	requestKind := extractRequestKindValue(requestText)
	prefill := make(map[string]any)
	for _, field := range fields {
		semantic := strings.ToLower(field.Key + " " + field.Label + " " + field.Description + " " + field.Placeholder)
		if email != "" && isAccountField(semantic) {
			prefill[field.Key] = email
			continue
		}
		if isRequestKindField(semantic) && isChoiceField(field) {
			if requestKind != "" {
				prefill[field.Key] = requestKind
			}
			continue
		}
		if purpose != "" && isPurposeField(semantic) {
			prefill[field.Key] = purpose
		}
	}
	if len(prefill) == 0 {
		return nil
	}
	return prefill
}

func buildFieldCollection(fields []FormField, prefill map[string]any, routing *RoutingFieldHint) *FieldCollection {
	summary := &FieldCollection{
		RecommendedNextStep: "prepare_draft",
	}
	if routing != nil {
		summary.RoutingFieldKey = routing.FieldKey
	}
	for _, field := range fields {
		item := FieldCollectionItem{
			Key:      field.Key,
			Label:    field.Label,
			Type:     field.Type,
			Required: field.Required,
		}
		if field.Required {
			summary.RequiredFields = append(summary.RequiredFields, item)
		}
		val, ok := prefill[field.Key]
		if ok && val != nil && strings.TrimSpace(fmt.Sprintf("%v", val)) != "" {
			prefilled := item
			prefilled.Value = val
			prefilled.Source = "prefill_suggestions"
			summary.PrefilledFields = append(summary.PrefilledFields, prefilled)
			continue
		}
		if field.Required {
			summary.MissingRequiredFields = append(summary.MissingRequiredFields, item)
		}
	}
	summary.ReadyForDraft = len(summary.MissingRequiredFields) == 0
	if summary.ReadyForDraft {
		summary.NextRequiredTool = "itsm.draft_prepare"
	} else {
		summary.RecommendedNextStep = "ask_missing_fields"
	}
	return summary
}

func isAccountField(semantic string) bool {
	return strings.Contains(semantic, "account") ||
		strings.Contains(semantic, "账号") ||
		strings.Contains(semantic, "vpn_account")
}

func isPurposeField(semantic string) bool {
	return strings.Contains(semantic, "usage") ||
		strings.Contains(semantic, "purpose") ||
		strings.Contains(semantic, "reason") ||
		strings.Contains(semantic, "用途") ||
		strings.Contains(semantic, "原因") ||
		strings.Contains(semantic, "说明")
}

func isRequestKindField(semantic string) bool {
	return strings.Contains(semantic, "request_kind") ||
		strings.Contains(semantic, "访问原因") ||
		strings.Contains(semantic, "申请原因") ||
		strings.Contains(semantic, "业务原因")
}

func isChoiceField(field FormField) bool {
	return field.Type == "select" || field.Type == "radio"
}

func extractPurposeText(requestText string) string {
	cleaned := emailPattern.ReplaceAllString(requestText, "")
	cleaned = strings.NewReplacer(
		"我想申请VPN", "", "我想申请 vpn", "", "我想申请", "",
		"我要申请VPN", "", "我要申请 vpn", "", "申请VPN", "", "申请 vpn", "",
		"想申请VPN", "", "想申请 vpn", "", "想申请", "", "申请", "",
		"开通VPN", "", "开VPN", "", "VPN", "", "vpn", "",
		"我的", "", "账号", "", "是", "",
	).Replace(cleaned)
	cleaned = strings.Trim(cleaned, " ，,。；;、\t\n\r")
	cleaned = strings.TrimSuffix(cleaned, "的")
	if cleaned == "" {
		return ""
	}

	knownPurposes := []string{
		"线上支持", "故障排查", "生产应急", "网络接入",
		"远程办公", "外部协作", "跨境访问", "安全合规", "安全审计", "合规检查",
	}
	for _, purpose := range knownPurposes {
		if strings.Contains(cleaned, purpose) {
			return cleaned
		}
	}
	return ""
}

type vpnRequestKindOption struct {
	Value string
	Route string
	Terms []string
}

var vpnRequestKindOptions = []vpnRequestKindOption{
	{Value: "online_support", Route: "network", Terms: []string{"online_support", "线上支持"}},
	{Value: "troubleshooting", Route: "network", Terms: []string{"troubleshooting", "故障排查", "排障", "网络调试", "网络诊断"}},
	{Value: "production_emergency", Route: "network", Terms: []string{"production_emergency", "生产应急", "应急"}},
	{Value: "network_access_issue", Route: "network", Terms: []string{"network_access_issue", "网络接入问题", "网络接入", "接入问题"}},
	{Value: "external_collaboration", Route: "security", Terms: []string{"external_collaboration", "外部协作"}},
	{Value: "long_term_remote_work", Route: "security", Terms: []string{"long_term_remote_work", "长期远程办公", "远程办公"}},
	{Value: "cross_border_access", Route: "security", Terms: []string{"cross_border_access", "跨境访问"}},
	{Value: "security_compliance", Route: "security", Terms: []string{"security_compliance", "安全合规事项", "安全合规", "合规事项", "安全审计", "合规检查"}},
}

func extractRequestKindValue(requestText string) string {
	requestText = strings.TrimSpace(requestText)
	if requestText == "" {
		return ""
	}
	seenValues := map[string]struct{}{}
	seenRoutes := map[string]struct{}{}
	values := make([]string, 0, 1)
	for _, option := range vpnRequestKindOptions {
		for _, term := range option.Terms {
			if strings.Contains(requestText, term) {
				if _, ok := seenValues[option.Value]; !ok {
					seenValues[option.Value] = struct{}{}
					seenRoutes[option.Route] = struct{}{}
					values = append(values, option.Value)
				}
				break
			}
		}
	}
	if len(values) == 0 || len(seenRoutes) > 1 {
		return ""
	}
	return values[0]
}

// ---------------------------------------------------------------------------
// 1. itsm.service_match
// ---------------------------------------------------------------------------

func serviceMatchHandler(op ServiceDeskOperator, store StateStore) ToolHandler {
	return func(ctx context.Context, userID uint, args json.RawMessage) (json.RawMessage, error) {
		var p struct {
			Query string `json:"query"`
		}
		if err := json.Unmarshal(args, &p); err != nil {
			return nil, fmt.Errorf("invalid args: %w", err)
		}
		if p.Query == "" {
			return nil, fmt.Errorf("query is required")
		}
		requestText := originalRequestText(ctx, p.Query)

		sid := sessionID(ctx)
		state, _ := store.GetState(sid)
		if state == nil {
			state = defaultState()
		}
		if shouldReuseLoadedService(state, p.Query) {
			return mustMarshal(map[string]any{
				"ok":                 true,
				"already_loaded":     true,
				"service_locked":     true,
				"loaded_service_id":  state.LoadedServiceID,
				"request_text":       state.RequestText,
				"prefill_form_data":  state.PrefillFormData,
				"state_stage":        state.Stage,
				"next_required_tool": NextExpectedAction(state),
			}), nil
		}

		matches, decision, err := op.MatchServices(ctx, p.Query)
		if err != nil {
			return nil, fmt.Errorf("match services: %w", err)
		}

		confirmationRequired := false
		var selectedServiceID uint
		switch decision.Kind {
		case MatchDecisionSelectService:
			selectedServiceID = decision.SelectedServiceID
			if selectedServiceID == 0 && len(matches) == 1 {
				selectedServiceID = matches[0].ID
			}
			if selectedServiceID == 0 {
				return nil, fmt.Errorf("select_service decision missing selected_service_id")
			}
			var selectedMatch *ServiceMatch
			for i := range matches {
				if matches[i].ID == selectedServiceID {
					selectedMatch = &matches[i]
					break
				}
			}
			if selectedMatch == nil {
				return nil, fmt.Errorf("select_service decision service_id %d not found in matches", selectedServiceID)
			}
			matches = []ServiceMatch{*selectedMatch}
		case MatchDecisionNeedClarification:
			confirmationRequired = true
		case MatchDecisionNoMatch:
			matches = nil
		default:
			return nil, fmt.Errorf("unknown service match decision: %s", decision.Kind)
		}

		// Collect candidate IDs.
		candidateIDs := make([]uint, len(matches))
		for i, m := range matches {
			candidateIDs[i] = m.ID
		}

		var topMatchID uint
		if len(matches) > 0 {
			topMatchID = matches[0].ID
		}

		// Persist state.
		if err := state.TransitionTo("candidates_ready"); err != nil {
			return nil, err
		}
		state.CandidateServiceIDs = candidateIDs
		state.TopMatchServiceID = topMatchID
		state.ConfirmationRequired = confirmationRequired
		if selectedServiceID > 0 {
			state.ConfirmedServiceID = selectedServiceID
		} else {
			state.ConfirmedServiceID = 0
		}
		state.RequestText = requestText
		state.PrefillFormData = nil
		if err := store.SaveState(sid, state); err != nil {
			return nil, fmt.Errorf("save state: %w", err)
		}

		return mustMarshal(map[string]any{
			"query":                  p.Query,
			"matches":                matches,
			"confirmation_required":  confirmationRequired,
			"selected_service_id":    selectedServiceID,
			"service_locked":         selectedServiceID > 0 && !confirmationRequired,
			"next_required_tool":     serviceMatchNextRequiredTool(selectedServiceID, confirmationRequired, len(matches)),
			"clarification_question": decision.ClarificationQuestion,
		}), nil
	}
}

func originalRequestText(ctx context.Context, fallback string) string {
	if raw := ctx.Value(app.UserMessageKey); raw != nil {
		if msg, ok := raw.(string); ok && strings.TrimSpace(msg) != "" {
			return strings.TrimSpace(msg)
		}
	}
	return fallback
}

func shouldReuseLoadedService(state *ServiceDeskState, query string) bool {
	if state == nil || state.LoadedServiceID == 0 {
		return false
	}
	switch state.Stage {
	case "service_loaded", "awaiting_confirmation", "confirmed":
	default:
		return false
	}
	return isShortContinuation(query)
}

func isShortContinuation(query string) bool {
	cleaned := strings.TrimSpace(query)
	cleaned = strings.Trim(cleaned, "。.!！?？,，;；：: \t\r\n")
	cleaned = strings.ReplaceAll(cleaned, " ", "")
	if cleaned == "" || len([]rune(cleaned)) > 12 {
		return false
	}
	confirmations := map[string]struct{}{
		"是": {}, "是的": {}, "对": {}, "对的": {}, "可以": {}, "可": {},
		"确认": {}, "继续": {}, "好的": {}, "好": {}, "嗯": {}, "嗯嗯": {},
		"没问题": {}, "提交": {}, "用这个": {}, "就这个": {}, "没错": {},
	}
	_, ok := confirmations[cleaned]
	return ok
}

func serviceMatchNextRequiredTool(selectedServiceID uint, confirmationRequired bool, matchCount int) string {
	if selectedServiceID > 0 && !confirmationRequired {
		return "itsm.service_load"
	}
	if confirmationRequired && matchCount > 0 {
		return "itsm.service_confirm"
	}
	return ""
}

// ---------------------------------------------------------------------------
// 2. itsm.service_confirm
// ---------------------------------------------------------------------------

func serviceConfirmHandler(store StateStore) ToolHandler {
	return func(ctx context.Context, userID uint, args json.RawMessage) (json.RawMessage, error) {
		var p struct {
			ServiceID uint `json:"service_id"`
		}
		if err := json.Unmarshal(args, &p); err != nil {
			return nil, fmt.Errorf("invalid args: %w", err)
		}
		if p.ServiceID == 0 {
			return nil, fmt.Errorf("service_id is required")
		}

		sid := sessionID(ctx)
		state, err := store.GetState(sid)
		if err != nil {
			return nil, fmt.Errorf("get state: %w", err)
		}
		if state == nil || state.Stage != "candidates_ready" {
			return nil, fmt.Errorf("当前阶段不允许确认服务，请先调用 service_match")
		}

		resolvedServiceID, resolvedFrom, err := resolveServiceID(state, p.ServiceID)
		if err != nil || resolvedFrom == "loaded_service" {
			return nil, fmt.Errorf("service_id %d 不在候选列表中，也不是合法候选序号", p.ServiceID)
		}

		state.ConfirmedServiceID = resolvedServiceID
		if err := state.TransitionTo("service_selected"); err != nil {
			return nil, err
		}
		if err := store.SaveState(sid, state); err != nil {
			return nil, fmt.Errorf("save state: %w", err)
		}

		return mustMarshal(map[string]any{
			"ok":                   true,
			"service_id":           resolvedServiceID,
			"confirmed_service_id": resolvedServiceID,
			"requested_service_id": p.ServiceID,
			"resolved_from":        resolvedFrom,
			"next_required_tool":   "itsm.service_load",
		}), nil
	}
}

// ---------------------------------------------------------------------------
// 3. itsm.service_load
// ---------------------------------------------------------------------------

func serviceLoadHandler(op ServiceDeskOperator, store StateStore) ToolHandler {
	return func(ctx context.Context, userID uint, args json.RawMessage) (json.RawMessage, error) {
		var p struct {
			ServiceID uint `json:"service_id"`
		}
		if err := json.Unmarshal(args, &p); err != nil {
			return nil, fmt.Errorf("invalid args: %w", err)
		}
		if p.ServiceID == 0 {
			return nil, fmt.Errorf("service_id is required")
		}

		sid := sessionID(ctx)
		state, err := store.GetState(sid)
		if err != nil {
			return nil, fmt.Errorf("get state: %w", err)
		}
		if state == nil {
			state = defaultState()
		}

		// If confirmation was required but not yet given, block.
		if state.ConfirmationRequired && state.ConfirmedServiceID == 0 {
			return nil, fmt.Errorf("请先调用 service_confirm")
		}

		resolvedServiceID, resolvedFrom, err := resolveServiceID(state, p.ServiceID)
		if err != nil {
			return nil, err
		}

		detail, err := op.LoadService(resolvedServiceID)
		if err != nil {
			return nil, fmt.Errorf("load service: %w", err)
		}
		detail.RequestedID = p.ServiceID
		detail.ResolvedFrom = resolvedFrom
		detail.PrefillSuggestions = buildPrefillSuggestions(state.RequestText, detail.FormFields)
		detail.FieldCollection = buildFieldCollection(detail.FormFields, detail.PrefillSuggestions, detail.RoutingFieldHint)
		syncConversationProgress(state, detail.FieldCollection.MissingRequiredFields)

		if state.LoadedServiceID == resolvedServiceID &&
			(state.Stage == "service_loaded" || state.Stage == "awaiting_confirmation" || state.Stage == "confirmed") {
			if state.FieldsHash != detail.FieldsHash {
				state.FieldsHash = detail.FieldsHash
				state.ConfirmedDraftVersion = 0
			}
			state.PrefillFormData = detail.PrefillSuggestions
			if err := store.SaveState(sid, state); err != nil {
				return nil, fmt.Errorf("save state: %w", err)
			}
			return mustMarshal(detail), nil
		}

		// Reset draft fields when switching to a different service.
		if state.LoadedServiceID != resolvedServiceID {
			state.DraftSummary = ""
			state.DraftFormData = nil
			state.DraftVersion = 0
			state.ConfirmedDraftVersion = 0
		}

		state.LoadedServiceID = resolvedServiceID
		state.FieldsHash = detail.FieldsHash
		state.PrefillFormData = detail.PrefillSuggestions
		if err := state.TransitionTo("service_loaded"); err != nil {
			return nil, err
		}
		if err := store.SaveState(sid, state); err != nil {
			return nil, fmt.Errorf("save state: %w", err)
		}

		return mustMarshal(detail), nil
	}
}

// ---------------------------------------------------------------------------
// 4. itsm.new_request
// ---------------------------------------------------------------------------

func newRequestHandler(store StateStore) ToolHandler {
	return func(ctx context.Context, userID uint, args json.RawMessage) (json.RawMessage, error) {
		sid := sessionID(ctx)
		state := defaultState()
		if err := store.SaveState(sid, state); err != nil {
			return nil, fmt.Errorf("save state: %w", err)
		}
		return mustMarshal(map[string]any{
			"ok":      true,
			"message": "已就绪，请描述您的需求",
		}), nil
	}
}

// ---------------------------------------------------------------------------
// 5. itsm.draft_prepare
// ---------------------------------------------------------------------------

func draftPrepareHandler(op ServiceDeskOperator, store StateStore) ToolHandler {
	return func(ctx context.Context, userID uint, args json.RawMessage) (json.RawMessage, error) {
		var p struct {
			Summary  string         `json:"summary"`
			FormData map[string]any `json:"form_data"`
		}
		if err := json.Unmarshal(args, &p); err != nil {
			return nil, fmt.Errorf("invalid args: %w", err)
		}

		sid := sessionID(ctx)
		state, err := store.GetState(sid)
		if err != nil {
			return nil, fmt.Errorf("get state: %w", err)
		}
		if state == nil || state.LoadedServiceID == 0 {
			return nil, fmt.Errorf("请先调用 service_load 加载服务")
		}

		// Load field definitions for validation.
		detail, err := op.LoadService(state.LoadedServiceID)
		if err != nil {
			return nil, fmt.Errorf("load service for validation: %w", err)
		}

		p.FormData = normalizeFormDataKeys(p.FormData, detail.FormFields)
		p.FormData = mergePrefillFormData(p.FormData, state.PrefillFormData)
		p.FormData = canonicalizeTimeSemanticFields(detail, p.Summary, p.FormData)

		warnings, missingRequired, blocking := validateDraftData(detail, p.FormData)
		timeWarnings, timeMissing, timeBlocking := validateDraftTimeSource(detail, p.Summary, p.FormData)
		warnings = append(warnings, timeWarnings...)
		missingRequired = append(missingRequired, timeMissing...)
		blocking = blocking || timeBlocking
		syncConversationProgress(state, missingRequired)

		if blocking {
			if err := store.SaveState(sid, state); err != nil {
				return nil, fmt.Errorf("save state: %w", err)
			}
			return mustMarshal(map[string]any{
				"ok":                      false,
				"ready_for_confirmation":  false,
				"next_required_tool":      "collect_missing_fields",
				"recommended_next_step":   "ask_missing_fields",
				"missing_required_fields": missingRequired,
				"summary":                 p.Summary,
				"form_data":               p.FormData,
				"warnings":                warnings,
			}), nil
		}

		// Determine if content changed; if so, bump draft version and reset confirmation.
		contentChanged := state.DraftSummary != p.Summary || hashFormData(state.DraftFormData) != hashFormData(p.FormData)
		if contentChanged {
			state.DraftVersion++
			state.ConfirmedDraftVersion = 0
		}

		state.DraftSummary = p.Summary
		state.DraftFormData = p.FormData
		if err := state.TransitionTo("awaiting_confirmation"); err != nil {
			return nil, err
		}
		state.MinDecisionReady = true
		if err := store.SaveState(sid, state); err != nil {
			return nil, fmt.Errorf("save state: %w", err)
		}

		return mustMarshal(map[string]any{
			"ok":                      true,
			"ready_for_confirmation":  true,
			"next_required_tool":      "itsm.draft_confirm",
			"recommended_next_step":   "show_draft_for_confirmation",
			"missing_required_fields": []FieldCollectionItem{},
			"draft_version":           state.DraftVersion,
			"service_id":              detail.ServiceID,
			"service_name":            detail.Name,
			"service_engine_type":     detail.EngineType,
			"summary":                 p.Summary,
			"form_data":               p.FormData,
			"form_schema":             detail.FormSchema,
			"warnings":                warnings,
		}), nil
	}
}

// ---------------------------------------------------------------------------
// 6. itsm.draft_confirm
// ---------------------------------------------------------------------------

func draftConfirmHandler(op ServiceDeskOperator, store StateStore) ToolHandler {
	return func(ctx context.Context, userID uint, args json.RawMessage) (json.RawMessage, error) {
		sid := sessionID(ctx)
		state, err := store.GetState(sid)
		if err != nil {
			return nil, fmt.Errorf("get state: %w", err)
		}
		if state == nil || state.Stage != "awaiting_confirmation" {
			return nil, fmt.Errorf("当前阶段不允许确认草稿，请先调用 draft_prepare")
		}

		// Verify fields_hash has not changed since service was loaded.
		if state.LoadedServiceID > 0 {
			detail, err := op.LoadService(state.LoadedServiceID)
			if err != nil {
				return nil, fmt.Errorf("load service for hash check: %w", err)
			}
			if detail.FieldsHash != state.FieldsHash {
				return nil, fmt.Errorf("服务表单字段已变更，请重新调用 service_load")
			}
			warnings, _, blocking := validateDraftData(detail, state.DraftFormData)
			if blocking {
				if len(warnings) > 0 {
					return nil, fmt.Errorf("草稿表单校验失败：%s", warnings[0].Message)
				}
				return nil, fmt.Errorf("草稿表单校验失败，请重新调用 draft_prepare")
			}
		}

		state.ConfirmedDraftVersion = state.DraftVersion
		if err := state.TransitionTo("confirmed"); err != nil {
			return nil, err
		}
		if err := store.SaveState(sid, state); err != nil {
			return nil, fmt.Errorf("save state: %w", err)
		}

		return mustMarshal(map[string]any{
			"ok":                      true,
			"draft_version":           state.DraftVersion,
			"confirmed_draft_version": state.ConfirmedDraftVersion,
		}), nil
	}
}

// ---------------------------------------------------------------------------
// 7. itsm.validate_participants
// ---------------------------------------------------------------------------

func validateParticipantsHandler(op ServiceDeskOperator, store StateStore) ToolHandler {
	return func(ctx context.Context, userID uint, args json.RawMessage) (json.RawMessage, error) {
		var p struct {
			ServiceID uint           `json:"service_id"`
			FormData  map[string]any `json:"form_data"`
		}
		if err := json.Unmarshal(args, &p); err != nil {
			return nil, fmt.Errorf("invalid args: %w", err)
		}
		if p.ServiceID == 0 {
			return nil, fmt.Errorf("service_id is required")
		}

		sid := sessionID(ctx)
		state, err := store.GetState(sid)
		if err != nil {
			return nil, fmt.Errorf("get state: %w", err)
		}
		if state == nil {
			state = defaultState()
		}
		resolvedServiceID, _, err := resolveServiceID(state, p.ServiceID)
		if err != nil {
			return nil, err
		}
		if len(p.FormData) == 0 && len(state.DraftFormData) > 0 {
			p.FormData = state.DraftFormData
		}

		result, err := op.ValidateParticipants(resolvedServiceID, p.FormData)
		if err != nil {
			return nil, fmt.Errorf("validate participants: %w", err)
		}

		return mustMarshal(result), nil
	}
}

// ---------------------------------------------------------------------------
// 8. itsm.ticket_create
// ---------------------------------------------------------------------------

func ticketCreateHandler(op ServiceDeskOperator, store StateStore) ToolHandler {
	return func(ctx context.Context, userID uint, args json.RawMessage) (json.RawMessage, error) {
		var p struct {
			ServiceID uint           `json:"service_id"`
			Summary   string         `json:"summary"`
			FormData  map[string]any `json:"form_data"`
		}
		if err := json.Unmarshal(args, &p); err != nil {
			return nil, fmt.Errorf("invalid args: %w", err)
		}
		if p.ServiceID == 0 {
			return nil, fmt.Errorf("service_id is required")
		}

		sid := sessionID(ctx)
		state, err := store.GetState(sid)
		if err != nil {
			return nil, fmt.Errorf("get state: %w", err)
		}
		if state == nil {
			state = defaultState()
		}

		resolvedServiceID, _, err := resolveServiceID(state, p.ServiceID)
		if err != nil {
			return nil, err
		}

		// Guard: loaded service must match.
		if state.LoadedServiceID != resolvedServiceID {
			return nil, fmt.Errorf("service_id %d 与已加载的服务 %d 不一致，请先调用 service_load", p.ServiceID, state.LoadedServiceID)
		}

		// Guard: draft must be confirmed and versions must match.
		if state.ConfirmedDraftVersion == 0 {
			return nil, fmt.Errorf("草稿未确认，请先调用 draft_confirm")
		}
		if state.ConfirmedDraftVersion != state.DraftVersion {
			return nil, fmt.Errorf("草稿已变更（当前版本 %d，已确认版本 %d），请重新调用 draft_confirm", state.DraftVersion, state.ConfirmedDraftVersion)
		}

		if state.ConfirmedDraftVersion > 0 {
			if state.DraftSummary != "" {
				p.Summary = state.DraftSummary
			}
			if len(state.DraftFormData) > 0 {
				p.FormData = state.DraftFormData
			}
		}

		result, err := op.CreateTicket(userID, resolvedServiceID, p.Summary, p.FormData, sid)
		if err != nil {
			return nil, fmt.Errorf("create ticket: %w", err)
		}

		// Reset state to idle after successful creation.
		resetState := defaultState()
		if err := store.SaveState(sid, resetState); err != nil {
			return nil, fmt.Errorf("save state: %w", err)
		}

		return mustMarshal(map[string]any{
			"ok":          true,
			"ticket_id":   result.TicketID,
			"ticket_code": result.TicketCode,
			"status":      result.Status,
		}), nil
	}
}

// ---------------------------------------------------------------------------
// 9. itsm.my_tickets
// ---------------------------------------------------------------------------

func myTicketsHandler(op ServiceDeskOperator) ToolHandler {
	return func(ctx context.Context, userID uint, args json.RawMessage) (json.RawMessage, error) {
		var p struct {
			Status string `json:"status"`
		}
		// Tolerate missing or empty args.
		if len(args) > 0 {
			_ = json.Unmarshal(args, &p)
		}

		tickets, err := op.ListMyTickets(userID, p.Status)
		if err != nil {
			return nil, fmt.Errorf("list my tickets: %w", err)
		}

		return mustMarshal(map[string]any{
			"ok":      true,
			"tickets": tickets,
		}), nil
	}
}

// ---------------------------------------------------------------------------
// 10. itsm.ticket_withdraw
// ---------------------------------------------------------------------------

func ticketWithdrawHandler(op ServiceDeskOperator) ToolHandler {
	return func(ctx context.Context, userID uint, args json.RawMessage) (json.RawMessage, error) {
		var p struct {
			TicketCode string `json:"ticket_code"`
			Reason     string `json:"reason"`
		}
		if err := json.Unmarshal(args, &p); err != nil {
			return nil, fmt.Errorf("invalid args: %w", err)
		}
		if p.TicketCode == "" {
			return nil, fmt.Errorf("ticket_code is required")
		}
		if p.Reason == "" {
			p.Reason = "用户撤回"
		}

		if err := op.WithdrawTicket(userID, p.TicketCode, p.Reason); err != nil {
			return nil, fmt.Errorf("withdraw ticket: %w", err)
		}

		return mustMarshal(map[string]any{
			"ok":          true,
			"ticket_code": p.TicketCode,
		}), nil
	}
}
