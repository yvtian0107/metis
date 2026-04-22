"use client"

import { useEffect, useMemo, useState } from "react"
import { useTranslation } from "react-i18next"
import { useForm } from "react-hook-form"
import { z } from "zod"
import { zodResolver } from "@hookform/resolvers/zod"
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query"
import { Flag, Pencil, Plus, Trash2 } from "lucide-react"
import { usePermission } from "@/hooks/use-permission"
import { toast } from "sonner"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Textarea } from "@/components/ui/textarea"
import {
  DataTableActions,
  DataTableActionsCell,
  DataTableActionsHead,
  DataTableCard,
  DataTableEmptyRow,
  DataTableLoadingRow,
  DataTableToolbar,
} from "@/components/ui/data-table"
import {
  Table, TableBody, TableCell, TableHead, TableHeader, TableRow,
} from "@/components/ui/table"
import {
  AlertDialog, AlertDialogAction, AlertDialogCancel, AlertDialogContent,
  AlertDialogDescription, AlertDialogFooter, AlertDialogHeader,
  AlertDialogTitle,
} from "@/components/ui/alert-dialog"
import {
  Sheet, SheetContent, SheetHeader, SheetTitle, SheetDescription, SheetFooter,
} from "@/components/ui/sheet"
import {
  Form, FormControl, FormField, FormItem, FormLabel, FormMessage,
} from "@/components/ui/form"
import {
  WorkspaceAlertIconAction,
  WorkspaceSearchField,
  WorkspaceFormSection,
  WorkspaceIconAction,
  WorkspaceBooleanStatus,
  WorkspaceColorSwatch,
} from "@/components/workspace/primitives"
import {
  type PriorityItem, fetchPriorities, createPriority, updatePriority, deletePriority,
} from "../../api"

function usePrioritySchema() {
  const { t } = useTranslation("itsm")
  return z.object({
    name: z.string().min(1, t("validation.nameRequired")),
    code: z.string().min(1, t("validation.codeRequired")),
    value: z.number().min(0),
    color: z.string().min(1),
    description: z.string().optional(),
    defaultResponseMinutes: z.number().min(0),
    defaultResolutionMinutes: z.number().min(0),
  })
}

type FormValues = z.infer<ReturnType<typeof usePrioritySchema>>

function matchesQuery(item: Pick<PriorityItem, "name" | "code" | "description">, query: string) {
  if (!query) return true
  const haystack = `${item.name} ${item.code} ${item.description ?? ""}`.toLowerCase()
  return haystack.includes(query)
}

function formatMinutes(minutes: number, unit: string) {
  return `${minutes} ${unit}`
}

function isHexColor(value: string) {
  return /^#[0-9a-fA-F]{6}$/.test(value)
}

