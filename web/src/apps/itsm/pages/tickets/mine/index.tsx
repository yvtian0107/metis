"use client"

import { useState, useMemo } from "react"
import { useTranslation } from "react-i18next"
import { useNavigate } from "react-router"
import { Ticket } from "lucide-react"
import { useListPage } from "@/hooks/use-list-page"
import { Badge } from "@/components/ui/badge"
import { Tabs, TabsList, TabsTrigger } from "@/components/ui/tabs"
import {
  DataTableCard, DataTableEmptyRow, DataTableLoadingRow, DataTablePagination,
} from "@/components/ui/data-table"
import {
  Table, TableBody, TableCell, TableHead, TableHeader, TableRow,
} from "@/components/ui/table"
import { type TicketItem } from "../../api"
import { SLABadge } from "../../components/sla-badge"

const STATUS_MAP: Record<string, { variant: "default" | "secondary" | "destructive" | "outline"; key: string }> = {
  pending: { variant: "secondary", key: "statusPending" },
  in_progress: { variant: "default", key: "statusInProgress" },
  waiting_approval: { variant: "outline", key: "statusWaitingApproval" },
  waiting_action: { variant: "outline", key: "statusWaitingAction" },
  completed: { variant: "default", key: "statusCompleted" },
  failed: { variant: "destructive", key: "statusFailed" },
  cancelled: { variant: "secondary", key: "statusCancelled" },
}

export function Component() {
  const { t } = useTranslation(["itsm", "common"])
  const navigate = useNavigate()
  const [statusTab, setStatusTab] = useState("")

  const extraParams = useMemo(() => {
    const params: Record<string, string> = {}
    if (statusTab) params.status = statusTab
    return params
  }, [statusTab])

  const {
    page, setPage, items, total, totalPages, isLoading,
  } = useListPage<TicketItem>({
    queryKey: "itsm-tickets-mine",
    endpoint: "/api/v1/itsm/tickets/mine",
    extraParams,
  })

  return (
    <div className="space-y-4">
      <h2 className="text-lg font-semibold">{t("itsm:tickets.mine")}</h2>

      <Tabs value={statusTab || "all"} onValueChange={(v) => { setStatusTab(v === "all" ? "" : v); setPage(1) }}>
        <TabsList>
          <TabsTrigger value="all">{t("itsm:tickets.allStatuses")}</TabsTrigger>
          <TabsTrigger value="pending">{t("itsm:tickets.statusPending")}</TabsTrigger>
          <TabsTrigger value="in_progress">{t("itsm:tickets.statusInProgress")}</TabsTrigger>
          <TabsTrigger value="completed">{t("itsm:tickets.statusCompleted")}</TabsTrigger>
          <TabsTrigger value="cancelled">{t("itsm:tickets.statusCancelled")}</TabsTrigger>
        </TabsList>
      </Tabs>

      <DataTableCard>
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead className="w-[120px]">{t("itsm:tickets.code")}</TableHead>
              <TableHead className="min-w-[200px]">{t("itsm:tickets.ticketTitle")}</TableHead>
              <TableHead className="w-[100px]">{t("itsm:tickets.priority")}</TableHead>
              <TableHead className="w-[100px]">{t("itsm:tickets.status")}</TableHead>
              <TableHead className="w-[100px]">{t("itsm:tickets.service")}</TableHead>
              <TableHead className="w-[80px]">{t("itsm:tickets.assignee")}</TableHead>
              <TableHead className="w-[100px]">{t("itsm:tickets.slaStatus")}</TableHead>
              <TableHead className="w-[140px]">{t("itsm:tickets.createdAt")}</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {isLoading ? (
              <DataTableLoadingRow colSpan={8} />
            ) : items.length === 0 ? (
              <DataTableEmptyRow colSpan={8} icon={Ticket} title={t("itsm:tickets.empty")} />
            ) : (
              items.map((item) => {
                const statusInfo = STATUS_MAP[item.status] ?? { variant: "secondary" as const, key: "statusPending" }
                return (
                  <TableRow key={item.id} className="cursor-pointer" onClick={() => navigate(`/itsm/tickets/${item.id}`)}>
                    <TableCell className="font-mono text-sm">{item.code}</TableCell>
                    <TableCell className="font-medium">{item.title}</TableCell>
                    <TableCell>
                      <span className="inline-flex items-center gap-1.5 text-sm">
                        <span className="inline-block h-2.5 w-2.5 rounded-full" style={{ backgroundColor: item.priorityColor }} />
                        {item.priorityName}
                      </span>
                    </TableCell>
                    <TableCell>
                      <Badge variant={statusInfo.variant}>{t(`itsm:tickets.${statusInfo.key}`)}</Badge>
                    </TableCell>
                    <TableCell className="text-sm text-muted-foreground">{item.serviceName}</TableCell>
                    <TableCell className="text-sm">{item.assigneeName || "—"}</TableCell>
                    <TableCell>
                      <SLABadge slaStatus={item.slaStatus} slaResolutionDeadline={item.slaResolutionDeadline} />
                    </TableCell>
                    <TableCell className="text-sm text-muted-foreground">{new Date(item.createdAt).toLocaleString()}</TableCell>
                  </TableRow>
                )
              })
            )}
          </TableBody>
        </Table>
      </DataTableCard>

      <DataTablePagination total={total} page={page} totalPages={totalPages} onPageChange={setPage} />
    </div>
  )
}
