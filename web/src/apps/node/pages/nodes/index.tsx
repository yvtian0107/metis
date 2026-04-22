import { useState } from "react"
import { useNavigate } from "react-router"
import { useTranslation } from "react-i18next"
import { useMutation, useQueryClient } from "@tanstack/react-query"
import { Plus, Server, Eye, Pencil, Trash2, Copy, Check } from "lucide-react"
import { usePermission } from "@/hooks/use-permission"
import { useListPage } from "@/hooks/use-list-page"
import { api } from "@/lib/api"
import { toast } from "sonner"
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
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog"
import { formatDateTime } from "@/lib/utils"
import { NodeSheet, type NodeItem } from "../../components/node-sheet"
import {
  WorkspaceAlertIconAction,
  WorkspaceIconAction,
  WorkspaceSearchField,
  WorkspaceStatus,
  type WorkspaceStatusTone,
} from "@/components/workspace/primitives"

function nodeStatusTone(status: string): WorkspaceStatusTone {
  if (status === "online") return "success"
  if (status === "pending") return "warning"
  if (status === "offline") return "neutral"
  return "info"
}

export function Component() {
  const { t } = useTranslation(["node", "common"])
  const navigate = useNavigate()
  const queryClient = useQueryClient()
  const [formOpen, setFormOpen] = useState(false)
  const [editing, setEditing] = useState<NodeItem | null>(null)
  const [statusFilter, setStatusFilter] = useState("")
  const [tokenDialogOpen, setTokenDialogOpen] = useState(false)
  const [createdToken, setCreatedToken] = useState("")
  const [copied, setCopied] = useState(false)

  const canCreate = usePermission("node:create")
  const canUpdate = usePermission("node:update")
  const canDelete = usePermission("node:delete")

  const {
    keyword, setKeyword, page, setPage,
    items: nodes, total, totalPages, isLoading, handleSearch,
  } = useListPage<NodeItem>({
    queryKey: "nodes",
    endpoint: "/api/v1/nodes",
    extraParams: statusFilter ? { status: statusFilter } : undefined,
  })

  const deleteMutation = useMutation({
    mutationFn: (id: number) => api.delete(`/api/v1/nodes/${id}`),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["nodes"] })
      toast.success(t("node:nodes.deleteSuccess"))
    },
    onError: (err) => toast.error(err.message),
  })

  function handleCreate() {
    setEditing(null)
    setFormOpen(true)
  }

  function handleEdit(item: NodeItem) {
    setEditing(item)
    setFormOpen(true)
  }

  function handleStatusFilter(value: string) {
    setStatusFilter(value === "all" ? "" : value)
    setPage(1)
  }

  function handleCreated(token: string) {
    setCreatedToken(token)
    setTokenDialogOpen(true)
    setCopied(false)
  }

  function handleCopyToken() {
    navigator.clipboard.writeText(createdToken).then(() => {
      setCopied(true)
      toast.success(t("node:nodes.tokenCopied"))
      setTimeout(() => setCopied(false), 2000)
    })
  }

  return (
    <div className="workspace-page">
      <div className="workspace-page-header">
        <div>
          <h2 className="workspace-page-title">{t("node:nodes.title")}</h2>
        </div>
        {canCreate && (
          <Button size="sm" onClick={handleCreate}>
            <Plus className="mr-1.5 h-4 w-4" />
            {t("node:nodes.create")}
          </Button>
        )}
      </div>

      <DataTableToolbar>
        <DataTableToolbarGroup>
          <form onSubmit={handleSearch} className="flex w-full flex-col gap-2 sm:flex-row sm:items-center">
            <WorkspaceSearchField
              value={keyword}
              onChange={setKeyword}
              placeholder={t("node:nodes.searchPlaceholder")}
              className="sm:w-80"
            />
            <Select value={statusFilter || "all"} onValueChange={handleStatusFilter}>
              <SelectTrigger className="w-full sm:w-[130px]">
                <SelectValue placeholder={t("node:nodes.allStatus")} />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="all">{t("node:nodes.allStatus")}</SelectItem>
                <SelectItem value="pending">{t("node:status.pending")}</SelectItem>
                <SelectItem value="online">{t("node:status.online")}</SelectItem>
                <SelectItem value="offline">{t("node:status.offline")}</SelectItem>
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
              <TableHead className="w-[100px]">{t("common:status")}</TableHead>
              <TableHead className="w-[80px]">{t("node:nodes.processCount")}</TableHead>
              <TableHead className="w-[150px]">{t("node:nodes.lastHeartbeat")}</TableHead>
              <TableHead className="w-[150px]">{t("common:createdAt")}</TableHead>
              <DataTableActionsHead className="min-w-[160px]">{t("common:actions")}</DataTableActionsHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {isLoading ? (
              <DataTableLoadingRow colSpan={6} />
            ) : nodes.length === 0 ? (
              <DataTableEmptyRow
                colSpan={6}
                icon={Server}
                title={t("node:nodes.empty")}
                description={canCreate ? t("node:nodes.emptyHint") : undefined}
              />
            ) : (
              nodes.map((item) => {
                return (
                  <TableRow key={item.id} className="cursor-pointer" onClick={() => navigate(`/node/nodes/${item.id}`)}>
                    <TableCell className="font-medium">{item.name}</TableCell>
                    <TableCell>
                      <WorkspaceStatus tone={nodeStatusTone(item.status)} label={t(`node:status.${item.status}`, item.status)} />
                    </TableCell>
                    <TableCell className="text-sm">{item.processCount}</TableCell>
                    <TableCell className="text-sm text-muted-foreground whitespace-nowrap">
                      {item.lastHeartbeat ? formatDateTime(item.lastHeartbeat) : t("node:nodes.neverConnected")}
                    </TableCell>
                    <TableCell className="text-sm text-muted-foreground whitespace-nowrap">
                      {formatDateTime(item.createdAt)}
                    </TableCell>
                    <DataTableActionsCell>
                      <DataTableActions>
                        <WorkspaceIconAction
                          label={t("node:nodes.detail")}
                          icon={Eye}
                          onClick={(e) => { e.stopPropagation(); navigate(`/node/nodes/${item.id}`) }}
                        />
                        {canUpdate && (
                          <WorkspaceIconAction
                            label={t("common:edit")}
                            icon={Pencil}
                            onClick={(e) => { e.stopPropagation(); handleEdit(item) }}
                          />
                        )}
                        {canDelete && (
                          <AlertDialog>
                            <span onClick={(e) => e.stopPropagation()}>
                              <WorkspaceAlertIconAction label={t("common:delete")} icon={Trash2} className="hover:text-destructive" />
                            </span>
                            <AlertDialogContent onClick={(e) => e.stopPropagation()}>
                              <AlertDialogHeader>
                                <AlertDialogTitle>{t("node:nodes.deleteTitle")}</AlertDialogTitle>
                                <AlertDialogDescription>
                                  {t("node:nodes.deleteDesc", { name: item.name })}
                                </AlertDialogDescription>
                              </AlertDialogHeader>
                              <AlertDialogFooter>
                                <AlertDialogCancel>{t("common:cancel")}</AlertDialogCancel>
                                <AlertDialogAction
                                  onClick={() => deleteMutation.mutate(item.id)}
                                  disabled={deleteMutation.isPending}
                                >
                                  {t("node:nodes.confirmDelete")}
                                </AlertDialogAction>
                              </AlertDialogFooter>
                            </AlertDialogContent>
                          </AlertDialog>
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

      <NodeSheet open={formOpen} onOpenChange={setFormOpen} node={editing} onCreated={handleCreated} />

      {/* Token display dialog after node creation */}
      <Dialog open={tokenDialogOpen} onOpenChange={setTokenDialogOpen}>
        <DialogContent className="sm:max-w-xl">
          <DialogHeader>
            <DialogTitle>{t("node:nodes.tokenTitle")}</DialogTitle>
            <DialogDescription>{t("node:nodes.tokenDesc")}</DialogDescription>
          </DialogHeader>
          <div className="space-y-4">
            <div className="relative">
              <pre className="rounded-lg bg-muted p-3 pr-10 text-xs font-mono break-all whitespace-pre-wrap">{createdToken}</pre>
              <Button
                variant="ghost"
                size="icon"
                className="absolute right-1 top-1 h-7 w-7"
                onClick={handleCopyToken}
              >
                {copied ? <Check className="h-3.5 w-3.5" /> : <Copy className="h-3.5 w-3.5" />}
              </Button>
            </div>
            <div className="space-y-3 text-sm">
              <p className="font-medium">{t("node:nodes.installGuide")}</p>
              <div className="space-y-1">
                <p className="text-muted-foreground">{t("node:nodes.installStep1")}</p>
              </div>
              <div className="space-y-1">
                <p className="text-muted-foreground">{t("node:nodes.installStep2")}</p>
                <pre className="rounded-lg bg-muted p-3 text-xs font-mono break-all whitespace-pre-wrap">{`server_url: "${window.location.origin}"\ntoken: "${createdToken}"`}</pre>
              </div>
              <div className="space-y-1">
                <p className="text-muted-foreground">{t("node:nodes.installStep3")}</p>
                <pre className="rounded-lg bg-muted p-3 text-xs font-mono">./sidecar -config sidecar.yaml</pre>
              </div>
            </div>
          </div>
        </DialogContent>
      </Dialog>
    </div>
  )
}
