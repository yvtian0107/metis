## Why

当前 AI Agent 聊天使用自定义 SSE JSON 协议，导致流式输出不流畅、结束时有闪烁，且前端 `ReactMarkdown` 每帧全量重解析造成卡顿。为彻底解决体验问题并对接社区标准，需将 SSE 协议迁移至 Vercel AI SDK Data Stream，同时引入 `ai-elements` 作为统一的 AI 文本渲染组件。

## What Changes

- **BREAKING**: 后端 `AgentGateway` 及 executor 的 SSE 输出格式从自定义 JSON 改为 Vercel AI SDK Data Stream（`data: 0:{...}\n\n` 格式）。
- **BREAKING**: 前端 `useChatStream` 自定义 hook 替换为 `@ai-sdk/react` 的 `useChat`，`DefaultChatTransport` 对接现有 `/api/v1/ai/sessions/:sid/stream` 端点。
- 前端聊天消息渲染从 `ReactMarkdown` + `react-syntax-highlighter` 迁移到 `ai-elements` 的 `MessageResponse` / `Message` 组件。
- 移除冗余的 `streamBufferRef` / `rafIdRef` 节流逻辑，`useChat` 内部已处理反压与缓冲。
- 保留现有的 thinking、plan progress、tool call 自定义 UI，通过 `UIMessage.parts` 或组件插槽兼容渲染。
- 后端 SSE handler 增加 `http.Flusher` 确保每包即时下发。

## Capabilities

### New Capabilities
- `ai-data-stream-protocol`: 定义 Data Stream 编码规范，包含 event type 映射、 flushing 策略及错误处理。

### Modified Capabilities
- `ai-agent-gateway`: SSE 输出格式改为 Vercel AI SDK Data Stream；`session_handler.go` 的 `Stream` 接口需适配编码器并 flush。
- `ai-agent-session`: 前端会话流式读取方式由自定义 `EventSource` 改为 `useChat` + `DefaultChatTransport`；`[sid].tsx` 状态管理重构。
- `ai-agent-chat-ui`: 消息渲染组件从 `ReactMarkdown` 迁移到 `ai-elements`；UI 结构保持现有 QA pair 布局。
- `ai-llm-client`: executor 输出的事件结构需能被 gateway 翻译为 Data Stream 的 `text-delta`、`tool-call`、`tool-result`、`finish` 等类型。

## Impact

- **后端**: `internal/app/ai/gateway.go`, `internal/app/ai/session_handler.go`, `internal/app/ai/executor_react.go`, `internal/app/ai/executor_plan.go`
- **前端**: `web/src/apps/ai/pages/chat/[sid].tsx`, `web/src/apps/ai/pages/chat/components/message-item.tsx`, `web/src/apps/ai/pages/chat/hooks/use-chat-stream.ts`
- **依赖**: 新增 `@ai-sdk/react`, `ai-elements` 到 `web/package.json`
- **兼容**: 旧版自定义 SSE 消费端不再支持（本次为破坏性变更，需确保无其他调用方）