export function Component() {
  const { t } = useTranslation(["itsm", "common"])
  const queryClient = useQueryClient()
  const [formOpen, setFormOpen] = useState(false)
  const [editing, setEditing] = useState<PriorityItem | null>(null)
  const [search, setSearch] = useState("")
  const schema = usePrioritySchema()

  const canCreate = usePermission("itsm:priority:create")
  const canUpdate = usePermission("itsm:priority:update")
  const canDelete = usePermission("itsm:priority:delete")

  const { data: items = [], isLoading } = useQuery({
    queryKey: ["itsm-priorities"],
    queryFn: () => fetchPriorities(),
  })

  const filteredItems = useMemo(() => {
    const query = search.trim().toLowerCase()
    return items
      .filter((item) => matchesQuery(item, query))
      .slice()
      .sort((a, b) => a.value - b.value || a.name.localeCompare(b.name))
  }, [items, search])

  const form = useForm<FormValues>({
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    resolver: zodResolver(schema as any),
    defaultValues: { name: "", code: "", value: 0, color: "#5b6f8f", description: "", defaultResponseMinutes: 0, defaultResolutionMinutes: 0 },
  })

  useEffect(() => {
    if (formOpen) {
      if (editing) {
        form.reset({
          name: editing.name,
          code: editing.code,
          value: editing.value,
          color: editing.color,
          description: editing.description,
          defaultResponseMinutes: editing.defaultResponseMinutes,
          defaultResolutionMinutes: editing.defaultResolutionMinutes,
        })
      } else {
        form.reset({ name: "", code: "", value: 0, color: "#5b6f8f", description: "", defaultResponseMinutes: 0, defaultResolutionMinutes: 0 })
      }
    }
  }, [formOpen, editing, form])

  const createMut = useMutation({
    mutationFn: (v: FormValues) => createPriority({ ...v, description: v.description ?? "" }),
    onSuccess: () => { queryClient.invalidateQueries({ queryKey: ["itsm-priorities"] }); setFormOpen(false); toast.success(t("itsm:priorities.createSuccess")) },
    onError: (err) => toast.error(err.message),
  })

  const updateMut = useMutation({
    mutationFn: (v: FormValues) => updatePriority(editing!.id, { ...v, description: v.description ?? "" }),
    onSuccess: () => { queryClient.invalidateQueries({ queryKey: ["itsm-priorities"] }); setFormOpen(false); toast.success(t("itsm:priorities.updateSuccess")) },
    onError: (err) => toast.error(err.message),
  })

  const deleteMut = useMutation({
    mutationFn: (id: number) => deletePriority(id),
    onSuccess: () => { queryClient.invalidateQueries({ queryKey: ["itsm-priorities"] }); toast.success(t("itsm:priorities.deleteSuccess")) },
    onError: (err) => toast.error(err.message),
  })

  function handleCreate() { setEditing(null); setFormOpen(true) }
  function handleEdit(item: PriorityItem) { setEditing(item); setFormOpen(true) }
  function onSubmit(values: FormValues) {
    if (editing) {
      updateMut.mutate(values)
    } else {
      createMut.mutate(values)
    }
  }

  const isPending = createMut.isPending || updateMut.isPending
  const minuteUnit = t("itsm:priorities.minuteShort")

  return (
    <div className="workspace-page">
      <div className="workspace-page-header">
        <div className="min-w-0">
          <h2 className="workspace-page-title">{t("itsm:priorities.title")}</h2>
          <p className="workspace-page-description">{t("itsm:priorities.pageDesc")}</p>
        </div>
        {canCreate && (
          <Button size="sm" onClick={handleCreate}>
            <Plus className="mr-1.5 h-4 w-4" />
            {t("itsm:priorities.create")}
          </Button>
        )}
      </div>

      <DataTableCard>
        <DataTableToolbar>
          <WorkspaceSearchField
            value={search}
            onChange={setSearch}
            placeholder={t("itsm:priorities.searchPlaceholder")}
          />
          <span className="text-xs text-muted-foreground">
            {t("itsm:priorities.listCount", { count: filteredItems.length })}
          </span>
        </DataTableToolbar>
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead className="min-w-[220px]">{t("itsm:priorities.name")}</TableHead>
              <TableHead className="w-[96px]">{t("itsm:priorities.value")}</TableHead>
              <TableHead className="min-w-[220px]">{t("itsm:priorities.defaultCommitment")}</TableHead>
              <TableHead className="w-[96px]">{t("common:status")}</TableHead>
              <DataTableActionsHead className="min-w-24">{t("common:actions")}</DataTableActionsHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {isLoading ? (
              <DataTableLoadingRow colSpan={5} />
            ) : items.length === 0 ? (
              <DataTableEmptyRow colSpan={5} icon={Flag} title={t("itsm:priorities.empty")} description={canCreate ? t("itsm:priorities.emptyHint") : undefined} />
            ) : filteredItems.length === 0 ? (
              <DataTableEmptyRow colSpan={5} icon={Flag} title={t("itsm:priorities.searchEmpty")} />
            ) : (
              filteredItems.map((item) => (
                <TableRow key={item.id} className="hover:bg-muted/22">
                  <TableCell>
                    <div className="flex min-w-0 items-center gap-3">
                      <WorkspaceColorSwatch color={item.color} />
                      <div className="min-w-0">
                        <div className="font-medium text-foreground/90">{item.name}</div>
                        <div className="mt-1 flex flex-wrap items-center gap-x-2 gap-y-1 text-xs text-muted-foreground">
                          <span className="font-mono">{item.code}</span>
                          <span className="font-mono">{item.color}</span>
                          {item.description ? <span className="truncate">{item.description}</span> : null}
                        </div>
                      </div>
                    </div>
                  </TableCell>
                  <TableCell className="text-sm tabular-nums">{item.value}</TableCell>
                  <TableCell className="text-sm">
                    <div className="flex flex-wrap gap-x-4 gap-y-1 text-foreground/82">
                      <span>{t("itsm:priorities.responseShort")} {formatMinutes(item.defaultResponseMinutes, minuteUnit)}</span>
                      <span>{t("itsm:priorities.resolutionShort")} {formatMinutes(item.defaultResolutionMinutes, minuteUnit)}</span>
                    </div>
                  </TableCell>
                  <TableCell>
                    <WorkspaceBooleanStatus active={item.isActive} activeLabel={t("itsm:priorities.active")} inactiveLabel={t("itsm:priorities.inactive")} />
                  </TableCell>
                  <DataTableActionsCell>
                    <DataTableActions>
                      {canUpdate && (
                        <WorkspaceIconAction
                          label={t("common:edit")}
                          icon={Pencil}
                          onClick={() => handleEdit(item)}
                        />
                      )}
                      {canDelete && (
                        <AlertDialog>
                          <WorkspaceAlertIconAction label={t("common:delete")} icon={Trash2} className="hover:text-destructive" />
                          <AlertDialogContent>
                            <AlertDialogHeader>
                              <AlertDialogTitle>{t("itsm:priorities.deleteTitle")}</AlertDialogTitle>
                              <AlertDialogDescription>{t("itsm:priorities.deleteDesc", { name: item.name })}</AlertDialogDescription>
                            </AlertDialogHeader>
                            <AlertDialogFooter>
                              <AlertDialogCancel size="sm">{t("common:cancel")}</AlertDialogCancel>
                              <AlertDialogAction size="sm" onClick={() => deleteMut.mutate(item.id)} disabled={deleteMut.isPending}>{t("itsm:priorities.confirmDelete")}</AlertDialogAction>
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
            <SheetTitle>{editing ? t("itsm:priorities.edit") : t("itsm:priorities.create")}</SheetTitle>
            <SheetDescription>{t("itsm:priorities.sheetDesc")}</SheetDescription>
          </SheetHeader>
          <Form {...form}>
            <form onSubmit={form.handleSubmit(onSubmit)} className="flex flex-1 flex-col gap-5 px-4">
              <WorkspaceFormSection title={t("itsm:priorities.formIdentity")}>
                <FormField control={form.control} name="name" render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t("itsm:priorities.name")}</FormLabel>
                    <FormControl><Input placeholder={t("itsm:priorities.namePlaceholder")} {...field} /></FormControl>
                    <FormMessage />
                  </FormItem>
                )} />
                <div className="grid grid-cols-2 gap-4">
                  <FormField control={form.control} name="code" render={({ field }) => (
                    <FormItem>
                      <FormLabel>{t("itsm:priorities.code")}</FormLabel>
                      <FormControl><Input placeholder={t("itsm:priorities.codePlaceholder")} {...field} /></FormControl>
                      <FormMessage />
                    </FormItem>
                  )} />
                  <FormField control={form.control} name="value" render={({ field }) => (
                    <FormItem>
                      <FormLabel>{t("itsm:priorities.value")}</FormLabel>
                      <FormControl><Input type="number" {...field} onChange={(e) => field.onChange(Number(e.target.value))} /></FormControl>
                      <FormMessage />
                    </FormItem>
                  )} />
                </div>
              </WorkspaceFormSection>
              <WorkspaceFormSection title={t("itsm:priorities.formVisual")}>
                <FormField control={form.control} name="color" render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t("itsm:priorities.color")}</FormLabel>
                    <FormControl>
                      <div className="grid grid-cols-[3rem_1fr] gap-3">
                        <Input type="color" {...field} value={isHexColor(field.value) ? field.value : "#5b6f8f"} className="h-9 w-12 p-1" />
                        <Input value={field.value} onChange={field.onChange} placeholder="#5b6f8f" className="font-mono" />
                      </div>
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )} />
              </WorkspaceFormSection>
              <WorkspaceFormSection title={t("itsm:priorities.formCommitment")}>
                <div className="grid grid-cols-2 gap-4">
                  <FormField control={form.control} name="defaultResponseMinutes" render={({ field }) => (
                    <FormItem>
                      <FormLabel>{t("itsm:priorities.defaultResponseMinutes")}</FormLabel>
                      <FormControl><Input type="number" {...field} onChange={(e) => field.onChange(Number(e.target.value))} /></FormControl>
                      <FormMessage />
                    </FormItem>
                  )} />
                  <FormField control={form.control} name="defaultResolutionMinutes" render={({ field }) => (
                    <FormItem>
                      <FormLabel>{t("itsm:priorities.defaultResolutionMinutes")}</FormLabel>
                      <FormControl><Input type="number" {...field} onChange={(e) => field.onChange(Number(e.target.value))} /></FormControl>
                      <FormMessage />
                    </FormItem>
                  )} />
                </div>
              </WorkspaceFormSection>
              <WorkspaceFormSection title={t("itsm:priorities.formDescription")}>
                <FormField control={form.control} name="description" render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t("itsm:priorities.description")}</FormLabel>
                    <FormControl><Textarea rows={3} {...field} /></FormControl>
                    <FormMessage />
                  </FormItem>
                )} />
              </WorkspaceFormSection>
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
