## MODIFIED Requirements

### Requirement: Get site info
The system SHALL provide a GET endpoint to retrieve public site information including version metadata.

#### Scenario: Site info with defaults
- **WHEN** GET /api/v1/site-info is called and no settings exist
- **THEN** the response SHALL return `{ "appName": "Metis", "hasLogo": false, "version": "<build version>", "gitCommit": "<build commit>", "buildTime": "<build time>" }`

#### Scenario: Site info with custom values
- **WHEN** GET /api/v1/site-info is called and system.app_name is set to "MyApp" and system.logo exists
- **THEN** the response SHALL return `{ "appName": "MyApp", "hasLogo": true, "version": "<build version>", "gitCommit": "<build commit>", "buildTime": "<build time>" }`

#### Scenario: Development mode version
- **WHEN** GET /api/v1/site-info is called and the binary was built without ldflags
- **THEN** the `version` field SHALL be `"dev"`, `gitCommit` SHALL be `""`, `buildTime` SHALL be `""`
