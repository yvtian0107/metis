package engine

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	appcore "metis/internal/app"
)

type txRecordingSubmitter struct {
	regularCalls int
	txCalls      int
	lastName     string
	lastPayload  json.RawMessage
}

func (s *txRecordingSubmitter) SubmitTask(string, json.RawMessage) error {
	s.regularCalls++
	return errors.New("regular submitter must not be used from workflow transaction")
}

func (s *txRecordingSubmitter) SubmitTaskTx(tx *gorm.DB, name string, payload json.RawMessage) error {
	if tx == nil {
		return errors.New("missing transaction")
	}
	s.txCalls++
	s.lastName = name
	s.lastPayload = append(s.lastPayload[:0], payload...)
	return nil
}

type failingTxSubmitter struct{}

func (s *failingTxSubmitter) SubmitTask(string, json.RawMessage) error {
	return errors.New("regular submitter must not be used from workflow transaction")
}

func (s *failingTxSubmitter) SubmitTaskTx(*gorm.DB, string, json.RawMessage) error {
	return errors.New("submit failed")
}

type availableDecisionExecutor struct{}

func (availableDecisionExecutor) Execute(context.Context, uint, appcore.AIDecisionRequest) (*appcore.AIDecisionResponse, error) {
	return nil, errors.New("not used by this test")
}

func TestSmartProgressContinuationUsesWorkflowTransaction(t *testing.T) {
	db := newSmartContinuationDB(t)

	ticket, activity := createSmartContinuationTicket(t, db, "", ActivityPending)
	submitter := &txRecordingSubmitter{}
	eng := NewSmartEngine(availableDecisionExecutor{}, nil, nil, nil, submitter, nil)
	err := db.Transaction(func(tx *gorm.DB) error {
		return eng.Progress(context.Background(), tx, ProgressParams{
			TicketID:   ticket.ID,
			ActivityID: activity.ID,
			Outcome:    "completed",
			OperatorID: 1,
		})
	})
	if err != nil {
		t.Fatalf("progress smart activity: %v", err)
	}
	if submitter.regularCalls != 0 {
		t.Fatalf("expected no regular submit calls, got %d", submitter.regularCalls)
	}
	if submitter.txCalls != 0 {
		t.Fatalf("expected no scheduler transaction submit calls, got %d", submitter.txCalls)
	}

	var reloaded ticketModel
	if err := db.First(&reloaded, ticket.ID).Error; err != nil {
		t.Fatalf("reload ticket: %v", err)
	}
	if reloaded.CurrentActivityID != nil {
		t.Fatalf("expected current_activity_id to be cleared while decisioning, got %d", *reloaded.CurrentActivityID)
	}
	if reloaded.Status != TicketStatusDecisioning {
		t.Fatalf("expected ticket status %q, got %q", TicketStatusDecisioning, reloaded.Status)
	}
}

func TestSmartProgressContinuationSubmitFailureRollsBackActivityCompletion(t *testing.T) {
	db := newSmartContinuationDB(t)

	ticket, activity := createSmartContinuationTicket(t, db, "", ActivityPending)
	eng := NewSmartEngine(availableDecisionExecutor{}, nil, nil, nil, &failingTxSubmitter{}, nil)
	err := db.Transaction(func(tx *gorm.DB) error {
		return eng.Progress(context.Background(), tx, ProgressParams{
			TicketID:   ticket.ID,
			ActivityID: activity.ID,
			Outcome:    "completed",
			OperatorID: 1,
		})
	})
	if err != nil {
		t.Fatalf("progress should not depend on smart-progress scheduler submission: %v", err)
	}

	var reloadedActivity activityModel
	if err := db.First(&reloadedActivity, activity.ID).Error; err != nil {
		t.Fatalf("reload activity: %v", err)
	}
	if reloadedActivity.Status != ActivityApproved {
		t.Fatalf("activity status should be %q, got %q", ActivityApproved, reloadedActivity.Status)
	}

	var reloadedTicket ticketModel
	if err := db.First(&reloadedTicket, ticket.ID).Error; err != nil {
		t.Fatalf("reload ticket: %v", err)
	}
	if reloadedTicket.CurrentActivityID != nil {
		t.Fatalf("ticket current_activity_id should clear after progress, got %v", reloadedTicket.CurrentActivityID)
	}
}

