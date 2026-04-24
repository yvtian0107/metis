package form

import (
	"encoding/json"
	"fmt"
	"net/mail"
	"net/url"
	"regexp"
	"strings"
)

// FieldValidationError represents a validation error for a specific field.
type FieldValidationError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

// ValidateFormData validates user-submitted data against the form schema.
// It checks each field's validation rules and returns all errors found.
func ValidateFormData(schema FormSchema, data map[string]any) []FieldValidationError {
	var errs []FieldValidationError

	for _, field := range schema.Fields {
		val, exists := data[field.Key]
		isEmpty := !exists || isEmptyFormValue(val)

		// Check required (from field.Required flag or validation rules)
		isRequired := field.Required
		for _, rule := range field.Validation {
			if rule.Rule == "required" {
				isRequired = true
				break
			}
		}

		if isRequired && isEmpty {
			msg := "此字段为必填项"
			for _, rule := range field.Validation {
				if rule.Rule == "required" && rule.Message != "" {
					msg = rule.Message
					break
				}
			}
			errs = append(errs, FieldValidationError{Field: field.Key, Message: msg})
			continue // skip further checks if required field is missing
		}

		if isEmpty {
			continue // optional field with no value — skip validation
		}

		if err := validateFieldValue(field, val, field.Key); err != nil {
			errs = append(errs, *err)
			continue
		}

		// Apply each validation rule
		for _, rule := range field.Validation {
			if rule.Rule == "required" {
				continue // already handled above
			}
			if err := applyRule(field.Key, rule, val); err != nil {
				errs = append(errs, *err)
			}
		}
	}

	return errs
}

func validateFieldValue(field FormField, val any, path string) *FieldValidationError {
	switch field.Type {
	case FieldText, FieldTextarea, FieldEmail, FieldURL, FieldSelect, FieldRadio,
		FieldDate, FieldDatetime, FieldUserPicker, FieldDeptPicker, FieldRichText:
		s, ok := val.(string)
		if !ok {
			return &FieldValidationError{Field: path, Message: fmt.Sprintf("%s 必须是字符串", field.Label)}
		}
		if (field.Type == FieldSelect || field.Type == FieldRadio) && !optionContains(field.Options, s) {
			return &FieldValidationError{Field: path, Message: fmt.Sprintf("%s 的值 %q 不在可选项中", field.Label, s)}
		}
	case FieldNumber:
		if !isNumber(val) {
			return &FieldValidationError{Field: path, Message: fmt.Sprintf("%s 必须是数字", field.Label)}
		}
	case FieldSwitch:
		if _, ok := val.(bool); !ok {
			return &FieldValidationError{Field: path, Message: fmt.Sprintf("%s 必须是布尔值", field.Label)}
		}
	case FieldCheckbox:
		if len(field.Options) == 0 {
			if _, ok := val.(bool); !ok {
				return &FieldValidationError{Field: path, Message: fmt.Sprintf("%s 必须是布尔值", field.Label)}
			}
			return nil
		}
		values, ok := toStringSlice(val)
		if !ok {
			return &FieldValidationError{Field: path, Message: fmt.Sprintf("%s 必须是字符串数组", field.Label)}
		}
		for _, item := range values {
			if !optionContains(field.Options, item) {
				return &FieldValidationError{Field: path, Message: fmt.Sprintf("%s 的值 %q 不在可选项中", field.Label, item)}
			}
		}
	case FieldMultiSelect:
		values, ok := toStringSlice(val)
		if !ok {
			return &FieldValidationError{Field: path, Message: fmt.Sprintf("%s 必须是字符串数组", field.Label)}
		}
		for _, item := range values {
			if !optionContains(field.Options, item) {
				return &FieldValidationError{Field: path, Message: fmt.Sprintf("%s 的值 %q 不在可选项中", field.Label, item)}
			}
		}
	case FieldDateRange:
		r, ok := toStringMap(val)
		if !ok {
			return &FieldValidationError{Field: path, Message: fmt.Sprintf("%s 必须是包含 start/end 的对象", field.Label)}
		}
		start, startOK := r["start"].(string)
		end, endOK := r["end"].(string)
		if !startOK || !endOK || strings.TrimSpace(start) == "" || strings.TrimSpace(end) == "" {
			return &FieldValidationError{Field: path, Message: fmt.Sprintf("%s 必须包含 start 和 end", field.Label)}
		}
	case FieldTable:
		rows, ok := toRowSlice(val)
		if !ok {
			return &FieldValidationError{Field: path, Message: fmt.Sprintf("%s 必须是行数组", field.Label)}
		}
		columns, err := TableColumns(field)
		if err != nil {
			return &FieldValidationError{Field: path, Message: err.Error()}
		}
		for i, row := range rows {
			if isEmptyFormValue(row) {
				return &FieldValidationError{Field: fmt.Sprintf("%s[%d]", path, i), Message: fmt.Sprintf("%s 包含空行", field.Label)}
			}
			for _, col := range columns {
				colField := FormField{
					Key:         col.Key,
					Type:        col.Type,
					Label:       col.Label,
					Placeholder: col.Placeholder,
					Required:    col.Required,
					Validation:  col.Validation,
					Options:     col.Options,
				}
				colVal, exists := row[col.Key]
				colEmpty := !exists || isEmptyFormValue(colVal)
				colRequired := isRequiredField(colField)
				colPath := fmt.Sprintf("%s[%d].%s", path, i, col.Key)
				if colRequired && colEmpty {
					return &FieldValidationError{Field: colPath, Message: requiredMessage(colField)}
				}
				if colEmpty {
					continue
				}
				if err := validateFieldValue(colField, colVal, colPath); err != nil {
					return err
				}
				for _, rule := range col.Validation {
					if rule.Rule == "required" {
						continue
					}
					if err := applyRule(colPath, rule, colVal); err != nil {
						return err
					}
				}
			}
		}
	}
	return nil
}

