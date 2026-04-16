import { useState } from "react"
import { useNavigate } from "react-router"
import { useTranslation } from "react-i18next"
import { Plus, Search, FileBadge, Ban, Download, Eye, Clock, ArrowUpCircle, Pause, Play } from "lucide-react"
import { useMutation, useQueryClient } from "@tanstack/react-query"
import { usePermission } from "@/hooks/use-permission"
import { useListPage } from "@/hooks/use-list-page"
import { api } from "@/lib/api"
import { toast } from "sonner"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Badge } from "@/components/ui/badge"
import {
  DataTableActions,
  DataTableActionsCell,
  DataTableActionsHead,
  DataTableCard,
  DataTableEmptyRow,
  DataTableLoadingRow,
  DataTablePagination,
  DataTableToolbar,
  DataTableToolbarGroup,
} from "@/components/ui/data-table"
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table"
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select"
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from "@/components/ui/alert-dialog"
import { formatDateTime } from "@/lib/utils"
import { IssueLicenseSheet } from "../../components/issue-license-sheet"
import { UpgradeLicenseSheet } from "../../components/upgrade-license-sheet"
import { RenewLicenseSheet } from "../../components/renew-license-sheet"

export interface LicenseItem {
  id: number
  productId: number | null
  licenseeId: number | null
  planName: string
  registrationCode: string
  status: string
  lifecycleStatus: string
  validFrom: string
  validUntil: string | null
  productName: string
  licenseeName: string
  createdAt: string
}

const LIFECYCLE_VARIANTS: Record<string, "default" | "secondary" | "destructive" | "outline"> = {
  pending: "secondary",
  active: "default",
  expired: "outline",
  suspended: "secondary",
  revoked: "destructive",
}

