package domain

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"time"

	"metis/internal/model"
)

// JSONMap is a map wrapper that handles SQLite TEXT columns for JSON objects.
type JSONMap json.RawMessage

func (j JSONMap) Value() (driver.Value, error) {
	if len(j) == 0 {
		return "{}", nil
	}
	return string(j), nil
}

func (j *JSONMap) Scan(src any) error {
	switch v := src.(type) {
	case string:
		*j = JSONMap(v)
	case []byte:
		*j = append(JSONMap(nil), v...)
	case nil:
		*j = JSONMap("{}")
	default:
		return fmt.Errorf("JSONMap.Scan: unsupported type %T", v)
	}
	return nil
}

func (j JSONMap) MarshalJSON() ([]byte, error) {
	if len(j) == 0 {
		return []byte("{}"), nil
	}
	return []byte(j), nil
}

func (j *JSONMap) UnmarshalJSON(data []byte) error {
	*j = append(JSONMap(nil), data...)
	return nil
}

// Department 部门
type Department struct {
	model.BaseModel
	Name        string `json:"name" gorm:"size:128;not null"`
	Code        string `json:"code" gorm:"size:64;uniqueIndex;not null"`
	ParentID    *uint  `json:"parentId" gorm:"index"`
	ManagerID   *uint  `json:"managerId" gorm:"index"`
	Sort        int    `json:"sort" gorm:"default:0"`
	Description string `json:"description" gorm:"size:255"`
	IsActive    bool   `json:"isActive" gorm:"not null;default:true"`
}

func (Department) TableName() string { return "departments" }

type DepartmentResponse struct {
	ID          uint      `json:"id"`
	Name        string    `json:"name"`
	Code        string    `json:"code"`
	ParentID    *uint     `json:"parentId"`
	ManagerID   *uint     `json:"managerId"`
	Sort        int       `json:"sort"`
	Description string    `json:"description"`
	IsActive    bool      `json:"isActive"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

func (d *Department) ToResponse() DepartmentResponse {
	return DepartmentResponse{
		ID:          d.ID,
		Name:        d.Name,
		Code:        d.Code,
		ParentID:    d.ParentID,
		ManagerID:   d.ManagerID,
		Sort:        d.Sort,
		Description: d.Description,
		IsActive:    d.IsActive,
		CreatedAt:   d.CreatedAt,
		UpdatedAt:   d.UpdatedAt,
	}
}

// Position 岗位
type Position struct {
	model.BaseModel
	Name        string `json:"name" gorm:"size:128;not null"`
	Code        string `json:"code" gorm:"size:64;uniqueIndex;not null"`
	Sort        int    `json:"sort" gorm:"default:0"`
	Description string `json:"description" gorm:"size:255"`
	IsActive    bool   `json:"isActive" gorm:"not null;default:true"`
}

func (Position) TableName() string { return "positions" }

type PositionResponse struct {
	ID              uint                        `json:"id"`
	Name            string                      `json:"name"`
	Code            string                      `json:"code"`
	Description     string                      `json:"description"`
	IsActive        bool                        `json:"isActive"`
	DepartmentCount int                         `json:"departmentCount"`
	MemberCount     int                         `json:"memberCount"`
	Departments     []PositionDepartmentSummary `json:"departments,omitempty"`
	CreatedAt       time.Time                   `json:"createdAt"`
	UpdatedAt       time.Time                   `json:"updatedAt"`
}

func (p *Position) ToResponse() PositionResponse {
	return PositionResponse{
		ID:          p.ID,
		Name:        p.Name,
		Code:        p.Code,
		Description: p.Description,
		IsActive:    p.IsActive,
		CreatedAt:   p.CreatedAt,
		UpdatedAt:   p.UpdatedAt,
	}
}

type PositionDepartmentSummary struct {
	ID   uint   `json:"id"`
	Name string `json:"name"`
	Code string `json:"code"`
}

// UserPosition 人员岗位关联
type UserPosition struct {
	model.BaseModel
	UserID       uint       `json:"userId" gorm:"not null;index:idx_user_pos_user_dep_pos,unique"`
	DepartmentID uint       `json:"departmentId" gorm:"not null;index:idx_user_pos_user_dep_pos,unique"`
	PositionID   uint       `json:"positionId" gorm:"not null;index:idx_user_pos_user_dep_pos,unique"`
	IsPrimary    bool       `json:"isPrimary" gorm:"not null;default:false"`
	Sort         int        `json:"sort" gorm:"default:0"`
	Department   Department `json:"department,omitempty" gorm:"foreignKey:DepartmentID"`
	Position     Position   `json:"position,omitempty" gorm:"foreignKey:PositionID"`
}

func (UserPosition) TableName() string { return "user_positions" }

type UserPositionResponse struct {
	ID           uint                `json:"id"`
	UserID       uint                `json:"userId"`
	DepartmentID uint                `json:"departmentId"`
	PositionID   uint                `json:"positionId"`
	IsPrimary    bool                `json:"isPrimary"`
	Sort         int                 `json:"sort"`
	Department   *DepartmentResponse `json:"department,omitempty"`
	Position     *PositionResponse   `json:"position,omitempty"`
}

// DepartmentPosition 部门可用职位关联
type DepartmentPosition struct {
	model.BaseModel
	DepartmentID uint `json:"departmentId" gorm:"not null;uniqueIndex:idx_dept_pos"`
	PositionID   uint `json:"positionId" gorm:"not null;uniqueIndex:idx_dept_pos"`
}

func (DepartmentPosition) TableName() string { return "department_positions" }

type AssignmentItem struct {
	UserID       uint      `json:"userId"`
	Username     string    `json:"username"`
	Email        string    `json:"email"`
	Avatar       string    `json:"avatar"`
	DepartmentID uint      `json:"departmentId"`
	PositionID   uint      `json:"positionId"`
	PositionName string    `json:"positionName"`
	IsPrimary    bool      `json:"isPrimary"`
	AssignmentID uint      `json:"assignmentId"`
	CreatedAt    time.Time `json:"createdAt"`
}

// MemberPositionItem represents a single position within a member's department assignment.
type MemberPositionItem struct {
	AssignmentID uint   `json:"assignmentId"`
	PositionID   uint   `json:"positionId"`
	PositionName string `json:"positionName"`
	IsPrimary    bool   `json:"isPrimary"`
}

// MemberWithPositions groups all positions for a user within a department.
type MemberWithPositions struct {
	UserID       uint                 `json:"userId"`
	Username     string               `json:"username"`
	Email        string               `json:"email"`
	Avatar       string               `json:"avatar"`
	DepartmentID uint                 `json:"departmentId"`
	Positions    []MemberPositionItem `json:"positions"`
	CreatedAt    time.Time            `json:"createdAt"`
}
