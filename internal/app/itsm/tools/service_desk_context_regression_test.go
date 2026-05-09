package tools

import (
	"context"
	"encoding/json"
	"testing"

	appcore "metis/internal/app"
)

type regressionStateStore struct {
	states map[uint]*ServiceDeskState
}

func newRegressionStateStore() *regressionStateStore {
	return &regressionStateStore{states: make(map[uint]*ServiceDeskState)}
}

func (m *regressionStateStore) GetState(sessionID uint) (*ServiceDeskState, error) {
	state, ok := m.states[sessionID]
	if !ok {
		return defaultState(), nil
	}
	return state, nil
}

func (m *regressionStateStore) SaveState(sessionID uint, state *ServiceDeskState) error {
	m.states[sessionID] = state
	return nil
}

type regressionOperator struct {
	detail *ServiceDetail
}

func (o *regressionOperator) MatchServices(ctx context.Context, query string) ([]ServiceMatch, MatchDecision, error) {
	return nil, MatchDecision{}, nil
}

func (o *regressionOperator) LoadService(serviceID uint) (*ServiceDetail, error) {
	return o.detail, nil
}

func (o *regressionOperator) CreateTicket(userID uint, serviceID uint, summary string, formData map[string]any, sessionID uint) (*TicketResult, error) {
	return &TicketResult{}, nil
}

func (o *regressionOperator) SubmitConfirmedDraft(userID uint, serviceID uint, serviceVersionID uint, summary string, formData map[string]any, sessionID uint, draftVersion int, fieldsHash string, requestHash string) (*TicketResult, error) {
	return &TicketResult{}, nil
}

func (o *regressionOperator) ListMyTickets(userID uint, status string) ([]TicketSummary, error) {
	return nil, nil
}

func (o *regressionOperator) WithdrawTicket(userID uint, ticketCode string, reason string) error {
	return nil
}

func (o *regressionOperator) ValidateParticipants(serviceID uint, formData map[string]any) (*ParticipantValidation, error) {
	return &ParticipantValidation{OK: true}, nil
}

func regressionVPNServiceDetail(serviceID uint) *ServiceDetail {
	return &ServiceDetail{
		ServiceID: serviceID,
		Name:      "VPN 开通申请",
		FormFields: []FormField{
			{Key: "vpn_account", Label: "VPN账号", Type: "text", Required: true},
			{Key: "device_usage", Label: "设备与用途说明", Type: "textarea", Required: true},
			{Key: "request_kind", Label: "访问原因", Type: "select", Required: true, Options: []FormOption{{Label: "线上支持", Value: "online_support"}}},
		},
		FieldsHash: "vpn123",
	}
}

func TestDraftPrepare_SummaryOnlyInputKeepsCollectMissingFieldsInCurrentContext(t *testing.T) {
	store := newRegressionStateStore()
	op := &regressionOperator{detail: regressionVPNServiceDetail(5)}
	store.states[1] = &ServiceDeskState{
		Stage:           "service_loaded",
		LoadedServiceID: 5,
		FieldsHash:      "vpn123",
	}
	ctx := context.WithValue(context.Background(), appcore.SessionIDKey, uint(1))

	result, err := draftPrepareHandler(op, store)(ctx, 1, []byte(`{"summary":"只有摘要"}`))
	if err != nil {
		t.Fatalf("prepare draft without form data: %v", err)
	}

	var payload map[string]any
	if err := json.Unmarshal(result, &payload); err != nil {
		t.Fatalf("unmarshal draft payload: %v", err)
	}
	if payload["next_required_tool"] != "collect_missing_fields" {
		t.Fatalf("expected next_required_tool=collect_missing_fields, got %+v", payload)
	}
	formData, ok := payload["form_data"].(map[string]any)
	if !ok {
		t.Fatalf("expected response to preserve form_data object, got %+v", payload["form_data"])
	}
	if len(formData) != 0 {
		t.Fatalf("expected empty form_data object, got %+v", formData)
	}

	contextResult, err := currentRequestContextHandler(store)(ctx, 1, []byte(`{}`))
	if err != nil {
		t.Fatalf("current request context: %v", err)
	}

	var contextPayload struct {
		NextExpectedAction string `json:"next_expected_action"`
		State              struct {
			PendingNextRequiredTool string `json:"pending_next_required_tool"`
		} `json:"state"`
	}
	if err := json.Unmarshal(contextResult, &contextPayload); err != nil {
		t.Fatalf("unmarshal context payload: %v", err)
	}
	if contextPayload.NextExpectedAction != "collect_missing_fields" {
		t.Fatalf("expected current context next_expected_action=collect_missing_fields, got %+v", contextPayload)
	}
	if contextPayload.State.PendingNextRequiredTool != "collect_missing_fields" {
		t.Fatalf("expected state to persist pending_next_required_tool, got %+v", contextPayload.State)
	}
}

