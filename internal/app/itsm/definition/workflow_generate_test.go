package definition

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	. "metis/internal/app/itsm/config"
	. "metis/internal/app/itsm/domain"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"metis/internal/app/itsm/engine"
	"metis/internal/llm"
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
	return `{"nodes":[{"id":"start","type":"start","data":{"label":"开始"}},{"id":"end","type":"end","data":{"label":"结束"}}],"edges":[{"id":"e1","source":"start","target":"end","data":{}}]}`
}

func workflowWithBlockingIssueForGenerateTest(userID uint) string {
	return fmt.Sprintf(`{
		"nodes": [
			{"id":"start","type":"start","data":{"label":"开始"}},
			{"id":"process","type":"process","data":{"label":"处理","participants":[{"type":"user","value":"%d"}]}},
			{"id":"end","type":"end","data":{"label":"结束"}}
		],
		"edges": [
			{"id":"e1","source":"start","target":"process","data":{}},
			{"id":"e2","source":"process","target":"end","data":{"outcome":"approved"}}
		]
	}`, userID)
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

// ---------------------------------------------------------------------------
// Layer 1: Unit tests — buildUserMessage / buildActionsContext
// ---------------------------------------------------------------------------

func TestBuildUserMessage_Basic(t *testing.T) {
	svc := &WorkflowGenerateService{}
	msg := svc.buildUserMessage("用户提交表单后经理处理", "", nil)

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
		{Name: "发送邮件", Code: "send-email", Description: "发送通知邮件"},
	})
	msg := svc.buildUserMessage("处理流程", actionsCtx, nil)

	if !strings.Contains(msg, "处理流程") {
		t.Fatal("message should contain the collaboration spec")
	}
	if !strings.Contains(msg, "可用动作") {
		t.Fatal("message should contain actions context")
	}
	if !strings.Contains(msg, "send-email") {
		t.Fatal("message should contain action code")
	}
}

