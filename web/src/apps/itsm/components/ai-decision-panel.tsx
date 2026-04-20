"use client"

import { useState } from "react"
import { useTranslation } from "react-i18next"
import { useMutation, useQueryClient } from "@tanstack/react-query"
import { Bot, ThumbsUp, ThumbsDown, Loader2 } from "lucide-react"
import { toast } from "sonner"
import { Button } from "@/components/ui/button"
import { Badge } from "@/components/ui/badge"
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"
import { Progress } from "@/components/ui/progress"
import {
  AlertDialog, AlertDialogAction, AlertDialogCancel, AlertDialogContent,
  AlertDialogDescription, AlertDialogFooter, AlertDialogHeader, AlertDialogTitle,
} from "@/components/ui/alert-dialog"
import { type ActivityItem, confirmActivity, rejectActivity } from "../api"

interface AIDecisionPanelProps {
  ticketId: number
  activity: ActivityItem
}

interface DecisionPlan {
  next_step_type: string
  next_step_name: string
  participant_id?: number
  participant_name?: string
  action_id?: number
  reasoning: string
}

export function AIDecisionPanel({ ticketId, activity }: AIDecisionPanelProps) {
  const { t } = useTranslation("itsm")
  const queryClient = useQueryClient()
  const [rejectDialogOpen, setRejectDialogOpen] = useState(false)

  const confidencePercent = (activity.confidence ?? 0) * 100
  const isPendingApproval = activity.status === "pending_approval"
  const canAct = activity.canAct

  let plan: DecisionPlan | null = null
  if (activity.aiDecision) {
    try {
      plan = JSON.parse(activity.aiDecision)
    } catch {
      // ignore
    }
  }

  const confirmMut = useMutation({
    mutationFn: () => confirmActivity(ticketId, activity.id),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["itsm-ticket", ticketId] })
      queryClient.invalidateQueries({ queryKey: ["itsm-ticket-activities", ticketId] })
      queryClient.invalidateQueries({ queryKey: ["itsm-ticket-timeline", ticketId] })
      toast.success(t("smart.confirmSuccess"))
    },
    onError: (err) => toast.error(err.message),
  })

  const rejectMut = useMutation({
    mutationFn: () => rejectActivity(ticketId, activity.id, t("smart.humanRejected")),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["itsm-ticket", ticketId] })
      queryClient.invalidateQueries({ queryKey: ["itsm-ticket-activities", ticketId] })
      queryClient.invalidateQueries({ queryKey: ["itsm-ticket-timeline", ticketId] })
      toast.success(t("smart.rejectSuccess"))
    },
    onError: (err) => toast.error(err.message),
  })

  const confidenceColor = confidencePercent >= 80
    ? "text-green-600"
    : confidencePercent >= 50
      ? "text-yellow-600"
      : "text-red-600"

  return (
    <Card>
      <CardHeader className="pb-3">
        <CardTitle className="flex items-center gap-2 text-base">
          <Bot className="h-4 w-4" />
          {t("smart.aiDecision")}
          {isPendingApproval && (
            <Badge variant="outline" className="ml-auto text-yellow-600 border-yellow-300">
              {t("smart.pendingApproval")}
            </Badge>
          )}
        </CardTitle>
      </CardHeader>
      <CardContent className="space-y-4">
        {/* Confidence */}
        <div className="space-y-1.5">
          <div className="flex items-center justify-between text-sm">
            <span className="text-muted-foreground">{t("smart.confidence")}</span>
            <span className={`font-medium ${confidenceColor}`}>{confidencePercent.toFixed(0)}%</span>
          </div>
          <Progress value={confidencePercent} className="h-2" />
        </div>

        {/* AI Reasoning */}
        {activity.aiReasoning && (
          <div className="space-y-1">
            <p className="text-sm text-muted-foreground">{t("smart.reasoning")}</p>
            <p className="text-sm whitespace-pre-wrap rounded-md bg-muted/50 p-3">
              {activity.aiReasoning}
            </p>
          </div>
        )}

        {/* Decision Plan Summary */}
        {plan && (
          <div className="space-y-2 rounded-md border p-3">
            <p className="text-sm font-medium">{t("smart.planSummary")}</p>
            <div className="grid gap-1 text-sm">
              <div className="flex justify-between">
                <span className="text-muted-foreground">{t("smart.nextStep")}</span>
                <span>{plan.next_step_name || plan.next_step_type}</span>
              </div>
              {plan.participant_name && (
                <div className="flex justify-between">
                  <span className="text-muted-foreground">{t("smart.participant")}</span>
                  <span>{plan.participant_name}</span>
                </div>
              )}
            </div>
          </div>
        )}

        {/* Confirm / Reject buttons */}
        {isPendingApproval && canAct && (
          <div className="flex gap-2 pt-1">
            <Button
              size="sm"
              onClick={() => confirmMut.mutate()}
              disabled={confirmMut.isPending || rejectMut.isPending}
            >
              {confirmMut.isPending ? (
                <Loader2 className="mr-1 h-3.5 w-3.5 animate-spin" />
              ) : (
                <ThumbsUp className="mr-1 h-3.5 w-3.5" />
              )}
              {t("smart.confirm")}
            </Button>
            <Button
              size="sm"
              variant="outline"
              onClick={() => setRejectDialogOpen(true)}
              disabled={confirmMut.isPending || rejectMut.isPending}
            >
              {rejectMut.isPending ? (
                <Loader2 className="mr-1 h-3.5 w-3.5 animate-spin" />
              ) : (
                <ThumbsDown className="mr-1 h-3.5 w-3.5" />
              )}
              {t("smart.reject")}
            </Button>
          </div>
        )}
        {isPendingApproval && !canAct && (
          <p className="text-sm text-muted-foreground">
            {t("smart.pendingApprovalReadonly", { defaultValue: "当前 AI 决策正在等待有权限的处理人确认。" })}
          </p>
        )}
      </CardContent>

      {/* Reject Confirmation Dialog */}
      <AlertDialog open={rejectDialogOpen} onOpenChange={setRejectDialogOpen}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>{t("smart.rejectConfirmTitle")}</AlertDialogTitle>
            <AlertDialogDescription>
              {activity.name && <>{t("smart.nextStep")}: {activity.name}<br /></>}
              {activity.confidence != null && <>{t("smart.confidence")}: {(activity.confidence * 100).toFixed(0)}%<br /></>}
              {t("smart.rejectConfirmDesc")}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>{t("common:cancel")}</AlertDialogCancel>
            <AlertDialogAction onClick={() => rejectMut.mutate()}>{t("smart.confirmReject")}</AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </Card>
  )
}
