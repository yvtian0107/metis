package bdd

// steps_boss_dialog_test.go — BDD step definitions for Boss high-risk change request dialog validation.
//
// Covers:
//   - BS-101 to BS-112, BS-114: service desk dialog follow-up and form validation
//   - Boss-specific: change_items completeness, multi-item preservation

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/cucumber/godog"

	ai "metis/internal/app/ai/runtime"
	. "metis/internal/app/itsm/domain"
	"metis/internal/app/itsm/tools"
	"metis/internal/llm"
)

// bossDraftPrepareNotCalled asserts that the Boss dialog agent did NOT call itsm.draft_prepare.
func (bc *bddContext) bossDraftPrepareNotCalled() error {
	return bc.thenDraftPrepareNotCalled()
}

// bossDraftConfirmNotCalled asserts that the Boss dialog agent did NOT complete draft_confirm.
func (bc *bddContext) bossDraftConfirmNotCalled() error {
	return bc.thenDraftNotCalledOrConfirmNotCalled()
}

// thenBossDraftCalled asserts that draft_prepare was called.
func (bc *bddContext) thenBossDraftCalled() error {
	if !hasToolCall(bc.dialogState.toolCalls, "itsm.draft_prepare") {
		names := make([]string, len(bc.dialogState.toolCalls))
		for i, c := range bc.dialogState.toolCalls {
			names[i] = c.Name
		}
		return fmt.Errorf("expected itsm.draft_prepare to be called, got: %v", names)
	}
	return nil
}

// thenBossDraftChangeItemsPreserved asserts that draft_prepare was called with a complete
// multi-item change_items array, including items with both read and read_write permissions.
func (bc *bddContext) thenBossDraftChangeItemsPreserved() error {
	args := getToolCallArgs(bc.dialogState.toolCalls, "itsm.draft_prepare")
	if args == nil {
		return fmt.Errorf("itsm.draft_prepare was not called")
	}

	var parsed struct {
		FormData map[string]any `json:"form_data"`
	}
	if err := json.Unmarshal(args, &parsed); err != nil {
		return fmt.Errorf("parse draft_prepare args: %w", err)
	}

	raw, ok := parsed.FormData["change_items"]
	if !ok {
		return fmt.Errorf("draft_prepare form_data missing 'change_items'")
	}

	items, ok := raw.([]any)
	if !ok {
		return fmt.Errorf("change_items is not an array, type: %T", raw)
	}

	if len(items) < 2 {
		return fmt.Errorf("expected at least 2 change_items for multi-item test, got %d", len(items))
	}

	// Check that at least one read and one read_write item exist.
	hasRead, hasReadWrite := false, false
	for _, item := range items {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		pl, _ := m["permission_level"].(string)
		switch pl {
		case "read":
			hasRead = true
		case "read_write":
			hasReadWrite = true
		}
	}

	if !hasRead || !hasReadWrite {
		return fmt.Errorf("expected mixed read/read_write change_items, hasRead=%v hasReadWrite=%v; items=%v", hasRead, hasReadWrite, items)
	}
	return nil
}

// ---------------------------------------------------------------------------
// Service setup for Boss dialog tests
// ---------------------------------------------------------------------------

const bossDraftSystemPrompt = `你是 IT 服务台智能体，帮助用户完成"高风险变更协同申请（Boss）"的提单流程。

工作流程：
1. 调用 itsm.service_match 匹配服务
2. 调用 itsm.service_load 加载服务详情（含表单定义和路由提示）
3. 收集用户信息，准备草稿
4. 调用 itsm.draft_prepare 校验并登记草稿

必填字段（全部缺一不可）：
- subject（申请主题）
- request_category（申请类别）：只接受 prod_change / access_grant / emergency_support
- risk_level（风险等级）：只接受 low / medium / high
- expected_finish_time（期望完成时间）
- change_window（变更窗口，开始 ~ 结束）：结束时间必须晚于开始时间
- impact_scope（影响范围）
- rollback_required（回滚要求）：只接受 required / not_required
- impact_modules（影响模块，多选）：只接受 gateway / payment / monitoring / order
- change_items（变更明细表，数组，至少一条）：每条必须包含 system、resource、permission_level；permission_level 只接受 read / read_write

关键规则：
- 上述任意必填字段缺失，必须先追问，不能调用 draft_prepare
- 时间窗口结束 <= 开始时，提示时间非法，不能调用 draft_prepare
- 枚举值不在允许列表内，提示使用受支持的选项，不能完成草稿确认
- change_items 为空数组或没有填写时，必须追问至少一条明细
- 明细行中缺少 system / resource / permission_level 任一字段时，必须提示缺失字段，不能调用 draft_prepare
- 多条明细时完整保留每一行，不能丢行或合并`

