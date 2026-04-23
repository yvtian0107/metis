package itsm

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
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
)

// WorkflowGenerateService handles one-shot path engine calls that turn collaboration specs into workflow JSON.
type WorkflowGenerateService struct {
	engineConfigSvc *EngineConfigService
	actionRepo      *ServiceActionRepo
	serviceDefSvc   *ServiceDefService
}

func NewWorkflowGenerateService(i do.Injector) (*WorkflowGenerateService, error) {
	return &WorkflowGenerateService{
		engineConfigSvc: do.MustInvoke[*EngineConfigService](i),
		actionRepo:      do.MustInvoke[*ServiceActionRepo](i),
		serviceDefSvc:   do.MustInvoke[*ServiceDefService](i),
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
	client, err := llm.NewClient(engineCfg.Protocol, engineCfg.BaseURL, engineCfg.APIKey)
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
	systemPrompt := itsmPathBuilderSystemPrompt

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
			Model:       engineCfg.Model,
			Messages:    messages,
			Temperature: &temp,
			MaxTokens:   engineCfg.MaxTokens,
		})
		cancel()

		if err != nil {
			slog.Warn("workflow generate: LLM call failed",
				"attempt", attempt+1, "error", err)
			if attempt < maxRetries {
				continue
			}
			return nil, fmt.Errorf("%w: LLM 调用失败 — %v", ErrWorkflowGeneration, err)
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
		lastWorkflowJSON = workflowJSON

		if len(validationErrors) == 0 {
			return s.buildGenerateResponse(req, workflowJSON, attempt, nil)
		}

		slog.Warn("workflow generate: validation failed",
			"attempt", attempt+1, "errorCount", len(validationErrors))
		lastErrors = validationErrors

		if attempt < maxRetries {
			continue
		}

		// Return last attempt with errors
		return s.buildGenerateResponse(req, lastWorkflowJSON, attempt, validationErrors)
	}

	// Should not reach here
	return nil, ErrWorkflowGeneration
}

func (s *WorkflowGenerateService) buildGenerateResponse(req *GenerateRequest, workflowJSON json.RawMessage, retries int, validationErrors []engine.ValidationError) (*GenerateResponse, error) {
	resp := &GenerateResponse{
		WorkflowJSON: workflowJSON,
		Retries:      retries,
		Errors:       validationErrors,
	}
	if req.ServiceID == 0 || s.serviceDefSvc == nil {
		return resp, nil
	}

	updated, err := s.serviceDefSvc.Update(req.ServiceID, map[string]any{
		"workflow_json":      JSONField(workflowJSON),
		"collaboration_spec": req.CollaborationSpec,
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
	return resp, nil
}

// buildActionsContext formats available service actions for the LLM prompt.
func (s *WorkflowGenerateService) buildActionsContext(actions []ServiceAction) string {
	var sb strings.Builder
	sb.WriteString("\n\n## 可用动作（Action）列表\n")
	sb.WriteString("以下动作可在工作流中作为 action 类型节点使用：\n\n")
	for _, a := range actions {
		sb.WriteString(fmt.Sprintf("- **%s**（code: `%s`）", a.Name, a.Code))
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
	}

	sb.WriteString("\n\n请仅输出合法的 JSON，不要包含任何额外文字或 markdown 代码块标记。")
	return sb.String()
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
