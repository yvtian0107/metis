package testutil

import (
	"fmt"
	"metis/internal/app/org/domain"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"metis/internal/database"
	"metis/internal/model"
)

func NewOrgTestDB(t *testing.T) *database.DB {
	t.Helper()
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())
	gdb, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	if err := gdb.AutoMigrate(
		&model.User{},
		&model.Role{},
		&domain.Department{},
		&domain.Position{},
		&domain.UserPosition{},
		&domain.DepartmentPosition{},
	); err != nil {
		t.Fatalf("migrate test db: %v", err)
	}
	return &database.DB{DB: gdb}
}

func SeedRole(t *testing.T, db *database.DB, code string) *model.Role {
	t.Helper()
	r := &model.Role{Name: code, Code: code}
	if err := db.Create(r).Error; err != nil {
		t.Fatalf("seed role: %v", err)
	}
	return r
}

func SeedUser(t *testing.T, db *database.DB, username string, roleID uint) *model.User {
	t.Helper()
	u := &model.User{Username: username, RoleID: roleID, IsActive: true}
	if err := db.Create(u).Error; err != nil {
		t.Fatalf("seed user: %v", err)
	}
	return u
}

func SeedDepartment(t *testing.T, db *database.DB, name, code string, parentID, managerID *uint, isActive bool) *domain.Department {
	t.Helper()
	d := &domain.Department{
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

func SeedPosition(t *testing.T, db *database.DB, name, code string, isActive bool) *domain.Position {
	t.Helper()
	p := &domain.Position{
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

func SeedAssignment(t *testing.T, db *database.DB, userID, deptID, posID uint, isPrimary bool) *domain.UserPosition {
	t.Helper()
	up := &domain.UserPosition{
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
