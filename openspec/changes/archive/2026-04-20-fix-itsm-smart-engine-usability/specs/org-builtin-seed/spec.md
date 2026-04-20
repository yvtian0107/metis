## MODIFIED Requirements

### Requirement: Built-in positions seed data
Org App SHALL define built-in positions that stay consistent with the participant codes used by the seeded smart ITSM services. The built-in set SHALL include at least: "IT管理员" (`it_admin`), "数据库管理员" (`db_admin`), "网络管理员" (`network_admin`), "安全管理员" (`security_admin`), "应用管理员" (`app_admin`), "运维管理员" (`ops_admin`), and any additional built-in position codes referenced by the built-in smart services. All positions SHALL have `IsActive=true`.

#### Scenario: Positions created on first install
- **WHEN** `App.Seed(db, enforcer, true)` is called during first installation
- **THEN** the system SHALL create all built-in positions needed by the seeded smart ITSM services with correct sort orders

#### Scenario: Smart-service participant codes are seed-compatible
- **WHEN** a built-in smart ITSM service references a participant position code during fresh install validation
- **THEN** that position code SHALL exist in the built-in Org seed set

#### Scenario: Positions not created on daily sync
- **WHEN** `App.Seed(db, enforcer, false)` is called during normal startup
- **THEN** the system SHALL NOT attempt to create positions

#### Scenario: Idempotent position creation
- **WHEN** `App.Seed(db, enforcer, true)` is called and a built-in position with the same code already exists
- **THEN** the system SHALL skip that position and not overwrite its fields
