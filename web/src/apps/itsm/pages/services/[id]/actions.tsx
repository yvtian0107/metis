"use client"

import { useState, useEffect } from "react"
import { useParams, useNavigate } from "react-router"
import { useTranslation } from "react-i18next"
import { useForm } from "react-hook-form"
import { z } from "zod"
import { zodResolver } from "@hookform/resolvers/zod"
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query"
import { ArrowLeft, Plus, Pencil, Trash2, Zap } from "lucide-react"
import { usePermission } from "@/hooks/use-permission"
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
  type ServiceActionItem,
  fetchServiceDef, fetchServiceActions,
  createServiceAction, updateServiceAction, deleteServiceAction,
} from "../../../api"
import {
  WorkspaceAlertIconAction,
  WorkspaceIconAction,
} from "@/components/workspace/primitives"

function useActionSchema() {
  const { t } = useTranslation("itsm")
  return z.object({
    name: z.string().min(1, t("validation.nameRequired")),
    code: z.string().min(1, t("validation.codeRequired")),
    actionType: z.string().min(1),
    configJson: z.string().optional(),
  })
}

type FormValues = z.infer<ReturnType<typeof useActionSchema>>

export function Component() {
  const { t } = useTranslation(["itsm", "common"])
  const { id } = useParams<{ id: string }>()
  const navigate = useNavigate()
  const queryClient = useQueryClient()
  const serviceId = Number(id)
  const [formOpen, setFormOpen] = useState(false)
  const [editing, setEditing] = useState<ServiceActionItem | null>(null)
  const schema = useActionSchema()

  const canUpdate = usePermission("itsm:service:update")

  const { data: service } = useQuery({
    queryKey: ["itsm-service", serviceId],
    queryFn: () => fetchServiceDef(serviceId),
    enabled: serviceId > 0,
  })

  const { data: items = [], isLoading } = useQuery({
    queryKey: ["itsm-service-actions", serviceId],
    queryFn: () => fetchServiceActions(serviceId),
    enabled: serviceId > 0,
  })

  const form = useForm<FormValues>({
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
    mutationFn: (v: FormValues) => {
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
    mutationFn: (v: FormValues) => {
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

  function onSubmit(v: FormValues) { if (editing) { updateMut.mutate(v) } else { createMut.mutate(v) } }
  const isPending = createMut.isPending || updateMut.isPending

  return (
    <div className="space-y-4">
      <div className="flex items-center gap-3">
        <Button variant="ghost" size="icon" onClick={() => navigate("/itsm/services")}>
          <ArrowLeft className="h-4 w-4" />
        </Button>
        <div className="flex-1">
          <h2 className="text-lg font-semibold">{t("itsm:actions.title")}</h2>
          {service && <p className="text-sm text-muted-foreground">{service.name} ({service.code})</p>}
        </div>
        {canUpdate && (
          <Button onClick={() => { setEditing(null); setFormOpen(true) }}>
            <Plus className="mr-1.5 h-4 w-4" />{t("itsm:actions.create")}
          </Button>
        )}
      </div>

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
