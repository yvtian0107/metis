package engine

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"metis/internal/app/itsm/form"
)

// processVariableModel is the engine-local alias for itsm_process_variables.
type processVariableModel struct {
	ID        uint      `gorm:"primaryKey;autoIncrement"`
	TicketID  uint      `gorm:"column:ticket_id;not null"`
	ScopeID   string    `gorm:"column:scope_id;size:64;not null;default:root"`
	Key       string    `gorm:"column:key;size:128;not null"`
	Value     string    `gorm:"column:value;type:text"`
	ValueType string    `gorm:"column:value_type;size:16;not null;default:string"`
	Source    string    `gorm:"column:source;size:128"`
	CreatedAt time.Time `gorm:"column:created_at;autoCreateTime"`
	UpdatedAt time.Time `gorm:"column:updated_at;autoUpdateTime"`
}

func (processVariableModel) TableName() string { return "itsm_process_variables" }

// writeFormBindings parses the form schema for fields with binding, extracts
// values from formData, validates them, and upserts them as process variables.
// currentNodeID is used for field-level permission checks (empty = skip checks).
func writeFormBindings(tx *gorm.DB, ticketID uint, scopeID string, formSchemaJSON string, formDataJSON string, source string, currentNodeID string) error {
	if formSchemaJSON == "" || formDataJSON == "" {
		return nil
	}

	// Parse full form schema for validation
	var fullSchema form.FormSchema
	if err := json.Unmarshal([]byte(formSchemaJSON), &fullSchema); err != nil {
		return nil // non-fatal: schema is malformed, skip binding
	}

	// Parse form data with UseNumber to preserve numeric precision
	var formData map[string]any
	dec := json.NewDecoder(strings.NewReader(formDataJSON))
	dec.UseNumber()
	if err := dec.Decode(&formData); err != nil {
		return nil // non-fatal
	}

	// Validate form data against schema
	if validationErrors := form.ValidateFormData(fullSchema, formData); len(validationErrors) > 0 {
		slog.Warn("form validation failed, skipping variable write",
			"ticketID", ticketID, "source", source, "errors", validationErrors)
		return &FormValidationError{Errors: validationErrors}
	}

	for _, field := range fullSchema.Fields {
		if field.Binding == "" {
			continue
		}

		// Check field-level permission: skip readonly/hidden fields
		if currentNodeID != "" && field.Permissions != nil {
			if perm, ok := field.Permissions[currentNodeID]; ok && (perm == "readonly" || perm == "hidden") {
				slog.Warn("field write skipped due to permission",
					"ticketID", ticketID, "field", field.Key, "nodeID", currentNodeID, "permission", perm)
				continue
			}
		}

		val, exists := formData[field.Key]
		if !exists {
			continue
		}
		if isEmptyFormValue(val) {
			continue
		}

		valueType := fieldTypeToValueType(field.Type, len(field.Options) > 0)
		serialized := serializeVarValue(val)

		v := processVariableModel{
			TicketID:  ticketID,
			ScopeID:   scopeID,
			Key:       field.Binding,
			Value:     serialized,
			ValueType: valueType,
			Source:    source,
		}
		if err := tx.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "ticket_id"}, {Name: "scope_id"}, {Name: "key"}},
			DoUpdates: clause.AssignmentColumns([]string{"value", "value_type", "source", "updated_at"}),
		}).Create(&v).Error; err != nil {
			return fmt.Errorf("write binding %s: %w", field.Binding, err)
		}
	}
	return nil
}

// FormValidationError wraps form field validation errors.
type FormValidationError struct {
	Errors []form.FieldValidationError
}

func (e *FormValidationError) Error() string {
	return fmt.Sprintf("form validation failed: %d field(s) invalid", len(e.Errors))
}

func isEmptyFormValue(val any) bool {
	if val == nil {
		return true
	}
	switch v := val.(type) {
	case string:
		return v == ""
	case []any:
		return len(v) == 0
	case map[string]any:
		return len(v) == 0
	default:
		return false
	}
}

// fieldTypeToValueType maps a form field type to a variable value_type.
func fieldTypeToValueType(fieldType string, hasOptions bool) string {
	switch fieldType {
	case "text", "textarea", "email", "url", "select", "radio", "rich_text",
		"user_picker", "dept_picker":
		return "string"
	case "number":
		return "number"
	case "switch":
		return "boolean"
	case "checkbox":
		if hasOptions {
			return "json"
		}
		return "boolean"
	case "date", "datetime":
		return "date"
	case "multi_select", "date_range", "table":
		return "json"
	default:
		return "string"
	}
}

// serializeVarValue converts a Go value to a string for storage.
func serializeVarValue(val any) string {
	if val == nil {
		return ""
	}
	switch v := val.(type) {
	case string:
		return v
	case json.Number:
		return v.String()
	case bool:
		return fmt.Sprintf("%v", v)
	default:
		b, err := json.Marshal(v)
		if err != nil {
			return fmt.Sprintf("%v", v)
		}
		return string(b)
	}
}
