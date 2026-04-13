## Context

当前 AI Agent 聊天系统使用完全自定义的 SSE JSON 协议：后端 Go 代码通过 `c.SSEvent("message", jsonData)` 发送离散事件，前端通过原生 `EventSource` 接收并手动管理 `content_delta` 拼接。该协议存在三个核心问题：

1. **无 Flush**：`gin.Context.SSEvent` 不会自动 flush，导致事件在缓冲区积压，流式体验卡顿。
2. **状态切换闪烁**：前端在收到 `done` 事件时立即清空 `pendingMessages` 再等待 `invalidateQueries` refetch，造成 DOM 卸载-重挂载的闪烁。
3. **渲染性能差**：`ReactMarkdown` + `remarkGfm` + `SyntaxHighlighter` 每帧全量重解析重建 DOM，无法平滑承载高频 text delta。

Vercel AI SDK 的 Data Stream 协议已成为社区事实标准（`ai` 包、`@ai-sdk/react`、`ai-elements` 均基于此）。迁移后，前端可直接使用官方 `useChat` 和 `MessageResponse`，后端只需在 gateway 层加标准化编码器，executor 内部事件格式保持最小改动。

## Goals / Non-Goals

**Goals:**
- 将 `GET /api/v1/ai/sessions/:sid/stream` 的 SSE payload 迁移为 Vercel AI SDK Data Stream 格式。
- 前端 `[sid].tsx` 及 `message-item.tsx` 迁移到 `@ai-sdk/react` + `ai-elements`。
- 解决 flush 导致的卡顿和状态切换导致的闪烁。
- 保留现有 thinking、plan progress、tool call UI 的展示能力（通过 `UIMessage.parts` 或组件插槽）。

**Non-Goals:**
- 不重写 executor 的核心 ReAct / Plan-and-Execute 逻辑，只改事件封装层。
- 不引入 AI Gateway（Vercel 产品）或 OIDC，只是借用其客户端 Data Stream 协议。
- 不修改数据库 message schema。
- 不支持 SSE 断线重连 replay（现有 spec 中有此要求，但当前实现也未支持，本次不扩展）。

## Decisions

### Decision 1: 后端采用 Data Stream 编码器，而不是改造 LLM client 协议
**Rationale**: `internal/llm/` 目前输出的是 `StreamEvent`（`content_delta`, `tool_call`, `done` 等），这是合理的 Go 层抽象。如果把它改成 Data Stream，会让 LLM client 过度依赖前端协议。更好的做法是在 `gateway.go` 或 `session_handler.go` 里加一层 `DataStreamEncoder`，将 `Event` 翻译为 Data Stream line。

**Alternative considered**: 直接改 `llm.ChatStream` 输出 Data Stream。Rejected：会污染 provider 抽象层，且 sidecar 远程执行的事件也需要统一翻译。

### Decision 2: Gateway 的 goroutine wrapper 里做翻译，而不是 handler 里
**Rationale**: `gateway.go` 已经有一个包装 goroutine（`outCh`）在读取 `eventCh` 并做 DB 持久化。在这里同时做 Data Stream 编码是最自然的：一个事件进来，同时做两件事：1) 写 DB；2) 转发给 SSE。`session_handler.go` 只做 `io.Copy` 式的流式下发。

**Alternative considered**: 在 `session_handler.go` 里翻译。Rejected：handler 看不到执行上下文，且 `c.Stream` 的回调不利于维护编码状态。

### Decision 3: 前端使用 `useChat` + `DefaultChatTransport`，而不是继续用 `EventSource`
**Rationale**: `useChat` 已经内置了反压、缓冲、 optimistic UI、自动滚动等能力，且与 `ai-elements` 的 `Message` 组件天然对接。`DefaultChatTransport` 允许我们指定自定义 API 路径，适配现有 `/api/v1/ai/sessions/:sid/stream`。

**Alternative considered**: 保留 `useChatStream` 但只换 `ai-elements` 渲染。Rejected：`useChatStream` 的状态机（pendingMessages + streamState 双轨）本身就是闪烁的根因，而且无法利用 `ai-elements` 的 `message.parts` 解析能力。

### Decision 4: `ai-elements` 以 `MessageResponse` 为主，保留现有 QAPair 布局
**Rationale**: 我们现有的布局是左右分栏 + 用户消息右对齐气泡，不是纯 `Message` 列表。`MessageResponse` 是 ai-elements 里专门用于"渲染一段 AI 生成的 markdown"的组件，不强制要求特定的外层列表结构。我们可以在 `QAPair` 里继续使用现有的 `UserQuery` 和 `ToolCall` 组件，只把 `AIResponse` 里的 `ReactMarkdown` 替换为 `MessageResponse`。

