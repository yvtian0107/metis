## ADDED Requirements

### Requirement: Test security settings service-layer
The service-layer test suite SHALL verify that `GetSecuritySettings` returns stored values with correct defaults and that `UpdateSecuritySettings` validates and persists configurations.

#### Scenario: Get security settings with defaults
- **WHEN** no security configs exist and `GetSecuritySettings` is called
- **THEN** it returns default values (maxConcurrentSessions=5, sessionTimeoutMinutes=10080, passwordMinLength=8, loginMaxAttempts=5, loginLockoutMinutes=30, captchaProvider="none")

#### Scenario: Get security settings with stored values
- **WHEN** security configs have been set in SystemConfig and `GetSecuritySettings` is called
- **THEN** it returns the stored values overriding defaults

#### Scenario: Update security settings with validation
- **WHEN** `UpdateSecuritySettings` is called with PasswordMinLength=0, SessionTimeoutMinutes=0, and an invalid CaptchaProvider
- **THEN** the stored values are corrected (PasswordMinLength=1, SessionTimeoutMinutes=10080, CaptchaProvider="none")

#### Scenario: Update security settings persists all fields
- **WHEN** `UpdateSecuritySettings` is called with valid settings
- **THEN** all corresponding SystemConfig keys are updated and readable via `GetSecuritySettings`

### Requirement: Test scheduler settings service-layer
The service-layer test suite SHALL verify that `GetSchedulerSettings` returns stored values with defaults and that `UpdateSchedulerSettings` persists configurations.

#### Scenario: Get scheduler settings with defaults
- **WHEN** no scheduler configs exist and `GetSchedulerSettings` is called
- **THEN** it returns default values (historyRetentionDays=30, auditRetentionDaysAuth=90, auditRetentionDaysOperation=365)

#### Scenario: Update scheduler settings persists fields
- **WHEN** `UpdateSchedulerSettings` is called with custom values
- **THEN** all corresponding SystemConfig keys are updated and readable via `GetSchedulerSettings`
