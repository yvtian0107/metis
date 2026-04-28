package ticket

import (
	"encoding/json"
	"testing"

	. "metis/internal/app/itsm/domain"
	"metis/internal/model"
)

func TestBuildDecisionExplanation_Fallbacks(t *testing.T) {
	resp := &TicketResponse{
		EngineType:        "smart",
		StatusLabel:       "AI 决策中",
		DecisioningReason: "",
		NextStepSummary:   "",
	}
	explanation := buildDecisionExplanation(resp, nil, nil)
	if explanation == nil {
		t.Fatal("expected explanation")
	}
	if explanation.Trigger == "" || explanation.Decision == "" || explanation.NextStep == "" {
		t.Fatalf("expected fallback fields, got %+v", explanation)
	}
}

func TestBuildDecisionExplanation_UsesActivityReasoning(t *testing.T) {
	activityID := uint(9)
	activity := &TicketActivity{
		BaseModel:         model.BaseModel{ID: activityID},
		AIReasoning:       "来自协作规范",
		DecisionReasoning: "转到网络管理员处理",
	}
	resp := &TicketResponse{EngineType: "smart", StatusLabel: "已同意，决策中", DecisioningReason: "activity_approved", NextStepSummary: "网络管理员处理"}
	explanation := buildDecisionExplanation(resp, activity, nil)
	if explanation.Basis != "来自协作规范" || explanation.Decision != "转到网络管理员处理" {
		t.Fatalf("expected activity reasoning to override defaults, got %+v", explanation)
	}
	if explanation.ActivityID == nil || *explanation.ActivityID != activityID {
		t.Fatalf("expected activity id set, got %+v", explanation.ActivityID)
	}
}

func TestBuildDecisionExplanation_PrefersSnapshot(t *testing.T) {
	activityID := uint(17)
	resp := &TicketResponse{
		EngineType:        "smart",
		StatusLabel:       "AI 决策中",
		DecisioningReason: "ai_decision",
		NextStepSummary:   "默认下一步",
	}
	activity := &TicketActivity{
		BaseModel:         model.BaseModel{ID: 21},
		AIReasoning:       "activity basis",
		DecisionReasoning: "activity decision",
	}
	snapshot := &DecisionExplanation{
		ActivityID:    &activityID,
		Basis:         "snapshot basis",
		Trigger:       "ai_decision_executed",
		Decision:      "snapshot decision",
		NextStep:      "snapshot next",
		HumanOverride: "snapshot override",
	}
	explanation := buildDecisionExplanation(resp, activity, snapshot)
	if explanation.ActivityID == nil || *explanation.ActivityID != activityID {
		t.Fatalf("expected snapshot activity id, got %+v", explanation.ActivityID)
	}
	if explanation.Basis != "snapshot basis" || explanation.Decision != "snapshot decision" || explanation.NextStep != "snapshot next" {
		t.Fatalf("expected snapshot fields to win, got %+v", explanation)
	}
	if explanation.HumanOverride != "snapshot override" {
		t.Fatalf("expected snapshot humanOverride, got %q", explanation.HumanOverride)
	}
}

func TestParseDecisionExplanationDetail(t *testing.T) {
	payload := map[string]any{
		"decision_explanation": map[string]any{
			"trigger":  "ai_decision_failed",
			"decision": "AI 决策失败",
			"nextStep": "等待人工介入",
		},
	}
	raw, _ := json.Marshal(payload)
	snapshot := parseDecisionExplanationDetail(JSONField(raw))
	if snapshot == nil {
		t.Fatal("expected snapshot parsed")
	}
	if snapshot.Trigger != "ai_decision_failed" || snapshot.Decision != "AI 决策失败" {
		t.Fatalf("unexpected snapshot: %+v", snapshot)
	}
}

func TestBuildRecoveryActions_ByStatus(t *testing.T) {
	resp := &TicketResponse{EngineType: "smart", Status: TicketStatusFailed, AIFailureCount: 2}
	actions := buildRecoveryActions(resp)
	if len(actions) != 2 {
		t.Fatalf("expected retry/handoff actions, got %+v", actions)
	}

	resp = &TicketResponse{EngineType: "smart", Status: TicketStatusCompleted, AIFailureCount: 0}
	actions = buildRecoveryActions(resp)
	if len(actions) != 0 {
		t.Fatalf("expected no actions for terminal completed ticket, got %+v", actions)
	}
}
