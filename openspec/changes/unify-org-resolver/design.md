## Context

Metis 的组织管理查询能力经历了两轮演进：

**第一轮（初始）**：4 套独立接口散布在 3 个 App 中——`app.OrgScopeResolver`（DataScope）、`app.OrgUserResolver`（ITSM 匹配）、`ai.OrgResolver`（AI 工具）、`engine.OrgService`（参与人解析）。

**第二轮（已完成）**：合并为 `app.OrgResolver` 统一接口，覆盖 DataScope + ID 映射 + 富上下文查询。`organization.org_context` 工具迁入 Org App。数据类型移入 `app` 包。

**当前状态**：统一接口缺少 "按组织条件找人" 的方法。`engine.OrgService` 仍然存在但生产环境为 nil，`ParticipantResolver` 对 position/department/manager 类型的解析全部失败。`operator.go` 的 `ValidateParticipants` 直接 JOIN 4 张 org 表。

## Goals / Non-Goals

**Goals:**
- `app.OrgResolver` 成为组织查询的唯一接口（删除 `engine.OrgService`）
- ITSM `ParticipantResolver` 在生产环境正确接通（非 nil）
- `ValidateParticipants` 零 raw SQL 跨域查询
- BDD 测试适配新接口，全部通过

**Non-Goals:**
- 不改 Org App 的 REST API
- 不改前端
- 不新增/删除任何 AI Tool（工具名不变）
- 不改 DataScope 中间件逻辑
- 不做性能优化（查询量极小，< 20 条/次）

## Decisions

### Decision 1: 扩展 `app.OrgResolver` 而非创建新接口

**选择**：在现有 `app.OrgResolver` 上新增 5 个方法。

**备选**：创建独立的 `app.OrgParticipantService` 接口。

**理由**：
- 避免再次出现 "多接口" 碎片化
- 消费者（ParticipantResolver、Operator）只需解析一个 IOC 类型
- 接口方法数量从 6 → 11，仍在合理范围
- 所有方法的底层实现都基于同一批 Org 表（departments、positions、user_positions），内聚性高

### Decision 2: `ParticipantResolver` 直接消费 `app.OrgResolver`

**选择**：删除 `engine.OrgService`，`ParticipantResolver` 的构造函数改为接收 `app.OrgResolver`。

**备选**：写一个 adapter 桥接 `app.OrgResolver` → `engine.OrgService`。

**理由**：
- adapter 只是多一层间接，没有实质好处
- `engine.OrgService` 从未有过真实实现，删除不影响任何现有功能
- 减少 ITSM engine 包对独立接口的维护负担

### Decision 3: `Operator` 接收 `app.OrgResolver` 替代 `*gorm.DB` 中的 org 查询

**选择**：`Operator` 新增 `orgResolver app.OrgResolver` 字段（可选，nil 安全），`ValidateParticipants` 通过该接口查询。

**备选**：让 `ValidateParticipants` 委托给 `ParticipantResolver`。

**理由**：
- `ValidateParticipants` 只需 "有没有活跃用户" 的计数检查，不需要完整的参与人解析流程
- `ParticipantResolver.Resolve` 需要 `ticketID` 和 `*gorm.DB` 参数，而 validate 发生在工单创建之前，没有 ticketID
- 直接调用 `OrgResolver.FindUsersByPositionCode` 更简单直接
- `Operator` 仍保留 `db` 字段用于 ITSM 自身的表查询（service_definitions、tickets 等），只是不再跨域查 org 表

### Decision 4: 用户活跃性检查保留在 Operator 内

**选择**：`ValidateParticipants` 中 type=user 的 `users.is_active` 检查保留为直接数据库查询。

**理由**：`users` 表属于内核，不属于 Org App 领域。OrgResolver 不应承担 "检查用户是否存在" 的职责。这个查询是对内核表的合法访问。

## Risks / Trade-offs

**[Risk] `app.OrgResolver` 接口膨胀** → 11 个方法仍可管理。如果未来继续增长，可拆分为嵌入式子接口（Go 支持 interface embedding）。

**[Risk] BDD 测试中 `testOrgService` 需要重写** → 改为实现 `app.OrgResolver`。由于测试代码中已有类似查询逻辑（steps_common_test.go），迁移成本低。

**[Risk] Org App 未安装时 ITSM 行为变化** → 无变化。当前 `ParticipantResolver.orgService` 为 nil 时已返回 "需要安装组织架构模块" 错误。改用 `app.OrgResolver` 后逻辑不变，nil 检查保持一致。
