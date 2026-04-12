import { useState } from "react"
import { useTranslation } from "react-i18next"
import { useMutation, useQueryClient } from "@tanstack/react-query"
import { Link } from "react-router"
import {
  Plus, Search, BookOpen, Pencil, Trash2, RefreshCw, ExternalLink,
} from "lucide-react"
import { usePermission } from "@/hooks/use-permission"
import { useListPage } from "@/hooks/use-list-page"
import { api } from "@/lib/api"
import { toast } from "sonner"
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
import { KnowledgeBaseForm } from "./components/knowledge-base-form"

export interface KnowledgeBaseItem {
  id: number
  name: string
  description: string
  sourceCount: number
  nodeCount: number
  edgeCount: number
  compileStatus: string
  compileMethod: string
  compileModelId: number
  compileConfig?: {
    targetContentLength: number
    minContentLength: number
    maxChunkSize: number
  }
  embeddingProviderId: number | null
  embeddingModelId: string
  autoCompile: boolean
  createdAt: string
  updatedAt: string
}

type CompileStatus = "idle" | "compiling" | "completed" | "error"

function CompileStatusBadge({ status }: { status: string }) {
  const { t } = useTranslation("ai")
  const s = status as CompileStatus

  if (s === "compiling") {
    return (
      <Badge variant="outline" className="border-transparent bg-blue-100 text-blue-700 dark:bg-blue-900/30 dark:text-blue-400 animate-pulse">
        {t("knowledge.compileStatus.compiling")}
      </Badge>
    )
  }
  if (s === "completed") {
    return (
      <Badge variant="outline" className="border-transparent bg-green-500/20 text-green-700 dark:bg-green-500/20 dark:text-green-400">
        {t("knowledge.compileStatus.completed")}
      </Badge>
    )
  }
  if (s === "error") {
    return (
      <Badge variant="destructive">
        {t("knowledge.compileStatus.error")}
      </Badge>
    )
  }
  return (
    <Badge variant="secondary">
      {t("knowledge.compileStatus.idle")}
    </Badge>
  )
}

