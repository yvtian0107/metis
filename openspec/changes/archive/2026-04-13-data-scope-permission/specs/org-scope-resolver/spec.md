## ADDED Requirements

### Requirement: OrgScopeResolver interface in kernel
The system SHALL define an `OrgScopeResolver` interface in the kernel App layer (`internal/app/`) with a single method: `GetUserDeptScope(userID uint) ([]uint, error)`. This interface SHALL be registered in the IOC container by the Org App when installed. When the Org App is not installed, the interface SHALL not be registered and middleware SHALL handle the nil case gracefully.

#### Scenario: OrgScopeResolver registered by Org App
- **WHEN** the system starts with the Org App installed (edition_full)
- **THEN** the IOC container SHALL have an `OrgScopeResolver` implementation registered, and `GetUserDeptScope` SHALL return the BFS-expanded list of department IDs the user belongs to

#### Scenario: OrgScopeResolver absent without Org App
- **WHEN** the system starts without the Org App (edition_lite or edition_license)
- **THEN** the IOC container SHALL have no `OrgScopeResolver` registered, and DataScopeMiddleware SHALL treat all users as having `ALL` scope

### Requirement: DataScopeMiddleware
The system SHALL provide a `DataScopeMiddleware` in `internal/middleware/` that resolves the current user's department scope and injects it into the Gin request context as `deptScope` (type `[]uint` or nil).

The middleware SHALL operate as follows:
- If `OrgScopeResolver` is nil → set `deptScope = nil` (no filtering)
- If `role.dataScope == "all"` → set `deptScope = nil`
- If `role.dataScope == "self"` → set `deptScope = []uint{}` (empty, only own records)
- If `role.dataScope == "dept"` → call `OrgScopeResolver.GetUserDeptScope` with depth=1 only
- If `role.dataScope == "dept_and_sub"` → call `OrgScopeResolver.GetUserDeptScope` (full BFS)
- If `role.dataScope == "custom"` → query `role_dept_scopes` for configured department IDs

The middleware SHALL be placed after `CasbinAuth` in the middleware chain.

#### Scenario: ALL scope injects nil
- **WHEN** a user with a role of dataScope `all` makes any API request
- **THEN** the Gin context SHALL have `deptScope = nil`, and repository queries SHALL NOT add any department filter

#### Scenario: DEPT_AND_SUB scope resolves departments
- **WHEN** a user assigned to department ID 5 (which has sub-departments 8, 9) with dataScope `dept_and_sub` makes an API request
- **THEN** the Gin context SHALL have `deptScope = [5, 8, 9]`

#### Scenario: SELF scope injects empty slice
- **WHEN** a user with dataScope `self` makes an API request
- **THEN** the Gin context SHALL have `deptScope = []uint{}` (empty slice, distinct from nil)

#### Scenario: Middleware is nil-safe without Org App
- **WHEN** the Org App is not installed and a user makes any API request
- **THEN** the middleware SHALL set `deptScope = nil` and proceed without error

### Requirement: ListParams DeptScope field
The system SHALL add a `DeptScope *[]uint` field to the shared `ListParams` struct in the repository layer. When `DeptScope` is non-nil, repository List methods SHALL add `WHERE department_id IN (?)` filter. When `DeptScope` is nil, no filter SHALL be applied.

#### Scenario: Nil DeptScope returns all records
- **WHEN** a List query is executed with `DeptScope = nil`
- **THEN** the query SHALL return all records regardless of department

#### Scenario: Non-nil DeptScope filters by department
- **WHEN** a List query is executed with `DeptScope = &[]uint{3, 7}`
- **THEN** the query SHALL only return records where `department_id IN (3, 7)`

#### Scenario: Empty DeptScope returns only own records
- **WHEN** a List query is executed with `DeptScope = &[]uint{}` and `userID` provided
- **THEN** the query SHALL only return records where `created_by = userID` or `owner_id = userID`
