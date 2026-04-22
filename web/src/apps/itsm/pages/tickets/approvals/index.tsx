"use client"

import { useState } from "react"
import { useTranslation } from "react-i18next"
import { useNavigate } from "react-router"
import { CheckCircle, X, Bot } from "lucide-react"
import { useMutation, useQueryClient } from "@tanstack/react-query"
import { toast } from "sonner"
import { useListPage } from "@/hooks/use-list-page"
import { withActiveMenuPermission } from "@/lib/navigation-state"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Badge } from "@/components/ui/badge"
import {
  Popover, PopoverContent, PopoverTrigger,
} from "@/components/ui/popover"
import {
  DataTableCard, DataTableEmptyRow, DataTableLoadingRow, DataTablePagination,
} from "@/components/ui/data-table"
import {
  Table, TableBody, TableCell, TableHead, TableHeader, TableRow,
} from "@/components/ui/table"
import { type ApprovalItem, approveActivity, denyActivity, confirmActivity, rejectActivity } from "../../../api"
import { SLABadge } from "../../../components/sla-badge"
import { TICKET_MENU_PERMISSION } from "../navigation"

export function Component() {
  const { t } = useTranslation(["itsm", "common"])
  const navigate = useNavigate()
  const queryClient = useQueryClient()

  const {
    page, setPage, items, total, totalPages, isLoading,
  } = useListPage<ApprovalItem>({
    queryKey: "itsm-approvals",
    endpoint: "/api/v1/itsm/tickets/approvals",
  })

  const approveMut = useMutation({
    mutationFn: (item: ApprovalItem) => approveActivity(item.ticketId, item.activityId),
    onSuccess: () => {
      toast.success(t("itsm:approval.approveSuccess"))
      queryClient.invalidateQueries({ queryKey: ["itsm-approvals"] })
    },
  })

  const denyMut = useMutation({
    mutationFn: ({ item, reason }: { item: ApprovalItem; reason?: string }) =>
      denyActivity(item.ticketId, item.activityId, reason),
    onSuccess: () => {
      toast.success(t("itsm:approval.denySuccess"))
      queryClient.invalidateQueries({ queryKey: ["itsm-approvals"] })
    },
  })

  const confirmMut = useMutation({
    mutationFn: (item: ApprovalItem) => confirmActivity(item.ticketId, item.activityId),
    onSuccess: () => {
      toast.success(t("itsm:smart.confirmSuccess"))
      queryClient.invalidateQueries({ queryKey: ["itsm-approvals"] })
    },
  })

  const rejectMut = useMutation({
    mutationFn: ({ item, reason }: { item: ApprovalItem; reason?: string }) =>
      rejectActivity(item.ticketId, item.activityId, reason ?? ""),
    onSuccess: () => {
      toast.success(t("itsm:smart.rejectSuccess"))
      queryClient.invalidateQueries({ queryKey: ["itsm-approvals"] })
    },
  })

  return (
    <div className="space-y-4">
      <h2 className="text-lg font-semibold">{t("itsm:approval.title")}</h2>

      <DataTableCard>
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead className="w-[120px]">{t("itsm:tickets.code")}</TableHead>
              <TableHead className="min-w-[180px]">{t("itsm:tickets.ticketTitle")}</TableHead>
              <TableHead className="w-[100px]">{t("itsm:tickets.priority")}</TableHead>
              <TableHead className="w-[100px]">{t("itsm:tickets.service")}</TableHead>
              <TableHead className="w-[120px]">{t("itsm:approval.activityName")}</TableHead>
              <TableHead className="w-[100px]">{t("itsm:tickets.slaStatus")}</TableHead>
              <TableHead className="w-[140px]">{t("itsm:tickets.createdAt")}</TableHead>
              <TableHead className="w-[140px] text-right">{t("common:actions")}</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {isLoading ? (
              <DataTableLoadingRow colSpan={8} />
            ) : items.length === 0 ? (
              <DataTableEmptyRow colSpan={8} icon={CheckCircle} title={t("itsm:approval.empty")} />
            ) : (
              items.map((item) =>
                item.approvalKind === "ai_confirm" ? (
                  <AIConfirmRow
                    key={`${item.ticketId}-${item.activityId}`}
                    item={item}
                    t={t}
                    onConfirm={() => confirmMut.mutate(item)}
                    onReject={(reason) => rejectMut.mutate({ item, reason })}
                    confirming={confirmMut.isPending}
                    rejecting={rejectMut.isPending}
                    onNavigate={() => navigate(`/itsm/tickets/${item.ticketId}`, { state: withActiveMenuPermission(TICKET_MENU_PERMISSION.approvals) })}
                  />
                ) : (
                  <ApprovalRow
                    key={`${item.ticketId}-${item.activityId}`}
                    item={item}
                    t={t}
                    onApprove={() => approveMut.mutate(item)}
                    onDeny={(reason) => denyMut.mutate({ item, reason })}
                    approving={approveMut.isPending}
                    denying={denyMut.isPending}
                    onNavigate={() => navigate(`/itsm/tickets/${item.ticketId}`, { state: withActiveMenuPermission(TICKET_MENU_PERMISSION.approvals) })}
                  />
                ),
              )
            )}
          </TableBody>
        </Table>
      </DataTableCard>

      <DataTablePagination total={total} page={page} totalPages={totalPages} onPageChange={setPage} />
    </div>
  )
}

