## Context

ITSM BDD 测试基础设施（godog suite、features 目录、bddContext 骨架）已在 `itsm-bdd-infrastructure` spec 中建立。当前 `bddContext` 仅含 `db` 和 `lastErr`，AutoMigrate 仅覆盖 3 个配置模型。VPN 开通申请场景需要完整的工单生命周期（Ticket → Activity → Assignment → Timeline）、Org 组织结构（Department → Position → UserPosition）、以及可工作的 ClassicEngine 实例。

bklite-cloud 参考实现使用 pytest-bdd + Django ORM，其 `common_steps.py` 提供了参与人准备、Agent 确认等公共步骤。Metis 需要用 godog + GORM 移植等价功能。

关键差异：
- bklite 有独立的 `UserDepartment` + `UserPosition`，Metis 只有 `UserPosition`（同时含 DepartmentID + PositionID）
- bklite 步骤直接调 Python 函数，Metis 需要在测试中实例化引擎组件

## Goals / Non-Goals

**Goals:**
- bddContext 能支撑 VPN 全部 9 个 Phase 的 BDD 测试
- 公共 Given 步骤可被所有 VPN feature 文件复用
- ClassicEngine 在测试中能真实驱动工单流转（含参与者解析）
- 每个 Scenario 完全隔离（独立内存 DB）

**Non-Goals:**
- 不实现 SmartEngine 测试支持（Phase 6+ 再做）
- 不实现服务台 tool chain 测试支持（Phase 6 再做）
- 不创建任何 .feature 文件（Phase 2-3 再做）
- 不搭建 IOC 容器——直接构造依赖

## Decisions

### D1: OrgService 用直接查 DB 实现，不 mock

**选择**: 实现 `testOrgService struct{ db *gorm.DB }` 直接查 `user_positions` 表
**替代方案**: 内存 mock（map 映射），更快但失去 DB 层验证
**理由**: BDD 测试的价值在于验证真实行为链路。OrgService 查 DB 让测试覆盖到 GORM 关联查询和 Org 模型的 table name 映射。性能差异可忽略（内存 SQLite）。

### D2: 工作流 fixture 中 participant value 使用动态 ID 替换

**选择**: fixture 作为模板函数，接收 position/department ID 参数，返回完整 workflow JSON
**替代方案**: 硬编码 ID（如 "1", "2"）
**理由**: GORM AutoIncrement 在不同场景下可能产生不同 ID。模板函数确保 fixture 始终引用正确的实体。这是 Phase 2 的实现细节，Phase 1 只需确保 bddContext 能存储这些 ID。

### D3: AutoMigrate 全量模型（包括暂不使用的）

**选择**: 一次性 AutoMigrate 全部 ITSM 17 模型 + Org 3 模型 + AI 2 模型 + Kernel 2 模型
**替代方案**: 按需增量添加
**理由**: 避免后续 Phase 每次都改 reset()。内存 SQLite 创建表极快（<1ms/表），没有性能顾虑。ClassicEngine 内部会查 TicketTimeline、ExecutionToken 等表，缺表会 panic。

### D4: TaskSubmitter 用 no-op 实现

**选择**: `noopSubmitter` 直接返回 nil，不真正提交异步任务
**理由**: BDD 测试中引擎的异步任务（action-execute, wait-timer 等）不需要真正调度。如果未来某个场景需要验证任务提交，可以改为 recording submitter。

### D5: bddContext 字段设计为通用 map

**选择**: `users map[string]*model.User`（key=身份标签如"申请人"）, `positions map[string]*org.Position`（key=code）
**理由**: 与 bklite 的 `scenario_ctx["participants_by_identity"]` 对齐。map 比固定字段灵活，不同 feature 可以注册不同的参与人集合。

## Risks / Trade-offs

**[跨包导入]** → ITSM 测试 import `metis/internal/app/org` 和 `metis/internal/app/ai`。已确认无循环依赖（Org/AI 不 import ITSM），且 `test_helpers_test.go` 已有 `ai.Agent` 导入先例。

**[SQLite 外键约束]** → 内存 SQLite 默认不启用外键约束，GORM 的 foreignKey tag 只用于 Preload，不影响测试。但 `UserPosition.DepartmentID` 如果引用不存在的 Department，不会报错。这是可接受的——BDD 步骤会先创建 Department 再创建 UserPosition。

**[engine 内部 model alias]** → ClassicEngine 内部定义了 `ticketModel`、`activityModel` 等私有 struct 作为 GORM model alias（与 ITSM 包的公开 Ticket struct 映射同一张表）。AutoMigrate 用公开 struct，引擎内部用私有 alias，两者共存无冲突。
