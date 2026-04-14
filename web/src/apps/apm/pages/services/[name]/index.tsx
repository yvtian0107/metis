import { useParams, useNavigate } from "react-router"
import { useTranslation } from "react-i18next"
import { useQuery } from "@tanstack/react-query"
import { ArrowLeft } from "lucide-react"
import { LineChart, Line, XAxis, YAxis, Tooltip, ResponsiveContainer, CartesianGrid } from "recharts"

import { Button } from "@/components/ui/button"
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table"

import { fetchServiceDetail, fetchTimeseries } from "../../../api"
import { useTimeRange } from "../../../hooks/use-time-range"
import { TimeRangePicker } from "../../../components/time-range-picker"

function ServiceDetailPage() {
  const { t } = useTranslation("apm")
  const { name } = useParams<{ name: string }>()
  const navigate = useNavigate()
  const { range, selectPreset, refresh, presets } = useTimeRange("last1h")

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

  const points = tsData?.points ?? []
  const ops = detail?.operations ?? []

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

  const handleOperationClick = (spanName: string) => {
    const params = new URLSearchParams({
      service: name!,
      operation: spanName,
      start: range.start,
      end: range.end,
    })
    navigate(`/apm/traces?${params}`)
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
        />
      </div>

      {isLoading ? (
        <div className="py-12 text-center text-muted-foreground">{t("loading")}</div>
      ) : !detail ? (
        <div className="py-12 text-center text-muted-foreground">{t("notConfigured")}</div>
      ) : (
        <>
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

          {/* Latency chart */}
          {chartData.length > 0 && (
            <div className="rounded-lg border p-4">
              <h2 className="mb-3 text-sm font-medium">{t("services.latencyTrend")}</h2>
              <ResponsiveContainer width="100%" height={220}>
                <LineChart data={chartData}>
                  <CartesianGrid strokeDasharray="3 3" className="stroke-muted" />
                  <XAxis dataKey="time" tick={{ fontSize: 11 }} className="text-muted-foreground" />
                  <YAxis tick={{ fontSize: 11 }} className="text-muted-foreground" />
                  <Tooltip
                    contentStyle={{ fontSize: 12, borderRadius: 8, border: "1px solid hsl(var(--border))", background: "hsl(var(--popover))", color: "hsl(var(--popover-foreground))" }}
                    formatter={(value: number) => [`${value.toFixed(1)}ms`]}
                  />
                  <Line type="monotone" dataKey="p50" stroke="hsl(var(--primary))" strokeWidth={1.5} dot={false} name="P50" />
                  <Line type="monotone" dataKey="p95" stroke="#f59e0b" strokeWidth={1.5} dot={false} name="P95" />
                  <Line type="monotone" dataKey="p99" stroke="#ef4444" strokeWidth={1.5} dot={false} name="P99" />
                </LineChart>
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
                  <TableHead className="w-[80px] text-right">{t("services.reqRate")}</TableHead>
                  <TableHead className="w-[90px] text-right">{t("services.avgDuration")}</TableHead>
                  <TableHead className="w-[80px] text-right">{t("services.p95")}</TableHead>
                  <TableHead className="w-[80px] text-right">{t("services.errorRate")}</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {ops.length === 0 ? (
                  <TableRow>
                    <TableCell colSpan={5} className="py-6 text-center text-muted-foreground">
                      {t("services.noData")}
                    </TableCell>
                  </TableRow>
                ) : (
                  ops.map((op) => (
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
        </>
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
