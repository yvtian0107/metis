"use client"

import { useEffect, useState, type ReactNode } from "react"
import { useParams, useNavigate } from "react-router"
import { useTranslation } from "react-i18next"
import { useForm } from "react-hook-form"
import { z } from "zod"
import { zodResolver } from "@hookform/resolvers/zod"
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query"
import {
  AlertTriangle,
  ArrowLeft,
  ArrowRight,
  Bot,
  ChevronDown,
  CheckCircle,
  CheckCircle2,
  CircleX,
  Clock,
  FileText,
  Info,
  Loader2,
  Play,
  PlusCircle,
  RotateCcw,
  ShieldCheck,
  UserPlus,
  XCircle,
  Zap,
  type LucideIcon,
} from "lucide-react"
import { toast } from "sonner"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import {
  Form,
  FormControl,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from "@/components/ui/form"
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select"
import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetFooter,
  SheetHeader,
  SheetTitle,
} from "@/components/ui/sheet"
import { Textarea } from "@/components/ui/textarea"
import { Progress } from "@/components/ui/progress"
import { Popover, PopoverContent, PopoverTrigger } from "@/components/ui/popover"
import { Collapsible, CollapsibleContent, CollapsibleTrigger } from "@/components/ui/collapsible"
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table"
import { usePermission } from "@/hooks/use-permission"
import { useAuthStore } from "@/stores/auth"
import {
  assignTicket,
  cancelTicket,
  fetchTicket,
  fetchTicketActivities,
  fetchTicketTimeline,
  fetchTicketTokens,
  fetchUsers,
  progressTicket,
  withdrawTicket,
  type ActivityItem,
  type TicketItem,
  type TimelineItem,
} from "../../../api"
import { SLABadge } from "../../../components/sla-badge"
import { TicketStatusBadge } from "../../../components/ticket-status-badge"
import { WorkflowViewer } from "../../../components/workflow"
import {
  parseFieldDisplayMeta as parseTicketFieldDisplayMeta,
  parseFieldDisplaySections,
  resolveFieldDisplayModel,
  resolveFieldDisplayValue as resolveTicketFieldDisplayValue,
  resolveFieldLabel as resolveTicketFieldLabel,
  toRecord as toDisplayRecord,
  type FieldDisplayMeta as TicketFieldDisplayMeta,
  type FieldDisplayModel,
} from "./field-display"
import { buildTicketActionContext } from "./ticket-action-context"

const TERMINAL_STATUSES = new Set(["completed", "rejected", "withdrawn", "cancelled", "failed"])
const DEFAULT_DECISIONING_MESSAGE = "流程决策岗正在生成下一步，页面会自动刷新。"
type ApprovalOutcome = "approved" | "rejected"

const DEFAULT_EVENT_STYLE = { icon: Clock, bg: "bg-muted", fg: "text-muted-foreground" }
const TIMELINE_EVENT_STYLE: Record<string, { icon: LucideIcon; bg: string; fg: string }> = {
  ticket_created:        { icon: PlusCircle, bg: "bg-blue-100", fg: "text-blue-600" },
  ticket_assigned:       { icon: UserPlus, bg: "bg-blue-100", fg: "text-blue-600" },
  ticket_completed:      { icon: CheckCircle, bg: "bg-green-100", fg: "text-green-600" },
  ticket_cancelled:      { icon: XCircle, bg: "bg-gray-200", fg: "text-gray-500" },
  withdrawn:             { icon: RotateCcw, bg: "bg-gray-200", fg: "text-gray-500" },
  workflow_started:      { icon: Play, bg: "bg-blue-100", fg: "text-blue-600" },
  workflow_completed:    { icon: CheckCircle, bg: "bg-green-100", fg: "text-green-600" },
  activity_completed:    { icon: CheckCircle, bg: "bg-green-100", fg: "text-green-600" },
  ai_decision_pending:           { icon: Bot, bg: "bg-amber-100", fg: "text-amber-600" },
  ai_decision_executed:          { icon: Bot, bg: "bg-green-100", fg: "text-green-600" },
  ai_decision_failed:            { icon: AlertTriangle, bg: "bg-red-100", fg: "text-red-600" },
  ai_decision_action_executed:   { icon: Zap, bg: "bg-green-100", fg: "text-green-600" },
  ai_disabled:           { icon: AlertTriangle, bg: "bg-amber-100", fg: "text-amber-600" },
  ai_retry:              { icon: RotateCcw, bg: "bg-amber-100", fg: "text-amber-600" },
  override_jump:         { icon: ArrowRight, bg: "bg-orange-100", fg: "text-orange-600" },
  override_reassign:     { icon: UserPlus, bg: "bg-orange-100", fg: "text-orange-600" },
}

interface DecisionActivity {
  type?: string
  participant_type?: string
  participant_id?: number
  participant_name?: string
  instructions?: string
  action_id?: number
}

interface DecisionPlan {
  next_step_type?: string
  next_step_name?: string
  execution_mode?: string
  activities?: DecisionActivity[]
  reasoning?: string
  confidence?: number
  evidence?: unknown[]
  tool_calls?: unknown[]
  knowledge_hits?: unknown[]
  action_executions?: unknown[]
  risk_flags?: unknown[]
}

function useAssignSchema() {
  const { t } = useTranslation("itsm")
  return z.object({
    assigneeId: z.number().min(1, t("validation.assigneeRequired")),
  })
}

function useCancelSchema() {
  const { t } = useTranslation("itsm")
  return z.object({
    reason: z.string().min(1, t("validation.reasonRequired")),
  })
}

function useApprovalSchema() {
  return z.object({
    opinion: z.string(),
  })
}

function getNodeOutcomes(activityType: string): ApprovalOutcome[] {
  if (activityType === "form") {
    return ["approved"]
  }
  return ["approved", "rejected"]
}

function parseDecision(activity?: ActivityItem | null): DecisionPlan | null {
  if (!activity?.aiDecision) return null
  try {
    return JSON.parse(activity.aiDecision) as DecisionPlan
  } catch {
    return null
  }
}

function confidenceOf(activity?: ActivityItem | null, plan?: DecisionPlan | null) {
  if (activity?.aiConfidence != null) return activity.aiConfidence
  if (plan?.confidence != null) return plan.confidence
  return null
}

function compactValue(value: unknown) {
  if (value == null || value === "") return "—"
  if (typeof value === "string" || typeof value === "number" || typeof value === "boolean") return String(value)
  try {
    return JSON.stringify(value)
  } catch {
    return String(value)
  }
}

function toRecord(value: unknown) {
  return value != null && typeof value === "object" && !Array.isArray(value)
    ? value as Record<string, unknown>
    : null
}

interface FieldDisplayMeta {
  label?: string
  valueLabels: Record<string, string>
}

function parseFieldDisplayMeta(schema: unknown) {
  return parseTicketFieldDisplayMeta(schema) as Record<string, FieldDisplayMeta>
}

function formatDate(value?: string | null) {
  return value ? new Date(value).toLocaleString() : "—"
}

function formatDateCompact(value?: string | null) {
  if (!value) return "—"
  const d = new Date(value)
  if (Number.isNaN(d.getTime())) return "—"
  const pad = (n: number) => String(n).padStart(2, "0")
  return `${d.getFullYear()}/${d.getMonth() + 1}/${d.getDate()} ${pad(d.getHours())}:${pad(d.getMinutes())}`
}

const GENERIC_STEP_TERMS = new Set([
  "处理",
  "process",
  "step",
  "node",
  "activity",
  "步骤",
  "节点",
  "活动",
])

