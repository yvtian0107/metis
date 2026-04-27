import { useTranslation } from "react-i18next"
import { useMemo } from "react"
import type React from "react"
import { type Node, type Edge, useReactFlow } from "@xyflow/react"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select"
import { Button } from "@/components/ui/button"
import { Switch } from "@/components/ui/switch"
import { Trash2, X } from "lucide-react"
import { cn } from "@/lib/utils"
import type { WFNodeData, WFEdgeData, NodeType, ConditionGroup } from "./types"
import { getNodeAccent } from "./visual-data"
import { WorkflowNodeIconGlyph } from "./visual"
import { ParticipantPicker } from "./panels/participant-picker"
import { ConditionBuilder } from "./panels/condition-builder"
import { VariableMappingEditor } from "./panels/variable-mapping-editor"
import { ScriptAssignmentEditor } from "./panels/script-assignment-editor"
import { ActionPicker } from "./panels/action-picker"
import { FormComposer, type FormSchema, type WorkflowNodeRef } from "../form-engine"

function PanelSection({ title, children, className }: { title: string; children: React.ReactNode; className?: string }) {
  return (
    <section className={cn("space-y-3 rounded-xl border border-border/55 bg-white/58 p-3", className)}>
      <div className="text-[11px] font-semibold uppercase tracking-[0.16em] text-muted-foreground/72">{title}</div>
      {children}
    </section>
  )
}

function toFormSchema(raw: unknown): FormSchema {
  if (raw && typeof raw === "object") {
    const schema = raw as FormSchema
    if (Array.isArray(schema.fields)) return schema
  }
  return { version: 1, fields: [] }
}

// ─── Node Property Panel ────────────────────────────────

interface NodePanelProps {
  node: Node & { data: WFNodeData }
  serviceId?: number
  intakeFormSchema?: unknown
  onIntakeFormSchemaChange?: (schema: unknown) => void
  onClose: () => void
}

