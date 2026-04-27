"use client"

import { useState } from "react"
import { useTranslation } from "react-i18next"
import { useForm } from "react-hook-form"
import { z } from "zod"
import { zodResolver } from "@hookform/resolvers/zod"
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import { ShieldAlert, ArrowRightLeft, UserRoundCog, RotateCcw } from "lucide-react"
import { toast } from "sonner"
import { Button } from "@/components/ui/button"
import { Textarea } from "@/components/ui/textarea"
import {
  DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu"
import {
  Sheet, SheetContent, SheetHeader, SheetTitle, SheetDescription, SheetFooter,
} from "@/components/ui/sheet"
import {
  Select, SelectContent, SelectItem, SelectTrigger, SelectValue,
} from "@/components/ui/select"
import {
  Form, FormControl, FormField, FormItem, FormLabel, FormMessage,
} from "@/components/ui/form"
import {
  AlertDialog, AlertDialogAction, AlertDialogCancel, AlertDialogContent,
  AlertDialogDescription, AlertDialogFooter, AlertDialogHeader, AlertDialogTitle,
} from "@/components/ui/alert-dialog"
import { cn } from "@/lib/utils"
import { overrideJump, overrideReassign, retryAI, fetchUsers } from "../api"

const STEP_TYPES = ["form", "process", "action", "notify", "wait"]

interface OverrideActionsProps {
  ticketId: number
  currentActivityId: number | null
  aiFailureCount?: number
  triggerClassName?: string
  onSuccess?: () => void
}

export function OverrideActions({ ticketId, currentActivityId, aiFailureCount, triggerClassName, onSuccess }: OverrideActionsProps) {
  const { t } = useTranslation("itsm")
  const queryClient = useQueryClient()
  const [jumpOpen, setJumpOpen] = useState(false)
  const [reassignOpen, setReassignOpen] = useState(false)
  const [retryDialogOpen, setRetryDialogOpen] = useState(false)
  const [retryReason, setRetryReason] = useState("")

  const { data: users = [] } = useQuery({
    queryKey: ["users-for-override"],
    queryFn: () => fetchUsers(),
    enabled: jumpOpen || reassignOpen,
  })

  const invalidateAll = () => {
    queryClient.invalidateQueries({ queryKey: ["itsm-ticket", ticketId] })
    queryClient.invalidateQueries({ queryKey: ["itsm-ticket-activities", ticketId] })
    queryClient.invalidateQueries({ queryKey: ["itsm-ticket-timeline", ticketId] })
  }

  const handleMutationSuccess = () => {
    invalidateAll()
    onSuccess?.()
  }

  // Jump
  const jumpSchema = z.object({
    activityType: z.string().min(1),
    assigneeId: z.number().optional(),
    reason: z.string().min(1),
  })
  const jumpForm = useForm<z.infer<typeof jumpSchema>>({
    resolver: zodResolver(jumpSchema),
    defaultValues: { activityType: "", reason: "" },
  })

  const jumpMut = useMutation({
    mutationFn: (v: z.infer<typeof jumpSchema>) => overrideJump(ticketId, v),
    onSuccess: () => { handleMutationSuccess(); setJumpOpen(false); toast.success(t("smart.jumpSuccess")) },
    onError: (err) => toast.error(err.message),
  })

  // Reassign
  const reassignSchema = z.object({
    activityId: z.number().min(1),
    newAssigneeId: z.number().min(1),
    reason: z.string().min(1),
  })
  const reassignForm = useForm<z.infer<typeof reassignSchema>>({
    resolver: zodResolver(reassignSchema),
    defaultValues: { activityId: currentActivityId ?? 0, newAssigneeId: 0, reason: "" },
  })

  const reassignMut = useMutation({
    mutationFn: (v: z.infer<typeof reassignSchema>) => overrideReassign(ticketId, v),
    onSuccess: () => { handleMutationSuccess(); setReassignOpen(false); toast.success(t("smart.reassignSuccess")) },
    onError: (err) => toast.error(err.message),
  })

  // Retry AI
  const retryMut = useMutation({
    mutationFn: () => retryAI(ticketId, retryReason.trim()),
    onSuccess: () => { handleMutationSuccess(); setRetryDialogOpen(false); setRetryReason(""); toast.success(t("smart.retrySuccess")) },
    onError: (err) => toast.error(err.message),
  })

  return (
    <>
      <DropdownMenu>
        <DropdownMenuTrigger asChild>
          <Button variant="outline" size="sm" className={cn(triggerClassName)}>
            <span className="grid w-[5.25rem] grid-cols-[0.875rem_minmax(0,1fr)] items-center gap-2 text-left text-[11px] leading-none">
              <ShieldAlert className="h-3.5 w-3.5 shrink-0 justify-self-center" />
              <span className="truncate font-medium">{t("smart.override")}</span>
            </span>
          </Button>
        </DropdownMenuTrigger>
        <DropdownMenuContent align="end">
          <DropdownMenuItem onClick={() => { jumpForm.reset({ activityType: "", reason: "" }); setJumpOpen(true) }}>
            <ArrowRightLeft className="mr-2 h-4 w-4" />
            {t("smart.jump")}
          </DropdownMenuItem>
          <DropdownMenuItem
            onClick={() => { reassignForm.reset({ activityId: currentActivityId ?? 0, newAssigneeId: 0, reason: "" }); setReassignOpen(true) }}
            disabled={!currentActivityId}
          >
            <UserRoundCog className="mr-2 h-4 w-4" />
            {t("smart.reassign")}
          </DropdownMenuItem>
          {(aiFailureCount ?? 0) > 0 && (
            <DropdownMenuItem onClick={() => { setRetryReason(""); setRetryDialogOpen(true) }} disabled={retryMut.isPending}>
              <RotateCcw className="mr-2 h-4 w-4" />
              {t("smart.retryAI")}
            </DropdownMenuItem>
          )}
        </DropdownMenuContent>
      </DropdownMenu>

      {/* Jump Sheet */}
      <Sheet open={jumpOpen} onOpenChange={setJumpOpen}>
        <SheetContent className="sm:max-w-md">
          <SheetHeader>
            <SheetTitle>{t("smart.jump")}</SheetTitle>
            <SheetDescription className="sr-only">{t("smart.jump")}</SheetDescription>
          </SheetHeader>
          <Form {...jumpForm}>
            <form onSubmit={jumpForm.handleSubmit((v) => jumpMut.mutate(v))} className="flex flex-1 flex-col gap-5 px-4">
              <FormField control={jumpForm.control} name="activityType" render={({ field }) => (
                <FormItem>
                  <FormLabel>{t("smart.stepType")}</FormLabel>
                  <Select onValueChange={field.onChange} value={field.value}>
                    <FormControl><SelectTrigger><SelectValue placeholder={t("smart.stepTypePlaceholder")} /></SelectTrigger></FormControl>
                    <SelectContent>
                      {STEP_TYPES.map((st) => (
                        <SelectItem key={st} value={st}>{t(`smart.stepType_${st}`, { defaultValue: st })}</SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                  <FormMessage />
                </FormItem>
              )} />
              <FormField control={jumpForm.control} name="assigneeId" render={({ field }) => (
                <FormItem>
                  <FormLabel>{t("smart.participant")}</FormLabel>
                  <Select onValueChange={(v) => field.onChange(Number(v))} value={field.value ? String(field.value) : ""}>
                    <FormControl><SelectTrigger><SelectValue placeholder={t("smart.participantPlaceholder")} /></SelectTrigger></FormControl>
                    <SelectContent>
                      {users.map((u) => (
                        <SelectItem key={u.id} value={String(u.id)}>{u.username}</SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                  <FormMessage />
                </FormItem>
              )} />
              <FormField control={jumpForm.control} name="reason" render={({ field }) => (
                <FormItem>
                  <FormLabel>{t("smart.overrideReason")}</FormLabel>
                  <FormControl><Textarea rows={3} placeholder={t("smart.overrideReasonPlaceholder")} {...field} /></FormControl>
                  <FormMessage />
                </FormItem>
              )} />
              <SheetFooter>
                <Button type="submit" size="sm" disabled={jumpMut.isPending}>
                  {jumpMut.isPending ? t("common:saving") : t("smart.confirmJump")}
                </Button>
              </SheetFooter>
            </form>
          </Form>
        </SheetContent>
      </Sheet>

      {/* Reassign Sheet */}
      <Sheet open={reassignOpen} onOpenChange={setReassignOpen}>
        <SheetContent className="sm:max-w-md">
          <SheetHeader>
            <SheetTitle>{t("smart.reassign")}</SheetTitle>
            <SheetDescription className="sr-only">{t("smart.reassign")}</SheetDescription>
          </SheetHeader>
          <Form {...reassignForm}>
            <form onSubmit={reassignForm.handleSubmit((v) => reassignMut.mutate(v))} className="flex flex-1 flex-col gap-5 px-4">
              <FormField control={reassignForm.control} name="newAssigneeId" render={({ field }) => (
                <FormItem>
                  <FormLabel>{t("smart.newAssignee")}</FormLabel>
                  <Select onValueChange={(v) => field.onChange(Number(v))} value={field.value ? String(field.value) : ""}>
                    <FormControl><SelectTrigger><SelectValue placeholder={t("smart.newAssigneePlaceholder")} /></SelectTrigger></FormControl>
                    <SelectContent>
                      {users.map((u) => (
                        <SelectItem key={u.id} value={String(u.id)}>{u.username}</SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                  <FormMessage />
                </FormItem>
              )} />
              <FormField control={reassignForm.control} name="reason" render={({ field }) => (
                <FormItem>
                  <FormLabel>{t("smart.overrideReason")}</FormLabel>
                  <FormControl><Textarea rows={3} placeholder={t("smart.overrideReasonPlaceholder")} {...field} /></FormControl>
                  <FormMessage />
                </FormItem>
              )} />
              <SheetFooter>
                <Button type="submit" size="sm" disabled={reassignMut.isPending}>
                  {reassignMut.isPending ? t("common:saving") : t("smart.confirmReassign")}
                </Button>
              </SheetFooter>
            </form>
          </Form>
        </SheetContent>
      </Sheet>

      {/* Retry AI Confirmation Dialog */}
      <AlertDialog open={retryDialogOpen} onOpenChange={setRetryDialogOpen}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>{t("smart.retryAI")}</AlertDialogTitle>
            <AlertDialogDescription>
              {t("smart.retryAIConfirmDesc", { defaultValue: "将重置 AI 失败计数并重新触发决策，请填写重试原因。" })}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <Textarea
            value={retryReason}
            onChange={(event) => setRetryReason(event.target.value)}
            rows={3}
            placeholder={t("smart.overrideReasonPlaceholder")}
          />
          <AlertDialogFooter>
            <AlertDialogCancel>{t("common:cancel")}</AlertDialogCancel>
            <AlertDialogAction onClick={() => retryMut.mutate()} disabled={!retryReason.trim() || retryMut.isPending}>
              {t("smart.retryAI")}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </>
  )
}
