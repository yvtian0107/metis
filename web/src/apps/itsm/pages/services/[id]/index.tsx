"use client"

import { useState, useEffect, useRef, lazy, Suspense, type ReactNode } from "react"
import { useParams, useNavigate } from "react-router"
import { useTranslation } from "react-i18next"
import { useForm, useWatch } from "react-hook-form"
import { z } from "zod"
import { zodResolver } from "@hookform/resolvers/zod"
import { useQuery, useMutation, useQueryClient, useIsMutating } from "@tanstack/react-query"
import { ArrowLeft, Save, Loader2, Sparkles, ShieldCheck, CheckCircle2, AlertTriangle, XCircle, RefreshCw } from "lucide-react"
import { usePermission } from "@/hooks/use-permission"
import { cn } from "@/lib/utils"
import { toast } from "sonner"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Textarea } from "@/components/ui/textarea"
import { Badge } from "@/components/ui/badge"
import { Switch } from "@/components/ui/switch"
import {
  Select, SelectContent, SelectGroup, SelectItem, SelectLabel, SelectTrigger, SelectValue,
} from "@/components/ui/select"
import {
  Form, FormControl, FormField, FormItem, FormLabel, FormMessage,
} from "@/components/ui/form"
import {
  type ServiceDefItem, type CatalogItem,
  type SLATemplateItem,
  fetchServiceDef, updateServiceDef,
  fetchCatalogTree, fetchSLATemplates,
  generateWorkflow,
  fetchServiceHealth,
  type ServiceHealthItem, type ServiceHealthCheck,
} from "../../../api"
import { SmartServiceConfig } from "../../../components/smart-service-config"
import { ServiceKnowledgeCard } from "../../../components/service-knowledge-card"
import { ServiceActionsSection } from "../../../components/service-actions-section"
import { ClassicWorkflowWorkbench } from "./classic-workflow-workbench"
import { itsmQueryKeys } from "../../../query-keys"
import { shouldResetServiceForm } from "../service-catalog-state"

const WorkflowPreview = lazy(() => import("./workflow-preview"))
const GENERATE_WORKFLOW_MUTATION_KEY = ["itsm-generate-workflow"] as const

// ─── Schema hooks ──────────────────────────────────────

function useBasicInfoSchema() {
  const { t } = useTranslation("itsm")
  return z.object({
    name: z.string().min(1, t("validation.nameRequired")),
    description: z.string().default(""),
    catalogId: z.number().min(1),
    slaId: z.number().nullable(),
    isActive: z.boolean().default(true),
    collaborationSpec: z.string().default(""),
  })
}

type BasicFormValues = z.infer<ReturnType<typeof useBasicInfoSchema>>

function basicFormValuesFromService(service: ServiceDefItem): BasicFormValues {
  return {
    name: service.name,
    description: service.description,
    catalogId: service.catalogId,
    slaId: service.slaId,
    isActive: service.isActive,
    collaborationSpec: service.collaborationSpec ?? "",
  }
}

// ─── Generate Workflow Button ─────────────────────────

function GenerateWorkflowButton({ serviceId, collaborationSpec }: {
  serviceId: number
  collaborationSpec: string
}) {
  const { t } = useTranslation(["itsm", "common"])
  const queryClient = useQueryClient()

  const generateMut = useMutation({
    mutationKey: [...GENERATE_WORKFLOW_MUTATION_KEY, serviceId],
    mutationFn: () => generateWorkflow({ serviceId, collaborationSpec }),
    onSuccess: (resp) => {
      if (resp.errors && resp.errors.length > 0) {
        toast.warning(t("itsm:generate.partialSuccess", { count: resp.errors.length }))
      } else {
        toast.success(t("itsm:generate.success"))
      }
      if (resp.service) {
        queryClient.setQueryData(itsmQueryKeys.services.detail(serviceId), {
          ...resp.service,
          publishHealthCheck: resp.healthCheck ?? resp.service.publishHealthCheck,
        })
      } else {
        queryClient.invalidateQueries({ queryKey: itsmQueryKeys.services.detail(serviceId) })
      }
      queryClient.invalidateQueries({ queryKey: itsmQueryKeys.services.lists() })
    },
    onError: (err) => toast.error(err.message),
  })

  const specEmpty = !collaborationSpec?.trim()

  return (
    <>
      <Button
        data-testid="itsm-generate-workflow-button"
        type="button"
        variant="outline"
        onClick={() => generateMut.mutate()}
        disabled={specEmpty || generateMut.isPending}
      >
        {generateMut.isPending ? (
          <Loader2 className="mr-1.5 h-4 w-4 animate-spin" />
        ) : (
          <Sparkles className="mr-1.5 h-4 w-4" />
        )}
        {generateMut.isPending ? t("itsm:generate.generating") : t("itsm:generate.button")}
      </Button>
      {specEmpty && (
        <span className="text-xs text-muted-foreground">{t("itsm:generate.specRequired")}</span>
      )}
    </>
  )
}