function normalizeStepText(value: string) {
  return value.trim().toLowerCase().replace(/[\s_-]+/g, "")
}

function isGenericStepText(value?: string | null) {
  if (!value) return false
  return GENERIC_STEP_TERMS.has(normalizeStepText(value))
}

function mapStepTypeToLabel(stepType?: string | null) {
  if (!stepType) return null
  const normalized = normalizeStepText(stepType)
  if (normalized === "process") return "等待人工处理"
  if (normalized === "处理") return "等待人工审核"
  if (normalized === "form" || normalized === "表单") return "等待补充信息"
  if (normalized === "approve" || normalized === "审批") return "等待审批决策"
  if (normalized === "action" || normalized === "动作") return "执行自动化动作"
  return null
}

function mapStatusToSuggestion(status?: string | null, smartState?: string | null) {
  const normalizedStatus = normalizeStepText(status ?? "")
  const normalizedSmartState = normalizeStepText(smartState ?? "")
  if (normalizedSmartState === "waitinghuman" || normalizedStatus === "waitinghuman") return "等待人工审核"
  return null
}

function resolveFieldLabel(
  key: string,
  fieldMeta: Record<string, FieldDisplayMeta>,
  t: (key: string, options?: Record<string, unknown>) => string,
  locale: string,
) {
  return resolveTicketFieldLabel(key, fieldMeta, t, locale)
}

function resolveFieldOptionLabel(
  fieldKey: string,
  rawValue: unknown,
  fieldMeta: Record<string, FieldDisplayMeta>,
  t: (key: string, options?: Record<string, unknown>) => string,
  locale: string,
) {
  return resolveTicketFieldDisplayValue(fieldKey, rawValue, fieldMeta, t, locale)
}

function resolveFieldDisplayValue(
  fieldKey: string,
  rawValue: unknown,
  fieldMeta: Record<string, FieldDisplayMeta>,
  t: (key: string, options?: Record<string, unknown>) => string,
  locale: string,
) {
  return resolveTicketFieldDisplayValue(fieldKey, rawValue, fieldMeta, t, locale)
  if (Array.isArray(rawValue)) {
    const separator = locale.startsWith("zh") ? "、" : ", "
    return (rawValue as unknown[])
      .map((item: unknown) => {
        if (typeof item === "string" || typeof item === "number" || typeof item === "boolean") {
          return resolveFieldOptionLabel(fieldKey, item, fieldMeta, t, locale)
        }
        return compactValue(item)
      })
      .join(separator)
  }
  if (typeof rawValue === "string" || typeof rawValue === "number" || typeof rawValue === "boolean") {
    return resolveFieldOptionLabel(fieldKey, rawValue, fieldMeta, t, locale)
  }
  return compactValue(rawValue)
}

void [toRecord, parseFieldDisplayMeta, resolveFieldLabel, resolveFieldDisplayValue]

function summarizeDecision(
  plan: DecisionPlan | null,
  fallback?: string | null,
  activityName?: string | null,
  context?: { activityType?: string | null; ticketStatus?: string | null; smartState?: string | null },
) {
  const first = plan?.activities?.[0]
  const candidates = [first?.instructions, plan?.next_step_name, activityName, fallback]
  let mappedFromGenericCandidate: string | null = null
  for (const candidate of candidates) {
    if (!candidate) continue
    const trimmed = candidate.trim()
    if (!trimmed) continue
    if (isGenericStepText(trimmed)) {
      mappedFromGenericCandidate = mappedFromGenericCandidate ?? mapStepTypeToLabel(trimmed)
      continue
    }
    return trimmed
  }
  const mapped = mappedFromGenericCandidate
    ?? mapStepTypeToLabel(first?.type || plan?.next_step_type)
    ?? mapStepTypeToLabel(context?.activityType)
    ?? mapStepTypeToLabel(fallback)
  if (mapped) return mapped
  const statusMapped = mapStatusToSuggestion(context?.ticketStatus, context?.smartState)
  if (statusMapped) return statusMapped
  return "等待系统给出下一步"
}

function ownerName(ticket: TicketItem, activity?: ActivityItem | null) {
  if (ticket.currentOwnerName) return ticket.currentOwnerName
  if (ticket.assigneeName) return ticket.assigneeName
  if (activity?.activityType === "action") return "自动化动作"
  return "AI 智能引擎"
}

function decisioningMessageForOutcome(outcome?: string) {
  switch (outcome) {
    case "approved":
      return "已通过当前审批，后台决策中。"
    case "rejected":
      return "已驳回当前审批，后台决策中。"
    default:
      return "已处理，后台决策中。"
  }
}

function factSource(ticket: TicketItem, t: (key: string) => string) {
  return ticket.source === "agent" ? t("itsm:tickets.sourceAgent") : t("itsm:tickets.sourceCatalog")
}

function DetailItem({ label, value, title }: { label: string; value: ReactNode; title?: string }) {
  return (
    <div className="space-y-1.5" title={title}>
      <p className="text-xs font-medium text-muted-foreground">{label}</p>
      <div className="truncate whitespace-nowrap text-sm leading-6 text-foreground">{value}</div>
    </div>
  )
}

function DecisionButtonContent({ icon: Icon, children }: { icon: LucideIcon; children: ReactNode }) {
  return (
    <span className="inline-flex w-full items-center justify-center gap-1.5 text-center text-[11px] leading-none">
      <Icon className="h-3.5 w-3.5 shrink-0" />
      <span className="truncate font-medium">{children}</span>
    </span>
  )
}

function outcomeLabel(activity: ActivityItem, outcome: string, t: (key: string) => string) {
  if (activity.activityType === "form" && outcome === "approved") return t("itsm:tickets.submit")
  if (activity.activityType === "process" && outcome === "approved") return t("itsm:tickets.complete")
  if (activity.activityType === "process" && outcome === "rejected") return t("itsm:tickets.reject")
  if (outcome === "approved") return t("itsm:tickets.approve")
  if (outcome === "rejected") return t("itsm:tickets.reject")
  return `${activity.name}: ${outcome}`
}

function EvidenceList({ title, items }: { title: string; items?: unknown[] }) {
  if (!items?.length) return null
  return (
    <div className="space-y-2">
      <p className="text-sm font-medium">{title}</p>
      <div className="grid gap-2 md:grid-cols-2">
        {items.slice(0, 6).map((item, idx) => (
          <div key={idx} className="rounded-lg border border-border/50 bg-background/45 p-3 text-xs leading-5 text-muted-foreground">
            {compactValue(item)}
          </div>
        ))}
      </div>
    </div>
  )
}

