package service

import (
	"errors"
	"fmt"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/samber/do/v2"
	"gorm.io/gorm"

	casbinpkg "metis/internal/casbin"
	"metis/internal/database"
	"metis/internal/model"
	"metis/internal/repository"
)

func newTestDBForRole(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())
	gdb, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	if err := gdb.AutoMigrate(
		&model.Role{},
		&model.RoleDeptScope{},
		&model.User{},
	); err != nil {
		t.Fatalf("migrate test db: %v", err)
	}
	return gdb
}

func newRoleServiceForTest(t *testing.T, db *gorm.DB) *RoleService {
	t.Helper()
	wrapped := &database.DB{DB: db}
	injector := do.New()
	do.ProvideValue(injector, wrapped)

	enforcer, err := casbinpkg.NewEnforcerWithDB(db)
	if err != nil {
		t.Fatalf("create casbin enforcer: %v", err)
	}
	do.ProvideValue(injector, enforcer)

	do.Provide(injector, repository.NewRole)
	do.Provide(injector, NewCasbin)

	roleRepo := do.MustInvoke[*repository.RoleRepo](injector)
	casbinSvc := do.MustInvoke[*CasbinService](injector)

	return &RoleService{
		roleRepo:  roleRepo,
		casbinSvc: casbinSvc,
	}
}

func seedRoleForTest(t *testing.T, db *gorm.DB, name, code string, isSystem bool) *model.Role {
	t.Helper()
	role := &model.Role{Name: name, Code: code, IsSystem: isSystem}
	if err := db.Create(role).Error; err != nil {
		t.Fatalf("seed role: %v", err)
	}
	return role
}

func seedUserForRoleTest(t *testing.T, db *gorm.DB, username string, roleID uint) {
	t.Helper()
	user := &model.User{Username: username, RoleID: roleID}
	if err := db.Create(user).Error; err != nil {
		t.Fatalf("seed user: %v", err)
	}
}

// 2. Create & Retrieve

func TestRoleServiceCreate_Success(t *testing.T) {
	db := newTestDBForRole(t)
	svc := newRoleServiceForTest(t, db)

	role, err := svc.Create("Editor", "editor", "Content editor", 10)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if role.ID == 0 {
		t.Fatal("expected role ID to be set")
	}
	if role.IsSystem {
		t.Fatal("expected IsSystem=false")
	}
	if role.DataScope != model.DataScopeAll {
		t.Fatalf("expected DataScope=all, got %s", role.DataScope)
	}
}

func TestRoleServiceCreate_RejectsDuplicateCode(t *testing.T) {
	db := newTestDBForRole(t)
	svc := newRoleServiceForTest(t, db)

	if _, err := svc.Create("Admin", "admin", "Admin", 0); err != nil {
		t.Fatalf("first create: %v", err)
	}
	_, err := svc.Create("Another Admin", "admin", "Desc", 1)
	if !errors.Is(err, ErrRoleCodeExists) {
		t.Fatalf("expected ErrRoleCodeExists, got %v", err)
	}
}

func TestRoleServiceGetByID_Success(t *testing.T) {
	db := newTestDBForRole(t)
	svc := newRoleServiceForTest(t, db)
	role := seedRoleForTest(t, db, "Editor", "editor", false)

	found, err := svc.GetByID(role.ID)
	if err != nil {
		t.Fatalf("get by id: %v", err)
	}
	if found.ID != role.ID {
		t.Fatalf("expected role ID %d, got %d", role.ID, found.ID)
	}
}

func TestRoleServiceGetByID_ReturnsNotFoundForMissing(t *testing.T) {
	db := newTestDBForRole(t)
	svc := newRoleServiceForTest(t, db)

	_, err := svc.GetByID(999)
	if !errors.Is(err, ErrRoleNotFound) {
		t.Fatalf("expected ErrRoleNotFound, got %v", err)
	}
}

