"use client"

import { useState, useMemo, useCallback, useEffect } from "react"
import { useTranslation } from "react-i18next"
import {
  ReactFlow, Background, Controls, MiniMap,
  Panel, type Node, type Edge, MarkerType, type ReactFlowInstance, useNodesState, useEdgesState,
} from "@xyflow/react"
import "@xyflow/react/dist/style.css"
import { LayoutGrid, Maximize2, X } from "lucide-react"
import { Button } from "@/components/ui/button"
import { Badge } from "@/components/ui/badge"
import { FormRenderer, type FormSchema, type FormField, type FieldOption } from "../../../components/form-engine"
import { nodeTypes } from "../../../components/workflow/nodes"
import { edgeTypes } from "../../../components/workflow/custom-edges"
import { applyViewerLayout } from "../../../components/workflow/auto-layout"
import { getNodeAccent } from "../../../components/workflow/visual-data"
import { WorkflowNodeIconGlyph } from "../../../components/workflow/visual"
import { type WFNodeData, type NodeType, type Participant } from "../../../components/workflow/types"
import "../../../components/workflow/style.css"

interface WorkflowPreviewProps {
  workflowJson: unknown
  embedded?: boolean
  focusTarget?: {
    kind: "workflow_node" | "workflow_edge"
    refId: string
    seq: number
  } | null
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

/** Parse and normalize form schema for readonly preview */
function parsePreviewSchema(schema: unknown): FormSchema | null {
  if (!schema || typeof schema !== "object") return null
  const rawSchema = schema as { version?: number; fields?: unknown[] }
  if (!Array.isArray(rawSchema.fields) || rawSchema.fields.length === 0) return null

  const fields: FormField[] = rawSchema.fields.flatMap((rawField) => {
    if (!rawField || typeof rawField !== "object") return []
    const fieldObj = rawField as {
      key?: unknown
      type?: unknown
      label?: unknown
      placeholder?: unknown
      description?: unknown
      required?: unknown
      options?: unknown
    }
    const key = typeof fieldObj.key === "string" ? fieldObj.key : ""
    if (key === "") return []

    const type = typeof fieldObj.type === "string" ? fieldObj.type : "text"
    const label = typeof fieldObj.label === "string" && fieldObj.label.trim().length > 0 ? fieldObj.label : key
    const placeholder = typeof fieldObj.placeholder === "string" ? fieldObj.placeholder : undefined
    const description = typeof fieldObj.description === "string" ? fieldObj.description : undefined
    const required = typeof fieldObj.required === "boolean" ? fieldObj.required : false

    const options: FieldOption[] | undefined = Array.isArray(fieldObj.options)
      ? fieldObj.options.flatMap((opt) => {
        if (typeof opt === "string" || typeof opt === "number" || typeof opt === "boolean") {
          return [{ label: String(opt), value: opt }]
        }
        if (opt && typeof opt === "object") {
          const optionObj = opt as { label?: unknown; value?: unknown }
          const value = optionObj.value
          if (typeof value === "string" || typeof value === "number" || typeof value === "boolean") {
            const optionLabel = typeof optionObj.label === "string" && optionObj.label.trim().length > 0
              ? optionObj.label
              : String(value)
            return [{ label: optionLabel, value }]
          }
        }
        return []
      })
      : undefined

    return [{
      key,
      type: type as FormField["type"],
      label,
      placeholder,
      description,
      required,
      options,
    }]
  })

  if (fields.length === 0) return null
  return {
    version: typeof rawSchema.version === "number" ? rawSchema.version : 1,
    fields,
  }
}

export default function WorkflowPreview({
  workflowJson,
  embedded = false,
  focusTarget,
}: WorkflowPreviewProps) {
  const { t } = useTranslation("itsm")
  const [manualSelectedNode, setManualSelectedNode] = useState<{ id: string; data: WFNodeData } | null>(null)
  const [flowInstance, setFlowInstance] = useState<ReactFlowInstance | null>(null)

  const { initialNodes, initialEdges } = useMemo<{ initialNodes: Node[]; initialEdges: Edge[] }>(() => {
    if (!workflowJson) return { initialNodes: [], initialEdges: [] }

    let wf: { nodes?: unknown[]; edges?: unknown[] }
    try {
      wf = typeof workflowJson === "string" ? JSON.parse(workflowJson) : workflowJson
    } catch {
      return { initialNodes: [], initialEdges: [] }
    }

    const rawNodes = (wf.nodes ?? []) as Array<{
      id: string; position: { x: number; y: number }; data?: Record<string, unknown>; type?: string
    }>
    const rawEdges = (wf.edges ?? []) as Array<{
      id: string; source: string; target: string; data?: Record<string, unknown>
    }>

    const focusedNodeID = focusTarget?.kind === "workflow_node" ? focusTarget.refId : ""
    const focusedEdgeID = focusTarget?.kind === "workflow_edge" ? focusTarget.refId : ""

    const nodes = rawNodes.map((n) => {
      const rawData = (n.data ?? {}) as Record<string, unknown>
      const nodeType = (rawData.nodeType ?? n.type ?? "process") as NodeType
      return {
        id: n.id,
        type: nodeType,
        position: n.position,
        data: { ...rawData, nodeType } as unknown as WFNodeData,
        selected: n.id === focusedNodeID,
        selectable: true as const,
      }
    }) as unknown as Node[]

    const edges: Edge[] = rawEdges.map((e) => ({
      id: e.id,
      source: e.source,
      target: e.target,
      type: "workflow",
      selected: e.id === focusedEdgeID,
      markerEnd: { type: MarkerType.ArrowClosed },
      data: { ...e.data, readonly: true } as Record<string, unknown>,
    }))

    return {
      initialNodes: applyViewerLayout(nodes, edges),
      initialEdges: edges,
    }
  }, [workflowJson, focusTarget])

  const [nodes, setNodes, onNodesChange] = useNodesState(initialNodes)
  const [edges, setEdges, onEdgesChange] = useEdgesState(initialEdges)

  useEffect(() => {
    setNodes(initialNodes)
    setEdges(initialEdges)
  }, [initialNodes, initialEdges, setNodes, setEdges])

  const selectedNode = useMemo(() => {
    if (focusTarget?.kind === "workflow_edge") {
      return null
    }
    if (manualSelectedNode) {
      return manualSelectedNode
    }
    if (focusTarget?.kind !== "workflow_node") {
      return null
    }
    const focusedNode = nodes.find((node) => node.id === focusTarget.refId)
    if (!focusedNode) {
      return null
    }
    return { id: focusedNode.id, data: focusedNode.data as unknown as WFNodeData }
  }, [focusTarget, manualSelectedNode, nodes])

  useEffect(() => {
    if (!focusTarget || !flowInstance) {
      return
    }

    if (focusTarget.kind === "workflow_node") {
      const focusedNode = nodes.find((node) => node.id === focusTarget.refId)
      if (!focusedNode) {
        return
      }
      void flowInstance.fitView({ nodes: [{ id: focusedNode.id }], duration: 260, padding: 0.35, maxZoom: 1.2 })
      return
    }

    const focusedEdge = edges.find((edge) => edge.id === focusTarget.refId)
    if (!focusedEdge) {
      return
    }
    void flowInstance.fitView({
      nodes: [{ id: focusedEdge.source }, { id: focusedEdge.target }],
      duration: 260,
      padding: 0.4,
      maxZoom: 1.1,
    })
  }, [focusTarget, flowInstance, nodes, edges])

  const onNodeClick = useCallback((_: React.MouseEvent, node: Node) => {
    setManualSelectedNode({ id: node.id, data: node.data as unknown as WFNodeData })
  }, [])

  const fitWorkflowView = useCallback(() => {
    void flowInstance?.fitView({ duration: 240, padding: 0.18, maxZoom: 1.05 })
  }, [flowInstance])

  const handleAutoLayout = useCallback(() => {
    setNodes(applyViewerLayout(nodes, edges) as typeof nodes)
    window.requestAnimationFrame(fitWorkflowView)
  }, [edges, fitWorkflowView, nodes, setNodes])

  return (
    <div
      className={embedded
        ? "workspace-surface flex min-h-[460px] gap-3 overflow-hidden rounded-[1.1rem]"
        : "flex min-h-[460px] gap-3 overflow-hidden rounded-2xl border border-border/55 bg-white/38"}
    >
      <div className="min-w-0 flex-1 transition-all">
        <ReactFlow
          nodes={nodes}
          edges={edges}
          onNodesChange={onNodesChange}
          onEdgesChange={onEdgesChange}
          nodeTypes={nodeTypes}
          edgeTypes={edgeTypes}
          nodesDraggable
          nodesConnectable={false}
          elementsSelectable={true}
          onNodeClick={onNodeClick}
          onPaneClick={() => setManualSelectedNode(null)}
          onInit={setFlowInstance}
          fitView
          fitViewOptions={{ padding: 0.12 }}
          className="workflow-builder-flow"
        >
          <Background gap={24} size={1.2} />
          <Controls showInteractive={false} position="bottom-left" />
          <Panel position="top-right" className="flex gap-1 rounded-xl border border-border/60 bg-white/75 p-1">
            <Button
              type="button"
              variant="ghost"
              size="icon"
              className="h-8 w-8"
              onClick={handleAutoLayout}
              title={t("workflow.autoLayout")}
              aria-label={t("workflow.autoLayout")}
            >
              <LayoutGrid className="h-3.5 w-3.5" />
            </Button>
            <Button
              type="button"
              variant="ghost"
              size="icon"
              className="h-8 w-8"
              onClick={fitWorkflowView}
              title={t("workflow.fitView")}
              aria-label={t("workflow.fitView")}
            >
              <Maximize2 className="h-3.5 w-3.5" />
            </Button>
          </Panel>
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
            <Button variant="ghost" size="icon" className="h-6 w-6" onClick={() => setManualSelectedNode(null)}>
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
                <span className="text-muted-foreground">绑定动作 ID:</span>
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
              const previewSchema = parsePreviewSchema(selectedNode.data.formSchema)
              return previewSchema != null ? (
                <div>
                  <span className="text-muted-foreground">{t("workflow.prop.formFields")} ({previewSchema.fields.length}):</span>
                  <div className="mt-2 max-h-[320px] overflow-auto rounded-xl border border-border/70 bg-muted/20 p-3">
                    <FormRenderer
                      schema={previewSchema}
                      mode="create"
                    />
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
