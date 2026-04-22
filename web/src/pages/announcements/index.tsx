import { useState } from "react"
import { useTranslation } from "react-i18next"
import { toast } from "sonner"
import { useMutation, useQueryClient } from "@tanstack/react-query"
import { Plus, Pencil, Trash2, Megaphone } from "lucide-react"
import { api } from "@/lib/api"
import { usePermission } from "@/hooks/use-permission"
import { useListPage } from "@/hooks/use-list-page"
import { Button } from "@/components/ui/button"
import {
  DataTableActions,
  DataTableActionsCell,
  DataTableActionsHead,
  DataTableCard,
  DataTableEmptyRow,
  DataTableLoadingRow,
  DataTablePagination,
  DataTableToolbar,
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
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from "@/components/ui/alert-dialog"
import {
  WorkspaceAlertIconAction,
  WorkspaceIconAction,
  WorkspaceSearchField,
} from "@/components/workspace/primitives"
import { formatDateTime } from "@/lib/utils"
import { addKernelNamespace } from "@/i18n"
import zhCNAnnouncements from "@/i18n/locales/zh-CN/announcements.json"
import enAnnouncements from "@/i18n/locales/en/announcements.json"
import { AnnouncementSheet } from "./announcement-sheet"

addKernelNamespace("announcements", zhCNAnnouncements, enAnnouncements)

interface Announcement {
  id: number
  title: string
  content: string
  createdAt: string
  updatedAt: string
  creatorUsername: string
}

export function Component() {
  const { t } = useTranslation(["announcements", "common"])
  const queryClient = useQueryClient()
  const [sheetOpen, setSheetOpen] = useState(false)
  const [editing, setEditing] = useState<Announcement | null>(null)
  const canCreate = usePermission("system:announcement:create")
  const canUpdate = usePermission("system:announcement:update")
  const canDelete = usePermission("system:announcement:delete")

  const {
    keyword, setKeyword, page, setPage,
    items: announcements, total, totalPages, isLoading, handleSearch,
  } = useListPage<Announcement>({ queryKey: "announcements", endpoint: "/api/v1/announcements" })

  const deleteMutation = useMutation({
    mutationFn: (id: number) => api.delete(`/api/v1/announcements/${id}`),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ["announcements"] }),
    onError: (err) => toast.error(err.message),
  })

  function handleCreate() {
    setEditing(null)
    setSheetOpen(true)
  }

  function handleEdit(item: Announcement) {
    setEditing(item)
    setSheetOpen(true)
  }

  return (
    <div className="workspace-page">
      <div className="workspace-page-header">
        <div>
          <h2 className="workspace-page-title">{t("title")}</h2>
        </div>
        {canCreate && (
          <Button size="sm" onClick={handleCreate}>
            <Plus className="mr-1.5 h-4 w-4" />
            {t("create")}
          </Button>
        )}
      </div>

      <DataTableToolbar>
        <form onSubmit={handleSearch}>
          <WorkspaceSearchField
            value={keyword}
            onChange={setKeyword}
            placeholder={t("searchPlaceholder")}
            className="sm:w-80"
          />
        </form>
      </DataTableToolbar>

      <DataTableCard>
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead className="w-16">ID</TableHead>
              <TableHead className="min-w-[240px]">{t("tableTitle")}</TableHead>
              <TableHead className="w-[140px]">{t("tablePublisher")}</TableHead>
              <TableHead className="w-[150px]">{t("tablePublishTime")}</TableHead>
              <DataTableActionsHead className="min-w-[148px]">{t("common:actions")}</DataTableActionsHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {isLoading ? (
              <DataTableLoadingRow colSpan={5} />
            ) : announcements.length === 0 ? (
              <DataTableEmptyRow
                colSpan={5}
                icon={Megaphone}
                title={t("emptyTitle")}
                description={canCreate ? t("emptyDescription") : undefined}
              />
            ) : (
              announcements.map((item) => (
                <TableRow key={item.id}>
                  <TableCell className="font-mono text-sm">{item.id}</TableCell>
                  <TableCell className="max-w-[360px] font-medium">
                    <span className="block truncate" title={item.title}>
                      {item.title}
                    </span>
                  </TableCell>
                  <TableCell className="text-sm text-muted-foreground whitespace-nowrap">
                    {item.creatorUsername || "-"}
                  </TableCell>
                  <TableCell className="text-sm text-muted-foreground whitespace-nowrap">
                    {formatDateTime(item.createdAt)}
                  </TableCell>
                  <DataTableActionsCell>
                    <DataTableActions>
                      {canUpdate && (
                        <WorkspaceIconAction label={t("common:edit")} icon={Pencil} onClick={() => handleEdit(item)} />
                      )}
                      {canDelete && (
                        <AlertDialog>
                          <WorkspaceAlertIconAction label={t("common:delete")} icon={Trash2} className="hover:text-destructive" />
                          <AlertDialogContent>
                            <AlertDialogHeader>
                              <AlertDialogTitle>{t("confirmDeleteTitle")}</AlertDialogTitle>
                              <AlertDialogDescription>
                                {t("confirmDeleteDescription", { name: item.title })}
                              </AlertDialogDescription>
                            </AlertDialogHeader>
                            <AlertDialogFooter>
                              <AlertDialogCancel>{t("common:cancel")}</AlertDialogCancel>
                              <AlertDialogAction onClick={() => deleteMutation.mutate(item.id)}>
                                {t("common:delete")}
                              </AlertDialogAction>
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

      <DataTablePagination
        total={total}
        page={page}
        totalPages={totalPages}
        onPageChange={setPage}
      />

      <AnnouncementSheet open={sheetOpen} onOpenChange={setSheetOpen} announcement={editing} />
    </div>
  )
}
