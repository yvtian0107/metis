package tools

import (
	"encoding/json"
	"log/slog"

	"gorm.io/gorm"

	"metis/internal/app/itsm/engine"
)

// ITSMTool defines a tool that ITSM registers into the ai_tools table.
type ITSMTool struct {
	Name             string
	DisplayName      string
	Description      string
	ParametersSchema json.RawMessage
}

// AllTools returns the ITSM tool definitions.
func AllTools() []ITSMTool {
	tools := []ITSMTool{
		{
			Name:        "itsm.service_match",
			DisplayName: "服务匹配",
			Description: "读取 ITSM 中的服务目录与服务扁平结构，返回 0~3 个最匹配的服务候选。",
			ParametersSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"query": {"type": "string", "description": "用户描述的需求（自然语言）"}
				},
				"required": ["query"]
			}`),
		},
		{
			Name:        "itsm.service_confirm",
			DisplayName: "服务确认",
			Description: "当服务匹配返回多个候选且用户明确选择某个候选时，确认并锁定为当前后续操作目标。service_id 是 service_match 返回的 matches 数组中对应项的 id 字段。",
			ParametersSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"service_id": {"type": "integer", "description": "用户选择的服务 ID（必须在 service_match 返回的候选列表中）"}
				},
				"required": ["service_id"]
			}`),
		},
		{
			Name:        "itsm.service_load",
			DisplayName: "服务加载",
			Description: "加载指定服务的协作规范、表单定义和动作配置；协作规范为主，流程图 JSON 仅参考。",
			ParametersSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"service_id": {"type": "integer", "description": "服务定义 ID"}
				},
				"required": ["service_id"]
			}`),
		},
		{
			Name:        "itsm.new_request",
			DisplayName: "新申请上下文",
			Description: "当用户明确表示再次申请、再提一张或开始新的工单时，重置当前服务台会话中的旧草稿和旧服务上下文。",
			ParametersSchema: json.RawMessage(`{
				"type": "object",
				"properties": {}
			}`),
		},
		{
			Name:        "itsm.draft_prepare",
			DisplayName: "草稿整理",
			Description: "在向用户展示任何拟提单草稿前必须先调用：登记当前草稿版本，并进入等待用户确认状态。",
			ParametersSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"summary": {"type": "string", "description": "工单摘要"},
					"form_data": {"type": "object", "description": "表单字段键值对（必须是完整表单，不能只传增量）"}
				},
				"required": ["summary", "form_data"]
			}`),
		},
		{
			Name:        "itsm.draft_confirm",
			DisplayName: "草稿确认",
			Description: "只有在用户明确确认当前草稿版本后才能调用；调用后才允许继续执行工单创建。",
			ParametersSchema: json.RawMessage(`{
				"type": "object",
				"properties": {}
			}`),
		},
		{
			Name:        "itsm.validate_participants",
			DisplayName: "参与者预检",
			Description: "在创建工单前校验审批参与者是否可达。对于有路由分支的服务，需传入 form_data 以定位将被激活的分支并检查其参与者。",
			ParametersSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"service_id": {"type": "integer", "description": "服务定义 ID"},
					"form_data": {"type": "object", "description": "表单数据（用于确定路由分支）"}
				},
				"required": ["service_id", "form_data"]
			}`),
		},
		{
			Name:        "itsm.ticket_create",
			DisplayName: "工单创建",
			Description: "在信息收集完成且草稿已确认后，将服务请求创建为 ITSM 工单。",
			ParametersSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"service_id": {"type": "integer", "description": "服务定义 ID"},
					"summary": {"type": "string", "description": "工单摘要"},
					"form_data": {"type": "object", "description": "完整表单数据"}
				},
				"required": ["service_id", "summary"]
			}`),
		},
		{
			Name:        "itsm.my_tickets",
			DisplayName: "我的工单查询",
			Description: "查询当前对话用户正在申请中的工单列表，返回工单号、摘要、状态、服务名等信息。",
			ParametersSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"status": {"type": "string", "description": "状态筛选（可选）"}
				}
			}`),
		},
		{
			Name:        "itsm.ticket_withdraw",
			DisplayName: "工单撤回",
			Description: "通过工单号撤回指定工单，仅申请人在工单尚未被处理时可撤回。",
			ParametersSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"ticket_code": {"type": "string", "description": "工单编号"},
					"reason": {"type": "string", "description": "撤回原因（可选）"}
				},
				"required": ["ticket_code"]
			}`),
		},
	}
	for _, def := range engine.DecisionToolDefs() {
		params, err := json.Marshal(def.Parameters)
		if err != nil {
			slog.Warn("ITSM tools seed: failed to marshal decision tool schema", "name", def.Name, "error", err)
			continue
		}
		tools = append(tools, ITSMTool{
			Name:             def.Name,
			DisplayName:      def.Name,
			Description:      def.Description,
			ParametersSchema: params,
		})
	}
	return tools
}

