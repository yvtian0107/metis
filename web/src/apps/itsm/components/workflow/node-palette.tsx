import { useTranslation } from "react-i18next"
import { useQuery } from "@tanstack/react-query"
import { type Node } from "@xyflow/react"
import type { DragEvent } from "react"
import type { NodeType } from "./types"
import type { WFNodeData } from "./types"
import { WORKFLOW_NODE_GROUPS, getNodeAccent } from "./visual-data"
import { WorkflowNodeIconGlyph } from "./visual"
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs"
import { Badge } from "@/components/ui/badge"
import { fetchServiceActions } from "../../api"
import type { WorkflowCapability } from "../../contract"
import { itsmQueryKeys } from "../../query-keys"
import type { FormField, FormSchema } from "../form-engine"

interface NodePaletteProps {
  serviceId?: number
  nodes?: Node[]
  intakeFormSchema?: unknown
  workflowCapability?: WorkflowCapability
}

function isFormSchema(raw: unknown): raw is FormSchema {
  return Boolean(raw && typeof raw === "object" && Array.isArray((raw as FormSchema).fields))
}

function fieldVariables(schema: unknown, prefix: string): Array<FormField & { variable: string }> {
  if (!isFormSchema(schema)) return []
  return schema.fields.map((field) => ({ ...field, variable: `${prefix}.${field.key}` }))
}

export function NodePalette({ serviceId, nodes = [], intakeFormSchema, workflowCapability }: NodePaletteProps) {
  const { t } = useTranslation("itsm")
  const { data: actions = [] } = useQuery({
    queryKey: serviceId ? itsmQueryKeys.services.actions(serviceId) : itsmQueryKeys.services.actions(0),
    queryFn: () => fetchServiceActions(serviceId ?? 0),
    enabled: !!serviceId,
  })

  const variables = [
    ...fieldVariables(intakeFormSchema, "form"),
    ...nodes.flatMap((node) => {
      const data = node.data as unknown as WFNodeData
      if (data.nodeType !== "form" && data.nodeType !== "process") return []
      return fieldVariables(data.formSchema, node.id)
    }),
  ]

  function onDragStart(event: DragEvent, nodeType: NodeType) {
    event.dataTransfer.setData("application/reactflow-nodetype", nodeType)
    event.dataTransfer.effectAllowed = "move"
  }

  return (
    <aside className="flex h-full min-h-0 w-[252px] shrink-0 flex-col overflow-hidden border-r border-border/55 bg-white/48">
      <div className="shrink-0 border-b border-border/45 px-3 py-3">
        <div className="text-sm font-semibold tracking-[-0.01em]">资源</div>
      </div>
      <Tabs defaultValue="nodes" className="min-h-0 flex-1 gap-0">
        <TabsList variant="line" className="grid h-10 w-full grid-cols-4 rounded-none border-b border-border/45 px-2">
          <TabsTrigger value="nodes" className="text-xs">节点</TabsTrigger>
          <TabsTrigger value="fields" className="text-xs">字段</TabsTrigger>
          <TabsTrigger value="actions" className="text-xs">动作</TabsTrigger>
          <TabsTrigger value="vars" className="text-xs">变量</TabsTrigger>
        </TabsList>
        <TabsContent value="nodes" className="min-h-0 overflow-y-auto px-3 py-3">
          <div className="space-y-4 pr-1">
            {WORKFLOW_NODE_GROUPS.map((group) => {
              const executableTypes = group.types.filter((nt) => workflowCapability?.nodeTypes[nt]?.executable)
              if (executableTypes.length === 0) return null
              return (
              <section key={group.label}>
                <div className="mb-1.5 px-1 text-[10px] font-semibold uppercase tracking-[0.18em] text-muted-foreground/72">
                  {t(group.label)}
                </div>
                <div className="space-y-1.5">
                  {executableTypes.map((nt) => {
                    const color = getNodeAccent(nt)
                    return (
                      <div
                        key={nt}
                        draggable
                        onDragStart={(e) => onDragStart(e, nt)}
                        className="group flex min-h-[3.125rem] cursor-grab items-center gap-2.5 rounded-xl border border-border/64 bg-white/72 px-2.5 py-2 transition hover:border-primary/35 hover:bg-white active:cursor-grabbing"
                      >
                        <div
                          className="flex size-7 shrink-0 items-center justify-center rounded-lg text-white"
                          style={{ backgroundColor: color }}
                        >
                          <WorkflowNodeIconGlyph nodeType={nt} />
                        </div>
                        <div className="min-w-0">
                          <div className="truncate text-[13px] font-medium text-foreground">{t(`workflow.node.${nt}`)}</div>
                          <div className="truncate text-[11px] text-muted-foreground">{t(`workflow.nodeDesc.${nt}`)}</div>
                        </div>
                      </div>
                    )
                  })}
                </div>
              </section>
            )})}
          </div>
        </TabsContent>
        <TabsContent value="fields" className="min-h-0 overflow-y-auto px-3 py-3">
          <div className="space-y-2">
            {variables.slice(0, 12).map((field) => (
              <div key={field.variable} className="rounded-lg border border-border/58 bg-white/58 px-2.5 py-2">
                <div className="truncate text-xs font-medium">{field.label}</div>
                <div className="mt-1 flex items-center gap-1.5">
                  <Badge variant="outline" className="h-4 px-1.5 text-[10px]">{t(`forms.type.${field.type}`)}</Badge>
                  <span className="truncate font-mono text-[10px] text-muted-foreground">{field.variable}</span>
                </div>
              </div>
            ))}
            {variables.length === 0 ? <div className="rounded-lg border border-dashed border-border/65 px-3 py-6 text-center text-xs text-muted-foreground">暂无表单字段</div> : null}
          </div>
        </TabsContent>
        <TabsContent value="actions" className="min-h-0 overflow-y-auto px-3 py-3">
          <div className="space-y-1.5">
            {actions.map((action) => (
              <div key={action.id} className="rounded-lg border border-border/58 bg-white/58 px-2.5 py-2">
                <div className="truncate text-xs font-medium">{action.name}</div>
                <div className="mt-1 flex items-center justify-between gap-2">
                  <span className="truncate font-mono text-[10px] text-muted-foreground">{action.code}</span>
                  <Badge variant="outline" className="h-4 px-1.5 text-[10px]">{action.actionType}</Badge>
                </div>
              </div>
            ))}
            {actions.length === 0 ? <div className="rounded-lg border border-dashed border-border/65 px-3 py-6 text-center text-xs text-muted-foreground">暂无可用动作</div> : null}
          </div>
        </TabsContent>
        <TabsContent value="vars" className="min-h-0 overflow-y-auto px-3 py-3">
          <div className="space-y-1.5">
            {variables.map((field) => (
              <div key={field.variable} className="rounded-lg border border-border/58 bg-white/58 px-2.5 py-2">
                <div className="truncate font-mono text-[11px] text-foreground/82">{field.variable}</div>
                <div className="mt-1 truncate text-[11px] text-muted-foreground">{field.label}</div>
              </div>
            ))}
            {variables.length === 0 ? <div className="rounded-lg border border-dashed border-border/65 px-3 py-6 text-center text-xs text-muted-foreground">暂无流程变量</div> : null}
          </div>
        </TabsContent>
      </Tabs>
    </aside>
  )
}
