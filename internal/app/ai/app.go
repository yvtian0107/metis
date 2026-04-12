package ai

import (
	"github.com/casbin/casbin/v2"
	"github.com/gin-gonic/gin"
	"github.com/samber/do/v2"
	"gorm.io/gorm"

	"metis/internal/app"
	"metis/internal/app/node"
	"metis/internal/scheduler"
)

func init() {
	app.Register(&AIApp{})
}

type AIApp struct {
	injector do.Injector
}

func (a *AIApp) Name() string { return "ai" }

func (a *AIApp) Models() []any {
	return []any{
		&Provider{}, &AIModel{}, &AILog{},
		&KnowledgeBase{}, &KnowledgeSource{}, &KnowledgeLog{},
		// Tool registry
		&Tool{}, &MCPServer{}, &Skill{},
		&AgentTool{}, &AgentMCPServer{}, &AgentSkill{},
		// Agent runtime
		&Agent{}, &AgentTemplate{}, &AgentKnowledgeBase{},
		&AgentSession{}, &SessionMessage{}, &AgentMemory{},
	}
}

func (a *AIApp) Seed(db *gorm.DB, enforcer *casbin.Enforcer) error {
	return seedAI(db, enforcer)
}

func (a *AIApp) Providers(i do.Injector) {
	a.injector = i
	do.Provide(i, NewProviderRepo)
	do.Provide(i, NewModelRepo)
	do.Provide(i, NewProviderService)
	do.Provide(i, NewModelService)
	do.Provide(i, NewProviderHandler)
	do.Provide(i, NewModelHandler)
	// FalkorDB
	do.Provide(i, NewFalkorDBClient)
	// Knowledge
	do.Provide(i, NewKnowledgeBaseRepo)
	do.Provide(i, NewKnowledgeSourceRepo)
	do.Provide(i, NewKnowledgeGraphRepo)
	do.Provide(i, NewKnowledgeBaseService)
	do.Provide(i, NewKnowledgeSourceService)
	do.Provide(i, NewKnowledgeEmbeddingService)
	do.Provide(i, NewKnowledgeBaseHandler)
	do.Provide(i, NewKnowledgeSourceHandler)
	do.Provide(i, NewKnowledgeNodeHandler)
	do.Provide(i, NewKnowledgeLogRepo)
	do.Provide(i, NewKnowledgeExtractService)
	do.Provide(i, NewKnowledgeCompileService)
	do.Provide(i, NewKnowledgeQueryHandler)
	// Tool registry
	do.Provide(i, NewToolRepo)
	do.Provide(i, NewToolService)
	do.Provide(i, NewToolHandler)
	do.Provide(i, NewMCPServerRepo)
	do.Provide(i, NewMCPServerService)
	do.Provide(i, NewMCPServerHandler)
	do.Provide(i, NewSkillRepo)
	do.Provide(i, NewSkillService)
	do.Provide(i, NewSkillHandler)
	// Tool bindings & assembly
	do.Provide(i, NewAgentToolRepo)
	do.Provide(i, NewAgentMCPServerRepo)
	do.Provide(i, NewAgentSkillRepo)
	do.Provide(i, NewToolAssemblyService)
	// Agent runtime
	do.Provide(i, NewAgentRepo)
	do.Provide(i, NewAgentService)
	do.Provide(i, NewAgentHandler)
	do.Provide(i, NewSessionRepo)
	do.Provide(i, NewSessionService)
	do.Provide(i, NewSessionHandler)
	do.Provide(i, NewMemoryRepo)
	do.Provide(i, NewMemoryService)
	do.Provide(i, NewMemoryHandler)
	do.Provide(i, NewAgentGateway)
}

