package engine

import (
	"fmt"
	"testing"
)

func TestEvaluateCondition_Equals(t *testing.T) {
	ctx := evalContext{"form.type": "vpn"}
	cond := GatewayCondition{Field: "form.type", Operator: "equals", Value: "vpn"}
	if !evaluateCondition(cond, ctx) {
		t.Error("equals should match")
	}
	cond.Value = "other"
	if evaluateCondition(cond, ctx) {
		t.Error("equals should not match different value")
	}
}

func TestEvaluateCondition_NotEquals(t *testing.T) {
	ctx := evalContext{"form.type": "vpn"}
	cond := GatewayCondition{Field: "form.type", Operator: "not_equals", Value: "other"}
	if !evaluateCondition(cond, ctx) {
		t.Error("not_equals should match")
	}
	cond.Value = "vpn"
	if evaluateCondition(cond, ctx) {
		t.Error("not_equals should not match same value")
	}
}

func TestEvaluateCondition_ContainsAny(t *testing.T) {
	ctx := evalContext{"form.type": "vpn_request"}
	// String contains
	cond := GatewayCondition{Field: "form.type", Operator: "contains_any", Value: "vpn"}
	if !evaluateCondition(cond, ctx) {
		t.Error("contains_any with string should match substring")
	}
	// Array contains
	cond.Value = []any{"vpn_request", "other"}
	if !evaluateCondition(cond, ctx) {
		t.Error("contains_any with array should match element")
	}
	cond.Value = []any{"other", "nope"}
	if evaluateCondition(cond, ctx) {
		t.Error("contains_any with array should not match")
	}
}

func TestEvaluateCondition_NumericComparisons(t *testing.T) {
	ctx := evalContext{"ticket.priority_id": float64(3)}

	tests := []struct {
		op   string
		val  any
		want bool
	}{
		{"gt", float64(2), true},
		{"gt", float64(3), false},
		{"lt", float64(4), true},
		{"lt", float64(3), false},
		{"gte", float64(3), true},
		{"gte", float64(4), false},
		{"lte", float64(3), true},
		{"lte", float64(2), false},
	}
	for _, tt := range tests {
		cond := GatewayCondition{Field: "ticket.priority_id", Operator: tt.op, Value: tt.val}
		got := evaluateCondition(cond, ctx)
		if got != tt.want {
			t.Errorf("%s %v: got %v, want %v", tt.op, tt.val, got, tt.want)
		}
	}
}

func TestEvaluateCondition_In(t *testing.T) {
	ctx := evalContext{"form.status": "open"}
	cond := GatewayCondition{Field: "form.status", Operator: "in", Value: []any{"open", "pending"}}
	if !evaluateCondition(cond, ctx) {
		t.Error("in should match")
	}
	cond.Value = []any{"closed", "resolved"}
	if evaluateCondition(cond, ctx) {
		t.Error("in should not match")
	}
}

func TestEvaluateCondition_NotIn(t *testing.T) {
	ctx := evalContext{"form.status": "open"}
	cond := GatewayCondition{Field: "form.status", Operator: "not_in", Value: []any{"closed", "resolved"}}
	if !evaluateCondition(cond, ctx) {
		t.Error("not_in should match when value not in set")
	}
	cond.Value = []any{"open", "pending"}
	if evaluateCondition(cond, ctx) {
		t.Error("not_in should not match when value in set")
	}
}

func TestEvaluateCondition_IsEmpty(t *testing.T) {
	ctx := evalContext{"form.name": ""}
	cond := GatewayCondition{Field: "form.name", Operator: "is_empty"}
	if !evaluateCondition(cond, ctx) {
		t.Error("is_empty should match empty string")
	}
	// Missing field
	cond.Field = "form.missing"
	if !evaluateCondition(cond, ctx) {
		t.Error("is_empty should match missing field")
	}
	// Non-empty
	ctx["form.name"] = "hello"
	cond.Field = "form.name"
	if evaluateCondition(cond, ctx) {
		t.Error("is_empty should not match non-empty")
	}
}

func TestEvaluateCondition_IsNotEmpty(t *testing.T) {
	ctx := evalContext{"form.name": "hello"}
	cond := GatewayCondition{Field: "form.name", Operator: "is_not_empty"}
	if !evaluateCondition(cond, ctx) {
		t.Error("is_not_empty should match non-empty")
	}
	ctx["form.name"] = ""
	if evaluateCondition(cond, ctx) {
		t.Error("is_not_empty should not match empty string")
	}
	// Missing field
	cond.Field = "form.missing"
	if evaluateCondition(cond, ctx) {
		t.Error("is_not_empty should not match missing field")
	}
}

func TestEvaluateCondition_Between(t *testing.T) {
	ctx := evalContext{"ticket.priority_id": float64(3)}
	cond := GatewayCondition{Field: "ticket.priority_id", Operator: "between", Value: []any{float64(1), float64(5)}}
	if !evaluateCondition(cond, ctx) {
		t.Error("between should match inclusive range")
	}
	cond.Value = []any{float64(4), float64(6)}
	if evaluateCondition(cond, ctx) {
		t.Error("between should not match out of range")
	}
	// Boundary inclusive
	cond.Value = []any{float64(3), float64(5)}
	if !evaluateCondition(cond, ctx) {
		t.Error("between should be inclusive on lower bound")
	}
}

