export { useAiChat, sessionMessagesToUIMessages } from "./use-ai-chat"
export type { UseAiChatOptions, UseAiChatReturn } from "./use-ai-chat"
export { QAPair, AIResponse } from "./message-pair"
export type { QAPairProps } from "./message-pair"
export { ThinkingBlock } from "./thinking-block"
export { PlanProgress } from "./plan-progress"
export { SessionSidebar } from "./session-sidebar"
export { ChatComposer } from "./composer"
export type { ChatComposerImage, ChatComposerProps } from "./composer"
export { AgentSwitcher, ChatHeader, ChatStatusDot } from "./chat-header"
export { groupUIMessagesIntoPairs, getStreamingExtras, getMainAssistantMessage } from "./utils"
export { createSurfacePartRenderer } from "./surface-registry"
export { ChatWorkspace } from "./chat-workspace"
export type { ChatWorkspaceProps } from "./chat-workspace"
export { useChatWorkspace } from "./use-chat-workspace"
export type { UseChatWorkspaceOptions } from "./use-chat-workspace"
export type {
  ChatMessagePair,
  ChatWorkspaceActions,
  ChatWorkspaceComposerConfig,
  ChatWorkspaceIdentity,
  ChatWorkspaceSidebarConfig,
  ChatWorkspaceSurfaceContext,
  ChatWorkspaceSurfaceRenderer,
} from "./types"