function ApprovalRow({
  item, t, onApprove, onDeny, approving, denying, onNavigate,
}: {
  item: ApprovalItem
  t: (key: string) => string
  onApprove: () => void
  onDeny: (reason?: string) => void
  approving: boolean
  denying: boolean
  onNavigate: () => void
}) {
  const [reason, setReason] = useState("")
  const [open, setOpen] = useState(false)

  return (
    <TableRow>
      <TableCell className="font-mono text-sm cursor-pointer hover:underline" onClick={onNavigate}>
        {item.ticketCode}
      </TableCell>
      <TableCell className="font-medium cursor-pointer hover:underline" onClick={onNavigate}>
        {item.ticketTitle}
      </TableCell>
      <TableCell>
        <span className="inline-flex items-center gap-1.5 text-sm">
          <span className="inline-block h-2.5 w-2.5 rounded-full" style={{ backgroundColor: item.priorityColor }} />
          {item.priorityName}
        </span>
      </TableCell>
      <TableCell className="text-sm text-muted-foreground">{item.serviceName}</TableCell>
      <TableCell className="text-sm">{item.activityName}</TableCell>
      <TableCell>
        <SLABadge slaStatus={item.slaStatus} slaResolutionDeadline={item.slaResolutionDeadline} />
      </TableCell>
      <TableCell className="text-sm text-muted-foreground">{new Date(item.createdAt).toLocaleString()}</TableCell>
      <TableCell className="text-right">
        <div className="flex items-center justify-end gap-1">
          <Button
            size="sm"
            variant="default"
            disabled={approving || denying}
            onClick={(e) => { e.stopPropagation(); onApprove() }}
          >
            <CheckCircle className="mr-1 h-3.5 w-3.5" />
            {t("itsm:approval.approve")}
          </Button>
          <Popover open={open} onOpenChange={setOpen}>
            <PopoverTrigger asChild>
              <Button
                size="sm"
                variant="destructive"
                disabled={approving || denying}
                onClick={(e) => e.stopPropagation()}
              >
                <X className="mr-1 h-3.5 w-3.5" />
                {t("itsm:approval.deny")}
              </Button>
            </PopoverTrigger>
            <PopoverContent className="w-72" onClick={(e) => e.stopPropagation()}>
              <div className="space-y-2">
                <p className="text-sm font-medium">{t("itsm:approval.reason")}</p>
                <Input
                  value={reason}
                  onChange={(e) => setReason(e.target.value)}
                  placeholder={t("itsm:approval.reasonPlaceholder")}
                />
                <Button
                  size="sm"
                  variant="destructive"
                  className="w-full"
                  disabled={denying}
                  onClick={() => {
                    onDeny(reason || undefined)
                    setOpen(false)
                    setReason("")
                  }}
                >
                  {t("itsm:approval.confirmDeny")}
                </Button>
              </div>
            </PopoverContent>
          </Popover>
        </div>
      </TableCell>
    </TableRow>
  )
}

