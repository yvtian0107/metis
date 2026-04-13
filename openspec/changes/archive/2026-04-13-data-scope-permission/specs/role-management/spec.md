## ADDED Requirements

### Requirement: Role dataScope field in CRUD API
The system SHALL include the `dataScope` field in all Role CRUD API responses and accept it in create/update requests.

#### Scenario: List roles includes dataScope
- **WHEN** GET /api/v1/roles
- **THEN** each role in the response SHALL include `dataScope` field (e.g., `"all"`, `"dept_and_sub"`)

#### Scenario: Create role with dataScope
- **WHEN** POST /api/v1/roles with `{name: "运维经理", code: "ops-manager", dataScope: "dept_and_sub"}`
- **THEN** the system SHALL create the role with the specified dataScope

#### Scenario: Role detail includes custom deptIds
- **WHEN** GET /api/v1/roles/:id for a role with dataScope `custom`
- **THEN** the response SHALL include `deptIds: [...]` with the configured department IDs

## MODIFIED Requirements

### Requirement: Role model
The system SHALL store roles with Name (display name), Code (unique identifier used as Casbin subject), Description, Sort (ordering weight), IsSystem (flag preventing deletion of built-in roles), and **DataScope** (data visibility scope enum: `all` | `dept_and_sub` | `dept` | `self` | `custom`, default `all`). The Role model SHALL embed BaseModel. For `custom` DataScope, the role SHALL have an associated `RoleDeptScope` collection.

#### Scenario: Create role record
- **WHEN** a new role is created with name "编辑员", code "editor"
- **THEN** the system SHALL store a Role record with auto-generated ID/timestamps, IsSystem=false, and DataScope="all"

#### Scenario: Code uniqueness
- **WHEN** a role with code "admin" already exists and another role is created with the same code
- **THEN** the system SHALL return a unique constraint violation error

### Requirement: Role management frontend page
The system SHALL provide a role management page at /roles with list view, create/edit Sheet, delete confirmation, and data scope configuration entry.

#### Scenario: View role list
- **WHEN** user navigates to /roles
- **THEN** the page SHALL display a table of roles with columns: name, code, description, sort, isSystem, dataScope (badge), actions

#### Scenario: Create role via Sheet
- **WHEN** user clicks "新增角色" button and fills the form
- **THEN** a new role SHALL be created and the list SHALL refresh

#### Scenario: System role indicators
- **WHEN** a role has IsSystem=true
- **THEN** the delete button SHALL be hidden and the code field SHALL be read-only in edit mode

#### Scenario: DataScope badge in role list
- **WHEN** user views the role list
- **THEN** each role row SHALL display a badge indicating the current dataScope (e.g., "全部数据", "本部门及下级", "仅本人")
