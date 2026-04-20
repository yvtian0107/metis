package ai

import (
	"log/slog"

	"github.com/casbin/casbin/v2"
	"gorm.io/gorm"

	"metis/internal/model"
)

func seedAI(db *gorm.DB, enforcer *casbin.Enforcer) error {
	upsertMenu := func(menu *model.Menu) error {
		var existing model.Menu
		if err := db.Where("permission = ?", menu.Permission).First(&existing).Error; err == nil {
			existing.ParentID = menu.ParentID
			existing.Name = menu.Name
			existing.Type = menu.Type
			existing.Path = menu.Path
			existing.Icon = menu.Icon
			existing.Sort = menu.Sort
			if err := db.Save(&existing).Error; err != nil {
				return err
			}
			menu.ID = existing.ID
			return nil
		}
		if err := db.Create(menu).Error; err != nil {
			return err
		}
		slog.Info("seed: created menu", "name", menu.Name, "permission", menu.Permission)
		return nil
	}

	seedButtons := func(parentID uint, buttons []model.Menu) {
		for _, btn := range buttons {
			var existing model.Menu
			if err := db.Where("permission = ?", btn.Permission).First(&existing).Error; err == nil {
				existing.ParentID = &parentID
				existing.Name = btn.Name
				existing.Type = btn.Type
				existing.Sort = btn.Sort
				if err := db.Save(&existing).Error; err != nil {
					slog.Error("seed: failed to update button menu", "permission", btn.Permission, "error", err)
				}
				continue
			}
			btn.ParentID = &parentID
			if err := db.Create(&btn).Error; err != nil {
				slog.Error("seed: failed to create button menu", "permission", btn.Permission, "error", err)
			}
		}
	}

	// 1. AI 管理目录
	var aiDir model.Menu
	if err := db.Where("permission = ?", "ai").First(&aiDir).Error; err != nil {
		aiDir = model.Menu{
			Name:       "AI 管理",
			Type:       model.MenuTypeDirectory,
			Icon:       "Brain",
			Permission: "ai",
			Sort:       100,
		}
		if err := db.Create(&aiDir).Error; err != nil {
			return err
		}
		slog.Info("seed: created menu", "name", aiDir.Name, "permission", aiDir.Permission)
	}

	agentDir := model.Menu{
		ParentID:   &aiDir.ID,
		Name:       "智能体",
		Type:       model.MenuTypeDirectory,
		Icon:       "Bot",
		Permission: "ai:group:agent",
		Sort:       0,
	}
	if err := upsertMenu(&agentDir); err != nil {
		return err
	}

	knowledgeDir := model.Menu{
		ParentID:   &aiDir.ID,
		Name:       "知识",
		Type:       model.MenuTypeDirectory,
		Icon:       "BookOpen",
		Permission: "ai:group:knowledge",
		Sort:       1,
	}
	if err := upsertMenu(&knowledgeDir); err != nil {
		return err
	}

	toolDir := model.Menu{
		ParentID:   &aiDir.ID,
		Name:       "工具",
		Type:       model.MenuTypeDirectory,
		Icon:       "Wrench",
		Permission: "ai:group:tool",
		Sort:       2,
	}
	if err := upsertMenu(&toolDir); err != nil {
		return err
	}

	modelAccessDir := model.Menu{
		ParentID:   &aiDir.ID,
		Name:       "模型接入",
		Type:       model.MenuTypeDirectory,
		Icon:       "Server",
		Permission: "ai:group:model-access",
		Sort:       3,
	}
	if err := upsertMenu(&modelAccessDir); err != nil {
		return err
	}

	// 2. 供应商管理菜单（含 inline 模型管理）
	var providerMenu model.Menu
	providerMenu = model.Menu{
		ParentID:   &modelAccessDir.ID,
		Name:       "供应商",
		Type:       model.MenuTypeMenu,
		Path:       "/ai/providers",
		Icon:       "Server",
		Permission: "ai:provider:list",
		Sort:       0,
	}
	if err := upsertMenu(&providerMenu); err != nil {
		return err
	}

	buttons := []model.Menu{
		{Name: "新增供应商", Type: model.MenuTypeButton, Permission: "ai:provider:create", Sort: 0},
		{Name: "编辑供应商", Type: model.MenuTypeButton, Permission: "ai:provider:update", Sort: 1},
		{Name: "删除供应商", Type: model.MenuTypeButton, Permission: "ai:provider:delete", Sort: 2},
		{Name: "连通测试", Type: model.MenuTypeButton, Permission: "ai:provider:test", Sort: 3},
		{Name: "新增模型", Type: model.MenuTypeButton, Permission: "ai:model:create", Sort: 4},
		{Name: "编辑模型", Type: model.MenuTypeButton, Permission: "ai:model:update", Sort: 5},
		{Name: "删除模型", Type: model.MenuTypeButton, Permission: "ai:model:delete", Sort: 6},
		{Name: "设为默认", Type: model.MenuTypeButton, Permission: "ai:model:default", Sort: 7},
		{Name: "同步模型", Type: model.MenuTypeButton, Permission: "ai:model:sync", Sort: 8},
	}
	seedButtons(providerMenu.ID, buttons)

	// 3. 知识库管理菜单
	var kbMenu model.Menu
	kbMenu = model.Menu{
		ParentID:   &knowledgeDir.ID,
		Name:       "知识库",
		Type:       model.MenuTypeMenu,
		Path:       "/ai/knowledge",
		Icon:       "BookOpen",
		Permission: "ai:knowledge:list",
		Sort:       0,
	}
	if err := upsertMenu(&kbMenu); err != nil {
		return err
	}

	kbButtons := []model.Menu{
		{Name: "新增知识库", Type: model.MenuTypeButton, Permission: "ai:knowledge:create", Sort: 0},
		{Name: "编辑知识库", Type: model.MenuTypeButton, Permission: "ai:knowledge:update", Sort: 1},
		{Name: "删除知识库", Type: model.MenuTypeButton, Permission: "ai:knowledge:delete", Sort: 2},
		{Name: "编译知识库", Type: model.MenuTypeButton, Permission: "ai:knowledge:compile", Sort: 3},
		{Name: "上传原料", Type: model.MenuTypeButton, Permission: "ai:knowledge:source:create", Sort: 4},
		{Name: "删除原料", Type: model.MenuTypeButton, Permission: "ai:knowledge:source:delete", Sort: 5},
	}
	seedButtons(kbMenu.ID, kbButtons)

	// 4. 工具菜单
	var builtinToolsMenu model.Menu
	builtinToolsMenu = model.Menu{
		ParentID:   &toolDir.ID,
		Name:       "内建工具",
		Type:       model.MenuTypeMenu,
		Path:       "/ai/tools/builtin",
		Icon:       "Wrench",
		Permission: "ai:tool:list",
		Sort:       0,
	}
	if err := upsertMenu(&builtinToolsMenu); err != nil {
		return err
	}

	builtinToolButtons := []model.Menu{
		{Name: "编辑内建工具", Type: model.MenuTypeButton, Permission: "ai:tool:update", Sort: 0},
	}
	seedButtons(builtinToolsMenu.ID, builtinToolButtons)

	var mcpMenu model.Menu
	mcpMenu = model.Menu{
		ParentID:   &toolDir.ID,
		Name:       "MCP 服务",
		Type:       model.MenuTypeMenu,
		Path:       "/ai/tools/mcp",
		Icon:       "PlugZap",
		Permission: "ai:mcp:list",
		Sort:       1,
	}
	if err := upsertMenu(&mcpMenu); err != nil {
		return err
	}

	mcpButtons := []model.Menu{
		{Name: "新增 MCP 服务", Type: model.MenuTypeButton, Permission: "ai:mcp:create", Sort: 1},
		{Name: "编辑 MCP 服务", Type: model.MenuTypeButton, Permission: "ai:mcp:update", Sort: 2},
		{Name: "删除 MCP 服务", Type: model.MenuTypeButton, Permission: "ai:mcp:delete", Sort: 3},
		{Name: "测试 MCP 连接", Type: model.MenuTypeButton, Permission: "ai:mcp:test", Sort: 4},
	}
	seedButtons(mcpMenu.ID, mcpButtons)

	var skillMenu model.Menu
	skillMenu = model.Menu{
		ParentID:   &toolDir.ID,
		Name:       "技能包",
		Type:       model.MenuTypeMenu,
		Path:       "/ai/tools/skills",
		Icon:       "Package",
		Permission: "ai:skill:list",
		Sort:       2,
	}
	if err := upsertMenu(&skillMenu); err != nil {
		return err
	}

	skillButtons := []model.Menu{
		{Name: "导入技能包", Type: model.MenuTypeButton, Permission: "ai:skill:create", Sort: 5},
		{Name: "编辑技能包", Type: model.MenuTypeButton, Permission: "ai:skill:update", Sort: 6},
		{Name: "删除技能包", Type: model.MenuTypeButton, Permission: "ai:skill:delete", Sort: 7},
	}
	seedButtons(skillMenu.ID, skillButtons)

	// 5. Agent 管理菜单 — 拆分为助手智能体和编码智能体
	var assistantAgentMenu model.Menu
	assistantAgentMenu = model.Menu{
		ParentID:   &agentDir.ID,
		Name:       "助手智能体",
		Type:       model.MenuTypeMenu,
		Path:       "/ai/assistant-agents",
		Icon:       "Bot",
		Permission: "ai:assistant-agent:list",
		Sort:       0,
	}
	if err := upsertMenu(&assistantAgentMenu); err != nil {
		return err
	}

	assistantButtons := []model.Menu{
		{Name: "新增助手智能体", Type: model.MenuTypeButton, Permission: "ai:assistant-agent:create", Sort: 0},
		{Name: "编辑助手智能体", Type: model.MenuTypeButton, Permission: "ai:assistant-agent:update", Sort: 1},
		{Name: "删除助手智能体", Type: model.MenuTypeButton, Permission: "ai:assistant-agent:delete", Sort: 2},
	}
	seedButtons(assistantAgentMenu.ID, assistantButtons)

	var codingAgentMenu model.Menu
	codingAgentMenu = model.Menu{
		ParentID:   &agentDir.ID,
		Name:       "编码智能体",
		Type:       model.MenuTypeMenu,
		Path:       "/ai/coding-agents",
		Icon:       "Code",
		Permission: "ai:coding-agent:list",
		Sort:       1,
	}
	if err := upsertMenu(&codingAgentMenu); err != nil {
		return err
	}

	codingButtons := []model.Menu{
		{Name: "新增编码智能体", Type: model.MenuTypeButton, Permission: "ai:coding-agent:create", Sort: 0},
		{Name: "编辑编码智能体", Type: model.MenuTypeButton, Permission: "ai:coding-agent:update", Sort: 1},
		{Name: "删除编码智能体", Type: model.MenuTypeButton, Permission: "ai:coding-agent:delete", Sort: 2},
	}
	seedButtons(codingAgentMenu.ID, codingButtons)

	// Soft-delete old "Agent" menu (ai:agent:list) if it still exists
	if err := db.Where("permission = ?", "ai:agent:list").Delete(&model.Menu{}).Error; err != nil {
		slog.Warn("seed: failed to cleanup old agent menu", "error", err)
	}
	// Also clean up old agent button menus
	for _, oldPerm := range []string{"ai:agent:create", "ai:agent:update", "ai:agent:delete"} {
		if err := db.Where("permission = ?", oldPerm).Delete(&model.Menu{}).Error; err != nil {
			slog.Warn("seed: failed to cleanup old agent button", "permission", oldPerm, "error", err)
		}
	}

	// 7. Agent templates seed
	agentTemplates := []AgentTemplate{
		// Assistant templates
		{
			Name:        "通用助手",
			Description: "通用 AI 助手，可回答问题、调用工具完成任务",
			Icon:        "Bot",
			Type:        AgentTypeAssistant,
			Config: model.JSONText(`{
				"strategy": "react",
				"systemPrompt": "你是一个通用 AI 助手，能够回答各种问题并帮助用户完成任务。",
				"temperature": 0.7,
				"maxTokens": 4096,
				"maxTurns": 10
			}`),
		},
		{
			Name:        "客服助手",
			Description: "面向客户的智能客服，挂载知识库回答产品问题",
			Icon:        "Headphones",
			Type:        AgentTypeAssistant,
			Config: model.JSONText(`{
				"strategy": "react",
				"systemPrompt": "你是一个专业的客服助手。请根据知识库中的信息回答客户问题，保持友好、专业的态度。如果不确定答案，请坦诚告知。",
				"temperature": 0.3,
				"maxTokens": 2048,
				"maxTurns": 5
			}`),
		},
		{
			Name:        "运维助手",
			Description: "运维 AI 助手，帮助排查故障、分析日志、调用监控工具",
			Icon:        "Terminal",
			Type:        AgentTypeAssistant,
			Config: model.JSONText(`{
				"strategy": "plan_and_execute",
				"systemPrompt": "你是一个运维助手，擅长排查线上故障、分析日志、查看监控数据。先制定排查计划，再逐步执行。",
				"temperature": 0.5,
				"maxTokens": 4096,
				"maxTurns": 15
			}`),
		},
		{
			Name:        "探索助手",
			Description: "思维伙伴，帮助探索创意、梳理需求、头脑风暴",
			Icon:        "Compass",
			Type:        AgentTypeAssistant,
			Config: model.JSONText(`{
				"strategy": "react",
				"systemPrompt": "你是一个探索型助手，擅长帮助用户梳理想法、发散思维、整理需求。用提问引导用户深入思考，而不是直接给出答案。",
				"temperature": 0.9,
				"maxTokens": 4096,
				"maxTurns": 20
			}`),
		},
		{
			Name:        "支持助手",
			Description: "内部技术支持助手，帮助团队成员解决技术问题",
			Icon:        "LifeBuoy",
			Type:        AgentTypeAssistant,
			Config: model.JSONText(`{
				"strategy": "react",
				"systemPrompt": "你是一个内部技术支持助手，帮助团队成员解决日常技术问题。优先参考知识库中的内部文档，给出具体可操作的解决方案。",
				"temperature": 0.5,
				"maxTokens": 4096,
				"maxTurns": 10
			}`),
		},
		// Coding templates
		{
			Name:        "Claude Code",
			Description: "基于 Claude Code 的编码智能体，适合复杂代码生成与重构",
			Icon:        "Code",
			Type:        AgentTypeCoding,
			Config: model.JSONText(`{
				"runtime": "claude-code",
				"execMode": "local"
			}`),
		},
		{
			Name:        "OpenCode",
			Description: "基于 OpenCode 的编码智能体，轻量级终端编程助手",
			Icon:        "Terminal",
			Type:        AgentTypeCoding,
			Config: model.JSONText(`{
				"runtime": "opencode",
				"execMode": "local"
			}`),
		},
		{
			Name:        "Codex",
			Description: "基于 OpenAI Codex 的编码智能体，擅长代码补全与生成",
			Icon:        "Cpu",
			Type:        AgentTypeCoding,
			Config: model.JSONText(`{
				"runtime": "codex",
				"execMode": "local"
			}`),
		},
		{
			Name:        "Aider",
			Description: "基于 Aider 的编码智能体，适合 Git 仓库内的增量编码",
			Icon:        "GitBranch",
			Type:        AgentTypeCoding,
			Config: model.JSONText(`{
				"runtime": "aider",
				"execMode": "local"
			}`),
		},
	}
	for _, tmpl := range agentTemplates {
		var existing AgentTemplate
		if err := db.Where("name = ?", tmpl.Name).First(&existing).Error; err != nil {
			if err := db.Create(&tmpl).Error; err != nil {
				slog.Error("seed: failed to create agent template", "name", tmpl.Name, "error", err)
				continue
			}
			slog.Info("seed: created agent template", "name", tmpl.Name)
		}
	}

	// 8. Builtin tools seed
	builtinTools := []Tool{
		{
			Toolkit:     "knowledge",
			Name:        "search_knowledge",
			DisplayName: "Search Knowledge",
			Description: "Search for relevant documents in a knowledge base using full-text search.",
			ParametersSchema: model.JSONText(`{
				"type": "object",
				"properties": {
					"knowledge_base_id": {"type": "integer", "description": "The ID of the knowledge base to search"},
					"query": {"type": "string", "description": "The search query"}
				},
				"required": ["knowledge_base_id", "query"]
			}`),
			IsActive: true,
		},
		{
			Toolkit:     "knowledge",
			Name:        "read_document",
			DisplayName: "Read Document",
			Description: "Read the full content of a specific document from a knowledge base.",
			ParametersSchema: model.JSONText(`{
				"type": "object",
				"properties": {
					"knowledge_base_id": {"type": "integer", "description": "The ID of the knowledge base"},
					"node_id": {"type": "integer", "description": "The ID of the knowledge node to read"}
				},
				"required": ["knowledge_base_id", "node_id"]
			}`),
			IsActive: true,
		},
		{
			Toolkit:     "network",
			Name:        "http_request",
			DisplayName: "HTTP Request",
			Description: "Make an HTTP request to an external URL and return the response.",
			ParametersSchema: model.JSONText(`{
				"type": "object",
				"properties": {
					"method": {"type": "string", "enum": ["GET", "POST", "PUT", "DELETE", "PATCH"], "description": "HTTP method"},
					"url": {"type": "string", "description": "The URL to request"},
					"headers": {"type": "object", "description": "Request headers"},
					"body": {"type": "string", "description": "Request body (for POST/PUT/PATCH)"}
				},
				"required": ["method", "url"]
			}`),
			IsActive: false,
		},
		{
			Toolkit:     "code",
			Name:        "execute_script",
			DisplayName: "Execute Script",
			Description: "Execute a script in a sandboxed environment and return stdout/stderr.",
			ParametersSchema: model.JSONText(`{
				"type": "object",
				"properties": {
					"language": {"type": "string", "enum": ["python", "bash"], "description": "Script language"},
					"code": {"type": "string", "description": "The script source code to execute"},
					"timeout": {"type": "integer", "description": "Execution timeout in seconds (default 30, max 300)"}
				},
				"required": ["language", "code"]
			}`),
			IsActive: false,
		},
		{
			Toolkit:     "general",
			Name:        "general.current_time",
			DisplayName: "获取当前时间",
			Description: "获取当前时间。支持传入标准 IANA 时区名（如 Asia/Shanghai），返回服务端时间、UTC 时间、中国时间和目标时区时间。",
			ParametersSchema: model.JSONText(`{
				"type": "object",
				"properties": {
					"timezone": {"type": "string", "description": "IANA 时区名（如 Asia/Shanghai），可选"}
				}
			}`),
			IsActive: true,
		},
		{
			Toolkit:     "general",
			Name:        "system.current_user_profile",
			DisplayName: "获取当前用户信息",
			Description: "读取当前提单用户的基础资料和组织归属（部门、岗位、角色），帮助服务台用已有信息补齐提单上下文。",
			ParametersSchema: model.JSONText(`{
				"type": "object",
				"properties": {}
			}`),
			IsActive: true,
		},
	}
	for _, tool := range builtinTools {
		var existing Tool
		if err := db.Where("name = ?", tool.Name).First(&existing).Error; err != nil {
			if err := db.Create(&tool).Error; err != nil {
				slog.Error("seed: failed to create builtin tool", "name", tool.Name, "error", err)
				continue
			}
			slog.Info("seed: created builtin tool", "name", tool.Name)
		} else if existing.Toolkit != tool.Toolkit {
			db.Model(&existing).Update("toolkit", tool.Toolkit)
			slog.Info("seed: updated toolkit for builtin tool", "name", tool.Name, "toolkit", tool.Toolkit)
		}
	}

	// 6. Casbin policies
	policies := [][]string{
		// Providers
		{"admin", "/api/v1/ai/providers", "GET"},
		{"admin", "/api/v1/ai/providers", "POST"},
		{"admin", "/api/v1/ai/providers/:id", "GET"},
		{"admin", "/api/v1/ai/providers/:id", "PUT"},
		{"admin", "/api/v1/ai/providers/:id", "DELETE"},
		{"admin", "/api/v1/ai/providers/:id/test", "POST"},
		{"admin", "/api/v1/ai/providers/:id/sync-models", "POST"},
		// Models
		{"admin", "/api/v1/ai/models", "GET"},
		{"admin", "/api/v1/ai/models", "POST"},
		{"admin", "/api/v1/ai/models/:id", "GET"},
		{"admin", "/api/v1/ai/models/:id", "PUT"},
		{"admin", "/api/v1/ai/models/:id", "DELETE"},
		{"admin", "/api/v1/ai/models/:id/default", "PATCH"},
		// Knowledge bases
		{"admin", "/api/v1/ai/knowledge-bases", "GET"},
		{"admin", "/api/v1/ai/knowledge-bases", "POST"},
		{"admin", "/api/v1/ai/knowledge-bases/:id", "GET"},
		{"admin", "/api/v1/ai/knowledge-bases/:id", "PUT"},
		{"admin", "/api/v1/ai/knowledge-bases/:id", "DELETE"},
		{"admin", "/api/v1/ai/knowledge-bases/:id/compile", "POST"},
		{"admin", "/api/v1/ai/knowledge-bases/:id/recompile", "POST"},
		{"admin", "/api/v1/ai/knowledge-bases/:id/progress", "GET"},
		// Knowledge sources
		{"admin", "/api/v1/ai/knowledge-bases/:id/sources", "GET"},
		{"admin", "/api/v1/ai/knowledge-bases/:id/sources", "POST"},
		{"admin", "/api/v1/ai/knowledge-bases/:id/sources/:sid", "GET"},
		{"admin", "/api/v1/ai/knowledge-bases/:id/sources/:sid", "DELETE"},
		// Knowledge nodes & logs
		{"admin", "/api/v1/ai/knowledge-bases/:id/nodes", "GET"},
		{"admin", "/api/v1/ai/knowledge-bases/:id/nodes/:nid", "GET"},
		{"admin", "/api/v1/ai/knowledge-bases/:id/nodes/:nid/graph", "GET"},
		{"admin", "/api/v1/ai/knowledge-bases/:id/logs", "GET"},
		{"admin", "/api/v1/ai/knowledge-bases/:id/graph", "GET"},
		{"admin", "/api/v1/ai/knowledge-bases/:id/search", "GET"},
		// Tools
		{"admin", "/api/v1/ai/tools", "GET"},
		{"admin", "/api/v1/ai/tools/:id", "PUT"},
		// MCP Servers
		{"admin", "/api/v1/ai/mcp-servers", "GET"},
		{"admin", "/api/v1/ai/mcp-servers", "POST"},
		{"admin", "/api/v1/ai/mcp-servers/:id", "GET"},
		{"admin", "/api/v1/ai/mcp-servers/:id", "PUT"},
		{"admin", "/api/v1/ai/mcp-servers/:id", "DELETE"},
		{"admin", "/api/v1/ai/mcp-servers/:id/test", "POST"},
		// Skills
		{"admin", "/api/v1/ai/skills", "GET"},
		{"admin", "/api/v1/ai/skills/:id", "GET"},
		{"admin", "/api/v1/ai/skills/import-github", "POST"},
		{"admin", "/api/v1/ai/skills/upload", "POST"},
		{"admin", "/api/v1/ai/skills/:id", "PUT"},
		{"admin", "/api/v1/ai/skills/:id/active", "PATCH"},
		{"admin", "/api/v1/ai/skills/:id", "DELETE"},
		// Agents (legacy — kept for internal use)
		{"admin", "/api/v1/ai/agents", "GET"},
		{"admin", "/api/v1/ai/agents", "POST"},
		{"admin", "/api/v1/ai/agents/templates", "GET"},
		{"admin", "/api/v1/ai/agents/:id", "GET"},
		{"admin", "/api/v1/ai/agents/:id", "PUT"},
		{"admin", "/api/v1/ai/agents/:id", "DELETE"},
		{"admin", "/api/v1/ai/agents/:id/memories", "GET"},
		{"admin", "/api/v1/ai/agents/:id/memories", "POST"},
		{"admin", "/api/v1/ai/agents/:id/memories/:mid", "DELETE"},
		// Assistant agents (typed)
		{"admin", "/api/v1/ai/assistant-agents", "GET"},
		{"admin", "/api/v1/ai/assistant-agents", "POST"},
		{"admin", "/api/v1/ai/assistant-agents/templates", "GET"},
		{"admin", "/api/v1/ai/assistant-agents/:id", "GET"},
		{"admin", "/api/v1/ai/assistant-agents/:id", "PUT"},
		{"admin", "/api/v1/ai/assistant-agents/:id", "DELETE"},
		// Coding agents (typed)
		{"admin", "/api/v1/ai/coding-agents", "GET"},
		{"admin", "/api/v1/ai/coding-agents", "POST"},
		{"admin", "/api/v1/ai/coding-agents/templates", "GET"},
		{"admin", "/api/v1/ai/coding-agents/:id", "GET"},
		{"admin", "/api/v1/ai/coding-agents/:id", "PUT"},
		{"admin", "/api/v1/ai/coding-agents/:id", "DELETE"},
		// Sessions
		{"admin", "/api/v1/ai/sessions", "GET"},
		{"admin", "/api/v1/ai/sessions", "POST"},
		{"admin", "/api/v1/ai/sessions/:sid", "GET"},
		{"admin", "/api/v1/ai/sessions/:sid", "DELETE"},
		{"admin", "/api/v1/ai/sessions/:sid/messages", "POST"},
		{"admin", "/api/v1/ai/sessions/:sid/stream", "GET"},
		{"admin", "/api/v1/ai/sessions/:sid/cancel", "POST"},
		{"admin", "/api/v1/ai/sessions/:sid/images", "POST"},
	}

	menuPerms := [][]string{
		{"admin", "ai", "read"},
		{"admin", "ai:group:agent", "read"},
		{"admin", "ai:group:knowledge", "read"},
		{"admin", "ai:group:tool", "read"},
		{"admin", "ai:group:model-access", "read"},
		{"admin", "ai:provider:list", "read"},
		{"admin", "ai:provider:create", "read"},
		{"admin", "ai:provider:update", "read"},
		{"admin", "ai:provider:delete", "read"},
		{"admin", "ai:provider:test", "read"},
		{"admin", "ai:model:create", "read"},
		{"admin", "ai:model:update", "read"},
		{"admin", "ai:model:delete", "read"},
		{"admin", "ai:model:default", "read"},
		{"admin", "ai:model:sync", "read"},
		{"admin", "ai:knowledge:list", "read"},
		{"admin", "ai:knowledge:create", "read"},
		{"admin", "ai:knowledge:update", "read"},
		{"admin", "ai:knowledge:delete", "read"},
		{"admin", "ai:knowledge:compile", "read"},
		{"admin", "ai:knowledge:source:create", "read"},
		{"admin", "ai:knowledge:source:delete", "read"},
		// Tool registry
		{"admin", "ai:tool:list", "read"},
		{"admin", "ai:tool:update", "read"},
		{"admin", "ai:mcp:list", "read"},
		{"admin", "ai:mcp:create", "read"},
		{"admin", "ai:mcp:update", "read"},
		{"admin", "ai:mcp:delete", "read"},
		{"admin", "ai:mcp:test", "read"},
		{"admin", "ai:skill:list", "read"},
		{"admin", "ai:skill:create", "read"},
		{"admin", "ai:skill:update", "read"},
		{"admin", "ai:skill:delete", "read"},
		// Agent (legacy — kept for internal use)
		{"admin", "ai:agent:list", "read"},
		{"admin", "ai:agent:create", "read"},
		{"admin", "ai:agent:update", "read"},
		{"admin", "ai:agent:delete", "read"},
		// Assistant agent
		{"admin", "ai:assistant-agent:list", "read"},
		{"admin", "ai:assistant-agent:create", "read"},
		{"admin", "ai:assistant-agent:update", "read"},
		{"admin", "ai:assistant-agent:delete", "read"},
		// Coding agent
		{"admin", "ai:coding-agent:list", "read"},
		{"admin", "ai:coding-agent:create", "read"},
		{"admin", "ai:coding-agent:update", "read"},
		{"admin", "ai:coding-agent:delete", "read"},
	}

	allPolicies := append(policies, menuPerms...)
	for _, p := range allPolicies {
		if has, _ := enforcer.HasPolicy(p); !has {
			if _, err := enforcer.AddPolicy(p); err != nil {
				slog.Error("seed: failed to add policy", "policy", p, "error", err)
			}
		}
	}

	// Permission migration: detect non-admin roles with old ai:agent:* perms,
	// grant equivalent new ai:assistant-agent:* + ai:coding-agent:* perms, then remove old perms.
	oldAgentPerms := []string{"ai:agent:list", "ai:agent:create", "ai:agent:update", "ai:agent:delete"}
	for _, oldPerm := range oldAgentPerms {
		// Find all roles that have this old permission
		existingPolicies, _ := enforcer.GetFilteredPolicy(1, oldPerm, "read")
		for _, ep := range existingPolicies {
			role := ep[0]
			if role == "admin" {
				continue // admin already has new perms from the seed above
			}
			suffix := oldPerm[len("ai:agent:"):]
			// Grant new typed perms
			newPerms := []string{
				"ai:assistant-agent:" + suffix,
				"ai:coding-agent:" + suffix,
			}
			for _, np := range newPerms {
				if has, _ := enforcer.HasPolicy([]string{role, np, "read"}); !has {
					if _, err := enforcer.AddPolicy([]string{role, np, "read"}); err != nil {
						slog.Error("seed: failed to migrate agent permission", "role", role, "perm", np, "error", err)
					} else {
						slog.Info("seed: migrated agent permission", "role", role, "old", oldPerm, "new", np)
					}
				}
			}
			// Remove old permission
			if _, err := enforcer.RemovePolicy([]string{role, oldPerm, "read"}); err != nil {
				slog.Warn("seed: failed to remove old agent permission", "role", role, "perm", oldPerm, "error", err)
			}
		}
	}

	// Cleanup: remove deprecated "对话" menu (merged into Agent cards)
	if err := db.Where("permission = ?", "ai:chat").Delete(&model.Menu{}).Error; err != nil {
		slog.Warn("seed: failed to cleanup chat menu", "error", err)
	}
	if has, _ := enforcer.HasPolicy([]string{"admin", "ai:chat", "read"}); has {
		if _, err := enforcer.RemovePolicy([]string{"admin", "ai:chat", "read"}); err != nil {
			slog.Warn("seed: failed to remove ai:chat policy", "error", err)
		}
	}

	return nil
}
