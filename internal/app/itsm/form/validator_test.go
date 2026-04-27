package form

import "testing"

func TestValidateFormDataRules(t *testing.T) {
	schema := FormSchema{
		Version: 1,
		Fields: []FormField{
			{Key: "title", Type: FieldText, Label: "标题", Required: true, Validation: []ValidationRule{{Rule: "required", Message: "请输入标题"}, {Rule: "minLength", Value: 3, Message: "标题太短"}}},
			{Key: "age", Type: FieldNumber, Label: "年龄", Validation: []ValidationRule{{Rule: "min", Value: 18, Message: "年龄太小"}, {Rule: "max", Value: 60, Message: "年龄太大"}}},
			{Key: "code", Type: FieldText, Label: "编码", Validation: []ValidationRule{{Rule: "pattern", Value: `^[A-Z]+$`, Message: "编码必须大写"}}},
			{Key: "email", Type: FieldEmail, Label: "邮箱", Validation: []ValidationRule{{Rule: "email", Message: "邮箱格式错误"}}},
			{Key: "homepage", Type: FieldURL, Label: "主页", Validation: []ValidationRule{{Rule: "url", Message: "URL格式错误"}}},
			{Key: "optional", Type: FieldText, Label: "选填"},
		},
	}

	errs := ValidateFormData(schema, map[string]any{
		"title":    "",
		"age":      17,
		"code":     "abc",
		"email":    "bad",
		"homepage": "ftp://example.test",
		"optional": "",
	})
	got := map[string]string{}
	for _, err := range errs {
		got[err.Field] = err.Message
	}

	want := map[string]string{
		"title":    "请输入标题",
		"age":      "年龄太小",
		"code":     "编码必须大写",
		"email":    "邮箱格式错误",
		"homepage": "URL格式错误",
	}
	if len(got) != len(want) {
		t.Fatalf("errors = %+v, want %+v", got, want)
	}
	for field, message := range want {
		if got[field] != message {
			t.Fatalf("field %s message = %q, want %q; all=%+v", field, got[field], message, got)
		}
	}
	if _, ok := got["optional"]; ok {
		t.Fatalf("optional empty field should not fail: %+v", got)
	}
}

func TestValidateFormDataAcceptsValidValues(t *testing.T) {
	schema := FormSchema{
		Version: 1,
		Fields: []FormField{
			{Key: "title", Type: FieldText, Label: "标题", Required: true, Validation: []ValidationRule{{Rule: "maxLength", Value: 8, Message: "标题太长"}}},
			{Key: "age", Type: FieldNumber, Label: "年龄", Validation: []ValidationRule{{Rule: "min", Value: 18, Message: "年龄太小"}, {Rule: "max", Value: 60, Message: "年龄太大"}}},
			{Key: "email", Type: FieldEmail, Label: "邮箱", Validation: []ValidationRule{{Rule: "email", Message: "邮箱格式错误"}}},
			{Key: "homepage", Type: FieldURL, Label: "主页", Validation: []ValidationRule{{Rule: "url", Message: "URL格式错误"}}},
		},
	}

	errs := ValidateFormData(schema, map[string]any{
		"title":    "VPN",
		"age":      30,
		"email":    "ops@example.test",
		"homepage": "https://example.test",
	})
	if len(errs) != 0 {
		t.Fatalf("expected valid data, got %+v", errs)
	}
}