export function Component() {
  const { t } = useTranslation(["license", "common"])
  const navigate = useNavigate()
  const queryClient = useQueryClient()
  const [formOpen, setFormOpen] = useState(false)
  const [statusFilter, setStatusFilter] = useState("")
  const [actionTarget, setActionTarget] = useState<LicenseItem | null>(null)
  const [actionType, setActionType] = useState<"revoke" | "suspend" | "reactivate" | null>(null)
  const [upgradeTarget, setUpgradeTarget] = useState<LicenseItem | null>(null)
  const [renewTarget, setRenewTarget] = useState<LicenseItem | null>(null)

  const canIssue = usePermission("license:license:issue")
  const canRevoke = usePermission("license:license:revoke")
  const canRenew = usePermission("license:license:renew")
  const canUpgrade = usePermission("license:license:upgrade")
  const canSuspend = usePermission("license:license:suspend")
  const canReactivate = usePermission("license:license:reactivate")

  const {
    keyword, setKeyword, page, setPage,
    items: licenses, total, totalPages, isLoading, handleSearch,
  } = useListPage<LicenseItem>({
    queryKey: "license-licenses",
    endpoint: "/api/v1/license/licenses",
    extraParams: statusFilter ? { lifecycleStatus: statusFilter } : undefined,
  })

  const actionMutation = useMutation({
    mutationFn: ({ id, action }: { id: number; action: string }) => {
      return api.post(`/api/v1/license/licenses/${id}/${action}`)
    },
    onSuccess: (_data, variables) => {
      queryClient.invalidateQueries({ queryKey: ["license-licenses"] })
      setActionTarget(null)
      setActionType(null)
      const keyMap: Record<string, string> = {
        revoke: "revokeSuccess",
        suspend: "suspendSuccess",
        reactivate: "reactivateSuccess",
      }
      toast.success(t(`license:licenses.${keyMap[variables.action]}`))
    },
    onError: (err) => toast.error(err.message),
  })

  function handleStatusFilter(value: string) {
    setStatusFilter(value === "all" ? "" : value)
    setPage(1)
  }

  async function handleExport(item: LicenseItem) {
    try {
      const blob = await api.download(`/api/v1/license/licenses/${item.id}/export`)
      const url = URL.createObjectURL(blob)
      const anchor = document.createElement("a")
      anchor.href = url
      anchor.download = `${item.productName || "license"}_${item.id}.lic`
      document.body.appendChild(anchor)
      anchor.click()
      anchor.remove()
      URL.revokeObjectURL(url)
    } catch (err) {
      toast.error(err instanceof Error ? err.message : t("license:licenses.exportFailed"))
    }
  }

  function openAction(item: LicenseItem, type: "revoke" | "suspend" | "reactivate") {
    setActionTarget(item)
    setActionType(type)
  }

  function confirmAction() {
    if (!actionTarget || !actionType) return
    actionMutation.mutate({ id: actionTarget.id, action: actionType })
  }

  const actionTitle = actionType ? t(`license:licenses.${actionType}Title`) : ""
  const actionDesc = actionType ? t(`license:licenses.${actionType}Desc`) : ""

  function isActionableLifecycle(status: string) {
    return status === "active" || status === "pending" || status === "expired"
  }

  function renderRowActions(item: LicenseItem) {
    const actionable = isActionableLifecycle(item.lifecycleStatus)

    return (
      <>
        <Button
          key="detail"
          variant="ghost"
          size="sm"
          className="px-2.5"
          onClick={() => navigate(`/license/licenses/${item.id}`)}
        >
          <Eye className="mr-1 h-3.5 w-3.5" />
          {t("license:licenses.detail")}
        </Button>

        {actionable && (
          <Button
            key="export"
            variant="ghost"
            size="sm"
            className="px-2.5"
            onClick={() => handleExport(item)}
          >
            <Download className="mr-1 h-3.5 w-3.5" />
            {t("common:export")}
          </Button>
        )}

        {actionable && canRenew && (
          <Button
            key="renew"
            variant="ghost"
            size="sm"
            className="px-2.5"
            onClick={() => setRenewTarget(item)}
          >
            <Clock className="mr-1 h-3.5 w-3.5" />
            {t("license:licenses.renew")}
          </Button>
        )}

        {actionable && canUpgrade && (
          <Button
            key="upgrade"
            variant="ghost"
            size="sm"
            className="px-2.5"
            onClick={() => setUpgradeTarget(item)}
          >
            <ArrowUpCircle className="mr-1 h-3.5 w-3.5" />
            {t("license:licenses.upgrade")}
          </Button>
        )}

        {actionable && canSuspend && (
          <Button
            key="suspend"
            variant="ghost"
            size="sm"
            className="px-2.5 text-amber-600 hover:text-amber-600"
            onClick={() => openAction(item, "suspend")}
          >
            <Pause className="mr-1 h-3.5 w-3.5" />
            {t("license:licenses.suspend")}
          </Button>
        )}

        {actionable && canRevoke && (
          <Button
            key="revoke"
            variant="ghost"
            size="sm"
            className="px-2.5 text-destructive hover:text-destructive"
            onClick={() => openAction(item, "revoke")}
          >
            <Ban className="mr-1 h-3.5 w-3.5" />
            {t("license:licenses.revoke")}
          </Button>
        )}

        {item.lifecycleStatus === "suspended" && canReactivate && (
          <Button
            key="reactivate"
            variant="ghost"
            size="sm"
            className="px-2.5 text-green-600 hover:text-green-600"
            onClick={() => openAction(item, "reactivate")}
          >
            <Play className="mr-1 h-3.5 w-3.5" />
            {t("license:licenses.reactivate")}
          </Button>
        )}
      </>
    )
  }

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <h2 className="text-lg font-semibold">{t("license:licenses.title")}</h2>
        {canIssue && (
          <Button size="sm" onClick={() => setFormOpen(true)}>
            <Plus className="mr-1.5 h-4 w-4" />
            {t("license:licenses.issue")}
          </Button>
        )}
      </div>

      <DataTableToolbar>
        <DataTableToolbarGroup>
          <form onSubmit={handleSearch} className="flex w-full flex-col gap-2 sm:flex-row sm:items-center">
            <div className="relative w-full sm:max-w-sm">
              <Search className="absolute left-2.5 top-2.5 h-4 w-4 text-muted-foreground" />
              <Input
                placeholder={t("license:licenses.searchPlaceholder")}
                value={keyword}
                onChange={(e) => setKeyword(e.target.value)}
                className="pl-8"
              />
            </div>
            <Select value={statusFilter || "all"} onValueChange={handleStatusFilter}>
              <SelectTrigger className="w-full sm:w-[130px]">
                <SelectValue placeholder={t("license:licenses.allStatus")} />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="all">{t("license:licenses.allStatus")}</SelectItem>
                <SelectItem value="pending">{t("license:lifecycleStatus.pending")}</SelectItem>
                <SelectItem value="active">{t("license:lifecycleStatus.active")}</SelectItem>
                <SelectItem value="expired">{t("license:lifecycleStatus.expired")}</SelectItem>
                <SelectItem value="suspended">{t("license:lifecycleStatus.suspended")}</SelectItem>
                <SelectItem value="revoked">{t("license:lifecycleStatus.revoked")}</SelectItem>
              </SelectContent>
            </Select>
            <Button type="submit" variant="outline">
              {t("common:search")}
            </Button>
          </form>
        </DataTableToolbarGroup>
      </DataTableToolbar>

      <DataTableCard>
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead className="min-w-[120px]">{t("license:licenses.plan")}</TableHead>
              <TableHead className="min-w-[100px]">{t("license:licenses.product")}</TableHead>
              <TableHead className="min-w-[100px]">{t("license:licenses.licensee")}</TableHead>
              <TableHead className="min-w-[120px]">{t("license:licenses.registrationCode")}</TableHead>
              <TableHead className="w-[80px]">{t("license:lifecycleStatus.title")}</TableHead>
              <TableHead className="w-[100px]">{t("license:licenses.validFrom")}</TableHead>
              <TableHead className="w-[100px]">{t("license:licenses.validUntil")}</TableHead>
              <TableHead className="w-[150px]">{t("license:licenses.issuedAt")}</TableHead>
              <DataTableActionsHead className="min-w-[180px]">{t("common:actions")}</DataTableActionsHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {isLoading ? (
              <DataTableLoadingRow colSpan={9} />
            ) : licenses.length === 0 ? (
              <DataTableEmptyRow
                colSpan={9}
                icon={FileBadge}
                title={t("license:licenses.empty")}
                description={canIssue ? t("license:licenses.emptyHint") : undefined}
              />
            ) : (
              licenses.map((item) => {
                const variant = LIFECYCLE_VARIANTS[item.lifecycleStatus] ?? ("outline" as const)
                const statusKey = item.lifecycleStatus as string
                return (
                  <TableRow key={item.id}>
                    <TableCell className="font-medium">{item.planName}</TableCell>
                    <TableCell className="text-sm">{item.productName || "-"}</TableCell>
                    <TableCell className="text-sm">{item.licenseeName || "-"}</TableCell>
                    <TableCell className="text-sm font-mono text-xs">{item.registrationCode}</TableCell>
                    <TableCell>
                      <Badge variant={variant}>{t(`license:lifecycleStatus.${statusKey}`, statusKey)}</Badge>
                    </TableCell>
                    <TableCell className="text-sm text-muted-foreground whitespace-nowrap">
                      {item.validFrom ? formatDateTime(item.validFrom, { dateOnly: true }) : "-"}
                    </TableCell>
                    <TableCell className="text-sm text-muted-foreground whitespace-nowrap">
                      {item.validUntil ? formatDateTime(item.validUntil, { dateOnly: true }) : t("license:licenses.permanent")}
                    </TableCell>
                    <TableCell className="text-sm text-muted-foreground whitespace-nowrap">
                      {formatDateTime(item.createdAt)}
                    </TableCell>
                    <DataTableActionsCell>
                      <DataTableActions>
                        {renderRowActions(item)}
                      </DataTableActions>
                    </DataTableActionsCell>
                  </TableRow>
                )
              })
            )}
          </TableBody>
        </Table>
      </DataTableCard>

      <DataTablePagination
        total={total}
        page={page}
        totalPages={totalPages}
        onPageChange={setPage}
      />

      <IssueLicenseSheet open={formOpen} onOpenChange={setFormOpen} />

      <UpgradeLicenseSheet license={upgradeTarget as any} open={!!upgradeTarget} onOpenChange={() => setUpgradeTarget(null)} />

      <RenewLicenseSheet license={renewTarget} open={!!renewTarget} onOpenChange={() => setRenewTarget(null)} />

      <AlertDialog open={actionTarget !== null} onOpenChange={(open) => { if (!open) { setActionTarget(null); setActionType(null) } }}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>{actionTitle}</AlertDialogTitle>
            <AlertDialogDescription>{actionDesc}</AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>{t("common:cancel")}</AlertDialogCancel>
            <AlertDialogAction
              onClick={confirmAction}
              disabled={actionMutation.isPending}
            >
              {actionMutation.isPending ? t("common:processing") : t("common:confirm")}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  )
}
