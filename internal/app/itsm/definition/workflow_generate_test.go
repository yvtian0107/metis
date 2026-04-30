package definition

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	appcore "metis/internal/app"
	. "metis/internal/app/itsm/config"
	. "metis/internal/app/itsm/domain"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"metis/internal/app/itsm/engine"
	"metis/internal/database"
	"metis/internal/llm"
	"metis/internal/model"
)

type fakePathEngineConfigProvider struct {
	cfg LLMEngineRuntimeConfig
	err error
}

func (p fakePathEngineConfigProvider) PathBuilderRuntimeConfig() (LLMEngineRuntimeConfig, error) {
	return p.cfg, p.err
}

type fakeWorkflowLLMClient struct {
	responses []llm.ChatResponse
	errs      []error
	calls     int
	requests  []llm.ChatRequest
}

func (c *fakeWorkflowLLMClient) Chat(_ context.Context, req llm.ChatRequest) (*llm.ChatResponse, error) {
	c.calls++
	c.requests = append(c.requests, req)
	idx := c.calls - 1
	if idx < len(c.errs) && c.errs[idx] != nil {
		return nil, c.errs[idx]
	}
	if idx < len(c.responses) {
		resp := c.responses[idx]
		return &resp, nil
	}
	return &llm.ChatResponse{}, nil
}

func (c *fakeWorkflowLLMClient) ChatStream(context.Context, llm.ChatRequest) (<-chan llm.StreamEvent, error) {
	return nil, llm.ErrNotSupported
}

func (c *fakeWorkflowLLMClient) Embedding(context.Context, llm.EmbeddingRequest) (*llm.EmbeddingResponse, error) {
	return nil, llm.ErrNotSupported
}

func newWorkflowGenerateServiceForRetryTest(client *fakeWorkflowLLMClient, maxRetries int) *WorkflowGenerateService {
	return &WorkflowGenerateService{
		engineConfigSvc: fakePathEngineConfigProvider{cfg: LLMEngineRuntimeConfig{
			Model:          "gpt-test",
			Protocol:       llm.ProtocolOpenAI,
			BaseURL:        "https://example.test/v1",
			APIKey:         "test-key",
			Temperature:    0.3,
			MaxTokens:      1024,
			MaxRetries:     maxRetries,
			TimeoutSeconds: 30,
		}},
		llmClientFactory: func(string, string, string) (llm.Client, error) {
			return client, nil
		},
	}
}

func validWorkflowJSONForGenerateTest() string {
	return `{"nodes":[{"id":"start","type":"start","data":{"label":"start"}},{"id":"request","type":"form","data":{"label":"request form","participants":[{"type":"requester"}],"formSchema":{"fields":[{"key":"summary","type":"textarea","label":"Summary"}]}}},{"id":"end","type":"end","data":{"label":"end"}}],"edges":[{"id":"e1","source":"start","target":"request","data":{}},{"id":"e2","source":"request","target":"end","data":{"outcome":"submitted"}}]}`
}

func validVPNWorkflowJSONForGenerateTest() string {
	return `{"nodes":[{"id":"start","type":"start","data":{"label":"开始"}},{"id":"request","type":"form","data":{"label":"填写 VPN 开通申请","participants":[{"type":"requester"}],"formSchema":{"fields":[{"key":"vpn_account","type":"text","label":"VPN账号"},{"key":"device_usage","type":"textarea","label":"设备与用途说明"},{"key":"request_kind","type":"select","label":"访问原因","options":["online_support","troubleshooting","production_emergency","network_access_issue","external_collaboration","long_term_remote_work","cross_border_access","security_compliance"]}]}}},{"id":"route","type":"exclusive","data":{"label":"访问原因路由"}},{"id":"network_process","type":"process","data":{"label":"网络管理员处理","participants":[{"type":"position_department","department_code":"it","position_code":"network_admin"}]}},{"id":"security_process","type":"process","data":{"label":"信息安全管理员处理","participants":[{"type":"position_department","department_code":"it","position_code":"security_admin"}]}},{"id":"end","type":"end","data":{"label":"结束"}}],"edges":[{"id":"edge_start_request","source":"start","target":"request","data":{}},{"id":"edge_request_route","source":"request","target":"route","data":{}},{"id":"edge_route_network","source":"route","target":"network_process","data":{"condition":{"field":"form.request_kind","operator":"contains_any","value":["online_support","troubleshooting","production_emergency","network_access_issue"]}}},{"id":"edge_route_security","source":"route","target":"security_process","data":{"condition":{"field":"form.request_kind","operator":"contains_any","value":["external_collaboration","long_term_remote_work","cross_border_access","security_compliance"]}}},{"id":"edge_network_end","source":"network_process","target":"end","data":{}},{"id":"edge_security_end","source":"security_process","target":"end","data":{}}]}`
}

func validServerAccessWorkflowJSONForGenerateTest() string {
	return `{"nodes":[{"id":"start","type":"start","data":{"label":"开始"}},{"id":"request","type":"form","data":{"label":"填写服务器临时访问申请","participants":[{"type":"requester"}],"formSchema":{"fields":[{"key":"target_servers","type":"textarea","label":"访问服务器"},{"key":"access_window","type":"date_range","label":"访问时段"},{"key":"operation_purpose","type":"textarea","label":"操作目的"},{"key":"access_reason","type":"textarea","label":"访问原因"}]}}},{"id":"route","type":"exclusive","data":{"label":"访问原因路由"}},{"id":"ops_process","type":"process","data":{"label":"运维管理员处理","participants":[{"type":"position_department","department_code":"it","position_code":"ops_admin"}]}},{"id":"network_process","type":"process","data":{"label":"网络管理员处理","participants":[{"type":"position_department","department_code":"it","position_code":"network_admin"}]}},{"id":"security_process","type":"process","data":{"label":"信息安全管理员处理","participants":[{"type":"position_department","department_code":"it","position_code":"security_admin"}]}},{"id":"end","type":"end","data":{"label":"结束"}}],"edges":[{"id":"edge_start_request","source":"start","target":"request","data":{}},{"id":"edge_request_route","source":"request","target":"route","data":{"outcome":"submitted"}},{"id":"edge_route_ops","source":"route","target":"ops_process","data":{"condition":{"field":"form.access_reason","operator":"contains_any","value":["应用发布","进程排障","日志排查","磁盘清理","主机巡检","生产运维操作"]}}},{"id":"edge_route_network","source":"route","target":"network_process","data":{"condition":{"field":"form.access_reason","operator":"contains_any","value":["网络抓包","连通性诊断","ACL调整","负载均衡变更","防火墙策略调整"]}}},{"id":"edge_route_security","source":"route","target":"security_process","data":{"condition":{"field":"form.access_reason","operator":"contains_any","value":["安全审计","入侵排查","漏洞修复验证","取证分析","合规检查"]}}},{"id":"edge_route_default","source":"route","target":"security_process","data":{"default":true}},{"id":"edge_ops_end_ok","source":"ops_process","target":"end","data":{"outcome":"approved"}},{"id":"edge_ops_end_reject","source":"ops_process","target":"end","data":{"outcome":"rejected"}},{"id":"edge_network_end_ok","source":"network_process","target":"end","data":{"outcome":"approved"}},{"id":"edge_network_end_reject","source":"network_process","target":"end","data":{"outcome":"rejected"}},{"id":"edge_security_end_ok","source":"security_process","target":"end","data":{"outcome":"approved"}},{"id":"edge_security_end_reject","source":"security_process","target":"end","data":{"outcome":"rejected"}}]}`
}

func validDBBackupWorkflowJSONForGenerateTest(precheckActionID, applyActionID uint) string {
	return fmt.Sprintf(`{"nodes":[{"id":"start","type":"start","data":{"label":"开始"}},{"id":"request","type":"form","data":{"label":"填写数据库备份白名单临时放行申请","participants":[{"type":"requester"}],"formSchema":{"fields":[{"key":"database_name","type":"text","label":"目标数据库"},{"key":"source_ip","type":"text","label":"来源 IP"},{"key":"whitelist_window","type":"text","label":"白名单放行时间窗"},{"key":"access_reason","type":"textarea","label":"申请原因"}]}}},{"id":"db_precheck_action","type":"action","data":{"label":"备份白名单预检","action_id":%d}},{"id":"db_process","type":"process","data":{"label":"数据库管理员处理","participants":[{"type":"position_department","department_code":"it","position_code":"db_admin"}]}},{"id":"db_apply_action","type":"action","data":{"label":"执行备份白名单放行","action_id":%d}},{"id":"end","type":"end","data":{"label":"结束"}}],"edges":[{"id":"edge_start_request","source":"start","target":"request","data":{}},{"id":"edge_request_precheck","source":"request","target":"db_precheck_action","data":{"outcome":"submitted"}},{"id":"edge_precheck_db","source":"db_precheck_action","target":"db_process","data":{"outcome":"success"}},{"id":"edge_db_apply_ok","source":"db_process","target":"db_apply_action","data":{"outcome":"approved"}},{"id":"edge_apply_end","source":"db_apply_action","target":"end","data":{"outcome":"success"}},{"id":"edge_db_end_reject","source":"db_process","target":"end","data":{"outcome":"rejected"}}]}`, precheckActionID, applyActionID)
}

func validBossWorkflowJSONForGenerateTest() string {
	return `{"nodes":[{"id":"start","type":"start","data":{"label":"开始"}},{"id":"request","type":"form","data":{"label":"填写高风险变更协同申请","participants":[{"type":"requester"}],"formSchema":{"fields":[{"key":"subject","type":"text","label":"申请主题"},{"key":"request_category","type":"select","label":"申请类别","options":["prod_change","access_grant","emergency_support"]},{"key":"risk_level","type":"radio","label":"风险等级","options":["low","medium","high"]},{"key":"expected_finish_time","type":"datetime","label":"期望完成时间"},{"key":"change_window","type":"date_range","label":"变更窗口"},{"key":"impact_scope","type":"textarea","label":"影响范围"},{"key":"rollback_required","type":"select","label":"回滚要求","options":["required","not_required"]},{"key":"impact_modules","type":"multi_select","label":"影响模块","options":["gateway","payment","monitoring","order"]},{"key":"change_items","type":"table","label":"变更明细表","props":{"columns":[{"key":"system","type":"text","label":"系统"},{"key":"resource","type":"text","label":"资源"},{"key":"permission_level","type":"select","label":"权限级别","options":["read","read_write"]},{"key":"effective_range","type":"date_range","label":"生效时段"},{"key":"reason","type":"text","label":"变更理由"}]}}]}}},{"id":"head_process","type":"process","data":{"label":"总部处理人处理","participants":[{"type":"position_department","department_code":"headquarters","position_code":"serial_reviewer"}]}},{"id":"ops_process","type":"process","data":{"label":"运维管理员处理","participants":[{"type":"position_department","department_code":"it","position_code":"ops_admin"}]}},{"id":"end","type":"end","data":{"label":"结束"}}],"edges":[{"id":"edge_start_request","source":"start","target":"request","data":{}},{"id":"edge_request_head","source":"request","target":"head_process","data":{"outcome":"submitted"}},{"id":"edge_head_ops_ok","source":"head_process","target":"ops_process","data":{"outcome":"approved"}},{"id":"edge_head_end_reject","source":"head_process","target":"end","data":{"outcome":"rejected"}},{"id":"edge_ops_end_ok","source":"ops_process","target":"end","data":{"outcome":"approved"}},{"id":"edge_ops_end_reject","source":"ops_process","target":"end","data":{"outcome":"rejected"}}]}`
}

