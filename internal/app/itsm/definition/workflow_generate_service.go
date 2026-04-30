package definition

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	appcore "metis/internal/app"
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
	engineConfigSvc      pathEngineConfigProvider
	actionRepo           *ServiceActionRepo
	serviceDefSvc        *ServiceDefService
	llmClientFactory     workflowLLMClientFactory
	orgResolver          appcore.OrgResolver
	orgStructureResolver appcore.OrgStructureResolver
}

func NewWorkflowGenerateService(i do.Injector) (*WorkflowGenerateService, error) {
	orgResolver, _ := do.InvokeAs[appcore.OrgResolver](i)
	orgStructureResolver, _ := do.InvokeAs[appcore.OrgStructureResolver](i)
	return &WorkflowGenerateService{
		engineConfigSvc:      do.MustInvoke[*EngineConfigService](i),
		actionRepo:           do.MustInvoke[*ServiceActionRepo](i),
		serviceDefSvc:        do.MustInvoke[*ServiceDefService](i),
		llmClientFactory:     llm.NewClient,
		orgResolver:          orgResolver,
		orgStructureResolver: orgStructureResolver,
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

	// 3. Load structured service context for prompt grounding
	promptCtx := workflowPromptContext{}
	if req.ServiceID > 0 {
		if s.actionRepo != nil {
			actions, err := s.actionRepo.ListByService(req.ServiceID)
			if err == nil && len(actions) > 0 {
				promptCtx.ActionsContext = s.buildActionsContext(actions)
			}
		}
		if s.serviceDefSvc != nil {
			if serviceDef, err := s.serviceDefSvc.Get(req.ServiceID); err == nil {
				promptCtx.FormContractContext = s.buildFormContractContext(serviceDef.IntakeFormSchema)
			} else {
				slog.Warn("workflow generate: failed to load service definition for prompt context", "serviceID", req.ServiceID, "error", err)
			}
		}
	}
	// 4. Build prompt and call LLM with retry
	maxRetries := engineCfg.MaxRetries
	temp := float32(engineCfg.Temperature)
	systemPrompt := strings.TrimSpace(engineCfg.SystemPrompt)
	if req.ServiceID > 0 && s.orgStructureResolver != nil {
		orgContext, err := s.collectOrgContextWithTools(ctx, client, engineCfg, systemPrompt, temp, req.CollaborationSpec, promptCtx)
		if err != nil {
			slog.Warn("workflow generate: org preflight failed; continuing without dynamic org context", "serviceID", req.ServiceID, "error", err)
		} else if orgContext != "" {
			promptCtx.OrgContext = orgContext
		}
	}

	var lastWorkflowJSON json.RawMessage
	var lastErrors []engine.ValidationError

	for attempt := 0; attempt <= maxRetries; attempt++ {
		userMsg := s.buildUserMessage(req.CollaborationSpec, promptCtx, lastErrors)

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
			lastErrors = []engine.ValidationError{
				{Message: fmt.Sprintf("参考路径生成引擎调用失败: %v", err)},
			}
			if attempt < maxRetries {
				continue
			}
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
			return nil, fmt.Errorf("%w: 参考路径未生成可用的申请确认表单", ErrWorkflowGeneration)
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
			return nil, fmt.Errorf("%w: 参考路径未生成可用的申请确认表单", ErrWorkflowGeneration)
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
	var health *ServiceHealthCheck
	if blocking := workflowBlockingValidationErrors(validationErrors); len(blocking) > 0 {
		health, err = s.serviceDefSvc.savePublishHealthCheck(req.ServiceID, publishHealthCheckFromValidationErrors(req.ServiceID, blocking))
		if err != nil {
			return nil, err
		}
	} else {
		health, err = s.serviceDefSvc.RefreshPublishHealthCheck(req.ServiceID)
		if err != nil {
			return nil, err
		}
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
		return nil, []engine.ValidationError{{Level: "blocking", Message: fmt.Sprintf("参考路径 JSON 解析失败：%v", err)}}
	}

	for _, node := range workflow.Nodes {
		if node.Type != "form" {
			continue
		}
		var data struct {
			FormSchema json.RawMessage `json:"formSchema"`
		}
		if err := json.Unmarshal(node.Data, &data); err != nil {
			return nil, []engine.ValidationError{{Level: "blocking", NodeID: node.ID, Message: fmt.Sprintf("表单节点无法解析：%v", err)}}
		}
		if len(data.FormSchema) == 0 || string(data.FormSchema) == "null" {
			continue
		}
		return normalizeGeneratedFormSchema(node.ID, data.FormSchema)
	}

	return nil, []engine.ValidationError{{Level: "blocking", Message: "参考路径缺少 requester 表单节点的 formSchema，无法生成申请确认表单"}}
}

func normalizeGeneratedFormSchema(nodeID string, raw json.RawMessage) (json.RawMessage, []engine.ValidationError) {
	var schemaMap map[string]any
	if err := json.Unmarshal(raw, &schemaMap); err != nil {
		return nil, []engine.ValidationError{{Level: "blocking", NodeID: nodeID, Message: fmt.Sprintf("formSchema 不是有效 JSON：%v", err)}}
	}
	if schemaMap["version"] == nil {
		schemaMap["version"] = float64(1)
	}
	fields, ok := schemaMap["fields"].([]any)
	if !ok || len(fields) == 0 {
		return nil, []engine.ValidationError{{Level: "blocking", NodeID: nodeID, Message: "formSchema.fields 不能为空"}}
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
		normalizeGeneratedTableColumns(field)
	}

	normalized, err := json.Marshal(schemaMap)
	if err != nil {
		return nil, []engine.ValidationError{{Level: "blocking", NodeID: nodeID, Message: fmt.Sprintf("formSchema 规范化失败：%v", err)}}
	}
	var schema form.FormSchema
	if err := json.Unmarshal(normalized, &schema); err != nil {
		return nil, []engine.ValidationError{{Level: "blocking", NodeID: nodeID, Message: fmt.Sprintf("formSchema 结构无效：%v", err)}}
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
		return nil, []engine.ValidationError{{Level: "blocking", NodeID: nodeID, Message: fmt.Sprintf("formSchema 序列化失败：%v", err)}}
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

func normalizeGeneratedTableColumns(field map[string]any) {
	if field["type"] != form.FieldTable {
		return
	}
	props, ok := field["props"].(map[string]any)
	if !ok {
		return
	}
	columns, ok := props["columns"].([]any)
	if !ok {
		return
	}
	for _, rawColumn := range columns {
		column, ok := rawColumn.(map[string]any)
		if !ok {
			continue
		}
		if required, ok := column["required"].(bool); !ok || !required {
			column["required"] = true
		}
		if normalized := normalizeGeneratedOptions(column["options"]); normalized != nil {
			column["options"] = normalized
		}
	}
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

func workflowBlockingValidationErrors(errs []engine.ValidationError) []engine.ValidationError {
	blocking := make([]engine.ValidationError, 0, len(errs))
	for _, e := range errs {
		if !e.IsWarning() {
			blocking = append(blocking, e)
		}
	}
	return blocking
}

func publishHealthCheckFromValidationErrors(serviceID uint, errs []engine.ValidationError) *ServiceHealthCheck {
	items := make([]ServiceHealthItem, 0, len(errs))
	for i, validationErr := range errs {
		message := strings.TrimSpace(validationErr.Message)
		if message == "" {
			message = "参考路径存在结构性阻塞项"
		}
		location := workflowValidationHealthLocation(validationErr)
		items = append(items, ServiceHealthItem{
			Key:            "reference_path",
			Label:          "参考路径",
			Status:         "fail",
			Message:        message,
			Location:       location,
			Recommendation: "按提示修正参考路径结构后重新生成或刷新检查。",
			Evidence:       fmt.Sprintf("工作流校验返回 blocking 错误 #%d: %s", i+1, message),
		})
	}
	return &ServiceHealthCheck{ServiceID: serviceID, Status: "fail", Items: items}
}

func workflowValidationHealthLocation(validationErr engine.ValidationError) ServiceHealthLocation {
	if edgeID := strings.TrimSpace(validationErr.EdgeID); edgeID != "" {
		return ServiceHealthLocation{
			Kind:  "workflow_edge",
			Path:  fmt.Sprintf("service.workflowJson.edges[id=%s]", edgeID),
			RefID: edgeID,
		}
	}
	if nodeID := strings.TrimSpace(validationErr.NodeID); nodeID != "" {
		return ServiceHealthLocation{
			Kind:  "workflow_node",
			Path:  fmt.Sprintf("service.workflowJson.nodes[id=%s]", nodeID),
			RefID: nodeID,
		}
	}
	return ServiceHealthLocation{
		Kind: "collaboration_spec",
		Path: "service.collaborationSpec",
	}
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

type workflowPromptContext struct {
	ActionsContext      string
	FormContractContext string
	OrgContext          string
}

func (s *WorkflowGenerateService) buildFormContractContext(raw JSONField) string {
	if len(raw) == 0 {
		return ""
	}
	schema, err := form.ParseSchema(json.RawMessage(raw))
	if err != nil || schema == nil || len(schema.Fields) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("\n\n## 已有申请表单契约\n")
	sb.WriteString("该服务已经配置申请确认表单。生成参考路径时必须复用这些字段 key、类型和选项值；排他网关条件引用表单字段时必须使用 form.<key>。\n\n")
	for _, field := range schema.Fields {
		label := strings.TrimSpace(field.Label)
		if label == "" {
			label = field.Key
		}
		sb.WriteString(fmt.Sprintf("- %s: key=`%s`, type=`%s`", label, field.Key, field.Type))
		if len(field.Options) > 0 {
			sb.WriteString(", options=")
			for i, option := range field.Options {
				if i > 0 {
					sb.WriteString(", ")
				}
				sb.WriteString(fmt.Sprintf("`%v`", option.Value))
				if option.Label != "" {
					sb.WriteString(fmt.Sprintf("（%s）", option.Label))
				}
			}
		}
		if field.Type == form.FieldTable {
			sb.WriteString(formatWorkflowTableColumns(field))
		}
		sb.WriteString("\n")
	}
	return sb.String()
}

func formatWorkflowTableColumns(field form.FormField) string {
	columns, err := form.TableColumns(field)
	if err != nil || len(columns) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString(", columns=")
	for i, column := range columns {
		if i > 0 {
			sb.WriteString(", ")
		}
		label := strings.TrimSpace(column.Label)
		if label == "" {
			label = column.Key
		}
		sb.WriteString(fmt.Sprintf("`%s`（%s/%s", column.Key, label, column.Type))
		if len(column.Options) > 0 {
			sb.WriteString(", options=")
			for j, option := range column.Options {
				if j > 0 {
					sb.WriteString(", ")
				}
				sb.WriteString(fmt.Sprintf("`%v`", option.Value))
				if option.Label != "" {
					sb.WriteString(fmt.Sprintf("（%s）", option.Label))
				}
			}
		}
		sb.WriteString("）")
	}
	return sb.String()
}

func (s *WorkflowGenerateService) buildOrgContext() string {
	if s.orgResolver == nil {
		return ""
	}
	ctx, err := s.orgResolver.QueryContext("", "", "", false)
	if err != nil || ctx == nil || (len(ctx.Departments) == 0 && len(ctx.Positions) == 0) {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("\n\n## 组织架构上下文\n")
	sb.WriteString("生成人工处理节点参与人时，优先按以下组织名称与 code 映射；特定部门中的特定岗位使用 position_department，并填入 department_code 与 position_code。\n\n")
	wroteItem := false
	for _, dept := range ctx.Departments {
		if !dept.IsActive || strings.TrimSpace(dept.Code) == "" {
			continue
		}
		name := strings.TrimSpace(dept.Name)
		if name == "" {
			name = dept.Code
		}
		sb.WriteString(fmt.Sprintf("- 部门：%s（code: `%s`）\n", name, dept.Code))
		wroteItem = true
	}
	for _, pos := range ctx.Positions {
		if !pos.IsActive || strings.TrimSpace(pos.Code) == "" {
			continue
		}
		name := strings.TrimSpace(pos.Name)
		if name == "" {
			name = pos.Code
		}
		sb.WriteString(fmt.Sprintf("- 岗位：%s（code: `%s`）\n", name, pos.Code))
		wroteItem = true
	}
	if !wroteItem {
		return ""
	}
	return sb.String()
}

const workflowOrgPreflightMaxTurns = 3

type workflowOrgSearchToolArgs struct {
	Query string   `json:"query"`
	Kinds []string `json:"kinds"`
	Limit int      `json:"limit"`
}

type workflowOrgResolveToolArgs struct {
	DepartmentHint string `json:"department_hint"`
	PositionHint   string `json:"position_hint"`
	Limit          int    `json:"limit"`
}

type workflowOrgToolResult struct {
	Tool           string
	Query          string
	Kinds          []string
	DepartmentHint string
	PositionHint   string
	Search         *appcore.OrgStructureSearchResult
	Resolve        *appcore.OrgParticipantResolveResult
}

func workflowOrgToolDefs() []llm.ToolDef {
	return []llm.ToolDef{
		{
			Name:        "workflow.org_search_structure",
			Description: "按中文名称或 code 搜索组织结构词表，仅返回部门和岗位，不返回用户。",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"query": map[string]any{"type": "string", "description": "要搜索的部门或岗位名称/code，例如 信息部、网络管理员、security_admin"},
					"kinds": map[string]any{
						"type":        "array",
						"description": "搜索类型，可选 department、position；为空时同时搜索部门和岗位。",
						"items":       map[string]any{"type": "string", "enum": []string{"department", "position"}},
					},
					"limit": map[string]any{"type": "integer", "description": "返回上限，最大 10"},
				},
				"required": []string{"query"},
			},
		},
		{
			Name:        "workflow.org_resolve_participant",
			Description: "把自然语言部门/岗位提示解析为 workflow 参与人配置候选，例如 position_department + department_code + position_code。",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"department_hint": map[string]any{"type": "string", "description": "部门名称或 code，例如 信息部、it"},
					"position_hint":   map[string]any{"type": "string", "description": "岗位名称或 code，例如 网络管理员、network_admin"},
					"limit":           map[string]any{"type": "integer", "description": "返回上限，最大 10"},
				},
			},
		},
	}
}

func (s *WorkflowGenerateService) collectOrgContextWithTools(ctx context.Context, client llm.Client, engineCfg LLMEngineRuntimeConfig, systemPrompt string, temp float32, spec string, promptCtx workflowPromptContext) (string, error) {
	messages := []llm.Message{
		{Role: llm.RoleSystem, Content: buildOrgPreflightSystemPrompt(systemPrompt)},
		{Role: llm.RoleUser, Content: s.buildOrgPreflightUserMessage(spec, promptCtx)},
	}
	tools := workflowOrgToolDefs()
	var collected []workflowOrgToolResult

	for turn := 0; turn < workflowOrgPreflightMaxTurns; turn++ {
		callCtx, cancel := context.WithTimeout(ctx, time.Duration(engineCfg.TimeoutSeconds)*time.Second)
		resp, err := client.Chat(callCtx, llm.ChatRequest{
			Model:       engineCfg.Model,
			Messages:    messages,
			Tools:       tools,
			Temperature: &temp,
			MaxTokens:   engineCfg.MaxTokens,
		})
		cancel()
		if err != nil {
			return "", fmt.Errorf("org preflight llm call: %w", err)
		}
		if resp == nil || len(resp.ToolCalls) == 0 {
			break
		}

		messages = append(messages, llm.Message{
			Role:      llm.RoleAssistant,
			Content:   resp.Content,
			ToolCalls: resp.ToolCalls,
		})
		for _, tc := range resp.ToolCalls {
			content, result := s.executeWorkflowOrgTool(tc)
			if result != nil {
				collected = append(collected, *result)
			}
			messages = append(messages, llm.Message{
				Role:       llm.RoleTool,
				ToolCallID: tc.ID,
				Content:    content,
			})
		}
	}

	return buildWorkflowOrgToolContext(collected), nil
}

func buildOrgPreflightSystemPrompt(systemPrompt string) string {
	var sb strings.Builder
	if strings.TrimSpace(systemPrompt) != "" {
		sb.WriteString(systemPrompt)
		sb.WriteString("\n\n---\n\n")
	}
	sb.WriteString("你是参考路径生成前的组织上下文查询助手。")
	sb.WriteString("只在需要把自然语言部门/岗位映射为 workflow 参与人配置时调用工具；")
	sb.WriteString("不要查询或要求返回具体用户。")
	return sb.String()
}

func (s *WorkflowGenerateService) buildOrgPreflightUserMessage(spec string, promptCtx workflowPromptContext) string {
	var sb strings.Builder
	sb.WriteString("请阅读协作规范和已有结构化契约，判断是否需要查询组织结构映射。\n")
	sb.WriteString("如果需要，请调用工具；如果不需要，直接简短回复无需查询。\n\n")
	sb.WriteString("## 协作规范\n\n")
	sb.WriteString(spec)
	if promptCtx.FormContractContext != "" {
		sb.WriteString(promptCtx.FormContractContext)
	}
	if promptCtx.ActionsContext != "" {
		sb.WriteString(promptCtx.ActionsContext)
	}
	return sb.String()
}

func (s *WorkflowGenerateService) executeWorkflowOrgTool(tc llm.ToolCall) (string, *workflowOrgToolResult) {
	if s.orgStructureResolver == nil {
		return workflowOrgToolError("组织结构解析器不可用"), nil
	}
	switch tc.Name {
	case "workflow.org_search_structure":
		var args workflowOrgSearchToolArgs
		if err := json.Unmarshal([]byte(tc.Arguments), &args); err != nil {
			return workflowOrgToolError(fmt.Sprintf("参数不是有效 JSON: %v", err)), nil
		}
		result, err := s.orgStructureResolver.SearchOrgStructure(args.Query, args.Kinds, args.Limit)
		if err != nil {
			return workflowOrgToolError(err.Error()), nil
		}
		return workflowOrgToolOK(result), &workflowOrgToolResult{
			Tool:   tc.Name,
			Query:  args.Query,
			Kinds:  args.Kinds,
			Search: result,
		}
	case "workflow.org_resolve_participant":
		var args workflowOrgResolveToolArgs
		if err := json.Unmarshal([]byte(tc.Arguments), &args); err != nil {
			return workflowOrgToolError(fmt.Sprintf("参数不是有效 JSON: %v", err)), nil
		}
		result, err := s.orgStructureResolver.ResolveOrgParticipant(args.DepartmentHint, args.PositionHint, args.Limit)
		if err != nil {
			return workflowOrgToolError(err.Error()), nil
		}
		return workflowOrgToolOK(result), &workflowOrgToolResult{
			Tool:           tc.Name,
			DepartmentHint: args.DepartmentHint,
			PositionHint:   args.PositionHint,
			Resolve:        result,
		}
	default:
		return workflowOrgToolError("未知组织上下文工具: " + tc.Name), nil
	}
}

func workflowOrgToolOK(result any) string {
	payload, err := json.Marshal(map[string]any{"ok": true, "result": result})
	if err != nil {
		return workflowOrgToolError(fmt.Sprintf("工具结果序列化失败: %v", err))
	}
	return string(payload)
}

func workflowOrgToolError(message string) string {
	payload, err := json.Marshal(map[string]any{"ok": false, "error": message})
	if err != nil {
		return `{"ok":false,"error":"工具结果序列化失败"}`
	}
	return string(payload)
}

func buildWorkflowOrgToolContext(results []workflowOrgToolResult) string {
	if len(results) == 0 {
		return ""
	}
	var sb strings.Builder
	sb.WriteString("\n\n## 按需查询到的组织上下文\n")
	sb.WriteString("以下组织结构映射来自本次按需工具查询。生成人工处理节点参与人时，特定部门中的特定岗位使用 position_department，并填入 department_code 与 position_code；不要输出具体用户。\n\n")
	wrote := false
	for _, result := range results {
		if result.Search != nil {
			sb.WriteString(formatWorkflowOrgSearchContext(result))
			wrote = true
		}
		if result.Resolve != nil {
			sb.WriteString(formatWorkflowOrgResolveContext(result))
			wrote = true
		}
	}
	if !wrote {
		return ""
	}
	return sb.String()
}

func formatWorkflowOrgSearchContext(result workflowOrgToolResult) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("- 组织搜索：query=`%s`", result.Query))
	if len(result.Kinds) > 0 {
		sb.WriteString(fmt.Sprintf(", kinds=`%s`", strings.Join(result.Kinds, ",")))
	}
	sb.WriteString("\n")
	for _, dept := range result.Search.Departments {
		sb.WriteString(fmt.Sprintf("  - 部门：%s（code: `%s`）\n", dept.Name, dept.Code))
	}
	for _, pos := range result.Search.Positions {
		sb.WriteString(fmt.Sprintf("  - 岗位：%s（code: `%s`）\n", pos.Name, pos.Code))
	}
	return sb.String()
}

func formatWorkflowOrgResolveContext(result workflowOrgToolResult) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("- 参与人解析：department_hint=`%s`, position_hint=`%s`\n", result.DepartmentHint, result.PositionHint))
	for _, candidate := range result.Resolve.Candidates {
		sb.WriteString(fmt.Sprintf("  - 候选：type=`%s`", candidate.Type))
		if candidate.DepartmentCode != "" {
			sb.WriteString(fmt.Sprintf(", department_code=`%s`", candidate.DepartmentCode))
			if candidate.DepartmentName != "" {
				sb.WriteString(fmt.Sprintf("（%s）", candidate.DepartmentName))
			}
		}
		if candidate.PositionCode != "" {
			sb.WriteString(fmt.Sprintf(", position_code=`%s`", candidate.PositionCode))
			if candidate.PositionName != "" {
				sb.WriteString(fmt.Sprintf("（%s）", candidate.PositionName))
			}
		}
		if candidate.CandidateCount > 0 {
			sb.WriteString(fmt.Sprintf(", candidate_count=%d", candidate.CandidateCount))
		}
		sb.WriteString("\n")
	}
	return sb.String()
}

