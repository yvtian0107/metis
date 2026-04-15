package engine

import (
	"encoding/json"
	"fmt"

	"github.com/expr-lang/expr"
	"gorm.io/gorm"
)

// evaluateExpression compiles and runs an expr-lang/expr expression in a sandboxed environment.
// Only arithmetic, comparison, logical, string concatenation, and ternary operators are allowed.
func evaluateExpression(expression string, env map[string]any) (any, error) {
	program, err := expr.Compile(expression, expr.Env(env))
	if err != nil {
		return nil, fmt.Errorf("expression compile error: %w", err)
	}

	result, err := expr.Run(program, env)
	if err != nil {
		return nil, fmt.Errorf("expression eval error: %w", err)
	}

	return result, nil
}

// inferValueType determines the process variable value_type from a Go value returned by expr.
func inferValueType(val any) string {
	switch val.(type) {
	case int, int8, int16, int32, int64,
		uint, uint8, uint16, uint32, uint64,
		float32, float64, json.Number:
		return "number"
	case bool:
		return "boolean"
	case string:
		return "string"
	default:
		return "json"
	}
}

// buildScriptEnv creates the expression environment map from process variables and ticket fields.
// Variable names are used directly as keys (no "var." prefix) for cleaner expressions.
func buildScriptEnv(tx *gorm.DB, ticketID uint, scopeID string) map[string]any {
	env := make(map[string]any)

	// Load process variables for the current scope
	var vars []processVariableModel
	tx.Where("ticket_id = ? AND scope_id = ?", ticketID, scopeID).Find(&vars)

	for _, v := range vars {
		env[v.Key] = deserializeVarValue(v.Value, v.ValueType)
	}

	// Inject read-only ticket fields
	var ticket ticketModel
	if tx.First(&ticket, ticketID).Error == nil {
		env["ticket_priority_id"] = ticket.PriorityID
		env["ticket_requester_id"] = ticket.RequesterID
		env["ticket_status"] = ticket.Status
	}

	return env
}