// ─── Publish Health Section ────────────────────────────

function healthTone(status: ServiceHealthItem["status"] | "empty") {
  if (status === "pass") {
    return {
      icon: CheckCircle2,
      badge: "default" as const,
      iconClassName: "text-emerald-600",
    }
  }
  if (status === "fail") {
    return {
      icon: XCircle,
      badge: "destructive" as const,
      iconClassName: "text-red-600",
    }
  }
  if (status === "empty") {
    return {
      icon: ShieldCheck,
      badge: "outline" as const,
      iconClassName: "text-muted-foreground",
    }
  }
  return {
    icon: AlertTriangle,
    badge: "secondary" as const,
    iconClassName: "text-amber-600",
  }
}

function healthStatusText(status: ServiceHealthItem["status"] | "empty") {
  if (status === "empty") return "暂无结果"
  if (status === "pass") return "可发布"
  if (status === "fail") return "需修复"
  return "存在歧义"
}

function healthItemStatusText(status: ServiceHealthItem["status"]) {
  if (status === "pass") return "正常"
  if (status === "fail") return "失败"
  return "需确认"
}

function healthLocationKindText(kind: ServiceHealthItem["location"]["kind"]) {
  if (kind === "collaboration_spec") return "协作规范"
  if (kind === "workflow_node") return "流程节点"
  if (kind === "workflow_edge") return "流程连线"
  if (kind === "action") return "服务动作"
  return "运行配置"
}

function canLocateWorkflow(item: ServiceHealthItem) {
  return (item.location.kind === "workflow_node" || item.location.kind === "workflow_edge") && !!item.location.refId
}

