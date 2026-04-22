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
)

// ToolHandler handles execution of a single tool call.
type ToolHandler func(ctx context.Context, userID uint, args json.RawMessage) (json.RawMessage, error)

// ---------------------------------------------------------------------------
// Session state
// ---------------------------------------------------------------------------

// ServiceDeskState represents the multi-turn conversation state for the service desk flow.
type ServiceDeskState struct {
	Stage                 string         `json:"stage"` // idle|candidates_ready|service_selected|service_loaded|awaiting_confirmation|confirmed
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
}

// validTransitions defines the allowed stage transitions.
// "idle" is always a valid target (reset via itsm.new_request).
var validTransitions = map[string][]string{
	"idle":                  {"candidates_ready"},
	"candidates_ready":      {"candidates_ready", "service_selected", "service_loaded"},
	"service_selected":      {"candidates_ready", "service_loaded"},
	"service_loaded":        {"candidates_ready", "awaiting_confirmation"},
	"awaiting_confirmation": {"candidates_ready", "confirmed", "awaiting_confirmation"},
	"confirmed":             {"candidates_ready"},
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
	UserID    uint
	ServiceID uint
	Summary   string
	FormData  map[string]any
	SessionID uint
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
	CollaborationSpec  string            `json:"collaboration_spec"`
	FormFields         []FormField       `json:"form_fields"`
	Actions            []ActionInfo      `json:"actions"`
	RoutingFieldHint   *RoutingFieldHint `json:"routing_field_hint,omitempty"`
	PrefillSuggestions map[string]any    `json:"prefill_suggestions,omitempty"`
	FieldCollection    *FieldCollection  `json:"field_collection,omitempty"`
	FieldsHash         string            `json:"fields_hash"`
}

// FormField describes one field on a service request form.
type FormField struct {
	Key         string       `json:"key"`
	Label       string       `json:"label"`
	Type        string       `json:"type"` // text, select, radio, checkbox, date, textarea, table
	Description string       `json:"description,omitempty"`
	Placeholder string       `json:"placeholder,omitempty"`
	Required    bool         `json:"required"`
	Options     []FormOption `json:"options,omitempty"`
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

func buildPrefillSuggestions(requestText string, fields []FormField) map[string]any {
	requestText = strings.TrimSpace(requestText)
	if requestText == "" || len(fields) == 0 {
		return nil
	}

	email := emailPattern.FindString(requestText)
	purpose := extractPurposeText(requestText)
	prefill := make(map[string]any)
	for _, field := range fields {
		semantic := strings.ToLower(field.Key + " " + field.Label + " " + field.Description + " " + field.Placeholder)
		if email != "" && isAccountField(semantic) {
			prefill[field.Key] = email
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
		strings.Contains(semantic, "request_kind") ||
		strings.Contains(semantic, "用途") ||
		strings.Contains(semantic, "原因") ||
		strings.Contains(semantic, "说明")
}

func extractPurposeText(requestText string) string {
	cleaned := emailPattern.ReplaceAllString(requestText, "")
	cleaned = strings.NewReplacer(
		"我要申请VPN", "", "我要申请 vpn", "", "申请VPN", "", "申请 vpn", "",
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

		if state.LoadedServiceID == resolvedServiceID &&
			(state.Stage == "service_loaded" || state.Stage == "awaiting_confirmation" || state.Stage == "confirmed") {
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

		// Build field index.
		fieldMap := make(map[string]FormField, len(detail.FormFields))
		for _, f := range detail.FormFields {
			fieldMap[f.Key] = f
		}

		// Validate form_data and collect warnings.
		type resolvedValue struct {
			Value string `json:"value"`
			Route string `json:"route"`
		}
		type warning struct {
			Type           string          `json:"type"`
			Field          string          `json:"field"`
			Message        string          `json:"message"`
			ResolvedValues []resolvedValue `json:"resolved_values,omitempty"`
		}
		var warnings []warning
		var missingRequired []FieldCollectionItem
		blocking := false

		// Check for missing required fields.
		for _, f := range detail.FormFields {
			if !f.Required {
				continue
			}
			val, exists := p.FormData[f.Key]
			if !exists || val == nil || val == "" {
				warnings = append(warnings, warning{
					Type:    "missing_required",
					Field:   f.Key,
					Message: fmt.Sprintf("必填字段 [%s] 未填写", f.Label),
				})
				missingRequired = append(missingRequired, FieldCollectionItem{
					Key:      f.Key,
					Label:    f.Label,
					Type:     f.Type,
					Required: true,
				})
				blocking = true
			}
		}

		// Check select/radio field values.
		for key, val := range p.FormData {
			f, ok := fieldMap[key]
			if !ok {
				continue
			}
			strVal := fmt.Sprintf("%v", val)
			if strVal == "" {
				continue
			}

			if (f.Type == "select" || f.Type == "radio") && len(f.Options) > 0 {
				// Check for comma-separated values in single-select fields.
				if strings.Contains(strVal, ",") {
					w := warning{
						Type:    "multivalue_on_single_field",
						Field:   key,
						Message: fmt.Sprintf("字段 [%s] 为单选，不支持多个值", f.Label),
					}
					// If this is the routing field, resolve each value to its route branch.
					if hint := detail.RoutingFieldHint; hint != nil && key == hint.FieldKey {
						parts := strings.Split(strVal, ",")
						rv := make([]resolvedValue, 0, len(parts))
						for _, p := range parts {
							v := strings.TrimSpace(p)
							if v == "" {
								continue
							}
							rv = append(rv, resolvedValue{Value: v, Route: hint.OptionRouteMap[v]})
						}
						w.ResolvedValues = rv
					}
					warnings = append(warnings, w)
					continue
				}
				// Check if value is a valid option.
				valid := false
				for _, opt := range f.Options {
					if opt.Value == strVal || opt.Label == strVal {
						valid = true
						break
					}
				}
				if !valid {
					warnings = append(warnings, warning{
						Type:    "invalid_option",
						Field:   key,
						Message: fmt.Sprintf("字段 [%s] 的值 [%s] 不在可选项中", f.Label, strVal),
					})
					blocking = true
				}
			}
		}

		if blocking {
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
			"summary":                 p.Summary,
			"form_data":               p.FormData,
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
