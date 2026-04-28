package definition

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	. "metis/internal/app/itsm/config"
	. "metis/internal/app/itsm/domain"
	"metis/internal/app/itsm/form"
	"strings"
	"time"

	"github.com/samber/do/v2"

	"metis/internal/app/itsm/engine"
	"metis/internal/llm"
)

var (
	ErrPathEngineNotConfigured = errors.New("参考路径生成未配置模型，请前往引擎设置页面设置")
	ErrCollaborationSpecEmpty  = errors.New("协作规范不能为空")
	ErrWorkflowGeneration      = errors.New("协作路径生成失败")
	ErrPathEngineUpstream      = errors.New("参考路径生成引擎上游调用失败")
)

type pathEngineConfigProvider interface {
	PathBuilderRuntimeConfig() (LLMEngineRuntimeConfig, error)
}

type workflowLLMClientFactory func(protocol, baseURL, apiKey string) (llm.Client, error)

// WorkflowGenerateService handles one-shot path engine calls that turn collaboration specs into workflow JSON.
type WorkflowGenerateService struct {
	engineConfigSvc  pathEngineConfigProvider
	actionRepo       *ServiceActionRepo
	serviceDefSvc    *ServiceDefService
	llmClientFactory workflowLLMClientFactory
}

func NewWorkflowGenerateService(i do.Injector) (*WorkflowGenerateService, error) {
	return &WorkflowGenerateService{
		engineConfigSvc:  do.MustInvoke[*EngineConfigService](i),
		actionRepo:       do.MustInvoke[*ServiceActionRepo](i),
		serviceDefSvc:    do.MustInvoke[*ServiceDefService](i),
		llmClientFactory: llm.NewClient,
	}, nil
}

// GenerateRequest is the input for workflow generation.
type GenerateRequest struct {
	ServiceID         uint   `json:"serviceId"`
	CollaborationSpec string `json:"collaborationSpec"`
}

// GenerateResponse is the output of workflow generation.
type GenerateResponse struct {
	WorkflowJSON json.RawMessage            `json:"workflowJson"`
	Retries      int                        `json:"retries"`
	Errors       []engine.ValidationError   `json:"errors,omitempty"`
	Saved        bool                       `json:"saved"`
	Service      *ServiceDefinitionResponse `json:"service,omitempty"`
	HealthCheck  *ServiceHealthCheck        `json:"healthCheck,omitempty"`
}

