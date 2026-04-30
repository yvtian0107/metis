package desk

import (
	"context"
	"encoding/json"
	"io"
	. "metis/internal/app/itsm/config"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/samber/do/v2"
	"gorm.io/gorm"

	appcore "metis/internal/app"
	ai "metis/internal/app/ai/runtime"
	"metis/internal/app/itsm/definition"
	"metis/internal/app/itsm/engine"
	"metis/internal/app/itsm/sla"
	"metis/internal/app/itsm/testutil"
	"metis/internal/app/itsm/ticket"
	"metis/internal/app/itsm/tools"
	"metis/internal/database"
	"metis/internal/model"
	"metis/internal/pkg/crypto"
	"metis/internal/repository"
	"metis/internal/scheduler"
)

type blockingServiceDeskMessageStore struct {
	called  chan struct{}
	release chan struct{}
}

func (s *blockingServiceDeskMessageStore) StoreMessageContext(ctx context.Context, sessionID uint, role, content string, metadata []byte, tokenCount int) (*ai.SessionMessage, error) {
	close(s.called)
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-s.release:
		return &ai.SessionMessage{}, nil
	}
}

func newEngineConfigServiceOnly(t *testing.T, db *gorm.DB) *EngineConfigService {
	t.Helper()
	injector := do.New()
	do.ProvideValue(injector, &database.DB{DB: db})
	do.Provide(injector, repository.NewSysConfig)
	do.Provide(injector, ai.NewAgentRepo)
	do.Provide(injector, ai.NewAgentService)
	do.Provide(injector, ai.NewModelRepo)
	do.Provide(injector, ai.NewProviderRepo)
	do.ProvideValue(injector, crypto.EncryptionKey(crypto.DeriveKey("test-secret")))
	do.Provide(injector, ai.NewProviderService)
	do.Provide(injector, ai.NewToolRepo)
	do.Provide(injector, ai.NewToolRuntimeService)
	do.Provide(injector, NewEngineConfigService)
	return do.MustInvoke[*EngineConfigService](injector)
}

func newTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	return testutil.NewTestDB(t)
}

func newGinContext(method, path string) (*gin.Context, *httptest.ResponseRecorder) {
	return testutil.NewGinContext(method, path)
}

type deskTestSubmitter struct {
	db *gorm.DB
}

type deskTestDecisionExecutor struct{}

func (deskTestDecisionExecutor) Execute(context.Context, uint, appcore.AIDecisionRequest) (*appcore.AIDecisionResponse, error) {
	return nil, nil
}

func (s *deskTestSubmitter) SubmitTask(name string, payload json.RawMessage) error {
	return s.db.Create(&model.TaskExecution{
		TaskName: name,
		Trigger:  scheduler.TriggerAPI,
		Status:   scheduler.ExecPending,
		Payload:  string(payload),
	}).Error
}

func (s *deskTestSubmitter) SubmitTaskTx(tx *gorm.DB, name string, payload json.RawMessage) error {
	return tx.Create(&model.TaskExecution{
		TaskName: name,
		Trigger:  scheduler.TriggerAPI,
		Status:   scheduler.ExecPending,
		Payload:  string(payload),
	}).Error
}

func newSubmissionTicketService(t *testing.T, db *gorm.DB) *ticket.TicketService {
	t.Helper()
	injector := do.New()
	wrapped := &database.DB{DB: db}
	resolver := engine.NewParticipantResolver(nil)
	submitter := &deskTestSubmitter{db: db}
	do.ProvideValue(injector, wrapped)
	do.Provide(injector, ticket.NewTicketRepo)
	do.Provide(injector, ticket.NewTimelineRepo)
	do.Provide(injector, definition.NewServiceDefRepo)
	do.Provide(injector, sla.NewSLATemplateRepo)
	do.Provide(injector, sla.NewPriorityRepo)
	do.ProvideValue(injector, engine.NewClassicEngine(resolver, nil, nil))
	do.ProvideValue(injector, engine.NewSmartEngine(deskTestDecisionExecutor{}, nil, nil, resolver, submitter, nil))
	do.Provide(injector, ticket.NewTicketService)
	return do.MustInvoke[*ticket.TicketService](injector)
}

