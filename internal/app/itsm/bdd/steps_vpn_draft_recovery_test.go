package bdd

// steps_vpn_draft_recovery_test.go — BDD step definitions for Service Desk Agent
// draft recovery: when draft_confirm fails due to fields hash mismatch (admin
// modified form mid-conversation), the agent should re-load and re-prepare.

import (
	"context"
	"fmt"
	"log"
	. "metis/internal/app/itsm/domain"
	"strings"
	"time"

	"github.com/cucumber/godog"
	"gorm.io/gorm"

	ai "metis/internal/app/ai/runtime"
	"metis/internal/app/itsm/tools"
	"metis/internal/llm"
)

// ---------------------------------------------------------------------------
// vpnFormSchemaV2 — adds an optional "remark" field to change FieldsHash
// ---------------------------------------------------------------------------

var vpnFormSchemaV2 = `{
	"version": 2,
	"fields": [
		{
			"key": "request_kind",
			"type": "select",
			"label": "访问原因",
			"required": true,
			"options": [
				{"label": "network_support", "value": "network_support"},
				{"label": "security", "value": "security"},
				{"label": "remote_maintenance", "value": "remote_maintenance"}
			]
		},
		{
			"key": "vpn_type",
			"type": "select",
			"label": "VPN类型",
			"required": true,
			"options": [
				{"label": "l2tp", "value": "l2tp"},
				{"label": "ipsec", "value": "ipsec"}
			]
		},
		{
			"key": "reason",
			"type": "textarea",
			"label": "申请原因",
			"required": true
		},
		{
			"key": "access_period",
			"type": "text",
			"label": "访问时段",
			"required": true
		},
		{
			"key": "remark",
			"type": "textarea",
			"label": "备注",
			"required": false
		}
	]
}`

// ---------------------------------------------------------------------------
// mutatingStateStore — wraps memStateStore, triggers DB form mutation
// ---------------------------------------------------------------------------

// mutatingStateStore wraps a memStateStore and modifies the ServiceDefinition
// IntakeFormSchema when state.Stage transitions to "awaiting_confirmation".
// This simulates an admin changing the form mid-conversation.
type mutatingStateStore struct {
	inner *memStateStore
	db    *gorm.DB
	armed bool // whether mutation is enabled
	fired bool // whether mutation has already fired
}

func newMutatingStateStore(db *gorm.DB) *mutatingStateStore {
	return &mutatingStateStore{
		inner: newMemStateStore(),
		db:    db,
		armed: true,
	}
}

func (m *mutatingStateStore) GetState(sessionID uint) (*tools.ServiceDeskState, error) {
	return m.inner.GetState(sessionID)
}

func (m *mutatingStateStore) SaveState(sessionID uint, state *tools.ServiceDeskState) error {
	if err := m.inner.SaveState(sessionID, state); err != nil {
		return err
	}

	// Trigger mutation once when stage becomes "awaiting_confirmation"
	// (i.e., draft_prepare just succeeded).
	if m.armed && !m.fired && state.Stage == "awaiting_confirmation" {
		log.Printf("[BDD-MUTATION] draft_prepare completed, mutating ServiceDefinition IntakeFormSchema")
		if err := m.db.Model(&ServiceDefinition{}).
			Where("code = ?", "vpn-activation-dialog").
			Update("intake_form_schema", vpnFormSchemaV2).Error; err != nil {
			log.Printf("[BDD-MUTATION] failed to mutate schema: %v", err)
			return err
		}
		m.fired = true
		log.Printf("[BDD-MUTATION] ServiceDefinition IntakeFormSchema mutated to v2")
	}

	return nil
}

// ---------------------------------------------------------------------------
// Test prompt with recovery rule
// ---------------------------------------------------------------------------

const draftRecoveryTestPrompt = `你是 IT 服务台智能体，帮助用户完成 VPN 开通申请的提单流程。

工作流程：
1. 调用 itsm.service_match 匹配服务
2. 调用 itsm.service_load 加载服务详情（含表单定义和路由提示）
3. 收集用户信息，准备草稿
4. 调用 itsm.draft_prepare 校验并登记草稿
5. 调用 itsm.draft_confirm 确认草稿
6. 调用 itsm.validate_participants 校验参与者
7. 调用 itsm.ticket_create 创建工单

重要：在本次对话中，用户已在消息中提供了全部必填信息并明确表示要提交。你必须一口气推进完整流程，从 service_match 到 draft_confirm，不要中途停下等用户确认。draft_prepare 成功后立即调用 draft_confirm。

关键规则：
- 调用 itsm.draft_prepare 时，summary 和 form_data 都必须传入；form_data 是 JSON 对象，key 为字段 key，value 为对应的值（必须使用 service_load 返回的字段定义中的 option value，而不是用户的原始措辞）
- 当用户提到多个访问原因且映射到同一路由分支时，合并为该分支对应的单个结构化值（取第一个匹配的 option value）填入路由字段，同时将用户原始的多个原因完整写入 summary 和 reason 字段
- 在调用 itsm.draft_prepare 前，先对照 service_load 返回的字段定义检查所有必填字段是否已收集；如果有必填字段缺失，必须先向用户追问缺失字段
- 如果 itsm.draft_confirm 返回含"字段已变更"的错误，说明管理员在对话期间修改了服务表单定义。此时必须重新调用 itsm.service_load 获取最新表单定义，再根据新定义调用 itsm.draft_prepare 重新准备草稿，然后再次调用 itsm.draft_confirm
- 可按需调用 system.current_user_profile 或 general.current_time；涉及相对时间时必须先调用 general.current_time。`

