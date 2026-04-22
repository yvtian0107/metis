"use client"

import { useState, useMemo } from "react"
import { useTranslation } from "react-i18next"
import { useNavigate } from "react-router"
import { Ticket, Search, Bot } from "lucide-react"
import { useListPage } from "@/hooks/use-list-page"
import { withActiveMenuPermission } from "@/lib/navigation-state"
import { Badge } from "@/components/ui/badge"
import { Input } from "@/components/ui/input"
import { Tabs, TabsList, TabsTrigger } from "@/components/ui/tabs"
import {
  DataTableCard, DataTableEmptyRow, DataTableLoadingRow, DataTablePagination,
} from "@/components/ui/data-table"
import {
  Table, TableBody, TableCell, TableHead, TableHeader, TableRow,
} from "@/components/ui/table"
import { type TicketItem } from "../../../api"
import { SLABadge } from "../../../components/sla-badge"
import { TICKET_MENU_PERMISSION } from "../navigation"

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
    keyword, setKeyword, handleSearch,
    page, setPage, items, total, totalPages, isLoading,
  } = useListPage<TicketItem>({
    queryKey: "itsm-tickets-mine",
    endpoint: "/api/v1/itsm/tickets/mine",
    extraParams,
  })

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <h2 className="text-lg font-semibold">{t("itsm:tickets.mine")}</h2>
        <form onSubmit={handleSearch} className="flex items-center gap-2">
          <div className="relative">
            <Search className="absolute left-2.5 top-2.5 h-4 w-4 text-muted-foreground" />
            <Input
              className="w-64 pl-8"
              placeholder={t("itsm:tickets.searchPlaceholder")}
              value={keyword}
              onChange={(e) => setKeyword(e.target.value)}
            />
          </div>
        </form>
      </div>

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
              items.map((item) => {
                const statusInfo = STATUS_MAP[item.status] ?? { variant: "secondary" as const, key: "statusPending" }
                return (
                  <TableRow
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
                      <Badge variant={statusInfo.variant}>{t(`itsm:tickets.${statusInfo.key}`)}</Badge>
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
