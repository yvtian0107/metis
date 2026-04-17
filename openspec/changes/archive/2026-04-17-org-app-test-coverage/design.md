## Context

The Org App (`internal/app/org/`) implements department management, position management, and user-position assignments. It was built through three prior changes (`add-org-position-module`, `improve-org-assignment`, `org-app-quality-overhaul`) and is now functionally mature. However, it has zero automated test coverage. The codebase includes:

- 3 models (`Department`, `Position`, `UserPosition`) with a custom `JSONMap` type
- 3 repositories with raw GORM queries and transaction-heavy assignment logic
- 3 services with business rules (tree building, primary position enforcement, scope BFS)
- 3 HTTP handlers mounted under `/api/v1/org`
- 2 scope resolver implementations consumed by the kernel's `DataScopeMiddleware`

Testing this app requires cross-table setups because `AssignmentRepo` joins with kernel `users`, and `DepartmentService` depends on `AssignmentRepo` for member counts.

## Goals / Non-Goals

**Goals:**
- Achieve comprehensive unit/integration test coverage for all Org App layers
- Verify transaction safety of primary-position demotion/promotion
- Verify tree builder and BFS scope resolver correctness
- Verify handler error-code mapping matches sentinel errors
- Create a reusable in-memory SQLite helper for future Org App work

**Non-Goals:**
- No changes to runtime behavior, API contracts, or frontend code
- No performance benchmarking or load testing
- No testing of kernel services outside the Org App boundary (kernel services used as real collaborators)

## Decisions

### 1. Test using real repositories + in-memory SQLite
**Rationale:** Org App logic is tightly coupled to GORM behaviors (transactions, preloads, soft deletes, joins). Mocking repositories would test very little of value. An in-memory SQLite database with the required migrations gives us fast, deterministic integration tests at the repository and service levels.

### 2. Shared test helper: `newOrgTestDB()`
**Rationale:** Every repository and service test needs the same 5 tables (`users`, `roles`, `departments`, `positions`, `user_positions`). A shared helper in a `helper_test.go` file avoids duplication and ensures schema consistency.

### 3. Handler tests use real services + real repositories
**Rationale:** Handlers in the Org App are thin orchestrators. The only external dependency beyond Org services is `service.UserService` (used by `AssignmentHandler.ListUsers`). We will either:
- Use the real `UserService` + test DB (since `users` table is already migrated), or
- Stub `AssignmentHandler.userSvc` if it proves simpler.

For consistency with prior handler tests in the codebase (e.g., `identity_source_test.go`), we will construct handlers directly with injected service instances rather than spinning up the full IOC container.

### 4. Service tests use real repos, not mocked
**Rationale:** The service layer contains business rules (e.g., `Delete` guards, `Update` partial updates, `GetUserDepartmentScope` BFS). These rules are best verified against real data states. The test overhead is minimal because SQLite is in-memory.

### 5. `JSONMap` tests cover edge cases
**Rationale:** `JSONMap` is a custom GORM driver type used in the Org App (and potentially elsewhere). Its `Scan` method handles `string`, `[]byte`, `nil`, and unsupported types. We will test all paths to prevent silent data corruption.

## Risks / Trade-offs

| Risk | Mitigation |
|------|------------|
| SQLite behaves differently from PostgreSQL for certain GORM features | We avoid PG-specific syntax (no `RETURNING` idiom, no JSONB operators). All tested queries are standard SQL compatible with both. |
| `UserService` dependency in `AssignmentHandler` requires seeding users with valid roles | Seed a minimal `Role{Code: "user"}` and `User{Username: "u1"}` in tests that need it. |
| Large number of tests makes a single change slow to implement | Tasks are grouped by layer (model → repo → service → handler → scope). We implement one group at a time and run tests incrementally. |
| Tests become brittle if they over-specify response shapes | Assert on key fields (IDs, names, flags) rather than deep-equal on large structs with timestamps. |
