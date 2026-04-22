package app

import (
	"context"
	"embed"
	"encoding/json"

	"github.com/casbin/casbin/v2"
	"github.com/gin-gonic/gin"
	"github.com/samber/do/v2"
	"gorm.io/gorm"

	"metis/internal/scheduler"
)

// App is the interface that pluggable modules must implement.
type App interface {
	Name() string
	Models() []any
	Seed(db *gorm.DB, enforcer *casbin.Enforcer, install bool) error
	Providers(i do.Injector)
	Routes(api *gin.RouterGroup)
	Tasks() []scheduler.TaskDef
}

// LocaleProvider is an optional interface an App can implement
// to supply additional locale JSON files for go-i18n.
type LocaleProvider interface {
	Locales() embed.FS
}

// OrgResolver is the unified interface implemented by the Org App.
// It provides all organisation-related queries consumed by:
//   - DataScopeMiddleware (department scope filtering)
//   - ITSM (multi-dimensional participant matching)
//   - AI tools (current_user_profile enrichment, org_context queries)
//
// When the Org App is not installed the resolver is nil and consumers
// handle the nil case gracefully.
type OrgResolver interface {
	// DataScope: department visibility
	GetUserDeptScope(userID uint, includeSubDepts bool) ([]uint, error)
	// ITSM: participant matching by IDs
	GetUserPositionIDs(userID uint) ([]uint, error)
	GetUserDepartmentIDs(userID uint) ([]uint, error)
	// AI tools: rich org info
	GetUserPositions(userID uint) ([]OrgPosition, error)
	GetUserDepartment(userID uint) (*OrgDepartment, error)
	QueryContext(username, deptCode, positionCode string, includeInactive bool) (*OrgContextResult, error)
	// Participant resolution: find users by org criteria
	FindUsersByPositionCode(posCode string) ([]uint, error)
	FindUsersByDepartmentCode(deptCode string) ([]uint, error)
	FindUsersByPositionAndDepartment(posCode, deptCode string) ([]uint, error)
	FindUsersByPositionID(positionID uint) ([]uint, error)
	FindUsersByDepartmentID(departmentID uint) ([]uint, error)
	FindManagerByUserID(userID uint) (uint, error)
}

// OrgDepartment represents a department in the organization.
type OrgDepartment struct {
	ID   uint   `json:"id"`
	Code string `json:"code"`
	Name string `json:"name"`
}

// OrgPosition represents a position held by a user.
type OrgPosition struct {
	ID        uint   `json:"id"`
	Code      string `json:"code"`
	Name      string `json:"name"`
	IsPrimary bool   `json:"is_primary"`
}

// OrgContextResult is the full result from an org context query.
type OrgContextResult struct {
	Users       []OrgContextUser       `json:"users"`
	Departments []OrgContextDepartment `json:"departments"`
	Positions   []OrgContextPosition   `json:"positions"`
	Summary     string                 `json:"summary"`
}

// OrgContextUser represents a user in the org context result.
type OrgContextUser struct {
	ID         uint           `json:"id"`
	Username   string         `json:"username"`
	Email      string         `json:"email"`
	Department *OrgDepartment `json:"department,omitempty"`
	Positions  []OrgPosition  `json:"positions,omitempty"`
	IsActive   bool           `json:"is_active"`
}

// OrgContextDepartment represents a department in the org context result.
type OrgContextDepartment struct {
	ID         uint   `json:"id"`
	Code       string `json:"code"`
	Name       string `json:"name"`
	ParentCode string `json:"parent_code,omitempty"`
	IsActive   bool   `json:"is_active"`
}

// OrgContextPosition represents a position in the org context result.
type OrgContextPosition struct {
	ID       uint   `json:"id"`
	Code     string `json:"code"`
	Name     string `json:"name"`
	IsActive bool   `json:"is_active"`
}

var apps []App

func Register(a App) { apps = append(apps, a) }
func All() []App     { return apps }

// AIAgentProvider is an optional interface implemented by the AI App.
// It provides agent configuration and LLM client creation for smart engines.
type AIAgentProvider interface {
	// GetAgentConfig returns agent configuration by ID (system prompt, model info, etc).
	GetAgentConfig(agentID uint) (*AIAgentConfig, error)
}

// AIAgentConfig holds agent configuration needed for LLM calls.
type AIAgentConfig struct {
	Name         string
	SystemPrompt string
	Temperature  float64
	MaxTokens    int
	Model        string // model identifier
	Protocol     string // "openai" or "anthropic"
	BaseURL      string
	APIKey       string
}

// AIKnowledgeSearcher is an optional interface implemented by the AI App.
// It searches knowledge bases for relevant context.
type AIKnowledgeSearcher interface {
	// SearchKnowledge searches the given knowledge bases for relevant content.
	SearchKnowledge(kbIDs []uint, query string, limit int) ([]AIKnowledgeResult, error)
}

// AIKnowledgeResult is a single result from knowledge search.
type AIKnowledgeResult struct {
	Title   string
	Content string
	Score   float64
}

// AIToolRegistry is an optional interface implemented by the AI App.
// It allows other apps to register tools for AI agents.
type AIToolRegistry interface {
	RegisterTool(toolkit, name, displayName, description string, parametersSchema string) (uint, error)
}

// ToolRegistryProvider is an optional interface an App can implement
// to provide a tool handler registry for AI agent tool dispatch.
// The returned value must satisfy ai.ToolHandlerRegistry.
type ToolRegistryProvider interface {
	GetToolRegistry() any
}

// AgentRuntimeContextProvider is an optional interface an App can implement
// to inject domain-specific session state into assistant agent runs.
type AgentRuntimeContextProvider interface {
	BuildAgentRuntimeContext(ctx context.Context, agentCode string, sessionID, userID uint) (string, error)
}

// AIDecisionExecutor runs AI decision cycles (ReAct tool-calling loops) for smart
// workflow engines. Implemented by the AI App; the engine provides domain context
// and tool handlers.
type AIDecisionExecutor interface {
	Execute(ctx context.Context, agentID uint, req AIDecisionRequest) (*AIDecisionResponse, error)
}

// AIToolDef defines a tool available during AI decision.
type AIToolDef struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Parameters  any    `json:"parameters"`
}

// AIDecisionRequest contains everything needed to run an AI decision cycle.
type AIDecisionRequest struct {
	SystemPrompt string
	UserMessage  string
	Tools        []AIToolDef
	ToolHandler  func(name string, args json.RawMessage) (json.RawMessage, error)
	MaxTurns     int // 0 = use default
}

// AIDecisionResponse contains the result of an AI decision cycle.
type AIDecisionResponse struct {
	Content      string
	InputTokens  int
	OutputTokens int
	Turns        int
}
