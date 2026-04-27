package engine

import (
	"encoding/json"
	"fmt"
	"strconv"

	"gorm.io/gorm"

	"metis/internal/app"
)

// ParticipantResolver resolves participant configurations to user IDs.
type ParticipantResolver struct {
	orgResolver app.OrgResolver // nil when Org App is not installed
}

func NewParticipantResolver(orgResolver app.OrgResolver) *ParticipantResolver {
	return &ParticipantResolver{orgResolver: orgResolver}
}

// Resolve returns user IDs for a given participant configuration.
func (r *ParticipantResolver) Resolve(tx *gorm.DB, ticketID uint, p Participant) ([]uint, error) {
	switch p.Type {
	case "requester":
		return r.resolveRequester(tx, ticketID)

	case "user":
		uid, err := strconv.ParseUint(p.Value, 10, 64)
		if err != nil {
			// Value is not numeric — try resolving as username
			var user struct{ ID uint }
			if dbErr := tx.Table("users").Where("username = ?", p.Value).Select("id").First(&user).Error; dbErr != nil {
				return nil, fmt.Errorf("user %q not found by ID or username: %w", p.Value, dbErr)
			}
			return []uint{user.ID}, nil
		}
		return []uint{uint(uid)}, nil

	case "requester_manager":
		return r.resolveRequesterManager(tx, ticketID)

	case "position":
		if r.orgResolver == nil {
			return nil, fmt.Errorf("参与人解析失败：position 类型需要安装组织架构模块")
		}
		posID, err := strconv.ParseUint(p.Value, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid position ID %q: %w", p.Value, err)
		}
		return r.resolveUsersByPositionID(tx, uint(posID))

	case "department":
		if r.orgResolver == nil {
			return nil, fmt.Errorf("参与人解析失败：department 类型需要安装组织架构模块")
		}
		deptID, err := strconv.ParseUint(p.Value, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid department ID %q: %w", p.Value, err)
		}
		return r.resolveUsersByDepartmentID(tx, uint(deptID))

	case "position_department":
		if r.orgResolver == nil {
			return nil, fmt.Errorf("参与人解析失败：position_department 类型需要安装组织架构模块")
		}
		if p.PositionCode == "" || p.DepartmentCode == "" {
			return nil, fmt.Errorf("position_department 类型需要同时指定 position_code 和 department_code")
		}
		return r.resolveUsersByPositionAndDepartment(tx, p.PositionCode, p.DepartmentCode)

	default:
		return nil, fmt.Errorf("unsupported participant type: %s", p.Type)
	}
}

func (r *ParticipantResolver) resolveRequester(tx *gorm.DB, ticketID uint) ([]uint, error) {
	var ticket ticketModel
	if err := tx.First(&ticket, ticketID).Error; err != nil {
		return nil, fmt.Errorf("ticket not found: %w", err)
	}
	if ticket.RequesterID == 0 {
		return nil, nil
	}
	return []uint{ticket.RequesterID}, nil
}

func (r *ParticipantResolver) resolveRequesterManager(tx *gorm.DB, ticketID uint) ([]uint, error) {
	var ticket ticketModel
	if err := tx.First(&ticket, ticketID).Error; err != nil {
		return nil, fmt.Errorf("ticket not found: %w", err)
	}

	if ticket.RequesterID == 0 {
		return nil, nil
	}

	var user struct {
		ManagerID *uint
	}
	if err := tx.Table("users").Where("id = ?", ticket.RequesterID).Select("manager_id").First(&user).Error; err != nil {
		return nil, fmt.Errorf("failed to find manager for user %d: %w", ticket.RequesterID, err)
	}
	if user.ManagerID == nil {
		return nil, nil
	}
	return []uint{*user.ManagerID}, nil
}

func (r *ParticipantResolver) resolveUsersByPositionID(tx *gorm.DB, positionID uint) ([]uint, error) {
	var userIDs []uint
	err := tx.Table("user_positions").
		Joins("JOIN users ON users.id = user_positions.user_id").
		Where("user_positions.position_id = ? AND users.is_active = ?", positionID, true).
		Pluck("DISTINCT users.id", &userIDs).Error
	return userIDs, err
}

func (r *ParticipantResolver) resolveUsersByDepartmentID(tx *gorm.DB, departmentID uint) ([]uint, error) {
	var userIDs []uint
	err := tx.Table("user_positions").
		Joins("JOIN users ON users.id = user_positions.user_id").
		Where("user_positions.department_id = ? AND users.is_active = ?", departmentID, true).
		Pluck("DISTINCT users.id", &userIDs).Error
	return userIDs, err
}

func (r *ParticipantResolver) resolveUsersByPositionAndDepartment(tx *gorm.DB, posCode, deptCode string) ([]uint, error) {
	var userIDs []uint
	err := tx.Table("user_positions").
		Joins("JOIN positions ON positions.id = user_positions.position_id").
		Joins("JOIN departments ON departments.id = user_positions.department_id").
		Joins("JOIN users ON users.id = user_positions.user_id").
		Where("positions.code = ? AND departments.code = ? AND users.is_active = ?", posCode, deptCode, true).
		Pluck("DISTINCT users.id", &userIDs).Error
	return userIDs, err
}

// resolveForToolArgs is the JSON structure for decision.resolve_participant tool arguments.
type resolveForToolArgs struct {
	Type           string `json:"type"`
	Value          string `json:"value,omitempty"`
	PositionCode   string `json:"position_code,omitempty"`
	DepartmentCode string `json:"department_code,omitempty"`
}

// ResolveForTool resolves participants from JSON tool arguments.
// It wraps Resolve() with JSON parameter parsing for the decision tool.
func (r *ParticipantResolver) ResolveForTool(tx *gorm.DB, ticketID uint, toolArgs json.RawMessage) ([]uint, error) {
	var args resolveForToolArgs
	if err := json.Unmarshal(toolArgs, &args); err != nil {
		return nil, fmt.Errorf("invalid tool arguments: %w", err)
	}
	if args.Type == "" {
		return nil, fmt.Errorf("participant type is required")
	}
	p := Participant{
		Type:           args.Type,
		Value:          args.Value,
		PositionCode:   args.PositionCode,
		DepartmentCode: args.DepartmentCode,
	}
	return r.Resolve(tx, ticketID, p)
}