// SeedTools registers ITSM tool definitions into the ai_tools table.
// It cleans up deprecated tools and upserts the current tool set.
func SeedTools(db *gorm.DB) error {
	// Check if ai_tools table is available
	var count int64
	if err := db.Table("ai_tools").Count(&count).Error; err != nil {
		slog.Info("ITSM tools seed: ai_tools table not available, skipping", "error", err)
		return nil
	}

	// Cleanup deprecated tools
	deprecatedTools := []string{"itsm.search_services", "itsm.query_ticket", "itsm.cancel_ticket", "itsm.add_comment"}
	for _, name := range deprecatedTools {
		var toolID uint
		if err := db.Table("ai_tools").Where("name = ?", name).Select("id").Row().Scan(&toolID); err == nil && toolID > 0 {
			db.Table("ai_agent_tools").Where("tool_id = ?", toolID).Delete(nil)
			db.Table("ai_tools").Where("id = ?", toolID).Delete(nil)
			slog.Info("ITSM tools seed: removed deprecated tool", "name", name)
		}
	}

	// Upsert current tools
	for _, tool := range AllTools() {
		var existing struct{ ID uint }
		if err := db.Table("ai_tools").Where("name = ?", tool.Name).Select("id").First(&existing).Error; err == nil {
			// Update existing
			db.Table("ai_tools").Where("id = ?", existing.ID).Updates(map[string]any{
				"display_name":      tool.DisplayName,
				"description":       tool.Description,
				"parameters_schema": string(tool.ParametersSchema),
			})
			slog.Info("ITSM tools seed: updated tool", "name", tool.Name)
		} else {
			// Create new
			toolkit := "itsm"
			if len(tool.Name) > 9 && tool.Name[:9] == "decision." {
				toolkit = "decision"
			}
			if err := db.Table("ai_tools").Create(map[string]any{
				"toolkit":           toolkit,
				"name":              tool.Name,
				"display_name":      tool.DisplayName,
				"description":       tool.Description,
				"parameters_schema": string(tool.ParametersSchema),
				"is_active":         true,
			}).Error; err != nil {
				slog.Error("ITSM tools seed: failed to create tool", "name", tool.Name, "error", err)
				continue
			}
			slog.Info("ITSM tools seed: created tool", "name", tool.Name)
		}
	}

	return nil
}

// presetAgent defines a preset agent to seed.
type presetAgent struct {
	Name         string
	Code         string
	Description  string
	Type         string
	Visibility   string
	Strategy     string
	SystemPrompt string
	Temperature  float64
	MaxTokens    int
	MaxTurns     int
	ToolNames    []string // tool names to bind (can include non-ITSM tools)
}

