package bdd

// steps_vpn_dialog_validation_test.go — BDD step definitions for Service Desk Agent dialog validation.
//
// Tests the agent's ability to:
//   - Detect cross-route conflicts and ask user to clarify
//   - Merge same-route multi-select without forcing a choice
//   - Ask for missing required fields before calling draft_prepare
//
// Uses real LLM via ReactExecutor + real ITSM tool handlers.

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	. "metis/internal/app/itsm/domain"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/cucumber/godog"
	"gorm.io/gorm"

	ai "metis/internal/app/ai/runtime"
	"metis/internal/app/itsm/tools"
	"metis/internal/llm"
)

// ---------------------------------------------------------------------------
// memStateStore — in-memory StateStore for dialog tests
// ---------------------------------------------------------------------------

type bddServiceMatcher struct {
	db *gorm.DB
}

func (m *bddServiceMatcher) MatchServices(ctx context.Context, query string) ([]tools.ServiceMatch, tools.MatchDecision, error) {
	type row struct {
		ID          uint
		Name        string
		Description string
	}
	var rows []row
	if err := m.db.Table("itsm_service_definitions").
		Where("is_active = ? AND deleted_at IS NULL", true).
		Select("id, name, description").
		Order("id ASC").
		Find(&rows).Error; err != nil {
		return nil, tools.MatchDecision{}, err
	}
	if len(rows) == 0 {
		return nil, tools.MatchDecision{Kind: tools.MatchDecisionNoMatch}, nil
	}
	matches := make([]tools.ServiceMatch, 0, len(rows))
	for _, row := range rows {
		matches = append(matches, tools.ServiceMatch{
			ID:          row.ID,
			Name:        row.Name,
			Description: row.Description,
			Score:       1,
			Reason:      "BDD 测试服务匹配",
		})
	}
	if len(matches) == 1 {
		return matches[:1], tools.MatchDecision{Kind: tools.MatchDecisionSelectService, SelectedServiceID: matches[0].ID}, nil
	}
	return matches, tools.MatchDecision{Kind: tools.MatchDecisionNeedClarification, ClarificationQuestion: "请选择要办理的服务"}, nil
}

type memStateStore struct {
	states map[uint]*tools.ServiceDeskState
}

func newMemStateStore() *memStateStore {
	return &memStateStore{states: make(map[uint]*tools.ServiceDeskState)}
}

func (m *memStateStore) GetState(sessionID uint) (*tools.ServiceDeskState, error) {
	s, ok := m.states[sessionID]
	if !ok {
		return &tools.ServiceDeskState{Stage: "idle"}, nil
	}
	return s, nil
}

func (m *memStateStore) SaveState(sessionID uint, state *tools.ServiceDeskState) error {
	m.states[sessionID] = state
	return nil
}

// ---------------------------------------------------------------------------
// Dialog test state on bddContext
// ---------------------------------------------------------------------------

// dialogTestState holds the state for a single dialog validation scenario.
type dialogTestState struct {
	toolCalls    []toolCallRecord
	toolResults  []toolResultRecord
	finalContent string
	userMessage  string
	mutateDraft  bool // flag: mutate form fields after draft_prepare
	// Dialog fields shared by service desk validation scenarios.
	currentUserID   uint
	currentUsername string
	messages        []dialogMessage
	dialogMode      string
	previousTickets []*Ticket
}

type toolCallRecord struct {
	Name string
	Args json.RawMessage
}

