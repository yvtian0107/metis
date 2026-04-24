import type { ReactNode } from "react"
import type { UIMessage } from "ai"
import type { AgentSession } from "@/lib/api"

export interface ChatMessagePair {
  userMessage: UIMessage
  aiMessages: UIMessage[]
}

export interface ChatWorkspaceIdentity {
  title: ReactNode
  subtitle?: ReactNode
  agentName?: string
  status?: ReactNode
  icon?: ReactNode
  onSwitchAgent?: () => void
  switchLabel?: string
}

export interface ChatWorkspaceComposerConfig {
  value: string
  onChange: (value: string) => void
  onSend: (content?: string) => void
  onStop?: () => void
  placeholder: string
  hint?: ReactNode
  disabled?: boolean
  pending?: boolean
  status?: "ready" | "submitted" | "streaming" | "error"
  allowImages?: boolean
  compact?: boolean
}

export interface ChatWorkspaceActions {
  regenerate: () => void
  retry?: () => void
  continueGeneration?: () => void
  cancel?: () => void
}

export interface ChatWorkspaceSurfaceContext {
  part: UIMessage["parts"][number]
  message: UIMessage
  session?: AgentSession
  actions: ChatWorkspaceActions
}

export interface ChatWorkspaceSurfaceRenderer {
  surfaceType: string
  suppressText?: boolean
  render: (context: ChatWorkspaceSurfaceContext) => ReactNode
}

export interface ChatWorkspaceSidebarConfig {
  content: ReactNode
  collapsed?: boolean
}
