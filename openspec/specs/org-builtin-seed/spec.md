# Capability: org-builtin-seed

## Purpose
Provides built-in seed data for the Org App, including a default department tree and position definitions, seeded during first installation or CLI re-seed.

## Requirements

### Requirement: Built-in departments seed data
Org App SHALL define 7 built-in departments in a 2-level tree structure. The root department SHALL be "总部" (code: `headquarters`). Six child departments SHALL be created under "总部": "研发部" (`rd`, sort=1), "运维部" (`ops`, sort=2), "测试部" (`qa`, sort=3), "市场部" (`marketing`, sort=4), "销售部" (`sales`, sort=5), "信息部" (`it`, sort=6). All departments SHALL have `IsActive=true`.

#### Scenario: Departments created on first install
- **WHEN** `App.Seed(db, enforcer, true)` is called during first installation
- **THEN** the system SHALL create all 7 departments with correct parent-child relationships and sort orders

#### Scenario: Departments not created on daily sync
- **WHEN** `App.Seed(db, enforcer, false)` is called during normal startup
- **THEN** the system SHALL NOT attempt to create departments

#### Scenario: Idempotent department creation
- **WHEN** `App.Seed(db, enforcer, true)` is called and a department with code `headquarters` already exists
- **THEN** the system SHALL skip that department and not overwrite its fields

#### Scenario: Root department created before children
- **WHEN** department seed runs
- **THEN** the "总部" root department SHALL be created first, and its ID used as `ParentID` for all child departments

### Requirement: Built-in positions seed data
Org App SHALL define 7 built-in positions with flat structure (no hierarchy). Positions: "IT管理员" (`it_admin`, sort=1), "数据库管理员" (`db_admin`, sort=2), "网络管理员" (`network_admin`, sort=3), "安全管理员" (`security_admin`, sort=4), "应用管理员" (`app_admin`, sort=5), "运维管理员" (`ops_admin`, sort=6), "总部助理" (`assistant`, sort=7). All positions SHALL have `IsActive=true`.

#### Scenario: Positions created on first install
- **WHEN** `App.Seed(db, enforcer, true)` is called during first installation
- **THEN** the system SHALL create all 7 positions with correct sort orders

#### Scenario: Positions not created on daily sync
- **WHEN** `App.Seed(db, enforcer, false)` is called during normal startup
- **THEN** the system SHALL NOT attempt to create positions

#### Scenario: Idempotent position creation
- **WHEN** `App.Seed(db, enforcer, true)` is called and a position with code `it_admin` already exists
- **THEN** the system SHALL skip that position and not overwrite its fields

### Requirement: Seed data descriptions
Each built-in department and position SHALL include a Chinese description field matching bklite-cloud definitions: 总部="公司总部", 研发部="负责产品研发", 运维部="负责系统运维", 测试部="负责质量测试", 市场部="负责市场推广", 销售部="负责销售业务", 信息部="负责信息技术支持", IT管理员="负责IT基础设施的日常管理和维护", 数据库管理员="负责数据库系统的管理、维护和优化", 网络管理员="负责网络设备和网络安全的管理维护", 安全管理员="负责信息安全策略制定和安全事件响应", 应用管理员="负责业务应用系统的部署和运维管理", 运维管理员="负责整体运维工作的协调和管理", 总部助理="负责总部审批与流程协作"。

#### Scenario: Department description present
- **WHEN** the "研发部" department is created during seed
- **THEN** its `Description` field SHALL be "负责产品研发"

#### Scenario: Position description present
- **WHEN** the "安全管理员" position is created during seed
- **THEN** its `Description` field SHALL be "负责信息安全策略制定和安全事件响应"
