import { useState, useEffect, useCallback, useMemo, useRef } from "react"
import { useTranslation } from "react-i18next"
import { BookOpen, FileText, Maximize2, ChevronDown, ChevronRight, Loader2 } from "lucide-react"
import ForceGraph2D from "react-force-graph-2d"
import { useQuery } from "@tanstack/react-query"
import { api } from "@/lib/api"
import { Button } from "@/components/ui/button"
import { Badge } from "@/components/ui/badge"
import { useKbSources } from "../hooks/use-kb-sources"
import { RELATION_COLORS, NODE_COLORS } from "./graph-constants"
import type { GraphResponse, EdgeItem, SourceItem } from "../types"

interface GraphNode {
  id: string
  name: string
  summary: string
  nodeType: string
  edgeCount: number
  sourceIds?: number[]
  hasContent?: boolean
  color: string
  val: number
  x?: number
  y?: number
}

interface GraphLink {
  source: string
  target: string
  relation: string
  color: string
}

export function KnowledgeGraphView({ kbId, highlightedNodeIds }: { kbId: number; highlightedNodeIds?: Set<string> }) {
  const { t } = useTranslation(["ai", "common"])
  const containerRef = useRef<HTMLDivElement>(null)
  const graphRef = useRef<{ zoomToFit: (ms?: number, px?: number) => void; zoom: (k: number, ms?: number) => void }>(null)
  const [selectedNode, setSelectedNode] = useState<GraphNode | null>(null)
  const [containerSize, setContainerSize] = useState<{ w: number; h: number } | null>(null)
  const [nodeContent, setNodeContent] = useState<string | null>(null)
  const [loadingContent, setLoadingContent] = useState(false)
  const [showContent, setShowContent] = useState(false)

  const { data, isLoading } = useQuery({
    queryKey: ["ai-kb-graph", kbId],
    queryFn: () => api.get<GraphResponse>(`/api/v1/ai/knowledge-bases/${kbId}/graph`),
  })

  const { data: sourcesData } = useKbSources(kbId)
  const sources = sourcesData?.items ?? []

  // ResizeObserver — container always mounted so ref is valid on first effect run
  useEffect(() => {
    const el = containerRef.current
    if (!el) return
    const observer = new ResizeObserver((entries) => {
      for (const entry of entries) {
        const { width, height } = entry.contentRect
        if (width > 0 && height > 0) {
          setContainerSize({ w: Math.round(width), h: Math.round(height) })
        }
      }
    })
    observer.observe(el)
    return () => observer.disconnect()
  }, [])

  const graphData = useMemo(() => {
    if (!data) return { nodes: [] as GraphNode[], links: [] as GraphLink[] }
    const nodes: GraphNode[] = data.nodes.map((n) => ({
      id: n.id,
      name: n.title,
      summary: n.summary,
      nodeType: n.nodeType,
      edgeCount: n.edgeCount,
      sourceIds: n.sourceIds,
      hasContent: n.hasContent,
      color: NODE_COLORS[n.nodeType] ?? NODE_COLORS.concept,
      val: 1 + Math.min(n.edgeCount, 10) * 0.3,
    }))
    const nodeIds = new Set(nodes.map((n) => n.id))
    const links: GraphLink[] = (data.edges as EdgeItem[])
      .filter((e) => nodeIds.has(e.fromNodeId) && nodeIds.has(e.toNodeId))
      .map((e) => ({
        source: e.fromNodeId,
        target: e.toNodeId,
        relation: e.relation,
        color: RELATION_COLORS[e.relation] ?? RELATION_COLORS.related,
      }))
    return { nodes, links }
  }, [data])

  const handleNodeClick = useCallback((node: GraphNode) => {
    setSelectedNode((prev) => {
      if (prev?.id === node.id) {
        // 关闭面板时重置内容状态
        setShowContent(false)
        setNodeContent(null)
        return null
      }
      // 切换节点时重置内容状态
      setShowContent(false)
      setNodeContent(null)
      return node
    })
  }, [])

  // 加载节点原文内容
  const toggleNodeContent = useCallback(async () => {
    if (!selectedNode || !selectedNode.hasContent) return
    // 如果已经显示内容，则收起
    if (showContent) {
      setShowContent(false)
      return
    }
    // 如果已经加载过内容，直接显示
    if (nodeContent !== null) {
      setShowContent(true)
      return
    }
    // 加载内容
    setLoadingContent(true)
    try {
      const resp = await api.get<{ content?: string }>(`/api/v1/ai/knowledge-bases/${kbId}/nodes/${selectedNode.id}`)
      setNodeContent(resp.content ?? "")
      setShowContent(true)
    } catch {
      setNodeContent("")
      setShowContent(true)
    } finally {
      setLoadingContent(false)
    }
  }, [selectedNode, kbId, nodeContent, showContent])

  // 截断长标签，添加省略号
  const truncateLabel = useCallback((text: string, maxLength: number = 12) => {
    if (text.length <= maxLength) return text
    return text.slice(0, maxLength) + "..."
  }, [])

  const nodeCanvasObject = useCallback(
    (node: GraphNode, ctx: CanvasRenderingContext2D, globalScale: number) => {
      const label = truncateLabel(node.name)
      const fontSize = Math.max(11 / globalScale, 2)
      ctx.font = `${fontSize}px sans-serif`
      const isSelected = selectedNode?.id === node.id
      const isHighlighted = highlightedNodeIds?.has(node.id) ?? false

      // Node circle
      const r = Math.sqrt(node.val) * 4
      ctx.beginPath()
      ctx.arc(node.x ?? 0, node.y ?? 0, r, 0, 2 * Math.PI)
      ctx.fillStyle = isSelected ? "#f59e0b" : node.color
      ctx.fill()

      // Selection ring (orange)
      if (isSelected) {
        ctx.strokeStyle = "#f59e0b"
        ctx.lineWidth = 2 / globalScale
        ctx.stroke()
      }

      // Highlight ring (green) — for recall search results
      if (isHighlighted && !isSelected) {
        ctx.beginPath()
        ctx.arc(node.x ?? 0, node.y ?? 0, r + 3 / globalScale, 0, 2 * Math.PI)
        ctx.strokeStyle = "#22c55e"
        ctx.lineWidth = 2 / globalScale
        ctx.stroke()
      }

      // Label - 提高显示阈值到 0.8，减少缩放小时的混乱
      if (globalScale > 0.8) {
        ctx.textAlign = "center"
        ctx.textBaseline = "top"
        ctx.fillStyle = isSelected ? "#f59e0b" : "#374151"
        ctx.fillText(label, node.x ?? 0, (node.y ?? 0) + r + 2)
      }
    },
    [selectedNode, highlightedNodeIds, truncateLabel],
  )

  const nodePointerAreaPaint = useCallback(
    (node: GraphNode, color: string, ctx: CanvasRenderingContext2D) => {
      const r = Math.sqrt(node.val) * 4 + 2
      ctx.beginPath()
      ctx.arc(node.x ?? 0, node.y ?? 0, r, 0, 2 * Math.PI)
      ctx.fillStyle = color
      ctx.fill()
    },
    [],
  )

  const handleZoomIn = useCallback(() => {
    graphRef.current?.zoom(1.5, 300)
  }, [])

  const handleZoomOut = useCallback(() => {
    graphRef.current?.zoom(0.67, 300)
  }, [])

  const handleZoomFit = useCallback(() => {
    graphRef.current?.zoomToFit(400, 40)
  }, [])

  // Auto-fit when engine stops (simulation completes)
  const handleEngineStop = useCallback(() => {
    graphRef.current?.zoomToFit(600, 50)
  }, [])

  // Auto-fit when graph data changes (initial load)
  useEffect(() => {
    if (graphData.nodes.length > 0 && containerSize) {
      // Small delay to ensure graph has rendered
      const timer = setTimeout(() => {
        graphRef.current?.zoomToFit(400, 50)
      }, 100)
      return () => clearTimeout(timer)
    }
  }, [graphData.nodes.length, containerSize])

  const showGraph = !isLoading && graphData.nodes.length > 0 && containerSize

  return (
    <div
      ref={containerRef}
      className="relative rounded-lg border bg-card overflow-hidden"
      style={{ height: "calc(100vh - 280px)", minHeight: 400 }}
    >
      {isLoading && (
        <div className="absolute inset-0 flex items-center justify-center text-sm text-muted-foreground">
          {t("common:loading")}
        </div>
      )}
      {!isLoading && graphData.nodes.length === 0 && (
        <div className="absolute inset-0 flex flex-col items-center justify-center gap-2 text-muted-foreground">
          <BookOpen className="h-10 w-10 stroke-1" />
          <p className="text-sm font-medium">{t("ai:knowledge.graph.emptyGraph")}</p>
        </div>
      )}
      {showGraph && (
        <ForceGraph2D
          ref={graphRef}
          graphData={graphData}
          width={containerSize.w}
          height={containerSize.h}
          nodeId="id"
          linkSource="source"
          linkTarget="target"
          linkColor={(link: GraphLink) => link.color}
          linkWidth={1.5}
          linkDirectionalArrowLength={4}
          linkDirectionalArrowRelPos={1}
          linkLabel={(link: GraphLink) => link.relation}
          nodeCanvasObject={nodeCanvasObject}
          nodePointerAreaPaint={nodePointerAreaPaint}
          onNodeClick={handleNodeClick}
          cooldownTicks={100}
          warmupTicks={30}
          enableZoomInteraction={true}
          enablePanInteraction={true}
          onEngineStop={handleEngineStop}
          // Tooltip: 悬停显示完整标题和节点信息
          nodeLabel={(node: GraphNode) => `${node.name}\n类型: ${t(`ai:knowledge.graph.nodeTypes.${node.nodeType}`)}\n关联数: ${node.edgeCount}`}
          // 增加节点间距，减少重叠
          d3Force={(engine: { force: (name: string) => { strength?: (v: number) => void; distance?: (v: number) => void } | undefined }) => {
            engine.force("charge")?.strength?.(-300)
            engine.force("link")?.distance?.(80)
          }}
        />
      )}
      {/* Legend */}
      {showGraph && (
        <div className="absolute bottom-3 left-3 flex items-center gap-3 rounded-md bg-card/90 backdrop-blur px-2.5 py-1.5 text-xs text-muted-foreground border">
          <span className="flex items-center gap-1">
            <span className="inline-block h-2.5 w-2.5 rounded-full" style={{ backgroundColor: NODE_COLORS.concept }} />
            {t("ai:knowledge.graph.nodeTypes.concept")}
          </span>
          <span className="flex items-center gap-1">
            <span className="inline-block h-2 w-4 rounded-sm" style={{ backgroundColor: RELATION_COLORS.related }} />
            {t("ai:knowledge.graph.edgeTypes.related")}
          </span>
          <span className="flex items-center gap-1">
            <span className="inline-block h-2 w-4 rounded-sm" style={{ backgroundColor: RELATION_COLORS.contradicts }} />
            {t("ai:knowledge.graph.edgeTypes.contradicts")}
          </span>
        </div>
      )}
      {/* Zoom controls */}
      {showGraph && (
        <div className="absolute bottom-3 right-3 flex flex-col gap-1">
          <Button variant="outline" size="icon" className="h-7 w-7 bg-card/90 backdrop-blur" onClick={handleZoomIn}>
            <span className="text-sm font-bold">+</span>
          </Button>
          <Button variant="outline" size="icon" className="h-7 w-7 bg-card/90 backdrop-blur" onClick={handleZoomOut}>
            <span className="text-sm font-bold">−</span>
          </Button>
          <Button variant="outline" size="icon" className="h-7 w-7 bg-card/90 backdrop-blur" onClick={handleZoomFit} title="Fit">
            <Maximize2 className="h-3.5 w-3.5" />
          </Button>
        </div>
      )}
      {/* Node detail panel */}
      {selectedNode && (
        <div className="absolute top-3 right-3 w-80 rounded-lg border bg-card/95 backdrop-blur p-3 shadow-lg text-sm max-h-[calc(100%-24px)] overflow-y-auto">
          <div className="flex items-center gap-2 mb-2">
            <h4 className="font-semibold truncate flex-1">{selectedNode.name}</h4>
            <Badge variant="outline" className="border-transparent text-xs" style={{ backgroundColor: selectedNode.color + "20", color: selectedNode.color }}>
              {t(`ai:knowledge.graph.nodeTypes.${selectedNode.nodeType}`)}
            </Badge>
          </div>
          <p className="text-muted-foreground text-xs leading-relaxed">
            {selectedNode.summary || "—"}
          </p>
          <div className="flex items-center gap-3 mt-2 text-xs text-muted-foreground">
            <span>{t("ai:knowledge.nodes.edgeCount")}: {selectedNode.edgeCount}</span>
          </div>
          
          {/* 查看原文按钮 */}
          {selectedNode.hasContent && (
            <div className="mt-2">
              <button
                onClick={toggleNodeContent}
                disabled={loadingContent}
                className="flex items-center gap-1 text-xs text-primary hover:underline disabled:opacity-50"
              >
                {loadingContent ? (
                  <>
                    <Loader2 className="h-3 w-3 animate-spin" />
                    {t("ai:knowledge.nodes.loadingContent")}
                  </>
                ) : showContent ? (
                  <>
                    <ChevronDown className="h-3 w-3" />
                    {t("ai:knowledge.recall.hideContent")}
                  </>
                ) : (
                  <>
                    <ChevronRight className="h-3 w-3" />
                    {t("ai:knowledge.recall.viewContent")}
                  </>
                )}
              </button>
            </div>
          )}
          
          {/* 原文内容 */}
          {showContent && nodeContent !== null && (
            <div className="mt-2">
              <pre className="rounded bg-muted p-2 text-xs font-mono whitespace-pre-wrap break-words max-h-48 overflow-y-auto">
                {nodeContent || t("ai:knowledge.nodes.noContent")}
              </pre>
            </div>
          )}
          
          {selectedNode.sourceIds && selectedNode.sourceIds.length > 0 && sources.length > 0 && (
            <div className="mt-2 pt-2 border-t">
              <span className="text-xs font-medium">{t("ai:knowledge.nodes.sources")}</span>
              <div className="mt-1 space-y-0.5">
                {selectedNode.sourceIds
                  .map(sid => sources.find((s: SourceItem) => s.id === sid))
                  .filter((s): s is SourceItem => s != null)
                  .map(src => (
                    <div key={src.id} className="flex items-center gap-1 text-xs text-muted-foreground">
                      <FileText className="h-3 w-3 shrink-0" />
                      <span className="truncate">{src.title}</span>
                      <Badge variant="outline" className="text-[10px] px-1 py-0">{src.format}</Badge>
                    </div>
                  ))}
              </div>
            </div>
          )}
        </div>
      )}
    </div>
  )
}
