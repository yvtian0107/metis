package config

import (
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/samber/do/v2"
	"gorm.io/gorm"

	ai "metis/internal/app/ai/runtime"
	"metis/internal/database"
	"metis/internal/model"
	"metis/internal/repository"
)

const (
	SmartTicketIntakeAgentKey       = "itsm.smart_ticket.intake.agent_id"
	SmartTicketDecisionAgentKey     = "itsm.smart_ticket.decision.agent_id"
	SmartTicketSLAAssuranceAgentKey = "itsm.smart_ticket.sla_assurance.agent_id"
	SmartTicketDecisionModeKey      = "itsm.smart_ticket.decision.mode"

	SmartTicketPathModelKey       = "itsm.smart_ticket.path.model_id"
	SmartTicketPathTemperatureKey = "itsm.smart_ticket.path.temperature"
	SmartTicketPathMaxRetriesKey  = "itsm.smart_ticket.path.max_retries"
	SmartTicketPathTimeoutKey     = "itsm.smart_ticket.path.timeout_seconds"

	SmartTicketGuardAuditLevelKey = "itsm.smart_ticket.guard.audit_level"
	SmartTicketGuardFallbackKey   = "itsm.smart_ticket.guard.fallback_assignee"
)

var (
	ErrEngineNotConfigured  = errors.New("智能岗位未配置，请前往智能岗位页面设置")
	ErrModelNotFound        = errors.New("模型不存在或已停用")
	ErrAgentNotFound        = errors.New("智能体不存在或已停用")
	ErrFallbackUserNotFound = errors.New("兜底处理人不存在或已停用")
	ErrInvalidEngineConfig  = errors.New("ITSM 配置无效")
)

// EngineConfigService manages ITSM smart staffing and model-engine runtime settings.
type EngineConfigService struct {
	agentSvc      *ai.AgentService
	modelRepo     *ai.ModelRepo
	providerSvc   *ai.ProviderService
	toolRuntime   *ai.ToolRuntimeService
	sysConfigRepo *repository.SysConfigRepo
	db            *gorm.DB
}

func NewEngineConfigService(i do.Injector) (*EngineConfigService, error) {
	db := do.MustInvoke[*database.DB](i)
	toolRuntime, _ := do.Invoke[*ai.ToolRuntimeService](i)
	return &EngineConfigService{
		agentSvc:      do.MustInvoke[*ai.AgentService](i),
		modelRepo:     do.MustInvoke[*ai.ModelRepo](i),
		providerSvc:   do.MustInvoke[*ai.ProviderService](i),
		toolRuntime:   toolRuntime,
		sysConfigRepo: do.MustInvoke[*repository.SysConfigRepo](i),
		db:            db.DB,
	}, nil
}

type SmartStaffingConfig struct {
	Posts  SmartStaffingPosts `json:"posts"`
	Health EngineHealth       `json:"health"`
}

type SmartStaffingPosts struct {
	Intake       EngineAgentSelector  `json:"intake"`
	Decision     EngineDecisionConfig `json:"decision"`
	SLAAssurance EngineAgentSelector  `json:"slaAssurance"`
}

type EngineSettingsConfig struct {
	Runtime EngineSettingsRuntime `json:"runtime"`
	Health  EngineHealth          `json:"health"`
}

type EngineSettingsRuntime struct {
	PathBuilder EnginePathConfig  `json:"pathBuilder"`
	Guard       EngineGuardConfig `json:"guard"`
}

type EngineAgentSelector struct {
	AgentID   uint   `json:"agentId"`
	AgentName string `json:"agentName"`
}

type EngineDecisionConfig struct {
	EngineAgentSelector
	Mode string `json:"mode"`
}

type EngineModelConfig struct {
	ModelID      uint    `json:"modelId"`
	ProviderID   uint    `json:"providerId"`
	ProviderName string  `json:"providerName"`
	ModelName    string  `json:"modelName"`
	Temperature  float64 `json:"temperature"`
}