func isRequiredField(field FormField) bool {
	if field.Required {
		return true
	}
	for _, rule := range field.Validation {
		if rule.Rule == "required" {
			return true
		}
	}
	return false
}

func requiredMessage(field FormField) string {
	for _, rule := range field.Validation {
		if rule.Rule == "required" && rule.Message != "" {
			return rule.Message
		}
	}
	return "此字段为必填项"
}

func isEmptyFormValue(val any) bool {
	if val == nil {
		return true
	}
	switch v := val.(type) {
	case string:
		return strings.TrimSpace(v) == ""
	case []string:
		return len(v) == 0
	case []any:
		return len(v) == 0
	case map[string]any:
		return len(v) == 0 || allMapValuesEmpty(v)
	default:
		return false
	}
}

func allMapValuesEmpty(m map[string]any) bool {
	if len(m) == 0 {
		return true
	}
	for _, val := range m {
		if !isEmptyFormValue(val) {
			return false
		}
	}
	return true
}

func toStringSlice(val any) ([]string, bool) {
	switch v := val.(type) {
	case []string:
		return v, true
	case []any:
		out := make([]string, 0, len(v))
		for _, item := range v {
			s, ok := item.(string)
			if !ok {
				return nil, false
			}
			out = append(out, s)
		}
		return out, true
	default:
		return nil, false
	}
}

func toStringMap(val any) (map[string]any, bool) {
	switch v := val.(type) {
	case map[string]any:
		return v, true
	default:
		b, err := json.Marshal(v)
		if err != nil {
			return nil, false
		}
		var out map[string]any
		if err := json.Unmarshal(b, &out); err != nil {
			return nil, false
		}
		return out, true
	}
}

func toRowSlice(val any) ([]map[string]any, bool) {
	switch v := val.(type) {
	case []map[string]any:
		return v, true
	case []any:
		out := make([]map[string]any, 0, len(v))
		for _, item := range v {
			row, ok := item.(map[string]any)
			if !ok {
				return nil, false
			}
			out = append(out, row)
		}
		return out, true
	default:
		return nil, false
	}
}

func isNumber(val any) bool {
	switch val.(type) {
	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64, float32, float64, json_number:
		return true
	default:
		return false
	}
}

func optionContains(options []FieldOption, value string) bool {
	if len(options) == 0 {
		return true
	}
	for _, opt := range options {
		if fmt.Sprintf("%v", opt.Value) == value {
			return true
		}
	}
	return false
}

func applyRule(key string, rule ValidationRule, val any) *FieldValidationError {
	switch rule.Rule {
	case "minLength":
		s := toString(val)
		limit := toInt(rule.Value)
		if len([]rune(s)) < limit {
			return &FieldValidationError{Field: key, Message: ruleMessage(rule, fmt.Sprintf("长度不能少于 %d 个字符", limit))}
		}
	case "maxLength":
		s := toString(val)
		limit := toInt(rule.Value)
		if len([]rune(s)) > limit {
			return &FieldValidationError{Field: key, Message: ruleMessage(rule, fmt.Sprintf("长度不能超过 %d 个字符", limit))}
		}
	case "min":
		n := toFloat(val)
		limit := toFloat(rule.Value)
		if n < limit {
			return &FieldValidationError{Field: key, Message: ruleMessage(rule, fmt.Sprintf("值不能小于 %v", rule.Value))}
		}
	case "max":
		n := toFloat(val)
		limit := toFloat(rule.Value)
		if n > limit {
			return &FieldValidationError{Field: key, Message: ruleMessage(rule, fmt.Sprintf("值不能大于 %v", rule.Value))}
		}
	case "pattern":
		s := toString(val)
		pattern := toString(rule.Value)
		re, err := regexp.Compile(pattern)
		if err != nil {
			return &FieldValidationError{Field: key, Message: fmt.Sprintf("无效的正则表达式: %s", pattern)}
		}
		if !re.MatchString(s) {
			return &FieldValidationError{Field: key, Message: ruleMessage(rule, "格式不正确")}
		}
	case "email":
		s := toString(val)
		if _, err := mail.ParseAddress(s); err != nil {
			return &FieldValidationError{Field: key, Message: ruleMessage(rule, "请输入有效的邮箱地址")}
		}
	case "url":
		s := toString(val)
		u, err := url.ParseRequestURI(s)
		if err != nil || (u.Scheme != "http" && u.Scheme != "https") {
			return &FieldValidationError{Field: key, Message: ruleMessage(rule, "请输入有效的 URL")}
		}
	}
	return nil
}

func ruleMessage(rule ValidationRule, fallback string) string {
	if rule.Message != "" {
		return rule.Message
	}
	return fallback
}

func toString(v any) string {
	if v == nil {
		return ""
	}
	switch s := v.(type) {
	case string:
		return s
	default:
		return fmt.Sprintf("%v", v)
	}
}

func toFloat(v any) float64 {
	switch n := v.(type) {
	case float64:
		return n
	case float32:
		return float64(n)
	case int:
		return float64(n)
	case int64:
		return float64(n)
	case json_number:
		f, _ := n.Float64()
		return f
	default:
		return 0
	}
}

// json_number is a type alias to avoid importing encoding/json just for json.Number
type json_number = interface{ Float64() (float64, error) }

func toInt(v any) int {
	return int(toFloat(v))
}
