"use client"

import { useState, useEffect, useRef } from "react"
import { useTranslation } from "react-i18next"
import { useQueryClient } from "@tanstack/react-query"
import { Bot, Loader2, AlertTriangle, CheckCircle2, XCircle, FileText, Clock } from "lucide-react"
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"
import { Button } from "@/components/ui/button"
import { Badge } from "@/components/ui/badge"
import { Alert, AlertDescription } from "@/components/ui/alert"
import { type TicketItem, type ActivityItem, fetchTicket } from "../api"
import { AIDecisionPanel } from "./ai-decision-panel"
import { OverrideActions } from "./override-actions"
import { FormDataDisplay } from "./form-data-display"

const TERMINAL_STATUSES = new Set(["completed", "cancelled", "failed"])
const HUMAN_ACTIVITY_TYPES = new Set(["approve", "form", "process"])
const MAX_AI_FAILURES = 3
const POLL_INTERVAL = 3000
const POLL_TIMEOUT = 60000

type SmartState =
  | "terminal"
  | "ai_disabled"
  | "pending_approval"
  | "human_activity"
  | "ai_reasoning"
  | "idle"

function determineState(ticket: TicketItem, activities: ActivityItem[]): SmartState {
  if (TERMINAL_STATUSES.has(ticket.status)) return "terminal"
  if (ticket.aiFailureCount >= MAX_AI_FAILURES) return "ai_disabled"

  const currentActivity = activities.find(
    (a) => a.id === ticket.currentActivityId,
  )

  if (currentActivity?.status === "pending_approval" && currentActivity.canAct) return "pending_approval"

  const activeActivity = activities.find(
    (a) =>
      (a.status === "pending" || a.status === "in_progress") &&
      HUMAN_ACTIVITY_TYPES.has(a.activityType),
  )
  if (activeActivity) return "human_activity"

  const hasActiveActivity = activities.some(
    (a) => a.status === "pending" || a.status === "in_progress" || a.status === "pending_approval",
  )
  if (!hasActiveActivity && !TERMINAL_STATUSES.has(ticket.status)) return "ai_reasoning"

  return "idle"
}

interface SmartCurrentActivityCardProps {
  ticket: TicketItem
  activities: ActivityItem[]
  currentUserId: number
}

