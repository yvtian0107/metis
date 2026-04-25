"use client"

import { useEffect, useMemo, useState } from "react"
import { useTranslation } from "react-i18next"
import { useNavigate } from "react-router"
import { ClipboardCheck, History, RefreshCw, Search } from "lucide-react"
import { useListPage } from "@/hooks/use-list-page"
import { withActiveMenuPermission } from "@/lib/navigation-state"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import {
  DataTableCard,
  DataTableEmptyRow,
  DataTableLoadingRow,
  DataTablePagination,
  DataTableToolbar,
  DataTableToolbarGroup,
} from "@/components/ui/data-table"
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table"
import { type TicketItem } from "../../../api"
import { SLABadge } from "../../../components/sla-badge"
import { TicketStatusBadge } from "../../../components/ticket-status-badge"
import { TICKET_MENU_PERMISSION } from "../navigation"

type ApprovalListMode = "pending" | "history"

export function ApprovalListPage({ mode }: { mode: ApprovalListMode }) {
  const { t } = useTranslation(["itsm", "common"])
  const navigate = useNavigate()
  const [scope] = useState(mode)

  const config = useMemo(() => {
    if (scope === "pending") {
      return {
        endpoint: "/api/v1/itsm/tickets/approvals/pending",
        icon: ClipboardCheck,
        permission: TICKET_MENU_PERMISSION.approvalPending,
        queryKey: "itsm-ticket-approval-pending",
        title: t("itsm:tickets.approvalPending"),
        description: t("itsm:tickets.approvalPendingDesc"),
        empty: t("itsm:tickets.approvalPendingEmpty"),
      }
    }
    return {
      endpoint: "/api/v1/itsm/tickets/approvals/history",
      icon: History,
      permission: TICKET_MENU_PERMISSION.approvalHistory,
      queryKey: "itsm-ticket-approval-history",
      title: t("itsm:tickets.approvalHistory"),
      description: t("itsm:tickets.approvalHistoryDesc"),
      empty: t("itsm:tickets.approvalHistoryEmpty"),
    }
  }, [scope, t])

  const {
    keyword,
    setKeyword,
    handleSearch,
    page,
    setPage,
    items,
    total,
    totalPages,
    isLoading,
    isFetching,
    refetch,
  } = useListPage<TicketItem>({
    queryKey: config.queryKey,
    endpoint: config.endpoint,
  })

  useEffect(() => {
    const interval = window.setInterval(() => {
      void refetch()
    }, 60000)
    return () => window.clearInterval(interval)
  }, [refetch])

  return (
    <div className="workspace-page">
      <div className="workspace-page-header">
        <div>
          <h2 className="workspace-page-title">{config.title}</h2>
          <p className="workspace-page-description">{config.description}</p>
        </div>
      </div>

      <DataTableToolbar>
        <DataTableToolbarGroup>
          <form onSubmit={handleSearch} className="flex w-full flex-col gap-2 sm:flex-row sm:items-center">
            <div className="relative w-full sm:max-w-sm">
              <Search className="absolute left-2.5 top-2.5 h-4 w-4 text-muted-foreground" />
              <Input
                className="pl-8"
                placeholder={t("itsm:tickets.searchPlaceholder")}
                value={keyword}
                onChange={(e) => setKeyword(e.target.value)}
              />
            </div>
            <Button type="submit" variant="outline" size="sm">{t("common:search")}</Button>
            <Button type="button" variant="outline" size="icon" aria-label={t("common:refresh")} onClick={() => void refetch()} disabled={isFetching}>
              <RefreshCw className={`h-4 w-4 ${isFetching ? "animate-spin" : ""}`} />
            </Button>
          </form>
        </DataTableToolbarGroup>
      </DataTableToolbar>

      <DataTableCard>
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead className="w-[150px]">{t("itsm:tickets.code")}</TableHead>
              <TableHead className="min-w-[240px]">{t("itsm:tickets.ticketTitle")}</TableHead>
              <TableHead className="w-[110px]">{t("itsm:tickets.priority")}</TableHead>
              <TableHead className="w-[110px]">{t("itsm:tickets.status")}</TableHead>
              <TableHead className="w-[140px]">{t("itsm:tickets.service")}</TableHead>
              <TableHead className="w-[130px]">{t("itsm:tickets.currentOwner")}</TableHead>
              <TableHead className="min-w-[180px]">{t("itsm:tickets.nextStep")}</TableHead>
              <TableHead className="w-[150px]">{t("itsm:tickets.slaStatus")}</TableHead>
              <TableHead className="w-[150px]">{t("itsm:tickets.createdAt")}</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {isLoading ? (
              <DataTableLoadingRow colSpan={9} />
            ) : items.length === 0 ? (
              <DataTableEmptyRow colSpan={9} icon={config.icon} title={config.empty} />
            ) : (
              items.map((item) => (
                <TableRow
                  data-testid={`itsm-ticket-row-${item.code}`}
                  key={item.id}
                  className="cursor-pointer"
                  onClick={() => navigate(`/itsm/tickets/${item.id}`, { state: withActiveMenuPermission(config.permission) })}
                >
                  <TableCell className="font-mono text-sm whitespace-nowrap">{item.code}</TableCell>
                  <TableCell className="font-medium">{item.title}</TableCell>
                  <TableCell>
                    <span className="inline-flex items-center gap-1.5 text-sm">
                      <span className="inline-block h-2.5 w-2.5 rounded-full" style={{ backgroundColor: item.priorityColor }} />
                      {item.priorityName}
                    </span>
                  </TableCell>
                  <TableCell><TicketStatusBadge ticket={item} /></TableCell>
                  <TableCell className="text-sm text-muted-foreground">{item.serviceName}</TableCell>
                  <TableCell className="text-sm">{item.currentOwnerName || item.assigneeName || "—"}</TableCell>
                  <TableCell className="text-sm text-muted-foreground">{item.nextStepSummary || "—"}</TableCell>
                  <TableCell><SLABadge slaStatus={item.slaStatus} slaResolutionDeadline={item.slaResolutionDeadline} /></TableCell>
                  <TableCell className="text-sm text-muted-foreground">{new Date(item.createdAt).toLocaleString()}</TableCell>
                </TableRow>
              ))
            )}
          </TableBody>
        </Table>
      </DataTableCard>

      <DataTablePagination total={total} page={page} totalPages={totalPages} onPageChange={setPage} />
    </div>
  )
}
