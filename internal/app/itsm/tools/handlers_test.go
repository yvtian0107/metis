package tools

import (
	"context"
	"encoding/json"
	"testing"

	"metis/internal/app"
)

// memStateStore is an in-memory StateStore for testing.
type memStateStore struct {
	states map[uint]*ServiceDeskState
}

func newMemStateStore() *memStateStore {
	return &memStateStore{states: make(map[uint]*ServiceDeskState)}
}

func (m *memStateStore) GetState(sessionID uint) (*ServiceDeskState, error) {
	s, ok := m.states[sessionID]
	if !ok {
		return defaultState(), nil
	}
	return s, nil
}

func (m *memStateStore) SaveState(sessionID uint, state *ServiceDeskState) error {
	m.states[sessionID] = state
	return nil
}

// stubOperator implements ServiceDeskOperator for unit tests.
// Only LoadService is needed for draft_prepare tests.
type stubOperator struct {
	detail             *ServiceDetail
	details            map[uint]*ServiceDetail
	matchResponse      []ServiceMatch
	matchDecision      MatchDecision
	createdServiceID   uint
	createdSummary     string
	createdFormData    map[string]any
	validatedServiceID uint
	validatedFormData  map[string]any
}

func (s *stubOperator) MatchServices(ctx context.Context, query string) ([]ServiceMatch, MatchDecision, error) {
	return s.matchResponse, s.matchDecision, nil
}
func (s *stubOperator) LoadService(serviceID uint) (*ServiceDetail, error) {
	if s.details != nil {
		if detail, ok := s.details[serviceID]; ok {
			return detail, nil
		}
	}
	return s.detail, nil
}
func (s *stubOperator) CreateTicket(userID uint, serviceID uint, summary string, formData map[string]any, sessionID uint) (*TicketResult, error) {
	s.createdServiceID = serviceID
	s.createdSummary = summary
	s.createdFormData = formData
	return &TicketResult{TicketID: 123, TicketCode: "TICK-000123", Status: "in_progress"}, nil
}
func (s *stubOperator) ListMyTickets(userID uint, status string) ([]TicketSummary, error) {
	return nil, nil
}
func (s *stubOperator) WithdrawTicket(userID uint, ticketCode string, reason string) error {
	return nil
}
func (s *stubOperator) ValidateParticipants(serviceID uint, formData map[string]any) (*ParticipantValidation, error) {
	s.validatedServiceID = serviceID
	s.validatedFormData = formData
	return &ParticipantValidation{OK: true}, nil
}

func vpnServiceDetail(serviceID uint) *ServiceDetail {
	return &ServiceDetail{
		ServiceID: serviceID,
		Name:      "VPN 开通申请",
		FormFields: []FormField{
			{Key: "vpn_account", Label: "VPN账号", Type: "text", Required: true},
			{Key: "device_usage", Label: "设备与用途说明", Type: "textarea", Required: true},
			{Key: "request_kind", Label: "访问原因", Type: "textarea", Required: true},
		},
		FieldsHash: "vpn123",
	}
}

func TestServiceMatch_SelectServiceAutoConfirmsAndAllowsLoad(t *testing.T) {
	store := newMemStateStore()
	op := &stubOperator{
		matchResponse: []ServiceMatch{
			{ID: 5, Name: "VPN 开通申请", CatalogPath: "基础设施与网络/网络与 VPN", Description: "VPN 开通", Score: 0.97, Reason: "用户明确要求申请 VPN"},
		},
		matchDecision: MatchDecision{Kind: MatchDecisionSelectService, SelectedServiceID: 5},
		details:       map[uint]*ServiceDetail{5: vpnServiceDetail(5)},
	}
	ctx := context.WithValue(context.Background(), app.SessionIDKey, uint(1))

	result, err := serviceMatchHandler(op, store)(ctx, 1, []byte(`{"query":"我要申请VPN"}`))
	if err != nil {
		t.Fatalf("service match: %v", err)
	}

	var resp struct {
		Matches              []ServiceMatch `json:"matches"`
		ConfirmationRequired bool           `json:"confirmation_required"`
		SelectedServiceID    uint           `json:"selected_service_id"`
	}
	if err := json.Unmarshal(result, &resp); err != nil {
		t.Fatalf("unmarshal match response: %v", err)
	}
	if resp.ConfirmationRequired {
		t.Fatalf("select_service should not require confirmation: %+v", resp)
	}
	if resp.SelectedServiceID != 5 || len(resp.Matches) != 1 || resp.Matches[0].Name != "VPN 开通申请" {
		t.Fatalf("expected only selected VPN service, got %+v", resp)
	}
	state := store.states[1]
	if state.Stage != "candidates_ready" || state.ConfirmedServiceID != 5 || state.TopMatchServiceID != 5 || state.ConfirmationRequired {
		t.Fatalf("expected selected service to be confirmed in state, got %+v", state)
	}

	if _, err := serviceLoadHandler(op, store)(ctx, 1, []byte(`{"service_id":5}`)); err != nil {
		t.Fatalf("selected service should load without service_confirm: %v", err)
	}
}

