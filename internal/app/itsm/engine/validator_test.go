package engine

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestValidateWorkflowAllowsRequesterParticipantOnForm(t *testing.T) {
	workflowJSON := json.RawMessage(`{
		"nodes": [
			{"id":"start","type":"start","data":{"label":"开始"}},
			{"id":"form","type":"form","data":{"label":"填写申请","participants":[{"type":"requester"}]}},
			{"id":"end","type":"end","data":{"label":"结束"}}
		],
		"edges": [
			{"id":"e1","source":"start","target":"form","data":{}},
			{"id":"e2","source":"form","target":"end","data":{}}
		]
	}`)

	if errs := ValidateWorkflow(workflowJSON); len(errs) > 0 {
		t.Fatalf("expected requester participant to validate, got %+v", errs)
	}
}

func TestValidateWorkflowMissingFormParticipantSuggestsRequester(t *testing.T) {
	workflowJSON := json.RawMessage(`{
		"nodes": [
			{"id":"start","type":"start","data":{"label":"开始"}},
			{"id":"form","type":"form","data":{"label":"填写临时访问申请"}},
			{"id":"end","type":"end","data":{"label":"结束"}}
		],
		"edges": [
			{"id":"e1","source":"start","target":"form","data":{}},
			{"id":"e2","source":"form","target":"end","data":{}}
		]
	}`)

	errs := ValidateWorkflow(workflowJSON)
	if len(errs) == 0 {
		t.Fatal("expected validation errors")
	}
	got := errs[0].Message
	if !strings.Contains(got, `{"type":"requester"}`) {
		t.Fatalf("expected requester repair suggestion, got %q", got)
	}
	if !strings.Contains(got, "form（填写临时访问申请）") {
		t.Fatalf("expected node label in validation message, got %q", got)
	}
}

func TestValidateWorkflowAllowsProcessOutcomesToShareEndNode(t *testing.T) {
	workflowJSON := json.RawMessage(`{
		"nodes": [
			{"id":"start","type":"start","data":{"label":"开始"}},
			{"id":"process","type":"process","data":{"label":"处理","participants":[{"type":"requester"}]}},
			{"id":"end","type":"end","data":{"label":"完成"}}
		],
		"edges": [
			{"id":"e1","source":"start","target":"process","data":{}},
			{"id":"e2","source":"process","target":"end","data":{"outcome":"approved"}},
			{"id":"e3","source":"process","target":"end","data":{"outcome":"rejected"}}
		]
	}`)

	var blocking []ValidationError
	for _, err := range ValidateWorkflow(workflowJSON) {
		if !err.IsWarning() {
			blocking = append(blocking, err)
		}
	}
	if len(blocking) > 0 {
		t.Fatalf("expected shared end node to validate, got %+v", blocking)
	}
}

func TestValidateWorkflowRejectsProcessOutcomesSharingNonEndNode(t *testing.T) {
	workflowJSON := json.RawMessage(`{
		"nodes": [
			{"id":"start","type":"start","data":{"label":"开始"}},
			{"id":"process","type":"process","data":{"label":"处理","participants":[{"type":"requester"}]}},
			{"id":"next","type":"process","data":{"label":"继续处理","participants":[{"type":"requester"}]}},
			{"id":"end","type":"end","data":{"label":"完成"}}
		],
		"edges": [
			{"id":"e1","source":"start","target":"process","data":{}},
			{"id":"e2","source":"process","target":"next","data":{"outcome":"approved"}},
			{"id":"e3","source":"process","target":"next","data":{"outcome":"rejected"}},
			{"id":"e4","source":"next","target":"end","data":{"outcome":"approved"}},
			{"id":"e5","source":"next","target":"end","data":{"outcome":"rejected"}}
		]
	}`)

	var found bool
	for _, err := range ValidateWorkflow(workflowJSON) {
		if !err.IsWarning() && strings.Contains(err.Message, "共同指向非结束节点") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected shared non-end target to be rejected")
	}
}

