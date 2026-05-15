"use client"

import { useEffect, useState, type ReactNode } from "react"
import { useTranslation } from "react-i18next"
import { useForm } from "react-hook-form"
import { z } from "zod"
import { zodResolver } from "@hookform/resolvers/zod"
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import { Pencil, Plus, Trash2, Zap } from "lucide-react"
import { toast } from "sonner"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Textarea } from "@/components/ui/textarea"
import {
  Select, SelectContent, SelectItem, SelectTrigger, SelectValue,
} from "@/components/ui/select"
import {
  DataTableActions, DataTableActionsCell, DataTableActionsHead,
  DataTableCard, DataTableEmptyRow, DataTableLoadingRow,
} from "@/components/ui/data-table"
import {
  Table, TableBody, TableCell, TableHead, TableHeader, TableRow,
} from "@/components/ui/table"
import {
  AlertDialog, AlertDialogAction, AlertDialogCancel, AlertDialogContent,
  AlertDialogDescription, AlertDialogFooter, AlertDialogHeader, AlertDialogTitle,
} from "@/components/ui/alert-dialog"
import {
  Sheet, SheetContent, SheetHeader, SheetTitle, SheetDescription, SheetFooter,
} from "@/components/ui/sheet"
import {
  Form, FormControl, FormField, FormItem, FormLabel, FormMessage,
} from "@/components/ui/form"
import {
  WorkspaceAlertIconAction,
  WorkspaceIconAction,
} from "@/components/workspace/primitives"
import { usePermission } from "@/hooks/use-permission"
import {
  type ServiceActionItem,
  createServiceAction,
  deleteServiceAction,
  fetchServiceActions,
  updateServiceAction,
} from "../api"
import { itsmQueryKeys } from "../query-keys"
import { parseServiceActionConfigInput } from "./service-action-config"
import {
  formatServiceActionType,
  SERVICE_ACTION_TYPE_HTTP,
  SERVICE_ACTION_TYPE_OPTIONS,
} from "./service-action-types"

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

export function ServiceActionsSection({
  serviceId,
  title,
  showHeader = true,
}: {
  serviceId: number
  title?: ReactNode
  showHeader?: boolean
}) {
  const { t } = useTranslation(["itsm", "common"])
  const queryClient = useQueryClient()
  const [formOpen, setFormOpen] = useState(false)
  const [editing, setEditing] = useState<ServiceActionItem | null>(null)
  const canUpdate = usePermission("itsm:service:update")
  const schema = useActionSchema()

  const { data: items = [], isLoading } = useQuery({
    queryKey: itsmQueryKeys.services.actions(serviceId),
    queryFn: () => fetchServiceActions(serviceId),
    enabled: serviceId > 0,
  })

  const form = useForm<ActionFormValues>({
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    resolver: zodResolver(schema as any),
    defaultValues: { name: "", code: "", actionType: SERVICE_ACTION_TYPE_HTTP, configJson: "" },
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
        form.reset({ name: "", code: "", actionType: SERVICE_ACTION_TYPE_HTTP, configJson: "" })
      }
    }
  }, [formOpen, editing, form])

  const invalidateAfterActionChange = () => {
    queryClient.invalidateQueries({ queryKey: itsmQueryKeys.services.actions(serviceId) })
    queryClient.invalidateQueries({ queryKey: itsmQueryKeys.services.detail(serviceId) })
    queryClient.invalidateQueries({ queryKey: itsmQueryKeys.services.lists() })
  }

  function toActionPayload(v: ActionFormValues) {
    const parsed = parseServiceActionConfigInput(v.configJson)
    if (!parsed.ok) {
      form.setError("configJson", { type: "validate", message: parsed.message })
      return null
    }
    return { name: v.name, code: v.code, actionType: v.actionType, configJson: parsed.value }
  }

  const createMut = useMutation({
    mutationFn: (v: ActionFormValues) => {
      const payload = toActionPayload(v)
      if (!payload) throw new Error("invalid-action-config")
      return createServiceAction(serviceId, payload)
    },
    onSuccess: () => {
      invalidateAfterActionChange()
      setFormOpen(false)
      toast.success(t("itsm:actions.createSuccess"))
    },
    onError: (err) => {
      if (err.message !== "invalid-action-config") toast.error(err.message)
    },
  })

  const updateMut = useMutation({
    mutationFn: (v: ActionFormValues) => {
      const payload = toActionPayload(v)
      if (!payload) throw new Error("invalid-action-config")
      return updateServiceAction(serviceId, editing!.id, payload)
    },
    onSuccess: () => {
      invalidateAfterActionChange()
      setFormOpen(false)
      toast.success(t("itsm:actions.updateSuccess"))
    },
    onError: (err) => {
      if (err.message !== "invalid-action-config") toast.error(err.message)
    },
  })

  const deleteMut = useMutation({
    mutationFn: (actionId: number) => deleteServiceAction(serviceId, actionId),
    onSuccess: () => {
      invalidateAfterActionChange()
      toast.success(t("itsm:actions.deleteSuccess"))
    },
    onError: (err) => toast.error(err.message),
  })

  function onSubmit(v: ActionFormValues) {
    if (editing) updateMut.mutate(v)
    else createMut.mutate(v)
  }
  const isPending = createMut.isPending || updateMut.isPending

  return (
    <>
      {showHeader && (
        <SectionHeader
          title={title ?? t("itsm:services.tabActions")}
          action={canUpdate ? (
            <Button size="sm" onClick={() => { setEditing(null); setFormOpen(true) }}>
              <Plus className="mr-1.5 h-4 w-4" />{t("itsm:actions.create")}
            </Button>
          ) : undefined}
        />
      )}

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
              <DataTableEmptyRow colSpan={4} icon={Zap} title={t("itsm:actions.empty")} description={canUpdate ? t("itsm:actions.emptyHint") : undefined} />
            ) : (
              items.map((item) => (
                <TableRow key={item.id}>
                  <TableCell className="font-medium">{item.name}</TableCell>
                  <TableCell className="text-sm text-muted-foreground">{item.code}</TableCell>
                  <TableCell className="text-sm">{formatServiceActionType(item.actionType, t)}</TableCell>
                  <DataTableActionsCell>
                    <DataTableActions>
                      {canUpdate && (
                        <WorkspaceIconAction label={t("common:edit")} icon={Pencil} onClick={() => { setEditing(item); setFormOpen(true) }} />
                      )}
                      {canUpdate && (
                        <AlertDialog>
                          <WorkspaceAlertIconAction label={t("common:delete")} icon={Trash2} className="hover:text-destructive" />
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
                      {SERVICE_ACTION_TYPE_OPTIONS.map((option) => (
                        <SelectItem key={option.value} value={option.value}>
                          {t(option.labelKey)}
                        </SelectItem>
                      ))}
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
