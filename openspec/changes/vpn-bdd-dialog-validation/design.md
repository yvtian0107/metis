## Context

服务台 Agent (`itsm.service_desk`) 通过多轮对话引导用户提单，工具链按 match → load → draft_prepare → confirm → create 状态机推进。Agent 的系统提示中有 Rule 11（跨路由冲突识别）、Rule 12（同路由多选合并）、Rule 13（必填缺失追问）三条对话校验规则，但目前：

1. `draft_prepare` 返回 `multivalue_on_single_field` warning 时不附带 `resolved_values`，Agent 需自行对照 `option_route_map` 推理——依赖 LLM 能力，不够可靠
2. 无 BDD 测试验证 Agent 在这三种场景下的实际行为

现有 BDD 基础设施覆盖了 SmartEngine 决策层（vpn_smart_flow / vpn_smart_engine_deterministic），但服务台 Agent 对话层完全没有 BDD 覆盖。

## Goals / Non-Goals

**Goals:**
- `draft_prepare` 的 `multivalue_on_single_field` warning 补全 `resolved_values`，携带每个值对应的路由分支
- 3 个 BDD Scenario 验证 Agent 的对话校验行为（跨路由冲突、同路由多选、必填缺失）
- 主断言工具调用序列，辅助断言回复内容

**Non-Goals:**
- 不覆盖 Classic Engine 场景
- 不测试 draft_prepare 工具本身的确定性行为（已有 spec 覆盖）
- 不改动 Agent 系统提示规则

## Decisions

### D1: resolved_values 实现位置

**选择**: 在 `draftPrepareHandler` 中，检测到 `multivalue_on_single_field` 时直接附加。

**理由**: `draftPrepareHandler` 已调用 `op.LoadService(state.LoadedServiceID)` 获取 `ServiceDetail`，其中包含 `RoutingFieldHint`。数据已就绪，无需额外查询或状态存储。

**替代方案**:
- 在 `service_load` 阶段将 `RoutingFieldHint` 写入 `ServiceDeskState` → 多余，draft_prepare 已能拿到
- 让 Agent 自行对照 `option_route_map` → 不可靠，依赖 LLM 推理

**实现**: 当 `multivalue_on_single_field` 且 `field == detail.RoutingFieldHint.FieldKey` 时，split 逗号值，逐个查 `OptionRouteMap`，生成 `resolved_values` 数组。

```go
type resolvedValue struct {
    Value string `json:"value"`
    Route string `json:"route"`
}

// 在 multivalue 检测分支后追加:
if hint := detail.RoutingFieldHint; hint != nil && key == hint.FieldKey {
    parts := strings.Split(strVal, ",")
    var rv []resolvedValue
    for _, p := range parts {
        v := strings.TrimSpace(p)
        rv = append(rv, resolvedValue{Value: v, Route: hint.OptionRouteMap[v]})
    }
    w.ResolvedValues = rv
}
```

### D2: BDD 测试架构 — ReactExecutor 直调

**选择**: 直接构造 `ReactExecutor` + 真实 LLM + 真实 ITSM 工具，不走 HTTP/SSE。

**理由**:
- 已有先例：`vpn_smart_flow.feature` 用真实 LLM 调用 SmartEngine
- ReactExecutor 有完整的 Event Channel 接口，可直接收集 tool call 事件
- 不经过 HTTP 层减少依赖，测试更聚焦

**测试入口**:
```
Given 步骤:
  1. 初始化 DB + seed users/positions/departments
  2. 发布 VPN 服务（smart engine, 含 routing_field_hint）
  3. 创建 ReactExecutor + ITSM ToolRegistry（内存 StateStore）
  4. 构造 Agent 系统提示 + 用户消息

When 步骤:
  执行 ReactExecutor.Execute()，收集所有 Event

Then 步骤:
  从 Event Channel 提取 EventTypeToolCall 事件，断言工具调用序列
  从最终 Content 断言回复内容（辅助）
```

### D3: 双路径断言策略

**选择**: 路径 A 优先，路径 B 作为可接受 fallback。

**路径 A**（最佳）: Agent 在 `draft_prepare` 前根据 `routing_field_hint` 识别冲突，不调用 `draft_prepare`。
**路径 B**（可接受）: Agent 调用了 `draft_prepare`，但收到 `multivalue_on_single_field` + `resolved_values` 后未继续推进到 `draft_confirm`。

**断言逻辑**:
```
if !hasToolCall("itsm.draft_prepare"):
    PASS  # 路径 A
else:
    assert !hasToolCall("itsm.draft_confirm")  # 路径 B
```

**理由**: LLM 行为非确定性。Rule 11 要求 draft 前拦截，但 Agent 可能偶尔走 fallback 路径。两条路径都体现了正确的对话校验能力——关键是 Agent 最终向用户澄清而非直接提交。

### D4: StateStore 测试实现

**选择**: 内存 map 实现 `StateStore` 接口，不依赖 DB 持久化。

**理由**: BDD 每个 Scenario 独立，无需跨 Scenario 状态。内存 StateStore 更快，无 DB 依赖。

```go
type memStateStore struct {
    states map[uint]*ServiceDeskState
}
```

### D5: 用户消息设计

三个 Scenario 的用户消息需精心设计，既要自然又要明确触发目标行为：

| Scenario | 用户消息 | 触发点 |
|----------|---------|--------|
| 跨路由冲突 | "申请VPN，需要网络调试和安全审计" | `network_support` + `security` 分属不同路由 |
| 同路由多选 | "申请VPN，需要网络调试和远程维护" | `network_support` + `remote_maintenance` 同一路由 |
| 必填缺失 | "帮我开个VPN" | 缺 vpn_type, access_period 等必填字段 |

## Risks / Trade-offs

**[LLM 非确定性]** → 双路径断言 + 宽松内容匹配（regex 而非精确字符串）。标记为 `@llm` tag，失败时可重试。

**[测试耗时]** → 真实 LLM 调用慢（每 Scenario 可能 10-30s）。与 smart_flow 测试共用 `@llm` tag，`make test-bdd` 默认跳过，需 `make test-llm` 专门运行。

**[VPN 服务路由配置]** → 测试依赖 VPN 服务的 workflow_json 包含 `exclusive_gateway`，需确保 `vpn_support_test.go` 中的服务定义包含正确的路由条件。复用现有 `givenSmartServicePublished` 并扩展路由配置。