// buildUserMessage constructs the user-facing prompt, optionally injecting previous validation errors.
func (s *WorkflowGenerateService) buildUserMessage(spec string, promptCtx workflowPromptContext, prevErrors []engine.ValidationError) string {
	var sb strings.Builder
	sb.WriteString("请根据以下协作规范生成工作流 JSON。\n\n")
	sb.WriteString("## 协作规范\n\n")
	sb.WriteString(spec)

	if promptCtx.FormContractContext != "" {
		sb.WriteString(promptCtx.FormContractContext)
	}

	if promptCtx.OrgContext != "" {
		sb.WriteString(promptCtx.OrgContext)
	}

	if looksLikeDBBackupWhitelistPromptSpec(spec) {
		sb.WriteString("\n\n## 数据库备份白名单运行时动作约束\n")
		sb.WriteString("该服务的预检和放行动作由智能引擎运行时执行；但为了让用户在流程图上看懂完整业务链路，参考路径 workflow_json 也必须表达这两个动作节点。\n")
		sb.WriteString("如果可用动作列表存在 code=`db_backup_whitelist_precheck` 和 code=`db_backup_whitelist_apply`，必须生成两个 type=\"action\" 节点，并使用对应的数字 action_id：申请表单 -> 备份白名单预检 action -> 数据库管理员处理 -> 执行备份白名单放行 action -> 结束。\n")
		sb.WriteString("数据库管理员处理 rejected 出边直接指向公共结束节点，不经过放行动作节点。\n")
		sb.WriteString("运行时仍由智能引擎优先通过 decision.execute_action 同步执行预检和放行动作，不要因为 workflow_json 中有 action 节点就改变为异步动作活动。\n")
		sb.WriteString("数据库管理员处理节点应使用按需组织上下文中的 position_department 参与人配置。\n")
	}

	if promptCtx.ActionsContext != "" {
		sb.WriteString(promptCtx.ActionsContext)
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
			sb.WriteString("- 如果没有可用动作 id，或动作应由智能体运行时通过工具调用完成且没有可视化要求，不要生成 action 节点；请改用 process/notify/end 表达参考路径。\n")
			sb.WriteString("- 例外：生产数据库备份白名单临时放行需要在流程图上可视化预检和放行动作；如果可用动作列表提供了对应动作 id，应保留两个 action 节点并修正 action_id。\n")
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

func looksLikeDBBackupWhitelistPromptSpec(spec string) bool {
	return strings.Contains(spec, "数据库备份") &&
		strings.Contains(spec, "白名单") &&
		(strings.Contains(spec, "放行") || strings.Contains(spec, "临时"))
}

func (s *WorkflowGenerateService) BuildUserMessage(spec string, actionsCtx string, prevErrors []engine.ValidationError) string {
	return s.buildUserMessage(spec, workflowPromptContext{ActionsContext: actionsCtx}, prevErrors)
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