func TestValidateWorkflowClassicNodeMatrix(t *testing.T) {
	workflowJSON := json.RawMessage(`{
		"nodes": [
			{"id":"start","type":"start","data":{"label":"开始"}},
			{"id":"action","type":"action","data":{"label":"动作","action_id":1}},
			{"id":"notify","type":"notify","data":{"label":"通知","channel_id":2}},
			{"id":"wait","type":"wait","data":{"label":"等待","wait_mode":"timer","duration":"1h","participants":[{"type":"requester"}]}},
			{"id":"script","type":"script","data":{"label":"脚本","assignments":[{"variable":"x","expression":"1 + 1"}]}},
			{"id":"sub","type":"subprocess","data":{"label":"子流程","subprocess_def":{"nodes":[{"id":"sub_start","type":"start","data":{"label":"子开始"}},{"id":"sub_end","type":"end","data":{"label":"子结束"}}],"edges":[{"id":"se1","source":"sub_start","target":"sub_end","data":{}}]}}},
			{"id":"end","type":"end","data":{"label":"结束"}}
		],
		"edges": [
			{"id":"e1","source":"start","target":"action","data":{}},
			{"id":"e2","source":"action","target":"notify","data":{"outcome":"success"}},
			{"id":"e3","source":"notify","target":"wait","data":{}},
			{"id":"e4","source":"wait","target":"script","data":{"default":true}},
			{"id":"e5","source":"script","target":"sub","data":{}},
			{"id":"e6","source":"sub","target":"end","data":{}}
		]
	}`)

	var blocking []ValidationError
	for _, err := range ValidateWorkflow(workflowJSON) {
		if !err.IsWarning() {
			blocking = append(blocking, err)
		}
	}
	if len(blocking) > 0 {
		t.Fatalf("expected workflow matrix to validate, got %+v", blocking)
	}
}

func TestValidateWorkflowRejectsNonRunnableClassicNodeConfig(t *testing.T) {
	tests := []struct {
		name    string
		node    string
		edge    string
		wantMsg string
	}{
		{
			name:    "action missing action_id",
			node:    `{"id":"node","type":"action","data":{"label":"动作"}}`,
			edge:    `{"id":"e2","source":"node","target":"end","data":{}}`,
			wantMsg: "action_id",
		},
		{
			name:    "wait missing wait_mode",
			node:    `{"id":"node","type":"wait","data":{"label":"等待","participants":[{"type":"requester"}]}}`,
			edge:    `{"id":"e2","source":"node","target":"end","data":{}}`,
			wantMsg: "wait_mode",
		},
		{
			name:    "script missing assignments",
			node:    `{"id":"node","type":"script","data":{"label":"脚本"}}`,
			edge:    `{"id":"e2","source":"node","target":"end","data":{}}`,
			wantMsg: "assignments",
		},
		{
			name:    "subprocess missing subprocess_def",
			node:    `{"id":"node","type":"subprocess","data":{"label":"子流程"}}`,
			edge:    `{"id":"e2","source":"node","target":"end","data":{}}`,
			wantMsg: "subprocess_def",
		},
		{
			name:    "timer event remains non runnable",
			node:    `{"id":"node","type":"timer","data":{"label":"定时事件"}}`,
			edge:    `{"id":"e2","source":"node","target":"end","data":{}}`,
			wantMsg: "尚未实现",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			workflowJSON := json.RawMessage(`{
				"nodes": [
					{"id":"start","type":"start","data":{"label":"开始"}},
					` + tt.node + `,
					{"id":"end","type":"end","data":{"label":"结束"}}
				],
				"edges": [
					{"id":"e1","source":"start","target":"node","data":{}},
					` + tt.edge + `
				]
			}`)

			errs := ValidateWorkflow(workflowJSON)
			if len(errs) == 0 {
				t.Fatal("expected validation error")
			}
			var found bool
			for _, err := range errs {
				if !err.IsWarning() && strings.Contains(err.Message, tt.wantMsg) {
					found = true
					break
				}
			}
			if !found {
				t.Fatalf("expected blocking message containing %q, got %+v", tt.wantMsg, errs)
			}
		})
	}
}

