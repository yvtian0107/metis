"use client"

import { useState, useEffect, useMemo, lazy, Suspense } from "react"
import { useParams, useNavigate } from "react-router"
import { useTranslation } from "react-i18next"
import { useForm } from "react-hook-form"
import { z } from "zod"
import { zodResolver } from "@hookform/resolvers/zod"
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query"
import { ArrowLeft, Plus, Pencil, Trash2, Zap, Save, Loader2 } from "lucide-react"
import { usePermission } from "@/hooks/use-permission"
import { toast } from "sonner"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Textarea } from "@/components/ui/textarea"
import { Badge } from "@/components/ui/badge"
import { Switch } from "@/components/ui/switch"
import {
  Select, SelectContent, SelectItem, SelectTrigger, SelectValue,
} from "@/components/ui/select"
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs"
import {
  DataTableActions, DataTableActionsCell, DataTableActionsHead,
  DataTableCard, DataTableEmptyRow, DataTableLoadingRow,
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
  type ServiceDefItem, type CatalogItem, type ServiceActionItem, type SmartAgentConfig,
  fetchServiceDef, updateServiceDef,
  fetchCatalogTree, fetchSLATemplates,
  fetchServiceActions, createServiceAction, updateServiceAction, deleteServiceAction,
} from "../../../api"
import { SmartServiceConfig } from "../../../components/smart-service-config"

const WorkflowPreview = lazy(() => import("./workflow-preview"))

// ─── Helpers ───────────────────────────────────────────

function flattenCatalogs(nodes: CatalogItem[], depth = 0): Array<CatalogItem & { depth: number }> {
  const result: Array<CatalogItem & { depth: number }> = []
  for (const n of nodes) {
    result.push({ ...n, depth })
    if (n.children?.length) result.push(...flattenCatalogs(n.children, depth + 1))
  }
  return result
}

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
    agentId: z.number().nullable().default(null),
    knowledgeBaseIds: z.array(z.number()).default([]),
    confidenceThreshold: z.number().default(0.8),
    decisionTimeout: z.number().default(30),
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

// ─── Actions Tab ───────────────────────────────────────

function ActionsTab({ serviceId }: { serviceId: number }) {
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
    onSuccess: () => { queryClient.invalidateQueries({ queryKey: ["itsm-service-actions", serviceId] }); setFormOpen(false); toast.success(t("itsm:actions.createSuccess")) },
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
    onSuccess: () => { queryClient.invalidateQueries({ queryKey: ["itsm-service-actions", serviceId] }); setFormOpen(false); toast.success(t("itsm:actions.updateSuccess")) },
    onError: (err) => toast.error(err.message),
  })

  const deleteMut = useMutation({
    mutationFn: (actionId: number) => deleteServiceAction(serviceId, actionId),
    onSuccess: () => { queryClient.invalidateQueries({ queryKey: ["itsm-service-actions", serviceId] }); toast.success(t("itsm:actions.deleteSuccess")) },
    onError: (err) => toast.error(err.message),
  })

  function onSubmit(v: ActionFormValues) { if (editing) { updateMut.mutate(v) } else { createMut.mutate(v) } }
  const isPending = createMut.isPending || updateMut.isPending

  return (
    <div className="space-y-4">
      {canUpdate && (
        <div className="flex justify-end">
          <Button size="sm" onClick={() => { setEditing(null); setFormOpen(true) }}>
            <Plus className="mr-1.5 h-4 w-4" />{t("itsm:actions.create")}
          </Button>
        </div>
      )}

      <DataTableCard>
        <Table>
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
              <DataTableEmptyRow colSpan={4} icon={Zap} title={t("itsm:actions.empty")} description={canUpdate ? t("itsm:actions.emptyHint") : undefined} />
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
    </div>
  )
}

// ─── Main Page Component ───────────────────────────────