function ServiceHealthSection({
  serviceId,
  health,
  isGenerating,
  canLocateInWorkflow,
  onLocateInWorkflow,
}: {
  serviceId: number
  health: ServiceHealthCheck | null
  isGenerating: boolean
  canLocateInWorkflow: boolean
  onLocateInWorkflow: (item: ServiceHealthItem) => void
}) {
  const { t } = useTranslation("itsm")
  const queryClient = useQueryClient()
  const refreshMut = useMutation({
    mutationFn: () => fetchServiceHealth(serviceId),
    onSuccess: (next) => {
      queryClient.setQueryData<ServiceDefItem | undefined>(itsmQueryKeys.services.detail(serviceId), (prev) => (
        prev ? { ...prev, publishHealthCheck: next } : prev
      ))
      toast.success(t("publishHealth.refreshSuccess"))
    },
    onError: (err) => toast.error(err.message),
  })
  const displayStatus: ServiceHealthItem["status"] | "empty" = !health ? "empty" : health.status
  const healthItems = health?.items ?? []
  const hasNoIssues = !!health && displayStatus === "pass" && healthItems.length === 0
  const overall = healthTone(displayStatus)
  const OverallIcon = overall.icon
  const isLoading = isGenerating || refreshMut.isPending
  const summaryTitle = displayStatus === "empty"
    ? "暂无检查结果"
    : hasNoIssues
      ? "未发现运行前阻塞项"
      : displayStatus === "pass"
        ? "未发现运行风险"
        : displayStatus === "warn"
          ? "存在歧义，需确认"
          : "请处理运行阻塞项"
  const summaryDescription = hasNoIssues
    ? "流程结构、参与者解析和服务引用已通过发布前检查。"
    : "仅检查智能引擎运行前的阻塞项、歧义和失效引用。"

  return (
    <section className="flex min-h-[500px] flex-col">
      <SectionHeader
        title={(
          <span className="inline-flex items-center gap-2">
            <ShieldCheck className="h-4 w-4" />
            发布健康检查
          </span>
        )}
        action={(
          <div className="inline-flex h-9 items-center rounded-full border border-border/60 bg-gradient-to-r from-white/86 to-white/72 p-1 shadow-[0_14px_32px_-24px_rgba(15,23,42,0.62)]">
            <span className="inline-flex items-center gap-1.5 rounded-full px-3 text-xs font-medium text-foreground/82">
              <OverallIcon className={cn("h-3.5 w-3.5", overall.iconClassName)} />
              {healthStatusText(displayStatus)}
            </span>
            <span className="mx-1.5 h-4 w-px bg-border/70" aria-hidden />
            <Button
              type="button"
              variant="ghost"
              size="sm"
              className="h-7 rounded-full px-3 text-xs"
              disabled={isLoading}
              onClick={() => refreshMut.mutate()}
            >
              {refreshMut.isPending ? (
                <Loader2 className="mr-1.5 h-3.5 w-3.5 animate-spin" />
              ) : (
                <RefreshCw className="mr-1.5 h-3.5 w-3.5" />
              )}
              {refreshMut.isPending ? t("publishHealth.refreshing") : t("publishHealth.refresh")}
            </Button>
          </div>
        )}
      />
      <div className="workspace-surface flex min-h-[460px] flex-1 flex-col overflow-hidden rounded-[1.1rem]">
        {isLoading ? (
          <div className="flex min-h-[260px] flex-1 items-center justify-center px-4 py-5">
            <div className="inline-flex items-center gap-2 text-sm text-muted-foreground">
              <Loader2 className="h-4 w-4 animate-spin" />
              <span>{isGenerating ? t("generate.generating") : t("publishHealth.refreshing")}</span>
            </div>
          </div>
        ) : (
          <>
        <div className="flex items-start gap-3 border-b border-border/45 px-4 py-3 text-sm">
          <OverallIcon className={cn("mt-0.5 h-4 w-4", overall.iconClassName)} />
          <div className="min-w-0 space-y-1">
            <p className="font-medium">{summaryTitle}</p>
            <p className="text-xs leading-5 text-muted-foreground">{summaryDescription}</p>
          </div>
        </div>
        <div className="divide-y divide-border/45">
          {healthItems.map((item) => {
            const tone = healthTone(item.status)
            const Icon = tone.icon
            const location = item.location ?? { kind: "runtime_config", path: "" }
            return (
              <div key={item.key} className="flex flex-wrap items-start gap-3 px-5 py-4">
                <Icon className={cn("mt-0.5 h-4 w-4", tone.iconClassName)} />
                <div className="min-w-0 flex-1">
                  <p className="text-sm font-medium">{item.label}</p>
                  <p className="mt-1 text-sm text-muted-foreground">{item.message}</p>
                  <div className="mt-2.5 grid gap-1.5 text-xs text-muted-foreground">
                    <p>
                      <span className="font-medium text-foreground/80">{t("publishHealth.location")}：</span>
                      {healthLocationKindText(location.kind)}
                      {location.refId ? ` · ${location.refId}` : ""}
                      {location.path ? ` · ${location.path}` : ""}
                    </p>
                    <p>
                      <span className="font-medium text-foreground/80">{t("publishHealth.recommendation")}：</span>
                      {item.recommendation || "请根据检查结论调整相关配置。"}
                    </p>
                    <p>
                      <span className="font-medium text-foreground/80">{t("publishHealth.evidence")}：</span>
                      {item.evidence || "未提供"}
                    </p>
                  </div>
                </div>
                {canLocateInWorkflow && canLocateWorkflow({ ...item, location }) && (
                  <Button
                    type="button"
                    variant="outline"
                    size="sm"
                    className="h-7 px-2.5 text-xs"
                    onClick={() => onLocateInWorkflow(item)}
                  >
                    {t("publishHealth.locate")}
                  </Button>
                )}
                <Badge variant={tone.badge}>{healthItemStatusText(item.status)}</Badge>
              </div>
            )
          })}
          {!healthItems.length && (
            <div className="flex min-h-[180px] items-center justify-center px-5 py-8 text-center">
              {hasNoIssues ? (
                <div className="max-w-[260px] space-y-2">
                  <CheckCircle2 className="mx-auto h-5 w-5 text-emerald-600" />
                  <p className="text-sm font-medium text-foreground">未发现运行前阻塞项</p>
                  <p className="text-xs leading-5 text-muted-foreground">没有需要处理的歧义、失效引用或参与者解析问题。</p>
                </div>
              ) : (
                <p className="text-sm text-muted-foreground">暂无检查结果。</p>
              )}
            </div>
          )}
        </div>
          </>
        )}
      </div>
    </section>
  )
}