// Generate turns a collaboration spec into validated workflow JSON via the path engine.
func (s *WorkflowGenerateService) Generate(ctx context.Context, req *GenerateRequest) (*GenerateResponse, error) {
	if strings.TrimSpace(req.CollaborationSpec) == "" {
		return nil, ErrCollaborationSpecEmpty
	}

	if s.engineConfigSvc == nil {
		return nil, ErrPathEngineNotConfigured
	}

	// 1. Load path engine runtime settings
	engineCfg, err := s.engineConfigSvc.PathBuilderRuntimeConfig()
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrPathEngineNotConfigured, err)
	}

	// 2. Create LLM client
	clientFactory := s.llmClientFactory
	if clientFactory == nil {
		clientFactory = llm.NewClient
	}
	client, err := clientFactory(engineCfg.Protocol, engineCfg.BaseURL, engineCfg.APIKey)
	if err != nil {
		return nil, fmt.Errorf("LLM 客户端创建失败: %w", err)
	}

	// 3. Load available actions for context
	var actionsContext string
	if req.ServiceID > 0 {
		actions, err := s.actionRepo.ListByService(req.ServiceID)
		if err == nil && len(actions) > 0 {
			actionsContext = s.buildActionsContext(actions)
		}
	}

	// 4. Build prompt and call LLM with retry
	maxRetries := engineCfg.MaxRetries
	temp := float32(engineCfg.Temperature)
	systemPrompt := strings.TrimSpace(engineCfg.SystemPrompt)

	var lastWorkflowJSON json.RawMessage
	var lastErrors []engine.ValidationError

	for attempt := 0; attempt <= maxRetries; attempt++ {
		userMsg := s.buildUserMessage(req.CollaborationSpec, actionsContext, lastErrors)

		messages := []llm.Message{
			{Role: llm.RoleSystem, Content: systemPrompt},
			{Role: llm.RoleUser, Content: userMsg},
		}

		timeoutSec := engineCfg.TimeoutSeconds
		callCtx, cancel := context.WithTimeout(ctx, time.Duration(timeoutSec)*time.Second)

		resp, err := client.Chat(callCtx, llm.ChatRequest{
			Model:          engineCfg.Model,
			Messages:       messages,
			Temperature:    &temp,
			MaxTokens:      engineCfg.MaxTokens,
			ResponseFormat: &llm.ResponseFormat{Type: "json_object"},
		})
		cancel()

		if err != nil {
			slog.Warn("workflow generate: LLM call failed",
				"attempt", attempt+1, "error", err)
			return nil, fmt.Errorf("%w: 参考路径生成引擎调用超时或不可用，请检查 AI 供应商上游、模型、密钥或额度配置: %v", ErrPathEngineUpstream, err)
		}

		// 6. Extract JSON from response
		workflowJSON, extractErr := extractJSON(resp.Content)
		if extractErr != nil {
			slog.Warn("workflow generate: JSON extraction failed",
				"attempt", attempt+1, "error", extractErr)
			lastErrors = []engine.ValidationError{
				{Message: fmt.Sprintf("输出不是有效 JSON: %v", extractErr)},
			}
			if attempt < maxRetries {
				continue
			}
			return nil, fmt.Errorf("%w: 无法从 LLM 输出中提取有效 JSON", ErrWorkflowGeneration)
		}

		// 7. Validate workflow
		validationErrors := engine.ValidateWorkflow(workflowJSON)
		intakeFormSchema, formErrors := extractGeneratedIntakeFormSchema(workflowJSON)
		validationErrors = append(validationErrors, formErrors...)
		lastWorkflowJSON = workflowJSON

		if len(validationErrors) == 0 || !hasBlockingErrors(validationErrors) {
			// No errors, or only warnings — save and return
			return s.buildGenerateResponse(req, workflowJSON, intakeFormSchema, attempt, validationErrors)
		}

		slog.Warn("workflow generate: validation failed",
			"attempt", attempt+1,
			"maxRetries", maxRetries,
			"retrying", attempt < maxRetries,
			"errorCount", len(validationErrors),
			"validationErrors", workflowValidationErrorsLogValue(validationErrors))
		lastErrors = validationErrors

		if attempt < maxRetries {
			continue
		}

		if len(formErrors) > 0 {
			return nil, fmt.Errorf("%w: å‚è€ƒè·¯å¾„æœªç”Ÿæˆå¯ç”¨çš„ç”³è¯·ç¡®è®¤è¡¨å•", ErrWorkflowGeneration)
		}

		// Return the last parsable draft with validation issues. The draft is
		// still useful as an agentic reference path; publish health carries risk.
		return s.buildGenerateResponse(req, lastWorkflowJSON, intakeFormSchema, attempt, validationErrors)
	}

	// Should not reach here
	return nil, ErrWorkflowGeneration
}

func workflowValidationErrorsLogValue(validationErrors []engine.ValidationError) string {
	const maxLoggedErrors = 5
	if len(validationErrors) == 0 {
		return ""
	}

	limit := len(validationErrors)
	if limit > maxLoggedErrors {
		limit = maxLoggedErrors
	}

	var sb strings.Builder
	for i := 0; i < limit; i++ {
		if i > 0 {
			sb.WriteString("; ")
		}
		validationErr := validationErrors[i]
		level := validationErr.Level
		if level == "" {
			level = "blocking"
		}
		sb.WriteString("[")
		sb.WriteString(level)
		sb.WriteString("]")
		if validationErr.NodeID != "" {
			sb.WriteString(" node=")
			sb.WriteString(validationErr.NodeID)
		}
		if validationErr.EdgeID != "" {
			sb.WriteString(" edge=")
			sb.WriteString(validationErr.EdgeID)
		}
		if validationErr.Message != "" {
			sb.WriteString(" ")
			sb.WriteString(validationErr.Message)
		}
	}
	if len(validationErrors) > limit {
		sb.WriteString(fmt.Sprintf("; ... %d more", len(validationErrors)-limit))
	}
	return sb.String()
}

