## ADDED Requirements

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
