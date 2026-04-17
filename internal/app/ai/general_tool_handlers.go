package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"metis/internal/app"
)

// GeneralToolHandler is the function signature for general tool implementations.
type GeneralToolHandler func(ctx context.Context, userID uint, args json.RawMessage) (json.RawMessage, error)

// --- Interfaces ---

// UserFinder finds user basic info by ID.
type UserFinder interface {
	FindByID(id uint) (*GeneralUserInfo, error)
}

// GeneralUserInfo holds basic user info for the current_user_profile tool.
type GeneralUserInfo struct {
	ID             uint   `json:"id"`
	Username       string `json:"username"`
	Email          string `json:"email"`
	Phone          string `json:"phone"`
	Avatar         string `json:"avatar"`
	RoleID         uint   `json:"roleId"`
	RoleName       string `json:"roleName"`
	RoleCode       string `json:"roleCode"`
	ManagerID      *uint  `json:"managerId,omitempty"`
	ManagerUsername string `json:"managerUsername,omitempty"`
}

// --- Registry ---

// GeneralToolRegistry manages and dispatches general tool handlers.
type GeneralToolRegistry struct {
	handlers    map[string]GeneralToolHandler
	userFinder  UserFinder
	orgResolver app.OrgResolver
}

// NewGeneralToolRegistry creates a new registry with the given dependencies.
// orgResolver may be nil if the Org App is not installed.
func NewGeneralToolRegistry(userFinder UserFinder, orgResolver app.OrgResolver) *GeneralToolRegistry {
	r := &GeneralToolRegistry{
		handlers:    make(map[string]GeneralToolHandler),
		userFinder:  userFinder,
		orgResolver: orgResolver,
	}

	r.handlers["general.current_time"] = r.handleCurrentTime
	r.handlers["system.current_user_profile"] = r.handleCurrentUserProfile

	return r
}

// Execute runs the named tool with the given arguments.
func (r *GeneralToolRegistry) Execute(ctx context.Context, toolName string, userID uint, args json.RawMessage) (json.RawMessage, error) {
	handler, ok := r.handlers[toolName]
	if !ok {
		return nil, fmt.Errorf("unknown general tool: %s", toolName)
	}
	return handler(ctx, userID, args)
}

// HasTool returns true if the registry has a handler for the given tool name.
func (r *GeneralToolRegistry) HasTool(name string) bool {
	_, ok := r.handlers[name]
	return ok
}

// --- Handler: general.current_time ---

type currentTimeArgs struct {
	Timezone string `json:"timezone"`
}

type currentTimeResult struct {
	ServerTime         string `json:"server_time"`
	UTCTime            string `json:"utc_time"`
	ChinaFormattedTime string `json:"china_formatted_time"`
	TargetTime         string `json:"target_time"`
	TargetTimezone     string `json:"target_timezone"`
}

func (r *GeneralToolRegistry) handleCurrentTime(_ context.Context, _ uint, args json.RawMessage) (json.RawMessage, error) {
	var params currentTimeArgs
	if len(args) > 0 {
		if err := json.Unmarshal(args, &params); err != nil {
			return nil, fmt.Errorf("invalid arguments: %w", err)
		}
	}

	now := time.Now()

	chinaLoc, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		chinaLoc = time.FixedZone("CST", 8*3600)
	}

	result := currentTimeResult{
		ServerTime:         now.Format(time.RFC3339),
		UTCTime:            now.UTC().Format(time.RFC3339),
		ChinaFormattedTime: now.In(chinaLoc).Format("2006-01-02 15:04:05"),
	}

	if params.Timezone != "" {
		loc, err := time.LoadLocation(params.Timezone)
		if err != nil {
			return nil, fmt.Errorf("invalid timezone %q: %w", params.Timezone, err)
		}
		result.TargetTime = now.In(loc).Format(time.RFC3339)
		result.TargetTimezone = params.Timezone
	}

	return json.Marshal(result)
}

// --- Handler: system.current_user_profile ---

type userProfileResult struct {
	User          *GeneralUserInfo   `json:"user"`
	Department    *app.OrgDepartment `json:"department,omitempty"`
	Positions     []app.OrgPosition  `json:"positions,omitempty"`
	MissingFields []string           `json:"missing_fields,omitempty"`
}

func (r *GeneralToolRegistry) handleCurrentUserProfile(_ context.Context, userID uint, _ json.RawMessage) (json.RawMessage, error) {
	user, err := r.userFinder.FindByID(userID)
	if err != nil {
		return nil, fmt.Errorf("failed to find user %d: %w", userID, err)
	}

	result := userProfileResult{
		User: user,
	}

	if r.orgResolver != nil {
		var missingFields []string

		dept, err := r.orgResolver.GetUserDepartment(userID)
		if err == nil && dept != nil {
			result.Department = dept
		} else {
			missingFields = append(missingFields, "department")
		}

		positions, err := r.orgResolver.GetUserPositions(userID)
		if err == nil && len(positions) > 0 {
			result.Positions = positions
		} else {
			missingFields = append(missingFields, "positions")
		}

		if len(missingFields) > 0 {
			result.MissingFields = missingFields
		}
	}

	return json.Marshal(result)
}
