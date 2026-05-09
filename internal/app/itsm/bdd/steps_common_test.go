package bdd

// steps_common_test.go — shared BDD test context, engine wiring, and common step definitions.
//
// bddContext holds the state shared across steps within a single scenario.
// Each scenario gets a fresh context via reset().

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	. "metis/internal/app/itsm/domain"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"time"

	"github.com/cucumber/godog"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"metis/internal/app"
	ai "metis/internal/app/ai/runtime"
	"metis/internal/app/itsm/engine"
	org "metis/internal/app/org/domain"
	"metis/internal/llm"
	"metis/internal/model"
)

// bddContext is the shared state container for BDD scenarios.
type bddContext struct {
	db      *gorm.DB
	lastErr error

	// Engines
	engine      *engine.ClassicEngine
	smartEngine *engine.SmartEngine
	llmCfg      llmConfig // LLM connection config (from env, shared across scenarios)

	// Participants (populated by Given steps)
	users       map[string]*model.User     // key = identity label (e.g. "申请人")
	usersByName map[string]*model.User     // key = username
	positions   map[string]*org.Position   // key = position code
	departments map[string]*org.Department // key = department code

	// Ticket lifecycle (populated by When steps, asserted by Then steps)
	collaborationSpec   string
	service             *ServiceDefinition
	priority            *Priority
	ticket              *Ticket
	tickets             map[string]*Ticket // multi-ticket scenarios, key = alias
	lastCompletedUserID uint
	fallbackUserID      uint // fallback assignee for participant validation scenarios
	slaAssuranceAgentID uint
	toolCalls           []bddToolCall
	toolResults         []bddToolResult
	dialogState         dialogTestState           // dialog validation test state
	actionReceiver      *LocalActionReceiver      // action HTTP test receiver (nil if not needed)
	serviceActions      map[string]*ServiceAction // key = action code
	activityCountMark   int64
	actionRequestMark   int
}

type bddToolCall struct {
	Name      string
	Arguments string
}

type bddToolResult struct {
	Name    string
	Output  string
	IsError bool
}

func newBDDContext() *bddContext {
	return &bddContext{
		llmCfg: loadLLMConfig(),
	}
}

// loadLLMConfig reads LLM config from env vars (once per test suite).
func loadLLMConfig() llmConfig {
	return llmConfig{
		baseURL: os.Getenv("LLM_TEST_BASE_URL"),
		apiKey:  os.Getenv("LLM_TEST_API_KEY"),
		model:   os.Getenv("LLM_TEST_MODEL"),
	}
}

// reset clears all state for a new scenario. Called in sc.Before.
func (bc *bddContext) reset() {
	bc.lastErr = nil
	bc.collaborationSpec = ""
	bc.service = nil
	bc.priority = nil
	bc.ticket = nil
	bc.users = make(map[string]*model.User)
	bc.usersByName = make(map[string]*model.User)
	bc.positions = make(map[string]*org.Position)
	bc.departments = make(map[string]*org.Department)
	bc.tickets = make(map[string]*Ticket)
	bc.lastCompletedUserID = 0
	bc.fallbackUserID = 0
	bc.slaAssuranceAgentID = 0
	bc.toolCalls = nil
	bc.toolResults = nil
	bc.dialogState = dialogTestState{}
	bc.serviceActions = make(map[string]*ServiceAction)
	bc.activityCountMark = 0
	bc.actionRequestMark = 0
	if bc.actionReceiver != nil {
		bc.actionReceiver.Clear()
	}

	// Fresh in-memory database per scenario.
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:bdd_%p?mode=memory&cache=shared", bc)), &gorm.Config{})
	if err != nil {
		panic(fmt.Sprintf("bdd: failed to open test db: %v", err))
	}

	// AutoMigrate all models needed for BDD scenarios.
	if err := db.AutoMigrate(
		// Kernel
		&model.User{},
		&model.Role{},
		// Org
		&org.Department{},
		&org.Position{},
		&org.UserPosition{},
		// AI
		&ai.Agent{},
		&ai.AgentSession{},
		// ITSM — configuration
		&ServiceCatalog{},
		&ServiceDefinition{},
		&ServiceDefinitionVersion{},
		&ServiceAction{},
		&Priority{},
		&SLATemplate{},
		&EscalationRule{},
		// ITSM — ticket lifecycle
		&Ticket{},
		&TicketActivity{},
		&TicketAssignment{},
		&TicketTimeline{},
		&TicketActionExecution{},
		&ServiceDeskSubmission{},
		// ITSM — incident
		&TicketLink{},
		&PostMortem{},
		// ITSM — process control
		&ProcessVariable{},
		&ExecutionToken{},
		// ITSM — knowledge
		&ServiceKnowledgeDocument{},
		&model.TaskExecution{},
	); err != nil {
		panic(fmt.Sprintf("bdd: failed to migrate: %v", err))
	}

	bc.db = db

	// Build ClassicEngine with test dependencies.
	orgSvc := &testOrgService{db: db}
	resolver := engine.NewParticipantResolver(orgSvc)

	// Choose submitter: syncActionSubmitter when actionReceiver is active, noopSubmitter otherwise.
	var submitter engine.TaskSubmitter = &noopSubmitter{}
	bc.engine = engine.NewClassicEngine(resolver, submitter, nil)

	if bc.actionReceiver != nil {
		submitter = &syncActionSubmitter{db: db, classicEngine: bc.engine}
	}

	// Build SmartEngine with test dependencies.
	executor := &testDecisionExecutor{db: db, llmCfg: bc.llmCfg, recordToolCall: bc.recordToolCall, recordToolResult: bc.recordToolResult}
	userProvider := &testUserProvider{db: db}
	bc.smartEngine = engine.NewSmartEngine(executor, nil, userProvider, resolver, submitter, &bddConfigProvider{bc: bc})
}