func TestValidateFormSchemaReferences(t *testing.T) {
	// Workflow: start -> form (request_kind, urgency) -> exclusive gateway -> two branches
	makeWorkflow := func(condField string) json.RawMessage {
		return json.RawMessage(`{
			"nodes": [
				{"id":"start","type":"start","data":{"label":"开始"}},
				{"id":"form1","type":"form","data":{"label":"申请表","participants":[{"type":"requester"}],"formSchema":{"fields":[{"key":"request_kind","type":"select","label":"类型"},{"key":"urgency","type":"select","label":"紧急程度"}]}}},
				{"id":"gw","type":"exclusive","data":{"label":"分支"}},
				{"id":"p1","type":"process","data":{"label":"处理A","participants":[{"type":"requester"}]}},
				{"id":"p2","type":"process","data":{"label":"处理B","participants":[{"type":"requester"}]}},
				{"id":"end","type":"end","data":{"label":"结束"}}
			],
			"edges": [
				{"id":"e1","source":"start","target":"form1","data":{}},
				{"id":"e2","source":"form1","target":"gw","data":{"outcome":"submitted"}},
				{"id":"e3","source":"gw","target":"p1","data":{"condition":{"field":"` + condField + `","operator":"equals","value":"high"}}},
				{"id":"e4","source":"gw","target":"p2","data":{"default":true}},
				{"id":"e5","source":"p1","target":"end","data":{"outcome":"approved"}},
				{"id":"e5r","source":"p1","target":"end","data":{"outcome":"rejected"}},
				{"id":"e6","source":"p2","target":"end","data":{"outcome":"approved"}},
				{"id":"e6r","source":"p2","target":"end","data":{"outcome":"rejected"}}
			]
		}`)
	}

	t.Run("field exists in formSchema", func(t *testing.T) {
		errs := ValidateWorkflow(makeWorkflow("form.urgency"))
		for _, e := range errs {
			if e.IsWarning() && strings.Contains(e.Message, "formSchema") {
				t.Fatalf("unexpected formSchema warning: %s", e.Message)
			}
		}
	})

	t.Run("field missing from formSchema", func(t *testing.T) {
		errs := ValidateWorkflow(makeWorkflow("form.nonexistent"))
		var found bool
		for _, e := range errs {
			if e.IsWarning() && strings.Contains(e.Message, "nonexistent") {
				found = true
				break
			}
		}
		if !found {
			t.Fatal("expected warning about missing formSchema field")
		}
	})

	t.Run("non-form field skipped", func(t *testing.T) {
		errs := ValidateWorkflow(makeWorkflow("ticket.status"))
		for _, e := range errs {
			if e.IsWarning() && strings.Contains(e.Message, "formSchema") {
				t.Fatalf("unexpected formSchema warning for non-form field: %s", e.Message)
			}
		}
	})

	t.Run("no upstream form node skipped", func(t *testing.T) {
		wf := json.RawMessage(`{
			"nodes": [
				{"id":"start","type":"start","data":{"label":"开始"}},
				{"id":"gw","type":"exclusive","data":{"label":"分支"}},
				{"id":"p1","type":"process","data":{"label":"A","participants":[{"type":"requester"}]}},
				{"id":"p2","type":"process","data":{"label":"B","participants":[{"type":"requester"}]}},
				{"id":"end","type":"end","data":{"label":"结束"}}
			],
			"edges": [
				{"id":"e1","source":"start","target":"gw","data":{}},
				{"id":"e2","source":"gw","target":"p1","data":{"condition":{"field":"form.request_kind","operator":"equals","value":"vpn"}}},
				{"id":"e3","source":"gw","target":"p2","data":{"default":true}},
				{"id":"e4","source":"p1","target":"end","data":{"outcome":"approved"}},
				{"id":"e4r","source":"p1","target":"end","data":{"outcome":"rejected"}},
				{"id":"e5","source":"p2","target":"end","data":{"outcome":"approved"}},
				{"id":"e5r","source":"p2","target":"end","data":{"outcome":"rejected"}}
			]
		}`)
		errs := ValidateWorkflow(wf)
		for _, e := range errs {
			if e.IsWarning() && strings.Contains(e.Message, "formSchema") {
				t.Fatalf("unexpected formSchema warning when no upstream form: %s", e.Message)
			}
		}
	})
}

