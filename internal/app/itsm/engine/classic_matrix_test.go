package engine

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

type classicMatrixFixture struct {
	db        *gorm.DB
	engine    *ClassicEngine
	submitter *recordingSubmitter
}

type recordingTask struct {
	name    string
	payload json.RawMessage
}

type recordingSubmitter struct {
	tasks []recordingTask
}

func (s *recordingSubmitter) SubmitTask(name string, payload json.RawMessage) error {
	s.tasks = append(s.tasks, recordingTask{name: name, payload: append(json.RawMessage(nil), payload...)})
	return nil
}

func (s *recordingSubmitter) SubmitTaskTx(_ *gorm.DB, name string, payload json.RawMessage) error {
	return s.SubmitTask(name, payload)
}

func newClassicMatrixFixture(t *testing.T) *classicMatrixFixture {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(
		&ticketModel{},
		&activityModel{},
		&assignmentModel{},
		&timelineModel{},
		&executionTokenModel{},
		&processVariableModel{},
	); err != nil {
		t.Fatalf("migrate db: %v", err)
	}
	if err := db.Exec("ALTER TABLE itsm_tickets ADD COLUMN assignee_id INTEGER").Error; err != nil {
		t.Fatalf("add ticket assignee_id: %v", err)
	}
	if err := db.Exec("CREATE UNIQUE INDEX idx_matrix_process_variables_unique ON itsm_process_variables(ticket_id, scope_id, key)").Error; err != nil {
		t.Fatalf("create process variable unique index: %v", err)
	}

	submitter := &recordingSubmitter{}
	return &classicMatrixFixture{
		db:        db,
		submitter: submitter,
		engine:    NewClassicEngine(NewParticipantResolver(nil), submitter, nil),
	}
}

func (f *classicMatrixFixture) createTicket(t *testing.T, workflow json.RawMessage) ticketModel {
	t.Helper()
	ticket := ticketModel{
		Code:         "TDD-001",
		Title:        "classic tdd",
		Status:       "pending",
		EngineType:   "classic",
		WorkflowJSON: string(workflow),
		RequesterID:  7,
		PriorityID:   4,
	}
	if err := f.db.Create(&ticket).Error; err != nil {
		t.Fatalf("create ticket: %v", err)
	}
	return ticket
}

func (f *classicMatrixFixture) start(t *testing.T, ticket ticketModel, workflow json.RawMessage, opts ...func(*StartParams)) error {
	t.Helper()
	params := StartParams{TicketID: ticket.ID, WorkflowJSON: workflow, RequesterID: ticket.RequesterID}
	for _, opt := range opts {
		opt(&params)
	}
	return f.engine.Start(context.Background(), f.db, params)
}

func (f *classicMatrixFixture) progress(t *testing.T, ticketID uint, activity activityModel, outcome string, result json.RawMessage) error {
	t.Helper()
	return f.engine.Progress(context.Background(), f.db, ProgressParams{
		TicketID:   ticketID,
		ActivityID: activity.ID,
		Outcome:    outcome,
		Result:     result,
		OperatorID: 7,
	})
}

func (f *classicMatrixFixture) firstActivity(t *testing.T, ticketID uint, activityType string) activityModel {
	t.Helper()
	var activity activityModel
	if err := f.db.Where("ticket_id = ? AND activity_type = ? AND status IN ?", ticketID, activityType, []string{ActivityPending, ActivityInProgress}).
		Order("id ASC").First(&activity).Error; err != nil {
		t.Fatalf("find %s activity: %v", activityType, err)
	}
	return activity
}

func (f *classicMatrixFixture) ticketStatus(t *testing.T, ticketID uint) string {
	t.Helper()
	var ticket ticketModel
	if err := f.db.First(&ticket, ticketID).Error; err != nil {
		t.Fatalf("reload ticket: %v", err)
	}
	return ticket.Status
}

