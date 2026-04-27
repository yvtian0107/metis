"use client"

import { useEffect, useMemo, useState } from "react"
import { useTranslation } from "react-i18next"
import { useNavigate } from "react-router"
import { Bot, RefreshCw, Search, Ticket } from "lucide-react"
import { useListPage } from "@/hooks/use-list-page"
import { withActiveMenuPermission } from "@/lib/navigation-state"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Tabs, TabsList, TabsTrigger } from "@/components/ui/tabs"
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
  const [statusTab, setStatusTab] = useState("")
  const [startDate, setStartDate] = useState("")
  const [endDate, setEndDate] = useState("")

  const extraParams = useMemo(() => {
    const params: Record<string, string> = {}
    if (statusTab) params.status = statusTab
    if (startDate) params.startDate = startDate
    if (endDate) params.endDate = endDate
    return params
  }, [statusTab, startDate, endDate])

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
        <DataTableToolbarGroup>
          <form onSubmit={handleSearch} className="flex w-full flex-col gap-2 sm:flex-row sm:items-center sm:flex-wrap">
            <div className="relative w-full sm:max-w-sm">
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
            <Button type="submit" variant="outline" size="sm">{t("common:search")}</Button>
            <Button type="button" variant="outline" size="icon" aria-label={t("common:refresh")} onClick={() => void refetch()} disabled={isFetching}>
              <RefreshCw className={`h-4 w-4 ${isFetching ? "animate-spin" : ""}`} />
            </Button>
          </form>
        </DataTableToolbarGroup>
      </DataTableToolbar>

      <Tabs value={statusTab || "all"} onValueChange={(v) => { setStatusTab(v === "all" ? "" : v); setPage(1) }}>
        <TabsList className="h-auto flex-wrap justify-start">
          <TabsTrigger value="all">{t("itsm:tickets.allStatuses")}</TabsTrigger>
          <TabsTrigger value="submitted">{t("itsm:tickets.statusSubmitted")}</TabsTrigger>
          <TabsTrigger value="waiting_human">{t("itsm:tickets.statusWaitingHuman")}</TabsTrigger>
          <TabsTrigger value="decisioning">{t("itsm:tickets.statusDecisioning")}</TabsTrigger>
          <TabsTrigger value="completed">{t("itsm:tickets.statusCompleted")}</TabsTrigger>
          <TabsTrigger value="rejected">{t("itsm:tickets.statusRejected")}</TabsTrigger>
          <TabsTrigger value="failed">{t("itsm:tickets.statusFailed")}</TabsTrigger>
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
              <TableHead className="w-[90px]">{t("itsm:services.engineType")}</TableHead>
              <TableHead className="w-[80px]">{t("itsm:tickets.assignee")}</TableHead>
              <TableHead className="w-[100px]">{t("itsm:tickets.slaStatus")}</TableHead>
              <TableHead className="w-[140px]">{t("itsm:tickets.createdAt")}</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {isLoading ? (
              <DataTableLoadingRow colSpan={9} />
            ) : items.length === 0 ? (
              <DataTableEmptyRow colSpan={9} icon={Ticket} title={t("itsm:tickets.empty")} />
            ) : (
              items.map((item) => (
                <TableRow
                  data-testid={`itsm-ticket-row-${item.code}`}
                  key={item.id}
                  className="cursor-pointer"
                  onClick={() => navigate(`/itsm/tickets/${item.id}`, { state: withActiveMenuPermission(TICKET_MENU_PERMISSION.mine) })}
                >
                  <TableCell className="font-mono text-sm">{item.code}</TableCell>
                  <TableCell className="font-medium">{item.title}</TableCell>
                  <TableCell>
                    <span className="inline-flex items-center gap-1.5 text-sm">
                      <span className="inline-block h-2.5 w-2.5 rounded-full" style={{ backgroundColor: item.priorityColor }} />
                      {item.priorityName}
                    </span>
                  </TableCell>
                  <TableCell>
                    <TicketStatusBadge ticket={item} />
                  </TableCell>
                  <TableCell className="text-sm text-muted-foreground">{item.serviceName}</TableCell>
                  <TableCell>
                    {item.engineType === "smart" ? (
                      <Badge variant="outline" className="gap-1 border-amber-300 bg-amber-50 text-amber-700">
                        <Bot className="h-3 w-3" />
                        {t("itsm:services.engineSmart")}
                      </Badge>
                    ) : (
                      <span className="text-sm text-muted-foreground">{t("itsm:services.engineClassic")}</span>
                    )}
                  </TableCell>
                  <TableCell className="text-sm">{item.assigneeName || "—"}</TableCell>
                  <TableCell>
                    <SLABadge slaStatus={item.slaStatus} slaResolutionDeadline={item.slaResolutionDeadline} />
                  </TableCell>
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
