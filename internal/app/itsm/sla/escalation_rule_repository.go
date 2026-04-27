package sla

import (
	"github.com/samber/do/v2"
	. "metis/internal/app/itsm/domain"

	"metis/internal/database"
)

type EscalationRuleRepo struct {
	db *database.DB
}

func NewEscalationRuleRepo(i do.Injector) (*EscalationRuleRepo, error) {
	db := do.MustInvoke[*database.DB](i)
	return &EscalationRuleRepo{db: db}, nil
}

func (r *EscalationRuleRepo) Create(rule *EscalationRule) error {
	return r.db.Create(rule).Error
}

func (r *EscalationRuleRepo) FindByID(id uint) (*EscalationRule, error) {
	var e EscalationRule
	if err := r.db.First(&e, id).Error; err != nil {
		return nil, err
	}
	return &e, nil
}

func (r *EscalationRuleRepo) Update(id uint, updates map[string]any) error {
	return r.db.Model(&EscalationRule{}).Where("id = ?", id).Updates(updates).Error
}

func (r *EscalationRuleRepo) Delete(id uint) error {
	return r.db.Delete(&EscalationRule{}, id).Error
}

func (r *EscalationRuleRepo) ListBySLA(slaID uint) ([]EscalationRule, error) {
	var items []EscalationRule
	if err := r.db.Where("sla_id = ?", slaID).Order("level ASC").Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

func (r *EscalationRuleRepo) FindBySLATriggerLevel(slaID uint, triggerType string, level int) (*EscalationRule, error) {
	var e EscalationRule
	if err := r.db.Where("sla_id = ? AND trigger_type = ? AND level = ?", slaID, triggerType, level).First(&e).Error; err != nil {
		return nil, err
	}
	return &e, nil
}
