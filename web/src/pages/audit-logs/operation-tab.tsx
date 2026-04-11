import { useState } from "react"
import { useTranslation } from "react-i18next"
import { useQuery } from "@tanstack/react-query"
import { Search, ClipboardList } from "lucide-react"
import { api, type PaginatedResponse } from "@/lib/api"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Badge } from "@/components/ui/badge"
import {
  DataTableCard,
  DataTableEmptyRow,
  DataTableLoadingRow,
  DataTablePagination,
  DataTableToolbar,
} from "@/components/ui/data-table"
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select"
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table"
import { formatDateTime } from "@/lib/utils"

interface AuditLog {
  id: number
  createdAt: string
  category: string
  userId: number | null
  username: string
  action: string
  resource: string
  resourceId: string
  summary: string
  level: string
  ipAddress: string
}

const resourceTypeKeys = [
  "user",
  "role",
  "menu",
  "settings",
  "announcement",
  "channel",
  "auth_provider",
  "session",
] as const

export function OperationTab() {
  const { t } = useTranslation(["audit", "common"])
  const [keyword, setKeyword] = useState("")
  const [searchKeyword, setSearchKeyword] = useState("")
  const [resource, setResource] = useState("")
  const [dateFrom, setDateFrom] = useState("")
  const [dateTo, setDateTo] = useState("")
  const [page, setPage] = useState(1)
  const pageSize = 20

  const { data, isLoading } = useQuery({
    queryKey: ["audit-logs", "operation", searchKeyword, resource, dateFrom, dateTo, page],
    queryFn: () => {
      const params = new URLSearchParams({
        category: "operation",
        page: String(page),
        pageSize: String(pageSize),
      })
      if (searchKeyword) params.set("keyword", searchKeyword)
      if (resource) params.set("resource", resource)
      if (dateFrom) params.set("dateFrom", dateFrom)
      if (dateTo) params.set("dateTo", dateTo)
      return api.get<PaginatedResponse<AuditLog>>(`/api/v1/audit-logs?${params}`)
    },
  })

  const items = data?.items ?? []
  const total = data?.total ?? 0
  const totalPages = Math.ceil(total / pageSize)

  function handleSearch(e: React.FormEvent) {
    e.preventDefault()
    setSearchKeyword(keyword)
    setPage(1)
  }

  return (
    <div className="space-y-4 pt-4">
      <DataTableToolbar className="flex-wrap items-center gap-2">
        <form onSubmit={handleSearch} className="flex items-center gap-2">
          <Input
            placeholder={t("audit:operation.searchPlaceholder")}
            value={keyword}
            onChange={(e) => setKeyword(e.target.value)}
            className="h-8 w-48"
          />
          <Button type="submit" variant="outline">
            <Search className="mr-1 h-3.5 w-3.5" />
            {t("common:search")}
          </Button>
        </form>
        <Select value={resource} onValueChange={(v) => { setResource(v === "all" ? "" : v); setPage(1) }}>
          <SelectTrigger size="sm" className="w-32">
            <SelectValue placeholder={t("audit:operation.resourceType")} />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="all">{t("audit:all")}</SelectItem>
            {resourceTypeKeys.map((key) => (
              <SelectItem key={key} value={key}>{t(`audit:operation.resources.${key}`)}</SelectItem>
            ))}
          </SelectContent>
        </Select>
        <Input
          type="date"
          value={dateFrom}
          onChange={(e) => { setDateFrom(e.target.value); setPage(1) }}
          className="h-8 w-36"
        />
        <span className="text-muted-foreground text-sm">{t("audit:dateTo")}</span>
        <Input
          type="date"
          value={dateTo}
          onChange={(e) => { setDateTo(e.target.value); setPage(1) }}
          className="h-8 w-36"
        />
      </DataTableToolbar>

      <DataTableCard>
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead className="w-[150px]">{t("audit:operation.columns.time")}</TableHead>
              <TableHead className="w-[140px]">{t("audit:operation.columns.operator")}</TableHead>
              <TableHead className="w-[120px]">{t("audit:operation.columns.action")}</TableHead>
              <TableHead className="w-[120px]">{t("audit:operation.columns.resourceType")}</TableHead>
              <TableHead className="min-w-[260px]">{t("audit:operation.columns.summary")}</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {isLoading ? (
              <DataTableLoadingRow colSpan={5} />
            ) : items.length === 0 ? (
              <DataTableEmptyRow colSpan={5} icon={ClipboardList} title={t("audit:operation.empty")} />
            ) : (
              items.map((log) => (
                <TableRow key={log.id}>
                  <TableCell className="text-sm text-muted-foreground whitespace-nowrap">
                    {formatDateTime(log.createdAt)}
                  </TableCell>
                  <TableCell className="font-medium">{log.username || "-"}</TableCell>
                  <TableCell>
                    <Badge variant="outline">
                      {t(`audit:operation.actions.${log.action}`, { defaultValue: log.action })}
                    </Badge>
                  </TableCell>
                  <TableCell className="text-sm">
                    {t(`audit:operation.resources.${log.resource}`, { defaultValue: log.resource ?? "-" })}
                  </TableCell>
                  <TableCell className="text-sm text-muted-foreground max-w-[300px] truncate">
                    {log.summary}
                  </TableCell>
                </TableRow>
              ))
            )}
          </TableBody>
        </Table>
      </DataTableCard>

      <DataTablePagination
        total={total}
        page={page}
        totalPages={totalPages}
        onPageChange={setPage}
      />
    </div>
  )
}