type EnginePathConfig struct {
	EngineModelConfig
	MaxRetries     int `json:"maxRetries"`
	TimeoutSeconds int `json:"timeoutSeconds"`
}

type EngineGuardConfig struct {
	AuditLevel       string `json:"auditLevel"`
	FallbackAssignee uint   `json:"fallbackAssignee"`
}

type EngineHealth struct {
	Items []EngineHealthItem `json:"items"`
}

type EngineHealthItem struct {
	Key     string `json:"key"`
	Label   string `json:"label"`
	Status  string `json:"status"`
	Message string `json:"message"`
}

type UpdateSmartStaffingRequest struct {
	Posts struct {
		Intake struct {
			AgentID uint `json:"agentId"`
		} `json:"intake"`
		Decision struct {
			AgentID uint   `json:"agentId"`
			Mode    string `json:"mode"`
		} `json:"decision"`
		SLAAssurance struct {
			AgentID uint `json:"agentId"`
		} `json:"slaAssurance"`
	} `json:"posts"`
}

type UpdateEngineSettingsRequest struct {
	Runtime struct {
		PathBuilder struct {
			ModelID        uint    `json:"modelId"`
			Temperature    float64 `json:"temperature"`
			MaxRetries     int     `json:"maxRetries"`
			TimeoutSeconds int     `json:"timeoutSeconds"`
		} `json:"pathBuilder"`
		Guard struct {
			AuditLevel       string `json:"auditLevel"`
			FallbackAssignee uint   `json:"fallbackAssignee"`
		} `json:"guard"`
	} `json:"runtime"`
}

type LLMEngineRuntimeConfig struct {
	Model          string
	Protocol       string
	BaseURL        string
	APIKey         string
	Temperature    float64
	MaxTokens      int
	MaxRetries     int
	TimeoutSeconds int
}

func (s *EngineConfigService) GetSmartStaffingConfig() (*SmartStaffingConfig, error) {
	cfg := &SmartStaffingConfig{
		Posts: SmartStaffingPosts{
			Intake: s.readAgentSelector(SmartTicketIntakeAgentKey),
			Decision: EngineDecisionConfig{
				EngineAgentSelector: s.readAgentSelector(SmartTicketDecisionAgentKey),
				Mode:                s.DecisionMode(),
			},
			SLAAssurance: s.readAgentSelector(SmartTicketSLAAssuranceAgentKey),
		},
	}
	cfg.Health = s.buildSmartStaffingHealth(cfg)
	return cfg, nil
}

func (s *EngineConfigService) UpdateSmartStaffingConfig(req *UpdateSmartStaffingRequest) error {
	if req.Posts.Intake.AgentID > 0 {
		if err := s.validateActiveAgent(req.Posts.Intake.AgentID); err != nil {
			return err
		}
	}
	if req.Posts.Decision.AgentID > 0 {
		if err := s.validateActiveAgent(req.Posts.Decision.AgentID); err != nil {
			return err
		}
	}
	if req.Posts.SLAAssurance.AgentID > 0 {
		if err := s.validateActiveAgent(req.Posts.SLAAssurance.AgentID); err != nil {
			return err
		}
	}
	if err := validateDecisionMode(req.Posts.Decision.Mode); err != nil {
		return err
	}

	s.setConfigValue(SmartTicketIntakeAgentKey, strconv.FormatUint(uint64(req.Posts.Intake.AgentID), 10))
	s.setConfigValue(SmartTicketDecisionAgentKey, strconv.FormatUint(uint64(req.Posts.Decision.AgentID), 10))
	s.setConfigValue(SmartTicketSLAAssuranceAgentKey, strconv.FormatUint(uint64(req.Posts.SLAAssurance.AgentID), 10))
	s.setConfigValue(SmartTicketDecisionModeKey, req.Posts.Decision.Mode)
	return nil
}

func (s *EngineConfigService) GetEngineSettingsConfig() (*EngineSettingsConfig, error) {
	cfg := &EngineSettingsConfig{
		Runtime: EngineSettingsRuntime{
			PathBuilder: s.readPathConfig(),
			Guard:       s.readGuardConfig(),
		},
	}
	cfg.Health = s.buildEngineSettingsHealth(cfg)
	return cfg, nil
}

