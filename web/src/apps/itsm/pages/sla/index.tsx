"use client"

import { useEffect, useMemo, useState } from "react"
import type { ReactNode } from "react"
import { useTranslation } from "react-i18next"
import { useForm, useWatch } from "react-hook-form"
import { z } from "zod"
import { zodResolver } from "@hookform/resolvers/zod"
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query"
import { ChevronRight, Pencil, Plus, Timer, Trash2 } from "lucide-react"
import { usePermission } from "@/hooks/use-permission"
import { toast } from "sonner"
import { cn } from "@/lib/utils"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Textarea } from "@/components/ui/textarea"
import {
  Select, SelectContent, SelectItem, SelectTrigger, SelectValue,
} from "@/components/ui/select"
import {
  DataTableActions, DataTableActionsCell, DataTableActionsHead,
  DataTableCard, DataTableEmptyRow, DataTableLoadingRow, DataTableToolbar,
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
} from "@/components/workspace/primitives"
import { ParticipantPicker } from "../../components/workflow/panels/participant-picker"
import type { Participant } from "../../components/workflow/types"
import {
  type SLATemplateItem, type EscalationRuleItem,
  fetchSLATemplates, createSLATemplate, updateSLATemplate, deleteSLATemplate,
  fetchEscalationRules, createEscalationRule, updateEscalationRule, deleteEscalationRule,
  fetchNotificationChannels, fetchPriorities,
} from "../../api"

function useSLASchema() {
  const { t } = useTranslation("itsm")
  return z.object({
    name: z.string().min(1, t("validation.nameRequired")),
    code: z.string().min(1, t("validation.codeRequired")),
    description: z.string().optional(),
    responseMinutes: z.number().min(1),
    resolutionMinutes: z.number().min(1),
  })
}

type SLAFormValues = z.infer<ReturnType<typeof useSLASchema>>

function useEscalationSchema() {
  const participantSchema = z.object({
    type: z.string().min(1),
    value: z.string().optional(),
    id: z.union([z.string(), z.number()]).optional(),
    name: z.string().optional(),
    position_code: z.string().optional(),
    department_code: z.string().optional(),
  })
  return z.object({
    triggerType: z.enum(["response_timeout", "resolution_timeout"]),
    level: z.number().min(1),
    waitMinutes: z.number().min(1),
    actionType: z.enum(["notify", "reassign", "escalate_priority"]),
    recipients: z.array(participantSchema),
    channelId: z.number(),
    subjectTemplate: z.string().optional(),
    bodyTemplate: z.string().optional(),
    assigneeCandidates: z.array(participantSchema),
    priorityId: z.number(),
  }).superRefine((value, ctx) => {
    if (value.actionType === "notify") {
      if (value.recipients.length === 0) {
        ctx.addIssue({ code: z.ZodIssueCode.custom, path: ["recipients"], message: "请选择通知接收人" })
      }
      if (!value.channelId) {
        ctx.addIssue({ code: z.ZodIssueCode.custom, path: ["channelId"], message: "请选择通知方式" })
      }
    }
    if (value.actionType === "reassign" && value.assigneeCandidates.length === 0) {
      ctx.addIssue({ code: z.ZodIssueCode.custom, path: ["assigneeCandidates"], message: "请选择改派候选人" })
    }
    if (value.actionType === "escalate_priority" && !value.priorityId) {
      ctx.addIssue({ code: z.ZodIssueCode.custom, path: ["priorityId"], message: "请选择目标优先级" })
    }
  })
}

type EscalationFormValues = z.infer<ReturnType<typeof useEscalationSchema>>

function formatMinutes(minutes: number, unit: string) {
  return `${minutes} ${unit}`
}

function matchesQuery(item: Pick<SLATemplateItem, "name" | "code" | "description">, query: string) {
  if (!query) return true
  const haystack = `${item.name} ${item.code} ${item.description ?? ""}`.toLowerCase()
  return haystack.includes(query)
}

