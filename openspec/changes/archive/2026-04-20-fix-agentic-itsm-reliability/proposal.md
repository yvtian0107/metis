## Why

Agentic ITSM 的 smart engine 在产品设计 review 中发现了 25 个问题，其中 4 个 P0 级别会导致工单在生产环境中卡死无法继续。核心原因是 smart engine 的流程推进依赖分散的触发点，缺乏统一的 continuation 机制，且多个关键路径（reject、action 完成、并行调度）存在断裂。前端缺少对破坏性操作的权限校验和确认保护。

## What Changes

### 流程连续性修复 (P0)
- Reject AI 决策后自动触发新的决策循环，将 reject 原因注入上下文
- Action activity 完成后提交 `itsm-smart-progress` task 推进下一步
- 并行计划中的 action 类 activity 正确调度执行
- Recovery 逻辑移除对不存在的 `ai_disabled_reason` 列的引用，改用 `ai_failure_count >= 3` 判断

### 并发安全修复 (P1)
- 并行活动收敛检查引入 `SELECT FOR UPDATE` 行锁，防止重复触发决策循环
- Recovery 排除 `pending_approval` 状态的活动，防止误判孤儿工单
- Sequential 计划改为逐步创建 activity（完成一个再创建下一个）

### 前端安全与体验修复 (P1-P2)
- Reject AI 决策增加确认弹窗
- Override 操作（jump/reassign/retry）增加权限校验
- Flow visualization 中 `overriddenBy` 显示用户名而非 ID
- Flow visualization 增加 `failed` 状态颜色（红色）
- Retry AI 增加确认弹窗
- 修复 header OverrideActions 未传 `aiFailureCount` 导致 Retry AI 不显示的问题

### 引擎健壮性修复 (P2)
- `execute_action` tool 使用带超时的 context 而非 `context.Background()`
- `execute_action` 增加幂等保护（检查 action 是否已执行成功）
- `Signal()` 根据 engine type 分派到正确的 engine
- Draft hash 使用 sorted keys 序列化，避免 map 遍历顺序不确定导致虚假变化
- Agent seed 不覆盖已自定义的 system prompt（仅首次创建时设置）
- Tool binding 在 UPDATE 时也同步更新

### 体验细节修复 (P3)
- Smart engine idle 状态显示"AI 正在准备下一步"而非空白
- 无 assignee 时的 human activity 显示提示信息
- Outcome 统一：HumanActivityActions 使用与后端一致的 outcome 值

## Capabilities

### New Capabilities

- `itsm-smart-continuation`: 统一的 smart engine 流程推进调度机制，确保任何 activity 状态变更都能可靠触发下一步决策循环

### Modified Capabilities

- `itsm-smart-engine`: 修复 sequential plan 创建逻辑、并行收敛竞态、Signal 分派
- `itsm-smart-recovery`: 修复 recovery 查询列名错误、排除 pending_approval 误判
- `itsm-smart-action-tool`: 修复 execute_action 的 context 和幂等性
- `itsm-decision-tools`: execute_action 超时和去重保护
- `itsm-agent-tools`: 修复 draft hash 序列化、状态机 re-match 清理
- `itsm-smart-ticket-detail`: 前端权限校验、确认弹窗、状态显示修复
- `itsm-smart-countersign`: 并行收敛的并发安全修复

## Impact

- **Backend**: `internal/app/itsm/engine/smart.go`, `smart_tools.go`, `tasks.go`, `engine.go`; `internal/app/itsm/ticket_service.go`; `internal/app/itsm/tools/handlers.go`, `provider.go`
- **Frontend**: `web/src/apps/itsm/components/ai-decision-panel.tsx`, `override-actions.tsx`, `smart-current-activity-card.tsx`, `smart-flow-visualization.tsx`; `web/src/apps/itsm/pages/tickets/[id]/index.tsx`
- **API**: 无新增 API，现有 override 端点需增加权限中间件
- **数据库**: 无 schema 变更
- **风险**: P0 修复涉及核心流程推进逻辑，需要完整的 BDD 测试覆盖
