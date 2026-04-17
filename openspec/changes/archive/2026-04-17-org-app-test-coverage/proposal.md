## Why

The Org App (`internal/app/org/`) manages departments, positions, and user assignments — a core part of the system's RBAC and data-scope infrastructure. Despite having mature feature implementations through multiple prior changes, the entire Org App currently has **zero automated test coverage**. Adding comprehensive tests is essential to prevent regressions in tree building, primary-position transactions, department scope resolution, and cross-table assignment logic.

## What Changes

- Add unit tests for `internal/app/org/model.go` (`JSONMap`, `Department.ToResponse()`, `Position.ToResponse()`)
- Add repository tests for `DepartmentRepo` (CRUD, tree queries, membership checks, `ListAllIDsWithParent`)
- Add repository tests for `PositionRepo` (CRUD, pagination, `pageSize=0` all-mode, `InUse` check)
- Add repository tests for `AssignmentRepo` (CRUD, transaction-safe primary management, auto-promote on removal, joins, counts, scope helpers)
- Add service tests for `DepartmentService` (business rules, tree building, delete guards)
- Add service tests for `PositionService` (business rules, delete guards)
- Add service tests for `AssignmentService` (allocation logic, FK validation, scope BFS, primary enforcement)
- Add handler tests for `DepartmentHandler`, `PositionHandler`, and `AssignmentHandler` (Gin integration tests covering success and error paths)
- Add tests for `OrgScopeResolverImpl` and `OrgUserResolverImpl`
- Create a shared test helper for in-memory SQLite with required kernel + org model migrations

## Capabilities

### New Capabilities
- `org-app-test-coverage`: Comprehensive unit and integration test coverage for the Org App's model, repository, service, handler, and scope resolver layers.

### Modified Capabilities
- *(none — this change only adds tests and shared test helpers, no spec-level behavior changes)*

## Impact

- `internal/app/org/*_test.go` (new test files)
- `internal/app/org/test_helper.go` (new shared test DB helper)
- No API or frontend changes
- No runtime behavior changes