func (f *classicMatrixFixture) ticketStatusOutcome(t *testing.T, ticketID uint) (string, string) {
	t.Helper()
	var ticket ticketModel
	if err := f.db.First(&ticket, ticketID).Error; err != nil {
		t.Fatalf("reload ticket: %v", err)
	}
	return ticket.Status, ticket.Outcome
}

func TestClassicMatrixStartToEndCompletesTicket(t *testing.T) {
	f := newClassicMatrixFixture(t)
	workflow := json.RawMessage(`{
		"nodes":[
			{"id":"start","type":"start","data":{"label":"开始"}},
			{"id":"end","type":"end","data":{"label":"结束"}}
		],
		"edges":[{"id":"e1","source":"start","target":"end","data":{"default":true}}]
	}`)
	ticket := f.createTicket(t, workflow)

	if err := f.start(t, ticket, workflow); err != nil {
		t.Fatalf("start: %v", err)
	}

	if got := f.ticketStatus(t, ticket.ID); got != "completed" {
		t.Fatalf("ticket status = %q, want completed", got)
	}
	var token executionTokenModel
	if err := f.db.Where("ticket_id = ?", ticket.ID).First(&token).Error; err != nil {
		t.Fatalf("find token: %v", err)
	}
	if token.Status != TokenCompleted {
		t.Fatalf("token status = %q, want completed", token.Status)
	}
	var count int64
	f.db.Model(&timelineModel{}).Where("ticket_id = ? AND event_type = ?", ticket.ID, "workflow_completed").Count(&count)
	if count != 1 {
		t.Fatalf("workflow_completed timeline count = %d, want 1", count)
	}
}

func TestClassicMatrixSharedEndPreservesHumanOutcomeOnTicket(t *testing.T) {
	tests := []struct {
		name        string
		outcome     string
		wantStatus  string
		wantOutcome string
	}{
		{name: "approved", outcome: ActivityApproved, wantStatus: TicketStatusCompleted, wantOutcome: TicketOutcomeApproved},
		{name: "rejected", outcome: ActivityRejected, wantStatus: TicketStatusRejected, wantOutcome: TicketOutcomeRejected},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newClassicMatrixFixture(t)
			workflow := json.RawMessage(`{
				"nodes":[
					{"id":"start","type":"start","data":{"label":"开始"}},
					{"id":"process","type":"process","data":{"label":"处理","participants":[{"type":"user","value":"7"}]}},
					{"id":"end","type":"end","data":{"label":"完成"}}
				],
				"edges":[
					{"id":"e1","source":"start","target":"process","data":{}},
					{"id":"e2","source":"process","target":"end","data":{"outcome":"approved"}},
					{"id":"e3","source":"process","target":"end","data":{"outcome":"rejected"}}
				]
			}`)
			ticket := f.createTicket(t, workflow)
			if err := f.start(t, ticket, workflow); err != nil {
				t.Fatalf("start: %v", err)
			}
			activity := f.firstActivity(t, ticket.ID, NodeProcess)

			if err := f.progress(t, ticket.ID, activity, tt.outcome, nil); err != nil {
				t.Fatalf("progress: %v", err)
			}

			gotStatus, gotOutcome := f.ticketStatusOutcome(t, ticket.ID)
			if gotStatus != tt.wantStatus || gotOutcome != tt.wantOutcome {
				t.Fatalf("ticket status/outcome = %s/%s, want %s/%s", gotStatus, gotOutcome, tt.wantStatus, tt.wantOutcome)
			}
		})
	}
}

