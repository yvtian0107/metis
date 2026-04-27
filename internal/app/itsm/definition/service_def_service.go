package definition

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	. "metis/internal/app/itsm/catalog"
	. "metis/internal/app/itsm/config"
	. "metis/internal/app/itsm/domain"
	"strings"
	"time"

	"github.com/samber/do/v2"
	"gorm.io/gorm"
	ai "metis/internal/app/ai/runtime"
	"metis/internal/database"

	"metis/internal/app/itsm/engine"
	"metis/internal/llm"
)

var (
	ErrServiceDefNotFound    = errors.New("service definition not found")
	ErrServiceCodeExists     = errors.New("service code already exists")
	ErrWorkflowValidation    = errors.New("workflow validation failed")
	ErrServiceEngineMismatch = errors.New("service engine field mismatch")
	ErrAgentNotAvailable     = errors.New("agent not available")
)

type publishHealthConfigProvider interface {
	HealthCheckRuntimeConfig() (LLMEngineRuntimeConfig, error)
	DecisionMode() string
	DecisionAgentID() uint
	FallbackAssigneeID() uint
	AuditLevel() string
}

type ServiceDefService struct {
	repo             *ServiceDefRepo
	db               *database.DB
	catalogs         *CatalogRepo
	engineConfigSvc  publishHealthConfigProvider
	llmClientFactory workflowLLMClientFactory
}

func NewServiceDefService(i do.Injector) (*ServiceDefService, error) {
	repo := do.MustInvoke[*ServiceDefRepo](i)
	db := do.MustInvoke[*database.DB](i)
	catalogs := do.MustInvoke[*CatalogRepo](i)
	engineConfigSvc := do.MustInvoke[*EngineConfigService](i)
	return &ServiceDefService{
		repo:             repo,
		db:               db,
		catalogs:         catalogs,
		engineConfigSvc:  engineConfigSvc,
		llmClientFactory: llm.NewClient,
	}, nil
}

func (s *ServiceDefService) Create(svc *ServiceDefinition) (*ServiceDefinition, error) {
	if _, err := s.repo.FindByCode(svc.Code); err == nil {
		return nil, ErrServiceCodeExists
	}
	if err := s.validateCatalogID(svc.CatalogID); err != nil {
		return nil, err
	}
	if err := s.validateEngineFields(svc.EngineType, svc.WorkflowJSON, svc.CollaborationSpec, svc.AgentID); err != nil {
		return nil, err
	}
	if err := s.validateAgent(svc.AgentID); err != nil {
		return nil, err
	}
	// Validate workflow_json for classic engine
	if svc.EngineType == "classic" && len(svc.WorkflowJSON) > 0 {
		if err := validateWorkflowJSON(json.RawMessage(svc.WorkflowJSON)); err != nil {
			return nil, err
		}
	}
	svc.IsActive = true
	if err := s.repo.Create(svc); err != nil {
		if IsSQLiteUniqueError(err) {
			return nil, ErrServiceCodeExists
		}
		return nil, err
	}
	return s.repo.FindByID(svc.ID)
}

func (s *ServiceDefService) Get(id uint) (*ServiceDefinition, error) {
	svc, err := s.repo.FindByID(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrServiceDefNotFound
		}
		return nil, err
	}
	return svc, nil
}

func (s *ServiceDefService) Update(id uint, updates map[string]any) (*ServiceDefinition, error) {
	existing, err := s.repo.FindByID(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrServiceDefNotFound
		}
		return nil, err
	}
	if code, ok := updates["code"].(string); ok && code != existing.Code {
		if _, err := s.repo.FindByCode(code); err == nil {
			return nil, ErrServiceCodeExists
		}
	}
	if catalogID, ok := updates["catalog_id"].(uint); ok {
		if err := s.validateCatalogID(catalogID); err != nil {
			return nil, err
		}
	}
	engineType := existing.EngineType
	if et, ok := updates["engine_type"].(string); ok {
		engineType = et
	}
	workflowJSON := existing.WorkflowJSON
	if v, ok := updates["workflow_json"].(JSONField); ok {
		workflowJSON = v
	}
	collaborationSpec := existing.CollaborationSpec
	if v, ok := updates["collaboration_spec"].(string); ok {
		collaborationSpec = v
	}
	agentID := existing.AgentID
	if v, ok := updates["agent_id"].(uint); ok {
		agentID = &v
	}
	if err := s.validateEngineFields(engineType, workflowJSON, collaborationSpec, agentID); err != nil {
		return nil, err
	}
	if err := s.validateAgent(agentID); err != nil {
		return nil, err
	}
	// Validate workflow_json if being updated for classic engine
	if wfJSON, ok := updates["workflow_json"]; ok {
		if engineType == "classic" {
			if raw, ok := wfJSON.(JSONField); ok && len(raw) > 0 {
				if err := validateWorkflowJSON(json.RawMessage(raw)); err != nil {
					return nil, err
				}
			}
		}
	}

	if err := s.repo.Update(id, updates); err != nil {
		if IsSQLiteUniqueError(err) {
			return nil, ErrServiceCodeExists
		}
		return nil, err
	}
	return s.repo.FindByID(id)
}