function RuleActionBadge({ children }: { children: ReactNode }) {
  return (
    <span className="inline-flex items-center rounded-full border border-border/60 bg-background/35 px-2 py-0.5 text-xs font-medium text-foreground/70">
      {children}
    </span>
  )
}

interface EscalationTargetConfig {
  recipients?: Participant[]
  channelId?: number
  subjectTemplate?: string
  bodyTemplate?: string
  assigneeCandidates?: Participant[]
  priorityId?: number
}

const defaultNotifySubject = "SLA 升级通知：{{ticket.code}}"
const defaultNotifyBody = "工单 {{ticket.code}} 已触发 SLA 升级规则，请及时处理。"

function readTargetConfig(value: unknown): EscalationTargetConfig {
  if (!value || typeof value !== "object") return {}
  return value as EscalationTargetConfig
}

function buildTargetConfig(v: EscalationFormValues): EscalationTargetConfig {
  if (v.actionType === "notify") {
    return {
      recipients: v.recipients,
      channelId: v.channelId,
      subjectTemplate: v.subjectTemplate ?? "",
      bodyTemplate: v.bodyTemplate ?? "",
    }
  }
  if (v.actionType === "reassign") {
    return { assigneeCandidates: v.assigneeCandidates }
  }
  return { priorityId: v.priorityId }
}

function escalationPayload(v: EscalationFormValues) {
  return {
    triggerType: v.triggerType,
    level: v.level,
    waitMinutes: v.waitMinutes,
    actionType: v.actionType,
    targetConfig: buildTargetConfig(v),
  }
}

function participantLabel(p: Participant) {
  if (p.type === "requester_manager") return "提交人上级"
  if (p.type === "position_department") {
    return [p.department_code, p.position_code].filter(Boolean).join(" / ") || "岗位+部门"
  }
  return p.name ?? p.value ?? p.type
}

function formatParticipants(items: Participant[] | undefined) {
  if (!items || items.length === 0) return "未配置"
  return items.map(participantLabel).join("、")
}

