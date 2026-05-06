"use client"

import { useEffect, useMemo, useState } from "react"
import { useNavigate } from "react-router"
import { useTranslation } from "react-i18next"
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import {
  AlertTriangle,
  ArrowUpRight,
  Bot,
  CheckCircle2,
  CircleX,
  Clock3,
  Loader2,
  RefreshCcw,
  Route,
  ShieldAlert,
  Ticket,
  UserPlus,
} from "lucide-react"
import { toast } from "sonner"
import { withActiveMenuPermission } from "@/lib/navigation-state"
import { cn } from "@/lib/utils"
import { usePermission } from "@/hooks/use-permission"
import { useAuthStore } from "@/stores/auth"
import { Button } from "@/components/ui/button"
import { ButtonGroup } from "@/components/ui/button-group"
import {
  DataTableCard,
  DataTableEmptyRow,
  DataTableLoadingRow,
  DataTablePagination,
  DataTableToolbar,
  DataTableToolbarGroup,
} from "@/components/ui/data-table"
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
  SheetHeader,
  SheetTitle,
} from "@/components/ui/sheet"
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table"
import { Textarea } from "@/components/ui/textarea"
import { WorkspaceSearchField, WorkspaceStatus } from "@/components/workspace/primitives"
import {
  assignTicket,
  cancelTicket,
  fetchDecisionQuality,
  fetchPriorities,
  fetchServiceDefs,
  fetchTicket,
  fetchTicketActivities,
  fetchTicketMonitor,
  fetchTicketTimeline,
  fetchUsers,
  progressTicket,
  type DecisionQualityItem,
  type TicketMonitorItem,
  type TimelineItem,
} from "../../api"
import type { EngineType, RiskLevel, TicketStatus } from "../../contract"
import { OverrideActions } from "../../components/override-actions"
import { SLABadge } from "../../components/sla-badge"
import { TicketStatusBadge } from "../../components/ticket-status-badge"
import { TICKET_STATUS_OPTIONS } from "../../components/ticket-status"
import { itsmQueryKeys } from "../../query-keys"
import { buildTicketActionContext } from "./[id]/ticket-action-context"
import {
  AUDIT_METRICS,
  auditMetricValue,
  auditReasonRows,
  nextAuditMetricSelection,
  type AuditMetricCode,
} from "./monitor-audit"
import { TICKET_MENU_PERMISSION } from "./navigation"

const PAGE_SIZE = 20

const RISK_OPTIONS = [
  { value: "all", labelKey: "monitor.riskAll" },
  { value: "blocked", labelKey: "monitor.riskBlocked" },
  { value: "risk", labelKey: "monitor.riskRisk" },
  { value: "normal", labelKey: "monitor.riskNormal" },
] as const

function formatDate(value?: string | null) {
  return value ? new Date(value).toLocaleString() : "-"
}

function formatWaiting(minutes: number) {
  if (!minutes || minutes < 1) return "<1m"
  if (minutes < 60) return `${minutes}m`
  const h = Math.floor(minutes / 60)
  const m = minutes % 60
  return m > 0 ? `${h}h ${m}m` : `${h}h`
}

function formatPercent(value: number) {
  return `${Math.round(value * 100)}%`
}

function formatSeconds(value: number) {
  if (!value || value < 1) return "<1s"
  if (value < 60) return `${Math.round(value)}s`
  const minutes = Math.floor(value / 60)
  const seconds = Math.round(value % 60)
  return seconds > 0 ? `${minutes}m ${seconds}s` : `${minutes}m`
}

function riskTone(riskLevel: string): "danger" | "warning" | "neutral" {
  if (riskLevel === "blocked") return "danger"
  if (riskLevel === "risk") return "warning"
  return "neutral"
}

function riskLabel(riskLevel: string, t: (key: string) => string) {
  if (riskLevel === "blocked") return t("monitor.riskBlocked")
  if (riskLevel === "risk") return t("monitor.riskRisk")
  return t("monitor.riskNormal")
}

function engineLabel(engineType: string, t: (key: string) => string) {
  return engineType === "smart" ? t("monitor.engineSmart") : t("monitor.engineClassic")
}