func (s *ServiceDefService) Delete(id uint) error {
	if _, err := s.repo.FindByID(id); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrServiceDefNotFound
		}
		return err
	}
	return s.repo.Delete(id)
}

func (s *ServiceDefService) List(params ServiceDefListParams) ([]ServiceDefinition, int64, error) {
	return s.repo.List(params)
}

func (s *ServiceDefService) HealthCheck(id uint) (*ServiceHealthCheck, error) {
	return s.RefreshPublishHealthCheck(id)
}

func (s *ServiceDefService) RefreshPublishHealthCheck(id uint) (*ServiceHealthCheck, error) {
	svc, err := s.Get(id)
	if err != nil {
		return nil, err
	}
	check, evalErr := s.computePublishHealthCheckWithAI(context.Background(), svc)
	if evalErr != nil {
		check = newPublishHealthEngineFailureCheck(svc.ID, evalErr.Error())
	}
	items, err := json.Marshal(check.Items)
	if err != nil {
		return nil, err
	}
	now := time.Now()
	if err := s.repo.Update(id, map[string]any{
		"publish_health_status":     check.Status,
		"publish_health_items":      JSONField(items),
		"publish_health_checked_at": now,
	}); err != nil {
		return nil, err
	}
	check.CheckedAt = &now
	return check, nil
}

func (s *ServiceDefService) RefreshPublishHealthCheckIfPresent(id uint) error {
	svc, err := s.Get(id)
	if err != nil {
		return err
	}
	if svc.PublishHealthCheckedAt == nil {
		return nil
	}
	_, err = s.RefreshPublishHealthCheck(id)
	return err
}

func (s *ServiceDefService) computePublishHealthCheckWithAI(ctx context.Context, svc *ServiceDefinition) (*ServiceHealthCheck, error) {
	if s.engineConfigSvc == nil {
		return nil, fmt.Errorf("发布健康检查引擎未初始化")
	}
	engineCfg, err := s.engineConfigSvc.HealthCheckRuntimeConfig()
	if err != nil {
		return nil, err
	}
	clientFactory := s.llmClientFactory
	if clientFactory == nil {
		clientFactory = llm.NewClient
	}
	client, err := clientFactory(engineCfg.Protocol, engineCfg.BaseURL, engineCfg.APIKey)
	if err != nil {
		return nil, fmt.Errorf("发布健康检查客户端创建失败: %w", err)
	}

	payload, err := s.buildPublishHealthPayload(svc)
	if err != nil {
		return nil, err
	}
	userPayload, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("发布健康检查上下文序列化失败: %w", err)
	}

	temp := float32(engineCfg.Temperature)
	timeoutSec := engineCfg.TimeoutSeconds
	if timeoutSec <= 0 {
		timeoutSec = 45
	}
	callCtx, cancel := context.WithTimeout(ctx, time.Duration(timeoutSec)*time.Second)
	defer cancel()

	resp, err := client.Chat(callCtx, llm.ChatRequest{
		Model: engineCfg.Model,
		Messages: []llm.Message{
			{Role: llm.RoleSystem, Content: engineCfg.SystemPrompt},
			{Role: llm.RoleUser, Content: string(userPayload)},
		},
		Temperature:    &temp,
		MaxTokens:      engineCfg.MaxTokens,
		ResponseFormat: &llm.ResponseFormat{Type: "json_object"},
	})
	if err != nil {
		return nil, fmt.Errorf("发布健康检查引擎调用失败: %w", err)
	}

	raw, err := extractJSON(resp.Content)
	if err != nil {
		return nil, fmt.Errorf("发布健康检查输出解析失败: %w", err)
	}
	var parsed struct {
		Status string              `json:"status"`
		Items  []ServiceHealthItem `json:"items"`
	}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil, fmt.Errorf("发布健康检查输出格式无效: %w", err)
	}
	return normalizePublishHealthCheck(svc.ID, parsed.Status, parsed.Items), nil
}

