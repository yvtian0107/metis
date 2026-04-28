import { Position } from "@xyflow/react"
import type { WFNodeData } from "../types"

export function workflowHandlePositions(data: WFNodeData) {
  return data._layoutDirection === "TB"
    ? { target: Position.Top, source: Position.Bottom }
    : { target: Position.Left, source: Position.Right }
}