// ---------------------------------------------------------------------------
// Test doubles for engine dependencies
// ---------------------------------------------------------------------------

// testOrgService implements app.OrgResolver by querying the BDD in-memory DB.
// Only participant resolution methods are exercised in tests; others return zero values.
type testOrgService struct {
	db *gorm.DB
}

func (s *testOrgService) GetUserDeptScope(_ uint, _ bool) ([]uint, error) { return nil, nil }
func (s *testOrgService) GetUserPositionIDs(_ uint) ([]uint, error)       { return nil, nil }
func (s *testOrgService) GetUserDepartmentIDs(_ uint) ([]uint, error)     { return nil, nil }
func (s *testOrgService) GetUserPositions(_ uint) ([]app.OrgPosition, error) {
	return nil, nil
}
func (s *testOrgService) GetUserDepartment(_ uint) (*app.OrgDepartment, error) {
	return nil, nil
}
func (s *testOrgService) QueryContext(_, _, _ string, _ bool) (*app.OrgContextResult, error) {
	return nil, nil
}

func (s *testOrgService) FindUsersByPositionCode(posCode string) ([]uint, error) {
	var pos org.Position
	if err := s.db.Where("code = ?", posCode).First(&pos).Error; err != nil {
		return nil, err
	}
	return s.FindUsersByPositionID(pos.ID)
}

func (s *testOrgService) FindUsersByDepartmentCode(deptCode string) ([]uint, error) {
	var dept org.Department
	if err := s.db.Where("code = ?", deptCode).First(&dept).Error; err != nil {
		return nil, err
	}
	return s.FindUsersByDepartmentID(dept.ID)
}

func (s *testOrgService) FindUsersByPositionAndDepartment(posCode, deptCode string) ([]uint, error) {
	var pos org.Position
	if err := s.db.Where("code = ?", posCode).First(&pos).Error; err != nil {
		return nil, fmt.Errorf("position code %q not found: %w", posCode, err)
	}
	var dept org.Department
	if err := s.db.Where("code = ?", deptCode).First(&dept).Error; err != nil {
		return nil, fmt.Errorf("department code %q not found: %w", deptCode, err)
	}
	var ups []org.UserPosition
	if err := s.db.Where("position_id = ? AND department_id = ?", pos.ID, dept.ID).Find(&ups).Error; err != nil {
		return nil, err
	}
	ids := make([]uint, 0, len(ups))
	for _, up := range ups {
		ids = append(ids, up.UserID)
	}
	return ids, nil
}

func (s *testOrgService) FindUsersByPositionID(positionID uint) ([]uint, error) {
	var ups []org.UserPosition
	if err := s.db.Where("position_id = ?", positionID).Find(&ups).Error; err != nil {
		return nil, err
	}
	ids := make([]uint, 0, len(ups))
	for _, up := range ups {
		ids = append(ids, up.UserID)
	}
	return ids, nil
}

func (s *testOrgService) FindUsersByDepartmentID(departmentID uint) ([]uint, error) {
	var ups []org.UserPosition
	if err := s.db.Where("department_id = ?", departmentID).Find(&ups).Error; err != nil {
		return nil, err
	}
	ids := make([]uint, 0, len(ups))
	for _, up := range ups {
		ids = append(ids, up.UserID)
	}
	return ids, nil
}

func (s *testOrgService) FindManagerByUserID(userID uint) (uint, error) {
	var user model.User
	if err := s.db.First(&user, userID).Error; err != nil {
		return 0, err
	}
	if user.ManagerID == nil {
		return 0, nil
	}
	return *user.ManagerID, nil
}

// Compile-time check.
var _ app.OrgResolver = (*testOrgService)(nil)

// noopSubmitter implements engine.TaskSubmitter as a no-op.
type noopSubmitter struct{}

func (n *noopSubmitter) SubmitTask(_ string, _ json.RawMessage) error { return nil }

var _ engine.TaskSubmitter = (*noopSubmitter)(nil)

// ---------------------------------------------------------------------------
// LocalActionReceiver — in-process HTTP server for testing action webhooks
// ---------------------------------------------------------------------------

