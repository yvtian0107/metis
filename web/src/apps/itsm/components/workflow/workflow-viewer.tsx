import { useMemo, useCallback, useState } from "react"
import { useTranslation } from "react-i18next"
import {
  ReactFlow,
  Background,
  Controls,
  MiniMap,
  type Node,
  type Edge,
  MarkerType,
} from "@xyflow/react"
import "@xyflow/react/dist/style.css"
import { X } from "lucide-react"
import { Button } from "@/components/ui/button"
import { Badge } from "@/components/ui/badge"
import { nodeTypes } from "./nodes"
import { edgeTypes } from "./custom-edges"
import { applyViewerLayout } from "./auto-layout"
import { getNodeAccent } from "./visual-data"
import { WorkflowNodeIconGlyph } from "./visual"
import { type WFNodeData, type WFEdgeData } from "./types"
import type { ActivityItem, TokenItem } from "../../api"
import "./style.css"

interface WorkflowViewerProps {
  workflowJson: unknown
  activities: ActivityItem[]
  tokens?: TokenItem[]
  currentActivityId?: number | null
}

export function WorkflowViewer({ workflowJson, activities, tokens = [], currentActivityId = null }: WorkflowViewerProps) {
  const { t } = useTranslation("itsm")
  const [popoverNodeId, setPopoverNodeId] = useState<string | null>(null)

  const useTokenMode = tokens.length > 0

  // Build token lookup by nodeId
  const tokensByNode = useMemo(() => {
    const map = new Map<string, TokenItem[]>()
    for (const tk of tokens) {
      const list = map.get(tk.nodeId) ?? []
      list.push(tk)
      map.set(tk.nodeId, list)
    }
    return map
  }, [tokens])

  // Build activity lookup: nodeId → ActivityItem[]
  const activitiesByNode = useMemo(() => {
    const map = new Map<string, ActivityItem[]>()
    for (const a of activities) {
      const list = map.get(a.nodeId) ?? []
      list.push(a)
      map.set(a.nodeId, list)
    }
    return map
  }, [activities])

  // Parse workflow JSON and apply state highlighting
  const { nodes, edges } = useMemo(() => {
    if (!workflowJson) return { nodes: [], edges: [] }

    let wf: { nodes?: unknown[]; edges?: unknown[] }
    try {
      wf = typeof workflowJson === "string" ? JSON.parse(workflowJson) : workflowJson
    } catch {
      return { nodes: [], edges: [] }
    }

    const rawNodes = (wf.nodes ?? []) as Array<{
      id: string; position: { x: number; y: number }; data?: Record<string, unknown>; type?: string
    }>
    const rawEdges = (wf.edges ?? []) as Array<{
      id: string; source: string; target: string; data?: Record<string, unknown>
    }>

    // Determine node states
    const completedNodeIds = new Set<string>()
    const activeNodeIds = new Set<string>()
    const cancelledNodeIds = new Set<string>()
    const failedNodeIds = new Set<string>()

    if (useTokenMode) {
      // Token-driven state
      for (const tk of tokens) {
        if (tk.status === "active") activeNodeIds.add(tk.nodeId)
        else if (tk.status === "completed") completedNodeIds.add(tk.nodeId)
        else if (tk.status === "failed") failedNodeIds.add(tk.nodeId)
        else if (tk.status === "cancelled") cancelledNodeIds.add(tk.nodeId)
      }
    } else {
      // Fallback: activity-driven state
      for (const a of activities) {
        if (a.id === currentActivityId) activeNodeIds.add(a.nodeId)
        if (a.status === "completed") completedNodeIds.add(a.nodeId)
        else if (a.status === "failed") failedNodeIds.add(a.nodeId)
        else if (a.status === "cancelled") cancelledNodeIds.add(a.nodeId)
        else if (a.status === "pending" || a.status === "in_progress") activeNodeIds.add(a.nodeId)
      }
    }

    // Track visited edges
    const visitedEdgeIds = new Set<string>()
    for (const e of rawEdges) {
      const srcDone = completedNodeIds.has(e.source) || activeNodeIds.has(e.source)
      const tgtDone = completedNodeIds.has(e.target) || activeNodeIds.has(e.target)
      if (srcDone && tgtDone) visitedEdgeIds.add(e.id)
    }

    const nodes = rawNodes.map((n) => {
      const nodeData = (n.data ?? {}) as unknown as WFNodeData
      const isActive = activeNodeIds.has(n.id)
      const isCompleted = completedNodeIds.has(n.id)
      const isCancelled = cancelledNodeIds.has(n.id)
      const isFailed = failedNodeIds.has(n.id)
      const workflowState: WFNodeData["_workflowState"] = isActive
        ? "active"
        : isFailed
          ? "failed"
          : isCompleted
            ? "completed"
            : isCancelled
              ? "cancelled"
              : "idle"

      return {
        id: n.id,
        type: nodeData.nodeType ?? (n.type === "workflow" ? "form" : n.type) ?? "form",
        position: n.position,
        data: { ...nodeData, _workflowState: workflowState },
        selectable: false,
        draggable: false,
      }
    }) as unknown as Node[]

    const edges: Edge[] = rawEdges.map((e) => ({
      id: e.id,
      source: e.source,
      target: e.target,
      type: "workflow",
      markerEnd: { type: MarkerType.ArrowClosed },
      data: {
        ...e.data,
        readonly: true,
        visited: visitedEdgeIds.has(e.id),
        failed: failedNodeIds.has(e.source) || failedNodeIds.has(e.target),
      } satisfies WFEdgeData,
    }))

    return { nodes: applyViewerLayout(nodes, edges), edges }
  }, [workflowJson, activities, tokens, useTokenMode, currentActivityId])

  const onNodeClick = useCallback((_: React.MouseEvent, node: Node) => {
    setPopoverNodeId((prev) => (prev === node.id ? null : node.id))
  }, [])

  const onPaneClick = useCallback(() => {
    setPopoverNodeId(null)
  }, [])

  const popoverActivities = popoverNodeId ? (activitiesByNode.get(popoverNodeId) ?? []) : []

  return (
    <div className="flex min-h-[460px] overflow-hidden rounded-2xl border border-border/55 bg-white/38">
      <div className="min-w-0 flex-1">
        <ReactFlow
          nodes={nodes}
          edges={edges}
          nodeTypes={nodeTypes}
          edgeTypes={edgeTypes}
          onNodeClick={onNodeClick}
          onPaneClick={onPaneClick}
          nodesDraggable={false}
          nodesConnectable={false}
          elementsSelectable={false}
          fitView
          fitViewOptions={{ padding: 0.12 }}
          className="workflow-builder-flow"
        >
          <Background gap={24} size={1.2} />
          <Controls showInteractive={false} position="bottom-left" />
          <MiniMap
            position="bottom-right"
            pannable
            zoomable
            nodeColor={(n) => {
              const nodeData = n.data as unknown as WFNodeData
              if (useTokenMode) {
                const tks = tokensByNode.get(n.id)
                if (tks?.some((tk) => tk.status === "active")) return "#2563eb"
                if (tks?.some((tk) => tk.status === "completed")) return "#059669"
                if (tks?.some((tk) => tk.status === "failed")) return "#dc2626"
              }
              return getNodeAccent(nodeData?.nodeType)
            }}
            maskColor="rgba(15,23,42,0.06)"
          />
        </ReactFlow>
      </div>
      {popoverNodeId && (
        <aside className="w-[340px] shrink-0 overflow-auto border-l border-border/50 bg-white/70 p-4">
          <div className="mb-3 flex items-center justify-between gap-2">
            {(() => {
              const node = nodes.find((item) => item.id === popoverNodeId)
              const data = node?.data as unknown as WFNodeData | undefined
              return (
                <div className="flex min-w-0 items-center gap-2">
                  <span className="flex size-7 shrink-0 items-center justify-center rounded-lg text-white" style={{ backgroundColor: getNodeAccent(data?.nodeType) }}>
                    <WorkflowNodeIconGlyph nodeType={data?.nodeType} />
                  </span>
                  <div className="min-w-0">
                    <div className="truncate text-sm font-semibold">{data?.label ?? popoverNodeId}</div>
                    <div className="text-xs text-muted-foreground">{data?.nodeType ? t(`workflow.node.${data.nodeType}` as const) : t("workflow.viewer.activityDetail")}</div>
                  </div>
                </div>
              )
            })()}
            <Button variant="ghost" size="icon" className="h-7 w-7" onClick={() => setPopoverNodeId(null)}>
              <X className="h-3.5 w-3.5" />
            </Button>
          </div>
          {popoverActivities.length === 0 ? (
            <p className="text-xs text-muted-foreground">{t("workflow.viewer.noActivities")}</p>
          ) : (
            <div className="space-y-2">
              {popoverActivities.map((a) => (
                <div key={a.id} className="flex items-center gap-2 text-sm">
                  <span className="font-medium">{a.name}</span>
                  <Badge
                    variant={a.status === "completed" ? "default" : a.status === "failed" ? "destructive" : "secondary"}
                    className="text-xs"
                  >
                    {a.status}
                  </Badge>
                  {a.transitionOutcome && (
                    <Badge variant="outline" className="text-xs">
                      {a.transitionOutcome}
                    </Badge>
                  )}
                  {a.finishedAt && (
                    <span className="text-xs text-muted-foreground">
                      {new Date(a.finishedAt).toLocaleString()}
                    </span>
                  )}
                </div>
              ))}
            </div>
          )}
        </aside>
      )}
    </div>
  )
}
