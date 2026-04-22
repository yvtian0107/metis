import { useState } from "react"
import { useTranslation } from "react-i18next"
import { toast } from "sonner"
import { useMutation, useQueryClient } from "@tanstack/react-query"
import { Plus, Shield, Pencil, Trash2, ShieldCheck, Database } from "lucide-react"
import { api } from "@/lib/api"
import { usePermission } from "@/hooks/use-permission"
import { useListPage } from "@/hooks/use-list-page"
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
import {
  WorkspaceAlertIconAction,
  WorkspaceIconAction,
  WorkspaceSearchField,
} from "@/components/workspace/primitives"
import { formatDateTime } from "@/lib/utils"
import { RoleSheet } from "./role-sheet"
import { PermissionDialog } from "./permission-dialog"
import type { Role } from "./types"

const dataScopeBadgeVariant: Record<string, "default" | "secondary" | "outline"> = {
  all: "secondary",
  dept_and_sub: "outline",
  dept: "outline",
  self: "outline",
  custom: "default",
}

export function Component() {
  const { t } = useTranslation(["roles", "common"])
  const queryClient = useQueryClient()
  const [sheetOpen, setSheetOpen] = useState(false)
  const [editing, setEditing] = useState<Role | null>(null)
  const [permRole, setPermRole] = useState<Role | null>(null)
  const canCreate = usePermission("system:role:create")
  const canUpdate = usePermission("system:role:update")
  const canDelete = usePermission("system:role:delete")
  const canAssign = usePermission("system:role:assign")

  const {
    keyword, setKeyword, page, setPage,
    items: roles, total, totalPages, isLoading, handleSearch,
  } = useListPage<Role>({ queryKey: "roles", endpoint: "/api/v1/roles" })

  const deleteMutation = useMutation({
    mutationFn: (id: number) => api.delete(`/api/v1/roles/${id}`),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ["roles"] }),
    onError: (err) => toast.error(err.message),
  })

  function handleCreate() {
    setEditing(null)
    setSheetOpen(true)
  }

  function handleEdit(role: Role) {
    setEditing(role)
    setSheetOpen(true)
  }

  return (
    <div className="workspace-page">
      <div className="workspace-page-header">
        <div>
          <h2 className="workspace-page-title">{t("roles:title")}</h2>
        </div>
        <Button size="sm" onClick={handleCreate} disabled={!canCreate}>
          <Plus className="mr-1.5 h-4 w-4" />
          {t("roles:createRole")}
        </Button>
      </div>

      <DataTableToolbar>
        <form onSubmit={handleSearch}>
          <WorkspaceSearchField
            value={keyword}
            onChange={setKeyword}
            placeholder={t("roles:searchPlaceholder")}
            className="sm:w-80"
          />
        </form>
      </DataTableToolbar>

      <DataTableCard>
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead className="w-16">ID</TableHead>
              <TableHead className="min-w-[180px]">{t("roles:roleName")}</TableHead>
              <TableHead className="w-[180px]">{t("roles:roleCode")}</TableHead>
              <TableHead className="min-w-[220px]">{t("common:description")}</TableHead>
              <TableHead className="w-[100px]">{t("common:type")}</TableHead>
              <TableHead className="w-[140px]">{t("roles:dataScope.label")}</TableHead>
              <TableHead className="w-[150px]">{t("common:createdAt")}</TableHead>
              <DataTableActionsHead className="min-w-[220px]">{t("common:actions")}</DataTableActionsHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {isLoading ? (
              <DataTableLoadingRow colSpan={8} />
            ) : roles.length === 0 ? (
              <DataTableEmptyRow
                colSpan={8}
                icon={ShieldCheck}
                title={t("roles:emptyTitle")}
                description={t("roles:emptyDescription")}
              />
            ) : (
              roles.map((role) => (
                <TableRow key={role.id}>
                  <TableCell className="font-mono text-sm">{role.id}</TableCell>
                  <TableCell className="font-medium">{role.name}</TableCell>
                  <TableCell className="font-mono text-sm">{role.code}</TableCell>
                  <TableCell className="max-w-[320px] text-sm text-muted-foreground">
                    <span className="block truncate" title={role.description || "-"}>
                      {role.description || "-"}
                    </span>
                  </TableCell>
                  <TableCell>
                    <Badge variant={role.isSystem ? "default" : "secondary"}>
                      {role.isSystem ? t("roles:builtIn") : t("roles:custom")}
                    </Badge>
                  </TableCell>
                  <TableCell>
                    <Badge variant={dataScopeBadgeVariant[role.dataScope] ?? "secondary"} className="gap-1 text-xs">
                      <Database className="h-3 w-3" />
                      {t(`roles:dataScope.${role.dataScope === "dept_and_sub" ? "deptAndSub" : role.dataScope}`)}
                    </Badge>
                  </TableCell>
                  <TableCell className="text-sm text-muted-foreground whitespace-nowrap">
                    {formatDateTime(role.createdAt)}
                  </TableCell>
                  <DataTableActionsCell>
                    <DataTableActions>
                      {canAssign && (
                        <WorkspaceIconAction label={t("roles:permissions")} icon={Shield} onClick={() => setPermRole(role)} />
                      )}
                      {canUpdate && (
                        <WorkspaceIconAction label={t("common:edit")} icon={Pencil} onClick={() => handleEdit(role)} />
                      )}
                      {canDelete && (role.isSystem ? (
                        <WorkspaceIconAction label={t("common:delete")} icon={Trash2} disabled />
                      ) : (
                        <AlertDialog>
                          <WorkspaceAlertIconAction label={t("common:delete")} icon={Trash2} className="hover:text-destructive" />
                          <AlertDialogContent>
                            <AlertDialogHeader>
                              <AlertDialogTitle>{t("roles:confirmDeleteTitle")}</AlertDialogTitle>
                              <AlertDialogDescription>
                                {t("roles:confirmDeleteDescription", { name: role.name })}
                              </AlertDialogDescription>
                            </AlertDialogHeader>
                            <AlertDialogFooter>
                              <AlertDialogCancel>{t("common:cancel")}</AlertDialogCancel>
                              <AlertDialogAction onClick={() => deleteMutation.mutate(role.id)}>
                                {t("common:delete")}
                              </AlertDialogAction>
                            </AlertDialogFooter>
                          </AlertDialogContent>
                        </AlertDialog>
                      ))}
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

      <RoleSheet open={sheetOpen} onOpenChange={setSheetOpen} role={editing} />
      <PermissionDialog
        open={!!permRole}
        onOpenChange={(open) => { if (!open) setPermRole(null) }}
        role={permRole}
      />
    </div>
  )
}
