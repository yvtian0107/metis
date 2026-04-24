"use client";

import { useState, useCallback, useMemo } from "react"
import { useTranslation } from "react-i18next"
import { AlertCircle, CheckCircle2, ChevronDown, ChevronRight, Clock3, Copy, Check, Loader2, RotateCcw, ThumbsUp, ThumbsDown, Pencil, Search, Wrench } from "lucide-react"
import { Button } from "@/components/ui/button"
import { toast } from "sonner"
import type { UIMessage, DynamicToolUIPart, ToolUIPart } from "ai"
import { MessageResponse } from "@/components/ai-elements/message"
import { cn } from "@/lib/utils"

interface QAPairProps {
  userMessage: UIMessage
  aiMessages: UIMessage[]
  agentName?: string
  isStreaming?: boolean
  streamingContent?: string
  onRegenerate?: () => void
  onEditMessage?: (messageId: number, content: string) => void
  doneMetrics?: { durationMs?: number; inputTokens?: number; outputTokens?: number }
  streamingExtras?: React.ReactNode
  renderDataPart?: (part: UIMessage["parts"][number], message: UIMessage) => React.ReactNode
  suppressTextWhenDataPart?: boolean
}

// User query — right-aligned pill (ChatGPT style)
function UserQuery({
  content,
  images,
  messageId,
  onEdit,
}: {
  content: string
  images?: string[]
  messageId?: number
  onEdit?: (messageId: number, content: string) => void
}) {
  const { t } = useTranslation(["ai"])
  const [editing, setEditing] = useState(false)
  const [editContent, setEditContent] = useState(content)

  const handleSave = useCallback(() => {
    const trimmed = editContent.trim()
    if (!trimmed || !messageId || !onEdit) return
    onEdit(messageId, trimmed)
    setEditing(false)
  }, [editContent, messageId, onEdit])

  const handleCancel = useCallback(() => {
    setEditContent(content)
    setEditing(false)
  }, [content])

  if (editing) {
    return (
      <div className="flex justify-end mb-6">
        <div className="max-w-[70%] w-full">
          <div className="rounded-3xl bg-secondary p-4">
            <textarea
              value={editContent}
              onChange={(e) => setEditContent(e.target.value)}
              className="w-full min-h-[60px] max-h-[200px] resize-none bg-transparent text-sm leading-relaxed focus:outline-none"
              autoFocus
            />
            <div className="flex items-center gap-2 mt-3">
              <Button size="sm" onClick={handleSave} disabled={!editContent.trim()}>
                {t("ai:chat.saveAndRegenerate")}
              </Button>
              <Button size="sm" variant="ghost" onClick={handleCancel}>
                {t("ai:chat.cancelEdit")}
              </Button>
            </div>
          </div>
        </div>
      </div>
    )
  }

  return (
    <div className="group flex justify-end mb-6">
      <div className="flex items-start gap-1.5 max-w-[70%]">
        {onEdit && messageId != null && !Number.isNaN(messageId) && (
          <Button
            variant="ghost"
            size="icon"
            className="h-7 w-7 shrink-0 mt-1 text-muted-foreground/0 group-hover:text-muted-foreground hover:!text-foreground transition-colors"
            onClick={() => setEditing(true)}
          >
            <Pencil className="h-3.5 w-3.5" />
          </Button>
        )}
        <div className="rounded-3xl bg-secondary px-5 py-2.5">
          {images && images.length > 0 && (
            <div className="flex flex-wrap gap-2 mb-2">
              {images.map((src, idx) => (
                <img
                  key={idx}
                  src={src}
                  alt={`image-${idx}`}
                  className="max-h-48 max-w-xs rounded-xl object-cover"
                />
              ))}
            </div>
          )}
          {content && <div className="text-[15px] leading-relaxed whitespace-pre-wrap">{content}</div>}
        </div>
      </div>
    </div>
  )
}

type ToolActivityStatus = "preparing" | "running" | "completed" | "error"

interface ToolActivity {
  id: string
  toolName: string
  toolArgs?: unknown
  durationMs?: number
  status: ToolActivityStatus
  errorText?: string
}

