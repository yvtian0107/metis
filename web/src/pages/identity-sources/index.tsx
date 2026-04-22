import { useState } from "react"
import { useTranslation } from "react-i18next"
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query"
import { Plus, Pencil, Trash2, Fingerprint, Plug, Loader2 } from "lucide-react"
import { api } from "@/lib/api"
import { usePermission } from "@/hooks/use-permission"
import { toast } from "sonner"
import { Button } from "@/components/ui/button"
import { Badge } from "@/components/ui/badge"
import { Switch } from "@/components/ui/switch"
import {
  DataTableActions,
  DataTableActionsCell,
  DataTableActionsHead,
  DataTableCard,
  DataTableEmptyRow,
  DataTableLoadingRow,
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
} from "@/components/workspace/primitives"
import { formatDateTime } from "@/lib/utils"
import { addKernelNamespace } from "@/i18n"
import zhCNIdentitySources from "@/i18n/locales/zh-CN/identitySources.json"
import enIdentitySources from "@/i18n/locales/en/identitySources.json"
import { IdentitySourceSheet, type IdentitySourceItem } from "./identity-source-sheet"

addKernelNamespace("identitySources", zhCNIdentitySources, enIdentitySources)

const TYPE_LABELS: Record<string, string> = {
  oidc: "OIDC",
  ldap: "LDAP",
}

export function Component() {
  const { t } = useTranslation(["identitySources", "common"])
  const queryClient = useQueryClient()
  const [formOpen, setFormOpen] = useState(false)
  const [editing, setEditing] = useState<IdentitySourceItem | null>(null)

  const canCreate = usePermission("system:identity-source:create")
  const canUpdate = usePermission("system:identity-source:update")
  const canDelete = usePermission("system:identity-source:delete")

  const { data: sources = [], isLoading } = useQuery({
    queryKey: ["identity-sources"],
    queryFn: () => api.get<IdentitySourceItem[]>("/api/v1/identity-sources"),
  })

  const deleteMutation = useMutation({
    mutationFn: (id: number) => api.delete(`/api/v1/identity-sources/${id}`),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["identity-sources"] })
      toast.success(t("deleteSuccess"))
    },
    onError: (err) => toast.error(err.message),
  })

  const toggleMutation = useMutation({
    mutationFn: (id: number) => api.patch(`/api/v1/identity-sources/${id}/toggle`),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["identity-sources"] })
    },
    onError: (err) => toast.error(err.message),
  })

  const testMutation = useMutation({
    mutationFn: async (id: number) => {
      const res = await api.post<{ success: boolean; message: string }>(
        `/api/v1/identity-sources/${id}/test`,
      )
      if (!res.success) throw new Error(res.message || t("testConnectionFailed"))
      return res
    },
    onSuccess: (res) => toast.success(res.message || t("testConnectionSuccess")),
    onError: (err) => toast.error(t("testConnectionFailedDetail", { message: err.message })),
  })

  function handleCreate() {
    setEditing(null)
    setFormOpen(true)
  }

  function handleEdit(item: IdentitySourceItem) {
    setEditing(item)
    setFormOpen(true)
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
            {t("createSource")}
          </Button>
        )}
      </div>

      <DataTableCard>
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead className="w-16">ID</TableHead>
              <TableHead className="min-w-[180px]">{t("common:name")}</TableHead>
              <TableHead className="w-[100px]">{t("common:type")}</TableHead>
              <TableHead className="min-w-[220px]">{t("domain")}</TableHead>
              <TableHead className="w-[100px]">{t("common:status")}</TableHead>
              <TableHead className="w-[150px]">{t("common:createdAt")}</TableHead>
              <DataTableActionsHead className="min-w-[244px]">{t("common:actions")}</DataTableActionsHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {isLoading ? (
              <DataTableLoadingRow colSpan={7} />
            ) : sources.length === 0 ? (
              <DataTableEmptyRow
                colSpan={7}
                icon={Fingerprint}
                title={t("emptyTitle")}
                description={canCreate ? t("emptyDescription") : undefined}
              />
            ) : (
              sources.map((item) => (
                <TableRow key={item.id}>
                  <TableCell className="font-mono text-sm">{item.id}</TableCell>
                  <TableCell className="font-medium">{item.name}</TableCell>
                  <TableCell>
                    <Badge variant="secondary">
                      {TYPE_LABELS[item.type] ?? item.type}
                    </Badge>
                  </TableCell>
                  <TableCell className="max-w-[320px] text-sm text-muted-foreground">
                    <span className="block truncate" title={item.domains || "-"}>
                      {item.domains || "-"}
                    </span>
                  </TableCell>
                  <TableCell>
                    <Switch
                      checked={item.enabled}
                      disabled={!canUpdate}
                      onCheckedChange={() => toggleMutation.mutate(item.id)}
                    />
                  </TableCell>
                  <TableCell className="text-sm text-muted-foreground whitespace-nowrap">
                    {formatDateTime(item.createdAt)}
                  </TableCell>
                  <DataTableActionsCell>
                    <DataTableActions>
                      {canUpdate && (
                        <WorkspaceIconAction
                          label={t("testConnection")}
                          icon={testMutation.isPending ? Loader2 : Plug}
                          disabled={testMutation.isPending}
                          onClick={() => testMutation.mutate(item.id)}
                          className={testMutation.isPending ? "[&_svg]:animate-spin" : undefined}
                        />
                      )}
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
                                {t("confirmDeleteDescription", { name: item.name })}
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

      <IdentitySourceSheet open={formOpen} onOpenChange={setFormOpen} source={editing} />
    </div>
  )
}
