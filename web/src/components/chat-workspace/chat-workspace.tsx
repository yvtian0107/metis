"use client"

import { ArrowDown, AlertTriangle, Loader2, RotateCw } from "lucide-react"
import type { UIMessage } from "ai"
import type { ReactNode, RefObject } from "react"
import { Button } from "@/components/ui/button"
import { cn } from "@/lib/utils"
import { ChatComposer, type ChatComposerProps } from "./composer"
import { ChatHeader } from "./chat-header"
import { MessageTimeline } from "./message-timeline"
import type { ChatWorkspaceActions, ChatWorkspaceIdentity, ChatWorkspaceSurfaceRenderer } from "./types"
import { createSurfacePartRenderer } from "./surface-registry"
import type { AgentSession } from "@/lib/api"

export interface ChatWorkspaceProps {
  identity: ChatWorkspaceIdentity
  sidebar?: ReactNode
  leading?: ReactNode
  actions?: ReactNode
  loading?: boolean
  emptyState?: ReactNode
  messages: UIMessage[]
  agentName?: string
  isBusy?: boolean
  status?: string
  error?: Error | null
  session?: AgentSession
  surfaces?: ChatWorkspaceSurfaceRenderer[]
  workspaceActions: ChatWorkspaceActions
  onEditMessage?: (messageId: number, content: string) => void
  composer: ChatComposerProps
  messagesEndRef?: RefObject<HTMLDivElement | null>
  scrollRef?: RefObject<HTMLDivElement | null>
  onScroll?: () => void
  showJumpToBottom?: boolean
  onJumpToBottom?: () => void
  getDoneMetrics?: () => { durationMs?: number; inputTokens?: number; outputTokens?: number } | undefined
  errorActions?: ReactNode
  density?: "comfortable" | "workbench"
  messageWidth?: "standard" | "wide"
  composerPlacement?: "floating" | "docked"
  emptyStateTone?: "ai" | "service-desk"
  className?: string
  headerClassName?: string
}

const messageWidthClass = {
  standard: "max-w-3xl",
  wide: "max-w-4xl",
} as const

const composerPlacementClass = {
  docked: "shrink-0 border-t border-border/55 bg-background/98 px-4 pb-3 pt-2",
  floating: "shrink-0 bg-gradient-to-t from-background via-background/96 to-background/70 px-4 pb-5 pt-2",
} as const

const workspaceDensityClass = {
  comfortable: "bg-background",
  workbench: "bg-[linear-gradient(180deg,hsl(var(--background)),hsl(var(--muted)/0.12))]",
} as const

const emptyStateClass = {
  ai: "h-full",
  "service-desk": "h-full bg-[radial-gradient(circle_at_50%_35%,hsl(var(--primary)/0.05),transparent_34%)]",
} as const

export function ChatWorkspace({
  identity,
  sidebar,
  leading,
  actions,
  loading,
  emptyState,
  messages,
  agentName,
  isBusy,
  status,
  error,
  session,
  surfaces,
  workspaceActions,
  onEditMessage,
  composer,
  messagesEndRef,
  scrollRef,
  onScroll,
  showJumpToBottom,
  onJumpToBottom,
  getDoneMetrics,
  errorActions,
  density = "comfortable",
  messageWidth = "standard",
  composerPlacement = "docked",
  emptyStateTone = "ai",
  className,
  headerClassName,
}: ChatWorkspaceProps) {
  const surfaceRenderer = createSurfacePartRenderer({ renderers: surfaces, session, actions: workspaceActions })
  const showEmptyState = messages.length === 0 && !isBusy

  return (
    <div className={cn("flex h-full min-h-0 overflow-hidden", workspaceDensityClass[density], className)}>
      {sidebar}
      <main className="flex h-full min-h-0 min-w-0 flex-1 flex-col overflow-hidden bg-background/82">
        <ChatHeader identity={identity} leading={leading} actions={actions} className={headerClassName} />

        <div
          ref={scrollRef}
          className="relative min-h-0 flex-1 overflow-y-auto overflow-x-hidden"
          onScroll={onScroll}
        >
          {loading ? (
            <div className="flex h-full items-center justify-center text-sm text-muted-foreground">
              <Loader2 className="mr-2 size-4 animate-spin" />
              载入会话
            </div>
          ) : showEmptyState ? (
            <div className={emptyStateClass[emptyStateTone]}>{emptyState}</div>
          ) : (
            <div className={cn("mx-auto px-4 pb-4", messageWidthClass[messageWidth])}>
              <MessageTimeline
                messages={messages}
                agentName={agentName}
                isBusy={isBusy}
                status={status}
                onRegenerate={workspaceActions.regenerate}
                onEditMessage={onEditMessage}
                renderDataPart={(part, message) => surfaceRenderer.render(part, message)}
                shouldSuppressDataPart={(part) => surfaceRenderer.shouldSuppressText(part)}
                doneMetrics={status === "ready" ? getDoneMetrics?.() : undefined}
              />

              {error && !isBusy && (
                <div className="py-6">
                  <div className="flex items-center gap-3 rounded-lg border-l-4 border-destructive bg-destructive/5 p-3">
                    <AlertTriangle className="h-4 w-4 shrink-0 text-destructive" />
                    <div className="min-w-0 flex-1">
                      <div className="text-sm font-medium text-destructive">生成失败</div>
                      <div className="mt-0.5 text-xs text-muted-foreground">{error.message}</div>
                    </div>
                    {errorActions ?? (
                      <>
                        {workspaceActions.continueGeneration && (
                          <Button variant="outline" size="sm" onClick={workspaceActions.continueGeneration}>
                            <RotateCw className="mr-1 h-3.5 w-3.5" />
                            继续生成
                          </Button>
                        )}
                        {workspaceActions.retry && (
                          <Button variant="ghost" size="sm" onClick={workspaceActions.retry}>
                            <RotateCw className="mr-1 h-3.5 w-3.5" />
                            重试
                          </Button>
                        )}
                      </>
                    )}
                  </div>
                </div>
              )}
              <div ref={messagesEndRef} />
            </div>
          )}

          {showJumpToBottom && (
            <Button
              type="button"
              variant="outline"
              size="sm"
              className="sticky bottom-3 left-1/2 z-10 h-8 -translate-x-1/2 rounded-full bg-background/95 px-3 shadow-sm"
              onClick={onJumpToBottom}
            >
              <ArrowDown className="mr-1.5 h-3.5 w-3.5" />
              跳到底部
            </Button>
          )}
        </div>

        <div className={composerPlacementClass[composerPlacement]}>
          <div className={cn("mx-auto w-full", messageWidthClass[messageWidth])}>
            <ChatComposer {...composer} />
          </div>
        </div>
      </main>
    </div>
  )
}
