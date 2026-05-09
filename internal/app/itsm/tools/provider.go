package tools

import (
	"encoding/json"
	"log/slog"

	"gorm.io/gorm"

	"metis/internal/app/itsm/engine"
)

// ITSMTool defines a tool that ITSM registers into the ai_tools table.
type ITSMTool struct {
	Name                string
	DisplayName         string
	Description         string
	ParametersSchema    json.RawMessage
	RuntimeConfigSchema json.RawMessage
	RuntimeConfig       json.RawMessage
}

// AllTools returns the ITSM tool definitions.
func AllTools() []ITSMTool {
	tools := []ITSMTool{
		{
			Name:        "itsm.service_match",
			DisplayName: "服务匹配",
			Description: "读取 ITSM 服务目录并以结构化语义判定返回权威匹配结果：明确命中时只返回一个 selected_service_id，并返回 service_locked/next_required_tool；真实歧义时返回候选；无匹配时返回空列表。",
			ParametersSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"query": {"type": "string", "description": "用户描述的需求（自然语言）"}
				},
				"required": ["query"]
			}`),
			RuntimeConfigSchema: json.RawMessage(`{
				"type": "object",
				"kind": "llm",
				"title": "服务匹配运行时",
				"properties": {
					"modelId": {"type": "integer", "title": "模型", "minimum": 1},
					"temperature": {"type": "number", "title": "温度", "minimum": 0, "maximum": 1, "default": 0.2},
					"maxTokens": {"type": "integer", "title": "最大 Token", "minimum": 256, "maximum": 8192, "default": 1024},
					"timeoutSeconds": {"type": "integer", "title": "超时时间", "minimum": 5, "maximum": 300, "default": 30}
				},
				"required": ["modelId", "temperature", "maxTokens", "timeoutSeconds"]
			}`),
			RuntimeConfig: json.RawMessage(`{"modelId":0,"temperature":0.2,"maxTokens":1024,"timeoutSeconds":30}`),
		},
		{
			Name:        "itsm.service_confirm",
			DisplayName: "服务确认",
			Description: "当服务匹配需要用户确认且用户明确选择某个候选时，确认并锁定为当前后续操作目标。service_id 优先使用 service_match 返回的 matches 数组中对应项的 id 字段；如果用户只表达“是的”“第一个”等候选序号，也可以传 1-based 候选序号，工具会解析为真实服务 ID。",
			ParametersSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"service_id": {"type": "integer", "description": "用户选择的真实服务 ID；或 1-based 候选序号（例如第一个候选传 1）"}
				},
				"required": ["service_id"]
			}`),
		},
		{
			Name:        "itsm.service_load",
			DisplayName: "服务加载",
			Description: "加载指定服务的协作规范、表单定义和动作配置；返回 prefill_suggestions 与 field_collection（必填字段、已预填字段、缺失字段和下一步建议）；service_id 可传真实服务 ID 或 1-based 候选序号。",
			ParametersSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"service_id": {"type": "integer", "description": "真实服务定义 ID，或 service_match 返回候选的 1-based 序号"}
				},
				"required": ["service_id"]
			}`),
		},
		{
			Name:        "itsm.current_request_context",
			DisplayName: "当前申请上下文",
			Description: "读取当前服务台会话状态，包括已匹配/已加载服务、原始诉求、预填字段、草稿版本和 next_expected_action；多轮继续、短确认或局部修改时优先调用，避免重复匹配和重复加载。",
			ParametersSchema: json.RawMessage(`{
				"type": "object",
				"properties": {}
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
			Description: "登记草稿并返回 ready_for_confirmation、missing_required_fields 和 next_required_tool。form_data 必须使用 service_load 返回的字段 key 和完整值；ready_for_confirmation=false 时必须停止当轮调用、转向用户追问缺口，禁止在同轮内重复调用此工具（空 form_data 二次重入会返回错误）。",
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
			Description: "在创建工单前校验处理参与者是否可达。service_id 可传真实服务 ID 或候选序号，但工具会以当前已加载服务为准；form_data 缺省时复用当前草稿表单数据。",
			ParametersSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"service_id": {"type": "integer", "description": "真实服务定义 ID，或 service_match 返回候选的 1-based 序号"},
					"form_data": {"type": "object", "description": "表单数据（用于确定路由分支）；缺省时复用当前草稿"}
				},
				"required": ["service_id"]
			}`),
		},
		{
			Name:        "itsm.ticket_create",
			DisplayName: "工单创建",
			Description: "在信息收集完成且草稿已确认后，将服务请求创建为 ITSM 工单。service_id 可传真实服务 ID 或候选序号，工具会以当前已加载服务为准；summary/form_data 缺省时优先使用已确认草稿。",
			ParametersSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"service_id": {"type": "integer", "description": "真实服务定义 ID，或 service_match 返回候选的 1-based 序号"},
					"summary": {"type": "string", "description": "工单摘要"},
					"form_data": {"type": "object", "description": "完整表单数据"}
				},
				"required": ["service_id"]
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
			Description: "按工单号撤回指定工单，仅申请人在工单尚未被处理时可撤回。",
			ParametersSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"ticket_code": {"type": "string", "description": "工单编号"},
					"reason": {"type": "string", "description": "撤回原因（可选）"}
				},
				"required": ["ticket_code"]
			}`),
		},
		{
			Name:        "sla.risk_queue",
			DisplayName: "SLA 风险队列",
			Description: "读取未终态工单的 SLA 风险与超时候选队列，供 SLA 保障岗判断需要处理的对象。",
			ParametersSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"status": {"type": "string", "description": "风险状态筛选，可选值：warning、critical、breached"}
				}
			}`),
		},
		{
			Name:        "sla.ticket_context",
			DisplayName: "SLA 工单上下文",
			Description: "读取指定工单的 SLA 状态、服务、优先级、责任人、当前活动和最近时间线。",
			ParametersSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"ticket_id": {"type": "integer", "description": "工单 ID"}
				},
				"required": ["ticket_id"]
			}`),
		},
		{
			Name:        "sla.escalation_rules",
			DisplayName: "SLA 升级规则",
			Description: "读取指定工单命中的 SLA 升级规则，返回触发类型、级别、等待时间和动作配置。",
			ParametersSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"ticket_id": {"type": "integer", "description": "工单 ID"},
					"trigger_type": {"type": "string", "description": "触发类型：response_timeout 或 resolution_timeout"}
				},
				"required": ["ticket_id", "trigger_type"]
			}`),
		},
		{
			Name:        "sla.trigger_escalation",
			DisplayName: "触发 SLA 升级",
			Description: "在已命中 SLA 规则时触发通知、改派或提优先级动作；调用结果必须写入审计时间线。",
			ParametersSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"ticket_id": {"type": "integer", "description": "工单 ID"},
					"rule_id": {"type": "integer", "description": "升级规则 ID"},
					"reasoning": {"type": "string", "description": "触发动作的证据和理由"}
				},
				"required": ["ticket_id", "rule_id", "reasoning"]
			}`),
		},
		{
			Name:        "sla.write_timeline",
			DisplayName: "写入 SLA 审计",
			Description: "把 SLA 保障岗的观察、建议、跳过原因或动作结果写入工单时间线。",
			ParametersSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"ticket_id": {"type": "integer", "description": "工单 ID"},
					"message": {"type": "string", "description": "时间线消息"},
					"reasoning": {"type": "string", "description": "证据、规则和判断依据"}
				},
				"required": ["ticket_id", "message"]
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
				"display_name":          tool.DisplayName,
				"description":           tool.Description,
				"parameters_schema":     string(tool.ParametersSchema),
				"runtime_config_schema": string(tool.RuntimeConfigSchema),
			})
			if len(tool.RuntimeConfig) > 0 {
				db.Table("ai_tools").
					Where("id = ? AND (runtime_config IS NULL OR runtime_config = '')", existing.ID).
					Update("runtime_config", string(tool.RuntimeConfig))
			}
			slog.Info("ITSM tools seed: updated tool", "name", tool.Name)
		} else {
			// Create new
			toolkit := "itsm"
			if len(tool.Name) > 9 && tool.Name[:9] == "decision." {
				toolkit = "decision"
			} else if len(tool.Name) > 4 && tool.Name[:4] == "sla." {
				toolkit = "sla"
			}
			if err := db.Table("ai_tools").Create(map[string]any{
				"toolkit":               toolkit,
				"name":                  tool.Name,
				"display_name":          tool.DisplayName,
				"description":           tool.Description,
				"parameters_schema":     string(tool.ParametersSchema),
				"runtime_config_schema": string(tool.RuntimeConfigSchema),
				"runtime_config":        string(tool.RuntimeConfig),
				"is_active":             true,
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
	ToolNames    []string
	SkillNames   []string
	MCPNames     []string
	KBNames      []string
	KGNames      []string
}

const serviceDeskAgentSystemPrompt = `你是 IT 服务台智能体，负责把用户的自然语言诉求稳定推进为可确认、可创建的 ITSM 工单；也可以回答服务目录、工单状态和流程规则类问题。

## 角色边界

- 用户只是咨询、查状态或问规则时，先自然回答；不要强行提单。
- 用户明确要申请、开通、报修、修改草稿、确认草稿或创建工单时，进入提单推进模式。
- 不要假装已经完成尚未完成的动作；不要把内部判断说成系统事实。

## 提单状态机

标准工具顺序只适用于全新的服务请求：
itsm.service_match ->（需要确认时 itsm.service_confirm）-> itsm.service_load -> itsm.draft_prepare -> 用户明确确认 -> itsm.draft_confirm -> itsm.validate_participants -> itsm.ticket_create。

- 多轮继续、短确认（如“是的”“确认”“继续”）或局部修改时，先依据系统注入的工具转录和 ITSM Runtime Context；不清楚当前阶段时调用 itsm.current_request_context。
- 已有 loaded_service_id 且 next_expected_action 指向 draft_prepare/draft_confirm/validate_participants 时，不要重新 service_match 或 service_load，除非用户明确说“新开/再提/换一个服务”。
- service_match 是服务选择的唯一事实来源。service_locked=true 时直接 service_load；confirmation_required=true 时先让用户选候选，再 service_confirm。
- service_load 返回服务规范、字段定义、prefill_suggestions 和 field_collection。后续字段判断以这些工具事实为准。
- draft_prepare 是展示任何草稿前的唯一入口，不是探测字段缺口的工具；调用前必须已从用户处取得所有必填字段的值。ready_for_confirmation=false 时：当轮立即停止工具调用，向用户说明缺失内容并逐项追问 missing_required_fields；等用户回复后在下一轮才能重新 draft_prepare；**绝对禁止**在未收到用户新信息的情况下于同一轮内用相同或更少数据重复调用 draft_prepare——工具检测到这一模式时会直接返回错误。ready_for_confirmation=true 时才能把草稿展示给用户确认。
- 用户确认当前草稿版本后才能 draft_confirm。创建工单前必须 validate_participants，失败时告知 failure_reason，不得 ticket_create。
- 用户说“再次申请”“再提一张”“新开一张”时，先调用 itsm.new_request，再重新匹配服务，不能复用上一张草稿。

## 字段填槽策略

- 优先使用 service_load.prefill_suggestions；它是工具从用户原话确定提取出的字段，不属于脑补。
- form_data 必须使用 service_load.form_fields 的 key 和字段类型约定的 JSON 值形态：text/textarea/email/url/select/radio/date/datetime/user_picker/dept_picker/rich_text 为 string；number 为 number；switch 与无 options 的 checkbox 为 boolean；multi_select 与有 options 的 checkbox 为 string[]；date_range 为 {"start":"...","end":"..."}（需要时分时必须使用完整 datetime）；table 为行对象数组。
- select/radio/multi_select/checkbox(options) 必须使用 option.value；不能把用户随口表达、逗号拼接字符串或 label 当作 value。
- table 字段必须按 service_load.form_fields.props.columns 生成行数据；每行 key 使用 column.key，不确定的必填列必须追问。
- 只补确定信息；账号、设备型号、时间窗口、处理人等不能从用户话里确定时保持缺失。
- system.current_user_profile.user.username 只是登录名，不是邮箱。涉及“邮箱”“Email”“我的邮箱”“账号邮箱”时，只能使用用户原文中的完整邮箱地址，或工具明确返回的邮箱字段；没有明确邮箱时必须追问，不得把用户名、姓名或账号名当邮箱。
- 用户已经给出的用途或原因，不要追问“是否还有其他具体原因”。复合字段如“设备与用途说明”不是独立设备型号字段；已有用途时不要追问设备型号。
- 追问缺失字段时只问 missing_required_fields 里的缺口，不把已预填字段重复问一遍。
- 路由字段存在 option_route_map 时，draft_prepare 前先判断是否跨路由；跨路由要让用户选择当前办理哪一路，同一路由多原因可合并为单值并在 summary/说明字段保留完整诉求。

## 时间字段

- 访问时段、执行窗口、生效时间等必须写成中国本地时间的绝对时间：YYYY-MM-DD HH:mm:ss，范围用 ~ 连接。
- 用户给“今天”“明天”“下午 5 点”“明天下午 5 点”等相对日期或缺少年月日但时刻明确的表达时，必须先调用 general.current_time，以返回的 china_formatted_time 为基准解析，不得使用过去日期。
- 用户只给“下午 5 点”这类没有日期但时刻明确的表达时，按距离当前时间最近的未来时刻解析；如果今天对应时刻已过去，就解析为明天同一时刻。
- 用户只给“今天上午”“明天晚上”“后天下午”等宽泛时段、无法唯一确定具体时分时，不得自行补全为具体时分，必须追问具体时间后再 draft_prepare。
- 用户给出 22:00-01:00、12:00-10:00 这类没有日期的明确时间区间时，先把开始时间解析为距离当前时间最近的未来时刻；如果结束不晚于开始，默认按跨天处理，把结束时间解释为次日对应时刻并写入草稿，由用户人工检查确认。
- “尽快”“随时”“越快越好”不能写入时间字段；需要追问具体时间。
- 起始时间早于当前时间时，不得 draft_prepare，必须说明问题并等待用户修正。

## 用户输出

- 语气自然、简洁、专业，优先告诉用户下一步。
- 草稿展示要包含摘要和关键字段，等待明确确认；用户模糊认可、继续补充或局部修改都不是最终确认。
- ticket_create 返回 ok=true 后，直接告知工单号和当前状态。`

const decisionAgentSystemPrompt = `你是流程决策智能体，负责为智能 ITSM 工单输出下一步可执行、可审计的 DecisionPlan。

## 不可变原则

- 证据优先：先使用 decision.ticket_context 获取完整上下文，再按需要查询知识、动作、参与者和 SLA。
- 一次只决策当前下一步；不要把未来处理链一次性展开。
- 参与者必须可解析。需要人工处理或表单时，先 decision.resolve_participant；候选为空时不得高置信输出该人工活动。
- 低置信时保守输出，让引擎进入管理员处置；不要为了推进流程编造岗位、用户、动作或条件。
- reasoning 必须说明证据来源、分支依据、参与者选择和风险点。

## 工具使用顺序

1. 必须先调用 decision.ticket_context，读取 status、current_activities、activity_history、action_progress、parallel_groups 和 is_terminal。
2. 本轮由 activity_completed 触发时，必须读取 completed_activity、completed_requirements 和 workflow_context；已完成且满足当前规范的人工活动不得重复生成；被 rejected 的人工活动必须先解释驳回原因、协作规范定义的恢复路径以及 workflow_json 与该路径的关系，不得无新证据重复创建同一处理任务。
3. 有服务知识库或规范不明确时，调用 decision.knowledge_search；无结果或不可用时可降级，但 reasoning 要说明。
4. 服务配置了动作时，先 decision.list_actions；规范要求同步预检、放行等动作时，优先用 decision.execute_action 元调用执行并观察结果。
5. 需要人工活动时，调用 decision.resolve_participant；候选大于 1 时可用 decision.user_workload 做负载选择。
6. SLA 可能影响优先级时，调用 decision.sla_status。

## 动作与完成判断

- 自动动作优先使用 decision.execute_action 在决策循环内同步执行；只有明确需要异步动作活动时，才输出 type/action 的活动。
- decision.ticket_context.action_progress.all_completed=true 只代表动作完成，不自动代表流程结束；必须同时满足服务规范允许结束、当前无待处理项、处理前置已完成。
- 只有在 current_activities 为空、parallel_groups 无未完成项、规范允许结束且前置动作/人工活动都完成时，才能输出 next_step_type=complete。
- 如果 completed_activity 已满足最后一个待处理人工前置条件，优先输出 complete，不要再次输出同一 process/form 活动。
- 如果 completed_activity.outcome=rejected 或 satisfied=false，必须按协作规范定义的恢复路径处理；协作规范未显式定义补充信息或返工路径时，不得退回申请人补充，也不得创建申请人补充/返工类人工活动；不得只因为原表单字段仍匹配就再次创建刚被驳回的同一处理任务。
- is_terminal=true 时不再创建活动。

## 输出约束

- 最终只输出 JSON DecisionPlan，不输出解释性正文。
- activities 中每个活动可包含 node_id 字段（对应 workflow_json 中的节点 ID，可选）；有 workflow_json 时建议填写，帮助引擎精确定位当前步骤在流程图中的位置。
- participant_type=requester 表示当前工单申请人，无需 participant_id；participant_type=user 时必须填 participant_id；participant_type=position_department 时必须填 position_code 和 department_code。
- 不允许把姓名当 username，不允许把岗位名称当 position_code，不允许把部门名称当 department_code。
- confidence 必须反映证据强度：未解析到参与者、知识冲突、动作失败或上下文不足时降低置信度。`

const slaAssuranceAgentSystemPrompt = `你是 SLA 保障岗，负责监督智能 ITSM 工单的 SLA 风险并在规则命中时触发升级动作。

## 职责边界

- SLA 是否超时只以系统计时器和 SLA 状态为准，不做主观改写。
- 你可以读取风险队列、工单上下文和升级规则，并在规则已命中时触发通知、改派或提优先级。
- 每次动作都必须说明证据：工单、触发类型、规则级别、动作类型、目标对象和风险原因。
- 未配置规则、证据不足或动作不允许时，不触发升级，只写入时间线说明。

## 操作要求

1. 先读取 sla.risk_queue 或指定工单上下文。
2. 对候选工单读取 sla.ticket_context 和 sla.escalation_rules。
3. 只有规则已命中时才调用 sla.trigger_escalation。
4. 所有观察、跳过、触发结果都用 sla.write_timeline 留痕。

## 输出

- 面向管理员，简洁说明已处理的风险和仍需人工介入的事项。`

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

	defaultModelID := resolveDefaultLLMModelID(db)
	agents := []presetAgent{
		{
			Name:         "IT 服务台智能体",
			Code:         "itsm.servicedesk",
			Description:  "IT 服务台智能体，引导用户完成服务匹配、信息收集、草稿确认与工单创建的全流程",
			Type:         "assistant",
			Visibility:   "public",
			Strategy:     "react",
			Temperature:  0.3,
			MaxTokens:    4096,
			MaxTurns:     20,
			SystemPrompt: serviceDeskAgentSystemPrompt,
			ToolNames: []string{
				"itsm.service_match",
				"itsm.service_confirm",
				"itsm.service_load",
				"itsm.current_request_context",
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
			Name:         "流程决策智能体",
			Code:         "itsm.decision",
			Description:  "ITSM 流程决策智能体，基于工单上下文和策略约束，使用多轮工具调用收集信息后给出下一步可执行、可审计的流程决策",
			Type:         "assistant",
			Visibility:   "private",
			Strategy:     "react",
			Temperature:  0.2,
			MaxTokens:    4096,
			MaxTurns:     8,
			SystemPrompt: decisionAgentSystemPrompt,
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
		{
			Name:         "SLA 保障智能体",
			Code:         "itsm.sla_assurance",
			Description:  "SLA 保障岗智能体，读取 SLA 风险队列和升级规则，在规则命中时触发通知、改派或提优先级并写入审计时间线",
			Type:         "assistant",
			Visibility:   "private",
			Strategy:     "react",
			Temperature:  0.2,
			MaxTokens:    4096,
			MaxTurns:     8,
			SystemPrompt: slaAssuranceAgentSystemPrompt,
			ToolNames: []string{
				"sla.risk_queue",
				"sla.ticket_context",
				"sla.escalation_rules",
				"sla.trigger_escalation",
				"sla.write_timeline",
			},
		},
	}

	for _, agent := range agents {
		// Existing preset agents are treated as user-owned runtime config.
		// Seed only initializes new records; it does not overwrite later edits.
		var existing struct {
			ID   uint
			Code string
		}
		if agent.Code != "" {
			if err := db.Table("ai_agents").Where("code = ?", agent.Code).Select("id", "code").First(&existing).Error; err == nil {
				slog.Info("ITSM agent seed: preset agent already exists, keeping user configuration", "name", agent.Name, "code", agent.Code)
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
				slog.Info("ITSM agent seed: backfilled preset agent code", "name", agent.Name, "code", agent.Code)
			}
			slog.Info("ITSM agent seed: preset agent matched by name, keeping user configuration", "name", agent.Name, "code", agent.Code)
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
		if defaultModelID != nil {
			record["model_id"] = *defaultModelID
		}
		result := db.Table("ai_agents").Create(record)
		if result.Error != nil {
			slog.Error("ITSM agent seed: failed to create agent", "name", agent.Name, "error", result.Error)
			continue
		}

		slog.Info("ITSM agent seed: created agent", "name", agent.Name)

		var agentRow struct{ ID uint }
		if err := db.Table("ai_agents").Where("name = ?", agent.Name).Select("id").First(&agentRow).Error; err != nil {
			slog.Error("ITSM agent seed: failed to find created agent for default bindings", "name", agent.Name, "error", err)
			continue
		}
		if err := seedPresetAgentBindings(db, agentRow.ID, agent); err != nil {
			slog.Warn("ITSM agent seed: failed to apply preset default bindings", "name", agent.Name, "error", err)
		}
	}

	return nil
}

func resolveDefaultLLMModelID(db *gorm.DB) *uint {
	var modelRow struct{ ID uint }
	if err := db.Table("ai_models").
		Where("type = ? AND status = ? AND is_default = ?", "llm", "active", true).
		Order("id ASC").
		Select("id").
		First(&modelRow).Error; err == nil && modelRow.ID > 0 {
		return &modelRow.ID
	}
	return nil
}

func seedPresetAgentBindings(db *gorm.DB, agentID uint, agent presetAgent) error {
	for _, binding := range []struct {
		table     string
		idColumn  string
		lookupTbl string
		lookupCol string
		names     []string
		kind      string
	}{
		{table: "ai_agent_tools", idColumn: "tool_id", lookupTbl: "ai_tools", lookupCol: "name", names: agent.ToolNames, kind: "tool"},
		{table: "ai_agent_skills", idColumn: "skill_id", lookupTbl: "ai_skills", lookupCol: "name", names: agent.SkillNames, kind: "skill"},
		{table: "ai_agent_mcp_servers", idColumn: "mcp_server_id", lookupTbl: "ai_mcp_servers", lookupCol: "name", names: agent.MCPNames, kind: "mcp"},
		{table: "ai_agent_knowledge_bases", idColumn: "knowledge_base_id", lookupTbl: "ai_knowledge_assets", lookupCol: "name", names: agent.KBNames, kind: "knowledge_base"},
		{table: "ai_agent_knowledge_graphs", idColumn: "knowledge_graph_id", lookupTbl: "ai_knowledge_assets", lookupCol: "name", names: agent.KGNames, kind: "knowledge_graph"},
	} {
		if err := seedNamedBindings(db, agentID, binding.table, binding.idColumn, binding.lookupTbl, binding.lookupCol, binding.names, binding.kind); err != nil {
			return err
		}
	}
	return nil
}

func seedNamedBindings(db *gorm.DB, agentID uint, bindingTable, bindingColumn, lookupTable, lookupColumn string, names []string, kind string) error {
	for _, name := range names {
		var target struct{ ID uint }
		query := db.Table(lookupTable).Where(lookupColumn+" = ?", name).Select("id")
		switch kind {
		case "knowledge_base":
			query = query.Where("category = ?", "kb")
		case "knowledge_graph":
			query = query.Where("category = ?", "kg")
		}
		if err := query.First(&target).Error; err != nil {
			slog.Warn("ITSM agent seed: preset binding target not found, skipping", "agent_id", agentID, "kind", kind, "name", name)
			continue
		}

		attrs := map[string]any{"agent_id": agentID, bindingColumn: target.ID}
		var count int64
		if err := db.Table(bindingTable).Where(attrs).Count(&count).Error; err != nil {
			return err
		}
		if count > 0 {
			continue
		}
		if err := db.Table(bindingTable).Create(attrs).Error; err != nil {
			return err
		}
		slog.Info("ITSM agent seed: added preset default binding", "agent_id", agentID, "kind", kind, "name", name)
	}
	return nil
}
