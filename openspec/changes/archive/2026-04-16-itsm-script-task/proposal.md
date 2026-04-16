## Why

BPMN Script Task 允许在流程中自动执行变量计算和数据转换，无需人工干预或外部 webhook。典型 ITSM 场景：
- "根据优先级和影响范围自动计算 SLA 等级"（`var.sla_level = var.priority == 'P0' ? 1 : var.impact == 'high' ? 2 : 3`）
- "格式化通知标题"（`var.notify_title = 'SLA 升级: ' + var.ticket_code`）
- "设置审批人数阈值"（`var.approver_count = var.amount > 10000 ? 3 : 1`）

当前这些逻辑只能硬编码在 action 节点的 webhook 中，增加了外部依赖和延迟。Script Task 作为自动节点内联执行，零网络开销。

## What Changes

- **新增 script 节点类型执行逻辑**：自动节点（IsAutoNode=true），执行后立即继续
- **NodeData 扩展**：`assignments: [{variable: string, expression: string}]` 定义变量赋值列表
- **handleScript 实现**：遍历 assignments，通过 `expr-lang/expr` 表达式引擎求值，写入 process variables（scope_id 来自 token），然后自动推进到下一节点
- **安全约束**：表达式引擎仅支持变量引用 + 算术 + 比较 + 字符串拼接 + 三元运算，禁用函数调用和 IO
- **从 UnimplementedNodeTypes 移除 NodeScript**
- **IsAutoNode 加入 NodeScript**

## Capabilities

### New Capabilities
- `itsm-script-task`: 脚本任务节点 + 变量赋值执行 + expr-lang/expr 表达式引擎

### Modified Capabilities
- `itsm-classic-engine`: processNode 新增 script case，IsAutoNode 包含 script

## Impact

- **后端**：`engine/classic.go` 新增 handleScript (~40 行)；`engine/engine.go` 常量更新 (~5 行)；`engine/expr.go` 表达式引擎封装 (~60 行)；新增 `expr-lang/expr` 依赖
- **前端**：无改动（⑥ itsm-bpmn-designer 中实现节点 UI）
- **依赖**：② itsm-process-variables（写入 process variables）
