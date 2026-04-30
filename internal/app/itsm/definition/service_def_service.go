package definition

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	appcore "metis/internal/app"
	. "metis/internal/app/itsm/catalog"
	. "metis/internal/app/itsm/config"
	. "metis/internal/app/itsm/domain"
	"strconv"
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
	ErrServiceDefNotFound     = errors.New("service definition not found")
	ErrServiceCodeExists      = errors.New("service code already exists")
	ErrWorkflowValidation     = errors.New("workflow validation failed")
	ErrServiceEngineMismatch  = errors.New("service engine field mismatch")
	ErrAgentNotAvailable      = errors.New("agent not available")
	ErrSLATemplateUnavailable = errors.New("SLA template not available")
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
	resolver         *engine.ParticipantResolver
}

type publishHealthValidationContext struct {
	workflowNodeIDs   map[string]struct{}
	workflowEdgeIDs   map[string]struct{}
	actionRefs        map[string]struct{}
	hasActionEvidence bool
}

func NewServiceDefService(i do.Injector) (*ServiceDefService, error) {
	repo := do.MustInvoke[*ServiceDefRepo](i)
	db := do.MustInvoke[*database.DB](i)
	catalogs := do.MustInvoke[*CatalogRepo](i)
	engineConfigSvc := do.MustInvoke[*EngineConfigService](i)
	orgResolver, _ := do.InvokeAs[appcore.OrgResolver](i)
	return &ServiceDefService{
		repo:             repo,
		db:               db,
		catalogs:         catalogs,
		engineConfigSvc:  engineConfigSvc,
		llmClientFactory: llm.NewClient,
		resolver:         engine.NewParticipantResolver(orgResolver),
	}, nil
}