export function Component() {
  const { t } = useTranslation(["itsm", "common"])
  const { id } = useParams<{ id: string }>()
  const navigate = useNavigate()
  const queryClient = useQueryClient()
  const serviceId = Number(id)
  const canUpdate = usePermission("itsm:service:update")
  const schema = useBasicInfoSchema()

  const { data: service, isLoading } = useQuery({
    queryKey: ["itsm-service", serviceId],
    queryFn: () => fetchServiceDef(serviceId),
    enabled: serviceId > 0,
  })

  const { data: catalogs = [] } = useQuery({
    queryKey: ["itsm-catalogs"],
    queryFn: () => fetchCatalogTree(),
  })

  const { data: slaTemplates = [] } = useQuery({
    queryKey: ["itsm-sla"],
    queryFn: () => fetchSLATemplates(),
  })

  const flatCatalogs = useMemo(() => flattenCatalogs(catalogs), [catalogs])

  const form = useForm<BasicFormValues>({
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    resolver: zodResolver(schema as any),
    defaultValues: {
      name: "", description: "", catalogId: 0, slaId: null, isActive: true,
      collaborationSpec: "", agentId: null, knowledgeBaseIds: [],
      confidenceThreshold: 0.8, decisionTimeout: 30,
    },
  })

  useEffect(() => {
    if (service) {
      const agentCfg = service.agentConfig as SmartAgentConfig | null
      form.reset({
        name: service.name,
        description: service.description,
        catalogId: service.catalogId,
        slaId: service.slaId,
        isActive: service.isActive,
        collaborationSpec: service.collaborationSpec ?? "",
        agentId: service.agentId ?? null,
        knowledgeBaseIds: service.knowledgeBaseIds ?? [],
        confidenceThreshold: agentCfg?.confidence_threshold ?? 0.8,
        decisionTimeout: agentCfg?.decision_timeout_seconds ?? 30,
      })
    }
  }, [service, form])

  const updateMut = useMutation({
    mutationFn: (v: BasicFormValues) => updateServiceDef(serviceId, {
      name: v.name,
      description: v.description,
      catalogId: v.catalogId,
      slaId: v.slaId,
      isActive: v.isActive,
      collaborationSpec: service?.engineType === "smart" ? v.collaborationSpec : undefined,
      agentId: service?.engineType === "smart" ? v.agentId : undefined,
      knowledgeBaseIds: service?.engineType === "smart" ? v.knowledgeBaseIds : undefined,
      agentConfig: service?.engineType === "smart" ? JSON.stringify({
        confidence_threshold: v.confidenceThreshold,
        decision_timeout_seconds: v.decisionTimeout,
      }) : undefined,
    } as Partial<ServiceDefItem>),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["itsm-service", serviceId] })
      queryClient.invalidateQueries({ queryKey: ["itsm-services"] })
      toast.success(t("itsm:services.updateSuccess"))
    },
    onError: (err) => toast.error(err.message),
  })

  // Early returns after all hooks
  if (isLoading) {
    return <div className="flex h-96 items-center justify-center"><Loader2 className="h-6 w-6 animate-spin text-muted-foreground" /></div>
  }

  if (!service) {
    return <div className="flex h-96 items-center justify-center text-muted-foreground">Not found</div>
  }

  return (
    <div className="space-y-4">
      {/* Header */}
      <div className="flex items-center gap-3">
        <Button variant="ghost" size="icon" onClick={() => navigate("/itsm/services")}>
          <ArrowLeft className="h-4 w-4" />
        </Button>
        <div className="flex-1">
          <div className="flex items-center gap-2">
            <h2 className="text-lg font-semibold">{service.name}</h2>
            <Badge variant={service.engineType === "smart" ? "default" : "outline"}>
              {service.engineType === "smart" ? t("itsm:services.engineSmart") : t("itsm:services.engineClassic")}
            </Badge>
            <Badge variant={service.isActive ? "default" : "secondary"}>
              {service.isActive ? t("itsm:services.active") : t("itsm:services.inactive")}
            </Badge>
          </div>
          <p className="text-sm text-muted-foreground">{service.code}</p>
        </div>
      </div>

      {/* Tabs */}
      <Tabs defaultValue="basic">
        <TabsList variant="line">
          <TabsTrigger value="basic">{t("itsm:services.tabBasicInfo")}</TabsTrigger>
          <TabsTrigger value="workflow">{t("itsm:services.tabWorkflow")}</TabsTrigger>
          <TabsTrigger value="actions">{t("itsm:services.tabActions")}</TabsTrigger>
        </TabsList>

        {/* Basic Info Tab */}
        <TabsContent value="basic" className="mt-4">
          <Form {...form}>
            <form onSubmit={form.handleSubmit((v) => updateMut.mutate(v))} className="max-w-2xl space-y-5">
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

              <FormField control={form.control} name="description" render={({ field }) => (
                <FormItem>
                  <FormLabel>{t("itsm:services.description")}</FormLabel>
                  <FormControl><Textarea rows={3} {...field} /></FormControl>
                  <FormMessage />
                </FormItem>
              )} />

              <FormField control={form.control} name="catalogId" render={({ field }) => (
                <FormItem>
                  <FormLabel>{t("itsm:services.catalog")}</FormLabel>
                  <Select onValueChange={(v) => field.onChange(Number(v))} value={String(field.value)}>
                    <FormControl><SelectTrigger><SelectValue placeholder={t("itsm:services.catalogPlaceholder")} /></SelectTrigger></FormControl>
                    <SelectContent>
                      {flatCatalogs.map((c) => (
                        <SelectItem key={c.id} value={String(c.id)}>{"─".repeat(c.depth)} {c.name}</SelectItem>
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
                    <FormControl><SelectTrigger><SelectValue placeholder={t("itsm:services.slaPlaceholder")} /></SelectTrigger></FormControl>
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

              <div className="space-y-1.5">
                <label className="text-sm font-medium">{t("itsm:services.engineType")}</label>
                <div>
                  <Badge variant={service.engineType === "smart" ? "default" : "outline"}>
                    {service.engineType === "smart" ? t("itsm:services.engineSmart") : t("itsm:services.engineClassic")}
                  </Badge>
                </div>
              </div>

              <FormField control={form.control} name="isActive" render={({ field }) => (
                <FormItem className="flex items-center gap-3">
                  <FormLabel className="mt-0">{t("itsm:services.status")}</FormLabel>
                  <FormControl>
                    <Switch checked={field.value} onCheckedChange={field.onChange} />
                  </FormControl>
                  <span className="text-sm text-muted-foreground">
                    {field.value ? t("itsm:services.active") : t("itsm:services.inactive")}
                  </span>
                </FormItem>
              )} />

              {service.engineType === "smart" && (
                <SmartServiceConfig
                  collaborationSpec={form.watch("collaborationSpec")}
                  onCollaborationSpecChange={(v) => form.setValue("collaborationSpec", v)}
                  agentId={form.watch("agentId")}
                  onAgentIdChange={(v) => form.setValue("agentId", v)}
                  knowledgeBaseIds={form.watch("knowledgeBaseIds")}
                  onKnowledgeBaseIdsChange={(v) => form.setValue("knowledgeBaseIds", v)}
                  confidenceThreshold={form.watch("confidenceThreshold")}
                  onConfidenceThresholdChange={(v) => form.setValue("confidenceThreshold", v)}
                  decisionTimeout={form.watch("decisionTimeout")}
                  onDecisionTimeoutChange={(v) => form.setValue("decisionTimeout", v)}
                />
              )}

              {canUpdate && (
                <Button type="submit" disabled={updateMut.isPending}>
                  <Save className="mr-1.5 h-4 w-4" />
                  {updateMut.isPending ? t("common:saving") : t("common:save")}
                </Button>
              )}
            </form>
          </Form>
        </TabsContent>

        {/* Workflow Tab */}
        <TabsContent value="workflow" className="mt-4">
          {!service.workflowJson ? (
            <div className="flex h-64 flex-col items-center justify-center gap-2 rounded-md border border-dashed text-muted-foreground">
              <p className="text-sm">{t("itsm:services.workflowEmpty")}</p>
              <p className="text-xs">{t("itsm:services.workflowEmptyHint")}</p>
            </div>
          ) : (
            <Suspense fallback={<div className="flex h-96 items-center justify-center"><Loader2 className="h-6 w-6 animate-spin text-muted-foreground" /></div>}>
              <WorkflowPreview workflowJson={service.workflowJson} />
            </Suspense>
          )}
        </TabsContent>

        {/* Actions Tab */}
        <TabsContent value="actions" className="mt-4">
          <ActionsTab serviceId={serviceId} />
        </TabsContent>
      </Tabs>
    </div>
  )
}
