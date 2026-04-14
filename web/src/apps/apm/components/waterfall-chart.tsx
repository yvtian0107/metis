import { useMemo, useState } from "react"
import type { Span } from "../api"
import { SpanDetailSheet } from "./span-detail-sheet"

// Assign distinct colors to services
const SERVICE_COLORS = [
  "bg-blue-500",
  "bg-emerald-500",
  "bg-violet-500",
  "bg-amber-500",
  "bg-cyan-500",
  "bg-pink-500",
  "bg-indigo-500",
  "bg-orange-500",
]

interface SpanNode {
  span: Span
  children: SpanNode[]
  depth: number
}

function buildTree(spans: Span[]): SpanNode[] {
  const map = new Map<string, SpanNode>()
  const roots: SpanNode[] = []

  for (const span of spans) {
    map.set(span.spanId, { span, children: [], depth: 0 })
  }

  for (const node of map.values()) {
    if (node.span.parentSpanId && map.has(node.span.parentSpanId)) {
      const parent = map.get(node.span.parentSpanId)!
      node.depth = parent.depth + 1
      parent.children.push(node)
    } else {
      roots.push(node)
    }
  }

  // Sort children by start time
  function sortChildren(node: SpanNode) {
    node.children.sort((a, b) => new Date(a.span.startTime).getTime() - new Date(b.span.startTime).getTime())
    node.children.forEach(sortChildren)
  }
  roots.forEach(sortChildren)

  return roots
}

function flattenTree(nodes: SpanNode[]): SpanNode[] {
  const result: SpanNode[] = []
  function walk(node: SpanNode) {
    result.push(node)
    node.children.forEach(walk)
  }
  nodes.forEach(walk)
  return result
}

interface WaterfallChartProps {
  spans: Span[]
}

export function WaterfallChart({ spans }: WaterfallChartProps) {
  const [selectedSpan, setSelectedSpan] = useState<Span | null>(null)

  const { flatNodes, traceStart, traceDuration, serviceColorMap } = useMemo(() => {
    const tree = buildTree(spans)
    const flat = flattenTree(tree)

    // Calculate trace time range
    let minTime = Infinity
    let maxTime = -Infinity
    for (const span of spans) {
      const start = new Date(span.startTime).getTime()
      const end = start + span.duration / 1e6 // ns to ms
      if (start < minTime) minTime = start
      if (end > maxTime) maxTime = end
    }

    // Assign colors to services
    const services = [...new Set(spans.map((s) => s.serviceName))]
    const colorMap = new Map<string, string>()
    services.forEach((svc, i) => {
      colorMap.set(svc, SERVICE_COLORS[i % SERVICE_COLORS.length])
    })

    return {
      flatNodes: flat,
      traceStart: minTime,
      traceDuration: maxTime - minTime,
      serviceColorMap: colorMap,
    }
  }, [spans])

  if (spans.length === 0) return null

  return (
    <>
      <div className="space-y-0.5">
        {flatNodes.map((node) => {
          const span = node.span
          const startMs = new Date(span.startTime).getTime() - traceStart
          const durationMs = span.duration / 1e6
          const leftPct = traceDuration > 0 ? (startMs / traceDuration) * 100 : 0
          const widthPct = traceDuration > 0 ? Math.max((durationMs / traceDuration) * 100, 0.5) : 100
          const isError = span.statusCode === "STATUS_CODE_ERROR"
          const colorClass = isError ? "bg-red-500" : (serviceColorMap.get(span.serviceName) ?? "bg-blue-500")

          return (
            <button
              key={span.spanId}
              type="button"
              className="group flex w-full items-center gap-2 rounded-md px-1 py-1 text-left hover:bg-muted/50 transition-colors"
              onClick={() => setSelectedSpan(span)}
            >
              {/* Label */}
              <div
                className="shrink-0 truncate text-xs"
                style={{ width: "240px", paddingLeft: `${node.depth * 16}px` }}
              >
                <span className="font-medium text-foreground">{span.serviceName}</span>
                <span className="ml-1 text-muted-foreground">{span.spanName}</span>
              </div>

              {/* Bar area */}
              <div className="relative h-6 flex-1 rounded bg-muted/30">
                <div
                  className={`absolute top-0.5 bottom-0.5 rounded-sm ${colorClass} opacity-80 group-hover:opacity-100 transition-opacity`}
                  style={{
                    left: `${leftPct}%`,
                    width: `${widthPct}%`,
                    minWidth: "2px",
                  }}
                />
                <span
                  className="absolute top-0.5 text-[10px] font-mono text-muted-foreground leading-5"
                  style={{ left: `${leftPct + widthPct + 0.5}%` }}
                >
                  {durationMs.toFixed(1)}ms
                </span>
              </div>
            </button>
          )
        })}
      </div>

      <SpanDetailSheet
        span={selectedSpan}
        open={selectedSpan !== null}
        onOpenChange={(open) => { if (!open) setSelectedSpan(null) }}
      />
    </>
  )
}