function EscalationRules({ slaId }: { slaId: number }) {
  const { t } = useTranslation(["itsm", "common"])
  const queryClient = useQueryClient()
  const [formOpen, setFormOpen] = useState(false)
  const [editing, setEditing] = useState<EscalationRuleItem | null>(null)
  const canUpdate = usePermission("itsm:sla:update")
  const canDelete = usePermission("itsm:sla:delete")
  const escSchema = useEscalationSchema()

  const { data: rules = [], isLoading } = useQuery({
    queryKey: ["itsm-escalation-rules", slaId],
    queryFn: () => fetchEscalationRules(slaId),
  })
  const { data: channels = [] } = useQuery({
    queryKey: ["itsm-notification-channels"],
    queryFn: () => fetchNotificationChannels(),
  })
  const { data: priorities = [] } = useQuery({
    queryKey: ["itsm-priorities-for-sla"],
    queryFn: () => fetchPriorities(),
  })

  const form = useForm<EscalationFormValues>({
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    resolver: zodResolver(escSchema as any),
    defaultValues: {
      triggerType: "response_timeout",
      level: 1,
      waitMinutes: 30,
      actionType: "notify",
      recipients: [],
      channelId: 0,
      subjectTemplate: defaultNotifySubject,
      bodyTemplate: defaultNotifyBody,
      assigneeCandidates: [],
      priorityId: 0,
    },
  })
  const actionType = useWatch({ control: form.control, name: "actionType" })

  useEffect(() => {
    if (formOpen) {
      if (editing) {
        const cfg = readTargetConfig(editing.targetConfig)
        form.reset({
          triggerType: editing.triggerType as "response_timeout" | "resolution_timeout",
          level: editing.level,
          waitMinutes: editing.waitMinutes,
          actionType: editing.actionType as "notify" | "reassign" | "escalate_priority",
          recipients: cfg.recipients ?? [],
          channelId: cfg.channelId ?? 0,
          subjectTemplate: cfg.subjectTemplate ?? defaultNotifySubject,
          bodyTemplate: cfg.bodyTemplate ?? defaultNotifyBody,
          assigneeCandidates: cfg.assigneeCandidates ?? [],
          priorityId: cfg.priorityId ?? 0,
        })
      } else {
        form.reset({
          triggerType: "response_timeout",
          level: 1,
          waitMinutes: 30,
          actionType: "notify",
          recipients: [],
          channelId: channels[0]?.id ?? 0,
          subjectTemplate: defaultNotifySubject,
          bodyTemplate: defaultNotifyBody,
          assigneeCandidates: [],
          priorityId: priorities[0]?.id ?? 0,
        })
      }
    }
  }, [formOpen, editing, form, channels, priorities])

  const createMut = useMutation({
    mutationFn: (v: EscalationFormValues) => createEscalationRule(slaId, escalationPayload(v)),
    onSuccess: () => { queryClient.invalidateQueries({ queryKey: ["itsm-escalation-rules", slaId] }); setFormOpen(false); toast.success(t("itsm:sla.escalation.createSuccess")) },
    onError: (err) => toast.error(err.message),
  })

  const updateMut = useMutation({
    mutationFn: (v: EscalationFormValues) => updateEscalationRule(slaId, editing!.id, escalationPayload(v)),
    onSuccess: () => { queryClient.invalidateQueries({ queryKey: ["itsm-escalation-rules", slaId] }); setFormOpen(false); toast.success(t("itsm:sla.escalation.updateSuccess")) },
    onError: (err) => toast.error(err.message),
  })

  const deleteMut = useMutation({
    mutationFn: (id: number) => deleteEscalationRule(slaId, id),
    onSuccess: () => { queryClient.invalidateQueries({ queryKey: ["itsm-escalation-rules", slaId] }); toast.success(t("itsm:sla.escalation.deleteSuccess")) },
    onError: (err) => toast.error(err.message),
  })

  function onSubmit(v: EscalationFormValues) {
    if (editing) {
      updateMut.mutate(v)
    } else {
      createMut.mutate(v)
    }
  }

  const isPending = createMut.isPending || updateMut.isPending
  const minuteUnit = t("itsm:sla.minuteShort")
  const triggerLabel = (v: string) => v === "response_timeout" ? t("itsm:sla.escalation.responseTimeout") : t("itsm:sla.escalation.resolutionTimeout")
  const actionLabel = (v: string) => ({ notify: t("itsm:sla.escalation.notify"), reassign: t("itsm:sla.escalation.reassign"), escalate_priority: t("itsm:sla.escalation.escalatePriority") })[v] ?? v
  const channelName = (id: number | undefined) => channels.find((channel) => channel.id === id)?.name ?? (id ? `#${id}` : "未配置")
  const priorityName = (id: number | undefined) => priorities.find((priority) => priority.id === id)?.name ?? (id ? `#${id}` : "未配置")
  const targetSummary = (rule: EscalationRuleItem) => {
    const cfg = readTargetConfig(rule.targetConfig)
    if (rule.actionType === "notify") return `${formatParticipants(cfg.recipients)} / ${channelName(cfg.channelId)}`
    if (rule.actionType === "reassign") return formatParticipants(cfg.assigneeCandidates)
    if (rule.actionType === "escalate_priority") return priorityName(cfg.priorityId)
    return "未配置"
  }

  return (
    <TableRow>
      <TableCell colSpan={6} className="bg-background/20 p-0">
        <div className="border-y border-border/35 px-4 py-4 sm:px-6">
          <div className="mb-3 flex flex-col gap-2 sm:flex-row sm:items-center sm:justify-between">
            <div>
              <h4 className="text-sm font-semibold text-foreground/82">{t("itsm:sla.escalations")}</h4>
              <p className="mt-0.5 text-xs text-muted-foreground">{t("itsm:sla.escalation.description")}</p>
            </div>
            {canUpdate && (
              <Button size="sm" variant="outline" onClick={() => { setEditing(null); setFormOpen(true) }}>
                <Plus className="mr-1.5 h-3.5 w-3.5" />{t("itsm:sla.escalation.create")}
              </Button>
            )}
          </div>

          {isLoading ? (
            <div className="rounded-xl border border-border/45 bg-background/25 px-4 py-5 text-sm text-muted-foreground">
              {t("common:loading")}
            </div>
          ) : rules.length === 0 ? (
            <div className="rounded-xl border border-dashed border-border/55 bg-background/25 px-4 py-5 text-sm text-muted-foreground">
              {t("itsm:sla.escalation.empty")}
            </div>
          ) : (
            <div className="overflow-hidden rounded-xl border border-border/45 bg-background/25">
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>{t("itsm:sla.escalation.triggerType")}</TableHead>
                    <TableHead className="w-[72px]">{t("itsm:sla.escalation.level")}</TableHead>
                    <TableHead className="w-[132px]">{t("itsm:sla.escalation.waitMinutes")}</TableHead>
                    <TableHead>{t("itsm:sla.escalation.actionType")}</TableHead>
                    <TableHead>{t("itsm:sla.escalation.targetConfig")}</TableHead>
                    <DataTableActionsHead className="min-w-24">{t("common:actions")}</DataTableActionsHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {rules.map((rule) => (
                    <TableRow key={rule.id}>
                      <TableCell className="text-sm">{triggerLabel(rule.triggerType)}</TableCell>
                      <TableCell className="text-sm tabular-nums">{rule.level}</TableCell>
                      <TableCell className="text-sm tabular-nums">{formatMinutes(rule.waitMinutes, minuteUnit)}</TableCell>
                      <TableCell><RuleActionBadge>{actionLabel(rule.actionType)}</RuleActionBadge></TableCell>
                      <TableCell className="max-w-[260px] truncate text-sm text-muted-foreground">{targetSummary(rule)}</TableCell>
                      <DataTableActionsCell>
                        <DataTableActions>
                          {canUpdate && (
                            <WorkspaceIconAction
                              label={t("common:edit")}
                              icon={Pencil}
                              onClick={() => { setEditing(rule); setFormOpen(true) }}
                            />
                          )}
                          {canDelete && (
                            <AlertDialog>
                              <WorkspaceAlertIconAction label={t("common:delete")} icon={Trash2} className="hover:text-destructive" />
                              <AlertDialogContent>
                                <AlertDialogHeader>
                                  <AlertDialogTitle>{t("itsm:sla.escalation.deleteTitle")}</AlertDialogTitle>
                                  <AlertDialogDescription>{t("itsm:sla.escalation.deleteDesc")}</AlertDialogDescription>
                                </AlertDialogHeader>
                                <AlertDialogFooter>
                                  <AlertDialogCancel size="sm">{t("common:cancel")}</AlertDialogCancel>
                                  <AlertDialogAction size="sm" onClick={() => deleteMut.mutate(rule.id)} disabled={deleteMut.isPending}>{t("itsm:sla.escalation.confirmDelete")}</AlertDialogAction>
                                </AlertDialogFooter>
                              </AlertDialogContent>
                            </AlertDialog>
                          )}
                        </DataTableActions>
                      </DataTableActionsCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            </div>
          )}

          <Sheet open={formOpen} onOpenChange={setFormOpen}>
            <SheetContent className="sm:max-w-xl">
              <SheetHeader>
                <SheetTitle>{editing ? t("itsm:sla.escalation.edit") : t("itsm:sla.escalation.create")}</SheetTitle>
                <SheetDescription>{t("itsm:sla.escalation.sheetDesc")}</SheetDescription>
              </SheetHeader>
              <Form {...form}>
                <form onSubmit={form.handleSubmit(onSubmit)} className="flex flex-1 flex-col gap-5 px-4">
                  <WorkspaceFormSection title={t("itsm:sla.formPolicy")}>
                    <FormField control={form.control} name="triggerType" render={({ field }) => (
                      <FormItem>
                        <FormLabel>{t("itsm:sla.escalation.triggerType")}</FormLabel>
                        <Select onValueChange={field.onChange} value={field.value}>
                          <FormControl><SelectTrigger><SelectValue /></SelectTrigger></FormControl>
                          <SelectContent>
                            <SelectItem value="response_timeout">{t("itsm:sla.escalation.responseTimeout")}</SelectItem>
                            <SelectItem value="resolution_timeout">{t("itsm:sla.escalation.resolutionTimeout")}</SelectItem>
                          </SelectContent>
                        </Select>
                        <FormMessage />
                      </FormItem>
                    )} />
                    <div className="grid grid-cols-2 gap-4">
                      <FormField control={form.control} name="level" render={({ field }) => (
                        <FormItem>
                          <FormLabel>{t("itsm:sla.escalation.level")}</FormLabel>
                          <FormControl><Input type="number" {...field} onChange={(e) => field.onChange(Number(e.target.value))} /></FormControl>
                          <FormMessage />
                        </FormItem>
                      )} />
                      <FormField control={form.control} name="waitMinutes" render={({ field }) => (
                        <FormItem>
                          <FormLabel>{t("itsm:sla.escalation.waitMinutes")}</FormLabel>
                          <FormControl><Input type="number" {...field} onChange={(e) => field.onChange(Number(e.target.value))} /></FormControl>
                          <FormMessage />
                        </FormItem>
                      )} />
                    </div>
                    <FormField control={form.control} name="actionType" render={({ field }) => (
                      <FormItem>
                        <FormLabel>{t("itsm:sla.escalation.actionType")}</FormLabel>
                        <Select onValueChange={field.onChange} value={field.value}>
                          <FormControl><SelectTrigger><SelectValue /></SelectTrigger></FormControl>
                          <SelectContent>
                            <SelectItem value="notify">{t("itsm:sla.escalation.notify")}</SelectItem>
                            <SelectItem value="reassign">{t("itsm:sla.escalation.reassign")}</SelectItem>
                            <SelectItem value="escalate_priority">{t("itsm:sla.escalation.escalatePriority")}</SelectItem>
                          </SelectContent>
                        </Select>
                        <FormMessage />
                      </FormItem>
                    )} />
                    {actionType === "notify" && (
                      <>
                        <FormField control={form.control} name="recipients" render={({ field }) => (
                          <FormItem>
                            <FormLabel>{t("itsm:sla.escalation.recipients")}</FormLabel>
                            <ParticipantPicker participants={field.value} onChange={field.onChange} />
                            <FormMessage />
                          </FormItem>
                        )} />
                        <FormField control={form.control} name="channelId" render={({ field }) => (
                          <FormItem>
                            <FormLabel>{t("itsm:sla.escalation.channel")}</FormLabel>
                            <Select value={String(field.value || "")} onValueChange={(value) => field.onChange(Number(value))}>
                              <FormControl><SelectTrigger><SelectValue placeholder={t("itsm:sla.escalation.channelPlaceholder")} /></SelectTrigger></FormControl>
                              <SelectContent>
                                {channels.map((channel) => (
                                  <SelectItem key={channel.id} value={String(channel.id)}>{channel.name} / {channel.type}</SelectItem>
                                ))}
                              </SelectContent>
                            </Select>
                            <FormMessage />
                          </FormItem>
                        )} />
                        <FormField control={form.control} name="subjectTemplate" render={({ field }) => (
                          <FormItem>
                            <FormLabel>{t("itsm:sla.escalation.subjectTemplate")}</FormLabel>
                            <FormControl><Input {...field} value={field.value ?? ""} /></FormControl>
                            <FormMessage />
                          </FormItem>
                        )} />
                        <FormField control={form.control} name="bodyTemplate" render={({ field }) => (
                          <FormItem>
                            <FormLabel>{t("itsm:sla.escalation.bodyTemplate")}</FormLabel>
                            <FormControl><Textarea rows={4} {...field} value={field.value ?? ""} /></FormControl>
                            <FormMessage />
                          </FormItem>
                        )} />
                      </>
                    )}
                    {actionType === "reassign" && (
                      <FormField control={form.control} name="assigneeCandidates" render={({ field }) => (
                        <FormItem>
                          <FormLabel>{t("itsm:sla.escalation.assigneeCandidates")}</FormLabel>
                          <ParticipantPicker participants={field.value} onChange={field.onChange} />
                          <FormMessage />
                        </FormItem>
                      )} />
                    )}
                    {actionType === "escalate_priority" && (
                      <FormField control={form.control} name="priorityId" render={({ field }) => (
                        <FormItem>
                          <FormLabel>{t("itsm:sla.escalation.targetPriority")}</FormLabel>
                          <Select value={String(field.value || "")} onValueChange={(value) => field.onChange(Number(value))}>
                            <FormControl><SelectTrigger><SelectValue placeholder={t("itsm:sla.escalation.targetPriorityPlaceholder")} /></SelectTrigger></FormControl>
                            <SelectContent>
                              {priorities.map((priority) => (
                                <SelectItem key={priority.id} value={String(priority.id)}>{priority.name} / {priority.code}</SelectItem>
                              ))}
                            </SelectContent>
                          </Select>
                          <FormMessage />
                        </FormItem>
                      )} />
                    )}
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
      </TableCell>
    </TableRow>
  )
}

export function Component() {
  const { t } = useTranslation(["itsm", "common"])
  const queryClient = useQueryClient()
  const [formOpen, setFormOpen] = useState(false)
  const [editing, setEditing] = useState<SLATemplateItem | null>(null)
  const [expandedId, setExpandedId] = useState<number | null>(null)
  const [search, setSearch] = useState("")
  const slaSchema = useSLASchema()

  const canCreate = usePermission("itsm:sla:create")
  const canUpdate = usePermission("itsm:sla:update")
  const canDelete = usePermission("itsm:sla:delete")

  const { data: items = [], isLoading } = useQuery({
    queryKey: ["itsm-sla"],
    queryFn: () => fetchSLATemplates(),
  })

  const filteredItems = useMemo(() => {
    const query = search.trim().toLowerCase()
    return items.filter((item) => matchesQuery(item, query))
  }, [items, search])

  const form = useForm<SLAFormValues>({
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    resolver: zodResolver(slaSchema as any),
    defaultValues: { name: "", code: "", description: "", responseMinutes: 240, resolutionMinutes: 1440 },
  })

  useEffect(() => {
    if (formOpen) {
      if (editing) {
        form.reset({ name: editing.name, code: editing.code, description: editing.description, responseMinutes: editing.responseMinutes, resolutionMinutes: editing.resolutionMinutes })
      } else {
        form.reset({ name: "", code: "", description: "", responseMinutes: 240, resolutionMinutes: 1440 })
      }
    }
  }, [formOpen, editing, form])

  const createMut = useMutation({
    mutationFn: (v: SLAFormValues) => createSLATemplate({ ...v, description: v.description ?? "" }),
    onSuccess: () => { queryClient.invalidateQueries({ queryKey: ["itsm-sla"] }); setFormOpen(false); toast.success(t("itsm:sla.createSuccess")) },
    onError: (err) => toast.error(err.message),
  })

  const updateMut = useMutation({
    mutationFn: (v: SLAFormValues) => updateSLATemplate(editing!.id, { ...v, description: v.description ?? "" }),
    onSuccess: () => { queryClient.invalidateQueries({ queryKey: ["itsm-sla"] }); setFormOpen(false); toast.success(t("itsm:sla.updateSuccess")) },
    onError: (err) => toast.error(err.message),
  })

  const deleteMut = useMutation({
    mutationFn: (id: number) => deleteSLATemplate(id),
    onSuccess: () => { queryClient.invalidateQueries({ queryKey: ["itsm-sla"] }); toast.success(t("itsm:sla.deleteSuccess")) },
    onError: (err) => toast.error(err.message),
  })

  function onSubmit(v: SLAFormValues) {
    if (editing) {
      updateMut.mutate(v)
    } else {
      createMut.mutate(v)
    }
  }

  const isPending = createMut.isPending || updateMut.isPending
  const minuteUnit = t("itsm:sla.minuteShort")

  return (
    <div className="workspace-page">
      <div className="workspace-page-header">
        <div className="min-w-0">
          <h2 className="workspace-page-title">{t("itsm:sla.title")}</h2>
          <p className="workspace-page-description">{t("itsm:sla.pageDesc")}</p>
        </div>
        {canCreate && (
          <Button size="sm" onClick={() => { setEditing(null); setFormOpen(true) }}>
            <Plus className="mr-1.5 h-4 w-4" />{t("itsm:sla.create")}
          </Button>
        )}
      </div>

      <DataTableCard>
        <DataTableToolbar>
          <WorkspaceSearchField
            value={search}
            onChange={setSearch}
            placeholder={t("itsm:sla.searchPlaceholder")}
          />
          <span className="text-xs text-muted-foreground">
            {t("itsm:sla.listCount", { count: filteredItems.length })}
          </span>
        </DataTableToolbar>
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead className="w-[44px]" />
              <TableHead className="min-w-[180px]">{t("itsm:sla.name")}</TableHead>
              <TableHead className="w-[132px]">{t("itsm:sla.responseMinutes")}</TableHead>
              <TableHead className="w-[132px]">{t("itsm:sla.resolutionMinutes")}</TableHead>
              <TableHead className="w-[96px]">{t("common:status")}</TableHead>
              <DataTableActionsHead className="min-w-24">{t("common:actions")}</DataTableActionsHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {isLoading ? (
              <DataTableLoadingRow colSpan={6} />
            ) : items.length === 0 ? (
              <DataTableEmptyRow colSpan={6} icon={Timer} title={t("itsm:sla.empty")} description={canCreate ? t("itsm:sla.emptyHint") : undefined} />
            ) : filteredItems.length === 0 ? (
              <DataTableEmptyRow colSpan={6} icon={Timer} title={t("itsm:sla.searchEmpty")} />
            ) : (
              filteredItems.flatMap((item) => {
                const isExpanded = expandedId === item.id
                const rows = [
                  <TableRow key={item.id} className="cursor-pointer hover:bg-muted/22" onClick={() => setExpandedId(isExpanded ? null : item.id)}>
                    <TableCell className="w-[44px] px-2">
                      <Button
                        type="button"
                        variant="ghost"
                        size="icon-xs"
                        className="text-muted-foreground"
                        onClick={(event) => {
                          event.stopPropagation()
                          setExpandedId(isExpanded ? null : item.id)
                        }}
                      >
                        <ChevronRight className={cn("h-4 w-4 transition-transform", isExpanded && "rotate-90")} />
                        <span className="sr-only">{t("itsm:sla.escalations")}</span>
                      </Button>
                    </TableCell>
                    <TableCell>
                      <div className="min-w-0">
                        <div className="font-medium text-foreground/90">{item.name}</div>
                        <div className="mt-1 flex flex-wrap items-center gap-x-2 gap-y-1 text-xs text-muted-foreground">
                          <span className="font-mono">{item.code}</span>
                          {item.description ? <span className="truncate">{item.description}</span> : null}
                        </div>
                      </div>
                    </TableCell>
                    <TableCell className="text-sm tabular-nums">{formatMinutes(item.responseMinutes, minuteUnit)}</TableCell>
                    <TableCell className="text-sm tabular-nums">{formatMinutes(item.resolutionMinutes, minuteUnit)}</TableCell>
                    <TableCell>
                      <WorkspaceBooleanStatus active={item.isActive} activeLabel={t("itsm:sla.active")} inactiveLabel={t("itsm:sla.inactive")} />
                    </TableCell>
                    <DataTableActionsCell>
                      <DataTableActions>
                        {canUpdate && (
                          <WorkspaceIconAction
                            label={t("common:edit")}
                            icon={Pencil}
                            onClick={(event) => { event.stopPropagation(); setEditing(item); setFormOpen(true) }}
                          />
                        )}
                        {canDelete && (
                          <AlertDialog>
                            <span onClick={(event) => event.stopPropagation()}>
                              <WorkspaceAlertIconAction label={t("common:delete")} icon={Trash2} className="hover:text-destructive" />
                            </span>
                            <AlertDialogContent>
                              <AlertDialogHeader>
                                <AlertDialogTitle>{t("itsm:sla.deleteTitle")}</AlertDialogTitle>
                                <AlertDialogDescription>{t("itsm:sla.deleteDesc", { name: item.name })}</AlertDialogDescription>
                              </AlertDialogHeader>
                              <AlertDialogFooter>
                                <AlertDialogCancel size="sm">{t("common:cancel")}</AlertDialogCancel>
                                <AlertDialogAction size="sm" onClick={() => deleteMut.mutate(item.id)} disabled={deleteMut.isPending}>{t("itsm:sla.confirmDelete")}</AlertDialogAction>
                              </AlertDialogFooter>
                            </AlertDialogContent>
                          </AlertDialog>
                        )}
                      </DataTableActions>
                    </DataTableActionsCell>
                  </TableRow>,
                ]
                if (isExpanded) {
                  rows.push(<EscalationRules key={`esc-${item.id}`} slaId={item.id} />)
                }
                return rows
              })
            )}
          </TableBody>
        </Table>
      </DataTableCard>

      <Sheet open={formOpen} onOpenChange={setFormOpen}>
        <SheetContent className="sm:max-w-lg">
          <SheetHeader>
            <SheetTitle>{editing ? t("itsm:sla.edit") : t("itsm:sla.create")}</SheetTitle>
            <SheetDescription>{t("itsm:sla.sheetDesc")}</SheetDescription>
          </SheetHeader>
          <Form {...form}>
            <form onSubmit={form.handleSubmit(onSubmit)} className="flex flex-1 flex-col gap-5 px-4">
              <WorkspaceFormSection title={t("itsm:sla.formIdentity")}>
                <FormField control={form.control} name="name" render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t("itsm:sla.name")}</FormLabel>
                    <FormControl><Input placeholder={t("itsm:sla.namePlaceholder")} {...field} /></FormControl>
                    <FormMessage />
                  </FormItem>
                )} />
                <FormField control={form.control} name="code" render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t("itsm:sla.code")}</FormLabel>
                    <FormControl><Input placeholder={t("itsm:sla.codePlaceholder")} {...field} /></FormControl>
                    <FormMessage />
                  </FormItem>
                )} />
              </WorkspaceFormSection>
              <WorkspaceFormSection title={t("itsm:sla.formCommitment")}>
                <div className="grid grid-cols-2 gap-4">
                  <FormField control={form.control} name="responseMinutes" render={({ field }) => (
                    <FormItem>
                      <FormLabel>{t("itsm:sla.responseMinutes")}</FormLabel>
                      <FormControl><Input type="number" {...field} onChange={(e) => field.onChange(Number(e.target.value))} /></FormControl>
                      <FormMessage />
                    </FormItem>
                  )} />
                  <FormField control={form.control} name="resolutionMinutes" render={({ field }) => (
                    <FormItem>
                      <FormLabel>{t("itsm:sla.resolutionMinutes")}</FormLabel>
                      <FormControl><Input type="number" {...field} onChange={(e) => field.onChange(Number(e.target.value))} /></FormControl>
                      <FormMessage />
                    </FormItem>
                  )} />
                </div>
              </WorkspaceFormSection>
              <WorkspaceFormSection title={t("itsm:sla.formDescription")}>
                <FormField control={form.control} name="description" render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t("itsm:sla.description")}</FormLabel>
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