func TestBuildUserMessage_WithPrevErrors(t *testing.T) {
	svc := &WorkflowGenerateService{}
	prevErrors := []engine.ValidationError{
		{NodeID: "node-1", Level: "blocking", Message: "缺少出边"},
		{EdgeID: "edge-2", Level: "blocking", Message: "引用了不存在的目标节点"},
		{NodeID: "node-3", Level: "blocking", Message: "人工节点 node-3 必须配置处理人"},
	}
	msg := svc.buildUserMessage("处理流程", "", prevErrors)

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

func TestPathBuilderSystemPromptRequiresHumanNodeParticipants(t *testing.T) {
	requiredSnippets := []string{
		"所有 form/process 等人工节点必须在 data 中配置非空 participants 数组",
		"不要把 participantType、positionCode、departmentCode 直接放在 data 上",
		`"participants":[{"type":"requester"}]`,
		`type: "requester" | "user"`,
		`"participants":[{"type":"position_department","department_code":"it","position_code":"network_admin"}]`,
		`"position_code":"security_admin"`,
	}
	for _, snippet := range requiredSnippets {
		if !strings.Contains(PathBuilderSystemPrompt, snippet) {
			t.Fatalf("system prompt missing required participant guidance: %s", snippet)
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

func TestGenerate_FailsFastOnLLMUpstreamError(t *testing.T) {
	client := &fakeWorkflowLLMClient{
		errs: []error{context.DeadlineExceeded},
	}
	svc := newWorkflowGenerateServiceForRetryTest(client, 3)

	_, err := svc.Generate(context.Background(), &GenerateRequest{
		CollaborationSpec: "用户提交 VPN 申请后经理审批",
	})
	if !errors.Is(err, ErrPathEngineUpstream) {
		t.Fatalf("expected ErrPathEngineUpstream, got %v", err)
	}
	if client.calls != 1 {
		t.Fatalf("expected LLM to be called once for upstream errors, got %d", client.calls)
	}
	if got := client.requests[0].ResponseFormat; got == nil || got.Type != "json_object" {
		t.Fatalf("expected json_object response format, got %+v", got)
	}
}

func TestWorkflowGenerateHandlerReturnsBadGatewayForLLMUpstreamError(t *testing.T) {
	client := &fakeWorkflowLLMClient{
		errs: []error{context.DeadlineExceeded},
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

func TestWorkflowGenerateHandlerReturnsOKForParsableWorkflowWithBlockingIssues(t *testing.T) {
	client := &fakeWorkflowLLMClient{
		responses: []llm.ChatResponse{{Content: `{"nodes":[],"edges":[]}`}},
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
	workflowJSON := json.RawMessage(`{"nodes":[],"edges":[]}`)
	resp, err := svc.buildGenerateResponse(&GenerateRequest{
		ServiceID:         service.ID,
		CollaborationSpec: "用户提交申请后直属经理处理",
	}, workflowJSON, 0, nil)
	if err != nil {
		t.Fatalf("build response: %v", err)
	}
	if resp.Service == nil || resp.HealthCheck == nil {
		t.Fatalf("expected service and health check in response, got %+v", resp)
	}
	if string(resp.Service.WorkflowJSON) != string(workflowJSON) {
		t.Fatalf("expected workflow json to be saved, got %s", resp.Service.WorkflowJSON)
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
	}, workflowJSON, 0, validationErrors)
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

const testSystemPrompt = `你是工作流 JSON 生成器。输入是协作规范，输出是严格的 JSON。

JSON schema:
{
  "nodes": [{"id": "string", "type": "string", "data": {"label": "string"}}],
  "edges": [{"id": "string", "source": "string", "target": "string", "data": {}}]
}

节点类型: start, end, form, process, process, action, notify, exclusive
每个 node 必须有 id, type。data 字段包含 label。
每个 edge 必须有 id, source, target。
必须恰好 1 个 start 节点，至少 1 个 end 节点。

排他网关(exclusive)节点至少有 2 条出边。出边的 data 中：
- 条件分支必须包含 "condition" 对象: {"field": "string", "operator": "equals", "value": "string"}
- 默认分支使用 "default": true
- 至少一条出边应标记 "default": true

示例：排他网关的出边 data:
  条件边: {"condition": {"field": "process.result", "operator": "equals", "value": "completed"}}
  默认边: {"default": true}

仅输出合法的 JSON，不要包含任何额外文字或 markdown 代码块标记。`

// callLLMForWorkflow calls the LLM with the test system prompt and returns
// the extracted + validated workflow JSON, along with diagnostic info.
func callLLMForWorkflow(t *testing.T, env llmTestEnv, spec string) (json.RawMessage, []engine.ValidationError) {
	t.Helper()

	client, err := llm.NewClient(llm.ProtocolOpenAI, env.baseURL, env.apiKey)
	if err != nil {
		t.Fatalf("failed to create LLM client: %v", err)
	}

	svc := &WorkflowGenerateService{}
	userMsg := svc.buildUserMessage(spec, "", nil)

	temp := float32(0.3)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	resp, err := client.Chat(ctx, llm.ChatRequest{
		Model: env.model,
		Messages: []llm.Message{
			{Role: llm.RoleSystem, Content: testSystemPrompt},
			{Role: llm.RoleUser, Content: userMsg},
		},
		Temperature: &temp,
		MaxTokens:   4096,
	})
	if err != nil {
		t.Fatalf("LLM call failed: %v", err)
	}

	t.Logf("LLM raw response (%d chars):\n%s", len(resp.Content), resp.Content)

	workflowJSON, err := extractJSON(resp.Content)
	if err != nil {
		t.Fatalf("extractJSON failed: %v\nraw response:\n%s", err, resp.Content)
	}

	validationErrors := engine.ValidateWorkflow(workflowJSON)
	return workflowJSON, validationErrors
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
	var startCount, endCount, exclusiveCount int
	for _, n := range def.Nodes {
		switch n.Type {
		case "start":
			startCount++
		case "end":
			endCount++
		case "exclusive":
			exclusiveCount++
		}
	}
	if startCount != 1 {
		t.Errorf("expected exactly 1 start node, got %d", startCount)
	}
	if endCount < 1 {
		t.Errorf("expected at least 1 end node, got %d", endCount)
	}
	if exclusiveCount < 1 {
		t.Errorf("expected at least 1 exclusive gateway node, got %d", exclusiveCount)
	}

	// Build outgoing edge map to verify exclusive gateway has ≥2 outgoing edges
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
