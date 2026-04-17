package org

import (
	"fmt"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"metis/internal/database"
	"metis/internal/model"
)

func newOrgTestDB(t *testing.T) *database.DB {
	t.Helper()
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())
	gdb, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	if err := gdb.AutoMigrate(
		&model.User{},
		&model.Role{},
		&Department{},
		&Position{},
		&UserPosition{},
		&DepartmentPosition{},
	); err != nil {
		t.Fatalf("migrate test db: %v", err)
	}
	return &database.DB{DB: gdb}
}

func seedRole(t *testing.T, db *database.DB, code string) *model.Role {
	t.Helper()
	r := &model.Role{Name: code, Code: code}
	if err := db.Create(r).Error; err != nil {
		t.Fatalf("seed role: %v", err)
	}
	return r
}

func seedUser(t *testing.T, db *database.DB, username string, roleID uint) *model.User {
	t.Helper()
	u := &model.User{Username: username, RoleID: roleID, IsActive: true}
	if err := db.Create(u).Error; err != nil {
		t.Fatalf("seed user: %v", err)
	}
	return u
}

func seedDepartment(t *testing.T, db *database.DB, name, code string, parentID, managerID *uint, isActive bool) *Department {
	t.Helper()
	d := &Department{
		Name:      name,
		Code:      code,
		ParentID:  parentID,
		ManagerID: managerID,
		IsActive:  true,
	}
	if err := db.Create(d).Error; err != nil {
		t.Fatalf("seed department: %v", err)
	}
	if !isActive {
		if err := db.Model(d).Update("is_active", false).Error; err != nil {
			t.Fatalf("update department is_active: %v", err)
		}
		d.IsActive = false
	}
	return d
}

func seedPosition(t *testing.T, db *database.DB, name, code string, isActive bool) *Position {
	t.Helper()
	p := &Position{
		Name:     name,
		Code:     code,
		IsActive: true,
	}
	if err := db.Create(p).Error; err != nil {
		t.Fatalf("seed position: %v", err)
	}
	if !isActive {
		if err := db.Model(p).Update("is_active", false).Error; err != nil {
			t.Fatalf("update position is_active: %v", err)
		}
		p.IsActive = false
	}
	return p
}

func seedAssignment(t *testing.T, db *database.DB, userID, deptID, posID uint, isPrimary bool) *UserPosition {
	t.Helper()
	up := &UserPosition{
		UserID:       userID,
		DepartmentID: deptID,
		PositionID:   posID,
		IsPrimary:    isPrimary,
	}
	if err := db.Create(up).Error; err != nil {
		t.Fatalf("seed assignment: %v", err)
	}
	return up
}
