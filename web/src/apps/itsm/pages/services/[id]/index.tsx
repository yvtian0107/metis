"use client"

import { useState, useEffect, lazy, Suspense, type ReactNode } from "react"
import { useParams, useNavigate } from "react-router"
import { useTranslation } from "react-i18next"
import { useForm, useWatch } from "react-hook-form"
import { z } from "zod"
import { zodResolver } from "@hookform/resolvers/zod"
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query"
import { ArrowLeft, Plus, Pencil, Trash2, Zap, Save, Loader2, Sparkles, ShieldCheck, CheckCircle2, AlertTriangle, XCircle } from "lucide-react"
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
  DataTableActions, DataTableActionsCell, DataTableActionsHead,
  DataTableCard, DataTableLoadingRow,
} from "@/components/ui/data-table"
import {
  Table, TableBody, TableCell, TableHead, TableHeader, TableRow,
} from "@/components/ui/table"
import {
  AlertDialog, AlertDialogAction, AlertDialogCancel, AlertDialogContent,
  AlertDialogDescription, AlertDialogFooter, AlertDialogHeader,
  AlertDialogTitle, AlertDialogTrigger,
} from "@/components/ui/alert-dialog"
import {
  Sheet, SheetContent, SheetHeader, SheetTitle, SheetDescription, SheetFooter,
} from "@/components/ui/sheet"
import {
  Form, FormControl, FormField, FormItem, FormLabel, FormMessage,
} from "@/components/ui/form"
import {
  type ServiceDefItem, type CatalogItem, type ServiceActionItem,
  type SLATemplateItem,
  fetchServiceDef, updateServiceDef,
  fetchCatalogTree, fetchSLATemplates,
  fetchServiceActions, createServiceAction, updateServiceAction, deleteServiceAction,
  generateWorkflow,
  type ServiceHealthItem, type ServiceHealthCheck,
} from "../../../api"
import { SmartServiceConfig } from "../../../components/smart-service-config"
import { ServiceKnowledgeCard } from "../../../components/service-knowledge-card"
import { FormDesigner } from "../../../components/form-engine"
import type { FormSchema } from "../../../components/form-engine"

const WorkflowPreview = lazy(() => import("./workflow-preview"))

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

function useActionSchema() {
  const { t } = useTranslation("itsm")
  return z.object({
    name: z.string().min(1, t("validation.nameRequired")),
    code: z.string().min(1, t("validation.codeRequired")),
    actionType: z.string().min(1),
    configJson: z.string().optional(),
  })
}

type ActionFormValues = z.infer<ReturnType<typeof useActionSchema>>

function CompactEmptyRow({ colSpan, icon: Icon, title, description }: {
  colSpan: number
  icon: React.ComponentType<{ className?: string }>
  title: ReactNode
  description?: ReactNode
}) {
  return (
    <TableRow>
      <TableCell colSpan={colSpan} className="h-28 text-center">
        <div className="flex flex-col items-center gap-1.5 text-muted-foreground">
          <Icon className="h-6 w-6 stroke-1" />
          <p className="text-sm font-medium">{title}</p>
          {description ? <p className="text-xs">{description}</p> : null}
        </div>
      </TableCell>
    </TableRow>
  )
}

// ─── Actions Section ──────────────────────────────────

