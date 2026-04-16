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
import { Badge } from "@/components/ui/badge"
import { nodeTypes } from "./nodes"
import { edgeTypes } from "./custom-edges"
import { type WFNodeData, NODE_COLORS } from "./types"
import type { ActivityItem, TokenItem } from "../../api"

interface WorkflowViewerProps {
  workflowJson: unknown
  activities: ActivityItem[]
  tokens?: TokenItem[]
  currentActivityId?: number | null
}

export function WorkflowViewer({ workflowJson, activities, tokens = [] }: WorkflowViewerProps) {
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

    if (useTokenMode) {
      // Token-driven state
      for (const tk of tokens) {
        if (tk.status === "active") activeNodeIds.add(tk.nodeId)
        else if (tk.status === "completed") completedNodeIds.add(tk.nodeId)
        else if (tk.status === "cancelled") cancelledNodeIds.add(tk.nodeId)
      }
    } else {
      // Fallback: activity-driven state
      for (const a of activities) {
        if (a.status === "completed") completedNodeIds.add(a.nodeId)
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

      let className = ""
      if (isActive) className = "ring-2 ring-green-500 ring-offset-2 animate-pulse"
      else if (isCompleted) className = "opacity-70"
      else if (isCancelled) className = "opacity-50 line-through"
      else className = "opacity-40"

      return {
        id: n.id,
        type: nodeData.nodeType ?? (n.type === "workflow" ? "form" : n.type) ?? "form",
        position: n.position,
        data: nodeData,
        className,
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
      data: e.data,
      style: visitedEdgeIds.has(e.id)
        ? { stroke: "#22c55e", strokeWidth: 2 }
        : { stroke: "#d4d4d8", strokeWidth: 1 },
      animated: visitedEdgeIds.has(e.id),
    }))

    return { nodes, edges }
  }, [workflowJson, activities, tokens, useTokenMode])

  const onNodeClick = useCallback((_: React.MouseEvent, node: Node) => {
    setPopoverNodeId((prev) => (prev === node.id ? null : node.id))
  }, [])

  const onPaneClick = useCallback(() => {
    setPopoverNodeId(null)
  }, [])

  const popoverActivities = popoverNodeId ? (activitiesByNode.get(popoverNodeId) ?? []) : []

  return (
    <div className="h-[400px] w-full rounded-md border">
      <ReactFlow
        nodes={nodes}
        edges={edges}
        nodeTypes={nodeTypes as any}
        edgeTypes={edgeTypes}
        onNodeClick={onNodeClick}
        onPaneClick={onPaneClick}
        nodesDraggable={false}
        nodesConnectable={false}
        elementsSelectable={false}
        fitView
        className="bg-muted/20"
      >
        <Background />
        <Controls showInteractive={false} />
        <MiniMap
          nodeColor={(n) => {
            const nodeData = n.data as unknown as WFNodeData
            if (useTokenMode) {
              const tks = tokensByNode.get(n.id)
              if (tks?.some((tk) => tk.status === "active")) return "#22c55e"
              if (tks?.some((tk) => tk.status === "completed")) return "#9ca3af"
              return NODE_COLORS[nodeData?.nodeType] ?? "#6b7280"
            }
            // Activity fallback
            const acts = activitiesByNode.get(n.id)
            if (acts?.some((a) => a.status === "completed")) return "#22c55e"
            if (acts?.some((a) => a.status === "pending" || a.status === "in_progress")) return "#3b82f6"
            return NODE_COLORS[nodeData?.nodeType] ?? "#6b7280"
          }}
          maskColor="rgba(0,0,0,0.05)"
        />
      </ReactFlow>
      {popoverNodeId && (
        <div className="border-t bg-muted/30 p-3 max-h-48 overflow-auto">
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
          <button className="mt-2 text-xs text-muted-foreground underline" onClick={() => setPopoverNodeId(null)}>
            {t("workflow.viewer.close")}
          </button>
        </div>
      )}
    </div>
  )
}
