## 1. Token 模型 + 常量

- [ ] 1.1 在 `engine/engine.go` 中新增 Token 状态常量（TokenActive/TokenWaiting/TokenCompleted/TokenCancelled/TokenSuspended）和 Token 类型常量（TokenMain/TokenParallel/TokenSubprocess/TokenMultiInstance/TokenBoundary），附注释标明各常量属于哪个 change
- [ ] 1.2 在 `engine/engine.go` 中将 `NodeGateway = "gateway"` 替换为 `NodeExclusive = "exclusive"`，新增 `NodeParallel = "parallel"`、`NodeInclusive = "inclusive"` 常量；新增高级节点常量 `NodeScript/NodeSubprocess/NodeTimer/NodeSignal/NodeBTimer/NodeBError`；更新 `ValidNodeTypes` map；更新 `IsAutoNode()` 和 `IsHumanNode()` 函数
- [ ] 1.3 在 `engine/classic.go` 中新增 `executionTokenModel` 轻量模型结构体（TableName `itsm_execution_tokens`），字段：ID, TicketID, ParentTokenID, NodeID, Status, TokenType, ScopeID, CreatedAt, UpdatedAt
- [ ] 1.4 在 `engine/classic.go` 的 `activityModel` 中新增 `TokenID *uint` 字段（gorm tag: column:token_id;index）
- [ ] 1.5 创建 `internal/app/itsm/model_token.go`：正式的 ExecutionToken 模型（嵌入 BaseModel），包含 ToResponse() 方法
- [ ] 1.6 在 ITSM `app.go` 的 Models() 中注册 ExecutionToken，确保 AutoMigrate

## 2. Engine 重构 — Start

- [ ] 2.1 修改 `ClassicEngine.Start()`：在解析 workflow 和找到 start 节点后，创建 root executionTokenModel（token_type=main, status=active, scope_id=root），将 processNode 调用改为传 token 而非 ticketID
- [ ] 2.2 修改 `processNode` 签名：将 `ticketID uint` 参数替换为 `token *executionTokenModel`，内部通过 `token.TicketID` 获取工单 ID；在进入新节点时更新 `token.NodeID`

## 3. Engine 重构 — Node Handlers

- [ ] 3.1 修改 `handleEnd`：签名改为接收 token，创建 Activity 时设置 TokenID，将 token.Status 更新为 completed
- [ ] 3.2 修改 `handleForm`：签名改为接收 token，创建 Activity 时设置 TokenID
- [ ] 3.3 修改 `handleApprove`：签名改为接收 token，创建 Activity 时设置 TokenID
- [ ] 3.4 修改 `handleProcess`：签名改为接收 token，创建 Activity 时设置 TokenID
- [ ] 3.5 修改 `handleAction`：签名改为接收 token，创建 Activity 时设置 TokenID
- [ ] 3.6 将 `handleGateway` 重命名为 `handleExclusive`，签名改为接收 token，processNode 调用传 token
- [ ] 3.7 修改 `handleNotify`：签名改为接收 token，processNode 调用传 token
- [ ] 3.8 修改 `handleWait`：签名改为接收 token，创建 Activity 时设置 TokenID
- [ ] 3.9 修改 `processNode` 中的 switch-case：`NodeGateway` → `NodeExclusive`，调用 `handleExclusive`；对 `NodeParallel`/`NodeInclusive` 返回"尚未实现"错误
- [ ] 3.10 修改 `writeFormBindings` 调用处：Start() 和 Progress() 中将硬编码 `"root"` scope 改为 `token.ScopeID`

## 4. Engine 重构 — Progress & Cancel

- [ ] 4.1 修改 `ClassicEngine.Progress()`：在加载 activity 后，通过 activity.TokenID 加载 executionTokenModel；校验 token 为 active 状态；传 token 给 processNode
- [ ] 4.2 修改 `ClassicEngine.Cancel()`：新增批量更新 executionTokenModel（status IN active,waiting → cancelled）；保留原有的 Activity 和 Assignment 取消逻辑

## 5. Validator 增强

- [ ] 5.1 修改 `ValidationError` 结构体：新增 `Level string` 字段（"error" 或 "warning"，默认 "error"）
- [ ] 5.2 修改 validator.go 中 gateway 校验逻辑：将 `NodeGateway` 引用改为 `NodeExclusive`，更新中文提示"网关节点"→"排他网关节点"
- [ ] 5.3 新增未实现节点类型 warning 逻辑：定义 `unimplementedNodeTypes` set（parallel/inclusive/script/subprocess/timer/signal/b_timer/b_error），遍历节点时如果 type 在此 set 中则追加 warning 级别 ValidationError
- [ ] 5.4 为 exclusive 节点新增与 gateway 相同的校验规则（至少两条出边、非默认出边须有条件）

## 6. condition.go 更新

- [ ] 6.1 修改 `condition.go` 中 `buildEvalContext` 或其调用处：确保引用 `NodeExclusive` 而非 `NodeGateway`（如有直接引用）
- [ ] 6.2 确认 `handleExclusive` 中调用 `buildEvalContext` 的逻辑无需其他改动

## 7. 前端节点类型同步

- [ ] 7.1 在前端 workflow editor 中将 `gateway` 节点类型改为 `exclusive`：搜索所有 `"gateway"` 引用并替换为 `"exclusive"`（包括节点面板、属性面板、节点图标/颜色映射）
- [ ] 7.2 在前端 workflow runtime viewer 中将 `gateway` 引用改为 `exclusive`
- [ ] 7.3 更新前端 i18n 翻译键：将 `gateway` 相关翻译改为 `exclusive`，新增 `parallel`/`inclusive` 翻译（如有在节点面板中展示）