func (s *WorkflowGenerateService) buildGenerateResponse(req *GenerateRequest, workflowJSON json.RawMessage, intakeFormSchema json.RawMessage, retries int, validationErrors []engine.ValidationError) (*GenerateResponse, error) {
	resp := &GenerateResponse{
		WorkflowJSON: workflowJSON,
		Retries:      retries,
		Errors:       validationErrors,
	}

	if req.ServiceID == 0 || s.serviceDefSvc == nil {
		resp.Saved = true
		return resp, nil
	}

	if len(intakeFormSchema) == 0 {
		var formErrors []engine.ValidationError
		intakeFormSchema, formErrors = extractGeneratedIntakeFormSchema(workflowJSON)
		if len(formErrors) > 0 {
			return nil, fmt.Errorf("%w: å‚è€ƒè·¯å¾„æœªç”Ÿæˆå¯ç”¨çš„ç”³è¯·ç¡®è®¤è¡¨å•", ErrWorkflowGeneration)
		}
	}

	updated, err := s.serviceDefSvc.Update(req.ServiceID, map[string]any{
		"workflow_json":      JSONField(workflowJSON),
		"collaboration_spec": req.CollaborationSpec,
		"intake_form_schema": JSONField(intakeFormSchema),
	})
	if err != nil {
		return nil, err
	}
	health, err := s.serviceDefSvc.RefreshPublishHealthCheck(req.ServiceID)
	if err != nil {
		return nil, err
	}
	updated, err = s.serviceDefSvc.Get(updated.ID)
	if err != nil {
		return nil, err
	}
	serviceResp := updated.ToResponse()
	resp.Service = &serviceResp
	resp.HealthCheck = health
	resp.Saved = true
	return resp, nil
}

func extractGeneratedIntakeFormSchema(workflowJSON json.RawMessage) (json.RawMessage, []engine.ValidationError) {
	var workflow struct {
		Nodes []struct {
			ID   string          `json:"id"`
			Type string          `json:"type"`
			Data json.RawMessage `json:"data"`
		} `json:"nodes"`
	}
	if err := json.Unmarshal(workflowJSON, &workflow); err != nil {
		return nil, []engine.ValidationError{{Level: "blocking", Message: fmt.Sprintf("å‚è€ƒè·¯å¾„ JSON è§£æžå¤±è´¥ï¼š%v", err)}}
	}

	for _, node := range workflow.Nodes {
		if node.Type != "form" {
			continue
		}
		var data struct {
			FormSchema json.RawMessage `json:"formSchema"`
		}
		if err := json.Unmarshal(node.Data, &data); err != nil {
			return nil, []engine.ValidationError{{Level: "blocking", NodeID: node.ID, Message: fmt.Sprintf("è¡¨å•èŠ‚ç‚¹æ— æ³•è§£æžï¼š%v", err)}}
		}
		if len(data.FormSchema) == 0 || string(data.FormSchema) == "null" {
			continue
		}
		return normalizeGeneratedFormSchema(node.ID, data.FormSchema)
	}

	return nil, []engine.ValidationError{{Level: "blocking", Message: "å‚è€ƒè·¯å¾„ç¼ºå°‘ requester è¡¨å•èŠ‚ç‚¹çš„ formSchemaï¼Œæ— æ³•ç”Ÿæˆç”³è¯·ç¡®è®¤è¡¨å•"}}
}

