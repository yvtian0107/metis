"use client"

import { useState, useMemo, useCallback } from "react"
import { useTranslation } from "react-i18next"
import {
  ReactFlow, Background, Controls, MiniMap,
  type Node, MarkerType,
} from "@xyflow/react"
import "@xyflow/react/dist/style.css"
import { X } from "lucide-react"
import { Button } from "@/components/ui/button"
import { Badge } from "@/components/ui/badge"
import { nodeTypes } from "../../../components/workflow/nodes"
import { edgeTypes } from "../../../components/workflow/custom-edges"
import { type WFNodeData, type NodeType, type Participant, NODE_COLORS } from "../../../components/workflow/types"

interface WorkflowPreviewProps {
  workflowJson: unknown
}

/** Format a participant for display */
function formatParticipant(p: Participant): string {
  if (p.type === "position_department") {
    const parts = [
      (p as unknown as Record<string, unknown>).department_code,
      (p as unknown as Record<string, unknown>).position_code,
    ].filter(Boolean)
    if (parts.length > 0) return parts.join(" / ")
  }
  if (p.name) return p.name
  if (p.value) return String(p.value)
  if (p.id) return String(p.id)
  return "-"
}

/** Get translated label for participant type */
function participantTypeLabel(type: string, t: (k: string) => string): string {
  const map: Record<string, string> = {
    user: t("workflow.participant.user"),
    position: t("workflow.participant.position"),
    department: t("workflow.participant.department"),
    position_department: t("workflow.participant.positionDepartment"),
    requester_manager: t("workflow.participant.requesterManager"),
  }
  return map[type] ?? type
}

export default function WorkflowPreview({ workflowJson }: WorkflowPreviewProps) {
  const { t } = useTranslation("itsm")
  const [selectedNode, setSelectedNode] = useState<{ id: string; data: WFNodeData } | null>(null)

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

    const nodes = rawNodes.map((n) => {
      const rawData = (n.data ?? {}) as Record<string, unknown>
      const nodeType = (rawData.nodeType ?? n.type ?? "process") as NodeType
      return {
        id: n.id,
        type: nodeType,
        position: n.position,
        data: { ...rawData, nodeType } as unknown as WFNodeData,
        selectable: true as const,
        draggable: false as const,
      }
    }) as unknown as Node[]

    const edges = rawEdges.map((e) => ({
      id: e.id,
      source: e.source,
      target: e.target,
      type: "workflow",
      markerEnd: { type: MarkerType.ArrowClosed },
      data: e.data,
      style: { strokeWidth: 1.5 },
    }))

    return { nodes, edges }
  }, [workflowJson])

  const onNodeClick = useCallback((_: React.MouseEvent, node: Node) => {
    setSelectedNode({ id: node.id, data: node.data as unknown as WFNodeData })
  }, [])

  return (
    <div className="flex gap-4">
      <div className={`${selectedNode ? "w-2/3" : "w-full"} h-[500px] rounded-md border transition-all`}>
        <ReactFlow
          nodes={nodes}
          edges={edges}
          nodeTypes={nodeTypes as any}
          edgeTypes={edgeTypes}
          nodesDraggable={false}
          nodesConnectable={false}
          elementsSelectable={true}
          onNodeClick={onNodeClick}
          onPaneClick={() => setSelectedNode(null)}
          fitView
          className="bg-muted/20"
        >
          <Background />
          <Controls showInteractive={false} />
          <MiniMap
            nodeColor={(n) => {
              const nodeData = n.data as unknown as WFNodeData
              return NODE_COLORS[nodeData?.nodeType] ?? "#6b7280"
            }}
            maskColor="rgba(0,0,0,0.05)"
          />
        </ReactFlow>
      </div>

      {selectedNode && (
        <div className="w-1/3 rounded-md border p-4 space-y-3">
          <div className="flex items-center justify-between">
            <h4 className="text-sm font-semibold">{t("workflow.viewer.activityDetail")}</h4>
            <Button variant="ghost" size="icon" className="h-6 w-6" onClick={() => setSelectedNode(null)}>
              <X className="h-3.5 w-3.5" />
            </Button>
          </div>

          <div className="space-y-2 text-sm">
            <div className="flex items-center gap-2">
              <span className="text-muted-foreground">{t("workflow.prop.label")}:</span>
              <span className="font-medium">{selectedNode.data.label}</span>
            </div>

            <div className="flex items-center gap-2">
              <span className="text-muted-foreground">{t("generate.nodeType")}:</span>
              <Badge
                variant="outline"
                style={{ borderColor: NODE_COLORS[selectedNode.data.nodeType], color: NODE_COLORS[selectedNode.data.nodeType] }}
              >
                {t(`workflow.node.${selectedNode.data.nodeType}` as const)}
              </Badge>
            </div>

            {selectedNode.data.participants && selectedNode.data.participants.length > 0 && (
              <div>
                <span className="text-muted-foreground">{t("workflow.prop.participants")}:</span>
                <div className="mt-1 flex flex-wrap gap-1">
                  {selectedNode.data.participants.map((p, i) => (
                    <Badge key={i} variant="secondary" className="text-xs">
                      {participantTypeLabel(p.type, t)} : {formatParticipant(p)}
                    </Badge>
                  ))}
                </div>
              </div>
            )}

            {selectedNode.data.executionMode && (
              <div className="flex items-center gap-2">
                <span className="text-muted-foreground">{t("workflow.prop.executionMode")}:</span>
                <span>{t(`workflow.prop.mode${selectedNode.data.executionMode.charAt(0).toUpperCase()}${selectedNode.data.executionMode.slice(1)}` as const)}</span>
              </div>
            )}

            {selectedNode.data.actionId && (
              <div className="flex items-center gap-2">
                <span className="text-muted-foreground">Action ID:</span>
                <span>{selectedNode.data.actionId}</span>
              </div>
            )}

            {selectedNode.data.waitMode && (
              <div className="flex items-center gap-2">
                <span className="text-muted-foreground">{t("workflow.prop.waitMode")}:</span>
                <span>{t(`workflow.prop.wait${selectedNode.data.waitMode.charAt(0).toUpperCase()}${selectedNode.data.waitMode.slice(1)}` as const)}</span>
                {selectedNode.data.duration && <span className="text-muted-foreground">({selectedNode.data.duration})</span>}
              </div>
            )}

            {selectedNode.data.formSchema != null && (
              <div>
                <span className="text-muted-foreground">{t("services.formSchema")}:</span>
                <pre className="mt-1 max-h-40 overflow-auto rounded bg-muted p-2 text-xs">
                  {JSON.stringify(selectedNode.data.formSchema, null, 2)}
                </pre>
              </div>
            )}
          </div>
        </div>
      )}
    </div>
  )
}
