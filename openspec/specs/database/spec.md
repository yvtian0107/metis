### Requirement: Database initialization with GORM
The system SHALL initialize a GORM database connection on startup using the configuration from `metis.yaml` (via the `config.MetisConfig` struct). When no config file exists (install mode), the system SHALL use default SQLite settings. The otelgorm plugin SHALL be registered after OTel initialization (which now happens after DB connection).

#### Scenario: Default SQLite initialization (install mode)
- **WHEN** no `metis.yaml` file exists
- **THEN** the system SHALL open a SQLite database at `metis.db` with foreign keys enabled and WAL journal mode, using hardcoded defaults

#### Scenario: SQLite from config file
- **WHEN** `metis.yaml` contains `db_driver: sqlite` and a `db_dsn` value
- **THEN** the system SHALL open SQLite at the specified path with the configured DSN

#### Scenario: PostgreSQL from config file
- **WHEN** `metis.yaml` contains `db_driver: postgres` and a valid PostgreSQL DSN
- **THEN** the system SHALL open a PostgreSQL connection using the provided DSN

#### Scenario: Unsupported driver
- **WHEN** `db_driver` in metis.yaml is set to an unsupported value
- **THEN** the system SHALL return an error indicating the driver is not supported

#### Scenario: OTel GORM plugin registration
- **WHEN** the database is initialized and OTel is subsequently enabled
- **THEN** the otelgorm plugin SHALL be registered with WithoutQueryVariables option after OTel initialization completes

### Requirement: Pure Go SQLite driver
The system SHALL use the `github.com/glebarez/sqlite` driver (no CGO dependency) for SQLite connections.

#### Scenario: Build without CGO
- **WHEN** the project is built with `CGO_ENABLED=0`
- **THEN** the binary SHALL compile and run successfully with SQLite support

### Requirement: AutoMigrate on startup
The system SHALL run GORM AutoMigrate for all registered models during database initialization, including all kernel models. In install mode, AutoMigrate SHALL be called during the installation process (not at DB init time). In normal mode, AutoMigrate SHALL run at startup as before.

#### Scenario: Install mode migration
- **WHEN** the installation is executed via `POST /api/v1/install/execute`
- **THEN** AutoMigrate SHALL be called for all kernel and app models as part of the installation sequence

#### Scenario: Normal mode startup
- **WHEN** the application starts in normal mode with an existing database
- **THEN** AutoMigrate SHALL run at startup to add any new columns or indexes without data loss