func workflowWithBlockingIssueForGenerateTest(userID uint) string {
	return fmt.Sprintf(`{
		"nodes": [
			{"id":"start","type":"start","data":{"label":"start"}},
			{"id":"request","type":"form","data":{"label":"request form","participants":[{"type":"requester"}],"formSchema":{"fields":[{"key":"summary","type":"textarea","label":"Summary"}]}}},
			{"id":"process","type":"process","data":{"label":"process","participants":[{"type":"user","value":"%d"}]}},
			{"id":"end","type":"end","data":{"label":"end"}}
		],
		"edges": [
			{"id":"e1","source":"start","target":"request","data":{}},
			{"id":"e2","source":"request","target":"process","data":{"outcome":"submitted"}},
			{"id":"e3","source":"process","target":"end","data":{"outcome":"approved"}}
		]
	}`, userID)
}

func TestGenerate_UsesConfiguredSystemPrompt(t *testing.T) {
	client := &fakeWorkflowLLMClient{
		responses: []llm.ChatResponse{{Content: validWorkflowJSONForGenerateTest()}},
	}
	svc := &WorkflowGenerateService{
		engineConfigSvc: fakePathEngineConfigProvider{cfg: LLMEngineRuntimeConfig{
			Model:          "gpt-test",
			Protocol:       llm.ProtocolOpenAI,
			BaseURL:        "https://example.test/v1",
			APIKey:         "test-key",
			Temperature:    0.3,
			MaxTokens:      1024,
			MaxRetries:     0,
			TimeoutSeconds: 30,
			SystemPrompt:   "configured prompt",
		}},
		llmClientFactory: func(string, string, string) (llm.Client, error) {
			return client, nil
		},
	}

	_, err := svc.Generate(context.Background(), &GenerateRequest{
		CollaborationSpec: "用户提交申请，服务台处理后结束",
	})
	if err != nil {
		t.Fatalf("generate failed: %v", err)
	}
	if len(client.requests) == 0 || len(client.requests[0].Messages) == 0 {
		t.Fatalf("expected at least one llm request")
	}
	if got := client.requests[0].Messages[0].Content; got != "configured prompt" {
		t.Fatalf("expected configured system prompt, got %q", got)
	}
}

// ---------------------------------------------------------------------------
// Layer 1: Unit tests — extractJSON
// ---------------------------------------------------------------------------