// ---------------------------------------------------------------------------
// Topology validation tests (task 3.7)
// ---------------------------------------------------------------------------

func TestValidateWorkflowNoCycle(t *testing.T) {
	workflowJSON := json.RawMessage(`{
		"nodes": [
			{"id":"start","type":"start","data":{"label":"开始"}},
			{"id":"form1","type":"form","data":{"label":"表单","participants":[{"type":"requester"}]}},
			{"id":"end","type":"end","data":{"label":"结束"}}
		],
		"edges": [
			{"id":"e1","source":"start","target":"form1","data":{}},
			{"id":"e2","source":"form1","target":"end","data":{}}
		]
	}`)

	errs := ValidateWorkflow(workflowJSON)
	for _, e := range errs {
		if strings.Contains(e.Message, "环路") {
			t.Fatalf("expected no cycle error in linear workflow, got: %s", e.Message)
		}
	}
}

func TestValidateWorkflowDirectCycle(t *testing.T) {
	// A→B→A direct cycle. Both nodes are form nodes so they are valid types.
	workflowJSON := json.RawMessage(`{
		"nodes": [
			{"id":"start","type":"start","data":{"label":"开始"}},
			{"id":"A","type":"form","data":{"label":"节点A","participants":[{"type":"requester"}]}},
			{"id":"B","type":"form","data":{"label":"节点B","participants":[{"type":"requester"}]}},
			{"id":"end","type":"end","data":{"label":"结束"}}
		],
		"edges": [
			{"id":"e1","source":"start","target":"A","data":{}},
			{"id":"e2","source":"A","target":"B","data":{}},
			{"id":"e3","source":"B","target":"A","data":{}},
			{"id":"e4","source":"B","target":"end","data":{}}
		]
	}`)

	errs := ValidateWorkflow(workflowJSON)
	var found bool
	for _, e := range errs {
		if strings.Contains(e.Message, "环路") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected cycle detection error containing '环路', got errors: %+v", errs)
	}
}

func TestValidateWorkflowIndirectCycle(t *testing.T) {
	// A→B→C→A indirect cycle.
	workflowJSON := json.RawMessage(`{
		"nodes": [
			{"id":"start","type":"start","data":{"label":"开始"}},
			{"id":"A","type":"form","data":{"label":"节点A","participants":[{"type":"requester"}]}},
			{"id":"B","type":"form","data":{"label":"节点B","participants":[{"type":"requester"}]}},
			{"id":"C","type":"form","data":{"label":"节点C","participants":[{"type":"requester"}]}},
			{"id":"end","type":"end","data":{"label":"结束"}}
		],
		"edges": [
			{"id":"e1","source":"start","target":"A","data":{}},
			{"id":"e2","source":"A","target":"B","data":{}},
			{"id":"e3","source":"B","target":"C","data":{}},
			{"id":"e4","source":"C","target":"A","data":{}},
			{"id":"e5","source":"C","target":"end","data":{}}
		]
	}`)

	errs := ValidateWorkflow(workflowJSON)
	var found bool
	for _, e := range errs {
		if strings.Contains(e.Message, "环路") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected cycle detection error containing '环路', got errors: %+v", errs)
	}
}

func TestValidateWorkflowDeadEnd(t *testing.T) {
	// form1 connects to form2, but form2 has no edge to end — form2 is a dead-end.
	workflowJSON := json.RawMessage(`{
		"nodes": [
			{"id":"start","type":"start","data":{"label":"开始"}},
			{"id":"form1","type":"form","data":{"label":"表单1","participants":[{"type":"requester"}]}},
			{"id":"form2","type":"form","data":{"label":"表单2","participants":[{"type":"requester"}]}},
			{"id":"form3","type":"form","data":{"label":"表单3","participants":[{"type":"requester"}]}},
			{"id":"end","type":"end","data":{"label":"结束"}}
		],
		"edges": [
			{"id":"e1","source":"start","target":"form1","data":{}},
			{"id":"e2","source":"form1","target":"form2","data":{}},
			{"id":"e3","source":"form1","target":"form3","data":{}},
			{"id":"e4","source":"form3","target":"end","data":{}}
		]
	}`)

	errs := ValidateWorkflow(workflowJSON)
	var found bool
	for _, e := range errs {
		if strings.Contains(e.Message, "无法到达终点") && e.NodeID == "form2" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected dead-end error for form2 containing '无法到达终点', got errors: %+v", errs)
	}
}