function ActionsSection({ serviceId }: { serviceId: number }) {
  const { t } = useTranslation(["itsm", "common"])
  const queryClient = useQueryClient()
  const [formOpen, setFormOpen] = useState(false)
  const [editing, setEditing] = useState<ServiceActionItem | null>(null)
  const canUpdate = usePermission("itsm:service:update")
  const schema = useActionSchema()

  const { data: items = [], isLoading } = useQuery({
    queryKey: ["itsm-service-actions", serviceId],
    queryFn: () => fetchServiceActions(serviceId),
    enabled: serviceId > 0,
  })

  const form = useForm<ActionFormValues>({
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    resolver: zodResolver(schema as any),
    defaultValues: { name: "", code: "", actionType: "webhook", configJson: "" },
  })

  useEffect(() => {
    if (formOpen) {
      if (editing) {
        form.reset({
          name: editing.name,
          code: editing.code,
          actionType: editing.actionType,
          configJson: editing.configJson ? JSON.stringify(editing.configJson, null, 2) : "",
        })
      } else {
        form.reset({ name: "", code: "", actionType: "webhook", configJson: "" })
      }
    }
  }, [formOpen, editing, form])

  const createMut = useMutation({
    mutationFn: (v: ActionFormValues) => {
      let configJson: unknown = null
      if (v.configJson) {
        try { configJson = JSON.parse(v.configJson) } catch { configJson = null }
      }
      return createServiceAction(serviceId, { name: v.name, code: v.code, actionType: v.actionType, configJson })
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["itsm-service-actions", serviceId] })
      queryClient.invalidateQueries({ queryKey: ["itsm-service", serviceId] })
      setFormOpen(false)
      toast.success(t("itsm:actions.createSuccess"))
    },
    onError: (err) => toast.error(err.message),
  })

  const updateMut = useMutation({
    mutationFn: (v: ActionFormValues) => {
      let configJson: unknown = null
      if (v.configJson) {
        try { configJson = JSON.parse(v.configJson) } catch { configJson = null }
      }
      return updateServiceAction(serviceId, editing!.id, { name: v.name, code: v.code, actionType: v.actionType, configJson })
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["itsm-service-actions", serviceId] })
      queryClient.invalidateQueries({ queryKey: ["itsm-service", serviceId] })
      setFormOpen(false)
      toast.success(t("itsm:actions.updateSuccess"))
    },
    onError: (err) => toast.error(err.message),
  })

  const deleteMut = useMutation({
    mutationFn: (actionId: number) => deleteServiceAction(serviceId, actionId),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["itsm-service-actions", serviceId] })
      queryClient.invalidateQueries({ queryKey: ["itsm-service", serviceId] })
      toast.success(t("itsm:actions.deleteSuccess"))
    },
    onError: (err) => toast.error(err.message),
  })

  function onSubmit(v: ActionFormValues) { if (editing) { updateMut.mutate(v) } else { createMut.mutate(v) } }
  const isPending = createMut.isPending || updateMut.isPending

  return (
    <>
      <SectionHeader
        title={t("itsm:services.tabActions")}
        action={canUpdate ? (
          <Button size="sm" onClick={() => { setEditing(null); setFormOpen(true) }}>
            <Plus className="mr-1.5 h-4 w-4" />{t("itsm:actions.create")}
          </Button>
        ) : undefined}
      />

      <DataTableCard className="rounded-[1.1rem]">
        <Table className="[&_td]:h-11 [&_th]:h-10 [&_th]:text-muted-foreground/72">
          <TableHeader>
            <TableRow>
              <TableHead className="min-w-[160px]">{t("itsm:actions.name")}</TableHead>
              <TableHead className="w-[120px]">{t("itsm:actions.code")}</TableHead>
              <TableHead className="w-[120px]">{t("itsm:actions.actionType")}</TableHead>
              <DataTableActionsHead className="min-w-[140px]">{t("common:actions")}</DataTableActionsHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {isLoading ? (
              <DataTableLoadingRow colSpan={4} />
            ) : items.length === 0 ? (
              <CompactEmptyRow colSpan={4} icon={Zap} title={t("itsm:actions.empty")} description={canUpdate ? t("itsm:actions.emptyHint") : undefined} />
            ) : (
              items.map((item) => (
                <TableRow key={item.id}>
                  <TableCell className="font-medium">{item.name}</TableCell>
                  <TableCell className="text-sm text-muted-foreground">{item.code}</TableCell>
                  <TableCell className="text-sm">{item.actionType}</TableCell>
                  <DataTableActionsCell>
                    <DataTableActions>
                      {canUpdate && (
                        <Button variant="ghost" size="sm" className="px-2.5" onClick={() => { setEditing(item); setFormOpen(true) }}>
                          <Pencil className="mr-1 h-3.5 w-3.5" />{t("common:edit")}
                        </Button>
                      )}
                      {canUpdate && (
                        <AlertDialog>
                          <AlertDialogTrigger asChild>
                            <Button variant="ghost" size="sm" className="px-2.5 text-destructive hover:text-destructive">
                              <Trash2 className="mr-1 h-3.5 w-3.5" />{t("common:delete")}
                            </Button>
                          </AlertDialogTrigger>
                          <AlertDialogContent>
                            <AlertDialogHeader>
                              <AlertDialogTitle>{t("itsm:actions.deleteTitle")}</AlertDialogTitle>
                              <AlertDialogDescription>{t("itsm:actions.deleteDesc", { name: item.name })}</AlertDialogDescription>
                            </AlertDialogHeader>
                            <AlertDialogFooter>
                              <AlertDialogCancel size="sm">{t("common:cancel")}</AlertDialogCancel>
                              <AlertDialogAction size="sm" onClick={() => deleteMut.mutate(item.id)} disabled={deleteMut.isPending}>{t("itsm:actions.confirmDelete")}</AlertDialogAction>
                            </AlertDialogFooter>
                          </AlertDialogContent>
                        </AlertDialog>
                      )}
                    </DataTableActions>
                  </DataTableActionsCell>
                </TableRow>
              ))
            )}
          </TableBody>
        </Table>
      </DataTableCard>

      <Sheet open={formOpen} onOpenChange={setFormOpen}>
        <SheetContent className="sm:max-w-lg">
          <SheetHeader>
            <SheetTitle>{editing ? t("itsm:actions.edit") : t("itsm:actions.create")}</SheetTitle>
            <SheetDescription className="sr-only">{editing ? t("itsm:actions.edit") : t("itsm:actions.create")}</SheetDescription>
          </SheetHeader>
          <Form {...form}>
            <form onSubmit={form.handleSubmit(onSubmit)} className="flex flex-1 flex-col gap-5 px-4">
              <FormField control={form.control} name="name" render={({ field }) => (
                <FormItem>
                  <FormLabel>{t("itsm:actions.name")}</FormLabel>
                  <FormControl><Input placeholder={t("itsm:actions.name")} {...field} /></FormControl>
                  <FormMessage />
                </FormItem>
              )} />
              <FormField control={form.control} name="code" render={({ field }) => (
                <FormItem>
                  <FormLabel>{t("itsm:actions.code")}</FormLabel>
                  <FormControl><Input placeholder={t("itsm:actions.code")} {...field} /></FormControl>
                  <FormMessage />
                </FormItem>
              )} />
              <FormField control={form.control} name="actionType" render={({ field }) => (
                <FormItem>
                  <FormLabel>{t("itsm:actions.actionType")}</FormLabel>
                  <Select onValueChange={field.onChange} value={field.value}>
                    <FormControl><SelectTrigger><SelectValue /></SelectTrigger></FormControl>
                    <SelectContent>
                      <SelectItem value="webhook">Webhook</SelectItem>
                      <SelectItem value="email">Email</SelectItem>
                      <SelectItem value="notification">Notification</SelectItem>
                    </SelectContent>
                  </Select>
                  <FormMessage />
                </FormItem>
              )} />
              <FormField control={form.control} name="configJson" render={({ field }) => (
                <FormItem>
                  <FormLabel>{t("itsm:actions.config")}</FormLabel>
                  <FormControl><Textarea rows={5} placeholder='{"url": "https://..."}' {...field} /></FormControl>
                  <FormMessage />
                </FormItem>
              )} />
              <SheetFooter>
                <Button type="submit" size="sm" disabled={isPending}>
                  {isPending ? t("common:saving") : editing ? t("common:save") : t("common:create")}
                </Button>
              </SheetFooter>
            </form>
          </Form>
        </SheetContent>
      </Sheet>
    </>
  )
}

