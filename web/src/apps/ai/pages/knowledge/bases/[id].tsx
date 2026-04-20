import { useState } from "react"
import { useParams, Link } from "react-router"
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import {
  ArrowLeft, RefreshCw, Plus, Search, Trash2, FileText,
} from "lucide-react"
import { api, type PaginatedResponse } from "@/lib/api"
import { toast } from "sonner"
import { usePermission } from "@/hooks/use-permission"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Tabs, TabsList, TabsTrigger, TabsContent } from "@/components/ui/tabs"
import {
  Table, TableBody, TableCell, TableHead, TableHeader, TableRow,
} from "@/components/ui/table"
import { DataTableCard, DataTablePagination } from "@/components/ui/data-table"
import {
  AlertDialog, AlertDialogAction, AlertDialogCancel, AlertDialogContent,
  AlertDialogDescription, AlertDialogFooter, AlertDialogHeader, AlertDialogTitle, AlertDialogTrigger,
} from "@/components/ui/alert-dialog"
import { formatDateTime, formatBytes } from "@/lib/utils"
import { AssetStatusBadge } from "../_shared/asset-status-badge"
import { BuildProgressDisplay } from "../_shared/build-progress"
import { SourcePickerSheet } from "../_shared/source-picker-sheet"
import { KbFormSheet } from "./components/kb-form-sheet"
import type {
  KnowledgeAsset, AssetSourceItem, ChunkItem, SearchResult, LogItem, BuildProgress,
} from "../_shared/types"

// ── Hooks ────────────────────────────────────────────────────────────────────

function useBuildProgress(id: number, status: string) {
  const queryClient = useQueryClient()
  const isBuilding = status === "building"

  const { data: progress } = useQuery({
    queryKey: ["ai-kb-build-progress", id],
    queryFn: () => api.get<BuildProgress>(`/api/v1/ai/knowledge/bases/${id}/progress`),
    enabled: isBuilding,
    refetchInterval: isBuilding ? 2000 : false,
    staleTime: 0,
  })

  // Refresh detail when build completes
  if (progress?.stage === "completed" && isBuilding) {
    queryClient.invalidateQueries({ queryKey: ["ai-kb-detail", id] })
  }

  return progress ?? null
}

// ── Overview Tab ─────────────────────────────────────────────────────────────

function OverviewTab({ kb, progress }: { kb: KnowledgeAsset; progress: BuildProgress | null }) {
  const queryClient = useQueryClient()
  const canBuild = usePermission("ai:knowledge:compile")

  const buildMutation = useMutation({
    mutationFn: () => api.post(`/api/v1/ai/knowledge/bases/${kb.id}/build`),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["ai-kb-detail", kb.id] })
      toast.success("构建任务已启动")
    },
    onError: (err) => toast.error(err.message),
  })

  const rebuildMutation = useMutation({
    mutationFn: () => api.post(`/api/v1/ai/knowledge/bases/${kb.id}/rebuild`),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["ai-kb-detail", kb.id] })
      toast.success("重建任务已启动")
    },
    onError: (err) => toast.error(err.message),
  })

  return (
    <div className="space-y-4">
      <div className="grid grid-cols-2 gap-4 sm:grid-cols-4">
        <StatCard label="素材数" value={kb.sourceCount} />
        <StatCard label="分块数" value={kb.chunkCount} />
        <StatCard label="状态" value={<AssetStatusBadge status={kb.status} />} />
        <StatCard label="最后构建" value={kb.builtAt ? formatDateTime(kb.builtAt) : "—"} />
      </div>

      <BuildProgressDisplay buildProgress={progress} />

      {canBuild && (
        <div className="flex gap-2">
          <Button
            size="sm"
            variant="outline"
            disabled={buildMutation.isPending || rebuildMutation.isPending || kb.status === "building"}
            onClick={() => buildMutation.mutate()}
          >
            <RefreshCw className="mr-1.5 h-4 w-4" />
            构建
          </Button>
          {kb.status === "ready" && (
            <Button
              size="sm"
              variant="outline"
              disabled={rebuildMutation.isPending}
              onClick={() => rebuildMutation.mutate()}
            >
              <RefreshCw className="mr-1.5 h-4 w-4" />
              重建
            </Button>
          )}
        </div>
      )}
    </div>
  )
}

function StatCard({ label, value }: { label: string; value: React.ReactNode }) {
  return (
    <div className="rounded-lg border p-3">
      <p className="text-xs text-muted-foreground">{label}</p>
      <div className="mt-1 text-lg font-semibold">{value}</div>
    </div>
  )
}

