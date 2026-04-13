## 1. Backend Data Stream Foundation

- [x] 1.1 Create `internal/app/ai/data_stream.go` with `DataStreamEncoder` supporting text-delta, reasoning, tool-invocation, error, and finish-message chunks
- [x] 1.2 Add unit tests for `DataStreamEncoder` covering content_delta, tool_call, tool_result, done, and error event translations
- [x] 1.3 Modify `gateway.go` wrapper goroutine to write encoded Data Stream lines into an `io.PipeWriter` alongside existing DB persistence
- [x] 1.4 Modify `session_handler.go` `Stream` method to copy from Data Stream reader with `http.Flusher.Flush()` after each write
- [x] 1.5 Ensure `executor_react.go` emits complete `tool_call` events (not fragmented) so encoder can produce valid tool-invocation chunks

## 2. Frontend Dependency & Hook Setup

- [x] 2.1 Install `ai`, `@ai-sdk/react`, and `ai-elements` into `web/package.json`
- [x] 2.2 Create `web/src/apps/ai/pages/chat/hooks/use-ai-chat.ts` wrapping `useChat` with a custom `ChatTransport` that reads Data Stream from existing SSE endpoint
- [x] 2.3 Verify `use-ai-chat.ts` handles connection, cancellation, and error events correctly

## 3. Frontend Chat Page Refactor

- [x] 3.1 Refactor `[sid].tsx` to replace `useChatStream` + `streamBufferRef` + `pendingMessages` with `use-ai-chat.ts`
- [x] 3.2 Remove `groupMessagesIntoPairs` and `qaPairs` memo; derive UI from `useChat.messages` while preserving existing layout structure
- [x] 3.3 Ensure auto-scroll, cancel button, and retry flow work with new `useChat` state (`status`, `stop`, `reload`)

## 4. Frontend Message Rendering Migration

- [x] 4.1 Replace `ReactMarkdown` + `react-syntax-highlighter` in `message-item.tsx` with `ai-elements` `MessageResponse`
- [x] 4.2 Preserve existing `ToolCall` / `ToolResult` collapsible blocks and position them consistently with `MessageResponse`
- [x] 4.3 Preserve `ThinkingBlock` and `PlanProgress` as custom UI above `MessageResponse` for the streaming pair
- [x] 4.4 Ensure user query images, edit flow, and action buttons (copy/regenerate/thumbs) remain functional

## 5. Integration & Cleanup

- [x] 5.1 Run `make dev` and `make web-dev` to verify end-to-end flow: send message → stream text → tool call → done
- [x] 5.2 Verify no console errors and no DOM flashes when stream transitions from streaming to completed
- [x] 5.3 Delete `use-chat-stream.ts` or move it to a legacy folder if rollback insurance is desired
- [x] 5.4 Run `cd web && bun run lint` to ensure React Compiler and ESLint pass
