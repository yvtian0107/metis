## Why

当前 AI 管理模块已经在数据模型上区分了 `created_by`、`user_id` 和 agent-user memory，但 Agent 详情、Session 详情、消息历史、流式执行与 memory 删除等路径仍存在仅按主键读取的实现，无法保证用户隔离。与此同时，聊天流式输出已经积累了 plan、reasoning、tool、memory update 等内部事件，继续把 Vercel UI stream 编码逻辑直接耦合在 Gateway 路径上，会增加后续协议演进和客户端兼容的成本。

## What Changes

- 为 AI Agent 运行时补齐基于当前登录用户的 ownership / visibility 校验，覆盖 Agent、Session、Message History 与 Memory 相关接口。
- 收紧会话与历史记录访问边界，确保只能访问、编辑、删除、流式执行当前用户拥有的 session 及其消息。
- 修正 memory API 的用户上下文读取与删除校验问题，确保 memory 的读写删除都遵循 agent-user 隔离模型。
- 将当前 SSE UI stream 编码从 Gateway 编排逻辑中抽离为独立的协议编码层，为未来引入 AGUI 等协议适配保留扩展点，但本次不新增 AGUI endpoint、不改变现有前端接口。
- 为以上行为补充服务层与 handler/gateway 层测试，覆盖越权访问、不可见 agent、跨用户 session/history/memory 访问等场景。

## Capabilities

### New Capabilities

- `ai-stream-protocol-abstraction`: 抽象 AI 事件到流式协议编码层，允许保持现有 Vercel UI stream 行为并为后续协议扩展预留接口

### Modified Capabilities

- `ai-agent`: 收紧 Agent 详情、更新、删除与建会话前的可见性/归属校验
- `ai-agent-session`: 收紧 session 详情、消息发送、编辑、删除、取消、继续与流式连接的所有权校验
- `ai-agent-memory`: 修正 memory 用户上下文读取并补齐删除操作的所有权校验
- `ai-agent-gateway`: 将内部事件到 SSE 输出的编码职责抽离出 Gateway，同时保持现有流式协议兼容

## Impact

- 后端代码主要影响 `internal/app/ai/agent_*`、`session_*`、`memory_*`、`gateway.go`、`data_stream.go` 及其测试。
- API 路由不新增也不删除，但多个接口的授权语义会变严格；原先可通过直接猜测 ID 访问他人资源的行为将被拒绝。
- 前端现有 `/api/v1/ai/sessions/:sid/stream` 与 `@ai-sdk/react` transport 保持不变。
- 为后续新增 AGUI 等协议 endpoint 提供更清晰的内部扩展边界，但该 endpoint 不在本次变更范围内。