// ── Sources Tab ──────────────────────────────────────────────────────────────

function SourcesTab({ kbId }: { kbId: number }) {
  const queryClient = useQueryClient()
  const canCreate = usePermission("ai:knowledge:create")
  const [pickerOpen, setPickerOpen] = useState(false)

  const { data, isLoading } = useQuery({
    queryKey: ["ai-asset-sources", kbId],
    queryFn: () => api.get<{ items: AssetSourceItem[]; total: number }>(
      `/api/v1/ai/knowledge/bases/${kbId}/sources?pageSize=100`,
    ),
  })
  const sources = data?.items ?? []

  const removeMutation = useMutation({
    mutationFn: (sourceId: number) =>
      api.delete(`/api/v1/ai/knowledge/bases/${kbId}/sources/${sourceId}`),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["ai-asset-sources", kbId] })
      toast.success("素材已移除")
    },
    onError: (err) => toast.error(err.message),
  })

  return (
    <div className="space-y-4">
      {canCreate && (
        <div className="flex items-center gap-2">
          <Button size="sm" variant="outline" onClick={() => setPickerOpen(true)}>
            <Plus className="mr-1.5 h-4 w-4" />
            添加素材
          </Button>
        </div>
      )}

      <DataTableCard>
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead className="min-w-[200px]">标题</TableHead>
              <TableHead className="w-[100px]">格式</TableHead>
              <TableHead className="w-[100px]">大小</TableHead>
              <TableHead className="w-[150px]">创建时间</TableHead>
              <TableHead className="w-[80px] text-right">操作</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {isLoading ? (
              <TableRow>
                <TableCell colSpan={5} className="h-28 text-center text-sm text-muted-foreground">加载中...</TableCell>
              </TableRow>
            ) : sources.length === 0 ? (
              <TableRow>
                <TableCell colSpan={5} className="h-44 text-center">
                  <div className="flex flex-col items-center gap-2 text-muted-foreground">
                    <FileText className="h-10 w-10 stroke-1" />
                    <p className="text-sm font-medium">暂无素材</p>
                  </div>
                </TableCell>
              </TableRow>
            ) : (
              sources.map((src) => (
                <TableRow key={src.id}>
                  <TableCell className="font-medium">{src.title}</TableCell>
                  <TableCell className="text-sm text-muted-foreground">{src.format || src.sourceType}</TableCell>
                  <TableCell className="text-sm text-muted-foreground">{formatBytes(src.byteSize)}</TableCell>
                  <TableCell className="text-sm text-muted-foreground whitespace-nowrap">{formatDateTime(src.createdAt)}</TableCell>
                  <TableCell className="text-right">
                    <Button
                      variant="ghost"
                      size="sm"
                      className="px-2 text-destructive hover:text-destructive"
                      disabled={removeMutation.isPending}
                      onClick={() => removeMutation.mutate(src.id)}
                    >
                      <Trash2 className="h-3.5 w-3.5" />
                    </Button>
                  </TableCell>
                </TableRow>
              ))
            )}
          </TableBody>
        </Table>
      </DataTableCard>

      <SourcePickerSheet
        open={pickerOpen}
        onOpenChange={setPickerOpen}
        assetId={kbId}
        addSourcesEndpoint={`/api/v1/ai/knowledge/bases/${kbId}/sources`}
        onSuccess={() => {
          queryClient.invalidateQueries({ queryKey: ["ai-asset-sources", kbId] })
          queryClient.invalidateQueries({ queryKey: ["ai-kb-detail", kbId] })
        }}
      />
    </div>
  )
}

// ── Content Tab (Chunks) ─────────────────────────────────────────────────────

