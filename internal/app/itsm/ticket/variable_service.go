package ticket

import (
	"encoding/json"
	"fmt"
	. "metis/internal/app/itsm/domain"

	"github.com/samber/do/v2"
	"gorm.io/gorm"

	"metis/internal/app/itsm/form"
)

// VariableService provides business logic for process variables.
type VariableService struct {
	repo *VariableRepository
}

func NewVariableService(i do.Injector) (*VariableService, error) {
	repo := do.MustInvoke[*VariableRepository](i)
	return &VariableService{repo: repo}, nil
}

// SetVariable validates the value_type and persists the variable.
func (s *VariableService) SetVariable(tx *gorm.DB, v *ProcessVariable) error {
	switch v.ValueType {
	case ValueTypeString, ValueTypeNumber, ValueTypeBoolean, ValueTypeJSON, ValueTypeDate:
		// valid
	default:
		return fmt.Errorf("unsupported value_type: %s", v.ValueType)
	}
	return s.repo.SetVariable(tx, v)
}

// GetVariable retrieves a single variable.
func (s *VariableService) GetVariable(ticketID uint, scopeID, key string) (*ProcessVariable, error) {
	return s.repo.GetVariable(ticketID, scopeID, key)
}

// ListByTicket returns all variables for a ticket.
func (s *VariableService) ListByTicket(ticketID uint) ([]ProcessVariable, error) {
	return s.repo.ListByTicket(ticketID)
}

// BulkSet writes multiple variables in a batch (used for form binding).
func (s *VariableService) BulkSet(tx *gorm.DB, vars []ProcessVariable) error {
	for i := range vars {
		if err := s.repo.SetVariable(tx, &vars[i]); err != nil {
			return err
		}
	}
	return nil
}

// InferValueType maps a form field type to a variable value_type.
// hasOptions indicates whether the field has predefined options (for checkbox disambiguation).
func InferValueType(fieldType string, hasOptions bool) string {
	switch fieldType {
	case form.FieldText, form.FieldTextarea, form.FieldEmail, form.FieldURL,
		form.FieldSelect, form.FieldRadio, form.FieldRichText,
		form.FieldUserPicker, form.FieldDeptPicker:
		return ValueTypeString
	case form.FieldNumber:
		return ValueTypeNumber
	case form.FieldSwitch:
		return ValueTypeBoolean
	case form.FieldCheckbox:
		if hasOptions {
			return ValueTypeJSON
		}
		return ValueTypeBoolean
	case form.FieldDate, form.FieldDatetime:
		return ValueTypeDate
	case form.FieldMultiSelect, form.FieldDateRange, form.FieldTable:
		return ValueTypeJSON
	default:
		return ValueTypeString
	}
}

// SerializeValue converts a Go value to a string for storage.
func SerializeValue(val any) string {
	if val == nil {
		return ""
	}
	switch v := val.(type) {
	case string:
		return v
	case json.Number:
		return v.String()
	case float64:
		return fmt.Sprintf("%v", v)
	case bool:
		return fmt.Sprintf("%v", v)
	default:
		// For slices, maps, etc. — marshal to JSON.
		b, err := json.Marshal(v)
		if err != nil {
			return fmt.Sprintf("%v", v)
		}
		return string(b)
	}
}
