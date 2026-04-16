"use client"

import { useState } from "react"
import { useParams, useNavigate } from "react-router"
import { useTranslation } from "react-i18next"
import { useForm } from "react-hook-form"
import { z } from "zod"
import { zodResolver } from "@hookform/resolvers/zod"
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query"
import {
  ArrowLeft, UserPlus, CheckCircle, XCircle, Clock,
  PlusCircle, Play, Bot, AlertTriangle, RotateCcw, ArrowRight,
  type LucideIcon,
} from "lucide-react"
import { toast } from "sonner"
import { Button } from "@/components/ui/button"
import { Badge } from "@/components/ui/badge"
import { Textarea } from "@/components/ui/textarea"
import {
  Select, SelectContent, SelectItem, SelectTrigger, SelectValue,
} from "@/components/ui/select"
import {
  Card, CardContent, CardHeader, CardTitle,
} from "@/components/ui/card"
import {
  AlertDialog, AlertDialogAction, AlertDialogCancel, AlertDialogContent,
  AlertDialogDescription, AlertDialogFooter, AlertDialogHeader,
  AlertDialogTitle, AlertDialogTrigger,
} from "@/components/ui/alert-dialog"
import {
  Sheet, SheetContent, SheetHeader, SheetTitle, SheetDescription, SheetFooter,
} from "@/components/ui/sheet"
import {
  Form, FormControl, FormField, FormItem, FormLabel, FormMessage,
} from "@/components/ui/form"
import { usePermission } from "@/hooks/use-permission"
import { useAuthStore } from "@/stores/auth"
import {
  fetchTicket, fetchTicketTimeline, fetchTicketActivities, fetchTicketTokens,
  assignTicket, completeTicket, cancelTicket, progressTicket, fetchUsers,
} from "../../../api"
import { WorkflowViewer } from "../../../components/workflow"
import { OverrideActions } from "../../../components/override-actions"
import { SmartFlowVisualization } from "../../../components/smart-flow-visualization"
import { SmartCurrentActivityCard } from "../../../components/smart-current-activity-card"
import { VariablesPanel } from "../../../components/variables-panel"

const STATUS_MAP: Record<string, { variant: "default" | "secondary" | "destructive" | "outline"; key: string }> = {
  pending: { variant: "secondary", key: "statusPending" },
  in_progress: { variant: "default", key: "statusInProgress" },
  waiting_approval: { variant: "outline", key: "statusWaitingApproval" },
  waiting_action: { variant: "outline", key: "statusWaitingAction" },
  completed: { variant: "default", key: "statusCompleted" },
  failed: { variant: "destructive", key: "statusFailed" },
  cancelled: { variant: "secondary", key: "statusCancelled" },
}