// bossDraftDialogWorkflowJSON is a minimal serial workflow for dialog-only tests.
// No LLM generation needed — we only test the intake dialog layer.
var bossDraftDialogWorkflowJSON = json.RawMessage(`{
  "nodes": [
    {"id":"start","type":"start","data":{"label":"开始","nodeType":"start"}},
    {"id":"p1","type":"process","data":{"label":"总部处理","nodeType":"process","participants":[{"type":"position_department","department_code":"headquarters","position_code":"serial_reviewer"}]}},
    {"id":"p2","type":"process","data":{"label":"运维处理","nodeType":"process","participants":[{"type":"position_department","department_code":"it","position_code":"ops_admin"}]}},
    {"id":"end","type":"end","data":{"label":"结束","nodeType":"end"}}
  ],
  "edges": [
    {"id":"e1","source":"start","target":"p1"},
    {"id":"e2","source":"p1","target":"p2","data":{"condition":{"field":"activity.outcome","operator":"eq","value":"completed"}}},
    {"id":"e3","source":"p1","target":"end","data":{"condition":{"field":"activity.outcome","operator":"eq","value":"rejected"}}},
    {"id":"e4","source":"p2","target":"end"}
  ]
}`)

const bossDraftDialogFormSchema = `{"version":1,"fields":[
  {"key":"subject","type":"text","label":"申请主题","required":true},
  {"key":"request_category","type":"select","label":"申请类别","required":true,"options":[
    {"label":"生产变更","value":"prod_change"},
    {"label":"访问授权","value":"access_grant"},
    {"label":"应急支持","value":"emergency_support"}
  ]},
  {"key":"risk_level","type":"radio","label":"风险等级","required":true,"options":[
    {"label":"低","value":"low"},
    {"label":"中","value":"medium"},
    {"label":"高","value":"high"}
  ]},
  {"key":"expected_finish_time","type":"datetime","label":"期望完成时间","required":true},
  {"key":"change_window","type":"date_range","label":"变更窗口","required":true},
  {"key":"impact_scope","type":"textarea","label":"影响范围","required":true},
  {"key":"rollback_required","type":"select","label":"回滚要求","required":true,"options":[
    {"label":"需要","value":"required"},
    {"label":"不需要","value":"not_required"}
  ]},
  {"key":"impact_modules","type":"multi_select","label":"影响模块","required":true,"options":[
    {"label":"网关","value":"gateway"},
    {"label":"支付","value":"payment"},
    {"label":"监控","value":"monitoring"},
    {"label":"订单","value":"order"}
  ]},
  {"key":"change_items","type":"table","label":"变更明细表","required":true,"props":{"columns":[
    {"key":"system","type":"text","label":"系统"},
    {"key":"resource","type":"text","label":"资源"},
    {"key":"permission_level","type":"select","label":"权限级别","options":[
      {"label":"只读","value":"read"},
      {"label":"读写","value":"read_write"}
    ]},
    {"key":"effective_range","type":"date_range","label":"生效时段"},
    {"key":"reason","type":"text","label":"变更理由"}
  ]}}
]}`

func publishBossDialogService(bc *bddContext) error {
	catalog := &ServiceCatalog{
		Name:     "变更管理（对话测试）",
		Code:     "boss-dialog-test",
		IsActive: true,
	}
	if err := bc.db.Create(catalog).Error; err != nil {
		return fmt.Errorf("create catalog: %w", err)
	}

	priority := &Priority{
		Name:     "高",
		Code:     "high-boss-dialog",
		Value:    1,
		Color:    "#f5222d",
		IsActive: true,
	}
	if err := bc.db.Create(priority).Error; err != nil {
		return fmt.Errorf("create priority: %w", err)
	}
	bc.priority = priority

	svc := &ServiceDefinition{
		Name:              "高风险变更协同申请（Boss）",
		Code:              "boss-serial-change-dialog",
		CatalogID:         catalog.ID,
		EngineType:        "smart",
		IntakeFormSchema:  JSONField(bossDraftDialogFormSchema),
		WorkflowJSON:      JSONField(bossDraftDialogWorkflowJSON),
		CollaborationSpec: bossCollaborationSpec,
		IsActive:          true,
	}
	if err := bc.db.Create(svc).Error; err != nil {
		return fmt.Errorf("create service: %w", err)
	}
	bc.service = svc
	return nil
}