func (s *ServiceDefService) buildPublishHealthPayload(svc *ServiceDefinition) (map[string]any, error) {
	actions := make([]map[string]any, 0)
	var actionRows []ServiceAction
	if err := s.db.DB.Where("service_id = ? AND deleted_at IS NULL", svc.ID).Order("id asc").Find(&actionRows).Error; err != nil {
		return nil, fmt.Errorf("读取服务动作失败: %w", err)
	}
	for _, action := range actionRows {
		actions = append(actions, map[string]any{
			"id":          action.ID,
			"code":        action.Code,
			"name":        action.Name,
			"description": action.Description,
			"prompt":      action.Prompt,
			"actionType":  action.ActionType,
			"isActive":    action.IsActive,
		})
	}

	var workflow any
	if len(svc.WorkflowJSON) > 0 {
		if err := json.Unmarshal([]byte(svc.WorkflowJSON), &workflow); err != nil {
			workflow = map[string]any{"raw": string(svc.WorkflowJSON)}
		}
	} else {
		workflow = map[string]any{}
	}

	return map[string]any{
		"service": map[string]any{
			"id":                svc.ID,
			"name":              svc.Name,
			"code":              svc.Code,
			"engineType":        svc.EngineType,
			"description":       svc.Description,
			"collaborationSpec": svc.CollaborationSpec,
			"workflowJson":      workflow,
			"serviceAgentId":    svc.AgentID,
		},
		"runtime": map[string]any{
			"decisionMode":     s.engineConfigSvc.DecisionMode(),
			"decisionAgentId":  s.engineConfigSvc.DecisionAgentID(),
			"fallbackAssignee": s.engineConfigSvc.FallbackAssigneeID(),
			"auditLevel":       s.engineConfigSvc.AuditLevel(),
		},
		"actions": actions,
	}, nil
}

func normalizePublishHealthCheck(serviceID uint, status string, items []ServiceHealthItem) *ServiceHealthCheck {
	normalizedItems := make([]ServiceHealthItem, 0, len(items))
	maxLevel := healthLevel(normalizePublishHealthStatus(status))
	for idx, item := range items {
		itemStatus := normalizePublishHealthStatus(item.Status)
		if itemStatus == "" {
			itemStatus = "warn"
		}
		if itemLevel := healthLevel(itemStatus); itemLevel > maxLevel {
			maxLevel = itemLevel
		}
		key := strings.TrimSpace(item.Key)
		if key == "" {
			key = fmt.Sprintf("health_item_%d", idx+1)
		}
		label := strings.TrimSpace(item.Label)
		if label == "" {
			label = fmt.Sprintf("检查项%d", idx+1)
		}
		message := strings.TrimSpace(item.Message)
		if message == "" {
			message = "请检查该项配置。"
		}
		normalizedItems = append(normalizedItems, ServiceHealthItem{
			Key:     key,
			Label:   label,
			Status:  itemStatus,
			Message: message,
		})
	}

	finalStatus := levelStatus(maxLevel)
	if finalStatus != "pass" && len(normalizedItems) == 0 {
		normalizedItems = []ServiceHealthItem{{
			Key:     "health_summary",
			Label:   "发布健康检查",
			Status:  finalStatus,
			Message: "发布健康检查返回了风险状态，但未提供详细项。",
		}}
	}

	return &ServiceHealthCheck{
		ServiceID: serviceID,
		Status:    finalStatus,
		Items:     normalizedItems,
	}
}

func newPublishHealthEngineFailureCheck(serviceID uint, message string) *ServiceHealthCheck {
	msg := strings.TrimSpace(message)
	if msg == "" {
		msg = "发布健康检查引擎不可用"
	}
	return &ServiceHealthCheck{
		ServiceID: serviceID,
		Status:    "fail",
		Items: []ServiceHealthItem{{
			Key:     "health_engine",
			Label:   "发布健康检查引擎",
			Status:  "fail",
			Message: msg,
		}},
	}
}

func normalizePublishHealthStatus(status string) string {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "pass":
		return "pass"
	case "warn":
		return "warn"
	case "fail":
		return "fail"
	default:
		return "fail"
	}
}

func healthLevel(status string) int {
	switch status {
	case "pass":
		return 0
	case "warn":
		return 1
	default:
		return 2
	}
}

func levelStatus(level int) string {
	switch {
	case level <= 0:
		return "pass"
	case level == 1:
		return "warn"
	default:
		return "fail"
	}
}

// validateWorkflowJSON runs the engine validator and wraps errors.
func validateWorkflowJSON(raw json.RawMessage) error {
	errs := engine.ValidateWorkflow(raw)
	for _, err := range errs {
		if !err.IsWarning() {
			return fmt.Errorf("%w: %s", ErrWorkflowValidation, err.Message)
		}
	}
	return nil
}

func (s *ServiceDefService) validateCatalogID(catalogID uint) error {
	if _, err := s.catalogs.FindByID(catalogID); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrCatalogNotFound
		}
		return err
	}
	return nil
}

func (s *ServiceDefService) validateEngineFields(engineType string, workflowJSON JSONField, collaborationSpec string, agentID *uint) error {
	switch engineType {
	case "classic":
		if agentID != nil && *agentID != 0 {
			return ErrServiceEngineMismatch
		}
	}
	return nil
}

func (s *ServiceDefService) validateAgent(agentID *uint) error {
	if agentID == nil || *agentID == 0 {
		return nil
	}
	var agent ai.Agent
	if err := s.db.First(&agent, *agentID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrAgentNotAvailable
		}
		return err
	}
	if !agent.IsActive {
		return ErrAgentNotAvailable
	}
	return nil
}
