## ADDED Requirements

### Requirement: Settings service test infrastructure
The system SHALL provide a test harness for `SettingsService` using an in-memory SQLite database and real `SysConfigRepo`, consistent with other kernel service tests.

#### Scenario: Setup test database
- **WHEN** a settings service test initializes
- **THEN** it SHALL migrate the `SystemConfig` table into a shared-memory SQLite database

#### Scenario: Setup DI container
- **WHEN** a settings service test needs dependencies
- **THEN** it SHALL use `samber/do` to provide the database, `SysConfigRepo`, and `SettingsService`

### Requirement: Test security settings retrieval and update
The service-layer test suite SHALL verify that `GetSecuritySettings` and `UpdateSecuritySettings` behave correctly with defaults, stored values, and validation.

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

### Requirement: Test scheduler settings retrieval and update
The service-layer test suite SHALL verify that `GetSchedulerSettings` and `UpdateSchedulerSettings` behave correctly with defaults and updates.

#### Scenario: Get scheduler settings with defaults
- **WHEN** no scheduler configs exist and `GetSchedulerSettings` is called
- **THEN** it returns default values (historyRetentionDays=30, auditRetentionDaysAuth=90, auditRetentionDaysOperation=365)

#### Scenario: Update scheduler settings persists fields
- **WHEN** `UpdateSchedulerSettings` is called with custom values
- **THEN** all corresponding SystemConfig keys are updated and readable via `GetSchedulerSettings`

### Requirement: Test settings service convenience getters
The service-layer test suite SHALL verify that convenience getters on `SettingsService` correctly read SystemConfig values and fall back to sensible defaults.

#### Scenario: GetPasswordPolicy returns mapped policy
- **WHEN** password policy configs exist in SystemConfig
- **THEN** `GetPasswordPolicy` returns a `PasswordPolicy` struct with those values

#### Scenario: GetSessionTimeoutMinutes falls back for invalid values
- **WHEN** `security.session_timeout_minutes` is set to a value less than or equal to 0
- **THEN** `GetSessionTimeoutMinutes` returns the default 10080

#### Scenario: GetCaptchaProvider returns stored or default
- **WHEN** `security.captcha_provider` is set to "image"
- **THEN** `GetCaptchaProvider` returns "image"; when unset it returns "none"

#### Scenario: IsRegistrationOpen returns boolean
- **WHEN** `security.registration_open` is set to "true"
- **THEN** `IsRegistrationOpen` returns true; when unset it returns false

#### Scenario: GetDefaultRoleCode returns stored or empty
- **WHEN** `security.default_role_code` is set to "editor"
- **THEN** `GetDefaultRoleCode` returns "editor"; when unset it returns ""

#### Scenario: IsTwoFactorRequired returns boolean
- **WHEN** `security.require_two_factor` is set to "true"
- **THEN** `IsTwoFactorRequired` returns true; when unset it returns false

#### Scenario: GetPasswordExpiryDays returns stored or zero
- **WHEN** `security.password_expiry_days` is set to 90
- **THEN** `GetPasswordExpiryDays` returns 90; when unset it returns 0

#### Scenario: GetLoginLockoutSettings returns pair
- **WHEN** login lockout configs exist
- **THEN** `GetLoginLockoutSettings` returns the configured max attempts and lockout minutes
