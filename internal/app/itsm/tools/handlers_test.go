package tools

import (
	"context"
	"encoding/json"
	"testing"
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
	detail *ServiceDetail
}

func (s *stubOperator) MatchServices(query string) ([]ServiceMatch, error) { return nil, nil }
func (s *stubOperator) LoadService(serviceID uint) (*ServiceDetail, error) {
	return s.detail, nil
}
func (s *stubOperator) CreateTicket(userID uint, serviceID uint, summary string, formData map[string]any, sessionID uint) (*TicketResult, error) {
	return nil, nil
}
func (s *stubOperator) ListMyTickets(userID uint, status string) ([]TicketSummary, error) {
	return nil, nil
}
func (s *stubOperator) WithdrawTicket(userID uint, ticketCode string, reason string) error {
	return nil
}
func (s *stubOperator) ValidateParticipants(serviceID uint, formData map[string]any) (*ParticipantValidation, error) {
	return &ParticipantValidation{OK: true}, nil
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

	ctx := context.WithValue(context.Background(), SessionIDKey, uint(1))

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

	ctx := context.WithValue(context.Background(), SessionIDKey, uint(1))

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
