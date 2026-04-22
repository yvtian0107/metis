import { useState } from "react"
import { useTranslation } from "react-i18next"
import { useMutation, useQueryClient } from "@tanstack/react-query"
import { Plus, Pencil, Trash2, Mail, Send } from "lucide-react"
import { api } from "@/lib/api"
import { usePermission } from "@/hooks/use-permission"
import { useListPage } from "@/hooks/use-list-page"
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
import zhCNChannels from "@/i18n/locales/zh-CN/channels.json"
import enChannels from "@/i18n/locales/en/channels.json"
import { ChannelSheet, type ChannelItem } from "./channel-sheet"
import { SendTestDialog } from "./send-test-dialog"
import { CHANNEL_TYPES } from "./channel-types"

addKernelNamespace("channels", zhCNChannels, enChannels)

export function Component() {
  const { t } = useTranslation(["channels", "common"])
  const queryClient = useQueryClient()
  const [formOpen, setFormOpen] = useState(false)
  const [editing, setEditing] = useState<ChannelItem | null>(null)
  const [sendTestOpen, setSendTestOpen] = useState(false)
  const [sendTestChannelId, setSendTestChannelId] = useState<number | null>(null)

  const canCreate = usePermission("system:channel:create")
  const canUpdate = usePermission("system:channel:update")
  const canDelete = usePermission("system:channel:delete")

  const {
    keyword, setKeyword, page, setPage,
    items: channels, total, totalPages, isLoading, handleSearch,
  } = useListPage<ChannelItem>({ queryKey: "channels", endpoint: "/api/v1/channels" })

  const deleteMutation = useMutation({
    mutationFn: (id: number) => api.delete(`/api/v1/channels/${id}`),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["channels"] })
      toast.success(t("toast.deleted"))
    },
    onError: (err) => toast.error(err.message),
  })

  const toggleMutation = useMutation({
    mutationFn: (id: number) => api.put(`/api/v1/channels/${id}/toggle`, {}),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["channels"] })
    },
    onError: (err) => toast.error(err.message),
  })

  function handleCreate() {
    setEditing(null)
    setFormOpen(true)
  }

  function handleEdit(item: ChannelItem) {
    setEditing(item)
    setFormOpen(true)
  }

  function handleSendTest(id: number) {
    setSendTestChannelId(id)
    setSendTestOpen(true)
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
              <TableHead className="min-w-[220px]">{t("common:name")}</TableHead>
              <TableHead className="w-[110px]">{t("common:type")}</TableHead>
              <TableHead className="w-[100px]">{t("common:status")}</TableHead>
              <TableHead className="w-[150px]">{t("common:createdAt")}</TableHead>
              <DataTableActionsHead className="min-w-[260px]">{t("common:actions")}</DataTableActionsHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {isLoading ? (
              <DataTableLoadingRow colSpan={6} />
            ) : channels.length === 0 ? (
              <DataTableEmptyRow
                colSpan={6}
                icon={Mail}
                title={t("emptyTitle")}
                description={canCreate ? t("emptyDescription") : undefined}
              />
            ) : (
              channels.map((item) => (
                <TableRow key={item.id}>
                  <TableCell className="font-mono text-sm">{item.id}</TableCell>
                  <TableCell className="font-medium">{item.name}</TableCell>
                  <TableCell>
                    <Badge variant="secondary">
                      {t(CHANNEL_TYPES[item.type]?.labelKey ?? item.type)}
                    </Badge>
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
                        <WorkspaceIconAction label={t("sendTest")} icon={Send} onClick={() => handleSendTest(item.id)} />
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

      <DataTablePagination
        total={total}
        page={page}
        totalPages={totalPages}
        onPageChange={setPage}
      />

      <ChannelSheet open={formOpen} onOpenChange={setFormOpen} channel={editing} />
      <SendTestDialog open={sendTestOpen} onOpenChange={setSendTestOpen} channelId={sendTestChannelId} />
    </div>
  )
}
