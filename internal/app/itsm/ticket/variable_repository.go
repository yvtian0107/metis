package ticket

import (
	"github.com/samber/do/v2"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	. "metis/internal/app/itsm/domain"

	"metis/internal/database"
)

// VariableRepository handles CRUD for process variables.
type VariableRepository struct {
	db *database.DB
}

func NewVariableRepository(i do.Injector) (*VariableRepository, error) {
	db := do.MustInvoke[*database.DB](i)
	return &VariableRepository{db: db}, nil
}

// SetVariable upserts a process variable (insert or update on conflict).
func (r *VariableRepository) SetVariable(tx *gorm.DB, v *ProcessVariable) error {
	if tx == nil {
		tx = r.db.DB
	}
	return tx.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "ticket_id"}, {Name: "scope_id"}, {Name: "key"}},
		DoUpdates: clause.AssignmentColumns([]string{"value", "value_type", "source", "updated_at"}),
	}).Create(v).Error
}

// GetVariable fetches a single variable by ticket + scope + key.
func (r *VariableRepository) GetVariable(ticketID uint, scopeID, key string) (*ProcessVariable, error) {
	var v ProcessVariable
	err := r.db.Where("ticket_id = ? AND scope_id = ? AND key = ?", ticketID, scopeID, key).First(&v).Error
	if err != nil {
		return nil, err
	}
	return &v, nil
}

// ListByTicket returns all variables for a given ticket.
func (r *VariableRepository) ListByTicket(ticketID uint) ([]ProcessVariable, error) {
	var vars []ProcessVariable
	err := r.db.Where("ticket_id = ?", ticketID).Order("key ASC").Find(&vars).Error
	return vars, err
}

// DeleteByTicket removes all variables for a given ticket.
func (r *VariableRepository) DeleteByTicket(tx *gorm.DB, ticketID uint) error {
	if tx == nil {
		tx = r.db.DB
	}
	return tx.Where("ticket_id = ?", ticketID).Delete(&ProcessVariable{}).Error
}

// DB exposes the underlying gorm.DB for transaction support.
func (r *VariableRepository) DB() *gorm.DB { return r.db.DB }