func TestSmartProgressContinuationWaitsForParallelGroupConvergence(t *testing.T) {
	db := newSmartContinuationDB(t)

	ticket, first := createSmartContinuationTicket(t, db, "parallel-group", ActivityPending)
	second := activityModel{
		TicketID:        ticket.ID,
		Name:            "并行处理 B",
		ActivityType:    NodeProcess,
		Status:          ActivityPending,
		ActivityGroupID: "parallel-group",
	}
	if err := db.Create(&second).Error; err != nil {
		t.Fatalf("create second activity: %v", err)
	}

	submitter := &txRecordingSubmitter{}
	eng := NewSmartEngine(availableDecisionExecutor{}, nil, nil, nil, submitter, nil)
	if err := db.Transaction(func(tx *gorm.DB) error {
		return eng.Progress(context.Background(), tx, ProgressParams{
			TicketID:   ticket.ID,
			ActivityID: first.ID,
			Outcome:    "completed",
			OperatorID: 1,
		})
	}); err != nil {
		t.Fatalf("progress first parallel activity: %v", err)
	}

	var waitingTicket ticketModel
	if err := db.First(&waitingTicket, ticket.ID).Error; err != nil {
		t.Fatalf("reload waiting ticket: %v", err)
	}
	if waitingTicket.CurrentActivityID == nil || *waitingTicket.CurrentActivityID != first.ID {
		t.Fatalf("parallel group should keep current activity until convergence, got %v", waitingTicket.CurrentActivityID)
	}
	if submitter.txCalls != 0 {
		t.Fatalf("expected no continuation task before parallel convergence, got %d", submitter.txCalls)
	}

	if err := db.Model(&ticketModel{}).Where("id = ?", ticket.ID).Update("current_activity_id", second.ID).Error; err != nil {
		t.Fatalf("move current activity to second: %v", err)
	}
	if err := db.Transaction(func(tx *gorm.DB) error {
		return eng.Progress(context.Background(), tx, ProgressParams{
			TicketID:   ticket.ID,
			ActivityID: second.ID,
			Outcome:    "completed",
			OperatorID: 1,
		})
	}); err != nil {
		t.Fatalf("progress second parallel activity: %v", err)
	}

	var convergedTicket ticketModel
	if err := db.First(&convergedTicket, ticket.ID).Error; err != nil {
		t.Fatalf("reload converged ticket: %v", err)
	}
	if convergedTicket.CurrentActivityID != nil {
		t.Fatalf("current_activity_id should clear after parallel convergence, got %d", *convergedTicket.CurrentActivityID)
	}
	if submitter.txCalls != 0 {
		t.Fatalf("expected no scheduler continuation task after convergence, got %d", submitter.txCalls)
	}
}