type toolResultRecord struct {
	Name    string
	Output  string
	IsError bool
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// serviceDeskSystemPrompt is a shortened version of the production prompt,
// focused on the dialog validation rules we're testing (11, 12, 13, 14).
const serviceDeskTestPrompt = `你是 IT 服务台智能体，帮助用户完成 VPN 开通申请的提单流程。

工作流程：
1. 调用 itsm.service_match 匹配服务
2. 调用 itsm.service_load 加载服务详情（含表单定义和路由提示）
3. 收集用户信息，准备草稿
4. 调用 itsm.draft_prepare 校验并登记草稿

关键规则：
- 调用 itsm.draft_prepare 时，summary 和 form_data 都必须传入；form_data 是 JSON 对象，key 为字段 key，value 为对应的值（必须使用 service_load 返回的字段定义中的 option value，而不是用户的原始措辞）
- service_load 返回 prefill_suggestions 时，必须优先采用这些建议补齐同名表单字段，再判断必填缺失。用户给出的邮箱可作为 VPN 账号；"线上支持用/远程办公/故障排查"等用途短语可填入设备与用途说明，访问原因必须填入结构化 option value
- 设备与用途说明不是单独的设备型号字段，用户已给出用途时不要再追问设备型号；用户已给出访问原因时不要再问"是否还有其他具体原因"
- 当用户提到多个访问原因且映射到同一路由分支时，合并为该分支对应的单个结构化值（取第一个匹配的 option value）填入路由字段，同时将用户原始的多个原因完整写入 summary 和 reason 字段
- 在调用 itsm.draft_prepare 之前，必须先根据 service_load 返回的 routing_field_hint 中的 option_route_map 判断用户的诉求是否跨越了多条路由分支。如果用户同时提到了映射到不同处理路径的多种需求，你必须主动向用户说明这些需求分属不同处理路径，请用户明确选择当前要办理哪一个，而不是替用户做选择或直接提交
- 在调用 itsm.draft_prepare 前，先对照 service_load 返回的字段定义检查所有必填字段是否已收集；如果有必填字段缺失，必须先向用户追问缺失字段
- 如果 itsm.draft_prepare 返回的 warnings 中包含 multivalue_on_single_field，根据 resolved_values 判断这些值是否属于同一路由分支：若跨路由，向用户说明并请用户选择；若同路由，修正为单值后重新调用
- 不需要调用 system.current_user_profile 或 general.current_time，直接使用用户消息中的信息`

func setupDialogTest(bc *bddContext) (func(ctx context.Context, userMsg string) error, error) {
	// Build LLM client.
	client, err := llm.NewClient(llm.ProtocolOpenAI, bc.llmCfg.baseURL, bc.llmCfg.apiKey)
	if err != nil {
		return nil, fmt.Errorf("create LLM client: %w", err)
	}

	// Build ITSM tool registry backed by real operator + memStateStore.
	op := tools.NewOperator(bc.db, nil, nil, nil, nil, &bddServiceMatcher{db: bc.db})
	store := newMemStateStore()
	registry := tools.NewRegistry(op, store)

	// The session ID used for state management.
	const testSessionID uint = 99
	const testUserID uint = 1

	// Build CompositeToolExecutor with only the ITSM registry.
	toolExec := ai.NewCompositeToolExecutor(
		[]ai.ToolHandlerRegistry{registry},
		testSessionID,
		testUserID,
	)

	// Build tool definitions from AllTools().
	var toolDefs []ai.ToolDefinition
	for _, t := range tools.AllTools() {
		toolDefs = append(toolDefs, ai.ToolDefinition{
			Type:        "builtin",
			Name:        t.Name,
			Description: t.Description,
			Parameters:  t.ParametersSchema,
		})
	}

	// Build ReactExecutor.
	executor := ai.NewReactExecutor(client, toolExec)

	// Return a run function that executes the agent with a user message.
	run := func(ctx context.Context, userMsg string) error {
		req := ai.ExecuteRequest{
			SessionID:    testSessionID,
			SystemPrompt: serviceDeskTestPrompt,
			Messages: []ai.ExecuteMessage{
				{Role: "user", Content: userMsg},
			},
			Tools:    toolDefs,
			MaxTurns: 10,
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

		// Collect events.
		bc.dialogState.toolCalls = nil
		bc.dialogState.toolResults = nil
		bc.dialogState.finalContent = ""
		var contentParts []string

		for evt := range ch {
			switch evt.Type {
			case ai.EventTypeToolCall:
				log.Printf("[BDD-DIALOG] tool_call: %s args=%s", evt.ToolName, string(evt.ToolArgs))
				bc.dialogState.toolCalls = append(bc.dialogState.toolCalls, toolCallRecord{
					Name: evt.ToolName,
					Args: evt.ToolArgs,
				})
			case ai.EventTypeToolResult:
				preview := evt.ToolOutput
				if len(preview) > 500 {
					preview = preview[:500] + "..."
				}
				log.Printf("[BDD-DIALOG] tool_result: %s output=%s", evt.ToolName, preview)
				bc.dialogState.toolResults = append(bc.dialogState.toolResults, toolResultRecord{
					Name:    evt.ToolName,
					Output:  evt.ToolOutput,
					IsError: strings.HasPrefix(evt.ToolOutput, "Error:"),
				})
			case ai.EventTypeContentDelta:
				contentParts = append(contentParts, evt.Text)
			case ai.EventTypeError:
				log.Printf("[BDD-DIALOG] error: %s", evt.Message)
				return fmt.Errorf("agent error: %s", evt.Message)
			}
		}

		bc.dialogState.finalContent = strings.Join(contentParts, "")
		return nil
	}

	return run, nil
}

func ptrFloat32(f float32) *float32 { return &f }

// hasToolCall checks if a specific tool was called.
func hasToolCall(calls []toolCallRecord, name string) bool {
	for _, c := range calls {
		if c.Name == name {
			return true
		}
	}
	return false
}

// getToolCallArgs returns the args of the first call to the named tool.
func getToolCallArgs(calls []toolCallRecord, name string) json.RawMessage {
	for _, c := range calls {
		if c.Name == name {
			return c.Args
		}
	}
	return nil
}

// toolCallCount returns the number of times a tool was called.
func toolCallCount(calls []toolCallRecord, name string) int {
	n := 0
	for _, c := range calls {
		if c.Name == name {
			n++
		}
	}
	return n
}

// ---------------------------------------------------------------------------
// Static VPN service for dialog validation (no LLM generation needed)
// ---------------------------------------------------------------------------

// vpnDialogWorkflowJSON provides a static workflow with edge conditions routing
// request_kind values to network or security processing.
var vpnDialogWorkflowJSON = json.RawMessage(`{
	"nodes": [
		{"id": "start", "type": "start", "data": {"label": "开始", "nodeType": "start"}},
		{"id": "gateway1", "type": "exclusive", "data": {"label": "路由网关", "nodeType": "exclusive"}},
		{"id": "process_net", "type": "process", "data": {"label": "网络管理处理", "nodeType": "process", "participants": [{"type": "position_department", "position_code": "network_admin", "department_code": "it"}]}},
		{"id": "process_sec", "type": "process", "data": {"label": "信息安全管理员处理", "nodeType": "process", "participants": [{"type": "position_department", "position_code": "security_admin", "department_code": "it"}]}},
		{"id": "end", "type": "end", "data": {"label": "结束", "nodeType": "end"}}
	],
	"edges": [
		{"id": "e1", "source": "start", "target": "gateway1"},
		{"id": "e2", "source": "gateway1", "target": "process_net", "data": {"condition": {"field": "form.request_kind", "operator": "contains_any", "value": ["online_support", "troubleshooting", "production_emergency", "network_access_issue"]}}},
		{"id": "e3", "source": "gateway1", "target": "process_sec", "data": {"condition": {"field": "form.request_kind", "operator": "contains_any", "value": ["external_collaboration", "long_term_remote_work", "cross_border_access", "security_compliance"]}}},
		{"id": "e4", "source": "process_net", "target": "end"},
		{"id": "e5", "source": "process_sec", "target": "end"}
	]
}`)

// vpnFormSchema is the form definition for VPN dialog validation tests.
var vpnFormSchema = `{
	"version": 1,
	"fields": [
		{
			"key": "request_kind",
			"type": "select",
				"label": "访问原因",
				"required": true,
				"options": [
					{"label": "线上支持", "value": "online_support"},
					{"label": "故障排查", "value": "troubleshooting"},
					{"label": "生产应急", "value": "production_emergency"},
					{"label": "网络接入问题", "value": "network_access_issue"},
					{"label": "外部协作", "value": "external_collaboration"},
					{"label": "长期远程办公", "value": "long_term_remote_work"},
					{"label": "跨境访问", "value": "cross_border_access"},
					{"label": "安全合规事项", "value": "security_compliance"}
				]
			},
		{
			"key": "vpn_type",
			"type": "select",
			"label": "VPN类型",
			"required": false,
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
			"required": false
		}
	]
}`

func publishVPNDialogService(bc *bddContext) error {
	// ServiceCatalog
	catalog := &ServiceCatalog{
		Name:     "VPN服务(对话测试)",
		Code:     "vpn-dialog",
		IsActive: true,
	}
	if err := bc.db.Create(catalog).Error; err != nil {
		return fmt.Errorf("create catalog: %w", err)
	}

	// Priority
	priority := &Priority{
		Name:     "普通",
		Code:     "normal-dialog",
		Value:    3,
		Color:    "#52c41a",
		IsActive: true,
	}
	if err := bc.db.Create(priority).Error; err != nil {
		return fmt.Errorf("create priority: %w", err)
	}
	bc.priority = priority

	// ServiceDefinition with inline form schema + static workflow
	svc := &ServiceDefinition{
		Name:              "VPN开通申请",
		Code:              "vpn-activation-dialog",
		CatalogID:         catalog.ID,
		EngineType:        "smart",
		IntakeFormSchema:  JSONField(vpnFormSchema),
		WorkflowJSON:      JSONField(vpnDialogWorkflowJSON),
		CollaborationSpec: vpnCollaborationSpec,
		IsActive:          true,
	}
	if err := bc.db.Create(svc).Error; err != nil {
		return fmt.Errorf("create service: %w", err)
	}
	bc.service = svc
	return nil
}

// ---------------------------------------------------------------------------
// Step definitions
// ---------------------------------------------------------------------------

func (bc *bddContext) givenDialogServicePublished() error {
	return publishVPNDialogService(bc)
}

func (bc *bddContext) givenUserMessage(msg string) error {
	bc.dialogState.userMessage = msg
	return nil
}

func (bc *bddContext) whenAgentProcessesMessage() error {
	run, err := setupDialogTest(bc)
	if err != nil {
		return fmt.Errorf("setup dialog test: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 180*time.Second)
	defer cancel()

	return run(ctx, bc.dialogState.userMessage)
}

func (bc *bddContext) thenToolCallSequenceContains(toolName string) error {
	if !hasToolCall(bc.dialogState.toolCalls, toolName) {
		names := make([]string, len(bc.dialogState.toolCalls))
		for i, c := range bc.dialogState.toolCalls {
			names[i] = c.Name
		}
		return fmt.Errorf("expected tool %q in call sequence, got: %v", toolName, names)
	}
	return nil
}

func (bc *bddContext) thenDraftNotCalledOrConfirmNotCalled() error {
	if !hasToolCall(bc.dialogState.toolCalls, "itsm.draft_prepare") {
		// Path A: Agent didn't call draft_prepare at all — best behavior.
		return nil
	}
	// Path B: Agent called draft_prepare but must not have called draft_confirm.
	if hasToolCall(bc.dialogState.toolCalls, "itsm.draft_confirm") {
		names := make([]string, len(bc.dialogState.toolCalls))
		for i, c := range bc.dialogState.toolCalls {
			names[i] = c.Name
		}
		return fmt.Errorf("agent called both draft_prepare and draft_confirm, should have stopped; calls: %v", names)
	}
	return nil
}

func (bc *bddContext) thenDraftPrepareCalledWithSingleRouteValue() error {
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

	val, ok := parsed.FormData["request_kind"]
	if !ok {
		return fmt.Errorf("draft_prepare form_data missing request_kind")
	}

	strVal := fmt.Sprintf("%v", val)
	if strings.Contains(strVal, ",") {
		return fmt.Errorf("expected single routing value, got comma-separated: %q", strVal)
	}
	return nil
}

func (bc *bddContext) thenResponseMatches(pattern string) error {
	re, err := regexp.Compile(pattern)
	if err != nil {
		return fmt.Errorf("invalid regex %q: %w", pattern, err)
	}
	if !re.MatchString(bc.dialogState.finalContent) {
		// Show first 200 chars of content for debugging.
		preview := bc.dialogState.finalContent
		if len(preview) > 200 {
			preview = preview[:200] + "..."
		}
		return fmt.Errorf("response does not match /%s/; content: %s", pattern, preview)
	}
	return nil
}

func (bc *bddContext) thenResponseNotMatches(pattern string) error {
	re, err := regexp.Compile(pattern)
	if err != nil {
		return fmt.Errorf("invalid regex %q: %w", pattern, err)
	}
	if re.MatchString(bc.dialogState.finalContent) {
		return fmt.Errorf("response should NOT match /%s/ but it does", pattern)
	}
	return nil
}

func (bc *bddContext) thenToolCalledAtLeast(toolName string, minCount int) error {
	actual := toolCallCount(bc.dialogState.toolCalls, toolName)
	if actual < minCount {
		names := make([]string, len(bc.dialogState.toolCalls))
		for i, c := range bc.dialogState.toolCalls {
			names[i] = c.Name
		}
		return fmt.Errorf("expected %q called >= %d times, got %d; calls: %v", toolName, minCount, actual, names)
	}
	return nil
}

// thenToolCalledAtLeastParsed parses the min count from string (for godog step).
func (bc *bddContext) thenToolCalledAtLeastParsed(toolName string, minCountStr string) error {
	n, err := strconv.Atoi(minCountStr)
	if err != nil {
		return fmt.Errorf("invalid count %q: %w", minCountStr, err)
	}
	return bc.thenToolCalledAtLeast(toolName, n)
}

// ---------------------------------------------------------------------------
// Registration
// ---------------------------------------------------------------------------

func registerDialogValidationSteps(sc *godog.ScenarioContext, bc *bddContext) {
	sc.Given(`^已发布 VPN 对话测试服务$`, bc.givenDialogServicePublished)
	sc.Given(`^用户消息为 "([^"]*)"$`, bc.givenUserMessage)
	sc.When(`^服务台 Agent 处理用户消息$`, bc.whenAgentProcessesMessage)
	sc.Then(`^工具调用序列包含 "([^"]*)"$`, bc.thenToolCallSequenceContains)
	sc.Then(`^Agent 未调用 draft_prepare 或未继续到 draft_confirm$`, bc.thenDraftNotCalledOrConfirmNotCalled)
	sc.Then(`^draft_prepare 的路由字段为单值$`, bc.thenDraftPrepareCalledWithSingleRouteValue)
	sc.Then(`^回复内容匹配 "([^"]*)"$`, bc.thenResponseMatches)
	sc.Then(`^回复内容不匹配 "([^"]*)"$`, bc.thenResponseNotMatches)
	sc.Then(`^"([^"]*)" 被调用至少 (\d+) 次$`, bc.thenToolCalledAtLeastParsed)
}
