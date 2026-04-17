## 1. Test Infrastructure

- [x] 1.1 Create `internal/service/settings_test.go` with `newTestDBForSettings`, `newSettingsServiceForTest`, and `seedSystemConfig` helpers using in-memory SQLite and real `SysConfigRepo`.

## 2. Security Settings Tests

- [x] 2.1 Implement `TestSettingsServiceGetSecuritySettings_Defaults` to verify default values when no configs exist.
- [x] 2.2 Implement `TestSettingsServiceGetSecuritySettings_StoredValues` to verify stored values override defaults.
- [x] 2.3 Implement `TestSettingsServiceUpdateSecuritySettings_Validation` to verify boundary corrections (PasswordMinLength, SessionTimeoutMinutes, CaptchaProvider).
- [x] 2.4 Implement `TestSettingsServiceUpdateSecuritySettings_PersistsAllFields` to verify round-trip read/write.

## 3. Scheduler Settings Tests

- [x] 3.1 Implement `TestSettingsServiceGetSchedulerSettings_Defaults` to verify default values.
- [x] 3.2 Implement `TestSettingsServiceUpdateSchedulerSettings_PersistsFields` to verify round-trip read/write.

## 4. Convenience Getter Tests

- [x] 4.1 Implement `TestSettingsServiceGetPasswordPolicy` to verify mapping to `token.PasswordPolicy`.
- [x] 4.2 Implement `TestSettingsServiceGetSessionTimeoutMinutes_Fallback` to verify fallback for invalid values.
- [x] 4.3 Implement `TestSettingsServiceGetCaptchaProvider` to verify stored/default behavior.
- [x] 4.4 Implement `TestSettingsServiceIsRegistrationOpen` to verify boolean parsing.
- [x] 4.5 Implement `TestSettingsServiceGetDefaultRoleCode` to verify string retrieval.
- [x] 4.6 Implement `TestSettingsServiceIsTwoFactorRequired` to verify boolean parsing.
- [x] 4.7 Implement `TestSettingsServiceGetPasswordExpiryDays` to verify int retrieval.
- [x] 4.8 Implement `TestSettingsServiceGetLoginLockoutSettings` to verify pair return.

## 5. Verification

- [x] 5.1 Run `go test ./internal/service/ -run TestSettingsService -v` and ensure all tests pass.
- [x] 5.2 Run `go test ./...` to confirm no regressions.