const SLA_VARIANT: Record<string, "default" | "secondary" | "destructive"> = {
  normal: "default",
  warning: "secondary",
  breached: "destructive",
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

const ACTIVE_STATUSES = new Set(["pending", "in_progress", "waiting_approval", "waiting_action"])

function getNodeOutcomes(activityType: string): string[] {
  switch (activityType) {
    case "form": return ["submitted"]
    case "approve": return ["approved", "rejected"]
    case "process": return ["completed"]
    default: return ["completed"]
  }
}

export function Component() {
  const { t } = useTranslation(["itsm", "common"])
  const { id } = useParams<{ id: string }>()
  const navigate = useNavigate()
  const queryClient = useQueryClient()
  const ticketId = Number(id)
  const [assignOpen, setAssignOpen] = useState(false)
  const [cancelOpen, setCancelOpen] = useState(false)

  const canAssign = usePermission("itsm:ticket:assign")
  const canComplete = usePermission("itsm:ticket:complete")
  const canCancel = usePermission("itsm:ticket:cancel")
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

  const assignMut = useMutation({
    mutationFn: (v: { assigneeId: number }) => assignTicket(ticketId, v.assigneeId),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["itsm-ticket", ticketId] })
      queryClient.invalidateQueries({ queryKey: ["itsm-ticket-timeline", ticketId] })
      setAssignOpen(false)
      toast.success(t("itsm:tickets.assignSuccess"))
    },
    onError: (err) => toast.error(err.message),
  })

  const completeMut = useMutation({
    mutationFn: () => completeTicket(ticketId),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["itsm-ticket", ticketId] })
      queryClient.invalidateQueries({ queryKey: ["itsm-ticket-timeline", ticketId] })
      toast.success(t("itsm:tickets.completeSuccess"))
    },
    onError: (err) => toast.error(err.message),
  })

  const cancelMut = useMutation({
    mutationFn: (v: { reason: string }) => cancelTicket(ticketId, v.reason),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["itsm-ticket", ticketId] })
      queryClient.invalidateQueries({ queryKey: ["itsm-ticket-timeline", ticketId] })
      queryClient.invalidateQueries({ queryKey: ["itsm-ticket-activities", ticketId] })
      setCancelOpen(false)
      toast.success(t("itsm:tickets.cancelSuccess"))
    },
    onError: (err) => toast.error(err.message),
  })

  const progressMut = useMutation({
    mutationFn: (data: { activityId: number; outcome: string }) => progressTicket(ticketId, data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["itsm-ticket", ticketId] })
      queryClient.invalidateQueries({ queryKey: ["itsm-ticket-timeline", ticketId] })
      queryClient.invalidateQueries({ queryKey: ["itsm-ticket-activities", ticketId] })
      toast.success(t("itsm:tickets.progressSuccess"))
    },
    onError: (err) => toast.error(err.message),
  })

  const isActive = ticket ? ACTIVE_STATUSES.has(ticket.status) : false
  const statusInfo = ticket ? (STATUS_MAP[ticket.status] ?? { variant: "secondary" as const, key: "statusPending" }) : null

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
    <div className="space-y-4">
      <div className="flex items-center gap-3">
        <Button variant="ghost" size="icon" onClick={() => navigate(-1)}>
          <ArrowLeft className="h-4 w-4" />
        </Button>
        <div className="flex-1">
          <div className="flex items-center gap-2">
            <h2 className="text-lg font-semibold">{ticket.code}</h2>
            {statusInfo && <Badge variant={statusInfo.variant}>{t(`itsm:tickets.${statusInfo.key}`)}</Badge>}
          </div>
          <p className="text-sm text-muted-foreground">{ticket.title}</p>
        </div>
        {isActive && (
          <div className="flex gap-2">
            {canAssign && (
              <Button variant="outline" size="sm" onClick={() => { assignForm.reset({ assigneeId: ticket.assigneeId ?? 0 }); setAssignOpen(true) }}>
                <UserPlus className="mr-1 h-3.5 w-3.5" />{t("itsm:tickets.assign")}
              </Button>
            )}
            {ticket.engineType === "smart" && (
              <OverrideActions ticketId={ticketId} currentActivityId={ticket.currentActivityId} />
            )}
            {canComplete && (
              <AlertDialog>
                <AlertDialogTrigger asChild>
                  <Button size="sm">
                    <CheckCircle className="mr-1 h-3.5 w-3.5" />{t("itsm:tickets.complete")}
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
            {canCancel && (
              <Button variant="outline" size="sm" className="text-destructive" onClick={() => { cancelForm.reset({ reason: "" }); setCancelOpen(true) }}>
                <XCircle className="mr-1 h-3.5 w-3.5" />{t("itsm:tickets.cancel")}
              </Button>
            )}
          </div>
        )}
      </div>

      <div className="grid gap-4 md:grid-cols-2">
        <Card>
          <CardHeader>
            <CardTitle className="text-base">{t("itsm:tickets.detail")}</CardTitle>
          </CardHeader>
          <CardContent className="space-y-3 text-sm">
            <div className="flex justify-between">
              <span className="text-muted-foreground">{t("itsm:tickets.service")}</span>
              <span>{ticket.serviceName}</span>
            </div>
            <div className="flex justify-between">
              <span className="text-muted-foreground">{t("itsm:tickets.priority")}</span>
              <span className="inline-flex items-center gap-1.5">
                <span className="inline-block h-2.5 w-2.5 rounded-full" style={{ backgroundColor: ticket.priorityColor }} />
                {ticket.priorityName}
              </span>
            </div>
            <div className="flex justify-between">
              <span className="text-muted-foreground">{t("itsm:tickets.requester")}</span>
              <span>{ticket.requesterName}</span>
            </div>
            <div className="flex justify-between">
              <span className="text-muted-foreground">{t("itsm:tickets.assignee")}</span>
              <span>{ticket.assigneeName || "—"}</span>
            </div>
            <div className="flex justify-between">
              <span className="text-muted-foreground">{t("itsm:tickets.source")}</span>
              <span>{ticket.source === "agent" ? t("itsm:tickets.sourceAgent") : t("itsm:tickets.sourceCatalog")}</span>
            </div>
            <div className="flex justify-between">
              <span className="text-muted-foreground">{t("itsm:tickets.createdAt")}</span>
              <span>{new Date(ticket.createdAt).toLocaleString()}</span>
            </div>
            {ticket.finishedAt && (
              <div className="flex justify-between">
                <span className="text-muted-foreground">{t("itsm:tickets.finishedAt")}</span>
                <span>{new Date(ticket.finishedAt).toLocaleString()}</span>
              </div>
            )}
            {ticket.description && (
              <div className="pt-2 border-t">
                <p className="text-muted-foreground mb-1">{t("itsm:tickets.description")}</p>
                <p className="whitespace-pre-wrap">{ticket.description}</p>
              </div>
            )}
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle className="text-base">{t("itsm:tickets.slaStatus")}</CardTitle>
          </CardHeader>
          <CardContent className="space-y-3 text-sm">
            {ticket.slaStatus && (
              <div className="flex justify-between">
                <span className="text-muted-foreground">{t("itsm:tickets.slaStatus")}</span>
                <Badge variant={SLA_VARIANT[ticket.slaStatus] ?? "secondary"}>
                  {t(`itsm:tickets.sla${ticket.slaStatus.charAt(0).toUpperCase() + ticket.slaStatus.slice(1)}`)}
                </Badge>
              </div>
            )}
            {ticket.slaResponseDeadline && (
              <div className="flex justify-between">
                <span className="text-muted-foreground">{t("itsm:tickets.slaResponseDeadline")}</span>
                <span>{new Date(ticket.slaResponseDeadline).toLocaleString()}</span>
              </div>
            )}
            {ticket.slaResolutionDeadline && (
              <div className="flex justify-between">
                <span className="text-muted-foreground">{t("itsm:tickets.slaResolutionDeadline")}</span>
                <span>{new Date(ticket.slaResolutionDeadline).toLocaleString()}</span>
              </div>
            )}
          </CardContent>
        </Card>
      </div>

      {/* Smart Engine: Current Activity Card */}
      {ticket.engineType === "smart" && (
        <SmartCurrentActivityCard ticket={ticket} activities={activities} currentUserId={currentUserId} />
      )}

      {/* Smart Engine: Flow Visualization */}
      {ticket.engineType === "smart" && activities.length > 0 && (
        <SmartFlowVisualization activities={activities} currentActivityId={ticket.currentActivityId} />
      )}

      {/* Workflow Viewer (classic engine only) */}
      {ticket.engineType === "classic" && !!ticket.workflowJson && (
        <Card>
          <CardHeader>
            <CardTitle className="text-base">{t("itsm:workflow.viewer.workflowGraph")}</CardTitle>
          </CardHeader>
          <CardContent>
            <WorkflowViewer
              workflowJson={ticket.workflowJson}
              activities={activities}
              tokens={tokens}
              currentActivityId={ticket.currentActivityId}
            />
            {/* Activity action buttons */}
            {isActive && activities.filter((a) => a.status === "pending" || a.status === "in_progress").length > 0 && (
              <div className="mt-3 flex flex-wrap gap-2 border-t pt-3">
                {activities
                  .filter((a) => a.status === "pending" || a.status === "in_progress")
                  .map((a) => {
                    const outcomes = getNodeOutcomes(a.activityType)
                    return outcomes.map((outcome) => (
                      <Button
                        key={`${a.id}-${outcome}`}
                        size="sm"
                        variant={outcome === "rejected" || outcome === "failed" ? "destructive" : "default"}
                        disabled={progressMut.isPending}
                        onClick={() => progressMut.mutate({ activityId: a.id, outcome })}
                      >
                        {a.name}: {outcome}
                      </Button>
                    ))
                  })}
              </div>
            )}
          </CardContent>
        </Card>
      )}

      {/* Process Variables Panel */}
      <VariablesPanel ticketId={ticketId} />

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
                    <div className="flex-1 pt-0.5">
                      <p className="text-sm">{event.content}</p>
                      <p className="text-xs text-muted-foreground mt-0.5">
                        {event.operatorName} · {new Date(event.createdAt).toLocaleString()}
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

      {/* Assign Sheet */}
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

      {/* Cancel Sheet */}
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
