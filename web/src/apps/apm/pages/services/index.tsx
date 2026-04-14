import { useState } from "react"
import { useNavigate } from "react-router"
import { useTranslation } from "react-i18next"
import { useQuery } from "@tanstack/react-query"
import { RefreshCw } from "lucide-react"

import { Button } from "@/components/ui/button"
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table"

import { fetchServices, type ServiceOverview, type SparklinePoint } from "../../api"
import { useTimeRange } from "../../hooks/use-time-range"
import { TimeRangePicker } from "../../components/time-range-picker"
import { Sparkline } from "../../components/sparkline"

function ServiceCatalogPage() {
  const { t } = useTranslation("apm")
  const navigate = useNavigate()
  const { range, selectPreset, refresh, presets } = useTimeRange("last1h")

  const [, setTick] = useState(0)

  const { data, isLoading, isError, refetch } = useQuery({
    queryKey: ["apm-services", range.start, range.end],
    queryFn: () => fetchServices(range.start, range.end),
  })

  const services = data?.services ?? []
  const sparklines = data?.sparklines ?? {}

  const formatDuration = (ms: number) => {
    if (ms < 1) return `${(ms * 1000).toFixed(0)}µs`
    if (ms < 1000) return `${ms.toFixed(1)}ms`
    return `${(ms / 1000).toFixed(2)}s`
  }

  const toSparklineData = (points: SparklinePoint[] | undefined) =>
    (points ?? []).map((p) => ({ value: p.requestCount }))

  return (
    <div className="space-y-4">
      {/* Header */}
      <div className="flex items-center justify-between">
        <h1 className="text-lg font-semibold">{t("services.title")}</h1>
        <div className="flex items-center gap-2">
          <TimeRangePicker
            value={range.label}
            presets={presets}
            onSelect={(label) => { selectPreset(label); setTick((t) => t + 1) }}
            onRefresh={() => { refresh(); setTick((t) => t + 1) }}
          />
          <Button variant="outline" size="sm" className="h-7" onClick={() => { refresh(); refetch() }}>
            <RefreshCw className="h-3.5 w-3.5" />
          </Button>
        </div>
      </div>

      {/* Table */}
      <div className="rounded-lg border">
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>{t("services.name")}</TableHead>
              <TableHead className="w-[80px] text-right">{t("services.reqRate")}</TableHead>
              <TableHead className="w-[90px] text-right">{t("services.avgDuration")}</TableHead>
              <TableHead className="w-[80px] text-right">{t("services.p95")}</TableHead>
              <TableHead className="w-[80px] text-right">{t("services.errorRate")}</TableHead>
              <TableHead className="w-[120px]">{t("services.trend")}</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {isLoading ? (
              <TableRow>
                <TableCell colSpan={6} className="py-8 text-center text-muted-foreground">
                  {t("loading")}
                </TableCell>
              </TableRow>
            ) : isError ? (
              <TableRow>
                <TableCell colSpan={6} className="py-8 text-center text-muted-foreground">
                  {t("notConfigured")}
                </TableCell>
              </TableRow>
            ) : services.length === 0 ? (
              <TableRow>
                <TableCell colSpan={6} className="py-8 text-center">
                  <p className="text-muted-foreground">{t("services.noData")}</p>
                </TableCell>
              </TableRow>
            ) : (
              services.map((svc: ServiceOverview) => (
                <TableRow
                  key={svc.serviceName}
                  className="cursor-pointer hover:bg-muted/50"
                  onClick={() => navigate(`/apm/services/${encodeURIComponent(svc.serviceName)}`)}
                >
                  <TableCell className="font-mono text-sm font-medium">{svc.serviceName}</TableCell>
                  <TableCell className="text-right font-mono text-xs">
                    {svc.requestCount}
                  </TableCell>
                  <TableCell className="text-right font-mono text-xs">
                    {formatDuration(svc.avgDurationMs)}
                  </TableCell>
                  <TableCell className="text-right font-mono text-xs">
                    {formatDuration(svc.p95Ms)}
                  </TableCell>
                  <TableCell className={`text-right font-mono text-xs ${svc.errorRate > 5 ? "text-red-600" : svc.errorRate > 1 ? "text-amber-600" : "text-emerald-600"}`}>
                    {svc.errorRate.toFixed(2)}%
                  </TableCell>
                  <TableCell>
                    <Sparkline
                      data={toSparklineData(sparklines[svc.serviceName])}
                      height={20}
                    />
                  </TableCell>
                </TableRow>
              ))
            )}
          </TableBody>
        </Table>
      </div>
    </div>
  )
}

export function Component() {
  return <ServiceCatalogPage />
}
