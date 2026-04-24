"use client"

import { ArrowDown, AlertTriangle, Loader2, RotateCw } from "lucide-react"
import type { ReactNode, RefObject } from "react"
import { Button } from "@/components/ui/button"
import { cn } from "@/lib/utils"
import { ChatComposer, type ChatComposerProps } from "./composer"
import { ChatHeader } from "./chat-header"
import { QAPair } from "./message-pair"
import type { ChatMessagePair, ChatWorkspaceActions, ChatWorkspaceIdentity, ChatWorkspaceSurfaceRenderer } from "./types"
import { createSurfacePartRenderer } from "./surface-registry"
import type { AgentSession } from "@/lib/api"

export interface ChatWorkspaceProps {
  identity: ChatWorkspaceIdentity
  sidebar?: ReactNode
  leading?: ReactNode
  actions?: ReactNode
  loading?: boolean
  emptyState?: ReactNode
  pairs: ChatMessagePair[]
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
  renderStreamingExtras?: (pair: ChatMessagePair, isStreaming: boolean) => ReactNode
  getDoneMetrics?: (pair: ChatMessagePair, index: number) => { durationMs?: number; inputTokens?: number; outputTokens?: number } | undefined
  errorActions?: ReactNode
  className?: string
}

export function ChatWorkspace({
  identity,
  sidebar,
  leading,
  actions,
  loading,
  emptyState,
  pairs,
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
  renderStreamingExtras,
  getDoneMetrics,
  errorActions,
  className,
}: ChatWorkspaceProps) {
  const lastPairIndex = pairs.length - 1
  const surfaceRenderer = createSurfacePartRenderer({ renderers: surfaces, session, actions: workspaceActions })

  return (
    <div className={cn("flex h-full min-h-0 overflow-hidden bg-background", className)}>
      {sidebar}
      <main className="flex h-full min-h-0 min-w-0 flex-1 flex-col overflow-hidden bg-background">
        <ChatHeader identity={identity} leading={leading} actions={actions} />

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
          ) : pairs.length === 0 ? (
            emptyState
          ) : (
            <div className="mx-auto max-w-3xl px-4 pb-4">
              {pairs.map((pair, index) => {
                const isLastPair = index === lastPairIndex
                const isStreamingThisPair = Boolean(isLastPair && isBusy)
                return (
                  <QAPair
                    key={pair.userMessage.id}
                    userMessage={pair.userMessage}
                    aiMessages={pair.aiMessages}
                    agentName={agentName}
                    isStreaming={isStreamingThisPair}
                    onRegenerate={isLastPair ? workspaceActions.regenerate : undefined}
                    onEditMessage={onEditMessage}
                    renderDataPart={(part, message) => surfaceRenderer.render(part, message)}
                    suppressTextWhenDataPart={pair.aiMessages.some((message) =>
                      message.parts?.some((part) => surfaceRenderer.shouldSuppressText(part)),
                    )}
                    doneMetrics={
                      isLastPair && status === "ready"
                        ? getDoneMetrics?.(pair, index)
                        : undefined
                    }
                    streamingExtras={isStreamingThisPair ? renderStreamingExtras?.(pair, true) : undefined}
                  />
                )
              })}

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

        <div className="shrink-0 bg-background px-4 pb-3 pt-1">
          <div className="mx-auto max-w-3xl">
            <ChatComposer {...composer} />
          </div>
        </div>
      </main>
    </div>
  )
}
