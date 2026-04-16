package service

import (
	"fmt"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/samber/do/v2"
	"gorm.io/gorm"

	"metis/internal/database"
	"metis/internal/model"
	"metis/internal/repository"
)

func newTestDBForSettings(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())
	gdb, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	if err := gdb.AutoMigrate(&model.SystemConfig{}); err != nil {
		t.Fatalf("migrate test db: %v", err)
	}
	return gdb
}

func newSettingsServiceForTest(t *testing.T, db *gorm.DB) *SettingsService {
	t.Helper()
	wrapped := &database.DB{DB: db}
	injector := do.New()
	do.ProvideValue(injector, wrapped)
	do.Provide(injector, repository.NewSysConfig)
	do.Provide(injector, NewSettings)
	return do.MustInvoke[*SettingsService](injector)
}

func seedSystemConfig(t *testing.T, db *gorm.DB, key, value string) {
	t.Helper()
	if err := db.Save(&model.SystemConfig{Key: key, Value: value}).Error; err != nil {
		t.Fatalf("seed system config: %v", err)
	}
}

// 1. Security Settings

func TestSettingsServiceGetSecuritySettings_Defaults(t *testing.T) {
	db := newTestDBForSettings(t)
	svc := newSettingsServiceForTest(t, db)

	s := svc.GetSecuritySettings()
	if s.MaxConcurrentSessions != 5 {
		t.Fatalf("expected MaxConcurrentSessions=5, got %d", s.MaxConcurrentSessions)
	}
	if s.SessionTimeoutMinutes != 10080 {
		t.Fatalf("expected SessionTimeoutMinutes=10080, got %d", s.SessionTimeoutMinutes)
	}
	if s.PasswordMinLength != 8 {
		t.Fatalf("expected PasswordMinLength=8, got %d", s.PasswordMinLength)
	}
	if s.LoginMaxAttempts != 5 {
		t.Fatalf("expected LoginMaxAttempts=5, got %d", s.LoginMaxAttempts)
	}
	if s.LoginLockoutMinutes != 30 {
		t.Fatalf("expected LoginLockoutMinutes=30, got %d", s.LoginLockoutMinutes)
	}
	if s.CaptchaProvider != "none" {
		t.Fatalf("expected CaptchaProvider=none, got %s", s.CaptchaProvider)
	}
	if s.PasswordRequireUpper || s.PasswordRequireLower || s.PasswordRequireNumber || s.PasswordRequireSpecial {
		t.Fatal("expected password requirements to be false by default")
	}
	if s.RequireTwoFactor || s.RegistrationOpen {
		t.Fatal("expected RequireTwoFactor and RegistrationOpen to be false by default")
	}
	if s.DefaultRoleCode != "" {
		t.Fatalf("expected DefaultRoleCode empty, got %s", s.DefaultRoleCode)
	}
}

func TestSettingsServiceGetSecuritySettings_StoredValues(t *testing.T) {
	db := newTestDBForSettings(t)
	svc := newSettingsServiceForTest(t, db)

	seedSystemConfig(t, db, "security.max_concurrent_sessions", "10")
	seedSystemConfig(t, db, "security.session_timeout_minutes", "60")
	seedSystemConfig(t, db, "security.password_min_length", "12")
	seedSystemConfig(t, db, "security.login_max_attempts", "3")
	seedSystemConfig(t, db, "security.login_lockout_minutes", "15")
	seedSystemConfig(t, db, "security.captcha_provider", "image")
	seedSystemConfig(t, db, "security.password_require_upper", "true")
	seedSystemConfig(t, db, "security.require_two_factor", "true")

	s := svc.GetSecuritySettings()
	if s.MaxConcurrentSessions != 10 {
		t.Fatalf("expected MaxConcurrentSessions=10, got %d", s.MaxConcurrentSessions)
	}
	if s.SessionTimeoutMinutes != 60 {
		t.Fatalf("expected SessionTimeoutMinutes=60, got %d", s.SessionTimeoutMinutes)
	}
	if s.PasswordMinLength != 12 {
		t.Fatalf("expected PasswordMinLength=12, got %d", s.PasswordMinLength)
	}
	if s.LoginMaxAttempts != 3 {
		t.Fatalf("expected LoginMaxAttempts=3, got %d", s.LoginMaxAttempts)
	}
	if s.LoginLockoutMinutes != 15 {
		t.Fatalf("expected LoginLockoutMinutes=15, got %d", s.LoginLockoutMinutes)
	}
	if s.CaptchaProvider != "image" {
		t.Fatalf("expected CaptchaProvider=image, got %s", s.CaptchaProvider)
	}
	if !s.PasswordRequireUpper {
		t.Fatal("expected PasswordRequireUpper=true")
	}
	if !s.RequireTwoFactor {
		t.Fatal("expected RequireTwoFactor=true")
	}
}