func TestSmartStartInitializesWorkflowWithoutRunningDecision(t *testing.T) {
	db := newSmartContinuationDB(t)

	agentID := uint(11)
	service := serviceModel{
		Name:       "智能 VPN 服务",
		EngineType: "smart",
		AgentID:    &agentID,
	}
	if err := db.Create(&service).Error; err != nil {
		t.Fatalf("create service: %v", err)
	}
	ticket := ticketModel{
		ServiceID:   service.ID,
		Status:      "pending",
		EngineType:  "smart",
		RequesterID: 7,
	}
	if err := db.Create(&ticket).Error; err != nil {
		t.Fatalf("create ticket: %v", err)
	}

	eng := NewSmartEngine(availableDecisionExecutor{}, nil, nil, nil, nil, nil)
	err := db.Transaction(func(tx *gorm.DB) error {
		return eng.Start(context.Background(), tx, StartParams{
			TicketID:    ticket.ID,
			RequesterID: ticket.RequesterID,
		})
	})
	if err != nil {
		t.Fatalf("start smart workflow: %v", err)
	}

	var reloaded ticketModel
	if err := db.First(&reloaded, ticket.ID).Error; err != nil {
		t.Fatalf("reload ticket: %v", err)
	}
	if reloaded.Status != TicketStatusDecisioning {
		t.Fatalf("expected ticket status %q, got %q", TicketStatusDecisioning, reloaded.Status)
	}

	var timelineCount int64
	if err := db.Model(&timelineModel{}).
		Where("ticket_id = ? AND event_type = ?", ticket.ID, "workflow_started").
		Count(&timelineCount).Error; err != nil {
		t.Fatalf("count timeline: %v", err)
	}
	if timelineCount != 1 {
		t.Fatalf("expected one workflow_started timeline, got %d", timelineCount)
	}

	var activityCount int64
	if err := db.Model(&activityModel{}).Where("ticket_id = ?", ticket.ID).Count(&activityCount).Error; err != nil {
		t.Fatalf("count activities: %v", err)
	}
	if activityCount != 0 {
		t.Fatalf("initial smart start should not run decision synchronously, got %d activities", activityCount)
	}
}

func TestSmartDecisionPositionAssignmentSingleSQLiteConnectionDoesNotBlock(t *testing.T) {
	db := newSmartContinuationDB(t)
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("get sql db: %v", err)
	}
	sqlDB.SetMaxOpenConns(1)
	sqlDB.SetMaxIdleConns(1)
	if err := db.Exec(`CREATE TABLE users (id integer primary key, username text, is_active boolean)`).Error; err != nil {
		t.Fatalf("create users: %v", err)
	}
	if err := db.Exec(`CREATE TABLE positions (id integer primary key, code text)`).Error; err != nil {
		t.Fatalf("create positions: %v", err)
	}
	if err := db.Exec(`CREATE TABLE departments (id integer primary key, code text)`).Error; err != nil {
		t.Fatalf("create departments: %v", err)
	}
	if err := db.Exec(`CREATE TABLE user_positions (user_id integer, position_id integer, department_id integer, deleted_at datetime)`).Error; err != nil {
		t.Fatalf("create user_positions: %v", err)
	}
	if err := db.Exec(`INSERT INTO users (id, username, is_active) VALUES (7, 'network-operator', true)`).Error; err != nil {
		t.Fatalf("seed user: %v", err)
	}
	if err := db.Exec(`INSERT INTO positions (id, code) VALUES (77, 'network_admin')`).Error; err != nil {
		t.Fatalf("seed position: %v", err)
	}
	if err := db.Exec(`INSERT INTO departments (id, code) VALUES (88, 'it')`).Error; err != nil {
		t.Fatalf("seed department: %v", err)
	}
	if err := db.Exec(`INSERT INTO user_positions (user_id, position_id, department_id) VALUES (7, 77, 88)`).Error; err != nil {
		t.Fatalf("seed user position: %v", err)
	}

	ticket := ticketModel{Status: "in_progress", EngineType: "smart"}
	if err := db.Create(&ticket).Error; err != nil {
		t.Fatalf("create ticket: %v", err)
	}
	eng := NewSmartEngine(
		availableDecisionExecutor{},
		nil,
		nil,
		NewParticipantResolver(&rootDBPositionResolver{db: db}),
		nil,
		nil,
	)
	plan := &DecisionPlan{
		NextStepType:  NodeProcess,
		ExecutionMode: "single",
		Activities: []DecisionActivity{{
			Type:            NodeProcess,
			ParticipantType: "position_department",
			PositionCode:    "network_admin",
			DepartmentCode:  "it",
			Instructions:    "网络管理员处理",
		}},
		Confidence: 0.95,
	}

	done := make(chan error, 1)
	go func() {
		done <- db.Transaction(func(tx *gorm.DB) error {
			return eng.executeDecisionPlan(tx, ticket.ID, plan)
		})
	}()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("execute decision plan: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("smart decision position assignment blocked with a single SQLite connection")
	}

	var assignment assignmentModel
	if err := db.Where("ticket_id = ?", ticket.ID).First(&assignment).Error; err != nil {
		t.Fatalf("load assignment: %v", err)
	}
	if assignment.UserID == nil || *assignment.UserID != 7 || assignment.PositionID == nil || *assignment.PositionID != 77 || assignment.DepartmentID == nil || *assignment.DepartmentID != 88 {
		t.Fatalf("expected transaction-scoped position assignment, got %+v", assignment)
	}

	var reloaded ticketModel
	if err := db.First(&reloaded, ticket.ID).Error; err != nil {
		t.Fatalf("reload ticket: %v", err)
	}
	if reloaded.CurrentActivityID == nil {
		t.Fatal("expected current activity to be set")
	}
}