// SeedAgents creates preset ITSM agents in the ai_agents table.
// Skips entirely if ai_agents table doesn't exist (AI App not installed).
func SeedAgents(db *gorm.DB) error {
	// Check if ai_agents table exists
	var count int64
	if err := db.Table("ai_agents").Count(&count).Error; err != nil {
		slog.Info("ITSM agent seed: ai_agents table not available, skipping")
		return nil
	}

	// Delete old agents and their tool bindings
	oldAgentNames := []string{"IT 服务台", "ITSM 流程决策", "ITSM 处理协助"}
	for _, name := range oldAgentNames {
		var agentID uint
		if err := db.Table("ai_agents").Where("name = ?", name).Select("id").Row().Scan(&agentID); err == nil && agentID > 0 {
			db.Table("ai_agent_tools").Where("agent_id = ?", agentID).Delete(nil)
			db.Table("ai_agents").Where("id = ?", agentID).Delete(nil)
			slog.Info("ITSM agent seed: removed old agent", "name", name)
		}
	}

	agents := []presetAgent{
		{
			Name:        "IT 服务台智能体",
			Code:        "itsm.servicedesk",
			Description: "IT 服务台智能体，引导用户完成服务匹配、信息收集、草稿确认与工单创建的全流程",
			Type:        "assistant",
			Visibility:  "public",
			Strategy:    "react",
			Temperature: 0.3,
			MaxTokens:   4096,
			MaxTurns:    20,
			SystemPrompt: `你是 IT 服务台智能体。你要像一位专业、耐心、有人味的服务台同事那样帮助用户处理日常咨询与提单协助，同时严格遵守 ITSM 流程。

你的服务风格：
1. 语气友好、自然、简洁，优先给用户清晰的下一步，而不是生硬地下指令
2. 可以用简短的人类化表达体现理解和协助意图，但不要闲聊、不要啰嗦、不要失去专业性
3. 当信息不足、服务不确定或流程被阻塞时，要坦诚说明原因，并温和地引导用户继续补充或确认
4. 不要假装已经完成尚未完成的动作，也不要把内部推理包装成既成事实

工作模式：
1. 如果用户是在做日常问答、流程咨询、服务目录咨询、状态确认、规则解释或使用指导，而不是要立即推进提单，你可以先直接友好回答，必要时再引导到合适的服务或下一步
2. 如果用户明确要办理服务、整理申请草稿、补全字段、修改草稿、确认草稿或推进提单，就进入"提单推进模式"
3. 不要为了维持流程感而把普通咨询强行升级成提单；也不要在用户已经明确要提单时只停留在泛泛解释

进入"提单推进模式"后，你的工作顺序必须是：
1. 先理解用户诉求
2. 优先调用个人信息工具读取当前提单用户的已有资料和组织归属
3. 调用服务匹配工具找出 0~3 个候选服务；如果证据不足，就宁可少给也不要勉强匹配。你必须把当前识别到的服务名称、匹配原因和是否需要确认明确告诉用户，不能只说"已完成服务匹配"；若 service_match 返回空列表，应告知用户当前无匹配服务，建议换个描述再试或联系 IT 管理员，不允许在无真实 service_id 的情况下继续推进
4. 仅当 service_match 返回 confirmation_required=false 时，可直接调用服务加载工具读取协作规范；若返回 confirmation_required=true，必须先向用户展示候选、等待确认，调用 itsm.service_confirm 锁定选择后再调用 itsm.service_load
5. 基于个人信息工具返回结果，只补问缺失字段和服务特有字段；不要反复询问已经明确的信息
6. 每次整理出拟提单摘要和表单字段后，都要把它当作"当前草稿版本"；若用户补充、修改或纠正任何字段，就必须把它视为新版本草稿并重新展示
7. 只要你准备向用户展示任何"拟提单 / 草稿 / 拟提交信息"，无论是第一次展示还是更新后的版本，都必须先调用 itsm.draft_prepare；严禁先口头展示、再事后补调
8. 如果用户在看完草稿后又修改了字段，你必须再次调用 itsm.draft_prepare 生成并展示新版本草稿，不能沿用旧确认
8.1 调用 itsm.draft_prepare 时，summary 和 form_data 必须对应"当前最新草稿"；其中 form_data 必须是当前已知完整表单，禁止传空对象、禁止遗漏未变更字段、不能只传本轮增量字段
8.2 如果用户在同一个会话里开始一张新的工单（尤其是说"再次申请""再提一张""新开一张"），必须先调用 itsm.new_request 重置旧草稿和旧服务上下文，再重新匹配或确认服务，不能复用上一张工单的草稿字段
9. 只有当用户明确确认你刚刚展示的当前版本草稿后，才能先调用 itsm.draft_confirm，再调用工单创建工具建单；禁止跳过 itsm.draft_confirm 直接调用 itsm.ticket_create
10. 调用工单创建工具时，service_id 必须直接使用服务匹配工具或服务加载工具返回的真实服务 id，必须是大于 0 的整数，绝不允许填写 0、空值、占位符或自行编造的 id

严格约束：
1. 个人信息工具只用于预填上下文，不足的信息仍然要继续追问
2. 在提单推进模式下，不要跳过服务匹配直接建单
3. 协作规范是主依据，流程图 JSON 仅作为参考
4. 当信息不足时，明确询问用户缺失字段；不要自行脑补
5. 未获得用户明确确认前，不允许创建工单
6. 不允许依赖固定关键字做确认判断，必须把"当前版本草稿"与用户最新回复对齐；只要草稿变了，就必须重新展示并重新确认
7. 用户的模糊认可、继续补充信息、局部修改或追问，都不能视为对最新草稿版本的最终确认
8. 工单创建前的标准顺序是：itsm.service_match ->（confirmation_required=true 时必须先调用 itsm.service_confirm）-> itsm.service_load -> itsm.draft_prepare ->（用户明确确认后）itsm.draft_confirm -> itsm.validate_participants -> itsm.ticket_create；如果顺序不满足，就继续补工具调用，不要直接输出最终提交结果
9. 工单创建工具会校验"已加载当前服务 + 已确认当前草稿版本"；不要在未满足前置条件时尝试碰运气调用
10. 如果服务匹配或服务加载还没有给出真实服务 id，就先补做前置工具调用，不允许硬填 service_id=0
10.1 当 service_match 返回 confirmation_required=true 时：先向用户展示候选并等待选择；用户确认后调用 itsm.service_confirm(service_id) 记录选择，再调用 itsm.service_load；任何候选都不可跳过这一步。用户以序号回复（"1""第一个""前者""后者"等）时，从 matches 数组取对应位置的 id 字段传入，不得把序号数字本身当作 service_id
11. 在调用 itsm.draft_prepare 之前，必须先根据 service_load 返回的 routing_field_hint 中的 option_route_map 判断用户的诉求是否跨越了多条路由分支。如果用户同时提到了映射到不同审批路径的多种需求，你必须主动向用户说明这些需求分属不同审批路径，请用户明确选择当前要办理哪一个，而不是替用户做选择、忽略冲突或直接把冲突数据提交给 draft_prepare
12. 当用户提到多个访问原因或类别，但它们全部映射到同一个路由分支，应合并为该分支对应的结构化字段值并继续流程，不要求用户二选一；同时将用户提到的所有具体原因完整写入 summary 及服务表单中描述访问目的或申请原因的字段，确保工单详情中可见用户完整诉求
13. 在调用 itsm.draft_prepare 前，先对照 service_load 返回的字段定义检查所有必填字段是否已收集；如果有必填字段缺失，必须先向用户追问缺失字段，不得带着空字段调用 itsm.draft_prepare
14. 如果 itsm.draft_prepare 返回的 warnings 中包含 multivalue_on_single_field，说明你在一个单选字段中传入了多个值。根据 resolved_values 判断这些值是否属于同一路由分支：若同路由，将路由字段修正为对应的结构化单值，将用户原始的多个原因完整写入 summary 及服务表单中描述访问目的或申请原因的字段，然后重新调用 itsm.draft_prepare；若跨路由，按规则 11 处理
15. 在调用 itsm.draft_prepare 前，检查 form_data 中是否有访问时段、执行窗口、生效时间或任何含时间意义的字段。只要字段值不是完整的绝对时间（即缺少具体年月日，如"今晚""明天""20:00到21:00"仅有钟表时间无日期、"下周一"、"X小时后"等），一律必须先调用 general.current_time，以返回的 china_formatted_time 为基准完成解析，将其转换为完整的含年月日时分的绝对时间（中国本地时间，UTC+8）后再写入 form_data，方可调用 draft_prepare。这是格式要求而非理解问题——即便你能推断出时间含义，也必须先获取基准时间并完成绝对化，不允许以相对表达或纯钟表时间写入 form_data。解析时间范围时，每个端点必须独立以 china_formatted_time 为基准锚定，不得为了使范围"看起来合理"而将任一端点的日期挪至下一个日历日；例如用户写"今晚 24:00 到 23:00"，24:00 应解析为 2026-04-16 00:00，23:00 应独立解析为 2026-04-15 23:00，而非 2026-04-16 23:00。日期中缺少年份时（如"4.12""3月5号"），必须使用工具返回的 china_formatted_time 中的年份来补全，不得依赖训练知识推断年份。最终写入 form_data 的值必须使用中国本地时间（UTC+8），格式为 YYYY-MM-DD HH:mm:ss，不得包含任何中文字符或自然语言词汇，也不得带时区后缀（如 +08:00 或 Z）；若字段是时间范围，同日范围格式为"YYYY-MM-DD HH:mm:ss~HH:mm:ss"，跨日范围格式为"YYYY-MM-DD HH:mm:ss~YYYY-MM-DD HH:mm:ss"（例：2026-04-15 20:00:00~22:00:00）。若用户只给出截止时间而未明确说明起始时间（如"X点前""不超过X点完成""3点之前"），禁止将 general.current_time 的返回值作为起始时间；必须向用户追问起始时间，不得继续调用 itsm.draft_prepare。
16. 完成绝对化后，必须将转换结果与同一次 general.current_time 调用返回的 china_formatted_time 做明确对比：若访问时段、执行窗口等代表未来操作窗口的字段，其起始时间已早于当前时间，必须明确告知用户"您填写的时间已经过去，请确认是否填写有误"；此时该字段必须保持未解决状态，禁止调用 itsm.draft_prepare，必须等待用户给出有效的未来时间、重新完成绝对化并再次对比确认后，方可继续。若该字段是时间范围，还必须检查起始时间是否严格早于结束时间；若起始时间 ≥ 结束时间，必须告知用户"您填写的时间范围无效（结束时间不晚于开始时间），请确认是否填写颠倒或有误，例如您是否想填写 [结束时间]~[起始时间]"；该字段必须保持未解决状态，禁止调用 itsm.draft_prepare，等待用户给出有效范围后方可继续。
17. 对无法解析为具体时间点的表达（如"尽快""随时""越快越好"），禁止将 general.current_time 的返回时间或任何猜测时间作为该字段的值；该字段必须置为空，禁止调用 itsm.draft_prepare，直接向用户说明需要填写具体时间范围，等待用户给出明确时间后再继续。
18. 在调用 itsm.draft_confirm 之后、itsm.ticket_create 之前，必须先调用 itsm.validate_participants(service_id, form_data) 校验审批参与者是否可达。若返回 ok=false，应将 failure_reason 告知用户，不允许继续调用 itsm.ticket_create
19. 当 itsm.ticket_create 返回 ok=true 时，工单已成功创建，直接向用户展示工单号和当前状态即可
20. 如果 itsm.draft_confirm 返回含"字段已变更"的错误，说明管理员在你对话期间修改了服务表单定义。此时必须重新调用 itsm.service_load 获取最新表单定义，再根据新定义调用 itsm.draft_prepare 重新准备草稿；若新增了必填字段，向用户追问后再继续

对用户说话时，请优先做到：先理解、再澄清、再推进；该回答时就自然回答，该收单时再严格收单，让用户感到你在认真协助他，而不是在机械执行脚本。`,
			ToolNames: []string{
				"itsm.service_match",
				"itsm.service_confirm",
				"itsm.service_load",
				"itsm.new_request",
				"itsm.draft_prepare",
				"itsm.draft_confirm",
				"itsm.validate_participants",
				"itsm.ticket_create",
				"itsm.my_tickets",
				"itsm.ticket_withdraw",
				"general.current_time",
				"system.current_user_profile",
				"organization.org_context",
			},
		},
		{
			Name:        "流程决策智能体",
			Code:        "itsm.decision",
			Description: "ITSM 流程决策智能体，基于工单上下文和策略约束，通过多轮工具调用收集信息后给出下一步可执行、可审计的流程决策",
			Type:        "assistant",
			Visibility:  "private",
			Strategy:    "react",
			Temperature: 0.2,
			MaxTokens:   4096,
			MaxTurns:    8,
			SystemPrompt: `你是流程决策智能体，负责为 ITSM 工单给出下一步可执行、可审计、可落地的流程决策。

你的核心职责：
1. 使用决策工具按需查询工单上下文、知识库、组织架构等信息
2. 基于收集到的信息和服务协作规范，判断当前只应该进入哪一个下一步
3. 当需要确定具体处理人时，必须先用 decision.resolve_participant 解析，再用 decision.user_workload 评估负载
4. 只能输出当前工单"下一步真正需要执行"的活动，不要把未来多步一次性展开
5. reasoning 必须解释判断依据，说明为什么是这个人、这个岗位，或者这个岗位+部门

推荐推理步骤：
1. 先用 decision.ticket_context 了解完整上下文（表单、SLA、活动历史）
2. 如需查阅处理规范，使用 decision.knowledge_search
3. 确定下一步类型后，用 decision.resolve_participant 解析指派人
4. 可选：用 decision.user_workload 做负载均衡，用 decision.similar_history 参考历史
5. 最终输出决策 JSON（不再调用任何工具）

决策原则：
1. 优先遵循明确规则，其次才是保守推断；不能为了让流程继续而编造参与者、节点或条件
2. 信息不足时，优先做保守决策：宁可指出需要人工介入，也不要输出高风险猜测
3. 审批节点仅支持通过/驳回，处理节点仅支持提交结果
4. 如果动作执行失败，要明确指出流程被阻塞且需要人工处理

严格约束：
1. 不要跳过工具或上下文校验直接编造结论
2. 不允许输出未在流程或策略中出现的参与方式
3. 不允许把姓名当作 username，不允许把岗位名称当作岗位 code，不允许把部门名称当作部门 code
4. 不允许为了"看起来完整"而补全不存在的审批链

请始终输出结构化、保守且可审计的判断。`,
			ToolNames: []string{
				"decision.ticket_context",
				"decision.knowledge_search",
				"decision.resolve_participant",
				"decision.user_workload",
				"decision.similar_history",
				"decision.sla_status",
				"decision.list_actions",
				"decision.execute_action",
			},
		},
	}

	for _, agent := range agents {
		// Match by code field for preset agents (upsert mode)
		var existing struct {
			ID   uint
			Code string
		}
		if agent.Code != "" {
			if err := db.Table("ai_agents").Where("code = ?", agent.Code).Select("id", "code").First(&existing).Error; err == nil {
				// Preset agent exists — skip system_prompt update to preserve custom edits
				slog.Info("ITSM agent seed: 智能体已存在，跳过 prompt 更新", "name", agent.Name, "code", agent.Code)
				syncAgentToolBindings(db, existing.ID, agent.ToolNames)
				continue
			}
		}

		// Fallback: check by name for backward compatibility
		if err := db.Table("ai_agents").Where("name = ?", agent.Name).Select("id", "code").First(&existing).Error; err == nil {
			// Agent exists by name — set code if missing, skip prompt update
			updates := map[string]any{}
			if existing.Code == "" && agent.Code != "" {
				updates["code"] = agent.Code
			}
			if len(updates) > 0 {
				db.Table("ai_agents").Where("id = ?", existing.ID).Updates(updates)
			}
			slog.Info("ITSM agent seed: 智能体已存在，跳过 prompt 更新", "name", agent.Name, "code", agent.Code)
			syncAgentToolBindings(db, existing.ID, agent.ToolNames)
			continue
		}

		// Create agent
		record := map[string]any{
			"name":          agent.Name,
			"code":          agent.Code,
			"description":   agent.Description,
			"type":          agent.Type,
			"visibility":    agent.Visibility,
			"strategy":      agent.Strategy,
			"system_prompt": agent.SystemPrompt,
			"temperature":   agent.Temperature,
			"max_tokens":    agent.MaxTokens,
			"max_turns":     agent.MaxTurns,
			"is_active":     true,
			"created_by":    1, // admin user
		}
		result := db.Table("ai_agents").Create(record)
		if result.Error != nil {
			slog.Error("ITSM agent seed: failed to create agent", "name", agent.Name, "error", result.Error)
			continue
		}

		slog.Info("ITSM agent seed: created agent", "name", agent.Name)

		// Bind tools for newly created agent
		if len(agent.ToolNames) > 0 {
			var agentRow struct{ ID uint }
			if err := db.Table("ai_agents").Where("name = ?", agent.Name).Select("id").First(&agentRow).Error; err != nil {
				slog.Error("ITSM agent seed: failed to find created agent for tool binding", "name", agent.Name, "error", err)
				continue
			}
			syncAgentToolBindings(db, agentRow.ID, agent.ToolNames)
		}
	}

	return nil
}

