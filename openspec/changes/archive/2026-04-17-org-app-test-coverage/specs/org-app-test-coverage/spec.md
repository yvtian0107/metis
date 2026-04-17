## ADDED Requirements

### Requirement: Model and utility tests
The Org App SHALL provide unit tests for `model.go` covering `JSONMap` serialization/deserialization edge cases and response mapping for `Department` and `Position`.

#### Scenario: JSONMap handles all valid and invalid scan inputs
- **WHEN** `JSONMap.Scan` is called with `string`, `[]byte`, `nil`, or an unsupported type
- **THEN** it SHALL produce the expected value for valid inputs and return an error for unsupported types

#### Scenario: Department.ToResponse maps all fields correctly
- **WHEN** a `Department` instance is converted via `ToResponse()`
- **THEN** the resulting `DepartmentResponse` SHALL contain matching field values

#### Scenario: Position.ToResponse maps all fields correctly
- **WHEN** a `Position` instance is converted via `ToResponse()`
- **THEN** the resulting `PositionResponse` SHALL contain matching field values

### Requirement: Department repository tests
The Org App SHALL provide integration tests for `DepartmentRepo` verifying CRUD operations, tree queries, and referential checks.

#### Scenario: Create and retrieve a department
- **WHEN** a department is created and then fetched by ID or code
- **THEN** the retrieved record SHALL match the created one

#### Scenario: Update department fields
- **WHEN** a department is updated with partial fields
- **THEN** only the specified fields SHALL be modified

#### Scenario: Delete a department
- **WHEN** an existing department is deleted
- **THEN** subsequent retrieval SHALL return a not-found error

#### Scenario: Detect children and members
- **WHEN** a department has sub-departments or assigned users
- **THEN** `HasChildren` and `HasMembers` SHALL return true; otherwise false

#### Scenario: ListAllIDsWithParent filters by active status
- **WHEN** `ListAllIDsWithParent(true)` is called
- **THEN** inactive departments SHALL be excluded from the result

### Requirement: Position repository tests
The Org App SHALL provide integration tests for `PositionRepo` verifying CRUD, pagination, and usage detection.

#### Scenario: Create and retrieve a position
- **WHEN** a position is created and fetched by ID or code
- **THEN** the retrieved record SHALL match

#### Scenario: List with pagination and keyword filtering
- **WHEN** positions are listed with keyword and pagination parameters
- **THEN** the result SHALL contain only matching items and a correct total count

#### Scenario: List returns all when pageSize is zero
- **WHEN** `List` is called with `pageSize=0`
- **THEN** all matching records SHALL be returned without pagination limits

#### Scenario: InUse detection
- **WHEN** a position is referenced by a `UserPosition`
- **THEN** `InUse` SHALL return true; otherwise false

### Requirement: Assignment repository tests
The Org App SHALL provide integration tests for `AssignmentRepo` covering CRUD, transaction-safe primary management, auto-promotion, and scope helpers.

#### Scenario: AddPosition creates a basic assignment
- **WHEN** `AddPosition` is called with a valid `UserPosition`
- **THEN** the record SHALL be persisted

#### Scenario: AddPositionWithPrimary demotes existing primary
- **WHEN** `AddPositionWithPrimary` is called with `setPrimary=true`
- **THEN** any existing primary assignment for that user SHALL be demoted atomically

#### Scenario: AddPositionWithPrimary auto-promotes on first assignment
- **WHEN** `AddPositionWithPrimary` is called for a user with no assignments and `autoSetPrimary=true`
- **THEN** the new assignment SHALL be marked as primary

#### Scenario: RemovePosition deletes and auto-promotes next primary
- **WHEN** a primary assignment is removed and the user has other assignments
- **THEN** the next assignment (by sort, then ID) SHALL be promoted to primary automatically

#### Scenario: RemovePosition handles last assignment gracefully
- **WHEN** the user's only assignment is removed
- **THEN** deletion SHALL succeed with no remaining assignments

#### Scenario: UpdatePositionWithPrimary atomically sets primary
- **WHEN** `UpdatePositionWithPrimary` is called with `setPrimary=true`
- **THEN** existing primaries SHALL be demoted and the target updated in one transaction

#### Scenario: SetPrimary validates target exists
- **WHEN** `SetPrimary` is called for a non-existent assignment
- **THEN** it SHALL return an error

