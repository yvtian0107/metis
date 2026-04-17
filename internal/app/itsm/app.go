package itsm

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/casbin/casbin/v2"
	"github.com/gin-gonic/gin"
	"github.com/samber/do/v2"
	"gorm.io/gorm"

	"metis/internal/app"
	"metis/internal/app/itsm/engine"
	"metis/internal/app/itsm/tools"
	"metis/internal/database"
	"metis/internal/model"
	"metis/internal/repository"
	"metis/internal/scheduler"
	"metis/internal/service"
)

func init() {
	app.Register(&ITSMApp{})
}

type ITSMApp struct {
	injector do.Injector
}

func (a *ITSMApp) Name() string { return "itsm" }

// GetToolRegistry implements app.ToolRegistryProvider.
func (a *ITSMApp) GetToolRegistry() any {
	return do.MustInvoke[*tools.Registry](a.injector)
}

func (a *ITSMApp) Models() []any {
	return []any{
		// Configuration models
		&ServiceCatalog{},
		&ServiceDefinition{},
		&ServiceAction{},
		&FormDefinition{},
		&Priority{},
		&SLATemplate{},
		&EscalationRule{},
		// Ticket lifecycle models
		&Ticket{},
		&TicketActivity{},
		&TicketAssignment{},
		&TicketTimeline{},
		&TicketActionExecution{},
		// Incident models
		&TicketLink{},
		&PostMortem{},
		// Process variables
		&ProcessVariable{},
		// Execution tokens
		&ExecutionToken{},
		// Knowledge documents
		&ServiceKnowledgeDocument{},
	}
}

func (a *ITSMApp) Seed(db *gorm.DB, enforcer *casbin.Enforcer, _ bool) error {
	return seedITSM(db, enforcer)
}

