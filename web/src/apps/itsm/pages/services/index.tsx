"use client"

import { useMemo, useState, useEffect } from "react"
import { useTranslation } from "react-i18next"
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query"
import { useNavigate, useSearchParams } from "react-router"
import { useForm } from "react-hook-form"
import { z } from "zod"
import { zodResolver } from "@hookform/resolvers/zod"
import { Plus, Cog } from "lucide-react"
import { usePermission } from "@/hooks/use-permission"
import { toast } from "sonner"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Textarea } from "@/components/ui/textarea"
import {
  Select, SelectContent, SelectGroup, SelectItem, SelectLabel, SelectTrigger, SelectValue,
} from "@/components/ui/select"
import {
  Sheet, SheetContent, SheetHeader, SheetTitle, SheetDescription, SheetFooter,
} from "@/components/ui/sheet"
import {
  Form, FormControl, FormField, FormItem, FormLabel, FormMessage,
} from "@/components/ui/form"
import {
  fetchCatalogTree, fetchCatalogServiceCounts, fetchServiceDefs, createServiceDef, deleteServiceDef,
  type CatalogItem, type ServiceDefItem,
} from "../../api"
import { CatalogNavPanel } from "../../components/catalog-nav-panel"
import { ServiceCard } from "../../components/service-card"
import { itsmQueryKeys } from "../../query-keys"
import { getCatalogSelection, getCreateServiceCatalogDefault } from "./service-catalog-state"

// ─── Create Schema ─────────────────────────────────────

function useCreateSchema() {
  const { t } = useTranslation("itsm")
  return z.object({
    name: z.string().min(1, t("validation.nameRequired")),
    code: z.string().min(1, t("validation.codeRequired")),
    catalogId: z.number().min(1),
    engineType: z.enum(["classic", "smart"]),
    description: z.string().optional(),
  })
}

type CreateFormValues = z.infer<ReturnType<typeof useCreateSchema>>

// ─── Helpers ───────────────────────────────────────────

// ─── Component ─────────────────────────────────────────