// ActionRecord captures a single HTTP request received by LocalActionReceiver.
type ActionRecord struct {
	Path   string
	Method string
	Body   string
}

type actionResponder func(ActionRecord) (int, string)

// LocalActionReceiver is an in-process HTTP server that records incoming requests.
type LocalActionReceiver struct {
	server     *httptest.Server
	mu         sync.Mutex
	records    []ActionRecord
	responders map[string]actionResponder
}

// NewLocalActionReceiver creates and starts a new LocalActionReceiver.
func NewLocalActionReceiver() *LocalActionReceiver {
	r := &LocalActionReceiver{responders: make(map[string]actionResponder)}
	r.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		body, _ := io.ReadAll(io.LimitReader(req.Body, 64*1024))
		rec := ActionRecord{
			Path:   req.URL.Path,
			Method: req.Method,
			Body:   string(body),
		}
		r.mu.Lock()
		r.records = append(r.records, rec)
		responder := r.responders[rec.Path]
		r.mu.Unlock()

		status := http.StatusOK
		respBody := `{"status":"ok"}`
		if responder != nil {
			status, respBody = responder(rec)
			if status == 0 {
				status = http.StatusOK
			}
			if respBody == "" {
				respBody = `{"status":"ok"}`
			}
		}
		w.WriteHeader(status)
		w.Write([]byte(respBody))
	}))
	return r
}

func (r *LocalActionReceiver) Records() []ActionRecord {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]ActionRecord, len(r.records))
	copy(out, r.records)
	return out
}

func (r *LocalActionReceiver) RecordsByPath(path string) []ActionRecord {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []ActionRecord
	for _, rec := range r.records {
		if rec.Path == path {
			out = append(out, rec)
		}
	}
	return out
}

func (r *LocalActionReceiver) Clear() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.records = nil
	r.responders = make(map[string]actionResponder)
}

func (r *LocalActionReceiver) SetResponder(path string, responder actionResponder) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.responders[path] = responder
}

func (r *LocalActionReceiver) URL(path string) string {
	return r.server.URL + path
}

func (r *LocalActionReceiver) Close() {
	r.server.Close()
}

// ---------------------------------------------------------------------------
// syncActionSubmitter — synchronous action execution for BDD tests
// ---------------------------------------------------------------------------

// syncActionSubmitter implements engine.TaskSubmitter. On "itsm-action-execute",
// it synchronously runs ActionExecutor.Execute() + classicEngine.Progress(),
// matching production HandleActionExecute behavior. Other tasks are no-op.
type syncActionSubmitter struct {
	db            *gorm.DB
	classicEngine *engine.ClassicEngine
}

func (s *syncActionSubmitter) SubmitTask(taskName string, payload json.RawMessage) error {
	if taskName != "itsm-action-execute" {
		return nil // no-op for other tasks
	}

	var p engine.ActionExecutePayload
	if err := json.Unmarshal(payload, &p); err != nil {
		return fmt.Errorf("syncActionSubmitter: invalid payload: %w", err)
	}

	executor := engine.NewActionExecutor(s.db)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	err := executor.Execute(ctx, p.TicketID, p.ActivityID, p.ActionID)

	outcome := "success"
	if err != nil {
		outcome = "failed"
	}

	// Smart engine tickets have no execution tokens — directly mark activity completed.
	var ticket struct{ EngineType string }
	s.db.Table("itsm_tickets").Select("engine_type").Where("id = ?", p.TicketID).First(&ticket)
	if ticket.EngineType == "smart" {
		now := time.Now()
		return s.db.Table("itsm_ticket_activities").Where("id = ?", p.ActivityID).Updates(map[string]any{
			"status":             "completed",
			"transition_outcome": outcome,
			"finished_at":        now,
		}).Error
	}

	return s.classicEngine.Progress(ctx, s.db, engine.ProgressParams{
		TicketID:   p.TicketID,
		ActivityID: p.ActivityID,
		Outcome:    outcome,
		OperatorID: 0,
	})
}

var _ engine.TaskSubmitter = (*syncActionSubmitter)(nil)

// testDecisionExecutor implements app.AIDecisionExecutor for BDD tests.
// Reads Agent record from DB, combines with LLM config from env, and runs the ReAct loop.
type testDecisionExecutor struct {
	db               *gorm.DB
	llmCfg           llmConfig
	recordToolCall   func(name string, args json.RawMessage)
	recordToolResult func(name string, output json.RawMessage, isError bool)
}