func TestSettingsServiceUpdateSecuritySettings_Validation(t *testing.T) {
	db := newTestDBForSettings(t)
	svc := newSettingsServiceForTest(t, db)

	req := SecuritySettings{
		PasswordMinLength:     0,
		SessionTimeoutMinutes: 0,
		CaptchaProvider:       "invalid",
	}
	if err := svc.UpdateSecuritySettings(req); err != nil {
		t.Fatalf("update security settings: %v", err)
	}

	s := svc.GetSecuritySettings()
	if s.PasswordMinLength != 1 {
		t.Fatalf("expected PasswordMinLength=1 after validation, got %d", s.PasswordMinLength)
	}
	if s.SessionTimeoutMinutes != 10080 {
		t.Fatalf("expected SessionTimeoutMinutes=10080 after validation, got %d", s.SessionTimeoutMinutes)
	}
	if s.CaptchaProvider != "none" {
		t.Fatalf("expected CaptchaProvider=none after validation, got %s", s.CaptchaProvider)
	}
}

func TestSettingsServiceUpdateSecuritySettings_PersistsAllFields(t *testing.T) {
	db := newTestDBForSettings(t)
	svc := newSettingsServiceForTest(t, db)

	req := SecuritySettings{
		MaxConcurrentSessions:  8,
		SessionTimeoutMinutes:  120,
		PasswordMinLength:      10,
		PasswordRequireUpper:   true,
		PasswordRequireLower:   true,
		PasswordRequireNumber:  true,
		PasswordRequireSpecial: true,
		PasswordExpiryDays:     90,
		LoginMaxAttempts:       4,
		LoginLockoutMinutes:    20,
		CaptchaProvider:        "image",
		RequireTwoFactor:       true,
		RegistrationOpen:       true,
		DefaultRoleCode:        "editor",
	}
	if err := svc.UpdateSecuritySettings(req); err != nil {
		t.Fatalf("update security settings: %v", err)
	}

	s := svc.GetSecuritySettings()
	if s.MaxConcurrentSessions != 8 {
		t.Fatalf("expected MaxConcurrentSessions=8, got %d", s.MaxConcurrentSessions)
	}
	if s.SessionTimeoutMinutes != 120 {
		t.Fatalf("expected SessionTimeoutMinutes=120, got %d", s.SessionTimeoutMinutes)
	}
	if s.PasswordMinLength != 10 {
		t.Fatalf("expected PasswordMinLength=10, got %d", s.PasswordMinLength)
	}
	if !s.PasswordRequireUpper || !s.PasswordRequireLower || !s.PasswordRequireNumber || !s.PasswordRequireSpecial {
		t.Fatal("expected all password requirements to be true")
	}
	if s.PasswordExpiryDays != 90 {
		t.Fatalf("expected PasswordExpiryDays=90, got %d", s.PasswordExpiryDays)
	}
	if s.LoginMaxAttempts != 4 {
		t.Fatalf("expected LoginMaxAttempts=4, got %d", s.LoginMaxAttempts)
	}
	if s.LoginLockoutMinutes != 20 {
		t.Fatalf("expected LoginLockoutMinutes=20, got %d", s.LoginLockoutMinutes)
	}
	if s.CaptchaProvider != "image" {
		t.Fatalf("expected CaptchaProvider=image, got %s", s.CaptchaProvider)
	}
	if !s.RequireTwoFactor {
		t.Fatal("expected RequireTwoFactor=true")
	}
	if !s.RegistrationOpen {
		t.Fatal("expected RegistrationOpen=true")
	}
	if s.DefaultRoleCode != "editor" {
		t.Fatalf("expected DefaultRoleCode=editor, got %s", s.DefaultRoleCode)
	}
}

// 2. Scheduler Settings

func TestSettingsServiceGetSchedulerSettings_Defaults(t *testing.T) {
	db := newTestDBForSettings(t)
	svc := newSettingsServiceForTest(t, db)

	s := svc.GetSchedulerSettings()
	if s.HistoryRetentionDays != 30 {
		t.Fatalf("expected HistoryRetentionDays=30, got %d", s.HistoryRetentionDays)
	}
	if s.AuditRetentionDaysAuth != 90 {
		t.Fatalf("expected AuditRetentionDaysAuth=90, got %d", s.AuditRetentionDaysAuth)
	}
	if s.AuditRetentionDaysOperation != 365 {
		t.Fatalf("expected AuditRetentionDaysOperation=365, got %d", s.AuditRetentionDaysOperation)
	}
}

func TestSettingsServiceUpdateSchedulerSettings_PersistsFields(t *testing.T) {
	db := newTestDBForSettings(t)
	svc := newSettingsServiceForTest(t, db)

	req := SchedulerSettings{
		HistoryRetentionDays:        7,
		AuditRetentionDaysAuth:      30,
		AuditRetentionDaysOperation: 180,
	}
	if err := svc.UpdateSchedulerSettings(req); err != nil {
		t.Fatalf("update scheduler settings: %v", err)
	}

	s := svc.GetSchedulerSettings()
	if s.HistoryRetentionDays != 7 {
		t.Fatalf("expected HistoryRetentionDays=7, got %d", s.HistoryRetentionDays)
	}
	if s.AuditRetentionDaysAuth != 30 {
		t.Fatalf("expected AuditRetentionDaysAuth=30, got %d", s.AuditRetentionDaysAuth)
	}
	if s.AuditRetentionDaysOperation != 180 {
		t.Fatalf("expected AuditRetentionDaysOperation=180, got %d", s.AuditRetentionDaysOperation)
	}
}

