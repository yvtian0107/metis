"use client"

import { Bot, ChevronDown } from "lucide-react"
import type { ReactNode } from "react"
import { Button } from "@/components/ui/button"
import { cn } from "@/lib/utils"
import type { ChatWorkspaceIdentity } from "./types"

export function ChatStatusDot({ className }: { className?: string }) {
  return (
    <span className={cn("relative flex size-2.5", className)}>
      <span className="absolute inline-flex size-full animate-ping rounded-full bg-emerald-400 opacity-45" />
      <span className="relative inline-flex size-2.5 rounded-full bg-emerald-500" />
    </span>
  )
}

export function AgentSwitcher({
  identity,
}: {
  identity: ChatWorkspaceIdentity
}) {
  const content = (
    <div className="flex min-w-0 items-center gap-3">
      <div className="flex size-8 shrink-0 items-center justify-center rounded-full border border-primary/20 bg-primary/8 text-primary">
        {identity.icon ?? <Bot className="size-4" />}
      </div>
      <div className="min-w-0">
        <div className="flex items-center gap-2">
          <div className="truncate text-sm font-semibold">{identity.title}</div>
          {identity.status ?? <ChatStatusDot />}
        </div>
        {identity.subtitle && (
          <div className="mt-0.5 truncate text-xs font-medium text-foreground/70">{identity.subtitle}</div>
        )}
      </div>
    </div>
  )

  if (!identity.onSwitchAgent) return content

  return (
    <Button
      type="button"
      variant="ghost"
      className="h-auto min-w-0 justify-start gap-2 px-0 py-0 hover:bg-transparent"
      onClick={identity.onSwitchAgent}
    >
      {content}
      <ChevronDown className="size-3.5 shrink-0 text-muted-foreground" />
      <span className="sr-only">{identity.switchLabel ?? "切换智能体"}</span>
    </Button>
  )
}

export function ChatHeader({
  identity,
  leading,
  actions,
  className,
}: {
  identity: ChatWorkspaceIdentity
  leading?: ReactNode
  actions?: ReactNode
  className?: string
}) {
  return (
    <div className={cn("flex h-14 shrink-0 items-center justify-between border-b border-border/70 px-5", className)}>
      <div className="flex min-w-0 items-center gap-2">
        {leading}
        <AgentSwitcher identity={identity} />
      </div>
      {actions && <div className="flex shrink-0 items-center gap-1">{actions}</div>}
    </div>
  )
}