export function Component() {
  const { t } = useTranslation(["itsm", "common"])
  const navigate = useNavigate()
  const queryClient = useQueryClient()
  const [searchParams, setSearchParams] = useSearchParams()
  const [createOpen, setCreateOpen] = useState(false)
  const [page, setPage] = useState(1)
  const createSchema = useCreateSchema()

  const canCreateService = usePermission("itsm:service:create")
  const canUpdateService = usePermission("itsm:service:update")
  const canDeleteService = usePermission("itsm:service:delete")
  const canCreateCatalog = usePermission("itsm:catalog:create")
  const canUpdateCatalog = usePermission("itsm:catalog:update")
  const canDeleteCatalog = usePermission("itsm:catalog:delete")

  // Parse selected catalog from URL
  const catalogParam = searchParams.get("catalog")
  const selectedCatalogId = catalogParam ? Number(catalogParam) : null

  function handleSelectCatalog(id: number | null) {
    setPage(1)
    if (id === null) {
      setSearchParams({})
    } else {
      setSearchParams({ catalog: String(id) })
    }
  }

  // ── Data fetching ────────────────────────────────────

  const { data: catalogs = [] } = useQuery({
    queryKey: itsmQueryKeys.catalogs.tree(),
    queryFn: () => fetchCatalogTree(),
  })

  const { data: serviceCounts } = useQuery({
    queryKey: itsmQueryKeys.catalogs.serviceCounts(),
    queryFn: () => fetchCatalogServiceCounts(),
  })

  const serviceListParams = useMemo(() => ({
    ...getCatalogSelection(catalogs, selectedCatalogId),
    page,
    pageSize: 24,
  }), [catalogs, page, selectedCatalogId])

  const { data: servicesData, isLoading: servicesLoading } = useQuery({
    queryKey: itsmQueryKeys.services.list(serviceListParams),
    queryFn: () => fetchServiceDefs(serviceListParams),
  })

  const allServices = useMemo(() => servicesData?.items ?? [], [servicesData])
  const totalServices = servicesData?.total ?? 0
  const totalPages = Math.max(1, Math.ceil(totalServices / serviceListParams.pageSize))

  // ── Derived data ─────────────────────────────────────

  const roots = useMemo(() => catalogs.filter((n) => !n.parentId), [catalogs])
  const catalogNames = useMemo(() => {
    const names = new Map<number, string>()
    for (const root of catalogs) {
      names.set(root.id, root.name)
      for (const child of root.children ?? []) {
        names.set(child.id, `${root.name} / ${child.name}`)
      }
    }
    return names
  }, [catalogs])

  const allServiceGroups = useMemo(() => {
    const groups = new Map<number, { catalog: CatalogItem; services: ServiceDefItem[] }>()
    const rootByCatalogId = new Map<number, CatalogItem>()

    for (const root of catalogs) {
      rootByCatalogId.set(root.id, root)
      for (const child of root.children ?? []) {
        rootByCatalogId.set(child.id, root)
      }
    }

    for (const service of allServices) {
      const root = rootByCatalogId.get(service.catalogId)
      if (!root) continue
      const group = groups.get(root.id)
      if (group) {
        group.services.push(service)
      } else {
        groups.set(root.id, { catalog: root, services: [service] })
      }
    }

    return roots
      .map((root) => groups.get(root.id))
      .filter((group): group is { catalog: CatalogItem; services: ServiceDefItem[] } => Boolean(group))
  }, [allServices, catalogs, roots])

  // ── Delete service ───────────────────────────────────

  const deleteMut = useMutation({
    mutationFn: (id: number) => deleteServiceDef(id),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: itsmQueryKeys.services.lists() })
      queryClient.invalidateQueries({ queryKey: itsmQueryKeys.catalogs.serviceCounts() })
      toast.success(t("itsm:services.deleteSuccess"))
    },
    onError: (err) => toast.error(err.message),
  })

  // ── Create service form ──────────────────────────────

  const createForm = useForm<CreateFormValues>({
    resolver: zodResolver(createSchema),
    defaultValues: { name: "", code: "", catalogId: 0, engineType: "smart", description: "" },
  })

  useEffect(() => {
    if (createOpen) {
      createForm.reset({
        name: "", code: "",
        catalogId: getCreateServiceCatalogDefault(catalogs, selectedCatalogId),
        engineType: "smart", description: "",
      })
    }
  }, [catalogs, createOpen, selectedCatalogId, createForm])

  const createMut = useMutation({
    mutationFn: (v: CreateFormValues) => createServiceDef({
      ...v, description: v.description ?? "",
    }),
    onSuccess: (data) => {
      toast.success(t("itsm:services.createSuccess"))
      setCreateOpen(false)
      queryClient.invalidateQueries({ queryKey: itsmQueryKeys.services.lists() })
      queryClient.invalidateQueries({ queryKey: itsmQueryKeys.catalogs.serviceCounts() })
      navigate(`/itsm/services/${data.id}`)
    },
    onError: (err) => toast.error(err.message),
  })

  // ── Render ───────────────────────────────────────────

  return (
    <div className="workspace-page flex h-[calc(100vh-theme(spacing.14)-theme(spacing.12))] flex-col">
      <div className="workspace-page-header shrink-0">
        <div>
          <h2 className="workspace-page-title">{t("itsm:services.title")}</h2>
        </div>
        {canCreateService && (
          <Button size="sm" onClick={() => setCreateOpen(true)}>
            <Plus className="mr-1.5 h-4 w-4" />{t("itsm:services.create")}
          </Button>
        )}
      </div>

      <div className="flex min-h-0 flex-1 gap-4">
          <CatalogNavPanel
            catalogs={catalogs}
            serviceCounts={serviceCounts}
            selectedCatalogId={selectedCatalogId}
          onSelect={handleSelectCatalog}
          canCreate={canCreateCatalog}
          canUpdate={canUpdateCatalog}
          canDelete={canDeleteCatalog}
        />

        <div className="min-w-0 flex-1 overflow-y-auto pr-1">
          {servicesLoading ? (
            <div className="workspace-surface flex h-48 items-center justify-center rounded-[1.25rem]">
              <div className="h-6 w-6 animate-spin rounded-full border-2 border-muted-foreground/25 border-t-primary/70" />
            </div>
          ) : roots.length === 0 ? (
            <EmptyState
              icon={Cog}
              title={t("itsm:services.empty")}
              description={canCreateService ? t("itsm:services.emptyHint") : undefined}
              action={canCreateService ? () => setCreateOpen(true) : undefined}
              actionLabel={t("itsm:services.create")}
            />
          ) : allServices.length === 0 ? (
            <EmptyState
              icon={Cog}
              title={selectedCatalogId === null ? t("itsm:services.empty") : t("itsm:services.emptyInCatalog")}
              description={canCreateService ? selectedCatalogId === null ? t("itsm:services.emptyHint") : t("itsm:services.emptyInCatalogHint") : undefined}
              action={canCreateService ? () => setCreateOpen(true) : undefined}
              actionLabel={selectedCatalogId === null ? t("itsm:services.create") : t("itsm:services.addService")}
            />
          ) : (
            <div className="space-y-3">
              {selectedCatalogId === null ? (
                <div className="space-y-5">
                  {allServiceGroups.map((group) => (
                    <section key={group.catalog.id} className="space-y-3">
                      <button
                        type="button"
                        onClick={() => handleSelectCatalog(group.catalog.id)}
                        className="flex w-full items-center border-b border-border/35 pb-2 text-left transition-colors hover:border-border/60"
                      >
                        <span className="text-[13px] font-semibold tracking-[-0.01em] text-foreground">
                          {group.catalog.name}
                        </span>
                      </button>
                      <div className="grid grid-cols-[repeat(auto-fill,minmax(260px,1fr))] gap-4">
                        {group.services.map((svc) => (
                          <ServiceCard
                            key={svc.id}
                            service={svc}
                            catalogName={catalogNames.get(svc.catalogId)}
                            canUpdate={canUpdateService}
                            canDelete={canDeleteService}
                            onDelete={(id) => deleteMut.mutate(id)}
                          />
                        ))}
                      </div>
                    </section>
                  ))}
                </div>
              ) : (
                <div className="grid grid-cols-[repeat(auto-fill,minmax(260px,1fr))] gap-4">
                  {allServices.map((svc) => (
                    <ServiceCard
                      key={svc.id}
                      service={svc}
                      catalogName={catalogNames.get(svc.catalogId)}
                      canUpdate={canUpdateService}
                      canDelete={canDeleteService}
                      onDelete={(id) => deleteMut.mutate(id)}
                    />
                  ))}
                </div>
              )}
              {totalPages > 1 && (
                <div className="flex justify-end gap-2 pt-2">
                  <Button variant="outline" size="sm" disabled={page <= 1} onClick={() => setPage((v) => Math.max(1, v - 1))}>
                    {t("common:previous")}
                  </Button>
                  <Button variant="outline" size="sm" disabled={page >= totalPages} onClick={() => setPage((v) => Math.min(totalPages, v + 1))}>
                    {t("common:next")}
                  </Button>
                </div>
              )}
            </div>
          )}
        </div>
      </div>

      {/* Create Service Sheet */}
      <Sheet open={createOpen} onOpenChange={setCreateOpen}>
        <SheetContent className="sm:max-w-lg">
          <SheetHeader>
            <SheetTitle>{t("itsm:services.create")}</SheetTitle>
            <SheetDescription className="sr-only">{t("itsm:services.create")}</SheetDescription>
          </SheetHeader>
          <Form {...createForm}>
            <form onSubmit={createForm.handleSubmit((v) => createMut.mutate(v))} className="flex flex-1 flex-col gap-5 px-4">
              <FormField control={createForm.control} name="name" render={({ field }) => (
                <FormItem>
                  <FormLabel>{t("itsm:services.name")}</FormLabel>
                  <FormControl><Input placeholder={t("itsm:services.namePlaceholder")} {...field} /></FormControl>
                  <FormMessage />
                </FormItem>
              )} />
              <FormField control={createForm.control} name="code" render={({ field }) => (
                <FormItem>
                  <FormLabel>{t("itsm:services.code")}</FormLabel>
                  <FormControl><Input placeholder={t("itsm:services.codePlaceholder")} {...field} /></FormControl>
                  <FormMessage />
                </FormItem>
              )} />
              <FormField control={createForm.control} name="catalogId" render={({ field }) => (
                <FormItem>
                  <FormLabel>{t("itsm:services.catalog")}</FormLabel>
                  <Select onValueChange={(v) => field.onChange(Number(v))} value={field.value ? String(field.value) : undefined}>
                    <FormControl><SelectTrigger><SelectValue placeholder={t("itsm:services.catalogPlaceholder")} /></SelectTrigger></FormControl>
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
              <FormField control={createForm.control} name="engineType" render={({ field }) => (
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
              <FormField control={createForm.control} name="description" render={({ field }) => (
                <FormItem>
                  <FormLabel>{t("itsm:services.description")}</FormLabel>
                  <FormControl><Textarea rows={3} {...field} /></FormControl>
                  <FormMessage />
                </FormItem>
              )} />
              <SheetFooter>
                <Button type="submit" size="sm" disabled={createMut.isPending}>
                  {createMut.isPending ? t("common:saving") : t("common:create")}
                </Button>
              </SheetFooter>
            </form>
          </Form>
        </SheetContent>
      </Sheet>
    </div>
  )
}

// ─── EmptyState ──────────────────────────────────────────

function EmptyState({ icon: Icon, title, description, action, actionLabel }: {
  icon: React.ComponentType<{ className?: string }>
  title: string
  description?: string
  action?: () => void
  actionLabel?: string
}) {
  return (
    <div className="workspace-surface flex min-h-[260px] flex-col items-center justify-center rounded-[1.25rem] px-6 py-14 text-center">
      <div className="mb-4 flex h-12 w-12 items-center justify-center rounded-2xl border border-border/55 bg-background/45">
        <Icon className="h-5 w-5 text-muted-foreground/65" />
      </div>
      <h3 className="text-base font-semibold tracking-[-0.01em]">{title}</h3>
      {description && <p className="mt-1 max-w-sm text-sm text-muted-foreground">{description}</p>}
      {action && actionLabel && (
        <Button className="mt-4" size="sm" onClick={action}>
          <Plus className="mr-1.5 h-4 w-4" />{actionLabel}
        </Button>
      )}
    </div>
  )
}