// ─── Basic Info Form ──────────────────────────────────
// Mounted only when service + catalogs + slaTemplates are all loaded,
// so useForm defaultValues and SelectItem options are guaranteed in sync.

function BasicInfoForm({ service, catalogs, slaTemplates }: {
  service: ServiceDefItem
  catalogs: CatalogItem[]
  slaTemplates: SLATemplateItem[]
}) {
  const { t } = useTranslation(["itsm", "common"])
  const queryClient = useQueryClient()
  const canUpdate = usePermission("itsm:service:update")
  const schema = useBasicInfoSchema()

  const form = useForm<BasicFormValues>({
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    resolver: zodResolver(schema as any),
    defaultValues: basicFormValuesFromService(service),
  })
  const previousServiceIdRef = useRef<number | null>(service.id)
  const collaborationSpec = useWatch({ control: form.control, name: "collaborationSpec" })

  useEffect(() => {
    if (shouldResetServiceForm({
      previousServiceId: previousServiceIdRef.current,
      nextServiceId: service.id,
      isDirty: form.formState.isDirty,
    })) {
      form.reset(basicFormValuesFromService(service))
    }
    previousServiceIdRef.current = service.id
  }, [form, form.formState.isDirty, service])

  const updateMut = useMutation({
    mutationFn: (v: BasicFormValues) => updateServiceDef(service.id, {
      name: v.name,
      description: v.description,
      catalogId: v.catalogId,
      slaId: v.slaId,
      isActive: v.isActive,
      collaborationSpec: service.engineType === "smart" ? v.collaborationSpec : undefined,
    } as Partial<ServiceDefItem>),
    onSuccess: (next) => {
      queryClient.setQueryData(itsmQueryKeys.services.detail(service.id), next)
      form.reset(basicFormValuesFromService(next))
      queryClient.invalidateQueries({ queryKey: itsmQueryKeys.services.lists() })
      queryClient.invalidateQueries({ queryKey: itsmQueryKeys.catalogs.serviceCounts() })
      toast.success(t("itsm:services.updateSuccess"))
    },
    onError: (err) => toast.error(err.message),
  })

  return (
    <Form {...form}>
      <form onSubmit={form.handleSubmit((v) => updateMut.mutate(v))} className="space-y-4">
        <div className="grid grid-cols-1 gap-4 md:grid-cols-2 xl:grid-cols-12">
          <FormField control={form.control} name="name" render={({ field }) => (
            <FormItem className="xl:col-span-3">
              <FormLabel>{t("itsm:services.name")}</FormLabel>
              <FormControl><Input placeholder={t("itsm:services.namePlaceholder")} {...field} /></FormControl>
              <FormMessage />
            </FormItem>
          )} />
          <div className="space-y-1.5 xl:col-span-2">
            <label className="text-sm font-medium">{t("itsm:services.code")}</label>
            <Input value={service.code} disabled />
          </div>
          <FormField control={form.control} name="catalogId" render={({ field }) => (
            <FormItem className="xl:col-span-3">
              <FormLabel>{t("itsm:services.catalog")}</FormLabel>
              <Select onValueChange={(v) => field.onChange(Number(v))} value={String(field.value)}>
                <FormControl><SelectTrigger className="w-full"><SelectValue placeholder={t("itsm:services.catalogPlaceholder")} /></SelectTrigger></FormControl>
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
              <FormMessage />
            </FormItem>
          )} />
          <FormField control={form.control} name="slaId" render={({ field }) => (
            <FormItem className="xl:col-span-2">
              <FormLabel>{t("itsm:services.sla")}</FormLabel>
              <Select onValueChange={(v) => field.onChange(v === "0" ? null : Number(v))} value={String(field.value ?? 0)}>
                <FormControl><SelectTrigger className="w-full"><SelectValue placeholder={t("itsm:services.slaPlaceholder")} /></SelectTrigger></FormControl>
                <SelectContent>
                  <SelectItem value="0">—</SelectItem>
                  {slaTemplates.map((s) => (
                    <SelectItem key={s.id} value={String(s.id)}>{s.name}</SelectItem>
                  ))}
                </SelectContent>
              </Select>
              <FormMessage />
            </FormItem>
          )} />
          <FormField control={form.control} name="isActive" render={({ field }) => (
            <FormItem className="xl:col-span-2">
              <FormLabel>{t("itsm:services.status")}</FormLabel>
              <div className="flex h-9 items-center justify-between gap-2 rounded-md border border-border/70 bg-background/42 px-3 shadow-[inset_0_1px_0_rgba(255,255,255,0.62)]">
                <Switch checked={field.value} onCheckedChange={field.onChange} />
                <span className="min-w-10 text-right text-sm text-muted-foreground">
                  {field.value ? t("itsm:services.active") : t("itsm:services.inactive")}
                </span>
              </div>
            </FormItem>
          )} />
        </div>

        <FormField control={form.control} name="description" render={({ field }) => (
          <FormItem>
            <FormLabel>{t("itsm:services.description")}</FormLabel>
            <FormControl><Textarea rows={2} {...field} /></FormControl>
            <FormMessage />
          </FormItem>
        )} />

        {service.engineType === "smart" && (
          <SmartServiceConfig
            collaborationSpec={collaborationSpec}
            onCollaborationSpecChange={(v) => form.setValue("collaborationSpec", v, { shouldDirty: true })}
          />
        )}

        <div className="flex items-center gap-3">
          {canUpdate && (
            <Button type="submit" disabled={updateMut.isPending}>
              <Save className="mr-1.5 h-4 w-4" />
              {updateMut.isPending ? t("common:saving") : t("common:save")}
            </Button>
          )}
          {service.engineType === "smart" && (
            <GenerateWorkflowButton
              serviceId={service.id}
              collaborationSpec={collaborationSpec}
            />
          )}
        </div>
      </form>
    </Form>
  )
}