// Compact tool activity row.
function ToolActivityRow({
  activity,
}: {
  activity: ToolActivity
}) {
  const { t } = useTranslation(["ai"])
  const [expanded, setExpanded] = useState(false)

  const toolName = activity.toolName || "unknown"
  const toolDisplayName = t(`ai:tools.toolDefs.${toolName}.name`, { defaultValue: toolName })
  const isKnowledgeSearch = toolName === "search_knowledge"
  const { argsText, parsedArgs } = useToolArgs(activity.toolArgs)
  const hasArgs = Boolean(argsText)
  const statusLabel = t(`ai:chat.toolStatus.${activity.status}`)

  const StatusIcon =
    activity.status === "completed"
      ? CheckCircle2
      : activity.status === "error"
        ? AlertCircle
        : activity.status === "running"
          ? Loader2
          : Clock3

  return (
    <div className="mb-3">
      <button
        type="button"
        className={cn(
          "group flex min-h-8 w-full items-center gap-2 rounded-md border border-border/55 bg-background/45 px-2.5 py-1.5 text-left text-xs text-muted-foreground transition-colors hover:border-border/80 hover:bg-accent/20 hover:text-foreground",
          activity.status === "error" && "border-destructive/30 bg-destructive/5 text-destructive",
        )}
        onClick={() => hasArgs && setExpanded((prev) => !prev)}
      >
        <div className="flex size-5 shrink-0 items-center justify-center rounded border border-border/50 bg-background/70">
          {isKnowledgeSearch
            ? <Search className="size-3 text-amber-600 dark:text-amber-400" />
            : <Wrench className="size-3 text-amber-600 dark:text-amber-400" />}
        </div>
        <StatusIcon
          className={cn(
            "size-3.5 shrink-0",
            activity.status === "running" && "animate-spin text-primary",
            activity.status === "completed" && "text-emerald-600 dark:text-emerald-400",
            activity.status === "error" && "text-destructive",
          )}
        />
        <span className="shrink-0">{statusLabel}</span>
        <span className="min-w-0 flex-1 truncate text-foreground/80">
          {isKnowledgeSearch && parsedArgs
            ? `${toolDisplayName}: "${parsedArgs.query ?? ""}"`
            : toolDisplayName}
        </span>
        {activity.durationMs != null && (
          <span className="shrink-0 text-[10px] text-muted-foreground/60">
            {(activity.durationMs / 1000).toFixed(1)}s
          </span>
        )}
        {hasArgs && (
          expanded
            ? <ChevronDown className="size-3 shrink-0 text-muted-foreground/70" />
            : <ChevronRight className="size-3 shrink-0 text-muted-foreground/70" />
        )}
      </button>
      {expanded && argsText && (
        <pre className="mt-1.5 max-h-44 overflow-auto rounded-md border border-border/50 bg-muted/35 p-3 text-xs font-mono text-muted-foreground">
          {argsText}
        </pre>
      )}
    </div>
  )
}

function useToolArgs(toolArgs: unknown) {
  return useMemo(() => {
    if (toolArgs == null) return { argsText: undefined, parsedArgs: null }
    if (typeof toolArgs === "string") {
      try {
        const parsed = JSON.parse(toolArgs) as Record<string, unknown>
        return { argsText: JSON.stringify(parsed, null, 2), parsedArgs: parsed }
      } catch {
        return { argsText: toolArgs, parsedArgs: null }
      }
    }
    if (typeof toolArgs === "object") {
      return {
        argsText: JSON.stringify(toolArgs, null, 2),
        parsedArgs: toolArgs as Record<string, unknown>,
      }
    }
    return { argsText: String(toolArgs), parsedArgs: null }
  }, [toolArgs])
}

type ChatToolPart = DynamicToolUIPart | ToolUIPart

function isChatToolPart(part: UIMessage["parts"][number]): part is ChatToolPart {
  return part.type === "dynamic-tool" || part.type.startsWith("tool-")
}

function isDataPart(part: UIMessage["parts"][number]) {
  return part.type.startsWith("data-")
}

function dataPartIdentity(part: UIMessage["parts"][number], fallback: number) {
  const data = (part as { data?: unknown }).data
  if (data && typeof data === "object" && "surfaceId" in data) {
    const surfaceId = (data as { surfaceId?: unknown }).surfaceId
    if (typeof surfaceId === "string" && surfaceId) {
      return `${part.type}:${surfaceId}`
    }
  }
  return `${part.type}:${fallback}`
}

function getToolName(part: ChatToolPart) {
  return part.type === "dynamic-tool" ? part.toolName : part.type.split("-").slice(1).join("-")
}