func TestEvaluateCondition_Matches(t *testing.T) {
	ctx := evalContext{"form.email": "user@example.com"}
	cond := GatewayCondition{Field: "form.email", Operator: "matches", Value: `^[a-z]+@example\.com$`}
	if !evaluateCondition(cond, ctx) {
		t.Error("matches should match valid regex")
	}
	cond.Value = `^admin@`
	if evaluateCondition(cond, ctx) {
		t.Error("matches should not match non-matching regex")
	}
	// Invalid regex should return false
	cond.Value = `[invalid`
	if evaluateCondition(cond, ctx) {
		t.Error("matches should return false for invalid regex")
	}
}

func TestEvaluateCondition_CompoundAnd(t *testing.T) {
	ctx := evalContext{"form.type": "vpn", "ticket.priority_id": float64(3)}
	cond := GatewayCondition{
		Logic: "and",
		Conditions: []GatewayCondition{
			{Field: "form.type", Operator: "equals", Value: "vpn"},
			{Field: "ticket.priority_id", Operator: "gt", Value: float64(2)},
		},
	}
	if !evaluateCondition(cond, ctx) {
		t.Error("AND compound should match when all true")
	}
	// One false
	cond.Conditions[1].Value = float64(5)
	if evaluateCondition(cond, ctx) {
		t.Error("AND compound should not match when one false")
	}
}

func TestEvaluateCondition_CompoundOr(t *testing.T) {
	ctx := evalContext{"form.type": "vpn", "ticket.priority_id": float64(3)}
	cond := GatewayCondition{
		Logic: "or",
		Conditions: []GatewayCondition{
			{Field: "form.type", Operator: "equals", Value: "other"},
			{Field: "ticket.priority_id", Operator: "gt", Value: float64(2)},
		},
	}
	if !evaluateCondition(cond, ctx) {
		t.Error("OR compound should match when any true")
	}
	// All false
	cond.Conditions[1].Value = float64(5)
	if evaluateCondition(cond, ctx) {
		t.Error("OR compound should not match when all false")
	}
}

func TestEvaluateCondition_NestedCompound(t *testing.T) {
	ctx := evalContext{"form.type": "vpn", "ticket.priority_id": float64(3), "form.status": "open"}
	cond := GatewayCondition{
		Logic: "and",
		Conditions: []GatewayCondition{
			{Field: "form.type", Operator: "equals", Value: "vpn"},
			{
				Logic: "or",
				Conditions: []GatewayCondition{
					{Field: "ticket.priority_id", Operator: "gt", Value: float64(5)},
					{Field: "form.status", Operator: "equals", Value: "open"},
				},
			},
		},
	}
	if !evaluateCondition(cond, ctx) {
		t.Error("nested compound should match (type=vpn AND (prio>5 OR status=open))")
	}
}

func TestEvaluateCondition_BackwardCompatSingleCondition(t *testing.T) {
	// A simple condition without Logic/Conditions should still work
	ctx := evalContext{"form.type": "vpn"}
	cond := GatewayCondition{Field: "form.type", Operator: "equals", Value: "vpn"}
	if !evaluateCondition(cond, ctx) {
		t.Error("backward-compat single condition should work")
	}
}

func TestEvaluateCondition_MissingField(t *testing.T) {
	ctx := evalContext{}
	cond := GatewayCondition{Field: "form.missing", Operator: "equals", Value: "x"}
	if evaluateCondition(cond, ctx) {
		t.Error("missing field should return false for equals")
	}
}

func TestEvaluateCondition_UnknownOperator(t *testing.T) {
	ctx := evalContext{"form.type": "vpn"}
	cond := GatewayCondition{Field: "form.type", Operator: "unknown_op", Value: "vpn"}
	if evaluateCondition(cond, ctx) {
		t.Error("unknown operator should return false")
	}
}

func TestDeserializeVarValue(t *testing.T) {
	tests := []struct {
		raw       string
		valueType string
		want      any
	}{
		{"", "string", nil},
		{"hello", "string", "hello"},
		{"42.5", "number", float64(42.5)},
		{"true", "boolean", true},
		{`{"key":"val"}`, "json", map[string]any{"key": "val"}},
		{"2024-01-01", "date", "2024-01-01"},
	}
	for _, tt := range tests {
		got := deserializeVarValue(tt.raw, tt.valueType)
		if got == nil && tt.want == nil {
			continue
		}
		// Compare via sprintf for simplicity
		gotStr, wantStr := fmt.Sprintf("%v", got), fmt.Sprintf("%v", tt.want)
		if gotStr != wantStr {
			t.Errorf("deserializeVarValue(%q, %q) = %v, want %v", tt.raw, tt.valueType, got, tt.want)
		}
	}
}
