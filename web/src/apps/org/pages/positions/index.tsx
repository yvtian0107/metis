"use client"

import { useState } from "react"
import { useTranslation } from "react-i18next"
import { useMutation, useQueryClient } from "@tanstack/react-query"
import { Plus, Pencil, Trash2, Briefcase, Building2, Users } from "lucide-react"
import { usePermission } from "@/hooks/use-permission"
import { useListPage } from "@/hooks/use-list-page"
import { api } from "@/lib/api"
import { toast } from "sonner"
import { Button } from "@/components/ui/button"
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
import { PositionSheet, type PositionItem } from "../../components/position-sheet"
import {
  WorkspaceAlertIconAction,
  WorkspaceBooleanStatus,
  WorkspaceIconAction,
  WorkspaceSearchField,
} from "@/components/workspace/primitives"

export function Component() {
  const { t } = useTranslation(["org", "common"])
  const queryClient = useQueryClient()
  const [formOpen, setFormOpen] = useState(false)
  const [editing, setEditing] = useState<PositionItem | null>(null)

  const canCreate = usePermission("org:position:create")
  const canUpdate = usePermission("org:position:update")
  const canDelete = usePermission("org:position:delete")

  const {
    keyword, setKeyword, page, setPage,
    items, total, totalPages, isLoading, handleSearch,
  } = useListPage<PositionItem>({
    queryKey: "positions",
    endpoint: "/api/v1/org/positions",
  })

  const deleteMutation = useMutation({
    mutationFn: (id: number) => api.delete(`/api/v1/org/positions/${id}`),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["positions"] })
      toast.success(t("org:positions.deleteSuccess"))
    },
    onError: (err) => toast.error(err.message),
  })

  function handleCreate() {
    setEditing(null)
    setFormOpen(true)
  }

  function handleEdit(item: PositionItem) {
    setEditing(item)
    setFormOpen(true)
  }

  return (
    <div className="workspace-page">
      <div className="workspace-page-header">
        <div>
          <div className="flex items-center gap-2">
            <h2 className="workspace-page-title">{t("org:positions.title")}</h2>
            <Badge variant="outline" className="bg-transparent font-normal text-muted-foreground">
              {t("org:positions.directory")}
            </Badge>
          </div>
        </div>
        {canCreate && (
          <Button size="sm" onClick={handleCreate}>
            <Plus className="mr-1.5 h-4 w-4" />
            {t("org:positions.create")}
          </Button>
        )}
      </div>

      <DataTableToolbar>
        <form onSubmit={handleSearch}>
          <WorkspaceSearchField
            value={keyword}
            onChange={setKeyword}
            placeholder={t("org:positions.searchPlaceholder")}
            className="sm:w-80"
          />
        </form>
      </DataTableToolbar>

      <DataTableCard>
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead className="min-w-[240px]">{t("org:positions.name")}</TableHead>
              <TableHead className="min-w-[220px]">{t("org:positions.coverage")}</TableHead>
              <TableHead className="w-[130px]">{t("org:positions.occupants")}</TableHead>
              <TableHead className="w-[110px]">{t("common:status")}</TableHead>
              <DataTableActionsHead>{t("common:actions")}</DataTableActionsHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {isLoading ? (
              <DataTableLoadingRow colSpan={5} />
            ) : items.length === 0 ? (
              <DataTableEmptyRow
                colSpan={5}
                icon={Briefcase}
                title={t("org:positions.empty")}
                description={canCreate ? t("org:positions.emptyHint") : undefined}
              />
            ) : (
              items.map((item) => (
                <TableRow key={item.id} className="border-border/45 hover:bg-surface-soft/45">
                  <TableCell className="py-3.5">
                    <div className="flex items-start gap-3">
                      <div className="flex h-9 w-9 shrink-0 items-center justify-center rounded-lg border border-border/55 bg-surface/50 text-muted-foreground">
                        <Briefcase className="h-4 w-4" />
                      </div>
                      <div className="min-w-0">
                        <div className="flex flex-wrap items-center gap-2">
                          <span className="truncate text-sm font-medium text-foreground">{item.name}</span>
                          <Badge variant="outline" className="h-5 bg-transparent px-1.5 text-[10px] font-normal text-muted-foreground">
                            {item.code}
                          </Badge>
                        </div>
                        {item.description && (
                          <p className="mt-1 line-clamp-2 text-xs leading-5 text-muted-foreground">{item.description}</p>
                        )}
                      </div>
                    </div>
                  </TableCell>
                  <TableCell>
                    {item.departments && item.departments.length > 0 ? (
                      <div className="flex flex-wrap gap-1.5">
                        {item.departments.slice(0, 3).map((dept) => (
                          <Badge key={dept.id} variant="outline" className="gap-1 bg-transparent font-normal">
                            <Building2 className="h-3 w-3" />
                            {dept.name}
                          </Badge>
                        ))}
                        {item.departments.length > 3 && (
                          <Badge variant="outline" className="bg-transparent font-normal text-muted-foreground">
                            +{item.departments.length - 3}
                          </Badge>
                        )}
                      </div>
                    ) : (
                      <span className="text-sm text-muted-foreground">{t("org:positions.noDepartments")}</span>
                    )}
                  </TableCell>
                  <TableCell>
                    <div className="flex items-center gap-2 text-sm text-muted-foreground">
                      <Users className="h-3.5 w-3.5" />
                      <span className="tabular-nums">{t("org:positions.membersCount", { count: item.memberCount ?? 0 })}</span>
                    </div>
                  </TableCell>
                  <TableCell>
                    <WorkspaceBooleanStatus
                      active={item.isActive}
                      activeLabel={t("org:positions.active")}
                      inactiveLabel={t("org:positions.inactive")}
                    />
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
                              <AlertDialogTitle>{t("org:positions.deleteTitle")}</AlertDialogTitle>
                              <AlertDialogDescription>
                                {t("org:positions.deleteDesc", { name: item.name })}
                              </AlertDialogDescription>
                            </AlertDialogHeader>
                            <AlertDialogFooter>
                              <AlertDialogCancel size="sm">{t("common:cancel")}</AlertDialogCancel>
                              <AlertDialogAction
                                size="sm"
                                onClick={() => deleteMutation.mutate(item.id)}
                                disabled={deleteMutation.isPending}
                              >
                                {t("org:positions.confirmDelete")}
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

      <PositionSheet open={formOpen} onOpenChange={setFormOpen} position={editing} />
    </div>
  )
}
