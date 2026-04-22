"use client"

import { useState, useMemo } from "react"
import { useTranslation } from "react-i18next"
import { useNavigate } from "react-router"
import { useQuery } from "@tanstack/react-query"
import { Search, Ticket, Plus } from "lucide-react"
import { usePermission } from "@/hooks/use-permission"
import { useListPage } from "@/hooks/use-list-page"
import { withActiveMenuPermission } from "@/lib/navigation-state"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Badge } from "@/components/ui/badge"
import {
  Select, SelectContent, SelectItem, SelectTrigger, SelectValue,
} from "@/components/ui/select"
import {
  DataTableCard, DataTableEmptyRow, DataTableLoadingRow,
  DataTablePagination, DataTableToolbar, DataTableToolbarGroup,
} from "@/components/ui/data-table"
import {
  Table, TableBody, TableCell, TableHead, TableHeader, TableRow,
} from "@/components/ui/table"
import {
  type TicketItem, fetchPriorities, fetchServiceDefs,
} from "../../api"
import { TICKET_MENU_PERMISSION } from "./navigation"

const STATUS_MAP: Record<string, { variant: "default" | "secondary" | "destructive" | "outline"; key: string }> = {
  pending: { variant: "secondary", key: "statusPending" },
  in_progress: { variant: "default", key: "statusInProgress" },
  waiting_approval: { variant: "outline", key: "statusWaitingApproval" },
  waiting_action: { variant: "outline", key: "statusWaitingAction" },
  completed: { variant: "default", key: "statusCompleted" },
  failed: { variant: "destructive", key: "statusFailed" },
  cancelled: { variant: "secondary", key: "statusCancelled" },
}

const SLA_VARIANT: Record<string, "default" | "secondary" | "destructive"> = {
  normal: "default",
  warning: "secondary",
  breached: "destructive",
}

export function Component() {
  const { t } = useTranslation(["itsm", "common"])
  const navigate = useNavigate()
  const canCreate = usePermission("itsm:ticket:create")

  const [statusFilter, setStatusFilter] = useState("")
  const [priorityFilter, setPriorityFilter] = useState("")
  const [serviceFilter, setServiceFilter] = useState("")

  const extraParams = useMemo(() => {
    const params: Record<string, string> = {}
    if (statusFilter) params.status = statusFilter
    if (priorityFilter) params.priorityId = priorityFilter
    if (serviceFilter) params.serviceId = serviceFilter
    return params
  }, [statusFilter, priorityFilter, serviceFilter])

  const {
    keyword, setKeyword, page, setPage,
    items, total, totalPages, isLoading, handleSearch,
  } = useListPage<TicketItem>({
    queryKey: "itsm-tickets",
    endpoint: "/api/v1/itsm/tickets",
    extraParams,
  })

  const { data: priorities = [] } = useQuery({
    queryKey: ["itsm-priorities"],
    queryFn: () => fetchPriorities(),
  })

  const { data: servicesData } = useQuery({
    queryKey: ["itsm-services-list"],
    queryFn: () => fetchServiceDefs({ page: 1, pageSize: 100 }),
  })
  const services = servicesData?.items ?? []

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <h2 className="text-lg font-semibold">{t("itsm:tickets.title")}</h2>
        {canCreate && (
          <Button onClick={() => navigate("/itsm/tickets/create")}>
            <Plus className="mr-1.5 h-4 w-4" />{t("itsm:tickets.create")}
          </Button>
        )}
      </div>

      <DataTableToolbar>
        <DataTableToolbarGroup>
          <form onSubmit={handleSearch} className="flex w-full flex-col gap-2 sm:flex-row sm:items-center sm:flex-wrap">
            <div className="relative w-full sm:max-w-sm">
              <Search className="absolute left-2.5 top-2.5 h-4 w-4 text-muted-foreground" />
              <Input placeholder={t("itsm:tickets.searchPlaceholder")} value={keyword} onChange={(e) => setKeyword(e.target.value)} className="pl-8" />
            </div>
            <Select value={statusFilter || "all"} onValueChange={(v) => { setStatusFilter(v === "all" ? "" : v); setPage(1) }}>
              <SelectTrigger className="w-[140px]"><SelectValue placeholder={t("itsm:tickets.allStatuses")} /></SelectTrigger>
              <SelectContent>
                <SelectItem value="all">{t("itsm:tickets.allStatuses")}</SelectItem>
                {Object.entries(STATUS_MAP).map(([k, v]) => (
                  <SelectItem key={k} value={k}>{t(`itsm:tickets.${v.key}`)}</SelectItem>
                ))}
              </SelectContent>
            </Select>
            <Select value={priorityFilter || "all"} onValueChange={(v) => { setPriorityFilter(v === "all" ? "" : v); setPage(1) }}>
              <SelectTrigger className="w-[140px]"><SelectValue placeholder={t("itsm:tickets.allPriorities")} /></SelectTrigger>
              <SelectContent>
                <SelectItem value="all">{t("itsm:tickets.allPriorities")}</SelectItem>
                {priorities.map((p) => (
                  <SelectItem key={p.id} value={String(p.id)}>
                    <span className="mr-1.5 inline-block h-2.5 w-2.5 rounded-full" style={{ backgroundColor: p.color }} />
                    {p.name}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
            <Select value={serviceFilter || "all"} onValueChange={(v) => { setServiceFilter(v === "all" ? "" : v); setPage(1) }}>
              <SelectTrigger className="w-[160px]"><SelectValue placeholder={t("itsm:tickets.allServices")} /></SelectTrigger>
              <SelectContent>
                <SelectItem value="all">{t("itsm:tickets.allServices")}</SelectItem>
                {services.map((s) => (
                  <SelectItem key={s.id} value={String(s.id)}>{s.name}</SelectItem>
                ))}
              </SelectContent>
            </Select>
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
              <TableHead className="w-[80px]">{t("itsm:tickets.slaStatus")}</TableHead>
              <TableHead className="w-[140px]">{t("itsm:tickets.createdAt")}</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {isLoading ? (
              <DataTableLoadingRow colSpan={8} />
            ) : items.length === 0 ? (
              <DataTableEmptyRow colSpan={8} icon={Ticket} title={t("itsm:tickets.empty")} description={canCreate ? t("itsm:tickets.emptyHint") : undefined} />
            ) : (
              items.map((item) => {
                const statusInfo = STATUS_MAP[item.status] ?? { variant: "secondary" as const, key: "statusPending" }
                return (
                  <TableRow
                    key={item.id}
                    className="cursor-pointer"
                    onClick={() => navigate(`/itsm/tickets/${item.id}`, { state: withActiveMenuPermission(TICKET_MENU_PERMISSION.list) })}
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
                      {item.slaStatus && (
                        <Badge variant={SLA_VARIANT[item.slaStatus] ?? "secondary"}>
                          {t(`itsm:tickets.sla${item.slaStatus.charAt(0).toUpperCase() + item.slaStatus.slice(1)}`)}
                        </Badge>
                      )}
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
