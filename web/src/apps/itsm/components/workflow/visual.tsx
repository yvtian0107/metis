import { useTranslation } from "react-i18next"
import type React from "react"
import {
  Bell,
  CheckCircle2,
  CircleDot,
  Clock,
  Code2,
  FileText,
  GitBranch,
  GitMerge,
  Layers,
  Play,
  Radio,
  ShieldCheck,
  Square,
  Wrench,
  XCircle,
  Zap,
} from "lucide-react"
import { cn } from "@/lib/utils"
import type { NodeType, WFNodeData } from "./types"
import { buildNodeSummary, getNodeAccent, getNodeRuntimeClass } from "./visual-data"

export function WorkflowNodeIconGlyph({ nodeType, className }: { nodeType?: NodeType; className?: string }) {
  const iconClassName = cn("size-3.5", className)

  switch (nodeType) {
    case "start":
      return <Play className={iconClassName} strokeWidth={2.2} />
    case "end":
      return <Square className={iconClassName} strokeWidth={2.2} />
    case "form":
      return <FileText className={iconClassName} strokeWidth={2.2} />
    case "approve":
      return <ShieldCheck className={iconClassName} strokeWidth={2.2} />
    case "process":
      return <Wrench className={iconClassName} strokeWidth={2.2} />
    case "action":
      return <Zap className={iconClassName} strokeWidth={2.2} />
    case "exclusive":
      return <GitBranch className={iconClassName} strokeWidth={2.2} />
    case "notify":
      return <Bell className={iconClassName} strokeWidth={2.2} />
    case "wait":
    case "timer":
      return <Clock className={iconClassName} strokeWidth={2.2} />
    case "signal":
      return <Radio className={iconClassName} strokeWidth={2.2} />
    case "parallel":
      return <GitMerge className={iconClassName} strokeWidth={2.2} />
    case "inclusive":
      return <CircleDot className={iconClassName} strokeWidth={2.2} />
    case "subprocess":
      return <Layers className={iconClassName} strokeWidth={2.2} />
    case "script":
      return <Code2 className={iconClassName} strokeWidth={2.2} />
    default:
      return <Wrench className={iconClassName} strokeWidth={2.2} />
  }
}

export function WorkflowNodeIconBox({ nodeType, className, iconClassName }: { nodeType?: NodeType; className?: string; iconClassName?: string }) {
  return (
    <span
      className={cn("flex size-7 shrink-0 items-center justify-center rounded-lg text-white", className)}
      style={{ backgroundColor: getNodeAccent(nodeType) }}
    >
      <WorkflowNodeIconGlyph nodeType={nodeType} className={iconClassName} />
    </span>
  )
}

export function WorkflowNodeStatus({ state }: { state?: WFNodeData["_workflowState"] }) {
  const { t } = useTranslation("itsm")

  if (state === "active") {
    return (
      <span className="inline-flex items-center gap-1 rounded-full border border-blue-500/20 bg-blue-500/10 px-1.5 py-0.5 text-[10px] font-medium text-blue-700">
        <span className="size-1.5 rounded-full bg-blue-500" />
        {t("workflow.state.active")}
      </span>
    )
  }

  if (state === "completed") {
    return <CheckCircle2 className="size-3.5 text-emerald-600" />
  }

  if (state === "failed") {
    return <XCircle className="size-3.5 text-red-600" />
  }

  if (state === "cancelled") {
    return <span className="rounded-full border border-border/70 px-1.5 py-0.5 text-[10px] text-muted-foreground">{t("workflow.state.cancelled")}</span>
  }

  return null
}

export function WorkflowNodeCard({
  data,
  selected,
  children,
  className,
}: {
  data: WFNodeData
  selected?: boolean
  children?: React.ReactNode
  className?: string
}) {
  const { t } = useTranslation("itsm")
  const accent = getNodeAccent(data.nodeType)
  const summary = buildNodeSummary(data, t)

  return (
    <div
      className={cn(
        "workflow-node-card group relative w-[240px] rounded-2xl border bg-white/82 text-foreground transition-[border-color,box-shadow,opacity]",
        "shadow-[0_1px_1px_rgba(15,23,42,0.04),0_12px_34px_-30px_rgba(15,23,42,0.38)]",
        selected && "border-primary/60 ring-2 ring-primary/15",
        getNodeRuntimeClass(data._workflowState),
        className,
      )}
    >
      <div className="flex items-start gap-2.5 px-3 py-3">
        <div
          className="flex size-7 shrink-0 items-center justify-center rounded-lg border border-white/70 text-white shadow-[inset_0_1px_0_rgba(255,255,255,0.26)]"
          style={{ backgroundColor: accent }}
        >
          <WorkflowNodeIconGlyph nodeType={data.nodeType} />
        </div>
        <div className="min-w-0 flex-1">
          <div className="flex min-w-0 items-center justify-between gap-2">
            <span className="truncate text-sm font-semibold tracking-[-0.01em]">{data.label}</span>
            <WorkflowNodeStatus state={data._workflowState} />
          </div>
          <div className="mt-1 text-[11px] font-medium text-muted-foreground">
            {t(`workflow.node.${data.nodeType}` as const)}
          </div>
        </div>
      </div>
      {(summary || children) && (
        <div className="border-t border-border/45 px-3 py-2">
          {summary && <div className="truncate text-xs text-muted-foreground">{summary}</div>}
          {children}
        </div>
      )}
    </div>
  )
}
