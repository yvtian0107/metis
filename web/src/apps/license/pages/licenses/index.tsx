import { useState } from "react"
import { useNavigate } from "react-router"
import { useTranslation } from "react-i18next"
import { Plus, Search, FileBadge, Ban, Download, Eye } from "lucide-react"
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

export interface LicenseItem {
  id: number
  productId: number | null
  licenseeId: number | null
  planName: string
  registrationCode: string
  status: string
  validFrom: string
  validUntil: string | null
  productName: string
  licenseeName: string
  createdAt: string
}

const STATUS_VARIANTS: Record<string, "default" | "destructive" | "outline"> = {
  issued: "default",
  revoked: "destructive",
}

export function Component() {
  const { t } = useTranslation(["license", "common"])
  const navigate = useNavigate()
  const queryClient = useQueryClient()
  const [formOpen, setFormOpen] = useState(false)
  const [statusFilter, setStatusFilter] = useState("")
  const [revokeTarget, setRevokeTarget] = useState<LicenseItem | null>(null)

  const canIssue = usePermission("license:license:issue")
  const canRevoke = usePermission("license:license:revoke")

  const {
    keyword, setKeyword, page, setPage,
    items: licenses, total, totalPages, isLoading, handleSearch,
  } = useListPage<LicenseItem>({
    queryKey: "license-licenses",
    endpoint: "/api/v1/license/licenses",
    extraParams: statusFilter ? { status: statusFilter } : undefined,
  })

  const revokeMutation = useMutation({
    mutationFn: (id: number) => api.patch(`/api/v1/license/licenses/${id}/revoke`),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["license-licenses"] })
      setRevokeTarget(null)
      toast.success(t("license:licenses.revokeSuccess"))
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
                <SelectItem value="issued">{t("license:status.issued")}</SelectItem>
                <SelectItem value="revoked">{t("license:status.revoked")}</SelectItem>
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
              <TableHead className="w-[80px]">{t("common:status")}</TableHead>
              <TableHead className="w-[100px]">{t("license:licenses.validFrom")}</TableHead>
              <TableHead className="w-[100px]">{t("license:licenses.validUntil")}</TableHead>
              <TableHead className="w-[150px]">{t("license:licenses.issuedAt")}</TableHead>
              <DataTableActionsHead className="min-w-[180px]">{t("common:actions")}</DataTableActionsHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {isLoading ? (
              <DataTableLoadingRow colSpan={8} />
            ) : licenses.length === 0 ? (
              <DataTableEmptyRow
                colSpan={8}
                icon={FileBadge}
                title={t("license:licenses.empty")}
                description={canIssue ? t("license:licenses.emptyHint") : undefined}
              />
            ) : (
              licenses.map((item) => {
                const variant = STATUS_VARIANTS[item.status] ?? ("outline" as const)
                const statusKey = item.status as string
                return (
                  <TableRow key={item.id}>
                    <TableCell className="font-medium">{item.planName}</TableCell>
                    <TableCell className="text-sm">{item.productName || "-"}</TableCell>
                    <TableCell className="text-sm">{item.licenseeName || "-"}</TableCell>
                    <TableCell>
                      <Badge variant={variant}>{t(`license:status.${statusKey}`, item.status)}</Badge>
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
                        <Button
                          variant="ghost"
                          size="sm"
                          className="px-2.5"
                          onClick={() => navigate(`/license/licenses/${item.id}`)}
                        >
                          <Eye className="mr-1 h-3.5 w-3.5" />
                          {t("license:licenses.detail")}
                        </Button>
                        {item.status === "issued" && (
                          <>
                            <Button
                              variant="ghost"
                              size="sm"
                              className="px-2.5"
                              onClick={() => handleExport(item)}
                            >
                              <Download className="mr-1 h-3.5 w-3.5" />
                              {t("common:export")}
                            </Button>
                            {canRevoke && (
                              <Button
                                variant="ghost"
                                size="sm"
                                className="px-2.5 text-destructive hover:text-destructive"
                                onClick={() => setRevokeTarget(item)}
                              >
                                <Ban className="mr-1 h-3.5 w-3.5" />
                                {t("license:licenses.revoke")}
                              </Button>
                            )}
                          </>
                        )}
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

      <AlertDialog open={revokeTarget !== null} onOpenChange={(open) => { if (!open) setRevokeTarget(null) }}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>{t("license:licenses.revokeTitle")}</AlertDialogTitle>
            <AlertDialogDescription>
              {t("license:licenses.revokeDesc")}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>{t("common:cancel")}</AlertDialogCancel>
            <AlertDialogAction
              onClick={() => revokeTarget && revokeMutation.mutate(revokeTarget.id)}
              disabled={revokeMutation.isPending}
            >
              {revokeMutation.isPending ? t("common:processing") : t("license:licenses.confirmRevoke")}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  )
}
