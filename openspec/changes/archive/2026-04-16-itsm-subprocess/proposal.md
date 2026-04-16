## Why

嵌入子流程（Embedded Subprocess）将一组节点封装为可复用的子单元，是 BPMN 中组织复杂流程的核心机制。ITSM 场景：
- "多级审批"子流程可被不同工作流引用
- "安全评审流程"作为子模块嵌入到变更管理流程中
- 子流程有独立的变量作用域——内部变量不会污染主流程

## What Changes

- **新增 subprocess 节点类型执行逻辑**
- **NodeData 扩展**：`subProcessDef: {nodes: [], edges: []}` 内嵌子流程定义（JSON 结构与主流程 workflow_json 相同）
- **handleSubprocess 实现**：
  - 解析 subProcessDef 生成独立的 nodeMap/outEdges
  - 创建子 token（token_type=subprocess, scope_id=node.id, parent=当前 token）
  - 找到子流程的 start 节点，调用 processNode 递归推进（传入子流程的 def/nodeMap/outEdges）
  - 子流程 end → 子 token completed → tryCompleteJoin 唤醒父 token 继续
- **变量作用域隔离**：子流程内的 writeFormBindings 使用 scope_id=node.id，与主流程 scope_id="root" 隔离
- **Boundary Event 支持**：subprocess 节点可附着 b_timer / b_error（依赖 ⑤b）
- **ValidateWorkflow 增强**：递归校验 subProcessDef 内部结构

## Capabilities

### New Capabilities
- `itsm-subprocess`: 嵌入子流程执行 + 变量作用域隔离

### Modified Capabilities
- `itsm-classic-engine`: processNode 新增 subprocess case
- `itsm-workflow-validator`: 递归校验 subProcessDef

## Impact

- **后端**：`engine/classic.go` 新增 handleSubprocess (~80 行)；`engine/validator.go` 递归校验 (~40 行)；`engine/engine.go` 常量更新 (~5 行)
- **前端**：无改动
- **依赖**：③ itsm-execution-tokens + ⑤b itsm-boundary-events（可选，支持 subprocess 上的 boundary events）
