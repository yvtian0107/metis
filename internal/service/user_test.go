package service

import (
	"errors"
	"fmt"
	"strconv"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/samber/do/v2"
	"gorm.io/gorm"

	"metis/internal/database"
	"metis/internal/model"
	"metis/internal/pkg/token"
	"metis/internal/repository"
)

func newTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())
	gdb, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	if err := gdb.AutoMigrate(
		&model.User{},
		&model.Role{},
		&model.SystemConfig{},
		&model.RefreshToken{},
	); err != nil {
		t.Fatalf("migrate test db: %v", err)
	}
	return gdb
}

func newUserServiceForTest(t *testing.T, db *gorm.DB) *UserService {
	t.Helper()
	wrapped := &database.DB{DB: db}
	injector := do.New()
	do.ProvideValue(injector, wrapped)
	do.Provide(injector, repository.NewUser)
	do.Provide(injector, repository.NewRefreshToken)
	do.Provide(injector, repository.NewSysConfig)
	do.Provide(injector, NewSettings)

	userRepo := do.MustInvoke[*repository.UserRepo](injector)
	refreshTokenRepo := do.MustInvoke[*repository.RefreshTokenRepo](injector)
	settingsSvc := do.MustInvoke[*SettingsService](injector)

	return &UserService{
		userRepo:         userRepo,
		refreshTokenRepo: refreshTokenRepo,
		settingsSvc:      settingsSvc,
	}
}

func seedRole(t *testing.T, db *gorm.DB, code string) *model.Role {
	t.Helper()
	role := &model.Role{Name: code, Code: code}
	if err := db.Create(role).Error; err != nil {
		t.Fatalf("seed role: %v", err)
	}
	return role
}

func seedUser(t *testing.T, db *gorm.DB, svc *UserService, username string, roleID uint) *model.User {
	t.Helper()
	user, err := svc.Create(username, "Password123!", username+"@example.com", "", roleID)
	if err != nil {
		t.Fatalf("seed user %s: %v", username, err)
	}
	return user
}

func setPasswordPolicy(t *testing.T, db *gorm.DB, minLen int, upper, lower, number, special bool) {
	t.Helper()
	configs := []model.SystemConfig{
		{Key: "security.password_min_length", Value: strconv.Itoa(minLen)},
		{Key: "security.password_require_upper", Value: strconv.FormatBool(upper)},
		{Key: "security.password_require_lower", Value: strconv.FormatBool(lower)},
		{Key: "security.password_require_number", Value: strconv.FormatBool(number)},
		{Key: "security.password_require_special", Value: strconv.FormatBool(special)},
	}
	for _, cfg := range configs {
		if err := db.Save(&cfg).Error; err != nil {
			t.Fatalf("set password policy: %v", err)
		}
	}
}

func createRefreshToken(t *testing.T, db *gorm.DB, userID uint, revoked bool) {
	t.Helper()
	rt := &model.RefreshToken{
		Token:     fmt.Sprintf("token-%d-%d", userID, time.Now().UnixNano()),
		UserID:    userID,
		ExpiresAt: time.Now().Add(24 * time.Hour),
		Revoked:   revoked,
	}
	if err := db.Create(rt).Error; err != nil {
		t.Fatalf("create refresh token: %v", err)
	}
}

func assertTokenRevoked(t *testing.T, db *gorm.DB, userID uint) {
	t.Helper()
	var count int64
	if err := db.Model(&model.RefreshToken{}).Where("user_id = ? AND revoked = ?", userID, false).Count(&count).Error; err != nil {
		t.Fatalf("count refresh tokens: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected all refresh tokens revoked for user %d, found %d active", userID, count)
	}
}

// 2. Create & Retrieve

func TestUserServiceCreate_Success(t *testing.T) {
	db := newTestDB(t)
	svc := newUserServiceForTest(t, db)
	role := seedRole(t, db, "test-role")

	user, err := svc.Create("alice", "Password123!", "alice@example.com", "1234567890", role.ID)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if user.ID == 0 {
		t.Fatal("expected user ID to be set")
	}
	if !user.IsActive {
		t.Fatal("expected IsActive=true")
	}
	if user.PasswordChangedAt == nil {
		t.Fatal("expected PasswordChangedAt to be set")
	}
	if !token.CheckPassword(user.Password, "Password123!") {
		t.Fatal("expected password to be hashed")
	}
	if user.RoleID != role.ID {
		t.Fatalf("expected role ID %d, got %d", role.ID, user.RoleID)
	}
}