function metricTone(metricCode: AuditMetricCode, value: number): "neutral" | "danger" | "warning" | "success" | "info" {
  if (metricCode === "active_total") return "info"
  if (metricCode === "blocked_total" || metricCode === "ai_incident_total") return value > 0 ? "danger" : "neutral"
  if (metricCode === "risk_total" || metricCode === "sla_risk_total") return value > 0 ? "warning" : "neutral"
  if (metricCode === "completed_today_total") return "success"
  return "neutral"
}

function reasonToneClass(severity: string) {
  if (severity === "risk") return "border-amber-200/70 bg-amber-50/65 text-amber-800"
  if (severity === "info") return "border-sky-200/70 bg-sky-50/65 text-sky-800"
  return "border-red-200/70 bg-red-50/65 text-red-800"
}

function MetricStrip({
  label,
  value,
  tone = "neutral",
  active = false,
  onClick,
}: {
  label: string
  value: number
  tone?: "neutral" | "danger" | "warning" | "success" | "info"
  active?: boolean
  onClick?: () => void
}) {
  return (
    <button
      type="button"
      aria-pressed={active}
      onClick={onClick}
      className={cn(
        "flex min-w-[8rem] items-center justify-between gap-3 border-b border-border/45 py-2 text-left transition hover:bg-muted/25 sm:border-b-0 sm:border-r sm:pr-4 last:border-r-0",
        active && "bg-primary/5 text-primary",
      )}
    >
      <span className="text-xs font-medium text-muted-foreground">{label}</span>
      <span className={cn(
        "font-mono text-lg font-semibold tabular-nums",
        tone === "danger" && "text-red-600",
        tone === "warning" && "text-amber-600",
        tone === "success" && "text-emerald-600",
        tone === "info" && "text-sky-600",
      )}>
        {value}
      </span>
    </button>
  )
}

