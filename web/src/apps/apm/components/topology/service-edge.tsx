import { memo, useState } from "react"
import { BaseEdge, getBezierPath, type EdgeProps } from "@xyflow/react"
import type { TopologyEdge } from "../../api"

export type ServiceEdgeData = TopologyEdge & {
  colorMode?: "errorRate" | "latency" | "throughput"
  filtered?: boolean
}

// --- Color mapping by mode ---

function getEdgeHealth(edge: TopologyEdge, mode: string): "critical" | "warning" | "healthy" {
  if (mode === "latency") {
    if (edge.p95DurationMs > 500) return "critical"
    if (edge.p95DurationMs > 100) return "warning"
    return "healthy"
  }
  // errorRate (default) + throughput fallback
  if (edge.errorRate > 5) return "critical"
  if (edge.errorRate > 1) return "warning"
  return "healthy"
}

const STROKE_COLORS = {
  healthy: { normal: "hsl(var(--muted-foreground) / 0.2)", hover: "hsl(var(--muted-foreground) / 0.45)" },
  warning: { normal: "hsl(38 92% 50% / 0.4)", hover: "hsl(38 92% 50% / 0.7)" },
  critical: { normal: "hsl(0 72% 51% / 0.45)", hover: "hsl(0 72% 51% / 0.75)" },
}

const THROUGHPUT_STROKE = {
  high: { normal: "hsl(263 70% 50% / 0.4)", hover: "hsl(263 70% 50% / 0.7)" },
  medium: { normal: "hsl(217 91% 60% / 0.35)", hover: "hsl(217 91% 60% / 0.65)" },
  low: { normal: "hsl(199 89% 48% / 0.25)", hover: "hsl(199 89% 48% / 0.5)" },
}

const DOT_COLORS = {
  healthy: "hsl(var(--primary) / 0.5)",
  warning: "hsl(38 92% 50% / 0.6)",
  critical: "hsl(0 72% 51% / 0.65)",
}

const MARKER_MAP = {
  healthy: "url(#arrow)",
  warning: "url(#arrow-warn)",
  critical: "url(#arrow-error)",
}

function getThroughputLevel(callCount: number) {
  if (callCount > 500) return "high"
  if (callCount > 100) return "medium"
  return "low"
}

export const ServiceEdge = memo(function ServiceEdge(props: EdgeProps) {
  const { sourceX, sourceY, targetX, targetY, sourcePosition, targetPosition, data } = props
  const edge = data as unknown as ServiceEdgeData
  const colorMode = edge.colorMode ?? "errorRate"
  const [hovered, setHovered] = useState(false)

  const [edgePath, labelX, labelY] = getBezierPath({
    sourceX, sourceY, targetX, targetY, sourcePosition, targetPosition,
  })

  const strokeWidth = Math.min(1.5 + Math.log2(Math.max(edge.callCount, 1)) * 0.5, 4.5)
  const health = getEdgeHealth(edge, colorMode)

  let strokeNormal: string
  let strokeHover: string
  let dotColor: string

  if (colorMode === "throughput") {
    const level = getThroughputLevel(edge.callCount)
    strokeNormal = THROUGHPUT_STROKE[level].normal
    strokeHover = THROUGHPUT_STROKE[level].hover
    dotColor = strokeHover
  } else {
    strokeNormal = STROKE_COLORS[health].normal
    strokeHover = STROKE_COLORS[health].hover
    dotColor = DOT_COLORS[health]
  }

  const speed = Math.max(1.5, 5 - Math.log2(Math.max(edge.callCount, 1)) * 0.4)

  return (
    <g className={`transition-opacity duration-200 ${edge.filtered ? "opacity-[0.15]" : "opacity-100"}`}>
      {/* Glow / halo */}
      <path
        d={edgePath}
        fill="none"
        stroke={health === "critical" ? "hsl(0 72% 51% / 0.06)" : "hsl(var(--primary) / 0.03)"}
        strokeWidth={strokeWidth + 6}
        strokeLinecap="round"
      />

      {/* Main line */}
      <BaseEdge
        path={edgePath}
        style={{
          stroke: hovered ? strokeHover : strokeNormal,
          strokeWidth: hovered ? strokeWidth + 0.5 : strokeWidth,
          transition: "stroke 0.15s, stroke-width 0.15s",
        }}
        markerEnd={MARKER_MAP[health]}
      />

      {/* Animated traffic dots */}
      <circle r={2} fill={dotColor}>
        <animateMotion dur={`${speed}s`} repeatCount="indefinite" path={edgePath} />
      </circle>
      {edge.callCount > 100 && (
        <circle r={1.5} fill={dotColor} opacity={0.6}>
          <animateMotion dur={`${speed * 1.3}s`} repeatCount="indefinite" path={edgePath} begin={`${speed * 0.4}s`} />
        </circle>
      )}

      {/* Hover target (invisible wide path) */}
      <path
        d={edgePath}
        fill="none"
        stroke="transparent"
        strokeWidth={28}
        className="cursor-pointer"
        onMouseEnter={() => setHovered(true)}
        onMouseLeave={() => setHovered(false)}
      />

      {/* Hover tooltip */}
      {hovered && (
        <foreignObject
          x={labelX - 120}
          y={labelY - 60}
          width={240}
          height={140}
          className="pointer-events-none overflow-visible"
          style={{ zIndex: 1000 }}
        >
          <div className="rounded-xl border bg-popover/95 backdrop-blur-md p-3 shadow-xl text-popover-foreground">
            <div className="flex items-center gap-1.5 text-xs font-semibold mb-2">
              <span className="truncate max-w-[85px]">{edge.caller}</span>
              <svg viewBox="0 0 16 16" className="w-3.5 h-3.5 flex-shrink-0 text-muted-foreground/60">
                <path d="M2 8h10M8 4l4 4-4 4" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round" />
              </svg>
              <span className="truncate max-w-[85px]">{edge.callee}</span>
            </div>
            <div className="space-y-1">
              <TRow label="Throughput" value={`${edge.callCount} calls`} />
              <TRow label="Avg Latency" value={`${edge.avgDurationMs.toFixed(1)} ms`} />
              <TRow label="P95 Latency" value={`${edge.p95DurationMs.toFixed(1)} ms`} />
              <TRow
                label="Error Rate"
                value={`${edge.errorRate.toFixed(1)}%`}
                cls={edge.errorRate > 5 ? "text-red-500 font-semibold" : edge.errorRate > 1 ? "text-amber-500" : ""}
              />
            </div>
          </div>
        </foreignObject>
      )}
    </g>
  )
})

function TRow({ label, value, cls = "" }: { label: string; value: string; cls?: string }) {
  return (
    <div className="flex items-center justify-between text-[11px]">
      <span className="text-muted-foreground">{label}</span>
      <span className={`font-mono ${cls || "text-foreground"}`}>{value}</span>
    </div>
  )
}