func TestRoleServiceGetByIDWithDeptScope_Success(t *testing.T) {
	db := newTestDBForRole(t)
	svc := newRoleServiceForTest(t, db)
	role := seedRoleForTest(t, db, "Custom", "custom", false)
	role.DataScope = model.DataScopeCustom
	if err := db.Save(role).Error; err != nil {
		t.Fatalf("update scope: %v", err)
	}
	deptIDs := []uint{1, 2, 3}
	if err := svc.roleRepo.SetCustomDeptIDs(role.ID, deptIDs); err != nil {
		t.Fatalf("set custom dept ids: %v", err)
	}

	found, foundDeptIDs, err := svc.GetByIDWithDeptScope(role.ID)
	if err != nil {
		t.Fatalf("get by id with dept scope: %v", err)
	}
	if found.ID != role.ID {
		t.Fatalf("expected role ID %d, got %d", role.ID, found.ID)
	}
	if len(foundDeptIDs) != 3 || foundDeptIDs[0] != 1 || foundDeptIDs[1] != 2 || foundDeptIDs[2] != 3 {
		t.Fatalf("unexpected dept IDs: %v", foundDeptIDs)
	}
}

func TestRoleServiceGetByIDWithDeptScope_ReturnsEmptyDeptIDsForRoleWithoutCustomScope(t *testing.T) {
	db := newTestDBForRole(t)
	svc := newRoleServiceForTest(t, db)
	role := seedRoleForTest(t, db, "Editor", "editor", false)

	_, deptIDs, err := svc.GetByIDWithDeptScope(role.ID)
	if err != nil {
		t.Fatalf("get by id with dept scope: %v", err)
	}
	if deptIDs == nil {
		t.Fatal("expected empty deptIDs slice, got nil")
	}
	if len(deptIDs) != 0 {
		t.Fatalf("expected no dept IDs, got %v", deptIDs)
	}
}

// 3. Update

func TestRoleServiceUpdate_Success(t *testing.T) {
	db := newTestDBForRole(t)
	svc := newRoleServiceForTest(t, db)
	role := seedRoleForTest(t, db, "Editor", "editor", false)

	newName := "Senior Editor"
	newDesc := "Updated description"
	updated, err := svc.Update(role.ID, UpdateRoleParams{
		Name:        &newName,
		Description: &newDesc,
	})
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if updated.Name != newName {
		t.Fatalf("expected name %s, got %s", newName, updated.Name)
	}
	if updated.Description != newDesc {
		t.Fatalf("expected description %s, got %s", newDesc, updated.Description)
	}
}

func TestRoleServiceUpdate_RejectsDuplicateCode(t *testing.T) {
	db := newTestDBForRole(t)
	svc := newRoleServiceForTest(t, db)
	_ = seedRoleForTest(t, db, "Admin", "admin", false)
	role := seedRoleForTest(t, db, "Editor", "editor", false)

	newCode := "admin"
	_, err := svc.Update(role.ID, UpdateRoleParams{Code: &newCode})
	if !errors.Is(err, ErrRoleCodeExists) {
		t.Fatalf("expected ErrRoleCodeExists, got %v", err)
	}
}

func TestRoleServiceUpdate_PreventsSystemRoleCodeChange(t *testing.T) {
	db := newTestDBForRole(t)
	svc := newRoleServiceForTest(t, db)
	role := seedRoleForTest(t, db, "Admin", "admin", true)

	newCode := "superadmin"
	_, err := svc.Update(role.ID, UpdateRoleParams{Code: &newCode})
	if !errors.Is(err, ErrSystemRole) {
		t.Fatalf("expected ErrSystemRole, got %v", err)
	}
}

func TestRoleServiceUpdate_AllowsSystemRoleNonCodeChanges(t *testing.T) {
	db := newTestDBForRole(t)
	svc := newRoleServiceForTest(t, db)
	role := seedRoleForTest(t, db, "Admin", "admin", true)

	newName := "Administrator"
	newDesc := "Built-in admin role"
	updated, err := svc.Update(role.ID, UpdateRoleParams{Name: &newName, Description: &newDesc})
	if err != nil {
		t.Fatalf("update system role metadata: %v", err)
	}
	if updated.Code != "admin" {
		t.Fatalf("expected code to remain admin, got %s", updated.Code)
	}
	if updated.Name != newName || updated.Description != newDesc {
		t.Fatalf("expected updated metadata, got %+v", updated)
	}
}