func (e *testDecisionExecutor) Execute(ctx context.Context, agentID uint, req app.AIDecisionRequest) (*app.AIDecisionResponse, error) {
	// 1. Read agent from DB for system prompt / temperature / max_tokens.
	var agent ai.Agent
	if err := e.db.First(&agent, agentID).Error; err != nil {
		return nil, fmt.Errorf("agent %d not found: %w", agentID, err)
	}

	// 2. Create LLM client.
	client, err := llm.NewClient(llm.ProtocolOpenAI, e.llmCfg.baseURL, e.llmCfg.apiKey)
	if err != nil {
		return nil, fmt.Errorf("create llm client: %w", err)
	}

	// 3. Compose system prompt: agent's own + domain prompt from request.
	systemPrompt := ""
	if agent.SystemPrompt != "" {
		systemPrompt = "## 角色定义\n\n" + agent.SystemPrompt + "\n\n---\n\n"
	}
	systemPrompt += req.SystemPrompt

	// 4. Convert tool defs.
	toolDefs := make([]llm.ToolDef, len(req.Tools))
	for i, t := range req.Tools {
		toolDefs[i] = llm.ToolDef{
			Name:        t.Name,
			Description: t.Description,
			Parameters:  t.Parameters,
		}
	}

	// 5. Build messages.
	messages := []llm.Message{
		{Role: llm.RoleSystem, Content: systemPrompt},
		{Role: llm.RoleUser, Content: req.UserMessage},
	}

	temp := float32(agent.Temperature)
	tempPtr := &temp
	maxTokens := agent.MaxTokens
	if maxTokens <= 0 {
		maxTokens = 4096
	}

	maxTurns := req.MaxTurns
	if maxTurns <= 0 {
		maxTurns = 8
	}

	// 6. ReAct loop: chat → tool calls → tool results → repeat.
	var totalInputTokens, totalOutputTokens int
	for turn := 0; turn < maxTurns; turn++ {
		chatReq := llm.ChatRequest{
			Model:       e.llmCfg.model,
			Messages:    messages,
			Tools:       toolDefs,
			MaxTokens:   maxTokens,
			Temperature: tempPtr,
		}

		resp, err := client.Chat(ctx, chatReq)
		if err != nil {
			return nil, fmt.Errorf("llm chat (turn %d): %w", turn, err)
		}

		totalInputTokens += resp.Usage.InputTokens
		totalOutputTokens += resp.Usage.OutputTokens

		// No tool calls → final answer.
		if len(resp.ToolCalls) == 0 {
			return &app.AIDecisionResponse{
				Content:      resp.Content,
				InputTokens:  totalInputTokens,
				OutputTokens: totalOutputTokens,
				Turns:        turn + 1,
			}, nil
		}

		// Append assistant message with tool calls.
		messages = append(messages, llm.Message{
			Role:      llm.RoleAssistant,
			Content:   resp.Content,
			ToolCalls: resp.ToolCalls,
		})

		// Execute tool calls via the provided handler.
		for _, tc := range resp.ToolCalls {
			if e.recordToolCall != nil {
				e.recordToolCall(tc.Name, json.RawMessage(tc.Arguments))
			}
			result, err := req.ToolHandler(tc.Name, json.RawMessage(tc.Arguments))
			var content string
			var resultPayload json.RawMessage
			isError := false
			if err != nil {
				content = fmt.Sprintf(`{"error": "%s"}`, err.Error())
				resultPayload = json.RawMessage(content)
				isError = true
			} else {
				content = string(result)
				resultPayload = result
			}
			if e.recordToolResult != nil {
				e.recordToolResult(tc.Name, resultPayload, isError)
			}
			messages = append(messages, llm.Message{
				Role:       llm.RoleTool,
				Content:    content,
				ToolCallID: tc.ID,
			})
		}
	}

	return nil, fmt.Errorf("决策循环超过最大轮数 (%d)，未产生最终决策", maxTurns)
}

var _ app.AIDecisionExecutor = (*testDecisionExecutor)(nil)

func (bc *bddContext) recordToolCall(name string, args json.RawMessage) {
	bc.toolCalls = append(bc.toolCalls, bddToolCall{Name: name, Arguments: string(args)})
}

func (bc *bddContext) recordToolResult(name string, output json.RawMessage, isError bool) {
	bc.toolResults = append(bc.toolResults, bddToolResult{Name: name, Output: string(output), IsError: isError})
}

func (bc *bddContext) hasToolCall(name string) bool {
	for _, call := range bc.toolCalls {
		if call.Name == name {
			return true
		}
	}
	return false
}

// testUserProvider implements engine.UserProvider for BDD tests.
// Queries all active users with their position/department info from the BDD in-memory DB.
type testUserProvider struct {
	db *gorm.DB
}

func (p *testUserProvider) ListActiveUsers() ([]engine.ParticipantCandidate, error) {
	var users []model.User
	if err := p.db.Where("is_active = ?", true).Find(&users).Error; err != nil {
		return nil, err
	}

	candidates := make([]engine.ParticipantCandidate, 0, len(users))
	for _, u := range users {
		c := engine.ParticipantCandidate{
			UserID: u.ID,
			Name:   u.Username,
		}
		// Try to find UserPosition + Position + Department
		var up org.UserPosition
		if err := p.db.Where("user_id = ?", u.ID).First(&up).Error; err == nil {
			var pos org.Position
			if err := p.db.First(&pos, up.PositionID).Error; err == nil {
				c.Position = pos.Code
			}
			var dept org.Department
			if err := p.db.First(&dept, up.DepartmentID).Error; err == nil {
				c.Department = dept.Code
			}
		}
		candidates = append(candidates, c)
	}
	return candidates, nil
}

