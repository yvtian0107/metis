## 1. Shared Test Infrastructure

- [x] 1.1 Create `internal/app/org/helper_test.go` with `newOrgTestDB(t)` using in-memory SQLite
- [x] 1.2 Auto-migrate `model.User`, `model.Role`, `Department`, `Position`, `UserPosition` in the helper
- [x] 1.3 Add seed helpers: `seedDepartment`, `seedPosition`, `seedUser`, `seedAssignment`
- [x] 1.4 Verify `go test ./internal/app/org/...` compiles with the helper

## 2. Model & Utility Tests

- [x] 2.1 Create `internal/app/org/model_test.go`
- [x] 2.2 Add `JSONMap.Value` test for empty and non-empty values
- [x] 2.3 Add `JSONMap.Scan` tests for string, []byte, nil, and unsupported type error paths
- [x] 2.4 Add `JSONMap.MarshalJSON/UnmarshalJSON` round-trip tests
- [x] 2.5 Add `Department.ToResponse()` field mapping test
- [x] 2.6 Add `Position.ToResponse()` field mapping test

## 3. Department Repository Tests

- [x] 3.1 Create `internal/app/org/department_repository_test.go`
- [x] 3.2 Add tests for `Create`, `FindByID`, `FindByCode`
- [x] 3.3 Add tests for `Update` partial fields and `Delete`
- [x] 3.4 Add tests for `ListAll` ordering and `ListActive` filtering
- [x] 3.5 Add tests for `HasChildren` (true/false) and `HasMembers` (true/false)
- [x] 3.6 Add tests for `ListAllIDsWithParent` with `activeOnly=true` and `activeOnly=false`

## 4. Position Repository Tests

- [x] 4.1 Create `internal/app/org/position_repository_test.go`
- [x] 4.2 Add tests for `Create`, `FindByID`, `FindByCode`
- [x] 4.3 Add tests for `Update` and `Delete`
- [x] 4.4 Add tests for `List` with keyword filter, pagination, and `pageSize=0` returning all
- [x] 4.5 Add tests for `ListActive` filtering
- [x] 4.6 Add tests for `InUse` (referenced by assignment vs not referenced)

## 5. Assignment Repository Tests

- [x] 5.1 Create `internal/app/org/assignment_repository_test.go`
- [x] 5.2 Add tests for `FindByID`, `FindByUserID`, `FindByDepartmentID` (with Preload verification)
- [x] 5.3 Add tests for `AddPosition` basic creation
- [x] 5.4 Add tests for `AddPositionWithPrimary` with `setPrimary=true` demoting existing primary
- [x] 5.5 Add tests for `AddPositionWithPrimary` with `autoSetPrimary=true` on first assignment
- [x] 5.6 Add tests for `AddPositionWithPrimary` with `autoSetPrimary=true` on non-first assignment
- [x] 5.7 Add tests for `ExistsByUserAndDept`
- [x] 5.8 Add tests for `RemovePosition` normal deletion and not-found error
- [x] 5.9 Add tests for `RemovePosition` auto-promoting next primary when primary is removed
- [x] 5.10 Add tests for `RemovePosition` graceful handling when removing the last assignment
- [x] 5.11 Add tests for `UpdatePositionWithPrimary` updating position only
- [x] 5.12 Add tests for `UpdatePositionWithPrimary` with `setPrimary=true` transaction behavior
- [x] 5.13 Add tests for `UpdatePositionWithPrimary` not-found error
- [x] 5.14 Add tests for `SetPrimary` success and not-found error
- [x] 5.15 Add tests for `ListUsersByDepartment` pagination, keyword filtering, and total count
- [x] 5.16 Add tests for `CountByDepartments` aggregation
- [x] 5.17 Add tests for `GetUserDepartmentIDs` and `GetUserPositionIDs`
- [x] 5.18 Add tests for `GetSubDepartmentIDs` with `activeOnly=true` and `activeOnly=false`
- [x] 5.19 Add tests for `GetUserPrimaryPosition`

## 6. Department Service Tests

- [x] 6.1 Create `internal/app/org/department_service_test.go`
- [x] 6.2 Add tests for `Create` success and `ErrDepartmentCodeExists`
- [x] 6.3 Add tests for `Get` success and `ErrDepartmentNotFound`
- [x] 6.4 Add tests for `ListAll`
- [x] 6.5 Add tests for `Tree` hierarchy and `MemberCount` correctness
- [x] 6.6 Add tests for `Update` partial fields and `ErrDepartmentCodeExists`
- [x] 6.7 Add tests for `Delete` success, `ErrDepartmentNotFound`, `ErrDepartmentHasChildren`, `ErrDepartmentHasMembers`