func TestSmartProgressFailureRecordsDiagnosticStateWithoutBlockingSQLite(t *testing.T) {
	db := newSmartContinuationDB(t)
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("get sql db: %v", err)
	}
	sqlDB.SetMaxOpenConns(1)
	sqlDB.SetMaxIdleConns(1)

	service := serviceModel{Name: "智能服务", EngineType: "smart"}
	if err := db.Create(&service).Error; err != nil {
		t.Fatalf("create service: %v", err)
	}
	ticket := ticketModel{
		ServiceID:  service.ID,
		Status:     "in_progress",
		EngineType: "smart",
	}
	if err := db.Create(&ticket).Error; err != nil {
		t.Fatalf("create ticket: %v", err)
	}

	payload, _ := json.Marshal(SmartProgressPayload{TicketID: ticket.ID, TriggerReason: "manual_retry"})
	handler := HandleSmartProgress(db, NewSmartEngine(nil, nil, nil, nil, nil, nil))
	done := make(chan error, 1)
	go func() {
		done <- handler(context.Background(), payload)
	}()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("handled smart progress failure should not propagate: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("smart-progress failure handling blocked with a single SQLite connection")
	}

	var reloaded ticketModel
	if err := db.First(&reloaded, ticket.ID).Error; err != nil {
		t.Fatalf("reload ticket: %v", err)
	}
	if reloaded.AIFailureCount != 1 {
		t.Fatalf("expected ai_failure_count 1, got %d", reloaded.AIFailureCount)
	}

	var timeline timelineModel
	if err := db.Where("ticket_id = ? AND event_type = ?", ticket.ID, "ai_decision_failed").First(&timeline).Error; err != nil {
		t.Fatalf("load diagnostic timeline: %v", err)
	}
	if timeline.Message == "" {
		t.Fatal("expected diagnostic timeline message")
	}
	var details struct {
		DecisionExplanation map[string]any `json:"decision_explanation"`
	}
	if err := json.Unmarshal([]byte(timeline.Details), &details); err != nil {
		t.Fatalf("decode decision explanation details: %v", err)
	}
	if details.DecisionExplanation == nil {
		t.Fatalf("expected decision_explanation details, got %q", timeline.Details)
	}
	if details.DecisionExplanation["trigger"] != "ai_decision_failed" {
		t.Fatalf("expected trigger ai_decision_failed, got %+v", details.DecisionExplanation)
	}
}

func newSmartContinuationDB(t *testing.T) *gorm.DB {
	t.Helper()

	dsn := fmt.Sprintf("file:smart_continuation_%s?mode=memory&cache=shared", t.Name())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(&serviceModel{}, &ticketModel{}, &activityModel{}, &assignmentModel{}, &timelineModel{}); err != nil {
		t.Fatalf("migrate db: %v", err)
	}
	return db
}

