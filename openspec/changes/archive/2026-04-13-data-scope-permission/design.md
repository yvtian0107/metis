## Context

当前 Metis 权限体系由两层构成：
1. **API 访问控制**：Casbin `(roleCode, /api/path, method)` 三元组，控制"能不能调接口"
2. **菜单可见控制**：Casbin `(roleCode, menu:perm, read)` 控制前端菜单展示

系统缺少第三层——**数据范围控制**（Row-level Scoping）。随着 ITSM 和可观测性模块引入，需要"运维部经理只看运维部数据"等按组织边界的数据隔离能力。

同时，组织架构（`internal/app/org/`）目前是可插拔 App，发许可（license）等独立服务不依赖组织，因此组织原语不能进内核。数据范围能力必须以**接口契约**的方式在内核中定义，由 Org App 按需实现。

## Goals / Non-Goals

**Goals:**
- 在 Role 上引入 `dataScope` 枚举策略（ALL / DEPT_AND_SUB / DEPT / SELF / CUSTOM）
- Kernel 定义 `OrgScopeResolver` 接口，DataScopeMiddleware nil-safe
- Org App 实现 `OrgScopeResolver`，基于现有 `GetUserDepartmentScope()` BFS 逻辑
- User 新增 `ManagerID` 自关联字段，支持多级审批链推导
- Org App 新增 UserGroup / UserGroupMember，支持 on-call 组等扁平团队
- 渐进式接入：各业务 repository 通过 `ListParams.DeptScope` 按需消费，不强制一次性改全

**Non-Goals:**
- 不引入多租户（TenantID），明确不在此次范围内
- 不改动 Casbin API 访问控制层（三元组模型不变）
- 不实现具体业务模块（ITSM 工单、可观测告警）的 scope 接入，仅提供基础设施
- 不做 Position 与 Department 的绑定（岗位仍为全局）

## Decisions

### 决策 1：DataScope 挂在 Role 上，而非 UserPosition 上

**选择**：`Role.DataScope` 枚举 + `RoleDeptScope` 自定义表

**备选方案**：在 `UserPosition` 上加 `dataScope` 字段（每个用户-部门分配独立控制）

**理由**：
- Role-based 心智模型更简单："运维经理角色能看本部门+下级"，一条策略覆盖所有该角色成员
- 现有 `User.RoleID` 是单一全局角色，无需改动认证体系
- UserPosition-based 方案需要在每次分配时单独配置 scope，运维成本高
- CUSTOM 类型用 `RoleDeptScope` 表补充，灵活性不损失

**运行时计算**：
```
role.dataScope == ALL          → deptScope = nil（不过滤）
role.dataScope == SELF         → deptScope = []（仅自己）
role.dataScope == DEPT         → GetUserDepartmentIDs(userID)
role.dataScope == DEPT_AND_SUB → GetUserDepartmentScope(userID)  ← 复用现有 BFS
role.dataScope == CUSTOM       → SELECT dept_id FROM role_dept_scopes WHERE role_id=?
```

---

### 决策 2：OrgScopeResolver 接口放在 internal/app/app.go（App 层接口包）

**选择**：在 `internal/app/` 下定义接口，而非放在 `internal/` kernel 包

**理由**：
- Org App 通过 `do.Provide()` 注册实现，DataScopeMiddleware 通过 `do.InvokeAs[OrgScopeResolver]` 尝试获取（可返回 nil）
- 避免 `internal/middleware` 直接 import App 层代码（保持分层方向）
- `internal/app/app.go` 本身已是 App 与 Kernel 的接口层，接口定义在此最自然

**nil-safe 处理**：
```go
resolver, _ := do.InvokeAs[OrgScopeResolver](i)  // nil if Org not installed
// middleware: if resolver == nil → skip scope filtering
```

---

### 决策 3：DataScopeMiddleware 注入 Gin Context，而非直接在 Repository 查询

**选择**：中间件解析 scope → `c.Set("deptScope", []uint{...})` → Handler 从 ctx 取出传给 Service/Repo

**备选方案**：每个 Service 方法自行调用 AssignmentService.GetUserDepartmentScope()

**理由**：
- 中间件统一入口，避免每个 Service 重复调用和缓存问题
- scope 解析成本（DB 查询）只在中间件执行一次，同一请求内复用
- Handler 层对 scope 感知，可按业务需要决定是否传入（部分接口不需要 scope 过滤）

---

### 决策 4：User.ManagerID 加在 kernel User 模型，而非 UserPosition

**选择**：`User.ManagerID *uint`（自关联）放在 `internal/model/user.go`

**理由**：
- 直属上级是用户的固有属性，独立于"在哪个部门任职"
- 一个人可能兼任多个部门，但直属上级通常唯一
- 放在 UserPosition 则需要从多个分配记录中判断"主要上级"，增加复杂度
- ITSM 审批链推导：`userID → user.managerID → manager.managerID → ...` 简单直接

---

### 决策 5：UserGroup 放在 Org App，而非独立 App 或 Kernel

**选择**：`internal/app/org/` 新增 UserGroup 模型

**理由**：
- UserGroup 的主要消费场景是与部门结构配合使用（ITSM 中 on-call 组 = 跨部门人员集合）
- 与 Department/Position/UserPosition 同属"组织管理"语义域
- 作为独立 App 过于轻量，拆分收益低于管理成本

## Risks / Trade-offs

**[风险 1] DataScope 中间件增加每次请求的 DB 查询** → 缓解：scope 解析结果仅对 DEPT/DEPT_AND_SUB/CUSTOM 类型需要查询；ALL/SELF 类型无 DB 开销。后续可加请求级内存缓存（sync.Map + requestID key）。

**[风险 2] 各业务 repository 渐进式接入，过渡期数据隔离不完整** → 缓解：明确文档标注哪些 List API 已接入 DeptScope，哪些尚未接入，避免误判安全边界。

**[风险 3] CUSTOM scope 的部门集合管理 UI 复杂度** → 缓解：CUSTOM 类型为高级功能，UI 使用部门多选树（复用现有 department tree 组件），初版不做继承语义。

**[风险 4] User.ManagerID 循环引用（A→B→A）** → 缓解：更新 ManagerID 时做循环检测（BFS/DFS 向上追溯，不超过配置的最大深度，默认 10 级）。

## Migration Plan

1. **数据库迁移（自动）** — GORM AutoMigrate 在启动时执行：
   - `roles` 表加列 `data_scope VARCHAR(32) DEFAULT 'all'`
   - `users` 表加列 `manager_id BIGINT DEFAULT NULL`
   - 新增 `role_dept_scopes` 表
   - 新增 `user_groups` / `user_group_members` 表

2. **存量数据兼容** — 现有 Role 的 dataScope 默认为 `all`，行为与当前完全一致，无数据迁移风险

3. **渐进式 scope 接入** — DataScopeMiddleware 上线后，各业务模块逐步在 ListParams 中读取 `deptScope`；未接入的接口行为不变（scope 字段为 nil，不过滤）

4. **回滚** — dataScope 字段默认 `all`，回滚代码后数据库列保留无害；RoleDeptScope 等新表可保留

## Open Questions

- **DataScope 是否需要在 Casbin 白名单之外独立白名单？** 例如某些公共 API（如公告、站点信息）不应受 scope 过滤，目前 DataScopeMiddleware 应与 Casbin 白名单复用同一个白名单列表，待确认。
- **UserGroup 是否需要 dataScope 语义？** 即"某用户组的成员可见该组相关数据"——此次先不引入，UserGroup 仅用于 ITSM 分组路由，不扩展到数据权限维度。