// 3. Convenience Getters

func TestSettingsServiceGetPasswordPolicy(t *testing.T) {
	db := newTestDBForSettings(t)
	svc := newSettingsServiceForTest(t, db)

	seedSystemConfig(t, db, "security.password_min_length", "12")
	seedSystemConfig(t, db, "security.password_require_upper", "true")
	seedSystemConfig(t, db, "security.password_require_special", "true")

	policy := svc.GetPasswordPolicy()
	if policy.MinLength != 12 {
		t.Fatalf("expected MinLength=12, got %d", policy.MinLength)
	}
	if !policy.RequireUpper {
		t.Fatal("expected RequireUpper=true")
	}
	if policy.RequireLower {
		t.Fatal("expected RequireLower=false")
	}
	if !policy.RequireSpecial {
		t.Fatal("expected RequireSpecial=true")
	}
}

func TestSettingsServiceGetSessionTimeoutMinutes_Fallback(t *testing.T) {
	db := newTestDBForSettings(t)
	svc := newSettingsServiceForTest(t, db)

	seedSystemConfig(t, db, "security.session_timeout_minutes", "-1")
	if v := svc.GetSessionTimeoutMinutes(); v != 10080 {
		t.Fatalf("expected fallback 10080 for <=0, got %d", v)
	}

	seedSystemConfig(t, db, "security.session_timeout_minutes", "0")
	if v := svc.GetSessionTimeoutMinutes(); v != 10080 {
		t.Fatalf("expected fallback 10080 for 0, got %d", v)
	}
}

func TestSettingsServiceGetCaptchaProvider(t *testing.T) {
	db := newTestDBForSettings(t)
	svc := newSettingsServiceForTest(t, db)

	if v := svc.GetCaptchaProvider(); v != "none" {
		t.Fatalf("expected default none, got %s", v)
	}

	seedSystemConfig(t, db, "security.captcha_provider", "image")
	if v := svc.GetCaptchaProvider(); v != "image" {
		t.Fatalf("expected image, got %s", v)
	}
}

func TestSettingsServiceIsRegistrationOpen(t *testing.T) {
	db := newTestDBForSettings(t)
	svc := newSettingsServiceForTest(t, db)

	if svc.IsRegistrationOpen() {
		t.Fatal("expected default false")
	}

	seedSystemConfig(t, db, "security.registration_open", "true")
	if !svc.IsRegistrationOpen() {
		t.Fatal("expected true")
	}
}

func TestSettingsServiceGetDefaultRoleCode(t *testing.T) {
	db := newTestDBForSettings(t)
	svc := newSettingsServiceForTest(t, db)

	if v := svc.GetDefaultRoleCode(); v != "" {
		t.Fatalf("expected default empty, got %s", v)
	}

	seedSystemConfig(t, db, "security.default_role_code", "editor")
	if v := svc.GetDefaultRoleCode(); v != "editor" {
		t.Fatalf("expected editor, got %s", v)
	}
}

func TestSettingsServiceIsTwoFactorRequired(t *testing.T) {
	db := newTestDBForSettings(t)
	svc := newSettingsServiceForTest(t, db)

	if svc.IsTwoFactorRequired() {
		t.Fatal("expected default false")
	}

	seedSystemConfig(t, db, "security.require_two_factor", "true")
	if !svc.IsTwoFactorRequired() {
		t.Fatal("expected true")
	}
}

func TestSettingsServiceGetPasswordExpiryDays(t *testing.T) {
	db := newTestDBForSettings(t)
	svc := newSettingsServiceForTest(t, db)

	if v := svc.GetPasswordExpiryDays(); v != 0 {
		t.Fatalf("expected default 0, got %d", v)
	}

	seedSystemConfig(t, db, "security.password_expiry_days", "90")
	if v := svc.GetPasswordExpiryDays(); v != 90 {
		t.Fatalf("expected 90, got %d", v)
	}
}

func TestSettingsServiceGetLoginLockoutSettings(t *testing.T) {
	db := newTestDBForSettings(t)
	svc := newSettingsServiceForTest(t, db)

	maxAttempts, lockoutMinutes := svc.GetLoginLockoutSettings()
	if maxAttempts != 5 || lockoutMinutes != 30 {
		t.Fatalf("expected defaults (5,30), got (%d,%d)", maxAttempts, lockoutMinutes)
	}

	seedSystemConfig(t, db, "security.login_max_attempts", "3")
	seedSystemConfig(t, db, "security.login_lockout_minutes", "15")

	maxAttempts, lockoutMinutes = svc.GetLoginLockoutSettings()
	if maxAttempts != 3 || lockoutMinutes != 15 {
		t.Fatalf("expected (3,15), got (%d,%d)", maxAttempts, lockoutMinutes)
	}
}