func TestServiceMatch_NeedClarificationRequiresServiceConfirm(t *testing.T) {
	store := newMemStateStore()
	op := &stubOperator{
		matchResponse: []ServiceMatch{
			{ID: 5, Name: "VPN 开通申请", Score: 0.72, Reason: "涉及 VPN"},
			{ID: 8, Name: "VPN 故障排查", Score: 0.7, Reason: "用户描述可能是故障"},
		},
		matchDecision: MatchDecision{Kind: MatchDecisionNeedClarification, ClarificationQuestion: "请选择是开通 VPN 还是排查 VPN 故障"},
	}
	ctx := context.WithValue(context.Background(), app.SessionIDKey, uint(1))

	result, err := serviceMatchHandler(op, store)(ctx, 1, []byte(`{"query":"VPN有问题，也想开通权限"}`))
	if err != nil {
		t.Fatalf("service match: %v", err)
	}

	var resp struct {
		Matches               []ServiceMatch `json:"matches"`
		ConfirmationRequired  bool           `json:"confirmation_required"`
		SelectedServiceID     uint           `json:"selected_service_id"`
		ClarificationQuestion string         `json:"clarification_question"`
	}
	if err := json.Unmarshal(result, &resp); err != nil {
		t.Fatalf("unmarshal match response: %v", err)
	}
	if !resp.ConfirmationRequired || resp.SelectedServiceID != 0 || len(resp.Matches) != 2 {
		t.Fatalf("expected clarification candidates without selected service, got %+v", resp)
	}
	if resp.ClarificationQuestion == "" {
		t.Fatalf("expected clarification question in response")
	}
	state := store.states[1]
	if state.ConfirmedServiceID != 0 || !state.ConfirmationRequired || len(state.CandidateServiceIDs) != 2 {
		t.Fatalf("expected confirmation to be required in state, got %+v", state)
	}
}

func TestServiceMatch_NoMatchClearsCandidates(t *testing.T) {
	store := newMemStateStore()
	op := &stubOperator{
		matchDecision: MatchDecision{Kind: MatchDecisionNoMatch},
	}
	ctx := context.WithValue(context.Background(), app.SessionIDKey, uint(1))

	result, err := serviceMatchHandler(op, store)(ctx, 1, []byte(`{"query":"我要领一杯咖啡"}`))
	if err != nil {
		t.Fatalf("service match: %v", err)
	}

	var resp struct {
		Matches              []ServiceMatch `json:"matches"`
		ConfirmationRequired bool           `json:"confirmation_required"`
		SelectedServiceID    uint           `json:"selected_service_id"`
	}
	if err := json.Unmarshal(result, &resp); err != nil {
		t.Fatalf("unmarshal match response: %v", err)
	}
	if resp.ConfirmationRequired || resp.SelectedServiceID != 0 || len(resp.Matches) != 0 {
		t.Fatalf("expected empty no-match response, got %+v", resp)
	}
	state := store.states[1]
	if state.ConfirmedServiceID != 0 || state.TopMatchServiceID != 0 || len(state.CandidateServiceIDs) != 0 {
		t.Fatalf("expected no-match state to clear candidates, got %+v", state)
	}
}