func TestRoleServiceUpdate_MigratesCasbinPoliciesOnCodeChange(t *testing.T) {
	db := newTestDBForRole(t)
	svc := newRoleServiceForTest(t, db)
	role := seedRoleForTest(t, db, "Editor", "editor", false)

	// Seed a Casbin policy for the old code
	oldPolicies := [][]string{{"editor", "/api/v1/posts", "GET"}}
	if err := svc.casbinSvc.SetPoliciesForRole(role.Code, oldPolicies); err != nil {
		t.Fatalf("set policies: %v", err)
	}

	newCode := "senior-editor"
	updated, err := svc.Update(role.ID, UpdateRoleParams{Code: &newCode})
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if updated.Code != newCode {
		t.Fatalf("expected code %s, got %s", newCode, updated.Code)
	}

	oldPoliciesAfter := svc.casbinSvc.GetPoliciesForRole("editor")
	if len(oldPoliciesAfter) != 0 {
		t.Fatalf("expected old policies to be removed, got %v", oldPoliciesAfter)
	}
	newPolicies := svc.casbinSvc.GetPoliciesForRole("senior-editor")
	if len(newPolicies) != 1 || newPolicies[0][1] != "/api/v1/posts" || newPolicies[0][2] != "GET" {
		t.Fatalf("expected policies to be migrated, got %v", newPolicies)
	}
}

func TestRoleServiceUpdate_ReturnsNotFoundForMissing(t *testing.T) {
	db := newTestDBForRole(t)
	svc := newRoleServiceForTest(t, db)

	newName := "X"
	_, err := svc.Update(999, UpdateRoleParams{Name: &newName})
	if !errors.Is(err, ErrRoleNotFound) {
		t.Fatalf("expected ErrRoleNotFound, got %v", err)
	}
}

// 4. Data Scope

func TestRoleServiceUpdateDataScope_Success(t *testing.T) {
	db := newTestDBForRole(t)
	svc := newRoleServiceForTest(t, db)
	role := seedRoleForTest(t, db, "Custom", "custom", false)

	deptIDs := []uint{10, 20}
	updated, err := svc.UpdateDataScope(role.ID, model.DataScopeCustom, deptIDs)
	if err != nil {
		t.Fatalf("update data scope: %v", err)
	}
	if updated.DataScope != model.DataScopeCustom {
		t.Fatalf("expected DataScope=custom, got %s", updated.DataScope)
	}

	var count int64
	if err := db.Model(&model.RoleDeptScope{}).Where("role_id = ?", role.ID).Count(&count).Error; err != nil {
		t.Fatalf("count dept scopes: %v", err)
	}
	if count != 2 {
		t.Fatalf("expected 2 dept scopes, got %d", count)
	}
}

func TestRoleServiceUpdateDataScope_ClearsDeptIDsWhenNotCustom(t *testing.T) {
	db := newTestDBForRole(t)
	svc := newRoleServiceForTest(t, db)
	role := seedRoleForTest(t, db, "Custom", "custom", false)

	// First set custom scope with dept IDs
	if _, err := svc.UpdateDataScope(role.ID, model.DataScopeCustom, []uint{1, 2}); err != nil {
		t.Fatalf("set custom scope: %v", err)
	}

	// Then change to all
	updated, err := svc.UpdateDataScope(role.ID, model.DataScopeAll, []uint{1, 2})
	if err != nil {
		t.Fatalf("update data scope: %v", err)
	}
	if updated.DataScope != model.DataScopeAll {
		t.Fatalf("expected DataScope=all, got %s", updated.DataScope)
	}

	var count int64
	if err := db.Model(&model.RoleDeptScope{}).Where("role_id = ?", role.ID).Count(&count).Error; err != nil {
		t.Fatalf("count dept scopes: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected 0 dept scopes after clearing, got %d", count)
	}
}

func TestRoleServiceUpdateDataScope_CustomWithEmptyDeptIDsClearsExistingScopes(t *testing.T) {
	db := newTestDBForRole(t)
	svc := newRoleServiceForTest(t, db)
	role := seedRoleForTest(t, db, "Custom", "custom", false)

	if _, err := svc.UpdateDataScope(role.ID, model.DataScopeCustom, []uint{1, 2}); err != nil {
		t.Fatalf("set custom scope: %v", err)
	}

	updated, err := svc.UpdateDataScope(role.ID, model.DataScopeCustom, nil)
	if err != nil {
		t.Fatalf("clear custom scope entries: %v", err)
	}
	if updated.DataScope != model.DataScopeCustom {
		t.Fatalf("expected DataScope=custom, got %s", updated.DataScope)
	}

	deptIDs, err := svc.roleRepo.GetCustomDeptIDs(role.ID)
	if err != nil {
		t.Fatalf("get custom dept ids: %v", err)
	}
	if deptIDs == nil {
		t.Fatal("expected empty deptIDs slice, got nil")
	}
	if len(deptIDs) != 0 {
		t.Fatalf("expected custom dept ids to be cleared, got %v", deptIDs)
	}
}

