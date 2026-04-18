package engine

import (
	"strings"
	"testing"
)

func TestExtractWorkflowHints_Empty(t *testing.T) {
	if got := extractWorkflowHints(""); got != "" {
		t.Errorf("expected empty for empty input, got %q", got)
	}
}

func TestExtractWorkflowHints_InvalidJSON(t *testing.T) {
	if got := extractWorkflowHints("{invalid}"); got != "" {
		t.Errorf("expected empty for invalid JSON, got %q", got)
	}
}

func TestExtractWorkflowHints_SerialFlow(t *testing.T) {
	wf := `{
		"nodes": [
			{"id": "start", "type": "start", "data": {"label": "开始"}},
			{"id": "n1", "type": "process", "data": {"label": "网络管理员处理", "participants": [{"type": "position_department", "position_code": "net_admin", "department_code": "it"}]}},
			{"id": "n2", "type": "approve", "data": {"label": "主管审批", "participants": [{"type": "requester_manager"}]}},
			{"id": "end", "type": "end", "data": {"label": "结束"}}
		],
		"edges": [
			{"id": "e1", "source": "start", "target": "n1", "data": {}},
			{"id": "e2", "source": "n1", "target": "n2", "data": {}},
			{"id": "e3", "source": "n2", "target": "end", "data": {}}
		]
	}`

	got := extractWorkflowHints(wf)
	if got == "" {
		t.Fatal("expected non-empty hints for serial flow")
	}

	// Should contain process and approve steps
	if !strings.Contains(got, "处理") {
		t.Errorf("expected hints to contain '处理', got %q", got)
	}
	if !strings.Contains(got, "审批") {
		t.Errorf("expected hints to contain '审批', got %q", got)
	}
	if !strings.Contains(got, "net_admin") {
		t.Errorf("expected hints to contain participant info, got %q", got)
	}
	if !strings.Contains(got, "申请人主管") {
		t.Errorf("expected hints to contain '申请人主管', got %q", got)
	}
}

func TestExtractWorkflowHints_GatewayBranch(t *testing.T) {
	wf := `{
		"nodes": [
			{"id": "start", "type": "start", "data": {"label": "开始"}},
			{"id": "gw", "type": "exclusive", "data": {"label": "判断类型"}},
			{"id": "n1", "type": "process", "data": {"label": "网络处理"}},
			{"id": "n2", "type": "process", "data": {"label": "安全处理"}},
			{"id": "end", "type": "end", "data": {"label": "结束"}}
		],
		"edges": [
			{"id": "e1", "source": "start", "target": "gw", "data": {}},
			{"id": "e2", "source": "gw", "target": "n1", "data": {"condition": {"field": "form.type", "operator": "equals", "value": "network"}}},
			{"id": "e3", "source": "gw", "target": "n2", "data": {"default": true}},
			{"id": "e4", "source": "n1", "target": "end", "data": {}},
			{"id": "e5", "source": "n2", "target": "end", "data": {}}
		]
	}`

	got := extractWorkflowHints(wf)
	if got == "" {
		t.Fatal("expected non-empty hints for gateway flow")
	}

	if !strings.Contains(got, "条件分支") {
		t.Errorf("expected hints to contain '条件分支', got %q", got)
	}
	if !strings.Contains(got, "form.type") {
		t.Errorf("expected hints to contain condition field, got %q", got)
	}
	if !strings.Contains(got, "默认") {
		t.Errorf("expected hints to contain '默认' for default branch, got %q", got)
	}
}

func TestExtractWorkflowHints_DegradesToEmpty(t *testing.T) {
	// No start node
	wf := `{
		"nodes": [
			{"id": "n1", "type": "process", "data": {"label": "处理"}},
			{"id": "end", "type": "end", "data": {"label": "结束"}}
		],
		"edges": [
			{"id": "e1", "source": "n1", "target": "end", "data": {}}
		]
	}`

	got := extractWorkflowHints(wf)
	if got != "" {
		t.Errorf("expected empty for workflow without start node, got %q", got)
	}
}