function ContentTab({ kbId }: { kbId: number }) {
  const [page, setPage] = useState(1)
  const pageSize = 20

  const { data, isLoading } = useQuery({
    queryKey: ["ai-kb-chunks", kbId, page],
    queryFn: () =>
      api.get<PaginatedResponse<ChunkItem>>(
        `/api/v1/ai/knowledge/bases/${kbId}/chunks?page=${page}&pageSize=${pageSize}`,
      ),
  })

  const chunks = data?.items ?? []
  const total = data?.total ?? 0
  const totalPages = Math.ceil(total / pageSize)

  return (
    <div className="space-y-4">
      <DataTableCard>
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead className="w-[60px]">#</TableHead>
              <TableHead className="w-[80px]">素材 ID</TableHead>
              <TableHead>内容预览</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {isLoading ? (
              <TableRow>
                <TableCell colSpan={3} className="h-28 text-center text-sm text-muted-foreground">加载中...</TableCell>
              </TableRow>
            ) : chunks.length === 0 ? (
              <TableRow>
                <TableCell colSpan={3} className="h-44 text-center text-sm text-muted-foreground">暂无分块数据</TableCell>
              </TableRow>
            ) : (
              chunks.map((chunk) => (
                <TableRow key={chunk.id}>
                  <TableCell className="text-sm text-muted-foreground">{chunk.chunkIndex}</TableCell>
                  <TableCell className="text-sm text-muted-foreground">{chunk.sourceId}</TableCell>
                  <TableCell className="text-sm max-w-[500px] truncate">{chunk.content}</TableCell>
                </TableRow>
              ))
            )}
          </TableBody>
        </Table>
      </DataTableCard>
      <DataTablePagination total={total} page={page} totalPages={totalPages} onPageChange={setPage} />
    </div>
  )
}

// ── Search Tab ───────────────────────────────────────────────────────────────

function SearchTab({ kbId }: { kbId: number }) {
  const [query, setQuery] = useState("")
  const [searchQuery, setSearchQuery] = useState("")

  const { data, isLoading } = useQuery({
    queryKey: ["ai-kb-search", kbId, searchQuery],
    queryFn: () =>
      api.post<SearchResult[]>(`/api/v1/ai/knowledge/bases/${kbId}/search`, {
        query: searchQuery,
        topK: 10,
        mode: "hybrid",
      }),
    enabled: searchQuery.length > 0,
  })

  function handleSearch(e: React.FormEvent) {
    e.preventDefault()
    setSearchQuery(query)
  }

  return (
    <div className="space-y-4">
      <form onSubmit={handleSearch} className="flex gap-2">
        <div className="relative flex-1">
          <Search className="absolute left-2.5 top-2.5 h-4 w-4 text-muted-foreground" />
          <Input
            placeholder="输入检索内容..."
            value={query}
            onChange={(e) => setQuery(e.target.value)}
            className="pl-8"
          />
        </div>
        <Button type="submit" disabled={!query.trim()}>检索</Button>
      </form>

      {isLoading && <p className="text-sm text-muted-foreground">检索中...</p>}

      {data && data.length > 0 && (
        <div className="grid gap-3">
          {data.map((r) => (
            <div key={r.id} className="rounded-lg border p-3 space-y-1">
              <div className="flex items-center justify-between">
                <p className="text-sm font-medium">{r.title || `#${r.id}`}</p>
                <span className="text-xs text-muted-foreground">score: {r.score.toFixed(3)}</span>
              </div>
              <p className="text-sm text-muted-foreground line-clamp-3">{r.content}</p>
            </div>
          ))}
        </div>
      )}

      {data && data.length === 0 && searchQuery && (
        <p className="text-sm text-muted-foreground text-center py-8">无匹配结果</p>
      )}
    </div>
  )
}

// ── Logs Tab ─────────────────────────────────────────────────────────────────

function LogsTab({ kbId }: { kbId: number }) {
  const { data, isLoading } = useQuery({
    queryKey: ["ai-kb-logs", kbId],
    queryFn: () => api.get<LogItem[]>(`/api/v1/ai/knowledge/bases/${kbId}/logs?limit=50`),
  })

  return (
    <div className="space-y-2">
      {isLoading && <p className="text-sm text-muted-foreground">加载中...</p>}
      {data && data.length === 0 && <p className="text-sm text-muted-foreground text-center py-8">暂无日志</p>}
      {data?.map((log) => (
        <div key={log.id} className="flex items-start gap-3 rounded-lg border p-3">
          <div className="flex-1 min-w-0">
            <p className="text-sm font-medium">{log.action}</p>
            {log.message && <p className="text-xs text-muted-foreground mt-0.5">{log.message}</p>}
          </div>
          <span className="text-xs text-muted-foreground whitespace-nowrap">{formatDateTime(log.createdAt)}</span>
        </div>
      ))}
    </div>
  )
}

// ── Settings Tab ─────────────────────────────────────────────────────────────