func normalizeGeneratedFormSchema(nodeID string, raw json.RawMessage) (json.RawMessage, []engine.ValidationError) {
	var schemaMap map[string]any
	if err := json.Unmarshal(raw, &schemaMap); err != nil {
		return nil, []engine.ValidationError{{Level: "blocking", NodeID: nodeID, Message: fmt.Sprintf("formSchema ä¸æ˜¯æœ‰æ•ˆ JSONï¼š%v", err)}}
	}
	if schemaMap["version"] == nil {
		schemaMap["version"] = float64(1)
	}
	fields, ok := schemaMap["fields"].([]any)
	if !ok || len(fields) == 0 {
		return nil, []engine.ValidationError{{Level: "blocking", NodeID: nodeID, Message: "formSchema.fields ä¸èƒ½ä¸ºç©º"}}
	}
	for _, rawField := range fields {
		field, ok := rawField.(map[string]any)
		if !ok {
			continue
		}
		if required, ok := field["required"].(bool); !ok || !required {
			field["required"] = true
		}
		if normalized := normalizeGeneratedOptions(field["options"]); normalized != nil {
			field["options"] = normalized
		}
	}

	normalized, err := json.Marshal(schemaMap)
	if err != nil {
		return nil, []engine.ValidationError{{Level: "blocking", NodeID: nodeID, Message: fmt.Sprintf("formSchema è§„èŒƒåŒ–å¤±è´¥ï¼š%v", err)}}
	}
	var schema form.FormSchema
	if err := json.Unmarshal(normalized, &schema); err != nil {
		return nil, []engine.ValidationError{{Level: "blocking", NodeID: nodeID, Message: fmt.Sprintf("formSchema ç»“æž„æ— æ•ˆï¼š%v", err)}}
	}
	if errs := form.ValidateSchema(schema); len(errs) > 0 {
		validationErrors := make([]engine.ValidationError, 0, len(errs))
		for _, err := range errs {
			validationErrors = append(validationErrors, engine.ValidationError{Level: "blocking", NodeID: nodeID, Message: "formSchema " + err.Error()})
		}
		return nil, validationErrors
	}
	canonical, err := json.Marshal(schema)
	if err != nil {
		return nil, []engine.ValidationError{{Level: "blocking", NodeID: nodeID, Message: fmt.Sprintf("formSchema åºåˆ—åŒ–å¤±è´¥ï¼š%v", err)}}
	}
	return canonical, nil
}

func normalizeGeneratedOptions(raw any) any {
	options, ok := raw.([]any)
	if !ok || len(options) == 0 {
		return raw
	}
	normalized := make([]any, 0, len(options))
	for _, option := range options {
		switch value := option.(type) {
		case string:
			normalized = append(normalized, map[string]any{"label": value, "value": value})
		case map[string]any:
			if value["value"] == nil && value["label"] != nil {
				value["value"] = value["label"]
			}
			if value["label"] == nil && value["value"] != nil {
				value["label"] = fmt.Sprintf("%v", value["value"])
			}
			normalized = append(normalized, value)
		default:
			label := fmt.Sprintf("%v", value)
			normalized = append(normalized, map[string]any{"label": label, "value": label})
		}
	}
	return normalized
}

// hasBlockingErrors returns true if any validation error has Level "blocking".
func hasBlockingErrors(errs []engine.ValidationError) bool {
	for _, e := range errs {
		if !e.IsWarning() {
			return true
		}
	}
	return false
}

// buildActionsContext formats available service actions for the LLM prompt.
func (s *WorkflowGenerateService) buildActionsContext(actions []ServiceAction) string {
	var sb strings.Builder
	sb.WriteString("\n\n## 可用动作（Action）列表\n")
	sb.WriteString("以下动作可在工作流中作为 action 类型节点使用：\n\n")
	for _, a := range actions {
		sb.WriteString(fmt.Sprintf("- **%s**（id: `%d`, code: `%s`）", a.Name, a.ID, a.Code))
		if a.Description != "" {
			sb.WriteString(fmt.Sprintf("：%s", a.Description))
		}
		sb.WriteString("\n")
	}
	return sb.String()
}