export function NodePropertyPanel({ node, serviceId, intakeFormSchema, onIntakeFormSchemaChange, onClose }: NodePanelProps) {
  const { t } = useTranslation("itsm")
  const { setNodes, deleteElements, getNodes } = useReactFlow()
  const data = node.data
  const nodeType = data.nodeType as NodeType

  const workflowNodes = useMemo<WorkflowNodeRef[]>(() => {
    const HUMAN_TYPES = new Set(["form", "process"])
    return getNodes()
      .filter((n) => HUMAN_TYPES.has((n.data as unknown as WFNodeData).nodeType))
      .map((n) => ({ id: n.id, label: (n.data as unknown as WFNodeData).label }))
  }, [getNodes])

  function updateData(patch: Partial<WFNodeData>) {
    setNodes((nds) => nds.map((n) => n.id === node.id ? { ...n, data: { ...n.data, ...patch } } : n))
  }

  function handleDelete() {
    deleteElements({ nodes: [{ id: node.id }] })
    onClose()
  }

  const hasParticipants = nodeType === "form" || nodeType === "process"
  const hasFormBinding = nodeType === "form" || nodeType === "process"
  const hasProcessMode = nodeType === "process"
  const hasAction = nodeType === "action"
  const hasScript = nodeType === "script"
  const hasNotify = nodeType === "notify"
  const hasWait = nodeType === "wait" || nodeType === "timer"
  const hasMapping = nodeType === "form" || nodeType === "process"
  const hasGatewayDirection = nodeType === "parallel" || nodeType === "inclusive"
  const isProtected = nodeType === "start" || nodeType === "end"
  const accent = getNodeAccent(nodeType)

  if (nodeType === "start") {
    return (
      <aside className="flex w-[392px] shrink-0 flex-col border-l border-border/55 bg-white/54">
        <div className="flex min-h-16 items-center justify-between gap-3 border-b border-border/50 px-4">
          <div className="flex min-w-0 items-center gap-2.5">
            <div className="flex size-8 shrink-0 items-center justify-center rounded-lg text-white" style={{ backgroundColor: accent }}>
              <WorkflowNodeIconGlyph nodeType={nodeType} className="size-4" />
            </div>
            <div className="min-w-0">
              <div className="truncate text-sm font-semibold">{data.label}</div>
              <div className="text-xs text-muted-foreground">进件表单入口</div>
            </div>
          </div>
          <Button variant="ghost" size="icon" className="h-8 w-8" onClick={onClose}><X size={14} /></Button>
        </div>
        <div className="min-h-0 flex-1 space-y-3 overflow-y-auto p-4">
          <PanelSection title={t("workflow.panel.identity")}>
            <div className="space-y-1.5">
              <Label className="text-xs">{t("workflow.prop.label")}</Label>
              <Input value={data.label} onChange={(e) => updateData({ label: e.target.value })} className="h-9 text-sm" />
            </div>
          </PanelSection>
          <PanelSection title="进件表单" className="min-h-[520px]">
            <FormComposer
              schema={toFormSchema(intakeFormSchema)}
              onChange={(schema) => onIntakeFormSchemaChange?.(schema.fields.length > 0 ? schema : undefined)}
              title="申请人提交字段"
            />
          </PanelSection>
        </div>
      </aside>
    )
  }

  return (
    <aside className="flex w-[392px] shrink-0 flex-col border-l border-border/55 bg-white/54">
      <div className="flex min-h-16 items-center justify-between gap-3 border-b border-border/50 px-4">
        <div className="flex min-w-0 items-center gap-2.5">
          <div className="flex size-8 shrink-0 items-center justify-center rounded-lg text-white" style={{ backgroundColor: accent }}>
            <WorkflowNodeIconGlyph nodeType={nodeType} className="size-4" />
          </div>
          <div className="min-w-0">
            <div className="truncate text-sm font-semibold">{data.label}</div>
            <div className="text-xs text-muted-foreground">{t(`workflow.node.${nodeType}`)}</div>
          </div>
        </div>
        <Button variant="ghost" size="icon" className="h-8 w-8" onClick={onClose}><X size={14} /></Button>
      </div>

      <div className="min-h-0 flex-1 space-y-3 overflow-y-auto p-4">
        <PanelSection title={t("workflow.panel.identity")}>
          <div className="space-y-1.5">
            <Label className="text-xs">{t("workflow.prop.label")}</Label>
            <Input value={data.label} onChange={(e) => updateData({ label: e.target.value })} className="h-9 text-sm" />
          </div>
          {hasParticipants && (
            <ParticipantPicker
              participants={data.participants ?? []}
              onChange={(participants) => updateData({ participants })}
            />
          )}
        </PanelSection>

        {(hasProcessMode || hasWait || hasNotify || hasAction || hasScript || hasGatewayDirection) && (
          <PanelSection title={t("workflow.panel.execution")}>
            {hasGatewayDirection && (
              <div className="space-y-1.5">
                <Label className="text-xs">网关方向</Label>
                <Select value={data.gateway_direction ?? "fork"} onValueChange={(v) => updateData({ gateway_direction: v as WFNodeData["gateway_direction"] })}>
                  <SelectTrigger className="h-9 text-sm"><SelectValue /></SelectTrigger>
                  <SelectContent>
                    <SelectItem value="fork">Fork 分发</SelectItem>
                    <SelectItem value="join">Join 汇聚</SelectItem>
                  </SelectContent>
                </Select>
              </div>
            )}
            {hasProcessMode && (
              <div className="space-y-1.5">
                <Label className="text-xs">{t("workflow.prop.executionMode")}</Label>
                <Select value={data.executionMode ?? "single"} onValueChange={(v) => updateData({ executionMode: v as WFNodeData["executionMode"] })}>
                  <SelectTrigger className="h-9 text-sm"><SelectValue /></SelectTrigger>
                  <SelectContent>
                    <SelectItem value="single">{t("workflow.prop.modeSingle")}</SelectItem>
                    <SelectItem value="parallel">{t("workflow.prop.modeParallel")}</SelectItem>
                    <SelectItem value="sequential">{t("workflow.prop.modeSequential")}</SelectItem>
                  </SelectContent>
                </Select>
              </div>
            )}
            {hasAction && serviceId && (
              <ActionPicker
                serviceId={serviceId}
                actionId={data.action_id}
                onChange={(actionId) => updateData({ action_id: actionId })}
              />
            )}
            {hasScript && (
              <ScriptAssignmentEditor
                assignments={data.assignments ?? []}
                onChange={(assignments) => updateData({ assignments })}
              />
            )}
            {hasNotify && (
              <>
                <div className="space-y-1.5">
                  <Label className="text-xs">{t("workflow.prop.channelType")}</Label>
                  <Input
                    type="number"
                    min={1}
                    value={data.channel_id ?? ""}
                    onChange={(e) => updateData({ channel_id: e.target.value ? Number(e.target.value) : undefined })}
                    placeholder="channel_id"
                    className="h-9 text-sm"
                  />
                </div>
                <div className="space-y-1.5">
                  <Label className="text-xs">{t("workflow.prop.template")}</Label>
                  <Input value={data.template ?? ""} onChange={(e) => updateData({ template: e.target.value })} className="h-9 text-sm" />
                </div>
              </>
            )}
            {hasWait && (
              <div className="space-y-1.5">
                <Label className="text-xs">{t("workflow.prop.waitMode")}</Label>
                <Select value={data.wait_mode ?? "signal"} onValueChange={(v) => updateData({ wait_mode: v as WFNodeData["wait_mode"] })}>
                  <SelectTrigger className="h-9 text-sm"><SelectValue /></SelectTrigger>
                  <SelectContent>
                    <SelectItem value="signal">{t("workflow.prop.waitSignal")}</SelectItem>
                    <SelectItem value="timer">{t("workflow.prop.waitTimer")}</SelectItem>
                  </SelectContent>
                </Select>
                {(data.wait_mode === "timer" || nodeType === "timer") && (
                  <div className="mt-2 space-y-1.5">
                    <Label className="text-xs">{t("workflow.prop.duration")}</Label>
                    <Input value={data.duration ?? ""} onChange={(e) => updateData({ duration: e.target.value })} placeholder="PT1H" className="h-9 text-sm" />
                  </div>
                )}
              </div>
            )}
          </PanelSection>
        )}

        {(hasFormBinding || hasMapping) && (
          <PanelSection title={t("workflow.panel.io")}>
            {hasFormBinding && (
              <FormComposer
                schema={toFormSchema(data.formSchema)}
                onChange={(schema) => updateData({ formSchema: schema.fields.length > 0 ? schema : undefined })}
                title={nodeType === "process" ? "处理结果字段" : "节点提交字段"}
                workflowNodes={workflowNodes}
              />
            )}
            {hasMapping && (
              <>
                <VariableMappingEditor
                  label={t("workflow.prop.inputMapping")}
                  mappings={data.inputMapping ?? []}
                  onChange={(inputMapping) => updateData({ inputMapping })}
                  sourceLabel={t("workflow.mapping.variable")}
                  targetLabel={t("workflow.mapping.formField")}
                />
                <VariableMappingEditor
                  label={t("workflow.prop.outputMapping")}
                  mappings={data.outputMapping ?? []}
                  onChange={(outputMapping) => updateData({ outputMapping })}
                  sourceLabel={t("workflow.mapping.formField")}
                  targetLabel={t("workflow.mapping.variable")}
                />
              </>
            )}
          </PanelSection>
        )}
      </div>

      {!isProtected && (
        <div className="border-t border-border/50 p-4">
          <Button variant="destructive" size="sm" className="w-full" onClick={handleDelete}>
            <Trash2 className="mr-1.5 h-3.5 w-3.5" />{t("workflow.prop.deleteNode")}
          </Button>
        </div>
      )}
    </aside>
  )
}

