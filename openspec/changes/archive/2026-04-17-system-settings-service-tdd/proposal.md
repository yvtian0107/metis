## Why

The system settings module (`SettingsService`) currently lacks service-layer test coverage. Following the established TDD pattern for kernel services, we need dedicated tests to ensure configuration group getters/setters, default value fallback, validation logic, and convenience getters are correct and regression-safe.

## What Changes

- Add `internal/service/settings_test.go` with TDD-style service-layer tests for `SettingsService`.
- Cover `GetSecuritySettings`, `UpdateSecuritySettings`, `GetSchedulerSettings`, `UpdateSchedulerSettings`, and convenience getters.
- Use in-memory SQLite with real `SysConfigRepo` (no mocks), consistent with existing kernel service test patterns.
- Add test requirements to the `settings-page` and `system-config` capability specs.

## Capabilities

### New Capabilities
- `system-settings-service-test`: Service-layer test requirements and scenarios for system settings management.

### Modified Capabilities
- `settings-page`: Add requirements covering service-layer test scenarios for security and scheduler settings.
- `system-config`: Add requirements covering service-layer test scenarios for config getter/setter defaults and validation.

## Impact

- `internal/service/settings_test.go` (new)
- `openspec/specs/settings-page/spec.md` (modified)
- `openspec/specs/system-config/spec.md` (modified)
- No breaking changes to APIs or frontend.