func TestClassicMatrixHumanNodesCreateAssignmentsAndProgress(t *testing.T) {
	tests := []struct {
		name       string
		nodeType   string
		outcome    string
		result     json.RawMessage
		formSchema string
	}{
		{name: "form", nodeType: NodeForm, outcome: "submitted", result: json.RawMessage(`{"account":"alice"}`), formSchema: `{"version":1,"fields":[{"key":"account","type":"text","label":"账号","binding":"account"}]}`},
		{name: "process", nodeType: NodeProcess, outcome: "completed", result: json.RawMessage(`{"result":"done"}`), formSchema: `{"version":1,"fields":[{"key":"result","type":"text","label":"结果","binding":"result"}]}`},
		{name: "approve", nodeType: NodeApprove, outcome: "approved", result: nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newClassicMatrixFixture(t)
			workflow := json.RawMessage(fmt.Sprintf(`{
				"nodes":[
					{"id":"start","type":"start","data":{"label":"开始"}},
					{"id":"human","type":%q,"data":{"label":"人工节点","participants":[{"type":"user","value":"7"}],"formSchema":%s}},
					{"id":"end","type":"end","data":{"label":"结束"}}
				],
				"edges":[
					{"id":"e1","source":"start","target":"human","data":{}},
					{"id":"e2","source":"human","target":"end","data":{"outcome":%q}}
				]
			}`, tt.nodeType, quotedRawJSON(tt.formSchema), tt.outcome))
			ticket := f.createTicket(t, workflow)

			if err := f.start(t, ticket, workflow); err != nil {
				t.Fatalf("start: %v", err)
			}
			activity := f.firstActivity(t, ticket.ID, tt.nodeType)

			var assignment assignmentModel
			if err := f.db.Where("activity_id = ?", activity.ID).First(&assignment).Error; err != nil {
				t.Fatalf("find assignment: %v", err)
			}
			if assignment.UserID == nil || *assignment.UserID != 7 || assignment.Status != "pending" {
				t.Fatalf("unexpected assignment: %+v", assignment)
			}

			if err := f.progress(t, ticket.ID, activity, tt.outcome, tt.result); err != nil {
				t.Fatalf("progress: %v", err)
			}
			if got := f.ticketStatus(t, ticket.ID); got != "completed" {
				t.Fatalf("ticket status = %q, want completed", got)
			}
		})
	}
}

func TestClassicMatrixFormBindingsSkipEmptyValues(t *testing.T) {
	f := newClassicMatrixFixture(t)
	workflow := json.RawMessage(`{
		"nodes":[
			{"id":"start","type":"start","data":{"label":"开始"}},
			{"id":"form","type":"form","data":{"label":"补充信息","participants":[{"type":"user","value":"7"}],"formSchema":{"version":1,"fields":[{"key":"node_value","type":"text","label":"节点值","binding":"node_value"},{"key":"node_empty","type":"text","label":"节点空值","binding":"node_empty"}]}}},
			{"id":"end","type":"end","data":{"label":"结束"}}
		],
		"edges":[
			{"id":"e1","source":"start","target":"form","data":{}},
			{"id":"e2","source":"form","target":"end","data":{"outcome":"submitted"}}
		]
	}`)
	ticket := f.createTicket(t, workflow)
	startSchema := `{"version":1,"fields":[{"key":"request_kind","type":"text","label":"类型","binding":"request_kind"},{"key":"empty_text","type":"text","label":"空值","binding":"empty_text"}]}`

	if err := f.start(t, ticket, workflow, func(params *StartParams) {
		params.StartFormSchema = startSchema
		params.StartFormData = `{"request_kind":"vpn","empty_text":""}`
	}); err != nil {
		t.Fatalf("start: %v", err)
	}

	activity := f.firstActivity(t, ticket.ID, NodeForm)
	if err := f.progress(t, ticket.ID, activity, "submitted", json.RawMessage(`{"node_value":"ok","node_empty":""}`)); err != nil {
		t.Fatalf("progress: %v", err)
	}

	var vars []processVariableModel
	if err := f.db.Where("ticket_id = ?", ticket.ID).Order("key ASC").Find(&vars).Error; err != nil {
		t.Fatalf("query vars: %v", err)
	}
	got := map[string]string{}
	for _, v := range vars {
		got[v.Key] = v.Value
	}
	if got["request_kind"] != "vpn" || got["node_value"] != "ok" {
		t.Fatalf("expected bound values, got %+v", got)
	}
	if _, ok := got["empty_text"]; ok {
		t.Fatalf("empty start field should not be written: %+v", got)
	}
	if _, ok := got["node_empty"]; ok {
		t.Fatalf("empty node field should not be written: %+v", got)
	}
}

