package repository

import (
	"fmt"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/samber/do/v2"
	"gorm.io/gorm"

	"metis/internal/database"
	"metis/internal/model"
)

func newTestUserRepo(t *testing.T) (*UserRepo, *gorm.DB) {
	t.Helper()
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	if err := db.AutoMigrate(&model.User{}, &model.Role{}); err != nil {
		t.Fatalf("migrate test db: %v", err)
	}
	injector := do.New()
	do.ProvideValue(injector, &database.DB{DB: db})
	do.Provide(injector, NewUser)
	return do.MustInvoke[*UserRepo](injector), db
}

func seedRoleForUserRepo(t *testing.T, db *gorm.DB, code string) *model.Role {
	t.Helper()
	role := &model.Role{Name: code, Code: code}
	if err := db.Create(role).Error; err != nil {
		t.Fatalf("seed role: %v", err)
	}
	return role
}

func seedUserForUserRepo(t *testing.T, db *gorm.DB, roleID uint, username, email, phone string, isActive bool, managerID *uint) *model.User {
	t.Helper()
	user := &model.User{
		Username: username,
		Password: "hashed",
		Email:    email,
		Phone:    phone,
		RoleID:   roleID,
		IsActive: isActive,
		ManagerID: managerID,
	}
	if err := db.Create(user).Error; err != nil {
		t.Fatalf("seed user: %v", err)
	}
	if !isActive {
		if err := db.Model(user).Update("is_active", false).Error; err != nil {
			t.Fatalf("set inactive user: %v", err)
		}
		user.IsActive = false
	}
	return user
}

func TestUserRepoList_PreloadsManager(t *testing.T) {
	repo, db := newTestUserRepo(t)
	role := seedRoleForUserRepo(t, db, "test-role")
	manager := seedUserForUserRepo(t, db, role.ID, "manager", "manager@example.com", "100", true, nil)
	user := seedUserForUserRepo(t, db, role.ID, "alice", "alice@example.com", "101", true, &manager.ID)

	result, err := repo.List(ListParams{Page: 1, PageSize: 20})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(result.Items) != 2 {
		t.Fatalf("expected 2 users, got %d", len(result.Items))
	}
	if result.Items[0].ID != user.ID {
		t.Fatalf("expected managed user %d first, got %d", user.ID, result.Items[0].ID)
	}
	if result.Items[0].Manager == nil || result.Items[0].Manager.ID != manager.ID {
		t.Fatalf("expected manager %d to be preloaded, got %+v", manager.ID, result.Items[0].Manager)
	}
}

func TestUserRepoList_FiltersKeywordAndActiveState(t *testing.T) {
	repo, db := newTestUserRepo(t)
	role := seedRoleForUserRepo(t, db, "test-role")
	seedUserForUserRepo(t, db, role.ID, "alice", "alice@example.com", "123", true, nil)
	seedUserForUserRepo(t, db, role.ID, "bob", "bob@example.com", "456", false, nil)
	seedUserForUserRepo(t, db, role.ID, "carol", "carol@example.com", "789", true, nil)

	active := true
	result, err := repo.List(ListParams{Keyword: "example", IsActive: &active, Page: 1, PageSize: 20})
	if err != nil {
		t.Fatalf("list filtered users: %v", err)
	}
	if result.Total != 2 {
		t.Fatalf("expected 2 active matched users, got %d", result.Total)
	}
	for _, item := range result.Items {
		if !item.IsActive {
			t.Fatalf("expected only active users, found inactive user %d", item.ID)
		}
	}

	phoneResult, err := repo.List(ListParams{Keyword: "789", Page: 1, PageSize: 20})
	if err != nil {
		t.Fatalf("list by phone: %v", err)
	}
	if len(phoneResult.Items) != 1 || phoneResult.Items[0].Username != "carol" {
		t.Fatalf("expected only carol by phone, got %+v", phoneResult.Items)
	}
}

func TestUserRepoList_AppliesDefaultPagination(t *testing.T) {
	repo, db := newTestUserRepo(t)
	role := seedRoleForUserRepo(t, db, "test-role")
	for i := 0; i < 25; i++ {
		seedUserForUserRepo(t, db, role.ID, fmt.Sprintf("user-%02d", i), fmt.Sprintf("user-%02d@example.com", i), fmt.Sprintf("%03d", i), true, nil)
	}

	result, err := repo.List(ListParams{Page: 0, PageSize: 0})
	if err != nil {
		t.Fatalf("list with default pagination: %v", err)
	}
	if result.Total != 25 {
		t.Fatalf("expected total 25 users, got %d", result.Total)
	}
	if len(result.Items) != 20 {
		t.Fatalf("expected default page size 20, got %d", len(result.Items))
	}
	if result.Items[0].Username != "user-24" {
		t.Fatalf("expected newest user first, got %s", result.Items[0].Username)
	}
}