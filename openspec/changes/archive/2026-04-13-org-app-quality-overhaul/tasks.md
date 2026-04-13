## 1. 后端：Primary 管理事务安全

- [x] 1.1 重构 `AssignmentRepo.AddPosition` 为 `AddPositionWithPrimary(up *UserPosition, setPrimary bool)`，在事务中处理 demote+create
- [x] 1.2 重构 `AssignmentRepo.UpdatePosition` 支持在事务中处理 isPrimary 变更（demote+update 原子操作）
- [x] 1.3 修改 `AssignmentService.AddUserPosition`：移除 `_ = s.demoteCurrentPrimary()`，改为调用新的 repo 事务方法
- [x] 1.4 修改 `AssignmentService.UpdateUserPosition`：移除 `_ = s.demoteCurrentPrimary()`，改为调用新的 repo 事务方法
- [x] 1.5 修复 `AssignmentRepo.SetPrimary`：在 demote-all 后验证目标 assignment 的 `RowsAffected > 0`，否则回滚并返回 `ErrAssignmentNotFound`
- [x] 1.6 删除 `AssignmentService.demoteCurrentPrimary` 方法（逻辑已收敛到 repo 事务）

## 2. 后端：外键验证与错误处理

- [x] 2.1 `AssignmentService` 注入 `DepartmentRepo` 和 `PositionRepo`（通过 IOC）
- [x] 2.2 `AddUserPosition` 增加部门存在性 + active 校验，返回 `ErrDepartmentNotFound` / `ErrDepartmentInactive`
- [x] 2.3 `AddUserPosition` 增加岗位存在性 + active 校验，返回 `ErrPositionNotFound` / `ErrPositionInactive`
- [x] 2.4 新增 sentinel errors：`ErrDepartmentInactive`, `ErrPositionNotFound`, `ErrPositionInactive`
- [x] 2.5 `AssignmentHandler.RemoveUserPosition` 增加 `gorm.ErrRecordNotFound` → HTTP 404 映射
- [x] 2.6 `AssignmentHandler.AddUserPosition` 增加新 sentinel errors 到 HTTP 400 映射

## 3. 后端：代码质量改进

- [x] 3.1 定义 `UpdateDepartmentInput` struct，重构 `DepartmentService.Update` 签名
- [x] 3.2 更新 `DepartmentHandler.Update` 适配新的 struct 入参
- [x] 3.3 重构 `GetUserDepartmentScope`：单次加载所有 active 部门 `(id, parent_id)`，内存 BFS 构建子树
- [x] 3.4 新增 `DepartmentRepo.ListAllIDsWithParent(activeOnly bool) ([]IDParent, error)` 方法
- [x] 3.5 修复或移除 `CountAssignments`：改为固定列名的方法签名（如 `CountByDepartment(deptID uint)`），或删除未使用的方法
- [x] 3.6 `PositionHandler.List` 支持 `pageSize=0` 返回全部（不分页）

## 4. 前端：Assignments 页面拆分

- [x] 4.1 创建 `pages/assignments/department-tree.tsx` — 提取 DepartmentTreeItem 组件和树面板逻辑
- [x] 4.2 创建 `pages/assignments/member-list.tsx` — 提取成员表格、搜索、分页逻辑
- [x] 4.3 创建 `pages/assignments/add-member-sheet.tsx` — 提取添加成员 Sheet
- [x] 4.4 重写 `pages/assignments/index.tsx` 为容器组件，协调子组件间的共享状态
- [x] 4.5 验证拆分后页面功能不变：部门树选择、成员列表、添加/移除成员、设置主岗位

## 5. 前端：交互优化

- [x] 5.1 用 shadcn Command 组件替换 add-member-sheet 中手搓的 Popover 用户选择器
- [x] 5.2 用户搜索输入增加 300ms debounce（用 `useDebouncedValue` 或 `setTimeout` 模式）
- [x] 5.3 岗位加载改用 `pageSize=0`，消除 `pageSize=9999` hack
- [x] 5.4 Add Member 表单改用 React Hook Form + Zod schema 验证
- [x] 5.5 将 queryFn 中的 `useRef` + `setState` 反模式改为 `useMemo` 派生初始展开状态