func TestSmartDecisionPositionAssignmentWithoutUsersWaitsForHuman(t *testing.T) {
	db := newSmartContinuationDB(t)
	if err := db.Exec(`CREATE TABLE users (id integer primary key, username text, is_active boolean, deleted_at datetime, manager_id integer)`).Error; err != nil {
		t.Fatalf("create users: %v", err)
	}
	if err := db.Exec(`CREATE TABLE positions (id integer primary key, code text)`).Error; err != nil {
		t.Fatalf("create positions: %v", err)
	}
	if err := db.Exec(`CREATE TABLE departments (id integer primary key, code text)`).Error; err != nil {
		t.Fatalf("create departments: %v", err)
	}
	if err := db.Exec(`CREATE TABLE user_positions (user_id integer, position_id integer, department_id integer, deleted_at datetime)`).Error; err != nil {
		t.Fatalf("create user_positions: %v", err)
	}
	if err := db.Exec(`INSERT INTO positions (id, code) VALUES (77, 'ops_admin')`).Error; err != nil {
		t.Fatalf("seed position: %v", err)
	}
	if err := db.Exec(`INSERT INTO departments (id, code) VALUES (88, 'it')`).Error; err != nil {
		t.Fatalf("seed department: %v", err)
	}

	service := serviceModel{EngineType: "smart"}
	if err := db.Create(&service).Error; err != nil {
		t.Fatalf("create service: %v", err)
	}
	ticket := ticketModel{Status: TicketStatusDecisioning, ServiceID: service.ID, EngineType: "smart"}
	if err := db.Create(&ticket).Error; err != nil {
		t.Fatalf("create ticket: %v", err)
	}

	engine := NewSmartEngine(nil, nil, nil, nil, nil, nil)
	engine.SetDB(db)

	plan := &DecisionPlan{
		NextStepType:  "process",
		ExecutionMode: "single",
		Confidence:    0.95,
		Activities: []DecisionActivity{{
			Type:            "process",
			ParticipantType: "position_department",
			PositionCode:    "ops_admin",
			DepartmentCode:  "it",
			Instructions:    "Handle server troubleshooting access",
		}},
	}

	if err := db.Transaction(func(tx *gorm.DB) error {
		return engine.executeSinglePlan(tx, ticket.ID, plan)
	}); err != nil {
		t.Fatalf("execute single plan: %v", err)
	}

	var reloaded ticketModel
	if err := db.First(&reloaded, ticket.ID).Error; err != nil {
		t.Fatalf("reload ticket: %v", err)
	}
	if reloaded.Status != TicketStatusWaitingHuman {
		t.Fatalf("expected ticket status waiting_human, got %s", reloaded.Status)
	}
	var assignment assignmentModel
	if err := db.Where("ticket_id = ? AND participant_type = ?", ticket.ID, "position_department").First(&assignment).Error; err != nil {
		t.Fatalf("load position assignment: %v", err)
	}
	if assignment.UserID != nil {
		t.Fatalf("expected unresolved assignment user to be nil, got %v", *assignment.UserID)
	}

	var timeline timelineModel
	if err := db.Where("ticket_id = ? AND event_type = ?", ticket.ID, "participant_resolution_pending").First(&timeline).Error; err != nil {
		t.Fatalf("load pending timeline: %v", err)
	}
	if timeline.Message == "" {
		t.Fatal("expected pending participant timeline message")
	}
}

func createSmartContinuationTicket(t *testing.T, db *gorm.DB, groupID string, activityStatus string) (ticketModel, activityModel) {
	t.Helper()
	ticket := ticketModel{Status: "in_progress", EngineType: "smart"}
	if err := db.Create(&ticket).Error; err != nil {
		t.Fatalf("create ticket: %v", err)
	}
	activity := activityModel{
		TicketID:        ticket.ID,
		Name:            "处理",
		ActivityType:    NodeProcess,
		Status:          activityStatus,
		ActivityGroupID: groupID,
	}
	if err := db.Create(&activity).Error; err != nil {
		t.Fatalf("create activity: %v", err)
	}
	if err := db.Model(&ticketModel{}).Where("id = ?", ticket.ID).Update("current_activity_id", activity.ID).Error; err != nil {
		t.Fatalf("set current activity: %v", err)
	}
	ticket.CurrentActivityID = &activity.ID
	return ticket, activity
}

