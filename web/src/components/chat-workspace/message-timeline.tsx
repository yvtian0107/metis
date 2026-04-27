"use client"

import { useCallback, useMemo, useState, type ReactNode } from "react"
import { useTranslation } from "react-i18next"
import { isDataUIPart, isReasoningUIPart, type UIMessage } from "ai"
import {
  AlertCircle,
  Check,
  CheckCircle2,
  ChevronDown,
  ChevronRight,
  Clock3,
  Copy,
  Loader2,
  Pencil,
  RotateCcw,
  Search,
  Sparkles,
  ThumbsDown,
  ThumbsUp,
  Wrench,
} from "lucide-react"
import { toast } from "sonner"

import { MessageResponse } from "@/components/ai-elements/message"
import { Button } from "@/components/ui/button"
import { cn } from "@/lib/utils"
import { PlanProgress } from "./plan-progress"
import { ThinkingBlock } from "./thinking-block"
import { collectToolActivitiesFromParts, type ToolActivity, type ToolActivityStatus } from "./tool-activities"

type FilePart = { type: "file"; url?: string; mediaType: string }

interface MessageTimelineProps {
  messages: UIMessage[]
  agentName?: string
  isBusy?: boolean
  status?: string
  onRegenerate?: () => void
  onEditMessage?: (messageId: number, content: string) => void
  doneMetrics?: { durationMs?: number; inputTokens?: number; outputTokens?: number }
  renderDataPart?: (part: UIMessage["parts"][number], message: UIMessage) => ReactNode
  shouldSuppressDataPart?: (part: UIMessage["parts"][number]) => boolean
}

type TimelineItem =
  | { type: "message"; message: UIMessage }
  | { type: "tool"; key: string; activity: ToolActivity }

function getMessageText(message: UIMessage) {
  return message.parts
    ?.filter((part): part is { type: "text"; text: string } => part.type === "text")
    .map((part) => part.text)
    .join("") || ""
}

function getMessageImages(message: UIMessage) {
  const metadataImages = (message.metadata as { images?: string[] } | undefined)?.images ?? []
  const fileImages =
    message.parts
      ?.filter((part): part is UIMessage["parts"][number] & FilePart => part.type === "file")
      .filter((part) => part.mediaType.startsWith("image/"))
      .map((part) => part.url)
      .filter((url): url is string => Boolean(url)) ?? []
  return [...metadataImages, ...fileImages]
}

function isStoredToolMessage(message: UIMessage) {
  const meta = message.metadata as { originalRole?: string } | undefined
  return meta?.originalRole === "tool_call" || meta?.originalRole === "tool_result"
}

function toolStatusFromMetadata(status: unknown): ToolActivityStatus | undefined {
  if (status === "running") return "running"
  if (status === "completed") return "completed"
  if (status === "error") return "error"
  return undefined
}

function activityFromStoredToolMessage(message: UIMessage, existing?: ToolActivity): ToolActivity {
  const meta = message.metadata as {
    originalRole?: string
    tool_name?: string
    tool_args?: unknown
    tool_call_id?: string
    duration_ms?: number
    status?: string
  } | undefined

  const isResult = meta?.originalRole === "tool_result"
  return {
    id: meta?.tool_call_id || String(message.id),
    toolName: existing?.toolName || meta?.tool_name || "unknown",
    toolArgs: existing?.toolArgs ?? meta?.tool_args,
    durationMs: typeof meta?.duration_ms === "number" ? meta.duration_ms : existing?.durationMs,
    status: toolStatusFromMetadata(meta?.status) ?? (isResult ? "completed" : "running"),
  }
}

function buildTimelineItems(messages: UIMessage[]) {
  const items: TimelineItem[] = []
  const toolItemIndex = new Map<string, number>()

  for (const message of messages) {
    if (!isStoredToolMessage(message)) {
      items.push({ type: "message", message })
      continue
    }

    const activity = activityFromStoredToolMessage(message)
    const existingIndex = toolItemIndex.get(activity.id)
    if (existingIndex == null) {
      toolItemIndex.set(activity.id, items.length)
      items.push({ type: "tool", key: activity.id, activity })
    } else {
      const existing = items[existingIndex]
      if (existing.type === "tool") {
        items[existingIndex] = {
          ...existing,
          activity: activityFromStoredToolMessage(message, existing.activity),
        }
      }
    }
  }

  return items
}

