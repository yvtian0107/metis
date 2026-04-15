"use client"

import { useState, useEffect, useMemo } from "react"
import { useTranslation } from "react-i18next"
import { useForm } from "react-hook-form"
import { z } from "zod"
import { zodResolver } from "@hookform/resolvers/zod"
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query"
import { useNavigate } from "react-router"
import { Plus, Search, Pencil, Trash2, Cog, Eye } from "lucide-react"
import { usePermission } from "@/hooks/use-permission"
import { useListPage } from "@/hooks/use-list-page"
import { toast } from "sonner"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Textarea } from "@/components/ui/textarea"
import { Badge } from "@/components/ui/badge"
import {
  Select, SelectContent, SelectItem, SelectTrigger, SelectValue,
} from "@/components/ui/select"
import {
  DataTableActions, DataTableActionsCell, DataTableActionsHead,
  DataTableCard, DataTableEmptyRow, DataTableLoadingRow,
  DataTablePagination, DataTableToolbar, DataTableToolbarGroup,
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
  type ServiceDefItem, type CatalogItem, type SmartAgentConfig,
  fetchCatalogTree, fetchSLATemplates,
  createServiceDef, updateServiceDef, deleteServiceDef,
} from "../../api"
import { SmartServiceConfig } from "../../components/smart-service-config"

function useServiceSchema() {
  const { t } = useTranslation("itsm")
  return z.object({
    name: z.string().min(1, t("validation.nameRequired")),
    code: z.string().min(1, t("validation.codeRequired")),
    catalogId: z.number().min(1),
    engineType: z.string().default("classic"),
    slaId: z.number().nullable(),
    description: z.string().optional(),
    sortOrder: z.number().default(0),
    collaborationSpec: z.string().default(""),
    agentId: z.number().nullable().default(null),
    knowledgeBaseIds: z.array(z.number()).default([]),
    confidenceThreshold: z.number().default(0.8),
    decisionTimeout: z.number().default(30),
  })
}

type FormValues = z.infer<ReturnType<typeof useServiceSchema>>

function flattenCatalogs(nodes: CatalogItem[], depth = 0): Array<CatalogItem & { depth: number }> {
  const result: Array<CatalogItem & { depth: number }> = []
  for (const n of nodes) {
    result.push({ ...n, depth })
    if (n.children?.length) result.push(...flattenCatalogs(n.children, depth + 1))
  }
  return result
}

