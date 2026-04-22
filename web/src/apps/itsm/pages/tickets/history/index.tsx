"use client"

import { useState, useMemo } from "react"
import { useTranslation } from "react-i18next"
import { useNavigate } from "react-router"
import { Ticket, Search } from "lucide-react"
import { useListPage } from "@/hooks/use-list-page"
import { withActiveMenuPermission } from "@/lib/navigation-state"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Badge } from "@/components/ui/badge"
import {
  DataTableCard, DataTableEmptyRow, DataTableLoadingRow, DataTablePagination,
  DataTableToolbar, DataTableToolbarGroup,
} from "@/components/ui/data-table"
import {
  Table, TableBody, TableCell, TableHead, TableHeader, TableRow,
} from "@/components/ui/table"
import { type TicketItem } from "../../../api"
import { SLABadge } from "../../../components/sla-badge"
import { TICKET_MENU_PERMISSION } from "../navigation"

const STATUS_MAP: Record<string, { variant: "default" | "secondary" | "destructive"; key: string }> = {
  completed: { variant: "default", key: "statusCompleted" },
  failed: { variant: "destructive", key: "statusFailed" },
  cancelled: { variant: "secondary", key: "statusCancelled" },
}

export function Component() {
  const { t } = useTranslation(["itsm", "common"])
  const navigate = useNavigate()
  const [startDate, setStartDate] = useState("")
  const [endDate, setEndDate] = useState("")

  const extraParams = useMemo(() => {
    const params: Record<string, string> = {}
    if (startDate) params.startDate = startDate
    if (endDate) params.endDate = endDate
    return params
  }, [startDate, endDate])

  const {
    keyword, setKeyword, page, setPage,
    items, total, totalPages, isLoading, handleSearch,
  } = useListPage<TicketItem>({
    queryKey: "itsm-tickets-history",
    endpoint: "/api/v1/itsm/tickets/history",
    extraParams,
  })

  return (
    <div className="space-y-4">
      <h2 className="text-lg font-semibold">{t("itsm:tickets.history")}</h2>

      <DataTableToolbar>
        <DataTableToolbarGroup>
          <form onSubmit={handleSearch} className="flex w-full flex-col gap-2 sm:flex-row sm:items-center sm:flex-wrap">
            <div className="relative w-full sm:max-w-sm">
              <Search className="absolute left-2.5 top-2.5 h-4 w-4 text-muted-foreground" />
              <Input placeholder={t("itsm:tickets.searchPlaceholder")} value={keyword} onChange={(e) => setKeyword(e.target.value)} className="pl-8" />
            </div>
            <Input type="date" value={startDate} onChange={(e) => setStartDate(e.target.value)} className="w-[160px]" placeholder={t("itsm:tickets.startDate")} />
            <Input type="date" value={endDate} onChange={(e) => setEndDate(e.target.value)} className="w-[160px]" placeholder={t("itsm:tickets.endDate")} />
            <Button type="submit" variant="outline" size="sm">{t("common:search")}</Button>
          </form>
        </DataTableToolbarGroup>
      </DataTableToolbar>

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
              <TableHead className="w-[140px]">{t("itsm:tickets.finishedAt")}</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {isLoading ? (
              <DataTableLoadingRow colSpan={8} />
            ) : items.length === 0 ? (
              <DataTableEmptyRow colSpan={8} icon={Ticket} title={t("itsm:tickets.empty")} />
            ) : (
              items.map((item) => {
                const statusInfo = STATUS_MAP[item.status] ?? { variant: "secondary" as const, key: "statusCompleted" }
                return (
                  <TableRow
                    key={item.id}
                    className="cursor-pointer"
                    onClick={() => navigate(`/itsm/tickets/${item.id}`, { state: withActiveMenuPermission(TICKET_MENU_PERMISSION.history) })}
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
                      <Badge variant={statusInfo.variant}>{t(`itsm:tickets.${statusInfo.key}`)}</Badge>
                    </TableCell>
                    <TableCell className="text-sm text-muted-foreground">{item.serviceName}</TableCell>
                    <TableCell className="text-sm">{item.assigneeName || "—"}</TableCell>
                    <TableCell>
                      <SLABadge slaStatus={item.slaStatus} finalOnly />
                    </TableCell>
                    <TableCell className="text-sm text-muted-foreground">{item.finishedAt ? new Date(item.finishedAt).toLocaleString() : "—"}</TableCell>
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
