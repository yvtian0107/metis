import { useState } from "react"
import { useTranslation } from "react-i18next"
import { toast } from "sonner"
import { useMutation, useQueryClient } from "@tanstack/react-query"
import { Plus, Pencil, Power, Trash2, Users, KeyRound } from "lucide-react"
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
  Tooltip,
  TooltipContent,
  TooltipProvider,
  TooltipTrigger,
} from "@/components/ui/tooltip"
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
  WorkspaceBooleanStatus,
  WorkspaceIconAction,
  WorkspaceSearchField,
} from "@/components/workspace/primitives"
import { cn, formatDateTime } from "@/lib/utils"
import { UserSheet } from "./user-sheet"
import type { User } from "@/stores/auth"

interface UserConnection {
  provider: string
  externalName: string
}

type UserWithAuth = User & {
  hasPassword?: boolean
  connections?: UserConnection[]
  manager?: { id: number; username: string } | null
}

const providerIcons: Record<string, string> = {
  github:
    "M12 2C6.477 2 2 6.484 2 12.017c0 4.425 2.865 8.18 6.839 9.504.5.092.682-.217.682-.483 0-.237-.008-.868-.013-1.703-2.782.605-3.369-1.343-3.369-1.343-.454-1.158-1.11-1.466-1.11-1.466-.908-.62.069-.608.069-.608 1.003.07 1.531 1.032 1.531 1.032.892 1.53 2.341 1.088 2.91.832.092-.647.35-1.088.636-1.338-2.22-.253-4.555-1.113-4.555-4.951 0-1.093.39-1.988 1.029-2.688-.103-.253-.446-1.272.098-2.65 0 0 .84-.27 2.75 1.026A9.564 9.564 0 0112 6.844c.85.004 1.705.115 2.504.337 1.909-1.296 2.747-1.027 2.747-1.027.546 1.379.202 2.398.1 2.651.64.7 1.028 1.595 1.028 2.688 0 3.848-2.339 4.695-4.566 4.943.359.309.678.92.678 1.855 0 1.338-.012 2.419-.012 2.747 0 .268.18.58.688.482A10.019 10.019 0 0022 12.017C22 6.484 17.522 2 12 2z",
  google:
    "M22.56 12.25c0-.78-.07-1.53-.2-2.25H12v4.26h5.92a5.06 5.06 0 01-2.2 3.32v2.77h3.57c2.08-1.92 3.28-4.74 3.28-8.1zM12 23c2.97 0 5.46-.98 7.28-2.66l-3.57-2.77c-.98.66-2.23 1.06-3.71 1.06-2.86 0-5.29-1.93-6.16-4.53H2.18v2.84C3.99 20.53 7.7 23 12 23zM5.84 14.09c-.22-.66-.35-1.36-.35-2.09s.13-1.43.35-2.09V7.07H2.18C1.43 8.55 1 10.22 1 12s.43 3.45 1.18 4.93l2.85-2.22.81-.62zM12 5.38c1.62 0 3.06.56 4.21 1.64l3.15-3.15C17.45 2.09 14.97 1 12 1 7.7 1 3.99 3.47 2.18 7.07l3.66 2.84c.87-2.6 3.3-4.53 6.16-4.53z",
}

const providerLabels: Record<string, string> = {
  github: "GitHub",
  google: "Google",
}

function LoginMethodIcons({ user, t }: { user: UserWithAuth; t: (key: string) => string }) {
  const methods: { key: string; label: string }[] = []
  if (user.hasPassword !== false) {
    methods.push({ key: "password", label: t("users:password") })
  }
  if (user.connections) {
    for (const conn of user.connections) {
      methods.push({
        key: conn.provider,
        label: `${providerLabels[conn.provider] ?? conn.provider} (${conn.externalName})`,
      })
    }
  }
  if (methods.length === 0) return <span className="text-muted-foreground">-</span>

  return (
    <TooltipProvider>
      <div className="flex min-h-8 items-center gap-1.5">
        {methods.map((m) => (
          <Tooltip key={m.key}>
            <TooltipTrigger asChild>
              <span
                className={cn(
                  "inline-flex h-7 w-7 items-center justify-center rounded-md border bg-muted/40 text-muted-foreground transition-colors hover:bg-muted"
                )}
              >
                {m.key === "password" ? (
                  <KeyRound className="h-4 w-4" />
                ) : (
                  <svg className="h-4 w-4" viewBox="0 0 24 24" fill="currentColor">
                    <path d={providerIcons[m.key] ?? ""} />
                  </svg>
                )}
              </span>
            </TooltipTrigger>
            <TooltipContent>{m.label}</TooltipContent>
          </Tooltip>
        ))}
      </div>
    </TooltipProvider>
  )
}

