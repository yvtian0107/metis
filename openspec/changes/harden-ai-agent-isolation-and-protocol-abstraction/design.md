## Context

当前 AI 管理模块在数据层已经具备按用户归属建模的基础：`ai_agents.created_by`、`ai_agent_sessions.user_id`、`ai_agent_memories(agent_id,user_id,key)`。但在服务与 handler 层，多个接口仍直接按主键读取资源，再继续执行详情返回、消息写入、流式执行、取消、编辑或删除操作，导致“列表按用户过滤，但详情与操作未按用户校验”的不一致行为。

另一个现状是，Gateway 已经拥有比普通聊天更丰富的内部事件模型，包括 reasoning、plan、step、tool、memory update 等，但当前 Vercel UI stream 编码逻辑直接与 Gateway 的执行编排、持久化后处理耦合在一起。短期内这不会影响现有前端，但会让未来追加 AGUI 等协议适配时需要改动 Gateway 主路径，而不是只新增一个编码器。

本次变更同时涉及 agent/session/memory/gateway 四个能力，属于安全边界与协议职责的交叉性调整，适合先明确设计再进入实现。

## Goals / Non-Goals

**Goals:**

- 让 Agent、Session、Message History 与 Memory 的读取和变更行为统一受当前登录用户约束。
- 在建会话、查看详情、发送消息、编辑消息、删除、取消、继续、上传图片、SSE 流式连接等路径上补齐 ownership / visibility 校验。
- 保持现有前端 API 与 `@ai-sdk/react` transport 不变，只收紧授权语义。
- 将内部 `Event` 到外部流式输出的协议编码职责从 Gateway 中抽离为独立层，保持当前 Vercel UI stream 输出兼容。
- 为越权访问与协议编码兼容补充回归测试。

**Non-Goals:**

- 不新增 AGUI endpoint。
- 不改造前端聊天 transport，也不切换现有流式协议。
- 不调整 AI Agent 的可见性模型（仍然使用 `private | team | public`）。
- 不引入新的数据库表或复杂迁移。

## Decisions

### Decision: 将“资源可访问性”收敛为显式服务层校验能力

在当前代码中，`List` 已经按用户过滤，但 `Get` 和后续操作没有统一的访问校验入口。继续在每个 handler 手写比较会造成重复和遗漏。

本次将把访问校验提升到服务层语义，区分两类资源：

- Agent: 按 visibility + created_by 判断当前用户是否可见/可操作
- Session/History: 按 `session.user_id == currentUserID` 判断是否可访问
- Memory: 按 memory 所属的 `(agent_id, user_id)` 判断是否可访问/删除

优先方案：在现有 service/repo 上增加带用户上下文的查询或校验方法，由 handler/gateway 调用。

不采用的方案：

- 仅依赖 Casbin 路由级授权。原因是 Casbin 只控制“能否访问某个接口”，不能表达记录级 owner/visibility 判定。
- 仅在 handler 层做 `if owner != currentUser`。原因是 Gateway、Service 和后续测试仍可能绕过 handler，安全边界不够集中。

### Decision: Agent 与 Session 违规访问统一返回 not found，而非向非 owner 暴露资源存在性

对于私有 agent、他人 session、他人 history 与 memory 删除操作，本次优先使用“按当前用户查询不到即 not found”的处理方式，避免通过 403 暴露资源存在性。

这与现有 handler 结构更兼容，因为很多路径已经基于 `ErrAgentNotFound` / `ErrSessionNotFound` 分支响应。

不采用的方案：

- 对所有越权访问都返回 403。原因是这会区分“资源存在但你不能看”和“资源不存在”，在 ID 可枚举时会放大信息泄露。

### Decision: 会话相关的所有后续动作都基于“当前用户可访问的 session”加载

`GET/PUT/DELETE /sessions/:sid`、`POST /messages`、`GET /stream`、`POST /cancel`、`POST /continue`、`POST /images` 的共同前提都是 session ownership。实现上不再先裸查 `sid`，再继续动作，而是统一通过带 `userID` 的 session 访问入口获取 session。

这样可以保证：

- history 不会跨用户泄露
- 流式执行不会替别人触发
- 取消/继续/编辑不会跨用户影响别人的运行状态

### Decision: 建会话前复用 Agent 可见性判定，而不是单独新增一套 session-create 规则

`POST /ai/sessions` 的核心风险是“知道 agent id 就能对他人的 private agent 建会话”。因此建会话时必须校验当前用户是否可见该 agent。

本次直接复用 Agent 的 visibility 规则：

- `private`: 仅创建者可见/可建会话
- `team`: 登录用户可见
- `public`: 登录用户可见

不采用的方案：

- 允许任何已认证用户对任意 agent 建会话，只在详情时拦截。原因是这会导致隐式 side effect 与历史脏数据。

### Decision: 协议抽象停留在“编码器接口”层，而不是本次就引入多协议路由

当前需求是为未来 AGUI 做准备，但明确不实现 AGUI endpoint。因此最小正确设计是把 Gateway 中的“事件持久化/状态更新”与“事件编码输出”分离：

- Gateway 继续负责编排、状态流转、消息持久化
- 新的编码层只负责把统一 `Event` 序列编码为某种流式协议
- 默认仍使用现有 Vercel UI stream 编码器

这让后续新增 AGUI 时，只需要：

- 新增一个 AGUI encoder
- 新增一个 endpoint 选择对应 encoder

而不必重新改写 Gateway 主路径。

不采用的方案：

- 现在就新增 AGUI endpoint。原因是超出本次范围，且会带来前后端协议对齐、事件语义映射与回归面扩大。
- 保持现状不抽象。原因是本次既然已经要改 Gateway 授权路径，顺手把编码职责拆开成本最低。

## Risks / Trade-offs

- [Risk] 现有前端或测试可能依赖“知道 sid 就能打开 stream/detail”的宽松行为 -> Mitigation: 保持路由与响应结构不变，只收紧为 not found/unauthorized，并补充覆盖越权场景的测试。
- [Risk] ownership 校验散落在 handler/service/gateway 多处，后续再次漂移 -> Mitigation: 把核心校验入口集中到 service/repo 方法，handler 与 gateway 只调用统一入口。
- [Risk] 协议抽象如果做得过重，会把本次安全修复拖复杂 -> Mitigation: 只抽象 encoder 接口，不改事件模型、不改前端协议、不新增 endpoint。
- [Risk] memory 接口当前 `userID` / `userId` 键名不一致可能在修复时暴露更多问题 -> Mitigation: 统一使用 JWT middleware 已设置的 `userId`，并为 memory handler 增加测试。

## Migration Plan

1. 先补服务层/仓储层的按用户查询与可见性判断方法。
2. 再让 Agent、Session、Memory handler 与 Gateway 统一切换到带用户上下文的访问入口。
3. 抽出协议编码接口，并让现有 Vercel UI stream 编码器接入该接口，确保 SSE 输出不变。
4. 运行 AI 模块相关测试，并补充越权访问与协议兼容测试。

本次不涉及 schema 变更，部署后即生效。

回滚策略：如果出现兼容问题，可回滚到旧版本代码；由于无数据迁移，本次变更可直接代码级回退。

## Open Questions

- Agent detail 对于“存在但不可见”的场景是否长期要区分 403 与 404？本次设计先采用 not found 以减少资源探测面。
- 未来如果引入 AGUI，是否需要同时在 spec 中把统一事件模型从“Vercel Data Stream 导向”改为“协议无关事件模型导向”？本次先不扩大到该层面。
