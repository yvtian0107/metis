import { useState, useMemo } from "react"
import { useParams, useNavigate } from "react-router"
import { useTranslation } from "react-i18next"
import { useQuery } from "@tanstack/react-query"
import { ArrowLeft, ArrowUpDown } from "lucide-react"
import {
  LineChart, Line, AreaChart, Area, BarChart, Bar,
  XAxis, YAxis, Tooltip, ResponsiveContainer, CartesianGrid,
} from "recharts"

import { Button } from "@/components/ui/button"
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table"
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs"
import { Badge } from "@/components/ui/badge"

import { fetchServiceDetail, fetchTimeseries, fetchLatencyDistribution, fetchErrors } from "../../../api"
import type { OperationStats, ErrorGroup } from "../../../api"
import { useTimeRange } from "../../../hooks/use-time-range"
import { TimeRangePicker } from "../../../components/time-range-picker"

type SortField = "requestCount" | "avgDurationMs" | "p95Ms" | "errorRate"
type SortDir = "asc" | "desc"

function SortHeader({ field, sortField, onToggle, children }: { field: SortField; sortField: SortField; onToggle: (f: SortField) => void; children: React.ReactNode }) {
  return (
    <button type="button" className="flex items-center gap-0.5" onClick={() => onToggle(field)}>
      {children}
      <ArrowUpDown className={`h-3 w-3 ${sortField === field ? "text-foreground" : "text-muted-foreground/40"}`} />
    </button>
  )
}