export function SmartCurrentActivityCard({ ticket, activities, currentUserId }: SmartCurrentActivityCardProps) {
  const { t } = useTranslation("itsm")
  const queryClient = useQueryClient()
  const state = determineState(ticket, activities)
  const [pollTimedOut, setPollTimedOut] = useState(false)
  const pollStartRef = useRef<number>(0)

  // Reset pollTimedOut when leaving ai_reasoning (render-time state adjustment)
  const [prevState, setPrevState] = useState(state)
  if (prevState !== state) {
    setPrevState(state)
    if (state !== "ai_reasoning" && pollTimedOut) {
      setPollTimedOut(false)
    }
  }

  // AI reasoning polling
  useEffect(() => {
    if (state !== "ai_reasoning") return
    pollStartRef.current = Date.now()
    const interval = setInterval(async () => {
      if (Date.now() - pollStartRef.current > POLL_TIMEOUT) {
        setPollTimedOut(true)
        clearInterval(interval)
        return
      }
      try {
        const fresh = await fetchTicket(ticket.id)
        if (fresh) {
          queryClient.setQueryData(["itsm-ticket", ticket.id], fresh)
          queryClient.invalidateQueries({ queryKey: ["itsm-ticket-activities", ticket.id] })
        }
      } catch {
        // ignore polling errors
      }
    }, POLL_INTERVAL)
    return () => clearInterval(interval)
  }, [state, ticket.id, queryClient])

  const currentActivity = activities.find((a) => a.id === ticket.currentActivityId)
  const activeHumanActivity = activities.find(
    (a) =>
      (a.status === "pending" || a.status === "in_progress") &&
      HUMAN_ACTIVITY_TYPES.has(a.activityType),
  )

  // --- State: Terminal ---
  if (state === "terminal") {
    const lastAiActivity = [...activities].reverse().find((a) => a.aiReasoning)
    const duration = ticket.finishedAt
      ? Math.round((new Date(ticket.finishedAt).getTime() - new Date(ticket.createdAt).getTime()) / 60000)
      : null
    const durationDisplay = duration != null
      ? duration >= 60
        ? `${Math.floor(duration / 60)} ${t("smart.hours", { defaultValue: "小时" })} ${duration % 60} ${t("smart.minutes")}`
        : `${duration} ${t("smart.minutes")}`
      : null
    return (
      <Card>
        <CardHeader className="pb-3">
          <CardTitle className="flex items-center gap-2 text-base">
            {ticket.status === "completed" ? (
              <CheckCircle2 className="h-4 w-4 text-green-600" />
            ) : (
              <XCircle className="h-4 w-4 text-muted-foreground" />
            )}
            {ticket.status === "completed" ? t("smart.terminalCompleted") : t("smart.terminalCancelled")}
          </CardTitle>
        </CardHeader>
        <CardContent className="space-y-3 text-sm">
          {durationDisplay != null && (
            <div className="flex justify-between">
              <span className="text-muted-foreground">{t("smart.processDuration")}</span>
              <span>{durationDisplay}</span>
            </div>
          )}
          {lastAiActivity?.aiReasoning && (
            <div className="space-y-1">
              <p className="text-muted-foreground">{t("smart.lastAiReasoning")}</p>
              <p className="whitespace-pre-wrap rounded-md bg-muted/50 p-3 text-sm">{lastAiActivity.aiReasoning}</p>
            </div>
          )}
        </CardContent>
      </Card>
    )
  }

  // --- State: AI Disabled ---
  if (state === "ai_disabled") {
    const lastFailedActivity = [...activities].reverse().find(
      (a) => a.status === "failed" || a.status === "rejected",
    )
    return (
      <Card className="border-amber-200">
        <CardHeader className="pb-3">
          <CardTitle className="flex items-center gap-2 text-base text-amber-700">
            <AlertTriangle className="h-4 w-4" />
            {t("smart.aiDisabled")}
          </CardTitle>
        </CardHeader>
        <CardContent className="space-y-4">
          <Alert variant="destructive" className="border-amber-200 bg-amber-50 text-amber-800 [&>svg]:text-amber-600">
            <AlertTriangle className="h-4 w-4" />
            <AlertDescription>
              {t("smart.aiDisabledDesc", { count: ticket.aiFailureCount })}
            </AlertDescription>
          </Alert>
          {lastFailedActivity?.aiReasoning && (
            <div className="space-y-1 text-sm">
              <p className="text-muted-foreground">{t("smart.lastFailureReason")}</p>
              <p className="whitespace-pre-wrap rounded-md bg-muted/50 p-3">{lastFailedActivity.aiReasoning}</p>
            </div>
          )}
          <div className="flex gap-2">
            <OverrideActions ticketId={ticket.id} currentActivityId={ticket.currentActivityId} aiFailureCount={ticket.aiFailureCount} />
          </div>
        </CardContent>
      </Card>
    )
  }

  // --- State: Pending Approval ---
  if (state === "pending_approval" && currentActivity) {
    return <AIDecisionPanel ticketId={ticket.id} activity={currentActivity} />
  }

  if (currentActivity?.status === "pending_approval") {
	return (
	  <Card>
	    <CardHeader className="pb-3">
	      <CardTitle className="flex items-center gap-2 text-base">
	        <Bot className="h-4 w-4" />
	        {t("smart.aiDecision")}
	        <Badge variant="outline" className="ml-auto text-yellow-600 border-yellow-300">
	          {t("smart.pendingApproval")}
	        </Badge>
	      </CardTitle>
	    </CardHeader>
	    <CardContent>
	      <p className="text-sm text-muted-foreground">
	        {t("smart.pendingApprovalReadonly", { defaultValue: "当前 AI 决策正在等待有权限的处理人确认。" })}
	      </p>
	    </CardContent>
	  </Card>
	)
  }

  // --- State: Human Activity ---
  if (state === "human_activity" && activeHumanActivity) {
    return (
      <Card>
        <CardHeader className="pb-3">
          <CardTitle className="flex items-center gap-2 text-base">
            <FileText className="h-4 w-4" />
            {activeHumanActivity.name}
            <Badge variant="outline" className="ml-auto">{activeHumanActivity.activityType}</Badge>
          </CardTitle>
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="grid gap-2 text-sm">
            <div className="flex justify-between">
              <span className="text-muted-foreground">{t("smart.activityType")}</span>
              <span>{activeHumanActivity.activityType}</span>
            </div>
            {activeHumanActivity.executionMode && (
              <div className="flex justify-between">
                <span className="text-muted-foreground">{t("smart.executionMode")}</span>
                <span>{activeHumanActivity.executionMode}</span>
              </div>
            )}
            {activeHumanActivity.startedAt && (
              <div className="flex justify-between">
                <span className="text-muted-foreground">{t("smart.startedAt")}</span>
                <span>{new Date(activeHumanActivity.startedAt).toLocaleString()}</span>
              </div>
            )}
          </div>
          {activeHumanActivity.formData ? <FormDataDisplay data={activeHumanActivity.formData} /> : null}
          {/* Action buttons only for the assignee */}
          {ticket.assigneeId === currentUserId && (
            <HumanActivityActions ticketId={ticket.id} activity={activeHumanActivity} />
          )}
          {!ticket.assigneeId && (
            <p className="text-sm text-muted-foreground pt-1">
              {t("smart.waitingAssignment", { defaultValue: "等待分配处理人" })}
            </p>
          )}
        </CardContent>
      </Card>
    )
  }

  // --- State: AI Reasoning ---
  if (state === "ai_reasoning") {
    return (
      <Card>
        <CardHeader className="pb-3">
          <CardTitle className="flex items-center gap-2 text-base">
            <Bot className="h-4 w-4" />
            {t("smart.aiReasoning")}
          </CardTitle>
        </CardHeader>
        <CardContent>
          {pollTimedOut ? (
            <div className="flex items-center gap-2 text-sm text-amber-600">
              <AlertTriangle className="h-4 w-4" />
              {t("smart.aiReasoningTimeout")}
              <Button
                variant="ghost"
                size="sm"
                onClick={() => {
                  setPollTimedOut(false)
                  queryClient.invalidateQueries({ queryKey: ["itsm-ticket", ticket.id] })
                  queryClient.invalidateQueries({ queryKey: ["itsm-ticket-activities", ticket.id] })
                }}
              >
                {t("smart.refresh")}
              </Button>
            </div>
          ) : (
            <div className="flex items-center gap-3 text-sm text-muted-foreground">
              <Loader2 className="h-4 w-4 animate-spin" />
              {t("smart.aiReasoningDesc")}
            </div>
          )}
        </CardContent>
      </Card>
    )
  }

  return (
    <Card>
      <CardHeader className="pb-3">
        <CardTitle className="flex items-center gap-2 text-base text-muted-foreground">
          <Clock className="h-4 w-4" />
          {t("smart.idleState", { defaultValue: "AI 正在准备下一步" })}
        </CardTitle>
      </CardHeader>
    </Card>
  )
}