func (a *ITSMApp) Providers(i do.Injector) {
	a.injector = i
	// Repositories
	do.Provide(i, NewCatalogRepo)
	do.Provide(i, NewServiceDefRepo)
	do.Provide(i, NewServiceActionRepo)
	do.Provide(i, NewFormDefRepo)
	do.Provide(i, NewPriorityRepo)
	do.Provide(i, NewSLATemplateRepo)
	do.Provide(i, NewEscalationRuleRepo)
	do.Provide(i, NewTicketRepo)
	do.Provide(i, NewTimelineRepo)
	do.Provide(i, NewVariableRepository)
	do.Provide(i, NewTokenRepository)

	// Engine components
	do.Provide(i, func(i do.Injector) (*engine.ParticipantResolver, error) {
		// Try to resolve OrgService (optional — nil if Org App not installed)
		var orgSvc engine.OrgService
		// Org App provides OrgScopeResolver; we don't have a direct OrgService interface yet,
		// so for now the resolver starts with nil (user type and requester_manager basic support)
		return engine.NewParticipantResolver(orgSvc), nil
	})

	do.Provide(i, func(i do.Injector) (*engine.ClassicEngine, error) {
		resolver := do.MustInvoke[*engine.ParticipantResolver](i)
		db := do.MustInvoke[*database.DB](i)
		// Create a TaskSubmitter that uses the scheduler engine
		submitter := &schedulerSubmitter{db: db.DB}
		// Try to resolve NotificationSender (optional — nil if MessageChannel service not available)
		var notifier engine.NotificationSender
		if mcSvc, err := do.Invoke[*service.MessageChannelService](i); err == nil && mcSvc != nil {
			notifier = &notificationAdapter{svc: mcSvc}
			slog.Info("ITSM ClassicEngine: notification sender available")
		}
		return engine.NewClassicEngine(resolver, submitter, notifier), nil
	})

	// SmartEngine — optional AI App dependencies
	do.Provide(i, func(i do.Injector) (*engine.SmartEngine, error) {
		db := do.MustInvoke[*database.DB](i)
		submitter := &schedulerSubmitter{db: db.DB}

		// Try to resolve AI App services (optional)
		var agentProvider engine.AgentProvider
		var knowledgeSearcher engine.KnowledgeSearcher

		aiAgent, err := do.InvokeAs[app.AIAgentProvider](i)
		if err == nil && aiAgent != nil {
			agentProvider = &aiAgentAdapter{provider: aiAgent}
			slog.Info("ITSM SmartEngine: AI Agent provider available")
		} else {
			slog.Info("ITSM SmartEngine: AI Agent provider not available, smart engine disabled")
		}

		aiKnowledge, err := do.InvokeAs[app.AIKnowledgeSearcher](i)
		if err == nil && aiKnowledge != nil {
			knowledgeSearcher = &aiKnowledgeAdapter{searcher: aiKnowledge}
		}

		// User provider for participant candidates
		userSvc := do.MustInvoke[*service.UserService](i)
		userProvider := &userProviderAdapter{userSvc: userSvc}

		// Participant resolver for tool-based participant resolution
		resolver := do.MustInvoke[*engine.ParticipantResolver](i)

		// Engine config provider for fallback assignee
		configProvider := do.MustInvoke[*EngineConfigService](i)

		return engine.NewSmartEngine(agentProvider, knowledgeSearcher, userProvider, resolver, submitter, configProvider), nil
	})

	// Services
	do.Provide(i, NewCatalogService)
	do.Provide(i, NewServiceDefService)
	do.Provide(i, NewServiceActionService)
	do.Provide(i, NewFormDefService)
	do.Provide(i, NewPriorityService)
	do.Provide(i, NewSLATemplateService)
	do.Provide(i, NewEscalationRuleService)
	do.Provide(i, NewTicketService)
	do.Provide(i, NewTimelineService)
	do.Provide(i, NewVariableService)
	do.Provide(i, NewKnowledgeDocRepo)
	do.Provide(i, NewKnowledgeDocService)
	// Engine config
	do.Provide(i, NewEngineConfigService)
	// Workflow generate
	do.Provide(i, NewWorkflowGenerateService)
	// Handlers
	do.Provide(i, NewCatalogHandler)
	do.Provide(i, NewServiceDefHandler)
	do.Provide(i, NewServiceActionHandler)
	do.Provide(i, NewFormDefHandler)
	do.Provide(i, NewPriorityHandler)
	do.Provide(i, NewSLATemplateHandler)
	do.Provide(i, NewEscalationRuleHandler)
	do.Provide(i, NewTicketHandler)
	do.Provide(i, NewKnowledgeDocHandler)
	do.Provide(i, NewEngineConfigHandler)
	do.Provide(i, NewWorkflowGenerateHandler)
	do.Provide(i, NewVariableHandler)
	do.Provide(i, NewTokenHandler)

	// ITSM tool chain (Operator, StateStore, Registry)
	do.Provide(i, func(i do.Injector) (*tools.Operator, error) {
		db := do.MustInvoke[*database.DB](i)
		resolver := do.MustInvoke[*engine.ParticipantResolver](i)
		ticketSvc := do.MustInvoke[*TicketService](i)
		withdrawFunc := func(ticketID uint, reason string, operatorID uint) error {
			_, err := ticketSvc.Withdraw(ticketID, reason, operatorID)
			return err
		}
		return tools.NewOperator(db.DB, resolver, withdrawFunc), nil
	})
	do.Provide(i, func(i do.Injector) (*tools.SessionStateStore, error) {
		db := do.MustInvoke[*database.DB](i)
		return tools.NewSessionStateStore(db.DB), nil
	})
	do.Provide(i, func(i do.Injector) (*tools.Registry, error) {
		op := do.MustInvoke[*tools.Operator](i)
		store := do.MustInvoke[*tools.SessionStateStore](i)
		return tools.NewRegistry(op, store), nil
	})
}