func TestUserServiceCreate_RejectsDuplicateUsername(t *testing.T) {
	db := newTestDB(t)
	svc := newUserServiceForTest(t, db)
	role := seedRole(t, db, "test-role")

	if _, err := svc.Create("alice", "Password123!", "alice@example.com", "", role.ID); err != nil {
		t.Fatalf("first create: %v", err)
	}
	_, err := svc.Create("alice", "Password123!", "alice2@example.com", "", role.ID)
	if !errors.Is(err, ErrUsernameExists) {
		t.Fatalf("expected ErrUsernameExists, got %v", err)
	}
}

func TestUserServiceCreate_EnforcesPasswordPolicy(t *testing.T) {
	db := newTestDB(t)
	svc := newUserServiceForTest(t, db)
	role := seedRole(t, db, "test-role")
	setPasswordPolicy(t, db, 12, true, true, true, true)

	_, err := svc.Create("alice", "short", "alice@example.com", "", role.ID)
	if !errors.Is(err, ErrPasswordViolation) {
		t.Fatalf("expected ErrPasswordViolation, got %v", err)
	}
}

func TestUserServiceGetByID_Success(t *testing.T) {
	db := newTestDB(t)
	svc := newUserServiceForTest(t, db)
	role := seedRole(t, db, "test-role")
	user := seedUser(t, db, svc, "alice", role.ID)

	found, err := svc.GetByID(user.ID)
	if err != nil {
		t.Fatalf("get by id: %v", err)
	}
	if found.ID != user.ID {
		t.Fatalf("expected user ID %d, got %d", user.ID, found.ID)
	}
	if found.Role.ID != role.ID {
		t.Fatal("expected Role to be preloaded")
	}
}

func TestUserServiceGetByID_ReturnsNotFoundForMissing(t *testing.T) {
	db := newTestDB(t)
	svc := newUserServiceForTest(t, db)

	_, err := svc.GetByID(999)
	if !errors.Is(err, ErrUserNotFound) {
		t.Fatalf("expected ErrUserNotFound, got %v", err)
	}
}

func TestUserServiceGetByIDWithManager_Success(t *testing.T) {
	db := newTestDB(t)
	svc := newUserServiceForTest(t, db)
	role := seedRole(t, db, "test-role")
	manager := seedUser(t, db, svc, "manager", role.ID)

	user, err := svc.Create("alice", "Password123!", "alice@example.com", "", role.ID)
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	user.ManagerID = &manager.ID
	if err := db.Save(user).Error; err != nil {
		t.Fatalf("set manager: %v", err)
	}

	found, err := svc.GetByIDWithManager(user.ID)
	if err != nil {
		t.Fatalf("get by id with manager: %v", err)
	}
	if found.Manager == nil {
		t.Fatal("expected Manager to be preloaded")
	}
	if found.Manager.ID != manager.ID {
		t.Fatalf("expected manager ID %d, got %d", manager.ID, found.Manager.ID)
	}
}

// 3. Update

func TestUserServiceUpdate_Success(t *testing.T) {
	db := newTestDB(t)
	svc := newUserServiceForTest(t, db)
	role := seedRole(t, db, "test-role")
	manager := seedUser(t, db, svc, "manager", role.ID)
	user := seedUser(t, db, svc, "alice", role.ID)

	newEmail := "alice2@example.com"
	newPhone := "999"
	newManagerID := manager.ID

	updated, err := svc.Update(user.ID, 9999, UpdateUserParams{
		Email:     &newEmail,
		Phone:     &newPhone,
		ManagerID: &newManagerID,
	})
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if updated.Email != newEmail {
		t.Fatalf("expected email %s, got %s", newEmail, updated.Email)
	}
	if updated.Phone != newPhone {
		t.Fatalf("expected phone %s, got %s", newPhone, updated.Phone)
	}
	if updated.ManagerID == nil || *updated.ManagerID != newManagerID {
		t.Fatalf("expected manager ID %d", newManagerID)
	}
}

