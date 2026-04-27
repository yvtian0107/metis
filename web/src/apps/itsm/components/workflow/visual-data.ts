import type { NodeType, Participant, WFNodeData } from "./types"
import { NODE_COLORS } from "./types"

export type WorkflowNodeGroup = {
  label: string
  types: NodeType[]
}

export const WORKFLOW_NODE_GROUPS: WorkflowNodeGroup[] = [
  { label: "workflow.group.human", types: ["form", "process"] },
  { label: "workflow.group.automation", types: ["action", "script", "notify"] },
  { label: "workflow.group.control", types: ["exclusive", "parallel", "inclusive"] },
  { label: "workflow.group.composite", types: ["wait", "subprocess"] },
  { label: "workflow.group.events", types: ["end"] },
]

export function getNodeAccent(nodeType?: NodeType): string {
  return nodeType ? NODE_COLORS[nodeType] ?? "#475569" : "#475569"
}

export function getNodeRuntimeClass(state: WFNodeData["_workflowState"]) {
  if (state === "active") return "border-blue-500/70 ring-2 ring-blue-500/20"
  if (state === "completed") return "border-emerald-500/55"
  if (state === "failed") return "border-red-500/65"
  if (state === "cancelled") return "border-border/60 opacity-65"
  return "border-border/70"
}

export function participantSummary(participants?: Participant[]): string {
  if (!participants || participants.length === 0) return ""
  const first = participants[0].name ?? participants[0].value ?? participants[0].type
  if (participants.length === 1) return String(first)
  return `${first} +${participants.length - 1}`
}

export function buildNodeSummary(data: WFNodeData, t: (key: string) => string): string {
  const parts: string[] = []

  if (data.nodeType === "form") {
    parts.push(data.formSchema ? t("workflow.summary.formBound") : t("workflow.summary.formUnbound"))
  }

  if (data.nodeType === "process") {
    const labels: Record<string, string> = {
      single: t("workflow.prop.modeSingle"),
      parallel: t("workflow.prop.modeParallel"),
      sequential: t("workflow.prop.modeSequential"),
    }
    parts.push(labels[data.executionMode ?? "single"] ?? data.executionMode ?? "")
  }

  if (data.nodeType === "action") {
    parts.push(data.action_id ? t("workflow.summary.actionBound") : t("workflow.summary.actionUnbound"))
  }

  if (data.nodeType === "notify") {
    parts.push(data.channel_id ? `#${data.channel_id}` : t("workflow.summary.notifyUnset"))
  }

  if ((data.nodeType === "timer" || data.wait_mode === "timer") && data.duration) {
    parts.push(data.duration)
  }

  const pSummary = participantSummary(data.participants)
  if (pSummary && (data.nodeType === "form" || data.nodeType === "process")) {
    parts.push(pSummary)
  }

  return parts.filter(Boolean).join(" · ")
}
