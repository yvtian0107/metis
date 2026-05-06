import { type TicketItem } from "../api"
import type { TicketStatus, StatusTone } from "../contract"

type BadgeVariant = "default" | "secondary" | "destructive" | "outline"

export const TICKET_STATUS_OPTIONS: Record<TicketStatus, { variant: BadgeVariant; key: string }> = {
  submitted: { variant: "secondary", key: "statusSubmitted" },
  waiting_human: { variant: "outline", key: "statusWaitingHuman" },
  approved_decisioning: { variant: "outline", key: "statusApprovedDecisioning" },
  rejected_decisioning: { variant: "outline", key: "statusRejectedDecisioning" },
  decisioning: { variant: "outline", key: "statusDecisioning" },
  executing_action: { variant: "outline", key: "statusExecutingAction" },
  completed: { variant: "default", key: "statusCompleted" },
  rejected: { variant: "destructive", key: "statusRejected" },
  withdrawn: { variant: "secondary", key: "statusWithdrawn" },
  cancelled: { variant: "secondary", key: "statusCancelled" },
  failed: { variant: "destructive", key: "statusFailed" },
}

const TONE_VARIANT: Record<StatusTone, BadgeVariant> = {
  success: "default",
  destructive: "destructive",
  secondary: "secondary",
  progress: "outline",
  warning: "outline",
}

export function getTicketStatusView(ticket: TicketItem) {
  const option = TICKET_STATUS_OPTIONS[ticket.status]
  if (!option) {
    throw new Error(`unknown ITSM ticket status: ${ticket.status}`)
  }
  const variant = TONE_VARIANT[ticket.statusTone] ?? option.variant
  return {
    key: option.key,
    variant,
    label: ticket.statusLabel,
  }
}
