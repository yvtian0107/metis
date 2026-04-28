import { memo } from "react"
import { Handle, type NodeProps } from "@xyflow/react"
import type { WFNodeData } from "../types"
import { WorkflowNodeCard } from "../visual"
import { workflowHandlePositions } from "./layout-handles"

function EventNodeInner({ data: rawData, selected }: NodeProps) {
  const data = rawData as unknown as WFNodeData
  const nodeType = data.nodeType
  const positions = workflowHandlePositions(data)

  return (
    <div className="relative">
      {nodeType !== "start" && <Handle type="target" position={positions.target} />}
      <WorkflowNodeCard data={data} selected={selected} className="min-h-[76px]" />
      {nodeType !== "end" && <Handle type="source" position={positions.source} />}
    </div>
  )
}

export const EventNode = memo(EventNodeInner)
