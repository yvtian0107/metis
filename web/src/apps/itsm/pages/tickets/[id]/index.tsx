"use client"

import { useState, type ReactNode } from "react"
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
  CheckCircle,
  CheckCircle2,
  CircleX,
  Clock,
  FileText,
  Play,
  PlusCircle,
  RotateCcw,
  ShieldCheck,
  UserPlus,
  XCircle,
  type LucideIcon,
} from "lucide-react"
import { toast } from "sonner"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
  AlertDialogTrigger,
} from "@/components/ui/alert-dialog"
import {
  Card,
  CardContent,
  CardHeader,
  CardTitle,
} from "@/components/ui/card"
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
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs"
import { Textarea } from "@/components/ui/textarea"
import { Progress } from "@/components/ui/progress"
import { usePermission } from "@/hooks/use-permission"
import { useAuthStore } from "@/stores/auth"
import {
  assignTicket,
  cancelTicket,
  completeTicket,
  confirmActivity,
  fetchTicket,
  fetchTicketActivities,
  fetchTicketTimeline,
  fetchTicketTokens,
  fetchUsers,
  progressTicket,
  rejectActivity,
  type ActivityItem,
  type TicketItem,
  type TimelineItem,
} from "../../../api"
import { OverrideActions } from "../../../components/override-actions"
import { SLABadge } from "../../../components/sla-badge"
import { SmartFlowVisualization } from "../../../components/smart-flow-visualization"
import { VariablesPanel } from "../../../components/variables-panel"
import { WorkflowViewer } from "../../../components/workflow"

const ACTIVE_STATUSES = new Set(["pending", "in_progress", "waiting_approval", "waiting_action"])
const TERMINAL_STATUSES = new Set(["completed", "cancelled", "failed"])
const HUMAN_ACTIVITY_TYPES = new Set(["approve", "form", "process"])

const STATUS_MAP: Record<string, { variant: "default" | "secondary" | "destructive" | "outline"; key: string }> = {
  pending: { variant: "secondary", key: "statusPending" },
  in_progress: { variant: "default", key: "statusInProgress" },
  waiting_approval: { variant: "outline", key: "statusWaitingApproval" },
  waiting_action: { variant: "outline", key: "statusWaitingAction" },
  completed: { variant: "default", key: "statusCompleted" },
  failed: { variant: "destructive", key: "statusFailed" },
  cancelled: { variant: "secondary", key: "statusCancelled" },
}