// ─── Generate Workflow Button ─────────────────────────

function GenerateWorkflowButton({ serviceId, collaborationSpec }: {
  serviceId: number
  collaborationSpec: string
}) {
  const { t } = useTranslation(["itsm", "common"])
  const queryClient = useQueryClient()

  const generateMut = useMutation({
    mutationFn: () => generateWorkflow({ serviceId, collaborationSpec }),
    onSuccess: (resp) => {
      if (resp.errors && resp.errors.length > 0) {
        toast.warning(t("itsm:generate.partialSuccess", { count: resp.errors.length }))
      } else {
        toast.success(t("itsm:generate.success"))
      }
      if (resp.service) {
        queryClient.setQueryData(["itsm-service", serviceId], resp.service)
      } else {
        queryClient.invalidateQueries({ queryKey: ["itsm-service", serviceId] })
      }
      queryClient.invalidateQueries({ queryKey: ["itsm-services"] })
    },
    onError: (err) => toast.error(err.message),
  })

  const specEmpty = !collaborationSpec?.trim()

  return (
    <>
      <Button
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

function ServiceHealthSection({ health }: { health: ServiceHealthCheck | null }) {
  const displayStatus: ServiceHealthItem["status"] | "empty" = !health ? "empty" : health.status
  const overall = healthTone(displayStatus)
  const OverallIcon = overall.icon

  return (
    <section className="flex min-h-full flex-col">
      <SectionHeader
        title={(
          <span className="inline-flex items-center gap-2">
            <ShieldCheck className="h-4 w-4" />
            发布健康检查
          </span>
        )}
        action={<Badge variant={overall.badge}>{healthStatusText(displayStatus)}</Badge>}
      />
      <div className="workspace-surface flex min-h-0 flex-1 flex-col overflow-hidden rounded-[1.1rem]">
        <div className="flex items-start gap-3 border-b border-border/45 px-4 py-3 text-sm">
          <OverallIcon className={cn("mt-0.5 h-4 w-4", overall.iconClassName)} />
          <div className="min-w-0 space-y-1">
            <p className="font-medium">
              {displayStatus === "empty"
                  ? "暂无检查结果"
                  : displayStatus === "pass"
                    ? "未发现运行风险"
                    : displayStatus === "warn"
                      ? "存在歧义，需确认"
                      : "请处理运行阻塞项"}
            </p>
            <p className="text-xs leading-5 text-muted-foreground">仅检查智能引擎运行前的阻塞项、歧义和失效引用。</p>
          </div>
        </div>
        <div className="divide-y divide-border/45">
          {(health?.items ?? []).map((item) => {
            const tone = healthTone(item.status)
            const Icon = tone.icon
            return (
              <div key={item.key} className="flex flex-wrap items-start gap-3 px-5 py-3.5">
                <Icon className={cn("mt-0.5 h-4 w-4", tone.iconClassName)} />
                <div className="min-w-0 flex-1">
                  <p className="text-sm font-medium">{item.label}</p>
                  <p className="text-sm text-muted-foreground">{item.message}</p>
                </div>
                <Badge variant={tone.badge}>{healthItemStatusText(item.status)}</Badge>
              </div>
            )
          })}
          {!health?.items?.length && (
            <div className="px-4 py-5 text-sm text-muted-foreground">
              {health ? "未发现需要处理的运行风险。" : "暂无检查结果。"}
            </div>
          )}
        </div>
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
    defaultValues: {
      name: service.name,
      description: service.description,
      catalogId: service.catalogId,
      slaId: service.slaId,
      isActive: service.isActive,
      collaborationSpec: service.collaborationSpec ?? "",
    },
  })
  const collaborationSpec = useWatch({ control: form.control, name: "collaborationSpec" })

  const updateMut = useMutation({
    mutationFn: (v: BasicFormValues) => updateServiceDef(service.id, {
      name: v.name,
      description: v.description,
      catalogId: v.catalogId,
      slaId: v.slaId,
      isActive: v.isActive,
      collaborationSpec: service.engineType === "smart" ? v.collaborationSpec : undefined,
    } as Partial<ServiceDefItem>),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["itsm-service", service.id] })
      queryClient.invalidateQueries({ queryKey: ["itsm-services"] })
      toast.success(t("itsm:services.updateSuccess"))
    },
    onError: (err) => toast.error(err.message),
  })

  return (
    <Form {...form}>
      <form onSubmit={form.handleSubmit((v) => updateMut.mutate(v))} className="space-y-4">
        <div className="grid grid-cols-1 gap-4 md:grid-cols-2 xl:grid-cols-[minmax(240px,1.25fr)_minmax(240px,1.15fr)_220px_190px_150px]">
          <FormField control={form.control} name="name" render={({ field }) => (
            <FormItem>
              <FormLabel>{t("itsm:services.name")}</FormLabel>
              <FormControl><Input placeholder={t("itsm:services.namePlaceholder")} {...field} /></FormControl>
              <FormMessage />
            </FormItem>
          )} />
          <div className="space-y-1.5">
            <label className="text-sm font-medium">{t("itsm:services.code")}</label>
            <Input value={service.code} disabled />
          </div>
          <FormField control={form.control} name="catalogId" render={({ field }) => (
            <FormItem>
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
            <FormItem>
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
            <FormItem>
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
            onCollaborationSpecChange={(v) => form.setValue("collaborationSpec", v)}
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

function IntakeFormSection({ serviceId, initialSchema }: { serviceId: number; initialSchema: unknown }) {
  const { t } = useTranslation(["itsm", "common"])
  const queryClient = useQueryClient()
  const canUpdate = usePermission("itsm:service:update")
  const [designerOpen, setDesignerOpen] = useState(false)

  const [schema, setSchema] = useState<FormSchema>(() => {
    const raw = initialSchema as FormSchema | null
    if (raw && Array.isArray(raw.fields)) return raw
    return { version: 1, fields: [] }
  })

  const fieldCount = schema.fields.length

  const saveMut = useMutation({
    mutationFn: () =>
      updateServiceDef(serviceId, { intakeFormSchema: schema.fields.length > 0 ? schema : null } as Partial<ServiceDefItem>),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["itsm-service", serviceId] })
      toast.success(t("itsm:intakeForm.saveSuccess"))
      setDesignerOpen(false)
    },
    onError: (err) => toast.error(err.message),
  })

  return (
    <>
      <SectionFrame
        title={t("itsm:intakeForm.title")}
        action={canUpdate ? (
          <Button variant="outline" size="sm" onClick={() => setDesignerOpen(true)}>
            <Pencil className="mr-1.5 h-3.5 w-3.5" />
            {t("itsm:intakeForm.design")}
          </Button>
        ) : undefined}
      >
        <p className="text-sm text-muted-foreground">
          {fieldCount > 0
            ? t("itsm:intakeForm.fieldCount", { count: fieldCount })
            : t("itsm:intakeForm.noFields")}
        </p>
      </SectionFrame>

      <Sheet open={designerOpen} onOpenChange={setDesignerOpen}>
        <SheetContent className="sm:max-w-4xl p-0 flex flex-col">
          <SheetHeader className="px-6 pt-6 pb-0">
            <SheetTitle>{t("itsm:intakeForm.title")}</SheetTitle>
            <SheetDescription className="sr-only">{t("itsm:intakeForm.title")}</SheetDescription>
          </SheetHeader>
          <div className="flex-1 min-h-0 px-6 py-4">
            <FormDesigner schema={schema} onChange={setSchema} />
          </div>
          <SheetFooter className="px-6 pb-6">
            <Button variant="outline" size="sm" onClick={() => setDesignerOpen(false)}>
              {t("common:cancel")}
            </Button>
            <Button size="sm" onClick={() => saveMut.mutate()} disabled={saveMut.isPending}>
              <Save className="mr-1.5 h-4 w-4" />
              {saveMut.isPending ? t("common:saving") : t("itsm:intakeForm.save")}
            </Button>
          </SheetFooter>
        </SheetContent>
      </Sheet>
    </>
  )
}

