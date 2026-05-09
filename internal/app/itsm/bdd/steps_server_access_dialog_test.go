package bdd

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/cucumber/godog"

	ai "metis/internal/app/ai/runtime"
	"metis/internal/app/itsm/domain"
	"metis/internal/app/itsm/tools"
	"metis/internal/llm"
)

const serverAccessDialogPrompt = `你是 IT 服务台的 Agentic 助手，负责整理“生产服务器临时访问申请”。

目标：
1. 先识别服务：service_match -> service_load。
2. 在信息不完整、时间非法、或诉求跨路由冲突时，优先追问或澄清。
3. 只有在字段足够、语义清晰时，才调用 itsm.draft_prepare。
4. 只有 ready_for_confirmation=true 且用户明确确认提交时，才继续 draft_confirm / validate_participants / ticket_create。

严格规则：
- 缺目标服务器时，必须追问目标服务器，不能假设。
- 缺访问时段时，必须追问具体访问窗口。
- 开始时间早于当前时间，或结束早于开始时间且无法合理解释时，不能直接提单，必须要求修正。
- 若同一轮诉求混合了不同处理路径（例如运维排障 + 防火墙策略），不能替用户单选，必须澄清当前要办理哪一路。
- 若一次申请多个服务器，保留完整服务器列表；不要静默丢失任何服务器。
- 如果用户后续补充推翻了前文意图，以最新澄清后的诉求为准。
- 对“异常访问证据保全、取证、异常访问核查”等语义，要倾向安全路线，并在回复中说明依据。

输出要求：
- 若不能提交，就明确说明还缺什么或哪里不合法。
- 若已经准备草稿，可用自然语言总结给用户确认，但不要编造不存在的字段。
`

var serverAccessDialogWorkflowJSON = json.RawMessage(`{
  "nodes": [
    {"id":"start","type":"start","data":{"label":"开始","nodeType":"start"}},
    {"id":"route","type":"exclusive","data":{"label":"智能路由","nodeType":"exclusive"}},
    {"id":"ops_process","type":"process","data":{"label":"运维处理","nodeType":"process","participants":[{"type":"position_department","position_code":"ops_admin","department_code":"it"}]}},
    {"id":"network_process","type":"process","data":{"label":"网络处理","nodeType":"process","participants":[{"type":"position_department","position_code":"network_admin","department_code":"it"}]}},
    {"id":"security_process","type":"process","data":{"label":"安全处理","nodeType":"process","participants":[{"type":"position_department","position_code":"security_admin","department_code":"it"}]}},
    {"id":"end","type":"end","data":{"label":"完成","nodeType":"end"}}
  ],
  "edges": [
    {"id":"e1","source":"start","target":"route"},
    {"id":"e2","source":"route","target":"ops_process","data":{"condition":{"field":"form.request_kind","operator":"contains_any","value":["ops_troubleshooting","application_diagnosis","host_inspection"]}}},
    {"id":"e3","source":"route","target":"network_process","data":{"condition":{"field":"form.request_kind","operator":"contains_any","value":["network_diagnostic","firewall_change","acl_change","load_balancer"]}}},
    {"id":"e4","source":"route","target":"security_process","data":{"condition":{"field":"form.request_kind","operator":"contains_any","value":["security_investigation","audit_forensics","compliance_check","abnormal_access_review"]}}},
    {"id":"e5","source":"route","target":"security_process","data":{"default":true}},
    {"id":"e6","source":"ops_process","target":"end"},
    {"id":"e7","source":"network_process","target":"end"},
    {"id":"e8","source":"security_process","target":"end"}
  ]
}`)

const serverAccessDialogFormSchema = `{
  "version": 1,
  "fields": [
    {
      "key": "request_kind",
      "type": "select",
      "label": "访问类型",
      "required": true,
      "options": [
        {"label": "运维排障", "value": "ops_troubleshooting"},
        {"label": "应用诊断", "value": "application_diagnosis"},
        {"label": "主机巡检", "value": "host_inspection"},
        {"label": "网络诊断", "value": "network_diagnostic"},
        {"label": "防火墙策略调整", "value": "firewall_change"},
        {"label": "ACL 调整", "value": "acl_change"},
        {"label": "负载均衡调整", "value": "load_balancer"},
        {"label": "安全取证", "value": "security_investigation"},
        {"label": "安全审计", "value": "audit_forensics"},
        {"label": "合规检查", "value": "compliance_check"},
        {"label": "异常访问核查", "value": "abnormal_access_review"}
      ]
    },
    {"key": "target_host", "type": "textarea", "label": "目标服务器", "required": true},
    {"key": "access_account", "type": "text", "label": "访问账号", "required": true},
    {"key": "source_ip", "type": "text", "label": "来源 IP", "required": true},
    {"key": "access_window", "type": "text", "label": "访问时段", "required": true},
    {"key": "access_reason", "type": "textarea", "label": "访问原因", "required": true}
  ]
}`

