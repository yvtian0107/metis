package itsm

import (
	"metis/internal/app/itsm/catalog"
	"metis/internal/app/itsm/config"
	"metis/internal/app/itsm/definition"
	"metis/internal/app/itsm/desk"
	"metis/internal/app/itsm/sla"
	"metis/internal/app/itsm/ticket"

	"github.com/gin-gonic/gin"
	"github.com/samber/do/v2"
)

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
		g.POST("/catalogs", catalogH.Create)
		g.GET("/catalogs/tree", catalogH.Tree)
		g.GET("/catalogs/service-counts", catalogH.ServiceCounts)
		g.PUT("/catalogs/:id", catalogH.Update)
		g.DELETE("/catalogs/:id", catalogH.Delete)

		g.POST("/services", serviceH.Create)
		g.GET("/services", serviceH.List)
		g.GET("/services/:id/health", serviceH.HealthCheck)
		g.GET("/services/:id", serviceH.Get)
		g.PUT("/services/:id", serviceH.Update)
		g.DELETE("/services/:id", serviceH.Delete)

		g.POST("/services/:id/actions", actionH.Create)
		g.GET("/services/:id/actions", actionH.List)
		g.PUT("/services/:id/actions/:actionId", actionH.Update)
		g.DELETE("/services/:id/actions/:actionId", actionH.Delete)

		g.POST("/services/:id/knowledge-documents", knowledgeDocH.Upload)
		g.GET("/services/:id/knowledge-documents", knowledgeDocH.List)
		g.DELETE("/services/:id/knowledge-documents/:docId", knowledgeDocH.Delete)

		g.GET("/smart-staffing/config", engineConfigH.GetSmartStaffing)
		g.PUT("/smart-staffing/config", engineConfigH.UpdateSmartStaffing)
		g.GET("/engine-settings/config", engineConfigH.GetEngineSettings)
		g.PUT("/engine-settings/config", engineConfigH.UpdateEngineSettings)

		g.POST("/workflows/generate", workflowGenH.Generate)
		g.GET("/workflows/capabilities", workflowGenH.Capabilities)

		g.GET("/service-desk/sessions/:sid/state", serviceDeskH.State)
		g.POST("/service-desk/sessions/:sid/draft/submit", serviceDeskH.SubmitDraft)

		g.POST("/priorities", priorityH.Create)
		g.GET("/priorities", priorityH.List)
		g.PUT("/priorities/:id", priorityH.Update)
		g.DELETE("/priorities/:id", priorityH.Delete)

		g.POST("/sla", slaH.Create)
		g.GET("/sla", slaH.List)
		g.GET("/sla/notification-channels", escalationH.NotificationChannels)
		g.PUT("/sla/:id", slaH.Update)
		g.DELETE("/sla/:id", slaH.Delete)

		g.POST("/sla/:id/escalations", escalationH.Create)
		g.GET("/sla/:id/escalations", escalationH.List)
		g.PUT("/sla/:id/escalations/:escalationId", escalationH.Update)
		g.DELETE("/sla/:id/escalations/:escalationId", escalationH.Delete)

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
		g.POST("/tickets/:id/progress", ticketH.Progress)
		g.POST("/tickets/:id/signal", ticketH.Signal)
		g.GET("/tickets/:id/activities", ticketH.Activities)
		g.GET("/tickets/:id/variables", variableH.List)
		g.PUT("/tickets/:id/variables/:key", variableH.Update)
		g.GET("/tickets/:id/tokens", tokenH.List)
		g.POST("/tickets/:id/override/jump", ticketH.OverrideJump)
		g.POST("/tickets/:id/override/reassign", ticketH.OverrideReassign)
		g.POST("/tickets/:id/override/retry-ai", ticketH.RetryAI)
		g.POST("/tickets/:id/recovery", ticketH.Recover)
		g.PUT("/tickets/:id/sla/pause", ticketH.SLAPause)
		g.PUT("/tickets/:id/sla/resume", ticketH.SLAResume)
		g.POST("/tickets/:id/transfer", ticketH.Transfer)
		g.POST("/tickets/:id/delegate", ticketH.Delegate)
		g.POST("/tickets/:id/claim", ticketH.Claim)
	}
}