func TestUserServiceUpdate_PreventsSelfRoleChange(t *testing.T) {
	db := newTestDB(t)
	svc := newUserServiceForTest(t, db)
	role := seedRole(t, db, "test-role")
	user := seedUser(t, db, svc, "alice", role.ID)

	newRoleID := uint(999)
	_, err := svc.Update(user.ID, user.ID, UpdateUserParams{RoleID: &newRoleID})
	if !errors.Is(err, ErrCannotSelf) {
		t.Fatalf("expected ErrCannotSelf, got %v", err)
	}
}

func TestUserServiceUpdate_ReturnsNotFoundForMissing(t *testing.T) {
	db := newTestDB(t)
	svc := newUserServiceForTest(t, db)

	newEmail := "x@example.com"
	_, err := svc.Update(999, 1, UpdateUserParams{Email: &newEmail})
	if !errors.Is(err, ErrUserNotFound) {
		t.Fatalf("expected ErrUserNotFound, got %v", err)
	}
}

func TestUserServiceUpdate_DetectsDirectCircularManager(t *testing.T) {
	db := newTestDB(t)
	svc := newUserServiceForTest(t, db)
	role := seedRole(t, db, "test-role")
	user := seedUser(t, db, svc, "alice", role.ID)

	selfID := user.ID
	_, err := svc.Update(user.ID, 9999, UpdateUserParams{ManagerID: &selfID})
	if !errors.Is(err, ErrCircularManagerChain) {
		t.Fatalf("expected ErrCircularManagerChain, got %v", err)
	}
}

func TestUserServiceUpdate_DetectsIndirectCircularManager(t *testing.T) {
	db := newTestDB(t)
	svc := newUserServiceForTest(t, db)
	role := seedRole(t, db, "test-role")

	// A -> B -> C, then try C -> A
	userA := seedUser(t, db, svc, "user-a", role.ID)
	userB := seedUser(t, db, svc, "user-b", role.ID)
	userC := seedUser(t, db, svc, "user-c", role.ID)

	userB.ManagerID = &userA.ID
	if err := db.Save(userB).Error; err != nil {
		t.Fatalf("set B manager: %v", err)
	}
	userC.ManagerID = &userB.ID
	if err := db.Save(userC).Error; err != nil {
		t.Fatalf("set C manager: %v", err)
	}

	_, err := svc.Update(userA.ID, 9999, UpdateUserParams{ManagerID: &userC.ID})
	if !errors.Is(err, ErrCircularManagerChain) {
		t.Fatalf("expected ErrCircularManagerChain, got %v", err)
	}
}

// 4. Delete, Reset Password & Unlock

func TestUserServiceDelete_Success(t *testing.T) {
	db := newTestDB(t)
	svc := newUserServiceForTest(t, db)
	role := seedRole(t, db, "test-role")
	currentUser := seedUser(t, db, svc, "current", role.ID)
	target := seedUser(t, db, svc, "target", role.ID)
	createRefreshToken(t, db, target.ID, false)

	if err := svc.Delete(target.ID, currentUser.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}

	var count int64
	if err := db.Model(&model.User{}).Where("id = ?", target.ID).Count(&count).Error; err != nil {
		t.Fatalf("count users: %v", err)
	}
	if count != 0 {
		t.Fatal("expected user to be deleted")
	}
	assertTokenRevoked(t, db, target.ID)
}

func TestUserServiceDelete_PreventsSelfDeletion(t *testing.T) {
	db := newTestDB(t)
	svc := newUserServiceForTest(t, db)
	role := seedRole(t, db, "test-role")
	user := seedUser(t, db, svc, "alice", role.ID)

	err := svc.Delete(user.ID, user.ID)
	if !errors.Is(err, ErrCannotSelf) {
		t.Fatalf("expected ErrCannotSelf, got %v", err)
	}
}

func TestUserServiceDelete_ReturnsNotFoundForMissing(t *testing.T) {
	db := newTestDB(t)
	svc := newUserServiceForTest(t, db)

	err := svc.Delete(999, 1)
	if !errors.Is(err, ErrUserNotFound) {
		t.Fatalf("expected ErrUserNotFound, got %v", err)
	}
}

