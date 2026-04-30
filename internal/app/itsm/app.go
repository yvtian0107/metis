package itsm

import (
	"context"
	"encoding/json"
	"log/slog"
	"metis/internal/app/itsm/bootstrap"
	"metis/internal/app/itsm/catalog"
	"metis/internal/app/itsm/config"
	"metis/internal/app/itsm/definition"
	"metis/internal/app/itsm/desk"
	"metis/internal/app/itsm/domain"
	"metis/internal/app/itsm/sla"
	"metis/internal/app/itsm/ticket"
	"strings"
	"time"

	"github.com/casbin/casbin/v2"
	"github.com/gin-gonic/gin"
	"github.com/samber/do/v2"
	"gorm.io/gorm"

	"metis/internal/app"
	aiapp "metis/internal/app/ai/runtime"
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

// BuildAgentRuntimeContext implements app.AgentRuntimeContextProvider for ITSM
// service desk sessions.
func (a *ITSMApp) BuildAgentRuntimeContext(ctx context.Context, _ string, sessionID, userID uint) (string, error) {
	if sessionID == 0 {
		return "", nil
	}
	configProvider := do.MustInvoke[*config.EngineConfigService](a.injector)
	intakeAgentID := configProvider.IntakeAgentID()
	if intakeAgentID == 0 {
		return "", nil
	}
	db := do.MustInvoke[*database.DB](a.injector)
	var row struct {
		ID uint
	}
	if err := db.DB.Table("ai_agent_sessions").
		Where("id = ? AND user_id = ? AND agent_id = ?", sessionID, userID, intakeAgentID).
		Select("id").
		First(&row).Error; err != nil {
		return "", nil
	}
	store := do.MustInvoke[*tools.SessionStateStore](a.injector)
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

// GenerateSessionTitle implements app.SessionTitleProvider.
func (a *ITSMApp) GenerateSessionTitle(ctx context.Context, sessionID, userID, agentID uint, firstUserMessage string) (string, bool, error) {
	titleSvc := do.MustInvoke[*desk.SessionTitleService](a.injector)
	return titleSvc.Generate(ctx, sessionID, userID, agentID, firstUserMessage)
}

func (a *ITSMApp) Models() []any {
	return []any{
		// Configuration models
		&domain.ServiceCatalog{},
		&domain.ServiceDefinition{},
		&domain.ServiceDefinitionVersion{},
		&domain.ServiceAction{},
		&domain.Priority{},
		&domain.SLATemplate{},
		&domain.EscalationRule{},
		// domain.Ticket lifecycle models
		&domain.Ticket{},
		&domain.TicketActivity{},
		&domain.TicketAssignment{},
		&domain.TicketTimeline{},
		&domain.TicketActionExecution{},
		&domain.ServiceDeskSubmission{},
		// Incident models
		&domain.TicketLink{},
		&domain.PostMortem{},
		// Process variables
		&domain.ProcessVariable{},
		// Execution tokens
		&domain.ExecutionToken{},
		// Knowledge documents
		&domain.ServiceKnowledgeDocument{},
	}
}

func (a *ITSMApp) Seed(db *gorm.DB, enforcer *casbin.Enforcer, _ bool) error {
	return bootstrap.SeedITSM(db, enforcer)
}

func (a *ITSMApp) Providers(i do.Injector) {
	a.injector = i
	// Repositories
	do.Provide(i, catalog.NewCatalogRepo)
	do.Provide(i, definition.NewServiceDefRepo)
	do.Provide(i, definition.NewServiceActionRepo)
	do.Provide(i, sla.NewPriorityRepo)
	do.Provide(i, sla.NewSLATemplateRepo)
	do.Provide(i, sla.NewEscalationRuleRepo)
	do.Provide(i, ticket.NewTicketRepo)
	do.Provide(i, ticket.NewTimelineRepo)
	do.Provide(i, ticket.NewVariableRepository)
	do.Provide(i, ticket.NewTokenRepository)

	// Engine components
	do.Provide(i, func(i do.Injector) (*engine.ParticipantResolver, error) {
		// OrgResolver is optional — nil if Org App not installed
		orgResolver, _ := do.InvokeAs[app.OrgResolver](i)
		return engine.NewParticipantResolver(orgResolver), nil
	})

	do.Provide(i, func(i do.Injector) (*engine.ClassicEngine, error) {
		resolver := do.MustInvoke[*engine.ParticipantResolver](i)
		db := do.MustInvoke[*database.DB](i)
		// Create a TaskSubmitter that uses the scheduler engine
		submitter := &schedulerSubmitter{db: db.DB}
		// Try to resolve NotificationSender (optional — nil if MessageChannel service not available)
		var notifier engine.NotificationSender
		if mcSvc, err := do.Invoke[*service.MessageChannelService](i); err == nil && mcSvc != nil {
			notifier = &notificationAdapter{svc: mcSvc, db: db.DB}
			slog.Info("ITSM ClassicEngine: notification sender available")
		}
		return engine.NewClassicEngine(resolver, submitter, notifier), nil
	})

	// SmartEngine — optional AI App dependencies
	do.Provide(i, func(i do.Injector) (*engine.SmartEngine, error) {
		db := do.MustInvoke[*database.DB](i)
		submitter := &schedulerSubmitter{db: db.DB}

		// Try to resolve AI App services (optional)
		var decisionExecutor app.AIDecisionExecutor
		var knowledgeSearcher engine.KnowledgeSearcher

		de, err := do.InvokeAs[app.AIDecisionExecutor](i)
		if err == nil && de != nil {
			decisionExecutor = de
			slog.Info("ITSM SmartEngine: AI DecisionExecutor available")
		} else {
			slog.Info("ITSM SmartEngine: AI DecisionExecutor not available, smart engine disabled")
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
		configProvider := do.MustInvoke[*config.EngineConfigService](i)

		se := engine.NewSmartEngine(decisionExecutor, knowledgeSearcher, userProvider, resolver, submitter, configProvider)
		se.SetDB(db.DB)
		se.SetActionExecutor(engine.NewActionExecutor(db.DB))
		return se, nil
	})

	// Services
	do.Provide(i, catalog.NewCatalogService)
	do.Provide(i, definition.NewServiceDefService)
	do.Provide(i, definition.NewServiceActionService)
	do.Provide(i, sla.NewPriorityService)
	do.Provide(i, sla.NewSLATemplateService)
	do.Provide(i, sla.NewEscalationRuleService)
	do.Provide(i, ticket.NewTicketService)
	do.Provide(i, ticket.NewTimelineService)
	do.Provide(i, ticket.NewVariableService)
	do.Provide(i, definition.NewKnowledgeDocRepo)
	do.Provide(i, definition.NewKnowledgeDocService)
	// Engine config
	do.Provide(i, config.NewEngineConfigService)
	do.Provide(i, desk.NewSessionTitleService)
	// Workflow generate
	do.Provide(i, definition.NewWorkflowGenerateService)
	// Handlers
	do.Provide(i, catalog.NewCatalogHandler)
	do.Provide(i, definition.NewServiceDefHandler)
	do.Provide(i, definition.NewServiceActionHandler)
	do.Provide(i, sla.NewPriorityHandler)
	do.Provide(i, sla.NewSLATemplateHandler)
	do.Provide(i, sla.NewEscalationRuleHandler)
	do.Provide(i, ticket.NewTicketHandler)
	do.Provide(i, definition.NewKnowledgeDocHandler)
	do.Provide(i, config.NewEngineConfigHandler)
	do.Provide(i, definition.NewWorkflowGenerateHandler)
	do.Provide(i, ticket.NewVariableHandler)
	do.Provide(i, ticket.NewTokenHandler)
	do.Provide(i, desk.NewServiceDeskHandler)

	// ITSM tool chain (Operator, StateStore, Registry)
	// NOTE: ticket.TicketService is resolved lazily inside withdrawFunc to break a circular
	// dependency: ticket.TicketService → SmartEngine → AgentGateway → collectToolRegistries
	// → tools.Registry → tools.Operator → ticket.TicketService.
	do.Provide(i, func(i do.Injector) (*tools.Operator, error) {
		db := do.MustInvoke[*database.DB](i)
		resolver := do.MustInvoke[*engine.ParticipantResolver](i)
		orgResolver, _ := do.InvokeAs[app.OrgResolver](i)
		withdrawFunc := func(ticketID uint, reason string, operatorID uint) error {
			ticketSvc := do.MustInvoke[*ticket.TicketService](i)
			_, err := ticketSvc.Withdraw(ticketID, reason, operatorID)
			return err
		}
		// TicketCreator is resolved lazily (same pattern as withdrawFunc) to break circular dep.
		var ticketCreator tools.TicketCreator
		ticketCreator = &lazyTicketCreator{injector: i}
		runtimeProvider := do.MustInvoke[*aiapp.ToolRuntimeService](i)
		matcher := definition.NewLLMServiceMatcher(db.DB, runtimeProvider, nil)
		return tools.NewOperator(db.DB, resolver, orgResolver, withdrawFunc, ticketCreator, matcher), nil
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
	catalogH := do.MustInvoke[*catalog.CatalogHandler](a.injector)
	serviceH := do.MustInvoke[*definition.ServiceDefHandler](a.injector)
	actionH := do.MustInvoke[*definition.ServiceActionHandler](a.injector)
	priorityH := do.MustInvoke[*sla.PriorityHandler](a.injector)
	slaH := do.MustInvoke[*sla.SLATemplateHandler](a.injector)
	escalationH := do.MustInvoke[*sla.EscalationRuleHandler](a.injector)
	ticketH := do.MustInvoke[*ticket.TicketHandler](a.injector)
	knowledgeDocH := do.MustInvoke[*definition.KnowledgeDocHandler](a.injector)
	engineConfigH := do.MustInvoke[*config.EngineConfigHandler](a.injector)
	workflowGenH := do.MustInvoke[*definition.WorkflowGenerateHandler](a.injector)
	variableH := do.MustInvoke[*ticket.VariableHandler](a.injector)
	tokenH := do.MustInvoke[*ticket.TokenHandler](a.injector)
	serviceDeskH := do.MustInvoke[*desk.ServiceDeskHandler](a.injector)

	g := api.Group("/itsm")
	{
		// Service Catalogs
		g.POST("/catalogs", catalogH.Create)
		g.GET("/catalogs/tree", catalogH.Tree)
		g.GET("/catalogs/service-counts", catalogH.ServiceCounts)
		g.PUT("/catalogs/:id", catalogH.Update)
		g.DELETE("/catalogs/:id", catalogH.Delete)

		// Service Definitions
		g.POST("/services", serviceH.Create)
		g.GET("/services", serviceH.List)
		g.GET("/services/:id/health", serviceH.HealthCheck)
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

		// Smart Staffing
		g.GET("/smart-staffing/config", engineConfigH.GetSmartStaffing)
		g.PUT("/smart-staffing/config", engineConfigH.UpdateSmartStaffing)
		g.GET("/engine-settings/config", engineConfigH.GetEngineSettings)
		g.PUT("/engine-settings/config", engineConfigH.UpdateEngineSettings)

		// Workflow Generate
		g.POST("/workflows/generate", workflowGenH.Generate)

		// Service Desk
		g.GET("/service-desk/sessions/:sid/state", serviceDeskH.State)
		g.POST("/service-desk/sessions/:sid/draft/submit", serviceDeskH.SubmitDraft)

		// Priorities
		g.POST("/priorities", priorityH.Create)
		g.GET("/priorities", priorityH.List)
		g.PUT("/priorities/:id", priorityH.Update)
		g.DELETE("/priorities/:id", priorityH.Delete)

		// SLA Templates
		g.POST("/sla", slaH.Create)
		g.GET("/sla", slaH.List)
		g.GET("/sla/notification-channels", escalationH.NotificationChannels)
		g.PUT("/sla/:id", slaH.Update)
		g.DELETE("/sla/:id", slaH.Delete)

		// Escalation Rules
		g.POST("/sla/:id/escalations", escalationH.Create)
		g.GET("/sla/:id/escalations", escalationH.List)
		g.PUT("/sla/:id/escalations/:escalationId", escalationH.Update)
		g.DELETE("/sla/:id/escalations/:escalationId", escalationH.Delete)

		// Tickets — special views must come before :id routes
		g.GET("/tickets/mine", ticketH.Mine)
		g.GET("/tickets/approvals/pending", ticketH.PendingApprovals)
		g.GET("/tickets/approvals/history", ticketH.ApprovalHistory)
		g.GET("/tickets/monitor", ticketH.Monitor)
		g.GET("/tickets/decision-quality", ticketH.DecisionQuality)
		g.POST("/tickets", ticketH.Create)
		g.GET("/tickets", ticketH.List)
		g.GET("/tickets/:id", ticketH.Get)
		g.PUT("/tickets/:id/assign", ticketH.Assign)
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
		g.POST("/tickets/:id/override/jump", ticketH.OverrideJump)
		g.POST("/tickets/:id/override/reassign", ticketH.OverrideReassign)
		g.POST("/tickets/:id/override/retry-ai", ticketH.RetryAI)
		g.POST("/tickets/:id/recovery", ticketH.Recover)
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
	configProvider := do.MustInvoke[*config.EngineConfigService](a.injector)
	resolver := do.MustInvoke[*engine.ParticipantResolver](a.injector)
	knowledgeDocSvc := do.MustInvoke[*definition.KnowledgeDocService](a.injector)
	var slaAssuranceExecutor app.AIDecisionExecutor
	if executor, err := do.InvokeAs[app.AIDecisionExecutor](a.injector); err == nil {
		slaAssuranceExecutor = executor
	}
	var notifier engine.NotificationSender
	if mcSvc, err := do.Invoke[*service.MessageChannelService](a.injector); err == nil && mcSvc != nil {
		notifier = &notificationAdapter{svc: mcSvc, db: db.DB}
	}

	return []scheduler.TaskDef{
		{
			Name:        "itsm-action-execute",
			Type:        scheduler.TypeAsync,
			Description: "Execute HTTP webhook for ITSM action nodes",
			Handler:     engine.HandleActionExecute(db.DB, classicEngine, smartEngine),
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
			Timeout:     2 * time.Minute,
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
			Handler:     definition.HandleDocParse(knowledgeDocSvc),
		},
		{
			Name:        "itsm-sla-check",
			Type:        scheduler.TypeScheduled,
			CronExpr:    "*/1 * * * *",
			Description: "Check SLA breaches and trigger escalation rules",
			Handler:     engine.HandleSLACheck(db.DB, configProvider, slaAssuranceExecutor, resolver, notifier),
		},
		{
			Name:        "itsm-smart-recovery",
			Type:        scheduler.TypeScheduled,
			CronExpr:    "@every 10m",
			Description: "Recover in_progress smart tickets that lost their decision cycle",
			Handler:     engine.HandleSmartRecovery(db.DB, smartEngine),
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

func (s *schedulerSubmitter) SubmitTaskTx(tx *gorm.DB, name string, payload json.RawMessage) error {
	exec := &model.TaskExecution{
		TaskName: name,
		Trigger:  scheduler.TriggerAPI,
		Status:   scheduler.ExecPending,
		Payload:  string(payload),
	}
	return tx.Create(exec).Error
}

// Ensure schedulerSubmitter implements engine.TaskSubmitter at compile time
var _ engine.TaskSubmitter = (*schedulerSubmitter)(nil)

// notificationAdapter adapts service.MessageChannelService to engine.NotificationSender.
type notificationAdapter struct {
	svc *service.MessageChannelService
	db  *gorm.DB
}

func (n *notificationAdapter) Send(ctx context.Context, channelID uint, subject, body string, recipientIDs []uint) error {
	if len(recipientIDs) == 0 {
		return engine.ErrNotificationNoRecipients
	}
	type userEmail struct {
		ID    uint
		Email string
	}
	var rows []userEmail
	if err := n.db.WithContext(ctx).
		Table("users").
		Select("id, email").
		Where("id IN ? AND deleted_at IS NULL AND is_active = ?", recipientIDs, true).
		Find(&rows).Error; err != nil {
		return err
	}

	emails := make([]string, 0, len(rows))
	for _, row := range rows {
		email := strings.TrimSpace(row.Email)
		if email != "" {
			emails = append(emails, email)
		}
	}
	if len(emails) == 0 {
		return engine.ErrNotificationNoEmail
	}
	return n.svc.Send(channelID, emails, subject, body)
}

var _ engine.NotificationSender = (*notificationAdapter)(nil)

// Ensure ClassicEngine implements engine.WorkflowEngine at compile time
var _ engine.WorkflowEngine = (*engine.ClassicEngine)(nil)

// Ensure SmartEngine implements engine.WorkflowEngine at compile time
var _ engine.WorkflowEngine = (*engine.SmartEngine)(nil)

// Ensure ITSMApp implements app.ToolRegistryProvider at compile time
var _ app.ToolRegistryProvider = (*ITSMApp)(nil)
var _ app.AgentRuntimeContextProvider = (*ITSMApp)(nil)
var _ app.SessionTitleProvider = (*ITSMApp)(nil)

// lazyTicketCreator defers resolution of ticket.TicketService to break circular dependency.
type lazyTicketCreator struct {
	injector do.Injector
}

func (l *lazyTicketCreator) CreateFromAgent(ctx context.Context, req tools.AgentTicketRequest) (*tools.AgentTicketResult, error) {
	ticketSvc := do.MustInvoke[*ticket.TicketService](l.injector)
	return ticketSvc.CreateFromAgent(ctx, req)
}

var _ tools.TicketCreator = (*lazyTicketCreator)(nil)

// Placeholder for background context usage
var _ = context.Background

// --- AI App adapters (bridge app.AI* interfaces to engine.* interfaces) ---

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
