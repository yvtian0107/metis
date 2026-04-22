"use client"

import { useTranslation } from "react-i18next"
import { Bot, User, ChevronRight } from "lucide-react"
import { Badge } from "@/components/ui/badge"
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"
import {
  Popover, PopoverContent, PopoverTrigger,
} from "@/components/ui/popover"
import { type ActivityItem } from "../api"

interface SmartFlowVisualizationProps {
  activities: ActivityItem[]
  currentActivityId: number | null
}

const STATUS_COLORS: Record<string, string> = {
  completed: "bg-green-500",
  in_progress: "bg-blue-500",
  pending: "bg-gray-400",
  cancelled: "bg-gray-300",
  failed: "bg-red-500",
}

function ConfidenceBadge({ confidence }: { confidence: number | null }) {
  if (confidence == null) return null
  const pct = Math.round(confidence * 100)
  const color = pct >= 80 ? "bg-green-100 text-green-700" : pct >= 50 ? "bg-yellow-100 text-yellow-700" : "bg-red-100 text-red-700"
  return <span className={`ml-1 inline-flex rounded-full px-1.5 py-0.5 text-[10px] font-medium ${color}`}>{pct}%</span>
}

export function SmartFlowVisualization({ activities, currentActivityId }: SmartFlowVisualizationProps) {
  const { t } = useTranslation("itsm")

  const sorted = [...activities].sort((a, b) => (a.sequenceOrder ?? 0) - (b.sequenceOrder ?? 0))

  if (sorted.length === 0) {
    return (
      <Card>
        <CardHeader>
          <CardTitle className="text-base">{t("smart.flowTitle")}</CardTitle>
        </CardHeader>
        <CardContent>
          <p className="text-sm text-muted-foreground">{t("smart.noActivities")}</p>
        </CardContent>
      </Card>
    )
  }

  return (
    <Card className="overflow-visible">
      <CardHeader>
        <CardTitle className="text-base">{t("smart.flowTitle")}</CardTitle>
      </CardHeader>
      <CardContent className="overflow-visible px-5 pb-5">
        <div className="flex min-h-[122px] items-start gap-1 overflow-x-auto overflow-y-visible px-3 py-3">
          {sorted.map((activity, idx) => {
            const isCurrent = activity.id === currentActivityId
            const isAI = activity.aiDecision != null || activity.aiConfidence != null
            const isOverridden = activity.overriddenBy != null

            return (
              <div key={activity.id} className="flex items-center">
                {idx > 0 && (
                  <ChevronRight className="mx-0.5 h-4 w-4 shrink-0 text-muted-foreground" />
                )}
                <Popover>
                  <PopoverTrigger asChild>
                    <button
                      type="button"
                      className={`flex flex-col items-center gap-1 rounded-lg border p-3 text-center transition-all min-w-[100px] ${
                        isCurrent
                          ? "ring-2 ring-blue-500 ring-offset-2 animate-pulse border-blue-300"
                          : activity.status === "completed"
                            ? "border-green-200 bg-green-50/50"
                            : activity.status === "cancelled"
                              ? "border-gray-200 opacity-60"
                              : "border-border"
                      }`}
                    >
                      <div className="flex items-center gap-1">
                        <div className={`h-2 w-2 rounded-full ${STATUS_COLORS[activity.status] ?? "bg-gray-400"}`} />
                        <span className="text-xs font-medium">{activity.name || activity.activityType}</span>
                      </div>
                      <div className="flex items-center gap-0.5">
                        {isAI && <Bot className="h-3 w-3 text-blue-500" />}
                        {isOverridden && <User className="h-3 w-3 text-orange-500" />}
                        {isAI && <ConfidenceBadge confidence={activity.aiConfidence} />}
                      </div>
                    </button>
                  </PopoverTrigger>
                  <PopoverContent className="w-72 text-sm" side="bottom">
                    <div className="space-y-2">
                      <div className="flex items-center justify-between">
                        <span className="font-medium">{activity.name || activity.activityType}</span>
                        <Badge variant="outline" className="text-xs">{activity.status}</Badge>
                      </div>
                      {activity.aiReasoning && (
                        <div>
                          <p className="text-xs text-muted-foreground mb-0.5">{t("smart.reasoning")}</p>
                          <p className="text-xs whitespace-pre-wrap">{activity.aiReasoning}</p>
                        </div>
                      )}
                      {activity.aiConfidence != null && (
                        <div className="flex justify-between text-xs">
                          <span className="text-muted-foreground">{t("smart.confidence")}</span>
                          <span>{Math.round(activity.aiConfidence * 100)}%</span>
                        </div>
                      )}
                      {activity.overriddenBy != null && (
                        <div className="flex justify-between text-xs">
                          <span className="text-muted-foreground">{t("smart.overriddenBy")}</span>
                          {/* TODO: API should return overriderName field for display */}
                          <span>{t("smart.overridden", { defaultValue: "已覆盖" })}</span>
                        </div>
                      )}
                      {activity.finishedAt && (
                        <div className="flex justify-between text-xs">
                          <span className="text-muted-foreground">{t("smart.finishedAt")}</span>
                          <span>{new Date(activity.finishedAt).toLocaleString()}</span>
                        </div>
                      )}
                    </div>
                  </PopoverContent>
                </Popover>
              </div>
            )
          })}
        </div>
      </CardContent>
    </Card>
  )
}
