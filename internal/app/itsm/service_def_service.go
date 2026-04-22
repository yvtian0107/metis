package itsm

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/samber/do/v2"
	"gorm.io/gorm"
	"metis/internal/app/ai"
	"metis/internal/database"

	"metis/internal/app/itsm/engine"
)

var (
	ErrServiceDefNotFound    = errors.New("service definition not found")
	ErrServiceCodeExists     = errors.New("service code already exists")
	ErrWorkflowValidation    = errors.New("workflow validation failed")
	ErrServiceEngineMismatch = errors.New("service engine field mismatch")
	ErrAgentNotAvailable     = errors.New("agent not available")
)

type ServiceDefService struct {
	repo     *ServiceDefRepo
	db       *database.DB
	catalogs *CatalogRepo
}

func NewServiceDefService(i do.Injector) (*ServiceDefService, error) {
	repo := do.MustInvoke[*ServiceDefRepo](i)
	db := do.MustInvoke[*database.DB](i)
	catalogs := do.MustInvoke[*CatalogRepo](i)
	return &ServiceDefService{repo: repo, db: db, catalogs: catalogs}, nil
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
		if isSQLiteUniqueError(err) {
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
		if isSQLiteUniqueError(err) {
			return nil, ErrServiceCodeExists
		}
		return nil, err
	}
	if existing.PublishHealthCheckedAt != nil {
		if _, err := s.RefreshPublishHealthCheck(id); err != nil {
			return nil, err
		}
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

type ServiceHealthItem struct {
	Key     string `json:"key"`
	Label   string `json:"label"`
	Status  string `json:"status"` // pass | warn | fail
	Message string `json:"message"`
}

type ServiceHealthCheck struct {
	ServiceID uint                `json:"serviceId"`
	Status    string              `json:"status"` // pass | warn | fail
	Items     []ServiceHealthItem `json:"items"`
	CheckedAt *time.Time          `json:"checkedAt,omitempty"`
}

func (s *ServiceDefService) HealthCheck(id uint) (*ServiceHealthCheck, error) {
	svc, err := s.Get(id)
	if err != nil {
		return nil, err
	}
	return svc.ToResponse().PublishHealthCheck, nil
}

func (s *ServiceDefService) RefreshPublishHealthCheck(id uint) (*ServiceHealthCheck, error) {
	svc, err := s.Get(id)
	if err != nil {
		return nil, err
	}
	check := s.computePublishHealthCheck(svc)
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

func (s *ServiceDefService) computePublishHealthCheck(svc *ServiceDefinition) *ServiceHealthCheck {
	id := svc.ID
	check := &ServiceHealthCheck{ServiceID: id, Status: "pass"}
	add := func(key, label, status, message string) {
		check.Items = append(check.Items, ServiceHealthItem{Key: key, Label: label, Status: status, Message: message})
		if status == "fail" {
			check.Status = "fail"
		} else if status == "warn" && check.Status == "pass" {
			check.Status = "warn"
		}
	}

	if svc.EngineType == "classic" {
		if len(svc.WorkflowJSON) == 0 {
			add("workflow", "流程定义", "fail", "经典引擎必须配置可执行工作流")
		} else if err := validateWorkflowJSON(json.RawMessage(svc.WorkflowJSON)); err != nil {
			add("workflow", "流程定义", "fail", err.Error())
		} else {
			add("workflow", "流程定义", "pass", "工作流结构校验通过")
		}
		return check
	}

	if strings.TrimSpace(svc.CollaborationSpec) == "" {
		add("collaboration_spec", "协作规范", "fail", "智能引擎需要协作规范作为决策策略")
	} else {
		add("collaboration_spec", "协作规范", "pass", "协作规范已配置")
	}

	if svc.AgentID == nil || *svc.AgentID == 0 {
		add("service_agent", "服务 Agent", "fail", "智能服务未绑定 Agent")
	} else if err := s.validateAgent(svc.AgentID); err != nil {
		add("service_agent", "服务 Agent", "fail", "绑定的 Agent 不存在或未启用")
	} else {
		add("service_agent", "服务 Agent", "pass", "服务 Agent 可用")
	}

	var decisionAgentID string
	_ = s.db.Table("system_configs").Where("\"key\" = ?", "itsm.engine.decision.agent_id").Select("value").Scan(&decisionAgentID).Error
	if strings.TrimSpace(decisionAgentID) == "" || decisionAgentID == "0" {
		add("decision_agent", "决策 Agent", "fail", "全局决策 Agent 未配置")
	} else {
		add("decision_agent", "决策 Agent", "pass", "全局决策 Agent 已配置")
	}

	if len(svc.WorkflowJSON) == 0 {
		add("reference_path", "参考路径", "warn", "未生成参考路径；direct_first 会降级为纯 AI 推理")
	} else {
		add("reference_path", "参考路径", "pass", "已配置参考路径/策略草图")
	}

	var activeActions int64
	s.db.Model(&ServiceAction{}).Where("service_id = ? AND is_active = ?", id, true).Count(&activeActions)
	if activeActions == 0 {
		add("actions", "自动化动作", "warn", "未配置自动化动作；如流程需要预检/放行，请先配置动作")
	} else {
		add("actions", "自动化动作", "pass", fmt.Sprintf("已配置 %d 个可用动作", activeActions))
	}

	var parsedDocs int64
	s.db.Model(&ServiceKnowledgeDocument{}).Where("service_id = ? AND parse_status = ?", id, "completed").Count(&parsedDocs)
	if parsedDocs == 0 {
		add("knowledge", "知识库", "warn", "未关联已解析知识文档；AI 将主要依赖协作规范和工单上下文")
	} else {
		add("knowledge", "知识库", "pass", fmt.Sprintf("已关联 %d 个可检索知识文档", parsedDocs))
	}

	var fallback string
	_ = s.db.Table("system_configs").Where("\"key\" = ?", "itsm.engine.general.fallback_assignee").Select("value").Scan(&fallback).Error
	if strings.TrimSpace(fallback) == "" || fallback == "0" {
		add("fallback", "兜底处理人", "warn", "未配置兜底处理人；参与者解析失败时需要管理员接管")
	} else {
		add("fallback", "兜底处理人", "pass", "兜底处理人已配置")
	}

	add("permissions", "权限与接管", "warn", "请确认审批人、管理员接管权限已按角色分配")
	return check
}

// validateWorkflowJSON runs the engine validator and wraps errors.
func validateWorkflowJSON(raw json.RawMessage) error {
	errs := engine.ValidateWorkflow(raw)
	if len(errs) == 0 {
		return nil
	}
	return fmt.Errorf("%w: %s", ErrWorkflowValidation, errs[0].Message)
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
