package ai

import (
	"log/slog"

	"github.com/casbin/casbin/v2"
	"github.com/gin-gonic/gin"
	"github.com/samber/do/v2"
	"gorm.io/gorm"

	"metis/internal/app"
	. "metis/internal/app/ai/runtime"
	nodenode "metis/internal/app/node/node"
	"metis/internal/handler"
	"metis/internal/scheduler"
	"metis/internal/service"
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
		// Knowledge (new unified model)
		&KnowledgeAsset{}, &KnowledgeSource{}, &KnowledgeAssetSource{},
		&RAGChunk{}, &KnowledgeLog{},
		// Legacy knowledge table (kept for migration)
		&KnowledgeBase{},
		// Tool registry
		&Tool{}, &MCPServer{}, &Skill{},
		&CapabilitySet{}, &CapabilitySetItem{},
		&AgentTool{}, &AgentMCPServer{}, &AgentSkill{},
		// Agent runtime
		&AgentCapabilitySet{}, &AgentCapabilitySetItem{},
		&Agent{}, &AgentTemplate{}, &AgentKnowledgeBase{}, &AgentKnowledgeGraph{},
		&AgentSession{}, &SessionMessage{}, &AgentMemory{},
	}
}

func (a *AIApp) Seed(db *gorm.DB, enforcer *casbin.Enforcer, _ bool) error {
	// One-time data migration from legacy knowledge tables
	if err := MigrateKnowledgeData(db); err != nil {
		slog.Error("knowledge data migration failed", "error", err)
		// Non-fatal: don't block startup
	}
	return SeedAI(db, enforcer)
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
	// Knowledge — new unified model
	do.Provide(i, NewKnowledgeAssetRepo)
	do.Provide(i, NewKnowledgeSourceRepo)
	do.Provide(i, NewKnowledgeSourceService)
	do.Provide(i, NewKnowledgeSourceHandler)
	do.Provide(i, NewKnowledgeLogRepo)
	do.Provide(i, NewKnowledgeAssetService)
	do.Provide(i, NewKnowledgeSearchService)
	do.Provide(i, func(i do.Injector) (app.AIKnowledgeSearcher, error) {
		return do.MustInvoke[*KnowledgeSearchService](i), nil
	})
	// Knowledge — graph engine
	do.Provide(i, NewConceptMapEngine)
	do.Provide(i, NewKnowledgeGraphHandler)
	// Knowledge — RAG engine
	do.Provide(i, NewRAGChunkRepo)
	do.Provide(i, NewNaiveChunkEngine)
	do.Provide(i, NewKnowledgeKBHandler)
	// Knowledge — legacy (graph compile pipeline, will be replaced by ConceptMapEngine)
	do.Provide(i, NewKnowledgeBaseRepo)
	do.Provide(i, NewKnowledgeGraphRepo)
	do.Provide(i, NewKnowledgeEmbeddingService)
	do.Provide(i, NewKnowledgeExtractService)
	do.Provide(i, NewKnowledgeCompileService)
	// Tool registry
	do.Provide(i, NewToolRepo)
	do.Provide(i, NewToolRuntimeService)
	do.Provide(i, NewToolService)
	do.Provide(i, NewToolHandler)
	do.Provide(i, NewMCPServerRepo)
	do.Provide(i, NewDefaultMCPRuntimeClient)
	do.Provide(i, NewMCPServerService)
	do.Provide(i, NewMCPServerHandler)
	do.Provide(i, NewSkillRepo)
	do.Provide(i, NewSkillService)
	do.Provide(i, NewSkillHandler)
	do.Provide(i, NewCapabilitySetRepo)
	do.Provide(i, NewCapabilitySetService)
	do.Provide(i, NewCapabilitySetHandler)
	// Tool bindings & assembly
	do.Provide(i, NewAgentToolRepo)
	do.Provide(i, NewAgentMCPServerRepo)
	do.Provide(i, NewAgentSkillRepo)
	do.Provide(i, NewToolAssemblyService)
	// Agent runtime
	do.Provide(i, NewAgentRepo)
	do.Provide(i, NewAgentService)
	do.Provide(i, NewAgentHandler)
	do.Provide(i, NewAssistantAgentHandler)
	do.Provide(i, NewCodingAgentHandler)
	do.Provide(i, NewSessionRepo)
	do.Provide(i, NewSessionService)
	do.Provide(i, NewSessionHandler)
	do.Provide(i, NewMemoryRepo)
	do.Provide(i, NewMemoryService)
	do.Provide(i, NewMemoryHandler)
	do.Provide(i, NewAgentGateway)
	do.Provide(i, func(i do.Injector) (app.AIAgentProvider, error) {
		return do.MustInvoke[*AgentGateway](i), nil
	})
	do.Provide(i, NewKnowledgeQueryHandler)
	do.Provide(i, NewKnowledgeToolRegistry)

	// DecisionExecutor for smart workflow engine decision cycles
	do.Provide(i, func(i do.Injector) (app.AIDecisionExecutor, error) {
		gw := do.MustInvoke[*AgentGateway](i)
		return NewDecisionExecutor(gw), nil
	})

	// General tool registry (used by CompositeToolExecutor in gateway)
	do.Provide(i, func(i do.Injector) (*GeneralToolRegistry, error) {
		userSvc := do.MustInvoke[*service.UserService](i)
		userFinder := &userFinderAdapter{userSvc: userSvc}

		// OrgResolver is optional (Org App may not be installed)
		orgResolver, _ := do.InvokeAs[app.OrgResolver](i)

		return NewGeneralToolRegistry(userFinder, orgResolver), nil
	})
}

