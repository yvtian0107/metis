package sla

import (
	"encoding/json"
	"errors"
	"fmt"
	. "metis/internal/app/itsm/domain"
	"strings"

	"github.com/samber/do/v2"
	"gorm.io/gorm"

	"metis/internal/database"
	"metis/internal/model"
	"metis/internal/service"
)

var (
	ErrEscalationRuleNotFound = errors.New("escalation rule not found")
	ErrEscalationLevelExists  = errors.New("escalation level already exists for this SLA and trigger type")
	ErrEscalationTargetConfig = errors.New("invalid escalation target config")
)

type EscalationRuleService struct {
	repo *EscalationRuleRepo
	db   *database.DB
}

func NewEscalationRuleService(i do.Injector) (*EscalationRuleService, error) {
	repo := do.MustInvoke[*EscalationRuleRepo](i)
	db := do.MustInvoke[*database.DB](i)
	return &EscalationRuleService{repo: repo, db: db}, nil
}

func (s *EscalationRuleService) Create(rule *EscalationRule) (*EscalationRule, error) {
	if err := validateEscalationTargetConfig(rule.ActionType, rule.TargetConfig); err != nil {
		return nil, err
	}
	if err := s.validateEscalationTargetReferences(rule.ActionType, rule.TargetConfig); err != nil {
		return nil, err
	}
	if _, err := s.repo.FindBySLATriggerLevel(rule.SLAID, rule.TriggerType, rule.Level); err == nil {
		return nil, ErrEscalationLevelExists
	}
	rule.IsActive = true
	if err := s.repo.Create(rule); err != nil {
		return nil, err
	}
	return s.repo.FindByID(rule.ID)
}

func (s *EscalationRuleService) Get(id uint) (*EscalationRule, error) {
	r, err := s.repo.FindByID(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrEscalationRuleNotFound
		}
		return nil, err
	}
	return r, nil
}

func (s *EscalationRuleService) Update(id uint, updates map[string]any) (*EscalationRule, error) {
	existing, err := s.repo.FindByID(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrEscalationRuleNotFound
		}
		return nil, err
	}
	candidate := *existing
	if v, ok := updates["trigger_type"].(string); ok {
		candidate.TriggerType = v
	}
	if v, ok := updates["level"].(int); ok {
		candidate.Level = v
	}
	if v, ok := updates["wait_minutes"].(int); ok {
		candidate.WaitMinutes = v
	}
	if v, ok := updates["action_type"].(string); ok {
		candidate.ActionType = v
	}
	if v, ok := updates["target_config"].(JSONField); ok {
		candidate.TargetConfig = v
	}
	if err := validateEscalationTargetConfig(candidate.ActionType, candidate.TargetConfig); err != nil {
		return nil, err
	}
	if err := s.validateEscalationTargetReferences(candidate.ActionType, candidate.TargetConfig); err != nil {
		return nil, err
	}
	if candidate.TriggerType != existing.TriggerType || candidate.Level != existing.Level {
		if other, err := s.repo.FindBySLATriggerLevel(candidate.SLAID, candidate.TriggerType, candidate.Level); err == nil && other.ID != id {
			return nil, ErrEscalationLevelExists
		}
	}
	if err := s.repo.Update(id, updates); err != nil {
		return nil, err
	}
	return s.repo.FindByID(id)
}

func (s *EscalationRuleService) Delete(id uint) error {
	if _, err := s.repo.FindByID(id); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrEscalationRuleNotFound
		}
		return err
	}
	return s.repo.Delete(id)
}

func (s *EscalationRuleService) ListBySLA(slaID uint) ([]EscalationRule, error) {
	return s.repo.ListBySLA(slaID)
}

type escalationParticipantConfig struct {
	Type           string `json:"type"`
	Value          string `json:"value,omitempty"`
	PositionCode   string `json:"position_code,omitempty"`
	DepartmentCode string `json:"department_code,omitempty"`
}