func (a *ITSMApp) Routes(api *gin.RouterGroup) {
	catalogH := do.MustInvoke[*CatalogHandler](a.injector)
	serviceH := do.MustInvoke[*ServiceDefHandler](a.injector)
	actionH := do.MustInvoke[*ServiceActionHandler](a.injector)
	formDefH := do.MustInvoke[*FormDefHandler](a.injector)
	priorityH := do.MustInvoke[*PriorityHandler](a.injector)
	slaH := do.MustInvoke[*SLATemplateHandler](a.injector)
	escalationH := do.MustInvoke[*EscalationRuleHandler](a.injector)
	ticketH := do.MustInvoke[*TicketHandler](a.injector)
	knowledgeDocH := do.MustInvoke[*KnowledgeDocHandler](a.injector)
	engineConfigH := do.MustInvoke[*EngineConfigHandler](a.injector)
	workflowGenH := do.MustInvoke[*WorkflowGenerateHandler](a.injector)
	variableH := do.MustInvoke[*VariableHandler](a.injector)
	tokenH := do.MustInvoke[*TokenHandler](a.injector)

	g := api.Group("/itsm")
	{
		// Form Definitions
		g.POST("/forms", formDefH.Create)
		g.GET("/forms", formDefH.List)
		g.GET("/forms/:id", formDefH.Get)
		g.PUT("/forms/:id", formDefH.Update)
		g.DELETE("/forms/:id", formDefH.Delete)

		// Service Catalogs
		g.POST("/catalogs", catalogH.Create)
		g.GET("/catalogs/tree", catalogH.Tree)
		g.PUT("/catalogs/:id", catalogH.Update)
		g.DELETE("/catalogs/:id", catalogH.Delete)

		// Service Definitions
		g.POST("/services", serviceH.Create)
		g.GET("/services", serviceH.List)
		g.GET("/services/:id", serviceH.Get)
		g.PUT("/services/:id", serviceH.Update)
		g.DELETE("/services/:id", serviceH.Delete)

		// Service Actions
		g.POST("/services/:id/actions", actionH.Create)
		g.GET("/services/:id/actions", actionH.List)
		g.PUT("/services/:id/actions/:actionId", actionH.Update)
		g.DELETE("/services/:id/actions/:actionId", actionH.Delete)

		// Service Knowledge Documents
		g.POST("/services/:id/knowledge-documents", knowledgeDocH.Upload)
		g.GET("/services/:id/knowledge-documents", knowledgeDocH.List)
		g.DELETE("/services/:id/knowledge-documents/:docId", knowledgeDocH.Delete)

		// Engine Config
		g.GET("/engine/config", engineConfigH.Get)
		g.PUT("/engine/config", engineConfigH.Update)

		// Workflow Generate
		g.POST("/workflows/generate", workflowGenH.Generate)

		// Priorities
		g.POST("/priorities", priorityH.Create)
		g.GET("/priorities", priorityH.List)
		g.PUT("/priorities/:id", priorityH.Update)
		g.DELETE("/priorities/:id", priorityH.Delete)

		// SLA Templates
		g.POST("/sla", slaH.Create)
		g.GET("/sla", slaH.List)
		g.PUT("/sla/:id", slaH.Update)
		g.DELETE("/sla/:id", slaH.Delete)

		// Escalation Rules
		g.POST("/sla/:id/escalations", escalationH.Create)
		g.GET("/sla/:id/escalations", escalationH.List)
		g.PUT("/sla/:id/escalations/:escalationId", escalationH.Update)
		g.DELETE("/sla/:id/escalations/:escalationId", escalationH.Delete)

		// Tickets — special views must come before :id routes
		g.GET("/tickets/mine", ticketH.Mine)
		g.GET("/tickets/todo", ticketH.Todo)
		g.GET("/tickets/history", ticketH.History)
		g.GET("/tickets/approvals", ticketH.Approvals)
		g.GET("/tickets/approvals/count", ticketH.ApprovalCount)
		g.POST("/tickets", ticketH.Create)
		g.GET("/tickets", ticketH.List)
		g.GET("/tickets/:id", ticketH.Get)
		g.PUT("/tickets/:id/assign", ticketH.Assign)
		g.PUT("/tickets/:id/complete", ticketH.Complete)
		g.PUT("/tickets/:id/cancel", ticketH.Cancel)
		g.PUT("/tickets/:id/withdraw", ticketH.Withdraw)
		g.GET("/tickets/:id/timeline", ticketH.Timeline)
		// Phase 2: Classic engine routes
		g.POST("/tickets/:id/progress", ticketH.Progress)
		g.POST("/tickets/:id/signal", ticketH.Signal)
		g.GET("/tickets/:id/activities", ticketH.Activities)
		// Process variables
		g.GET("/tickets/:id/variables", variableH.List)
		g.PUT("/tickets/:id/variables/:key", variableH.Update)
		// Execution tokens
		g.GET("/tickets/:id/tokens", tokenH.List)
		// Phase 3: Smart engine override routes
		g.POST("/tickets/:id/activities/:aid/confirm", ticketH.ConfirmActivity)
		g.POST("/tickets/:id/activities/:aid/reject", ticketH.RejectActivity)
		g.POST("/tickets/:id/override/jump", ticketH.OverrideJump)
		g.POST("/tickets/:id/override/reassign", ticketH.OverrideReassign)
		g.POST("/tickets/:id/override/retry-ai", ticketH.RetryAI)
		// Phase 4: Approval routes
		g.POST("/tickets/:id/activities/:aid/approve", ticketH.ApproveActivity)
		g.POST("/tickets/:id/activities/:aid/deny", ticketH.DenyActivity)
		// SLA pause/resume
		g.PUT("/tickets/:id/sla/pause", ticketH.SLAPause)
		g.PUT("/tickets/:id/sla/resume", ticketH.SLAResume)
		// Task dispatch: transfer, delegate, claim
		g.POST("/tickets/:id/transfer", ticketH.Transfer)
		g.POST("/tickets/:id/delegate", ticketH.Delegate)
		g.POST("/tickets/:id/claim", ticketH.Claim)
	}
}

