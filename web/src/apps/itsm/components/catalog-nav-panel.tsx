import { useState, useEffect, useMemo } from "react"
import { useTranslation } from "react-i18next"
import { useForm } from "react-hook-form"
import { z } from "zod"
import { zodResolver } from "@hookform/resolvers/zod"
import { useMutation, useQueryClient } from "@tanstack/react-query"
import {
  Plus, Pencil, Trash2, FolderTree, MoreHorizontal,
  ShieldCheck, Monitor, Globe, Container, ShieldAlert, Bell,
  User, Lock, KeyRound, LayoutGrid, Video, Server, Database,
  Bug, FileSearch, LineChart, BellRing, Clock, ChevronsUpDown,
  type LucideIcon,
} from "lucide-react"
import { cn } from "@/lib/utils"
import { toast } from "sonner"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Textarea } from "@/components/ui/textarea"
import {
  DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuSeparator, DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu"
import {
  AlertDialog, AlertDialogAction, AlertDialogCancel, AlertDialogContent,
  AlertDialogDescription, AlertDialogFooter, AlertDialogHeader, AlertDialogTitle,
} from "@/components/ui/alert-dialog"
import {
  Sheet, SheetContent, SheetHeader, SheetTitle, SheetDescription, SheetFooter,
} from "@/components/ui/sheet"
import {
  Popover, PopoverContent, PopoverTrigger,
} from "@/components/ui/popover"
import {
  Form, FormControl, FormField, FormItem, FormLabel, FormMessage,
} from "@/components/ui/form"
import {
  type CatalogItem, type CatalogServiceCounts,
  createCatalog, updateCatalog, deleteCatalog,
} from "../api"
import { itsmQueryKeys } from "../query-keys"
import { getDisplayedCatalogCount } from "../pages/services/service-catalog-state"

// ─── Icon mapping ──────────────────────────────────────

const iconMap: Record<string, LucideIcon> = {
  ShieldCheck, Monitor, Globe, Container, ShieldAlert, Bell,
  User, Lock, KeyRound, LayoutGrid, Video, Server, Database,
  Bug, FileSearch, LineChart, BellRing, Clock, FolderTree,
}

function CatalogIcon({ name, className }: { name?: string; className?: string }) {
  const Icon = (name && iconMap[name]) || FolderTree
  return <Icon className={className} />
}

function CatalogCount({ value, className }: { value: number; className?: string }) {
  return (
    <span className={cn(
      "ml-2 min-w-7 shrink-0 text-right font-mono text-[11px] leading-none tabular-nums text-muted-foreground/65",
      className,
    )}>
      {value}
    </span>
  )
}

// ─── Schema ────────────────────────────────────────────

function useCatalogSchema() {
  const { t } = useTranslation("itsm")
  return z.object({
    name: z.string().min(1, t("validation.nameRequired")),
    code: z.string().min(1, t("validation.codeRequired")),
    parentId: z.number().nullable(),
    sortOrder: z.number().default(0),
    icon: z.string().optional(),
    description: z.string().optional(),
  })
}

type FormValues = z.infer<ReturnType<typeof useCatalogSchema>>

// ─── Props ─────────────────────────────────────────────

interface CatalogNavPanelProps {
  catalogs: CatalogItem[]
  serviceCounts?: CatalogServiceCounts
  selectedCatalogId: number | null // null = "全部"
  onSelect: (catalogId: number | null) => void
  canCreate: boolean
  canUpdate: boolean
  canDelete: boolean
}

// ─── Component ─────────────────────────────────────────

export function CatalogNavPanel({
  catalogs, serviceCounts, selectedCatalogId,
  onSelect, canCreate, canUpdate, canDelete,
}: CatalogNavPanelProps) {
  const { t } = useTranslation(["itsm", "common"])
  const queryClient = useQueryClient()
  const [formOpen, setFormOpen] = useState(false)
  const [editing, setEditing] = useState<CatalogItem | null>(null)
  const [formParentId, setFormParentId] = useState<number | null>(null)
  const [deleteTarget, setDeleteTarget] = useState<CatalogItem | null>(null)
  const schema = useCatalogSchema()

  const roots = useMemo(() => catalogs.filter((n) => !n.parentId), [catalogs])

  const totalServices = getDisplayedCatalogCount(serviceCounts, null)

  const form = useForm<FormValues>({
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    resolver: zodResolver(schema as any),
    defaultValues: { name: "", code: "", parentId: null, sortOrder: 0, icon: "", description: "" },
  })

  useEffect(() => {
    if (formOpen) {
      if (editing) {
        form.reset({
          name: editing.name,
          code: editing.code,
          parentId: editing.parentId,
          sortOrder: editing.sortOrder,
          icon: editing.icon ?? "",
          description: editing.description,
        })
      } else {
        form.reset({ name: "", code: "", parentId: formParentId, sortOrder: 0, icon: "", description: "" })
      }
    }
  }, [formOpen, editing, formParentId, form])

  const createMut = useMutation({
    mutationFn: (v: FormValues) => createCatalog({
      name: v.name, code: v.code, parentId: v.parentId, description: v.description, icon: v.icon, sortOrder: v.sortOrder,
    }),
    onSuccess: () => { queryClient.invalidateQueries({ queryKey: itsmQueryKeys.catalogs.tree() }); setFormOpen(false); toast.success(t("itsm:catalogs.createSuccess")) },
    onError: (err) => toast.error(err.message),
  })

  const updateMut = useMutation({
    mutationFn: (v: FormValues) => updateCatalog(editing!.id, {
      name: v.name, code: v.code, parentId: v.parentId, description: v.description, icon: v.icon, sortOrder: v.sortOrder,
    }),
    onSuccess: () => { queryClient.invalidateQueries({ queryKey: itsmQueryKeys.catalogs.tree() }); setFormOpen(false); toast.success(t("itsm:catalogs.updateSuccess")) },
    onError: (err) => toast.error(err.message),
  })

  const deleteMut = useMutation({
    mutationFn: (id: number) => deleteCatalog(id),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: itsmQueryKeys.catalogs.tree() })
      queryClient.invalidateQueries({ queryKey: itsmQueryKeys.catalogs.serviceCounts() })
      toast.success(t("itsm:catalogs.deleteSuccess"))
    },
    onError: (err) => toast.error(err.message),
  })

  function onSubmit(v: FormValues) { if (editing) updateMut.mutate(v); else createMut.mutate(v) }
  const isPending = createMut.isPending || updateMut.isPending

  function openCreateRoot() { setEditing(null); setFormParentId(null); setFormOpen(true) }
  function openCreateChild(parentId: number) { setEditing(null); setFormParentId(parentId); setFormOpen(true) }
  function openEdit(item: CatalogItem) { setEditing(item); setFormOpen(true) }

  return (
    <>
      <div className="workspace-surface flex w-60 shrink-0 flex-col overflow-hidden rounded-[1.15rem]">
        <div className="flex-1 overflow-y-auto p-1.5">
          {/* "全部" */}
          <button
            type="button"
            onClick={() => onSelect(null)}
            className={cn(
              "workspace-nav-item flex w-full items-center gap-2 rounded-lg px-2.5 py-2 text-left transition-colors",
              "text-muted-foreground hover:bg-background/58 hover:text-foreground",
              selectedCatalogId === null && "bg-background/70 font-medium text-foreground",
            )}
          >
            <FolderTree className="h-4 w-4 shrink-0 text-muted-foreground" />
            <span className="min-w-0 flex-1 truncate">{t("itsm:services.allNav")}</span>
            <CatalogCount value={totalServices} />
          </button>

          {/* Group sections */}
          {roots.map((root) => (
            <div key={root.id} className="mt-3">
              {/* Section header */}
              <div className={cn(
                "group/header relative rounded-lg transition-colors",
                selectedCatalogId === root.id && "bg-background/70",
              )}>
                <button
                  type="button"
                  onClick={() => onSelect(root.id)}
                  className="flex min-w-0 w-full items-center gap-1.5 px-2.5 py-1.5 text-left"
                >
                  <CatalogIcon name={root.icon} className="h-3.5 w-3.5 shrink-0 text-muted-foreground/60" />
                  <span className={cn(
                    "workspace-nav-section-title min-w-0 flex-1 truncate",
                    selectedCatalogId === root.id && "text-foreground",
                  )}>
                    {root.name}
                  </span>
                  <CatalogCount
                    value={getDisplayedCatalogCount(serviceCounts, root.id, "root")}
                    className={cn(
                      (canUpdate || canDelete || canCreate) && "transition-opacity group-hover/header:opacity-0",
                    )}
                  />
                </button>
                {(canUpdate || canDelete || canCreate) && (
                  <div className="absolute right-2 top-1/2 -translate-y-1/2 opacity-0 transition-opacity group-hover/header:opacity-100" onClick={(event) => event.stopPropagation()}>
                    <DropdownMenu>
                      <DropdownMenuTrigger asChild>
                        <button type="button" className="inline-flex h-5 w-5 items-center justify-center rounded text-muted-foreground/50 hover:bg-background/60 hover:text-foreground">
                          <MoreHorizontal className="h-3.5 w-3.5" />
                        </button>
                      </DropdownMenuTrigger>
                      <DropdownMenuContent align="end" className="w-40">
                        {canUpdate && (
                          <DropdownMenuItem onClick={() => openEdit(root)}>
                            <Pencil className="mr-2 h-3.5 w-3.5" />{t("itsm:catalogs.edit")}
                          </DropdownMenuItem>
                        )}
                        {canCreate && (
                          <DropdownMenuItem onClick={() => openCreateChild(root.id)}>
                            <Plus className="mr-2 h-3.5 w-3.5" />{t("itsm:catalogs.createChild")}
                          </DropdownMenuItem>
                        )}
                        {canDelete && (
                          <>
                            <DropdownMenuSeparator />
                            <DropdownMenuItem className="text-destructive focus:text-destructive" onClick={() => setDeleteTarget(root)}>
                              <Trash2 className="mr-2 h-3.5 w-3.5" />{t("common:delete")}
                            </DropdownMenuItem>
                          </>
                        )}
                      </DropdownMenuContent>
                    </DropdownMenu>
                  </div>
                )}
              </div>

              {/* Child items */}
              <nav className="flex flex-col gap-0.5">
                {root.children?.map((child) => (
                  <button
                    key={child.id}
                    type="button"
                    onClick={() => onSelect(child.id)}
                    className={cn(
                      "workspace-nav-item flex w-full items-center gap-2 rounded-lg px-2.5 py-1.5 pl-7 text-left transition-colors",
                      "text-muted-foreground hover:bg-background/58 hover:text-foreground",
                      selectedCatalogId === child.id && "bg-background/70 font-medium text-foreground",
                    )}
                  >
                    <span className="min-w-0 flex-1 truncate">{child.name}</span>
                    <CatalogCount value={getDisplayedCatalogCount(serviceCounts, child.id, "child")} />
                  </button>
                ))}
                {(!root.children || root.children.length === 0) && (
                  <p className="px-2.5 py-1.5 pl-7 text-xs text-muted-foreground/50">{t("itsm:catalogs.childrenEmpty")}</p>
                )}
              </nav>
            </div>
          ))}
        </div>

        {/* Bottom: create root */}
        {canCreate && (
          <div className="shrink-0 border-t border-border/45 p-1.5">
            <button
              type="button"
              onClick={openCreateRoot}
              className="workspace-nav-item flex w-full items-center justify-center gap-1.5 rounded-lg border border-border/55 bg-background/40 px-2.5 py-2 font-medium text-foreground/78 transition-colors hover:bg-background/70 hover:text-foreground"
            >
              <Plus className="h-4 w-4" />{t("itsm:catalogs.create")}
            </button>
          </div>
        )}
      </div>

      {/* Catalog Sheet */}
      <Sheet open={formOpen} onOpenChange={setFormOpen}>
        <SheetContent className="sm:max-w-lg">
          <SheetHeader>
            <SheetTitle>{editing ? t("itsm:catalogs.edit") : t("itsm:catalogs.create")}</SheetTitle>
            <SheetDescription className="sr-only">{editing ? t("itsm:catalogs.edit") : t("itsm:catalogs.create")}</SheetDescription>
          </SheetHeader>
          <Form {...form}>
            <form onSubmit={form.handleSubmit(onSubmit)} className="flex flex-1 flex-col gap-5 px-4">
              <FormField control={form.control} name="name" render={({ field }) => (
                <FormItem>
                  <FormLabel>{t("itsm:catalogs.name")}</FormLabel>
                  <FormControl><Input placeholder={t("itsm:catalogs.namePlaceholder")} {...field} /></FormControl>
                  <FormMessage />
                </FormItem>
              )} />
              <FormField control={form.control} name="code" render={({ field }) => (
                <FormItem>
                  <FormLabel>{t("itsm:catalogs.code")}</FormLabel>
                  <FormControl><Input placeholder={t("itsm:catalogs.codePlaceholder")} {...field} /></FormControl>
                  <FormMessage />
                </FormItem>
              )} />
              <FormField control={form.control} name="icon" render={({ field }) => (
                <FormItem>
                  <FormLabel>{t("itsm:catalogs.icon")}</FormLabel>
                  <Popover>
                    <PopoverTrigger asChild>
                      <FormControl>
                        <Button variant="outline" role="combobox" className="w-full justify-between">
                          <span className="inline-flex items-center gap-2">
                            <CatalogIcon name={field.value} className="h-4 w-4 text-muted-foreground" />
                            <span className="text-muted-foreground">{field.value || t("itsm:catalogs.iconPlaceholder")}</span>
                          </span>
                          <ChevronsUpDown className="ml-2 h-4 w-4 shrink-0 opacity-50" />
                        </Button>
                      </FormControl>
                    </PopoverTrigger>
                    <PopoverContent className="w-72 p-2">
                      <div className="grid grid-cols-6 gap-1">
                        {Object.keys(iconMap).map((iconName) => (
                          <button
                            key={iconName}
                            type="button"
                            onClick={() => field.onChange(iconName)}
                            className={cn(
                              "flex h-9 w-9 items-center justify-center rounded-md transition-colors",
                              field.value === iconName ? "bg-accent text-accent-foreground" : "hover:bg-muted",
                            )}
                            title={iconName}
                          >
                            <CatalogIcon name={iconName} className="h-4 w-4" />
                          </button>
                        ))}
                      </div>
                    </PopoverContent>
                  </Popover>
                  <FormMessage />
                </FormItem>
              )} />
              <FormField control={form.control} name="description" render={({ field }) => (
                <FormItem>
                  <FormLabel>{t("itsm:catalogs.description")}</FormLabel>
                  <FormControl><Textarea rows={3} {...field} /></FormControl>
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

      {/* Delete confirmation */}
      <AlertDialog open={!!deleteTarget} onOpenChange={(open) => { if (!open) setDeleteTarget(null) }}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>{t("itsm:catalogs.deleteTitle")}</AlertDialogTitle>
            <AlertDialogDescription>{t("itsm:catalogs.deleteDesc", { name: deleteTarget?.name })}</AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel size="sm">{t("common:cancel")}</AlertDialogCancel>
            <AlertDialogAction size="sm" onClick={() => { if (deleteTarget) { deleteMut.mutate(deleteTarget.id); setDeleteTarget(null) } }} disabled={deleteMut.isPending}>{t("itsm:catalogs.confirmDelete")}</AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </>
  )
}