func (a *AIApp) Routes(api *gin.RouterGroup) {
	providerH := do.MustInvoke[*ProviderHandler](a.injector)
	modelH := do.MustInvoke[*ModelHandler](a.injector)
	kbH := do.MustInvoke[*KnowledgeBaseHandler](a.injector)
	sourceH := do.MustInvoke[*KnowledgeSourceHandler](a.injector)
	nodeH := do.MustInvoke[*KnowledgeNodeHandler](a.injector)
	queryH := do.MustInvoke[*KnowledgeQueryHandler](a.injector)

	providers := api.Group("/ai/providers")
	{
		providers.POST("", providerH.Create)
		providers.GET("", providerH.List)
		providers.GET("/:id", providerH.Get)
		providers.PUT("/:id", providerH.Update)
		providers.DELETE("/:id", providerH.Delete)
		providers.POST("/:id/test", providerH.TestConnection)
		providers.POST("/:id/sync-models", modelH.SyncModels)
	}

	models := api.Group("/ai/models")
	{
		models.POST("", modelH.Create)
		models.GET("", modelH.List)
		models.GET("/:id", modelH.Get)
		models.PUT("/:id", modelH.Update)
		models.DELETE("/:id", modelH.Delete)
		models.PATCH("/:id/default", modelH.SetDefault)
	}

	kbs := api.Group("/ai/knowledge-bases")
	{
		kbs.POST("", kbH.Create)
		kbs.GET("", kbH.List)
		kbs.GET("/:id", kbH.Get)
		kbs.PUT("/:id", kbH.Update)
		kbs.DELETE("/:id", kbH.Delete)
		kbs.POST("/:id/compile", kbH.Compile)
		kbs.POST("/:id/recompile", kbH.Recompile)
		kbs.GET("/:id/progress", kbH.GetProgress)
		// Sources
		kbs.POST("/:id/sources", sourceH.Create)
		kbs.GET("/:id/sources", sourceH.List)
		kbs.GET("/:id/sources/:sid", sourceH.Get)
		kbs.DELETE("/:id/sources/:sid", sourceH.Delete)
		// Nodes & Graph
		kbs.GET("/:id/graph", nodeH.GetFullGraph)
		kbs.GET("/:id/nodes", nodeH.List)
		kbs.GET("/:id/nodes/:nid", nodeH.Get)
		kbs.GET("/:id/nodes/:nid/graph", nodeH.GetGraph)
		// Logs
		kbs.GET("/:id/logs", nodeH.ListLogs)
		// Search (admin-facing, JWT auth)
		kbs.GET("/:id/search", queryH.SearchByKb)
	}

	// Tool registry
	toolH := do.MustInvoke[*ToolHandler](a.injector)
	tools := api.Group("/ai/tools")
	{
		tools.GET("", toolH.List)
		tools.PUT("/:id", toolH.Update)
	}

	mcpH := do.MustInvoke[*MCPServerHandler](a.injector)
	mcpServers := api.Group("/ai/mcp-servers")
	{
		mcpServers.POST("", mcpH.Create)
		mcpServers.GET("", mcpH.List)
		mcpServers.GET("/:id", mcpH.Get)
		mcpServers.PUT("/:id", mcpH.Update)
		mcpServers.DELETE("/:id", mcpH.Delete)
		mcpServers.POST("/:id/test", mcpH.TestConnection)
	}

	skillH := do.MustInvoke[*SkillHandler](a.injector)
	skills := api.Group("/ai/skills")
	{
		skills.GET("", skillH.List)
		skills.GET("/:id", skillH.Get)
		skills.POST("/import-github", skillH.ImportGitHub)
		skills.POST("/upload", skillH.Upload)
		skills.PUT("/:id", skillH.Update)
		skills.PATCH("/:id/active", skillH.ToggleActive)
		skills.DELETE("/:id", skillH.Delete)
	}

	// Agent runtime
	agentH := do.MustInvoke[*AgentHandler](a.injector)
	agents := api.Group("/ai/agents")
	{
		agents.POST("", agentH.Create)
		agents.GET("", agentH.List)
		agents.GET("/templates", agentH.ListTemplates)
		agents.GET("/:id", agentH.Get)
		agents.PUT("/:id", agentH.Update)
		agents.DELETE("/:id", agentH.Delete)
	}

	sessionH := do.MustInvoke[*SessionHandler](a.injector)
	sessions := api.Group("/ai/sessions")
	{
		sessions.POST("", sessionH.Create)
		sessions.GET("", sessionH.List)
		sessions.GET("/:sid", sessionH.Get)
		sessions.DELETE("/:sid", sessionH.Delete)
		sessions.POST("/:sid/messages", sessionH.SendMessage)
		sessions.GET("/:sid/stream", sessionH.Stream)
		sessions.POST("/:sid/cancel", sessionH.Cancel)
	}

	memoryH := do.MustInvoke[*MemoryHandler](a.injector)
	memories := api.Group("/ai/agents/:id/memories")
	{
		memories.GET("", memoryH.List)
		memories.POST("", memoryH.Create)
		memories.DELETE("/:mid", memoryH.Delete)
	}

	// Agent knowledge query API (Sidecar token auth, bypasses JWT+Casbin)
	r := do.MustInvoke[*gin.Engine](a.injector)
	nodeRepo := do.MustInvoke[*node.NodeRepo](a.injector)
	knowledge := r.Group("/api/v1/ai/knowledge", node.NodeTokenMiddleware(nodeRepo))
	{
		knowledge.GET("/search", queryH.Search)
		knowledge.GET("/nodes/:id", queryH.GetNode)
		knowledge.GET("/nodes/:id/graph", queryH.GetNodeGraph)
	}

	// Internal API for Agent to download skill packages (Node token auth)
	internal := r.Group("/api/v1/ai/internal", node.NodeTokenMiddleware(nodeRepo))
	{
		internal.GET("/skills/:id/package", skillH.Package)
	}
}

func (a *AIApp) Tasks() []scheduler.TaskDef {
	extractSvc := do.MustInvoke[*KnowledgeExtractService](a.injector)
	compileSvc := do.MustInvoke[*KnowledgeCompileService](a.injector)
	var defs []scheduler.TaskDef
	defs = append(defs, extractSvc.TaskDefs()...)
	defs = append(defs, compileSvc.TaskDefs()...)
	return defs
}
