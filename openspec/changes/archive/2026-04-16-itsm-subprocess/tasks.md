## Tasks

### NodeData & 常量更新

- [x] 1. 在 `workflow.go` 的 NodeData struct 新增 `SubProcessDef json.RawMessage` 字段（json tag: `subprocess_def,omitempty`）
- [x] 2. 在 `engine.go` 的 UnimplementedNodeTypes 中移除 NodeSubprocess

### resolveWorkflowContext 辅助函数

- [x] 3. 在 `classic.go` 新增 `resolveWorkflowContext(tx, token, ticketWorkflowJSON)` 函数：若 token.TokenType == TokenSubprocess 且 ParentTokenID != nil，加载父 token → 从主流程 nodeMap 找到 subprocess 节点 → 解析 SubProcessDef → 返回子流程的 def/maps；否则返回主流程的 def/maps

### handleSubprocess 实现

- [x] 4. 在 `classic.go` 新增 `handleSubprocess(ctx, tx, token, operatorID, node, nodeData, depth)` 方法：解析 SubProcessDef → BuildMaps → 父 token 设 waiting → 创建 subprocess token（parent=token.ID, type=subprocess, scope_id=node.ID）→ FindStartNode → processNode 到起始目标
- [x] 5. 在 `classic.go` processNode 的 switch 中新增 `case NodeSubprocess` 路由到 handleSubprocess

### completeSubprocess 实现

- [x] 6. 在 `classic.go` 新增 `completeSubprocess(ctx, tx, token, operatorID, node, depth)` 方法：创建 end activity → 完成子流程 token → 加载 ticket.WorkflowJSON 解析主流程 → 重新激活父 token → 从 subprocess 节点出边 processNode 到下一目标
- [x] 7. 修改 `classic.go` processNode 的 `case NodeEnd`：在调用 handleEnd 前检查 `token.TokenType == TokenSubprocess`，若是则调用 completeSubprocess

### Progress 及 Task Handler 子流程感知

- [x] 8. 修改 `classic.go` 的 `Progress()` 方法：将直接的 ParseWorkflowDef + BuildMaps 替换为调用 resolveWorkflowContext，使子流程内 activity 的 edge matching 使用正确的子流程 def
- [x] 9. 修改 `tasks.go` 的 `HandleBoundaryTimer`：workflow 加载部分改用 resolveWorkflowContext（使用宿主 token 解析）
- [x] 10. 修改 `tasks.go` 的 `tryHandleBoundaryError`：workflow 加载部分改用 resolveWorkflowContext

### Validator 增强

- [x] 11. 在 `validator.go` 新增 subprocess 节点校验规则：SubProcessDef 非空且可解析、subprocess 有且仅有一条出边、递归校验 SubProcessDef 内部结构（复用 ValidateWorkflow 逻辑）、子流程内不允许嵌套 subprocess 节点

### 编译验证

- [x] 12. 运行 `go build -tags dev ./cmd/server/` 确认无编译错误