function AIConfirmRow({
  item, t, onConfirm, onReject, confirming, rejecting, onNavigate,
}: {
  item: ApprovalItem
  t: (key: string) => string
  onConfirm: () => void
  onReject: (reason?: string) => void
  confirming: boolean
  rejecting: boolean
  onNavigate: () => void
}) {
  const [reason, setReason] = useState("")
  const [open, setOpen] = useState(false)

  return (
    <TableRow className="bg-amber-50/50">
      <TableCell className="font-mono text-sm cursor-pointer hover:underline" onClick={onNavigate}>
        <span className="inline-flex items-center gap-1">
          <Bot className="h-3.5 w-3.5 text-amber-600" />
          {item.ticketCode}
        </span>
      </TableCell>
      <TableCell className="font-medium cursor-pointer hover:underline" onClick={onNavigate}>
        {item.ticketTitle}
      </TableCell>
      <TableCell>
        <span className="inline-flex items-center gap-1.5 text-sm">
          <span className="inline-block h-2.5 w-2.5 rounded-full" style={{ backgroundColor: item.priorityColor }} />
          {item.priorityName}
        </span>
      </TableCell>
      <TableCell className="text-sm text-muted-foreground">{item.serviceName}</TableCell>
      <TableCell>
        <div className="flex items-center gap-1.5 text-sm">
          <Badge variant="outline" className="border-amber-300 bg-amber-50 text-amber-700">
            {t("itsm:smart.aiDecision")}
          </Badge>
          {item.aiConfidence > 0 && (
            <span className="text-xs text-muted-foreground">{Math.round(item.aiConfidence * 100)}%</span>
          )}
        </div>
      </TableCell>
      <TableCell>
        <SLABadge slaStatus={item.slaStatus} slaResolutionDeadline={item.slaResolutionDeadline} />
      </TableCell>
      <TableCell className="text-sm text-muted-foreground">{new Date(item.createdAt).toLocaleString()}</TableCell>
      <TableCell className="text-right">
        <div className="flex items-center justify-end gap-1">
          <Button
            size="sm"
            variant="default"
            disabled={confirming || rejecting}
            onClick={(e) => { e.stopPropagation(); onConfirm() }}
          >
            <CheckCircle className="mr-1 h-3.5 w-3.5" />
            {t("itsm:smart.confirm")}
          </Button>
          <Popover open={open} onOpenChange={setOpen}>
            <PopoverTrigger asChild>
              <Button
                size="sm"
                variant="outline"
                disabled={confirming || rejecting}
                onClick={(e) => e.stopPropagation()}
              >
                <X className="mr-1 h-3.5 w-3.5" />
                {t("itsm:smart.reject")}
              </Button>
            </PopoverTrigger>
            <PopoverContent className="w-72" onClick={(e) => e.stopPropagation()}>
              <div className="space-y-2">
                <p className="text-sm font-medium">{t("itsm:approval.reason")}</p>
                <Input
                  value={reason}
                  onChange={(e) => setReason(e.target.value)}
                  placeholder={t("itsm:approval.reasonPlaceholder")}
                />
                <Button
                  size="sm"
                  variant="destructive"
                  className="w-full"
                  disabled={rejecting}
                  onClick={() => {
                    onReject(reason || undefined)
                    setOpen(false)
                    setReason("")
                  }}
                >
                  {t("itsm:approval.confirmDeny")}
                </Button>
              </div>
            </PopoverContent>
          </Popover>
        </div>
      </TableCell>
    </TableRow>
  )
}
