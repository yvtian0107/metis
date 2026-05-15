package engine

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"gorm.io/gorm"
)

func TestApplyDBBackupWhitelistGuardRunsPrecheckAndRoutesToDBAdmin(t *testing.T) {
	db := newSmartGuardDB(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()

	service := createDBBackupGuardService(t, db, server.URL)
	ticket := ticketModel{
		ID:        42,
		ServiceID: service.ID,
		Status:    TicketStatusDecisioning,
		FormData:  validDBBackupFormData,
	}
	if err := db.Create(&ticket).Error; err != nil {
		t.Fatalf("create ticket: %v", err)
	}

	engine := &SmartEngine{actionExecutor: NewActionExecutor(db)}
	plan := &DecisionPlan{NextStepType: "complete", Confidence: 0.1}
	if err := engine.applyDBBackupWhitelistGuard(context.Background(), db, ticket.ID, plan, &service); err != nil {
		t.Fatalf("apply db backup whitelist guard: %v", err)
	}

	if plan.NextStepType != NodeProcess || plan.ExecutionMode != "single" || len(plan.Activities) != 1 {
		t.Fatalf("expected single process plan after precheck, got %+v", plan)
	}
	if got := plan.Activities[0]; got.ParticipantType != "position_department" || got.PositionCode != "db_admin" || got.DepartmentCode != "it" {
		t.Fatalf("expected db_admin routing after precheck, got %+v", got)
	}
	if plan.Confidence < DefaultConfidenceThreshold || !strings.Contains(plan.Reasoning, "预检动作已执行成功") {
		t.Fatalf("expected upgraded confidence and precheck reasoning, got %+v", plan)
	}

	var execRow actionExecutionModel
	if err := db.Where("ticket_id = ? AND service_action_id = ?", ticket.ID, 8).First(&execRow).Error; err != nil {
		t.Fatalf("load precheck execution: %v", err)
	}
	if execRow.Status != "success" {
		t.Fatalf("expected precheck success row, got %+v", execRow)
	}

	var timeline timelineModel
	if err := db.Where("ticket_id = ? AND event_type = ?", ticket.ID, "ai_decision_action_executed").First(&timeline).Error; err != nil {
		t.Fatalf("load precheck timeline: %v", err)
	}
	if !strings.Contains(timeline.Message, "预检") {
		t.Fatalf("expected precheck timeline message, got %q", timeline.Message)
	}
}

func TestApplyDBBackupWhitelistGuardCompletesAfterDBAdminAndApply(t *testing.T) {
	db := newSmartGuardDB(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()

	service := createDBBackupGuardService(t, db, server.URL)
	ticket := ticketModel{
		ID:        43,
		ServiceID: service.ID,
		Status:    TicketStatusDecisioning,
		FormData:  validDBBackupFormData,
	}
	if err := db.Create(&ticket).Error; err != nil {
		t.Fatalf("create ticket: %v", err)
	}
	if err := db.Create(&actionExecutionModel{TicketID: ticket.ID, ServiceActionID: 8, Status: "success"}).Error; err != nil {
		t.Fatalf("seed precheck execution: %v", err)
	}
	seedCompletedPositionProcess(t, db, ticket.ID, 100, 1, 1, ActivityApproved)

	engine := &SmartEngine{actionExecutor: NewActionExecutor(db)}
	plan := &DecisionPlan{NextStepType: NodeProcess, Confidence: 0.2}
	if err := engine.applyDBBackupWhitelistGuard(context.Background(), db, ticket.ID, plan, &service); err != nil {
		t.Fatalf("apply db backup whitelist guard: %v", err)
	}

	if plan.NextStepType != "complete" || len(plan.Activities) != 0 {
		t.Fatalf("expected complete plan after dba and apply, got %+v", plan)
	}
	if !strings.Contains(plan.Reasoning, "放行动作已执行成功") {
		t.Fatalf("expected apply completion reasoning, got %q", plan.Reasoning)
	}

	var executions []actionExecutionModel
	if err := db.Where("ticket_id = ?", ticket.ID).Order("id asc").Find(&executions).Error; err != nil {
		t.Fatalf("list executions: %v", err)
	}
	if len(executions) != 2 || executions[1].ServiceActionID != 9 || executions[1].Status != "success" {
		t.Fatalf("expected apply execution appended after precheck, got %+v", executions)
	}
}

// TestApplyDBBackupWhitelistGuardRecognizesAssignCompletion is a regression test
// for the bug in TICK-00109: when an admin uses the "Assign" operation to take
// over a db_admin step, the Assign() call used to clear position_id/department_id.
// The guard's ticketHasSatisfiedPositionProcess INNER-JOINs on position_id, so a
// NULL value caused it to conclude the db_admin step had never happened, routing
// the ticket back to db_admin instead of executing the apply action.
//
// After the fix, Assign() preserves position_id and department_id, so the guard
// correctly recognises the step as completed even though participant_type="user".
func TestApplyDBBackupWhitelistGuardRecognizesAssignCompletion(t *testing.T) {
	db := newSmartGuardDB(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()

	service := createDBBackupGuardService(t, db, server.URL)
	ticket := ticketModel{
		ID:        44,
		ServiceID: service.ID,
		Status:    TicketStatusDecisioning,
		FormData:  validDBBackupFormData,
	}
	if err := db.Create(&ticket).Error; err != nil {
		t.Fatalf("create ticket: %v", err)
	}
	// precheck already done
	if err := db.Create(&actionExecutionModel{TicketID: ticket.ID, ServiceActionID: 8, Status: "success"}).Error; err != nil {
		t.Fatalf("seed precheck execution: %v", err)
	}
	// db_admin step completed by admin via Assign() — participant_type="user" but
	// position_id/department_id preserved (positionID=1 maps to db_admin in test DB)
	const adminUserID uint = 99
	seedCompletedUserAssignedPositionProcess(t, db, ticket.ID, 200, 1, 1, adminUserID, ActivityApproved)

	engine := &SmartEngine{actionExecutor: NewActionExecutor(db)}
	plan := &DecisionPlan{NextStepType: NodeProcess, Confidence: 0.2}
	if err := engine.applyDBBackupWhitelistGuard(context.Background(), db, ticket.ID, plan, &service); err != nil {
		t.Fatalf("apply db backup whitelist guard: %v", err)
	}

	// Guard should advance to "complete" (execute apply action then end), not re-route to db_admin.
	if plan.NextStepType != "complete" {
		t.Fatalf("expected complete plan when db_admin step was processed via Assign, got nextStepType=%q (plan=%+v)", plan.NextStepType, plan)
	}
	if len(plan.Activities) != 0 {
		t.Fatalf("expected no pending activities in complete plan, got %+v", plan.Activities)
	}

	// Apply action must have been executed.
	var executions []actionExecutionModel
	if err := db.Where("ticket_id = ?", ticket.ID).Order("id asc").Find(&executions).Error; err != nil {
		t.Fatalf("list executions: %v", err)
	}
	if len(executions) != 2 || executions[1].ServiceActionID != 9 || executions[1].Status != "success" {
		t.Fatalf("expected apply execution after assign-completed db_admin step, got %+v", executions)
	}
}

func TestApplyBossSerialChangeGuardFollowsRequiredTwoStageSequence(t *testing.T) {
	t.Run("routes to headquarters first", func(t *testing.T) {
		db := newSmartGuardDB(t)
		if err := db.Create(&ticketModel{ID: 51, Status: TicketStatusDecisioning}).Error; err != nil {
			t.Fatalf("create ticket: %v", err)
		}
		plan := &DecisionPlan{NextStepType: "complete"}
		if err := (&SmartEngine{}).applyBossSerialChangeGuard(db, 51, plan); err != nil {
			t.Fatalf("apply boss serial guard: %v", err)
		}
		assertSingleDepartmentRoute(t, plan, "headquarters", "serial_reviewer")
	})

	t.Run("routes to ops after headquarters completion", func(t *testing.T) {
		db := newSmartGuardDB(t)
		if err := db.Create(&ticketModel{ID: 52, Status: TicketStatusDecisioning}).Error; err != nil {
			t.Fatalf("create ticket: %v", err)
		}
		seedCompletedPositionProcess(t, db, 52, 101, 2, 2, ActivityApproved)
		plan := &DecisionPlan{NextStepType: "complete"}
		if err := (&SmartEngine{}).applyBossSerialChangeGuard(db, 52, plan); err != nil {
			t.Fatalf("apply boss serial guard: %v", err)
		}
		assertSingleDepartmentRoute(t, plan, "it", "ops_admin")
	})

	t.Run("completes after both stages succeed", func(t *testing.T) {
		db := newSmartGuardDB(t)
		if err := db.Create(&ticketModel{ID: 53, Status: TicketStatusDecisioning}).Error; err != nil {
			t.Fatalf("create ticket: %v", err)
		}
		seedCompletedPositionProcess(t, db, 53, 102, 2, 2, ActivityApproved)
		seedCompletedPositionProcess(t, db, 53, 103, 3, 1, ActivityApproved)
		plan := &DecisionPlan{NextStepType: NodeProcess}
		if err := (&SmartEngine{}).applyBossSerialChangeGuard(db, 53, plan); err != nil {
			t.Fatalf("apply boss serial guard: %v", err)
		}
		if plan.NextStepType != "complete" || len(plan.Activities) != 0 || !strings.Contains(plan.Reasoning, "均已完成") {
			t.Fatalf("expected completion after both serial stages, got %+v", plan)
		}
	})
}

func TestApplyDeterministicServiceGuardsDispatchesWhitelistPolicy(t *testing.T) {
	db := newSmartGuardDB(t)
	if err := db.Create(&ticketModel{
		ID:        54,
		ServiceID: 7,
		Status:    TicketStatusDecisioning,
		FormData:  validDBBackupFormData,
	}).Error; err != nil {
		t.Fatalf("create ticket: %v", err)
	}
	if err := db.Create(&serviceActionModel{ID: 8, Name: "预检", Code: "db_backup_whitelist_precheck", ServiceID: 7, IsActive: true, ActionType: "http", ConfigJSON: `{"url":"http://127.0.0.1:1","method":"POST","body":"{}","timeout":1,"retries":0}`}).Error; err != nil {
		t.Fatalf("seed precheck action: %v", err)
	}
	if err := db.Create(&actionExecutionModel{TicketID: 54, ServiceActionID: 8, Status: "success"}).Error; err != nil {
		t.Fatalf("seed precheck execution: %v", err)
	}

	engine := &SmartEngine{}
	plan := &DecisionPlan{NextStepType: "complete"}
	svc := &serviceModel{
		ID:                7,
		CollaborationSpec: dbBackupWhitelistSpec,
	}
	if err := engine.applyDeterministicServiceGuards(context.Background(), db, 54, plan, svc); err != nil {
		t.Fatalf("apply deterministic guards: %v", err)
	}
	assertSingleDepartmentRoute(t, plan, "it", "db_admin")
}

func TestAccessPurposeRoutePolicyAndSingleHumanGuardRespectRoutingEvidence(t *testing.T) {
	t.Run("routes network diagnostics to network admin", func(t *testing.T) {
		db := newSmartGuardDB(t)
		if err := db.Create(&ticketModel{
			ID:       61,
			Status:   TicketStatusDecisioning,
			Title:    "生产访问",
			FormData: `{"access_reason":"需要抓包排查网络访问路径"}`,
		}).Error; err != nil {
			t.Fatalf("create ticket: %v", err)
		}
		plan := &DecisionPlan{NextStepType: "complete"}
		applied, err := accessPurposeRoutePolicy(context.Background(), &SmartEngine{}, db, 61, plan, &serviceModel{CollaborationSpec: testServerAccessRoutingSpec})
		if err != nil {
			t.Fatalf("apply access purpose policy: %v", err)
		}
		if !applied {
			t.Fatal("expected access purpose policy to apply")
		}
		assertSingleDepartmentRoute(t, plan, "it", "network_admin")
	})

	t.Run("completed routed position ends workflow", func(t *testing.T) {
		db := newSmartGuardDB(t)
		if err := db.Create(&ticketModel{
			ID:       62,
			Status:   TicketStatusDecisioning,
			Title:    "生产访问",
			FormData: `{"operation_purpose":"抓包定位网络链路异常"}`,
		}).Error; err != nil {
			t.Fatalf("create ticket: %v", err)
		}
		seedCompletedPositionProcess(t, db, 62, 120, 11, 1, ActivityApproved)

		plan := &DecisionPlan{NextStepType: NodeProcess}
		applied, err := accessPurposeRoutePolicy(context.Background(), &SmartEngine{}, db, 62, plan, &serviceModel{CollaborationSpec: testServerAccessRoutingSpec})
		if err != nil {
			t.Fatalf("apply access purpose policy: %v", err)
		}
		if !applied || plan.NextStepType != "complete" || len(plan.Activities) != 0 {
			t.Fatalf("expected completed routed position to finish workflow, got applied=%v plan=%+v", applied, plan)
		}
	})

	t.Run("ambiguous purpose blocks single route", func(t *testing.T) {
		db := newSmartGuardDB(t)
		if err := db.Create(&ticketModel{
			ID:       63,
			Status:   TicketStatusDecisioning,
			Title:    "生产访问",
			FormData: `{"access_reason":"安全审计并抓包定位异常访问"}`,
		}).Error; err != nil {
			t.Fatalf("create ticket: %v", err)
		}
		plan := &DecisionPlan{NextStepType: "complete"}
		applied, err := accessPurposeRoutePolicy(context.Background(), &SmartEngine{}, db, 63, plan, &serviceModel{CollaborationSpec: testServerAccessRoutingSpec})
		if !applied {
			t.Fatal("expected ambiguous access purpose to still apply deterministic guard")
		}
		if err == nil || !strings.Contains(err.Error(), "不得高置信结束或选择单一路由") {
			t.Fatalf("expected ambiguous purpose guard error, got %v", err)
		}
		if plan.NextStepType != "complete" || len(plan.Activities) != 0 {
			t.Fatalf("expected ambiguous purpose to avoid mutating route, got %+v", plan)
		}
	})
}

func TestGuardHelpersClassifyWhitelistActionsAndCompletedPositions(t *testing.T) {
	if !isDBBackupWhitelistActionCode("db_backup_whitelist_precheck") || !isDBBackupWhitelistActionCode("db_backup_whitelist_apply") {
		t.Fatal("expected whitelist action codes to be recognized")
	}
	if isDBBackupWhitelistActionCode("notify") {
		t.Fatal("did not expect unrelated action code to be treated as whitelist action")
	}

	db := newSmartGuardDB(t)
	if err := db.Create(&ticketModel{ID: 64, Status: TicketStatusDecisioning}).Error; err != nil {
		t.Fatalf("create ticket: %v", err)
	}
	done, err := ticketHasCompletedPositionProcess(db, 64, "db_admin")
	if err != nil {
		t.Fatalf("ticketHasCompletedPositionProcess empty: %v", err)
	}
	if done {
		t.Fatal("expected no completed position work before seeding activity")
	}

	seedCompletedPositionProcess(t, db, 64, 130, 1, 1, ActivityApproved)
	done, err = ticketHasCompletedPositionProcess(db, 64, "db_admin")
	if err != nil {
		t.Fatalf("ticketHasCompletedPositionProcess seeded: %v", err)
	}
	if !done {
		t.Fatal("expected completed db_admin process to be detected")
	}
}

func TestExecuteServiceActionOnceHonorsCacheSnapshotAndMissingExecutor(t *testing.T) {
	t.Run("cached success skips executor", func(t *testing.T) {
		db := newSmartGuardDB(t)
		if err := db.Create(&ticketModel{ID: 71, Status: TicketStatusDecisioning}).Error; err != nil {
			t.Fatalf("create ticket: %v", err)
		}
		if err := db.Create(&serviceActionModel{
			ID:         21,
			Name:       "预检",
			Code:       "db_backup_whitelist_precheck",
			ServiceID:  7,
			IsActive:   true,
			ActionType: "http",
			ConfigJSON: `{"url":"http://127.0.0.1:1","method":"POST","body":"{}","timeout":1,"retries":0}`,
		}).Error; err != nil {
			t.Fatalf("seed cached action: %v", err)
		}
		if err := db.Create(&actionExecutionModel{TicketID: 71, ServiceActionID: 21, Status: "success"}).Error; err != nil {
			t.Fatalf("seed cached execution: %v", err)
		}

		err := (&SmartEngine{}).executeServiceActionOnce(context.Background(), db, 71, &serviceModel{ID: 7}, "db_backup_whitelist_precheck")
		if err != nil {
			t.Fatalf("execute cached action: %v", err)
		}

		var count int64
		if err := db.Model(&timelineModel{}).Where("ticket_id = ? AND event_type = ?", 71, "ai_decision_action_executed").Count(&count).Error; err != nil {
			t.Fatalf("count cached timelines: %v", err)
		}
		if count != 0 {
			t.Fatalf("expected cached action to skip timeline writes, got %d", count)
		}
	})

	t.Run("snapshot action executes without live action row", func(t *testing.T) {
		db := newSmartGuardDB(t)
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"ok":true}`))
		}))
		defer server.Close()

		if err := db.Create(&ticketModel{ID: 72, Status: TicketStatusDecisioning, Code: "TICK-SNAPSHOT"}).Error; err != nil {
			t.Fatalf("create ticket: %v", err)
		}
		svc := &serviceModel{
			ID:          8,
			ActionsJSON: fmt.Sprintf(`[{"id":31,"code":"db_backup_whitelist_apply","name":"放行","description":"snapshot","actionType":"http","configJson":{"url":%q,"method":"POST","body":"{}","timeout":5,"retries":0},"isActive":true}]`, server.URL),
		}
		engine := &SmartEngine{actionExecutor: NewActionExecutor(db)}
		if err := engine.executeServiceActionOnce(context.Background(), db, 72, svc, "db_backup_whitelist_apply"); err != nil {
			t.Fatalf("execute snapshot action: %v", err)
		}

		var execRow actionExecutionModel
		if err := db.Where("ticket_id = ? AND service_action_id = ?", 72, 31).First(&execRow).Error; err != nil {
			t.Fatalf("load snapshot execution: %v", err)
		}
		if execRow.Status != "success" {
			t.Fatalf("expected snapshot execution success, got %+v", execRow)
		}
	})

	t.Run("missing executor is rejected before execution", func(t *testing.T) {
		db := newSmartGuardDB(t)
		if err := db.Create(&ticketModel{ID: 73, Status: TicketStatusDecisioning}).Error; err != nil {
			t.Fatalf("create ticket: %v", err)
		}
		err := (&SmartEngine{}).executeServiceActionOnce(context.Background(), db, 73, &serviceModel{ID: 9}, "db_backup_whitelist_apply")
		if err == nil || !strings.Contains(err.Error(), "动作执行器不可用") {
			t.Fatalf("expected missing executor error, got %v", err)
		}
	})
}

func newSmartGuardDB(t *testing.T) *gorm.DB {
	t.Helper()

	db := newSmartContinuationDB(t)
	if err := db.AutoMigrate(&serviceActionModel{}, &actionExecutionModel{}); err != nil {
		t.Fatalf("migrate action tables: %v", err)
	}
	if err := db.Exec(`ALTER TABLE itsm_service_actions ADD COLUMN deleted_at datetime`).Error; err != nil {
		t.Fatalf("add service_actions.deleted_at: %v", err)
	}
	for _, stmt := range []string{
		`CREATE TABLE positions (id integer primary key, code text)`,
		`CREATE TABLE departments (id integer primary key, code text)`,
	} {
		if err := db.Exec(stmt).Error; err != nil {
			t.Fatalf("seed schema %q: %v", stmt, err)
		}
	}
	if err := db.Exec(`INSERT INTO positions (id, code) VALUES (1, 'db_admin'), (2, 'serial_reviewer'), (3, 'ops_admin'), (11, 'network_admin'), (12, 'security_admin')`).Error; err != nil {
		t.Fatalf("seed positions: %v", err)
	}
	if err := db.Exec(`INSERT INTO departments (id, code) VALUES (1, 'it'), (2, 'headquarters')`).Error; err != nil {
		t.Fatalf("seed departments: %v", err)
	}
	return db
}

func createDBBackupGuardService(t *testing.T, db *gorm.DB, serverURL string) serviceModel {
	t.Helper()

	service := serviceModel{
		ID:                7,
		Name:              "数据库备份白名单",
		EngineType:        "smart",
		CollaborationSpec: dbBackupWhitelistSpec,
	}
	if err := db.Create(&service).Error; err != nil {
		t.Fatalf("create service: %v", err)
	}
	actions := []serviceActionModel{
		{ID: 8, Name: "预检", Code: "db_backup_whitelist_precheck", ServiceID: service.ID, IsActive: true, ActionType: "http", ConfigJSON: fmt.Sprintf(`{"url":%q,"method":"POST","body":"{}","timeout":5,"retries":0}`, serverURL)},
		{ID: 9, Name: "放行", Code: "db_backup_whitelist_apply", ServiceID: service.ID, IsActive: true, ActionType: "http", ConfigJSON: fmt.Sprintf(`{"url":%q,"method":"POST","body":"{}","timeout":5,"retries":0}`, serverURL)},
	}
	for _, action := range actions {
		if err := db.Create(&action).Error; err != nil {
			t.Fatalf("create guard action %s: %v", action.Code, err)
		}
	}
	return service
}

func seedCompletedPositionProcess(t *testing.T, db *gorm.DB, ticketID, activityID, positionID, departmentID uint, outcome string) {
	t.Helper()

	assigneeID := uint(1)
	activity := activityModel{
		ID:                activityID,
		TicketID:          ticketID,
		Name:              "处理",
		ActivityType:      NodeProcess,
		Status:            HumanActivityResultStatus(outcome),
		TransitionOutcome: outcome,
	}
	if err := db.Create(&activity).Error; err != nil {
		t.Fatalf("create completed activity: %v", err)
	}
	if err := db.Create(&assignmentModel{
		TicketID:        ticketID,
		ActivityID:      activity.ID,
		ParticipantType: "position_department",
		PositionID:      &positionID,
		DepartmentID:    &departmentID,
		AssigneeID:      &assigneeID,
		Status:          "completed",
	}).Error; err != nil {
		t.Fatalf("create completed assignment: %v", err)
	}
}

// seedCompletedUserAssignedPositionProcess simulates the outcome of Assign():
// participant_type is "user" (a named user processed the step), but position_id
// and department_id are still set to the original role context.  This is the
// correct state after the Phase-1 fix to Assign().
func seedCompletedUserAssignedPositionProcess(t *testing.T, db *gorm.DB, ticketID, activityID, positionID, departmentID, userAssigneeID uint, outcome string) {
	t.Helper()

	activity := activityModel{
		ID:                activityID,
		TicketID:          ticketID,
		Name:              "处理",
		ActivityType:      NodeProcess,
		Status:            HumanActivityResultStatus(outcome),
		TransitionOutcome: outcome,
	}
	if err := db.Create(&activity).Error; err != nil {
		t.Fatalf("create completed activity: %v", err)
	}
	if err := db.Create(&assignmentModel{
		TicketID:        ticketID,
		ActivityID:      activity.ID,
		ParticipantType: "user", // Assign() sets this to "user"
		UserID:          &userAssigneeID,
		PositionID:      &positionID,   // preserved — not cleared by Assign()
		DepartmentID:    &departmentID, // preserved — not cleared by Assign()
		AssigneeID:      &userAssigneeID,
		Status:          "completed",
	}).Error; err != nil {
		t.Fatalf("create completed user-assigned assignment: %v", err)
	}
}

func assertSingleDepartmentRoute(t *testing.T, plan *DecisionPlan, wantDepartment, wantPosition string) {
	t.Helper()

	if plan.NextStepType != NodeProcess || plan.ExecutionMode != "single" || len(plan.Activities) != 1 {
		t.Fatalf("expected single process route, got %+v", plan)
	}
	got := plan.Activities[0]
	if got.ParticipantType != "position_department" || got.DepartmentCode != wantDepartment || got.PositionCode != wantPosition {
		t.Fatalf("expected %s/%s route, got %+v", wantDepartment, wantPosition, got)
	}
}

const dbBackupWhitelistSpec = `员工在 IT 服务台申请生产数据库备份白名单临时放行时，服务台需要确认目标数据库、发起备份访问的来源 IP、白名单放行时间窗，以及这次临时放行的申请原因。
申请资料收齐后，系统会先做一次白名单参数预检，确认数据库、来源 IP、放行窗口和申请原因满足放行前置条件。预检通过后，交给信息部数据库管理员处理。
数据库管理员完成处理后，系统执行备份白名单放行；放行成功后流程结束。驳回时不进入补充或返工，流程按驳回结果结束。`

const validDBBackupFormData = `{"database_name":"prod-orders","source_ip":"10.0.0.8","whitelist_window":"2026-05-10 20:00 ~ 2026-05-10 22:00","access_reason":"应急备份"}`