// ---------------------------------------------------------------------------
// setupDialogTestWithMutation — variant of setupDialogTest with mutatingStateStore
// ---------------------------------------------------------------------------

func setupDialogTestWithMutation(bc *bddContext) (func(ctx context.Context, userMsg string) error, error) {
	client, err := llm.NewClient(llm.ProtocolOpenAI, bc.llmCfg.baseURL, bc.llmCfg.apiKey)
	if err != nil {
		return nil, fmt.Errorf("create LLM client: %w", err)
	}

	op := tools.NewOperator(bc.db, nil, nil, nil, nil, &bddServiceMatcher{db: bc.db})
	store := newMutatingStateStore(bc.db)
	registry := tools.NewRegistry(op, store)

	const testSessionID uint = 99
	const testUserID uint = 1

	toolExec := ai.NewCompositeToolExecutor(
		[]ai.ToolHandlerRegistry{registry},
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

	executor := ai.NewReactExecutor(client, toolExec)

	run := func(ctx context.Context, userMsg string) error {
		req := ai.ExecuteRequest{
			SessionID:    testSessionID,
			SystemPrompt: draftRecoveryTestPrompt,
			Messages: []ai.ExecuteMessage{
				{Role: "user", Content: userMsg},
			},
			Tools:    toolDefs,
			MaxTurns: 15, // more turns to allow recovery
			AgentConfig: ai.AgentExecuteConfig{
				ModelName:   bc.llmCfg.model,
				Temperature: ptrFloat32(0.2),
				MaxTokens:   4096,
			},
		}

		ch, err := executor.Execute(ctx, req)
		if err != nil {
			return fmt.Errorf("execute agent: %w", err)
		}

		bc.dialogState.toolCalls = nil
		bc.dialogState.toolResults = nil
		bc.dialogState.finalContent = ""
		var contentParts []string

		for evt := range ch {
			switch evt.Type {
			case ai.EventTypeToolCall:
				log.Printf("[BDD-RECOVERY] tool_call: %s args=%s", evt.ToolName, string(evt.ToolArgs))
				bc.dialogState.toolCalls = append(bc.dialogState.toolCalls, toolCallRecord{
					Name: evt.ToolName,
					Args: evt.ToolArgs,
				})
			case ai.EventTypeToolResult:
				preview := evt.ToolOutput
				if len(preview) > 500 {
					preview = preview[:500] + "..."
				}
				log.Printf("[BDD-RECOVERY] tool_result: %s output=%s", evt.ToolName, preview)
				bc.dialogState.toolResults = append(bc.dialogState.toolResults, toolResultRecord{
					Name:    evt.ToolName,
					Output:  evt.ToolOutput,
					IsError: strings.HasPrefix(evt.ToolOutput, "Error:"),
				})
			case ai.EventTypeContentDelta:
				contentParts = append(contentParts, evt.Text)
			case ai.EventTypeError:
				log.Printf("[BDD-RECOVERY] error: %s", evt.Message)
				return fmt.Errorf("agent error: %s", evt.Message)
			}
		}

		bc.dialogState.finalContent = strings.Join(contentParts, "")
		return nil
	}

	return run, nil
}

// ---------------------------------------------------------------------------
// Step definitions
// ---------------------------------------------------------------------------

func (bc *bddContext) givenFieldsWillMutateAfterDraftPrepare() error {
	bc.dialogState.mutateDraft = true
	return nil
}

func (bc *bddContext) whenAgentProcessesMessageWithMutation() error {
	run, err := setupDialogTestWithMutation(bc)
	if err != nil {
		return fmt.Errorf("setup dialog test with mutation: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 180*time.Second)
	defer cancel()

	return run(ctx, bc.dialogState.userMessage)
}

// ---------------------------------------------------------------------------
// Registration
// ---------------------------------------------------------------------------

func registerDraftRecoverySteps(sc *godog.ScenarioContext, bc *bddContext) {
	sc.Given(`^服务字段将在草稿准备后变更$`, bc.givenFieldsWillMutateAfterDraftPrepare)
	sc.When(`^服务台 Agent 处理用户消息（含字段变更）$`, bc.whenAgentProcessesMessageWithMutation)
}
