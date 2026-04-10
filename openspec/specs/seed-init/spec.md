# Capability: seed-init

## Purpose
Provides idempotent database seeding of built-in roles, menus, Casbin policies, and default SystemConfig values. Seeding is split into `seed.Install()` for first-time installation (full seed) and `seed.Sync()` for incremental updates on subsequent startups.

## Requirements

### Requirement: Seed execution context
The seed logic SHALL be split into two functions: `seed.Install()` for first-time installation and `seed.Sync()` for subsequent startups.

#### Scenario: Install-time full seed
- **WHEN** `seed.Install(db, enforcer)` is called during installation
- **THEN** the system SHALL create built-in roles, the default menu tree, Casbin policies, default SystemConfig values, and default auth providers

#### Scenario: Startup incremental sync
- **WHEN** `seed.Sync(db, enforcer)` is called on normal startup
- **THEN** the system SHALL only add new roles, menus, and Casbin policies that don't already exist. It SHALL NOT overwrite existing SystemConfig values or auth providers.

#### Scenario: Sync output
- **WHEN** `seed.Sync()` completes
- **THEN** the function SHALL return a Result with counts of created/skipped items (same format as before)

### Requirement: Built-in roles seed data
The seed SHALL create two system roles: admin (code="admin", name="管理员", isSystem=true, sort=0) and user (code="user", name="普通用户", isSystem=true, sort=1).

#### Scenario: Admin role created
- **WHEN** seed runs and no role with code "admin" exists
- **THEN** the system SHALL create the admin role with IsSystem=true

#### Scenario: User role created
- **WHEN** seed runs and no role with code "user" exists
- **THEN** the system SHALL create the user role with IsSystem=true

### Requirement: Built-in menu tree seed data
The seed SHALL create the default menu tree including: "系统管理" (directory, sort=9999) with children "用户管理" (menu, path "/users", permission "system:user:list"), "角色管理" (menu, path "/roles", permission "system:role:list"), "菜单管理" (menu, path "/menus", permission "system:menu:list"), "系统配置" (menu, path "/config", permission "system:config:list"), "系统设置" (menu, path "/settings", permission "system:settings:list"). Each menu type entry SHALL also have button children for CRUD operations. The seed SHALL NOT include a "首页" menu item.

#### Scenario: Menu tree created with button permissions
- **WHEN** seed runs and the "用户管理" menu is created
- **THEN** it SHALL have button children: "新增用户" (permission "system:user:create"), "编辑用户" (permission "system:user:update"), "删除用户" (permission "system:user:delete"), "重置密码" (permission "system:user:reset-password")

#### Scenario: No home menu in seed
- **WHEN** seed runs on a fresh database
- **THEN** the seed SHALL NOT create a "首页" menu item with permission "home"

#### Scenario: System management sort order
- **WHEN** seed runs and creates the "系统管理" directory menu
- **THEN** the menu SHALL have sort=9999, ensuring it appears after all App module menus

### Requirement: Built-in Casbin policies seed
The seed SHALL create Casbin policies granting admin role full access to all API endpoints and all menu permissions. The user role SHALL get policies for basic auth endpoints and view own profile. The user role SHALL NOT receive a policy for "home" permission.

#### Scenario: Admin full API access
- **WHEN** seed runs
- **THEN** admin role SHALL have Casbin policies for all registered API paths and methods

#### Scenario: Admin full menu access
- **WHEN** seed runs
- **THEN** admin role SHALL have Casbin policies for all menu permission identifiers with action "read"

#### Scenario: User basic access
- **WHEN** seed runs
- **THEN** user role SHALL have Casbin policies for basic auth endpoints only, without any "home" permission policy
