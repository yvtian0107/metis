package itsm

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
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
	check := &ServiceHealthCheck{ServiceID: id, Status: "pass", Items: []ServiceHealthItem{}}
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
		}
		return check
	}

	if strings.TrimSpace(svc.CollaborationSpec) == "" {
		add("collaboration_spec", "协作规范", "fail", "协作规范为空，智能引擎缺少决策策略")
	}

	if svc.AgentID == nil || *svc.AgentID == 0 {
		add("service_agent", "服务 Agent", "fail", "智能服务未绑定 Agent")
	} else if err := s.validateAgent(svc.AgentID); err != nil {
		add("service_agent", "服务 Agent", "fail", "绑定的 Agent 不存在或未启用")
	}

	decisionAgentID := strings.TrimSpace(s.systemConfigValue("itsm.engine.decision.agent_id"))
	if decisionAgentID == "" || decisionAgentID == "0" {
		add("decision_agent", "决策 Agent", "fail", "全局决策 Agent 未配置")
	} else {
		id, err := strconv.ParseUint(decisionAgentID, 10, 64)
		if err != nil {
			add("decision_agent", "决策 Agent", "fail", "全局决策 Agent 配置值不是有效 Agent ID")
		} else if agentID := uint(id); agentID == 0 || s.validateAgent(&agentID) != nil {
			add("decision_agent", "决策 Agent", "fail", "全局决策 Agent 不存在或未启用")
		}
	}

	decisionMode := normalizedHealthDecisionMode(s.systemConfigValue("itsm.engine.decision.decision_mode"))
	if issue := s.checkReferencePathRisk(svc, decisionMode); issue != nil {
		add(issue.Key, issue.Label, issue.Status, issue.Message)
	}

	if issue := s.checkFallbackRisk(); issue != nil {
		add(issue.Key, issue.Label, issue.Status, issue.Message)
	}

	return check
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
			return &ServiceHealthItem{
				Key:     "reference_path",
				Label:   "参考路径",
				Status:  "warn",
				Message: "当前为 direct_first 模式，但未生成参考路径；运行时会退化为纯 AI 推理",
			}
		}
		return nil
	}

	validationErrors := engine.ValidateWorkflow(json.RawMessage(svc.WorkflowJSON))
	for _, err := range validationErrors {
		if !err.IsWarning() {
			return &ServiceHealthItem{
				Key:     "reference_path",
				Label:   "参考路径",
				Status:  "fail",
				Message: fmt.Sprintf("参考路径结构错误：%s", err.Message),
			}
		}
	}
	for _, err := range validationErrors {
		if err.IsWarning() {
			return &ServiceHealthItem{
				Key:     "reference_path",
				Label:   "参考路径",
				Status:  "warn",
				Message: err.Message,
			}
		}
	}

	def, err := engine.ParseWorkflowDef(json.RawMessage(svc.WorkflowJSON))
	if err != nil {
		return &ServiceHealthItem{
			Key:     "reference_path",
			Label:   "参考路径",
			Status:  "fail",
			Message: fmt.Sprintf("参考路径 JSON 解析失败：%v", err),
		}
	}

	if decisionMode == "direct_first" && !workflowHasExtractableHints(def) {
		return &ServiceHealthItem{
			Key:     "reference_path",
			Label:   "参考路径",
			Status:  "warn",
			Message: "当前为 direct_first 模式，但参考路径无法提取有效运行提示；运行时会退化为纯 AI 推理",
		}
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
	nodeMap := make(map[string]engine.WFNode, len(def.Nodes))
	outEdges := make(map[string][]engine.WFEdge, len(def.Edges))
	var startID string
	for _, node := range def.Nodes {
		nodeMap[node.ID] = node
		if node.Type == engine.NodeStart {
			startID = node.ID
		}
	}
	for _, edge := range def.Edges {
		outEdges[edge.Source] = append(outEdges[edge.Source], edge)
	}
	if startID == "" {
		return false
	}

	queue := []string{startID}
	visited := make(map[string]bool, len(def.Nodes))
	for len(queue) > 0 {
		nodeID := queue[0]
		queue = queue[1:]
		if visited[nodeID] {
			continue
		}
		visited[nodeID] = true
		node, ok := nodeMap[nodeID]
		if !ok {
			continue
		}
		if node.Type != engine.NodeStart {
			return true
		}
		for _, edge := range outEdges[nodeID] {
			queue = append(queue, edge.Target)
		}
	}
	return false
}