var _ engine.UserProvider = (*testUserProvider)(nil)

// bddConfigProvider implements engine.EngineConfigProvider for BDD tests.
// Dynamically reads the agent ID from bddContext.service (set after service publish).
type bddConfigProvider struct {
	bc *bddContext
}

func (p *bddConfigProvider) FallbackAssigneeID() uint {
	return p.bc.fallbackUserID
}

func (p *bddConfigProvider) DecisionMode() string {
	return "" // default
}

func (p *bddConfigProvider) DecisionAgentID() uint {
	if p.bc.service != nil && p.bc.service.AgentID != nil {
		return *p.bc.service.AgentID
	}
	return 0
}

func (p *bddConfigProvider) SLAAssuranceAgentID() uint {
	return p.bc.slaAssuranceAgentID
}

func (p *bddConfigProvider) AuditLevel() string {
	return "full"
}

func (p *bddConfigProvider) SLACriticalThresholdSeconds() int {
	return 1800
}

func (p *bddConfigProvider) SLAWarningThresholdSeconds() int {
	return 3600
}

func (p *bddConfigProvider) SimilarHistoryLimit() int {
	return 5
}

func (p *bddConfigProvider) ParallelConvergenceTimeout() time.Duration { return 72 * time.Hour }

var _ engine.EngineConfigProvider = (*bddConfigProvider)(nil)

// ---------------------------------------------------------------------------
// Common step definitions
// ---------------------------------------------------------------------------

// givenSystemInitialized is a no-op — reset() already handles initialization.
func (bc *bddContext) givenSystemInitialized() error {
	return nil
}

// givenParticipants parses a DataTable and creates User/Department/Position/UserPosition records.
//
// Expected DataTable format:
//
//	| 身份 | 用户名 | 部门 | 岗位 |
//	| 申请人 | vpn-requester | - | - |
//	| 网络管理员处理人 | network-operator | it | network_admin |
func (bc *bddContext) givenParticipants(table *godog.Table) error {
	if len(table.Rows) < 2 {
		return fmt.Errorf("participants table must have a header row and at least one data row")
	}

	// Parse header to find column indices.
	header := table.Rows[0]
	colIdx := make(map[string]int)
	for i, cell := range header.Cells {
		colIdx[cell.Value] = i
	}
	for _, required := range []string{"身份", "用户名", "部门", "岗位"} {
		if _, ok := colIdx[required]; !ok {
			return fmt.Errorf("participants table missing required column: %s", required)
		}
	}

	for _, row := range table.Rows[1:] {
		identity := row.Cells[colIdx["身份"]].Value
		username := row.Cells[colIdx["用户名"]].Value
		deptCode := row.Cells[colIdx["部门"]].Value
		posCode := row.Cells[colIdx["岗位"]].Value

		// Create or get User.
		user := &model.User{Username: username, IsActive: true}
		if err := bc.db.Where("username = ?", username).FirstOrCreate(user).Error; err != nil {
			return fmt.Errorf("create user %q: %w", username, err)
		}
		bc.users[identity] = user
		bc.usersByName[username] = user

		// Create Department if not "-".
		var dept *org.Department
		if deptCode != "-" && deptCode != "" {
			dept = &org.Department{Code: deptCode, Name: deptCode, IsActive: true}
			if err := bc.db.Where("code = ?", deptCode).FirstOrCreate(dept).Error; err != nil {
				return fmt.Errorf("create department %q: %w", deptCode, err)
			}
			bc.departments[deptCode] = dept
		}

		// Create Position + UserPosition if not "-".
		if posCode != "-" && posCode != "" {
			pos := &org.Position{Code: posCode, Name: posCode, IsActive: true}
			if err := bc.db.Where("code = ?", posCode).FirstOrCreate(pos).Error; err != nil {
				return fmt.Errorf("create position %q: %w", posCode, err)
			}
			bc.positions[posCode] = pos

			// UserPosition requires a department.
			if dept == nil {
				return fmt.Errorf("position %q specified without a department for user %q", posCode, username)
			}
			up := &org.UserPosition{
				UserID:       user.ID,
				DepartmentID: dept.ID,
				PositionID:   pos.ID,
				IsPrimary:    true,
			}
			if err := bc.db.Where("user_id = ? AND department_id = ?", user.ID, dept.ID).
				FirstOrCreate(up).Error; err != nil {
				return fmt.Errorf("create user_position for %q: %w", username, err)
			}
		}
	}

	return nil
}

// givenVPNCollaborationSpec stores the VPN collaboration spec in bddContext.
func (bc *bddContext) givenVPNCollaborationSpec() error {
	bc.collaborationSpec = vpnCollaborationSpec
	return nil
}

