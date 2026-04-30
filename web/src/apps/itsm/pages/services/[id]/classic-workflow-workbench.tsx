import { useState } from "react"
import { useTranslation } from "react-i18next"
import { useNavigate } from "react-router"
import { useMutation, useQueryClient } from "@tanstack/react-query"
import { type Edge, type Node } from "@xyflow/react"
import { ArrowLeft, Settings2, Zap } from "lucide-react"
import { toast } from "sonner"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Textarea } from "@/components/ui/textarea"
import { Label } from "@/components/ui/label"
import { Switch } from "@/components/ui/switch"
import { Badge } from "@/components/ui/badge"
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectLabel,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select"
import { WorkspaceStatus } from "@/components/workspace/primitives"
import { usePermission } from "@/hooks/use-permission"
import { WorkflowEditor } from "../../../components/workflow"
import type { FormSchema } from "../../../components/form-engine"
import {
  updateServiceDef,
  type CatalogItem,
  type ServiceDefItem,
  type SLATemplateItem,
} from "../../../api"
import { itsmQueryKeys } from "../../../query-keys"

interface ClassicWorkflowWorkbenchProps {
  service: ServiceDefItem
  catalogs: CatalogItem[]
  slaTemplates: SLATemplateItem[]
}

type ServiceDraft = {
  name: string
  catalogId: number
  slaId: number | null
  isActive: boolean
  description: string
}

function toFormSchema(raw: unknown): FormSchema {
  if (raw && typeof raw === "object") {
    const schema = raw as FormSchema
    if (Array.isArray(schema.fields)) return schema
  }
  return { version: 1, fields: [] }
}

function cleanFormSchema(raw: unknown) {
  const schema = toFormSchema(raw)
  return schema.fields.length > 0 ? schema : null
}

function parseWorkflow(raw: unknown): { nodes: Node[]; edges: Edge[] } | undefined {
  if (!raw) return undefined
  try {
    const workflow = typeof raw === "string" ? JSON.parse(raw) : raw
    if (!workflow || typeof workflow !== "object") return undefined
    const data = workflow as { nodes?: Node[]; edges?: Edge[] }
    return { nodes: data.nodes ?? [], edges: data.edges ?? [] }
  } catch {
    return undefined
  }
}

export function ClassicWorkflowWorkbench({ service, catalogs, slaTemplates }: ClassicWorkflowWorkbenchProps) {
  const { t } = useTranslation(["itsm", "common"])
  const navigate = useNavigate()
  const queryClient = useQueryClient()
  const canUpdate = usePermission("itsm:service:update")
  const [serviceDraft, setServiceDraft] = useState<ServiceDraft>({
    name: service.name,
    catalogId: service.catalogId,
    slaId: service.slaId,
    isActive: service.isActive,
    description: service.description ?? "",
  })
  const [intakeFormSchema, setIntakeFormSchema] = useState<unknown>(() => cleanFormSchema(service.intakeFormSchema))

  const saveMut = useMutation({
    mutationFn: (workflow: { nodes: Node[]; edges: Edge[] }) => updateServiceDef(service.id, {
      name: serviceDraft.name,
      catalogId: serviceDraft.catalogId,
      slaId: serviceDraft.slaId,
      isActive: serviceDraft.isActive,
      description: serviceDraft.description,
      intakeFormSchema: cleanFormSchema(intakeFormSchema),
      workflowJson: workflow,
    } as Partial<ServiceDefItem>),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: itsmQueryKeys.services.detail(service.id) })
      queryClient.invalidateQueries({ queryKey: itsmQueryKeys.services.lists() })
      queryClient.invalidateQueries({ queryKey: itsmQueryKeys.catalogs.serviceCounts() })
      toast.success(t("itsm:workflow.saveSuccess"))
    },
    onError: (err) => toast.error(err.message),
  })

  return (
    <div className="flex h-[calc(100vh-theme(spacing.14)-theme(spacing.8))] min-h-[720px] flex-col overflow-hidden rounded-lg border border-border/55 bg-background/62">
      <div className="flex min-h-14 shrink-0 items-center justify-between gap-3 border-b border-border/50 bg-white/48 px-4">
        <div className="flex min-w-0 items-center gap-3">
          <Button variant="ghost" size="icon-sm" onClick={() => navigate("/itsm/services")}>
            <ArrowLeft className="h-4 w-4" />
          </Button>
          <div className="min-w-0">
            <div className="flex min-w-0 flex-wrap items-center gap-2">
              <h2 className="truncate text-base font-semibold tracking-[-0.02em]">{serviceDraft.name || service.name}</h2>
              <Badge variant="outline" className="h-5">经典</Badge>
              <WorkspaceStatus tone={serviceDraft.isActive ? "success" : "neutral"} label={serviceDraft.isActive ? "启用" : "停用"} />
            </div>
            <p className="mt-0.5 truncate font-mono text-xs text-muted-foreground">{service.code}</p>
          </div>
        </div>
        <div className="flex shrink-0 items-center gap-2">
          <Button variant="outline" size="sm" onClick={() => navigate(`/itsm/services/${service.id}/actions`)}>
            <Zap className="mr-1.5 h-3.5 w-3.5" />
            动作
          </Button>
        </div>
      </div>
      <WorkflowEditor
        key={service.updatedAt}
        initialData={parseWorkflow(service.workflowJson)}
        serviceId={service.id}
        intakeFormSchema={intakeFormSchema}
        onIntakeFormSchemaChange={setIntakeFormSchema}
        onSave={(workflow) => saveMut.mutate(workflow)}
        saving={saveMut.isPending}
        inspectorFallback={(
          <ServiceInspector
            service={service}
            catalogs={catalogs}
            slaTemplates={slaTemplates}
            draft={serviceDraft}
            onDraftChange={setServiceDraft}
            canUpdate={canUpdate}
          />
        )}
      />
    </div>
  )
}

