## 1. Model & Migration

- [x] 1.1 Add `DepartmentPosition` struct to `internal/app/org/model.go` with `DepartmentID`, `PositionID` (composite unique index), and `BaseModel` embedding; add `DepartmentPositionResponse` and `ToResponse()` method; add `TableName()` returning `department_positions`
- [x] 1.2 Register `DepartmentPosition` in `App.Models()` in `internal/app/org/app.go`

## 2. Repository Layer

- [x] 2.1 Add department-position repository methods to `department_repository.go`: `GetAllowedPositions(deptID)`, `SetAllowedPositions(deptID, positionIDs)` (transaction: delete all + batch insert), `IsPositionAllowed(deptID, positionID) bool` (returns true if no restrictions or position is in list)
- [x] 2.2 Update Tree query in `department_repository.go` to LEFT JOIN `users` table on `manager_id` to fetch manager username

## 3. Service Layer

- [x] 3.1 Add `GetAllowedPositions(deptID)` and `SetAllowedPositions(deptID, positionIDs)` methods to `DepartmentService`
- [x] 3.2 Update `DepartmentService.Tree()` to populate `ManagerName` field in `DepartmentTreeNode`
- [x] 3.3 Add position-department validation in `AssignmentService.AddUserPosition()` and `UpdateUserPosition()`: call `IsPositionAllowed()` and return `ErrPositionNotAllowedInDept` if invalid

## 4. Handler & Routing

- [x] 4.1 Add `GetAllowedPositions` and `SetAllowedPositions` handler methods to `DepartmentHandler`
- [x] 4.2 Register routes `GET /departments/:id/positions` and `PUT /departments/:id/positions` in `app.go` Routes
- [x] 4.3 Add Casbin policies for the new routes in `seed.go`

## 5. Seed Data

- [x] 5.1 Add default allowed positions for existing seeded departments in `seed.go` (e.g., IT部门 → IT管理员/网络管理员/安全管理员; 研发部 → 应用管理员; etc.)

## 6. Frontend: Department Form

- [x] 6.1 Remove `sort` field from `department-sheet.tsx` schema and form UI
- [x] 6.2 Add manager user selector to `department-sheet.tsx`
- [x] 6.3 Add allowed positions multi-select to `department-sheet.tsx`

## 7. Frontend: Department List

- [x] 7.1 Add `managerName` column to the tree table in `departments/index.tsx` (between code and status)
- [x] 7.2 Update `DepartmentItem` interface and `TreeNode` to include `managerName` field

## 8. i18n

- [x] 8.1 Add translation keys for manager selector, allowed positions, and validation messages in org locales (zh-CN and en)