// syncAgentToolBindings ensures the agent's tool bindings match the desired ToolNames list.
// It adds missing bindings and removes stale ones.
func syncAgentToolBindings(db *gorm.DB, agentID uint, toolNames []string) {
	// Resolve desired tool IDs
	desiredToolIDs := make(map[uint]string) // toolID -> toolName
	for _, toolName := range toolNames {
		var toolRow struct{ ID uint }
		if err := db.Table("ai_tools").Where("name = ?", toolName).Select("id").First(&toolRow).Error; err != nil {
			slog.Warn("ITSM agent seed: tool not found, skipping binding", "agent_id", agentID, "tool", toolName)
			continue
		}
		desiredToolIDs[toolRow.ID] = toolName
	}

	// Query current bindings
	var currentBindings []struct {
		ToolID uint `gorm:"column:tool_id"`
	}
	db.Table("ai_agent_tools").Where("agent_id = ?", agentID).Select("tool_id").Find(&currentBindings)

	currentSet := make(map[uint]bool)
	for _, b := range currentBindings {
		currentSet[b.ToolID] = true
	}

	// Add missing bindings
	for toolID := range desiredToolIDs {
		if !currentSet[toolID] {
			db.Table("ai_agent_tools").Create(map[string]any{
				"agent_id": agentID,
				"tool_id":  toolID,
			})
			slog.Info("ITSM agent seed: added tool binding", "agent_id", agentID, "tool", desiredToolIDs[toolID])
		}
	}

	// Remove stale bindings
	for _, b := range currentBindings {
		if _, wanted := desiredToolIDs[b.ToolID]; !wanted {
			db.Table("ai_agent_tools").Where("agent_id = ? AND tool_id = ?", agentID, b.ToolID).Delete(nil)
			slog.Info("ITSM agent seed: removed stale tool binding", "agent_id", agentID, "tool_id", b.ToolID)
		}
	}
}
