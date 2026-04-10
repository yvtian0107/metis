# Capability: system-config

## Purpose
Provides the SystemConfig K/V table for internal system configuration storage. Access is restricted to internal service methods only (SettingsService, SiteInfoHandler, SchedulerTask, InstallHandler) -- no public API routes except for the install status endpoint.

## Requirements

### Requirement: SystemConfig K/V table
The system SHALL provide a SystemConfig table with Key (primary key), Value, Remark, CreatedAt, and UpdatedAt fields. The table SHALL NOT be exposed via public API routes except for the install status endpoint. Access SHALL be through internal service methods only (SettingsService, SiteInfoHandler, SchedulerTask, InstallHandler).

#### Scenario: Table structure
- **WHEN** the database is initialized
- **THEN** the system_configs table SHALL exist with columns: key (varchar 255, PK), value (text), remark (varchar 500), created_at, updated_at

#### Scenario: Internal access only
- **WHEN** any code needs to read or write a system config value
- **THEN** it SHALL use the SysConfigService or SettingsService methods, NOT direct HTTP API calls

#### Scenario: Seed default configs during installation
- **WHEN** `seed.Install()` runs during installation
- **THEN** the system SHALL create all default config entries (security.*, scheduler.*, audit.*, server_port, otel.*, site.name)

#### Scenario: No config overwrite on sync
- **WHEN** `seed.Sync()` runs on subsequent startups
- **THEN** existing SystemConfig values SHALL NOT be overwritten. Only missing keys SHALL be added with defaults.

### Requirement: Server port config in SystemConfig
The system SHALL support a `server_port` config key in SystemConfig with default value `8080`. This value SHALL be read at startup to determine the HTTP listen port.

#### Scenario: Config seeded during installation
- **WHEN** the installation completes
- **THEN** the `server_port` key SHALL exist in SystemConfig with value `8080` and remark `HTTP 服务监听端口（修改后需重启）`

#### Scenario: Admin changes port
- **WHEN** an admin updates `server_port` to `9090` via settings
- **THEN** the change SHALL take effect on the next server restart

### Requirement: OTel config keys in SystemConfig
The system SHALL support the following OTel config keys in SystemConfig: `otel.enabled` (default "false"), `otel.exporter_endpoint` (default "http://localhost:4318"), `otel.service_name` (default "metis"), `otel.sample_rate` (default "1.0").

#### Scenario: OTel configs seeded during installation
- **WHEN** the installation completes
- **THEN** all four OTel config keys SHALL exist in SystemConfig with their default values

### Requirement: Site name config in SystemConfig
The system SHALL support a `site.name` config key in SystemConfig. This value SHALL be set during installation to the user-provided site name.

#### Scenario: Site name set during installation
- **WHEN** the installation completes with site_name "My Metis"
- **THEN** the `site.name` key SHALL exist in SystemConfig with value "My Metis"

### Requirement: Installation flag in SystemConfig
The system SHALL use `app.installed` key in SystemConfig as the installation completion flag. The value SHALL be `true` when the system is installed.

#### Scenario: Fresh database
- **WHEN** the system starts with a new empty database
- **THEN** `app.installed` SHALL not exist in SystemConfig (or be absent), indicating the system is not installed

#### Scenario: After installation
- **WHEN** the installation completes successfully
- **THEN** `app.installed` SHALL be set to `true` in SystemConfig

### Requirement: Scheduler history retention config
The system SHALL support a `scheduler.history_retention_days` config key with default value `30`. This config SHALL be readable by the scheduler engine to determine how many days of task execution history to retain.

#### Scenario: Config seeded on installation
- **WHEN** the installation completes
- **THEN** the `scheduler.history_retention_days` key SHALL exist in system_configs with value `30` and remark `任务执行历史保留天数，0 表示永不清理`

#### Scenario: Admin updates retention
- **WHEN** an admin updates `scheduler.history_retention_days` to `7` via the config API
- **THEN** the next cleanup task execution SHALL delete records older than 7 days