func registerServerAccessDialogSteps(sc *godog.ScenarioContext, bc *bddContext) {
	sc.Given(`^server access dialog participants exist:$`, bc.givenParticipants)
	sc.Given(`^server access dialog service is published$`, bc.givenServerAccessDialogServicePublished)
	sc.Given(`^server access dialog is open for requester "([^"]*)"$`, bc.givenServiceDeskDialog)

	sc.When(`^requester says "([^"]*)"$`, bc.whenServiceDeskUserSays)
	sc.When(`^the server access agent processes the dialog$`, bc.whenServerAccessAgentProcessesDialog)

	sc.Then(`^tool call sequence contains "([^"]*)"$`, bc.thenToolCallSequenceContains)
	sc.Then(`^agent does not call draft_prepare or draft_confirm$`, bc.thenDraftNotCalledOrConfirmNotCalled)
	sc.Then(`^agent does not call draft_prepare$`, bc.thenDraftPrepareNotCalled)
	sc.Then(`^draft is not ready for confirmation$`, bc.thenDraftNotReadyForConfirmation)
	sc.Then(`^response matches "([^"]*)"$`, bc.thenResponseMatches)
	sc.Then(`^response does not match "([^"]*)"$`, bc.thenResponseNotMatches)
	sc.Then(`^"([^"]*)" is called at least (\d+) times$`, bc.thenToolCalledAtLeastParsed)
	sc.Then(`^draft_prepare field "([^"]*)" contains "([^"]*)"$`, bc.thenDraftPrepareFieldContains)
	sc.Then(`^draft_prepare field "([^"]*)" equals "([^"]*)"$`, bc.thenServerAccessDraftFieldEquals)
	sc.Then(`^draft_prepare field "([^"]*)" contains all of "([^"]*)"$`, bc.thenDraftPrepareFieldContainsAll)
}

func publishServerAccessDialogService(bc *bddContext) error {
	catalog := &domain.ServiceCatalog{
		Name:     "生产服务器访问（服务台对话）",
		Code:     "server-access-dialog",
		IsActive: true,
	}
	if err := bc.db.Create(catalog).Error; err != nil {
		return fmt.Errorf("create catalog: %w", err)
	}

	priority := &domain.Priority{
		Name:     "高",
		Code:     "high-server-dialog",
		Value:    2,
		Color:    "#fa8c16",
		IsActive: true,
	}
	if err := bc.db.Create(priority).Error; err != nil {
		return fmt.Errorf("create priority: %w", err)
	}
	bc.priority = priority

	svc := &domain.ServiceDefinition{
		Name:              "生产服务器临时访问申请",
		Code:              "server-access-dialog",
		CatalogID:         catalog.ID,
		EngineType:        "smart",
		IntakeFormSchema:  domain.JSONField(serverAccessDialogFormSchema),
		WorkflowJSON:      domain.JSONField(serverAccessDialogWorkflowJSON),
		CollaborationSpec: serverAccessCollaborationSpec,
		IsActive:          true,
	}
	if err := bc.db.Create(svc).Error; err != nil {
		return fmt.Errorf("create service: %w", err)
	}
	bc.service = svc
	return nil
}

func (bc *bddContext) givenServerAccessDialogServicePublished() error {
	return publishServerAccessDialogService(bc)
}

func (bc *bddContext) whenServerAccessAgentProcessesDialog() error {
	run, err := setupServerAccessDialogTest(bc)
	if err != nil {
		return fmt.Errorf("setup server access dialog: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 180*time.Second)
	defer cancel()
	return run(ctx)
}

func setupServerAccessDialogTest(bc *bddContext) (func(ctx context.Context) error, error) {
	client, err := llm.NewClient(llm.ProtocolOpenAI, bc.llmCfg.baseURL, bc.llmCfg.apiKey)
	if err != nil {
		return nil, fmt.Errorf("create LLM client: %w", err)
	}

	op := tools.NewOperator(bc.db, nil, nil, nil, nil, &bddServiceMatcher{db: bc.db})
	store := newMemStateStore()
	registry := tools.NewRegistry(op, store)

	const testSessionID uint = 109
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
			SystemPrompt: serverAccessDialogPrompt,
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
			return fmt.Errorf("execute agent: %w", err)
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
				return fmt.Errorf("agent error: %s", evt.Message)
			}
		}

		bc.dialogState.finalContent = strings.Join(contentParts, "")
		return nil
	}

	return run, nil
}

func (bc *bddContext) thenDraftPrepareFieldContains(field, expected string) error {
	value, err := bc.draftPrepareFormValue(field)
	if err != nil {
		return err
	}
	if !strings.Contains(value, expected) {
		return fmt.Errorf("expected draft_prepare %q to contain %q, got %q", field, expected, value)
	}
	return nil
}

func (bc *bddContext) thenServerAccessDraftFieldEquals(field, expected string) error {
	value, err := bc.draftPrepareFormValue(field)
	if err != nil {
		return err
	}
	if value != expected {
		return fmt.Errorf("expected draft_prepare %q to equal %q, got %q", field, expected, value)
	}
	return nil
}

func (bc *bddContext) thenDraftPrepareFieldContainsAll(field, expectedCSV string) error {
	value, err := bc.draftPrepareFormValue(field)
	if err != nil {
		return err
	}
	for _, part := range strings.Split(expectedCSV, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if !strings.Contains(value, part) {
			return fmt.Errorf("expected draft_prepare %q to contain %q, got %q", field, part, value)
		}
	}
	return nil
}