func TestValidateWorkflowAllowsMultipleTerminalBranches(t *testing.T) {
	workflowJSON := json.RawMessage(`{
		"nodes": [
			{"id":"start","type":"start","data":{"label":"开始"}},
			{"id":"gw","type":"exclusive","data":{"label":"分支"}},
			{"id":"form_ok","type":"form","data":{"label":"正常补充","participants":[{"type":"requester"}]}},
			{"id":"form_reject","type":"form","data":{"label":"驳回确认","participants":[{"type":"requester"}]}},
			{"id":"end_ok","type":"end","data":{"label":"正常结束"}},
			{"id":"end_reject","type":"end","data":{"label":"驳回结束"}}
		],
		"edges": [
			{"id":"e1","source":"start","target":"gw","data":{}},
			{"id":"e2","source":"gw","target":"form_ok","data":{"condition":{"field":"ticket.status","operator":"equals","value":"approved"}}},
			{"id":"e3","source":"gw","target":"form_reject","data":{"default":true}},
			{"id":"e4","source":"form_ok","target":"end_ok","data":{}},
			{"id":"e5","source":"form_reject","target":"end_reject","data":{}}
		]
	}`)

	errs := ValidateWorkflow(workflowJSON)
	for _, e := range errs {
		if strings.Contains(e.Message, "无法到达终点") {
			t.Fatalf("expected all branches to reach one terminal node, got dead-end error: %+v", errs)
		}
	}
}

func TestValidateWorkflowDoesNotRequireEndToReachAnotherEnd(t *testing.T) {
	workflowJSON := json.RawMessage(`{
		"nodes": [
			{"id":"start","type":"start","data":{"label":"开始"}},
			{"id":"gw","type":"exclusive","data":{"label":"分支"}},
			{"id":"end_ok","type":"end","data":{"label":"正常结束"}},
			{"id":"end_reject","type":"end","data":{"label":"驳回结束"}}
		],
		"edges": [
			{"id":"e1","source":"start","target":"gw","data":{}},
			{"id":"e2","source":"gw","target":"end_ok","data":{"condition":{"field":"ticket.status","operator":"equals","value":"approved"}}},
			{"id":"e3","source":"gw","target":"end_reject","data":{"default":true}}
		]
	}`)

	errs := ValidateWorkflow(workflowJSON)
	for _, e := range errs {
		if e.NodeID == "end_reject" && strings.Contains(e.Message, "无法到达终点") {
			t.Fatalf("end node must not be required to reach another end node, got %+v", errs)
		}
	}
}

