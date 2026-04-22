import { useState } from "react"
import { useTranslation } from "react-i18next"
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import { Plus, Cpu, Pencil, Trash2, Server } from "lucide-react"
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
import {
  Sheet,
  SheetContent,
  SheetHeader,
  SheetTitle,
  SheetDescription,
} from "@/components/ui/sheet"
import { formatDateTime } from "@/lib/utils"
import { ProcessDefSheet, type ProcessDefItem } from "../../components/process-def-sheet"
import { NODE_STATUS_VARIANTS, PROCESS_STATUS_VARIANTS } from "../../constants"
import {
  WorkspaceAlertIconAction,
  WorkspaceIconAction,
  WorkspaceSearchField,
} from "@/components/workspace/primitives"

const RESTART_POLICY_VARIANTS: Record<string, "default" | "secondary" | "outline"> = {
  always: "default",
  on_failure: "secondary",
  never: "outline",
}

interface ProcessDefNodeItem {
  nodeId: number
  nodeName: string
  nodeStatus: string
  processStatus: string
  pid: number
  configVersion: string
  boundAt: string
}

export function Component() {
  const { t } = useTranslation(["node", "common"])
  const queryClient = useQueryClient()
  const [formOpen, setFormOpen] = useState(false)
  const [editing, setEditing] = useState<ProcessDefItem | null>(null)
  const [nodesSheetOpen, setNodesSheetOpen] = useState(false)
  const [viewingDef, setViewingDef] = useState<ProcessDefItem | null>(null)

  const canCreate = usePermission("node:process-def:create")
  const canUpdate = usePermission("node:process-def:update")
  const canDelete = usePermission("node:process-def:delete")

  const {
    keyword, setKeyword, page, setPage,
    items: processDefs, total, totalPages, isLoading, handleSearch,
  } = useListPage<ProcessDefItem>({
    queryKey: "process-defs",
    endpoint: "/api/v1/process-defs",
  })

  const deleteMutation = useMutation({
    mutationFn: (id: number) => api.delete(`/api/v1/process-defs/${id}`),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["process-defs"] })
      toast.success(t("node:processDefs.deleteSuccess"))
    },
    onError: (err) => toast.error(err.message),
  })

  const { data: nodesData, isLoading: isNodesLoading } = useQuery({
    queryKey: ["process-def-nodes", viewingDef?.id],
    queryFn: () => api.get<ProcessDefNodeItem[]>(`/api/v1/process-defs/${viewingDef!.id}/nodes`),
    enabled: nodesSheetOpen && !!viewingDef,
  })

  const defNodes = nodesData ?? []

  function handleCreate() {
    setEditing(null)
    setFormOpen(true)
  }

  function handleEdit(item: ProcessDefItem) {
    setEditing(item)
    setFormOpen(true)
  }

  return (
    <div className="workspace-page">
      <div className="workspace-page-header">
        <div>
          <h2 className="workspace-page-title">{t("node:processDefs.title")}</h2>
        </div>
        {canCreate && (
          <Button size="sm" onClick={handleCreate}>
            <Plus className="mr-1.5 h-4 w-4" />
            {t("node:processDefs.create")}
          </Button>
        )}
      </div>

      <DataTableToolbar>
        <form onSubmit={handleSearch}>
          <WorkspaceSearchField
            value={keyword}
            onChange={setKeyword}
            placeholder={t("node:processDefs.searchPlaceholder")}
            className="sm:w-80"
          />
        </form>
      </DataTableToolbar>

      <DataTableCard>
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead className="min-w-[180px]">{t("node:processDefs.displayName")}</TableHead>
              <TableHead className="w-[150px]">{t("node:processDefs.name")}</TableHead>
              <TableHead className="w-[200px]">{t("node:processDefs.startCommand")}</TableHead>
              <TableHead className="w-[120px]">{t("node:processDefs.restartPolicy")}</TableHead>
              <TableHead className="w-[100px]">{t("node:processDefs.probeType")}</TableHead>
              <TableHead className="w-[150px]">{t("common:createdAt")}</TableHead>
              <DataTableActionsHead className="min-w-[140px]">{t("common:actions")}</DataTableActionsHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {isLoading ? (
              <DataTableLoadingRow colSpan={7} />
            ) : processDefs.length === 0 ? (
              <DataTableEmptyRow
                colSpan={7}
                icon={Cpu}
                title={t("node:processDefs.empty")}
                description={canCreate ? t("node:processDefs.emptyHint") : undefined}
              />
            ) : (
              processDefs.map((item) => {
                const rpVariant = RESTART_POLICY_VARIANTS[item.restartPolicy] ?? ("secondary" as const)
                return (
                  <TableRow key={item.id}>
                    <TableCell className="font-medium">{item.displayName}</TableCell>
                    <TableCell className="font-mono text-sm text-muted-foreground">{item.name}</TableCell>
                    <TableCell className="font-mono text-sm text-muted-foreground truncate max-w-[200px]">
                      {item.startCommand}
                    </TableCell>
                    <TableCell>
                      <Badge variant={rpVariant}>
                        {t(`node:processDefs.${item.restartPolicy}`, item.restartPolicy)}
                      </Badge>
                    </TableCell>
                    <TableCell className="text-sm">
                      {t(`node:processDefs.${item.probeType}`, item.probeType)}
                    </TableCell>
                    <TableCell className="text-sm text-muted-foreground whitespace-nowrap">
                      {formatDateTime(item.createdAt)}
                    </TableCell>
                    <DataTableActionsCell>
                      <DataTableActions>
                        <WorkspaceIconAction
                          label={t("node:processDefs.viewNodes")}
                          icon={Server}
                          onClick={() => { setViewingDef(item); setNodesSheetOpen(true) }}
                        />
                        {canUpdate && (
                          <WorkspaceIconAction label={t("common:edit")} icon={Pencil} onClick={() => handleEdit(item)} />
                        )}
                        {canDelete && (
                          <AlertDialog>
                            <WorkspaceAlertIconAction label={t("common:delete")} icon={Trash2} className="hover:text-destructive" />
                            <AlertDialogContent>
                              <AlertDialogHeader>
                                <AlertDialogTitle>{t("node:processDefs.deleteTitle")}</AlertDialogTitle>
                                <AlertDialogDescription>
                                  {t("node:processDefs.deleteDesc", { name: item.displayName })}
                                </AlertDialogDescription>
                              </AlertDialogHeader>
                              <AlertDialogFooter>
                                <AlertDialogCancel>{t("common:cancel")}</AlertDialogCancel>
                                <AlertDialogAction
                                  onClick={() => deleteMutation.mutate(item.id)}
                                  disabled={deleteMutation.isPending}
                                >
                                  {t("node:processDefs.confirmDelete")}
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

      <ProcessDefSheet open={formOpen} onOpenChange={setFormOpen} processDef={editing} />

      <Sheet open={nodesSheetOpen} onOpenChange={(open) => { setNodesSheetOpen(open); if (!open) setViewingDef(null) }}>
        <SheetContent className="sm:max-w-lg overflow-y-auto">
          <SheetHeader>
            <SheetTitle>{t("node:processDefs.viewNodes")}</SheetTitle>
            <SheetDescription className="sr-only">
              {viewingDef?.displayName}
            </SheetDescription>
          </SheetHeader>
          <div className="px-4">
            {viewingDef && (
              <p className="text-sm text-muted-foreground mb-4">
                {viewingDef.displayName} ({viewingDef.name})
              </p>
            )}
            {isNodesLoading ? (
              <div className="rounded-lg border p-8 text-center text-sm text-muted-foreground">
                {t("common:loading")}
              </div>
            ) : defNodes.length === 0 ? (
              <div className="rounded-lg border p-8 text-center text-sm text-muted-foreground">
                {t("node:processDefs.noNodes")}
              </div>
            ) : (
              <div className="overflow-hidden rounded-xl border bg-card">
                <Table>
                  <TableHeader>
                    <TableRow>
                      <TableHead>{t("common:name")}</TableHead>
                      <TableHead className="w-[80px]">{t("node:processDefs.nodeStatus")}</TableHead>
                      <TableHead className="w-[80px]">{t("node:processDefs.processStatusCol")}</TableHead>
                      <TableHead className="w-[60px]">{t("node:nodes.pid")}</TableHead>
                      <TableHead className="w-[120px]">{t("node:processDefs.boundAt")}</TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {defNodes.map((n) => (
                      <TableRow key={n.nodeId}>
                        <TableCell className="font-medium">{n.nodeName}</TableCell>
                        <TableCell>
                          <Badge variant={NODE_STATUS_VARIANTS[n.nodeStatus] ?? "secondary"}>
                            {t(`node:status.${n.nodeStatus}`, n.nodeStatus)}
                          </Badge>
                        </TableCell>
                        <TableCell>
                          <Badge variant={PROCESS_STATUS_VARIANTS[n.processStatus] ?? "secondary"}>
                            {t(`node:status.${n.processStatus}`, n.processStatus)}
                          </Badge>
                        </TableCell>
                        <TableCell className="font-mono text-sm">{n.pid || "-"}</TableCell>
                        <TableCell className="text-sm text-muted-foreground whitespace-nowrap">
                          {formatDateTime(n.boundAt)}
                        </TableCell>
                      </TableRow>
                    ))}
                  </TableBody>
                </Table>
              </div>
            )}
          </div>
        </SheetContent>
      </Sheet>
    </div>
  )
}