// --- Human Activity Action Buttons ---
import { useMutation } from "@tanstack/react-query"
import { toast } from "sonner"
import { progressTicket } from "../api"

function HumanActivityActions({ ticketId, activity }: { ticketId: number; activity: ActivityItem }) {
  const { t } = useTranslation("itsm")
  const queryClient = useQueryClient()

  const progressMut = useMutation({
    mutationFn: (outcome: string) => progressTicket(ticketId, { activityId: activity.id, outcome }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["itsm-ticket", ticketId] })
      queryClient.invalidateQueries({ queryKey: ["itsm-ticket-activities", ticketId] })
      queryClient.invalidateQueries({ queryKey: ["itsm-ticket-timeline", ticketId] })
      toast.success(t("tickets.progressSuccess"))
    },
    onError: (err) => toast.error(err.message),
  })

  if (activity.activityType === "approve") {
    return (
      <div className="flex gap-2 pt-1">
        <Button size="sm" onClick={() => progressMut.mutate("approved")} disabled={progressMut.isPending}>
          <CheckCircle2 className="mr-1 h-3.5 w-3.5" />
          {t("approval.approve")}
        </Button>
        <Button size="sm" variant="outline" onClick={() => progressMut.mutate("rejected")} disabled={progressMut.isPending}>
          <XCircle className="mr-1 h-3.5 w-3.5" />
          {t("approval.deny")}
        </Button>
      </div>
    )
  }

  return (
    <div className="flex gap-2 pt-1">
      <Button size="sm" onClick={() => progressMut.mutate("completed")} disabled={progressMut.isPending}>
        <CheckCircle2 className="mr-1 h-3.5 w-3.5" />
        {t("smart.submit")}
      </Button>
    </div>
  )
}