func TestUserServiceResetPassword_Success(t *testing.T) {
	db := newTestDB(t)
	svc := newUserServiceForTest(t, db)
	role := seedRole(t, db, "test-role")
	user := seedUser(t, db, svc, "alice", role.ID)
	createRefreshToken(t, db, user.ID, false)

	// Set ForcePasswordReset to true first
	user.ForcePasswordReset = true
	if err := db.Save(user).Error; err != nil {
		t.Fatalf("set force reset: %v", err)
	}

	if err := svc.ResetPassword(user.ID, "NewPassword123!"); err != nil {
		t.Fatalf("reset password: %v", err)
	}

	var updated model.User
	if err := db.First(&updated, user.ID).Error; err != nil {
		t.Fatalf("reload user: %v", err)
	}
	if !token.CheckPassword(updated.Password, "NewPassword123!") {
		t.Fatal("expected password to be hashed and updated")
	}
	if updated.ForcePasswordReset {
		t.Fatal("expected ForcePasswordReset to be false")
	}
	assertTokenRevoked(t, db, user.ID)
}

func TestUserServiceResetPassword_EnforcesPasswordPolicy(t *testing.T) {
	db := newTestDB(t)
	svc := newUserServiceForTest(t, db)
	role := seedRole(t, db, "test-role")
	user := seedUser(t, db, svc, "alice", role.ID)
	setPasswordPolicy(t, db, 20, true, true, true, true)

	err := svc.ResetPassword(user.ID, "short")
	if !errors.Is(err, ErrPasswordViolation) {
		t.Fatalf("expected ErrPasswordViolation, got %v", err)
	}
}

func TestUserServiceResetPassword_ReturnsNotFoundForMissing(t *testing.T) {
	db := newTestDB(t)
	svc := newUserServiceForTest(t, db)

	err := svc.ResetPassword(999, "NewPassword123!")
	if !errors.Is(err, ErrUserNotFound) {
		t.Fatalf("expected ErrUserNotFound, got %v", err)
	}
}

func TestUserServiceUnlockUser_Success(t *testing.T) {
	db := newTestDB(t)
	svc := newUserServiceForTest(t, db)
	role := seedRole(t, db, "test-role")
	user := seedUser(t, db, svc, "alice", role.ID)

	// Lock the user first
	user.FailedLoginAttempts = 5
	until := time.Now().Add(30 * time.Minute)
	user.LockedUntil = &until
	if err := db.Save(user).Error; err != nil {
		t.Fatalf("lock user: %v", err)
	}

	if err := svc.UnlockUser(user.ID); err != nil {
		t.Fatalf("unlock: %v", err)
	}

	var updated model.User
	if err := db.First(&updated, user.ID).Error; err != nil {
		t.Fatalf("reload user: %v", err)
	}
	if updated.FailedLoginAttempts != 0 {
		t.Fatalf("expected FailedLoginAttempts=0, got %d", updated.FailedLoginAttempts)
	}
	if updated.LockedUntil != nil {
		t.Fatal("expected LockedUntil to be nil")
	}
}

func TestUserServiceUnlockUser_ReturnsNotFoundForMissing(t *testing.T) {
	db := newTestDB(t)
	svc := newUserServiceForTest(t, db)

	err := svc.UnlockUser(999)
	if !errors.Is(err, ErrUserNotFound) {
		t.Fatalf("expected ErrUserNotFound, got %v", err)
	}
}

// 5. Activation, Deactivation & Manager Chain

func TestUserServiceActivate_Success(t *testing.T) {
	db := newTestDB(t)
	svc := newUserServiceForTest(t, db)
	role := seedRole(t, db, "test-role")
	user := seedUser(t, db, svc, "alice", role.ID)

	user.IsActive = false
	if err := db.Save(user).Error; err != nil {
		t.Fatalf("deactivate user: %v", err)
	}

	activated, err := svc.Activate(user.ID)
	if err != nil {
		t.Fatalf("activate: %v", err)
	}
	if !activated.IsActive {
		t.Fatal("expected IsActive=true")
	}
}

func TestUserServiceActivate_ReturnsNotFoundForMissing(t *testing.T) {
	db := newTestDB(t)
	svc := newUserServiceForTest(t, db)

	_, err := svc.Activate(999)
	if !errors.Is(err, ErrUserNotFound) {
		t.Fatalf("expected ErrUserNotFound, got %v", err)
	}
}