func TestDraftPrepare_SuccessClearsPendingNextRequiredTool(t *testing.T) {
	store := newRegressionStateStore()
	op := &regressionOperator{detail: regressionVPNServiceDetail(5)}
	store.states[1] = &ServiceDeskState{
		Stage:                   "service_loaded",
		LoadedServiceID:         5,
		FieldsHash:              "vpn123",
		PendingNextRequiredTool: "collect_missing_fields",
	}
	ctx := context.WithValue(context.Background(), appcore.SessionIDKey, uint(1))

	_, err := draftPrepareHandler(op, store)(ctx, 1, []byte(`{
		"summary":"VPN 开通申请 - 线上支持用",
		"form_data":{
			"vpn_account":"wenhaowu@dev.com",
			"device_usage":"线上支持用",
			"request_kind":"online_support"
		}
	}`))
	if err != nil {
		t.Fatalf("prepare complete draft: %v", err)
	}

	contextResult, err := currentRequestContextHandler(store)(ctx, 1, []byte(`{}`))
	if err != nil {
		t.Fatalf("current request context: %v", err)
	}

	var contextPayload struct {
		NextExpectedAction string `json:"next_expected_action"`
		State              struct {
			Stage                   string `json:"stage"`
			PendingNextRequiredTool string `json:"pending_next_required_tool"`
		} `json:"state"`
	}
	if err := json.Unmarshal(contextResult, &contextPayload); err != nil {
		t.Fatalf("unmarshal context payload: %v", err)
	}
	if contextPayload.NextExpectedAction != "itsm.draft_confirm" {
		t.Fatalf("expected current context next_expected_action=itsm.draft_confirm, got %+v", contextPayload)
	}
	if contextPayload.State.Stage != "awaiting_confirmation" {
		t.Fatalf("expected state to advance to awaiting_confirmation, got %+v", contextPayload.State)
	}
	if contextPayload.State.PendingNextRequiredTool != "" {
		t.Fatalf("expected pending_next_required_tool to be cleared, got %+v", contextPayload.State)
	}
}

func TestDraftPrepare_MissingSmartFormSchemaKeepsGenerateReferencePathInCurrentContext(t *testing.T) {
	store := newRegressionStateStore()
	op := &regressionOperator{detail: &ServiceDetail{
		ServiceID:  5,
		Name:       "未生成参考路径的智能服务",
		EngineType: "smart",
		FieldsHash: "empty-form-schema",
		FormFields: nil,
		FormSchema: nil,
	}}
	store.states[1] = &ServiceDeskState{
		Stage:           "service_loaded",
		LoadedServiceID: 5,
		FieldsHash:      "empty-form-schema",
	}
	ctx := context.WithValue(context.Background(), appcore.SessionIDKey, uint(1))

	result, err := draftPrepareHandler(op, store)(ctx, 1, []byte(`{"summary":"申请","form_data":{}}`))
	if err != nil {
		t.Fatalf("prepare draft without generated form schema: %v", err)
	}

	var payload map[string]any
	if err := json.Unmarshal(result, &payload); err != nil {
		t.Fatalf("unmarshal draft payload: %v", err)
	}
	if payload["next_required_tool"] != "generate_reference_path" {
		t.Fatalf("expected next_required_tool=generate_reference_path, got %+v", payload)
	}

	contextResult, err := currentRequestContextHandler(store)(ctx, 1, []byte(`{}`))
	if err != nil {
		t.Fatalf("current request context: %v", err)
	}

	var contextPayload struct {
		NextExpectedAction string `json:"next_expected_action"`
		State              struct {
			PendingNextRequiredTool string `json:"pending_next_required_tool"`
		} `json:"state"`
	}
	if err := json.Unmarshal(contextResult, &contextPayload); err != nil {
		t.Fatalf("unmarshal context payload: %v", err)
	}
	if contextPayload.NextExpectedAction != "generate_reference_path" {
		t.Fatalf("expected current context next_expected_action=generate_reference_path, got %+v", contextPayload)
	}
	if contextPayload.State.PendingNextRequiredTool != "generate_reference_path" {
		t.Fatalf("expected state to persist generate_reference_path, got %+v", contextPayload.State)
	}
}