func TestServiceDeskFlow_UsesLoadedServiceAndConfirmedDraftWhenModelFallsBackToIndex(t *testing.T) {
	store := newMemStateStore()
	op := &stubOperator{details: map[uint]*ServiceDetail{5: vpnServiceDetail(5)}}
	ctx := context.WithValue(context.Background(), app.SessionIDKey, uint(1))

	store.states[1] = &ServiceDeskState{Stage: "candidates_ready", CandidateServiceIDs: []uint{5}, TopMatchServiceID: 5, ConfirmationRequired: true}
	if _, err := serviceConfirmHandler(store)(ctx, 1, []byte(`{"service_id":1}`)); err != nil {
		t.Fatalf("confirm candidate index: %v", err)
	}
	if _, err := serviceLoadHandler(op, store)(ctx, 1, []byte(`{"service_id":5}`)); err != nil {
		t.Fatalf("load resolved service: %v", err)
	}
	draftArgs, _ := json.Marshal(map[string]any{
		"summary": "VPN 开通申请 - 账号：wenhaowu@dev.com，设备：安卓手机，用途：线上支持",
		"form_data": map[string]any{
			"VPN账号":   "wenhaowu@dev.com",
			"设备与用途说明": "安卓手机",
			"访问原因":    "线上支持",
		},
	})
	result, err := draftPrepareHandler(op, store)(ctx, 1, draftArgs)
	if err != nil {
		t.Fatalf("prepare draft: %v", err)
	}
	var draftResp struct {
		OK       bool           `json:"ok"`
		FormData map[string]any `json:"form_data"`
	}
	if err := json.Unmarshal(result, &draftResp); err != nil {
		t.Fatalf("unmarshal draft response: %v", err)
	}
	if !draftResp.OK {
		t.Fatalf("expected draft to be ok, got %s", string(result))
	}
	if draftResp.FormData["vpn_account"] != "wenhaowu@dev.com" ||
		draftResp.FormData["device_usage"] != "安卓手机" ||
		draftResp.FormData["request_kind"] != "线上支持" {
		t.Fatalf("expected label keys to be canonicalized, got %+v", draftResp.FormData)
	}
	if _, err := draftConfirmHandler(op, store)(ctx, 1, []byte(`{}`)); err != nil {
		t.Fatalf("confirm draft: %v", err)
	}
	if _, err := validateParticipantsHandler(op, store)(ctx, 1, []byte(`{"service_id":1}`)); err != nil {
		t.Fatalf("validate participants with candidate index: %v", err)
	}
	if op.validatedServiceID != 5 || op.validatedFormData["vpn_account"] != "wenhaowu@dev.com" {
		t.Fatalf("expected participant validation to use loaded service and draft data, got service=%d data=%+v", op.validatedServiceID, op.validatedFormData)
	}
	if _, err := ticketCreateHandler(op, store)(ctx, 1, []byte(`{"service_id":1,"summary":"ignored by confirmed draft"}`)); err != nil {
		t.Fatalf("create ticket with candidate index: %v", err)
	}
	if op.createdServiceID != 5 {
		t.Fatalf("expected ticket to be created with service 5, got %d", op.createdServiceID)
	}
	if op.createdSummary != "VPN 开通申请 - 账号：wenhaowu@dev.com，设备：安卓手机，用途：线上支持" {
		t.Fatalf("expected confirmed draft summary to be used, got %q", op.createdSummary)
	}
	if op.createdFormData["request_kind"] != "线上支持" {
		t.Fatalf("expected confirmed draft form data to be used, got %+v", op.createdFormData)
	}
}

