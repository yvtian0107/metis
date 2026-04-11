## MODIFIED Requirements

### Requirement: Install execution endpoint
The `POST /api/v1/install/execute` endpoint SHALL accept additional fields `locale` (string, optional) and `timezone` (string, optional) in the request body. During installation, these values MUST be saved as SystemConfig entries: `system.locale` and `system.timezone`. If omitted, `system.locale` defaults to `"zh-CN"` and `system.timezone` defaults to `"UTC"`.

#### Scenario: Install with locale and timezone
- **WHEN** the install request includes `{"locale": "en", "timezone": "America/New_York", ...}`
- **THEN** SystemConfig entries `system.locale = "en"` and `system.timezone = "America/New_York"` are created

#### Scenario: Install without locale and timezone
- **WHEN** the install request omits locale and timezone fields
- **THEN** SystemConfig entries `system.locale = "zh-CN"` and `system.timezone = "UTC"` are created
