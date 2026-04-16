import { useState, useCallback, useRef, useEffect } from "react"
import { useTranslation } from "react-i18next"
import {
  ReactFlow,
  ReactFlowProvider,
  Background,
  Controls,
  MiniMap,
  addEdge,
  useNodesState,
  useEdgesState,
  type Connection,
  type Node,
  type Edge,
  MarkerType,
  Panel,
  useReactFlow,
} from "@xyflow/react"
import "@xyflow/react/dist/style.css"
import { Button } from "@/components/ui/button"
import {
  ContextMenu,
  ContextMenuContent,
  ContextMenuItem,
  ContextMenuSeparator,
  ContextMenuTrigger,
} from "@/components/ui/context-menu"
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip"
import { Save, Undo2, Redo2, LayoutGrid, Copy, Trash2, MousePointer, ClipboardPaste } from "lucide-react"
import { toast } from "sonner"
import { nodeTypes } from "./nodes"
import { edgeTypes } from "./custom-edges"
import { NodePalette } from "./node-palette"
import { NodePropertyPanel, EdgePropertyPanel } from "./property-panel"
import { type WFNodeData, type WFEdgeData, type NodeType, NODE_COLORS } from "./types"
import { applyDagreLayout } from "./auto-layout"
import { useUndoRedo } from "./use-undo-redo"

interface WorkflowEditorProps {
  initialData?: { nodes: Node[]; edges: Edge[] }
  onSave: (data: { nodes: Node[]; edges: Edge[] }) => void
  saving?: boolean
  serviceId?: number
  validationErrors?: Array<{ nodeId?: string; edgeId?: string; message: string }>
}

let nodeId = 0
function getNodeId() { return `node_${Date.now()}_${++nodeId}` }

