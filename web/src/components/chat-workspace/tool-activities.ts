import type { DynamicToolUIPart, ToolUIPart, UIMessage } from "ai"

export type ToolActivityStatus = "preparing" | "running" | "completed" | "error"
export type ChatToolPart = DynamicToolUIPart | ToolUIPart

export interface ToolActivity {
  id: string
  toolName: string
  toolArgs?: unknown
  durationMs?: number
  status: ToolActivityStatus
  errorText?: string
}

export function isChatToolPart(part: UIMessage["parts"][number]): part is ChatToolPart {
  return part.type === "dynamic-tool" || part.type.startsWith("tool-")
}

export function getToolName(part: ChatToolPart) {
  return part.type === "dynamic-tool" ? part.toolName : part.type.split("-").slice(1).join("-")
}

export function getToolStatusFromPart(part: ChatToolPart): ToolActivityStatus {
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

function toolActivityRank(status: ToolActivityStatus) {
  if (status === "error") return 4
  if (status === "completed") return 3
  if (status === "running") return 2
  return 1
}

function mergeToolActivity(existing: ToolActivity, next: ToolActivity): ToolActivity {
  const shouldUseNext = toolActivityRank(next.status) >= toolActivityRank(existing.status)
  const base = shouldUseNext ? next : existing
  const fallback = shouldUseNext ? existing : next
  return {
    ...base,
    toolName: base.toolName || fallback.toolName,
    toolArgs: base.toolArgs ?? fallback.toolArgs,
    durationMs: base.durationMs ?? fallback.durationMs,
    errorText: base.errorText ?? fallback.errorText,
  }
}

export function collectToolActivitiesFromParts(message: UIMessage): ToolActivity[] {
  const activities = new Map<string, ToolActivity>()

  for (const part of message.parts?.filter(isChatToolPart) ?? []) {
    const activity = {
      id: part.toolCallId,
      toolName: getToolName(part),
      toolArgs: part.input,
      status: getToolStatusFromPart(part),
      errorText: part.errorText,
    }
    const existing = activities.get(activity.id)
    activities.set(activity.id, existing ? mergeToolActivity(existing, activity) : activity)
  }

  return Array.from(activities.values())
}