func (s *ServiceDefService) checkWorkflowActionRisk(serviceID uint, def *engine.WorkflowDef) *ServiceHealthItem {
	for _, node := range def.Nodes {
		if node.Type != engine.NodeAction {
			continue
		}
		data, err := engine.ParseNodeData(node.Data)
		if err != nil {
			return &ServiceHealthItem{
				Key:     "reference_path_action",
				Label:   "参考路径动作",
				Status:  "warn",
				Message: fmt.Sprintf("动作节点 %s 配置无法解析：%v", node.ID, err),
			}
		}
		if data.ActionID == 0 {
			return &ServiceHealthItem{
				Key:     "reference_path_action",
				Label:   "参考路径动作",
				Status:  "warn",
				Message: fmt.Sprintf("动作节点 %s 未绑定可执行动作", node.ID),
			}
		}
		var count int64
		s.db.Model(&ServiceAction{}).
			Where("id = ? AND service_id = ? AND is_active = ?", data.ActionID, serviceID, true).
			Count(&count)
		if count == 0 {
			return &ServiceHealthItem{
				Key:     "reference_path_action",
				Label:   "参考路径动作",
				Status:  "warn",
				Message: fmt.Sprintf("动作节点 %s 引用的动作 ID=%d 不存在或未启用", node.ID, data.ActionID),
			}
		}
	}
	return nil
}

func (s *ServiceDefService) checkWorkflowParticipantRisk(def *engine.WorkflowDef) *ServiceHealthItem {
	for _, node := range def.Nodes {
		if node.Type != engine.NodeForm && node.Type != engine.NodeProcess {
			continue
		}
		data, err := engine.ParseNodeData(node.Data)
		if err != nil {
			return &ServiceHealthItem{
				Key:     "reference_path_participant",
				Label:   "参考路径参与者",
				Status:  "warn",
				Message: fmt.Sprintf("人工节点 %s 参与者配置无法解析：%v", node.ID, err),
			}
		}
		if len(data.Participants) == 0 {
			return &ServiceHealthItem{
				Key:     "reference_path_participant",
				Label:   "参考路径参与者",
				Status:  "warn",
				Message: fmt.Sprintf("人工节点 %s 未配置参与者，运行时需要 AI 额外判断处理人", node.ID),
			}
		}
		for _, participant := range data.Participants {
			if issue := s.checkParticipantRisk(node.ID, participant); issue != nil {
				return issue
			}
		}
	}
	return nil
}

func (s *ServiceDefService) checkParticipantRisk(nodeID string, participant engine.Participant) *ServiceHealthItem {
	item := func(message string) *ServiceHealthItem {
		return &ServiceHealthItem{
			Key:     "reference_path_participant",
			Label:   "参考路径参与者",
			Status:  "warn",
			Message: message,
		}
	}

	switch participant.Type {
	case "user":
		value := strings.TrimSpace(participant.Value)
		if value == "" {
			return item(fmt.Sprintf("人工节点 %s 的 user 参与者缺少用户标识", nodeID))
		}
		var count int64
		if id, err := strconv.ParseUint(value, 10, 64); err == nil {
			s.db.Table("users").Where("id = ? AND is_active = ?", uint(id), true).Count(&count)
		} else {
			s.db.Table("users").Where("username = ? AND is_active = ?", value, true).Count(&count)
		}
		if count == 0 {
			return item(fmt.Sprintf("人工节点 %s 的 user 参与者 %q 不存在或未启用", nodeID, value))
		}
	case "position", "department":
		if strings.TrimSpace(participant.Value) == "" {
			return item(fmt.Sprintf("人工节点 %s 的 %s 参与者缺少标识", nodeID, participant.Type))
		}
	case "position_department":
		if strings.TrimSpace(participant.PositionCode) == "" || strings.TrimSpace(participant.DepartmentCode) == "" {
			return item(fmt.Sprintf("人工节点 %s 的 position_department 参与者缺少岗位或部门编码", nodeID))
		}
	case "requester_manager":
		return nil
	default:
		return item(fmt.Sprintf("人工节点 %s 使用了不支持的参与者类型 %q", nodeID, participant.Type))
	}
	return nil
}

func (s *ServiceDefService) checkFallbackRisk() *ServiceHealthItem {
	fallback := strings.TrimSpace(s.systemConfigValue("itsm.engine.general.fallback_assignee"))
	if fallback == "" || fallback == "0" {
		return nil
	}
	id, err := strconv.ParseUint(fallback, 10, 64)
	if err != nil {
		return &ServiceHealthItem{
			Key:     "fallback",
			Label:   "兜底处理人",
			Status:  "warn",
			Message: "兜底处理人配置值不是有效用户 ID",
		}
	}
	var count int64
	s.db.Table("users").Where("id = ? AND is_active = ?", uint(id), true).Count(&count)
	if count == 0 {
		return &ServiceHealthItem{
			Key:     "fallback",
			Label:   "兜底处理人",
			Status:  "warn",
			Message: fmt.Sprintf("已配置的兜底处理人 ID=%d 不存在或未启用", id),
		}
	}
	return nil
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