// buildUserMessage constructs the user-facing prompt, optionally injecting previous validation errors.
func (s *WorkflowGenerateService) buildUserMessage(spec string, actionsCtx string, prevErrors []engine.ValidationError) string {
	var sb strings.Builder
	sb.WriteString("请根据以下协作规范生成工作流 JSON。\n\n")
	sb.WriteString("## 协作规范\n\n")
	sb.WriteString(spec)

	if actionsCtx != "" {
		sb.WriteString(actionsCtx)
	}

	if len(prevErrors) > 0 {
		sb.WriteString("\n\n## 上一次生成的工作流存在以下问题，请修正：\n\n")
		for _, e := range prevErrors {
			prefix := ""
			if e.NodeID != "" {
				prefix = fmt.Sprintf("[节点 %s] ", e.NodeID)
			} else if e.EdgeID != "" {
				prefix = fmt.Sprintf("[边 %s] ", e.EdgeID)
			}
			sb.WriteString(fmt.Sprintf("- %s%s\n", prefix, e.Message))
		}
		if validationErrorsRequireParticipantRepair(prevErrors) {
			sb.WriteString("\n## 参与人修正要求\n\n")
			sb.WriteString("- 每个 form/process 等人工节点都必须在 data.participants 中配置非空处理人数组。\n")
			sb.WriteString("- 不要使用 data.participantType、data.positionCode、data.departmentCode；必须使用 participants 数组内的 type、position_code、department_code。\n")
			sb.WriteString("- 如果缺处理人的节点是 form 表单填写节点，且表示申请人补充/填写资料，应生成：")
			sb.WriteString(`"participants":[{"type":"requester"}]`)
			sb.WriteString("。\n")
			sb.WriteString("- 如果协作规范写明 position_department、部门编码 it、岗位编码 network_admin，应生成：")
			sb.WriteString(`"participants":[{"type":"position_department","department_code":"it","position_code":"network_admin"}]`)
			sb.WriteString("。\n")
			sb.WriteString("- 如果不同网关分支进入不同岗位处理节点，每个 process 节点必须分别配置对应岗位的 participants。\n")
		}
		if validationErrorsRequireActionRepair(prevErrors) {
			sb.WriteString("\n## 动作节点修正要求\n\n")
			sb.WriteString("- 只有协作规范明确要求在参考路径 workflow_json 里编排系统动作，且“可用动作列表”给出了动作 id 时，才允许生成 type=\"action\" 节点。\n")
			sb.WriteString("- 如果没有可用动作 id，或动作应由智能体运行时通过工具调用完成，不要生成 action 节点；请改用 process/notify/end 表达参考路径。\n")
			sb.WriteString("- 保留人工处理语义：人工处理、并行处理、部门岗位处理必须使用 type=\"process\"，不能写成 action。\n")
		}
		if validationErrorsRequireRejectedEdgeRepair(prevErrors) {
			sb.WriteString("\n## 人工节点出边修正要求\n\n")
			sb.WriteString("- 每个 type=\"process\" 节点都必须有且仅有两条决策出边：一条 data.outcome=\"approved\"，一条 data.outcome=\"rejected\"。\n")
			sb.WriteString("- rejected 出边不能省略；如果 approved 和 rejected 都表示流程结束，可以共同指向同一个 type=\"end\" 节点。\n")
			sb.WriteString("- 如果协作规范没有明确写驳回后补充、返工或恢复路径，rejected 出边应指向公共结束节点，驳回语义由 edge.data.outcome=\"rejected\" 表达。\n")
			sb.WriteString("- 不要凭空生成“退回申请人补充”或 form 返工节点；只有协作规范明确写了补充/返工路径时才允许这样生成。\n")
			sb.WriteString("- 多个 process 节点如果通过或驳回后都是结束，应复用同一个 end 节点，不要拆成“驳回结束”和“完成”。\n")
		}
		if validationErrorsRequireGatewayRepair(prevErrors) {
			sb.WriteString("\n## 网关修正要求\n\n")
			sb.WriteString("- exclusive 只表示条件分支，必须至少两条出边；不要用 exclusive 表示并行汇聚或单出边汇聚。\n")
			sb.WriteString("- 并行拆分必须使用 type=\"parallel\" 且 data.gateway_direction=\"fork\"；并行汇聚必须使用 type=\"parallel\" 且 data.gateway_direction=\"join\"。\n")
			sb.WriteString("- parallel fork 至少两条出边；parallel join 至少两条入边且有且仅有一条出边。\n")
		}
	}

	sb.WriteString("\n\n请仅输出合法的 JSON，不要包含任何额外文字或 markdown 代码块标记。")
	return sb.String()
}

