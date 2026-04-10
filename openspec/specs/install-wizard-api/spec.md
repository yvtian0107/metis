# Capability: install-wizard-api

## Purpose
Provides the backend API endpoints for the browser-based install wizard. Handles install status checking, database connection testing, and full installation execution including config file generation, database migration, seeding, and hot switch to normal mode.

## Requirements

### Requirement: Install status endpoint
The system SHALL provide `GET /api/v1/install/status` that returns the current installation state. This endpoint SHALL be accessible without authentication.

#### Scenario: System not installed
- **WHEN** `GET /api/v1/install/status` is called and the system is not installed
- **THEN** the response SHALL return `{"code":0,"data":{"installed":false}}`

#### Scenario: System already installed
- **WHEN** `GET /api/v1/install/status` is called and `app.installed` is `true` in SystemConfig
- **THEN** the response SHALL return `{"code":0,"data":{"installed":true}}`

### Requirement: Database connection test endpoint
The system SHALL provide `POST /api/v1/install/check-db` that tests a database connection with user-provided parameters. This endpoint SHALL only be available when the system is not installed.

#### Scenario: Test PostgreSQL connection success
- **WHEN** `POST /api/v1/install/check-db` is called with `{"driver":"postgres","host":"localhost","port":5432,"user":"metis","password":"secret","dbname":"metis"}`
- **THEN** the system SHALL attempt to connect to the PostgreSQL database and return `{"code":0,"data":{"success":true}}`

#### Scenario: Test PostgreSQL connection failure
- **WHEN** `POST /api/v1/install/check-db` is called with invalid PostgreSQL credentials
- **THEN** the system SHALL return `{"code":0,"data":{"success":false,"error":"connection refused"}}` with the actual error message

#### Scenario: Endpoint blocked after installation
- **WHEN** `POST /api/v1/install/check-db` is called after the system is installed
- **THEN** the system SHALL return HTTP 403

### Requirement: Execute installation endpoint
The system SHALL provide `POST /api/v1/install/execute` that performs the full installation. This endpoint SHALL only be available when the system is not installed.

#### Scenario: SQLite installation
- **WHEN** `POST /api/v1/install/execute` is called with `{"db_driver":"sqlite","site_name":"My Metis","admin_username":"admin","admin_password":"Pass1234","admin_email":"admin@example.com"}`
- **THEN** the system SHALL:
  1. Generate `jwt_secret` and `license_key_secret` (64-char hex each)
  2. Write `metis.yaml` with SQLite defaults and generated secrets
  3. Run AutoMigrate for all kernel + app models
  4. Run `seed.Install()` (roles, menus, policies, default configs)
  5. Create the admin user with the provided credentials
  6. Set `app.installed=true` in SystemConfig
  7. Initialize all business services and routes (hot switch to normal mode)
  8. Return `{"code":0,"message":"ok"}`

#### Scenario: PostgreSQL installation
- **WHEN** `POST /api/v1/install/execute` is called with `{"db_driver":"postgres","db_host":"localhost","db_port":5432,"db_user":"metis","db_password":"secret","db_name":"metis","site_name":"My Metis","admin_username":"admin","admin_password":"Pass1234","admin_email":"admin@example.com"}`
- **THEN** the system SHALL compose the PostgreSQL DSN, then follow the same steps as SQLite installation

#### Scenario: Installation with password validation
- **WHEN** `POST /api/v1/install/execute` is called with admin_password shorter than 8 characters
- **THEN** the system SHALL return HTTP 400 with a validation error message

#### Scenario: Endpoint blocked after installation
- **WHEN** `POST /api/v1/install/execute` is called after the system is installed
- **THEN** the system SHALL return HTTP 403

#### Scenario: Installation failure rollback
- **WHEN** installation fails at any step (DB migration, seed, admin creation)
- **THEN** the system SHALL return an error response with details and NOT set `app.installed=true`
