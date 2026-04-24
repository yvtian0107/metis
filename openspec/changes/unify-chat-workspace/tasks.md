## 1. Shared Chat Workspace Foundation

- [x] 1.1 Create the shared `web/src/components/chat-workspace/` module structure with exported types for workspace identity, composer options, sidebar options, surface renderer context, and workspace actions.
- [x] 1.2 Move `groupUIMessagesIntoPairs` into the shared chat workspace layer and update callers to import the shared helper.
- [x] 1.3 Move the message pair rendering primitives out of `web/src/apps/ai/pages/chat/components/message-item.tsx` into the shared chat workspace layer without importing ITSM code.
- [x] 1.4 Add a surface registry abstraction that can render text, tool activity, reasoning, plan progress, generic data surfaces, and business-registered surfaces.
- [x] 1.5 Add a shared `useChatWorkspace` hook that wraps session loading, `useAiChat`, send, cancel, continue, retry, regenerate, usage metrics, busy state, and query invalidation callbacks.

## 2. Unified Composer and Attachments

- [x] 2.1 Implement a shared composer with multiline auto-resize, IME-safe Enter send, Shift+Enter newline, disabled/read-only states, send button, and stop state integration.
- [x] 2.2 Add shared image paste/select preview, removal, upload, and send integration using existing `sessionApi.uploadMessageImage` and `sessionApi.sendMessage`.
- [x] 2.3 Ensure failed image upload keeps the current text and image previews and shows a user-visible error.
- [x] 2.4 Replace manual send button SVG usage with the shared icon/button implementation.

## 3. Shared Header, Agent Switcher, and Sidebar

- [x] 3.1 Implement a shared `ChatHeader` with consistent density, title, subtitle, status, action area, and responsive behavior.
- [x] 3.2 Implement a shared `AgentSwitcher` visual component whose selection behavior is provided by the calling business surface.
- [x] 3.3 Implement a reusable `ChatSessionSidebar` supporting loading, empty state, selected state, new session, date grouping, and optional actions such as rename/delete/pin/collapse.
- [x] 3.4 Wire AI Management-specific memory, delete, rename, pin, and collapse actions through the shared header/sidebar configuration.
- [x] 3.5 Wire ITSM-specific service intake agent identity, smart staffing unavailable state, and staffing configuration navigation through the shared header/switcher configuration.

## 4. Migrate AI Management Chat

- [x] 4.1 Refactor `/ai/chat/:sid` to render through `ChatWorkspace` while preserving current session loading, streaming, image input, user message editing, regenerate, continue generation, cancel, delete session, and memory panel behavior.
- [x] 4.2 Refactor `/ai/chat` empty/autostart flow to use the shared workspace shell or shared empty-state primitives where applicable.
- [x] 4.3 Remove AI chat page-level duplicate scroll, composer, stop, error, and message grouping logic after migration.
- [ ] 4.4 Verify AI Management chat still handles text streaming, tool activity, reasoning, plan progress, image messages, message edit/regenerate, and memory panel.

## 5. Migrate ITSM Service Desk

- [x] 5.1 Refactor `web/src/apps/itsm/pages/service-desk/index.tsx` to render active sessions through `ChatWorkspace`.
- [x] 5.2 Enable shared image input for ITSM Service Desk messages using the shared composer.
- [x] 5.3 Register an ITSM `itsm.draft_form` surface renderer that handles loading, editable confirmation, inline submit errors, submitted state, ticket code display, and workspace invalidation.
- [x] 5.4 Replace ITSM local `ServiceDeskComposer`, message grouping, scroll handling, stop button, and header implementation with shared workspace configuration.
- [x] 5.5 Preserve ITSM welcome stage, service desk prompt suggestions, not-on-duty state, service intake agent label, and smart staffing navigation through workspace slots/configuration.
- [ ] 5.6 Verify ITSM Service Desk supports text messages, image messages, streaming responses, draft confirmation surface, draft submission, and unavailable service intake agent state.

## 6. Cleanup and Design Baseline

- [x] 6.1 Delete obsolete duplicated chat helpers and page-local composer code after both surfaces use the shared implementation.
- [x] 6.2 Update imports so shared chat workspace code has no dependency on `apps/itsm` or route-specific AI chat pages.
- [x] 6.3 Update `DESIGN.md` with the Chat Workspace design baseline: one conversation system, shared composer/header/sidebar/surface registry, and ITSM-specific service desk surface rules.
- [x] 6.4 Review Chinese copy for ITSM service desk and AI chat to keep product nouns consistent, especially service intake agent and smart staffing wording.

## 7. Verification

- [ ] 7.1 Run `cd web && bun run lint`.
- [x] 7.2 Run `cd web && bun run build`.
- [ ] 7.3 Manually smoke-test AI Management chat with a normal text prompt, an image prompt, a tool/reasoning response, and session sidebar actions.
- [ ] 7.4 Manually smoke-test ITSM Service Desk with a text request, an image request, draft form confirmation, ticket submission, and no-intake-agent state.
