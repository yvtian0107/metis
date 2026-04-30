"use client"

import { useTranslation } from "react-i18next"
import { Badge } from "@/components/ui/badge"
import { type TicketItem } from "../api"
import { getTicketStatusView } from "./ticket-status"

export function TicketStatusBadge({ ticket }: { ticket: TicketItem }) {
  const { t } = useTranslation("itsm")
  const status = getTicketStatusView(ticket)

  return (
    <Badge variant={status.variant}>
      {status.key ? t(`tickets.${status.key}`, { defaultValue: status.label }) : status.label}
    </Badge>
  )
}