type escalationTargetConfigPayload struct {
	Recipients         []escalationParticipantConfig `json:"recipients,omitempty"`
	AssigneeCandidates []escalationParticipantConfig `json:"assigneeCandidates,omitempty"`
	ChannelID          uint                          `json:"channelId,omitempty"`
	SubjectTemplate    string                        `json:"subjectTemplate,omitempty"`
	BodyTemplate       string                        `json:"bodyTemplate,omitempty"`
	PriorityID         uint                          `json:"priorityId,omitempty"`
}

func validateEscalationTargetConfig(actionType string, raw JSONField) error {
	if strings.TrimSpace(string(raw)) == "" || strings.TrimSpace(string(raw)) == "null" {
		return fmt.Errorf("%w: targetConfig is required", ErrEscalationTargetConfig)
	}
	var cfg escalationTargetConfigPayload
	if err := json.Unmarshal([]byte(raw), &cfg); err != nil {
		return fmt.Errorf("%w: %v", ErrEscalationTargetConfig, err)
	}
	switch actionType {
	case "notify":
		if len(cfg.Recipients) == 0 {
			return fmt.Errorf("%w: notify requires recipients", ErrEscalationTargetConfig)
		}
		if cfg.ChannelID == 0 {
			return fmt.Errorf("%w: notify requires channelId", ErrEscalationTargetConfig)
		}
		for i, p := range cfg.Recipients {
			if err := validateEscalationParticipant(p); err != nil {
				return fmt.Errorf("%w: recipients[%d] %v", ErrEscalationTargetConfig, i, err)
			}
		}
	case "reassign":
		if len(cfg.AssigneeCandidates) == 0 {
			return fmt.Errorf("%w: reassign requires assigneeCandidates", ErrEscalationTargetConfig)
		}
		for i, p := range cfg.AssigneeCandidates {
			if err := validateEscalationParticipant(p); err != nil {
				return fmt.Errorf("%w: assigneeCandidates[%d] %v", ErrEscalationTargetConfig, i, err)
			}
		}
	case "escalate_priority":
		if cfg.PriorityID == 0 {
			return fmt.Errorf("%w: escalate_priority requires priorityId", ErrEscalationTargetConfig)
		}
	default:
		return fmt.Errorf("%w: unsupported actionType %s", ErrEscalationTargetConfig, actionType)
	}
	return nil
}

func validateEscalationParticipant(p escalationParticipantConfig) error {
	switch p.Type {
	case "user", "position", "department":
		if strings.TrimSpace(p.Value) == "" {
			return errors.New("requires value")
		}
	case "position_department":
		if strings.TrimSpace(p.PositionCode) == "" || strings.TrimSpace(p.DepartmentCode) == "" {
			return errors.New("requires position_code and department_code")
		}
	case "requester", "requester_manager":
		return nil
	default:
		return fmt.Errorf("unsupported participant type %s", p.Type)
	}
	return nil
}

func (s *EscalationRuleService) validateEscalationTargetReferences(actionType string, raw JSONField) error {
	var cfg escalationTargetConfigPayload
	if err := json.Unmarshal([]byte(raw), &cfg); err != nil {
		return fmt.Errorf("%w: %v", ErrEscalationTargetConfig, err)
	}
	switch actionType {
	case "notify":
		var channel model.MessageChannel
		if err := s.db.Where("id = ? AND enabled = ?", cfg.ChannelID, true).First(&channel).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return fmt.Errorf("%w: %v", ErrEscalationTargetConfig, service.ErrChannelNotFound)
			}
			return err
		}
	case "escalate_priority":
		var priority Priority
		if err := s.db.Where("id = ? AND is_active = ?", cfg.PriorityID, true).First(&priority).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return fmt.Errorf("%w: priority not found", ErrEscalationTargetConfig)
			}
			return err
		}
	}
	return nil
}
