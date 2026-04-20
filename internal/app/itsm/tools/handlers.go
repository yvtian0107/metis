package tools

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
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
	MatchServices(query string) ([]ServiceMatch, error)
	LoadService(serviceID uint) (*ServiceDetail, error)
	CreateTicket(userID uint, serviceID uint, summary string, formData map[string]any, sessionID uint) (*TicketResult, error)
	ListMyTickets(userID uint, status string) ([]TicketSummary, error)
	WithdrawTicket(userID uint, ticketCode string, reason string) error
	ValidateParticipants(serviceID uint, formData map[string]any) (*ParticipantValidation, error)
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
	ServiceID         uint              `json:"service_id"`
	Name              string            `json:"name"`
	CollaborationSpec string            `json:"collaboration_spec"`
	FormFields        []FormField       `json:"form_fields"`
	Actions           []ActionInfo      `json:"actions"`
	RoutingFieldHint  *RoutingFieldHint `json:"routing_field_hint,omitempty"`
	FieldsHash        string            `json:"fields_hash"`
}

// FormField describes one field on a service request form.
type FormField struct {
	Key      string   `json:"key"`
	Label    string   `json:"label"`
	Type     string   `json:"type"` // text, select, radio, checkbox, date, textarea, table
	Required bool     `json:"required"`
	Options  []string `json:"options,omitempty"`
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
	r.handlers["itsm.new_request"] = newRequestHandler(store)
	r.handlers["itsm.draft_prepare"] = draftPrepareHandler(op, store)
	r.handlers["itsm.draft_confirm"] = draftConfirmHandler(op, store)
	r.handlers["itsm.validate_participants"] = validateParticipantsHandler(op)
	r.handlers["itsm.ticket_create"] = ticketCreateHandler(op, store)
	r.handlers["itsm.my_tickets"] = myTicketsHandler(op)
	r.handlers["itsm.ticket_withdraw"] = ticketWithdrawHandler(op)

	return r
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

		matches, err := op.MatchServices(p.Query)
		if err != nil {
			return nil, fmt.Errorf("match services: %w", err)
		}

		// Determine whether confirmation is required.
		confirmationRequired := false
		if len(matches) >= 2 {
			diff := matches[0].Score - matches[1].Score
			if diff < 0 {
				diff = -diff
			}
			if diff < 0.1 {
				confirmationRequired = true
			}
		}
		if len(matches) > 0 && matches[0].Score < 0.8 {
			confirmationRequired = true
		}

		// Auto-select when unambiguous.
		var selectedServiceID uint
		if !confirmationRequired && len(matches) == 1 {
			selectedServiceID = matches[0].ID
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
		sid := sessionID(ctx)
		state, _ := store.GetState(sid)
		if state == nil {
			state = defaultState()
		}
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
		if err := store.SaveState(sid, state); err != nil {
			return nil, fmt.Errorf("save state: %w", err)
		}

		return mustMarshal(map[string]any{
			"query":                 p.Query,
			"matches":               matches,
			"confirmation_required": confirmationRequired,
			"selected_service_id":   selectedServiceID,
		}), nil
	}
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

		// Verify service_id is among candidates.
		found := false
		for _, cid := range state.CandidateServiceIDs {
			if cid == p.ServiceID {
				found = true
				break
			}
		}
		if !found {
			return nil, fmt.Errorf("service_id %d 不在候选列表中", p.ServiceID)
		}

		state.ConfirmedServiceID = p.ServiceID
		if err := state.TransitionTo("service_selected"); err != nil {
			return nil, err
		}
		if err := store.SaveState(sid, state); err != nil {
			return nil, fmt.Errorf("save state: %w", err)
		}

		return mustMarshal(map[string]any{
			"ok":                   true,
			"service_id":           p.ServiceID,
			"confirmed_service_id": p.ServiceID,
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

		detail, err := op.LoadService(p.ServiceID)
		if err != nil {
			return nil, fmt.Errorf("load service: %w", err)
		}

		// Reset draft fields when switching to a different service.
		if state.LoadedServiceID != p.ServiceID {
			state.DraftSummary = ""
			state.DraftFormData = nil
			state.DraftVersion = 0
			state.ConfirmedDraftVersion = 0
		}

		state.LoadedServiceID = p.ServiceID
		state.FieldsHash = detail.FieldsHash
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
					if opt == strVal {
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
				}
			}
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
			"ok":            true,
			"draft_version": state.DraftVersion,
			"summary":       p.Summary,
			"form_data":     p.FormData,
			"warnings":      warnings,
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

func validateParticipantsHandler(op ServiceDeskOperator) ToolHandler {
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

		result, err := op.ValidateParticipants(p.ServiceID, p.FormData)
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

		// Guard: loaded service must match.
		if state.LoadedServiceID != p.ServiceID {
			return nil, fmt.Errorf("service_id %d 与已加载的服务 %d 不一致，请先调用 service_load", p.ServiceID, state.LoadedServiceID)
		}

		// Guard: draft must be confirmed and versions must match.
		if state.ConfirmedDraftVersion == 0 {
			return nil, fmt.Errorf("草稿未确认，请先调用 draft_confirm")
		}
		if state.ConfirmedDraftVersion != state.DraftVersion {
			return nil, fmt.Errorf("草稿已变更（当前版本 %d，已确认版本 %d），请重新调用 draft_confirm", state.DraftVersion, state.ConfirmedDraftVersion)
		}

		result, err := op.CreateTicket(userID, p.ServiceID, p.Summary, p.FormData, sid)
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
