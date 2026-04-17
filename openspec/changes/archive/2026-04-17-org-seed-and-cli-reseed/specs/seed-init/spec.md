## MODIFIED Requirements

### Requirement: Seed execution context
The seed logic SHALL be split into two functions: `seed.Install()` for first-time installation and `seed.Sync()` for subsequent startups. The `App` interface `Seed` method SHALL accept an `install bool` parameter to distinguish first-time installation from daily sync: `Seed(db *gorm.DB, enforcer *casbin.Enforcer, install bool) error`.

#### Scenario: Install-time full seed
- **WHEN** `seed.Install(db, enforcer)` is called during installation
- **THEN** the system SHALL create built-in roles, the default menu tree, Casbin policies, default SystemConfig values, and default auth providers

#### Scenario: Install seed with custom locale
- **WHEN** installation provides `locale: "en"` and `timezone: "America/New_York"`
- **THEN** `seed.Install()` creates SystemConfig entries with these values

#### Scenario: Startup incremental sync
- **WHEN** `seed.Sync(db, enforcer)` is called on normal startup
- **THEN** the system SHALL only add new roles, menus, and Casbin policies that don't already exist. It SHALL NOT overwrite existing SystemConfig values or auth providers.

#### Scenario: Sync does not overwrite locale config
- **WHEN** `seed.Sync()` runs on a subsequent startup
- **THEN** existing `system.locale` and `system.timezone` values are NOT overwritten (incremental-only behavior preserved)

#### Scenario: Sync output
- **WHEN** `seed.Sync()` completes
- **THEN** the function SHALL return a Result with counts of created/skipped items (same format as before)

#### Scenario: App.Seed called with install=true during installation
- **WHEN** the Install wizard's hotSwitch calls `App.Seed()`
- **THEN** it SHALL pass `install=true` so Apps can seed install-only data (e.g., built-in departments)

#### Scenario: App.Seed called with install=false on normal startup
- **WHEN** the server starts normally and calls `App.Seed()` for each registered App
- **THEN** it SHALL pass `install=false` so Apps only run sync-safe logic (menus, policies)

#### Scenario: All Apps implement updated signature
- **WHEN** the `App` interface changes to `Seed(db *gorm.DB, enforcer *casbin.Enforcer, install bool) error`
- **THEN** all existing App implementations (ai, apm, itsm, license, node, observe, org) SHALL update their `Seed` method signature to match
