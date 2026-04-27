package runtime

import (
	"encoding/json"
	"testing"
)

func TestMakeITSMDraftReadySurface_OnlyForSmartDraft(t *testing.T) {
	output := `{
		"ok": true,
		"ready_for_confirmation": true,
		"service_id": 5,
		"service_name": "VPN 开通申请",
		"service_engine_type": "smart",
		"draft_version": 2,
		"summary": "申请 VPN",
		"form_data": {"vpn_account": "wenhaowu@dev.com"},
		"form_schema": {"version": 1, "fields": [{"key": "vpn_account", "type": "text", "label": "VPN账号"}]}
	}`

	event, ok := makeITSMDraftReadySurface("call_1", output)
	if !ok {
		t.Fatal("expected smart draft_prepare output to produce UI surface")
	}
	if event.Type != EventTypeUISurface || event.SurfaceType != "itsm.draft_form" || event.SurfaceID == "" {
		t.Fatalf("unexpected surface event: %+v", event)
	}

	var payload struct {
		Status       string         `json:"status"`
		ServiceID    uint           `json:"serviceId"`
		DraftVersion int            `json:"draftVersion"`
		Values       map[string]any `json:"values"`
	}
	if err := json.Unmarshal(event.SurfaceData, &payload); err != nil {
		t.Fatalf("unmarshal surface data: %v", err)
	}
	if payload.Status != "ready" || payload.ServiceID != 5 || payload.DraftVersion != 2 {
		t.Fatalf("unexpected payload: %+v", payload)
	}
	if payload.Values["vpn_account"] != "wenhaowu@dev.com" {
		t.Fatalf("expected form values in surface payload, got %+v", payload.Values)
	}
}

func TestMakeITSMDraftReadySurface_IgnoresClassicDraft(t *testing.T) {
	output := `{
		"ok": true,
		"ready_for_confirmation": true,
		"service_id": 6,
		"service_name": "经典服务",
		"service_engine_type": "classic",
		"draft_version": 1
	}`
	if _, ok := makeITSMDraftReadySurface("call_1", output); ok {
		t.Fatal("classic draft_prepare output must not produce agentic UI surface")
	}
}