// thenTicketStatusIs refreshes the ticket from DB and asserts its status.
func (bc *bddContext) thenTicketStatusIs(expected string) error {
	if bc.ticket == nil {
		return fmt.Errorf("no ticket in context")
	}
	if err := bc.db.First(bc.ticket, bc.ticket.ID).Error; err != nil {
		return fmt.Errorf("refresh ticket: %w", err)
	}
	if bc.ticket.Status != expected {
		return fmt.Errorf("expected ticket status %q, got %q", expected, bc.ticket.Status)
	}
	return nil
}

// thenTicketStatusIsNot refreshes the ticket from DB and asserts its status is not the given value.
func (bc *bddContext) thenTicketStatusIsNot(notExpected string) error {
	if bc.ticket == nil {
		return fmt.Errorf("no ticket in context")
	}
	if err := bc.db.First(bc.ticket, bc.ticket.ID).Error; err != nil {
		return fmt.Errorf("refresh ticket: %w", err)
	}
	if bc.ticket.Status == notExpected {
		return fmt.Errorf("expected ticket status NOT to be %q, but it is", notExpected)
	}
	return nil
}

// registerCommonSteps registers all shared step definitions.
func registerCommonSteps(sc *godog.ScenarioContext, bc *bddContext) {
	sc.Given(`^已完成系统初始化$`, bc.givenSystemInitialized)
	sc.Given(`^已准备好以下参与人、岗位与职责$`, bc.givenParticipants)
	sc.Given(`^已定义 VPN 开通申请协作规范$`, bc.givenVPNCollaborationSpec)
	sc.Then(`^工单状态为 "([^"]*)"$`, bc.thenTicketStatusIs)
	sc.Then(`^工单状态不为 "([^"]*)"$`, bc.thenTicketStatusIsNot)
	sc.Then(`^当前处理任务分配到岗位 "([^"]*)"$`, bc.thenCurrentProcessAssignedToPosition)
	sc.Then(`^当前处理任务仅对 "([^"]*)" 可见$`, bc.thenCurrentProcessOnlyVisibleTo)
	sc.Then(`^"([^"]*)" 认领当前工单应失败$`, bc.thenClaimShouldFail)
	sc.Then(`^"([^"]*)" 处理当前工单应失败$`, bc.thenProcessShouldFail)

	sc.When(`^智能引擎执行决策循环直到工单完成$`, bc.whenSmartEngineDecisionCycleUntilComplete)
}

// ---------------------------------------------------------------------------
// Responsibility boundary step definitions
// ---------------------------------------------------------------------------

// thenCurrentProcessAssignedToPosition asserts the current activity's assignment
// targets the expected position — either via PositionID on the assignment,
// or via the assigned user belonging to that position+department.
func (bc *bddContext) thenCurrentProcessAssignedToPosition(positionCode string) error {
	activity, err := bc.getCurrentActivity()
	if err != nil {
		return err
	}

	var assignments []TicketAssignment
	if err := bc.db.Where("activity_id = ?", activity.ID).Find(&assignments).Error; err != nil {
		return fmt.Errorf("query assignments for activity %d: %w", activity.ID, err)
	}
	if len(assignments) == 0 {
		return fmt.Errorf("no assignments found for activity %d", activity.ID)
	}

	orgSvc := &testOrgService{db: bc.db}

	for _, assignment := range assignments {
		// Check 1: direct PositionID match.
		if assignment.PositionID != nil {
			for code, pos := range bc.positions {
				if pos.ID == *assignment.PositionID && code == positionCode {
					return nil
				}
			}
		}

		// Check 2: assigned user belongs to the expected position (for smart engine).
		var userID uint
		if assignment.AssigneeID != nil {
			userID = *assignment.AssigneeID
		} else if assignment.UserID != nil {
			userID = *assignment.UserID
		}
		if userID > 0 {
			// Find what department code is associated with this position code.
			for deptCode := range bc.departments {
				eligibleIDs, err := orgSvc.FindUsersByPositionAndDepartment(positionCode, deptCode)
				if err != nil {
					continue
				}
				for _, uid := range eligibleIDs {
					if uid == userID {
						return nil
					}
				}
			}
		}
	}

	return fmt.Errorf("no assignment for activity %d targets position %q", activity.ID, positionCode)
}

