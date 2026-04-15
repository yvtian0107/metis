"use client"

import { useMemo } from "react"
import {
  ReactFlow, Background, Controls, MiniMap,
  type Node, type Edge, MarkerType,
} from "@xyflow/react"
import "@xyflow/react/dist/style.css"
import { nodeTypes } from "../../../components/workflow/custom-nodes"
import { type WFNodeData, NODE_COLORS } from "../../../components/workflow/types"

interface WorkflowPreviewProps {
  workflowJson: unknown
}

export default function WorkflowPreview({ workflowJson }: WorkflowPreviewProps) {
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

    const nodes: Node[] = rawNodes.map((n) => ({
      id: n.id,
      type: "workflow",
      position: n.position,
      data: (n.data ?? {}) as WFNodeData,
      selectable: false,
      draggable: false,
    }))

    const edges: Edge[] = rawEdges.map((e) => ({
      id: e.id,
      source: e.source,
      target: e.target,
      type: "smoothstep",
      markerEnd: { type: MarkerType.ArrowClosed },
      data: e.data,
      style: { stroke: "#6b7280", strokeWidth: 1.5 },
    }))

    return { nodes, edges }
  }, [workflowJson])

  return (
    <div className="h-[500px] w-full rounded-md border">
      <ReactFlow
        nodes={nodes}
        edges={edges}
        nodeTypes={nodeTypes}
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
            const nodeData = n.data as WFNodeData
            return NODE_COLORS[nodeData?.nodeType] ?? "#6b7280"
          }}
          maskColor="rgba(0,0,0,0.05)"
        />
      </ReactFlow>
    </div>
  )
}
