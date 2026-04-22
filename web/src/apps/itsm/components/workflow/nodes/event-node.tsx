import { memo } from "react"
import { Handle, Position, type NodeProps } from "@xyflow/react"
import type { WFNodeData } from "../types"
import { WorkflowNodeCard } from "../visual"

function EventNodeInner({ data: rawData, selected }: NodeProps) {
  const data = rawData as unknown as WFNodeData
  const nodeType = data.nodeType

  return (
    <div className="relative">
      {nodeType !== "start" && <Handle type="target" position={Position.Left} />}
      <WorkflowNodeCard data={data} selected={selected} className="min-h-[76px]" />
      {nodeType !== "end" && <Handle type="source" position={Position.Right} />}
    </div>
  )
}

export const EventNode = memo(EventNodeInner)