func TestDraftPrepare_MissingRequiredFormDataDoesNotAdvanceState(t *testing.T) {
	store := newMemStateStore()
	op := &stubOperator{detail: vpnServiceDetail(5)}
	store.states[1] = &ServiceDeskState{Stage: "service_loaded", LoadedServiceID: 5, FieldsHash: "vpn123"}
	ctx := context.WithValue(context.Background(), app.SessionIDKey, uint(1))

	result, err := draftPrepareHandler(op, store)(ctx, 1, []byte(`{"summary":"只有摘要"}`))
	if err != nil {
		t.Fatalf("prepare draft without form data: %v", err)
	}
	var resp struct {
		OK       bool `json:"ok"`
		Warnings []struct {
			Type  string `json:"type"`
			Field string `json:"field"`
		} `json:"warnings"`
	}
	if err := json.Unmarshal(result, &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if resp.OK {
		t.Fatalf("expected missing required fields to block draft, got %s", string(result))
	}
	if len(resp.Warnings) != 3 {
		t.Fatalf("expected 3 missing required warnings, got %+v", resp.Warnings)
	}
	if state := store.states[1]; state.Stage != "service_loaded" || state.DraftVersion != 0 {
		t.Fatalf("state should not advance on blocked draft: %+v", state)
	}
}

func TestServiceLoad_IsIdempotentAfterDraftConfirmed(t *testing.T) {
	store := newMemStateStore()
	op := &stubOperator{details: map[uint]*ServiceDetail{5: vpnServiceDetail(5)}}
	store.states[1] = &ServiceDeskState{
		Stage:                 "confirmed",
		CandidateServiceIDs:   []uint{5},
		ConfirmedServiceID:    5,
		LoadedServiceID:       5,
		DraftSummary:          "保留草稿",
		DraftFormData:         map[string]any{"vpn_account": "wenhaowu@dev.com"},
		DraftVersion:          1,
		ConfirmedDraftVersion: 1,
		FieldsHash:            "vpn123",
	}
	ctx := context.WithValue(context.Background(), app.SessionIDKey, uint(1))

	if _, err := serviceLoadHandler(op, store)(ctx, 1, []byte(`{"service_id":1}`)); err != nil {
		t.Fatalf("idempotent load with candidate index: %v", err)
	}
	state := store.states[1]
	if state.Stage != "confirmed" || state.DraftSummary != "保留草稿" || state.DraftVersion != 1 || state.ConfirmedDraftVersion != 1 {
		t.Fatalf("idempotent load should preserve confirmed draft state, got %+v", state)
	}
}

func TestServiceConfirm_AcceptsCandidateIndex(t *testing.T) {
	store := newMemStateStore()
	store.states[1] = &ServiceDeskState{
		Stage:               "candidates_ready",
		CandidateServiceIDs: []uint{5},
		TopMatchServiceID:   5,
	}
	handler := serviceConfirmHandler(store)
	ctx := context.WithValue(context.Background(), app.SessionIDKey, uint(1))

	result, err := handler(ctx, 1, []byte(`{"service_id":1}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var resp struct {
		OK                 bool   `json:"ok"`
		ServiceID          uint   `json:"service_id"`
		ConfirmedServiceID uint   `json:"confirmed_service_id"`
		ResolvedFrom       string `json:"resolved_from"`
		NextRequiredTool   string `json:"next_required_tool"`
	}
	if err := json.Unmarshal(result, &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if !resp.OK || resp.ServiceID != 5 || resp.ConfirmedServiceID != 5 {
		t.Fatalf("expected candidate index to resolve to service 5, got %+v", resp)
	}
	if resp.ResolvedFrom != "candidate_index" {
		t.Fatalf("expected resolved_from=candidate_index, got %q", resp.ResolvedFrom)
	}
	if resp.NextRequiredTool != "itsm.service_load" {
		t.Fatalf("expected next_required_tool=itsm.service_load, got %q", resp.NextRequiredTool)
	}
	state := store.states[1]
	if state.Stage != "service_selected" || state.ConfirmedServiceID != 5 {
		t.Fatalf("unexpected state after confirm: %+v", state)
	}
}

func TestServiceConfirm_AcceptsCandidateIndexInMultiCandidateList(t *testing.T) {
	store := newMemStateStore()
	store.states[1] = &ServiceDeskState{
		Stage:               "candidates_ready",
		CandidateServiceIDs: []uint{5, 8},
		TopMatchServiceID:   5,
	}
	handler := serviceConfirmHandler(store)
	ctx := context.WithValue(context.Background(), app.SessionIDKey, uint(1))

	result, err := handler(ctx, 1, []byte(`{"service_id":2}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var resp struct {
		ServiceID    uint   `json:"service_id"`
		ResolvedFrom string `json:"resolved_from"`
	}
	if err := json.Unmarshal(result, &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if resp.ServiceID != 8 || resp.ResolvedFrom != "candidate_index" {
		t.Fatalf("expected second candidate index to resolve to service 8, got %+v", resp)
	}
	if got := store.states[1].ConfirmedServiceID; got != 8 {
		t.Fatalf("expected confirmed service id 8, got %d", got)
	}
}

func TestServiceConfirm_AcceptsExactCandidateID(t *testing.T) {
	store := newMemStateStore()
	store.states[1] = &ServiceDeskState{
		Stage:               "candidates_ready",
		CandidateServiceIDs: []uint{5, 8},
		TopMatchServiceID:   5,
	}
	handler := serviceConfirmHandler(store)
	ctx := context.WithValue(context.Background(), app.SessionIDKey, uint(1))

	result, err := handler(ctx, 1, []byte(`{"service_id":8}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var resp struct {
		ServiceID    uint   `json:"service_id"`
		ResolvedFrom string `json:"resolved_from"`
	}
	if err := json.Unmarshal(result, &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if resp.ServiceID != 8 || resp.ResolvedFrom != "service_id" {
		t.Fatalf("expected exact service id 8, got %+v", resp)
	}
	if got := store.states[1].ConfirmedServiceID; got != 8 {
		t.Fatalf("expected confirmed service id 8, got %d", got)
	}
}

func TestServiceConfirm_RejectsUnknownIDOrIndex(t *testing.T) {
	store := newMemStateStore()
	store.states[1] = &ServiceDeskState{
		Stage:               "candidates_ready",
		CandidateServiceIDs: []uint{5, 8},
		TopMatchServiceID:   5,
	}
	handler := serviceConfirmHandler(store)
	ctx := context.WithValue(context.Background(), app.SessionIDKey, uint(1))

	if _, err := handler(ctx, 1, []byte(`{"service_id":9}`)); err == nil {
		t.Fatal("expected unknown service id/index to fail")
	}
	if state := store.states[1]; state.Stage != "candidates_ready" || state.ConfirmedServiceID != 0 {
		t.Fatalf("state should remain unchanged after failed confirm: %+v", state)
	}
}

func TestDraftPrepare_MultivalueRoutingField_ResolvedValues(t *testing.T) {
	store := newMemStateStore()
	op := &stubOperator{
		detail: &ServiceDetail{
			ServiceID: 1,
			FormFields: []FormField{
				{Key: "request_kind", Label: "访问原因", Type: "select", Required: true, Options: []string{"network_support", "security", "remote_maintenance"}},
			},
			RoutingFieldHint: &RoutingFieldHint{
				FieldKey: "request_kind",
				OptionRouteMap: map[string]string{
					"network_support":    "网络管理审批",
					"security":           "安全管理审批",
					"remote_maintenance": "网络管理审批",
				},
			},
			FieldsHash: "abc123",
		},
	}

	handler := draftPrepareHandler(op, store)

	// Pre-set state so service is loaded.
	store.states[1] = &ServiceDeskState{
		Stage:           "service_loaded",
		LoadedServiceID: 1,
		FieldsHash:      "abc123",
	}

	ctx := context.WithValue(context.Background(), app.SessionIDKey, uint(1))

	// Case 1: Routing field with cross-route multi-values → resolved_values present.
	args, _ := json.Marshal(map[string]any{
		"summary":   "VPN申请",
		"form_data": map[string]any{"request_kind": "network_support,security"},
	})
	result, err := handler(ctx, 1, args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var resp struct {
		Warnings []struct {
			Type           string `json:"type"`
			Field          string `json:"field"`
			ResolvedValues []struct {
				Value string `json:"value"`
				Route string `json:"route"`
			} `json:"resolved_values"`
		} `json:"warnings"`
	}
	if err := json.Unmarshal(result, &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}

	// Find multivalue warning.
	var found bool
	for _, w := range resp.Warnings {
		if w.Type == "multivalue_on_single_field" && w.Field == "request_kind" {
			found = true
			if len(w.ResolvedValues) != 2 {
				t.Fatalf("expected 2 resolved_values, got %d", len(w.ResolvedValues))
			}
			if w.ResolvedValues[0].Value != "network_support" || w.ResolvedValues[0].Route != "网络管理审批" {
				t.Errorf("resolved_values[0] = %+v, want {network_support, 网络管理审批}", w.ResolvedValues[0])
			}
			if w.ResolvedValues[1].Value != "security" || w.ResolvedValues[1].Route != "安全管理审批" {
				t.Errorf("resolved_values[1] = %+v, want {security, 安全管理审批}", w.ResolvedValues[1])
			}
		}
	}
	if !found {
		t.Fatal("expected multivalue_on_single_field warning for request_kind")
	}
}

func TestDraftPrepare_MultivalueNonRoutingField_NoResolvedValues(t *testing.T) {
	store := newMemStateStore()
	op := &stubOperator{
		detail: &ServiceDetail{
			ServiceID: 1,
			FormFields: []FormField{
				{Key: "request_kind", Label: "访问原因", Type: "select", Required: true, Options: []string{"network_support"}},
				{Key: "vpn_type", Label: "VPN类型", Type: "select", Required: true, Options: []string{"l2tp", "ipsec"}},
			},
			RoutingFieldHint: &RoutingFieldHint{
				FieldKey:       "request_kind",
				OptionRouteMap: map[string]string{"network_support": "网络管理审批"},
			},
			FieldsHash: "abc123",
		},
	}

	handler := draftPrepareHandler(op, store)

	store.states[1] = &ServiceDeskState{
		Stage:           "service_loaded",
		LoadedServiceID: 1,
		FieldsHash:      "abc123",
	}

	ctx := context.WithValue(context.Background(), app.SessionIDKey, uint(1))

	// Multi-value on non-routing field → no resolved_values.
	args, _ := json.Marshal(map[string]any{
		"summary":   "VPN申请",
		"form_data": map[string]any{"request_kind": "network_support", "vpn_type": "l2tp,ipsec"},
	})
	result, err := handler(ctx, 1, args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var resp struct {
		Warnings []struct {
			Type           string `json:"type"`
			Field          string `json:"field"`
			ResolvedValues []struct {
				Value string `json:"value"`
				Route string `json:"route"`
			} `json:"resolved_values"`
		} `json:"warnings"`
	}
	if err := json.Unmarshal(result, &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}

	for _, w := range resp.Warnings {
		if w.Type == "multivalue_on_single_field" && w.Field == "vpn_type" {
			if len(w.ResolvedValues) != 0 {
				t.Errorf("expected no resolved_values for non-routing field vpn_type, got %d", len(w.ResolvedValues))
			}
			return
		}
	}
	t.Fatal("expected multivalue_on_single_field warning for vpn_type")
}

// --- State Machine Transition Tests ---

func TestTransitionTo_ValidTransitions(t *testing.T) {
	tests := []struct {
		from string
		to   string
	}{
		{"idle", "candidates_ready"},
		{"candidates_ready", "service_selected"},
		{"candidates_ready", "service_loaded"},
		{"service_selected", "service_loaded"},
		{"service_loaded", "awaiting_confirmation"},
		{"awaiting_confirmation", "confirmed"},
		{"awaiting_confirmation", "awaiting_confirmation"}, // re-draft
		{"confirmed", "candidates_ready"},                  // restart
		// "candidates_ready" is universally allowed
		{"service_loaded", "candidates_ready"},
		{"awaiting_confirmation", "candidates_ready"},
	}
	for _, tt := range tests {
		s := &ServiceDeskState{Stage: tt.from}
		if err := s.TransitionTo(tt.to); err != nil {
			t.Errorf("TransitionTo(%q → %q) returned error: %v", tt.from, tt.to, err)
		}
		if s.Stage != tt.to {
			t.Errorf("after TransitionTo(%q → %q), Stage = %q", tt.from, tt.to, s.Stage)
		}
	}
}

func TestTransitionTo_InvalidTransitions(t *testing.T) {
	tests := []struct {
		from string
		to   string
	}{
		{"idle", "confirmed"},
		{"idle", "service_loaded"},
		{"idle", "awaiting_confirmation"},
		{"candidates_ready", "confirmed"},
		{"service_selected", "confirmed"},
		{"confirmed", "confirmed"},
	}
	for _, tt := range tests {
		s := &ServiceDeskState{Stage: tt.from}
		if err := s.TransitionTo(tt.to); err == nil {
			t.Errorf("TransitionTo(%q → %q) expected error, got nil", tt.from, tt.to)
		}
		if s.Stage != tt.from {
			t.Errorf("after failed TransitionTo(%q → %q), Stage changed to %q", tt.from, tt.to, s.Stage)
		}
	}
}

func TestTransitionTo_IdleAlwaysAllowed(t *testing.T) {
	stages := []string{"idle", "candidates_ready", "service_selected", "service_loaded", "awaiting_confirmation", "confirmed"}
	for _, from := range stages {
		s := &ServiceDeskState{Stage: from}
		if err := s.TransitionTo("idle"); err != nil {
			t.Errorf("TransitionTo(%q → idle) returned error: %v", from, err)
		}
		if s.Stage != "idle" {
			t.Errorf("after TransitionTo(%q → idle), Stage = %q", from, s.Stage)
		}
	}
}
