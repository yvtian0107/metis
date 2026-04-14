import { useCallback, useMemo } from "react"
import { useNavigate } from "react-router"
import { useTranslation } from "react-i18next"
import dagre from "@dagrejs/dagre"

import type { TopologyGraph } from "../api"

interface ServiceMapProps {
  graph: TopologyGraph
}

const NODE_WIDTH = 160
const NODE_HEIGHT = 64
const PADDING = 40

export function ServiceMap({ graph }: ServiceMapProps) {
  const { t } = useTranslation("apm")
  const navigate = useNavigate()

  const layout = useMemo(() => {
    const g = new dagre.graphlib.Graph()
    g.setGraph({ rankdir: "LR", nodesep: 60, ranksep: 120 })
    g.setDefaultEdgeLabel(() => ({}))

    for (const node of graph.nodes) {
      g.setNode(node.serviceName, { width: NODE_WIDTH, height: NODE_HEIGHT })
    }
    for (const edge of graph.edges) {
      g.setEdge(edge.caller, edge.callee)
    }

    dagre.layout(g)

    const nodes = graph.nodes.map((n) => {
      const pos = g.node(n.serviceName)
      return { ...n, x: pos.x, y: pos.y }
    })

    const edges = graph.edges.map((e) => {
      const points = g.edge(e.caller, e.callee)?.points ?? []
      return { ...e, points }
    })

    const graphMeta = g.graph()
    const width = (graphMeta.width ?? 400) + PADDING * 2
    const height = (graphMeta.height ?? 300) + PADDING * 2

    return { nodes, edges, width, height }
  }, [graph])

  const handleNodeClick = useCallback(
    (serviceName: string) => {
      navigate(`/apm/services/${encodeURIComponent(serviceName)}`)
    },
    [navigate],
  )

  const buildPath = useCallback((points: Array<{ x: number; y: number }>) => {
    if (points.length === 0) return ""
    const [first, ...rest] = points
    let d = `M ${first.x + PADDING} ${first.y + PADDING}`
    if (rest.length <= 1) {
      for (const p of rest) {
        d += ` L ${p.x + PADDING} ${p.y + PADDING}`
      }
    } else {
      // smooth curve through points
      for (let i = 0; i < rest.length; i++) {
        const p = rest[i]
        if (i === 0) {
          const prev = first
          const cx = (prev.x + p.x) / 2 + PADDING
          const cy = (prev.y + p.y) / 2 + PADDING
          d += ` Q ${prev.x + PADDING} ${prev.y + PADDING} ${cx} ${cy}`
        } else {
          const prev = rest[i - 1]
          const cx = (prev.x + p.x) / 2 + PADDING
          const cy = (prev.y + p.y) / 2 + PADDING
          d += ` T ${cx} ${cy}`
        }
      }
      const last = rest[rest.length - 1]
      d += ` L ${last.x + PADDING} ${last.y + PADDING}`
    }
    return d
  }, [])

  const edgeStrokeWidth = useCallback(
    (callCount: number) => {
      const maxCalls = Math.max(...graph.edges.map((e) => e.callCount), 1)
      return 1.5 + (callCount / maxCalls) * 3
    },
    [graph.edges],
  )

  return (
    <svg width={layout.width} height={layout.height} className="overflow-visible">
      <defs>
        <marker id="arrow" viewBox="0 0 10 6" refX="10" refY="3" markerWidth="8" markerHeight="6" orient="auto">
          <path d="M 0 0 L 10 3 L 0 6 Z" className="fill-muted-foreground/60" />
        </marker>
        <marker
          id="arrow-error"
          viewBox="0 0 10 6"
          refX="10"
          refY="3"
          markerWidth="8"
          markerHeight="6"
          orient="auto"
        >
          <path d="M 0 0 L 10 3 L 0 6 Z" className="fill-red-500/70" />
        </marker>
      </defs>

      {/* Edges */}
      {layout.edges.map((edge) => {
        const isError = edge.errorRate > 0.05
        return (
          <g key={`${edge.caller}-${edge.callee}`}>
            <path
              d={buildPath(edge.points)}
              fill="none"
              className={isError ? "stroke-red-500/60" : "stroke-muted-foreground/30"}
              strokeWidth={edgeStrokeWidth(edge.callCount)}
              markerEnd={isError ? "url(#arrow-error)" : "url(#arrow)"}
            />
            {edge.points.length > 0 && (
              <EdgeLabel edge={edge} points={edge.points} t={t} />
            )}
          </g>
        )
      })}

      {/* Nodes */}
      {layout.nodes.map((node) => {
        const isError = node.errorRate > 0.05
        const x = node.x - NODE_WIDTH / 2 + PADDING
        const y = node.y - NODE_HEIGHT / 2 + PADDING
        return (
          <g
            key={node.serviceName}
            className="cursor-pointer"
            onClick={() => handleNodeClick(node.serviceName)}
          >
            <rect
              x={x}
              y={y}
              width={NODE_WIDTH}
              height={NODE_HEIGHT}
              rx={8}
              className={
                isError
                  ? "fill-red-500/10 stroke-red-500/50"
                  : "fill-card stroke-border"
              }
              strokeWidth={1.5}
            />
            <text
              x={x + NODE_WIDTH / 2}
              y={y + 24}
              textAnchor="middle"
              className="fill-foreground text-xs font-medium"
            >
              {node.serviceName.length > 18
                ? node.serviceName.slice(0, 16) + "..."
                : node.serviceName}
            </text>
            <text
              x={x + NODE_WIDTH / 2}
              y={y + 44}
              textAnchor="middle"
              className={`text-[10px] ${isError ? "fill-red-500" : "fill-muted-foreground"}`}
            >
              {node.requestCount} {t("topology.requests")}
              {node.errorRate > 0 ? ` | ${(node.errorRate * 100).toFixed(1)}% err` : ""}
            </text>
          </g>
        )
      })}
    </svg>
  )
}

function EdgeLabel({
  edge,
  points,
  t,
}: {
  edge: { avgDurationMs: number; callCount: number; errorRate: number }
  points: Array<{ x: number; y: number }>
  t: (key: string) => string
}) {
  const mid = points[Math.floor(points.length / 2)]
  const isError = edge.errorRate > 0.05
  return (
    <text
      x={mid.x + PADDING}
      y={mid.y + PADDING - 6}
      textAnchor="middle"
      className={`text-[9px] ${isError ? "fill-red-500/80" : "fill-muted-foreground/70"}`}
    >
      {edge.callCount} {t("topology.calls")} | {edge.avgDurationMs.toFixed(0)}ms
    </text>
  )
}