export function Component() {
  const { t } = useTranslation(["itsm", "common"])
  const [keyword, setKeyword] = useState("")
  const [submittedKeyword, setSubmittedKeyword] = useState("")
  const [riskFilter, setRiskFilter] = useState<RiskLevel | "all">("all")
  const [engineFilter, setEngineFilter] = useState<EngineType | "all">("all")
  const [statusFilter, setStatusFilter] = useState<TicketStatus | "all">("all")
  const [priorityFilter, setPriorityFilter] = useState("all")
  const [serviceFilter, setServiceFilter] = useState("all")
  const [qualityDimension, setQualityDimension] = useState<"service" | "department">("service")
  const [activeMetricCode, setActiveMetricCode] = useState<AuditMetricCode | null>(null)
  const [page, setPage] = useState(1)
  const [selectedTicket, setSelectedTicket] = useState<TicketMonitorItem | null>(null)

  const monitorParams = useMemo(() => ({
    keyword: submittedKeyword || undefined,
    riskLevel: riskFilter === "all" ? undefined : riskFilter,
    engineType: engineFilter === "all" ? undefined : engineFilter,
    status: statusFilter === "all" ? undefined : statusFilter,
    priorityId: priorityFilter === "all" ? undefined : Number(priorityFilter),
    serviceId: serviceFilter === "all" ? undefined : Number(serviceFilter),
    metricCode: activeMetricCode ?? undefined,
    page,
    pageSize: PAGE_SIZE,
  }), [activeMetricCode, engineFilter, page, priorityFilter, riskFilter, serviceFilter, statusFilter, submittedKeyword])

  const { data, isLoading, isFetching, refetch } = useQuery({
    queryKey: ["itsm-ticket-monitor", monitorParams],
    queryFn: () => fetchTicketMonitor(monitorParams),
  })

  useEffect(() => {
    const interval = window.setInterval(() => {
      void refetch()
    }, 60000)
    return () => window.clearInterval(interval)
  }, [refetch])

  const { data: priorities = [] } = useQuery({
    queryKey: itsmQueryKeys.priorities.all,
    queryFn: () => fetchPriorities(),
  })

  const { data: servicesData } = useQuery({
    queryKey: ["itsm-services-list"],
    queryFn: () => fetchServiceDefs({ page: 1, pageSize: 100 }),
  })
  const services = servicesData?.items ?? []

  const { data: decisionQuality } = useQuery({
    queryKey: ["itsm-decision-quality", qualityDimension],
    queryFn: () => fetchDecisionQuality({ dimension: qualityDimension, windowDays: 30 }),
  })

  const items = data?.items ?? []
  const summary = data?.summary
  const qualityItems = (decisionQuality?.items ?? []).slice(0, 5)
  const total = data?.total ?? 0
  const totalPages = Math.ceil(total / PAGE_SIZE)

  function resetPageAnd(run: () => void) {
    run()
    setPage(1)
  }

  return (
    <div className="workspace-page">
      <div className="workspace-page-header">
        <div className="min-w-0">
          <h2 className="workspace-page-title">{t("tickets.title")}</h2>
          <p className="workspace-page-description">{t("tickets.monitorDesc")}</p>
        </div>
        <Button variant="outline" size="sm" onClick={() => {
          resetPageAnd(() => setSubmittedKeyword(keyword))
          void refetch()
        }} disabled={isFetching}>
          {isFetching ? <Loader2 className="h-4 w-4 animate-spin" /> : <RefreshCcw className="h-4 w-4" />}
          {t("monitor.refresh")}
        </Button>
      </div>

      <section className="workspace-surface rounded-[1.25rem] px-4 py-3">
        <div className="grid gap-x-4 sm:grid-cols-2 lg:grid-cols-4 xl:grid-cols-8">
          {AUDIT_METRICS.map((metric) => {
            const value = auditMetricValue(summary, metric.code)
            return (
              <MetricStrip
                key={metric.code}
                label={t(metric.labelKey)}
                value={value}
                tone={metricTone(metric.code, value)}
                active={activeMetricCode === metric.code}
                onClick={() => resetPageAnd(() => {
                  setActiveMetricCode((current) => nextAuditMetricSelection(current, metric.code))
                })}
              />
            )
          })}
        </div>
      </section>

      <section className="workspace-surface rounded-[1.25rem] px-4 py-3">
        <div className="mb-3 flex items-center justify-between gap-3">
          <div>
            <p className="text-sm font-semibold">{t("monitor.qualityTitle")}</p>
            <p className="text-xs text-muted-foreground">
              {decisionQuality?.version
                ? `${t("monitor.qualityVersion")}: ${decisionQuality.version} · ${t("monitor.qualityTrendHint")}`
                : t("monitor.qualityTrendHint")}
            </p>
          </div>
          <ButtonGroup>
            <Button size="sm" variant={qualityDimension === "service" ? "default" : "outline"} onClick={() => setQualityDimension("service")}>
              {t("monitor.qualityByService")}
            </Button>
            <Button size="sm" variant={qualityDimension === "department" ? "default" : "outline"} onClick={() => setQualityDimension("department")}>
              {t("monitor.qualityByDepartment")}
            </Button>
          </ButtonGroup>
        </div>
        <div className="overflow-x-auto">
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>{t("monitor.qualityDimension")}</TableHead>
                <TableHead>{t("monitor.qualityApprovalRate")}</TableHead>
                <TableHead>{t("monitor.qualityRejectionRate")}</TableHead>
                <TableHead>{t("monitor.qualityRetryRate")}</TableHead>
                <TableHead>{t("monitor.qualityLatency")}</TableHead>
                <TableHead>{t("monitor.qualityRecoveryRate")}</TableHead>
                <TableHead>{t("monitor.qualityDecisionCount")}</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {qualityItems.length === 0 ? (
                <DataTableEmptyRow colSpan={7} icon={Ticket} title={t("monitor.qualityEmpty")} />
              ) : (
                qualityItems.map((item: DecisionQualityItem) => (
                  <TableRow key={`${item.dimensionType}-${item.dimensionId}`}>
                    <TableCell className="font-medium">{item.dimensionName || "-"}</TableCell>
                    <TableCell>{formatPercent(item.approvalRate)}</TableCell>
                    <TableCell>{formatPercent(item.rejectionRate)}</TableCell>
                    <TableCell>{formatPercent(item.retryRate)}</TableCell>
                    <TableCell>{formatSeconds(item.avgDecisionLatencySeconds)}</TableCell>
                    <TableCell>{formatPercent(item.recoverySuccessRate)}</TableCell>
                    <TableCell>{item.decisionCount}</TableCell>
                  </TableRow>
                ))
              )}
            </TableBody>
          </Table>
        </div>
      </section>

      <DataTableToolbar>
        <DataTableToolbarGroup className="lg:flex-wrap">
          <form
            className="flex min-w-0 flex-1 flex-col gap-2 sm:flex-row sm:items-center"
            onSubmit={(event) => {
              event.preventDefault()
              resetPageAnd(() => setSubmittedKeyword(keyword))
            }}
          >
            <WorkspaceSearchField
              value={keyword}
              onChange={setKeyword}
              placeholder={t("tickets.searchPlaceholder")}
              className="sm:w-80"
            />
            <Button type="submit" size="sm" variant="outline">{t("common:search")}</Button>
          </form>
          <ButtonGroup className="overflow-x-auto">
            {RISK_OPTIONS.map((option) => (
              <Button
                key={option.value}
                type="button"
                variant={riskFilter === option.value ? "default" : "outline"}
                size="sm"
                onClick={() => resetPageAnd(() => setRiskFilter(option.value))}
              >
                {t(option.labelKey)}
              </Button>
            ))}
          </ButtonGroup>
        </DataTableToolbarGroup>
        <div className="flex flex-wrap items-center gap-2">
          <Select value={engineFilter} onValueChange={(v) => resetPageAnd(() => setEngineFilter(v as EngineType | "all"))}>
            <SelectTrigger className="h-8 w-[128px]"><SelectValue /></SelectTrigger>
            <SelectContent>
              <SelectItem value="all">{t("monitor.engineAll")}</SelectItem>
              <SelectItem value="smart">{t("monitor.engineSmart")}</SelectItem>
              <SelectItem value="classic">{t("monitor.engineClassic")}</SelectItem>
            </SelectContent>
          </Select>
          <Select value={statusFilter} onValueChange={(v) => resetPageAnd(() => setStatusFilter(v as TicketStatus | "all"))}>
            <SelectTrigger className="h-8 w-[136px]"><SelectValue /></SelectTrigger>
            <SelectContent>
              <SelectItem value="all">{t("tickets.allStatuses")}</SelectItem>
              {Object.entries(TICKET_STATUS_OPTIONS).map(([k, v]) => (
                <SelectItem key={k} value={k}>{t(`tickets.${v.key}`)}</SelectItem>
              ))}
            </SelectContent>
          </Select>
          <Select value={priorityFilter} onValueChange={(v) => resetPageAnd(() => setPriorityFilter(v))}>
            <SelectTrigger className="h-8 w-[136px]"><SelectValue /></SelectTrigger>
            <SelectContent>
              <SelectItem value="all">{t("tickets.allPriorities")}</SelectItem>
              {priorities.map((p) => (
                <SelectItem key={p.id} value={String(p.id)}>{p.name}</SelectItem>
              ))}
            </SelectContent>
          </Select>
          <Select value={serviceFilter} onValueChange={(v) => resetPageAnd(() => setServiceFilter(v))}>
            <SelectTrigger className="h-8 w-[164px]"><SelectValue /></SelectTrigger>
            <SelectContent>
              <SelectItem value="all">{t("tickets.allServices")}</SelectItem>
              {services.map((service) => (
                <SelectItem key={service.id} value={String(service.id)}>{service.name}</SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>
      </DataTableToolbar>

      <DataTableCard className="overflow-x-auto">
        <Table className="min-w-[1120px]">
          <TableHeader>
            <TableRow>
              <TableHead className="w-[112px]">{t("monitor.risk")}</TableHead>
              <TableHead className="min-w-[240px]">{t("tickets.ticketTitle")}</TableHead>
              <TableHead className="w-[180px]">{t("tickets.service")}</TableHead>
              <TableHead className="w-[140px]">{t("monitor.owner")}</TableHead>
              <TableHead className="min-w-[180px]">{t("monitor.currentStep")}</TableHead>
              <TableHead className="w-[110px]">{t("monitor.waiting")}</TableHead>
              <TableHead className="w-[140px]">{t("tickets.slaStatus")}</TableHead>
              <TableHead className="w-[154px]">{t("tickets.updatedAt")}</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {isLoading ? (
              <DataTableLoadingRow colSpan={8} />
            ) : items.length === 0 ? (
              <DataTableEmptyRow colSpan={8} icon={Ticket} title={t("monitor.empty")} />
            ) : (
              items.map((item) => (
                <MonitorRow key={item.id} item={item} onOpen={() => setSelectedTicket(item)} />
              ))
            )}
          </TableBody>
        </Table>
      </DataTableCard>

      <DataTablePagination total={total} page={page} totalPages={totalPages} onPageChange={setPage} />

      <MonitorTicketSheet
        monitorItem={selectedTicket}
        open={selectedTicket != null}
        onOpenChange={(open) => {
          if (!open) setSelectedTicket(null)
        }}
      />
    </div>
  )
}

function MonitorRow({ item, onOpen }: { item: TicketMonitorItem; onOpen: () => void }) {
  const { t } = useTranslation("itsm")
  const primaryReason = item.stuckReasons?.[0]
  return (
    <TableRow className="cursor-pointer align-top hover:bg-muted/25" onClick={onOpen}>
      <TableCell>
        <div className="flex items-center gap-2">
          <span className={cn(
            "h-9 w-1 rounded-full",
            item.riskLevel === "blocked" && "bg-red-500",
            item.riskLevel === "risk" && "bg-amber-500",
            item.riskLevel === "normal" && "bg-muted-foreground/30",
          )} />
          <WorkspaceStatus tone={riskTone(item.riskLevel)} label={riskLabel(item.riskLevel, t)} />
        </div>
      </TableCell>
      <TableCell>
        <div className="min-w-0">
          <div className="flex min-w-0 items-center gap-2">
            <span className="font-mono text-xs text-muted-foreground">{item.code}</span>
            <TicketStatusBadge ticket={item} />
          </div>
          <p className="mt-1 line-clamp-1 font-medium text-foreground">{item.title}</p>
          {primaryReason ? <p className="mt-1 line-clamp-1 text-xs text-muted-foreground">{primaryReason}</p> : null}
        </div>
      </TableCell>
      <TableCell>
        <div className="min-w-0">
          <p className="truncate text-sm font-medium">{item.serviceName || "-"}</p>
          <p className="mt-1 inline-flex items-center gap-1.5 text-xs text-muted-foreground">
            {item.engineType === "smart" ? <Bot className="h-3.5 w-3.5" /> : <Route className="h-3.5 w-3.5" />}
            {engineLabel(item.engineType, t)}
          </p>
        </div>
      </TableCell>
      <TableCell className="text-sm">{item.currentOwnerName || item.assigneeName || "-"}</TableCell>
      <TableCell>
        <p className="line-clamp-1 text-sm font-medium">{item.currentActivityName || item.nextStepSummary || t("monitor.waitingSystem")}</p>
        <p className="mt-1 text-xs text-muted-foreground">{item.currentActivityType || item.smartState || "-"}</p>
      </TableCell>
      <TableCell className="font-mono text-sm tabular-nums">{formatWaiting(item.waitingMinutes)}</TableCell>
      <TableCell>
        <SLABadge slaStatus={item.slaStatus} slaResolutionDeadline={item.slaResolutionDeadline} />
      </TableCell>
      <TableCell className="text-xs text-muted-foreground">{formatDate(item.updatedAt)}</TableCell>
    </TableRow>
  )
}

function MonitorTicketSheet({
  monitorItem,
  open,
  onOpenChange,
}: {
  monitorItem: TicketMonitorItem | null
  open: boolean
  onOpenChange: (open: boolean) => void
}) {
  const { t } = useTranslation(["itsm", "common"])
  const navigate = useNavigate()
  const queryClient = useQueryClient()
  const canAssign = usePermission("itsm:ticket:assign")
  const canCancel = usePermission("itsm:ticket:cancel")
  const canOverride = usePermission("itsm:ticket:override")
  const currentUserId = useAuthStore((s) => s.user?.id ?? 0)
  const ticketId = monitorItem?.id ?? null
  const [actionDraft, setActionDraft] = useState({
    ticketId: null as number | null,
    assigneeId: "",
    opinion: "",
    cancelReason: "",
  })
  const scopedDraft = actionDraft.ticketId === ticketId
    ? actionDraft
    : { ticketId, assigneeId: "", opinion: "", cancelReason: "" }
  const updateDraft = (patch: Partial<Omit<typeof actionDraft, "ticketId">>) => {
    setActionDraft({ ...scopedDraft, ...patch, ticketId })
  }
  const resetDraft = () => {
    setActionDraft({ ticketId: null, assigneeId: "", opinion: "", cancelReason: "" })
  }

  const enabled = open && ticketId != null
  const { data: ticket } = useQuery({
    queryKey: ["itsm-ticket", ticketId],
    queryFn: () => fetchTicket(ticketId!),
    enabled,
  })
  const { data: activities = [] } = useQuery({
    queryKey: ["itsm-ticket-activities", ticketId],
    queryFn: () => fetchTicketActivities(ticketId!),
    enabled,
  })
  const { data: timeline = [] } = useQuery({
    queryKey: ["itsm-ticket-timeline", ticketId],
    queryFn: () => fetchTicketTimeline(ticketId!),
    enabled,
  })
  const { data: users = [] } = useQuery({
    queryKey: ["users-for-monitor-actions"],
    queryFn: () => fetchUsers(),
    enabled,
  })

  const actionContext = buildTicketActionContext({
    ticket,
    activities,
    currentUserId,
    canAssignPermission: canAssign,
    canCancelPermission: canCancel,
  })
  const displayActivity = actionContext.displayHumanActivity
  const actionableActivity = actionContext.selectedActionableActivity
  const reasonRows = auditReasonRows(monitorItem?.monitorReasons, monitorItem?.stuckReasons)
  const reasonTitle = monitorItem?.riskLevel === "risk" ? t("monitor.riskReasons") : t("monitor.blockingReasons")
  const emptyReasonText = monitorItem?.riskLevel === "risk" ? t("monitor.noRiskReasons") : t("monitor.noBlockingReasons")

  function invalidate() {
    if (!ticketId) return
    queryClient.invalidateQueries({ queryKey: ["itsm-ticket-monitor"] })
    queryClient.invalidateQueries({ queryKey: ["itsm-ticket", ticketId] })
    queryClient.invalidateQueries({ queryKey: ["itsm-ticket-activities", ticketId] })
    queryClient.invalidateQueries({ queryKey: ["itsm-ticket-timeline", ticketId] })
  }

  const assignMut = useMutation({
    mutationFn: () => assignTicket(ticketId!, Number(scopedDraft.assigneeId)),
    onSuccess: () => {
      toast.success(t("itsm:tickets.assignSuccess"))
      updateDraft({ assigneeId: "" })
      invalidate()
    },
    onError: (err) => toast.error(err.message),
  })

  const progressMut = useMutation({
    mutationFn: (outcome: "approved" | "rejected") => progressTicket(ticketId!, {
      activityId: actionableActivity!.id,
      outcome,
      opinion: scopedDraft.opinion,
    }),
    onSuccess: () => {
      toast.success(t("itsm:tickets.progressSuccess"))
      updateDraft({ opinion: "" })
      invalidate()
    },
    onError: (err) => toast.error(err.message),
  })

  const cancelMut = useMutation({
    mutationFn: () => cancelTicket(ticketId!, scopedDraft.cancelReason.trim()),
    onSuccess: () => {
      toast.success(t("itsm:tickets.cancelSuccess"))
      updateDraft({ cancelReason: "" })
      invalidate()
    },
    onError: (err) => toast.error(err.message),
  })

  return (
    <Sheet open={open} onOpenChange={(nextOpen) => {
      if (!nextOpen) resetDraft()
      onOpenChange(nextOpen)
    }}>
      <SheetContent className="w-full border-l-border/70 bg-background shadow-none backdrop-blur-none sm:max-w-[560px]">
        <SheetHeader className="border-b border-border/55 pr-12">
          <SheetTitle className="flex items-center gap-2">
            <ShieldAlert className="h-4 w-4" />
            {ticket?.code ?? t("monitor.ticketDiagnosis")}
          </SheetTitle>
          <SheetDescription>{ticket?.title ?? t("monitor.loadingTicket")}</SheetDescription>
        </SheetHeader>

        <div className="min-h-0 flex-1 space-y-4 overflow-y-auto px-4 pb-4">
          {!ticket ? (
            <div className="flex items-center gap-2 py-10 text-sm text-muted-foreground">
              <Loader2 className="h-4 w-4 animate-spin" />
              {t("monitor.loadingTicket")}
            </div>
          ) : (
            <>
              <section className="space-y-3 border-b border-border/50 pb-4">
                <div className="flex flex-wrap items-center gap-2">
                  <TicketStatusBadge ticket={ticket} />
                  <WorkspaceStatus tone={ticket.engineType === "smart" ? "info" : "neutral"} label={engineLabel(ticket.engineType, t)} />
                  <SLABadge slaStatus={ticket.slaStatus} slaResolutionDeadline={ticket.slaResolutionDeadline} />
                </div>
	                <div className="grid gap-3 text-sm sm:grid-cols-2">
	                  <Fact label={t("tickets.service")} value={ticket.serviceName || "-"} />
	                  <Fact label={t("monitor.owner")} value={ticket.currentOwnerName || ticket.assigneeName || "-"} />
	                  <Fact label={t("monitor.currentStep")} value={displayActivity?.name || monitorItem?.currentActivityName || ticket.nextStepSummary || "-"} />
	                  <Fact label={t("tickets.updatedAt")} value={formatDate(ticket.updatedAt)} />
	                </div>
	              </section>

	              <section className="space-y-2 border-b border-border/50 pb-4">
	                <h3 className="text-sm font-semibold">{reasonTitle}</h3>
	                {reasonRows.length ? (
	                  <div className="space-y-2">
	                    {reasonRows.map((reason, index) => (
	                      <div key={`${reason.ruleCode || reason.message}-${index}`} className={cn("rounded-lg border px-3 py-2 text-sm", reasonToneClass(reason.severity))}>
	                        <div className="flex gap-2">
	                          <AlertTriangle className="mt-0.5 h-4 w-4 shrink-0" />
	                          <div className="min-w-0 flex-1">
	                            <p>{reason.message}</p>
	                            {reason.type === "structured" && (
	                              <p className="mt-1 break-all font-mono text-[11px] opacity-75">
	                                {t("monitor.metricCode")}: {reason.metricCode} · {t("monitor.ruleCode")}: {reason.ruleCode}
	                              </p>
	                            )}
	                          </div>
	                        </div>
	                        {reason.evidence.length > 0 && (
	                          <div className="mt-2 grid gap-1 border-t border-current/15 pt-2 text-[11px]">
	                            <p className="font-medium opacity-75">{t("monitor.evidence")}</p>
	                            {reason.evidence.map((entry) => (
	                              <div key={entry.field} className="grid grid-cols-[minmax(0,0.9fr)_minmax(0,1.1fr)] gap-2 font-mono opacity-80">
	                                <span className="truncate">{entry.field}</span>
	                                <span className="break-all">{entry.value}</span>
	                              </div>
	                            ))}
	                          </div>
	                        )}
	                      </div>
	                    ))}
	                  </div>
	                ) : (
                  <p className="rounded-lg border border-border/50 bg-background/35 px-3 py-2 text-sm text-muted-foreground">
                    {emptyReasonText}
                  </p>
                )}
              </section>

	              {actionContext.isActive && (
	                <section className="space-y-3 border-b border-border/50 pb-4">
	                  <h3 className="text-sm font-semibold">{t("monitor.manualActions")}</h3>
	                  {actionContext.canProcess && actionableActivity ? (
	                    <div className="space-y-2">
	                      <Textarea
	                        value={scopedDraft.opinion}
	                        onChange={(event) => updateDraft({ opinion: event.target.value })}
                        rows={3}
                        placeholder={t("tickets.approvalOpinionPlaceholder")}
                      />
	                      <div className="grid grid-cols-2 gap-2">
	                        <Button size="sm" disabled={progressMut.isPending || !actionableActivity} onClick={() => progressMut.mutate("approved")}>
	                          <CheckCircle2 className="h-4 w-4" />
	                          {t("tickets.approve")}
	                        </Button>
	                        <Button size="sm" variant="outline" className="text-destructive" disabled={progressMut.isPending || !actionableActivity} onClick={() => progressMut.mutate("rejected")}>
	                          <CircleX className="h-4 w-4" />
	                          {t("tickets.reject")}
	                        </Button>
                      </div>
                    </div>
                  ) : (
                    <p className="rounded-lg border border-border/50 bg-background/35 px-3 py-2 text-sm text-muted-foreground">
                      {t("monitor.noHumanActivity")}
                    </p>
                  )}

	                  {actionContext.canAssign && (
                    <div className="grid gap-2 sm:grid-cols-[minmax(0,1fr)_auto]">
	                      <Select value={scopedDraft.assigneeId} onValueChange={(value) => updateDraft({ assigneeId: value })}>
                        <SelectTrigger><SelectValue placeholder={t("tickets.assigneePlaceholder")} /></SelectTrigger>
                        <SelectContent>
                          {users.map((user) => (
                            <SelectItem key={user.id} value={String(user.id)}>{user.username}</SelectItem>
                          ))}
                        </SelectContent>
                      </Select>
	                      <Button size="sm" variant="outline" disabled={!scopedDraft.assigneeId || assignMut.isPending} onClick={() => assignMut.mutate()}>
                        <UserPlus className="h-4 w-4" />
                        {t("tickets.assign")}
                      </Button>
                    </div>
                  )}

                  {ticket.engineType === "smart" && canOverride && (
                    <OverrideActions
                      ticketId={ticket.id}
                      currentActivityId={ticket.currentActivityId}
                      aiFailureCount={ticket.aiFailureCount}
                      triggerClassName="w-full justify-center"
                      onSuccess={invalidate}
                    />
                  )}

	                  {actionContext.canCancel && (
                    <div className="space-y-2">
	                      <Textarea
	                        value={scopedDraft.cancelReason}
	                        onChange={(event) => updateDraft({ cancelReason: event.target.value })}
                        rows={2}
                        placeholder={t("tickets.cancelReasonPlaceholder")}
                      />
                      <Button size="sm" variant="outline" className="w-full text-destructive" disabled={cancelMut.isPending} onClick={() => cancelMut.mutate()}>
                        <CircleX className="h-4 w-4" />
                        {t("tickets.cancel")}
                      </Button>
                    </div>
                  )}
                </section>
              )}

              <section className="space-y-2">
                <div className="flex items-center justify-between gap-3">
                  <h3 className="text-sm font-semibold">{t("monitor.recentTimeline")}</h3>
                  <Button
                    size="sm"
                    variant="ghost"
                    onClick={() => navigate(`/itsm/tickets/${ticket.id}`, { state: withActiveMenuPermission(TICKET_MENU_PERMISSION.list) })}
                  >
                    <ArrowUpRight className="h-4 w-4" />
                    {t("monitor.openDetail")}
                  </Button>
                </div>
                <div className="space-y-2">
                  {timeline.slice(0, 5).map((event) => (
                    <TimelineLine key={event.id} event={event} />
                  ))}
                  {timeline.length === 0 && (
                    <p className="rounded-lg border border-border/50 bg-background/35 px-3 py-2 text-sm text-muted-foreground">
                      {t("monitor.noTimeline")}
                    </p>
                  )}
                </div>
              </section>
            </>
          )}
        </div>
      </SheetContent>
    </Sheet>
  )
}

function Fact({ label, value }: { label: string; value: string }) {
  return (
    <div className="min-w-0">
      <p className="text-xs text-muted-foreground">{label}</p>
      <p className="mt-1 truncate font-medium">{value}</p>
    </div>
  )
}

function TimelineLine({ event }: { event: TimelineItem }) {
  return (
    <div className="grid grid-cols-[1rem_minmax(0,1fr)] gap-2 text-sm">
      <Clock3 className="mt-0.5 h-3.5 w-3.5 text-muted-foreground" />
      <div className="min-w-0">
        <p className="line-clamp-2">{event.message || event.content}</p>
        <p className="mt-0.5 text-xs text-muted-foreground">{formatDate(event.createdAt)}</p>
      </div>
    </div>
  )
}
