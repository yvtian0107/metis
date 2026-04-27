export {
  createOptimisticUserMessage,
  sessionMessagesToUIMessages,
  useAiChat,
} from "./use-ai-chat"
export { hasUnmatchedPendingUserMessages, mergeTimelineMessages } from "./message-merge"
export type { UseAiChatOptions, UseAiChatReturn } from "./use-ai-chat"
export { AIResponse, MessageTimeline } from "./message-timeline"
export { ThinkingBlock } from "./thinking-block"
export { PlanProgress } from "./plan-progress"
export { SessionSidebar } from "./session-sidebar"
export { ChatComposer } from "./composer"
export type { ChatComposerImage, ChatComposerProps } from "./composer"
export { AgentIdentity, ChatHeader, ChatStatusDot } from "./chat-header"
export { createSurfacePartRenderer } from "./surface-registry"
export { ChatWorkspace } from "./chat-workspace"
export type { ChatWorkspaceProps } from "./chat-workspace"
export { useChatWorkspace } from "./use-chat-workspace"
export type { UseChatWorkspaceOptions } from "./use-chat-workspace"
export type {
  ChatWorkspaceActions,
  ChatWorkspaceComposerConfig,
  ChatWorkspaceIdentity,
  ChatWorkspaceSidebarConfig,
  ChatWorkspaceSurfaceContext,
  ChatWorkspaceSurfaceRenderer,
} from "./types"
