import { useState } from "react"
import { useTranslation } from "react-i18next"
import { Plus, Search, Building2, Pencil, Archive, ArchiveRestore } from "lucide-react"
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
import { LicenseeSheet, type LicenseeItem } from "../../components/licensee-sheet"

const STATUS_VARIANTS: Record<string, "default" | "secondary" | "outline"> = {
  active: "default",
  archived: "outline",
}

export function Component() {
  const { t } = useTranslation(["license", "common"])
  const queryClient = useQueryClient()
  const [formOpen, setFormOpen] = useState(false)
  const [editing, setEditing] = useState<LicenseeItem | null>(null)
  const [statusFilter, setStatusFilter] = useState("")
  const [archiveTarget, setArchiveTarget] = useState<LicenseeItem | null>(null)

  const canCreate = usePermission("license:licensee:create")
  const canUpdate = usePermission("license:licensee:update")
  const canArchive = usePermission("license:licensee:archive")

  const {
    keyword, setKeyword, page, setPage,
    items: licensees, total, totalPages, isLoading, handleSearch,
  } = useListPage<LicenseeItem>({
    queryKey: "license-licensees",
    endpoint: "/api/v1/license/licensees",
    extraParams: statusFilter ? { status: statusFilter } : undefined,
  })

  const statusMutation = useMutation({
    mutationFn: ({ id, status }: { id: number; status: string }) =>
      api.patch(`/api/v1/license/licensees/${id}/status`, { status }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["license-licensees"] })
      setArchiveTarget(null)
      toast.success(t("license:licensees.statusUpdateSuccess"))
    },
    onError: (err) => toast.error(err.message),
  })

  function handleCreate() {
    setEditing(null)
    setFormOpen(true)
  }

  function handleEdit(item: LicenseeItem) {
    setEditing(item)
    setFormOpen(true)
  }

  function handleStatusFilter(value: string) {
    setStatusFilter(value === "all" ? "" : value)
    setPage(1)
  }

  function handleArchive(item: LicenseeItem) {
    setArchiveTarget(item)
  }

  function confirmArchive() {
    if (!archiveTarget) return
    const newStatus = archiveTarget.status === "active" ? "archived" : "active"
    statusMutation.mutate({ id: archiveTarget.id, status: newStatus })
  }

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <h2 className="text-lg font-semibold">{t("license:licensees.title")}</h2>
        {canCreate && (
          <Button size="sm" onClick={handleCreate}>
            <Plus className="mr-1.5 h-4 w-4" />
            {t("license:licensees.create")}
          </Button>
        )}
      </div>

      <DataTableToolbar>
        <DataTableToolbarGroup>
          <form onSubmit={handleSearch} className="flex w-full flex-col gap-2 sm:flex-row sm:items-center">
            <div className="relative w-full sm:max-w-sm">
              <Search className="absolute left-2.5 top-2.5 h-4 w-4 text-muted-foreground" />
              <Input
                placeholder={t("license:licensees.searchPlaceholder")}
                value={keyword}
                onChange={(e) => setKeyword(e.target.value)}
                className="pl-8"
              />
            </div>
            <Select value={statusFilter || "all"} onValueChange={handleStatusFilter}>
              <SelectTrigger className="w-full sm:w-[130px]">
                <SelectValue placeholder={t("license:licensees.allStatus")} />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="all">{t("license:licensees.allStatus")}</SelectItem>
                <SelectItem value="active">{t("license:status.active")}</SelectItem>
                <SelectItem value="archived">{t("license:status.archived")}</SelectItem>
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
              <TableHead className="min-w-[180px]">{t("common:name")}</TableHead>
              <TableHead className="w-[120px]">{t("license:licensees.contactName")}</TableHead>
              <TableHead className="w-[80px]">{t("common:status")}</TableHead>
              <TableHead className="w-[150px]">{t("common:createdAt")}</TableHead>
              <DataTableActionsHead className="min-w-[140px]">{t("common:actions")}</DataTableActionsHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {isLoading ? (
              <DataTableLoadingRow colSpan={5} />
            ) : licensees.length === 0 ? (
              <DataTableEmptyRow
                colSpan={5}
                icon={Building2}
                title={t("license:licensees.empty")}
                description={canCreate ? t("license:licensees.emptyHint") : undefined}
              />
            ) : (
              licensees.map((item) => {
                const variant = STATUS_VARIANTS[item.status] ?? ("secondary" as const)
                const statusKey = item.status as string
                return (
                  <TableRow key={item.id}>
                    <TableCell className="font-medium">{item.name}</TableCell>
                    <TableCell className="text-sm">{item.contactName || "-"}</TableCell>
                    <TableCell>
                      <Badge variant={variant}>{t(`license:status.${statusKey}`, item.status)}</Badge>
                    </TableCell>
                    <TableCell className="text-sm text-muted-foreground whitespace-nowrap">
                      {formatDateTime(item.createdAt)}
                    </TableCell>
                    <DataTableActionsCell>
                      <DataTableActions>
                        {canUpdate && (
                          <Button
                            variant="ghost"
                            size="sm"
                            className="px-2.5"
                            onClick={() => handleEdit(item)}
                          >
                            <Pencil className="mr-1 h-3.5 w-3.5" />
                            {t("common:edit")}
                          </Button>
                        )}
                        {canArchive && (
                          <Button
                            variant="ghost"
                            size="sm"
                            className="px-2.5"
                            onClick={() => handleArchive(item)}
                          >
                            {item.status === "active" ? (
                              <>
                                <Archive className="mr-1 h-3.5 w-3.5" />
                                {t("license:licensees.archive")}
                              </>
                            ) : (
                              <>
                                <ArchiveRestore className="mr-1 h-3.5 w-3.5" />
                                {t("license:licensees.restore")}
                              </>
                            )}
                          </Button>
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

      <LicenseeSheet open={formOpen} onOpenChange={setFormOpen} licensee={editing} />

      <AlertDialog open={archiveTarget !== null} onOpenChange={(open) => { if (!open) setArchiveTarget(null) }}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>
              {archiveTarget?.status === "active" ? t("license:licensees.archiveTitle") : t("license:licensees.restoreTitle")}
            </AlertDialogTitle>
            <AlertDialogDescription>
              {archiveTarget?.status === "active"
                ? t("license:licensees.archiveDesc", { name: archiveTarget?.name })
                : t("license:licensees.restoreDesc", { name: archiveTarget?.name })}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>{t("common:cancel")}</AlertDialogCancel>
            <AlertDialogAction onClick={confirmArchive} disabled={statusMutation.isPending}>
              {statusMutation.isPending ? t("common:processing") : t("common:confirm")}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  )
}
