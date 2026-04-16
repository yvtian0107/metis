## Context

ITSM 引擎当前已注册 `subprocess` 节点类型（ValidNodeTypes），但标记为未实现（UnimplementedNodeTypes）。Token 类型中已预留 `TokenSubprocess = "subprocess"`，变量作用域机制（scope_id）已在 ② itsm-process-variables 中建立。

现有基础设施：
- `processNode(ctx, tx, def, nodeMap, outEdges, token, operatorID, node, depth)` 递归推进，def/nodeMap/outEdges 作为参数传递
- `handleEnd` 处理 token 完成：检测 ParentTokenID 判断子 token → 计数 remaining → 全部完成则处理父 token
- `Progress()` / `HandleActionExecute` / `HandleWaitTimer` / `HandleBoundaryTimer` 从 ticket.WorkflowJSON 加载主流程 def
- 边界事件（⑤b）已支持 b_timer/b_error 的 attach/cancel/trigger

## Goals / Non-Goals

**Goals:**
- 实现嵌入式子流程（embedded subprocess）执行逻辑
- 子流程变量作用域隔离（scope_id = subprocess node ID）
- 子流程完成后自动恢复父流程
- Progress / task handler 对子流程内节点的透明支持
- 递归校验 SubProcessDef 结构完整性

**Non-Goals:**
- 嵌套子流程（subprocess 内再嵌 subprocess）— v1 仅支持单层
- 可复用子流程（call activity / 引用外部流程定义）— 未来增强
- 子流程变量 I/O 映射（input/output variable mapping）— v1 纯隔离
- 子流程上的边界事件（b_timer/b_error attach to subprocess node）— 未来增强
- 多实例子流程（multi-instance subprocess）— 未来增强

## Decisions

### D1: SubProcessDef 存储方式

| 方案 | 描述 | 优劣 |
|------|------|------|
| **A: json.RawMessage** | NodeData 新增 `SubProcessDef json.RawMessage` | 按需解析，NodeData 轻量 |
| B: *WorkflowDef 结构体 | NodeData 嵌入已解析的 WorkflowDef | 反序列化时耦合，空值处理复杂 |

**选择 A**。与 NodeData 中其他可选字段风格一致，仅在 handleSubprocess 和 validator 中解析。

### D2: handleSubprocess 执行流程

```
handleSubprocess(ctx, tx, token, operatorID, node, nodeData, depth)
  ├─ ParseWorkflowDef(nodeData.SubProcessDef)
  ├─ subDef.BuildMaps() → subNodeMap, subOutEdges
  ├─ 父 token → status = waiting
  ├─ 创建子 token (parent=token.ID, type=subprocess, scope_id=node.ID)
  ├─ subDef.FindStartNode() → 找到起始节点
  ├─ 创建 "subprocess_started" timeline
  └─ processNode(ctx, tx, subDef, subNodeMap, subOutEdges, subToken, ..., startTarget, depth+1)
```

关键：processNode 传入子流程的 def/maps，后续递归调用自然使用子流程上下文。

### D3: 子流程完成 — completeSubprocess

**问题**：子流程 end 节点触发 handleEnd 时，现有逻辑会标记父 token 为 completed（并行分支语义）。但子流程完成后父 token 应**继续推进**，而非终止。

**方案**：在 processNode 的 NodeEnd case 中，检测 `token.TokenType == TokenSubprocess`，调用独立的 `completeSubprocess` 而非 handleEnd。

```
processNode → NodeEnd:
  if token.TokenType == TokenSubprocess:
    → completeSubprocess(ctx, tx, token, operatorID, node, depth)
  else:
    → handleEnd(...)
```

completeSubprocess 流程：
1. 创建 end activity（completed）
2. 完成子流程 token
3. 加载 ticket.WorkflowJSON → 解析主流程 def
4. 重新激活父 token（waiting → active）
5. 从 subprocess 节点的出边继续（processNode 下一个目标节点）

### D4: resolveWorkflowContext 辅助函数

**问题**：Progress() / HandleActionExecute / HandleWaitTimer / HandleBoundaryTimer 都从 ticket.WorkflowJSON 加载工作流。但子流程内的 activity 节点存在于子流程 def 中，不在主流程 def 里。

**方案**：新增 `resolveWorkflowContext(tx, token, ticket)` 辅助函数：

```go
func resolveWorkflowContext(tx *gorm.DB, token *executionTokenModel, ticketWorkflowJSON string) (*WorkflowDef, map[string]*WFNode, map[string][]*WFEdge, error)
```

逻辑：
1. 解析主流程 def
2. 若 token.TokenType == TokenSubprocess 且 ParentTokenID != nil：
   - 加载父 token → 找到 subprocess 节点 → 解析 SubProcessDef
   - 返回子流程的 def/nodeMap/outEdges
3. 否则返回主流程的 def/nodeMap/outEdges

所有需要 workflow context 的入口统一调用此函数。v1 仅支持单层（父 token 必须是主流程 token）。

### D5: 变量作用域隔离

子流程 token 的 scope_id 设为 subprocess 节点 ID（如 `"node_subprocess_1"`）。子流程内的 form binding / script assignment 写入该 scope。主流程使用 scope_id = `"root"`。

v1 无跨 scope 访问。buildScriptEnv 已按 scope_id 查询变量，无需修改。

### D6: Subprocess 出边约束

类似 script 节点，subprocess 必须有且仅有一条出边。Validator 新增此约束。

### D7: Validator 递归校验

遇到 subprocess 节点时：
1. 检查 SubProcessDef 非空且可解析
2. 递归调用校验逻辑（复用相同规则）
3. 子流程内的 ValidationError 添加 NodeID 前缀以区分上下文

## Risks / Trade-offs

- **单层限制** → 嵌套子流程在 v1 不支持。resolveWorkflowContext 仅检查一层 parent。若需嵌套，需改为 token 链向上遍历。Mitigation：validator 拒绝子流程内出现 subprocess 节点。
- **WorkflowJSON 重复加载** → completeSubprocess 和 resolveWorkflowContext 需要重新解析主流程 JSON。Mitigation：单次解析开销极小（JSON 通常 < 100KB），不需要缓存。
- **子流程边界事件** → v1 不支持。subprocess 节点不会被 attachBoundaryEvents 处理（它只对 form/approve/process 生效）。未来增强需在 processNode 中为 subprocess case 添加 attach 调用。