// thenCurrentProcessOnlyVisibleTo asserts that only the specified user is the assignee
// or is eligible for the current activity's assignment.
func (bc *bddContext) thenCurrentProcessOnlyVisibleTo(username string) error {
	expectedUser, ok := bc.usersByName[username]
	if !ok {
		return fmt.Errorf("user %q not found in context", username)
	}

	activity, err := bc.getCurrentActivity()
	if err != nil {
		return err
	}

	var assignments []TicketAssignment
	if err := bc.db.Where("activity_id = ?", activity.ID).Find(&assignments).Error; err != nil {
		return fmt.Errorf("query assignments for activity %d: %w", activity.ID, err)
	}
	if len(assignments) == 0 {
		// No assignment record — check AI decision for participant_id referencing the expected user.
		if len(activity.AIDecision) > 0 {
			var plan engine.DecisionPlan
			if err := json.Unmarshal([]byte(activity.AIDecision), &plan); err == nil {
				for _, da := range plan.Activities {
					if da.ParticipantID != nil && *da.ParticipantID == expectedUser.ID {
						return nil // AI intended this user
					}
				}
			}
		}
		return fmt.Errorf("no assignments found for activity %d", activity.ID)
	}

	orgSvc := &testOrgService{db: bc.db}

	for _, assignment := range assignments {
		// Case 1: assignment has PositionID+DepartmentID — check via org resolution.
		if assignment.PositionID != nil && assignment.DepartmentID != nil {
			var posCode, deptCode string
			for code, pos := range bc.positions {
				if pos.ID == *assignment.PositionID {
					posCode = code
					break
				}
			}
			for code, dept := range bc.departments {
				if dept.ID == *assignment.DepartmentID {
					deptCode = code
					break
				}
			}

			eligibleIDs, err := orgSvc.FindUsersByPositionAndDepartment(posCode, deptCode)
			if err != nil {
				return fmt.Errorf("resolve eligible users for %s/%s: %w", deptCode, posCode, err)
			}

			found := false
			for _, uid := range eligibleIDs {
				if uid == expectedUser.ID {
					found = true
					break
				}
			}
			if !found {
				return fmt.Errorf("expected user %q is NOT eligible for %s/%s", username, deptCode, posCode)
			}

			for otherName, otherUser := range bc.usersByName {
				if otherName == username {
					continue
				}
				for _, uid := range eligibleIDs {
					if uid == otherUser.ID {
						return fmt.Errorf("user %q should NOT be eligible for %s/%s, but is", otherName, deptCode, posCode)
					}
				}
			}
			return nil
		}

		// Case 2: assignment has direct UserID or AssigneeID — check it matches expected user.
		var assignedUserID uint
		if assignment.AssigneeID != nil {
			assignedUserID = *assignment.AssigneeID
		} else if assignment.UserID != nil {
			assignedUserID = *assignment.UserID
		}
		if assignedUserID > 0 {
			if assignedUserID != expectedUser.ID {
				return fmt.Errorf("assignment is for user ID %d, not %q (ID %d)", assignedUserID, username, expectedUser.ID)
			}
			return nil
		}
	}

	return fmt.Errorf("could not determine visibility for assignments on activity %d", activity.ID)
}

// thenClaimShouldFail asserts that the specified user cannot claim the current activity's assignment
// because they are not the assigned user or not in the assignment's position+department.
func (bc *bddContext) thenClaimShouldFail(username string) error {
	user, ok := bc.usersByName[username]
	if !ok {
		return fmt.Errorf("user %q not found in context", username)
	}

	activity, err := bc.getCurrentActivity()
	if err != nil {
		return err
	}

	var assignments []TicketAssignment
	if err := bc.db.Where("activity_id = ?", activity.ID).Find(&assignments).Error; err != nil {
		return fmt.Errorf("query assignments for activity %d: %w", activity.ID, err)
	}

	orgSvc := &testOrgService{db: bc.db}

	for _, assignment := range assignments {
		// Check position+department eligibility.
		if assignment.PositionID != nil && assignment.DepartmentID != nil {
			var posCode, deptCode string
			for code, pos := range bc.positions {
				if pos.ID == *assignment.PositionID {
					posCode = code
					break
				}
			}
			for code, dept := range bc.departments {
				if dept.ID == *assignment.DepartmentID {
					deptCode = code
					break
				}
			}

			eligibleIDs, _ := orgSvc.FindUsersByPositionAndDepartment(posCode, deptCode)
			for _, uid := range eligibleIDs {
				if uid == user.ID {
					return fmt.Errorf("user %q IS eligible for %s/%s — claim should not fail", username, deptCode, posCode)
				}
			}
			continue
		}

		// Check direct user assignment.
		var assignedUserID uint
		if assignment.AssigneeID != nil {
			assignedUserID = *assignment.AssigneeID
		} else if assignment.UserID != nil {
			assignedUserID = *assignment.UserID
		}
		if assignedUserID > 0 && assignedUserID == user.ID {
			return fmt.Errorf("user %q IS directly assigned — claim should not fail", username)
		}
	}

	// User is not eligible — assertion passes.
	return nil
}

// thenProcessShouldFail asserts that the specified user cannot directly process the current activity
// because they are not in the assignment's position+department.
func (bc *bddContext) thenProcessShouldFail(username string) error {
	// Same eligibility check — if user can't claim, they can't process either.
	return bc.thenClaimShouldFail(username)
}

// ---------------------------------------------------------------------------
// Shared smart engine retry step
// ---------------------------------------------------------------------------

