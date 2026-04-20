package handler

import (
	"fmt"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"metis/internal/model"
	"metis/internal/pkg/token"
)

type testDepartment struct {
	ID   uint   `gorm:"primaryKey"`
	Code string `gorm:"size:64"`
}

func (testDepartment) TableName() string { return "departments" }

type testPosition struct {
	ID   uint   `gorm:"primaryKey"`
	Code string `gorm:"size:64"`
}

func (testPosition) TableName() string { return "positions" }

type testUserPosition struct {
	ID           uint `gorm:"primaryKey"`
	UserID       uint
	DepartmentID uint
	PositionID   uint
	IsPrimary    bool
}

func (testUserPosition) TableName() string { return "user_positions" }

func newTestDBForInstallHandler(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())
	gdb, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	if err := gdb.AutoMigrate(
		&model.User{},
		&model.Role{},
		&testDepartment{},
		&testPosition{},
		&testUserPosition{},
	); err != nil {
		t.Fatalf("migrate test db: %v", err)
	}
	return gdb
}

func TestUpsertInstallAdmin_ReusesExistingUser(t *testing.T) {
	db := newTestDBForInstallHandler(t)

	role := model.Role{Name: "Admin", Code: model.RoleAdmin}
	if err := db.Create(&role).Error; err != nil {
		t.Fatalf("create role: %v", err)
	}

	oldPassword, err := token.HashPassword("OldPassword123!")
	if err != nil {
		t.Fatalf("hash old password: %v", err)
	}
	user := model.User{
		Username: "admin",
		Password: oldPassword,
		Email:    "old@example.com",
		RoleID:   999,
		IsActive: false,
	}
	if err := db.Create(&user).Error; err != nil {
		t.Fatalf("create existing user: %v", err)
	}

	if err := upsertInstallAdmin(db, "admin", "NewPassword123!", "new@example.com", role.ID); err != nil {
		t.Fatalf("upsert install admin: %v", err)
	}

	var updated model.User
	if err := db.First(&updated, user.ID).Error; err != nil {
		t.Fatalf("reload updated user: %v", err)
	}
	if updated.Email != "new@example.com" {
		t.Fatalf("expected updated email, got %s", updated.Email)
	}
	if updated.RoleID != role.ID {
		t.Fatalf("expected role id %d, got %d", role.ID, updated.RoleID)
	}
	if !updated.IsActive {
		t.Fatal("expected user to be active")
	}
	if updated.PasswordChangedAt == nil {
		t.Fatal("expected password changed at to be set")
	}
	if !token.CheckPassword(updated.Password, "NewPassword123!") {
		t.Fatal("expected password to be replaced")
	}
}

func TestAssignInstallAdminOrgIdentity_WaitsForOrgSeedData(t *testing.T) {
	db := newTestDBForInstallHandler(t)

	user := model.User{Username: "admin", IsActive: true}
	if err := db.Create(&user).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}

	if err := assignInstallAdminOrgIdentity(db, "admin"); err != nil {
		t.Fatalf("assign before org seed should not fail: %v", err)
	}

	var count int64
	if err := db.Table("user_positions").Count(&count).Error; err != nil {
		t.Fatalf("count user positions before seed: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected no user positions before org seed, got %d", count)
	}

	if err := db.Create(&testDepartment{Code: "it"}).Error; err != nil {
		t.Fatalf("create department: %v", err)
	}
	if err := db.Create(&testPosition{Code: "it_admin"}).Error; err != nil {
		t.Fatalf("create position: %v", err)
	}

	if err := assignInstallAdminOrgIdentity(db, "admin"); err != nil {
		t.Fatalf("assign after org seed: %v", err)
	}

	if err := db.Table("user_positions").Where("user_id = ? AND is_primary = ?", user.ID, true).Count(&count).Error; err != nil {
		t.Fatalf("count user positions after seed: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected 1 user position after org seed, got %d", count)
	}
}
