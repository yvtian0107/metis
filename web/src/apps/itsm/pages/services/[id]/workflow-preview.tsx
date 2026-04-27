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
import { applyDagreLayout } from "../../../components/workflow/auto-layout"
import { getNodeAccent } from "../../../components/workflow/visual-data"
import { WorkflowNodeIconGlyph } from "../../../components/workflow/visual"
import { type WFNodeData, type NodeType, type Participant, type WFEdgeData } from "../../../components/workflow/types"
import "../../../components/workflow/style.css"

interface WorkflowPreviewProps {
  workflowJson: unknown
}

/** Format a participant for display */
function formatParticipant(p: Participant): string {
  if (p.type === "position_department") {
    const parts = [p.department_code, p.position_code].filter(Boolean)
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

/** Parse formSchema fields for display */
function parsePreviewFields(schema: unknown): Array<{ key: string; type: string; label: string; options?: string[] }> {
  if (!schema || typeof schema !== "object") return []
  const s = schema as { fields?: Array<{ key: string; type: string; label: string; options?: string[] }> }
  return Array.isArray(s.fields) ? s.fields : []
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
      data: { ...e.data, readonly: true } satisfies WFEdgeData,
    }))

    return { nodes: applyDagreLayout(nodes, edges), edges }
  }, [workflowJson])

  const onNodeClick = useCallback((_: React.MouseEvent, node: Node) => {
    setSelectedNode({ id: node.id, data: node.data as unknown as WFNodeData })
  }, [])

  return (
    <div className="flex min-h-[460px] gap-3 overflow-hidden rounded-2xl border border-border/55 bg-white/38">
      <div className="min-w-0 flex-1 transition-all">
        <ReactFlow
          nodes={nodes}
          edges={edges}
          nodeTypes={nodeTypes}
          edgeTypes={edgeTypes}
          nodesDraggable={false}
          nodesConnectable={false}
          elementsSelectable={true}
          onNodeClick={onNodeClick}
          onPaneClick={() => setSelectedNode(null)}
          fitView
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
              return getNodeAccent(nodeData?.nodeType)
            }}
            maskColor="rgba(15,23,42,0.06)"
          />
        </ReactFlow>
      </div>

      {selectedNode && (
        <aside className="w-[340px] shrink-0 border-l border-border/50 bg-white/68">
          <div className="flex min-h-14 items-center justify-between border-b border-border/50 px-4">
            <div className="flex min-w-0 items-center gap-2">
              <span className="flex size-7 shrink-0 items-center justify-center rounded-lg text-white" style={{ backgroundColor: getNodeAccent(selectedNode.data.nodeType) }}>
                <WorkflowNodeIconGlyph nodeType={selectedNode.data.nodeType} />
              </span>
              <h4 className="truncate text-sm font-semibold">{t("workflow.viewer.activityDetail")}</h4>
            </div>
            <Button variant="ghost" size="icon" className="h-6 w-6" onClick={() => setSelectedNode(null)}>
              <X className="h-3.5 w-3.5" />
            </Button>
          </div>

          <div className="space-y-3 p-4 text-sm">
            <div className="flex items-center gap-2">
              <span className="text-muted-foreground">{t("workflow.prop.label")}:</span>
              <span className="font-medium">{selectedNode.data.label}</span>
            </div>

            <div className="flex items-center gap-2">
              <span className="text-muted-foreground">{t("generate.nodeType")}:</span>
              <Badge
                variant="outline"
                style={{ borderColor: getNodeAccent(selectedNode.data.nodeType), color: getNodeAccent(selectedNode.data.nodeType) }}
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

            {selectedNode.data.action_id && (
              <div className="flex items-center gap-2">
                <span className="text-muted-foreground">Action ID:</span>
                <span>{selectedNode.data.action_id}</span>
              </div>
            )}

            {selectedNode.data.wait_mode && (
              <div className="flex items-center gap-2">
                <span className="text-muted-foreground">{t("workflow.prop.waitMode")}:</span>
                <span>{t(`workflow.prop.wait${selectedNode.data.wait_mode.charAt(0).toUpperCase()}${selectedNode.data.wait_mode.slice(1)}` as const)}</span>
                {selectedNode.data.duration && <span className="text-muted-foreground">({selectedNode.data.duration})</span>}
              </div>
            )}

            {selectedNode.data.formSchema != null && (() => {
              const fields = parsePreviewFields(selectedNode.data.formSchema)
              return fields.length > 0 ? (
                <div>
                  <span className="text-muted-foreground">{t("workflow.prop.formFields")} ({fields.length}):</span>
                  <div className="mt-1 rounded border p-1.5 space-y-0.5">
                    {fields.map((f) => (
                      <div key={f.key} className="flex items-center justify-between text-xs">
                        <span>{f.label || f.key}</span>
                        <span className="text-muted-foreground">{f.type}{f.options ? ` (${f.options.length})` : ""}</span>
                      </div>
                    ))}
                  </div>
                </div>
              ) : (
                <div>
                  <span className="text-muted-foreground">{t("services.formSchema")}:</span>
                  <pre className="mt-1 max-h-40 overflow-auto rounded bg-muted p-2 text-xs">
                    {JSON.stringify(selectedNode.data.formSchema, null, 2)}
                  </pre>
                </div>
              )
            })()}
          </div>
        </aside>
      )}
    </div>
  )
}
