"use client"

import { useEffect, useMemo, useState } from "react"
import { useTranslation } from "react-i18next"
import { useNavigate } from "react-router"
import { RefreshCw, Search, Ticket } from "lucide-react"
import { useListPage } from "@/hooks/use-list-page"
import { withActiveMenuPermission } from "@/lib/navigation-state"
import { Button } from "@/components/ui/button"
import { ButtonGroup } from "@/components/ui/button-group"
import { Input } from "@/components/ui/input"
import {
  DataTableCard, DataTableEmptyRow, DataTableLoadingRow, DataTablePagination,
  DataTableToolbar, DataTableToolbarGroup,
} from "@/components/ui/data-table"
import {
  Table, TableBody, TableCell, TableHead, TableHeader, TableRow,
} from "@/components/ui/table"
import { type TicketItem } from "../../../api"
import { SLABadge } from "../../../components/sla-badge"
import { TicketStatusBadge } from "../../../components/ticket-status-badge"
import { TICKET_MENU_PERMISSION } from "../navigation"

export function Component() {
  const { t } = useTranslation(["itsm", "common"])
  const navigate = useNavigate()
  const [statusGroup, setStatusGroup] = useState<"active" | "terminal" | "all">("active")
  const [startDate, setStartDate] = useState("")
  const [endDate, setEndDate] = useState("")

  const extraParams = useMemo(() => {
    const params: Record<string, string> = {}
    if (statusGroup !== "all") params.status = statusGroup
    if (startDate) params.startDate = startDate
    if (endDate) params.endDate = endDate
    return params
  }, [statusGroup, startDate, endDate])

  const {
    keyword, setKeyword, handleSearch,
    page, setPage, items, total, totalPages, isLoading, isFetching, refetch,
  } = useListPage<TicketItem>({
    queryKey: "itsm-tickets-mine",
    endpoint: "/api/v1/itsm/tickets/mine",
    extraParams,
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
          <h2 className="workspace-page-title">{t("itsm:tickets.mine")}</h2>
          <p className="workspace-page-description">{t("itsm:tickets.mineDesc")}</p>
        </div>
      </div>

      <DataTableToolbar>
        <DataTableToolbarGroup className="min-w-0 flex-1">
          <form onSubmit={handleSearch} className="flex min-w-0 w-full flex-col gap-2 sm:flex-row sm:items-center">
            <div className="relative w-full sm:max-w-md">
              <Search className="absolute left-2.5 top-2.5 h-4 w-4 text-muted-foreground" />
              <Input
                className="pl-8"
                placeholder={t("itsm:tickets.searchPlaceholder")}
                value={keyword}
                onChange={(e) => setKeyword(e.target.value)}
              />
            </div>
            <Input
              type="date"
              value={startDate}
              onChange={(e) => {
                setStartDate(e.target.value)
                setPage(1)
              }}
              className="w-[160px]"
              placeholder={t("itsm:tickets.startDate")}
              aria-label={t("itsm:tickets.startDate")}
            />
            <Input
              type="date"
              value={endDate}
              onChange={(e) => {
                setEndDate(e.target.value)
                setPage(1)
              }}
              className="w-[160px]"
              placeholder={t("itsm:tickets.endDate")}
              aria-label={t("itsm:tickets.endDate")}
            />
          </form>
        </DataTableToolbarGroup>
        <DataTableToolbarGroup className="flex-none justify-start sm:justify-end">
          <Button
            type="button"
            variant="outline"
            size="icon"
            aria-label={t("common:refresh")}
            onClick={() => void refetch()}
            disabled={isFetching}
          >
            <RefreshCw className={`h-4 w-4 ${isFetching ? "animate-spin" : ""}`} />
          </Button>
          <ButtonGroup>
            <Button
              type="button"
              size="sm"
              variant={statusGroup === "active" ? "default" : "outline"}
              onClick={() => { setStatusGroup("active"); setPage(1) }}
            >
              {t("itsm:tickets.groupActive")}
            </Button>
            <Button
              type="button"
              size="sm"
              variant={statusGroup === "terminal" ? "default" : "outline"}
              onClick={() => { setStatusGroup("terminal"); setPage(1) }}
            >
              {t("itsm:tickets.groupTerminal")}
            </Button>
            <Button
              type="button"
              size="sm"
              variant={statusGroup === "all" ? "default" : "outline"}
              onClick={() => { setStatusGroup("all"); setPage(1) }}
            >
              {t("itsm:tickets.groupAll")}
            </Button>
          </ButtonGroup>
        </DataTableToolbarGroup>
      </DataTableToolbar>

      <DataTableCard className="overflow-x-auto">
        <Table className="min-w-[1180px]">
          <TableHeader>
            <TableRow>
              <TableHead className="w-[132px] whitespace-nowrap">{t("itsm:tickets.code")}</TableHead>
              <TableHead className="min-w-[260px] whitespace-nowrap">{t("itsm:tickets.ticketTitle")}</TableHead>
              <TableHead className="w-[180px] whitespace-nowrap">{t("itsm:tickets.service")}</TableHead>
              <TableHead className="w-[112px] whitespace-nowrap">{t("itsm:tickets.priority")}</TableHead>
              <TableHead className="w-[120px] whitespace-nowrap">{t("itsm:tickets.status")}</TableHead>
              <TableHead className="w-[148px] whitespace-nowrap">{t("itsm:tickets.currentOwner")}</TableHead>
              <TableHead className="w-[136px] whitespace-nowrap">{t("itsm:tickets.slaStatus")}</TableHead>
              <TableHead className="w-[176px] whitespace-nowrap">{t("itsm:tickets.updatedAt")}</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {isLoading ? (
              <DataTableLoadingRow colSpan={8} />
            ) : items.length === 0 ? (
              <DataTableEmptyRow colSpan={8} icon={Ticket} title={t("itsm:tickets.mineEmpty")} />
            ) : (
              items.map((item) => (
                <TableRow
                  data-testid={`itsm-ticket-row-${item.code}`}
                  key={item.id}
                  className="cursor-pointer"
                  onClick={() => navigate(`/itsm/tickets/${item.id}`, { state: withActiveMenuPermission(TICKET_MENU_PERMISSION.mine) })}
                >
                  <TableCell className="font-mono text-sm whitespace-nowrap">{item.code}</TableCell>
                  <TableCell className="max-w-[320px] font-medium whitespace-nowrap truncate" title={item.title}>{item.title}</TableCell>
                  <TableCell className="max-w-[220px] text-sm text-muted-foreground whitespace-nowrap truncate" title={item.serviceName}>{item.serviceName}</TableCell>
                  <TableCell>
                    <span className="inline-flex items-center gap-1.5 whitespace-nowrap text-sm">
                      <span className="inline-block h-2.5 w-2.5 rounded-full" style={{ backgroundColor: item.priorityColor }} />
                      {item.priorityName}
                    </span>
                  </TableCell>
                  <TableCell className="whitespace-nowrap">
                    <TicketStatusBadge ticket={item} />
                  </TableCell>
                  <TableCell className="max-w-[180px] text-sm whitespace-nowrap truncate" title={item.currentOwnerName || item.assigneeName || "—"}>
                    {item.currentOwnerName || item.assigneeName || "—"}
                  </TableCell>
                  <TableCell className="whitespace-nowrap">
                    <SLABadge slaStatus={item.slaStatus} slaResolutionDeadline={item.slaResolutionDeadline} />
                  </TableCell>
                  <TableCell className="text-sm text-muted-foreground whitespace-nowrap">{new Date(item.updatedAt).toLocaleString()}</TableCell>
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