// whenSmartEngineDecisionCycleUntilComplete runs the decision cycle up to 8 times,
// stopping early once the ticket reaches a terminal state ("completed", "cancelled", "failed").
// Between attempts it auto-completes any pending/in_progress activities the LLM may have
// created, clearing the path for a completion decision on the next cycle.
func (bc *bddContext) whenSmartEngineDecisionCycleUntilComplete() error {
	if bc.ticket == nil {
		return fmt.Errorf("no ticket in context")
	}

	const maxAttempts = 8
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		// Refresh ticket status.
		bc.db.First(bc.ticket, bc.ticket.ID)
		if isTerminal(bc.ticket.Status) {
			return nil
		}

		// Between retries (attempt > 1): auto-complete any pending activities the LLM created
		// on previous cycles, so they don't block the completion decision.
		if attempt > 1 {
			bc.autoProcessBlockingActivities()
		}

		ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
		var completedID *uint
		var lastCompleted TicketActivity
		if err := bc.db.Where("ticket_id = ? AND status IN ?", bc.ticket.ID, engine.CompletedActivityStatuses()).
			Order("id DESC").First(&lastCompleted).Error; err == nil {
			completedID = &lastCompleted.ID
		}

		err := bc.smartEngine.RunDecisionCycleForTicket(ctx, bc.db, bc.ticket.ID, completedID)
		cancel()

		if err != nil {
			if err == engine.ErrAIDecisionFailed || err == engine.ErrAIDisabled {
				log.Printf("decision cycle attempt %d/%d: %v", attempt, maxAttempts, err)
				continue
			}
			log.Printf("decision cycle attempt %d/%d failed: %v", attempt, maxAttempts, err)
		}

		bc.db.First(bc.ticket, bc.ticket.ID)
		if isTerminal(bc.ticket.Status) {
			return nil
		}
	}

	bc.db.First(bc.ticket, bc.ticket.ID)
	return nil // let the Then step assert the final status
}

func isTerminal(status string) bool {
	return engine.IsTerminalTicketStatus(status)
}

// autoProcessBlockingActivities finds any pending/in_progress activities (non-parallel)
// for the current ticket and auto-completes them via SmartEngine.Progress so the next
// decision cycle can produce a "complete" decision.
func (bc *bddContext) autoProcessBlockingActivities() {
	var activities []TicketActivity
	bc.db.Where("ticket_id = ? AND status IN ?",
		bc.ticket.ID, []string{"pending", "in_progress"}).
		Find(&activities)

	for _, act := range activities {
		bc.autoProcessSingleActivity(act)
	}
}

// autoProcessSingleActivity completes a single pending/in_progress activity.
func (bc *bddContext) autoProcessSingleActivity(act TicketActivity) {
	// Determine operator from assignment.
	var assignment TicketAssignment
	if err := bc.db.Where("activity_id = ?", act.ID).First(&assignment).Error; err != nil {
		// No assignment — create one with the requester as fallback.
		fallbackID := bc.ticket.RequesterID
		provider := &testUserProvider{db: bc.db}
		candidates, _ := provider.ListActiveUsers()
		for _, c := range candidates {
			if c.UserID != bc.ticket.RequesterID {
				fallbackID = c.UserID
				break
			}
		}
		bc.db.Create(&TicketAssignment{
			TicketID:        bc.ticket.ID,
			ActivityID:      act.ID,
			ParticipantType: "user",
			UserID:          &fallbackID,
			AssigneeID:      &fallbackID,
			Status:          "claimed",
			IsCurrent:       true,
		})
		assignment.AssigneeID = &fallbackID
	}

	var operatorID uint
	if assignment.AssigneeID != nil {
		operatorID = *assignment.AssigneeID
	} else if assignment.UserID != nil {
		operatorID = *assignment.UserID
	} else {
		// Resolve from position+department.
		operatorID = bc.resolveOperatorFromAssignment(assignment)
	}
	if operatorID == 0 {
		return
	}

	// Claim if not yet claimed.
	bc.db.Model(&TicketAssignment{}).
		Where("activity_id = ?", act.ID).
		Updates(map[string]any{"assignee_id": operatorID, "status": "claimed"})

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	err := bc.smartEngine.Progress(ctx, bc.db, engine.ProgressParams{
		TicketID:   bc.ticket.ID,
		ActivityID: act.ID,
		Outcome:    "completed",
		OperatorID: operatorID,
	})
	cancel()
	if err != nil {
		log.Printf("[auto-complete] activity %d: %v", act.ID, err)
	}
}

// resolveOperatorFromAssignment attempts to resolve an operator user ID from position/department in assignment.
func (bc *bddContext) resolveOperatorFromAssignment(assignment TicketAssignment) uint {
	if assignment.PositionID == nil || assignment.DepartmentID == nil {
		return 0
	}
	orgSvc := &testOrgService{db: bc.db}
	for posCode, p := range bc.positions {
		if p.ID == *assignment.PositionID {
			for deptCode, d := range bc.departments {
				if d.ID == *assignment.DepartmentID {
					userIDs, _ := orgSvc.FindUsersByPositionAndDepartment(posCode, deptCode)
					if len(userIDs) > 0 {
						return userIDs[0]
					}
				}
			}
		}
	}
	return 0
}