export function Component() {
  const { t } = useTranslation(["itsm", "common"])
  const navigate = useNavigate()
  const queryClient = useQueryClient()
  const [formOpen, setFormOpen] = useState(false)
  const [editing, setEditing] = useState<ServiceDefItem | null>(null)
  const [catalogFilter, setCatalogFilter] = useState("")
  const schema = useServiceSchema()

  const canCreate = usePermission("itsm:service:create")
  const canUpdate = usePermission("itsm:service:update")
  const canDelete = usePermission("itsm:service:delete")

  const extraParams = useMemo(() => {
    const params: Record<string, string> = {}
    if (catalogFilter) params.catalogId = catalogFilter
    return params
  }, [catalogFilter])

  const {
    keyword, setKeyword, page, setPage,
    items, total, totalPages, isLoading, handleSearch,
  } = useListPage<ServiceDefItem>({
    queryKey: "itsm-services",
    endpoint: "/api/v1/itsm/services",
    extraParams,
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

  const form = useForm<FormValues>({
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    resolver: zodResolver(schema as any),
    defaultValues: { name: "", code: "", catalogId: 0, engineType: "classic", slaId: null, description: "", sortOrder: 0, collaborationSpec: "", agentId: null, knowledgeBaseIds: [], confidenceThreshold: 0.8, decisionTimeout: 30 },
  })

  useEffect(() => {
    if (formOpen) {
      if (editing) {
        const agentCfg = editing.agentConfig as SmartAgentConfig | null
        form.reset({
          name: editing.name, code: editing.code, catalogId: editing.catalogId,
          engineType: editing.engineType, slaId: editing.slaId, description: editing.description,
          sortOrder: editing.sortOrder,
          collaborationSpec: editing.collaborationSpec ?? "",
          agentId: editing.agentId ?? null,
          knowledgeBaseIds: editing.knowledgeBaseIds ?? [],
          confidenceThreshold: agentCfg?.confidence_threshold ?? 0.8,
          decisionTimeout: agentCfg?.decision_timeout_seconds ?? 30,
        })
      } else {
        form.reset({
          name: "", code: "", catalogId: flatCatalogs[0]?.id ?? 0, engineType: "classic",
          slaId: null, description: "", sortOrder: 0,
          collaborationSpec: "", agentId: null, knowledgeBaseIds: [],
          confidenceThreshold: 0.8, decisionTimeout: 30,
        })
      }
    }
  }, [formOpen, editing, form, flatCatalogs])

  const createMut = useMutation({
    mutationFn: (v: FormValues) => createServiceDef({
      ...v,
      description: v.description ?? "",
      collaborationSpec: v.engineType === "smart" ? v.collaborationSpec : undefined,
      agentId: v.engineType === "smart" ? v.agentId : undefined,
      knowledgeBaseIds: v.engineType === "smart" ? v.knowledgeBaseIds : undefined,
      agentConfig: v.engineType === "smart" ? JSON.stringify({
        confidence_threshold: v.confidenceThreshold,
        decision_timeout_seconds: v.decisionTimeout,
      }) : undefined,
    } as Parameters<typeof createServiceDef>[0]),
    onSuccess: () => { queryClient.invalidateQueries({ queryKey: ["itsm-services"] }); setFormOpen(false); toast.success(t("itsm:services.createSuccess")) },
    onError: (err) => toast.error(err.message),
  })

  const updateMut = useMutation({
    mutationFn: (v: FormValues) => updateServiceDef(editing!.id, {
      ...v,
      description: v.description ?? "",
      collaborationSpec: v.engineType === "smart" ? v.collaborationSpec : undefined,
      agentId: v.engineType === "smart" ? v.agentId : undefined,
      knowledgeBaseIds: v.engineType === "smart" ? v.knowledgeBaseIds : undefined,
      agentConfig: v.engineType === "smart" ? JSON.stringify({
        confidence_threshold: v.confidenceThreshold,
        decision_timeout_seconds: v.decisionTimeout,
      }) : undefined,
    } as Partial<ServiceDefItem>),
    onSuccess: () => { queryClient.invalidateQueries({ queryKey: ["itsm-services"] }); setFormOpen(false); toast.success(t("itsm:services.updateSuccess")) },
    onError: (err) => toast.error(err.message),
  })

  const deleteMut = useMutation({
    mutationFn: (id: number) => deleteServiceDef(id),
    onSuccess: () => { queryClient.invalidateQueries({ queryKey: ["itsm-services"] }); toast.success(t("itsm:services.deleteSuccess")) },
    onError: (err) => toast.error(err.message),
  })

  function onSubmit(v: FormValues) { if (editing) { updateMut.mutate(v) } else { createMut.mutate(v) } }
  const isPending = createMut.isPending || updateMut.isPending

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <h2 className="text-lg font-semibold">{t("itsm:services.title")}</h2>
        {canCreate && (
          <Button onClick={() => { setEditing(null); setFormOpen(true) }}>
            <Plus className="mr-1.5 h-4 w-4" />{t("itsm:services.create")}
          </Button>
        )}
      </div>

      <DataTableToolbar>
        <DataTableToolbarGroup>
          <form onSubmit={handleSearch} className="flex w-full flex-col gap-2 sm:flex-row sm:items-center">
            <div className="relative w-full sm:max-w-sm">
              <Search className="absolute left-2.5 top-2.5 h-4 w-4 text-muted-foreground" />
              <Input placeholder={t("itsm:services.searchPlaceholder")} value={keyword} onChange={(e) => setKeyword(e.target.value)} className="pl-8" />
            </div>
            <Select value={catalogFilter} onValueChange={(v) => { setCatalogFilter(v === "all" ? "" : v); setPage(1) }}>
              <SelectTrigger className="w-[180px]"><SelectValue placeholder={t("itsm:services.allCatalogs")} /></SelectTrigger>
              <SelectContent>
                <SelectItem value="all">{t("itsm:services.allCatalogs")}</SelectItem>
                {flatCatalogs.map((c) => (
                  <SelectItem key={c.id} value={String(c.id)}>{"─".repeat(c.depth)} {c.name}</SelectItem>
                ))}
              </SelectContent>
            </Select>
            <Button type="submit" variant="outline" size="sm">{t("common:search")}</Button>
          </form>
        </DataTableToolbarGroup>
      </DataTableToolbar>

      <DataTableCard>
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead className="min-w-[180px]">{t("itsm:services.name")}</TableHead>
              <TableHead className="w-[120px]">{t("itsm:services.code")}</TableHead>
              <TableHead className="w-[100px]">{t("itsm:services.engineType")}</TableHead>
              <TableHead className="w-[80px]">{t("common:status")}</TableHead>
              <DataTableActionsHead className="min-w-[140px]">{t("common:actions")}</DataTableActionsHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {isLoading ? (
              <DataTableLoadingRow colSpan={5} />
            ) : items.length === 0 ? (
              <DataTableEmptyRow colSpan={5} icon={Cog} title={t("itsm:services.empty")} description={canCreate ? t("itsm:services.emptyHint") : undefined} />
            ) : (
              items.map((item) => (
                <TableRow key={item.id}>
                  <TableCell className="font-medium">{item.name}</TableCell>
                  <TableCell className="text-sm text-muted-foreground">{item.code}</TableCell>
                  <TableCell>
                    <Badge variant={item.engineType === "smart" ? "default" : "outline"}>
                      {item.engineType === "smart" ? t("itsm:services.engineSmart") : t("itsm:services.engineClassic")}
                    </Badge>
                  </TableCell>
                  <TableCell>
                    <Badge variant={item.isActive ? "default" : "secondary"}>
                      {item.isActive ? t("itsm:services.active") : t("itsm:services.inactive")}
                    </Badge>
                  </TableCell>
                  <DataTableActionsCell>
                    <DataTableActions>
                      <Button variant="ghost" size="sm" className="px-2.5" onClick={() => navigate(`/itsm/services/${item.id}`)}>
                        <Eye className="mr-1 h-3.5 w-3.5" />{t("itsm:services.view")}
                      </Button>
                      {canUpdate && (
                        <Button variant="ghost" size="sm" className="px-2.5" onClick={() => { setEditing(item); setFormOpen(true) }}>
                          <Pencil className="mr-1 h-3.5 w-3.5" />{t("common:edit")}
                        </Button>
                      )}
                      {canDelete && (
                        <AlertDialog>
                          <AlertDialogTrigger asChild>
                            <Button variant="ghost" size="sm" className="px-2.5 text-destructive hover:text-destructive">
                              <Trash2 className="mr-1 h-3.5 w-3.5" />{t("common:delete")}
                            </Button>
                          </AlertDialogTrigger>
                          <AlertDialogContent>
                            <AlertDialogHeader>
                              <AlertDialogTitle>{t("itsm:services.deleteTitle")}</AlertDialogTitle>
                              <AlertDialogDescription>{t("itsm:services.deleteDesc", { name: item.name })}</AlertDialogDescription>
                            </AlertDialogHeader>
                            <AlertDialogFooter>
                              <AlertDialogCancel size="sm">{t("common:cancel")}</AlertDialogCancel>
                              <AlertDialogAction size="sm" onClick={() => deleteMut.mutate(item.id)} disabled={deleteMut.isPending}>{t("itsm:services.confirmDelete")}</AlertDialogAction>
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

      <DataTablePagination total={total} page={page} totalPages={totalPages} onPageChange={setPage} />

      <Sheet open={formOpen} onOpenChange={setFormOpen}>
        <SheetContent className="sm:max-w-lg">
          <SheetHeader>
            <SheetTitle>{editing ? t("itsm:services.edit") : t("itsm:services.create")}</SheetTitle>
            <SheetDescription className="sr-only">{editing ? t("itsm:services.edit") : t("itsm:services.create")}</SheetDescription>
          </SheetHeader>
          <Form {...form}>
            <form onSubmit={form.handleSubmit(onSubmit)} className="flex flex-1 flex-col gap-5 px-4">
              <FormField control={form.control} name="name" render={({ field }) => (
                <FormItem>
                  <FormLabel>{t("itsm:services.name")}</FormLabel>
                  <FormControl><Input placeholder={t("itsm:services.namePlaceholder")} {...field} /></FormControl>
                  <FormMessage />
                </FormItem>
              )} />
              <FormField control={form.control} name="code" render={({ field }) => (
                <FormItem>
                  <FormLabel>{t("itsm:services.code")}</FormLabel>
                  <FormControl><Input placeholder={t("itsm:services.codePlaceholder")} {...field} /></FormControl>
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
              <FormField control={form.control} name="engineType" render={({ field }) => (
                <FormItem>
                  <FormLabel>{t("itsm:services.engineType")}</FormLabel>
                  <Select onValueChange={field.onChange} value={field.value}>
                    <FormControl><SelectTrigger><SelectValue /></SelectTrigger></FormControl>
                    <SelectContent>
                      <SelectItem value="classic">{t("itsm:services.engineClassic")}</SelectItem>
                      <SelectItem value="smart">{t("itsm:services.engineSmart")}</SelectItem>
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
              <FormField control={form.control} name="description" render={({ field }) => (
                <FormItem>
                  <FormLabel>{t("itsm:services.description")}</FormLabel>
                  <FormControl><Textarea rows={3} {...field} /></FormControl>
                  <FormMessage />
                </FormItem>
              )} />
              {form.watch("engineType") === "smart" && (
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