function UserMessage({
  message,
  onEdit,
}: {
  message: UIMessage
  onEdit?: (messageId: number, content: string) => void
}) {
  const { t } = useTranslation(["ai"])
  const content = getMessageText(message)
  const images = getMessageImages(message)
  const messageId = Number(message.id)
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
      <div className="flex justify-end py-4">
        <div className="w-full max-w-[78%]">
          <div className="rounded-3xl bg-secondary p-4">
            <textarea
              value={editContent}
              onChange={(event) => setEditContent(event.target.value)}
              className="max-h-[200px] min-h-[60px] w-full resize-none bg-transparent text-sm leading-relaxed focus:outline-none"
              autoFocus
            />
            <div className="mt-3 flex items-center gap-2">
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
    <div className="group flex justify-end py-4">
      <div className="flex max-w-[78%] items-start gap-1.5">
        {onEdit && messageId != null && !Number.isNaN(messageId) && (
          <Button
            variant="ghost"
            size="icon"
            className="mt-1 size-7 shrink-0 text-muted-foreground/0 transition-colors group-hover:text-muted-foreground hover:!text-foreground"
            onClick={() => setEditing(true)}
          >
            <Pencil className="size-3.5" />
          </Button>
        )}
        <div className="rounded-[1.4rem] bg-secondary/85 px-5 py-2.5 shadow-[0_10px_28px_-24px_hsl(var(--foreground))]">
          {images.length > 0 && (
            <div className="mb-2 flex flex-wrap gap-2">
              {images.map((src, index) => (
                <img
                  key={`${src}-${index}`}
                  src={src}
                  alt={`image-${index}`}
                  className="max-h-48 max-w-xs rounded-xl border border-border/50 object-cover"
                />
              ))}
            </div>
          )}
          {content && <div className="whitespace-pre-wrap text-[15px] leading-relaxed">{content}</div>}
        </div>
      </div>
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

function ToolActivityRow({ activity }: { activity: ToolActivity }) {
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
    <div className="py-1.5">
      <button
        type="button"
        data-testid="chat-tool-activity"
        data-status={activity.status}
        className={cn(
          "group flex min-h-8 w-full items-center gap-2 rounded-lg border border-border/55 bg-background/55 px-2.5 py-1.5 text-left text-xs text-muted-foreground transition-colors hover:border-border/80 hover:bg-accent/20 hover:text-foreground",
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
        <pre className="mt-1.5 max-h-44 overflow-auto rounded-md border border-border/50 bg-muted/35 p-3 font-mono text-xs text-muted-foreground">
          {argsText}
        </pre>
      )}
    </div>
  )
}

function isDataPart(part: UIMessage["parts"][number]) {
  return part.type.startsWith("data-")
}

function dataPartIdentity(part: UIMessage["parts"][number], fallback: number) {
  const data = (part as { data?: unknown }).data
  if (data && typeof data === "object" && "surfaceId" in data) {
    const surfaceId = (data as { surfaceId?: unknown }).surfaceId
    if (typeof surfaceId === "string" && surfaceId) return `${part.type}:${surfaceId}`
  }
  return `${part.type}:${fallback}`
}

function getAssistantExtras(message: UIMessage, isStreaming?: boolean) {
  const reasoningParts = message.parts?.filter(isReasoningUIPart) ?? []
  const thinkingText = reasoningParts.map((part) => part.text).join("")
  const dataParts = message.parts?.filter(isDataUIPart) ?? []
  let planSteps: { description: string; durationMs?: number }[] = []
  let planStepIndex = -1

  for (const part of dataParts) {
    const data = part.data as Record<string, unknown> | undefined
    if (!data) continue
    if (part.type === "data-plan" && Array.isArray(data.steps)) {
      planSteps = (data.steps as Array<{ description?: string }>).map((step) => ({
        description: step.description || "",
        durationMs: undefined,
      }))
      planStepIndex = 0
    } else if (part.type === "data-step" && typeof data.index === "number") {
      if (data.state === "start") {
        planStepIndex = data.index
      } else if (data.state === "done") {
        const index = data.index
        if (planSteps[index]) {
          planSteps[index].durationMs = typeof data.durationMs === "number" ? data.durationMs : undefined
        }
        planStepIndex = index + 1
      }
    }
  }

  return (
    <>
      {thinkingText && <ThinkingBlock content={thinkingText} isStreaming={Boolean(isStreaming)} />}
      {planSteps.length > 0 && (
        <PlanProgress steps={planSteps} currentStepIndex={planStepIndex} isStreaming={Boolean(isStreaming)} />
      )}
    </>
  )
}

export function AIResponse({
  content,
  isStreaming,
  onRegenerate,
  doneMetrics,
}: {
  content: string
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
    if (doneMetrics.outputTokens) parts.push(`${doneMetrics.outputTokens} tokens`)
    return parts.join(" · ")
  }, [doneMetrics])

  return (
    <div>
      <div className="text-[15px] leading-7">
        {!showMarkdown ? (
          <div className="whitespace-pre-wrap">{content}</div>
        ) : content ? (
          <MessageResponse>{content}</MessageResponse>
        ) : isStreaming ? (
          <ProcessingRow />
        ) : null}
      </div>

      {!isStreaming && content && (
        <div className="mt-3 flex items-center gap-1">
          <Button
            variant="ghost"
            size="sm"
            className="h-7 px-2 text-xs text-muted-foreground hover:text-foreground"
            onClick={handleCopy}
          >
            {copied
              ? <><Check className="mr-1 size-3.5" />{t("ai:chat.copied")}</>
              : <><Copy className="mr-1 size-3.5" />{t("ai:chat.copy")}</>}
          </Button>
          {onRegenerate && (
            <Button
              variant="ghost"
              size="sm"
              className="h-7 px-2 text-xs text-muted-foreground hover:text-foreground"
              onClick={onRegenerate}
            >
              <RotateCcw className="mr-1 size-3.5" />
              {t("ai:chat.regenerate")}
            </Button>
          )}
          <Button variant="ghost" size="sm" className="size-7 p-0 text-muted-foreground hover:text-foreground">
            <ThumbsUp className="size-3.5" />
          </Button>
          <Button variant="ghost" size="sm" className="size-7 p-0 text-muted-foreground hover:text-foreground">
            <ThumbsDown className="size-3.5" />
          </Button>
          {metricsText && <span className="ml-auto text-[10px] text-muted-foreground/50">{metricsText}</span>}
        </div>
      )}
    </div>
  )
}