func (a *AIApp) Routes(api *gin.RouterGroup) {
	providerH := do.MustInvoke[*ProviderHandler](a.injector)
	modelH := do.MustInvoke[*ModelHandler](a.injector)
	sourceH := do.MustInvoke[*KnowledgeSourceHandler](a.injector)

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

	// Knowledge — source pool (new)
	sources := api.Group("/ai/knowledge/sources")
	{
		sources.POST("/upload", sourceH.Upload)
		sources.POST("/url", sourceH.AddURL)
		sources.POST("/text", sourceH.AddText)
		sources.GET("", sourceH.List)
		sources.GET("/:id", sourceH.Get)
		sources.GET("/:id/content", sourceH.GetContent)
		sources.GET("/:id/references", sourceH.GetReferences)
		sources.DELETE("/:id", sourceH.Delete)
	}

	// Knowledge — type registry
	api.GET("/ai/knowledge/types", func(c *gin.Context) {
		category := c.Query("category")
		types := ListAssetTypes(category)
		handler.OK(c, types)
	})

	// Knowledge — graph assets (KG)
	graphH := do.MustInvoke[*KnowledgeGraphHandler](a.injector)
	graphs := api.Group("/ai/knowledge/graphs")
	{
		graphs.POST("", graphH.Create)
		graphs.GET("", graphH.List)
		graphs.GET("/:id", graphH.Get)
		graphs.PUT("/:id", graphH.Update)
		graphs.DELETE("/:id", graphH.Delete)
		// Source associations
		graphs.POST("/:id/sources", graphH.AddSources)
		graphs.GET("/:id/sources", graphH.ListSources)
		graphs.DELETE("/:id/sources/:sourceId", graphH.RemoveSource)
		// Build
		graphs.POST("/:id/build", graphH.Build)
		graphs.POST("/:id/rebuild", graphH.Rebuild)
		graphs.GET("/:id/progress", graphH.GetProgress)
		// Logs
		graphs.GET("/:id/logs", graphH.ListLogs)
		// Graph content
		graphs.GET("/:id/nodes", graphH.ListNodes)
		graphs.GET("/:id/nodes/:nodeId", graphH.GetNode)
		graphs.GET("/:id/nodes/:nodeId/subgraph", graphH.GetNodeSubgraph)
		graphs.GET("/:id/graph", graphH.GetFullGraph)
		// Search
		graphs.POST("/:id/search", graphH.Search)
	}

	// Knowledge — knowledge base assets (KB / RAG)
	kbH := do.MustInvoke[*KnowledgeKBHandler](a.injector)
	bases := api.Group("/ai/knowledge/bases")
	{
		bases.POST("", kbH.Create)
		bases.GET("", kbH.List)
		bases.GET("/:id", kbH.Get)
		bases.PUT("/:id", kbH.Update)
		bases.DELETE("/:id", kbH.Delete)
		// Source associations
		bases.POST("/:id/sources", kbH.AddSources)
		bases.GET("/:id/sources", kbH.ListSources)
		bases.DELETE("/:id/sources/:sourceId", kbH.RemoveSource)
		// Build
		bases.POST("/:id/build", kbH.Build)
		bases.POST("/:id/rebuild", kbH.Rebuild)
		bases.GET("/:id/progress", kbH.GetProgress)
		// Content
		bases.GET("/:id/chunks", kbH.ListChunks)
		// Logs
		bases.GET("/:id/logs", kbH.ListLogs)
		// Search
		bases.POST("/:id/search", kbH.Search)
	}

	// Tool registry
	toolH := do.MustInvoke[*ToolHandler](a.injector)
	tools := api.Group("/ai/tools")
	{
		tools.GET("", toolH.List)
		tools.PUT("/:id", toolH.Update)
		tools.PATCH("/:id/runtime", toolH.UpdateRuntime)
	}

	capabilitySetH := do.MustInvoke[*CapabilitySetHandler](a.injector)
	capabilitySets := api.Group("/ai/capability-sets")
	{
		capabilitySets.GET("", capabilitySetH.List)
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

	// Agent runtime (legacy — kept for internal use)
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

	// Typed agent routes
	assistantH := do.MustInvoke[*AssistantAgentHandler](a.injector)
	assistantAgents := api.Group("/ai/assistant-agents")
	{
		assistantAgents.POST("", assistantH.Create)
		assistantAgents.GET("", assistantH.List)
		assistantAgents.GET("/templates", assistantH.ListTemplates)
		assistantAgents.GET("/:id", assistantH.Get)
		assistantAgents.PUT("/:id", assistantH.Update)
		assistantAgents.DELETE("/:id", assistantH.Delete)
	}

	codingH := do.MustInvoke[*CodingAgentHandler](a.injector)
	codingAgents := api.Group("/ai/coding-agents")
	{
		codingAgents.POST("", codingH.Create)
		codingAgents.GET("", codingH.List)
		codingAgents.GET("/templates", codingH.ListTemplates)
		codingAgents.GET("/:id", codingH.Get)
		codingAgents.PUT("/:id", codingH.Update)
		codingAgents.DELETE("/:id", codingH.Delete)
	}

	sessionH := do.MustInvoke[*SessionHandler](a.injector)
	sessions := api.Group("/ai/sessions")
	{
		sessions.POST("", sessionH.Create)
		sessions.GET("", sessionH.List)
		sessions.GET("/:sid", sessionH.Get)
		sessions.PUT("/:sid", sessionH.Update)
		sessions.DELETE("/:sid", sessionH.Delete)
		sessions.POST("/:sid/chat", sessionH.Chat)
		sessions.POST("/:sid/messages", sessionH.SendMessage)
		sessions.PUT("/:sid/messages/:mid", sessionH.EditMessage)
		sessions.GET("/:sid/stream", sessionH.Stream)
		sessions.POST("/:sid/cancel", sessionH.Cancel)
		sessions.POST("/:sid/continue", sessionH.Continue)
		sessions.POST("/:sid/images", sessionH.UploadImage)
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
	nodeRepo := do.MustInvoke[*nodenode.NodeRepo](a.injector)
	queryH := do.MustInvoke[*KnowledgeQueryHandler](a.injector)

	sidecarAPI := r.Group("/api/v1/ai/sidecar", nodenode.NodeTokenMiddleware(nodeRepo))
	{
		sidecarAPI.POST("/knowledge/search", queryH.Search)
	}

	// Internal API for Agent to download skill packages (Node token auth)
	internal := r.Group("/api/v1/ai/internal", nodenode.NodeTokenMiddleware(nodeRepo))
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

// collectToolRegistries gathers all ToolHandlerRegistry instances from registered Apps.
// It always includes the GeneralToolRegistry first, then any App that implements
// app.ToolRegistryProvider (e.g. ITSM).
func collectToolRegistries(i do.Injector) []ToolHandlerRegistry {
	generalReg := do.MustInvoke[*GeneralToolRegistry](i)
	knowledgeReg := do.MustInvoke[*KnowledgeToolRegistry](i)

	var registries []ToolHandlerRegistry
	registries = append(registries, generalReg)
	registries = append(registries, knowledgeReg)

	// Discover registries from other Apps.
	for _, a := range app.All() {
		trp, ok := a.(app.ToolRegistryProvider)
		if !ok {
			continue
		}
		raw := trp.GetToolRegistry()
		if reg, ok := raw.(ToolHandlerRegistry); ok {
			registries = append(registries, reg)
			slog.Info("AI: discovered tool registry", "app", a.Name())
		}
	}

	return registries
}

func collectRuntimeContextProviders() []app.AgentRuntimeContextProvider {
	var providers []app.AgentRuntimeContextProvider
	for _, a := range app.All() {
		provider, ok := a.(app.AgentRuntimeContextProvider)
		if !ok {
			continue
		}
		providers = append(providers, provider)
		slog.Info("AI: discovered runtime context provider", "app", a.Name())
	}
	return providers
}

// --- userFinderAdapter bridges service.UserService to the UserFinder interface ---

type userFinderAdapter struct {
	userSvc *service.UserService
}

func (a *userFinderAdapter) FindByID(id uint) (*GeneralUserInfo, error) {
	user, err := a.userSvc.GetByIDWithManager(id)
	if err != nil {
		return nil, err
	}
	info := &GeneralUserInfo{
		ID:       user.ID,
		Username: user.Username,
		Email:    user.Email,
		Phone:    user.Phone,
		Avatar:   user.Avatar,
		RoleID:   user.RoleID,
	}
	if user.Role.ID > 0 {
		info.RoleName = user.Role.Name
		info.RoleCode = user.Role.Code
	}
	if user.ManagerID != nil {
		info.ManagerID = user.ManagerID
		if user.Manager != nil {
			info.ManagerUsername = user.Manager.Username
		}
	}
	return info, nil
}