func (s *EngineConfigService) UpdateEngineSettingsConfig(req *UpdateEngineSettingsRequest) error {
	if req.Runtime.PathBuilder.ModelID > 0 {
		if err := s.validateActiveModel(req.Runtime.PathBuilder.ModelID); err != nil {
			return err
		}
	}
	if err := validateTemperature("参考路径", req.Runtime.PathBuilder.Temperature); err != nil {
		return err
	}
	if req.Runtime.PathBuilder.MaxRetries < 0 || req.Runtime.PathBuilder.MaxRetries > 10 {
		return fmt.Errorf("%w: 参考路径最大重试次数必须在 0 到 10 之间", ErrInvalidEngineConfig)
	}
	if req.Runtime.PathBuilder.TimeoutSeconds < 10 || req.Runtime.PathBuilder.TimeoutSeconds > 300 {
		return fmt.Errorf("%w: 参考路径超时时间必须在 10 到 300 秒之间", ErrInvalidEngineConfig)
	}
	if err := validateAuditLevel(req.Runtime.Guard.AuditLevel); err != nil {
		return err
	}
	if req.Runtime.Guard.FallbackAssignee > 0 {
		if err := s.validateFallbackAssignee(req.Runtime.Guard.FallbackAssignee); err != nil {
			return err
		}
	}

	s.setConfigValue(SmartTicketPathModelKey, strconv.FormatUint(uint64(req.Runtime.PathBuilder.ModelID), 10))
	s.setConfigValue(SmartTicketPathTemperatureKey, formatFloat(req.Runtime.PathBuilder.Temperature))
	s.setConfigValue(SmartTicketPathMaxRetriesKey, strconv.Itoa(req.Runtime.PathBuilder.MaxRetries))
	s.setConfigValue(SmartTicketPathTimeoutKey, strconv.Itoa(req.Runtime.PathBuilder.TimeoutSeconds))
	s.setConfigValue(SmartTicketGuardAuditLevelKey, req.Runtime.Guard.AuditLevel)
	s.setConfigValue(SmartTicketGuardFallbackKey, strconv.FormatUint(uint64(req.Runtime.Guard.FallbackAssignee), 10))
	return nil
}

func (s *EngineConfigService) PathBuilderRuntimeConfig() (LLMEngineRuntimeConfig, error) {
	cfg := s.readPathConfig()
	return s.buildLLMRuntimeConfig("参考路径生成引擎", cfg.ModelID, cfg.Temperature, 4096, cfg.MaxRetries, cfg.TimeoutSeconds)
}

func (s *EngineConfigService) buildLLMRuntimeConfig(label string, modelID uint, temperature float64, maxTokens int, maxRetries int, timeoutSeconds int) (LLMEngineRuntimeConfig, error) {
	if modelID == 0 {
		return LLMEngineRuntimeConfig{}, fmt.Errorf("%s未配置模型", label)
	}
	m, err := s.modelRepo.FindByID(modelID)
	if err != nil || m.Status != ai.ModelStatusActive {
		return LLMEngineRuntimeConfig{}, ErrModelNotFound
	}
	if m.Provider == nil || m.Provider.Status != ai.ProviderStatusActive {
		return LLMEngineRuntimeConfig{}, ErrModelNotFound
	}
	apiKey, err := s.providerSvc.DecryptAPIKey(m.Provider)
	if err != nil {
		return LLMEngineRuntimeConfig{}, fmt.Errorf("API Key 解密失败: %w", err)
	}
	return LLMEngineRuntimeConfig{
		Model:          m.ModelID,
		Protocol:       ai.ProtocolForType(m.Provider.Type),
		BaseURL:        m.Provider.BaseURL,
		APIKey:         apiKey,
		Temperature:    temperature,
		MaxTokens:      maxTokens,
		MaxRetries:     maxRetries,
		TimeoutSeconds: timeoutSeconds,
	}, nil
}

