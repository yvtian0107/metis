import type { ReactNode } from "react"
import type { AgentSession } from "@/lib/api"
import type { ChatComposerAttachmentTone, ChatComposerMaxWidth, ChatComposerVariant } from "./composer"
import type { UIMessage } from "ai"

export interface ChatWorkspaceIdentity {
  title: ReactNode
  subtitle?: ReactNode
  agentName?: string
  status?: ReactNode
  icon?: ReactNode
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
  variant?: ChatComposerVariant
  maxWidth?: ChatComposerMaxWidth
  minRows?: number
  showToolbarHint?: boolean
  attachmentTone?: ChatComposerAttachmentTone
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
