import { useTranslation } from "react-i18next"
import { Badge } from "@/components/ui/badge"
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip"

const SLA_VARIANT: Record<string, "default" | "secondary" | "destructive"> = {
  on_track: "default",
  breached_response: "destructive",
  breached_resolution: "destructive",
  normal: "default",
  warning: "secondary",
  breached: "destructive",
}

const SLA_LABEL_KEY: Record<string, string> = {
  on_track: "tickets.slaOnTrack",
  breached_response: "tickets.slaBreachedResponse",
  breached_resolution: "tickets.slaBreachedResolution",
  normal: "tickets.slaNormal",
  warning: "tickets.slaWarning",
  breached: "tickets.slaBreached",
}

function formatRemaining(deadline: string | null): string | null {
  if (!deadline) return null
  const diff = new Date(deadline).getTime() - Date.now()
  if (diff <= 0) return null
  const hours = Math.floor(diff / 3600000)
  const minutes = Math.floor((diff % 3600000) / 60000)
  if (hours >= 24) {
    const days = Math.floor(hours / 24)
    return `${days}d ${hours % 24}h`
  }
  return `${hours}h ${minutes}m`
}

interface SLABadgeProps {
  slaStatus: string | null | undefined
  slaResolutionDeadline?: string | null
  /** If true, only show the final status label (no remaining time) */
  finalOnly?: boolean
}

export function SLABadge({ slaStatus, slaResolutionDeadline, finalOnly }: SLABadgeProps) {
  const { t } = useTranslation("itsm")

  if (!slaStatus) return <span className="text-muted-foreground">—</span>

  const variant = SLA_VARIANT[slaStatus] ?? "secondary"
  const label = t(SLA_LABEL_KEY[slaStatus] ?? "tickets.slaUnknown", { status: slaStatus })
  const remaining = finalOnly ? null : formatRemaining(slaResolutionDeadline ?? null)

  if (!remaining) {
    return <Badge variant={variant} className="whitespace-nowrap">{label}</Badge>
  }

  return (
    <Tooltip>
      <TooltipTrigger asChild>
        <Badge variant={variant} className="cursor-default whitespace-nowrap">
          {label}
          <span className="ml-1 font-normal opacity-80">({remaining})</span>
        </Badge>
      </TooltipTrigger>
      <TooltipContent>
        {t("tickets.slaResolutionDeadline")}: {new Date(slaResolutionDeadline!).toLocaleString()}
      </TooltipContent>
    </Tooltip>
  )
}