func (s *ServiceDefService) Create(svc *ServiceDefinition) (*ServiceDefinition, error) {
	if _, err := s.repo.FindByCode(svc.Code); err == nil {
		return nil, ErrServiceCodeExists
	}
	if err := s.validateCatalogID(svc.CatalogID); err != nil {
		return nil, err
	}
	if err := s.validateSLAID(svc.SLAID); err != nil {
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
	if rawSLAID, ok := updates["sla_id"]; ok {
		slaID, err := parseOptionalSLAID(rawSLAID)
		if err != nil {
			return nil, err
		}
		if err := s.validateSLAID(slaID); err != nil {
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
		slog.Warn("publish health check skipped llm diagnostics", "service_id", svc.ID, "error", evalErr)
		if check == nil {
			check = s.computePublishHealthCheckBase(svc)
		}
	}
	return s.savePublishHealthCheck(id, check)
}

func (s *ServiceDefService) savePublishHealthCheck(id uint, check *ServiceHealthCheck) (*ServiceHealthCheck, error) {
	if check == nil {
		check = &ServiceHealthCheck{ServiceID: id, Status: "pass", Items: []ServiceHealthItem{}}
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
	baseCheck := s.computePublishHealthCheckBase(svc)
	if svc.EngineType == "classic" || baseCheck.Status != "pass" {
		return baseCheck, nil
	}
	if s.engineConfigSvc == nil {
		return baseCheck, fmt.Errorf("发布健康检查引擎未初始化")
	}

	engineCfg, err := s.engineConfigSvc.HealthCheckRuntimeConfig()
	if err != nil {
		return baseCheck, err
	}
	clientFactory := s.llmClientFactory
	if clientFactory == nil {
		clientFactory = llm.NewClient
	}
	client, err := clientFactory(engineCfg.Protocol, engineCfg.BaseURL, engineCfg.APIKey)
	if err != nil {
		return baseCheck, fmt.Errorf("发布健康检查客户端创建失败: %w", err)
	}

	payload, validationCtx, err := s.buildPublishHealthPayload(svc)
	if err != nil {
		return baseCheck, err
	}
	userPayload, err := json.Marshal(payload)
	if err != nil {
		return baseCheck, fmt.Errorf("发布健康检查上下文序列化失败: %w", err)
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
		return baseCheck, fmt.Errorf("发布健康检查引擎调用失败: %w", err)
	}

	raw, err := extractJSON(resp.Content)
	if err != nil {
		return baseCheck, fmt.Errorf("发布健康检查输出解析失败: %w", err)
	}
	var parsed struct {
		Status string              `json:"status"`
		Items  []ServiceHealthItem `json:"items"`
	}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return baseCheck, fmt.Errorf("发布健康检查输出格式无效: %w", err)
	}
	normalized := normalizePublishHealthCheck(svc.ID, parsed.Status, parsed.Items, validationCtx)
	return mergePublishHealthChecks(baseCheck, normalized), nil
}

func (s *ServiceDefService) computePublishHealthCheckBase(svc *ServiceDefinition) *ServiceHealthCheck {
	check := &ServiceHealthCheck{ServiceID: svc.ID, Status: "pass", Items: []ServiceHealthItem{}}
	add := func(item *ServiceHealthItem) {
		if item == nil {
			return
		}
		check.Items = append(check.Items, *item)
		if healthLevel(item.Status) > healthLevel(check.Status) {
			check.Status = normalizePublishHealthStatus(item.Status)
		}
	}

	if svc.EngineType == "classic" {
		if len(svc.WorkflowJSON) == 0 {
			add(&ServiceHealthItem{Key: "workflow", Label: "流程定义", Status: "fail", Message: "经典引擎必须配置可执行工作流"})
		} else if err := validateWorkflowJSON(json.RawMessage(svc.WorkflowJSON)); err != nil {
			add(&ServiceHealthItem{Key: "workflow", Label: "流程定义", Status: "fail", Message: err.Error()})
		}
		return check
	}

	if strings.TrimSpace(svc.CollaborationSpec) == "" {
		add(&ServiceHealthItem{Key: "collaboration_spec", Label: "协作规范", Status: "fail", Message: "协作规范为空，智能引擎缺少决策策略"})
	}
	if !hasUsableIntakeFormSchema(svc.IntakeFormSchema) {
		add(&ServiceHealthItem{Key: "intake_form", Label: "申请表单", Status: "warn", Message: "尚未生成申请确认表单，请先生成参考路径后再在服务台使用"})
	}
	decisionAgentID := strings.TrimSpace(s.systemConfigValue(SmartTicketDecisionAgentKey))
	switch {
	case decisionAgentID == "", decisionAgentID == "0":
		add(&ServiceHealthItem{Key: "decision_agent", Label: "流程决策岗", Status: "fail", Message: "流程决策岗未上岗"})
	default:
		id, err := strconv.ParseUint(decisionAgentID, 10, 64)
		if err != nil {
			add(&ServiceHealthItem{Key: "decision_agent", Label: "流程决策岗", Status: "fail", Message: "流程决策岗配置值不是有效智能体 ID"})
		} else if agentID := uint(id); agentID == 0 || s.validateAgent(&agentID) != nil {
			add(&ServiceHealthItem{Key: "decision_agent", Label: "流程决策岗", Status: "fail", Message: "流程决策岗上岗智能体不存在或未启用"})
		}
	}

	decisionMode := normalizedHealthDecisionMode(s.systemConfigValue(SmartTicketDecisionModeKey))
	add(s.checkPathEngineRisk())
	add(s.checkReferencePathRisk(svc, decisionMode))
	add(s.checkFallbackRisk())
	return check
}

func mergePublishHealthChecks(base, extra *ServiceHealthCheck) *ServiceHealthCheck {
	if base == nil {
		return extra
	}
	if extra == nil {
		return base
	}
	items := make([]ServiceHealthItem, 0, len(base.Items)+len(extra.Items))
	items = append(items, base.Items...)
	items = append(items, extra.Items...)
	status := base.Status
	if healthLevel(extra.Status) > healthLevel(status) {
		status = extra.Status
	}
	return &ServiceHealthCheck{ServiceID: base.ServiceID, Status: normalizePublishHealthStatus(status), Items: items}
}

func hasUsableIntakeFormSchema(raw JSONField) bool {
	if len(raw) == 0 {
		return false
	}
	var schema struct {
		Fields []json.RawMessage `json:"fields"`
	}
	if err := json.Unmarshal([]byte(raw), &schema); err != nil {
		return false
	}
	return len(schema.Fields) > 0
}

func (s *ServiceDefService) checkPathEngineRisk() *ServiceHealthItem {
	modelID := strings.TrimSpace(s.systemConfigValue(SmartTicketPathModelKey))
	if modelID == "" || modelID == "0" {
		return &ServiceHealthItem{Key: "path_engine", Label: "参考路径生成", Status: "fail", Message: "参考路径生成未配置模型，无法生成协作路径"}
	}
	id, err := strconv.ParseUint(modelID, 10, 64)
	if err != nil || id == 0 || s.validateEngineModel(uint(id)) != nil {
		return &ServiceHealthItem{Key: "path_engine", Label: "参考路径生成", Status: "fail", Message: "参考路径生成模型不存在或未启用，无法生成协作路径"}
	}
	return nil
}

func (s *ServiceDefService) validateEngineModel(modelID uint) error {
	var modelRow struct {
		ID             uint
		Status         string
		ProviderStatus string
	}
	if err := s.db.Table("ai_models").
		Select("ai_models.id, ai_models.status, ai_providers.status AS provider_status").
		Joins("JOIN ai_providers ON ai_providers.id = ai_models.provider_id").
		Where("ai_models.id = ?", modelID).
		First(&modelRow).Error; err != nil {
		return err
	}
	if modelRow.Status != ai.ModelStatusActive || modelRow.ProviderStatus != ai.ProviderStatusActive {
		return ErrModelNotFound
	}
	return nil
}

func (s *ServiceDefService) systemConfigValue(key string) string {
	var value string
	_ = s.db.Table("system_configs").Where("\"key\" = ?", key).Select("value").Scan(&value).Error
	return value
}

func normalizedHealthDecisionMode(decisionMode string) string {
	if strings.TrimSpace(decisionMode) == "" {
		return "direct_first"
	}
	return strings.TrimSpace(decisionMode)
}

func (s *ServiceDefService) checkReferencePathRisk(svc *ServiceDefinition, decisionMode string) *ServiceHealthItem {
	if len(svc.WorkflowJSON) == 0 {
		if decisionMode == "direct_first" {
			return &ServiceHealthItem{Key: "reference_path", Label: "参考路径", Status: "warn", Message: "当前为 direct_first 模式，但未生成参考路径；运行时会退化为纯 AI 推理"}
		}
		return nil
	}

	validationErrors := engine.ValidateWorkflow(json.RawMessage(svc.WorkflowJSON))
	for _, err := range validationErrors {
		if !err.IsWarning() {
			return &ServiceHealthItem{Key: "reference_path", Label: "参考路径", Status: "fail", Message: fmt.Sprintf("参考路径结构错误：%s", err.Message)}
		}
	}
	for _, err := range validationErrors {
		if err.IsWarning() {
			return &ServiceHealthItem{Key: "reference_path", Label: "参考路径", Status: "warn", Message: err.Message}
		}
	}

	def, err := engine.ParseWorkflowDef(json.RawMessage(svc.WorkflowJSON))
	if err != nil {
		return &ServiceHealthItem{Key: "reference_path", Label: "参考路径", Status: "fail", Message: fmt.Sprintf("参考路径 JSON 解析失败：%v", err)}
	}
	if decisionMode == "direct_first" && !workflowHasExtractableHints(def) {
		return &ServiceHealthItem{Key: "reference_path", Label: "参考路径", Status: "warn", Message: "当前为 direct_first 模式，但参考路径无法提取有效运行提示；运行时会退化为纯 AI 推理"}
	}
	if issue := s.checkWorkflowActionRisk(svc.ID, def); issue != nil {
		return issue
	}
	if issue := s.checkWorkflowParticipantRisk(def); issue != nil {
		return issue
	}
	return nil
}

func workflowHasExtractableHints(def *engine.WorkflowDef) bool {
	if def == nil {
		return false
	}
	for _, node := range def.Nodes {
		switch node.Type {
		case engine.NodeStart, engine.NodeEnd:
			continue
		default:
			return true
		}
	}
	return len(def.Edges) > 0
}

func (s *ServiceDefService) checkWorkflowActionRisk(serviceID uint, def *engine.WorkflowDef) *ServiceHealthItem {
	if def == nil {
		return nil
	}
	for _, node := range def.Nodes {
		if node.Type != engine.NodeAction {
			continue
		}
		data, err := engine.ParseNodeData(node.Data)
		if err != nil {
			return &ServiceHealthItem{Key: "reference_path_action", Label: "参考路径动作", Status: "warn", Message: fmt.Sprintf("动作节点 %s 配置无法解析：%v", node.ID, err)}
		}
		if data.ActionID == 0 {
			return &ServiceHealthItem{Key: "reference_path_action", Label: "参考路径动作", Status: "warn", Message: fmt.Sprintf("动作节点 %s 未绑定可执行动作", node.ID)}
		}
		var count int64
		s.db.Model(&ServiceAction{}).Where("id = ? AND service_id = ? AND is_active = ?", data.ActionID, serviceID, true).Count(&count)
		if count == 0 {
			return &ServiceHealthItem{Key: "reference_path_action", Label: "参考路径动作", Status: "warn", Message: fmt.Sprintf("动作节点 %s 引用的动作 ID=%d 不存在或未启用", node.ID, data.ActionID)}
		}
	}
	return nil
}

func (s *ServiceDefService) checkWorkflowParticipantRisk(def *engine.WorkflowDef) *ServiceHealthItem {
	if def == nil {
		return nil
	}
	if issue := s.checkWorkflowParticipantAvailability(def); issue != nil {
		return issue
	}
	for _, node := range def.Nodes {
		if node.Type != engine.NodeForm && node.Type != engine.NodeApprove && node.Type != engine.NodeProcess {
			continue
		}
		data, err := engine.ParseNodeData(node.Data)
		if err != nil {
			return &ServiceHealthItem{Key: "reference_path_participant", Label: "参考路径参与者", Status: "warn", Message: fmt.Sprintf("人工节点 %s 参与者配置无法解析：%v", node.ID, err)}
		}
		if len(data.Participants) == 0 {
			return &ServiceHealthItem{Key: "reference_path_participant", Label: "参考路径参与者", Status: "warn", Message: fmt.Sprintf("人工节点 %s 未配置参与者，运行时需要 AI 额外判断处理人", node.ID)}
		}
	}
	return nil
}

func (s *ServiceDefService) checkWorkflowParticipantAvailability(def *engine.WorkflowDef) *ServiceHealthItem {
	if def == nil {
		return nil
	}
	for _, node := range def.Nodes {
		if node.Type != engine.NodeForm && node.Type != engine.NodeApprove && node.Type != engine.NodeProcess {
			continue
		}
		data, err := engine.ParseNodeData(node.Data)
		if err != nil {
			continue
		}
		for _, participant := range data.Participants {
			if participant.Type == "requester" || participant.Type == "requester_manager" {
				continue
			}
			if s.resolver == nil {
				return &ServiceHealthItem{
					Key:     "reference_path_participant",
					Label:   "Reference Participants",
					Status:  "warn",
					Message: fmt.Sprintf("manual node %s cannot verify participant availability because org resolver is unavailable", node.ID),
				}
			}
			userIDs, resolveErr := s.resolver.Resolve(s.db.DB, 0, participant)
			if resolveErr != nil {
				return &ServiceHealthItem{
					Key:     "reference_path_participant",
					Label:   "Reference Participants",
					Status:  "fail",
					Message: fmt.Sprintf("manual node %s participant %s cannot be resolved: %v", node.ID, describeWorkflowParticipant(participant), resolveErr),
				}
			}
			if len(userIDs) == 0 {
				return &ServiceHealthItem{
					Key:     "reference_path_participant",
					Label:   "Reference Participants",
					Status:  "fail",
					Message: fmt.Sprintf("manual node %s participant %s has no active approver", node.ID, describeWorkflowParticipant(participant)),
				}
			}
		}
	}
	return nil
}

func describeWorkflowParticipant(p engine.Participant) string {
	switch p.Type {
	case "user":
		if strings.TrimSpace(p.Value) != "" {
			return fmt.Sprintf("user(%s)", strings.TrimSpace(p.Value))
		}
	case "position":
		if strings.TrimSpace(p.Value) != "" {
			return fmt.Sprintf("position(%s)", strings.TrimSpace(p.Value))
		}
	case "department":
		if strings.TrimSpace(p.Value) != "" {
			return fmt.Sprintf("department(%s)", strings.TrimSpace(p.Value))
		}
	case "position_department":
		return fmt.Sprintf("position_department(%s@%s)", strings.TrimSpace(p.PositionCode), strings.TrimSpace(p.DepartmentCode))
	}
	return p.Type
}

func (s *ServiceDefService) checkFallbackRisk() *ServiceHealthItem {
	if s.engineConfigSvc == nil {
		return nil
	}
	fallbackID := s.engineConfigSvc.FallbackAssigneeID()
	if fallbackID == 0 {
		return nil
	}
	var user struct {
		IsActive bool
	}
	if err := s.db.Table("users").Where("id = ? AND deleted_at IS NULL", fallbackID).Select("is_active").First(&user).Error; err != nil || !user.IsActive {
		return &ServiceHealthItem{Key: "fallback_assignee", Label: "兜底处理人", Status: "warn", Message: "兜底处理人不存在或未启用，参与者无法解析时将无法自动兜底"}
	}
	return nil
}

func (s *ServiceDefService) buildPublishHealthPayload(svc *ServiceDefinition) (map[string]any, publishHealthValidationContext, error) {
	actions := make([]map[string]any, 0)
	var actionRows []ServiceAction
	if err := s.db.DB.Where("service_id = ? AND deleted_at IS NULL", svc.ID).Order("id asc").Find(&actionRows).Error; err != nil {
		return nil, publishHealthValidationContext{}, fmt.Errorf("璇诲彇鏈嶅姟鍔ㄤ綔澶辫触: %w", err)
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

	payload := map[string]any{
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
			"decisionMode":    s.engineConfigSvc.DecisionMode(),
			"decisionAgentId": s.engineConfigSvc.DecisionAgentID(),
		},
		"actions": actions,
	}
	return payload, buildPublishHealthValidationContext(svc, actionRows), nil
}

func normalizePublishHealthCheck(serviceID uint, status string, items []ServiceHealthItem, ctx publishHealthValidationContext) *ServiceHealthCheck {
	normalizedItems := make([]ServiceHealthItem, 0, len(items))
	declaredStatus := normalizePublishHealthStatus(status)
	maxLevel := healthLevel("pass")
	skippedNonActionableIssue := false
	invalidActionableIssue := false
	for idx, item := range items {
		if isNonActionableLLMHealthIssue(item) {
			skippedNonActionableIssue = true
			continue
		}
		itemStatus := normalizePublishHealthStatus(item.Status)
		if itemStatus == "" {
			itemStatus = "warn"
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
		recommendation := strings.TrimSpace(item.Recommendation)
		evidence := strings.TrimSpace(item.Evidence)
		location := ServiceHealthLocation{
			Kind:  strings.ToLower(strings.TrimSpace(item.Location.Kind)),
			Path:  strings.TrimSpace(item.Location.Path),
			RefID: strings.TrimSpace(item.Location.RefID),
		}
		if !isValidHealthLocation(location, ctx) || recommendation == "" || evidence == "" {
			invalidActionableIssue = true
			continue
		}
		if itemLevel := healthLevel(itemStatus); itemLevel > maxLevel {
			maxLevel = itemLevel
		}
		normalizedItems = append(normalizedItems, ServiceHealthItem{
			Key:            key,
			Label:          label,
			Status:         itemStatus,
			Message:        message,
			Location:       location,
			Recommendation: recommendation,
			Evidence:       evidence,
		})
	}

	finalStatus := levelStatus(maxLevel)
	if len(normalizedItems) == 0 && (declaredStatus != "pass" || invalidActionableIssue || skippedNonActionableIssue) {
		if invalidActionableIssue {
			slog.Warn("publish health check discarded invalid llm diagnostics", "service_id", serviceID, "declared_status", declaredStatus, "item_count", len(items))
		}
		if skippedNonActionableIssue {
			slog.Warn("publish health check discarded non-actionable llm diagnostics", "service_id", serviceID, "declared_status", declaredStatus, "item_count", len(items))
		}
		return &ServiceHealthCheck{
			ServiceID: serviceID,
			Status:    "pass",
			Items:     []ServiceHealthItem{},
		}
	}

	return &ServiceHealthCheck{
		ServiceID: serviceID,
		Status:    finalStatus,
		Items:     normalizedItems,
	}
}

func isNonActionableLLMHealthIssue(item ServiceHealthItem) bool {
	return isLLMRuntimeConfigIssue(item) || isLLMFallbackAssigneeIssue(item) || isLLMParticipantValidationGuess(item) || isLLMSystemCapabilityGuess(item)
}

func isLLMRuntimeConfigIssue(item ServiceHealthItem) bool {
	return strings.ToLower(strings.TrimSpace(item.Location.Kind)) == "runtime_config" || strings.HasPrefix(strings.ToLower(strings.TrimSpace(item.Location.Path)), "runtime.")
}

func isLLMFallbackAssigneeIssue(item ServiceHealthItem) bool {
	key := strings.ToLower(strings.TrimSpace(item.Key))
	path := strings.ToLower(strings.TrimSpace(item.Location.Path))
	label := strings.ToLower(strings.TrimSpace(item.Label))
	message := strings.ToLower(strings.TrimSpace(item.Message))
	evidence := strings.ToLower(strings.TrimSpace(item.Evidence))
	recommendation := strings.ToLower(strings.TrimSpace(item.Recommendation))

	if key == "fallback_assignee" || key == "fallbackassignee" || key == "fallback_assignee_validation" {
		return true
	}
	if path == "runtime.fallbackassignee" || path == "runtime.fallback_assignee" || strings.HasPrefix(path, "runtime.fallbackassignee.") || strings.HasPrefix(path, "runtime.fallback_assignee.") {
		return true
	}
	text := label + " " + message + " " + evidence + " " + recommendation
	return strings.Contains(text, "兜底处理人") && (strings.Contains(text, "校验") || strings.Contains(text, "验证"))
}

func isLLMParticipantValidationGuess(item ServiceHealthItem) bool {
	key := strings.ToLower(strings.TrimSpace(item.Key))
	path := strings.ToLower(strings.TrimSpace(item.Location.Path))
	locationKind := strings.ToLower(strings.TrimSpace(item.Location.Kind))
	refID := strings.TrimSpace(item.Location.RefID)
	label := strings.ToLower(strings.TrimSpace(item.Label))
	message := strings.ToLower(strings.TrimSpace(item.Message))
	evidence := strings.ToLower(strings.TrimSpace(item.Evidence))
	recommendation := strings.ToLower(strings.TrimSpace(item.Recommendation))
	text := key + " " + label + " " + message + " " + evidence + " " + recommendation

	if !isParticipantHealthText(key, path, text) || !isValidationGuessText(text) {
		return false
	}
	if locationKind == "workflow_node" && refID != "" && hasConcreteParticipantConflictEvidence(text) {
		return false
	}
	return true
}

func isLLMSystemCapabilityGuess(item ServiceHealthItem) bool {
	key := strings.ToLower(strings.TrimSpace(item.Key))
	label := strings.ToLower(strings.TrimSpace(item.Label))
	message := strings.ToLower(strings.TrimSpace(item.Message))
	evidence := strings.ToLower(strings.TrimSpace(item.Evidence))
	recommendation := strings.ToLower(strings.TrimSpace(item.Recommendation))
	text := key + " " + label + " " + message + " " + evidence + " " + recommendation
	if isValidationGuessText(text) {
		return true
	}
	infrastructureKeywords := []string{
		"审计日志", "审计配置", "审计存储", "存储位置", "日志存储", "日志路径", "日志服务",
		"数据库", "文件系统", "基础设施", "落盘", "未提供具体存储", "未提供存储", "未提供相关说明",
	}
	for _, keyword := range infrastructureKeywords {
		if strings.Contains(text, strings.ToLower(keyword)) {
			return true
		}
	}
	return false
}

func isParticipantHealthText(key string, path string, text string) bool {
	if strings.Contains(key, "participant") || strings.Contains(key, "assignee") {
		return true
	}
	if strings.Contains(path, "participant") || strings.Contains(path, "assignee") {
		return true
	}
	keywords := []string{"参与者", "处理人", "单人", "岗位编码", "部门编码", "position_department", "position_code", "department_code"}
	for _, keyword := range keywords {
		if strings.Contains(text, strings.ToLower(keyword)) {
			return true
		}
	}
	return false
}

func isValidationGuessText(text string) bool {
	keywords := []string{
		"未验证", "未校验", "没有验证", "没有校验", "未明确验证", "未明确校验",
		"缺少验证", "缺少校验", "验证缺失", "校验缺失", "验证不足", "校验不足",
		"未提供校验", "未提供验证", "是否符合协作规范", "确保所有流程节点", "确保参与者",
	}
	for _, keyword := range keywords {
		if strings.Contains(text, strings.ToLower(keyword)) {
			return true
		}
	}
	return false
}

func hasConcreteParticipantConflictEvidence(text string) bool {
	hasActual := strings.Contains(text, "实际") || strings.Contains(text, "当前") || strings.Contains(text, "配置为") || strings.Contains(text, "实际值")
	hasExpected := strings.Contains(text, "期望") || strings.Contains(text, "应为") || strings.Contains(text, "要求") || strings.Contains(text, "协作规范")
	hasCode := strings.Contains(text, "position_code") || strings.Contains(text, "department_code") || strings.Contains(text, "ops_admin") || strings.Contains(text, "network_admin") || strings.Contains(text, "security_admin")
	return hasActual && hasExpected && hasCode
}

func buildPublishHealthValidationContext(svc *ServiceDefinition, actions []ServiceAction) publishHealthValidationContext {
	ctx := publishHealthValidationContext{
		workflowNodeIDs: map[string]struct{}{},
		workflowEdgeIDs: map[string]struct{}{},
		actionRefs:      map[string]struct{}{},
	}

	for _, action := range actions {
		ctx.actionRefs[fmt.Sprintf("%d", action.ID)] = struct{}{}
		if code := strings.TrimSpace(action.Code); code != "" {
			ctx.actionRefs[code] = struct{}{}
		}
	}

	workflowHasActionNode := false
	var parsed struct {
		Nodes []struct {
			ID   string         `json:"id"`
			Type string         `json:"type"`
			Data map[string]any `json:"data"`
		} `json:"nodes"`
		Edges []struct {
			ID string `json:"id"`
		} `json:"edges"`
	}
	if len(svc.WorkflowJSON) > 0 && json.Unmarshal([]byte(svc.WorkflowJSON), &parsed) == nil {
		for _, node := range parsed.Nodes {
			if id := strings.TrimSpace(node.ID); id != "" {
				ctx.workflowNodeIDs[id] = struct{}{}
			}
			rawType := strings.ToLower(strings.TrimSpace(node.Type))
			if rawType == "" && node.Data != nil {
				if nodeType, ok := node.Data["nodeType"].(string); ok {
					rawType = strings.ToLower(strings.TrimSpace(nodeType))
				}
			}
			if rawType == "action" {
				workflowHasActionNode = true
			}
		}
		for _, edge := range parsed.Edges {
			if id := strings.TrimSpace(edge.ID); id != "" {
				ctx.workflowEdgeIDs[id] = struct{}{}
			}
		}
	}

	ctx.hasActionEvidence = workflowHasActionNode || collaborationSpecMentionsAction(svc.CollaborationSpec)
	return ctx
}

func collaborationSpecMentionsAction(spec string) bool {
	text := strings.ToLower(strings.TrimSpace(spec))
	if text == "" {
		return false
	}
	keywords := []string{
		"自动化", "动作", "action", "webhook", "脚本", "script", "通知", "邮件", "短信", "调用接口", "api", "自动执行",
	}
	for _, keyword := range keywords {
		if strings.Contains(text, keyword) {
			return true
		}
	}
	return false
}

func isValidHealthLocation(loc ServiceHealthLocation, ctx publishHealthValidationContext) bool {
	if loc.Kind == "" || loc.Path == "" {
		return false
	}

	switch loc.Kind {
	case "collaboration_spec":
		return loc.Path == "service.collaborationSpec"
	case "workflow_node":
		if loc.RefID == "" {
			return false
		}
		if _, ok := ctx.workflowNodeIDs[loc.RefID]; !ok {
			return false
		}
		return strings.HasPrefix(loc.Path, "service.workflowJson.nodes")
	case "workflow_edge":
		if loc.RefID == "" {
			return false
		}
		if _, ok := ctx.workflowEdgeIDs[loc.RefID]; !ok {
			return false
		}
		return strings.HasPrefix(loc.Path, "service.workflowJson.edges")
	case "action":
		if !ctx.hasActionEvidence || loc.RefID == "" {
			return false
		}
		if _, ok := ctx.actionRefs[loc.RefID]; !ok {
			return false
		}
		return strings.HasPrefix(loc.Path, "actions")
	case "runtime_config":
		return strings.HasPrefix(loc.Path, "runtime.")
	default:
		return false
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

func (s *ServiceDefService) validateSLAID(slaID *uint) error {
	if slaID == nil || *slaID == 0 {
		return nil
	}
	var sla SLATemplate
	if err := s.db.Where("id = ? AND is_active = ?", *slaID, true).First(&sla).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrSLATemplateUnavailable
		}
		return err
	}
	return nil
}

func parseOptionalSLAID(value any) (*uint, error) {
	if value == nil {
		return nil, nil
	}
	switch v := value.(type) {
	case uint:
		return &v, nil
	case *uint:
		return v, nil
	case int:
		if v < 0 {
			return nil, ErrSLATemplateUnavailable
		}
		parsed := uint(v)
		return &parsed, nil
	default:
		return nil, ErrSLATemplateUnavailable
	}
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