func TestUserServiceDeactivate_Success(t *testing.T) {
	db := newTestDB(t)
	svc := newUserServiceForTest(t, db)
	role := seedRole(t, db, "test-role")
	currentUser := seedUser(t, db, svc, "current", role.ID)
	target := seedUser(t, db, svc, "target", role.ID)
	createRefreshToken(t, db, target.ID, false)

	deactivated, err := svc.Deactivate(target.ID, currentUser.ID)
	if err != nil {
		t.Fatalf("deactivate: %v", err)
	}
	if deactivated.IsActive {
		t.Fatal("expected IsActive=false")
	}
	assertTokenRevoked(t, db, target.ID)
}

func TestUserServiceDeactivate_PreventsSelfDeactivation(t *testing.T) {
	db := newTestDB(t)
	svc := newUserServiceForTest(t, db)
	role := seedRole(t, db, "test-role")
	user := seedUser(t, db, svc, "alice", role.ID)

	_, err := svc.Deactivate(user.ID, user.ID)
	if !errors.Is(err, ErrCannotSelf) {
		t.Fatalf("expected ErrCannotSelf, got %v", err)
	}
}

func TestUserServiceGetManagerChain_Success(t *testing.T) {
	db := newTestDB(t)
	svc := newUserServiceForTest(t, db)
	role := seedRole(t, db, "test-role")

	// Chain: user -> mgr1 -> mgr2
	mgr2 := seedUser(t, db, svc, "mgr2", role.ID)
	mgr1, _ := svc.Create("mgr1", "Password123!", "mgr1@example.com", "", role.ID)
	mgr1.ManagerID = &mgr2.ID
	if err := db.Save(mgr1).Error; err != nil {
		t.Fatalf("set mgr1 manager: %v", err)
	}
	user, _ := svc.Create("alice", "Password123!", "alice@example.com", "", role.ID)
	user.ManagerID = &mgr1.ID
	if err := db.Save(user).Error; err != nil {
		t.Fatalf("set user manager: %v", err)
	}

	chain, err := svc.GetManagerChain(user.ID)
	if err != nil {
		t.Fatalf("get manager chain: %v", err)
	}
	if len(chain) != 2 {
		t.Fatalf("expected chain length 2, got %d", len(chain))
	}
	if chain[0].ID != mgr1.ID {
		t.Fatalf("expected first manager %d, got %d", mgr1.ID, chain[0].ID)
	}
	if chain[1].ID != mgr2.ID {
		t.Fatalf("expected second manager %d, got %d", mgr2.ID, chain[1].ID)
	}
}

func TestUserServiceGetManagerChain_ReturnsNotFoundForMissing(t *testing.T) {
	db := newTestDB(t)
	svc := newUserServiceForTest(t, db)

	_, err := svc.GetManagerChain(999)
	if !errors.Is(err, ErrUserNotFound) {
		t.Fatalf("expected ErrUserNotFound, got %v", err)
	}
}

func TestUserServiceGetManagerChain_BreaksOnCycle(t *testing.T) {
	db := newTestDB(t)
	svc := newUserServiceForTest(t, db)
	role := seedRole(t, db, "test-role")

	userA := seedUser(t, db, svc, "user-a", role.ID)
	userB := seedUser(t, db, svc, "user-b", role.ID)

	// A -> B -> A (cycle)
	userA.ManagerID = &userB.ID
	if err := db.Save(userA).Error; err != nil {
		t.Fatalf("set A manager: %v", err)
	}
	userB.ManagerID = &userA.ID
	if err := db.Save(userB).Error; err != nil {
		t.Fatalf("set B manager: %v", err)
	}

	chain, err := svc.GetManagerChain(userA.ID)
	if err != nil {
		t.Fatalf("get manager chain: %v", err)
	}
	if len(chain) != 1 {
		t.Fatalf("expected chain length 1 (break before cycle), got %d", len(chain))
	}
	if chain[0].ID != userB.ID {
		t.Fatalf("expected manager %d, got %d", userB.ID, chain[0].ID)
	}
}

