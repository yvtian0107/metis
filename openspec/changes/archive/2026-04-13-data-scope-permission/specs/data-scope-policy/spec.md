## ADDED Requirements

### Requirement: DataScope enum on Role model
The system SHALL add a `DataScope` field to the Role model, with valid values: `all`, `dept_and_sub`, `dept`, `self`, `custom`. Default value SHALL be `all`. The Role model SHALL include a `RoleDeptScope` association for the `custom` type.

#### Scenario: Default DataScope on new role
- **WHEN** POST /api/v1/roles with `{name: "普通角色", code: "viewer"}`
- **THEN** the created role SHALL have `dataScope: "all"`

#### Scenario: Create role with dept_and_sub scope
- **WHEN** POST /api/v1/roles with `{name: "部门经理", code: "dept-manager", dataScope: "dept_and_sub"}`
- **THEN** the system SHALL create the role and return `dataScope: "dept_and_sub"`

#### Scenario: Invalid dataScope value
- **WHEN** POST /api/v1/roles with `{dataScope: "unknown_value"}`
- **THEN** the system SHALL return 400 with message "invalid data scope value"

### Requirement: RoleDeptScope table for CUSTOM scope
The system SHALL provide a `role_dept_scopes` table storing `(roleID, departmentID)` pairs. This table SHALL only be meaningful when the role's `dataScope` is `custom`.

#### Scenario: Set custom scope departments
- **WHEN** PUT /api/v1/roles/:id/data-scope with `{dataScope: "custom", deptIds: [1, 3, 7]}`
- **THEN** the system SHALL set the role's dataScope to `custom` and replace the role_dept_scopes entries for this role with the given deptIds

#### Scenario: Clear custom scope on type change
- **WHEN** PUT /api/v1/roles/:id/data-scope with `{dataScope: "all"}`
- **THEN** the system SHALL update dataScope to `all` and delete all role_dept_scopes entries for this role

#### Scenario: Get role with custom scope details
- **WHEN** GET /api/v1/roles/:id and the role has dataScope `custom`
- **THEN** the response SHALL include `deptIds: [1, 3, 7]` listing the configured department IDs

### Requirement: DataScope API endpoint
The system SHALL provide a dedicated endpoint to configure a role's data scope, separate from general role update.

#### Scenario: Admin configures data scope
- **WHEN** PUT /api/v1/roles/:id/data-scope with valid payload
- **THEN** the system SHALL update the role's dataScope and RoleDeptScope records atomically, returning the updated role

#### Scenario: Cannot configure admin role data scope
- **WHEN** PUT /api/v1/roles/:id/data-scope for the system admin role
- **THEN** the system SHALL return 400 with message "cannot modify system admin data scope"