type rootDBPositionResolver struct {
	db *gorm.DB
}

func (r *rootDBPositionResolver) GetUserDeptScope(uint, bool) ([]uint, error) {
	return nil, nil
}

func (r *rootDBPositionResolver) GetUserPositionIDs(uint) ([]uint, error) {
	return nil, nil
}

func (r *rootDBPositionResolver) GetUserDepartmentIDs(uint) ([]uint, error) {
	return nil, nil
}

func (r *rootDBPositionResolver) GetUserPositions(uint) ([]appcore.OrgPosition, error) {
	return nil, nil
}

func (r *rootDBPositionResolver) GetUserDepartment(uint) (*appcore.OrgDepartment, error) {
	return nil, nil
}

func (r *rootDBPositionResolver) QueryContext(string, string, string, bool) (*appcore.OrgContextResult, error) {
	return nil, nil
}

func (r *rootDBPositionResolver) FindUsersByPositionCode(string) ([]uint, error) {
	return nil, nil
}

func (r *rootDBPositionResolver) FindUsersByDepartmentCode(string) ([]uint, error) {
	return nil, nil
}

func (r *rootDBPositionResolver) FindUsersByPositionAndDepartment(posCode, deptCode string) ([]uint, error) {
	var userIDs []uint
	err := r.db.Table("user_positions").
		Joins("JOIN positions ON positions.id = user_positions.position_id").
		Joins("JOIN departments ON departments.id = user_positions.department_id").
		Joins("JOIN users ON users.id = user_positions.user_id").
		Where("positions.code = ? AND departments.code = ? AND user_positions.deleted_at IS NULL AND users.is_active = ?", posCode, deptCode, true).
		Pluck("DISTINCT users.id", &userIDs).Error
	return userIDs, err
}

func (r *rootDBPositionResolver) FindUsersByPositionID(positionID uint) ([]uint, error) {
	var userIDs []uint
	err := r.db.Table("user_positions").
		Joins("JOIN users ON users.id = user_positions.user_id").
		Where("user_positions.position_id = ? AND user_positions.deleted_at IS NULL AND users.is_active = ?", positionID, true).
		Pluck("DISTINCT users.id", &userIDs).Error
	return userIDs, err
}

func (r *rootDBPositionResolver) FindUsersByDepartmentID(departmentID uint) ([]uint, error) {
	var userIDs []uint
	err := r.db.Table("user_positions").
		Joins("JOIN users ON users.id = user_positions.user_id").
		Where("user_positions.department_id = ? AND user_positions.deleted_at IS NULL AND users.is_active = ?", departmentID, true).
		Pluck("DISTINCT users.id", &userIDs).Error
	return userIDs, err
}

func (r *rootDBPositionResolver) FindManagerByUserID(userID uint) (uint, error) {
	var user struct {
		ManagerID *uint
	}
	if err := r.db.Table("users").Where("id = ?", userID).Select("manager_id").First(&user).Error; err != nil {
		return 0, err
	}
	if user.ManagerID == nil {
		return 0, nil
	}
	return *user.ManagerID, nil
}

// regularRecordingSubmitter records SubmitTask calls (non-transactional).
// Used by recovery tests where HandleSmartRecovery calls SubmitProgressTask → SubmitTask.
type regularRecordingSubmitter struct {
	calls    int
	lastName string
}

func (s *regularRecordingSubmitter) SubmitTask(name string, _ json.RawMessage) error {
	s.calls++
	s.lastName = name
	return nil
}

// --- Task 2.3: ensureContinuation in Start/Cancel ---