// setupBossDialogTest sets up a ReactExecutor and returns a run function for Boss dialog tests.
func setupBossDialogTest(bc *bddContext) (func(ctx context.Context) error, error) {
	client, err := llm.NewClient(llm.ProtocolOpenAI, bc.llmCfg.baseURL, bc.llmCfg.apiKey)
	if err != nil {
		return nil, fmt.Errorf("create LLM client: %w", err)
	}

	op := tools.NewOperator(bc.db, nil, nil, nil, nil, &bddServiceMatcher{db: bc.db})
	store := newMemStateStore()
	registry := tools.NewRegistry(op, store)

	const testSessionID uint = 199
	testUserID := bc.dialogState.currentUserID
	if testUserID == 0 {
		testUserID = 1
	}

	toolExec := ai.NewCompositeToolExecutor(
		[]ai.ToolHandlerRegistry{registry, ai.NewGeneralToolRegistry(nil, nil)},
		testSessionID,
		testUserID,
	)

	var toolDefs []ai.ToolDefinition
	for _, t := range tools.AllTools() {
		toolDefs = append(toolDefs, ai.ToolDefinition{
			Type:        "builtin",
			Name:        t.Name,
			Description: t.Description,
			Parameters:  t.ParametersSchema,
		})
	}
	toolDefs = append(toolDefs, ai.ToolDefinition{
		Type:        "builtin",
		Name:        "general.current_time",
		Description: "Return current time in Asia/Shanghai and UTC.",
		Parameters:  json.RawMessage(`{"type":"object","properties":{"timezone":{"type":"string"}}}`),
	})

	executor := ai.NewReactExecutor(client, toolExec)

	run := func(ctx context.Context) error {
		msgs := make([]ai.ExecuteMessage, 0, len(bc.dialogState.messages))
		if len(bc.dialogState.messages) == 0 && strings.TrimSpace(bc.dialogState.userMessage) != "" {
			msgs = append(msgs, ai.ExecuteMessage{Role: "user", Content: bc.dialogState.userMessage})
		} else {
			for _, msg := range bc.dialogState.messages {
				role := msg.Role
				if role == "" {
					role = "user"
				}
				msgs = append(msgs, ai.ExecuteMessage{Role: role, Content: msg.Content})
			}
		}

		req := ai.ExecuteRequest{
			SessionID:    testSessionID,
			SystemPrompt: bossDraftSystemPrompt,
			Messages:     msgs,
			Tools:        toolDefs,
			MaxTurns:     12,
			AgentConfig: ai.AgentExecuteConfig{
				ModelName:   bc.llmCfg.model,
				Temperature: ptrFloat32(0.15),
				MaxTokens:   4096,
			},
		}

		ch, err := executor.Execute(ctx, req)
		if err != nil {
			return fmt.Errorf("execute boss dialog agent: %w", err)
		}

		bc.dialogState.toolCalls = nil
		bc.dialogState.toolResults = nil
		bc.dialogState.finalContent = ""
		var contentParts []string
		toolNamesByID := map[string]string{}

		for evt := range ch {
			switch evt.Type {
			case ai.EventTypeToolCall:
				toolNamesByID[evt.ToolCallID] = evt.ToolName
				bc.dialogState.toolCalls = append(bc.dialogState.toolCalls, toolCallRecord{
					ID:   evt.ToolCallID,
					Name: evt.ToolName,
					Args: evt.ToolArgs,
				})
			case ai.EventTypeToolResult:
				bc.dialogState.toolResults = append(bc.dialogState.toolResults, toolResultRecord{
					ID:      evt.ToolCallID,
					Name:    toolNamesByID[evt.ToolCallID],
					Output:  evt.ToolOutput,
					IsError: strings.HasPrefix(evt.ToolOutput, "Error:"),
				})
			case ai.EventTypeContentDelta:
				contentParts = append(contentParts, evt.Text)
			case ai.EventTypeError:
				return fmt.Errorf("boss dialog agent error: %s", evt.Message)
			}
		}

		bc.dialogState.finalContent = strings.Join(contentParts, "")
		return nil
	}

	return run, nil
}

// ---------------------------------------------------------------------------
// Step implementations
// ---------------------------------------------------------------------------

func (bc *bddContext) givenBossDialogServicePublished() error {
	return publishBossDialogService(bc)
}

func (bc *bddContext) givenBossDialogFor(username string) error {
	user, ok := bc.usersByName[username]
	if !ok {
		return fmt.Errorf("user %q not found in context", username)
	}
	bc.dialogState.currentUserID = user.ID
	bc.dialogState.currentUsername = username
	bc.dialogState.messages = nil
	return nil
}

func (bc *bddContext) whenBossAgentProcessesDialog() error {
	run, err := setupBossDialogTest(bc)
	if err != nil {
		return fmt.Errorf("setup boss dialog: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 180*time.Second)
	defer cancel()
	return run(ctx)
}

// ---------------------------------------------------------------------------
// Registration
// ---------------------------------------------------------------------------

func registerBossDialogSteps(sc *godog.ScenarioContext, bc *bddContext) {
	sc.Given(`^已发布高风险变更协同申请对话测试服务$`, bc.givenBossDialogServicePublished)
	sc.Given(`^"([^"]*)" 发起高风险变更协同申请对话$`, bc.givenBossDialogFor)

	sc.Given(`^用户消息为 "([^"]*)"$`, bc.givenUserMessage)
	sc.When(`^服务台 Boss Agent 处理对话$`, bc.whenBossAgentProcessesDialog)

	sc.Then(`^服务台未调用 draft_prepare$`, bc.bossDraftPrepareNotCalled)
	sc.Then(`^服务台未完成草稿确认$`, bc.bossDraftConfirmNotCalled)
	sc.Then(`^服务台调用了 draft_prepare$`, bc.thenBossDraftCalled)
	sc.Then(`^draft_prepare 的 change_items 包含完整的多条明细$`, bc.thenBossDraftChangeItemsPreserved)
	sc.Then(`^回复内容匹配 "([^"]*)"$`, bc.thenResponseMatches)
}