export function Component() {
  const { t } = useTranslation(["users", "common"])
  const queryClient = useQueryClient()
  const [sheetOpen, setSheetOpen] = useState(false)
  const [editing, setEditing] = useState<User | null>(null)
  const canCreate = usePermission("system:user:create")
  const canUpdate = usePermission("system:user:update")
  const canDelete = usePermission("system:user:delete")

  const {
    keyword, setKeyword, page, setPage,
    items: users, total, totalPages, isLoading, handleSearch,
  } = useListPage<UserWithAuth>({ queryKey: "users", endpoint: "/api/v1/users" })

  const deleteMutation = useMutation({
    mutationFn: (id: number) => api.delete(`/api/v1/users/${id}`),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ["users"] }),
    onError: (err) => toast.error(err.message),
  })

  const toggleActiveMutation = useMutation({
    mutationFn: ({ id, active }: { id: number; active: boolean }) =>
      api.post(`/api/v1/users/${id}/${active ? "activate" : "deactivate"}`, {}),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ["users"] }),
  })

  function handleCreate() {
    setEditing(null)
    setSheetOpen(true)
  }

  function handleEdit(user: User) {
    setEditing(user)
    setSheetOpen(true)
  }

  return (
    <div className="workspace-page">
      <div className="workspace-page-header">
        <div>
          <h2 className="workspace-page-title">{t("users:title")}</h2>
          <p className="workspace-page-description">{t("users:noUsersDescription")}</p>
        </div>
        <div className="flex items-center gap-2">
          <Button size="sm" onClick={handleCreate} disabled={!canCreate}>
            <Plus className="mr-1.5 h-4 w-4" />
            {t("users:createUser")}
          </Button>
        </div>
      </div>

      <DataTableToolbar>
        <form onSubmit={handleSearch}>
          <WorkspaceSearchField
            value={keyword}
            onChange={setKeyword}
            placeholder={t("users:searchPlaceholder")}
            className="sm:w-80"
          />
        </form>
      </DataTableToolbar>

      <DataTableCard>
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead className="w-16">ID</TableHead>
              <TableHead className="min-w-[180px]">{t("users:username")}</TableHead>
              <TableHead className="min-w-[180px]">{t("users:email")}</TableHead>
              <TableHead className="min-w-[140px]">{t("users:phone")}</TableHead>
              <TableHead className="w-[110px] text-center">{t("users:role")}</TableHead>
              <TableHead className="w-[120px]">{t("users:manager")}</TableHead>
              <TableHead className="w-[120px] text-center">{t("users:loginMethod")}</TableHead>
              <TableHead className="w-[100px] text-center">{t("common:status")}</TableHead>
              <TableHead className="w-[150px] text-center">{t("common:createdAt")}</TableHead>
              <DataTableActionsHead className="min-w-[208px]">{t("common:actions")}</DataTableActionsHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {isLoading ? (
              <DataTableLoadingRow colSpan={10} />
            ) : users.length === 0 ? (
              <DataTableEmptyRow
                colSpan={10}
                icon={Users}
                title={t("users:noUsers")}
                description={t("users:noUsersDescription")}
              />
            ) : (
              users.map((user) => (
                <TableRow key={user.id}>
                  <TableCell className="font-mono text-sm">{user.id}</TableCell>
                  <TableCell>
                    <div className="min-w-0">
                      <div className="font-medium">{user.username}</div>
                    </div>
                  </TableCell>
                  <TableCell className="max-w-[220px] text-sm text-muted-foreground">
                    <span className="block truncate" title={user.email || "-"}>
                      {user.email || "-"}
                    </span>
                  </TableCell>
                  <TableCell className="text-sm text-muted-foreground">
                    {user.phone || "-"}
                  </TableCell>
                  <TableCell className="text-center">
                    <Badge variant={user.role?.code === "admin" ? "default" : "secondary"}>
                      {user.role?.name || t("users:noRoleAssigned")}
                    </Badge>
                  </TableCell>
                  <TableCell className="text-sm text-muted-foreground">
                    {user.manager ? user.manager.username : <span className="text-muted-foreground/50">—</span>}
                  </TableCell>
                  <TableCell className="text-center">
                    <div className="flex justify-center">
                      <LoginMethodIcons user={user} t={t} />
                    </div>
                  </TableCell>
                  <TableCell className="text-center">
                    <WorkspaceBooleanStatus
                      active={user.isActive}
                      activeLabel={t("common:active")}
                      inactiveLabel={t("common:inactive")}
                    />
                  </TableCell>
                  <TableCell className="text-center text-sm text-muted-foreground whitespace-nowrap tabular-nums">
                    {formatDateTime(user.createdAt)}
                  </TableCell>
                  <DataTableActionsCell>
                    <DataTableActions>
                      {canUpdate && (
                        <WorkspaceIconAction label={t("common:edit")} icon={Pencil} onClick={() => handleEdit(user)} />
                      )}
                      {canUpdate && (
                        <WorkspaceIconAction
                          label={user.isActive ? t("common:disable") : t("common:enable")}
                          icon={Power}
                          onClick={() =>
                            toggleActiveMutation.mutate({
                              id: user.id,
                              active: !user.isActive,
                            })
                          }
                        />
                      )}
                      {canDelete && (
                        <AlertDialog>
                          <WorkspaceAlertIconAction label={t("common:delete")} icon={Trash2} className="hover:text-destructive" />
                          <AlertDialogContent>
                            <AlertDialogHeader>
                              <AlertDialogTitle>{t("users:confirmDelete")}</AlertDialogTitle>
                              <AlertDialogDescription>
                                {t("users:deleteConfirm", { name: user.username })}
                              </AlertDialogDescription>
                            </AlertDialogHeader>
                            <AlertDialogFooter>
                              <AlertDialogCancel>{t("common:cancel")}</AlertDialogCancel>
                              <AlertDialogAction onClick={() => deleteMutation.mutate(user.id)}>
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

      <UserSheet open={sheetOpen} onOpenChange={setSheetOpen} user={editing} />
    </div>
  )
}