func TestExtractJSON_BareJSON(t *testing.T) {
	input := `{"nodes":[],"edges":[]}`
	got, err := extractJSON(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !json.Valid(got) {
		t.Fatalf("result is not valid JSON: %s", got)
	}
}

func TestExtractJSON_MarkdownCodeBlock(t *testing.T) {
	input := "Here is the workflow:\n```json\n{\"nodes\":[],\"edges\":[]}\n```\nDone."
	got, err := extractJSON(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !json.Valid(got) {
		t.Fatalf("result is not valid JSON: %s", got)
	}
}

func TestExtractJSON_TextWrapped(t *testing.T) {
	input := `Here is the workflow: {"nodes":[],"edges":[]} Hope this helps!`
	got, err := extractJSON(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !json.Valid(got) {
		t.Fatalf("result is not valid JSON: %s", got)
	}
}

func TestExtractJSON_Invalid(t *testing.T) {
	input := "I cannot generate a workflow for this request."
	_, err := extractJSON(input)
	if err == nil {
		t.Fatal("expected error for invalid input, got nil")
	}
}

func TestExtractGeneratedIntakeFormSchema_NormalizesStringOptions(t *testing.T) {
	workflow := json.RawMessage(`{"nodes":[{"id":"form1","type":"form","data":{"formSchema":{"fields":[{"key":"reason","type":"select","label":"Reason","options":["ops","security"]}]}}}],"edges":[]}`)

	schemaJSON, errs := extractGeneratedIntakeFormSchema(workflow)
	if len(errs) > 0 {
		t.Fatalf("expected schema extraction to pass, got %+v", errs)
	}
	var schema struct {
		Fields []struct {
			Key      string `json:"key"`
			Required bool   `json:"required"`
			Options  []struct {
				Label string `json:"label"`
				Value string `json:"value"`
			} `json:"options"`
		} `json:"fields"`
	}
	if err := json.Unmarshal(schemaJSON, &schema); err != nil {
		t.Fatalf("unmarshal schema: %v", err)
	}
	if len(schema.Fields) != 1 || schema.Fields[0].Key != "reason" || !schema.Fields[0].Required {
		t.Fatalf("unexpected normalized schema: %s", schemaJSON)
	}
	if len(schema.Fields[0].Options) != 2 || schema.Fields[0].Options[0].Value != "ops" {
		t.Fatalf("expected string options to be normalized, got %s", schemaJSON)
	}
}

func TestExtractGeneratedIntakeFormSchema_ValidatesAdvancedFieldShapes(t *testing.T) {
	workflow := json.RawMessage(`{"nodes":[{"id":"form1","type":"form","data":{"formSchema":{"fields":[
		{"key":"window","type":"date_range","label":"访问时段"},
		{"key":"items","type":"table","label":"明细","props":{"columns":[
			{"key":"host","type":"text","label":"主机","required":true},
			{"key":"kind","type":"select","label":"类型","required":true,"options":["ops","security"]}
		]}}
	]}}}],"edges":[]}`)

	schemaJSON, errs := extractGeneratedIntakeFormSchema(workflow)
	if len(errs) > 0 {
		t.Fatalf("expected advanced schema extraction to pass, got %+v", errs)
	}
	var schema struct {
		Fields []struct {
			Key      string `json:"key"`
			Required bool   `json:"required"`
			Props    struct {
				Columns []struct {
					Key      string `json:"key"`
					Required bool   `json:"required"`
					Options  []struct {
						Label string `json:"label"`
						Value string `json:"value"`
					} `json:"options"`
				} `json:"columns"`
			} `json:"props"`
		} `json:"fields"`
	}
	if err := json.Unmarshal(schemaJSON, &schema); err != nil {
		t.Fatalf("unmarshal schema: %v", err)
	}
	if len(schema.Fields) != 2 || schema.Fields[0].Key != "window" || !schema.Fields[0].Required {
		t.Fatalf("unexpected normalized schema: %s", schemaJSON)
	}
	if len(schema.Fields[1].Props.Columns) != 2 || !schema.Fields[1].Props.Columns[0].Required {
		t.Fatalf("expected table columns to be canonicalized, got %s", schemaJSON)
	}
	if got := schema.Fields[1].Props.Columns[1].Options[0].Value; got != "ops" {
		t.Fatalf("expected table select options to be normalized, got %s", schemaJSON)
	}
}

func TestExtractGeneratedIntakeFormSchema_RejectsUnrenderableAdvancedField(t *testing.T) {
	_, errs := extractGeneratedIntakeFormSchema(json.RawMessage(`{"nodes":[{"id":"form1","type":"form","data":{"formSchema":{"fields":[
		{"key":"items","type":"table","label":"明细"}
	]}}}],"edges":[]}`))
	if len(errs) == 0 {
		t.Fatal("expected table field without columns to be rejected")
	}
	if !strings.Contains(errs[0].Message, "table field must define props.columns") {
		t.Fatalf("expected actionable table error, got %+v", errs)
	}
}

func TestExtractGeneratedIntakeFormSchema_RequiresFormSchema(t *testing.T) {
	_, errs := extractGeneratedIntakeFormSchema(json.RawMessage(`{"nodes":[{"id":"start","type":"start","data":{}}],"edges":[]}`))
	if len(errs) == 0 || errs[0].Level != "blocking" {
		t.Fatalf("expected missing form schema blocking error, got %+v", errs)
	}
}

// ---------------------------------------------------------------------------
// Layer 1: Unit tests — buildUserMessage / buildActionsContext
// ---------------------------------------------------------------------------

func TestBuildUserMessage_Basic(t *testing.T) {
	svc := &WorkflowGenerateService{}
	msg := svc.buildUserMessage("用户提交表单后经理处理", workflowPromptContext{}, nil)

	if !strings.Contains(msg, "用户提交表单后经理处理") {
		t.Fatal("message should contain the collaboration spec")
	}
	if strings.Contains(msg, "可用动作") {
		t.Fatal("message should not contain actions context when empty")
	}
	if strings.Contains(msg, "上一次生成") {
		t.Fatal("message should not contain previous errors when nil")
	}
}

func TestBuildUserMessage_WithActions(t *testing.T) {
	svc := &WorkflowGenerateService{}
	actionsCtx := svc.buildActionsContext([]ServiceAction{
		{BaseModel: model.BaseModel{ID: 7}, Name: "发送邮件", Code: "send-email", Description: "发送通知邮件"},
	})
	msg := svc.buildUserMessage("处理流程", workflowPromptContext{ActionsContext: actionsCtx}, nil)

	if !strings.Contains(msg, "处理流程") {
		t.Fatal("message should contain the collaboration spec")
	}
	if !strings.Contains(msg, "可用动作") {
		t.Fatal("message should contain actions context")
	}
	if !strings.Contains(msg, "send-email") {
		t.Fatal("message should contain action code")
	}
	if !strings.Contains(msg, "id: `7`") {
		t.Fatal("message should contain action id")
	}
}

func TestBuildUserMessage_WithPrevErrors(t *testing.T) {
	svc := &WorkflowGenerateService{}
	prevErrors := []engine.ValidationError{
		{NodeID: "node-1", Level: "blocking", Message: "缺少出边"},
		{EdgeID: "edge-2", Level: "blocking", Message: "引用了不存在的目标节点"},
		{NodeID: "node-3", Level: "blocking", Message: "人工节点 node-3 必须配置处理人"},
	}
	msg := svc.buildUserMessage("处理流程", workflowPromptContext{}, prevErrors)

	if !strings.Contains(msg, "上一次生成的工作流存在以下问题") {
		t.Fatal("message should contain previous error header")
	}
	if !strings.Contains(msg, "[节点 node-1]") {
		t.Fatal("message should contain node-prefixed error")
	}
	if !strings.Contains(msg, "[边 edge-2]") {
		t.Fatal("message should contain edge-prefixed error")
	}
	if !strings.Contains(msg, "参与人修正要求") {
		t.Fatal("message should contain participant repair guidance")
	}
	if !strings.Contains(msg, `"participants":[{"type":"requester"}]`) {
		t.Fatal("message should contain exact requester participant shape")
	}
	if !strings.Contains(msg, `"participants":[{"type":"position_department","department_code":"it","position_code":"network_admin"}]`) {
		t.Fatal("message should contain exact position_department participant shape")
	}
}

func TestBuildUserMessage_GuidesRejectedEdgeRepair(t *testing.T) {
	svc := &WorkflowGenerateService{}
	prevErrors := []engine.ValidationError{
		{NodeID: "node-process", Level: "blocking", Message: `process 节点 node-process 缺少 outcome="rejected" 的出边；协作规范未定义驳回恢复路径时 rejected 应指向公共结束节点，驳回语义由 edge.data.outcome="rejected" 表达`},
	}
	msg := svc.buildUserMessage("处理流程", workflowPromptContext{}, prevErrors)

	requiredSnippets := []string{
		"人工节点出边修正要求",
		`data.outcome="approved"`,
		`data.outcome="rejected"`,
		"共同指向同一个 type=\"end\" 节点",
		"驳回语义由 edge.data.outcome=\"rejected\" 表达",
		"复用同一个 end 节点，不要拆成“驳回结束”和“完成”",
		"不要凭空生成“退回申请人补充”",
	}
	for _, snippet := range requiredSnippets {
		if !strings.Contains(msg, snippet) {
			t.Fatalf("message missing rejected edge repair guidance: %s", snippet)
		}
	}
	if strings.Contains(msg, "end_rejected") || strings.Contains(msg, "不能和 approved 指向同一个目标节点") {
		t.Fatalf("message still contains old rejected end guidance: %s", msg)
	}
}

func TestBuildUserMessage_GuidesGatewayRepair(t *testing.T) {
	svc := &WorkflowGenerateService{}
	prevErrors := []engine.ValidationError{
		{NodeID: "node-converge", Level: "blocking", Message: "排他网关节点 node-converge 至少需要两条出边"},
	}
	msg := svc.buildUserMessage("并行处理流程", workflowPromptContext{}, prevErrors)

	requiredSnippets := []string{
		"网关修正要求",
		"exclusive 只表示条件分支",
		`type="parallel"`,
		`data.gateway_direction="fork"`,
		`data.gateway_direction="join"`,
	}
	for _, snippet := range requiredSnippets {
		if !strings.Contains(msg, snippet) {
			t.Fatalf("message missing gateway repair guidance: %s", snippet)
		}
	}
}

func TestBuildUserMessage_WithFormAndOrgContext(t *testing.T) {
	svc := &WorkflowGenerateService{}
	formCtx := strings.Join([]string{
		"## 已有申请表单契约",
		"- VPN账号: key=`vpn_account`, type=`text`",
		"- 设备与用途说明: key=`device_usage`, type=`textarea`",
		"- 访问原因: key=`request_kind`, type=`select`, options=`online_support`, `troubleshooting`, `production_emergency`, `network_access_issue`, `external_collaboration`, `long_term_remote_work`, `cross_border_access`, `security_compliance`",
	}, "\n")
	orgCtx := strings.Join([]string{
		"## 组织架构上下文",
		"- 部门：信息部（code: `it`）",
		"- 岗位：网络管理员（code: `network_admin`）",
		"- 岗位：信息安全管理员（code: `security_admin`）",
	}, "\n")
	msg := svc.buildUserMessage("员工申请 VPN 开通，网络支持类交给网络管理员，安全合规类交给信息安全管理员。", workflowPromptContext{FormContractContext: formCtx, OrgContext: orgCtx}, nil)

	for _, snippet := range []string{
		"已有申请表单契约",
		"vpn_account",
		"device_usage",
		"request_kind",
		"online_support",
		"security_compliance",
		"组织架构上下文",
		"信息部",
		"it",
		"网络管理员",
		"network_admin",
		"信息安全管理员",
		"security_admin",
	} {
		if !strings.Contains(msg, snippet) {
			t.Fatalf("expected prompt to contain %q, got %s", snippet, msg)
		}
	}
}

func TestPathBuilderSystemPromptRequiresHumanNodeParticipants(t *testing.T) {
	requiredSnippets := []string{
		"所有 form/process 等人工节点必须在 data 中配置非空 participants 数组",
		"不要把 participantType、positionCode、departmentCode 直接放在 data 上",
		`"participants":[{"type":"requester"}]`,
		`type: "requester" | "user"`,
		`"participants":[{"type":"position_department","department_code":"it","position_code":"network_admin"}]`,
		`"position_code":"security_admin"`,
		"edge.data.outcome、edge.data.condition.field、edge.data.condition.value 必须使用稳定机器值",
	}
	for _, snippet := range requiredSnippets {
		if !strings.Contains(PathBuilderSystemPrompt, snippet) {
			t.Fatalf("system prompt missing required participant guidance: %s", snippet)
		}
	}
}

func TestPathBuilderSystemPromptGuidesParallelGateway(t *testing.T) {
	requiredSnippets := []string{
		"| parallel | 并行网关（拆分/汇聚） | label, nodeType, gateway_direction |",
		`data.gateway_direction="fork"`,
		`data.gateway_direction="join"`,
		"不要用 exclusive 网关做并行汇聚",
	}
	for _, snippet := range requiredSnippets {
		if !strings.Contains(PathBuilderSystemPrompt, snippet) {
			t.Fatalf("system prompt missing parallel gateway guidance: %s", snippet)
		}
	}
}

func TestPathBuilderSystemPromptUsesBackendSnakeCaseRuntimeFields(t *testing.T) {
	requiredSnippets := []string{
		"label, nodeType, action_id",
		"label, nodeType, wait_mode",
		"action_id 必须是可用动作列表中的数字 id",
		"如果没有可用动作列表",
		"不要生成 action 节点",
	}
	for _, snippet := range requiredSnippets {
		if !strings.Contains(PathBuilderSystemPrompt, snippet) {
			t.Fatalf("system prompt missing backend snake_case field guidance: %s", snippet)
		}
	}
	misleadingSnippets := []string{
		"actionId",
		"waitMode",
	}
	for _, snippet := range misleadingSnippets {
		if strings.Contains(PathBuilderSystemPrompt, snippet) {
			t.Fatalf("system prompt still contains frontend/camelCase runtime field: %s", snippet)
		}
	}
}

func TestPathBuilderSystemPromptReusesEquivalentEndNodes(t *testing.T) {
	requiredSnippets := []string{
		"同一个 process 节点的 approved 和 rejected 可以指向同一个 end 节点",
		"业务结果由 edge.data.outcome 表达，不由 end 节点名称表达",
		"默认只生成一个公共终态",
		"多个 process 节点的 rejected 出边如果最终也是结束，也应全部指向同一个 end",
		"rejected 出边都不可省略",
		"不要生成“驳回结束”“通过完成”",
	}
	for _, snippet := range requiredSnippets {
		if !strings.Contains(PathBuilderSystemPrompt, snippet) {
			t.Fatalf("system prompt missing end-node reuse guidance: %s", snippet)
		}
	}

	misleadingSnippets := []string{
		"即使两条路径最终都到达结束，也必须创建两个独立的 end 节点",
		"不能复用同一个。这样画布上才能清晰呈现 Y 形审批分支",
		"同一个 process 节点的 approved 和 rejected 必须指向不同的目标节点",
		"end_rejected",
	}
	for _, snippet := range misleadingSnippets {
		if strings.Contains(PathBuilderSystemPrompt, snippet) {
			t.Fatalf("system prompt still contains misleading end-node guidance: %s", snippet)
		}
	}
}

func TestPathBuilderSystemPromptGuidesServerAccessFieldKeysAndRouting(t *testing.T) {
	requiredSnippets := []string{
		"生产服务器临时访问申请",
		"target_servers：访问服务器",
		"access_window：访问时段",
		"operation_purpose：操作目的",
		"access_reason：访问原因",
		"必须基于 form.access_reason 路由",
		"不要改用 form.reason、form.purpose、form.request_kind、form.access_purpose",
	}
	for _, snippet := range requiredSnippets {
		if !strings.Contains(PathBuilderSystemPrompt, snippet) {
			t.Fatalf("system prompt missing server access field/routing guidance: %s", snippet)
		}
	}
}

func TestPathBuilderSystemPromptGuidesBossFieldKeys(t *testing.T) {
	requiredSnippets := []string{
		"高风险变更协同申请（Boss）",
		"subject：申请主题",
		"request_category：申请类别",
		"prod_change、access_grant、emergency_support",
		"risk_level：风险等级",
		"low、medium、high",
		"rollback_required：回滚要求",
		"impact_modules：影响模块",
		"gateway、payment、monitoring、order",
		"change_items：变更明细表",
		"system、resource、permission_level、effective_range、reason",
		"read、read_write",
		"先总部处理人，再信息部运维管理员",
	}
	for _, snippet := range requiredSnippets {
		if !strings.Contains(PathBuilderSystemPrompt, snippet) {
			t.Fatalf("system prompt missing boss field guidance: %s", snippet)
		}
	}
}

func TestWorkflowValidationMessageGuidesParticipantRepair(t *testing.T) {
	errs := engine.ValidateWorkflow(json.RawMessage(`{
		"nodes": [
			{"id":"start","type":"start","data":{"label":"开始"}},
			{"id":"process_network","type":"process","data":{"label":"网络管理员处理"}},
			{"id":"end","type":"end","data":{"label":"结束"}}
		],
		"edges": [
			{"id":"e1","source":"start","target":"process_network","data":{}},
			{"id":"e2","source":"process_network","target":"end","data":{}}
		]
	}`))

	if len(errs) == 0 {
		t.Fatal("expected validation errors")
	}
	got := errs[0].Message
	if !strings.Contains(got, "data.participants") || !strings.Contains(got, "position_department") {
		t.Fatalf("expected actionable participant repair message, got %q", got)
	}
	if !strings.Contains(got, "process_network（网络管理员处理）") {
		t.Fatalf("expected validation message to include node label, got %q", got)
	}
}

func TestWorkflowValidationMessageGuidesSharedRejectedEnd(t *testing.T) {
	errs := engine.ValidateWorkflow(json.RawMessage(`{
		"nodes": [
			{"id":"start","type":"start","data":{"label":"开始"}},
			{"id":"process_network","type":"process","data":{"label":"网络管理员处理","participants":[{"type":"position_department","department_code":"it","position_code":"network_admin"}]}},
			{"id":"end_completed","type":"end","data":{"label":"完成"}}
		],
		"edges": [
			{"id":"e1","source":"start","target":"process_network","data":{}},
			{"id":"e2","source":"process_network","target":"end_completed","data":{"outcome":"approved"}}
		]
	}`))

	var got string
	for _, err := range errs {
		if strings.Contains(err.Message, `outcome="rejected"`) {
			got = err.Message
			break
		}
	}
	if got == "" {
		t.Fatalf("expected rejected-edge validation message, got %+v", errs)
	}
	if !strings.Contains(got, "公共结束节点") || !strings.Contains(got, `edge.data.outcome="rejected"`) {
		t.Fatalf("expected shared rejected end guidance, got %q", got)
	}
	if strings.Contains(got, "独立的 end 节点") || strings.Contains(got, "end_rejected") {
		t.Fatalf("validation message still encourages duplicate end nodes: %q", got)
	}
}

func TestBuildActionsContext(t *testing.T) {
	svc := &WorkflowGenerateService{}
	actions := []ServiceAction{
		{Name: "发送邮件", Code: "send-email", Description: "发送通知邮件给相关人员"},
		{Name: "创建工单", Code: "create-ticket", Description: ""},
	}
	result := svc.buildActionsContext(actions)

	if !strings.Contains(result, "可用动作") {
		t.Fatal("should contain header")
	}
	if !strings.Contains(result, "send-email") {
		t.Fatal("should contain first action code")
	}
	if !strings.Contains(result, "发送通知邮件给相关人员") {
		t.Fatal("should contain first action description")
	}
	if !strings.Contains(result, "create-ticket") {
		t.Fatal("should contain second action code")
	}
}

func TestGenerate_RetriesLLMUpstreamError(t *testing.T) {
	client := &fakeWorkflowLLMClient{
		errs:      []error{context.DeadlineExceeded, nil},
		responses: []llm.ChatResponse{{}, {Content: validWorkflowJSONForGenerateTest()}},
	}
	svc := newWorkflowGenerateServiceForRetryTest(client, 3)

	resp, err := svc.Generate(context.Background(), &GenerateRequest{
		CollaborationSpec: "用户提交 VPN 申请后经理审批",
	})
	if err != nil {
		t.Fatalf("generate workflow after retry: %v", err)
	}
	if client.calls != 2 {
		t.Fatalf("expected LLM upstream error to retry once, got %d calls", client.calls)
	}
	if got := client.requests[0].ResponseFormat; got == nil || got.Type != "json_object" {
		t.Fatalf("expected json_object response format, got %+v", got)
	}
	if resp.Retries != 1 {
		t.Fatalf("expected retries=1, got %d", resp.Retries)
	}
}

func TestWorkflowGenerateHandlerReturnsBadGatewayForLLMUpstreamError(t *testing.T) {
	client := &fakeWorkflowLLMClient{
		errs: []error{context.DeadlineExceeded, context.DeadlineExceeded, context.DeadlineExceeded, context.DeadlineExceeded},
	}
	h := &WorkflowGenerateHandler{svc: newWorkflowGenerateServiceForRetryTest(client, 3)}
	c, rec := newGinContext(http.MethodPost, "/api/v1/itsm/workflows/generate")
	c.Request.Body = io.NopCloser(bytes.NewBufferString(`{"collaborationSpec":"用户提交 VPN 申请后经理审批"}`))
	c.Request.Header.Set("Content-Type", "application/json")

	h.Generate(c)

	if rec.Code != http.StatusBadGateway {
		t.Fatalf("expected status 502, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestGenerateRejectsEmptyCollaborationSpec(t *testing.T) {
	client := &fakeWorkflowLLMClient{}
	svc := newWorkflowGenerateServiceForRetryTest(client, 1)

	_, err := svc.Generate(context.Background(), &GenerateRequest{CollaborationSpec: "   "})
	if !errors.Is(err, ErrCollaborationSpecEmpty) {
		t.Fatalf("expected ErrCollaborationSpecEmpty, got %v", err)
	}
	if client.calls != 0 {
		t.Fatalf("expected LLM not to be called, got %d calls", client.calls)
	}
}

func TestGenerateFailsWhenPathEngineConfigMissing(t *testing.T) {
	svc := &WorkflowGenerateService{
		engineConfigSvc: fakePathEngineConfigProvider{err: errors.New("model missing")},
	}

	_, err := svc.Generate(context.Background(), &GenerateRequest{
		CollaborationSpec: "用户提交 VPN 申请后经理审批",
	})
	if !errors.Is(err, ErrPathEngineNotConfigured) {
		t.Fatalf("expected ErrPathEngineNotConfigured, got %v", err)
	}
}

func TestGenerate_RetriesJSONExtractionFailure(t *testing.T) {
	client := &fakeWorkflowLLMClient{
		responses: []llm.ChatResponse{
			{Content: "not json"},
			{Content: validWorkflowJSONForGenerateTest()},
		},
	}
	svc := newWorkflowGenerateServiceForRetryTest(client, 1)

	resp, err := svc.Generate(context.Background(), &GenerateRequest{
		CollaborationSpec: "用户提交 VPN 申请后经理审批",
	})
	if err != nil {
		t.Fatalf("generate workflow: %v", err)
	}
	if client.calls != 2 {
		t.Fatalf("expected JSON extraction failure to retry once, got %d calls", client.calls)
	}
	if resp.Retries != 1 {
		t.Fatalf("expected retries=1, got %d", resp.Retries)
	}
}

func TestGenerate_ReturnsErrorWhenJSONExtractionNeverSucceeds(t *testing.T) {
	client := &fakeWorkflowLLMClient{
		responses: []llm.ChatResponse{{Content: "not json"}},
	}
	svc := newWorkflowGenerateServiceForRetryTest(client, 0)

	_, err := svc.Generate(context.Background(), &GenerateRequest{
		CollaborationSpec: "用户提交 VPN 申请后经理审批",
	})
	if !errors.Is(err, ErrWorkflowGeneration) {
		t.Fatalf("expected ErrWorkflowGeneration, got %v", err)
	}
	if client.calls != 1 {
		t.Fatalf("expected one LLM call, got %d", client.calls)
	}
}

func TestGenerate_RetriesValidationFailure(t *testing.T) {
	client := &fakeWorkflowLLMClient{
		responses: []llm.ChatResponse{
			{Content: `{"nodes":[],"edges":[]}`},
			{Content: validWorkflowJSONForGenerateTest()},
		},
	}
	svc := newWorkflowGenerateServiceForRetryTest(client, 1)

	resp, err := svc.Generate(context.Background(), &GenerateRequest{
		CollaborationSpec: "用户提交 VPN 申请后经理审批",
	})
	if err != nil {
		t.Fatalf("generate workflow: %v", err)
	}
	if client.calls != 2 {
		t.Fatalf("expected validation failure to retry once, got %d calls", client.calls)
	}
	if resp.Retries != 1 {
		t.Fatalf("expected retries=1, got %d", resp.Retries)
	}
}

type vpnWorkflowGenerateOrgResolver struct {
	testServiceDefOrgResolver
}

func (vpnWorkflowGenerateOrgResolver) QueryContext(string, string, string, bool) (*appcore.OrgContextResult, error) {
	return &appcore.OrgContextResult{
		Departments: []appcore.OrgContextDepartment{
			{Code: "it", Name: "信息部", IsActive: true},
		},
		Positions: []appcore.OrgContextPosition{
			{Code: "network_admin", Name: "网络管理员", IsActive: true},
			{Code: "security_admin", Name: "信息安全管理员", IsActive: true},
		},
		Summary: "测试组织上下文",
	}, nil
}

type vpnWorkflowGenerateOrgStructureResolver struct{}

func (vpnWorkflowGenerateOrgStructureResolver) SearchOrgStructure(query string, kinds []string, limit int) (*appcore.OrgStructureSearchResult, error) {
	return &appcore.OrgStructureSearchResult{
		Departments: []appcore.OrgContextDepartment{{Code: "it", Name: "信息部", IsActive: true}},
		Positions: []appcore.OrgContextPosition{
			{Code: "network_admin", Name: "网络管理员", IsActive: true},
			{Code: "security_admin", Name: "信息安全管理员", IsActive: true},
		},
		Summary: "测试组织结构搜索: " + query,
	}, nil
}

func (vpnWorkflowGenerateOrgStructureResolver) ResolveOrgParticipant(departmentHint, positionHint string, limit int) (*appcore.OrgParticipantResolveResult, error) {
	candidate := appcore.OrgParticipantCandidate{
		Type:           "position_department",
		DepartmentCode: "it",
		DepartmentName: "信息部",
		CandidateCount: 2,
	}
	if strings.Contains(positionHint, "安全") {
		candidate.PositionCode = "security_admin"
		candidate.PositionName = "信息安全管理员"
	} else {
		candidate.PositionCode = "network_admin"
		candidate.PositionName = "网络管理员"
	}
	return &appcore.OrgParticipantResolveResult{
		Candidates: []appcore.OrgParticipantCandidate{candidate},
		Summary:    "测试参与人解析",
	}, nil
}

type serverAccessWorkflowGenerateOrgStructureResolver struct{}

func (serverAccessWorkflowGenerateOrgStructureResolver) SearchOrgStructure(query string, kinds []string, limit int) (*appcore.OrgStructureSearchResult, error) {
	return &appcore.OrgStructureSearchResult{
		Departments: []appcore.OrgContextDepartment{{Code: "it", Name: "信息部", IsActive: true}},
		Positions: []appcore.OrgContextPosition{
			{Code: "ops_admin", Name: "运维管理员", IsActive: true},
			{Code: "network_admin", Name: "网络管理员", IsActive: true},
			{Code: "security_admin", Name: "信息安全管理员", IsActive: true},
		},
		Summary: "测试组织结构搜索: " + query,
	}, nil
}

func (serverAccessWorkflowGenerateOrgStructureResolver) ResolveOrgParticipant(departmentHint, positionHint string, limit int) (*appcore.OrgParticipantResolveResult, error) {
	candidate := appcore.OrgParticipantCandidate{
		Type:           "position_department",
		DepartmentCode: "it",
		DepartmentName: "信息部",
		CandidateCount: 3,
	}
	switch {
	case strings.Contains(positionHint, "网络"):
		candidate.PositionCode = "network_admin"
		candidate.PositionName = "网络管理员"
	case strings.Contains(positionHint, "安全"):
		candidate.PositionCode = "security_admin"
		candidate.PositionName = "信息安全管理员"
	default:
		candidate.PositionCode = "ops_admin"
		candidate.PositionName = "运维管理员"
	}
	return &appcore.OrgParticipantResolveResult{
		Candidates: []appcore.OrgParticipantCandidate{candidate},
		Summary:    "测试参与人解析",
	}, nil
}

type dbBackupWorkflowGenerateOrgStructureResolver struct{}

func (dbBackupWorkflowGenerateOrgStructureResolver) SearchOrgStructure(query string, kinds []string, limit int) (*appcore.OrgStructureSearchResult, error) {
	return &appcore.OrgStructureSearchResult{
		Departments: []appcore.OrgContextDepartment{{Code: "it", Name: "信息部", IsActive: true}},
		Positions:   []appcore.OrgContextPosition{{Code: "db_admin", Name: "数据库管理员", IsActive: true}},
		Summary:     "测试组织结构搜索: " + query,
	}, nil
}

func (dbBackupWorkflowGenerateOrgStructureResolver) ResolveOrgParticipant(departmentHint, positionHint string, limit int) (*appcore.OrgParticipantResolveResult, error) {
	return &appcore.OrgParticipantResolveResult{
		Candidates: []appcore.OrgParticipantCandidate{{
			Type:           "position_department",
			DepartmentCode: "it",
			DepartmentName: "信息部",
			PositionCode:   "db_admin",
			PositionName:   "数据库管理员",
			CandidateCount: 1,
		}},
		Summary: "测试参与人解析",
	}, nil
}

type bossWorkflowGenerateOrgStructureResolver struct{}

func (bossWorkflowGenerateOrgStructureResolver) SearchOrgStructure(query string, kinds []string, limit int) (*appcore.OrgStructureSearchResult, error) {
	return &appcore.OrgStructureSearchResult{
		Departments: []appcore.OrgContextDepartment{
			{Code: "headquarters", Name: "总部", IsActive: true},
			{Code: "it", Name: "信息部", IsActive: true},
		},
		Positions: []appcore.OrgContextPosition{
			{Code: "serial_reviewer", Name: "总部处理人", IsActive: true},
			{Code: "ops_admin", Name: "运维管理员", IsActive: true},
		},
		Summary: "测试组织结构搜索: " + query,
	}, nil
}

func (bossWorkflowGenerateOrgStructureResolver) ResolveOrgParticipant(departmentHint, positionHint string, limit int) (*appcore.OrgParticipantResolveResult, error) {
	candidate := appcore.OrgParticipantCandidate{
		Type:           "position_department",
		CandidateCount: 1,
	}
	if strings.Contains(departmentHint, "总部") || strings.Contains(positionHint, "总部") {
		candidate.DepartmentCode = "headquarters"
		candidate.DepartmentName = "总部"
		candidate.PositionCode = "serial_reviewer"
		candidate.PositionName = "总部处理人"
	} else {
		candidate.DepartmentCode = "it"
		candidate.DepartmentName = "信息部"
		candidate.PositionCode = "ops_admin"
		candidate.PositionName = "运维管理员"
	}
	return &appcore.OrgParticipantResolveResult{
		Candidates: []appcore.OrgParticipantCandidate{candidate},
		Summary:    "测试参与人解析",
	}, nil
}

func TestGenerate_WithServiceIDUsesOrgToolPreflightForVPNContext(t *testing.T) {
	db := newTestDB(t)
	catSvc := newCatalogServiceForTest(t, db)
	root, _ := catSvc.Create("Root", "root", "", "", nil, 10)

	vpnSchema := JSONField(`{"version":1,"fields":[{"key":"vpn_account","type":"text","label":"VPN账号"},{"key":"device_usage","type":"textarea","label":"设备与用途说明"},{"key":"request_kind","type":"select","label":"访问原因","options":[{"label":"线上支持","value":"online_support"},{"label":"故障排查","value":"troubleshooting"},{"label":"生产应急","value":"production_emergency"},{"label":"网络接入问题","value":"network_access_issue"},{"label":"外部协作","value":"external_collaboration"},{"label":"长期远程办公","value":"long_term_remote_work"},{"label":"跨境访问","value":"cross_border_access"},{"label":"安全合规事项","value":"security_compliance"}]}]}`)
	service := ServiceDefinition{
		Name:             "VPN 开通申请",
		Code:             "vpn-access-request",
		CatalogID:        root.ID,
		EngineType:       "smart",
		IntakeFormSchema: vpnSchema,
		IsActive:         true,
	}
	if err := db.Create(&service).Error; err != nil {
		t.Fatalf("create vpn service: %v", err)
	}

	client := &fakeWorkflowLLMClient{
		responses: []llm.ChatResponse{
			{ToolCalls: []llm.ToolCall{
				{ID: "call_network", Name: "workflow.org_resolve_participant", Arguments: `{"department_hint":"信息部","position_hint":"网络管理员","limit":10}`},
				{ID: "call_security", Name: "workflow.org_resolve_participant", Arguments: `{"department_hint":"信息部","position_hint":"信息安全管理员","limit":10}`},
			}},
			{Content: "组织上下文已收集"},
			{Content: validVPNWorkflowJSONForGenerateTest()},
		},
	}
	svc := newWorkflowGenerateServiceForRetryTest(client, 0)
	svc.serviceDefSvc = newServiceDefServiceForTest(t, db)
	svc.orgStructureResolver = vpnWorkflowGenerateOrgStructureResolver{}

	naturalSpec := "员工申请 VPN 开通时，确认账号、设备或用途、访问原因。网络支持类交给网络管理员，外部协作、跨境访问或安全合规类交给信息安全管理员。"
	resp, err := svc.Generate(context.Background(), &GenerateRequest{ServiceID: service.ID, CollaborationSpec: naturalSpec})
	if err != nil {
		t.Fatalf("generate workflow: %v", err)
	}
	if len(client.requests) != 3 {
		t.Fatalf("expected org preflight two-turn exchange plus final JSON generation, got %d requests", len(client.requests))
	}
	if len(client.requests[0].Tools) != 2 {
		t.Fatalf("expected org preflight tools, got %+v", client.requests[0].Tools)
	}
	if client.requests[0].ResponseFormat != nil {
		t.Fatalf("preflight request should not force JSON response format")
	}
	if len(client.requests[2].Tools) != 0 {
		t.Fatalf("final workflow generation should not expose org tools, got %+v", client.requests[2].Tools)
	}
	if got := client.requests[2].ResponseFormat; got == nil || got.Type != "json_object" {
		t.Fatalf("final workflow generation should still use json_object response format, got %+v", got)
	}
	if len(client.requests[2].Messages) < 2 {
		t.Fatalf("expected LLM request with user prompt, got %+v", client.requests)
	}
	userPrompt := client.requests[2].Messages[1].Content
	for _, snippet := range []string{
		naturalSpec,
		"已有申请表单契约",
		"vpn_account",
		"device_usage",
		"request_kind",
		"online_support",
		"security_compliance",
		"按需查询到的组织上下文",
		"信息部",
		"it",
		"网络管理员",
		"network_admin",
		"信息安全管理员",
		"security_admin",
	} {
		if !strings.Contains(userPrompt, snippet) {
			t.Fatalf("expected generated prompt to contain %q, got %s", snippet, userPrompt)
		}
	}
	workflowText := string(resp.WorkflowJSON)
	for _, snippet := range []string{"form.request_kind", "network_admin", "security_admin"} {
		if !strings.Contains(workflowText, snippet) {
			t.Fatalf("expected generated workflow to contain %q, got %s", snippet, workflowText)
		}
	}
}

func TestGenerate_WithServiceIDUsesOrgToolPreflightForServerAccessContext(t *testing.T) {
	db := newTestDB(t)
	catSvc := newCatalogServiceForTest(t, db)
	root, _ := catSvc.Create("Root", "root", "", "", nil, 10)

	serverAccessSchema := JSONField(`{"version":1,"fields":[{"key":"target_servers","type":"textarea","label":"访问服务器"},{"key":"access_window","type":"date_range","label":"访问时段"},{"key":"operation_purpose","type":"textarea","label":"操作目的"},{"key":"access_reason","type":"textarea","label":"访问原因"}]}`)
	service := ServiceDefinition{
		Name:             "生产服务器临时访问申请",
		Code:             "prod-server-temporary-access",
		CatalogID:        root.ID,
		EngineType:       "smart",
		IntakeFormSchema: serverAccessSchema,
		IsActive:         true,
	}
	if err := db.Create(&service).Error; err != nil {
		t.Fatalf("create server access service: %v", err)
	}

	client := &fakeWorkflowLLMClient{
		responses: []llm.ChatResponse{
			{ToolCalls: []llm.ToolCall{
				{ID: "call_ops", Name: "workflow.org_resolve_participant", Arguments: `{"department_hint":"信息部","position_hint":"运维管理员","limit":10}`},
				{ID: "call_network", Name: "workflow.org_resolve_participant", Arguments: `{"department_hint":"信息部","position_hint":"网络管理员","limit":10}`},
				{ID: "call_security", Name: "workflow.org_resolve_participant", Arguments: `{"department_hint":"信息部","position_hint":"信息安全管理员","limit":10}`},
			}},
			{Content: "组织上下文已收集"},
			{Content: validServerAccessWorkflowJSONForGenerateTest()},
		},
	}
	svc := newWorkflowGenerateServiceForRetryTest(client, 0)
	svc.serviceDefSvc = newServiceDefServiceForTest(t, db)
	svc.orgStructureResolver = serverAccessWorkflowGenerateOrgStructureResolver{}

	naturalSpec := `员工在 IT 服务台申请生产服务器临时访问时，服务台需要确认要访问的服务器或资源范围、访问时段、本次操作目的，以及为什么需要临时进入生产环境。

访问原因通常分为三类：应用发布、进程排障、日志排查、磁盘清理、主机巡检、生产运维操作偏主机和应用运维，交给信息部运维管理员处理；网络抓包、连通性诊断、ACL 调整、负载均衡变更、防火墙策略调整偏网络诊断与策略处理，交给信息部网络管理员处理；安全审计、入侵排查、漏洞修复验证、取证分析、合规检查偏安全与合规风险，交给信息部信息安全管理员处理。

处理人完成处理后流程结束。`
	resp, err := svc.Generate(context.Background(), &GenerateRequest{ServiceID: service.ID, CollaborationSpec: naturalSpec})
	if err != nil {
		t.Fatalf("generate workflow: %v", err)
	}
	if len(client.requests) != 3 {
		t.Fatalf("expected org preflight two-turn exchange plus final JSON generation, got %d requests", len(client.requests))
	}
	userPrompt := client.requests[2].Messages[1].Content
	for _, snippet := range []string{
		naturalSpec,
		"已有申请表单契约",
		"target_servers",
		"access_window",
		"operation_purpose",
		"access_reason",
		"按需查询到的组织上下文",
		"信息部",
		"it",
		"运维管理员",
		"ops_admin",
		"网络管理员",
		"network_admin",
		"信息安全管理员",
		"security_admin",
	} {
		if !strings.Contains(userPrompt, snippet) {
			t.Fatalf("expected generated prompt to contain %q, got %s", snippet, userPrompt)
		}
	}
	workflowText := string(resp.WorkflowJSON)
	for _, snippet := range []string{"form.access_reason", "ops_admin", "network_admin", "security_admin"} {
		if !strings.Contains(workflowText, snippet) {
			t.Fatalf("expected generated workflow to contain %q, got %s", snippet, workflowText)
		}
	}
}

func TestGenerate_WithServiceIDUsesOrgToolPreflightForDBBackupContext(t *testing.T) {
	db := newTestDB(t)
	catSvc := newCatalogServiceForTest(t, db)
	root, _ := catSvc.Create("Root", "root", "", "", nil, 10)

	dbBackupSchema := JSONField(`{"version":1,"fields":[{"key":"database_name","type":"text","label":"目标数据库"},{"key":"source_ip","type":"text","label":"来源 IP"},{"key":"whitelist_window","type":"text","label":"白名单放行时间窗"},{"key":"access_reason","type":"textarea","label":"申请原因"}]}`)
	service := ServiceDefinition{
		Name:             "生产数据库备份白名单临时放行申请",
		Code:             "db-backup-whitelist-action-flow",
		CatalogID:        root.ID,
		EngineType:       "smart",
		IntakeFormSchema: dbBackupSchema,
		IsActive:         true,
	}
	if err := db.Create(&service).Error; err != nil {
		t.Fatalf("create db backup service: %v", err)
	}
	precheckAction := ServiceAction{
		Name:        "备份白名单预检",
		Code:        "db_backup_whitelist_precheck",
		Description: "校验数据库、来源 IP、放行窗口和申请原因满足放行前置条件",
		ActionType:  "http",
		ServiceID:   service.ID,
		IsActive:    true,
	}
	if err := db.Create(&precheckAction).Error; err != nil {
		t.Fatalf("create precheck action: %v", err)
	}
	applyAction := ServiceAction{
		Name:        "执行备份白名单放行",
		Code:        "db_backup_whitelist_apply",
		Description: "数据库管理员处理完成后执行备份白名单放行",
		ActionType:  "http",
		ServiceID:   service.ID,
		IsActive:    true,
	}
	if err := db.Create(&applyAction).Error; err != nil {
		t.Fatalf("create apply action: %v", err)
	}

	client := &fakeWorkflowLLMClient{
		responses: []llm.ChatResponse{
			{ToolCalls: []llm.ToolCall{
				{ID: "call_db", Name: "workflow.org_resolve_participant", Arguments: `{"department_hint":"信息部","position_hint":"数据库管理员","limit":10}`},
			}},
			{Content: "组织上下文已收集"},
			{Content: validDBBackupWorkflowJSONForGenerateTest(precheckAction.ID, applyAction.ID)},
		},
	}
	svc := newWorkflowGenerateServiceForRetryTest(client, 0)
	svc.serviceDefSvc = newServiceDefServiceForTest(t, db)
	svc.actionRepo = &ServiceActionRepo{db: &database.DB{DB: db}}
	svc.orgStructureResolver = dbBackupWorkflowGenerateOrgStructureResolver{}

	naturalSpec := `员工在 IT 服务台申请生产数据库备份白名单临时放行时，服务台需要确认目标数据库、发起备份访问的来源 IP、白名单放行时间窗，以及这次临时放行的申请原因。
申请资料收齐后，系统会先做一次白名单参数预检，确认数据库、来源 IP、放行窗口和申请原因满足放行前置条件。预检通过后，交给信息部数据库管理员处理。
数据库管理员完成处理后，系统执行备份白名单放行；放行成功后流程结束。驳回时不进入补充或返工，流程按驳回结果结束。`
	resp, err := svc.Generate(context.Background(), &GenerateRequest{ServiceID: service.ID, CollaborationSpec: naturalSpec})
	if err != nil {
		t.Fatalf("generate workflow: %v", err)
	}
	if len(client.requests) != 3 {
		t.Fatalf("expected org preflight two-turn exchange plus final JSON generation, got %d requests", len(client.requests))
	}
	userPrompt := client.requests[2].Messages[1].Content
	for _, snippet := range []string{
		naturalSpec,
		"已有申请表单契约",
		"database_name",
		"source_ip",
		"whitelist_window",
		"access_reason",
		"按需查询到的组织上下文",
		"信息部",
		"it",
		"数据库管理员",
		"db_admin",
		"必须生成两个 type=\"action\" 节点",
		"db_backup_whitelist_precheck",
		"db_backup_whitelist_apply",
		fmt.Sprintf("id: `%d`", precheckAction.ID),
		fmt.Sprintf("id: `%d`", applyAction.ID),
		"decision.execute_action",
	} {
		if !strings.Contains(userPrompt, snippet) {
			t.Fatalf("expected generated prompt to contain %q, got %s", snippet, userPrompt)
		}
	}
	if strings.Contains(userPrompt, "workflow_json 不生成 type=\"action\"") {
		t.Fatalf("db backup prompt still forbids action nodes: %s", userPrompt)
	}
	workflowText := string(resp.WorkflowJSON)
	for _, snippet := range []string{
		"database_name",
		"source_ip",
		"whitelist_window",
		"access_reason",
		"db_admin",
		`"type":"action"`,
		"db_precheck_action",
		"db_apply_action",
		fmt.Sprintf(`"action_id":%d`, precheckAction.ID),
		fmt.Sprintf(`"action_id":%d`, applyAction.ID),
	} {
		if !strings.Contains(workflowText, snippet) {
			t.Fatalf("expected generated workflow to contain %q, got %s", snippet, workflowText)
		}
	}
	var generated struct {
		Edges []struct {
			Source string `json:"source"`
			Target string `json:"target"`
			Data   struct {
				Outcome string `json:"outcome"`
			} `json:"data"`
		} `json:"edges"`
	}
	if err := json.Unmarshal(resp.WorkflowJSON, &generated); err != nil {
		t.Fatalf("unmarshal generated workflow: %v", err)
	}
	for _, edge := range generated.Edges {
		if edge.Source == "db_process" && edge.Data.Outcome == "rejected" && edge.Target == "db_apply_action" {
			t.Fatalf("rejected edge must not pass through apply action: %s", workflowText)
		}
	}
}

func TestGenerate_WithServiceIDUsesOrgToolPreflightForBossContext(t *testing.T) {
	db := newTestDB(t)
	catSvc := newCatalogServiceForTest(t, db)
	root, _ := catSvc.Create("Root", "root", "", "", nil, 10)

	bossSchema := JSONField(`{"version":1,"fields":[{"key":"subject","type":"text","label":"申请主题"},{"key":"request_category","type":"select","label":"申请类别","options":[{"label":"生产变更","value":"prod_change"},{"label":"访问授权","value":"access_grant"},{"label":"应急支持","value":"emergency_support"}]},{"key":"risk_level","type":"radio","label":"风险等级","options":[{"label":"低","value":"low"},{"label":"中","value":"medium"},{"label":"高","value":"high"}]},{"key":"expected_finish_time","type":"datetime","label":"期望完成时间"},{"key":"change_window","type":"date_range","label":"变更窗口"},{"key":"impact_scope","type":"textarea","label":"影响范围"},{"key":"rollback_required","type":"select","label":"回滚要求","options":[{"label":"需要","value":"required"},{"label":"不需要","value":"not_required"}]},{"key":"impact_modules","type":"multi_select","label":"影响模块","options":[{"label":"网关","value":"gateway"},{"label":"支付","value":"payment"},{"label":"监控","value":"monitoring"},{"label":"订单","value":"order"}]},{"key":"change_items","type":"table","label":"变更明细表","props":{"columns":[{"key":"system","type":"text","label":"系统"},{"key":"resource","type":"text","label":"资源"},{"key":"permission_level","type":"select","label":"权限级别","options":[{"label":"只读","value":"read"},{"label":"读写","value":"read_write"}]},{"key":"effective_range","type":"date_range","label":"生效时段"},{"key":"reason","type":"text","label":"变更理由"}]}}]}`)
	service := ServiceDefinition{
		Name:             "高风险变更协同申请（Boss）",
		Code:             "boss-serial-change-request",
		CatalogID:        root.ID,
		EngineType:       "smart",
		IntakeFormSchema: bossSchema,
		IsActive:         true,
	}
	if err := db.Create(&service).Error; err != nil {
		t.Fatalf("create boss service: %v", err)
	}

	client := &fakeWorkflowLLMClient{
		responses: []llm.ChatResponse{
			{ToolCalls: []llm.ToolCall{
				{ID: "call_head", Name: "workflow.org_resolve_participant", Arguments: `{"department_hint":"总部","position_hint":"总部处理人","limit":10}`},
				{ID: "call_ops", Name: "workflow.org_resolve_participant", Arguments: `{"department_hint":"信息部","position_hint":"运维管理员","limit":10}`},
			}},
			{Content: "组织上下文已收集"},
			{Content: validBossWorkflowJSONForGenerateTest()},
		},
	}
	svc := newWorkflowGenerateServiceForRetryTest(client, 0)
	svc.serviceDefSvc = newServiceDefServiceForTest(t, db)
	svc.orgStructureResolver = bossWorkflowGenerateOrgStructureResolver{}

	naturalSpec := `员工在 IT 服务台提交高风险变更协同申请时，服务台需要确认申请主题、申请类别、风险等级、期望完成时间、变更窗口、影响范围、回滚要求、影响模块，以及每一项变更明细。
申请类别包括生产变更、访问授权和应急支持；风险等级包括低、中、高；回滚要求包括需要和不需要；影响模块可选择网关、支付、监控和订单。变更明细需要说明系统、资源、权限级别、生效时段和变更理由，权限级别包括只读和读写。
申请提交后，先交给总部处理人处理；总部处理人完成后，再交给信息部运维管理员处理。运维管理员完成处理后流程结束。`
	resp, err := svc.Generate(context.Background(), &GenerateRequest{ServiceID: service.ID, CollaborationSpec: naturalSpec})
	if err != nil {
		t.Fatalf("generate workflow: %v", err)
	}
	if len(client.requests) != 3 {
		t.Fatalf("expected org preflight two-turn exchange plus final JSON generation, got %d requests", len(client.requests))
	}
	userPrompt := client.requests[2].Messages[1].Content
	for _, snippet := range []string{
		naturalSpec,
		"已有申请表单契约",
		"subject",
		"request_category",
		"prod_change",
		"risk_level",
		"rollback_required",
		"impact_modules",
		"change_items",
		"permission_level",
		"read_write",
		"按需查询到的组织上下文",
		"总部",
		"headquarters",
		"总部处理人",
		"serial_reviewer",
		"信息部",
		"it",
		"运维管理员",
		"ops_admin",
	} {
		if !strings.Contains(userPrompt, snippet) {
			t.Fatalf("expected generated prompt to contain %q, got %s", snippet, userPrompt)
		}
	}
	workflowText := string(resp.WorkflowJSON)
	for _, snippet := range []string{"subject", "request_category", "change_items", "headquarters", "serial_reviewer", "it", "ops_admin"} {
		if !strings.Contains(workflowText, snippet) {
			t.Fatalf("expected generated workflow to contain %q, got %s", snippet, workflowText)
		}
	}
	if strings.Contains(workflowText, "serial-reviewer") {
		t.Fatalf("boss reference workflow must not contain legacy fixed user, got %s", workflowText)
	}
}

func TestWorkflowGenerateHandlerReturnsOKForParsableWorkflowWithBlockingIssues(t *testing.T) {
	client := &fakeWorkflowLLMClient{
		responses: []llm.ChatResponse{{Content: workflowWithBlockingIssueForGenerateTest(42)}},
	}
	h := &WorkflowGenerateHandler{svc: newWorkflowGenerateServiceForRetryTest(client, 0)}
	c, rec := newGinContext(http.MethodPost, "/api/v1/itsm/workflows/generate")
	c.Request.Body = io.NopCloser(bytes.NewBufferString(`{"collaborationSpec":"用户提交 VPN 申请后经理审批"}`))
	c.Request.Header.Set("Content-Type", "application/json")

	h.Generate(c)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var got struct {
		Code int              `json:"code"`
		Data GenerateResponse `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if got.Code != 0 {
		t.Fatalf("expected unified response code 0, got %+v", got)
	}
	if len(got.Data.Errors) == 0 {
		t.Fatalf("expected validation errors in response, got %+v", got.Data)
	}
	if !got.Data.Saved {
		t.Fatalf("expected no-service draft response to be marked saved, got %+v", got.Data)
	}
}

func TestWorkflowValidationErrorsLogValue(t *testing.T) {
	got := workflowValidationErrorsLogValue([]engine.ValidationError{
		{NodeID: "gateway-1", EdgeID: "edge-2", Level: "blocking", Message: "排他网关缺少默认分支"},
		{NodeID: "approve-1", Message: "处理节点缺少参与人"},
	})

	if !strings.Contains(got, "[blocking] node=gateway-1 edge=edge-2 排他网关缺少默认分支") {
		t.Fatalf("expected first validation error details, got %q", got)
	}
	if !strings.Contains(got, "[blocking] node=approve-1 处理节点缺少参与人") {
		t.Fatalf("expected default blocking level and node details, got %q", got)
	}
}

func TestWorkflowValidationErrorsLogValueTruncatesLongLists(t *testing.T) {
	got := workflowValidationErrorsLogValue([]engine.ValidationError{
		{Message: "err-1"},
		{Message: "err-2"},
		{Message: "err-3"},
		{Message: "err-4"},
		{Message: "err-5"},
		{Message: "err-6"},
	})

	if strings.Contains(got, "err-6") {
		t.Fatalf("expected log details to be truncated, got %q", got)
	}
	if !strings.Contains(got, "... 1 more") {
		t.Fatalf("expected truncated count, got %q", got)
	}
}

func TestBuildGenerateResponse_PersistsWorkflowAndHealthSnapshot(t *testing.T) {
	db := newTestDB(t)
	serviceDefs := newServiceDefServiceForTest(t, db)
	catSvc := newCatalogServiceForTest(t, db)

	root, _ := catSvc.Create("Root", "root", "", "", nil, 10)
	service, err := serviceDefs.Create(&ServiceDefinition{
		Name:       "Smart",
		Code:       "smart-generate-response",
		CatalogID:  root.ID,
		EngineType: "smart",
	})
	if err != nil {
		t.Fatalf("create service: %v", err)
	}

	svc := &WorkflowGenerateService{serviceDefSvc: serviceDefs}
	workflowJSON := json.RawMessage(validWorkflowJSONForGenerateTest())
	resp, err := svc.buildGenerateResponse(&GenerateRequest{
		ServiceID:         service.ID,
		CollaborationSpec: "用户提交申请后直属经理处理",
	}, workflowJSON, nil, 0, nil)
	if err != nil {
		t.Fatalf("build response: %v", err)
	}
	if resp.Service == nil || resp.HealthCheck == nil {
		t.Fatalf("expected service and health check in response, got %+v", resp)
	}
	if string(resp.Service.WorkflowJSON) != string(workflowJSON) {
		t.Fatalf("expected workflow json to be saved, got %s", resp.Service.WorkflowJSON)
	}
	if len(resp.Service.IntakeFormSchema) == 0 {
		t.Fatal("expected generated intake form schema to be saved")
	}
	if resp.Service.CollaborationSpec != "用户提交申请后直属经理处理" {
		t.Fatalf("expected collaboration spec to be saved, got %q", resp.Service.CollaborationSpec)
	}
	if resp.Service.PublishHealthCheck == nil {
		t.Fatal("expected service response to include saved health snapshot")
	}
}

func TestBuildGenerateResponse_PersistsBlockingDraftAndHealthFailure(t *testing.T) {
	db := newTestDB(t)
	serviceDefs := newServiceDefServiceForTest(t, db)
	catSvc := newCatalogServiceForTest(t, db)

	root, _ := catSvc.Create("Root", "root", "", "", nil, 10)
	user := createServiceHealthUser(t, db, "operator", true)
	serviceAgent := createServiceHealthAgent(t, db, "service-agent", true)
	decisionAgent := createServiceHealthAgent(t, db, "decision-agent", true)
	setServiceHealthDecisionAgent(t, db, decisionAgent.ID)
	seedServiceHealthPathEngine(t, db)
	service, err := serviceDefs.Create(&ServiceDefinition{
		Name:              "Smart",
		Code:              "smart-blocking-draft",
		CatalogID:         root.ID,
		EngineType:        "smart",
		CollaborationSpec: "旧协作规范",
		AgentID:           &serviceAgent.ID,
	})
	if err != nil {
		t.Fatalf("create service: %v", err)
	}

	workflowJSON := json.RawMessage(workflowWithBlockingIssueForGenerateTest(user.ID))
	validationErrors := engine.ValidateWorkflow(workflowJSON)
	if !hasBlockingErrors(validationErrors) {
		t.Fatalf("expected blocking validation errors, got %+v", validationErrors)
	}

	svc := &WorkflowGenerateService{serviceDefSvc: serviceDefs}
	resp, err := svc.buildGenerateResponse(&GenerateRequest{
		ServiceID:         service.ID,
		CollaborationSpec: "用户提交申请后由处理人处理",
	}, workflowJSON, nil, 0, validationErrors)
	if err != nil {
		t.Fatalf("build response: %v", err)
	}
	if !resp.Saved {
		t.Fatalf("expected blocking draft to be saved, got %+v", resp)
	}
	if resp.Service == nil || resp.HealthCheck == nil {
		t.Fatalf("expected service and health check in response, got %+v", resp)
	}
	if len(resp.Errors) != len(validationErrors) {
		t.Fatalf("expected validation errors to be preserved, got %+v", resp.Errors)
	}
	if resp.HealthCheck.Status != "fail" {
		t.Fatalf("expected health check fail for blocking draft, got %+v", resp.HealthCheck)
	}
	if !serviceHealthHasItem(resp.HealthCheck, "reference_path", "fail") {
		t.Fatalf("expected reference_path fail item, got %+v", resp.HealthCheck.Items)
	}
	if serviceHealthHasItem(resp.HealthCheck, "health_engine", "fail") {
		t.Fatalf("blocking draft should not be masked as health_engine failure: %+v", resp.HealthCheck.Items)
	}
	if !serviceHealthHasMessageContaining(resp.HealthCheck, "reference_path", `缺少 outcome="rejected"`) {
		t.Fatalf("expected reference_path item to expose rejected-edge validation error, got %+v", resp.HealthCheck.Items)
	}
	if string(resp.Service.WorkflowJSON) != string(workflowJSON) {
		t.Fatalf("expected workflow json to be saved, got %s", resp.Service.WorkflowJSON)
	}
	if resp.Service.CollaborationSpec != "用户提交申请后由处理人处理" {
		t.Fatalf("expected collaboration spec to be saved, got %q", resp.Service.CollaborationSpec)
	}
}

// ---------------------------------------------------------------------------
// Layer 2: LLM integration tests — environment-gated
// ---------------------------------------------------------------------------

type llmTestEnv struct {
	baseURL string
	apiKey  string
	model   string
}

func requireLLMEnv(t *testing.T) llmTestEnv {
	t.Helper()
	baseURL := os.Getenv("LLM_TEST_BASE_URL")
	apiKey := os.Getenv("LLM_TEST_API_KEY")
	model := os.Getenv("LLM_TEST_MODEL")
	if baseURL == "" || apiKey == "" || model == "" {
		t.Skip("LLM integration test skipped: set LLM_TEST_BASE_URL, LLM_TEST_API_KEY, LLM_TEST_MODEL")
	}
	return llmTestEnv{baseURL: baseURL, apiKey: apiKey, model: model}
}

// callLLMForWorkflow calls the LLM with the production PathBuilder prompt and
// feeds blocking validation errors back into the next attempt.
func callLLMForWorkflow(t *testing.T, env llmTestEnv, spec string) (json.RawMessage, []engine.ValidationError) {
	return callLLMForWorkflowWithContext(t, env, spec, workflowPromptContext{})
}

func callLLMForWorkflowWithContext(t *testing.T, env llmTestEnv, spec string, promptCtx workflowPromptContext) (json.RawMessage, []engine.ValidationError) {
	t.Helper()

	client, err := llm.NewClient(llm.ProtocolOpenAI, env.baseURL, env.apiKey)
	if err != nil {
		t.Fatalf("failed to create LLM client: %v", err)
	}

	temp := float32(0.3)
	svc := &WorkflowGenerateService{}
	var lastErrors []engine.ValidationError
	var lastWorkflowJSON json.RawMessage

	for attempt := 0; attempt <= 3; attempt++ {
		userMsg := svc.buildUserMessage(spec, promptCtx, lastErrors)
		ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
		resp, err := client.Chat(ctx, llm.ChatRequest{
			Model: env.model,
			Messages: []llm.Message{
				{Role: llm.RoleSystem, Content: PathBuilderSystemPrompt},
				{Role: llm.RoleUser, Content: userMsg},
			},
			Temperature:    &temp,
			MaxTokens:      4096,
			ResponseFormat: &llm.ResponseFormat{Type: "json_object"},
		})
		cancel()
		if err != nil {
			t.Fatalf("LLM call failed on attempt %d: %v", attempt+1, err)
		}

		t.Logf("LLM raw response attempt %d (%d chars):\n%s", attempt+1, len(resp.Content), resp.Content)

		workflowJSON, err := extractJSON(resp.Content)
		if err != nil {
			lastErrors = []engine.ValidationError{{Level: "blocking", Message: fmt.Sprintf("输出不是有效 JSON: %v", err)}}
			if attempt < 3 {
				continue
			}
			t.Fatalf("extractJSON failed after retries: %v\nraw response:\n%s", err, resp.Content)
		}
		lastWorkflowJSON = workflowJSON

		validationErrors := engine.ValidateWorkflow(workflowJSON)
		lastErrors = blockingValidationErrors(validationErrors)
		if len(lastErrors) == 0 {
			return workflowJSON, validationErrors
		}
		if attempt == 3 {
			return workflowJSON, validationErrors
		}
	}

	return lastWorkflowJSON, lastErrors
}

func blockingValidationErrors(validationErrors []engine.ValidationError) []engine.ValidationError {
	var blocking []engine.ValidationError
	for _, validationErr := range validationErrors {
		if !validationErr.IsWarning() {
			blocking = append(blocking, validationErr)
		}
	}
	return blocking
}

func TestLLMExtract_SimpleWorkflow(t *testing.T) {
	env := requireLLMEnv(t)

	spec := `这是一个简单的报修服务流程：
1. 用户提交报修表单，填写故障描述
2. IT 支持工程师处理报修
3. 流程结束`

	workflowJSON, validationErrors := callLLMForWorkflow(t, env, spec)

	// Parse and check structural invariants
	def, err := engine.ParseWorkflowDef(workflowJSON)
	if err != nil {
		t.Fatalf("ParseWorkflowDef failed: %v", err)
	}

	var startCount, endCount int
	for _, n := range def.Nodes {
		switch n.Type {
		case "start":
			startCount++
		case "end":
			endCount++
		}
	}
	if startCount != 1 {
		t.Errorf("expected exactly 1 start node, got %d", startCount)
	}
	if endCount < 1 {
		t.Errorf("expected at least 1 end node, got %d", endCount)
	}

	// Check no error-level validation errors (warnings are OK)
	for _, ve := range validationErrors {
		if !ve.IsWarning() {
			t.Errorf("validation error: [%s] %s", ve.NodeID, ve.Message)
		}
	}
}

func TestLLMExtract_BranchWorkflow(t *testing.T) {
	env := requireLLMEnv(t)

	spec := `这是一个需要处理的 VPN 申请流程：
1. 用户提交 VPN 申请表单
2. 部门经理处理
3. 如果处理完成，IT 执行开通操作，然后结束
4. 如果处理取消，通知用户被取消，然后结束`

	workflowJSON, validationErrors := callLLMForWorkflow(t, env, spec)

	def, err := engine.ParseWorkflowDef(workflowJSON)
	if err != nil {
		t.Fatalf("ParseWorkflowDef failed: %v", err)
	}

	// Check structural invariants
	var startCount, endCount int
	for _, n := range def.Nodes {
		switch n.Type {
		case "start":
			startCount++
		case "end":
			endCount++
		}
	}
	if startCount != 1 {
		t.Errorf("expected exactly 1 start node, got %d", startCount)
	}
	if endCount < 1 {
		t.Errorf("expected at least 1 end node, got %d", endCount)
	}

	// Build outgoing edge map to verify any generated exclusive gateway is valid.
	outEdges := make(map[string]int)
	for _, e := range def.Edges {
		outEdges[e.Source]++
	}
	for _, n := range def.Nodes {
		if n.Type == "exclusive" {
			if outEdges[n.ID] < 2 {
				t.Errorf("exclusive gateway %s has %d outgoing edges, expected ≥2", n.ID, outEdges[n.ID])
			}
		}
	}

	// Check no error-level validation errors
	for _, ve := range validationErrors {
		if !ve.IsWarning() {
			t.Errorf("validation error: [%s] %s", ve.NodeID, ve.Message)
		}
	}
}

func TestLLMExtract_VPNBranchWorkflowPreservesFormAndRoutingContract(t *testing.T) {
	env := requireLLMEnv(t)

	spec := `员工在 IT 服务台申请开通 VPN 时，服务台需要确认 VPN 账号、准备用什么设备或场景使用，以及这次访问的主要原因。
访问原因包括线上支持、故障排查、生产应急、网络接入问题、外部协作、长期远程办公、跨境访问和安全合规事项。
线上支持、故障排查、生产应急、网络接入问题偏网络连通与业务支持，交给信息部网络管理员处理；外部协作、长期远程办公、跨境访问、安全合规事项涉及外部、长期、跨境或合规风险，交给信息部信息安全管理员处理。
处理人完成处理后流程结束。`
	svc := &WorkflowGenerateService{}
	promptCtx := workflowPromptContext{
		FormContractContext: svc.buildFormContractContext(JSONField(`{"version":1,"fields":[{"key":"vpn_account","type":"text","label":"VPN账号"},{"key":"device_usage","type":"textarea","label":"设备与用途说明"},{"key":"request_kind","type":"select","label":"访问原因","options":[{"label":"线上支持","value":"online_support"},{"label":"故障排查","value":"troubleshooting"},{"label":"生产应急","value":"production_emergency"},{"label":"网络接入问题","value":"network_access_issue"},{"label":"外部协作","value":"external_collaboration"},{"label":"长期远程办公","value":"long_term_remote_work"},{"label":"跨境访问","value":"cross_border_access"},{"label":"安全合规事项","value":"security_compliance"}]}]}`)),
		OrgContext: strings.Join([]string{
			"\n\n## 组织架构上下文",
			"- 部门：信息部（code: `it`）",
			"- 岗位：网络管理员（code: `network_admin`）",
			"- 岗位：信息安全管理员（code: `security_admin`）",
		}, "\n"),
	}

	workflowJSON, validationErrors := callLLMForWorkflowWithContext(t, env, spec, promptCtx)
	if blocking := blockingValidationErrors(validationErrors); len(blocking) > 0 {
		t.Fatalf("expected no blocking validation errors, got %+v\nworkflow=%s", blocking, workflowJSON)
	}

	def, err := engine.ParseWorkflowDef(workflowJSON)
	if err != nil {
		t.Fatalf("ParseWorkflowDef failed: %v", err)
	}

	formSchema := generatedFormSchemaForLLMTest(t, workflowJSON)
	fieldKeys := make(map[string]bool)
	optionValues := make(map[string]bool)
	for _, field := range formSchema.Fields {
		fieldKeys[field.Key] = true
		if field.Key == "request_kind" {
			if field.Type != "select" {
				t.Fatalf("request_kind should be select, got %q", field.Type)
			}
			for _, option := range field.Options {
				optionValues[fmt.Sprintf("%v", option.Value)] = true
			}
		}
	}
	expectedKeys := []string{"vpn_account", "device_usage", "request_kind"}
	if len(fieldKeys) != len(expectedKeys) {
		t.Fatalf("expected only VPN intake fields %v, got %#v", expectedKeys, fieldKeys)
	}
	for _, key := range expectedKeys {
		if !fieldKeys[key] {
			t.Fatalf("expected formSchema field %q, got %#v", key, fieldKeys)
		}
	}
	for _, value := range []string{"online_support", "troubleshooting", "production_emergency", "network_access_issue", "external_collaboration", "long_term_remote_work", "cross_border_access", "security_compliance"} {
		if !optionValues[value] {
			t.Fatalf("expected request_kind option %q, got %#v", value, optionValues)
		}
	}

	var networkParticipant, securityParticipant bool
	var requestKindConditions int
	for _, node := range def.Nodes {
		nd, err := engine.ParseNodeData(node.Data)
		if err != nil {
			t.Fatalf("parse node %s data: %v", node.ID, err)
		}
		for _, participant := range nd.Participants {
			if participant.Type == "position_department" && participant.DepartmentCode == "it" && participant.PositionCode == "network_admin" {
				networkParticipant = true
			}
			if participant.Type == "position_department" && participant.DepartmentCode == "it" && participant.PositionCode == "security_admin" {
				securityParticipant = true
			}
		}
	}
	for _, edge := range def.Edges {
		if edge.Data.Condition != nil && edge.Data.Condition.Field == "form.request_kind" {
			requestKindConditions++
		}
	}
	if !networkParticipant || !securityParticipant {
		t.Fatalf("expected network/security position_department participants, got workflow=%s", workflowJSON)
	}
	if requestKindConditions < 2 {
		t.Fatalf("expected request_kind branch conditions, got workflow=%s", workflowJSON)
	}
}

func generatedFormSchemaForLLMTest(t *testing.T, workflowJSON json.RawMessage) formSchemaForLLMTest {
	t.Helper()
	schemaJSON, errs := extractGeneratedIntakeFormSchema(workflowJSON)
	if len(errs) > 0 {
		t.Fatalf("extract generated form schema: %+v", errs)
	}
	var schema formSchemaForLLMTest
	if err := json.Unmarshal(schemaJSON, &schema); err != nil {
		t.Fatalf("unmarshal generated form schema: %v", err)
	}
	return schema
}

type formSchemaForLLMTest struct {
	Fields []struct {
		Key     string `json:"key"`
		Type    string `json:"type"`
		Options []struct {
			Value any `json:"value"`
		} `json:"options"`
	} `json:"fields"`
}