#### Scenario: ListUsersByDepartment joins and paginates correctly
- **WHEN** users are assigned to a department and `ListUsersByDepartment` is queried with keyword and pagination
- **THEN** the result SHALL include correct user fields, primary flags, and total count

#### Scenario: CountByDepartments aggregates correctly
- **WHEN** multiple users are assigned to various departments
- **THEN** `CountByDepartments` SHALL return accurate per-department counts

### Requirement: Department service tests
The Org App SHALL provide service-level tests for `DepartmentService` verifying business rules and tree construction.

#### Scenario: Create rejects duplicate codes
- **WHEN** a department is created with a code that already exists
- **THEN** `ErrDepartmentCodeExists` SHALL be returned

#### Scenario: Tree builds hierarchy with member counts
- **WHEN** departments form a parent-child hierarchy and users are assigned
- **THEN** `Tree()` SHALL return a nested structure with correct `MemberCount` per node

#### Scenario: Update rejects code collision with other departments
- **WHEN** a department is updated to use a code already owned by another department
- **THEN** `ErrDepartmentCodeExists` SHALL be returned

#### Scenario: Delete guards against children and members
- **WHEN** `Delete` is called on a department that has sub-departments or members
- **THEN** `ErrDepartmentHasChildren` or `ErrDepartmentHasMembers` SHALL be returned

### Requirement: Position service tests
The Org App SHALL provide service-level tests for `PositionService` verifying business rules.

#### Scenario: Create rejects duplicate codes
- **WHEN** a position is created with an existing code
- **THEN** `ErrPositionCodeExists` SHALL be returned

#### Scenario: Delete guards against in-use positions
- **WHEN** `Delete` is called on a position referenced by assignments
- **THEN** `ErrPositionInUse` SHALL be returned

### Requirement: Assignment service tests
The Org App SHALL provide service-level tests for `AssignmentService` verifying allocation logic, foreign key validation, and scope resolution.

#### Scenario: AddUserPosition validates department and position exist and are active
- **WHEN** `AddUserPosition` is called with a missing, inactive department or position
- **THEN** the corresponding sentinel error (`ErrDepartmentNotFound`, `ErrDepartmentInactive`, `ErrPositionNotFound`, `ErrPositionInactive`) SHALL be returned

#### Scenario: AddUserPosition prevents duplicate department assignment
- **WHEN** a user is already assigned to the target department
- **THEN** `ErrAlreadyAssigned` SHALL be returned

#### Scenario: GetUserDepartmentScope performs BFS including sub-departments
- **WHEN** a user is assigned to departments that have child departments
- **THEN** `GetUserDepartmentScope` SHALL include the assigned departments and all active descendants

#### Scenario: GetUserDepartmentScope skips inactive branches
- **WHEN** an assigned department has inactive child departments
- **THEN** the inactive branches SHALL be excluded from the scope

### Requirement: Handler tests
The Org App SHALL provide HTTP handler integration tests for all public endpoints under `/api/v1/org`.

#### Scenario: Department endpoints return correct status codes
- **WHEN** `Create`, `List`, `Tree`, `Get`, `Update`, and `Delete` endpoints are exercised with valid and invalid inputs
- **THEN** success cases SHALL return 200, validation/business errors SHALL return 400, and not-found SHALL return 404

#### Scenario: Position endpoints return correct status codes
- **WHEN** `Create`, `List`, `Get`, `Update`, and `Delete` endpoints are exercised with valid and invalid inputs
- **THEN** appropriate 200/400/404 responses SHALL be returned

#### Scenario: Assignment endpoints return correct status codes
- **WHEN** `GetUserPositions`, `AddUserPosition`, `RemoveUserPosition`, `UpdateUserPosition`, `SetPrimary`, and `ListUsers` are exercised
- **THEN** success cases SHALL return 200, missing/invalid inputs SHALL return 400, and not-found SHALL return 404

### Requirement: Scope resolver tests
The Org App SHALL provide tests for `OrgScopeResolverImpl` and `OrgUserResolverImpl`.

#### Scenario: OrgScopeResolverImpl expands scope when requested
- **WHEN** `GetUserDeptScope` is called with `includeSubDepts=true` versus `false`
- **THEN** the former SHALL include descendants while the latter SHALL return only directly assigned departments

#### Scenario: OrgUserResolverImpl returns IDs correctly
- **WHEN** `GetUserDepartmentIDs` and `GetUserPositionIDs` are called for a user with assignments
- **THEN** they SHALL return the distinct assigned IDs