// ─── Page Sections ─────────────────────────────────────

function SectionHeader({ title, action }: {
  title: ReactNode
  action?: ReactNode
}) {
  return (
    <div className="mb-3 flex items-center justify-between gap-3">
      <h3 className="text-sm font-semibold text-foreground/82">{title}</h3>
      {action}
    </div>
  )
}

function SectionFrame({ title, action, children }: {
  title: ReactNode
  action?: ReactNode
  children: ReactNode
}) {
  return (
    <section>
      <SectionHeader title={title} action={action} />
      <div className="workspace-surface rounded-[1.25rem] p-5">
        {children}
      </div>
    </section>
  )
}

// ─── Main Page Component ───────────────────────────────

export function Component() {
  const { t } = useTranslation(["itsm", "common"])
  const { id } = useParams<{ id: string }>()
  const navigate = useNavigate()
  const serviceId = Number(id)

  const { data: service, isLoading } = useQuery({
    queryKey: ["itsm-service", serviceId],
    queryFn: () => fetchServiceDef(serviceId),
    enabled: serviceId > 0,
  })

  const { data: catalogs, isLoading: catalogsLoading } = useQuery({
    queryKey: ["itsm-catalogs"],
    queryFn: () => fetchCatalogTree(),
  })

  const { data: slaTemplates, isLoading: slaLoading } = useQuery({
    queryKey: ["itsm-sla"],
    queryFn: () => fetchSLATemplates(),
  })

  if (isLoading || catalogsLoading || slaLoading) {
    return <div className="flex h-96 items-center justify-center"><Loader2 className="h-6 w-6 animate-spin text-muted-foreground" /></div>
  }

  if (!service) {
    return <div className="flex h-96 items-center justify-center text-muted-foreground">Not found</div>
  }

  const workflowSection = (
    <SectionFrame
      title={service.engineType === "smart" ? "参考路径/策略草图" : t("itsm:services.tabWorkflow")}
      action={service.engineType === "classic" && !!service.workflowJson ? (
        <Button variant="outline" size="sm" onClick={() => navigate(`/itsm/services/${serviceId}/workflow`)}>
          <Pencil className="mr-1.5 h-3.5 w-3.5" />{t("itsm:workflow.editWorkflow")}
        </Button>
      ) : undefined}
    >
      {!service.workflowJson ? (
        <div className="flex min-h-32 flex-col items-center justify-center gap-2 rounded-xl border border-dashed border-border/70 bg-background/25 px-4 py-7 text-muted-foreground">
          <p className="text-sm">
            {service.engineType === "smart" ? "暂无参考路径" : t("itsm:services.workflowEmpty")}
          </p>
          {service.engineType === "classic" ? (
            <Button variant="outline" size="sm" onClick={() => navigate(`/itsm/services/${serviceId}/workflow`)}>
              <Pencil className="mr-1.5 h-3.5 w-3.5" />{t("itsm:workflow.designWorkflow")}
            </Button>
          ) : (
            <p className="text-xs">{t("itsm:generate.workflowEmptySmartHint")}</p>
          )}
        </div>
      ) : (
        <Suspense fallback={<div className="flex h-80 items-center justify-center"><Loader2 className="h-6 w-6 animate-spin text-muted-foreground" /></div>}>
          <WorkflowPreview workflowJson={service.workflowJson} />
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
        <BasicInfoForm key={service.updatedAt} service={service} catalogs={catalogs ?? []} slaTemplates={slaTemplates ?? []} />
      </SectionFrame>

      {service.engineType === "smart" ? (
        <div className="grid items-stretch gap-4 xl:grid-cols-[minmax(0,1fr)_minmax(320px,0.38fr)]">
          <div className="min-w-0">{workflowSection}</div>
          <ServiceHealthSection health={service.publishHealthCheck} />
        </div>
      ) : (
        <>
          <IntakeFormSection serviceId={serviceId} initialSchema={service.intakeFormSchema} />
          {workflowSection}
        </>
      )}

      <section className="space-y-3">
        <ActionsSection serviceId={serviceId} />
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