**Alternative considered**: 用 `Message` 组件完全替换 `QAPair`。Rejected：`Message` 组件的设计假设是完整的聊天消息列表，而我们的 thinking、plan、tool UI 是穿插在 QA 对之间的，强行套用会丢失布局控制。

### Decision 5: thinking / plan / tool 自定义 UI 不通过 Data Stream 标准 part 渲染
**Rationale**: Data Stream 的 `UIMessage.parts` 只定义了 `text`, `tool-invocation`, `reasoning` 等标准类型，但没有 `plan` 这种自定义概念。为了保留现有产品体验，我们采用"混合渲染"：`MessageResponse` 负责标准 `text` 和 `tool-invocation` parts；thinking / plan 继续由我们自己的 `ThinkingBlock` / `PlanProgress` 组件渲染，放在 `MessageResponse` 的上方插槽中。

**Alternative considered**: 把 plan 封装成 annotation 或自定义 part。Rejected：`ai-elements` 对非标准 part 的支持不稳定，且过度设计。

## Risks / Trade-offs

| Risk | Mitigation |
|------|-----------|
| `ai-elements` 和 React 19 / Vite 8 存在兼容性问题 | 先安装并跑通一个最小示例（单独渲染一段 markdown），再改主页面 |
| `useChat` 的 `DefaultChatTransport` 无法直接消费我们现有的 endpoint（没有 `/api/chat` 标准路由） | 自定义 `ChatTransport` 实现，内部仍然用 `EventSource` 读取 Data Stream 格式 |
| 后端翻译层引入新 bug（如 tool_call 参数 JSON 转义错误） | 增加单元测试覆盖 DataStreamEncoder 的核心转换路径 |
| 前端体积增加（新增 `@ai-sdk/react` + `ai-elements`） | 评估 bundle size，若超过 150KB gzipped 则考虑按需引入子包 |
| 破坏现有远程 sidecar 的执行链路 | sidecar 仍然发送自定义 NDJSON `Event` 给 `POST /api/v1/ai/sessions/:sid/events`，gateway 的 decoder 不变；encoder 只影响最终流给浏览器的 SSE |

## Migration Plan

1. **Backend**:
   - 新增 `internal/app/ai/data_stream.go`：`DataStreamEncoder` + `WriteTo(w io.Writer)`。
   - 修改 `gateway.go`：包装 goroutine 里，`outCh <- evt` 之前调用 `encoder.Encode(evt)` 写入一个 `io.PipeWriter`，另一端交给 SSE handler。
   - 修改 `session_handler.go`：`c.Stream` 回调从 `c.SSEvent` 改为直接 `io.Copy` Data Stream reader；每次 write 后 `flush`。
   - 调整 `executor_react.go`：明确 `tool_call` 事件的输出时机（在 streaming 完成后而不是中间 fragmented）。

2. **Frontend**:
   - 安装依赖：`bun add ai @ai-sdk/react ai-elements`。
   - 新建 `web/src/apps/ai/pages/chat/hooks/use-ai-chat.ts`：包装 `useChat`，自定义 `transport` 读取 `/api/v1/ai/sessions/:sid/stream`。
   - 重构 `[sid].tsx`：移除 `pendingMessages`、`streamBufferRef`、`rafIdRef`、`groupMessagesIntoPairs`；用 `useChat` 提供的 `messages` + `status` 驱动 UI。
   - 重构 `message-item.tsx`：`AIResponse` 内部把 `content` 转成 `UIMessagePart[]` 传给 `MessageResponse`；保留 `ToolCall` 和 thinking/plan 插槽。
   - 删除 `use-chat-stream.ts`（或保留重命名为 legacy）。

3. **Rollback**:
   - 所有改动集中在 `internal/app/ai/` 和 `web/src/apps/ai/pages/chat/`。
   - 若发现问题，回滚到上一 commit 即可；无数据库 migration。

## Open Questions

1. `ai-elements` 的 `MessageResponse` 是否支持通过 props 传入已渲染好的 tool-call UI（我们需要显示 duration 和可折叠的参数）？
2. 我们的 Data Stream 里 `thinking` 是否映射到 `reasoning` part 还是保持为组件外插槽？
3. `useChat` 在接收到 `tool_result` 后是否会自动把 `tool-invocation` part 从 `call` 状态更新为 `result`？如果不自动，我们需要在前端做状态同步。