function InnerEditor({ initialData, onSave, saving, serviceId, validationErrors }: WorkflowEditorProps) {
  const { t } = useTranslation("itsm")
  const reactFlowWrapper = useRef<HTMLDivElement>(null)
  const rfInstance = useReactFlow()
  const [selectedNode, setSelectedNode] = useState<(Node & { data: WFNodeData }) | null>(null)
  const [selectedEdge, setSelectedEdge] = useState<(Edge & { data?: WFEdgeData }) | null>(null)

  // Migrate legacy "workflow" type to specific nodeType
  const migratedNodes = (initialData?.nodes ?? []).map((n) => ({
    ...n,
    type: (n.data as unknown as WFNodeData).nodeType ?? n.type,
  }))
  const migratedEdges = (initialData?.edges ?? []).map((e) => ({
    ...e,
    type: "workflow",
  }))

  const [nodes, setNodes, onNodesChange] = useNodesState(migratedNodes)
  const [edges, setEdges, onEdgesChange] = useEdgesState(migratedEdges)

  const { undo, redo, push, canUndo, canRedo } = useUndoRedo()
  const clipboardRef = useRef<{ nodes: Node[]; edges: Edge[] } | null>(null)

  // Track validation errors on nodes
  const errorsByNode = new Map<string, string>()
  const errorsByEdge = new Map<string, string>()
  for (const err of validationErrors ?? []) {
    if (err.nodeId) errorsByNode.set(err.nodeId, err.message)
    if (err.edgeId) errorsByEdge.set(err.edgeId, err.message)
  }

  // Push undo on node/edge changes
  const prevNodesRef = useRef(nodes)
  const prevEdgesRef = useRef(edges)
  useEffect(() => {
    const nodesChanged = nodes !== prevNodesRef.current
    const edgesChanged = edges !== prevEdgesRef.current
    if (nodesChanged || edgesChanged) {
      push({ nodes: prevNodesRef.current, edges: prevEdgesRef.current })
      prevNodesRef.current = nodes
      prevEdgesRef.current = edges
    }
  }, [nodes, edges, push])

  const onConnect = useCallback((params: Connection) => {
    setEdges((eds) => addEdge({
      ...params,
      type: "workflow",
      markerEnd: { type: MarkerType.ArrowClosed },
      data: { outcome: "", isDefault: false } as Record<string, unknown>,
    }, eds))
  }, [setEdges])

  const onDragOver = useCallback((event: React.DragEvent) => {
    event.preventDefault()
    event.dataTransfer.dropEffect = "move"
  }, [])

  const onDrop = useCallback((event: React.DragEvent) => {
    event.preventDefault()
    const nodeType = event.dataTransfer.getData("application/reactflow-nodetype") as NodeType
    if (!nodeType || !reactFlowWrapper.current) return

    const bounds = reactFlowWrapper.current.getBoundingClientRect()
    const position = rfInstance.screenToFlowPosition({
      x: event.clientX - bounds.left,
      y: event.clientY - bounds.top,
    })

    const newNode: Node = {
      id: getNodeId(),
      type: nodeType,
      position,
      data: {
        label: t(`workflow.node.${nodeType}`),
        nodeType,
        ...(nodeType === "approve" ? { executionMode: "single" } : {}),
        ...(nodeType === "wait" || nodeType === "timer" ? { waitMode: nodeType === "timer" ? "timer" : "signal" } : {}),
      } satisfies WFNodeData,
    }
    setNodes((nds) => [...nds, newNode] as typeof nds)
  }, [rfInstance, setNodes, t])

  const onNodeClick = useCallback((_: React.MouseEvent, node: Node) => {
    setSelectedEdge(null)
    setSelectedNode(node as Node & { data: WFNodeData })
  }, [])

  const onEdgeClick = useCallback((_: React.MouseEvent, edge: Edge) => {
    setSelectedNode(null)
    setSelectedEdge(edge as Edge & { data?: WFEdgeData })
  }, [])

  const onPaneClick = useCallback(() => {
    setSelectedNode(null)
    setSelectedEdge(null)
  }, [])

  function handleSave() {
    onSave({ nodes, edges })
  }

  function handleAutoLayout() {
    const layouted = applyDagreLayout(nodes, edges)
    setNodes(layouted as typeof nodes)
  }

  function handleUndo() {
    const state = undo()
    if (state) {
      setNodes(state.nodes as typeof nodes)
      setEdges(state.edges as typeof edges)
    }
  }

  function handleRedo() {
    const state = redo()
    if (state) {
      setNodes(state.nodes as typeof nodes)
      setEdges(state.edges as typeof edges)
    }
  }

  function handleCopy() {
    const selected = nodes.filter((n) => n.selected && (n.data as unknown as WFNodeData).nodeType !== "start" && (n.data as unknown as WFNodeData).nodeType !== "end")
    if (selected.length === 0) return
    const selectedIds = new Set(selected.map((n) => n.id))
    const selectedEdges = edges.filter((e) => selectedIds.has(e.source) && selectedIds.has(e.target))
    clipboardRef.current = { nodes: selected, edges: selectedEdges }
    toast.success(t("workflow.copied", { count: selected.length }))
  }

  function handlePaste() {
    const clip = clipboardRef.current
    if (!clip || clip.nodes.length === 0) return
    const idMap = new Map<string, string>()
    const newNodes = clip.nodes.map((n) => {
      const newId = getNodeId()
      idMap.set(n.id, newId)
      return { ...n, id: newId, position: { x: n.position.x + 40, y: n.position.y + 40 }, selected: false }
    })
    const newEdges = clip.edges.map((e) => ({
      ...e,
      id: `edge_${Date.now()}_${Math.random().toString(36).slice(2, 6)}`,
      source: idMap.get(e.source) ?? e.source,
      target: idMap.get(e.target) ?? e.target,
      selected: false,
    }))
    setNodes((nds) => [...nds, ...newNodes] as typeof nds)
    setEdges((eds) => [...eds, ...newEdges] as typeof eds)
  }

  function handleDeleteSelected() {
    const nodesToDelete = nodes.filter((n) => n.selected && (n.data as unknown as WFNodeData).nodeType !== "start" && (n.data as unknown as WFNodeData).nodeType !== "end")
    const edgesToDelete = edges.filter((e) => e.selected)
    if (nodesToDelete.length > 0 || edgesToDelete.length > 0) {
      rfInstance.deleteElements({ nodes: nodesToDelete.map((n) => ({ id: n.id })), edges: edgesToDelete.map((e) => ({ id: e.id })) })
    }
  }

  function handleSelectAll() {
    setNodes((nds) => nds.map((n) => ({ ...n, selected: true })))
    setEdges((eds) => eds.map((e) => ({ ...e, selected: true })))
  }

  // Keyboard shortcuts
  useEffect(() => {
    function onKeyDown(e: KeyboardEvent) {
      const target = e.target as HTMLElement
      if (target.tagName === "INPUT" || target.tagName === "TEXTAREA" || target.tagName === "SELECT") return

      if ((e.ctrlKey || e.metaKey) && e.key === "z" && !e.shiftKey) {
        e.preventDefault()
        handleUndo()
      } else if ((e.ctrlKey || e.metaKey) && e.key === "z" && e.shiftKey) {
        e.preventDefault()
        handleRedo()
      } else if ((e.ctrlKey || e.metaKey) && e.key === "c") {
        e.preventDefault()
        handleCopy()
      } else if ((e.ctrlKey || e.metaKey) && e.key === "v") {
        e.preventDefault()
        handlePaste()
      } else if ((e.ctrlKey || e.metaKey) && e.key === "a") {
        e.preventDefault()
        handleSelectAll()
      } else if (e.key === "Delete" || e.key === "Backspace") {
        handleDeleteSelected()
      } else if (["ArrowUp", "ArrowDown", "ArrowLeft", "ArrowRight"].includes(e.key)) {
        const delta = e.shiftKey ? 1 : 10
        const dx = e.key === "ArrowLeft" ? -delta : e.key === "ArrowRight" ? delta : 0
        const dy = e.key === "ArrowUp" ? -delta : e.key === "ArrowDown" ? delta : 0
        if (dx !== 0 || dy !== 0) {
          e.preventDefault()
          setNodes((nds) => nds.map((n) => n.selected ? { ...n, position: { x: n.position.x + dx, y: n.position.y + dy } } : n))
        }
      }
    }
    window.addEventListener("keydown", onKeyDown)
    return () => window.removeEventListener("keydown", onKeyDown)
  })

  // Decorate nodes with validation error classes
  const decoratedNodes = nodes.map((n) => {
    const err = errorsByNode.get(n.id)
    if (!err) return n
    return { ...n, className: "!border-destructive ring-1 ring-destructive/50" }
  })

  const edgeSourceNodeType = selectedEdge
    ? (nodes.find((n) => n.id === selectedEdge.source)?.data as unknown as WFNodeData | undefined)?.nodeType
    : undefined

  return (
    <div className="flex h-full" ref={reactFlowWrapper}>
      <NodePalette />
      <div className="flex-1">
        <ContextMenu>
          <ContextMenuTrigger asChild>
            <div className="h-full">
              <ReactFlow
                nodes={decoratedNodes}
                edges={edges}
                onNodesChange={onNodesChange}
                onEdgesChange={onEdgesChange}
                onConnect={onConnect}
                onDrop={onDrop}
                onDragOver={onDragOver}
                onNodeClick={onNodeClick}
                onEdgeClick={onEdgeClick}
                onPaneClick={onPaneClick}
                nodeTypes={nodeTypes as any}
                edgeTypes={edgeTypes}
                defaultEdgeOptions={{
                  type: "workflow",
                  markerEnd: { type: MarkerType.ArrowClosed },
                }}
                fitView
                selectNodesOnDrag
                multiSelectionKeyCode="Shift"
                className="bg-background"
              >
                <Background />
                <Controls />
                <MiniMap
                  nodeColor={(n) => NODE_COLORS[(n.data as unknown as WFNodeData)?.nodeType] ?? "#6b7280"}
                  maskColor="rgba(0,0,0,0.1)"
                />
                <Panel position="top-right" className="flex gap-1">
                  <Tooltip>
                    <TooltipTrigger asChild>
                      <Button variant="outline" size="icon" className="h-8 w-8" onClick={handleUndo} disabled={!canUndo}>
                        <Undo2 size={14} />
                      </Button>
                    </TooltipTrigger>
                    <TooltipContent>Ctrl+Z</TooltipContent>
                  </Tooltip>
                  <Tooltip>
                    <TooltipTrigger asChild>
                      <Button variant="outline" size="icon" className="h-8 w-8" onClick={handleRedo} disabled={!canRedo}>
                        <Redo2 size={14} />
                      </Button>
                    </TooltipTrigger>
                    <TooltipContent>Ctrl+Shift+Z</TooltipContent>
                  </Tooltip>
                  <Tooltip>
                    <TooltipTrigger asChild>
                      <Button variant="outline" size="icon" className="h-8 w-8" onClick={handleAutoLayout}>
                        <LayoutGrid size={14} />
                      </Button>
                    </TooltipTrigger>
                    <TooltipContent>{t("workflow.autoLayout")}</TooltipContent>
                  </Tooltip>
                  <Button size="sm" onClick={handleSave} disabled={saving}>
                    <Save className="mr-1.5 h-3.5 w-3.5" />
                    {saving ? t("workflow.saving") : t("workflow.save")}
                  </Button>
                </Panel>
              </ReactFlow>
            </div>
          </ContextMenuTrigger>
          <ContextMenuContent>
            <ContextMenuItem onClick={handlePaste}>
              <ClipboardPaste className="mr-2 h-4 w-4" />{t("workflow.ctx.paste")}
            </ContextMenuItem>
            <ContextMenuItem onClick={handleAutoLayout}>
              <LayoutGrid className="mr-2 h-4 w-4" />{t("workflow.autoLayout")}
            </ContextMenuItem>
            <ContextMenuItem onClick={handleSelectAll}>
              <MousePointer className="mr-2 h-4 w-4" />{t("workflow.ctx.selectAll")}
            </ContextMenuItem>
            <ContextMenuSeparator />
            <ContextMenuItem onClick={handleCopy} disabled={!nodes.some((n) => n.selected)}>
              <Copy className="mr-2 h-4 w-4" />{t("workflow.ctx.copy")}
            </ContextMenuItem>
            <ContextMenuItem onClick={handleDeleteSelected} disabled={!nodes.some((n) => n.selected)} className="text-destructive">
              <Trash2 className="mr-2 h-4 w-4" />{t("workflow.ctx.delete")}
            </ContextMenuItem>
          </ContextMenuContent>
        </ContextMenu>
      </div>
      {selectedNode && (
        <NodePropertyPanel node={selectedNode} serviceId={serviceId} onClose={() => setSelectedNode(null)} />
      )}
      {selectedEdge && (
        <EdgePropertyPanel edge={selectedEdge} sourceNodeType={edgeSourceNodeType} onClose={() => setSelectedEdge(null)} />
      )}
    </div>
  )
}

export function WorkflowEditor(props: WorkflowEditorProps) {
  return (
    <ReactFlowProvider>
      <InnerEditor {...props} />
    </ReactFlowProvider>
  )
}