## 7. Position Service Tests

- [x] 7.1 Create `internal/app/org/position_service_test.go`
- [x] 7.2 Add tests for `Create` success and `ErrPositionCodeExists`
- [x] 7.3 Add tests for `Get` success and `ErrPositionNotFound`
- [x] 7.4 Add tests for `List` with keyword and pagination
- [x] 7.5 Add tests for `ListActive`
- [x] 7.6 Add tests for `Update` partial fields and `ErrPositionCodeExists`
- [x] 7.7 Add tests for `Delete` success, `ErrPositionNotFound`, `ErrPositionInUse`

## 8. Assignment Service Tests

- [x] 8.1 Create `internal/app/org/assignment_service_test.go`
- [x] 8.2 Add tests for `GetUserPositions` response shape and preloaded data
- [x] 8.3 Add tests for `AddUserPosition` success (non-primary and primary)
- [x] 8.4 Add tests for `AddUserPosition` returning `ErrDepartmentNotFound` and `ErrDepartmentInactive`
- [x] 8.5 Add tests for `AddUserPosition` returning `ErrPositionNotFound` and `ErrPositionInactive`
- [x] 8.6 Add tests for `AddUserPosition` returning `ErrAlreadyAssigned`
- [x] 8.7 Add tests for `RemoveUserPosition` success and `ErrAssignmentNotFound`
- [x] 8.8 Add tests for `UpdateUserPosition` success, no-op, and `ErrAssignmentNotFound`
- [x] 8.9 Add tests for `SetPrimary` success and `ErrAssignmentNotFound`
- [x] 8.10 Add tests for `ListDepartmentMembers` pagination and keyword filtering
- [x] 8.11 Add tests for `GetUserDepartmentIDs`
- [x] 8.12 Add tests for `GetUserDepartmentScope` BFS expansion including sub-departments
- [x] 8.13 Add tests for `GetUserDepartmentScope` skipping inactive branches
- [x] 8.14 Add tests for `GetUserDepartmentScope` returning nil when user has no assignments

## 9. Handler Tests

- [x] 9.1 Create `internal/app/org/department_handler_test.go`
- [x] 9.2 Add tests for `Create` (200 and 400 duplicate code)
- [x] 9.3 Add tests for `List` and `Tree` (200)
- [x] 9.4 Add tests for `Get` (200 and 404)
- [x] 9.5 Add tests for `Update` (200, 400 duplicate code, 404)
- [x] 9.6 Add tests for `Delete` (200, 400 has children/members, 404)
- [x] 9.7 Create `internal/app/org/position_handler_test.go`
- [x] 9.8 Add tests for `Create` (200 and 400 duplicate code)
- [x] 9.9 Add tests for `List` (200)
- [x] 9.10 Add tests for `Get` (200 and 404)
- [x] 9.11 Add tests for `Update` (200, 400 duplicate code, 404)
- [x] 9.12 Add tests for `Delete` (200, 400 in use, 404)
- [x] 9.13 Create `internal/app/org/assignment_handler_test.go`
- [x] 9.14 Add tests for `GetUserPositions` (200)
- [x] 9.15 Add tests for `AddUserPosition` (200 and 400 sentinel errors)
- [x] 9.16 Add tests for `RemoveUserPosition` (200 and 404)
- [x] 9.17 Add tests for `UpdateUserPosition` (200 and 404)
- [x] 9.18 Add tests for `SetPrimary` (200 and 404)
- [x] 9.19 Add tests for `ListUsers` (200 and 400 missing departmentId)

## 10. Scope Resolver Tests

- [x] 10.1 Create `internal/app/org/scope_resolver_test.go`
- [x] 10.2 Add tests for `OrgScopeResolverImpl.GetUserDeptScope` with `includeSubDepts=true`
- [x] 10.3 Add tests for `OrgScopeResolverImpl.GetUserDeptScope` with `includeSubDepts=false`
- [x] 10.4 Add tests for `OrgUserResolverImpl.GetUserDepartmentIDs`
- [x] 10.5 Add tests for `OrgUserResolverImpl.GetUserPositionIDs`

## 11. Verification & Regression

- [x] 11.1 Run `go test ./internal/app/org/...` and ensure all tests pass
- [x] 11.2 Run `go build -tags dev ./cmd/server/` to confirm no compilation regressions
- [x] 11.3 Run `go test ./...` to ensure no other package tests are broken (pre-existing failures in `internal/app/ai` unrelated to org app)