func (s *EngineConfigService) readPathConfig() EnginePathConfig {
	cfg := EnginePathConfig{
		EngineModelConfig: EngineModelConfig{
			ModelID:     uint(s.getConfigInt(SmartTicketPathModelKey, 0)),
			Temperature: s.getConfigFloat(SmartTicketPathTemperatureKey, 0.3),
		},
		MaxRetries:     s.getConfigInt(SmartTicketPathMaxRetriesKey, 1),
		TimeoutSeconds: s.getConfigInt(SmartTicketPathTimeoutKey, 60),
	}
	s.fillModelMeta(&cfg.EngineModelConfig)
	return cfg
}

func (s *EngineConfigService) fillModelMeta(cfg *EngineModelConfig) {
	if cfg.ModelID == 0 {
		return
	}
	m, err := s.modelRepo.FindByID(cfg.ModelID)
	if err != nil {
		return
	}
	cfg.ModelName = m.DisplayName
	cfg.ProviderID = m.ProviderID
	if m.Provider != nil {
		cfg.ProviderName = m.Provider.Name
	}
}

func (s *EngineConfigService) readGuardConfig() EngineGuardConfig {
	return EngineGuardConfig{
		AuditLevel:       s.getConfigValue(SmartTicketGuardAuditLevelKey, "full"),
		FallbackAssignee: uint(s.getConfigInt(SmartTicketGuardFallbackKey, 0)),
	}
}

func (s *EngineConfigService) readAgentSelector(configKey string) EngineAgentSelector {
	agentID := uint(s.getConfigInt(configKey, 0))
	if agentID == 0 {
		return EngineAgentSelector{}
	}
	agent, err := s.agentSvc.Get(agentID)
	if err != nil {
		return EngineAgentSelector{AgentID: agentID}
	}
	return EngineAgentSelector{AgentID: agent.ID, AgentName: agent.Name}
}

func (s *EngineConfigService) validateActiveAgent(agentID uint) error {
	agent, err := s.agentSvc.Get(agentID)
	if err != nil {
		return ErrAgentNotFound
	}
	if !agent.IsActive {
		return ErrAgentNotFound
	}
	return nil
}

func (s *EngineConfigService) validateActiveModel(modelID uint) error {
	m, err := s.modelRepo.FindByID(modelID)
	if err != nil || m.Status != ai.ModelStatusActive || m.Provider == nil || m.Provider.Status != ai.ProviderStatusActive {
		return ErrModelNotFound
	}
	return nil
}

func (s *EngineConfigService) validateFallbackAssignee(userID uint) error {
	var user struct{ IsActive bool }
	if err := s.db.Table("users").Where("id = ? AND deleted_at IS NULL", userID).
		Select("is_active").First(&user).Error; err != nil {
		return ErrFallbackUserNotFound
	}
	if !user.IsActive {
		return ErrFallbackUserNotFound
	}
	return nil
}

func (s *EngineConfigService) getConfigValue(key, defaultVal string) string {
	cfg, err := s.sysConfigRepo.Get(key)
	if err != nil {
		return defaultVal
	}
	if cfg.Value == "" {
		return defaultVal
	}
	return cfg.Value
}

func (s *EngineConfigService) getConfigInt(key string, defaultVal int) int {
	v := s.getConfigValue(key, "")
	if v == "" {
		return defaultVal
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return defaultVal
	}
	return n
}

func (s *EngineConfigService) getConfigFloat(key string, defaultVal float64) float64 {
	v := s.getConfigValue(key, "")
	if v == "" {
		return defaultVal
	}
	n, err := strconv.ParseFloat(v, 64)
	if err != nil {
		return defaultVal
	}
	return n
}

func (s *EngineConfigService) setConfigValue(key, value string) {
	cfg, err := s.sysConfigRepo.Get(key)
	if err != nil {
		cfg = &model.SystemConfig{Key: key}
	}
	cfg.Value = value
	_ = s.sysConfigRepo.Set(cfg)
}