func TestSmartStartTriggersEnsureContinuation(t *testing.T) {
	db := newSmartContinuationDB(t)

	agentID := uint(11)
	service := serviceModel{
		Name:       "智能 VPN 服务",
		EngineType: "smart",
		AgentID:    &agentID,
	}
	if err := db.Create(&service).Error; err != nil {
		t.Fatalf("create service: %v", err)
	}
	ticket := ticketModel{
		ServiceID:   service.ID,
		Status:      "pending",
		EngineType:  "smart",
		RequesterID: 7,
	}
	if err := db.Create(&ticket).Error; err != nil {
		t.Fatalf("create ticket: %v", err)
	}

	submitter := &txRecordingSubmitter{}
	eng := NewSmartEngine(availableDecisionExecutor{}, nil, nil, nil, submitter, nil)
	err := db.Transaction(func(tx *gorm.DB) error {
		return eng.Start(context.Background(), tx, StartParams{
			TicketID:    ticket.ID,
			RequesterID: ticket.RequesterID,
		})
	})
	if err != nil {
		t.Fatalf("start smart workflow: %v", err)
	}

	if submitter.txCalls != 0 {
		t.Fatalf("expected smart start to avoid scheduler submit calls, got %d", submitter.txCalls)
	}
}

func TestSmartCancelCallsEnsureContinuationButNoTask(t *testing.T) {
	db := newSmartContinuationDB(t)

	ticket, _ := createSmartContinuationTicket(t, db, "", ActivityPending)
	submitter := &txRecordingSubmitter{}
	eng := NewSmartEngine(availableDecisionExecutor{}, nil, nil, nil, submitter, nil)

	err := db.Transaction(func(tx *gorm.DB) error {
		return eng.Cancel(context.Background(), tx, CancelParams{
			TicketID:   ticket.ID,
			OperatorID: 1,
		})
	})
	if err != nil {
		t.Fatalf("cancel smart workflow: %v", err)
	}

	if submitter.txCalls != 0 {
		t.Fatalf("expected no tx submit calls for cancelled (terminal) ticket, got %d", submitter.txCalls)
	}
}

func TestSmartStartAIDisabledNoTask(t *testing.T) {
	db := newSmartContinuationDB(t)

	agentID := uint(11)
	service := serviceModel{
		Name:       "智能 VPN 服务",
		EngineType: "smart",
		AgentID:    &agentID,
	}
	if err := db.Create(&service).Error; err != nil {
		t.Fatalf("create service: %v", err)
	}
	ticket := ticketModel{
		ServiceID:      service.ID,
		Status:         "pending",
		EngineType:     "smart",
		RequesterID:    7,
		AIFailureCount: MaxAIFailureCount,
	}
	if err := db.Create(&ticket).Error; err != nil {
		t.Fatalf("create ticket: %v", err)
	}

	submitter := &txRecordingSubmitter{}
	eng := NewSmartEngine(availableDecisionExecutor{}, nil, nil, nil, submitter, nil)
	err := db.Transaction(func(tx *gorm.DB) error {
		return eng.Start(context.Background(), tx, StartParams{
			TicketID:    ticket.ID,
			RequesterID: ticket.RequesterID,
		})
	})
	if err != nil {
		t.Fatalf("start smart workflow: %v", err)
	}

	if submitter.txCalls != 0 {
		t.Fatalf("expected no tx submit calls when AI is circuit-broken, got %d", submitter.txCalls)
	}
}

// --- Task 6.4: Recovery dedup tests ---

func newSmartRecoveryDB(t *testing.T) *gorm.DB {
	t.Helper()
	db := newSmartContinuationDB(t)
	// HandleSmartRecovery queries "deleted_at IS NULL" which is not part of the
	// lightweight ticketModel. Add the column so the raw SQL does not fail.
	if err := db.Exec("ALTER TABLE itsm_tickets ADD COLUMN deleted_at datetime").Error; err != nil {
		t.Fatalf("add deleted_at column: %v", err)
	}
	return db
}

func clearRecoverySubmissions() {
	recoverySubmissionsMu.Lock()
	for k := range recoverySubmissions {
		delete(recoverySubmissions, k)
	}
	recoverySubmissionsMu.Unlock()
}

