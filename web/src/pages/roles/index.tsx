import { useState } from "react"
import { useTranslation } from "react-i18next"
import { useMutation, useQueryClient } from "@tanstack/react-query"
import { Plus, Search, Shield, Pencil, Trash2, ShieldCheck } from "lucide-react"
import { api } from "@/lib/api"
import { usePermission } from "@/hooks/use-permission"
import { useListPage } from "@/hooks/use-list-page"
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
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
  AlertDialogTrigger,
} from "@/components/ui/alert-dialog"
import { formatDateTime } from "@/lib/utils"
import { RoleSheet } from "./role-sheet"
import { PermissionDialog } from "./permission-dialog"
import type { Role } from "./types"

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
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <h2 className="text-lg font-semibold">{t("roles:title")}</h2>
        <Button size="sm" onClick={handleCreate} disabled={!canCreate}>
          <Plus className="mr-1.5 h-4 w-4" />
          {t("roles:createRole")}
        </Button>
      </div>

      <DataTableToolbar>
        <DataTableToolbarGroup>
          <form onSubmit={handleSearch} className="flex w-full flex-col gap-2 sm:flex-row sm:items-center">
            <div className="relative w-full sm:max-w-sm">
              <Search className="absolute left-2.5 top-2.5 h-4 w-4 text-muted-foreground" />
              <Input
                placeholder={t("roles:searchPlaceholder")}
                value={keyword}
                onChange={(e) => setKeyword(e.target.value)}
                className="pl-8"
              />
            </div>
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
              <TableHead className="w-16">ID</TableHead>
              <TableHead className="min-w-[180px]">{t("roles:roleName")}</TableHead>
              <TableHead className="w-[180px]">{t("roles:roleCode")}</TableHead>
              <TableHead className="min-w-[220px]">{t("common:description")}</TableHead>
              <TableHead className="w-[100px]">{t("common:type")}</TableHead>
              <TableHead className="w-[150px]">{t("common:createdAt")}</TableHead>
              <DataTableActionsHead className="min-w-[220px]">{t("common:actions")}</DataTableActionsHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {isLoading ? (
              <DataTableLoadingRow colSpan={7} />
            ) : roles.length === 0 ? (
              <DataTableEmptyRow
                colSpan={7}
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
                  <TableCell className="text-sm text-muted-foreground whitespace-nowrap">
                    {formatDateTime(role.createdAt)}
                  </TableCell>
                  <DataTableActionsCell>
                    <DataTableActions>
                      {canAssign && (
                        <Button variant="ghost" size="sm" className="px-2.5" onClick={() => setPermRole(role)}>
                          <Shield className="mr-1 h-3.5 w-3.5" />
                          {t("roles:permissions")}
                        </Button>
                      )}
                      {canUpdate && (
                        <Button variant="ghost" size="sm" className="px-2.5" onClick={() => handleEdit(role)}>
                          <Pencil className="mr-1 h-3.5 w-3.5" />
                          {t("common:edit")}
                        </Button>
                      )}
                      {canDelete && (role.isSystem ? (
                        <Button variant="ghost" size="sm" disabled className="px-2.5 text-muted-foreground">
                          <Trash2 className="mr-1 h-3.5 w-3.5" />
                          {t("common:delete")}
                        </Button>
                      ) : (
                        <AlertDialog>
                          <AlertDialogTrigger asChild>
                            <Button
                              variant="ghost"
                              size="sm"
                              className="px-2.5 text-destructive hover:text-destructive"
                            >
                              <Trash2 className="mr-1 h-3.5 w-3.5" />
                              {t("common:delete")}
                            </Button>
                          </AlertDialogTrigger>
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
