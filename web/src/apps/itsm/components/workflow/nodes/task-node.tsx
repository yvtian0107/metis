import { memo } from "react"
import { Handle, type NodeProps } from "@xyflow/react"
import type { WFNodeData } from "../types"
import { WorkflowNodeCard } from "../visual"
import { workflowHandlePositions } from "./layout-handles"

function TaskNodeInner({ data: rawData, selected }: NodeProps) {
  const data = rawData as unknown as WFNodeData
  const positions = workflowHandlePositions(data)

  return (
    <div className="relative">
      <Handle type="target" position={positions.target} />
      <WorkflowNodeCard data={data} selected={selected} />
      <Handle type="source" position={positions.source} />
    </div>
  )
}

export const TaskNode = memo(TaskNodeInner)