func TestSmartRecoveryFirstRunSubmits(t *testing.T) {
	clearRecoverySubmissions()

	db := newSmartRecoveryDB(t)

	// Create a decisioning smart ticket with no active activities.
	ticket := ticketModel{
		Status:     TicketStatusDecisioning,
		EngineType: "smart",
	}
	if err := db.Create(&ticket).Error; err != nil {
		t.Fatalf("create ticket: %v", err)
	}

	submitter := &regularRecordingSubmitter{}
	eng := NewSmartEngine(availableDecisionExecutor{}, nil, nil, nil, submitter, nil)

	handler := HandleSmartRecovery(db, eng)
	if err := handler(context.Background(), nil); err != nil {
		t.Fatalf("smart recovery handler: %v", err)
	}

	if submitter.calls != 0 {
		t.Fatalf("expected recovery to avoid scheduler submit calls, got %d", submitter.calls)
	}

	// Verify dedup map was populated
	recoverySubmissionsMu.Lock()
	_, recorded := recoverySubmissions[ticket.ID]
	recoverySubmissionsMu.Unlock()
	if !recorded {
		t.Fatal("expected recovery submission to be recorded in dedup map")
	}
}

func TestSmartRecoveryDedupSkipsRecent(t *testing.T) {
	clearRecoverySubmissions()

	db := newSmartRecoveryDB(t)

	ticket := ticketModel{
		Status:     TicketStatusDecisioning,
		EngineType: "smart",
	}
	if err := db.Create(&ticket).Error; err != nil {
		t.Fatalf("create ticket: %v", err)
	}

	submitter := &regularRecordingSubmitter{}
	eng := NewSmartEngine(availableDecisionExecutor{}, nil, nil, nil, submitter, nil)
	handler := HandleSmartRecovery(db, eng)

	// First run should dispatch direct recovery and populate dedup.
	if err := handler(context.Background(), nil); err != nil {
		t.Fatalf("first recovery run: %v", err)
	}
	if submitter.calls != 0 {
		t.Fatalf("expected no scheduler submit after first run, got %d", submitter.calls)
	}

	// Second run within 10 minutes — should skip (dedup)
	if err := handler(context.Background(), nil); err != nil {
		t.Fatalf("second recovery run: %v", err)
	}
	if submitter.calls != 0 {
		t.Fatalf("expected still no scheduler submit after second run, got %d", submitter.calls)
	}
}

func TestSmartRecoveryDedupExpiresAfter10Min(t *testing.T) {
	clearRecoverySubmissions()

	db := newSmartRecoveryDB(t)

	ticket := ticketModel{
		Status:     TicketStatusDecisioning,
		EngineType: "smart",
	}
	if err := db.Create(&ticket).Error; err != nil {
		t.Fatalf("create ticket: %v", err)
	}

	// Pre-populate the dedup map with an entry older than 10 minutes
	recoverySubmissionsMu.Lock()
	recoverySubmissions[ticket.ID] = time.Now().Add(-11 * time.Minute)
	recoverySubmissionsMu.Unlock()

	submitter := &regularRecordingSubmitter{}
	eng := NewSmartEngine(availableDecisionExecutor{}, nil, nil, nil, submitter, nil)
	handler := HandleSmartRecovery(db, eng)

	if err := handler(context.Background(), nil); err != nil {
		t.Fatalf("recovery after expiry: %v", err)
	}

	if submitter.calls != 0 {
		t.Fatalf("expected no scheduler submit after dedup entry expired, got %d", submitter.calls)
	}

	// Verify the dedup map was updated with a fresh timestamp
	recoverySubmissionsMu.Lock()
	ts, ok := recoverySubmissions[ticket.ID]
	recoverySubmissionsMu.Unlock()
	if !ok {
		t.Fatal("expected dedup entry to exist after resubmission")
	}
	if time.Since(ts) > 5*time.Second {
		t.Fatalf("expected fresh dedup timestamp, got %v ago", time.Since(ts))
	}
}
