package form

import "testing"

func TestValidateSchemaStructuredFields(t *testing.T) {
	tests := []struct {
		name   string
		schema FormSchema
		field  string
	}{
		{
			name: "select requires options",
			schema: FormSchema{Version: 1, Fields: []FormField{
				{Key: "kind", Type: FieldSelect, Label: "类型"},
			}},
			field: "fields[0].options",
		},
		{
			name: "multi select duplicate option value",
			schema: FormSchema{Version: 1, Fields: []FormField{
				{Key: "tags", Type: FieldMultiSelect, Label: "标签", Options: []FieldOption{{Label: "A", Value: "a"}, {Label: "B", Value: "a"}}},
			}},
			field: "fields[0].options[1].value",
		},
		{
			name: "table requires columns",
			schema: FormSchema{Version: 1, Fields: []FormField{
				{Key: "items", Type: FieldTable, Label: "明细"},
			}},
			field: "fields[0].props.columns",
		},
		{
			name: "table rejects nested table",
			schema: FormSchema{Version: 1, Fields: []FormField{
				{Key: "items", Type: FieldTable, Label: "明细", Props: map[string]any{"columns": []TableColumn{
					{Key: "nested", Type: FieldTable, Label: "嵌套"},
				}}},
			}},
			field: "fields[0].props.columns[0].type",
		},
		{
			name: "table column select requires options",
			schema: FormSchema{Version: 1, Fields: []FormField{
				{Key: "items", Type: FieldTable, Label: "明细", Props: map[string]any{"columns": []TableColumn{
					{Key: "kind", Type: FieldSelect, Label: "类型"},
				}}},
			}},
			field: "fields[0].props.columns[0].options",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errs := ValidateSchema(tt.schema)
			if len(errs) == 0 {
				t.Fatal("expected schema validation error")
			}
			found := false
			for _, err := range errs {
				if err.Field == tt.field {
					found = true
					break
				}
			}
			if !found {
				t.Fatalf("expected error field %s, got %+v", tt.field, errs)
			}
		})
	}

	valid := FormSchema{Version: 1, Fields: []FormField{
		{Key: "items", Type: FieldTable, Label: "明细", Props: map[string]any{"columns": []TableColumn{
			{Key: "name", Type: FieldText, Label: "名称", Required: true},
			{Key: "kind", Type: FieldSelect, Label: "类型", Options: []FieldOption{{Label: "网络", Value: "network"}}},
		}}},
	}}
	if errs := ValidateSchema(valid); len(errs) != 0 {
		t.Fatalf("expected valid schema, got %+v", errs)
	}
}
