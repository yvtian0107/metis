import { memo } from "react"
import { Handle, Position, type NodeProps } from "@xyflow/react"
import type { WFNodeData } from "../types"
import { WorkflowNodeCard } from "../visual"

function TaskNodeInner({ data: rawData, selected }: NodeProps) {
  const data = rawData as unknown as WFNodeData

  return (
    <div className="relative">
      <Handle type="target" position={Position.Left} />
      <WorkflowNodeCard data={data} selected={selected} />
      <Handle type="source" position={Position.Right} />
    </div>
  )
}

export const TaskNode = memo(TaskNodeInner)