// TestDraftPrepare_EmptyFormDataRetryIsRejectedWhenPendingCollect verifies that
// when draft_prepare has already blocked with collect_missing_fields, calling it
// again with empty/nil form_data returns an explicit error rather than silently
// looping — this breaks the agent retry loop observed in production.
func TestDraftPrepare_EmptyFormDataRetryIsRejectedWhenPendingCollect(t *testing.T) {
	store := newRegressionStateStore()
	op := &regressionOperator{detail: regressionVPNServiceDetail(5)}
	store.states[1] = &ServiceDeskState{
		Stage:                   "service_loaded",
		LoadedServiceID:         5,
		FieldsHash:              "vpn123",
		PendingNextRequiredTool: "collect_missing_fields", // already blocked once
	}
	ctx := context.WithValue(context.Background(), appcore.SessionIDKey, uint(1))

	// Retry with only summary and no form_data — should be an explicit error, not ok=false.
	_, err := draftPrepareHandler(op, store)(ctx, 1, []byte(`{"summary":"仅摘要重试"}`))
	if err == nil {
		t.Fatal("expected error when retrying draft_prepare with empty form_data after collect_missing_fields block")
	}
	if !containsString(err.Error(), "collect_missing_fields") {
		t.Fatalf("expected error message to mention collect_missing_fields, got: %v", err)
	}
}

// TestDraftPrepare_NonEmptyFormDataRetryIsAllowedWhenPendingCollect verifies that
// when the agent provides non-empty form_data (user has answered), the call
// proceeds normally even if PendingNextRequiredTool was collect_missing_fields.
func TestDraftPrepare_NonEmptyFormDataRetryIsAllowedWhenPendingCollect(t *testing.T) {
	store := newRegressionStateStore()
	op := &regressionOperator{detail: regressionVPNServiceDetail(5)}
	store.states[1] = &ServiceDeskState{
		Stage:                   "service_loaded",
		LoadedServiceID:         5,
		FieldsHash:              "vpn123",
		PendingNextRequiredTool: "collect_missing_fields",
	}
	ctx := context.WithValue(context.Background(), appcore.SessionIDKey, uint(1))

	// Retry with real form_data — should NOT be rejected.
	result, err := draftPrepareHandler(op, store)(ctx, 1, []byte(`{
		"summary":"VPN \u5f00\u901a\u7533\u8bf7",
		"form_data":{
			"vpn_account":"wenhaowu@dev.com",
			"device_usage":"\u7ebf\u4e0a\u652f\u6301\u7528",
			"request_kind":"online_support"
		}
	}`))
	if err != nil {
		t.Fatalf("non-empty form_data retry should not be rejected: %v", err)
	}
	var payload map[string]any
	if err := json.Unmarshal(result, &payload); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if payload["ok"] != true {
		t.Fatalf("expected ok=true when form_data is complete, got %+v", payload)
	}
}

func containsString(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		func() bool {
			for i := 0; i <= len(s)-len(substr); i++ {
				if s[i:i+len(substr)] == substr {
					return true
				}
			}
			return false
		}())
}
