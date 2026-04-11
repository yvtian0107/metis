## MODIFIED Requirements

### Requirement: Install seed creates locale and timezone config
The `seed.Install()` function SHALL create SystemConfig entries for `system.locale` and `system.timezone` using the values provided during installation. If no values are provided, defaults are `"zh-CN"` and `"UTC"`.

#### Scenario: Install seed with custom locale
- **WHEN** installation provides `locale: "en"` and `timezone: "America/New_York"`
- **THEN** `seed.Install()` creates SystemConfig entries with these values

#### Scenario: Sync does not overwrite locale config
- **WHEN** `seed.Sync()` runs on a subsequent startup
- **THEN** existing `system.locale` and `system.timezone` values are NOT overwritten (incremental-only behavior preserved)

## ADDED Requirements

### Requirement: Built-in menu titles remain as-is in database
Seed data for built-in menus SHALL continue to store Chinese titles in the database (e.g., `title: "用户管理"`). The frontend is responsible for translating built-in menu titles using the menu's `permission` field as a translation key lookup. No database schema changes to menus are required for i18n.

#### Scenario: Menu seed unchanged
- **WHEN** `seed.Install()` creates the "用户管理" menu
- **THEN** the database stores `title = "用户管理"` and `permission = "system:user:list"` (same as current behavior)

### Requirement: Built-in role names remain as-is in database
Seed data for built-in roles SHALL continue to store Chinese names in the database (e.g., `name: "管理员"`). The frontend translates built-in role names using the role's `code` field as a key lookup (e.g., `role.code = "admin"` → `t('roles.builtin.admin')`).

#### Scenario: Role seed unchanged
- **WHEN** `seed.Install()` creates the admin role
- **THEN** the database stores `name = "管理员"` and `code = "admin"` (same as current behavior)
