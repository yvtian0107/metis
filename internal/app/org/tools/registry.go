package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"metis/internal/app"
)

// OrgToolRegistry handles organization-related AI tool calls.
// It implements the ToolHandlerRegistry interface (HasTool + Execute).
type OrgToolRegistry struct {
	resolver app.OrgResolver
}

// NewOrgToolRegistry creates a new OrgToolRegistry.
func NewOrgToolRegistry(resolver app.OrgResolver) *OrgToolRegistry {
	return &OrgToolRegistry{resolver: resolver}
}

// HasTool returns true for organization.org_context.
func (r *OrgToolRegistry) HasTool(name string) bool {
	return name == "organization.org_context"
}

// Execute dispatches tool calls.
func (r *OrgToolRegistry) Execute(_ context.Context, toolName string, _ uint, args json.RawMessage) (json.RawMessage, error) {
	if toolName != "organization.org_context" {
		return nil, fmt.Errorf("unknown org tool: %s", toolName)
	}
	return r.handleOrgContext(args)
}

type orgContextArgs struct {
	Username        string `json:"username"`
	DepartmentCode  string `json:"department_code"`
	PositionCode    string `json:"position_code"`
	IncludeInactive bool   `json:"include_inactive"`
}

func (r *OrgToolRegistry) handleOrgContext(args json.RawMessage) (json.RawMessage, error) {
	var params orgContextArgs
	if len(args) > 0 {
		if err := json.Unmarshal(args, &params); err != nil {
			return nil, fmt.Errorf("invalid arguments: %w", err)
		}
	}

	result, err := r.resolver.QueryContext(params.Username, params.DepartmentCode, params.PositionCode, params.IncludeInactive)
	if err != nil {
		return nil, fmt.Errorf("org context query failed: %w", err)
	}

	return json.Marshal(result)
}
