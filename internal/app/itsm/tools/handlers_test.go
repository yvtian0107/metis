package tools

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"metis/internal/app"
	"metis/internal/app/itsm/form"
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
	detail              *ServiceDetail
	details             map[uint]*ServiceDetail
	matchResponse       []ServiceMatch
	matchDecision       MatchDecision
	matchQueries        []string
	createdServiceID    uint
	createdSummary      string
	createdFormData     map[string]any
	createdDraftVersion int
	createdFieldsHash   string
	createdRequestHash  string
	validatedServiceID  uint
	validatedFormData   map[string]any
	participantResult   *ParticipantValidation
}

func (s *stubOperator) MatchServices(ctx context.Context, query string) ([]ServiceMatch, MatchDecision, error) {
	s.matchQueries = append(s.matchQueries, query)
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
func (s *stubOperator) SubmitConfirmedDraft(userID uint, serviceID uint, summary string, formData map[string]any, sessionID uint, draftVersion int, fieldsHash string, requestHash string) (*TicketResult, error) {
	s.createdServiceID = serviceID
	s.createdSummary = summary
	s.createdFormData = formData
	s.createdDraftVersion = draftVersion
	s.createdFieldsHash = fieldsHash
	s.createdRequestHash = requestHash
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
	if s.participantResult != nil {
		return s.participantResult, nil
	}
	return &ParticipantValidation{OK: true}, nil
}

func vpnServiceDetail(serviceID uint) *ServiceDetail {
	requestKindOptions := []FormOption{
		{Label: "线上支持", Value: "online_support"},
		{Label: "故障排查", Value: "troubleshooting"},
		{Label: "生产应急", Value: "production_emergency"},
		{Label: "网络接入问题", Value: "network_access_issue"},
		{Label: "外部协作", Value: "external_collaboration"},
		{Label: "长期远程办公", Value: "long_term_remote_work"},
		{Label: "跨境访问", Value: "cross_border_access"},
		{Label: "安全合规事项", Value: "security_compliance"},
	}
	return &ServiceDetail{
		ServiceID: serviceID,
		Name:      "VPN 开通申请",
		FormFields: []FormField{
			{Key: "vpn_account", Label: "VPN账号", Type: "text", Required: true, Description: "用于登录 VPN 的账号，用户给出的邮箱可直接作为账号"},
			{Key: "device_usage", Label: "设备与用途说明", Type: "textarea", Required: true, Description: "说明访问 VPN 的设备或用途；用户已给出用途时不需要额外追问设备型号"},
			{Key: "request_kind", Label: "访问原因", Type: "select", Required: true, Description: "选择 VPN 访问原因", Options: requestKindOptions},
		},
		RoutingFieldHint: &RoutingFieldHint{
			FieldKey: "request_kind",
			OptionRouteMap: map[string]string{
				"online_support":         "网络管理员处理",
				"troubleshooting":        "网络管理员处理",
				"production_emergency":   "网络管理员处理",
				"network_access_issue":   "网络管理员处理",
				"external_collaboration": "信息安全管理员处理",
				"long_term_remote_work":  "信息安全管理员处理",
				"cross_border_access":    "信息安全管理员处理",
				"security_compliance":    "信息安全管理员处理",
			},
		},
		FieldsHash: "vpn123",
	}
}

func smartVPNServiceDetail(serviceID uint) *ServiceDetail {
	detail := vpnServiceDetail(serviceID)
	detail.EngineType = "smart"
	detail.FormSchema = map[string]any{
		"version": 1,
		"fields": []map[string]any{
			{"key": "vpn_account", "type": "text", "label": "VPN账号", "required": true},
			{"key": "device_usage", "type": "textarea", "label": "设备与用途说明", "required": true},
			{"key": "request_kind", "type": "select", "label": "访问原因", "required": true},
		},
	}
	return detail
}

func TestServiceLoad_ReturnsPrefillSuggestionsFromRequestText(t *testing.T) {
	store := newMemStateStore()
	op := &stubOperator{
		matchResponse: []ServiceMatch{
			{ID: 5, Name: "VPN 开通申请", CatalogPath: "基础设施与网络/网络与 VPN", Description: "VPN 开通", Score: 0.97, Reason: "用户明确要求申请 VPN"},
		},
		matchDecision: MatchDecision{Kind: MatchDecisionSelectService, SelectedServiceID: 5},
		details:       map[uint]*ServiceDetail{5: vpnServiceDetail(5)},
	}
	ctx := context.WithValue(context.Background(), app.SessionIDKey, uint(1))

	if _, err := serviceMatchHandler(op, store)(ctx, 1, []byte(`{"query":"我要申请VPN，线上支持用的，wenhaowu@dev.com"}`)); err != nil {
		t.Fatalf("service match: %v", err)
	}
	result, err := serviceLoadHandler(op, store)(ctx, 1, []byte(`{"service_id":5}`))
	if err != nil {
		t.Fatalf("service load: %v", err)
	}

	var resp ServiceDetail
	if err := json.Unmarshal(result, &resp); err != nil {
		t.Fatalf("unmarshal service detail: %v", err)
	}
	if resp.PrefillSuggestions["vpn_account"] != "wenhaowu@dev.com" ||
		resp.PrefillSuggestions["device_usage"] != "线上支持用" ||
		resp.PrefillSuggestions["request_kind"] != "online_support" {
		t.Fatalf("unexpected prefill suggestions: %+v", resp.PrefillSuggestions)
	}
	state := store.states[1]
	if state.RequestText != "我要申请VPN，线上支持用的，wenhaowu@dev.com" {
		t.Fatalf("expected request text persisted, got %q", state.RequestText)
	}
	if state.PrefillFormData["vpn_account"] != "wenhaowu@dev.com" {
		t.Fatalf("expected prefill data in state, got %+v", state.PrefillFormData)
	}
	if resp.FieldCollection == nil {
		t.Fatalf("expected field collection summary")
	}
	if len(resp.FieldCollection.RequiredFields) != 3 {
		t.Fatalf("expected three required fields, got %+v", resp.FieldCollection.RequiredFields)
	}
	if len(resp.FieldCollection.PrefilledFields) != 3 {
		t.Fatalf("expected three prefilled fields, got %+v", resp.FieldCollection.PrefilledFields)
	}
	if len(resp.FieldCollection.MissingRequiredFields) != 0 {
		t.Fatalf("expected no missing required fields, got %+v", resp.FieldCollection.MissingRequiredFields)
	}
	if !resp.FieldCollection.ReadyForDraft || resp.FieldCollection.NextRequiredTool != "itsm.draft_prepare" {
		t.Fatalf("expected draft to be ready, got %+v", resp.FieldCollection)
	}
}

func TestServiceLoad_UsesOriginalUserMessageWhenMatchQueryIsAbbreviated(t *testing.T) {
	store := newMemStateStore()
	op := &stubOperator{
		matchResponse: []ServiceMatch{
			{ID: 5, Name: "VPN 开通申请", CatalogPath: "基础设施与网络/网络与 VPN", Description: "VPN 开通", Score: 0.97, Reason: "用户明确要求申请 VPN"},
		},
		matchDecision: MatchDecision{Kind: MatchDecisionSelectService, SelectedServiceID: 5},
		details:       map[uint]*ServiceDetail{5: vpnServiceDetail(5)},
	}
	ctx := context.WithValue(context.Background(), app.SessionIDKey, uint(1))
	ctx = context.WithValue(ctx, app.UserMessageKey, "我要申请VPN，线上支持用的，wenhaowu@dev.com")

	if _, err := serviceMatchHandler(op, store)(ctx, 1, []byte(`{"query":"申请VPN"}`)); err != nil {
		t.Fatalf("service match: %v", err)
	}
	result, err := serviceLoadHandler(op, store)(ctx, 1, []byte(`{"service_id":5}`))
	if err != nil {
		t.Fatalf("service load: %v", err)
	}

	var resp ServiceDetail
	if err := json.Unmarshal(result, &resp); err != nil {
		t.Fatalf("unmarshal service detail: %v", err)
	}
	if resp.PrefillSuggestions["vpn_account"] != "wenhaowu@dev.com" ||
		resp.PrefillSuggestions["device_usage"] != "线上支持用" ||
		resp.PrefillSuggestions["request_kind"] != "online_support" {
		t.Fatalf("expected prefill from original user message, got %+v", resp.PrefillSuggestions)
	}
	if state := store.states[1]; state.RequestText != "我要申请VPN，线上支持用的，wenhaowu@dev.com" {
		t.Fatalf("expected original request text in state, got %q", state.RequestText)
	}
}

func TestServiceLoad_ReturnsMissingFieldsAndRecommendedStep(t *testing.T) {
	store := newMemStateStore()
	op := &stubOperator{
		matchResponse: []ServiceMatch{
			{ID: 5, Name: "VPN 开通申请", Description: "VPN 开通", Score: 0.97, Reason: "用户明确要求申请 VPN"},
		},
		matchDecision: MatchDecision{Kind: MatchDecisionSelectService, SelectedServiceID: 5},
		details:       map[uint]*ServiceDetail{5: vpnServiceDetail(5)},
	}
	ctx := context.WithValue(context.Background(), app.SessionIDKey, uint(1))

	if _, err := serviceMatchHandler(op, store)(ctx, 1, []byte(`{"query":"帮我开个VPN"}`)); err != nil {
		t.Fatalf("service match: %v", err)
	}
	result, err := serviceLoadHandler(op, store)(ctx, 1, []byte(`{"service_id":5}`))
	if err != nil {
		t.Fatalf("service load: %v", err)
	}

	var resp ServiceDetail
	if err := json.Unmarshal(result, &resp); err != nil {
		t.Fatalf("unmarshal service detail: %v", err)
	}
	if resp.FieldCollection == nil {
		t.Fatalf("expected field collection summary")
	}
	if len(resp.FieldCollection.MissingRequiredFields) != 3 {
		t.Fatalf("expected three missing required fields, got %+v", resp.FieldCollection.MissingRequiredFields)
	}
	if resp.FieldCollection.ReadyForDraft {
		t.Fatalf("expected draft to be blocked by missing fields")
	}
	if resp.FieldCollection.RecommendedNextStep != "ask_missing_fields" || resp.FieldCollection.NextRequiredTool != "" {
		t.Fatalf("expected ask_missing_fields recommendation, got %+v", resp.FieldCollection)
	}
	state := store.states[1]
	if state == nil {
		t.Fatal("expected service state to be persisted")
	}
	if state.MinDecisionReady {
		t.Fatalf("expected min decision not ready, got state %+v", state)
	}
	if len(state.MissingFields) != 3 || len(state.AskedFields) != 3 {
		t.Fatalf("expected missing/asked fields to track all required fields, got missing=%v asked=%v", state.MissingFields, state.AskedFields)
	}
}

func TestServiceMatch_ShortConfirmationReusesLoadedServiceWithoutClearingPrefill(t *testing.T) {
	store := newMemStateStore()
	op := &stubOperator{
		matchResponse: []ServiceMatch{
			{ID: 5, Name: "VPN 开通申请", CatalogPath: "基础设施与网络/网络与 VPN", Description: "VPN 开通", Score: 0.97, Reason: "用户明确要求申请 VPN"},
		},
		matchDecision: MatchDecision{Kind: MatchDecisionSelectService, SelectedServiceID: 5},
		details:       map[uint]*ServiceDetail{5: vpnServiceDetail(5)},
	}
	ctx := context.WithValue(context.Background(), app.SessionIDKey, uint(1))

	if _, err := serviceMatchHandler(op, store)(ctx, 1, []byte(`{"query":"我要申请VPN，线上支持用的，wenhaowu@dev.com"}`)); err != nil {
		t.Fatalf("service match: %v", err)
	}
	if _, err := serviceLoadHandler(op, store)(ctx, 1, []byte(`{"service_id":5}`)); err != nil {
		t.Fatalf("service load: %v", err)
	}
	if len(op.matchQueries) != 1 {
		t.Fatalf("expected initial match only, got queries %+v", op.matchQueries)
	}

	result, err := serviceMatchHandler(op, store)(ctx, 1, []byte(`{"query":"是的"}`))
	if err != nil {
		t.Fatalf("short confirmation match guard: %v", err)
	}

	var resp struct {
		AlreadyLoaded    bool   `json:"already_loaded"`
		LoadedServiceID  uint   `json:"loaded_service_id"`
		NextRequiredTool string `json:"next_required_tool"`
		StateStage       string `json:"state_stage"`
	}
	if err := json.Unmarshal(result, &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if !resp.AlreadyLoaded || resp.LoadedServiceID != 5 || resp.NextRequiredTool != "itsm.draft_prepare" {
		t.Fatalf("expected existing loaded service reuse, got %+v", resp)
	}
	if len(op.matchQueries) != 1 {
		t.Fatalf("short confirmation should not call matcher again, got queries %+v", op.matchQueries)
	}
	state := store.states[1]
	if state.RequestText != "我要申请VPN，线上支持用的，wenhaowu@dev.com" {
		t.Fatalf("request text was overwritten: %q", state.RequestText)
	}
	if state.PrefillFormData["vpn_account"] != "wenhaowu@dev.com" ||
		state.PrefillFormData["device_usage"] != "线上支持用" ||
		state.PrefillFormData["request_kind"] != "online_support" {
		t.Fatalf("prefill was cleared or changed: %+v", state.PrefillFormData)
	}
}

func TestCurrentRequestContext_ReturnsStateAndNextExpectedAction(t *testing.T) {
	store := newMemStateStore()
	store.states[1] = &ServiceDeskState{
		Stage:           "service_loaded",
		LoadedServiceID: 5,
		RequestText:     "我要申请VPN，线上支持用的，wenhaowu@dev.com",
		PrefillFormData: map[string]any{
			"vpn_account":  "wenhaowu@dev.com",
			"device_usage": "线上支持用",
			"request_kind": "online_support",
		},
	}
	ctx := context.WithValue(context.Background(), app.SessionIDKey, uint(1))

	result, err := currentRequestContextHandler(store)(ctx, 1, []byte(`{}`))
	if err != nil {
		t.Fatalf("current context: %v", err)
	}

	var resp struct {
		Stage              string         `json:"stage"`
		LoadedServiceID    uint           `json:"loaded_service_id"`
		RequestText        string         `json:"request_text"`
		PrefillFormData    map[string]any `json:"prefill_form_data"`
		MissingFields      []string       `json:"missing_fields"`
		AskedFields        []string       `json:"asked_fields"`
		MinDecisionReady   bool           `json:"min_decision_ready"`
		NextExpectedAction string         `json:"next_expected_action"`
	}
	if err := json.Unmarshal(result, &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if resp.Stage != "service_loaded" || resp.LoadedServiceID != 5 || resp.NextExpectedAction != "itsm.draft_prepare" {
		t.Fatalf("unexpected context response: %+v", resp)
	}
	if resp.RequestText == "" || resp.PrefillFormData["vpn_account"] != "wenhaowu@dev.com" {
		t.Fatalf("expected request text and prefill data, got %+v", resp)
	}
	if resp.MinDecisionReady || len(resp.MissingFields) != 0 || len(resp.AskedFields) != 0 {
		t.Fatalf("expected no missing/asked fields in prefilled state, got %+v", resp)
	}
}

func TestDraftPrepare_UsesPrefillSuggestionsBeforeRequiredValidation(t *testing.T) {
	store := newMemStateStore()
	op := &stubOperator{detail: vpnServiceDetail(5)}
	store.states[1] = &ServiceDeskState{
		Stage:           "service_loaded",
		LoadedServiceID: 5,
		FieldsHash:      "vpn123",
		PrefillFormData: map[string]any{
			"vpn_account":  "wenhaowu@dev.com",
			"device_usage": "线上支持用",
			"request_kind": "online_support",
		},
	}
	ctx := context.WithValue(context.Background(), app.SessionIDKey, uint(1))

	result, err := draftPrepareHandler(op, store)(ctx, 1, []byte(`{"summary":"VPN 开通申请 - 线上支持用","form_data":{"request_kind":"online_support"}}`))
	if err != nil {
		t.Fatalf("prepare draft: %v", err)
	}

	var resp struct {
		OK                   bool           `json:"ok"`
		ReadyForConfirmation bool           `json:"ready_for_confirmation"`
		NextRequiredTool     string         `json:"next_required_tool"`
		FormData             map[string]any `json:"form_data"`
		Warnings             []struct {
			Type  string `json:"type"`
			Field string `json:"field"`
		} `json:"warnings"`
	}
	if err := json.Unmarshal(result, &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if !resp.OK {
		t.Fatalf("expected prefill to satisfy required fields, got %s", string(result))
	}
	if !resp.ReadyForConfirmation || resp.NextRequiredTool != "itsm.draft_confirm" {
		t.Fatalf("expected draft to be ready for confirmation, got ready=%v next=%q", resp.ReadyForConfirmation, resp.NextRequiredTool)
	}
	if resp.FormData["vpn_account"] != "wenhaowu@dev.com" ||
		resp.FormData["device_usage"] != "线上支持用" ||
		resp.FormData["request_kind"] != "online_support" {
		t.Fatalf("expected complete form data from prefill, got %+v", resp.FormData)
	}
}

func TestDraftPrepare_BlocksAmbiguousRelativeTimeWindow(t *testing.T) {
	store := newMemStateStore()
	detail := vpnServiceDetail(5)
	detail.FormFields = append(detail.FormFields, FormField{
		Key:      "access_period",
		Label:    "访问时段",
		Type:     "text",
		Required: false,
	})
	op := &stubOperator{detail: detail}
	store.states[1] = &ServiceDeskState{
		Stage:           "service_loaded",
		LoadedServiceID: 5,
		FieldsHash:      "vpn123",
	}
	ctx := context.WithValue(context.Background(), app.SessionIDKey, uint(1))

	result, err := draftPrepareHandler(op, store)(ctx, 1, []byte(`{
		"summary":"VPN 开通申请，访问时段明天晚上",
		"form_data":{
			"vpn_account":"wenhaowu@dev.com",
			"device_usage":"线上支持用",
			"request_kind":"online_support",
			"access_period":"2026-04-29 18:00:00 ~ 2026-04-29 23:59:59"
		}
	}`))
	if err != nil {
		t.Fatalf("prepare draft: %v", err)
	}

	var resp struct {
		OK                    bool `json:"ok"`
		ReadyForConfirmation  bool `json:"ready_for_confirmation"`
		MissingRequiredFields []struct {
			Key    string `json:"key"`
			Source string `json:"source"`
		} `json:"missing_required_fields"`
		Warnings []struct {
			Type  string `json:"type"`
			Field string `json:"field"`
		} `json:"warnings"`
	}
	if err := json.Unmarshal(result, &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if resp.OK || resp.ReadyForConfirmation {
		t.Fatalf("expected ambiguous time window to block draft confirmation, got %s", string(result))
	}
	foundWarning := false
	for _, warning := range resp.Warnings {
		if warning.Type == "ambiguous_time" && warning.Field == "access_period" {
			foundWarning = true
			break
		}
	}
	if !foundWarning {
		t.Fatalf("expected ambiguous_time warning for access_period, got %+v", resp.Warnings)
	}
}

func TestDraftPrepare_CanonicalizesLabeledAbsoluteTimeIntoTimeField(t *testing.T) {
	store := newMemStateStore()
	detail := vpnServiceDetail(5)
	detail.FormFields = append(detail.FormFields, FormField{
		Key:      "access_period",
		Label:    "访问时段",
		Type:     "text",
		Required: false,
	})
	op := &stubOperator{detail: detail}
	store.states[1] = &ServiceDeskState{
		Stage:           "service_loaded",
		LoadedServiceID: 5,
		FieldsHash:      "vpn123",
	}
	ctx := context.WithValue(context.Background(), app.SessionIDKey, uint(1))

	result, err := draftPrepareHandler(op, store)(ctx, 1, []byte(`{
		"summary":"VPN 开通申请",
		"form_data":{
			"vpn_account":"wenhaowu@dev.com",
			"device_usage":"线上支持用",
			"request_kind":"online_support",
			"reason":"线上支持用，访问时段2026-04-28 12:00:00~2026-04-29 10:00:00"
		}
	}`))
	if err != nil {
		t.Fatalf("prepare draft: %v", err)
	}

	var resp struct {
		OK                   bool           `json:"ok"`
		ReadyForConfirmation bool           `json:"ready_for_confirmation"`
		FormData             map[string]any `json:"form_data"`
	}
	if err := json.Unmarshal(result, &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if !resp.OK || !resp.ReadyForConfirmation {
		t.Fatalf("expected canonicalized time field to be ready, got %s", string(result))
	}
	if resp.FormData["access_period"] != "2026-04-28 12:00:00~2026-04-29 10:00:00" {
		t.Fatalf("expected access_period to be canonicalized, got %+v", resp.FormData)
	}
}

func TestDraftPrepare_BlocksUsernameForEmailSemanticAccountField(t *testing.T) {
	store := newMemStateStore()
	op := &stubOperator{detail: vpnServiceDetail(5)}
	store.states[1] = &ServiceDeskState{
		Stage:           "service_loaded",
		LoadedServiceID: 5,
		FieldsHash:      "vpn123",
	}
	ctx := context.WithValue(context.Background(), app.SessionIDKey, uint(1))

	result, err := draftPrepareHandler(op, store)(ctx, 1, []byte(`{
		"summary":"VPN 开通申请 - 线上支持用",
		"form_data":{
			"vpn_account":"admin",
			"device_usage":"线上支持用",
			"request_kind":"online_support"
		}
	}`))
	if err != nil {
		t.Fatalf("prepare draft: %v", err)
	}

	var resp struct {
		OK                    bool `json:"ok"`
		ReadyForConfirmation  bool `json:"ready_for_confirmation"`
		MissingRequiredFields []struct {
			Key string `json:"key"`
		} `json:"missing_required_fields"`
		Warnings []struct {
			Type    string `json:"type"`
			Field   string `json:"field"`
			Message string `json:"message"`
		} `json:"warnings"`
	}
	if err := json.Unmarshal(result, &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if resp.OK || resp.ReadyForConfirmation {
		t.Fatalf("expected username-as-email to block draft, got %s", string(result))
	}
	if len(resp.MissingRequiredFields) != 1 || resp.MissingRequiredFields[0].Key != "vpn_account" {
		t.Fatalf("expected vpn_account to be returned as missing, got %+v", resp.MissingRequiredFields)
	}
	if len(resp.Warnings) != 1 || resp.Warnings[0].Type != "invalid_email" || resp.Warnings[0].Field != "vpn_account" {
		t.Fatalf("expected invalid_email warning for vpn_account, got %+v", resp.Warnings)
	}
	if !strings.Contains(resp.Warnings[0].Message, "不能用用户名代替邮箱") {
		t.Fatalf("expected username substitution warning message, got %q", resp.Warnings[0].Message)
	}
}

func TestDraftPrepare_AllowsEmailForEmailSemanticAccountField(t *testing.T) {
	store := newMemStateStore()
	op := &stubOperator{detail: vpnServiceDetail(5)}
	store.states[1] = &ServiceDeskState{
		Stage:           "service_loaded",
		LoadedServiceID: 5,
		FieldsHash:      "vpn123",
	}
	ctx := context.WithValue(context.Background(), app.SessionIDKey, uint(1))

	result, err := draftPrepareHandler(op, store)(ctx, 1, []byte(`{
		"summary":"VPN 开通申请 - 线上支持用",
		"form_data":{
			"vpn_account":"admin@local.dev",
			"device_usage":"线上支持用",
			"request_kind":"online_support"
		}
	}`))
	if err != nil {
		t.Fatalf("prepare draft: %v", err)
	}

	var resp struct {
		OK                   bool           `json:"ok"`
		ReadyForConfirmation bool           `json:"ready_for_confirmation"`
		NextRequiredTool     string         `json:"next_required_tool"`
		FormData             map[string]any `json:"form_data"`
	}
	if err := json.Unmarshal(result, &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if !resp.OK || !resp.ReadyForConfirmation || resp.NextRequiredTool != "itsm.draft_confirm" {
		t.Fatalf("expected email account to be accepted, got %s", string(result))
	}
	if resp.FormData["vpn_account"] != "admin@local.dev" {
		t.Fatalf("expected email account to be preserved, got %+v", resp.FormData)
	}
}

func TestDraftPrepare_BlocksFreeTextForVPNRequestKind(t *testing.T) {
	store := newMemStateStore()
	op := &stubOperator{detail: vpnServiceDetail(5)}
	store.states[1] = &ServiceDeskState{
		Stage:           "service_loaded",
		LoadedServiceID: 5,
		FieldsHash:      "vpn123",
	}
	ctx := context.WithValue(context.Background(), app.SessionIDKey, uint(1))

	result, err := draftPrepareHandler(op, store)(ctx, 1, []byte(`{
		"summary":"VPN 开通申请 - 线上支持用",
		"form_data":{
			"vpn_account":"admin@local.dev",
			"device_usage":"线上支持用",
			"request_kind":"线上支持用"
		}
	}`))
	if err != nil {
		t.Fatalf("prepare draft: %v", err)
	}

	var resp struct {
		OK                   bool `json:"ok"`
		ReadyForConfirmation bool `json:"ready_for_confirmation"`
		Warnings             []struct {
			Type  string `json:"type"`
			Field string `json:"field"`
		} `json:"warnings"`
	}
	if err := json.Unmarshal(result, &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if resp.OK || resp.ReadyForConfirmation {
		t.Fatalf("expected free-text request_kind to block draft, got %s", string(result))
	}
	if len(resp.Warnings) != 1 || resp.Warnings[0].Type != "invalid_option" || resp.Warnings[0].Field != "request_kind" {
		t.Fatalf("expected invalid_option for request_kind, got %+v", resp.Warnings)
	}
}

func TestDraftPrepare_BlocksInvalidStructuredFields(t *testing.T) {
	store := newMemStateStore()
	op := &stubOperator{detail: &ServiceDetail{
		ServiceID: 9,
		Name:      "复杂表单",
		FormFields: []FormField{
			{Key: "title", Label: "标题", Type: form.FieldText, Required: true},
			{Key: "tags", Label: "标签", Type: form.FieldMultiSelect, Required: true, Options: []FormOption{{Label: "VPN", Value: "vpn"}}},
			{Key: "items", Label: "明细", Type: form.FieldTable, Required: true, Props: map[string]any{"columns": []form.TableColumn{
				{Key: "name", Type: form.FieldText, Label: "名称", Required: true},
				{Key: "kind", Type: form.FieldSelect, Label: "类型", Required: true, Options: []form.FieldOption{{Label: "网络", Value: "network"}}},
			}}},
		},
		FieldsHash: "complex123",
	}}
	store.states[1] = &ServiceDeskState{Stage: "service_loaded", LoadedServiceID: 9, FieldsHash: "complex123"}
	ctx := context.WithValue(context.Background(), app.SessionIDKey, uint(1))

	tests := []struct {
		name string
		data map[string]any
		want string
	}{
		{
			name: "multi select text",
			data: map[string]any{"title": "申请", "tags": "vpn", "items": []any{map[string]any{"name": "A", "kind": "network"}}},
			want: "tags",
		},
		{
			name: "table shape",
			data: map[string]any{"title": "申请", "tags": []any{"vpn"}, "items": map[string]any{"name": "A"}},
			want: "items",
		},
		{
			name: "table column required",
			data: map[string]any{"title": "申请", "tags": []any{"vpn"}, "items": []any{map[string]any{"kind": "network"}}},
			want: "items[0].name",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			args, _ := json.Marshal(map[string]any{"summary": "复杂表单", "form_data": tt.data})
			result, err := draftPrepareHandler(op, store)(ctx, 1, args)
			if err != nil {
				t.Fatalf("prepare draft: %v", err)
			}
			var resp struct {
				OK                   bool `json:"ok"`
				ReadyForConfirmation bool `json:"ready_for_confirmation"`
				Warnings             []struct {
					Field string `json:"field"`
				} `json:"warnings"`
			}
			if err := json.Unmarshal(result, &resp); err != nil {
				t.Fatalf("unmarshal response: %v", err)
			}
			if resp.OK || resp.ReadyForConfirmation {
				t.Fatalf("expected invalid structured field to block draft, got %s", string(result))
			}
			if len(resp.Warnings) == 0 || resp.Warnings[0].Field != tt.want {
				t.Fatalf("expected first warning field %s, got %+v", tt.want, resp.Warnings)
			}
		})
	}
}

func TestDraftPrepare_StillReportsMissingAccountWhenRequestHasNoAccount(t *testing.T) {
	store := newMemStateStore()
	op := &stubOperator{detail: vpnServiceDetail(5)}
	store.states[1] = &ServiceDeskState{
		Stage:           "service_loaded",
		LoadedServiceID: 5,
		FieldsHash:      "vpn123",
		PrefillFormData: map[string]any{
			"device_usage": "线上支持用",
			"request_kind": "online_support",
		},
	}
	ctx := context.WithValue(context.Background(), app.SessionIDKey, uint(1))

	result, err := draftPrepareHandler(op, store)(ctx, 1, []byte(`{"summary":"VPN 开通申请 - 线上支持用","form_data":{}}`))
	if err != nil {
		t.Fatalf("prepare draft: %v", err)
	}

	var resp struct {
		OK                    bool   `json:"ok"`
		ReadyForConfirmation  bool   `json:"ready_for_confirmation"`
		NextRequiredTool      string `json:"next_required_tool"`
		MissingRequiredFields []struct {
			Key string `json:"key"`
		} `json:"missing_required_fields"`
		Warnings []struct {
			Type  string `json:"type"`
			Field string `json:"field"`
		} `json:"warnings"`
	}
	if err := json.Unmarshal(result, &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if resp.OK {
		t.Fatalf("expected missing vpn_account to block draft, got %s", string(result))
	}
	if resp.ReadyForConfirmation || resp.NextRequiredTool != "collect_missing_fields" {
		t.Fatalf("expected missing fields recommendation, got ready=%v next=%q", resp.ReadyForConfirmation, resp.NextRequiredTool)
	}
	if len(resp.Warnings) != 1 || resp.Warnings[0].Field != "vpn_account" {
		t.Fatalf("expected only vpn_account missing, got %+v", resp.Warnings)
	}
	if len(resp.MissingRequiredFields) != 1 || resp.MissingRequiredFields[0].Key != "vpn_account" {
		t.Fatalf("expected missing field detail for vpn_account, got %+v", resp.MissingRequiredFields)
	}
	if state := store.states[1]; state == nil || !containsAll(state.MissingFields, "vpn_account") || !containsAll(state.AskedFields, "vpn_account") {
		t.Fatalf("expected state to track missing/asked vpn_account, got %+v", state)
	}
}

func TestDraftPrepare_DoesNotReaskConfirmedFields(t *testing.T) {
	store := newMemStateStore()
	op := &stubOperator{
		matchResponse: []ServiceMatch{
			{ID: 5, Name: "VPN 开通申请", Description: "VPN 开通", Score: 0.97, Reason: "用户明确要求申请 VPN"},
		},
		matchDecision: MatchDecision{Kind: MatchDecisionSelectService, SelectedServiceID: 5},
		details:       map[uint]*ServiceDetail{5: vpnServiceDetail(5)},
	}
	ctx := context.WithValue(context.Background(), app.SessionIDKey, uint(1))

	if _, err := serviceMatchHandler(op, store)(ctx, 1, []byte(`{"query":"帮我开个VPN"}`)); err != nil {
		t.Fatalf("service match: %v", err)
	}
	if _, err := serviceLoadHandler(op, store)(ctx, 1, []byte(`{"service_id":5}`)); err != nil {
		t.Fatalf("service load: %v", err)
	}
	if state := store.states[1]; state == nil || len(state.MissingFields) != 3 {
		t.Fatalf("expected initial missing fields captured, got %+v", state)
	}

	result, err := draftPrepareHandler(op, store)(ctx, 1, []byte(`{"summary":"VPN申请","form_data":{"vpn_account":"wenhaowu@dev.com"}}`))
	if err != nil {
		t.Fatalf("prepare draft: %v", err)
	}
	var resp struct {
		OK                    bool `json:"ok"`
		MissingRequiredFields []struct {
			Key string `json:"key"`
		} `json:"missing_required_fields"`
	}
	if err := json.Unmarshal(result, &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if resp.OK {
		t.Fatalf("expected draft still blocked, got %s", string(result))
	}
	for _, item := range resp.MissingRequiredFields {
		if item.Key == "vpn_account" {
			t.Fatalf("expected confirmed field vpn_account not to be re-asked, got %+v", resp.MissingRequiredFields)
		}
	}
	state := store.states[1]
	if state == nil || containsAll(state.MissingFields, "vpn_account") || containsAll(state.AskedFields, "vpn_account") {
		t.Fatalf("expected confirmed field removed from missing/asked tracking, got %+v", state)
	}
}

func containsAll(values []string, target string) bool {
	for _, item := range values {
		if item == target {
			return true
		}
	}
	return false
}

func TestSubmitDraft_UsesSubmittedFormDataForSmartTicket(t *testing.T) {
	store := newMemStateStore()
	op := &stubOperator{detail: smartVPNServiceDetail(5)}
	store.states[1] = &ServiceDeskState{
		Stage:           "awaiting_confirmation",
		LoadedServiceID: 5,
		DraftSummary:    "VPN 开通申请",
		DraftFormData: map[string]any{
			"vpn_account":  "old@example.com",
			"device_usage": "线上支持",
			"request_kind": "online_support",
		},
		DraftVersion: 1,
		FieldsHash:   "vpn123",
	}

	result, err := SubmitDraft(op, store, 1, 7, DraftSubmitRequest{
		DraftVersion: 1,
		Summary:      "VPN 开通申请 - 修改后",
		FormData: map[string]any{
			"vpn_account":  "new@example.com",
			"device_usage": "生产应急",
			"request_kind": "production_emergency",
		},
	})
	if err != nil {
		t.Fatalf("submit draft: %v", err)
	}
	if !result.OK || result.TicketCode != "TICK-000123" {
		t.Fatalf("expected ticket created, got %+v", result)
	}
	if op.createdServiceID != 5 || op.createdSummary != "VPN 开通申请 - 修改后" {
		t.Fatalf("unexpected created ticket target: service=%d summary=%q", op.createdServiceID, op.createdSummary)
	}
	if op.createdFormData["vpn_account"] != "new@example.com" || op.createdFormData["request_kind"] != "production_emergency" {
		t.Fatalf("expected submitted form data to create ticket, got %+v", op.createdFormData)
	}
	if state := store.states[1]; state.Stage != "idle" {
		t.Fatalf("expected state reset after submit, got %+v", state)
	}
	if result.Surface == nil || result.Surface["surfaceType"] != "itsm.draft_form" {
		t.Fatalf("expected submitted surface metadata, got %+v", result.Surface)
	}
}

func TestSubmitDraft_RejectsNonSmartService(t *testing.T) {
	store := newMemStateStore()
	op := &stubOperator{detail: vpnServiceDetail(5)}
	store.states[1] = &ServiceDeskState{
		Stage:           "awaiting_confirmation",
		LoadedServiceID: 5,
		DraftVersion:    1,
		FieldsHash:      "vpn123",
	}

	if _, err := SubmitDraft(op, store, 1, 7, DraftSubmitRequest{DraftVersion: 1}); err == nil {
		t.Fatal("expected non-smart service submit to be cancelled")
	}
	if op.createdServiceID != 0 {
		t.Fatalf("ticket should not be created for non-smart service, got service %d", op.createdServiceID)
	}
}

func TestSubmitDraft_RejectsStaleDraftVersion(t *testing.T) {
	store := newMemStateStore()
	op := &stubOperator{detail: smartVPNServiceDetail(5)}
	store.states[1] = &ServiceDeskState{
		Stage:           "awaiting_confirmation",
		LoadedServiceID: 5,
		DraftVersion:    3,
		FieldsHash:      "vpn123",
	}

	if _, err := SubmitDraft(op, store, 1, 7, DraftSubmitRequest{DraftVersion: 2}); err == nil {
		t.Fatal("expected stale draft version to be cancelled")
	}
}

func TestSubmitDraft_RejectsChangedFieldsHash(t *testing.T) {
	store := newMemStateStore()
	detail := smartVPNServiceDetail(5)
	detail.FieldsHash = "changed"
	op := &stubOperator{detail: detail}
	store.states[1] = &ServiceDeskState{
		Stage:           "awaiting_confirmation",
		LoadedServiceID: 5,
		DraftVersion:    1,
		FieldsHash:      "vpn123",
	}

	if _, err := SubmitDraft(op, store, 1, 7, DraftSubmitRequest{DraftVersion: 1}); err == nil {
		t.Fatal("expected changed fields hash to be cancelled")
	}
}

func TestSubmitDraft_ParticipantPrecheckFailureDoesNotCreateTicket(t *testing.T) {
	store := newMemStateStore()
	op := &stubOperator{
		detail:            smartVPNServiceDetail(5),
		participantResult: &ParticipantValidation{OK: false, FailureReason: "no owner", NodeLabel: "处理人", Guidance: "补充负责人"},
	}
	store.states[1] = &ServiceDeskState{
		Stage:           "awaiting_confirmation",
		LoadedServiceID: 5,
		DraftVersion:    1,
		FieldsHash:      "vpn123",
	}

	result, err := SubmitDraft(op, store, 1, 7, DraftSubmitRequest{
		DraftVersion: 1,
		Summary:      "VPN 开通申请",
		FormData: map[string]any{
			"vpn_account":  "new@example.com",
			"device_usage": "生产应急",
			"request_kind": "production_emergency",
		},
	})
	if err != nil {
		t.Fatalf("submit draft: %v", err)
	}
	if result.OK || result.FailureReason != "no owner" {
		t.Fatalf("expected precheck failure result, got %+v", result)
	}
	if op.createdServiceID != 0 {
		t.Fatalf("ticket should not be created when precheck fails, got service %d", op.createdServiceID)
	}
	if state := store.states[1]; state.Stage != "awaiting_confirmation" {
		t.Fatalf("state should stay awaiting confirmation after precheck failure, got %+v", state)
	}
}

func TestParseFormFields_PreservesFieldContextAndOptionValues(t *testing.T) {
	fields := parseFormFields(`{
		"version": 1,
		"fields": [
			{
				"key": "request_kind",
				"type": "select",
				"label": "访问原因",
				"description": "选择 VPN 访问原因",
				"placeholder": "例如：线上支持",
				"required": true,
				"options": [
					{"label": "线上支持", "value": "network_support"},
					{"label": "安全审计", "value": "security"}
				]
			}
		]
	}`)
	if len(fields) != 1 {
		t.Fatalf("expected one field, got %+v", fields)
	}
	field := fields[0]
	if field.Description != "选择 VPN 访问原因" || field.Placeholder != "例如：线上支持" {
		t.Fatalf("expected field context to be preserved, got %+v", field)
	}
	if len(field.Options) != 2 || field.Options[0].Label != "线上支持" || field.Options[0].Value != "network_support" {
		t.Fatalf("expected option label/value to be preserved, got %+v", field.Options)
	}
}

func TestExtractRoutingHint_ReadsEdgeConditionsAndNormalizesFormField(t *testing.T) {
	hint := extractRoutingHint(`{
		"nodes": [
			{"id": "route", "type": "exclusive", "data": {"label": "访问原因路由"}},
			{"id": "network", "type": "process", "data": {"label": "网络管理员处理"}},
			{"id": "security", "type": "process", "data": {"label": "信息安全管理员处理"}}
		],
		"edges": [
			{"id": "e1", "source": "route", "target": "network", "data": {"condition": {"field": "form.request_kind", "operator": "contains_any", "value": ["online_support", "troubleshooting"]}}},
			{"id": "e2", "source": "route", "target": "security", "data": {"condition": {"field": "form.request_kind", "operator": "contains_any", "value": ["external_collaboration"]}}}
		]
	}`)
	if hint == nil {
		t.Fatal("expected routing hint")
	}
	if hint.FieldKey != "request_kind" {
		t.Fatalf("expected normalized field request_kind, got %q", hint.FieldKey)
	}
	if hint.OptionRouteMap["online_support"] != "网络管理员处理" ||
		hint.OptionRouteMap["external_collaboration"] != "信息安全管理员处理" {
		t.Fatalf("unexpected route map: %+v", hint.OptionRouteMap)
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
		ServiceLocked        bool           `json:"service_locked"`
		NextRequiredTool     string         `json:"next_required_tool"`
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
	if !resp.ServiceLocked || resp.NextRequiredTool != "itsm.service_load" {
		t.Fatalf("expected locked service and service_load next, got %+v", resp)
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
		ServiceLocked         bool           `json:"service_locked"`
		NextRequiredTool      string         `json:"next_required_tool"`
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
	if resp.ServiceLocked || resp.NextRequiredTool != "itsm.service_confirm" {
		t.Fatalf("expected service_confirm next step, got %+v", resp)
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
			"访问原因":    "online_support",
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
		draftResp.FormData["request_kind"] != "online_support" {
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
	if op.createdFormData["request_kind"] != "online_support" {
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
				{Key: "request_kind", Label: "访问原因", Type: "select", Required: true, Options: []FormOption{{Label: "network_support", Value: "network_support"}, {Label: "security", Value: "security"}, {Label: "remote_maintenance", Value: "remote_maintenance"}}},
			},
			RoutingFieldHint: &RoutingFieldHint{
				FieldKey: "request_kind",
				OptionRouteMap: map[string]string{
					"network_support":    "网络管理处理",
					"security":           "安全管理处理",
					"remote_maintenance": "网络管理处理",
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
			if w.ResolvedValues[0].Value != "network_support" || w.ResolvedValues[0].Route != "网络管理处理" {
				t.Errorf("resolved_values[0] = %+v, want {network_support, 网络管理处理}", w.ResolvedValues[0])
			}
			if w.ResolvedValues[1].Value != "security" || w.ResolvedValues[1].Route != "安全管理处理" {
				t.Errorf("resolved_values[1] = %+v, want {security, 安全管理处理}", w.ResolvedValues[1])
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
				{Key: "request_kind", Label: "访问原因", Type: "select", Required: true, Options: []FormOption{{Label: "network_support", Value: "network_support"}}},
				{Key: "vpn_type", Label: "VPN类型", Type: "select", Required: true, Options: []FormOption{{Label: "l2tp", Value: "l2tp"}, {Label: "ipsec", Value: "ipsec"}}},
			},
			RoutingFieldHint: &RoutingFieldHint{
				FieldKey:       "request_kind",
				OptionRouteMap: map[string]string{"network_support": "网络管理处理"},
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