func (s *WorkflowGenerateService) BuildUserMessage(spec string, actionsCtx string, prevErrors []engine.ValidationError) string {
	return s.buildUserMessage(spec, actionsCtx, prevErrors)
}

func validationErrorsRequireParticipantRepair(validationErrors []engine.ValidationError) bool {
	for _, validationErr := range validationErrors {
		msg := validationErr.Message
		if strings.Contains(msg, "处理人") ||
			strings.Contains(msg, "参与人") ||
			strings.Contains(msg, "participants") ||
			strings.Contains(msg, "position_code") ||
			strings.Contains(msg, "department_code") {
			return true
		}
	}
	return false
}

func validationErrorsRequireActionRepair(validationErrors []engine.ValidationError) bool {
	for _, validationErr := range validationErrors {
		msg := validationErr.Message
		if strings.Contains(msg, "action_id") ||
			strings.Contains(msg, "动作节点") {
			return true
		}
	}
	return false
}

func validationErrorsRequireRejectedEdgeRepair(validationErrors []engine.ValidationError) bool {
	for _, validationErr := range validationErrors {
		msg := validationErr.Message
		if strings.Contains(msg, `outcome="approved"`) ||
			strings.Contains(msg, `outcome="rejected"`) ||
			strings.Contains(msg, "驳回路径") ||
			strings.Contains(msg, "approved 和 rejected") {
			return true
		}
	}
	return false
}

func validationErrorsRequireGatewayRepair(validationErrors []engine.ValidationError) bool {
	for _, validationErr := range validationErrors {
		msg := validationErr.Message
		if strings.Contains(msg, "排他网关") ||
			strings.Contains(msg, "并行网关") ||
			strings.Contains(msg, "包含网关") ||
			strings.Contains(msg, "gateway_direction") ||
			strings.Contains(msg, "fork") ||
			strings.Contains(msg, "join") {
			return true
		}
	}
	return false
}

// extractJSON attempts to extract a JSON object from the LLM response content.
// It handles cases where the LLM wraps JSON in markdown code blocks.
func extractJSON(content string) (json.RawMessage, error) {
	content = strings.TrimSpace(content)

	// Try direct parse first
	if json.Valid([]byte(content)) {
		return json.RawMessage(content), nil
	}

	// Try extracting from markdown code block: ```json ... ``` or ``` ... ```
	if idx := strings.Index(content, "```"); idx >= 0 {
		start := idx + 3
		// Skip optional language tag (e.g., "json")
		if nl := strings.Index(content[start:], "\n"); nl >= 0 {
			start += nl + 1
		}
		if end := strings.Index(content[start:], "```"); end >= 0 {
			candidate := strings.TrimSpace(content[start : start+end])
			if json.Valid([]byte(candidate)) {
				return json.RawMessage(candidate), nil
			}
		}
	}

	// Try finding first { to last }
	first := strings.Index(content, "{")
	last := strings.LastIndex(content, "}")
	if first >= 0 && last > first {
		candidate := content[first : last+1]
		if json.Valid([]byte(candidate)) {
			return json.RawMessage(candidate), nil
		}
	}

	return nil, fmt.Errorf("无法从输出中提取有效 JSON")
}

func ExtractJSON(content string) (json.RawMessage, error) {
	return extractJSON(content)
}
