package itsm

// steps_common_test.go — shared BDD test context, engine wiring, and common step definitions.
//
// bddContext holds the state shared across steps within a single scenario.
// Each scenario gets a fresh context via reset().

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/cucumber/godog"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"metis/internal/app/ai"
	"metis/internal/app/itsm/engine"
	"metis/internal/app/org"
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
	collaborationSpec string
	service           *ServiceDefinition
	priority          *Priority
	ticket            *Ticket
	tickets           map[string]*Ticket // multi-ticket scenarios, key = alias
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
		&ServiceAction{},
		&FormDefinition{},
		&Priority{},
		&SLATemplate{},
		&EscalationRule{},
		// ITSM — ticket lifecycle
		&Ticket{},
		&TicketActivity{},
		&TicketAssignment{},
		&TicketTimeline{},
		&TicketActionExecution{},
		// ITSM — incident
		&TicketLink{},
		&PostMortem{},
		// ITSM — process control
		&ProcessVariable{},
		&ExecutionToken{},
		// ITSM — knowledge
		&ServiceKnowledgeDocument{},
	); err != nil {
		panic(fmt.Sprintf("bdd: failed to migrate: %v", err))
	}

	bc.db = db

	// Build ClassicEngine with test dependencies.
	orgSvc := &testOrgService{db: db}
	resolver := engine.NewParticipantResolver(orgSvc)
	bc.engine = engine.NewClassicEngine(resolver, &noopSubmitter{}, nil)

	// Build SmartEngine with test dependencies.
	agentProvider := &testAgentProvider{db: db, llmCfg: bc.llmCfg}
	userProvider := &testUserProvider{db: db}
	bc.smartEngine = engine.NewSmartEngine(agentProvider, nil, userProvider, resolver, &noopSubmitter{})
}

// ---------------------------------------------------------------------------
// Test doubles for engine dependencies
// ---------------------------------------------------------------------------

// testOrgService implements engine.OrgService by querying the BDD in-memory DB.
type testOrgService struct {
	db *gorm.DB
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

func (s *testOrgService) FindUsersByPositionCodeAndDepartmentCode(positionCode, departmentCode string) ([]uint, error) {
	var pos org.Position
	if err := s.db.Where("code = ?", positionCode).First(&pos).Error; err != nil {
		return nil, fmt.Errorf("position code %q not found: %w", positionCode, err)
	}
	var dept org.Department
	if err := s.db.Where("code = ?", departmentCode).First(&dept).Error; err != nil {
		return nil, fmt.Errorf("department code %q not found: %w", departmentCode, err)
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

// Compile-time check.
var _ engine.OrgService = (*testOrgService)(nil)

// noopSubmitter implements engine.TaskSubmitter as a no-op.
type noopSubmitter struct{}

func (n *noopSubmitter) SubmitTask(_ string, _ json.RawMessage) error { return nil }

var _ engine.TaskSubmitter = (*noopSubmitter)(nil)

// testAgentProvider implements engine.AgentProvider for BDD tests.
// Reads Agent record from DB, combines with LLM config from env.
type testAgentProvider struct {
	db     *gorm.DB
	llmCfg llmConfig
}

func (p *testAgentProvider) GetAgentConfig(agentID uint) (*engine.SmartAgentConfig, error) {
	var agent ai.Agent
	if err := p.db.First(&agent, agentID).Error; err != nil {
		return nil, fmt.Errorf("agent %d not found: %w", agentID, err)
	}
	return &engine.SmartAgentConfig{
		Name:         agent.Name,
		SystemPrompt: agent.SystemPrompt,
		Temperature:  agent.Temperature,
		MaxTokens:    agent.MaxTokens,
		Protocol:     "openai",
		BaseURL:      p.llmCfg.baseURL,
		APIKey:       p.llmCfg.apiKey,
		Model:        p.llmCfg.model,
	}, nil
}

var _ engine.AgentProvider = (*testAgentProvider)(nil)

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
//	| 网络管理员审批人 | network-operator | it | network_admin |
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
}