function ServiceDetailPage() {
  const { t } = useTranslation("apm")
  const { name } = useParams<{ name: string }>()
  const navigate = useNavigate()
  const { range, selectPreset, setCustomRange, refresh, presets, refreshInterval, setRefreshInterval } = useTimeRange("last1h")

  const [sortField, setSortField] = useState<SortField>("requestCount")
  const [sortDir, setSortDir] = useState<SortDir>("desc")

  const { data: detail, isLoading } = useQuery({
    queryKey: ["apm-service-detail", name, range.start, range.end],
    queryFn: () => fetchServiceDetail(name!, range.start, range.end),
    enabled: !!name,
  })

  const { data: tsData } = useQuery({
    queryKey: ["apm-timeseries", name, range.start, range.end],
    queryFn: () => fetchTimeseries({ start: range.start, end: range.end, service: name }),
    enabled: !!name,
  })

  const { data: latencyData } = useQuery({
    queryKey: ["apm-latency-dist", name, range.start, range.end],
    queryFn: () => fetchLatencyDistribution({ start: range.start, end: range.end, service: name }),
    enabled: !!name,
  })

  const { data: errorsData } = useQuery({
    queryKey: ["apm-errors", name, range.start, range.end],
    queryFn: () => fetchErrors({ start: range.start, end: range.end, service: name }),
    enabled: !!name,
  })

  const points = tsData?.points ?? []
  const latencyBuckets = latencyData?.buckets ?? []
  const errors = errorsData?.errors ?? []

  const formatDuration = (ms: number) => {
    if (ms < 1) return `${(ms * 1000).toFixed(0)}µs`
    if (ms < 1000) return `${ms.toFixed(1)}ms`
    return `${(ms / 1000).toFixed(2)}s`
  }

  const chartData = points.map((p) => ({
    time: new Date(p.timestamp).toLocaleTimeString([], { hour: "2-digit", minute: "2-digit" }),
    p50: p.p50Ms,
    p95: p.p95Ms,
    p99: p.p99Ms,
    requests: p.requestCount,
    errorRate: p.errorRate,
  }))

  const histogramData = latencyBuckets.map((b) => ({
    range: `${b.rangeStartMs.toFixed(0)}-${b.rangeEndMs.toFixed(0)}`,
    count: b.count,
  }))

  const sortedOps = useMemo(() => {
    const items = detail?.operations ?? []
    const sorted = [...items]
    sorted.sort((a, b) => {
      const av = a[sortField]
      const bv = b[sortField]
      return sortDir === "asc" ? av - bv : bv - av
    })
    return sorted
  }, [detail?.operations, sortField, sortDir])

  const toggleSort = (field: SortField) => {
    if (sortField === field) {
      setSortDir(sortDir === "asc" ? "desc" : "asc")
    } else {
      setSortField(field)
      setSortDir("desc")
    }
  }

  const handleOperationClick = (spanName: string) => {
    const params = new URLSearchParams({
      service: name!,
      operation: spanName,
      start: range.start,
      end: range.end,
    })
    navigate(`/apm/traces?${params}`)
  }

  const tooltipStyle = {
    fontSize: 12,
    borderRadius: 8,
    border: "1px solid hsl(var(--border))",
    background: "hsl(var(--popover))",
    color: "hsl(var(--popover-foreground))",
  }

  return (
    <div className="space-y-4">
      {/* Header */}
      <div className="flex items-center gap-3">
        <Button variant="ghost" size="sm" className="h-8 w-8 p-0" onClick={() => navigate("/apm/services")}>
          <ArrowLeft className="h-4 w-4" />
        </Button>
        <div className="flex-1">
          <h1 className="text-lg font-semibold">{name}</h1>
        </div>
        <TimeRangePicker
          value={range.label}
          presets={presets}
          onSelect={selectPreset}
          onRefresh={refresh}
          onCustomRange={setCustomRange}
          refreshInterval={refreshInterval}
          onRefreshIntervalChange={setRefreshInterval}
        />
      </div>

      {isLoading ? (
        <div className="py-12 text-center text-muted-foreground">{t("loading")}</div>
      ) : !detail ? (
        <div className="py-12 text-center text-muted-foreground">{t("notConfigured")}</div>
      ) : (
        <Tabs defaultValue="overview">
          <TabsList>
            <TabsTrigger value="overview">{t("services.overview", "Overview")}</TabsTrigger>
            <TabsTrigger value="errors">
              {t("services.errors", "Errors")}
              {errors.length > 0 && (
                <Badge variant="destructive" className="ml-1.5 text-[10px] px-1.5 py-0">
                  {errors.length}
                </Badge>
              )}
            </TabsTrigger>
          </TabsList>

          <TabsContent value="overview" className="mt-4 space-y-4">
            {/* Metric cards */}
            <div className="grid grid-cols-4 gap-3">
              <MetricCard label={t("services.throughput")} value={String(detail.requestCount)} />
              <MetricCard label={t("services.avgDuration")} value={formatDuration(detail.avgDurationMs)} />
              <MetricCard label={t("services.p95")} value={formatDuration(detail.p95Ms)} />
              <MetricCard
                label={t("services.errorRate")}
                value={`${detail.errorRate.toFixed(2)}%`}
                color={detail.errorRate > 5 ? "text-red-600" : detail.errorRate > 1 ? "text-amber-600" : "text-emerald-600"}
              />
            </div>

            {/* Charts row */}
            {chartData.length > 0 && (
              <div className="grid grid-cols-3 gap-3">
                {/* Request Rate */}
                <div className="rounded-lg border p-3">
                  <h3 className="mb-2 text-xs font-medium text-muted-foreground">{t("services.requestRate", "Request Rate")}</h3>
                  <ResponsiveContainer width="100%" height={140}>
                    <AreaChart data={chartData}>
                      <CartesianGrid strokeDasharray="3 3" className="stroke-muted" />
                      <XAxis dataKey="time" tick={{ fontSize: 9 }} className="text-muted-foreground" />
                      <YAxis tick={{ fontSize: 9 }} className="text-muted-foreground" />
                      <Tooltip contentStyle={tooltipStyle} />
                      <Area type="monotone" dataKey="requests" stroke="hsl(var(--primary))" fill="hsl(var(--primary) / 0.15)" strokeWidth={1.5} name="Requests" />
                    </AreaChart>
                  </ResponsiveContainer>
                </div>

                {/* Error Rate */}
                <div className="rounded-lg border p-3">
                  <h3 className="mb-2 text-xs font-medium text-muted-foreground">{t("services.errorRate")}</h3>
                  <ResponsiveContainer width="100%" height={140}>
                    <AreaChart data={chartData}>
                      <CartesianGrid strokeDasharray="3 3" className="stroke-muted" />
                      <XAxis dataKey="time" tick={{ fontSize: 9 }} className="text-muted-foreground" />
                      <YAxis tick={{ fontSize: 9 }} className="text-muted-foreground" unit="%" />
                      <Tooltip contentStyle={tooltipStyle} formatter={(v) => [`${Number(v).toFixed(2)}%`]} />
                      <Area type="monotone" dataKey="errorRate" stroke="#ef4444" fill="#ef444420" strokeWidth={1.5} name="Error Rate" />
                    </AreaChart>
                  </ResponsiveContainer>
                </div>

                {/* Latency */}
                <div className="rounded-lg border p-3">
                  <h3 className="mb-2 text-xs font-medium text-muted-foreground">{t("services.latencyTrend")}</h3>
                  <ResponsiveContainer width="100%" height={140}>
                    <LineChart data={chartData}>
                      <CartesianGrid strokeDasharray="3 3" className="stroke-muted" />
                      <XAxis dataKey="time" tick={{ fontSize: 9 }} className="text-muted-foreground" />
                      <YAxis tick={{ fontSize: 9 }} className="text-muted-foreground" />
                      <Tooltip contentStyle={tooltipStyle} formatter={(v) => [`${Number(v).toFixed(1)}ms`]} />
                      <Line type="monotone" dataKey="p50" stroke="hsl(var(--primary))" strokeWidth={1.5} dot={false} name="P50" />
                      <Line type="monotone" dataKey="p95" stroke="#f59e0b" strokeWidth={1.5} dot={false} name="P95" />
                      <Line type="monotone" dataKey="p99" stroke="#ef4444" strokeWidth={1.5} dot={false} name="P99" />
                    </LineChart>
                  </ResponsiveContainer>
                </div>
              </div>
            )}

            {/* Latency Distribution */}
            {histogramData.length > 0 && (
              <div className="rounded-lg border p-3">
                <h3 className="mb-2 text-xs font-medium text-muted-foreground">{t("services.latencyDist", "Latency Distribution")}</h3>
                <ResponsiveContainer width="100%" height={160}>
                  <BarChart data={histogramData}>
                    <CartesianGrid strokeDasharray="3 3" className="stroke-muted" />
                    <XAxis dataKey="range" tick={{ fontSize: 8 }} className="text-muted-foreground" angle={-30} textAnchor="end" height={40} />
                    <YAxis tick={{ fontSize: 9 }} className="text-muted-foreground" />
                    <Tooltip contentStyle={tooltipStyle} />
                    <Bar dataKey="count" fill="hsl(var(--primary) / 0.7)" radius={[2, 2, 0, 0]} name="Count" />
                  </BarChart>
                </ResponsiveContainer>
              </div>
            )}

            {/* Operations table */}
            <div className="rounded-lg border">
              <h2 className="px-4 pt-3 text-sm font-medium">{t("services.operations")}</h2>
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>{t("services.operationName")}</TableHead>
                    <TableHead className="w-[80px] text-right">
                      <SortHeader field="requestCount" sortField={sortField} onToggle={toggleSort}>{t("services.reqRate")}</SortHeader>
                    </TableHead>
                    <TableHead className="w-[90px] text-right">
                      <SortHeader field="avgDurationMs" sortField={sortField} onToggle={toggleSort}>{t("services.avgDuration")}</SortHeader>
                    </TableHead>
                    <TableHead className="w-[80px] text-right">
                      <SortHeader field="p95Ms" sortField={sortField} onToggle={toggleSort}>{t("services.p95")}</SortHeader>
                    </TableHead>
                    <TableHead className="w-[80px] text-right">
                      <SortHeader field="errorRate" sortField={sortField} onToggle={toggleSort}>{t("services.errorRate")}</SortHeader>
                    </TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {sortedOps.length === 0 ? (
                    <TableRow>
                      <TableCell colSpan={5} className="py-6 text-center text-muted-foreground">
                        {t("services.noData")}
                      </TableCell>
                    </TableRow>
                  ) : (
                    sortedOps.map((op: OperationStats) => (
                      <TableRow
                        key={op.spanName}
                        className="cursor-pointer hover:bg-muted/50"
                        onClick={() => handleOperationClick(op.spanName)}
                      >
                        <TableCell className="font-mono text-xs">{op.spanName}</TableCell>
                        <TableCell className="text-right font-mono text-xs">{op.requestCount}</TableCell>
                        <TableCell className="text-right font-mono text-xs">{formatDuration(op.avgDurationMs)}</TableCell>
                        <TableCell className="text-right font-mono text-xs">{formatDuration(op.p95Ms)}</TableCell>
                        <TableCell className={`text-right font-mono text-xs ${op.errorRate > 5 ? "text-red-600" : op.errorRate > 1 ? "text-amber-600" : "text-emerald-600"}`}>
                          {op.errorRate.toFixed(2)}%
                        </TableCell>
                      </TableRow>
                    ))
                  )}
                </TableBody>
              </Table>
            </div>
          </TabsContent>

          <TabsContent value="errors" className="mt-4">
            <div className="rounded-lg border">
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>{t("services.errorType", "Error Type")}</TableHead>
                    <TableHead>{t("services.errorMessage", "Message")}</TableHead>
                    <TableHead className="w-[70px] text-right">{t("services.errorCount", "Count")}</TableHead>
                    <TableHead className="w-[120px] text-right">{t("services.errorLastSeen", "Last Seen")}</TableHead>
                    <TableHead className="w-[150px]">{t("services.errorServices", "Services")}</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {errors.length === 0 ? (
                    <TableRow>
                      <TableCell colSpan={5} className="py-8 text-center text-muted-foreground">
                        {t("services.noErrors", "No errors found")}
                      </TableCell>
                    </TableRow>
                  ) : (
                    errors.map((err: ErrorGroup, i: number) => (
                      <TableRow key={`${err.errorType}-${i}`}>
                        <TableCell className="font-mono text-xs text-red-500">{err.errorType || "Unknown"}</TableCell>
                        <TableCell className="text-xs max-w-[300px] truncate" title={err.message}>{err.message}</TableCell>
                        <TableCell className="text-right font-mono text-xs font-medium">{err.count}</TableCell>
                        <TableCell className="text-right text-xs text-muted-foreground">
                          {new Date(err.lastSeen).toLocaleString(undefined, {
                            month: "short",
                            day: "numeric",
                            hour: "2-digit",
                            minute: "2-digit",
                          })}
                        </TableCell>
                        <TableCell>
                          <div className="flex flex-wrap gap-1">
                            {err.services.map((svc) => (
                              <Badge key={svc} variant="outline" className="text-[10px]">{svc}</Badge>
                            ))}
                          </div>
                        </TableCell>
                      </TableRow>
                    ))
                  )}
                </TableBody>
              </Table>
            </div>
          </TabsContent>
        </Tabs>
      )}
    </div>
  )
}

function MetricCard({ label, value, color }: { label: string; value: string; color?: string }) {
  return (
    <div className="rounded-lg border p-3">
      <p className="text-xs text-muted-foreground">{label}</p>
      <p className={`mt-1 text-xl font-semibold font-mono ${color ?? ""}`}>{value}</p>
    </div>
  )
}

export function Component() {
  return <ServiceDetailPage />
}
