import { memo, useState } from "react"
import { Handle, Position, type NodeProps } from "@xyflow/react"
import { Minus, Plus } from "lucide-react"
import type { WFNodeData } from "../types"
import { WorkflowNodeCard } from "../visual"

function SubprocessNodeInner({ data: rawData, selected }: NodeProps) {
  const data = rawData as unknown as WFNodeData
  const [expanded, setExpanded] = useState(data.subprocessExpanded ?? false)

  return (
    <div className="relative">
      <Handle type="target" position={Position.Left} />
      <WorkflowNodeCard data={data} selected={selected} className="w-[260px]">
        {expanded && (
          <div className="mt-2 flex h-14 items-center justify-center rounded-lg border border-dashed border-border/70 bg-background/45 text-xs text-muted-foreground">
            子流程缩略
          </div>
        )}
        <button
          type="button"
          onClick={(e) => {
            e.stopPropagation()
            setExpanded((v) => !v)
          }}
          className="mt-2 inline-flex min-h-6 items-center gap-1 rounded-md px-1.5 text-[11px] text-muted-foreground hover:bg-muted/70 hover:text-foreground"
        >
          {expanded ? <Minus className="size-3" /> : <Plus className="size-3" />}
          {expanded ? "收起" : "展开"}
        </button>
      </WorkflowNodeCard>
      <Handle type="source" position={Position.Right} />
    </div>
  )
}

export const SubprocessNode = memo(SubprocessNodeInner)