function ServiceInspector({
  service,
  catalogs,
  slaTemplates,
  draft,
  onDraftChange,
  canUpdate,
}: {
  service: ServiceDefItem
  catalogs: CatalogItem[]
  slaTemplates: SLATemplateItem[]
  draft: ServiceDraft
  onDraftChange: (draft: ServiceDraft) => void
  canUpdate: boolean
}) {
  const setDraft = (patch: Partial<ServiceDraft>) => onDraftChange({ ...draft, ...patch })

  return (
    <aside className="flex w-[392px] shrink-0 flex-col border-l border-border/55 bg-white/54">
      <div className="flex min-h-16 items-center gap-2.5 border-b border-border/50 px-4">
        <div className="flex size-8 shrink-0 items-center justify-center rounded-lg border border-border/55 bg-background/70 text-primary">
          <Settings2 className="size-4" />
        </div>
        <div className="min-w-0">
          <div className="truncate text-sm font-semibold">服务属性</div>
          <div className="truncate text-xs text-muted-foreground">经典流程设计上下文</div>
        </div>
      </div>
      <div className="min-h-0 flex-1 space-y-3 overflow-y-auto p-4">
        <section className="space-y-3 rounded-xl border border-border/55 bg-white/58 p-3">
          <div className="text-[11px] font-semibold uppercase tracking-[0.16em] text-muted-foreground/72">身份</div>
          <div className="space-y-1.5">
            <Label className="text-xs">服务名称</Label>
            <Input value={draft.name} onChange={(event) => setDraft({ name: event.target.value })} disabled={!canUpdate} className="h-9 text-sm" />
          </div>
          <div className="space-y-1.5">
            <Label className="text-xs">服务编码</Label>
            <Input value={service.code} disabled className="h-9 font-mono text-sm" />
          </div>
          <div className="space-y-1.5">
            <Label className="text-xs">所属分类</Label>
            <Select value={String(draft.catalogId)} onValueChange={(value) => setDraft({ catalogId: Number(value) })} disabled={!canUpdate}>
              <SelectTrigger className="h-9 text-sm"><SelectValue /></SelectTrigger>
              <SelectContent>
                {catalogs.map((parent) => (
                  <SelectGroup key={parent.id}>
                    <SelectLabel className="text-xs font-semibold text-muted-foreground">{parent.name}</SelectLabel>
                    {parent.children?.length ? (
                      parent.children.map((child) => (
                        <SelectItem key={child.id} value={String(child.id)} className="pl-6">{child.name}</SelectItem>
                      ))
                    ) : (
                      <SelectItem value={String(parent.id)} className="pl-6">{parent.name}</SelectItem>
                    )}
                  </SelectGroup>
                ))}
              </SelectContent>
            </Select>
          </div>
        </section>

        <section className="space-y-3 rounded-xl border border-border/55 bg-white/58 p-3">
          <div className="text-[11px] font-semibold uppercase tracking-[0.16em] text-muted-foreground/72">运行</div>
          <div className="space-y-1.5">
            <Label className="text-xs">SLA 模板</Label>
            <Select value={String(draft.slaId ?? 0)} onValueChange={(value) => setDraft({ slaId: value === "0" ? null : Number(value) })} disabled={!canUpdate}>
              <SelectTrigger className="h-9 text-sm"><SelectValue /></SelectTrigger>
              <SelectContent>
                <SelectItem value="0">-</SelectItem>
                {slaTemplates.map((sla) => (
                  <SelectItem key={sla.id} value={String(sla.id)}>{sla.name}</SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>
          <div className="flex items-center justify-between gap-3 rounded-lg border border-border/55 bg-background/45 px-3 py-2">
            <Label className="text-xs">启用服务</Label>
            <Switch checked={draft.isActive} onCheckedChange={(isActive) => setDraft({ isActive })} disabled={!canUpdate} />
          </div>
        </section>

        <section className="space-y-3 rounded-xl border border-border/55 bg-white/58 p-3">
          <div className="text-[11px] font-semibold uppercase tracking-[0.16em] text-muted-foreground/72">描述</div>
          <Textarea
            value={draft.description}
            onChange={(event) => setDraft({ description: event.target.value })}
            rows={4}
            disabled={!canUpdate}
            className="resize-none bg-white/55 text-sm"
          />
        </section>

      </div>
    </aside>
  )
}