function getToolStatusFromPart(part: ChatToolPart): ToolActivityStatus {
  if (part.state === "input-streaming") return "preparing"
  if (part.state === "input-available") return "running"
  if (part.state === "output-error" || Boolean(part.errorText)) return "error"
  if (part.state === "output-available") {
    if (typeof part.output === "string" && (part.output.startsWith("Error:") || part.output.includes("unknown tool:"))) {
      return "error"
    }
    return "completed"
  }
  return "running"
}

function toolStatusFromMetadata(status: unknown): ToolActivityStatus | undefined {
  if (status === "running") return "running"
  if (status === "completed") return "completed"
  if (status === "error") return "error"
  return undefined
}

function collectToolActivities(aiMessages: UIMessage[], mainAiMessage?: UIMessage): ToolActivity[] {
  const activities = new Map<string, ToolActivity>()
  const order: string[] = []

  const addActivity = (activity: ToolActivity) => {
    const existing = activities.get(activity.id)
    if (!existing) {
      activities.set(activity.id, activity)
      order.push(activity.id)
      return
    }
    activities.set(activity.id, { ...existing, ...activity })
  }

  for (const tool of aiMessages) {
    const meta = tool.metadata as {
      originalRole?: string
      tool_name?: string
      tool_args?: unknown
      tool_call_id?: string
      duration_ms?: number
      status?: string
    } | undefined
    if (meta?.originalRole === "tool_call") {
      const id = meta.tool_call_id || String(tool.id)
      addActivity({
        id,
        toolName: meta.tool_name || "unknown",
        toolArgs: meta.tool_args,
        durationMs: meta.duration_ms,
        status: toolStatusFromMetadata(meta.status) ?? "running",
      })
    } else if (meta?.originalRole === "tool_result") {
      const id = meta.tool_call_id || String(tool.id)
      const existing = activities.get(id)
      addActivity({
        id,
        toolName: existing?.toolName || meta.tool_name || "unknown",
        toolArgs: existing?.toolArgs ?? meta.tool_args,
        durationMs: typeof meta.duration_ms === "number" ? meta.duration_ms : existing?.durationMs,
        status: toolStatusFromMetadata(meta.status) ?? "completed",
      })
    }
  }

  for (const part of mainAiMessage?.parts?.filter(isChatToolPart) ?? []) {
    const id = part.toolCallId
    addActivity({
      id,
      toolName: getToolName(part),
      toolArgs: part.input,
      status: getToolStatusFromPart(part),
      errorText: part.errorText,
    })
  }

  return order.map((id) => activities.get(id)).filter((activity): activity is ToolActivity => Boolean(activity))
}

// AI Response display
export function AIResponse({
  content,
  agentName,
  isStreaming,
  onRegenerate,
  doneMetrics,
}: {
  content: string
  agentName?: string
  isStreaming?: boolean
  onRegenerate?: () => void
  doneMetrics?: { durationMs?: number; inputTokens?: number; outputTokens?: number }
}) {
  const { t } = useTranslation(["ai"])
  const [copied, setCopied] = useState(false)
  const showMarkdown = !isStreaming

  const handleCopy = useCallback(async () => {
    try {
      await navigator.clipboard.writeText(content)
      setCopied(true)
      setTimeout(() => setCopied(false), 2000)
    } catch {
      toast.error("Copy failed")
    }
  }, [content])

  const metricsText = useMemo(() => {
    if (!doneMetrics?.durationMs) return null
    const parts: string[] = []
    const durationSec = (doneMetrics.durationMs / 1000).toFixed(1)
    if (doneMetrics.outputTokens && doneMetrics.durationMs > 0) {
      const tokPerSec = Math.round((doneMetrics.outputTokens / doneMetrics.durationMs) * 1000)
      parts.push(`${tokPerSec} tok/s`)
    }
    parts.push(`${durationSec}s`)
    if (doneMetrics.outputTokens) {
      parts.push(`${doneMetrics.outputTokens} tokens`)
    }
    return parts.join(" · ")
  }, [doneMetrics])

  return (
    <div className="mb-6">
      {agentName && (
        <div className="text-xs font-medium text-muted-foreground mb-1.5">{agentName}</div>
      )}
      <div className="text-base leading-relaxed">
        {!showMarkdown ? (
          <div className="whitespace-pre-wrap">{content}</div>
        ) : content ? (
          <MessageResponse>{content}</MessageResponse>
        ) : null}
      </div>

      {!isStreaming && content && (
        <div className="flex items-center gap-1 mt-3">
          <Button
            variant="ghost"
            size="sm"
            className="h-7 px-2 text-xs text-muted-foreground hover:text-foreground"
            onClick={handleCopy}
          >
            {copied
              ? <><Check className="h-3.5 w-3.5 mr-1" />{t("ai:chat.copied")}</>
              : <><Copy className="h-3.5 w-3.5 mr-1" />{t("ai:chat.copy")}</>}
          </Button>
          {onRegenerate && (
            <Button
              variant="ghost"
              size="sm"
              className="h-7 px-2 text-xs text-muted-foreground hover:text-foreground"
              onClick={onRegenerate}
            >
              <RotateCcw className="h-3.5 w-3.5 mr-1" />
              {t("ai:chat.regenerate")}
            </Button>
          )}
          <Button variant="ghost" size="sm" className="h-7 w-7 p-0 text-muted-foreground hover:text-foreground">
            <ThumbsUp className="h-3.5 w-3.5" />
          </Button>
          <Button variant="ghost" size="sm" className="h-7 w-7 p-0 text-muted-foreground hover:text-foreground">
            <ThumbsDown className="h-3.5 w-3.5" />
          </Button>
          {metricsText && (
            <span className="ml-auto text-[10px] text-muted-foreground/50">{metricsText}</span>
          )}
        </div>
      )}
    </div>
  )
}

