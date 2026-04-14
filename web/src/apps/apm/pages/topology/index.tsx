import { useTranslation } from "react-i18next"
import { useQuery } from "@tanstack/react-query"
import { Network } from "lucide-react"

import { fetchTopology } from "../../api"
import { useTimeRange } from "../../hooks/use-time-range"
import { TimeRangePicker } from "../../components/time-range-picker"
import { ServiceMap } from "../../components/service-map"

function TopologyPage() {
  const { t } = useTranslation("apm")
  const { range, selectPreset, refresh, presets } = useTimeRange("last1h")

  const { data, isLoading } = useQuery({
    queryKey: ["apm-topology", range.start, range.end],
    queryFn: () => fetchTopology(range.start, range.end),
  })

  const hasData = data && data.nodes && data.nodes.length > 0

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-2">
          <Network className="h-5 w-5 text-muted-foreground" />
          <h1 className="text-lg font-semibold">{t("topology.title")}</h1>
        </div>
        <TimeRangePicker value={range.label} presets={presets} onSelect={selectPreset} onRefresh={refresh} />
      </div>

      {isLoading ? (
        <div className="py-20 text-center text-muted-foreground">{t("loading")}</div>
      ) : !hasData ? (
        <div className="py-20 text-center">
          <p className="text-muted-foreground">{t("topology.noData")}</p>
          <p className="mt-1 text-sm text-muted-foreground/60">{t("topology.noDataHint")}</p>
        </div>
      ) : (
        <div className="overflow-auto rounded-lg border bg-card p-4">
          <ServiceMap graph={data} />
        </div>
      )}
    </div>
  )
}

export function Component() {
  return <TopologyPage />
}
