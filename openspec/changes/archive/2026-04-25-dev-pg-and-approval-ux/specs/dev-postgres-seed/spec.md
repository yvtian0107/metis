## ADDED Requirements

### Requirement: DefaultDevConfig generates PostgreSQL configuration

`config.DefaultDevConfig()` SHALL return a `MetisConfig` with:
- `db_driver`: `"postgres"`
- `db_dsn`: `"host=localhost port=5432 user=postgres password=password dbname=postgres sslmode=disable"`
- `clickhouse.dsn`: `"clickhouse://default:@localhost:9000/otel"`
- `falkordb.addr`: `"localhost:6379"`

#### Scenario: First-time seed-dev without existing config.yml
- **WHEN** `make seed-dev` runs and no `config.yml` exists
- **THEN** `loadOrCreateSeedDevConfig` SHALL call `DefaultDevConfig()` instead of `DefaultSQLiteConfig()`
- **AND** the generated `config.yml` SHALL contain PG driver, ClickHouse DSN, and FalkorDB address

#### Scenario: Existing config.yml is preserved
- **WHEN** `make seed-dev` runs and `config.yml` already exists
- **THEN** the existing configuration SHALL be loaded as-is without modification

### Requirement: reset-pg Makefile target

The project SHALL provide a `make reset-pg` target that drops and recreates the PostgreSQL database, then re-seeds.

#### Scenario: Developer resets database
- **WHEN** developer runs `make reset-pg`
- **THEN** the `postgres` database SHALL be dropped (with force to disconnect active sessions)
- **AND** a fresh `postgres` database SHALL be created
- **AND** `config.yml` SHALL be removed
- **AND** `make seed-dev` SHALL run to re-initialize

### Requirement: Updated clean target

`make clean` SHALL remove `config.yml` (as before) and no longer reference SQLite-specific files as the primary cleanup.

#### Scenario: Developer runs make clean
- **WHEN** developer runs `make clean`
- **THEN** `config.yml` and any SQLite files (`metis.db*`) SHALL be removed