func TestValidateFormDataStructuredValues(t *testing.T) {
	schema := FormSchema{
		Version: 1,
		Fields: []FormField{
			{Key: "tags", Type: FieldMultiSelect, Label: "标签", Required: true, Options: []FieldOption{{Label: "VPN", Value: "vpn"}, {Label: "网络", Value: "network"}}},
			{Key: "agree", Type: FieldCheckbox, Label: "同意", Required: true},
			{Key: "systems", Type: FieldCheckbox, Label: "系统", Required: true, Options: []FieldOption{{Label: "ERP", Value: "erp"}}},
			{Key: "range", Type: FieldDateRange, Label: "日期范围", Required: true},
			{Key: "items", Type: FieldTable, Label: "明细", Required: true, Props: map[string]any{
				"columns": []TableColumn{
					{Key: "name", Type: FieldText, Label: "名称", Required: true},
					{Key: "kind", Type: FieldSelect, Label: "类型", Required: true, Options: []FieldOption{{Label: "网络", Value: "network"}}},
				},
			}},
		},
	}

	tests := []struct {
		name string
		data map[string]any
		want string
	}{
		{
			name: "empty array fails required",
			data: map[string]any{"tags": []any{}},
			want: "tags",
		},
		{
			name: "multi select must be array",
			data: map[string]any{"tags": "vpn", "agree": true, "systems": []any{"erp"}, "range": map[string]any{"start": "2026-01-01", "end": "2026-01-02"}, "items": []any{map[string]any{"name": "A", "kind": "network"}}},
			want: "tags",
		},
		{
			name: "multi select option membership",
			data: map[string]any{"tags": []any{"bad"}, "agree": true, "systems": []any{"erp"}, "range": map[string]any{"start": "2026-01-01", "end": "2026-01-02"}, "items": []any{map[string]any{"name": "A", "kind": "network"}}},
			want: "tags",
		},
		{
			name: "checkbox without options must be boolean",
			data: map[string]any{"tags": []any{"vpn"}, "agree": "yes", "systems": []any{"erp"}, "range": map[string]any{"start": "2026-01-01", "end": "2026-01-02"}, "items": []any{map[string]any{"name": "A", "kind": "network"}}},
			want: "agree",
		},
		{
			name: "checkbox with options must be array",
			data: map[string]any{"tags": []any{"vpn"}, "agree": true, "systems": "erp", "range": map[string]any{"start": "2026-01-01", "end": "2026-01-02"}, "items": []any{map[string]any{"name": "A", "kind": "network"}}},
			want: "systems",
		},
		{
			name: "date range requires start and end",
			data: map[string]any{"tags": []any{"vpn"}, "agree": true, "systems": []any{"erp"}, "range": map[string]any{"start": "2026-01-01"}, "items": []any{map[string]any{"name": "A", "kind": "network"}}},
			want: "range",
		},
		{
			name: "table must be row array",
			data: map[string]any{"tags": []any{"vpn"}, "agree": true, "systems": []any{"erp"}, "range": map[string]any{"start": "2026-01-01", "end": "2026-01-02"}, "items": map[string]any{"name": "A"}},
			want: "items",
		},
		{
			name: "table column required",
			data: map[string]any{"tags": []any{"vpn"}, "agree": true, "systems": []any{"erp"}, "range": map[string]any{"start": "2026-01-01", "end": "2026-01-02"}, "items": []any{map[string]any{"kind": "network"}}},
			want: "items[0].name",
		},
		{
			name: "table column option membership",
			data: map[string]any{"tags": []any{"vpn"}, "agree": true, "systems": []any{"erp"}, "range": map[string]any{"start": "2026-01-01", "end": "2026-01-02"}, "items": []any{map[string]any{"name": "A", "kind": "bad"}}},
			want: "items[0].kind",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errs := ValidateFormData(schema, tt.data)
			if len(errs) == 0 {
				t.Fatalf("expected validation error for %s", tt.want)
			}
			if errs[0].Field != tt.want {
				t.Fatalf("first error field = %s, want %s; all=%+v", errs[0].Field, tt.want, errs)
			}
		})
	}

	valid := map[string]any{
		"tags":    []any{"vpn", "network"},
		"agree":   true,
		"systems": []any{"erp"},
		"range":   map[string]any{"start": "2026-01-01", "end": "2026-01-02"},
		"items":   []any{map[string]any{"name": "A", "kind": "network"}},
	}
	if errs := ValidateFormData(schema, valid); len(errs) != 0 {
		t.Fatalf("expected structured values to be valid, got %+v", errs)
	}
}