// ─── Intake Form Section ─────────────────────────────

function SectionHeader({ title, action }: {
  title: ReactNode
  action?: ReactNode
}) {
  return (
    <div className="mb-3 flex min-h-10 items-center justify-between gap-3">
      <h3 className="text-sm font-semibold text-foreground/82">{title}</h3>
      {action}
    </div>
  )
}

function SectionFrame({ title, action, children, noSurface = false }: {
  title: ReactNode
  action?: ReactNode
  children: ReactNode
  noSurface?: boolean
}) {
  return (
    <section>
      <SectionHeader title={title} action={action} />
      {noSurface ? (
        children
      ) : (
        <div className="workspace-surface rounded-[1.25rem] p-5">
          {children}
        </div>
      )}
    </section>
  )
}

// ─── Main Page Component ───────────────────────────────

export function Component() {
  const { t } = useTranslation(["itsm", "common"])
  const { id } = useParams<{ id: string }>()
  const navigate = useNavigate()
  const serviceId = Number(id)
  const [workflowFocus, setWorkflowFocus] = useState<{
    kind: "workflow_node" | "workflow_edge"
    refId: string
    seq: number
  } | null>(null)

  const { data: service, isLoading } = useQuery({
    queryKey: itsmQueryKeys.services.detail(serviceId),
    queryFn: () => fetchServiceDef(serviceId),
    enabled: serviceId > 0,
  })

  const { data: catalogs, isLoading: catalogsLoading } = useQuery({
    queryKey: itsmQueryKeys.catalogs.tree(),
    queryFn: () => fetchCatalogTree(),
  })

  const { data: slaTemplates, isLoading: slaLoading } = useQuery({
    queryKey: itsmQueryKeys.sla.all,
    queryFn: () => fetchSLATemplates(),
  })
  const isGeneratingWorkflow = useIsMutating({ mutationKey: [...GENERATE_WORKFLOW_MUTATION_KEY, serviceId] }) > 0

  if (isLoading || catalogsLoading || slaLoading) {
    return <div className="flex h-96 items-center justify-center"><Loader2 className="h-6 w-6 animate-spin text-muted-foreground" /></div>
  }

  if (!service) {
    return <div className="flex h-96 items-center justify-center text-muted-foreground">Not found</div>
  }

  if (service.engineType === "classic") {
    return (
      <ClassicWorkflowWorkbench
        service={service}
        catalogs={catalogs ?? []}
        slaTemplates={slaTemplates ?? []}
      />
    )
  }

  const workflowSection = (
    <SectionFrame
      noSurface
      title="参考路径/策略草图"
    >
      {isGeneratingWorkflow ? (
        <div className="flex min-h-32 flex-col items-center justify-center gap-2 rounded-xl border border-dashed border-border/70 bg-background/25 px-4 py-7 text-muted-foreground">
          <Loader2 className="h-5 w-5 animate-spin" />
          <p className="text-sm">{t("itsm:generate.generating")}</p>
        </div>
      ) : !service.workflowJson ? (
        <div className="flex min-h-32 flex-col items-center justify-center gap-2 rounded-xl border border-dashed border-border/70 bg-background/25 px-4 py-7 text-muted-foreground">
          <p className="text-sm">
            暂无参考路径
          </p>
          <p className="text-xs">{t("itsm:generate.workflowEmptySmartHint")}</p>
        </div>
      ) : (
        <Suspense fallback={<div className="flex h-80 items-center justify-center"><Loader2 className="h-6 w-6 animate-spin text-muted-foreground" /></div>}>
          <WorkflowPreview
            workflowJson={service.workflowJson}
            embedded
            focusTarget={workflowFocus}
          />
        </Suspense>
      )}
    </SectionFrame>
  )

  return (
    <div className="workspace-page">
      <div className="workspace-page-header">
        <div className="flex min-w-0 items-start gap-3">
          <Button variant="ghost" size="icon-sm" className="mt-1" onClick={() => navigate("/itsm/services")}>
            <ArrowLeft className="h-4 w-4" />
          </Button>
          <div className="min-w-0">
            <div className="flex flex-wrap items-center gap-2">
              <h2 className="workspace-page-title truncate">{service.name}</h2>
              <Badge variant={service.engineType === "smart" ? "default" : "outline"}>
                {service.engineType === "smart" ? t("itsm:services.engineSmart") : t("itsm:services.engineClassic")}
              </Badge>
              <Badge variant={service.isActive ? "secondary" : "outline"}>
                {service.isActive ? t("itsm:services.active") : t("itsm:services.inactive")}
              </Badge>
            </div>
            <p className="workspace-page-description font-mono">{service.code}</p>
          </div>
        </div>
      </div>

      <SectionFrame title={t("itsm:services.tabBasicInfo")}>
        <BasicInfoForm service={service} catalogs={catalogs ?? []} slaTemplates={slaTemplates ?? []} />
      </SectionFrame>

      {service.engineType === "smart" ? (
        <div className="grid items-stretch gap-4 xl:grid-cols-[minmax(0,1fr)_minmax(280px,340px)]">
          <div className="min-w-0">{workflowSection}</div>
          <ServiceHealthSection
            serviceId={serviceId}
            health={service.publishHealthCheck}
            isGenerating={isGeneratingWorkflow}
            canLocateInWorkflow={!!service.workflowJson}
            onLocateInWorkflow={(item) => {
              const location = item.location
              if (!location.refId) {
                return
              }
              if (location.kind !== "workflow_node" && location.kind !== "workflow_edge") {
                return
              }
              setWorkflowFocus({
                kind: location.kind,
                refId: location.refId,
                seq: Date.now(),
              })
            }}
          />
        </div>
      ) : (
        <>
          {workflowSection}
        </>
      )}

      <section className="space-y-3">
        <ServiceActionsSection serviceId={serviceId} />
      </section>

      {service.engineType === "smart" && (
        <section className="space-y-3">
          <ServiceKnowledgeCard
            serviceId={serviceId}
            title={t("itsm:knowledge.title")}
          />
        </section>
      )}
    </div>
  )
}
