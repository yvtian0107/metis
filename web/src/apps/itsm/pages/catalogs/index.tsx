"use client"

import { useState, useEffect, useMemo } from "react"
import { useTranslation } from "react-i18next"
import { useForm } from "react-hook-form"
import { z } from "zod"
import { zodResolver } from "@hookform/resolvers/zod"
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query"
import {
  Plus, Pencil, Trash2, FolderTree,
  ShieldCheck, Monitor, Globe, Container, ShieldAlert, Bell,
  User, Lock, KeyRound, LayoutGrid, Video, Server, Database,
  Bug, FileSearch, LineChart, BellRing, Clock, ChevronsUpDown,
  type LucideIcon,
} from "lucide-react"
import { usePermission } from "@/hooks/use-permission"
import { toast } from "sonner"
import { cn } from "@/lib/utils"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Textarea } from "@/components/ui/textarea"
import { Badge } from "@/components/ui/badge"
import { Card, CardContent } from "@/components/ui/card"
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
  Popover, PopoverContent, PopoverTrigger,
} from "@/components/ui/popover"
import {
  type CatalogItem, fetchCatalogTree, createCatalog, updateCatalog, deleteCatalog,
} from "../../api"

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

// ─── Component ─────────────────────────────────────────

export function Component() {
  const { t } = useTranslation(["itsm", "common"])
  const queryClient = useQueryClient()
  const [formOpen, setFormOpen] = useState(false)
  const [editing, setEditing] = useState<CatalogItem | null>(null)
  const [selectedRootId, setSelectedRootId] = useState<number | null>(null)
  // formMode: "root" = creating/editing a root category, "child" = creating/editing a child
  const [formMode, setFormMode] = useState<"root" | "child">("root")
  const [deleteRootTarget, setDeleteRootTarget] = useState<CatalogItem | null>(null)
  const schema = useCatalogSchema()

  const canCreate = usePermission("itsm:catalog:create")
  const canUpdate = usePermission("itsm:catalog:update")
  const canDelete = usePermission("itsm:catalog:delete")

  const { data: tree = [], isLoading } = useQuery({
    queryKey: ["itsm-catalogs"],
    queryFn: () => fetchCatalogTree(),
  })

  // Root categories (top-level, no parent)
  const roots = useMemo(() => tree.filter((n) => !n.parentId), [tree])

  // Derive effective selected ID: use user selection if valid, else fall back to first root
  const effectiveRootId = useMemo(() => {
    if (selectedRootId !== null && roots.find((r) => r.id === selectedRootId)) {
      return selectedRootId
    }
    return roots.length > 0 ? roots[0].id : null
  }, [roots, selectedRootId])

  const selectedRoot = useMemo(() => roots.find((r) => r.id === effectiveRootId) ?? null, [roots, effectiveRootId])
  const children = useMemo(() => selectedRoot?.children ?? [], [selectedRoot])

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
      } else if (formMode === "child" && effectiveRootId) {
        form.reset({ name: "", code: "", parentId: effectiveRootId, sortOrder: 0, icon: "", description: "" })
      } else {
        form.reset({ name: "", code: "", parentId: null, sortOrder: 0, icon: "", description: "" })
      }
    }
  }, [formOpen, editing, formMode, effectiveRootId, form])

  const createMut = useMutation({
    mutationFn: (v: FormValues) => createCatalog({
      name: v.name, code: v.code, parentId: v.parentId, description: v.description, icon: v.icon, sortOrder: v.sortOrder,
    }),
    onSuccess: () => { queryClient.invalidateQueries({ queryKey: ["itsm-catalogs"] }); setFormOpen(false); toast.success(t("itsm:catalogs.createSuccess")) },
    onError: (err) => toast.error(err.message),
  })

  const updateMut = useMutation({
    mutationFn: (v: FormValues) => updateCatalog(editing!.id, {
      name: v.name, code: v.code, parentId: v.parentId, description: v.description, icon: v.icon, sortOrder: v.sortOrder,
    }),
    onSuccess: () => { queryClient.invalidateQueries({ queryKey: ["itsm-catalogs"] }); setFormOpen(false); toast.success(t("itsm:catalogs.updateSuccess")) },
    onError: (err) => toast.error(err.message),
  })

  const deleteMut = useMutation({
    mutationFn: (id: number) => deleteCatalog(id),
    onSuccess: () => { queryClient.invalidateQueries({ queryKey: ["itsm-catalogs"] }); toast.success(t("itsm:catalogs.deleteSuccess")) },
    onError: (err) => toast.error(err.message),
  })

  function onSubmit(v: FormValues) { if (editing) { updateMut.mutate(v) } else { createMut.mutate(v) } }
  const isPending = createMut.isPending || updateMut.isPending

  function openCreateRoot() { setEditing(null); setFormMode("root"); setFormOpen(true) }
  function openCreateChild() { setEditing(null); setFormMode("child"); setFormOpen(true) }
  function openEditRoot(item: CatalogItem) { setEditing(item); setFormMode("root"); setFormOpen(true) }
  function openEditChild(item: CatalogItem) { setEditing(item); setFormMode("child"); setFormOpen(true) }

  // ── Full-page empty state ─────────────────────────────
  if (!isLoading && roots.length === 0) {
    return (
      <div className="space-y-4">
        <h2 className="text-lg font-semibold">{t("itsm:catalogs.title")}</h2>
        <DataTableCard>
          <Table>
            <TableBody>
              <DataTableEmptyRow colSpan={4} icon={FolderTree} title={t("itsm:catalogs.empty")} description={canCreate ? t("itsm:catalogs.emptyHint") : undefined} />
            </TableBody>
          </Table>
          {canCreate && (
            <div className="flex justify-center pb-6">
              <Button onClick={openCreateRoot}>
                <Plus className="mr-1.5 h-4 w-4" />{t("itsm:catalogs.create")}
              </Button>
            </div>
          )}
        </DataTableCard>
        {renderSheet()}
      </div>
    )
  }

  // ── Sheet (shared for root + child) ───────────────────
  function renderSheet() {
    return (
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
                              field.value === iconName ? "bg-accent text-accent-foreground" : "hover:bg-muted"
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
    )
  }

  // ── Main layout ───────────────────────────────────────
  return (
    <div className="space-y-4">
      <h2 className="text-lg font-semibold">{t("itsm:catalogs.title")}</h2>

      {isLoading ? (
        <DataTableCard>
          <Table><TableBody><DataTableLoadingRow colSpan={4} /></TableBody></Table>
        </DataTableCard>
      ) : (
        <div className="flex gap-4 items-start">
          {/* ── Left panel: root categories ──────────────── */}
          <Card className="w-72 shrink-0 sticky top-0 flex flex-col min-h-[calc(100vh-10rem)]">
            <CardContent className="flex-1 overflow-y-auto p-2">
              <nav className="flex flex-col gap-0.5">
                {roots.map((root) => (
                  <div
                    key={root.id}
                    className={cn(
                      "flex items-center gap-2 rounded-md px-3 py-2.5 text-left text-sm transition-colors cursor-pointer",
                      "hover:bg-accent",
                      effectiveRootId === root.id && "bg-accent font-medium",
                    )}
                    onClick={() => setSelectedRootId(root.id)}
                  >
                    <CatalogIcon name={root.icon} className="h-4 w-4 shrink-0 text-muted-foreground" />
                    <span className="flex-1 truncate">{root.name}</span>
                    <Badge variant="outline" className="ml-auto text-xs tabular-nums border-transparent bg-muted/60">
                      {root.children?.length ?? 0}
                    </Badge>
                    {(canUpdate || canDelete) && (
                      <span className="flex items-center shrink-0 -mr-1">
                        {canUpdate && (
                          <button type="button" className="h-6 w-6 inline-flex items-center justify-center rounded text-muted-foreground/50 hover:text-primary" onClick={(e) => { e.stopPropagation(); openEditRoot(root) }}>
                            <Pencil className="h-3 w-3" />
                          </button>
                        )}
                        {canDelete && (
                          <button type="button" className="h-6 w-6 inline-flex items-center justify-center rounded text-muted-foreground/50 hover:text-destructive" onClick={(e) => { e.stopPropagation(); setDeleteRootTarget(root) }}>
                            <Trash2 className="h-3 w-3" />
                          </button>
                        )}
                      </span>
                    )}
                  </div>
                ))}
              </nav>
            </CardContent>
            {canCreate && (
              <div className="shrink-0 border-t p-2">
                <button
                  type="button"
                  onClick={openCreateRoot}
                  className="flex w-full items-center justify-center gap-1.5 rounded-md border border-dashed border-muted-foreground/25 px-3 py-2 text-sm text-muted-foreground transition-colors hover:border-primary/50 hover:text-primary"
                >
                  <Plus className="h-4 w-4" />{t("itsm:catalogs.create")}
                </button>
              </div>
            )}
          </Card>

          {/* ── Right panel: selected root detail + children ── */}
          <div className="flex-1 min-w-0 space-y-4">
            {selectedRoot && (
              <>
                {/* Header */}
                <div className="flex items-start justify-between gap-4">
                  <div className="flex items-center gap-3 min-w-0">
                    <CatalogIcon name={selectedRoot.icon} className="h-6 w-6 shrink-0 text-muted-foreground" />
                    <div className="min-w-0">
                      <h3 className="text-base font-medium truncate">{selectedRoot.name}</h3>
                      {selectedRoot.description && (
                        <p className="text-sm text-muted-foreground truncate">{selectedRoot.description}</p>
                      )}
                    </div>
                  </div>
                  <div className="flex items-center gap-2 shrink-0">
                    {canUpdate && (
                      <Button variant="outline" size="sm" onClick={() => openEditRoot(selectedRoot)}>
                        <Pencil className="mr-1 h-3.5 w-3.5" />{t("common:edit")}
                      </Button>
                    )}
                    {canCreate && (
                      <Button size="sm" onClick={openCreateChild}>
                        <Plus className="mr-1 h-3.5 w-3.5" />{t("itsm:catalogs.createChild")}
                      </Button>
                    )}
                  </div>
                </div>

                {/* Children table */}
                <DataTableCard>
                  <Table>
                    <TableHeader>
                      <TableRow>
                        <TableHead className="min-w-[160px]">{t("itsm:catalogs.name")}</TableHead>
                        <TableHead className="min-w-[140px]">{t("itsm:catalogs.code")}</TableHead>
                        <TableHead className="hidden lg:table-cell">{t("itsm:catalogs.description")}</TableHead>
                        <TableHead className="w-[80px]">{t("common:status")}</TableHead>
                        <DataTableActionsHead className="min-w-[140px]">{t("common:actions")}</DataTableActionsHead>
                      </TableRow>
                    </TableHeader>
                    <TableBody>
                      {children.length === 0 ? (
                        <DataTableEmptyRow colSpan={5} icon={FolderTree} title={t("itsm:catalogs.childrenEmpty")} description={canCreate ? t("itsm:catalogs.childrenEmptyHint") : undefined} />
                      ) : (
                        children.map((child) => (
                          <TableRow key={child.id}>
                            <TableCell className="font-medium">
                              <span className="inline-flex items-center gap-1.5">
                                <CatalogIcon name={child.icon} className="h-4 w-4 text-muted-foreground" />
                                {child.name}
                              </span>
                            </TableCell>
                            <TableCell className="text-sm text-muted-foreground font-mono">{child.code}</TableCell>
                            <TableCell className="text-sm text-muted-foreground hidden lg:table-cell truncate max-w-[240px]">{child.description}</TableCell>
                            <TableCell>
                              <Badge variant={child.isActive ? "default" : "secondary"}>
                                {child.isActive ? t("itsm:catalogs.active") : t("itsm:catalogs.inactive")}
                              </Badge>
                            </TableCell>
                            <DataTableActionsCell>
                              <DataTableActions>
                                {canUpdate && (
                                  <Button variant="ghost" size="sm" className="px-2.5" onClick={() => openEditChild(child)}>
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
                                        <AlertDialogTitle>{t("itsm:catalogs.deleteTitle")}</AlertDialogTitle>
                                        <AlertDialogDescription>{t("itsm:catalogs.deleteDesc", { name: child.name })}</AlertDialogDescription>
                                      </AlertDialogHeader>
                                      <AlertDialogFooter>
                                        <AlertDialogCancel size="sm">{t("common:cancel")}</AlertDialogCancel>
                                        <AlertDialogAction size="sm" onClick={() => deleteMut.mutate(child.id)} disabled={deleteMut.isPending}>{t("itsm:catalogs.confirmDelete")}</AlertDialogAction>
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
              </>
            )}
          </div>
        </div>
      )}

      {renderSheet()}

      {/* Delete root category confirmation */}
      <AlertDialog open={!!deleteRootTarget} onOpenChange={(open) => { if (!open) setDeleteRootTarget(null) }}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>{t("itsm:catalogs.deleteTitle")}</AlertDialogTitle>
            <AlertDialogDescription>{t("itsm:catalogs.deleteDesc", { name: deleteRootTarget?.name })}</AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel size="sm">{t("common:cancel")}</AlertDialogCancel>
            <AlertDialogAction size="sm" onClick={() => { if (deleteRootTarget) { deleteMut.mutate(deleteRootTarget.id); setDeleteRootTarget(null) } }} disabled={deleteMut.isPending}>{t("itsm:catalogs.confirmDelete")}</AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  )
}