func TestUserServiceGetManagerChain_RespectsMaxDepth(t *testing.T) {
	db := newTestDB(t)
	svc := newUserServiceForTest(t, db)
	role := seedRole(t, db, "test-role")

	// Create 12 managers in a chain
	var prev *model.User
	for i := 0; i < 12; i++ {
		u, _ := svc.Create(fmt.Sprintf("mgr%d", i), "Password123!", fmt.Sprintf("mgr%d@example.com", i), "", role.ID)
		if prev != nil {
			u.ManagerID = &prev.ID
			if err := db.Save(u).Error; err != nil {
				t.Fatalf("set manager: %v", err)
			}
		}
		prev = u
	}
	// prev is the deepest user with 11 ancestors in chain
	chain, err := svc.GetManagerChain(prev.ID)
	if err != nil {
		t.Fatalf("get manager chain: %v", err)
	}
	if len(chain) != 10 {
		t.Fatalf("expected chain length 10, got %d", len(chain))
	}
}

func TestUserServiceClearManager_Success(t *testing.T) {
	db := newTestDB(t)
	svc := newUserServiceForTest(t, db)
	role := seedRole(t, db, "test-role")
	manager := seedUser(t, db, svc, "manager", role.ID)
	user := seedUser(t, db, svc, "alice", role.ID)
	user.ManagerID = &manager.ID
	if err := db.Save(user).Error; err != nil {
		t.Fatalf("set manager: %v", err)
	}

	cleared, err := svc.ClearManager(user.ID)
	if err != nil {
		t.Fatalf("clear manager: %v", err)
	}
	if cleared.ManagerID != nil {
		t.Fatalf("expected ManagerID nil, got %v", *cleared.ManagerID)
	}
}

func TestUserServiceClearManager_ReturnsNotFoundForMissing(t *testing.T) {
	db := newTestDB(t)
	svc := newUserServiceForTest(t, db)

	_, err := svc.ClearManager(999)
	if !errors.Is(err, ErrUserNotFound) {
		t.Fatalf("expected ErrUserNotFound, got %v", err)
	}
}

func TestUserServiceList_PreloadsManagerForManagedUsers(t *testing.T) {
	db := newTestDB(t)
	svc := newUserServiceForTest(t, db)
	role := seedRole(t, db, "test-role")
	manager := seedUser(t, db, svc, "manager", role.ID)
	user := seedUser(t, db, svc, "alice", role.ID)
	user.ManagerID = &manager.ID
	if err := db.Save(user).Error; err != nil {
		t.Fatalf("set manager: %v", err)
	}

	result, err := svc.List(repository.ListParams{Page: 1, PageSize: 20})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(result.Items) != 2 {
		t.Fatalf("expected 2 users, got %d", len(result.Items))
	}
	if result.Items[0].ID != user.ID {
		t.Fatalf("expected managed user %d first, got %d", user.ID, result.Items[0].ID)
	}
	if result.Items[0].Manager == nil {
		t.Fatal("expected manager to be preloaded for list item")
	}
	if result.Items[0].Manager.ID != manager.ID {
		t.Fatalf("expected manager ID %d, got %d", manager.ID, result.Items[0].Manager.ID)
	}
}

func TestUserServiceCreateWithParams_SetsManagerWhenProvided(t *testing.T) {
	db := newTestDB(t)
	svc := newUserServiceForTest(t, db)
	role := seedRole(t, db, "test-role")
	manager := seedUser(t, db, svc, "manager", role.ID)

	user, err := svc.CreateWithParams(CreateUserParams{
		Username:  "alice",
		Password:  "Password123!",
		Email:     "alice@example.com",
		Phone:     "1234567890",
		RoleID:    role.ID,
		ManagerID: &manager.ID,
	})
	if err != nil {
		t.Fatalf("create with params: %v", err)
	}
	if user.ManagerID == nil || *user.ManagerID != manager.ID {
		t.Fatalf("expected manager ID %d, got %v", manager.ID, user.ManagerID)
	}
	if user.Manager == nil || user.Manager.ID != manager.ID {
		t.Fatalf("expected manager to be preloaded, got %+v", user.Manager)
	}
}

