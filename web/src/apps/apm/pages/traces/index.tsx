import { useState, useCallback } from "react"
import { useNavigate, useSearchParams } from "react-router"
import { useTranslation } from "react-i18next"
import { useQuery } from "@tanstack/react-query"
import { Search, AlertCircle, CheckCircle2, ChevronLeft, ChevronRight, RefreshCw } from "lucide-react"

import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select"
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table"
import { Badge } from "@/components/ui/badge"

import { fetchTraces, type TraceSummary } from "../../api"
import { useTimeRange } from "../../hooks/use-time-range"
import { TimeRangePicker } from "../../components/time-range-picker"

function TraceExplorerPage() {
  const { t } = useTranslation("apm")
  const navigate = useNavigate()
  const [searchParams] = useSearchParams()
  const { range, selectPreset, refresh, presets } = useTimeRange("last1h")

  // Initialize filters from URL searchParams (for Service Detail → Trace Explorer navigation)
  const [service, setService] = useState(searchParams.get("service") ?? "")
  const [operation, setOperation] = useState(searchParams.get("operation") ?? "")
  const [status, setStatus] = useState(searchParams.get("status") ?? "")
  const [durationMin, setDurationMin] = useState(searchParams.get("duration_min") ?? "")
  const [durationMax, setDurationMax] = useState(searchParams.get("duration_max") ?? "")

  // Use URL start/end if provided (from Service Detail jump), otherwise use time range picker
  const urlStart = searchParams.get("start")
  const urlEnd = searchParams.get("end")
  const effectiveStart = urlStart || range.start
  const effectiveEnd = urlEnd || range.end
  const [page, setPage] = useState(1)
  const pageSize = 20

  const { data, isLoading, isError, refetch } = useQuery({
    queryKey: ["apm-traces", effectiveStart, effectiveEnd, service, operation, status, durationMin, durationMax, page],
    queryFn: () =>
      fetchTraces({
        start: effectiveStart,
        end: effectiveEnd,
        service: service || undefined,
        operation: operation || undefined,
        status: status || undefined,
        duration_min: durationMin ? parseFloat(durationMin) : undefined,
        duration_max: durationMax ? parseFloat(durationMax) : undefined,
        page,
        page_size: pageSize,
      }),
  })

  const traces = data?.items ?? []
  const total = data?.total ?? 0
  const totalPages = Math.ceil(total / pageSize)

  const handleSearch = useCallback(() => {
    setPage(1)
    refetch()
  }, [refetch])

  const formatDuration = (ms: number) => {
    if (ms < 1) return `${(ms * 1000).toFixed(0)}µs`
    if (ms < 1000) return `${ms.toFixed(1)}ms`
    return `${(ms / 1000).toFixed(2)}s`
  }

  const getDurationColor = (ms: number) => {
    if (ms < 100) return "text-emerald-600"
    if (ms < 500) return "text-amber-600"
    return "text-red-600"
  }

  return (
    <div className="space-y-4">
      {/* Header */}
      <div className="flex items-center justify-between">
        <h1 className="text-lg font-semibold">{t("traceExplorer")}</h1>
        <div className="flex items-center gap-2">
          <TimeRangePicker
            value={range.label}
            presets={presets}
            onSelect={(label) => { selectPreset(label); setPage(1) }}
            onRefresh={() => { refresh(); setPage(1) }}
          />
          <Button variant="outline" size="sm" className="h-7" onClick={() => { refresh(); refetch() }}>
            <RefreshCw className="h-3.5 w-3.5" />
          </Button>
        </div>
      </div>

      {/* Filters */}
      <div className="flex flex-wrap items-end gap-3">
        <div className="w-40">
          <Input
            placeholder={t("filters.servicePlaceholder")}
            value={service}
            onChange={(e) => setService(e.target.value)}
            className="h-8 text-sm"
          />
        </div>
        <div className="w-40">
          <Input
            placeholder={t("filters.operationPlaceholder")}
            value={operation}
            onChange={(e) => setOperation(e.target.value)}
            className="h-8 text-sm"
          />
        </div>
        <div className="w-28">
          <Select value={status} onValueChange={setStatus}>
            <SelectTrigger className="h-8 text-sm">
              <SelectValue placeholder={t("filters.status")} />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="all">{t("filters.statusAll")}</SelectItem>
              <SelectItem value="ok">{t("filters.statusOk")}</SelectItem>
              <SelectItem value="error">{t("filters.statusError")}</SelectItem>
            </SelectContent>
          </Select>
        </div>
        <div className="w-24">
          <Input
            type="number"
            placeholder={t("filters.durationMin")}
            value={durationMin}
            onChange={(e) => setDurationMin(e.target.value)}
            className="h-8 text-sm"
          />
        </div>
        <div className="w-24">
          <Input
            type="number"
            placeholder={t("filters.durationMax")}
            value={durationMax}
            onChange={(e) => setDurationMax(e.target.value)}
            className="h-8 text-sm"
          />
        </div>
        <Button size="sm" className="h-8" onClick={handleSearch}>
          <Search className="mr-1 h-3.5 w-3.5" />
          {t("filters.search")}
        </Button>
      </div>

      {/* Table */}
      <div className="rounded-lg border">
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead className="w-[180px]">{t("table.traceId")}</TableHead>
              <TableHead>{t("table.rootOperation")}</TableHead>
              <TableHead>{t("table.service")}</TableHead>
              <TableHead className="w-[100px]">{t("table.duration")}</TableHead>
              <TableHead className="w-[70px]">{t("table.spans")}</TableHead>
              <TableHead className="w-[70px]">{t("table.status")}</TableHead>
              <TableHead className="w-[160px]">{t("table.time")}</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {isLoading ? (
              <TableRow>
                <TableCell colSpan={7} className="text-center py-8 text-muted-foreground">
                  {t("loading")}
                </TableCell>
              </TableRow>
            ) : isError ? (
              <TableRow>
                <TableCell colSpan={7} className="text-center py-8 text-muted-foreground">
                  {t("notConfigured")}
                </TableCell>
              </TableRow>
            ) : traces.length === 0 ? (
              <TableRow>
                <TableCell colSpan={7} className="text-center py-8">
                  <p className="text-muted-foreground">{t("table.noData")}</p>
                  <p className="text-xs text-muted-foreground mt-1">{t("table.noDataHint")}</p>
                </TableCell>
              </TableRow>
            ) : (
              traces.map((trace: TraceSummary) => (
                <TableRow
                  key={trace.traceId}
                  className="cursor-pointer hover:bg-muted/50"
                  onClick={() => navigate(`/apm/traces/${trace.traceId}`)}
                >
                  <TableCell className="font-mono text-xs">
                    {trace.traceId.slice(0, 16)}...
                  </TableCell>
                  <TableCell className="font-mono text-xs">{trace.rootOperation}</TableCell>
                  <TableCell>
                    <Badge variant="outline" className="text-xs font-normal">
                      {trace.serviceName}
                    </Badge>
                  </TableCell>
                  <TableCell className={`font-mono text-xs ${getDurationColor(trace.durationMs)}`}>
                    {formatDuration(trace.durationMs)}
                  </TableCell>
                  <TableCell className="text-xs text-muted-foreground">{trace.spanCount}</TableCell>
                  <TableCell>
                    {trace.hasError ? (
                      <AlertCircle className="h-4 w-4 text-red-500" />
                    ) : (
                      <CheckCircle2 className="h-4 w-4 text-emerald-500" />
                    )}
                  </TableCell>
                  <TableCell className="text-xs text-muted-foreground">
                    {new Date(trace.timestamp).toLocaleString()}
                  </TableCell>
                </TableRow>
              ))
            )}
          </TableBody>
        </Table>
      </div>

      {/* Pagination */}
      {totalPages > 1 && (
        <div className="flex items-center justify-between">
          <p className="text-sm text-muted-foreground">
            {total} traces
          </p>
          <div className="flex items-center gap-1">
            <Button
              variant="outline"
              size="sm"
              className="h-7 w-7 p-0"
              disabled={page <= 1}
              onClick={() => setPage((p) => p - 1)}
            >
              <ChevronLeft className="h-4 w-4" />
            </Button>
            <span className="px-2 text-sm text-muted-foreground">
              {page} / {totalPages}
            </span>
            <Button
              variant="outline"
              size="sm"
              className="h-7 w-7 p-0"
              disabled={page >= totalPages}
              onClick={() => setPage((p) => p + 1)}
            >
              <ChevronRight className="h-4 w-4" />
            </Button>
          </div>
        </div>
      )}
    </div>
  )
}

export function Component() {
  return <TraceExplorerPage />
}
