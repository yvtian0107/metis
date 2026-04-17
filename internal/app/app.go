package app

import (
	"embed"

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

// OrgScopeResolver is an optional interface implemented by the Org App.
// It resolves the set of department IDs visible to a given user based on
// their organisational assignments. DataScopeMiddleware uses this interface;
// when the Org App is not installed the resolver is nil and no dept filtering
// is applied (equivalent to DataScopeAll).
type OrgScopeResolver interface {
	// GetUserDeptScope returns the department IDs the user can access.
	// For DataScopeDept it returns only the user's directly assigned departments.
	// For DataScopeDeptAndSub it returns those plus all active sub-departments (BFS).
	GetUserDeptScope(userID uint, includeSubDepts bool) ([]uint, error)
}

// OrgUserResolver is an optional interface implemented by the Org App.
// It resolves the user's position and department IDs for multi-dimensional
// participant matching (e.g. ITSM ticket assignment resolution).
type OrgUserResolver interface {
	GetUserPositionIDs(userID uint) ([]uint, error)
	GetUserDepartmentIDs(userID uint) ([]uint, error)
}

var apps []App

func Register(a App) { apps = append(apps, a) }
func All() []App     { return apps }

// AIAgentProvider is an optional interface implemented by the AI App.
// It provides agent configuration and LLM client creation for smart engines.
type AIAgentProvider interface {
	// GetAgentConfig returns agent configuration by ID (system prompt, model info, etc).
	GetAgentConfig(agentID uint) (*AIAgentConfig, error)
	// GetAgentConfigByCode returns agent configuration by code (e.g. "itsm.decision").
	GetAgentConfigByCode(code string) (*AIAgentConfig, error)
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