const DEFAULT_EVENT_STYLE = { icon: Clock, bg: "bg-muted", fg: "text-muted-foreground" }
const TIMELINE_EVENT_STYLE: Record<string, { icon: LucideIcon; bg: string; fg: string }> = {
  ticket_created:        { icon: PlusCircle, bg: "bg-blue-100", fg: "text-blue-600" },
  ticket_assigned:       { icon: UserPlus, bg: "bg-blue-100", fg: "text-blue-600" },
  ticket_completed:      { icon: CheckCircle, bg: "bg-green-100", fg: "text-green-600" },
  ticket_cancelled:      { icon: XCircle, bg: "bg-gray-200", fg: "text-gray-500" },
  workflow_started:      { icon: Play, bg: "bg-blue-100", fg: "text-blue-600" },
  workflow_completed:    { icon: CheckCircle, bg: "bg-green-100", fg: "text-green-600" },
  activity_completed:    { icon: CheckCircle, bg: "bg-green-100", fg: "text-green-600" },
  activity_approved:     { icon: CheckCircle, bg: "bg-green-100", fg: "text-green-600" },
  activity_denied:       { icon: XCircle, bg: "bg-red-100", fg: "text-red-600" },
  ai_decision_pending:   { icon: Bot, bg: "bg-amber-100", fg: "text-amber-600" },
  ai_decision_executed:  { icon: Bot, bg: "bg-green-100", fg: "text-green-600" },
  ai_decision_confirmed: { icon: CheckCircle, bg: "bg-green-100", fg: "text-green-600" },
  ai_decision_rejected:  { icon: XCircle, bg: "bg-red-100", fg: "text-red-600" },
  ai_decision_failed:    { icon: AlertTriangle, bg: "bg-red-100", fg: "text-red-600" },
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

function getNodeOutcomes(activityType: string): string[] {
  switch (activityType) {
    case "form": return ["submitted"]
    case "approve": return ["approved", "rejected"]
    case "process": return ["completed"]
    default: return ["completed"]
  }
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

function formatDate(value?: string | null) {
  return value ? new Date(value).toLocaleString() : "—"
}

function summarizeDecision(plan: DecisionPlan | null, fallback?: string | null) {
  const first = plan?.activities?.[0]
  if (first?.instructions) return first.instructions
  if (plan?.next_step_name) return plan.next_step_name
  if (first?.type) return `${first.type}${plan?.execution_mode ? ` · ${plan.execution_mode}` : ""}`
  return plan?.next_step_type || fallback || "等待系统给出下一步"
}

function ownerName(ticket: TicketItem, activity?: ActivityItem | null) {
  if (ticket.currentOwnerName) return ticket.currentOwnerName
  if (ticket.assigneeName) return ticket.assigneeName
  if (activity?.status === "pending_approval") return "AI 决策确认人"
  if (activity?.activityType === "action") return "自动化动作"
  return "AI 智能引擎"
}

function factSource(ticket: TicketItem, t: (key: string) => string) {
  return ticket.source === "agent" ? t("itsm:tickets.sourceAgent") : t("itsm:tickets.sourceCatalog")
}

function FactItem({ label, value }: { label: string; value: ReactNode }) {
  return (
    <div className="min-w-0 rounded-lg border border-border/50 bg-background/35 px-3 py-2">
      <p className="text-[11px] text-muted-foreground">{label}</p>
      <div className="mt-1 truncate text-sm font-medium">{value}</div>
    </div>
  )
}

function SectionBlock({ label, value }: { label: string; value: ReactNode }) {
  return (
    <div className="rounded-lg border border-border/50 bg-background/35 p-4">
      <p className="text-xs font-medium text-muted-foreground">{label}</p>
      <div className="mt-2 text-sm leading-6 text-foreground">{value}</div>
    </div>
  )
}

function DecisionButtonContent({ icon: Icon, children }: { icon: LucideIcon; children: ReactNode }) {
  return (
    <span className="grid w-[5.25rem] grid-cols-[0.875rem_minmax(0,1fr)] items-center gap-2 text-left text-[11px] leading-none">
      <Icon className="h-3.5 w-3.5 shrink-0 justify-self-center" />
      <span className="truncate font-medium">{children}</span>
    </span>
  )
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

function AIEvidencePanel({
  ticket,
  activity,
  plan,
}: {
  ticket: TicketItem
  activity?: ActivityItem | null
  plan: DecisionPlan | null
}) {
  const formRecord = toRecord(ticket.formData)
  const activityFormRecord = toRecord(activity?.formData)
  const confidence = confidenceOf(activity, plan)
  const confidencePct = confidence == null ? null : Math.round(confidence * 100)
  const firstActivity = plan?.activities?.[0]

  return (
    <Card>
      <CardHeader>
        <CardTitle className="flex items-center gap-2 text-base">
          <Bot className="h-4 w-4" />
          AI 依据
        </CardTitle>
      </CardHeader>
      <CardContent className="space-y-5">
        <div className="grid gap-3 md:grid-cols-3">
          <SectionBlock
            label="判断依据"
            value={activity?.aiReasoning ? "AI 已记录推理摘要" : formRecord ? "申请字段与运行轨迹" : "流程运行轨迹"}
          />
          <SectionBlock label="下一步建议" value={summarizeDecision(plan, ticket.nextStepSummary)} />
          <SectionBlock label="置信度" value={confidencePct == null ? "—" : `${confidencePct}%`} />
        </div>

        {confidencePct != null && (
          <div className="rounded-lg border border-border/50 bg-background/35 p-4">
            <div className="mb-2 flex items-center justify-between text-xs text-muted-foreground">
              <span>置信度边界</span>
              <span>{confidencePct >= 80 ? "可自动推进" : confidencePct >= 50 ? "建议观察" : "需要人工确认"}</span>
            </div>
            <Progress value={confidencePct} className="h-2" />
          </div>
        )}

        {activity?.aiReasoning && (
          <div className="space-y-2">
            <p className="text-sm font-medium">推理摘要</p>
            <p className="whitespace-pre-wrap rounded-lg border border-border/50 bg-background/45 p-4 text-sm leading-6 text-muted-foreground">
              {activity.aiReasoning}
            </p>
          </div>
        )}

        {firstActivity && (
          <div className="grid gap-3 md:grid-cols-4">
            <SectionBlock label="步骤类型" value={firstActivity.type || plan?.next_step_type || "—"} />
            <SectionBlock label="执行模式" value={plan?.execution_mode || "single"} />
            <SectionBlock label="参与者" value={firstActivity.participant_name || firstActivity.participant_type || firstActivity.participant_id || "—"} />
            <SectionBlock label="动作 ID" value={firstActivity.action_id || "—"} />
          </div>
        )}

        {(formRecord || activityFormRecord) && (
          <div className="space-y-2">
            <p className="text-sm font-medium">申请字段</p>
            <div className="grid gap-2 md:grid-cols-2">
              {Object.entries(activityFormRecord ?? formRecord ?? {}).slice(0, 10).map(([key, value]) => (
                <div key={key} className="rounded-lg border border-border/50 bg-background/45 p-3 text-xs">
                  <span className="text-muted-foreground">{key}</span>
                  <p className="mt-1 truncate font-medium">{compactValue(value)}</p>
                </div>
              ))}
            </div>
          </div>
        )}

        <EvidenceList title="知识库命中" items={plan?.knowledge_hits ?? activity?.knowledgeHits} />
        <EvidenceList title="工具调用" items={plan?.tool_calls ?? activity?.toolCalls} />
        <EvidenceList title="动作执行" items={plan?.action_executions ?? activity?.actionExecutions} />
        <EvidenceList title="风险标记" items={plan?.risk_flags ?? activity?.riskFlags} />

        {!activity?.aiReasoning && !plan && !formRecord && (
          <p className="text-sm text-muted-foreground">暂无结构化 AI 证据，先查看流程轨迹与审计时间线。</p>
        )}
      </CardContent>
    </Card>
  )
}

function TimelinePanel({ timeline }: { timeline: TimelineItem[] }) {
  const { t } = useTranslation(["itsm", "common"])

  return (
    <Card>
      <CardHeader>
        <CardTitle className="text-base">{t("itsm:tickets.timeline")}</CardTitle>
      </CardHeader>
      <CardContent>
        {timeline.length === 0 ? (
          <p className="text-sm text-muted-foreground">{t("itsm:tickets.empty")}</p>
        ) : (
          <div className="relative space-y-0">
            {timeline.map((event, idx) => {
              const style = TIMELINE_EVENT_STYLE[event.eventType] ?? DEFAULT_EVENT_STYLE
              const Icon = style.icon
              return (
                <div key={event.id} className="flex gap-3 pb-6 last:pb-0">
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
      </CardContent>
    </Card>
  )
}

export function Component() {
  const { t } = useTranslation(["itsm", "common"])
  const { id } = useParams<{ id: string }>()
  const navigate = useNavigate()
  const queryClient = useQueryClient()
  const ticketId = Number(id)
  const [assignOpen, setAssignOpen] = useState(false)
  const [cancelOpen, setCancelOpen] = useState(false)
  const [rejectAiOpen, setRejectAiOpen] = useState(false)

  const canAssign = usePermission("itsm:ticket:assign")
  const canComplete = usePermission("itsm:ticket:complete")
  const canCancel = usePermission("itsm:ticket:cancel")
  const canOverride = usePermission("itsm:ticket:override")
  const currentUser = useAuthStore((s) => s.user)
  const currentUserId = currentUser?.id ?? 0

  const assignSchema = useAssignSchema()
  const cancelSchema = useCancelSchema()

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
    enabled: ticketId > 0,
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

  const invalidateTicket = () => {
    queryClient.invalidateQueries({ queryKey: ["itsm-ticket", ticketId] })
    queryClient.invalidateQueries({ queryKey: ["itsm-ticket-timeline", ticketId] })
    queryClient.invalidateQueries({ queryKey: ["itsm-ticket-activities", ticketId] })
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

  const completeMut = useMutation({
    mutationFn: () => completeTicket(ticketId),
    onSuccess: () => {
      invalidateTicket()
      toast.success(t("itsm:tickets.completeSuccess"))
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

  const progressMut = useMutation({
    mutationFn: (data: { activityId: number; outcome: string }) => progressTicket(ticketId, data),
    onSuccess: () => {
      invalidateTicket()
      toast.success(t("itsm:tickets.progressSuccess"))
    },
    onError: (err) => toast.error(err.message),
  })

  const confirmAiMut = useMutation({
    mutationFn: (activityId: number) => confirmActivity(ticketId, activityId),
    onSuccess: () => {
      invalidateTicket()
      toast.success(t("itsm:smart.confirmSuccess"))
    },
    onError: (err) => toast.error(err.message),
  })

  const rejectAiMut = useMutation({
    mutationFn: (activityId: number) => rejectActivity(ticketId, activityId, t("itsm:smart.humanRejected")),
    onSuccess: () => {
      invalidateTicket()
      setRejectAiOpen(false)
      toast.success(t("itsm:smart.rejectSuccess"))
    },
    onError: (err) => toast.error(err.message),
  })

  const currentActivity = ticket ? activities.find((a) => a.id === ticket.currentActivityId) : undefined
  const activeHumanActivity = activities.find(
    (a) => ["pending", "in_progress"].includes(a.status) && HUMAN_ACTIVITY_TYPES.has(a.activityType),
  )
  const explanationActivity = currentActivity ?? [...activities].reverse().find((a) => a.aiDecision || a.aiReasoning)
  const plan = parseDecision(explanationActivity)
  const confidence = confidenceOf(explanationActivity, plan)
  const confidencePct = confidence == null ? null : Math.round(confidence * 100)
  const isActive = ticket ? ACTIVE_STATUSES.has(ticket.status) : false
  const isTerminal = ticket ? TERMINAL_STATUSES.has(ticket.status) : false
  const statusInfo = ticket ? (STATUS_MAP[ticket.status] ?? { variant: "secondary" as const, key: "statusPending" }) : null
  const isAiConfirmation = currentActivity?.status === "pending_approval"
  const actionableActivity = isAiConfirmation ? currentActivity : activeHumanActivity
  const isCurrentUserResponsible = Boolean(
    ticket?.canAct || actionableActivity?.canAct || (ticket?.assigneeId && ticket.assigneeId === currentUserId),
  )
  const nextStep = ticket ? summarizeDecision(plan, ticket.nextStepSummary || actionableActivity?.name) : ""
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
              {statusInfo && <Badge variant={statusInfo.variant}>{t(`itsm:tickets.${statusInfo.key}`)}</Badge>}
              <Badge variant="outline">{ticket.engineType === "smart" ? "智能工单" : "经典流程"}</Badge>
            </div>
            <p className="workspace-page-description">{ticket.title}</p>
          </div>
        </div>
      </div>

      <div className="grid gap-4 xl:grid-cols-[minmax(0,1fr)_360px]">
        <main className="min-w-0 space-y-4">
          <section className="workspace-surface rounded-[1.25rem] p-5">
            <div className="flex flex-wrap items-center justify-between gap-3">
              <div className="inline-flex items-center gap-2 rounded-full border border-border/60 bg-background/45 px-3 py-1 text-xs font-medium text-muted-foreground">
                <ShieldCheck className="h-3.5 w-3.5" />
                审批摘要
              </div>
              <div className="text-sm">
                <span className="text-muted-foreground">当前责任方</span>
                <span className="ml-2 font-semibold">{owner}</span>
              </div>
            </div>

            <div className="mt-4 grid gap-3 lg:grid-cols-[minmax(0,1fr)_minmax(240px,0.55fr)]">
              <SectionBlock
                label="工单诉求"
                value={ticket.description ? <span className="whitespace-pre-wrap">{ticket.description}</span> : ticket.title}
              />
              <SectionBlock label="下一步判断" value={nextStep} />
            </div>

            <div className="mt-4 grid gap-3 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-6">
              <FactItem label={t("itsm:tickets.service")} value={ticket.serviceName} />
              <FactItem
                label={t("itsm:tickets.priority")}
                value={(
                  <span className="inline-flex min-w-0 items-center gap-1.5">
                    <span className="inline-block h-2.5 w-2.5 shrink-0 rounded-full" style={{ backgroundColor: ticket.priorityColor }} />
                    <span className="truncate">{ticket.priorityName}</span>
                  </span>
                )}
              />
              <FactItem label={t("itsm:tickets.requester")} value={ticket.requesterName} />
              <FactItem label={t("itsm:tickets.source")} value={factSource(ticket, t)} />
              <FactItem label={t("itsm:tickets.createdAt")} value={formatDate(ticket.createdAt)} />
              <FactItem label={t("itsm:tickets.slaStatus")} value={<SLABadge slaStatus={ticket.slaStatus} slaResolutionDeadline={ticket.slaResolutionDeadline} />} />
            </div>
          </section>

          <Tabs defaultValue="ai" className="space-y-3">
            <TabsList className="workspace-surface rounded-xl p-1.5" variant="default">
              <TabsTrigger value="ai">
                <Bot className="h-4 w-4" />
                AI 依据
              </TabsTrigger>
              <TabsTrigger value="flow">
                <ShieldCheck className="h-4 w-4" />
                流程轨迹
              </TabsTrigger>
              <TabsTrigger value="variables">
                <FileText className="h-4 w-4" />
                变量
              </TabsTrigger>
              <TabsTrigger value="timeline">
                <Clock className="h-4 w-4" />
                时间线
              </TabsTrigger>
            </TabsList>

            <TabsContent value="ai">
              <AIEvidencePanel ticket={ticket} activity={explanationActivity} plan={plan} />
            </TabsContent>

            <TabsContent value="flow">
              {ticket.engineType === "smart" ? (
                activities.length > 0 ? (
                  <SmartFlowVisualization activities={activities} currentActivityId={ticket.currentActivityId} />
                ) : (
                  <Card>
                    <CardContent className="py-8 text-sm text-muted-foreground">暂无活动记录。</CardContent>
                  </Card>
                )
              ) : ticket.workflowJson ? (
                <Card>
                  <CardHeader>
                    <CardTitle className="text-base">{t("itsm:workflow.viewer.workflowGraph")}</CardTitle>
                  </CardHeader>
                  <CardContent className="overflow-visible">
                    <WorkflowViewer
                      workflowJson={ticket.workflowJson}
                      activities={activities}
                      tokens={tokens}
                      currentActivityId={ticket.currentActivityId}
                    />
                  </CardContent>
                </Card>
              ) : (
                <Card>
                  <CardContent className="py-8 text-sm text-muted-foreground">暂无流程图。</CardContent>
                </Card>
              )}
            </TabsContent>

            <TabsContent value="variables">
              <VariablesPanel ticketId={ticketId} />
            </TabsContent>

            <TabsContent value="timeline">
              <TimelinePanel timeline={timeline} />
            </TabsContent>
          </Tabs>
        </main>

        <aside className="xl:sticky xl:top-4 xl:self-start">
          <Card className="py-5">
            <CardHeader className="px-5 pb-1">
              <CardTitle className="flex items-center justify-between gap-3 text-base">
                <span className="inline-flex items-center gap-2">
                  <CheckCircle2 className="h-4 w-4" />
                  决策栏
                </span>
                {confidencePct != null && (
                  <Badge variant={confidencePct >= 80 ? "default" : confidencePct >= 50 ? "secondary" : "destructive"} className="h-6 px-2 text-[11px]">
                    {t("itsm:smart.confidence")} {confidencePct}%
                  </Badge>
                )}
              </CardTitle>
            </CardHeader>
            <CardContent className="space-y-4 px-5">
              <div className="grid grid-cols-2 gap-x-4 gap-y-3">
                <div className="text-sm">
                  <span className="text-muted-foreground">责任人</span>
                  <p className="mt-1 font-medium">{owner}</p>
                </div>
                <div className="text-sm">
                  <span className="text-muted-foreground">SLA 风险</span>
                  <div className="mt-1">
                    <SLABadge slaStatus={ticket.slaStatus} slaResolutionDeadline={ticket.slaResolutionDeadline} />
                  </div>
                </div>
                <div className="text-sm">
                  <span className="text-muted-foreground">{t("itsm:tickets.slaResponseDeadline")}</span>
                  <p className="mt-1 text-xs">{formatDate(ticket.slaResponseDeadline)}</p>
                </div>
                <div className="text-sm">
                  <span className="text-muted-foreground">{t("itsm:tickets.slaResolutionDeadline")}</span>
                  <p className="mt-1 text-xs">{formatDate(ticket.slaResolutionDeadline)}</p>
                </div>
              </div>

              <div className="grid grid-cols-2 gap-2 border-t border-border/50 pt-3 [&_[data-slot=button]]:h-8 [&_[data-slot=button]]:text-xs">
                {isActive && isAiConfirmation && currentActivity && isCurrentUserResponsible && (
                  <>
                    <Button size="sm" className="w-full" onClick={() => confirmAiMut.mutate(currentActivity.id)} disabled={confirmAiMut.isPending || rejectAiMut.isPending}>
                      <DecisionButtonContent icon={CheckCircle2}>{t("itsm:smart.confirm")}</DecisionButtonContent>
                    </Button>
                    <Button size="sm" variant="outline" className="w-full text-destructive" onClick={() => setRejectAiOpen(true)} disabled={confirmAiMut.isPending || rejectAiMut.isPending}>
                      <DecisionButtonContent icon={CircleX}>{t("itsm:smart.reject")}</DecisionButtonContent>
                    </Button>
                  </>
                )}

                {isActive && !isAiConfirmation && activeHumanActivity && isCurrentUserResponsible && getNodeOutcomes(activeHumanActivity.activityType).map((outcome) => (
                  <Button
                    key={`${activeHumanActivity.id}-${outcome}`}
                    size="sm"
                    className="w-full"
                    variant={outcome === "rejected" || outcome === "failed" ? "outline" : "default"}
                    disabled={progressMut.isPending}
                    onClick={() => progressMut.mutate({ activityId: activeHumanActivity.id, outcome })}
                  >
                    <DecisionButtonContent icon={outcome === "rejected" ? CircleX : CheckCircle2}>
                      {activeHumanActivity.activityType === "approve"
                        ? outcome === "rejected" ? t("itsm:approval.deny") : t("itsm:approval.approve")
                        : `${activeHumanActivity.name}: ${outcome}`}
                    </DecisionButtonContent>
                  </Button>
                ))}

                {isActive && canAssign && (
                  <Button size="sm" variant="outline" className="w-full" onClick={() => { assignForm.reset({ assigneeId: ticket.assigneeId ?? 0 }); setAssignOpen(true) }}>
                    <DecisionButtonContent icon={UserPlus}>{t("itsm:tickets.assign")}</DecisionButtonContent>
                  </Button>
                )}

                {isActive && ticket.engineType === "smart" && canOverride && (
                  <div>
                    <OverrideActions
                      ticketId={ticketId}
                      currentActivityId={ticket.currentActivityId}
                      aiFailureCount={ticket.aiFailureCount}
                      triggerClassName="h-8 w-full text-xs"
                    />
                  </div>
                )}

                {isActive && canComplete && (
                  <AlertDialog>
                    <AlertDialogTrigger asChild>
                      <Button size="sm" variant="outline" className="w-full">
                        <DecisionButtonContent icon={CheckCircle}>{t("itsm:tickets.complete")}</DecisionButtonContent>
                      </Button>
                    </AlertDialogTrigger>
                    <AlertDialogContent>
                      <AlertDialogHeader>
                        <AlertDialogTitle>{t("itsm:tickets.complete")}</AlertDialogTitle>
                        <AlertDialogDescription>{t("itsm:tickets.code")}: {ticket.code}</AlertDialogDescription>
                      </AlertDialogHeader>
                      <AlertDialogFooter>
                        <AlertDialogCancel size="sm">{t("common:cancel")}</AlertDialogCancel>
                        <AlertDialogAction size="sm" onClick={() => completeMut.mutate()} disabled={completeMut.isPending}>{t("itsm:tickets.complete")}</AlertDialogAction>
                      </AlertDialogFooter>
                    </AlertDialogContent>
                  </AlertDialog>
                )}

                {isActive && canCancel && (
                  <Button size="sm" variant="outline" className="w-full text-destructive" onClick={() => { cancelForm.reset({ reason: "" }); setCancelOpen(true) }}>
                    <DecisionButtonContent icon={CircleX}>{t("itsm:tickets.cancel")}</DecisionButtonContent>
                  </Button>
                )}

                {(!isActive || isTerminal) && (
                  <p className="col-span-2 rounded-lg border border-border/50 bg-background/35 p-3 text-sm text-muted-foreground">
                    当前工单不可继续审批，证据区保留完整流程与审计记录。
                  </p>
                )}

                {isActive && actionableActivity && !isCurrentUserResponsible && (
                  <p className="col-span-2 rounded-lg border border-border/50 bg-background/35 p-3 text-sm text-muted-foreground">
                    当前步骤正在等待责任人处理，你可以查看依据和审计记录。
                  </p>
                )}
              </div>
            </CardContent>
          </Card>
        </aside>
      </div>

      <AlertDialog open={rejectAiOpen} onOpenChange={setRejectAiOpen}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>{t("itsm:smart.rejectConfirmTitle", { defaultValue: "拒绝 AI 决策" })}</AlertDialogTitle>
            <AlertDialogDescription>
              {currentActivity?.name && <>{t("itsm:smart.nextStep")}: {currentActivity.name}<br /></>}
              {confidencePct != null && <>{t("itsm:smart.confidence")}: {confidencePct}%<br /></>}
              {t("itsm:smart.rejectConfirmDesc", { defaultValue: "拒绝后系统会保留审计记录，并等待后续人工处理或接管。" })}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>{t("common:cancel")}</AlertDialogCancel>
            <AlertDialogAction onClick={() => currentActivity && rejectAiMut.mutate(currentActivity.id)} disabled={!currentActivity || rejectAiMut.isPending}>
              {t("itsm:smart.confirmReject", { defaultValue: "确认拒绝" })}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>

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
    </div>
  )
}
