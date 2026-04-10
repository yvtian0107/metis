# Capability: site-info-api

## Purpose
Defines the backend API endpoints for managing site information including the application name and logo.

## Requirements

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

### Requirement: Update site name
The system SHALL provide a PUT endpoint to update the site name.

#### Scenario: Set site name
- **WHEN** PUT /api/v1/site-info is called with `{ "appName": "NewName" }`
- **THEN** the system SHALL upsert system_config with key "system.app_name" and value "NewName"
- **AND** the response SHALL return the updated site info

#### Scenario: Empty site name rejected
- **WHEN** PUT /api/v1/site-info is called with `{ "appName": "" }`
- **THEN** the system SHALL return 400 Bad Request

### Requirement: Get logo as image
The system SHALL provide a GET endpoint that returns the logo as a binary image.

#### Scenario: Logo exists
- **WHEN** GET /api/v1/site-info/logo is called and system.logo contains a base64 data URL
- **THEN** the response SHALL return the decoded binary image with the original Content-Type (e.g., image/png)

#### Scenario: No logo
- **WHEN** GET /api/v1/site-info/logo is called and no logo is set
- **THEN** the system SHALL return 404 Not Found

### Requirement: Upload logo
The system SHALL provide a PUT endpoint to upload a logo as base64 data URL.

#### Scenario: Valid logo upload
- **WHEN** PUT /api/v1/site-info/logo is called with `{ "data": "data:image/png;base64,..." }`
- **THEN** the system SHALL upsert system_config with key "system.logo" and the data URL as value
- **AND** the response SHALL return success

#### Scenario: Logo exceeds size limit
- **WHEN** PUT /api/v1/site-info/logo is called with a base64 payload larger than 2MB decoded
- **THEN** the system SHALL return 400 Bad Request with an error message

#### Scenario: Invalid data URL format
- **WHEN** PUT /api/v1/site-info/logo is called with a value not matching `data:image/*;base64,...`
- **THEN** the system SHALL return 400 Bad Request

### Requirement: Delete logo
The system SHALL provide a DELETE endpoint to remove the logo.

#### Scenario: Delete existing logo
- **WHEN** DELETE /api/v1/site-info/logo is called and a logo exists
- **THEN** the system SHALL delete the system_config entry with key "system.logo"

#### Scenario: Delete non-existent logo
- **WHEN** DELETE /api/v1/site-info/logo is called and no logo exists
- **THEN** the system SHALL return 404 Not Found