function AssistantMessage({
  message,
  agentName,
  isStreaming,
  onRegenerate,
  doneMetrics,
  renderDataPart,
  shouldSuppressDataPart,
}: {
  message: UIMessage
  agentName?: string
  isStreaming?: boolean
  onRegenerate?: () => void
  doneMetrics?: { durationMs?: number; inputTokens?: number; outputTokens?: number }
  renderDataPart?: (part: UIMessage["parts"][number], message: UIMessage) => ReactNode
  shouldSuppressDataPart?: (part: UIMessage["parts"][number]) => boolean
}) {
  const content = getMessageText(message)
  const toolActivities = collectToolActivitiesFromParts(message)
  const dataParts = message.parts?.filter(isDataPart) ?? []
  const latestDataParts = Array.from(
    dataParts.reduce((acc, part, index) => acc.set(dataPartIdentity(part, index), part), new Map<string, UIMessage["parts"][number]>()),
  )
  const renderedDataParts = renderDataPart
    ? latestDataParts
        .map(([key, part]) => ({ key, node: renderDataPart(part, message) }))
        .filter((item) => item.node != null)
    : []
  const suppressText = dataParts.some((part) => shouldSuppressDataPart?.(part))
  const showMainResponse = !suppressText || renderedDataParts.length === 0
  const hasVisibleContent = content || toolActivities.length > 0 || renderedDataParts.length > 0 || isStreaming

  if (!hasVisibleContent) return null

  return (
    <div className="py-4">
      {agentName && <div className="mb-2 text-xs font-medium text-muted-foreground">{agentName}</div>}
      {getAssistantExtras(message, isStreaming)}
      {toolActivities.map((activity) => (
        <ToolActivityRow key={activity.id} activity={activity} />
      ))}
      {renderedDataParts.map((item) => (
        <div key={item.key} className="mb-4">{item.node}</div>
      ))}
      {showMainResponse && (
        <AIResponse
          content={content}
          isStreaming={isStreaming}
          onRegenerate={onRegenerate}
          doneMetrics={doneMetrics}
        />
      )}
    </div>
  )
}

function ProcessingRow() {
  return (
    <div className="flex items-center gap-2 text-sm text-muted-foreground">
      <Sparkles className="size-3.5 text-primary" />
      正在处理
      <span className="flex gap-1">
        <span className="size-1 animate-pulse rounded-full bg-muted-foreground/55" />
        <span className="size-1 animate-pulse rounded-full bg-muted-foreground/45 [animation-delay:120ms]" />
        <span className="size-1 animate-pulse rounded-full bg-muted-foreground/35 [animation-delay:240ms]" />
      </span>
    </div>
  )
}

function PendingAssistantRow({ agentName }: { agentName?: string }) {
  return (
    <div className="py-4">
      {agentName && <div className="mb-2 text-xs font-medium text-muted-foreground">{agentName}</div>}
      <ProcessingRow />
    </div>
  )
}

export function MessageTimeline({
  messages,
  agentName,
  isBusy,
  status,
  onRegenerate,
  onEditMessage,
  doneMetrics,
  renderDataPart,
  shouldSuppressDataPart,
}: MessageTimelineProps) {
  const items = useMemo(() => buildTimelineItems(messages), [messages])
  const lastMessage = messages[messages.length - 1]
  const lastAssistantMessageId = [...messages].reverse().find((message) => message.role === "assistant" && !isStoredToolMessage(message))?.id
  const showPendingAssistant = Boolean(isBusy && (!lastMessage || lastMessage.role === "user"))

  return (
    <>
      {items.map((item) => {
        if (item.type === "tool") {
          return <ToolActivityRow key={`tool-${item.key}`} activity={item.activity} />
        }

        const message = item.message
        if (message.role === "user") {
          return <UserMessage key={message.id} message={message} onEdit={onEditMessage} />
        }

        return (
          <AssistantMessage
            key={message.id}
            message={message}
            agentName={agentName}
            isStreaming={Boolean(isBusy && message.id === lastAssistantMessageId)}
            onRegenerate={message.id === lastAssistantMessageId ? onRegenerate : undefined}
            doneMetrics={message.id === lastAssistantMessageId && status === "ready" ? doneMetrics : undefined}
            renderDataPart={renderDataPart}
            shouldSuppressDataPart={shouldSuppressDataPart}
          />
        )
      })}
      {showPendingAssistant && <PendingAssistantRow agentName={agentName} />}
    </>
  )
}