func (a *ITSMApp) Tasks() []scheduler.TaskDef {
	db := do.MustInvoke[*database.DB](a.injector)
	classicEngine := do.MustInvoke[*engine.ClassicEngine](a.injector)
	smartEngine := do.MustInvoke[*engine.SmartEngine](a.injector)
	knowledgeDocSvc := do.MustInvoke[*KnowledgeDocService](a.injector)

	return []scheduler.TaskDef{
		{
			Name:        "itsm-action-execute",
			Type:        scheduler.TypeAsync,
			Description: "Execute HTTP webhook for ITSM action nodes",
			Handler:     engine.HandleActionExecute(db.DB, classicEngine),
		},
		{
			Name:        "itsm-wait-timer",
			Type:        scheduler.TypeAsync,
			Description: "Check and trigger ITSM wait timer nodes",
			Handler:     engine.HandleWaitTimer(db.DB, classicEngine),
		},
		{
			Name:        "itsm-smart-progress",
			Type:        scheduler.TypeAsync,
			Description: "Execute AI decision cycle for smart engine tickets",
			Handler:     engine.HandleSmartProgress(db.DB, smartEngine),
		},
		{
			Name:        "itsm-boundary-timer",
			Type:        scheduler.TypeAsync,
			Description: "Handle boundary timer expiry for ITSM workflow nodes",
			Handler:     engine.HandleBoundaryTimer(db.DB, classicEngine),
		},
		{
			Name:        "itsm-doc-parse",
			Type:        scheduler.TypeAsync,
			Description: "Parse uploaded knowledge documents for ITSM services",
			Handler:     handleDocParse(knowledgeDocSvc),
		},
		{
			Name:        "itsm-sla-check",
			Type:        scheduler.TypeScheduled,
			CronExpr:    "*/1 * * * *",
			Description: "Check SLA breaches and trigger escalation rules",
			Handler:     engine.HandleSLACheck(db.DB),
		},
	}
}

// schedulerSubmitter implements engine.TaskSubmitter by creating scheduler task records.
type schedulerSubmitter struct {
	db *gorm.DB
}

func (s *schedulerSubmitter) SubmitTask(name string, payload json.RawMessage) error {
	exec := &model.TaskExecution{
		TaskName: name,
		Trigger:  scheduler.TriggerAPI,
		Status:   scheduler.ExecPending,
		Payload:  string(payload),
	}
	return s.db.Create(exec).Error
}

// Ensure schedulerSubmitter implements engine.TaskSubmitter at compile time
var _ engine.TaskSubmitter = (*schedulerSubmitter)(nil)

// notificationAdapter adapts service.MessageChannelService to engine.NotificationSender.
type notificationAdapter struct {
	svc     *service.MessageChannelService
}