func TestRoleServiceUpdateDataScope_RejectsInvalidScope(t *testing.T) {
	db := newTestDBForRole(t)
	svc := newRoleServiceForTest(t, db)
	role := seedRoleForTest(t, db, "Editor", "editor", false)

	_, err := svc.UpdateDataScope(role.ID, model.DataScope("invalid"), nil)
	if !errors.Is(err, ErrDataScopeInvalid) {
		t.Fatalf("expected ErrDataScopeInvalid, got %v", err)
	}
}

func TestRoleServiceUpdateDataScope_PreventsAdminScopeChange(t *testing.T) {
	db := newTestDBForRole(t)
	svc := newRoleServiceForTest(t, db)
	role := seedRoleForTest(t, db, "Admin", model.RoleAdmin, true)

	_, err := svc.UpdateDataScope(role.ID, model.DataScopeDept, nil)
	if !errors.Is(err, ErrSystemRole) {
		t.Fatalf("expected ErrSystemRole, got %v", err)
	}
}

func TestRoleServiceUpdateDataScope_ReturnsNotFoundForMissing(t *testing.T) {
	db := newTestDBForRole(t)
	svc := newRoleServiceForTest(t, db)

	_, err := svc.UpdateDataScope(999, model.DataScopeAll, nil)
	if !errors.Is(err, ErrRoleNotFound) {
		t.Fatalf("expected ErrRoleNotFound, got %v", err)
	}
}

// 5. Delete

func TestRoleServiceDelete_Success(t *testing.T) {
	db := newTestDBForRole(t)
	svc := newRoleServiceForTest(t, db)
	role := seedRoleForTest(t, db, "Editor", "editor", false)

	// Set custom dept scope and Casbin policies
	if _, err := svc.UpdateDataScope(role.ID, model.DataScopeCustom, []uint{1, 2}); err != nil {
		t.Fatalf("set custom scope: %v", err)
	}
	if err := svc.casbinSvc.SetPoliciesForRole(role.Code, [][]string{{"editor", "/api/v1/posts", "GET"}}); err != nil {
		t.Fatalf("set policies: %v", err)
	}

	if err := svc.Delete(role.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}

	var count int64
	if err := db.Model(&model.Role{}).Where("id = ?", role.ID).Count(&count).Error; err != nil {
		t.Fatalf("count roles: %v", err)
	}
	if count != 0 {
		t.Fatal("expected role to be deleted")
	}

	if err := db.Model(&model.RoleDeptScope{}).Where("role_id = ?", role.ID).Count(&count).Error; err != nil {
		t.Fatalf("count dept scopes: %v", err)
	}
	if count != 0 {
		t.Fatal("expected dept scopes to be cleared")
	}

	policies := svc.casbinSvc.GetPoliciesForRole(role.Code)
	if len(policies) != 0 {
		t.Fatalf("expected casbin policies to be cleared, got %v", policies)
	}
}

func TestRoleServiceDelete_PreventsSystemRoleDeletion(t *testing.T) {
	db := newTestDBForRole(t)
	svc := newRoleServiceForTest(t, db)
	role := seedRoleForTest(t, db, "Admin", "admin", true)

	err := svc.Delete(role.ID)
	if !errors.Is(err, ErrSystemRoleDel) {
		t.Fatalf("expected ErrSystemRoleDel, got %v", err)
	}
}

func TestRoleServiceDelete_PreventsDeletionWhenUsersAssigned(t *testing.T) {
	db := newTestDBForRole(t)
	svc := newRoleServiceForTest(t, db)
	role := seedRoleForTest(t, db, "Editor", "editor", false)
	seedUserForRoleTest(t, db, "alice", role.ID)

	err := svc.Delete(role.ID)
	if !errors.Is(err, ErrRoleHasUsers) {
		t.Fatalf("expected ErrRoleHasUsers, got %v", err)
	}
}

func TestRoleServiceDelete_ReturnsNotFoundForMissing(t *testing.T) {
	db := newTestDBForRole(t)
	svc := newRoleServiceForTest(t, db)

	err := svc.Delete(999)
	if !errors.Is(err, ErrRoleNotFound) {
		t.Fatalf("expected ErrRoleNotFound, got %v", err)
	}
}
