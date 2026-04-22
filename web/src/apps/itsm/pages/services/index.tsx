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
  type CatalogItem, type ServiceDefItem,
  fetchCatalogTree, fetchServiceDefs, createServiceDef, deleteServiceDef,
} from "../../api"
import { CatalogNavPanel } from "../../components/catalog-nav-panel"
import { ServiceCard } from "../../components/service-card"

// ─── Create Schema ─────────────────────────────────────

function useCreateSchema() {
  const { t } = useTranslation("itsm")
  return z.object({
    name: z.string().min(1, t("validation.nameRequired")),
    code: z.string().min(1, t("validation.codeRequired")),
    catalogId: z.number().min(1),
    engineType: z.string().default("smart"),
    description: z.string().optional(),
  })
}

type CreateFormValues = z.infer<ReturnType<typeof useCreateSchema>>

// ─── Helpers ───────────────────────────────────────────

/** Build child→root mapping from catalog tree */
function buildChildToRootMap(catalogs: CatalogItem[]) {
  const map = new Map<number, number>()
  for (const root of catalogs) {
    if (!root.parentId && root.children) {
      for (const child of root.children) {
        map.set(child.id, root.id)
      }
    }
  }
  return map
}

/** Group services by root catalog */
function groupByRoot(
  services: ServiceDefItem[],
  childToRoot: Map<number, number>,
  roots: CatalogItem[],
) {
  const groups = new Map<number, ServiceDefItem[]>()
  for (const root of roots) groups.set(root.id, [])
  for (const svc of services) {
    const rootId = childToRoot.get(svc.catalogId)
    if (rootId !== undefined) {
      const arr = groups.get(rootId)
      if (arr) arr.push(svc)
    }
  }
  return groups
}

// ─── Component ─────────────────────────────────────────

export function Component() {
  const { t } = useTranslation(["itsm", "common"])
  const navigate = useNavigate()
  const queryClient = useQueryClient()
  const [searchParams, setSearchParams] = useSearchParams()
  const [createOpen, setCreateOpen] = useState(false)
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
    if (id === null) {
      setSearchParams({})
    } else {
      setSearchParams({ catalog: String(id) })
    }
  }

  // ── Data fetching ────────────────────────────────────

  const { data: catalogs = [] } = useQuery({
    queryKey: ["itsm-catalogs"],
    queryFn: () => fetchCatalogTree(),
  })

  const { data: servicesData, isLoading: servicesLoading } = useQuery({
    queryKey: ["itsm-services-all"],
    queryFn: () => fetchServiceDefs({ pageSize: 100 }),
  })

  const allServices = useMemo(() => servicesData?.items ?? [], [servicesData])

  // ── Derived data ─────────────────────────────────────

  const roots = useMemo(() => catalogs.filter((n) => !n.parentId), [catalogs])
  const childToRoot = useMemo(() => buildChildToRootMap(catalogs), [catalogs])

  const filteredServices = useMemo(() => {
    if (selectedCatalogId === null) return allServices
    return allServices.filter((s) => s.catalogId === selectedCatalogId)
  }, [allServices, selectedCatalogId])

  const groupedServices = useMemo(
    () => groupByRoot(allServices, childToRoot, roots),
    [allServices, childToRoot, roots],
  )

  // ── Delete service ───────────────────────────────────

  const deleteMut = useMutation({
    mutationFn: (id: number) => deleteServiceDef(id),
    onSuccess: () => { queryClient.invalidateQueries({ queryKey: ["itsm-services-all"] }); toast.success(t("itsm:services.deleteSuccess")) },
    onError: (err) => toast.error(err.message),
  })

  // ── Create service form ──────────────────────────────

  const createForm = useForm<CreateFormValues>({
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    resolver: zodResolver(createSchema as any),
    defaultValues: { name: "", code: "", catalogId: 0, engineType: "smart", description: "" },
  })

  useEffect(() => {
    if (createOpen) {
      createForm.reset({
        name: "", code: "",
        catalogId: selectedCatalogId ?? 0,
        engineType: "smart", description: "",
      })
    }
  }, [createOpen, selectedCatalogId, createForm])

  const createMut = useMutation({
    mutationFn: (v: CreateFormValues) => createServiceDef({
      ...v, description: v.description ?? "",
    }),
    onSuccess: (data) => {
      toast.success(t("itsm:services.createSuccess"))
      setCreateOpen(false)
      queryClient.invalidateQueries({ queryKey: ["itsm-services-all"] })
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
          services={allServices}
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
          ) : selectedCatalogId === null ? (
            roots.length === 0 ? (
              <EmptyState
                icon={Cog}
                title={t("itsm:services.empty")}
                description={canCreateService ? t("itsm:services.emptyHint") : undefined}
                action={canCreateService ? () => setCreateOpen(true) : undefined}
                actionLabel={t("itsm:services.create")}
              />
            ) : (
              <div className="space-y-6">
                {roots.map((root) => {
                  const group = groupedServices.get(root.id) ?? []
                  if (group.length === 0) return null
                  return (
                    <div key={root.id}>
                      <div className="mb-3 flex items-center gap-3 border-b border-border/45 pb-2">
                        <h3 className="text-sm font-semibold text-foreground/82">{root.name}</h3>
                        <span className="text-xs text-muted-foreground">{group.length}</span>
                      </div>
                      <div className="grid grid-cols-[repeat(auto-fill,minmax(260px,1fr))] gap-4">
                        {group.map((svc) => (
                          <ServiceCard
                            key={svc.id}
                            service={svc}
                            canUpdate={canUpdateService}
                            canDelete={canDeleteService}
                            onDelete={(id) => deleteMut.mutate(id)}
                          />
                        ))}
                      </div>
                    </div>
                  )
                })}
                {allServices.length === 0 && (
                  <EmptyState
                    icon={Cog}
                    title={t("itsm:services.empty")}
                    description={canCreateService ? t("itsm:services.emptyHint") : undefined}
                    action={canCreateService ? () => setCreateOpen(true) : undefined}
                    actionLabel={t("itsm:services.create")}
                  />
                )}
              </div>
            )
          ) : (
            /* Filtered view: flat grid */
            filteredServices.length === 0 ? (
              <EmptyState
                icon={Cog}
                title={t("itsm:services.emptyInCatalog")}
                description={canCreateService ? t("itsm:services.emptyInCatalogHint") : undefined}
                action={canCreateService ? () => setCreateOpen(true) : undefined}
                actionLabel={t("itsm:services.addService")}
              />
            ) : (
              <div className="grid grid-cols-[repeat(auto-fill,minmax(260px,1fr))] gap-4">
                {filteredServices.map((svc) => (
                  <ServiceCard
                    key={svc.id}
                    service={svc}
                    canUpdate={canUpdateService}
                    canDelete={canDeleteService}
                    onDelete={(id) => deleteMut.mutate(id)}
                  />
                ))}
              </div>
            )
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