func TestClassicMatrixExclusiveGatewayBranches(t *testing.T) {
	tests := []struct {
		name      string
		formData  string
		wantLabel string
		wantErr   bool
	}{
		{name: "condition match", formData: `{"request_kind":"network"}`, wantLabel: "网络处理"},
		{name: "default branch", formData: `{"request_kind":"other"}`, wantLabel: "默认处理"},
		{name: "no branch", formData: `{"request_kind":"none"}`, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newClassicMatrixFixture(t)
			defaultEdge := `{"id":"e3","source":"gateway","target":"fallback","data":{"default":true}}`
			if tt.wantErr {
				defaultEdge = `{"id":"e3","source":"gateway","target":"fallback","data":{"condition":{"field":"form.request_kind","operator":"equals","value":"missing"}}}`
			}
			workflow := json.RawMessage(fmt.Sprintf(`{
				"nodes":[
					{"id":"start","type":"start","data":{"label":"开始"}},
					{"id":"gateway","type":"exclusive","data":{"label":"判断"}},
					{"id":"network","type":"process","data":{"label":"网络处理","participants":[{"type":"user","value":"7"}]}},
					{"id":"fallback","type":"process","data":{"label":"默认处理","participants":[{"type":"user","value":"7"}]}},
					{"id":"end","type":"end","data":{"label":"结束"}}
				],
				"edges":[
					{"id":"e1","source":"start","target":"gateway","data":{}},
					{"id":"e2","source":"gateway","target":"network","data":{"condition":{"field":"form.request_kind","operator":"equals","value":"network"}}},
					%s,
					{"id":"e4","source":"network","target":"end","data":{"outcome":"completed"}},
					{"id":"e5","source":"fallback","target":"end","data":{"outcome":"completed"}}
				]
			}`, defaultEdge))
			ticket := f.createTicket(t, workflow)

			err := f.start(t, ticket, workflow, func(params *StartParams) {
				params.StartFormSchema = `{"version":1,"fields":[{"key":"request_kind","type":"text","label":"类型","binding":"request_kind"}]}`
				params.StartFormData = tt.formData
			})
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected gateway error")
				}
				if got := f.ticketStatus(t, ticket.ID); got != "failed" {
					t.Fatalf("ticket status = %q, want failed", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("start: %v", err)
			}
			activity := f.firstActivity(t, ticket.ID, NodeProcess)
			if activity.Name != tt.wantLabel {
				t.Fatalf("activity label = %q, want %q", activity.Name, tt.wantLabel)
			}
		})
	}
}