func (n *notificationAdapter) Send(ctx context.Context, channelID uint, subject, body string, recipientIDs []uint) error {
	// For now, convert user IDs to string placeholders for email recipients.
	// In a full implementation, you'd look up user emails from the user service.
	// The channel driver (email) expects actual email addresses in the "to" field.
	var to []string
	for _, uid := range recipientIDs {
		// Use user ID as placeholder — the channel service will need user email resolution
		to = append(to, fmt.Sprintf("user:%d", uid))
	}
	return n.svc.SendTest(channelID, to, subject, body)
}

var _ engine.NotificationSender = (*notificationAdapter)(nil)

// Ensure ClassicEngine implements engine.WorkflowEngine at compile time
var _ engine.WorkflowEngine = (*engine.ClassicEngine)(nil)

// Ensure SmartEngine implements engine.WorkflowEngine at compile time
var _ engine.WorkflowEngine = (*engine.SmartEngine)(nil)

// Ensure ITSMApp implements app.ToolRegistryProvider at compile time
var _ app.ToolRegistryProvider = (*ITSMApp)(nil)

// Placeholder for background context usage
var _ = context.Background

// --- AI App adapters (bridge app.AI* interfaces to engine.* interfaces) ---

// aiAgentAdapter adapts app.AIAgentProvider to engine.AgentProvider.
type aiAgentAdapter struct {
	provider app.AIAgentProvider
}

func (a *aiAgentAdapter) GetAgentConfig(agentID uint) (*engine.SmartAgentConfig, error) {
	cfg, err := a.provider.GetAgentConfig(agentID)
	if err != nil {
		return nil, err
	}
	return &engine.SmartAgentConfig{
		Name:         cfg.Name,
		SystemPrompt: cfg.SystemPrompt,
		Temperature:  cfg.Temperature,
		MaxTokens:    cfg.MaxTokens,
		Model:        cfg.Model,
		Protocol:     cfg.Protocol,
		BaseURL:      cfg.BaseURL,
		APIKey:       cfg.APIKey,
	}, nil
}

func (a *aiAgentAdapter) GetAgentConfigByCode(code string) (*engine.SmartAgentConfig, error) {
	cfg, err := a.provider.GetAgentConfigByCode(code)
	if err != nil {
		return nil, err
	}
	return &engine.SmartAgentConfig{
		Name:         cfg.Name,
		SystemPrompt: cfg.SystemPrompt,
		Temperature:  cfg.Temperature,
		MaxTokens:    cfg.MaxTokens,
		Model:        cfg.Model,
		Protocol:     cfg.Protocol,
		BaseURL:      cfg.BaseURL,
		APIKey:       cfg.APIKey,
	}, nil
}

// aiKnowledgeAdapter adapts app.AIKnowledgeSearcher to engine.KnowledgeSearcher.
type aiKnowledgeAdapter struct {
	searcher app.AIKnowledgeSearcher
}

func (a *aiKnowledgeAdapter) Search(kbIDs []uint, query string, limit int) ([]engine.KnowledgeResult, error) {
	results, err := a.searcher.SearchKnowledge(kbIDs, query, limit)
	if err != nil {
		return nil, err
	}
	var out []engine.KnowledgeResult
	for _, r := range results {
		out = append(out, engine.KnowledgeResult{
			Title:   r.Title,
			Content: r.Content,
			Score:   r.Score,
		})
	}
	return out, nil
}

// userProviderAdapter adapts service.UserService to engine.UserProvider.
type userProviderAdapter struct {
	userSvc *service.UserService
}

func (a *userProviderAdapter) ListActiveUsers() ([]engine.ParticipantCandidate, error) {
	active := true
	result, err := a.userSvc.List(repository.ListParams{
		IsActive: &active,
		Page:     1,
		PageSize: 500,
	})
	if err != nil {
		return nil, err
	}
	var candidates []engine.ParticipantCandidate
	for _, u := range result.Items {
		candidates = append(candidates, engine.ParticipantCandidate{
			UserID: u.ID,
			Name:   u.Username,
		})
	}
	return candidates, nil
}

