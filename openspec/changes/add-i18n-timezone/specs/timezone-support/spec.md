## ADDED Requirements

### Requirement: Database stores UTC only
All timestamps in the database SHALL remain in UTC. No database schema changes are needed for timezone storage of data timestamps. The `time.Time` fields in Go models continue to use UTC.

#### Scenario: Record created with UTC timestamp
- **WHEN** a new record is inserted via GORM
- **THEN** `CreatedAt` and `UpdatedAt` are stored as UTC

### Requirement: API returns UTC timestamps
All API responses containing timestamps SHALL return them in RFC 3339 / ISO 8601 format with UTC timezone (e.g., `"2024-01-15T08:30:00Z"`).

#### Scenario: User list returns UTC times
- **WHEN** the frontend requests `/api/v1/users`
- **THEN** each user's `createdAt` field is in UTC ISO 8601 format

### Requirement: Frontend formats time by user timezone
The `formatDateTime` utility SHALL accept the user's timezone (from auth store or system config) and locale, using `Intl.DateTimeFormat` with the `timeZone` option. It MUST NOT hardcode any locale or timezone.

#### Scenario: User in Asia/Shanghai sees local time
- **WHEN** a timestamp `"2024-01-15T08:30:00Z"` is displayed and user timezone is `Asia/Shanghai`
- **THEN** the formatted output shows `2024/01/15 16:30` (UTC+8)

#### Scenario: User in America/New_York sees local time
- **WHEN** a timestamp `"2024-01-15T08:30:00Z"` is displayed and user timezone is `America/New_York`
- **THEN** the formatted output shows `2024/01/15 03:30` (UTC-5)

#### Scenario: Format respects active locale
- **WHEN** the active locale is `en` and timezone is `UTC`
- **THEN** the date format follows English conventions (e.g., `1/15/2024 08:30`)

### Requirement: Timezone resolution priority
The system SHALL resolve the active timezone in this order: (1) user's `timezone` preference from auth store, (2) system default `system.timezone` from site info, (3) browser's `Intl.DateTimeFormat().resolvedOptions().timeZone`, (4) `"UTC"` as hardcoded fallback.

#### Scenario: User timezone overrides system default
- **WHEN** user has `timezone: "America/New_York"` and system default is `Asia/Shanghai`
- **THEN** all times display in America/New_York

#### Scenario: No user timezone, system default used
- **WHEN** user's timezone is empty and system default is `Asia/Shanghai`
- **THEN** all times display in Asia/Shanghai

### Requirement: Timezone uses IANA identifiers
All timezone values SHALL be IANA timezone identifiers (e.g., `Asia/Shanghai`, `America/New_York`, `Europe/London`). The system MUST NOT use UTC offset numbers (e.g., `+8`) as the primary timezone representation.

#### Scenario: Valid IANA timezone stored
- **WHEN** user sets their timezone to `Asia/Tokyo`
- **THEN** the value `"Asia/Tokyo"` is stored in the user record

### Requirement: Relative time display
The system MAY use locale-aware relative time formatting (e.g., "3 minutes ago", "3 分钟前") for recent timestamps where appropriate, using `Intl.RelativeTimeFormat` with the active locale.

#### Scenario: Recent timestamp shows relative time
- **WHEN** a timestamp is less than 24 hours old and the locale is `zh-CN`
- **THEN** it MAY display as "3 小时前" instead of the absolute time
