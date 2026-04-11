## ADDED Requirements

### Requirement: System locale and timezone config keys
The SystemConfig table SHALL support two new keys: `system.locale` (default: `"zh-CN"`) and `system.timezone` (default: `"UTC"`). These MUST be seeded during installation with the values chosen in the install wizard.

#### Scenario: Default config after fresh install
- **WHEN** installation completes with default settings
- **THEN** SystemConfig contains `system.locale = "zh-CN"` and `system.timezone = "UTC"`

#### Scenario: Config accessible via settings service
- **WHEN** the settings service reads `system.locale`
- **THEN** it returns the stored locale value (e.g., `"en"`)

### Requirement: Site info API includes locale and timezone
The site info API response (used by frontend to initialize) SHALL include `locale` and `timezone` fields from SystemConfig, so the frontend can determine the system default before user authentication.

#### Scenario: Site info returns system locale
- **WHEN** the frontend fetches site info on page load
- **THEN** the response includes `locale: "zh-CN"` and `timezone: "Asia/Shanghai"`
