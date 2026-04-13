## Why

Org App（部门、岗位、人员分配）的业务逻辑和前端代码存在多个严重缺陷：后端主岗位操作有并发竞态条件（会导致数据不一致）、错误被静默吞掉、缺少外键实体验证、有潜在 SQL 注入风险；前端有 866 行的 God Component、pageSize=9999 hack、无 debounce 的搜索、手搓 Combobox 等问题。这些问题在多人并发使用时会导致数据损坏，需要在功能扩展前修复。

## What Changes

### 后端业务逻辑修复
- 将 `AddUserPosition` 和 `UpdateUserPosition` 中的 primary 管理操作包裹在数据库事务中，消除并发竞态
- 移除 `_ = s.demoteCurrentPrimary(userID)` 中被忽略的错误，改为正确传播
- `SetPrimary` 增加目标 assignment 存在性验证，防止清空所有 primary
- `AddUserPosition` 增加部门和岗位的存在性 + active 状态校验
- 移除 `CountAssignments` 中动态 map key 拼接 SQL 的模式，改为安全的参数化查询
- `DepartmentService.Update` 的 8 个 pointer 参数改为 struct 入参
- `RemoveUserPosition` 增加 `ErrAssignmentNotFound` 错误映射
- `GetUserDepartmentScope` 的 BFS N+1 查询优化为单次全量加载

### 前端重构
- 将 `assignments/index.tsx`（866 行）拆分为独立子组件：DepartmentTreePanel、MemberListPanel、AddMemberSheet
- 用 shadcn/ui `Command` 组件替换手搓的 Popover Combobox
- 用户搜索增加 debounce（300ms）
- 消除 `pageSize=9999` hack，改为专用的 list-all endpoint 或前端缓存策略
- Add Member 表单改用 React Hook Form + Zod，与项目其他表单保持一致
- 将 `useRef` + queryFn 内 setState 的反模式改为 `useEffect` 监听

## Capabilities

### New Capabilities
- `org-assignment-integrity`: 人员分配操作的事务安全、外键验证、primary 一致性保证
- `org-frontend-quality`: Org App 前端组件拆分、表单规范化、交互优化

### Modified Capabilities
- `error-handling-quality`: Org App handler 层增加更完整的 sentinel error → HTTP status 映射

## Impact

- **后端文件**：`internal/app/org/assignment_service.go`、`assignment_repository.go`、`assignment_handler.go`、`department_service.go`、`position_repository.go`
- **前端文件**：`web/src/apps/org/pages/assignments/index.tsx`（拆分为多个文件）、`web/src/apps/org/components/`（新增子组件）
- **API**：无 breaking change，仅内部逻辑修复
- **数据库**：无 schema 变更
- **依赖**：无新增依赖（`Command` 组件已在 shadcn/ui 中）