// QA Pair component — document-flow layout
export function QAPair({
  userMessage,
  aiMessages,
  agentName,
  isStreaming,
  streamingContent,
  onRegenerate,
  onEditMessage,
  doneMetrics,
  streamingExtras,
  renderDataPart,
  suppressTextWhenDataPart,
}: QAPairProps) {
  const userImages = (userMessage.metadata as { images?: string[] } | undefined)?.images
  const userText = userMessage.parts
    ?.filter((p): p is { type: "text"; text: string } => p.type === "text")
    .map((p) => p.text)
    .join("") || ""

  const mainAiMessages = aiMessages.filter((m) => {
    const meta = m.metadata as { originalRole?: string } | undefined
    return !["tool_call", "tool_result"].includes(meta?.originalRole || "")
  })

  const mainAiMessage = mainAiMessages[mainAiMessages.length - 1]
  const toolActivities = collectToolActivities(aiMessages, mainAiMessage)
  const dataParts = mainAiMessages.flatMap((message) => message.parts?.filter(isDataPart) ?? [])
  const latestDataParts = Array.from(
    dataParts.reduce((acc, part, index) => acc.set(dataPartIdentity(part, index), part), new Map<string, UIMessage["parts"][number]>()),
  )
  const renderedDataParts = renderDataPart
    ? latestDataParts
        .map(([key, part]) => {
          const message = mainAiMessages.find((item) => item.parts?.includes(part)) ?? mainAiMessage
          return { key, node: message ? renderDataPart(part, message) : null }
        })
        .filter((item) => item.node != null)
    : []

  const mainContent = streamingContent || (
    mainAiMessage?.parts
      ?.filter((p): p is { type: "text"; text: string } => p.type === "text")
      .map((p) => p.text)
      .join("") || ""
  )
  const showMainResponse = !suppressTextWhenDataPart || renderedDataParts.length === 0
  const hasAuxiliaryResponse = toolActivities.length > 0 || renderedDataParts.length > 0
  const showAssistantHeader = Boolean(
    agentName && (hasAuxiliaryResponse || showMainResponse && (mainAiMessage || streamingContent)),
  )

  return (
    <div className="py-6">
      <UserQuery
        content={userText}
        images={userImages}
        messageId={Number(userMessage.id)}
        onEdit={onEditMessage}
      />

      {showAssistantHeader && (
        <div className="mb-1.5 text-xs font-medium text-muted-foreground">{agentName}</div>
      )}

      {/* Streaming extras (thinking block, plan progress, loading dots) */}
      {streamingExtras}

      {toolActivities.map((activity) => (
        <ToolActivityRow key={activity.id} activity={activity} />
      ))}

      {renderedDataParts.map((item) => (
        <div key={item.key}>{item.node}</div>
      ))}

      {/* AI response */}
      {showMainResponse && (mainAiMessage || streamingContent) && (
        <AIResponse
          content={mainContent}
          isStreaming={isStreaming}
          onRegenerate={onRegenerate}
          doneMetrics={doneMetrics}
        />
      )}
    </div>
  )
}

export type { QAPairProps }
