import { isDataUIPart, isReasoningUIPart, type UIMessage } from "ai"
import type { ReactNode } from "react"
import { ThinkingBlock } from "./thinking-block"
import { PlanProgress } from "./plan-progress"
import type { ChatMessagePair } from "./types"

export function groupUIMessagesIntoPairs(messages: UIMessage[]): ChatMessagePair[] {
  const pairs: ChatMessagePair[] = []
  for (const msg of messages) {
    if (msg.role === "user") {
      pairs.push({ userMessage: msg, aiMessages: [] })
    } else if (pairs.length > 0) {
      pairs[pairs.length - 1].aiMessages.push(msg)
    }
  }
  return pairs
}

export function getMainAssistantMessage(pair: ChatMessagePair) {
  return pair.aiMessages
    .filter((m) => {
      const meta = m.metadata as { originalRole?: string } | undefined
      return !["tool_call", "tool_result"].includes(meta?.originalRole || "")
    })
    .pop()
}

export function getStreamingExtras(
  pair: ChatMessagePair,
  isStreaming: boolean,
  agentName?: string,
): ReactNode {
  const mainAiMessage = getMainAssistantMessage(pair)
  const reasoningParts = mainAiMessage?.parts?.filter(isReasoningUIPart) || []
  const thinkingText = reasoningParts.map((p) => p.text).join("")
  const dataParts = mainAiMessage?.parts?.filter(isDataUIPart) || []

  let planSteps: { description: string; durationMs?: number }[] = []
  let planStepIndex = -1
  for (const part of dataParts) {
    const d = part.data as Record<string, unknown> | undefined
    if (!d) continue
    if (part.type === "data-plan" && Array.isArray(d.steps)) {
      planSteps = (d.steps as Array<{ description?: string }>).map((s) => ({
        description: s.description || "",
        durationMs: undefined,
      }))
      planStepIndex = 0
    } else if (part.type === "data-step" && typeof d.index === "number") {
      if (d.state === "start") {
        planStepIndex = d.index as number
      } else if (d.state === "done") {
        const idx = d.index as number
        if (planSteps[idx]) {
          planSteps[idx].durationMs = typeof d.durationMs === "number" ? d.durationMs : undefined
        }
        planStepIndex = idx + 1
      }
    }
  }

  const textParts =
    mainAiMessage?.parts?.filter(
      (p): p is { type: "text"; text: string } => p.type === "text",
    ) || []
  const hasText = textParts.some((p) => p.text)
  const hasTools = mainAiMessage?.parts?.some((p) => p.type === "dynamic-tool" || p.type.startsWith("tool-"))
  const hasContent = hasText || thinkingText || planSteps.length > 0 || hasTools

  const extras: ReactNode[] = []
  if (thinkingText) {
    extras.push(<ThinkingBlock key="thinking" content={thinkingText} isStreaming={isStreaming} />)
  }
  if (planSteps.length > 0) {
    extras.push(
      <PlanProgress
        key="plan"
        steps={planSteps}
        currentStepIndex={planStepIndex}
        isStreaming={isStreaming}
      />,
    )
  }
  if (isStreaming && !hasContent) {
    extras.push(
      <div key="loading" className="mb-4 flex items-center gap-2 text-sm text-muted-foreground">
        {agentName && <span className="text-xs font-medium">{agentName}</span>}
        <span className="flex gap-1">
          <span className="h-1.5 w-1.5 animate-bounce rounded-full bg-foreground/40 [animation-delay:0ms]" />
          <span className="h-1.5 w-1.5 animate-bounce rounded-full bg-foreground/40 [animation-delay:150ms]" />
          <span className="h-1.5 w-1.5 animate-bounce rounded-full bg-foreground/40 [animation-delay:300ms]" />
        </span>
      </div>,
    )
  }
  return extras.length > 0 ? <>{extras}</> : null
}
