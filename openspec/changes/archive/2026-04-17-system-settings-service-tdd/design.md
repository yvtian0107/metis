## Context

The `SettingsService` (`internal/service/settings.go`) provides grouped configuration access for security settings, scheduler settings, and various convenience getters used across the kernel. It relies on `SysConfigRepo` for K/V persistence. There is currently no dedicated service-layer test file. We will follow the established test pattern: in-memory SQLite with a real `samber/do` injector and real repository.

## Goals / Non-Goals

**Goals:**
- Add comprehensive `internal/service/settings_test.go` covering all `SettingsService` public methods.
- Verify default value fallback, type conversion, update validation, and convenience getter behavior.
- Keep tests fast and deterministic with shared-memory SQLite.

**Non-Goals:**
- No handler-level or frontend tests.
- No changes to production code unless tests reveal bugs.

## Decisions

1. **Test database**: Use `file:test?mode=memory&cache=shared` SQLite DSN, migrating only the `SystemConfig` table.
2. **Seed helper**: Create `seedSystemConfig` to quickly insert K/V entries for testing getters and updates.
3. **Validation coverage**: Assert `UpdateSecuritySettings` boundary behaviors (`PasswordMinLength < 1`, `SessionTimeoutMinutes < 1`, invalid `CaptchaProvider`).