function SettingsTab({ kb }: { kb: KnowledgeAsset }) {
  const queryClient = useQueryClient()
  const [editOpen, setEditOpen] = useState(false)

  const deleteMutation = useMutation({
    mutationFn: () => api.delete(`/api/v1/ai/knowledge/bases/${kb.id}`),
    onSuccess: () => {
      toast.success("知识库已删除")
      window.history.back()
    },
    onError: (err) => toast.error(err.message),
  })

  return (
    <div className="space-y-6 max-w-2xl">
      <div>
        <Button size="sm" variant="outline" onClick={() => setEditOpen(true)}>编辑知识库</Button>
      </div>

      <div className="rounded-lg border border-destructive/30 p-4 space-y-3">
        <h3 className="text-sm font-semibold text-destructive">危险区域</h3>
        <p className="text-sm text-muted-foreground">删除知识库将清除所有关联数据，此操作不可撤销。</p>
        <AlertDialog>
          <AlertDialogTrigger asChild>
            <Button variant="destructive" size="sm">删除知识库</Button>
          </AlertDialogTrigger>
          <AlertDialogContent>
            <AlertDialogHeader>
              <AlertDialogTitle>确认删除</AlertDialogTitle>
              <AlertDialogDescription>
                确定要删除「{kb.name}」吗？所有分块和配置将被永久删除。
              </AlertDialogDescription>
            </AlertDialogHeader>
            <AlertDialogFooter>
              <AlertDialogCancel>取消</AlertDialogCancel>
              <AlertDialogAction onClick={() => deleteMutation.mutate()} disabled={deleteMutation.isPending}>
                确认删除
              </AlertDialogAction>
            </AlertDialogFooter>
          </AlertDialogContent>
        </AlertDialog>
      </div>

      <KbFormSheet
        open={editOpen}
        onOpenChange={(open) => {
          setEditOpen(open)
          if (!open) queryClient.invalidateQueries({ queryKey: ["ai-kb-detail", kb.id] })
        }}
        knowledgeBase={kb}
      />
    </div>
  )
}

// ── Main Page ────────────────────────────────────────────────────────────────

export function Component() {
  const { id } = useParams<{ id: string }>()
  const kbId = Number(id)

  const { data: kb, isLoading } = useQuery({
    queryKey: ["ai-kb-detail", kbId],
    queryFn: () => api.get<KnowledgeAsset>(`/api/v1/ai/knowledge/bases/${kbId}`),
    enabled: !Number.isNaN(kbId),
  })

  const progress = useBuildProgress(kbId, kb?.status ?? "idle")

  if (isLoading) {
    return (
      <div className="flex items-center justify-center h-48 text-sm text-muted-foreground">
        加载中...
      </div>
    )
  }

  if (!kb) {
    return (
      <div className="flex items-center justify-center h-48 text-sm text-muted-foreground">
        知识库不存在
      </div>
    )
  }

  return (
    <div className="space-y-4">
      <div className="flex items-center gap-3">
        <Button variant="ghost" size="sm" asChild>
          <Link to="/ai/knowledge/bases">
            <ArrowLeft className="h-4 w-4" />
          </Link>
        </Button>
        <div className="flex-1 min-w-0">
          <div className="flex items-center gap-2">
            <h2 className="text-lg font-semibold truncate">{kb.name}</h2>
            <AssetStatusBadge status={kb.status} />
          </div>
          {kb.description && (
            <p className="text-sm text-muted-foreground truncate">{kb.description}</p>
          )}
        </div>
      </div>

      <Tabs defaultValue="overview">
        <TabsList>
          <TabsTrigger value="overview">概览</TabsTrigger>
          <TabsTrigger value="sources">素材</TabsTrigger>
          <TabsTrigger value="content">内容</TabsTrigger>
          <TabsTrigger value="search">检索测试</TabsTrigger>
          <TabsTrigger value="logs">日志</TabsTrigger>
          <TabsTrigger value="settings">设置</TabsTrigger>
        </TabsList>
        <TabsContent value="overview" className="mt-4">
          <OverviewTab kb={kb} progress={progress} />
        </TabsContent>
        <TabsContent value="sources" className="mt-4">
          <SourcesTab kbId={kbId} />
        </TabsContent>
        <TabsContent value="content" className="mt-4">
          <ContentTab kbId={kbId} />
        </TabsContent>
        <TabsContent value="search" className="mt-4">
          <SearchTab kbId={kbId} />
        </TabsContent>
        <TabsContent value="logs" className="mt-4">
          <LogsTab kbId={kbId} />
        </TabsContent>
        <TabsContent value="settings" className="mt-4">
          <SettingsTab kb={kb} />
        </TabsContent>
      </Tabs>
    </div>
  )
}
