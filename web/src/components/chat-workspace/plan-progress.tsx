import { useState, useCallback } from "react"
import { useTranslation } from "react-i18next"
import { ChevronRight, ChevronDown, CheckCircle2, Circle, Loader2 } from "lucide-react"

interface PlanStep {
  description: string
  durationMs?: number
}

interface PlanProgressProps {
  steps: PlanStep[]
  currentStepIndex: number
  isStreaming: boolean
  totalDurationMs?: number
}

export function PlanProgress({ steps, currentStepIndex, isStreaming, totalDurationMs }: PlanProgressProps) {
  const { t } = useTranslation(["ai"])
  const [userCollapsed, setUserCollapsed] = useState(false)
  const isComplete = !isStreaming && currentStepIndex >= steps.length

  // Auto-collapse when complete, unless user has expanded
  const expanded = userCollapsed ? false : !isComplete

  const handleToggle = useCallback(() => {
    setUserCollapsed(prev => !prev)
  }, [])

  if (steps.length === 0) return null

  const progress = isComplete ? steps.length : currentStepIndex

  return (
    <div className="mb-4 rounded-lg border bg-muted/20 px-3 py-2.5">
      <button
        type="button"
        className="flex items-center gap-2 text-xs font-medium text-muted-foreground hover:text-foreground transition-colors w-full"
        onClick={handleToggle}
      >
        {expanded ? <ChevronDown className="h-3.5 w-3.5 shrink-0" /> : <ChevronRight className="h-3.5 w-3.5 shrink-0" />}
        {isComplete ? (
          <span className="flex items-center gap-1.5">
            <CheckCircle2 className="h-3.5 w-3.5 text-green-500" />
            {t("ai:chat.planCompleted", {
              steps: steps.length,
              duration: totalDurationMs ? (totalDurationMs / 1000).toFixed(1) : "—",
            })}
          </span>
        ) : (
          <span className="flex items-center gap-1.5">
            {t("ai:chat.planTitle")}
            <span className="text-[10px] text-muted-foreground/60">
              {t("ai:chat.planProgress", { current: progress, total: steps.length })}
            </span>
          </span>
        )}
      </button>

      {expanded && (
        <div className="mt-2 ml-1 space-y-1.5">
          {steps.map((step, i) => {
            const isCompleted = i < currentStepIndex
            const isCurrent = i === currentStepIndex && isStreaming
            const isPending = i > currentStepIndex || (i === currentStepIndex && !isStreaming && !isComplete)

            return (
              <div key={i} className="flex items-start gap-2 text-xs">
                {isCompleted && <CheckCircle2 className="h-3.5 w-3.5 text-green-500 mt-0.5 shrink-0" />}
                {isCurrent && <Loader2 className="h-3.5 w-3.5 text-primary animate-spin mt-0.5 shrink-0" />}
                {isPending && !isCurrent && <Circle className="h-3.5 w-3.5 text-muted-foreground/40 mt-0.5 shrink-0" />}
                <span className={isCurrent ? "text-foreground" : isCompleted ? "text-muted-foreground" : "text-muted-foreground/60"}>
                  {step.description}
                </span>
                {isCompleted && step.durationMs != null && (
                  <span className="ml-auto text-[10px] text-muted-foreground/50 shrink-0">
                    {(step.durationMs / 1000).toFixed(1)}s
                  </span>
                )}
              </div>
            )
          })}

          {/* Progress bar */}
          <div className="mt-2 h-1 rounded-full bg-muted overflow-hidden">
            <div
              className="h-full rounded-full bg-primary transition-all duration-300"
              style={{ width: `${(progress / steps.length) * 100}%` }}
            />
          </div>
        </div>
      )}
    </div>
  )
}
