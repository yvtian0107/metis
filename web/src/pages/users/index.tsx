import { useState } from "react"
import { useTranslation } from "react-i18next"
import { useMutation, useQueryClient } from "@tanstack/react-query"
import { Plus, Search, Pencil, Power, Trash2, Users, KeyRound } from "lucide-react"
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
  AlertDialogTrigger,
} from "@/components/ui/alert-dialog"
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
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <h2 className="text-lg font-semibold">{t("users:title")}</h2>
        <Button size="sm" onClick={handleCreate} disabled={!canCreate}>
          <Plus className="mr-1.5 h-4 w-4" />
          {t("users:createUser")}
        </Button>
      </div>

      <DataTableToolbar>
        <DataTableToolbarGroup>
          <form onSubmit={handleSearch} className="flex w-full flex-col gap-2 sm:flex-row sm:items-center">
            <div className="relative w-full sm:max-w-sm">
              <Search className="absolute left-2.5 top-2.5 h-4 w-4 text-muted-foreground" />
              <Input
                placeholder={t("users:searchPlaceholder")}
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
              <TableHead className="min-w-[180px]">{t("users:username")}</TableHead>
              <TableHead className="min-w-[180px]">{t("users:email")}</TableHead>
              <TableHead className="min-w-[140px]">{t("users:phone")}</TableHead>
              <TableHead className="w-[110px] text-center">{t("users:role")}</TableHead>
              <TableHead className="w-[120px] text-center">{t("users:loginMethod")}</TableHead>
              <TableHead className="w-[100px] text-center">{t("common:status")}</TableHead>
              <TableHead className="w-[150px] text-center">{t("common:createdAt")}</TableHead>
              <DataTableActionsHead className="min-w-[208px]">{t("common:actions")}</DataTableActionsHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {isLoading ? (
              <DataTableLoadingRow colSpan={9} />
            ) : users.length === 0 ? (
              <DataTableEmptyRow
                colSpan={9}
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
                  <TableCell className="text-center">
                    <div className="flex justify-center">
                      <LoginMethodIcons user={user} t={t} />
                    </div>
                  </TableCell>
                  <TableCell className="text-center">
                    <Badge variant={user.isActive ? "default" : "outline"}>
                      {user.isActive ? t("common:active") : t("common:inactive")}
                    </Badge>
                  </TableCell>
                  <TableCell className="text-center text-sm text-muted-foreground whitespace-nowrap tabular-nums">
                    {formatDateTime(user.createdAt)}
                  </TableCell>
                  <DataTableActionsCell>
                    <DataTableActions>
                      {canUpdate && (
                        <Button variant="ghost" size="sm" className="px-2.5" onClick={() => handleEdit(user)}>
                          <Pencil className="mr-1 h-3.5 w-3.5" />
                          {t("common:edit")}
                        </Button>
                      )}
                      {canUpdate && (
                        <Button
                          variant="ghost"
                          size="sm"
                          className="px-2.5"
                          onClick={() =>
                            toggleActiveMutation.mutate({
                              id: user.id,
                              active: !user.isActive,
                            })
                          }
                        >
                          <Power className="mr-1 h-3.5 w-3.5" />
                          {user.isActive ? t("common:disable") : t("common:enable")}
                        </Button>
                      )}
                      {canDelete && (
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