func (s *EngineConfigService) buildSmartStaffingHealth(cfg *SmartStaffingConfig) EngineHealth {
	items := []EngineHealthItem{
		s.agentHealth("intake", "服务受理岗", cfg.Posts.Intake.AgentID, []string{
			"itsm.service_match",
			"itsm.service_load",
			"itsm.draft_prepare",
			"itsm.draft_confirm",
			"itsm.validate_participants",
			"itsm.ticket_create",
		}),
		s.agentHealth("decision", "流程决策岗", cfg.Posts.Decision.AgentID, []string{
			"decision.ticket_context",
			"decision.resolve_participant",
			"decision.sla_status",
			"decision.list_actions",
			"decision.execute_action",
		}),
		s.agentHealth("slaAssurance", "SLA 保障岗", cfg.Posts.SLAAssurance.AgentID, []string{
			"sla.risk_queue",
			"sla.ticket_context",
			"sla.escalation_rules",
			"sla.trigger_escalation",
			"sla.write_timeline",
		}),
	}
	return EngineHealth{Items: items}
}

func (s *EngineConfigService) buildEngineSettingsHealth(cfg *EngineSettingsConfig) EngineHealth {
	items := []EngineHealthItem{
		s.pathHealth(cfg.Runtime.PathBuilder),
		s.guardHealth(cfg.Runtime.Guard),
	}
	return EngineHealth{Items: items}
}

func (s *EngineConfigService) agentHealth(key, label string, agentID uint, requiredTools []string) EngineHealthItem {
	if agentID == 0 {
		return EngineHealthItem{Key: key, Label: label, Status: "fail", Message: label + "未上岗"}
	}
	agent, err := s.agentSvc.Get(agentID)
	if err != nil || !agent.IsActive {
		return EngineHealthItem{Key: key, Label: label, Status: "fail", Message: label + "上岗智能体不存在或未启用"}
	}
	if agent.ModelID == nil || *agent.ModelID == 0 {
		return EngineHealthItem{Key: key, Label: label, Status: "fail", Message: label + "上岗智能体未配置模型"}
	}
	if missing := s.missingAgentTools(agentID, requiredTools); len(missing) > 0 {
		return EngineHealthItem{Key: key, Label: label, Status: "fail", Message: label + "工具缺失：" + missing[0]}
	}
	if requiresTool(requiredTools, "itsm.service_match") {
		if s.toolRuntime == nil {
			return EngineHealthItem{Key: key, Label: label, Status: "fail", Message: "服务匹配 Tool 运行时不可用，请前往 AI Tools 配置"}
		}
		if _, err := s.toolRuntime.LLMRuntimeConfig("itsm.service_match"); err != nil {
			return EngineHealthItem{Key: key, Label: label, Status: "fail", Message: "服务匹配 Tool 未就绪，请前往 AI Tools 配置：" + err.Error()}
		}
	}
	return EngineHealthItem{Key: key, Label: label, Status: "pass", Message: label + "已上岗"}
}

func requiresTool(required []string, name string) bool {
	for _, item := range required {
		if item == name {
			return true
		}
	}
	return false
}

func (s *EngineConfigService) modelEngineHealth(key, label string, cfg EngineModelConfig, timeoutSeconds int) EngineHealthItem {
	if cfg.ModelID == 0 {
		return EngineHealthItem{Key: key, Label: label, Status: "fail", Message: label + "未配置模型"}
	}
	if err := s.validateActiveModel(cfg.ModelID); err != nil {
		return EngineHealthItem{Key: key, Label: label, Status: "fail", Message: label + "模型不存在或未启用"}
	}
	if timeoutSeconds <= 0 {
		return EngineHealthItem{Key: key, Label: label, Status: "fail", Message: label + "超时时间无效"}
	}
	return EngineHealthItem{Key: key, Label: label, Status: "pass", Message: label + "已就绪"}
}