function RequestFieldValue({
  label,
  model,
}: {
  label: string
  model: FieldDisplayModel
}) {
  const [tableOpen, setTableOpen] = useState(false)

  if (model.kind === "tags") {
    return (
      <div className="mt-1.5 flex min-h-[32px] flex-wrap content-start gap-2">
        {model.values.length > 0 ? model.values.map((value) => (
          <Badge key={`${label}-${value}`} variant="secondary" className="rounded-full px-2.5 py-1 text-xs font-medium">
            {value}
          </Badge>
        )) : (
          <span className="text-sm text-muted-foreground">-</span>
        )}
      </div>
    )
  }

  if (model.kind === "long_text") {
    if (!model.expandable) {
      return (
        <p className="mt-1.5 whitespace-pre-wrap text-sm font-medium leading-6 text-foreground">
          {model.value}
        </p>
      )
    }

    return (
      <details className="mt-1.5 rounded-xl bg-muted/35 px-3 py-2">
        <summary className="cursor-pointer list-none text-xs font-medium text-muted-foreground">
          查看详情
        </summary>
        <p className="mt-2 whitespace-pre-wrap text-sm leading-6 text-foreground">
          {model.value}
        </p>
      </details>
    )
  }

  if (model.kind === "table") {
    return (
      <Collapsible open={tableOpen} onOpenChange={setTableOpen}>
        <div className="mt-2 rounded-2xl bg-muted/30">
          <div className="flex flex-col gap-2 px-4 py-3 md:flex-row md:items-start md:justify-between">
            <div className="min-w-0">
              <p className="text-xs font-medium text-muted-foreground">{model.summaryLabel}</p>
              <p className="mt-1 text-sm leading-6 text-foreground">
                {model.summary}
              </p>
            </div>
            <CollapsibleTrigger asChild>
              <Button type="button" variant="ghost" size="sm" className="h-8 w-fit self-start gap-1.5 px-2 text-xs">
                查看明细
                <ChevronDown className={`h-3.5 w-3.5 transition-transform ${tableOpen ? "rotate-180" : ""}`} />
              </Button>
            </CollapsibleTrigger>
          </div>

          <CollapsibleContent className="border-t border-border/35 px-4 py-3">
            <div className="hidden md:block">
              <Table>
                <TableHeader>
                  <TableRow>
                    {model.columns.map((column) => (
                      <TableHead key={`${label}-${column.key}`}>{column.label}</TableHead>
                    ))}
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {model.rows.map((row, rowIndex) => (
                    <TableRow key={`${label}-row-${rowIndex}`}>
                      {model.columns.map((column) => {
                        const cell = row.cells.find((item) => item.key === column.key)
                        return (
                          <TableCell key={`${label}-${rowIndex}-${column.key}`} className="align-top text-sm leading-6">
                            <span className="whitespace-pre-wrap break-words">{cell?.value ?? "-"}</span>
                          </TableCell>
                        )
                      })}
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            </div>

            <div className="space-y-3 md:hidden">
              {model.rows.map((row, rowIndex) => (
                <div key={`${label}-mobile-${rowIndex}`} className="rounded-xl border border-border/45 bg-background/55 p-3">
                  <p className="text-xs font-medium text-muted-foreground">记录 {rowIndex + 1}</p>
                  <div className="mt-2 grid gap-2">
                    {row.cells.map((cell) => (
                      <div key={`${label}-mobile-${rowIndex}-${cell.key}`} className="rounded-lg bg-muted/35 px-3 py-2">
                        <p className="text-[11px] font-medium text-muted-foreground">{cell.label}</p>
                        <p className="mt-1 whitespace-pre-wrap break-words text-sm leading-6 text-foreground">
                          {cell.value}
                        </p>
                      </div>
                    ))}
                  </div>
                </div>
              ))}
            </div>
          </CollapsibleContent>
        </div>
      </Collapsible>
    )
  }

  return (
    <p className="mt-1.5 text-sm font-medium leading-6 text-foreground">
      {model.value}
    </p>
  )
}

function isSimpleFieldModel(model: FieldDisplayModel) {
  if (model.kind === "table") return false
  if (model.kind === "long_text") return !model.expandable
  return true
}

function chunkSimpleFieldEntries<T>(entries: T[]) {
  const rows: T[][] = []
  for (let index = 0; index < entries.length; index += 2) {
    rows.push(entries.slice(index, index + 2))
  }
  return rows
}

function RequestFieldCell({
  label,
  model,
}: {
  label: string
  model: FieldDisplayModel
}) {
  return (
    <div className="flex min-h-[88px] flex-col justify-start px-1 py-1">
      <p className="min-h-[16px] text-[11px] font-medium tracking-[0.08em] text-muted-foreground/90">{label}</p>
      <RequestFieldValue label={label} model={model} />
    </div>
  )
}

function RequestFieldBlock({
  label,
  model,
}: {
  label: string
  model: FieldDisplayModel
}) {
  return (
    <div className="rounded-2xl bg-background/40 px-4 py-3">
      <p className="min-h-[16px] text-[11px] font-medium tracking-[0.08em] text-muted-foreground/90">{label}</p>
      {model.kind === "long_text" && model.expandable ? (
        <>
          <p className="mt-1.5 line-clamp-4 whitespace-pre-wrap text-sm font-medium leading-6 text-foreground">{model.value}</p>
          <RequestFieldValue label={label} model={model} />
        </>
      ) : (
        <RequestFieldValue label={label} model={model} />
      )}
    </div>
  )
}

function RequestFieldsPanel({
  ticket,
  activity,
}: {
  ticket: TicketItem
  activity?: ActivityItem | null
}) {
  const { t, i18n } = useTranslation("itsm")
  const record = toDisplayRecord(activity?.formData) ?? toDisplayRecord(ticket.formData)
  if (!record) return null

  const fieldMeta = {
    ...parseTicketFieldDisplayMeta(ticket.intakeFormSchema),
    ...parseTicketFieldDisplayMeta(activity?.formSchema),
  } satisfies Record<string, TicketFieldDisplayMeta>
  const locale = i18n.resolvedLanguage || i18n.language || "zh-CN"
  const schemaSections = parseFieldDisplaySections(activity?.formSchema)
  const fallbackSections = schemaSections.length > 0 ? schemaSections : parseFieldDisplaySections(ticket.intakeFormSchema)
  const remainingKeys = new Set(Object.keys(record))
  const sections: Array<{ title?: string; description?: string; collapsible?: boolean; fields: string[] }> = []
  for (const section of fallbackSections) {
    const fields = section.fields.filter((field) => remainingKeys.has(field))
    fields.forEach((field) => remainingKeys.delete(field))
    if (fields.length > 0) {
      sections.push({ ...section, fields })
    }
  }

  if (remainingKeys.size > 0) {
    sections.push({
      title: locale.startsWith("zh") ? "补充字段" : "Additional fields",
      fields: [...remainingKeys],
    })
  }

  const effectiveSections = sections.length > 0
    ? sections
    : [{
      title: locale.startsWith("zh") ? "申请内容" : "Request payload",
      fields: Object.keys(record),
    }]

  return (
    <section className="workspace-surface rounded-[1.1rem] p-5">
      <div className="flex flex-col gap-3 border-b border-border/45 pb-4 md:flex-row md:items-start md:justify-between">
        <div className="min-w-0">
          <div className="flex items-center gap-2 text-base font-semibold">
            <FileText className="h-4 w-4" />
            申请内容
          </div>
          <p className="mt-1 text-sm text-muted-foreground">
            按表单结构回看申请信息，复杂字段支持展开查看完整明细。
          </p>
        </div>
        <Badge variant="outline" className="h-fit rounded-full px-3 py-1 text-xs">
          {Object.keys(record).length} 个字段
        </Badge>
      </div>

      <div className="mt-5 space-y-5">
        {effectiveSections.map((section, sectionIndex) => (
          <div key={`${section.title ?? "section"}-${sectionIndex}`} className="space-y-4">
            <div className="flex flex-col gap-1 border-b border-border/25 pb-3">
              <p className="text-sm font-semibold">{section.title ?? "申请内容"}</p>
              {section.description && (
                <p className="text-xs leading-5 text-muted-foreground">{section.description}</p>
              )}
            </div>
            {(() => {
              const entries = section.fields.map((fieldKey) => ({
                fieldKey,
                label: resolveTicketFieldLabel(fieldKey, fieldMeta, t, locale),
                model: resolveFieldDisplayModel(fieldKey, record[fieldKey], fieldMeta, t, locale),
              }))

              const blocks: ReactNode[] = []
              let bufferedSimpleEntries: typeof entries = []

              const flushSimpleEntries = () => {
                if (bufferedSimpleEntries.length === 0) return
                const rows = chunkSimpleFieldEntries(bufferedSimpleEntries)
                blocks.push(
                  <div key={`${section.title ?? "section"}-${sectionIndex}-simple`} className="space-y-3">
                    {rows.map((row, rowIndex) => (
                      <div key={`${section.title ?? "section"}-${sectionIndex}-row-${rowIndex}`} className="grid gap-x-6 gap-y-3 md:grid-cols-2">
                        {row.map((entry) => (
                          <RequestFieldCell key={entry.fieldKey} label={entry.label} model={entry.model} />
                        ))}
                      </div>
                    ))}
                  </div>,
                )
                bufferedSimpleEntries = []
              }

              for (const entry of entries) {
                if (isSimpleFieldModel(entry.model)) {
                  bufferedSimpleEntries.push(entry)
                  continue
                }
                flushSimpleEntries()
                blocks.push(
                  <RequestFieldBlock
                    key={entry.fieldKey}
                    label={entry.label}
                    model={entry.model}
                  />,
                )
              }

              flushSimpleEntries()
              return <div className="space-y-3">{blocks}</div>
            })()}
          </div>
        ))}
      </div>
    </section>
  )
}

function AIEvidencePanel({
  ticket,
  activity,
  plan,
}: {
  ticket: TicketItem
  activity?: ActivityItem | null
  plan: DecisionPlan | null
}) {
  useTranslation("itsm")
  const hasFormRecord = Boolean(toDisplayRecord(activity?.formData) ?? toDisplayRecord(ticket.formData))
  const confidence = confidenceOf(activity, plan)
  const confidencePct = confidence == null ? null : Math.round(confidence * 100)
  const firstActivity = plan?.activities?.[0]
  const nextStepSuggestion = summarizeDecision(
    plan,
    ticket.nextStepSummary,
    activity?.name,
    { activityType: activity?.activityType, ticketStatus: ticket.status, smartState: ticket.smartState },
  )

  return (
    <section className="workspace-surface rounded-[1.1rem] p-5">
      <div className="flex items-center gap-2 text-base font-semibold">
          <Bot className="h-4 w-4" />
          AI 依据
      </div>

      <div className="mt-4 grid gap-x-8 gap-y-4 border-b border-border/45 pb-4 md:grid-cols-3">
        <DetailItem
          label="判断依据"
          value={activity?.aiReasoning ? "AI 已记录推理摘要" : hasFormRecord ? "申请字段与运行轨迹" : "流程运行轨迹"}
        />
        <DetailItem label="下一步建议" value={nextStepSuggestion} title={nextStepSuggestion} />
        <div className="space-y-1.5">
          <div className="inline-flex items-center gap-1 text-xs font-medium text-muted-foreground">
            <span>置信度</span>
            {activity?.aiReasoning && (
              <Popover>
                <PopoverTrigger asChild>
                  <button
                    type="button"
                    aria-label="查看推理摘要"
                    className="inline-flex h-4 w-4 items-center justify-center rounded-full text-muted-foreground transition-colors hover:bg-muted hover:text-foreground"
                  >
                    <Info className="h-3.5 w-3.5" />
                  </button>
                </PopoverTrigger>
                <PopoverContent align="end" className="w-[22rem] max-w-[85vw] p-3">
                  <p className="text-xs font-medium text-foreground">推理摘要</p>
                  <p className="mt-2 max-h-60 overflow-y-auto whitespace-pre-wrap pr-1 text-xs leading-5 text-muted-foreground">
                    {activity.aiReasoning}
                  </p>
                </PopoverContent>
              </Popover>
            )}
          </div>
          <div className="truncate whitespace-nowrap text-sm leading-6 text-foreground">
            {confidencePct == null ? "—" : `${confidencePct}%`}
          </div>
        </div>
      </div>

      <div className="mt-4 space-y-5">
        {confidencePct != null && (
          <div className="space-y-2">
            <div className="text-xs text-muted-foreground">置信度</div>
            <Progress value={confidencePct} className="h-2" />
          </div>
        )}

        {firstActivity && (
          <div className="grid gap-x-8 gap-y-4 border-t border-border/45 pt-4 md:grid-cols-4">
            <DetailItem label="步骤类型" value={firstActivity.type || plan?.next_step_type || "—"} />
            <DetailItem label="执行模式" value={plan?.execution_mode || "single"} />
            <DetailItem label="参与者" value={firstActivity.participant_name || firstActivity.participant_type || firstActivity.participant_id || "—"} />
            <DetailItem label="动作 ID" value={firstActivity.action_id || "—"} />
          </div>
        )}

        <EvidenceList title="知识库命中" items={plan?.knowledge_hits ?? activity?.knowledgeHits} />
        <EvidenceList title="工具调用" items={plan?.tool_calls ?? activity?.toolCalls} />
        <EvidenceList title="动作执行" items={plan?.action_executions ?? activity?.actionExecutions} />
        <EvidenceList title="风险标记" items={plan?.risk_flags ?? activity?.riskFlags} />

        {!activity?.aiReasoning && !plan && !hasFormRecord && (
          <p className="text-sm text-muted-foreground">暂无结构化 AI 证据，先查看流程轨迹与审计时间线。</p>
        )}
      </div>
    </section>
  )
}

function TimelinePanel({ timeline }: { timeline: TimelineItem[] }) {
  const { t } = useTranslation(["itsm", "common"])
  return (
    <section className="workspace-surface rounded-[1.1rem] p-5">
      <h3 className="text-base font-semibold">{t("itsm:tickets.timeline")}</h3>
      <div className="mt-4">
        {timeline.length === 0 ? (
          <p className="text-sm text-muted-foreground">{t("itsm:tickets.empty")}</p>
        ) : (
          <div className="relative space-y-0">
            {timeline.map((event, idx) => {
              const style = TIMELINE_EVENT_STYLE[event.eventType] ?? DEFAULT_EVENT_STYLE
              const Icon = style.icon
              return (
                <div key={event.id} className="flex gap-3 pb-5 last:pb-0">
                  <div className="flex flex-col items-center">
                    <div className={`flex h-6 w-6 items-center justify-center rounded-full ${style.bg}`}>
                      <Icon className={`h-3 w-3 ${style.fg}`} />
                    </div>
                    {idx < timeline.length - 1 && <div className="w-px flex-1 bg-border" />}
                  </div>
                  <div className="min-w-0 flex-1 pt-0.5">
                    <p className="text-sm">{event.content}</p>
                    <p className="mt-0.5 text-xs text-muted-foreground">
                      {event.operatorName} · {formatDate(event.createdAt)}
                    </p>
                    {event.eventType === "ai_decision_action_executed" && event.details != null && (() => {
                      const d = toRecord(event.details) ?? toRecord(
                        (() => { try { return JSON.parse(event.details as unknown as string) } catch { return null } })()
                      )
                      if (!d) return null
                      const httpStatus = d["http_status"] as number | undefined
                      const respPreview = d["response_preview"] as string | undefined
                      const errMsg = d["error"] as string | undefined
                      if (!httpStatus && !respPreview && !errMsg) return null
                      return (
                        <details className="mt-1.5">
                          <summary className="cursor-pointer text-xs text-muted-foreground hover:text-foreground">
                            {httpStatus ? `HTTP ${httpStatus}` : "执行详情"}
                          </summary>
                          <div className="mt-1 rounded-md bg-muted/50 p-2.5 text-xs leading-relaxed space-y-1">
                            {httpStatus != null && <p><span className="font-medium">状态码：</span>{httpStatus}</p>}
                            {errMsg && <p className="text-red-600"><span className="font-medium">错误：</span>{errMsg}</p>}
                            {respPreview && (
                              <p className="break-all whitespace-pre-wrap"><span className="font-medium">响应：</span>{respPreview}</p>
                            )}
                          </div>
                        </details>
                      )
                    })()}
                    {event.reasoning && (
                      <details className="mt-1.5">
                        <summary className="cursor-pointer text-xs text-muted-foreground hover:text-foreground">
                          {t("itsm:smart.reasoning")}
                        </summary>
                        <p className="mt-1 whitespace-pre-wrap rounded-md bg-muted/50 p-2.5 text-xs leading-relaxed">
                          {event.reasoning}
                        </p>
                      </details>
                    )}
                  </div>
                </div>
              )
            })}
          </div>
        )}
      </div>
    </section>
  )
}

function CompactEmpty({ text }: { text: string }) {
  return (
    <section className="workspace-surface rounded-[1.1rem] p-5 text-sm text-muted-foreground">
      {text}
    </section>
  )
}

function ticketSummaryText(ticket: TicketItem) {
  return ticket.description || ticket.title
}

function conciseSLA(ticket: TicketItem, t: (key: string) => string) {
  const map: Record<string, string> = {
    on_track: t("itsm:tickets.slaOnTrack"),
    breached_response: t("itsm:tickets.slaBreachedResponse"),
    breached_resolution: t("itsm:tickets.slaBreachedResolution"),
    normal: t("itsm:tickets.slaNormal"),
    warning: t("itsm:tickets.slaWarning"),
    breached: t("itsm:tickets.slaBreached"),
  }
  return map[ticket.slaStatus] ?? ticket.slaStatus ?? "—"
}

function SummaryChip({ children }: { children: ReactNode }) {
  return (
    <div className="inline-flex items-center gap-2 rounded-full border border-border/55 bg-background/35 px-3 py-1 text-xs font-medium text-muted-foreground">
      {children}
    </div>
  )
}

function DividerMeta({ label, value, title }: { label: string; value: ReactNode; title?: string }) {
  return (
    <div className="min-w-0 space-y-1" title={title}>
      <p className="truncate whitespace-nowrap text-[11px] text-muted-foreground">{label}</p>
      <p className="truncate whitespace-nowrap text-sm font-medium tracking-[-0.01em]">{value}</p>
    </div>
  )
}

function SummaryBand({
  ticket,
  owner,
  nextStep,
  t,
}: {
  ticket: TicketItem
  owner: string
  nextStep: string
  t: (key: string) => string
}) {
  const summary = ticketSummaryText(ticket)
  return (
    <section className="workspace-surface rounded-[1.2rem] p-5">
      <div className="flex flex-wrap items-center justify-between gap-3 border-b border-border/45 pb-3">
        <SummaryChip>
          <ShieldCheck className="h-3.5 w-3.5" />
          处置摘要
        </SummaryChip>
        <p className="text-sm">
          <span className="text-muted-foreground">当前责任方</span>
          <span className="ml-2 font-semibold">{owner}</span>
        </p>
      </div>

      <div className="grid gap-4 border-b border-border/45 py-4 lg:grid-cols-[minmax(0,1fr)_minmax(220px,0.45fr)]">
        <DividerMeta label="工单诉求" value={summary} title={summary} />
        <DividerMeta label="下一步" value={nextStep} title={nextStep} />
      </div>

      <div className="grid gap-x-5 gap-y-3 pt-4 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-6">
        <DividerMeta label={t("itsm:tickets.service")} value={ticket.serviceName} title={ticket.serviceName} />
        <DividerMeta
          label={t("itsm:tickets.priority")}
          value={(
            <span className="inline-flex min-w-0 items-center gap-1.5">
              <span className="inline-block h-2.5 w-2.5 shrink-0 rounded-full" style={{ backgroundColor: ticket.priorityColor }} />
              <span className="truncate">{ticket.priorityName}</span>
            </span>
          )}
          title={ticket.priorityName}
        />
        <DividerMeta label={t("itsm:tickets.requester")} value={ticket.requesterName} title={ticket.requesterName} />
        <DividerMeta label={t("itsm:tickets.source")} value={factSource(ticket, t)} title={factSource(ticket, t)} />
        <DividerMeta label={t("itsm:tickets.createdAt")} value={formatDateCompact(ticket.createdAt)} title={formatDate(ticket.createdAt)} />
        <DividerMeta label={t("itsm:tickets.slaStatus")} value={conciseSLA(ticket, t)} title={conciseSLA(ticket, t)} />
      </div>
    </section>
  )
}

function FlatAside({
  ticket,
  owner,
  confidencePct,
  decisioningMessage,
  isDecisioning,
  isActive,
  displayHumanActivity,
  selectedActionableActivity,
  actionableActivities,
  selectedActionableActivityId,
  setSelectedActionableActivityId,
  isCurrentUserResponsible,
  progressPending,
  openApprovalSheet,
  getNodeOutcomes,
  outcomeLabel,
  t,
  canProcess,
  canAssign,
  assignForm,
  setAssignOpen,
  canCancel,
  cancelForm,
  setCancelOpen,
  canWithdraw,
  withdrawForm,
  setWithdrawOpen,
}: {
  ticket: TicketItem
  owner: string
  confidencePct: number | null
  decisioningMessage: string
  isDecisioning: boolean
  isActive: boolean
  displayHumanActivity: ActivityItem | undefined
  selectedActionableActivity: ActivityItem | undefined
  actionableActivities: ActivityItem[]
  selectedActionableActivityId: number | null
  setSelectedActionableActivityId: (activityId: number) => void
  isCurrentUserResponsible: boolean
  progressPending: boolean
  openApprovalSheet: (activityId: number, outcome: ApprovalOutcome) => void
  getNodeOutcomes: (activityType: string) => ApprovalOutcome[]
  outcomeLabel: (activity: ActivityItem, outcome: string, t: (key: string) => string) => string
  t: (key: string) => string
  canProcess: boolean
  canAssign: boolean
  assignForm: ReturnType<typeof useForm<{ assigneeId: number }>>
  setAssignOpen: (open: boolean) => void
  canCancel: boolean
  cancelForm: ReturnType<typeof useForm<{ reason: string }>>
  setCancelOpen: (open: boolean) => void
  canWithdraw: boolean
  withdrawForm: ReturnType<typeof useForm<{ reason: string }>>
  setWithdrawOpen: (open: boolean) => void
}) {
  return (
    <aside className="workspace-surface rounded-[1.2rem] p-4">
      <div className="flex items-center justify-between gap-3 border-b border-border/45 pb-3">
        <h3 className="inline-flex items-center gap-2 text-base font-semibold">
          <CheckCircle2 className="h-4 w-4" />
          处置栏
        </h3>
        {confidencePct != null && (
          <Badge variant={confidencePct >= 80 ? "default" : confidencePct >= 50 ? "secondary" : "destructive"} className="h-6 px-2 text-[11px]">
            {t("itsm:smart.confidence")} {confidencePct}%
          </Badge>
        )}
      </div>

      <div className="grid grid-cols-2 gap-x-4 gap-y-3 py-3 text-sm">
        <DetailItem label="责任人" value={owner} title={owner} />
        <DetailItem label="SLA 风险" value={<SLABadge slaStatus={ticket.slaStatus} slaResolutionDeadline={ticket.slaResolutionDeadline} />} />
        <DetailItem label={t("itsm:tickets.slaResponseDeadline")} value={formatDateCompact(ticket.slaResponseDeadline)} title={formatDate(ticket.slaResponseDeadline)} />
        <DetailItem label={t("itsm:tickets.slaResolutionDeadline")} value={formatDateCompact(ticket.slaResolutionDeadline)} title={formatDate(ticket.slaResolutionDeadline)} />
      </div>

      <div className="grid grid-cols-2 gap-2 border-t border-border/50 pt-3 [&_[data-slot=button]]:h-8 [&_[data-slot=button]]:text-xs">
        {isDecisioning && (
          <p className="col-span-2 inline-flex items-center gap-2 rounded-lg border border-sky-200 bg-sky-50 p-3 text-sm text-sky-800">
            <Loader2 className="h-4 w-4 animate-spin" />
            {decisioningMessage}
          </p>
        )}

        {canProcess && actionableActivities.length > 1 && (
          <div className="col-span-2 space-y-1.5">
            <p className="text-xs font-medium text-muted-foreground">待处理节点</p>
            <Select
              value={selectedActionableActivityId ? String(selectedActionableActivityId) : ""}
              onValueChange={(value) => setSelectedActionableActivityId(Number(value))}
              disabled={progressPending}
            >
              <SelectTrigger className="h-8 text-xs">
                <SelectValue placeholder="选择处理节点" />
              </SelectTrigger>
              <SelectContent>
                {actionableActivities.map((activity) => (
                  <SelectItem key={activity.id} value={String(activity.id)}>
                    {activity.name || activity.activityType || `#${activity.id}`}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>
        )}

        {canProcess && isActive && !isDecisioning && selectedActionableActivity && isCurrentUserResponsible && getNodeOutcomes(selectedActionableActivity.activityType).map((outcome) => (
          <Button
            data-testid={outcome === "approved" ? "itsm-ticket-approve-button" : "itsm-ticket-reject-button"}
            key={`${selectedActionableActivity.id}-${outcome}`}
            size="sm"
            className={outcome === "rejected" ? "w-full text-destructive" : "w-full"}
            variant={outcome === "approved" ? "default" : "outline"}
            disabled={progressPending}
            onClick={() => openApprovalSheet(selectedActionableActivity.id, outcome)}
          >
            <DecisionButtonContent icon={outcome === "approved" ? CheckCircle2 : CircleX}>{outcomeLabel(selectedActionableActivity, outcome, t)}</DecisionButtonContent>
          </Button>
        ))}

        {isActive && !isDecisioning && canAssign && (
          <Button size="sm" variant="outline" className="w-full" onClick={() => { assignForm.reset({ assigneeId: ticket.assigneeId ?? 0 }); setAssignOpen(true) }}>
            <DecisionButtonContent icon={UserPlus}>{t("itsm:tickets.assign")}</DecisionButtonContent>
          </Button>
        )}

        {isActive && !isDecisioning && canCancel && (
          <Button size="sm" variant="outline" className="w-full text-destructive" onClick={() => { cancelForm.reset({ reason: "" }); setCancelOpen(true) }}>
            <DecisionButtonContent icon={CircleX}>{t("itsm:tickets.cancel")}</DecisionButtonContent>
          </Button>
        )}

        {canWithdraw && (
          <Button size="sm" variant="outline" className="w-full" onClick={() => { withdrawForm.reset({ reason: "" }); setWithdrawOpen(true) }}>
            <DecisionButtonContent icon={RotateCcw}>{t("itsm:tickets.withdraw")}</DecisionButtonContent>
          </Button>
        )}

        {isActive && !isDecisioning && displayHumanActivity && !isCurrentUserResponsible && (
          <p className="col-span-2 rounded-lg border border-border/50 bg-background/35 p-3 text-sm text-muted-foreground">
            当前步骤正在等待责任人处理，你可以查看依据和审计记录。
          </p>
        )}
      </div>
    </aside>
  )
}

function renderFlowTab(
  ticket: TicketItem,
  activities: ActivityItem[],
  tokens: unknown[],
  t: (key: string) => string,
) {
  if (ticket.engineType === "smart") {
    return null
  }
  if (ticket.workflowJson) {
    return (
      <section className="workspace-surface rounded-[1.1rem] p-5">
        <h3 className="text-base font-semibold">{t("itsm:workflow.viewer.workflowGraph")}</h3>
        <div className="mt-4 overflow-visible">
          <WorkflowViewer
            workflowJson={ticket.workflowJson}
            activities={activities}
            tokens={tokens as never[]}
            currentActivityId={ticket.currentActivityId}
          />
        </div>
      </section>
    )
  }
  return <CompactEmpty text="暂无流程图。" />
}

export function Component() {
  const { t } = useTranslation(["itsm", "common"])
  const { id } = useParams<{ id: string }>()
  const navigate = useNavigate()
  const queryClient = useQueryClient()
  const ticketId = Number(id)
  const [assignOpen, setAssignOpen] = useState(false)
  const [cancelOpen, setCancelOpen] = useState(false)
  const [withdrawOpen, setWithdrawOpen] = useState(false)
  const [approvalOpen, setApprovalOpen] = useState(false)
  const [approvalOutcome, setApprovalOutcome] = useState<ApprovalOutcome>("approved")
  const [approvalActivityId, setApprovalActivityId] = useState<number | null>(null)
  const [selectedActionableActivityId, setSelectedActionableActivityId] = useState<number | null>(null)
  const [decisioningMessage, setDecisioningMessage] = useState(DEFAULT_DECISIONING_MESSAGE)

  const canAssign = usePermission("itsm:ticket:assign")
  const canCancel = usePermission("itsm:ticket:cancel")
  const currentUser = useAuthStore((s) => s.user)
  const currentUserId = currentUser?.id ?? 0

  const assignSchema = useAssignSchema()
  const cancelSchema = useCancelSchema()
  const approvalSchema = useApprovalSchema()

  const { data: ticket, isLoading } = useQuery({
    queryKey: ["itsm-ticket", ticketId],
    queryFn: () => fetchTicket(ticketId),
    enabled: ticketId > 0,
  })

  const { data: timeline = [] } = useQuery({
    queryKey: ["itsm-ticket-timeline", ticketId],
    queryFn: () => fetchTicketTimeline(ticketId),
    enabled: ticketId > 0,
  })

  const { data: activities = [] } = useQuery({
    queryKey: ["itsm-ticket-activities", ticketId],
    queryFn: () => fetchTicketActivities(ticketId),
    enabled: ticketId > 0,
  })

  const { data: tokens = [] } = useQuery({
    queryKey: ["itsm-ticket-tokens", ticketId],
    queryFn: () => fetchTicketTokens(ticketId),
    enabled: ticketId > 0 && ticket?.engineType === "classic" && Boolean(ticket.workflowJson),
  })

  const { data: users = [] } = useQuery({
    queryKey: ["users-for-assign"],
    queryFn: () => fetchUsers(),
    enabled: assignOpen,
  })

  const assignForm = useForm<{ assigneeId: number }>({
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    resolver: zodResolver(assignSchema as any),
    defaultValues: { assigneeId: 0 },
  })

  const cancelForm = useForm<{ reason: string }>({
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    resolver: zodResolver(cancelSchema as any),
    defaultValues: { reason: "" },
  })

  const withdrawForm = useForm<{ reason: string }>({
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    resolver: zodResolver(cancelSchema as any),
    defaultValues: { reason: "" },
  })

  const approvalForm = useForm<{ opinion: string }>({
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    resolver: zodResolver(approvalSchema as any),
    defaultValues: { opinion: "" },
  })

  const invalidateTicketDetail = () => {
    queryClient.invalidateQueries({ queryKey: ["itsm-ticket", ticketId] })
    queryClient.invalidateQueries({ queryKey: ["itsm-ticket-timeline", ticketId] })
    queryClient.invalidateQueries({ queryKey: ["itsm-ticket-activities", ticketId] })
  }

  const invalidateTicketLists = () => {
    queryClient.invalidateQueries({ queryKey: ["itsm-ticket-monitor"] })
    queryClient.invalidateQueries({ queryKey: ["itsm-tickets-mine"] })
    queryClient.invalidateQueries({ queryKey: ["itsm-ticket-approval-pending"] })
    queryClient.invalidateQueries({ queryKey: ["itsm-ticket-approval-history"] })
  }

  const invalidateTicket = () => {
    invalidateTicketDetail()
    invalidateTicketLists()
  }

  const markSmartDecisioning = (message: string) => {
    setDecisioningMessage(message)
    queryClient.setQueryData<TicketItem>(["itsm-ticket", ticketId], (current) => {
      if (!current || current.engineType !== "smart") return current
      const rejected = message.includes("驳回")
      return {
        ...current,
        assigneeId: null,
        assigneeName: "",
        canAct: false,
        currentActivityId: null,
        currentOwnerName: "AI 智能引擎",
        currentOwnerType: "ai",
        nextStepSummary: "后台决策中",
        smartState: "ai_reasoning",
        status: rejected ? "rejected_decisioning" : "approved_decisioning",
        statusLabel: rejected ? "已驳回，决策中" : "已同意，决策中",
        statusTone: "progress",
      }
    })
  }

  const assignMut = useMutation({
    mutationFn: (v: { assigneeId: number }) => assignTicket(ticketId, v.assigneeId),
    onSuccess: () => {
      invalidateTicket()
      setAssignOpen(false)
      toast.success(t("itsm:tickets.assignSuccess"))
    },
    onError: (err) => toast.error(err.message),
  })

  const cancelMut = useMutation({
    mutationFn: (v: { reason: string }) => cancelTicket(ticketId, v.reason),
    onSuccess: () => {
      invalidateTicket()
      setCancelOpen(false)
      toast.success(t("itsm:tickets.cancelSuccess"))
    },
    onError: (err) => toast.error(err.message),
  })

  const withdrawMut = useMutation({
    mutationFn: (v: { reason: string }) => withdrawTicket(ticketId, v.reason),
    onSuccess: () => {
      invalidateTicket()
      setWithdrawOpen(false)
      toast.success(t("itsm:tickets.withdrawSuccess"))
    },
    onError: (err) => toast.error(err.message),
  })

  const progressMut = useMutation({
    mutationFn: (data: { activityId: number; outcome: ApprovalOutcome; opinion: string }) => progressTicket(ticketId, data),
    onMutate: (data) => {
      markSmartDecisioning(decisioningMessageForOutcome(data.outcome))
      setApprovalOpen(false)
      setApprovalActivityId(null)
      approvalForm.reset({ opinion: "" })
    },
    onSuccess: () => {
      invalidateTicket()
      toast.success(t("itsm:tickets.progressSuccess"))
    },
    onError: (err) => {
      invalidateTicketDetail()
      toast.error(err.message)
    },
  })

  const openApprovalSheet = (activityId: number, outcome: ApprovalOutcome) => {
    setApprovalActivityId(activityId)
    setApprovalOutcome(outcome)
    approvalForm.reset({ opinion: "" })
    setApprovalOpen(true)
  }

  useEffect(() => {
    if (ticket?.engineType !== "smart" || ticket.smartState !== "ai_reasoning") return

    const interval = window.setInterval(() => {
      queryClient.invalidateQueries({ queryKey: ["itsm-ticket", ticketId] })
      queryClient.invalidateQueries({ queryKey: ["itsm-ticket-timeline", ticketId] })
      queryClient.invalidateQueries({ queryKey: ["itsm-ticket-activities", ticketId] })
    }, 60000)
    return () => window.clearInterval(interval)
  }, [queryClient, ticket?.engineType, ticket?.smartState, ticketId])

  // When timeline reports a terminal event but ticket status hasn't caught up yet,
  // force an immediate refetch of the ticket query to close the sync gap.
  useEffect(() => {
    if (!ticket || TERMINAL_STATUSES.has(ticket.status)) return
    const hasTerminalEvent = timeline.some(
      (e) => e.eventType === "workflow_completed" || e.eventType === "ticket_cancelled",
    )
    if (hasTerminalEvent) {
      queryClient.invalidateQueries({ queryKey: ["itsm-ticket", ticketId] })
    }
  }, [queryClient, ticket, ticketId, timeline])

  const actionContext = buildTicketActionContext({
    ticket,
    activities,
    currentUserId,
    canAssignPermission: canAssign,
    canCancelPermission: canCancel,
  })

  const currentActivity = ticket ? activities.find((a) => a.id === ticket.currentActivityId) : undefined
  const activeHumanActivity = actionContext.displayHumanActivity
  const selectedActionableActivity = selectedActionableActivityId
    ? actionContext.actionableActivities.find((activity) => activity.id === selectedActionableActivityId)
    : undefined
  const actionableActivity = selectedActionableActivity ?? actionContext.selectedActionableActivity
  const explanationActivity = currentActivity ?? [...activities].reverse().find((a) => a.aiDecision || a.aiReasoning)
  const plan = parseDecision(explanationActivity)
  const confidence = confidenceOf(explanationActivity, plan)
  const confidencePct = confidence == null ? null : Math.round(confidence * 100)
  const isActive = actionContext.isActive
  const isDecisioning = actionContext.isDecisioning
  const isCurrentUserResponsible = Boolean(actionableActivity?.canAct)
  const nextStep = ticket
    ? (isDecisioning ? t("itsm:tickets.statusDecisioning") : summarizeDecision(
      plan,
      ticket.nextStepSummary,
      actionableActivity?.name,
      { activityType: actionableActivity?.activityType, ticketStatus: ticket.status, smartState: ticket.smartState },
    ))
    : ""
  const owner = ticket ? ownerName(ticket, actionableActivity) : "—"

  if (isLoading) {
    return (
      <div className="flex items-center justify-center py-20">
        <div className="text-muted-foreground">{t("common:loading")}</div>
      </div>
    )
  }

  if (!ticket) {
    return (
      <div className="flex items-center justify-center py-20">
        <div className="text-muted-foreground">{t("itsm:tickets.empty")}</div>
      </div>
    )
  }

  return (
    <div className="workspace-page">
      <div className="workspace-page-header">
        <div className="flex min-w-0 items-start gap-3">
          <Button variant="ghost" size="icon" onClick={() => navigate(-1)} aria-label="返回">
            <ArrowLeft className="h-4 w-4" />
          </Button>
          <div className="min-w-0">
            <div className="flex flex-wrap items-center gap-2">
              <h2 className="workspace-page-title truncate">{ticket.code}</h2>
              <TicketStatusBadge ticket={ticket} />
              <Badge variant="outline">{ticket.engineType === "smart" ? "智能工单" : "经典流程"}</Badge>
            </div>
          </div>
        </div>
      </div>

      <div className="grid gap-4 xl:grid-cols-[minmax(0,1fr)_300px]">
        <main className="min-w-0 space-y-4">
          <SummaryBand ticket={ticket} owner={owner} nextStep={nextStep} t={t} />
          <AIEvidencePanel ticket={ticket} activity={explanationActivity} plan={plan} />
          <RequestFieldsPanel ticket={ticket} activity={explanationActivity} />
          <TimelinePanel timeline={timeline} />
          {renderFlowTab(ticket, activities, tokens, t)}
        </main>

        <div className="xl:sticky xl:top-4 xl:self-start">
          <FlatAside
            ticket={ticket}
            owner={owner}
            confidencePct={confidencePct}
            decisioningMessage={decisioningMessage}
            isDecisioning={isDecisioning}
            isActive={isActive}
            displayHumanActivity={activeHumanActivity}
            selectedActionableActivity={actionableActivity}
            actionableActivities={actionContext.actionableActivities}
            selectedActionableActivityId={actionableActivity?.id ?? null}
            setSelectedActionableActivityId={setSelectedActionableActivityId}
            isCurrentUserResponsible={isCurrentUserResponsible}
            progressPending={progressMut.isPending}
            openApprovalSheet={openApprovalSheet}
            getNodeOutcomes={getNodeOutcomes}
            outcomeLabel={outcomeLabel}
            t={t}
            canProcess={actionContext.canProcess}
            canAssign={actionContext.canAssign}
            assignForm={assignForm}
            setAssignOpen={setAssignOpen}
            canCancel={actionContext.canCancel}
            cancelForm={cancelForm}
            setCancelOpen={setCancelOpen}
            canWithdraw={actionContext.canWithdraw}
            withdrawForm={withdrawForm}
            setWithdrawOpen={setWithdrawOpen}
          />
        </div>
      </div>

      <Sheet open={assignOpen} onOpenChange={setAssignOpen}>
        <SheetContent className="sm:max-w-md">
          <SheetHeader>
            <SheetTitle>{t("itsm:tickets.assign")}</SheetTitle>
            <SheetDescription className="sr-only">{t("itsm:tickets.assign")}</SheetDescription>
          </SheetHeader>
          <Form {...assignForm}>
            <form onSubmit={assignForm.handleSubmit((v) => assignMut.mutate(v))} className="flex flex-1 flex-col gap-5 px-4">
              <FormField control={assignForm.control} name="assigneeId" render={({ field }) => (
                <FormItem>
                  <FormLabel>{t("itsm:tickets.assignee")}</FormLabel>
                  <Select onValueChange={(v) => field.onChange(Number(v))} value={field.value ? String(field.value) : ""}>
                    <FormControl><SelectTrigger><SelectValue placeholder={t("itsm:tickets.assigneePlaceholder")} /></SelectTrigger></FormControl>
                    <SelectContent>
                      {users.map((u) => (
                        <SelectItem key={u.id} value={String(u.id)}>{u.username}</SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                  <FormMessage />
                </FormItem>
              )} />
              <SheetFooter>
                <Button type="submit" size="sm" disabled={assignMut.isPending}>
                  {assignMut.isPending ? t("common:saving") : t("itsm:tickets.assign")}
                </Button>
              </SheetFooter>
            </form>
          </Form>
        </SheetContent>
      </Sheet>

      <Sheet open={approvalOpen} onOpenChange={setApprovalOpen}>
        <SheetContent className="sm:max-w-md">
          <SheetHeader>
            <SheetTitle>{approvalOutcome === "approved" ? t("itsm:tickets.approve") : t("itsm:tickets.reject")}</SheetTitle>
            <SheetDescription>{t("itsm:tickets.approvalOpinionDesc")}</SheetDescription>
          </SheetHeader>
          <Form {...approvalForm}>
            <form
              onSubmit={approvalForm.handleSubmit((v) => {
                if (!approvalActivityId) return
                progressMut.mutate({
                  activityId: approvalActivityId,
                  outcome: approvalOutcome,
                  opinion: v.opinion,
                })
              })}
              className="flex flex-1 flex-col gap-5 px-4"
            >
              <FormField control={approvalForm.control} name="opinion" render={({ field }) => (
                <FormItem>
                  <FormLabel>{t("itsm:tickets.approvalOpinion")}</FormLabel>
                  <FormControl>
                    <Textarea rows={4} placeholder={t("itsm:tickets.approvalOpinionPlaceholder")} {...field} />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )} />
              <SheetFooter>
                <Button
                  data-testid={approvalOutcome === "approved" ? "itsm-ticket-confirm-approve-button" : "itsm-ticket-confirm-reject-button"}
                  type="submit"
                  size="sm"
                  variant={approvalOutcome === "approved" ? "default" : "destructive"}
                  disabled={progressMut.isPending}
                >
                  {progressMut.isPending ? t("common:saving") : approvalOutcome === "approved" ? t("itsm:tickets.confirmApprove") : t("itsm:tickets.confirmReject")}
                </Button>
              </SheetFooter>
            </form>
          </Form>
        </SheetContent>
      </Sheet>

      <Sheet open={cancelOpen} onOpenChange={setCancelOpen}>
        <SheetContent className="sm:max-w-md">
          <SheetHeader>
            <SheetTitle>{t("itsm:tickets.cancelTitle")}</SheetTitle>
            <SheetDescription className="sr-only">{t("itsm:tickets.cancelTitle")}</SheetDescription>
          </SheetHeader>
          <Form {...cancelForm}>
            <form onSubmit={cancelForm.handleSubmit((v) => cancelMut.mutate(v))} className="flex flex-1 flex-col gap-5 px-4">
              <FormField control={cancelForm.control} name="reason" render={({ field }) => (
                <FormItem>
                  <FormLabel>{t("itsm:tickets.cancelReason")}</FormLabel>
                  <FormControl><Textarea rows={3} placeholder={t("itsm:tickets.cancelReasonPlaceholder")} {...field} /></FormControl>
                  <FormMessage />
                </FormItem>
              )} />
              <SheetFooter>
                <Button type="submit" size="sm" variant="destructive" disabled={cancelMut.isPending}>
                  {cancelMut.isPending ? t("common:saving") : t("itsm:tickets.confirmCancel")}
                </Button>
              </SheetFooter>
            </form>
          </Form>
        </SheetContent>
      </Sheet>

      <Sheet open={withdrawOpen} onOpenChange={setWithdrawOpen}>
        <SheetContent className="sm:max-w-md">
          <SheetHeader>
            <SheetTitle>{t("itsm:tickets.withdrawTitle")}</SheetTitle>
            <SheetDescription className="sr-only">{t("itsm:tickets.withdrawTitle")}</SheetDescription>
          </SheetHeader>
          <Form {...withdrawForm}>
            <form onSubmit={withdrawForm.handleSubmit((v) => withdrawMut.mutate(v))} className="flex flex-1 flex-col gap-5 px-4">
              <FormField control={withdrawForm.control} name="reason" render={({ field }) => (
                <FormItem>
                  <FormLabel>{t("itsm:tickets.withdrawReason")}</FormLabel>
                  <FormControl><Textarea rows={3} placeholder={t("itsm:tickets.withdrawReasonPlaceholder")} {...field} /></FormControl>
                  <FormMessage />
                </FormItem>
              )} />
              <SheetFooter>
                <Button type="submit" size="sm" variant="destructive" disabled={withdrawMut.isPending}>
                  {withdrawMut.isPending ? t("common:saving") : t("itsm:tickets.confirmWithdraw")}
                </Button>
              </SheetFooter>
            </form>
          </Form>
        </SheetContent>
      </Sheet>
    </div>
  )
}