func TestUserServiceList_SearchesAcrossUsernameEmailAndPhone(t *testing.T) {
	db := newTestDB(t)
	svc := newUserServiceForTest(t, db)
	role := seedRole(t, db, "test-role")
	if _, err := svc.Create("alice", "Password123!", "alice@example.com", "111", role.ID); err != nil {
		t.Fatalf("create alice: %v", err)
	}
	if _, err := svc.Create("bob", "Password123!", "bob@example.com", "222", role.ID); err != nil {
		t.Fatalf("create bob: %v", err)
	}
	if _, err := svc.Create("charlie", "Password123!", "charlie@example.com", "333", role.ID); err != nil {
		t.Fatalf("create charlie: %v", err)
	}

	usernameResult, err := svc.List(repository.ListParams{Keyword: "ali", Page: 1, PageSize: 20})
	if err != nil {
		t.Fatalf("list by username: %v", err)
	}
	if len(usernameResult.Items) != 1 || usernameResult.Items[0].Username != "alice" {
		t.Fatalf("expected only alice by username search, got %+v", usernameResult.Items)
	}

	emailResult, err := svc.List(repository.ListParams{Keyword: "bob@example", Page: 1, PageSize: 20})
	if err != nil {
		t.Fatalf("list by email: %v", err)
	}
	if len(emailResult.Items) != 1 || emailResult.Items[0].Username != "bob" {
		t.Fatalf("expected only bob by email search, got %+v", emailResult.Items)
	}

	phoneResult, err := svc.List(repository.ListParams{Keyword: "333", Page: 1, PageSize: 20})
	if err != nil {
		t.Fatalf("list by phone: %v", err)
	}
	if len(phoneResult.Items) != 1 || phoneResult.Items[0].Username != "charlie" {
		t.Fatalf("expected only charlie by phone search, got %+v", phoneResult.Items)
	}
}

func TestUserServiceList_AppliesActiveFilterAndDefaultPagination(t *testing.T) {
	db := newTestDB(t)
	svc := newUserServiceForTest(t, db)
	role := seedRole(t, db, "test-role")
	for i := 0; i < 25; i++ {
		user := seedUser(t, db, svc, fmt.Sprintf("user-%02d", i), role.ID)
		if i%2 == 0 {
			user.IsActive = false
			if err := db.Save(user).Error; err != nil {
				t.Fatalf("deactivate user %d: %v", i, err)
			}
		}
	}

	active := true
	result, err := svc.List(repository.ListParams{IsActive: &active, Page: 0, PageSize: 0})
	if err != nil {
		t.Fatalf("list active users: %v", err)
	}
	if result.Total != 12 {
		t.Fatalf("expected 12 active users, got %d", result.Total)
	}
	if len(result.Items) != 12 {
		t.Fatalf("expected 12 items on first page, got %d", len(result.Items))
	}
	for _, item := range result.Items {
		if !item.IsActive {
			t.Fatalf("expected only active users, found inactive user %d", item.ID)
		}
	}

	unfiltered, err := svc.List(repository.ListParams{Page: 0, PageSize: 0})
	if err != nil {
		t.Fatalf("list default pagination: %v", err)
	}
	if unfiltered.Total != 25 {
		t.Fatalf("expected total 25 users, got %d", unfiltered.Total)
	}
	if len(unfiltered.Items) != 20 {
		t.Fatalf("expected default page size 20, got %d", len(unfiltered.Items))
	}
}

func TestUserServiceUpdateProfile_Success(t *testing.T) {
	db := newTestDB(t)
	svc := newUserServiceForTest(t, db)
	role := seedRole(t, db, "test-role")
	user := seedUser(t, db, svc, "alice", role.ID)
	user.Locale = "zh-CN"
	user.Timezone = "Asia/Shanghai"

	if err := svc.UpdateProfile(user); err != nil {
		t.Fatalf("update profile: %v", err)
	}

	updated, err := svc.GetByID(user.ID)
	if err != nil {
		t.Fatalf("reload user: %v", err)
	}
	if updated.Locale != "zh-CN" {
		t.Fatalf("expected locale zh-CN, got %s", updated.Locale)
	}
	if updated.Timezone != "Asia/Shanghai" {
		t.Fatalf("expected timezone Asia/Shanghai, got %s", updated.Timezone)
	}
}