func configureIntakeAgent(t *testing.T, db *gorm.DB, agentID uint) {
	t.Helper()
	if err := db.Save(&model.SystemConfig{
		Key:   SmartTicketIntakeAgentKey,
		Value: strconv.FormatUint(uint64(agentID), 10),
	}).Error; err != nil {
		t.Fatalf("configure intake agent: %v", err)
	}
}

func TestServiceDeskSessionVerificationUsesConfiguredIntakeAgent(t *testing.T) {
	db := newTestDB(t)
	userID := uint(7)
	intake := ai.Agent{Name: "自定义服务受理岗", Type: ai.AgentTypeAssistant, IsActive: true, Visibility: "private", CreatedBy: 1}
	presetCode := "itsm.servicedesk"
	preset := ai.Agent{Name: "默认服务台预设", Code: &presetCode, Type: ai.AgentTypeAssistant, IsActive: true, Visibility: "private", CreatedBy: 1}
	if err := db.Create(&intake).Error; err != nil {
		t.Fatalf("create intake agent: %v", err)
	}
	if err := db.Create(&preset).Error; err != nil {
		t.Fatalf("create preset agent: %v", err)
	}
	configureIntakeAgent(t, db, intake.ID)

	intakeSession := ai.AgentSession{AgentID: intake.ID, UserID: userID, Status: "running"}
	presetSession := ai.AgentSession{AgentID: preset.ID, UserID: userID, Status: "running"}
	if err := db.Create(&intakeSession).Error; err != nil {
		t.Fatalf("create intake session: %v", err)
	}
	if err := db.Create(&presetSession).Error; err != nil {
		t.Fatalf("create preset session: %v", err)
	}

	handler := &ServiceDeskHandler{
		db:             db,
		configProvider: newEngineConfigServiceOnly(t, db),
		stateStore:     tools.NewSessionStateStore(db),
	}

	c, rec := newGinContext(http.MethodGet, "/state")
	c.Params = gin.Params{{Key: "sid", Value: strconv.FormatUint(uint64(intakeSession.ID), 10)}}
	c.Set("userId", userID)
	handler.State(c)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected configured intake session to pass state API, got %d body=%s", rec.Code, rec.Body.String())
	}

	c, rec = newGinContext(http.MethodPost, "/draft/submit")
	c.Params = gin.Params{{Key: "sid", Value: strconv.FormatUint(uint64(intakeSession.ID), 10)}}
	c.Set("userId", userID)
	handler.SubmitDraft(c)
	if rec.Code == http.StatusNotFound {
		t.Fatalf("expected draft API to pass dynamic intake session verification, got body=%s", rec.Body.String())
	}

	c, rec = newGinContext(http.MethodGet, "/state")
	c.Params = gin.Params{{Key: "sid", Value: strconv.FormatUint(uint64(presetSession.ID), 10)}}
	c.Set("userId", userID)
	handler.State(c)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected non-configured preset session to be rejected as 404, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestServiceDeskSessionVerificationRequiresConfiguredIntakeAgent(t *testing.T) {
	db := newTestDB(t)
	agent := ai.Agent{Name: "未上岗受理岗", Type: ai.AgentTypeAssistant, IsActive: true, Visibility: "private", CreatedBy: 1}
	if err := db.Create(&agent).Error; err != nil {
		t.Fatalf("create agent: %v", err)
	}
	session := ai.AgentSession{AgentID: agent.ID, UserID: 7, Status: "running"}
	if err := db.Create(&session).Error; err != nil {
		t.Fatalf("create session: %v", err)
	}

	handler := &ServiceDeskHandler{
		db:             db,
		configProvider: newEngineConfigServiceOnly(t, db),
		stateStore:     tools.NewSessionStateStore(db),
	}
	c, rec := newGinContext(http.MethodGet, "/state")
	c.Params = gin.Params{{Key: "sid", Value: strconv.FormatUint(uint64(session.ID), 10)}}
	c.Set("userId", uint(7))
	handler.State(c)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected missing intake config to return 400, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestServiceDeskSubmitDraftReturnsBeforeSubmittedSurfacePersistence(t *testing.T) {
	db := newTestDB(t)
	userID := uint(7)
	intake := ai.Agent{Name: "自定义服务受理岗", Type: ai.AgentTypeAssistant, IsActive: true, Visibility: "private", CreatedBy: 1}
	if err := db.Create(&intake).Error; err != nil {
		t.Fatalf("create intake agent: %v", err)
	}
	configureIntakeAgent(t, db, intake.ID)
	session := ai.AgentSession{AgentID: intake.ID, UserID: userID, Status: "running"}
	if err := db.Create(&session).Error; err != nil {
		t.Fatalf("create intake session: %v", err)
	}

	service := testutil.SeedSmartSubmissionService(t, db)
	ticketSvc := newSubmissionTicketService(t, db)
	operator := tools.NewOperator(db, nil, nil, nil, ticketSvc, nil)
	detail, err := operator.LoadService(service.ID)
	if err != nil {
		t.Fatalf("load service detail: %v", err)
	}
	stateStore := tools.NewSessionStateStore(db)
	if err := stateStore.SaveState(session.ID, &tools.ServiceDeskState{
		Stage:           "awaiting_confirmation",
		LoadedServiceID: service.ID,
		DraftSummary:    "VPN 开通申请",
		DraftFormData: map[string]any{
			"vpn_account":  "admin@dev.com",
			"device_usage": "线上支持用",
			"request_kind": "online_support",
		},
		DraftVersion: 1,
		FieldsHash:   detail.FieldsHash,
	}); err != nil {
		t.Fatalf("save draft state: %v", err)
	}

	messageStore := &blockingServiceDeskMessageStore{
		called:  make(chan struct{}),
		release: make(chan struct{}),
	}
	defer close(messageStore.release)
	handler := &ServiceDeskHandler{
		db:             db,
		configProvider: newEngineConfigServiceOnly(t, db),
		stateStore:     stateStore,
		operator:       operator,
		sessionSvc:     messageStore,
	}

	body := `{"draftVersion":1,"summary":"VPN 开通申请","formData":{"vpn_account":"admin@dev.com","device_usage":"线上支持用","request_kind":"online_support"}}`
	c, rec := newGinContext(http.MethodPost, "/draft/submit")
	c.Params = gin.Params{{Key: "sid", Value: strconv.FormatUint(uint64(session.ID), 10)}}
	c.Set("userId", userID)
	c.Request.Body = io.NopCloser(strings.NewReader(body))
	c.Request.ContentLength = int64(len(body))
	c.Request.Header.Set("Content-Type", "application/json")

	done := make(chan struct{})
	go func() {
		handler.SubmitDraft(c)
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(300 * time.Millisecond):
		t.Fatal("submit draft response was blocked by submitted surface persistence")
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected submit draft to return 200, got %d body=%s", rec.Code, rec.Body.String())
	}

	var resp struct {
		Code int                     `json:"code"`
		Data tools.DraftSubmitResult `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Code != 0 || !resp.Data.OK || resp.Data.TicketCode == "" {
		t.Fatalf("expected successful draft submit with ticket code, got %+v body=%s", resp, rec.Body.String())
	}
	select {
	case <-messageStore.called:
	case <-time.After(time.Second):
		t.Fatal("expected submitted surface persistence to run in background")
	}

	var created struct {
		Status string
	}
	if err := db.Table("itsm_tickets").Where("code = ?", resp.Data.TicketCode).Select("status").First(&created).Error; err != nil {
		t.Fatalf("load created ticket: %v", err)
	}
	if created.Status != engine.TicketStatusDecisioning {
		t.Fatalf("expected smart ticket to enter decisioning without scheduler task, got %q", created.Status)
	}
}

func TestITSMRuntimeContextUsesConfiguredIntakeAgentSession(t *testing.T) {
	db := newTestDB(t)
	userID := uint(7)
	intake := ai.Agent{Name: "无 Code 服务受理岗", Type: ai.AgentTypeAssistant, IsActive: true, Visibility: "private", CreatedBy: 1}
	otherCode := "itsm.servicedesk"
	other := ai.Agent{Name: "默认服务台预设", Code: &otherCode, Type: ai.AgentTypeAssistant, IsActive: true, Visibility: "private", CreatedBy: 1}
	if err := db.Create(&intake).Error; err != nil {
		t.Fatalf("create intake agent: %v", err)
	}
	if err := db.Create(&other).Error; err != nil {
		t.Fatalf("create other agent: %v", err)
	}
	configureIntakeAgent(t, db, intake.ID)

	intakeSession := ai.AgentSession{AgentID: intake.ID, UserID: userID, Status: "running"}
	otherSession := ai.AgentSession{AgentID: other.ID, UserID: userID, Status: "running"}
	if err := db.Create(&intakeSession).Error; err != nil {
		t.Fatalf("create intake session: %v", err)
	}
	if err := db.Create(&otherSession).Error; err != nil {
		t.Fatalf("create other session: %v", err)
	}
	store := tools.NewSessionStateStore(db)
	if err := store.SaveState(intakeSession.ID, &tools.ServiceDeskState{
		Stage:            "service_loaded",
		LoadedServiceID:  42,
		RequestText:      "我要提交 VPN 申请",
		MissingFields:    []string{"vpn_account"},
		AskedFields:      []string{"vpn_account"},
		MinDecisionReady: false,
	}); err != nil {
		t.Fatalf("save intake state: %v", err)
	}
	if err := store.SaveState(otherSession.ID, &tools.ServiceDeskState{
		Stage:           "service_loaded",
		LoadedServiceID: 99,
	}); err != nil {
		t.Fatalf("save other state: %v", err)
	}

	block, err := buildAgentRuntimeContextForTest(context.Background(), db, newEngineConfigServiceOnly(t, db), store, intakeSession.ID, userID)
	if err != nil {
		t.Fatalf("build runtime context: %v", err)
	}
	if !strings.Contains(block, "ITSM Service Desk Runtime Context") || !strings.Contains(block, `"loaded_service_id": 42`) {
		t.Fatalf("expected runtime context for configured intake session, got %q", block)
	}
	if !strings.Contains(block, `"missing_fields": [`) || !strings.Contains(block, `"asked_fields": [`) || !strings.Contains(block, `"min_decision_ready": false`) {
		t.Fatalf("expected runtime context to include conversation progress fields, got %q", block)
	}

	block, err = buildAgentRuntimeContextForTest(context.Background(), db, newEngineConfigServiceOnly(t, db), store, otherSession.ID, userID)
	if err != nil {
		t.Fatalf("build runtime context for other session: %v", err)
	}
	if block != "" {
		t.Fatalf("expected empty context for non-configured session, got %q", block)
	}
}

func buildAgentRuntimeContextForTest(ctx context.Context, db *gorm.DB, configProvider *EngineConfigService, store *tools.SessionStateStore, sessionID, userID uint) (string, error) {
	intakeAgentID := configProvider.IntakeAgentID()
	if intakeAgentID == 0 {
		return "", nil
	}
	var row struct {
		ID uint
	}
	if err := db.Table("ai_agent_sessions").
		Where("id = ? AND user_id = ? AND agent_id = ?", sessionID, userID, intakeAgentID).
		Select("id").
		First(&row).Error; err != nil {
		return "", nil
	}
	state, err := store.GetState(sessionID)
	if err != nil {
		return "", err
	}
	if state == nil || state.Stage == "idle" {
		return "", nil
	}
	payload := map[string]any{
		"stage":                   state.Stage,
		"candidate_service_ids":   state.CandidateServiceIDs,
		"top_match_service_id":    state.TopMatchServiceID,
		"confirmed_service_id":    state.ConfirmedServiceID,
		"confirmation_required":   state.ConfirmationRequired,
		"loaded_service_id":       state.LoadedServiceID,
		"request_text":            state.RequestText,
		"prefill_form_data":       state.PrefillFormData,
		"draft_summary":           state.DraftSummary,
		"draft_form_data":         state.DraftFormData,
		"draft_version":           state.DraftVersion,
		"confirmed_draft_version": state.ConfirmedDraftVersion,
		"missing_fields":          state.MissingFields,
		"asked_fields":            state.AskedFields,
		"min_decision_ready":      state.MinDecisionReady,
		"next_expected_action":    tools.NextExpectedAction(state),
	}
	b, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return "", err
	}
	return "## ITSM Service Desk Runtime Context\nUse this session state as current facts. Continue from next_expected_action unless the user explicitly starts a new request.\n```json\n" + string(b) + "\n```", nil
}
