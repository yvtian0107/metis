## MODIFIED Requirements

### Requirement: User model fields
The User model SHALL include `Locale` (string, max 10, e.g., "zh-CN", "en") and `Timezone` (string, max 50, e.g., "Asia/Shanghai") fields. Both fields are optional and default to empty string (meaning "use system default"). These fields MUST be included in the `ToResponse()` output and accepted in create/update user API requests.

#### Scenario: Create user with locale preference
- **WHEN** an admin creates a user with `{"locale": "en", "timezone": "America/New_York", ...}`
- **THEN** the user record stores `locale = "en"` and `timezone = "America/New_York"`

#### Scenario: User with empty locale uses system default
- **WHEN** a user has `locale = ""` in their record
- **THEN** the frontend resolves to the system default locale

## ADDED Requirements

### Requirement: User profile locale and timezone update
Authenticated users SHALL be able to update their own `locale` and `timezone` via `PUT /api/v1/user/profile` (or equivalent profile endpoint). This is separate from admin user management — users control their own language and timezone preferences.

#### Scenario: User updates their locale
- **WHEN** the user sends `PUT /api/v1/user/profile` with `{"locale": "en"}`
- **THEN** the user's locale is updated to "en"
- **AND** the next API response includes the updated user with `locale: "en"`

#### Scenario: User updates their timezone
- **WHEN** the user sends `PUT /api/v1/user/profile` with `{"timezone": "Europe/London"}`
- **THEN** the user's timezone is updated to "Europe/London"

### Requirement: Current user info includes locale and timezone
The `GET /api/v1/user/info` (or equivalent current-user endpoint) SHALL return the user's `locale` and `timezone` fields so the frontend can initialize the correct language and timezone on login.

#### Scenario: User info response includes locale fields
- **WHEN** the frontend fetches current user info after login
- **THEN** the response includes `locale` and `timezone` fields (may be empty strings)