export function Component() {
  const { t } = useTranslation(["ai", "common"])
  const queryClient = useQueryClient()
  const [formOpen, setFormOpen] = useState(false)
  const [editing, setEditing] = useState<KnowledgeBaseItem | null>(null)

  const canCreate = usePermission("ai:knowledge:create")
  const canUpdate = usePermission("ai:knowledge:update")
  const canDelete = usePermission("ai:knowledge:delete")
  const canCompile = usePermission("ai:knowledge:compile")

  const {
    keyword, setKeyword, page, setPage,
    items: knowledgeBases, total, totalPages, isLoading, handleSearch,
  } = useListPage<KnowledgeBaseItem>({
    queryKey: "ai-knowledge-bases",
    endpoint: "/api/v1/ai/knowledge-bases",
  })

  const deleteMutation = useMutation({
    mutationFn: (id: number) => api.delete(`/api/v1/ai/knowledge-bases/${id}`),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["ai-knowledge-bases"] })
      toast.success(t("ai:knowledge.deleteSuccess"))
    },
    onError: (err) => toast.error(err.message),
  })

  const compileMutation = useMutation({
    mutationFn: (id: number) => api.post(`/api/v1/ai/knowledge-bases/${id}/compile`),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["ai-knowledge-bases"] })
      toast.success(t("ai:knowledge.compileStarted"))
    },
    onError: (err) => toast.error(err.message),
  })

  function handleCreate() {
    setEditing(null)
    setFormOpen(true)
  }

  function handleEdit(item: KnowledgeBaseItem) {
    setEditing(item)
    setFormOpen(true)
  }

  const colSpan = 7

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <h2 className="text-lg font-semibold">{t("ai:knowledge.title")}</h2>
        {canCreate && (
          <Button size="sm" onClick={handleCreate}>
            <Plus className="mr-1.5 h-4 w-4" />
            {t("ai:knowledge.create")}
          </Button>
        )}
      </div>

      <DataTableToolbar>
        <DataTableToolbarGroup>
          <form onSubmit={handleSearch} className="flex w-full flex-col gap-2 sm:flex-row sm:items-center">
            <div className="relative w-full sm:max-w-sm">
              <Search className="absolute left-2.5 top-2.5 h-4 w-4 text-muted-foreground" />
              <Input
                placeholder={t("ai:knowledge.searchPlaceholder")}
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
              <TableHead className="min-w-[160px]">{t("ai:knowledge.name")}</TableHead>
              <TableHead className="min-w-[200px]">{t("ai:knowledge.description")}</TableHead>
              <TableHead className="w-[90px]">{t("ai:knowledge.sourceCount")}</TableHead>
              <TableHead className="w-[90px]">{t("ai:knowledge.nodeCount")}</TableHead>
              <TableHead className="w-[110px]">{t("ai:knowledge.compileStatus.label")}</TableHead>
              <TableHead className="w-[150px]">{t("common:createdAt")}</TableHead>
              <DataTableActionsHead className="min-w-[180px]">{t("common:actions")}</DataTableActionsHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {isLoading ? (
              <DataTableLoadingRow colSpan={colSpan} />
            ) : knowledgeBases.length === 0 ? (
              <DataTableEmptyRow
                colSpan={colSpan}
                icon={BookOpen}
                title={t("ai:knowledge.empty")}
                description={canCreate ? t("ai:knowledge.emptyHint") : undefined}
              />
            ) : (
              knowledgeBases.map((item) => (
                <TableRow key={item.id}>
                  <TableCell className="font-medium">
                    <Link
                      to={`/ai/knowledge/${item.id}`}
                      className="hover:underline text-primary"
                    >
                      {item.name}
                    </Link>
                  </TableCell>
                  <TableCell className="text-sm text-muted-foreground max-w-[200px] truncate">
                    {item.description || "—"}
                  </TableCell>
                  <TableCell className="text-sm">{item.sourceCount}</TableCell>
                  <TableCell className="text-sm">{item.nodeCount}</TableCell>
                  <TableCell>
                    <CompileStatusBadge status={item.compileStatus} />
                  </TableCell>
                  <TableCell className="text-sm text-muted-foreground whitespace-nowrap">
                    {formatDateTime(item.createdAt)}
                  </TableCell>
                  <DataTableActionsCell>
                    <DataTableActions>
                      <Button
                        variant="ghost"
                        size="sm"
                        className="px-2"
                        asChild
                      >
                        <Link to={`/ai/knowledge/${item.id}`}>
                          <ExternalLink className="mr-1 h-3.5 w-3.5" />
                          {t("common:view")}
                        </Link>
                      </Button>
                      {canCompile && (
                        <Button
                          variant="ghost"
                          size="sm"
                          className="px-2"
                          disabled={compileMutation.isPending || item.compileStatus === "compiling"}
                          onClick={() => compileMutation.mutate(item.id)}
                        >
                          <RefreshCw className="mr-1 h-3.5 w-3.5" />
                          {t("ai:knowledge.compile")}
                        </Button>
                      )}
                      {canUpdate && (
                        <Button
                          variant="ghost"
                          size="sm"
                          className="px-2"
                          onClick={() => handleEdit(item)}
                        >
                          <Pencil className="mr-1 h-3.5 w-3.5" />
                          {t("common:edit")}
                        </Button>
                      )}
                      {canDelete && (
                        <AlertDialog>
                          <AlertDialogTrigger asChild>
                            <Button
                              variant="ghost"
                              size="sm"
                              className="px-2 text-destructive hover:text-destructive"
                            >
                              <Trash2 className="mr-1 h-3.5 w-3.5" />
                              {t("common:delete")}
                            </Button>
                          </AlertDialogTrigger>
                          <AlertDialogContent>
                            <AlertDialogHeader>
                              <AlertDialogTitle>{t("ai:knowledge.deleteTitle")}</AlertDialogTitle>
                              <AlertDialogDescription>
                                {t("ai:knowledge.deleteDesc", { name: item.name })}
                              </AlertDialogDescription>
                            </AlertDialogHeader>
                            <AlertDialogFooter>
                              <AlertDialogCancel>{t("common:cancel")}</AlertDialogCancel>
                              <AlertDialogAction
                                onClick={() => deleteMutation.mutate(item.id)}
                                disabled={deleteMutation.isPending}
                              >
                                {t("ai:knowledge.confirmDelete")}
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

      <KnowledgeBaseForm
        open={formOpen}
        onOpenChange={setFormOpen}
        knowledgeBase={editing}
      />
    </div>
  )
}