func TestClassicMatrixParallelAndInclusiveJoinWaitForAllBranches(t *testing.T) {
	tests := []struct {
		name     string
		nodeType string
		data     string
		edges    string
	}{
		{
			name:     "parallel",
			nodeType: NodeParallel,
			data:     `{"label":"并行分发","gateway_direction":"fork"}`,
			edges:    `{"id":"e2","source":"fork","target":"a","data":{}},{"id":"e3","source":"fork","target":"b","data":{}}`,
		},
		{
			name:     "inclusive",
			nodeType: NodeInclusive,
			data:     `{"label":"包含分发","gateway_direction":"fork"}`,
			edges:    `{"id":"e2","source":"fork","target":"a","data":{"condition":{"field":"form.need_a","operator":"equals","value":"yes"}}},{"id":"e3","source":"fork","target":"b","data":{"default":true}}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newClassicMatrixFixture(t)
			workflow := json.RawMessage(fmt.Sprintf(`{
				"nodes":[
					{"id":"start","type":"start","data":{"label":"开始"}},
					{"id":"fork","type":%q,"data":%s},
					{"id":"a","type":"process","data":{"label":"A","participants":[{"type":"user","value":"7"}]}},
					{"id":"b","type":"process","data":{"label":"B","participants":[{"type":"user","value":"7"}]}},
					{"id":"join","type":%q,"data":{"label":"汇聚","gateway_direction":"join"}},
					{"id":"end","type":"end","data":{"label":"结束"}}
				],
				"edges":[
					{"id":"e1","source":"start","target":"fork","data":{}},
					%s,
					{"id":"e4","source":"a","target":"join","data":{"outcome":"completed"}},
					{"id":"e5","source":"b","target":"join","data":{"outcome":"completed"}},
					{"id":"e6","source":"join","target":"end","data":{}}
				]
			}`, tt.nodeType, tt.data, tt.nodeType, tt.edges))
			ticket := f.createTicket(t, workflow)

			if err := f.start(t, ticket, workflow, func(params *StartParams) {
				params.StartFormSchema = `{"version":1,"fields":[{"key":"need_a","type":"text","label":"需要A","binding":"need_a"}]}`
				params.StartFormData = `{"need_a":"yes"}`
			}); err != nil {
				t.Fatalf("start: %v", err)
			}

			var activities []activityModel
			if err := f.db.Where("ticket_id = ? AND activity_type = ? AND status = ?", ticket.ID, NodeProcess, ActivityPending).
				Order("name ASC").Find(&activities).Error; err != nil {
				t.Fatalf("query activities: %v", err)
			}
			if len(activities) != 2 {
				t.Fatalf("pending process activities = %d, want 2", len(activities))
			}

			if err := f.progress(t, ticket.ID, activities[0], "completed", nil); err != nil {
				t.Fatalf("progress first branch: %v", err)
			}
			if got := f.ticketStatus(t, ticket.ID); got != TicketStatusWaitingHuman {
				t.Fatalf("ticket status after first branch = %q, want %s", got, TicketStatusWaitingHuman)
			}

			if err := f.progress(t, ticket.ID, activities[1], "completed", nil); err != nil {
				t.Fatalf("progress second branch: %v", err)
			}
			if got := f.ticketStatus(t, ticket.ID); got != "completed" {
				t.Fatalf("ticket status after join = %q, want completed", got)
			}
		})
	}
}

func TestClassicMatrixAutoNodes(t *testing.T) {
	t.Run("action submits task", func(t *testing.T) {
		f := newClassicMatrixFixture(t)
		workflow := json.RawMessage(`{
			"nodes":[
				{"id":"start","type":"start","data":{"label":"开始"}},
				{"id":"action","type":"action","data":{"label":"动作","action_id":42}},
				{"id":"end","type":"end","data":{"label":"结束"}}
			],
			"edges":[{"id":"e1","source":"start","target":"action","data":{}},{"id":"e2","source":"action","target":"end","data":{"outcome":"success"}}]
		}`)
		ticket := f.createTicket(t, workflow)
		if err := f.start(t, ticket, workflow); err != nil {
			t.Fatalf("start: %v", err)
		}
		if len(f.submitter.tasks) != 1 || f.submitter.tasks[0].name != "itsm-action-execute" {
			t.Fatalf("unexpected tasks: %+v", f.submitter.tasks)
		}
		var payload ActionExecutePayload
		if err := json.Unmarshal(f.submitter.tasks[0].payload, &payload); err != nil {
			t.Fatalf("decode action payload: %v", err)
		}
		if payload.TicketID != ticket.ID || payload.ActionID != 42 || payload.ActivityID == 0 {
			t.Fatalf("unexpected payload: %+v", payload)
		}
	})

	t.Run("notify advances without notifier", func(t *testing.T) {
		f := newClassicMatrixFixture(t)
		workflow := json.RawMessage(`{
			"nodes":[
				{"id":"start","type":"start","data":{"label":"开始"}},
				{"id":"notify","type":"notify","data":{"label":"通知","template":"hello"}},
				{"id":"end","type":"end","data":{"label":"结束"}}
			],
			"edges":[{"id":"e1","source":"start","target":"notify","data":{}},{"id":"e2","source":"notify","target":"end","data":{}}]
		}`)
		ticket := f.createTicket(t, workflow)
		if err := f.start(t, ticket, workflow); err != nil {
			t.Fatalf("start: %v", err)
		}
		if got := f.ticketStatus(t, ticket.ID); got != "completed" {
			t.Fatalf("ticket status = %q, want completed", got)
		}
	})

	t.Run("wait signal and timer create active activities", func(t *testing.T) {
		tests := []struct {
			name       string
			data       string
			wantStatus string
			wantTask   bool
		}{
			{name: "signal", data: `{"label":"等信号","wait_mode":"signal"}`, wantStatus: ActivityPending},
			{name: "timer", data: `{"label":"等时间","wait_mode":"timer","duration":"1h"}`, wantStatus: ActivityInProgress, wantTask: true},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				f := newClassicMatrixFixture(t)
				workflow := json.RawMessage(fmt.Sprintf(`{
					"nodes":[
						{"id":"start","type":"start","data":{"label":"开始"}},
						{"id":"wait","type":"wait","data":%s},
						{"id":"end","type":"end","data":{"label":"结束"}}
					],
					"edges":[{"id":"e1","source":"start","target":"wait","data":{}},{"id":"e2","source":"wait","target":"end","data":{"outcome":"timeout","default":true}}]
				}`, tt.data))
				ticket := f.createTicket(t, workflow)
				if err := f.start(t, ticket, workflow); err != nil {
					t.Fatalf("start: %v", err)
				}
				activity := f.firstActivity(t, ticket.ID, NodeWait)
				if activity.Status != tt.wantStatus {
					t.Fatalf("wait status = %q, want %q", activity.Status, tt.wantStatus)
				}
				if gotTask := len(f.submitter.tasks) == 1 && f.submitter.tasks[0].name == "itsm-wait-timer"; gotTask != tt.wantTask {
					t.Fatalf("timer task presence = %v, want %v; tasks=%+v", gotTask, tt.wantTask, f.submitter.tasks)
				}
			})
		}
	})
}

func TestClassicMatrixScriptAndSubprocess(t *testing.T) {
	t.Run("script writes variables and subsequent assignments can reuse them", func(t *testing.T) {
		f := newClassicMatrixFixture(t)
		workflow := json.RawMessage(`{
			"nodes":[
				{"id":"start","type":"start","data":{"label":"开始"}},
				{"id":"script","type":"script","data":{"label":"脚本","assignments":[{"variable":"score","expression":"ticket_priority_id + 1"},{"variable":"next_score","expression":"score + 1"}]}},
				{"id":"end","type":"end","data":{"label":"结束"}}
			],
			"edges":[{"id":"e1","source":"start","target":"script","data":{}},{"id":"e2","source":"script","target":"end","data":{}}]
		}`)
		ticket := f.createTicket(t, workflow)
		if err := f.start(t, ticket, workflow); err != nil {
			t.Fatalf("start: %v", err)
		}

		var vars []processVariableModel
		if err := f.db.Where("ticket_id = ?", ticket.ID).Find(&vars).Error; err != nil {
			t.Fatalf("query vars: %v", err)
		}
		got := map[string]string{}
		for _, v := range vars {
			got[v.Key] = v.Value
		}
		if got["score"] != "5" || got["next_score"] != "6" {
			t.Fatalf("unexpected script vars: %+v", got)
		}
	})

	t.Run("subprocess isolates scope then resumes parent workflow", func(t *testing.T) {
		f := newClassicMatrixFixture(t)
		workflow := json.RawMessage(`{
			"nodes":[
				{"id":"start","type":"start","data":{"label":"开始"}},
				{"id":"sub","type":"subprocess","data":{"label":"子流程","subprocess_def":{"nodes":[{"id":"sub_start","type":"start","data":{"label":"子开始"}},{"id":"sub_form","type":"form","data":{"label":"子表单","participants":[{"type":"user","value":"7"}],"formSchema":{"version":1,"fields":[{"key":"sub_value","type":"text","label":"子值","binding":"sub_value"}]}}},{"id":"sub_end","type":"end","data":{"label":"子结束"}}],"edges":[{"id":"se1","source":"sub_start","target":"sub_form","data":{}},{"id":"se2","source":"sub_form","target":"sub_end","data":{"outcome":"submitted"}}]}}},
				{"id":"end","type":"end","data":{"label":"结束"}}
			],
			"edges":[{"id":"e1","source":"start","target":"sub","data":{}},{"id":"e2","source":"sub","target":"end","data":{}}]
		}`)
		ticket := f.createTicket(t, workflow)
		if err := f.start(t, ticket, workflow); err != nil {
			t.Fatalf("start: %v", err)
		}

		activity := f.firstActivity(t, ticket.ID, NodeForm)
		if activity.NodeID != "sub_form" {
			t.Fatalf("activity node = %q, want sub_form", activity.NodeID)
		}
		if err := f.progress(t, ticket.ID, activity, "submitted", json.RawMessage(`{"sub_value":"inside"}`)); err != nil {
			t.Fatalf("progress subprocess form: %v", err)
		}
		if got := f.ticketStatus(t, ticket.ID); got != "completed" {
			t.Fatalf("ticket status = %q, want completed", got)
		}

		var scopedVar processVariableModel
		if err := f.db.Where("ticket_id = ? AND scope_id = ? AND key = ?", ticket.ID, "sub", "sub_value").First(&scopedVar).Error; err != nil {
			t.Fatalf("expected subprocess scoped variable: %v", err)
		}
		if scopedVar.Value != "inside" {
			t.Fatalf("subprocess var = %q, want inside", scopedVar.Value)
		}
	})
}

func TestClassicMatrixFailureProtection(t *testing.T) {
	f := newClassicMatrixFixture(t)
	workflow := json.RawMessage(`{"nodes":[{"id":"bad","type":"process","data":{"label":"bad","participants":[{"type":"user","value":"7"}]}}],"edges":[]}`)
	ticket := f.createTicket(t, workflow)
	if err := f.start(t, ticket, workflow); !errors.Is(err, ErrNoStartNode) {
		t.Fatalf("expected ErrNoStartNode, got %v", err)
	}

	deepNodes := `[{"id":"start","type":"start","data":{"label":"开始"}}`
	deepEdges := ``
	prev := "start"
	for i := 0; i < MaxAutoDepth+2; i++ {
		id := fmt.Sprintf("n%d", i)
		deepNodes += fmt.Sprintf(`,{"id":%q,"type":"script","data":{"label":"脚本","assignments":[{"variable":"v%d","expression":"%d"}]}}`, id, i, i)
		if deepEdges != "" {
			deepEdges += ","
		}
		deepEdges += fmt.Sprintf(`{"id":"e%d","source":%q,"target":%q,"data":{}}`, i, prev, id)
		prev = id
	}
	deepNodes += `,{"id":"end","type":"end","data":{"label":"结束"}}]`
	deepEdges += fmt.Sprintf(`,{"id":"e_end","source":%q,"target":"end","data":{}}`, prev)
	deepWorkflow := json.RawMessage(fmt.Sprintf(`{"nodes":%s,"edges":[%s]}`, deepNodes, deepEdges))
	deepTicket := f.createTicket(t, deepWorkflow)
	if err := f.start(t, deepTicket, deepWorkflow); !errors.Is(err, ErrMaxDepthExceeded) {
		t.Fatalf("expected ErrMaxDepthExceeded, got %v", err)
	}
	if got := f.ticketStatus(t, deepTicket.ID); got != "failed" {
		t.Fatalf("deep ticket status = %q, want failed", got)
	}
}

func quotedRawJSON(raw string) string {
	if raw == "" {
		return "null"
	}
	return raw
}
