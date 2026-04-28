import { memo } from "react"
import {
  BaseEdge,
  getBezierPath,
  type EdgeProps,
  EdgeLabelRenderer,
  MarkerType,
  Position,
  useReactFlow,
  type Edge,
  type Node,
} from "@xyflow/react"
import { Plus } from "lucide-react"
import { useTranslation } from "react-i18next"
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu"
import { Button } from "@/components/ui/button"
import { cn } from "@/lib/utils"
import type { NodeType, WFEdgeData } from "./types"
import {
  buildEdgeLabelDisplay,
  buildConditionDisplayDictFromNodes,
  createEmptyConditionDisplayDict,
} from "./types"
import { WORKFLOW_NODE_GROUPS, getNodeAccent } from "./visual-data"
import { WorkflowNodeIconGlyph } from "./visual"
import { defaultNodeData } from "./workflow-contract"

let insertedNodeId = 0
const EDGE_HANDLE_GAP = 8

function getInsertedNodeId() {
  insertedNodeId += 1
  return `node_edge_${insertedNodeId}`
}

function getInsertedEdgeId(suffix: string) {
  insertedNodeId += 1
  return `edge_insert_${insertedNodeId}_${suffix}`
}

function WorkflowEdgeInner({
  id,
  sourceX,
  sourceY,
  targetX,
  targetY,
  data,
  selected,
  markerEnd,
  source,
  target,
  sourceHandleId,
  targetHandleId,
  sourcePosition,
  targetPosition,
}: EdgeProps & { data?: WFEdgeData }) {
  const { t } = useTranslation("itsm")
  const { setNodes, setEdges, getNodes } = useReactFlow()
  const resolvedSourcePosition = sourcePosition ?? Position.Right
  const resolvedTargetPosition = targetPosition ?? Position.Left
  const sourcePoint = offsetEdgePoint(sourceX, sourceY, resolvedSourcePosition, EDGE_HANDLE_GAP)
  const targetPoint = offsetEdgePoint(targetX, targetY, resolvedTargetPosition, EDGE_HANDLE_GAP)
  const [edgePath, labelX, labelY] = getBezierPath({
    sourceX: sourcePoint.x,
    sourceY: sourcePoint.y,
    sourcePosition: resolvedSourcePosition,
    targetX: targetPoint.x,
    targetY: targetPoint.y,
    targetPosition: resolvedTargetPosition,
    curvature: 0.16,
  })

  const edgeData = data as WFEdgeData | undefined
  const displayDict = buildConditionDisplayDictFromNodes(getNodes()) || createEmptyConditionDisplayDict()
  const labelDisplay = buildEdgeLabelDisplay(edgeData, displayDict, {
    operatorLabels: {
      equals: t("workflow.edge.operator.equals"),
      not_equals: t("workflow.edge.operator.not_equals"),
      contains_any: t("workflow.edge.operator.contains_any"),
      gt: t("workflow.edge.operator.gt"),
      lt: t("workflow.edge.operator.lt"),
      gte: t("workflow.edge.operator.gte"),
      lte: t("workflow.edge.operator.lte"),
      is_empty: t("workflow.edge.operator.is_empty"),
      is_not_empty: t("workflow.edge.operator.is_not_empty"),
    },
    logicLabels: {
      and: t("workflow.edge.logic.and"),
      or: t("workflow.edge.logic.or"),
    },
    valueSeparator: t("workflow.edge.valueSeparator"),
  }, {
    defaultLabel: t("workflow.edge.defaultPath"),
    outcomeLabels: {
      submitted: t("workflow.edge.outcome.submitted"),
      completed: t("workflow.edge.outcome.completed"),
      approved: t("workflow.edge.outcome.approved"),
      rejected: t("workflow.edge.outcome.rejected"),
      success: t("workflow.edge.outcome.success"),
      failed: t("workflow.edge.outcome.failed"),
      timeout: t("workflow.edge.outcome.timeout"),
      cancelled: t("workflow.edge.outcome.cancelled"),
    },
  })
  const isCompleted = labelDisplay.isCompleted
  const primaryLabel = labelDisplay.primary
  const rawLabel = labelDisplay.raw
  const hasLabel = !!primaryLabel || !!rawLabel

  const strokeColor = selected
    ? "var(--color-primary)"
    : edgeData?.failed
      ? "#dc2626"
      : edgeData?.visited || isCompleted
        ? "#059669"
        : "color-mix(in oklab, var(--color-muted-foreground) 32%, transparent)"

  function handleInsert(nodeType: NodeType) {
    const newNodeId = getInsertedNodeId()
    const newEdgeIdA = getInsertedEdgeId("a")
    const newEdgeIdB = getInsertedEdgeId("b")
    const position = { x: labelX - 120, y: labelY - 48 }

    const newNode: Node<Record<string, unknown>> = {
      id: newNodeId,
      type: nodeType,
      position,
      data: defaultNodeData(nodeType, t(`workflow.node.${nodeType}`)) as unknown as Record<string, unknown>,
      selected: true,
    }

    setNodes((nodes) => [...nodes.map((node) => ({ ...node, selected: false })), newNode])
    setEdges((edges) => {
      const current = edges.find((edge) => edge.id === id)
      const rest = edges.filter((edge) => edge.id !== id)
      const baseData = (current?.data ?? {}) as WFEdgeData
      const firstEdge: Edge<Record<string, unknown>> = {
        id: newEdgeIdA,
        source,
        sourceHandle: sourceHandleId,
        target: newNodeId,
        type: "workflow",
        markerEnd: { type: MarkerType.ArrowClosed },
        data: { ...baseData } as Record<string, unknown>,
      }
      const secondEdge: Edge<Record<string, unknown>> = {
        id: newEdgeIdB,
        source: newNodeId,
        target,
        targetHandle: targetHandleId,
        type: "workflow",
        markerEnd: { type: MarkerType.ArrowClosed },
        data: { outcome: "", default: false } satisfies WFEdgeData as Record<string, unknown>,
      }
      return [...rest, firstEdge, secondEdge]
    })
  }

  return (
    <>
      <BaseEdge
        id={id}
        path={edgePath}
        markerEnd={markerEnd}
        style={{
          stroke: strokeColor,
          strokeWidth: selected ? 2.2 : edgeData?.visited ? 2 : 1.6,
          strokeDasharray: edgeData?.readonly && !edgeData?.visited ? "0" : undefined,
        }}
      />
      <EdgeLabelRenderer>
        <div
          className="nodrag nopan group absolute flex -translate-x-1/2 -translate-y-1/2 items-center gap-1"
          style={{
            transform: `translate(-50%, -50%) translate(${labelX}px, ${labelY}px)`,
          }}
        >
          {hasLabel && (
            <div className="pointer-events-none max-w-[180px]">
              <div
                className={cn(
                  "truncate rounded-full border bg-white/90 px-2 py-0.5 text-[10px] font-medium shadow-[0_8px_22px_-18px_rgba(15,23,42,0.42)]",
                  isCompleted && "border-emerald-500/20 text-emerald-700",
                  labelDisplay.kind === "condition" && "border-amber-500/20 text-amber-700",
                  labelDisplay.kind === "default" && "border-border/70 text-muted-foreground",
                )}
                title={rawLabel}
              >
                {primaryLabel}
              </div>
            </div>
          )}
          {!edgeData?.readonly && (
            <DropdownMenu>
              <DropdownMenuTrigger asChild>
                <Button
                  type="button"
                  variant="outline"
                  size="icon"
                  className="size-6 rounded-full border-border/70 bg-white/90 p-0 opacity-70 shadow-[0_12px_28px_-22px_rgba(15,23,42,0.6)] transition hover:opacity-100 data-[state=open]:opacity-100"
                  aria-label={t("workflow.insertNode")}
                >
                  <Plus className="size-3.5" />
                </Button>
              </DropdownMenuTrigger>
              <DropdownMenuContent side="bottom" align="center" className="w-56">
                {WORKFLOW_NODE_GROUPS.flatMap((group) => group.types).map((nodeType) => {
                  return (
                    <DropdownMenuItem key={nodeType} className="gap-2" onClick={() => handleInsert(nodeType)}>
                      <span
                        className="flex size-6 shrink-0 items-center justify-center rounded-md text-white"
                        style={{ backgroundColor: getNodeAccent(nodeType) }}
                      >
                        <WorkflowNodeIconGlyph nodeType={nodeType} className="size-3" />
                      </span>
                      <span>{t(`workflow.node.${nodeType}`)}</span>
                    </DropdownMenuItem>
                  )
                })}
              </DropdownMenuContent>
            </DropdownMenu>
          )}
          </div>
      </EdgeLabelRenderer>
    </>
  )
}

export const WorkflowEdge = memo(WorkflowEdgeInner)

export const edgeTypes = {
  workflow: WorkflowEdge,
}

function offsetEdgePoint(x: number, y: number, position: Position, gap: number) {
  if (position === Position.Left) return { x: x - gap, y }
  if (position === Position.Right) return { x: x + gap, y }
  if (position === Position.Top) return { x, y: y - gap }
  return { x, y: y + gap }
}