func (s *EngineConfigService) pathHealth(path EnginePathConfig) EngineHealthItem {
	base := s.modelEngineHealth("pathBuilder", "参考路径生成", path.EngineModelConfig, path.TimeoutSeconds)
	if base.Status != "pass" {
		return base
	}
	if path.MaxRetries < 0 {
		return EngineHealthItem{Key: "pathBuilder", Label: "参考路径生成", Status: "fail", Message: "参考路径运行参数无效"}
	}
	return EngineHealthItem{Key: "pathBuilder", Label: "参考路径生成", Status: "pass", Message: "参考路径生成已就绪"}
}

func (s *EngineConfigService) guardHealth(guard EngineGuardConfig) EngineHealthItem {
	if guard.FallbackAssignee == 0 {
		return EngineHealthItem{Key: "guard", Label: "异常兜底与审计", Status: "warn", Message: "未指定兜底处理人，异常时只能进入人工处置队列"}
	}
	if err := s.validateFallbackAssignee(guard.FallbackAssignee); err != nil {
		return EngineHealthItem{Key: "guard", Label: "异常兜底与审计", Status: "fail", Message: "兜底处理人不存在或未启用"}
	}
	return EngineHealthItem{Key: "guard", Label: "异常兜底与审计", Status: "pass", Message: "异常兜底与审计已就绪"}
}

func (s *EngineConfigService) missingAgentTools(agentID uint, required []string) []string {
	if len(required) == 0 {
		return nil
	}
	var rows []struct{ Name string }
	if err := s.db.Table("ai_tools").
		Select("ai_tools.name").
		Joins("JOIN ai_agent_tools ON ai_agent_tools.tool_id = ai_tools.id").
		Where("ai_agent_tools.agent_id = ? AND ai_tools.name IN ? AND ai_tools.is_active = ?", agentID, required, true).
		Find(&rows).Error; err != nil {
		return required
	}
	have := map[string]bool{}
	for _, row := range rows {
		have[row.Name] = true
	}
	var missing []string
	for _, name := range required {
		if !have[name] {
			missing = append(missing, name)
		}
	}
	return missing
}

func validateDecisionMode(mode string) error {
	switch mode {
	case "direct_first", "ai_only":
		return nil
	default:
		return fmt.Errorf("%w: 流程决策岗模式无效", ErrInvalidEngineConfig)
	}
}

func validateAuditLevel(level string) error {
	switch level {
	case "full", "summary", "off":
		return nil
	default:
		return fmt.Errorf("%w: 审计级别无效", ErrInvalidEngineConfig)
	}
}

func validateTemperature(label string, value float64) error {
	if value < 0 || value > 1 {
		return fmt.Errorf("%w: %s温度必须在 0 到 1 之间", ErrInvalidEngineConfig, label)
	}
	return nil
}

func formatFloat(value float64) string {
	return strconv.FormatFloat(value, 'f', -1, 64)
}

func (s *EngineConfigService) FallbackAssigneeID() uint {
	return uint(s.getConfigInt(SmartTicketGuardFallbackKey, 0))
}

func (s *EngineConfigService) DecisionMode() string {
	return s.getConfigValue(SmartTicketDecisionModeKey, "direct_first")
}

func (s *EngineConfigService) DecisionAgentID() uint {
	return uint(s.getConfigInt(SmartTicketDecisionAgentKey, 0))
}

func (s *EngineConfigService) SLAAssuranceAgentID() uint {
	return uint(s.getConfigInt(SmartTicketSLAAssuranceAgentKey, 0))
}

func (s *EngineConfigService) IntakeAgentID() uint {
	return uint(s.getConfigInt(SmartTicketIntakeAgentKey, 0))
}

func (s *EngineConfigService) AuditLevel() string {
	return s.getConfigValue(SmartTicketGuardAuditLevelKey, "full")
}

func (s *EngineConfigService) SLACriticalThresholdSeconds() int {
	return 1800
}

func (s *EngineConfigService) SLAWarningThresholdSeconds() int {
	return 3600
}

func (s *EngineConfigService) SimilarHistoryLimit() int {
	return 5
}

func (s *EngineConfigService) ParallelConvergenceTimeout() time.Duration {
	return 72 * time.Hour
}
