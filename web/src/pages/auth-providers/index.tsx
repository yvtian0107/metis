import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import { useTranslation } from "react-i18next"
import { toast } from "sonner"
import { Pencil, KeyRound } from "lucide-react"
import { useState } from "react"
import { api } from "@/lib/api"
import { usePermission } from "@/hooks/use-permission"
import { Switch } from "@/components/ui/switch"
import {
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
import { WorkspaceIconAction } from "@/components/workspace/primitives"
import { addKernelNamespace } from "@/i18n"
import zhCNAuthProviders from "@/i18n/locales/zh-CN/authProviders.json"
import enAuthProviders from "@/i18n/locales/en/authProviders.json"
import { ProviderSheet, type AuthProvider } from "./provider-sheet"

addKernelNamespace("authProviders", zhCNAuthProviders, enAuthProviders)

export function Component() {
  const { t } = useTranslation(["authProviders", "common"])
  const queryClient = useQueryClient()
  const canUpdate = usePermission("system:auth-provider:update")
  const [sheetOpen, setSheetOpen] = useState(false)
  const [editing, setEditing] = useState<AuthProvider | null>(null)

  const { data: providers = [], isLoading } = useQuery({
    queryKey: ["auth-providers"],
    queryFn: () => api.get<AuthProvider[]>("/api/v1/admin/auth-providers"),
  })

  const toggleMutation = useMutation({
    mutationFn: (key: string) =>
      api.patch(`/api/v1/admin/auth-providers/${key}/toggle`),
    onSuccess: () =>
      queryClient.invalidateQueries({ queryKey: ["auth-providers"] }),
    onError: (err) => toast.error(err.message),
  })

  function handleEdit(provider: AuthProvider) {
    setEditing(provider)
    setSheetOpen(true)
  }

  return (
    <div className="workspace-page">
      <div className="workspace-page-header">
        <div>
          <h2 className="workspace-page-title">{t("title")}</h2>
        </div>
      </div>

      <DataTableCard>
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead className="min-w-[180px]">{t("provider")}</TableHead>
              <TableHead className="min-w-[180px]">{t("clientId")}</TableHead>
              <TableHead className="min-w-[180px]">{t("clientSecret")}</TableHead>
              <TableHead className="min-w-[220px]">{t("callbackUrl")}</TableHead>
              <TableHead className="w-[100px]">{t("common:status")}</TableHead>
              <DataTableActionsHead className="min-w-[108px]">
                {t("common:actions")}
              </DataTableActionsHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {isLoading ? (
              <DataTableLoadingRow colSpan={6} />
            ) : providers.length === 0 ? (
              <DataTableEmptyRow colSpan={6} icon={KeyRound} title={t("emptyTitle")} />
            ) : (
              providers.map((p) => (
                <TableRow key={p.providerKey}>
                  <TableCell className="font-medium">
                    {p.displayName}
                    <span className="ml-2 text-xs text-muted-foreground">
                      {p.providerKey}
                    </span>
                  </TableCell>
                  <TableCell className="font-mono text-sm text-muted-foreground">
                    {p.clientId || "-"}
                  </TableCell>
                  <TableCell className="max-w-[220px] text-sm text-muted-foreground">
                    <span className="block truncate" title={p.clientSecret || "-"}>
                      {p.clientSecret || "-"}
                    </span>
                  </TableCell>
                  <TableCell className="text-sm text-muted-foreground max-w-[200px] truncate">
                    {p.callbackUrl || "-"}
                  </TableCell>
                  <TableCell>
                    <Switch
                      checked={p.enabled}
                      onCheckedChange={() =>
                        toggleMutation.mutate(p.providerKey)
                      }
                      disabled={!canUpdate || toggleMutation.isPending}
                    />
                  </TableCell>
                  <DataTableActionsCell className="text-center">
                    {canUpdate && (
                      <WorkspaceIconAction label={t("common:edit")} icon={Pencil} onClick={() => handleEdit(p)} />
                    )}
                  </DataTableActionsCell>
                </TableRow>
              ))
            )}
          </TableBody>
        </Table>
      </DataTableCard>

      <ProviderSheet
        open={sheetOpen}
        onOpenChange={setSheetOpen}
        provider={editing}
      />
    </div>
  )
}