func TestValidateWorkflowInvalidParticipantType(t *testing.T) {
	workflowJSON := json.RawMessage(`{
		"nodes": [
			{"id":"start","type":"start","data":{"label":"开始"}},
			{"id":"form1","type":"form","data":{"label":"表单","participants":[{"type":"invalid_type"}]}},
			{"id":"end","type":"end","data":{"label":"结束"}}
		],
		"edges": [
			{"id":"e1","source":"start","target":"form1","data":{}},
			{"id":"e2","source":"form1","target":"end","data":{}}
		]
	}`)

	errs := ValidateWorkflow(workflowJSON)
	var found bool
	for _, e := range errs {
		if strings.Contains(e.Message, "非法的参与者类型") && !e.IsWarning() {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected blocking error containing '非法的参与者类型', got errors: %+v", errs)
	}
}

func TestValidateWorkflowValidParticipantTypes(t *testing.T) {
	validTypes := []string{
		"user", "position", "department",
		"position_department", "requester", "requester_manager",
	}
	for _, pt := range validTypes {
		t.Run(pt, func(t *testing.T) {
			var participantJSON string
			switch pt {
			case "user":
				participantJSON = `{"type":"user","value":"admin"}`
			case "position":
				participantJSON = `{"type":"position","value":"manager"}`
			case "department":
				participantJSON = `{"type":"department","value":"it"}`
			case "position_department":
				participantJSON = `{"type":"position_department","position_code":"admin","department_code":"it"}`
			case "requester":
				participantJSON = `{"type":"requester"}`
			case "requester_manager":
				participantJSON = `{"type":"requester_manager"}`
			}

			workflowJSON := json.RawMessage(`{
				"nodes": [
					{"id":"start","type":"start","data":{"label":"开始"}},
					{"id":"form1","type":"form","data":{"label":"表单","participants":[` + participantJSON + `]}},
					{"id":"end","type":"end","data":{"label":"结束"}}
				],
				"edges": [
					{"id":"e1","source":"start","target":"form1","data":{}},
					{"id":"e2","source":"form1","target":"end","data":{}}
				]
			}`)

			errs := ValidateWorkflow(workflowJSON)
			for _, e := range errs {
				if strings.Contains(e.Message, "非法的参与者类型") {
					t.Fatalf("participant type %q should be valid, got error: %s", pt, e.Message)
				}
			}
		})
	}
}

func TestValidateWorkflowBlockingVsWarning(t *testing.T) {
	// Workflow with both a topology issue (dead-end) and a formSchema reference issue.
	// form2 is a dead-end (topology → blocking), and the gateway condition references
	// a nonexistent form field (formSchema → warning).
	workflowJSON := json.RawMessage(`{
		"nodes": [
			{"id":"start","type":"start","data":{"label":"开始"}},
			{"id":"form1","type":"form","data":{"label":"申请表","participants":[{"type":"requester"}],"formSchema":{"fields":[{"key":"urgency","type":"select","label":"紧急程度"}]}}},
			{"id":"gw","type":"exclusive","data":{"label":"分支"}},
			{"id":"p1","type":"process","data":{"label":"处理A","participants":[{"type":"requester"}]}},
			{"id":"p2","type":"process","data":{"label":"处理B","participants":[{"type":"requester"}]}},
			{"id":"form2","type":"form","data":{"label":"死胡同","participants":[{"type":"requester"}]}},
			{"id":"end","type":"end","data":{"label":"结束"}}
		],
		"edges": [
			{"id":"e1","source":"start","target":"form1","data":{}},
			{"id":"e2","source":"form1","target":"gw","data":{"outcome":"submitted"}},
			{"id":"e3","source":"gw","target":"p1","data":{"condition":{"field":"form.nonexistent_field","operator":"equals","value":"high"}}},
			{"id":"e4","source":"gw","target":"p2","data":{"default":true}},
			{"id":"e5","source":"p1","target":"end","data":{"outcome":"approved"}},
			{"id":"e5r","source":"p1","target":"end","data":{"outcome":"rejected"}},
			{"id":"e6","source":"p2","target":"end","data":{"outcome":"approved"}},
			{"id":"e6r","source":"p2","target":"end","data":{"outcome":"rejected"}},
			{"id":"e7","source":"gw","target":"form2","data":{"condition":{"field":"form.urgency","operator":"equals","value":"low"}}}
		]
	}`)

	errs := ValidateWorkflow(workflowJSON)

	var foundTopologyBlocking, foundFormSchemaWarning bool
	for _, e := range errs {
		if strings.Contains(e.Message, "无法到达终点") {
			if e.Level != "blocking" {
				t.Fatalf("expected dead-end error to be blocking, got level=%q: %s", e.Level, e.Message)
			}
			foundTopologyBlocking = true
		}
		if strings.Contains(e.Message, "formSchema") && strings.Contains(e.Message, "nonexistent_field") {
			if e.Level != "warning" {
				t.Fatalf("expected formSchema reference error to be warning, got level=%q: %s", e.Level, e.Message)
			}
			foundFormSchemaWarning = true
		}
	}

	if !foundTopologyBlocking {
		t.Fatalf("expected a blocking topology error (dead-end), got errors: %+v", errs)
	}
	if !foundFormSchemaWarning {
		t.Fatalf("expected a warning-level formSchema reference error, got errors: %+v", errs)
	}
}