// ─── Edge Property Panel ────────────────────────────────

interface EdgePanelProps {
  edge: Edge & { data?: WFEdgeData }
  sourceNodeType?: NodeType
  onClose: () => void
}

export function EdgePropertyPanel({ edge, sourceNodeType, onClose }: EdgePanelProps) {
  const { t } = useTranslation("itsm")
  const { setEdges, deleteElements } = useReactFlow()
  const data = (edge.data ?? {}) as WFEdgeData

  function updateData(patch: Partial<WFEdgeData>) {
    setEdges((eds) => eds.map((e) => e.id === edge.id ? { ...e, data: { ...e.data, ...patch } } : e))
  }

  function handleDelete() {
    deleteElements({ edges: [{ id: edge.id }] })
    onClose()
  }

  const isGateway = sourceNodeType === "exclusive" || sourceNodeType === "parallel" || sourceNodeType === "inclusive"

  return (
    <aside className="flex w-[392px] shrink-0 flex-col border-l border-border/55 bg-white/54">
      <div className="flex min-h-16 items-center justify-between gap-3 border-b border-border/50 px-4">
        <div>
          <div className="text-sm font-semibold">{t("workflow.prop.edge")}</div>
          <div className="text-xs text-muted-foreground">{edge.source} → {edge.target}</div>
        </div>
        <Button variant="ghost" size="icon" className="h-8 w-8" onClick={onClose}><X size={14} /></Button>
      </div>

      <div className="min-h-0 flex-1 space-y-3 overflow-y-auto p-4">
        <PanelSection title={t("workflow.panel.identity")}>
          <div className="space-y-1.5">
            <Label className="text-xs">{t("workflow.prop.outcome")}</Label>
            <Input value={data.outcome ?? ""} onChange={(e) => updateData({ outcome: e.target.value })} placeholder="e.g. completed" className="h-9 text-sm" />
          </div>

          <div className="flex items-center gap-2 rounded-lg border border-border/55 bg-background/45 px-3 py-2">
            <Switch checked={(data.default ?? data.isDefault) ?? false} onCheckedChange={(v) => updateData({ default: v, isDefault: undefined })} />
            <Label className="text-xs">{t("workflow.prop.defaultEdge")}</Label>
          </div>
        </PanelSection>

        {isGateway && !(data.default ?? data.isDefault) && (
          <PanelSection title={t("workflow.prop.condition")}>
            <ConditionBuilder
              condition={data.condition}
              onChange={(condition: ConditionGroup | undefined) => updateData({ condition })}
            />
          </PanelSection>
        )}
      </div>

      <div className="border-t border-border/50 p-4">
        <Button variant="destructive" size="sm" className="w-full" onClick={handleDelete}>
          <Trash2 className="mr-1.5 h-3.5 w-3.5" />{t("workflow.prop.deleteEdge")}
        </Button>
      </div>
    </aside>
  )
}
