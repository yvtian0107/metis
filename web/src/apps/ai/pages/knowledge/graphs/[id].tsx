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
import { Badge } from "@/components/ui/badge"
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
import { KgFormSheet } from "./components/kg-form-sheet"
import type {
  KnowledgeAsset, AssetSourceItem, NodeItem, SearchResult, LogItem, BuildProgress, GraphResponse,
} from "../_shared/types"

// ── Hooks ────────────────────────────────────────────────────────────────────

function useBuildProgress(id: number, status: string) {
  const queryClient = useQueryClient()
  const isBuilding = status === "building"

  const { data: progress } = useQuery({
    queryKey: ["ai-kg-build-progress", id],
    queryFn: () => api.get<BuildProgress>(`/api/v1/ai/knowledge/graphs/${id}/progress`),
    enabled: isBuilding,
    refetchInterval: isBuilding ? 2000 : false,
    staleTime: 0,
  })

  if (progress?.stage === "completed" && isBuilding) {
    queryClient.invalidateQueries({ queryKey: ["ai-kg-detail", id] })
  }

  return progress ?? null
}

// ── Overview Tab ─────────────────────────────────────────────────────────────

function OverviewTab({ kg, progress }: { kg: KnowledgeAsset; progress: BuildProgress | null }) {
  const queryClient = useQueryClient()
  const canBuild = usePermission("ai:knowledge:compile")

  const buildMutation = useMutation({
    mutationFn: () => api.post(`/api/v1/ai/knowledge/graphs/${kg.id}/build`),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["ai-kg-detail", kg.id] })
      toast.success("构建任务已启动")
    },
    onError: (err) => toast.error(err.message),
  })

  const rebuildMutation = useMutation({
    mutationFn: () => api.post(`/api/v1/ai/knowledge/graphs/${kg.id}/rebuild`),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["ai-kg-detail", kg.id] })
      toast.success("重建任务已启动")
    },
    onError: (err) => toast.error(err.message),
  })

  return (
    <div className="space-y-4">
      <div className="grid grid-cols-2 gap-4 sm:grid-cols-4">
        <StatCard label="素材数" value={kg.sourceCount} />
        <StatCard label="节点数" value={kg.nodeCount} />
        <StatCard label="边数" value={kg.edgeCount} />
        <StatCard label="状态" value={<AssetStatusBadge status={kg.status} />} />
      </div>

      <BuildProgressDisplay buildProgress={progress} />

      {canBuild && (
        <div className="flex gap-2">
          <Button
            size="sm"
            variant="outline"
            disabled={buildMutation.isPending || rebuildMutation.isPending || kg.status === "building"}
            onClick={() => buildMutation.mutate()}
          >
            <RefreshCw className="mr-1.5 h-4 w-4" />
            构建
          </Button>
          {kg.status === "ready" && (
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

function SourcesTab({ kgId }: { kgId: number }) {
  const queryClient = useQueryClient()
  const canCreate = usePermission("ai:knowledge:create")
  const [pickerOpen, setPickerOpen] = useState(false)

  const { data, isLoading } = useQuery({
    queryKey: ["ai-asset-sources", kgId, "kg"],
    queryFn: () => api.get<{ items: AssetSourceItem[]; total: number }>(
      `/api/v1/ai/knowledge/graphs/${kgId}/sources?pageSize=100`,
    ),
  })
  const sources = data?.items ?? []

  const removeMutation = useMutation({
    mutationFn: (sourceId: number) =>
      api.delete(`/api/v1/ai/knowledge/graphs/${kgId}/sources/${sourceId}`),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["ai-asset-sources", kgId, "kg"] })
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
        assetId={kgId}
        addSourcesEndpoint={`/api/v1/ai/knowledge/graphs/${kgId}/sources`}
        onSuccess={() => {
          queryClient.invalidateQueries({ queryKey: ["ai-asset-sources", kgId, "kg"] })
          queryClient.invalidateQueries({ queryKey: ["ai-kg-detail", kgId] })
        }}
      />
    </div>
  )
}

// ── Graph Tab ────────────────────────────────────────────────────────────────

function GraphTab({ kgId }: { kgId: number }) {
  const { data, isLoading } = useQuery({
    queryKey: ["ai-kg-graph", kgId],
    queryFn: () => api.get<GraphResponse>(`/api/v1/ai/knowledge/graphs/${kgId}/graph`),
  })

  if (isLoading) {
    return <p className="text-sm text-muted-foreground py-8 text-center">加载图谱数据...</p>
  }

  if (!data || (data.nodes.length === 0 && data.edges.length === 0)) {
    return (
      <div className="flex flex-col items-center gap-2 py-12 text-muted-foreground">
        <p className="text-sm font-medium">图谱为空</p>
        <p className="text-xs">构建知识图谱后将在此展示节点和关系</p>
      </div>
    )
  }

  // Placeholder: in future, render with force-directed graph component
  return (
    <div className="rounded-lg border p-4 space-y-3">
      <div className="flex gap-4 text-sm text-muted-foreground">
        <span>{data.nodes.length} 个节点</span>
        <span>{data.edges.length} 条边</span>
      </div>
      <p className="text-xs text-muted-foreground">
        图谱可视化组件待接入。当前可在「节点」标签页查看详细数据。
      </p>
    </div>
  )
}

// ── Nodes Tab ────────────────────────────────────────────────────────────────

function NodesTab({ kgId }: { kgId: number }) {
  const [page, setPage] = useState(1)
  const [keyword, setKeyword] = useState("")
  const [searchKeyword, setSearchKeyword] = useState("")
  const pageSize = 20

  const { data, isLoading } = useQuery({
    queryKey: ["ai-kg-nodes", kgId, page, searchKeyword],
    queryFn: () => {
      const params = new URLSearchParams({ page: String(page), pageSize: String(pageSize) })
      if (searchKeyword) params.set("keyword", searchKeyword)
      return api.get<PaginatedResponse<NodeItem>>(
        `/api/v1/ai/knowledge/graphs/${kgId}/nodes?${params}`,
      )
    },
  })

  const nodes = data?.items ?? []
  const total = data?.total ?? 0
  const totalPages = Math.ceil(total / pageSize)

  function handleSearch(e: React.FormEvent) {
    e.preventDefault()
    setSearchKeyword(keyword)
    setPage(1)
  }

  return (
    <div className="space-y-4">
      <form onSubmit={handleSearch} className="flex gap-2">
        <div className="relative flex-1 max-w-sm">
          <Search className="absolute left-2.5 top-2.5 h-4 w-4 text-muted-foreground" />
          <Input
            placeholder="搜索节点..."
            value={keyword}
            onChange={(e) => setKeyword(e.target.value)}
            className="pl-8"
          />
        </div>
        <Button type="submit" variant="outline">搜索</Button>
      </form>

      <DataTableCard>
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead className="min-w-[160px]">标题</TableHead>
              <TableHead className="min-w-[200px]">摘要</TableHead>
              <TableHead className="w-[100px]">类型</TableHead>
              <TableHead className="w-[150px]">关键词</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {isLoading ? (
              <TableRow>
                <TableCell colSpan={4} className="h-28 text-center text-sm text-muted-foreground">加载中...</TableCell>
              </TableRow>
            ) : nodes.length === 0 ? (
              <TableRow>
                <TableCell colSpan={4} className="h-44 text-center text-sm text-muted-foreground">暂无节点数据</TableCell>
              </TableRow>
            ) : (
              nodes.map((node) => (
                <TableRow key={node.id}>
                  <TableCell className="font-medium">{node.title}</TableCell>
                  <TableCell className="text-sm text-muted-foreground max-w-[200px] truncate">{node.summary || "—"}</TableCell>
                  <TableCell>
                    <Badge variant="outline">{node.nodeType}</Badge>
                  </TableCell>
                  <TableCell className="text-sm text-muted-foreground">
                    {node.keywords?.slice(0, 3).join(", ") || "—"}
                  </TableCell>
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

function SearchTab({ kgId }: { kgId: number }) {
  const [query, setQuery] = useState("")
  const [searchQuery, setSearchQuery] = useState("")

  const { data, isLoading } = useQuery({
    queryKey: ["ai-kg-search", kgId, searchQuery],
    queryFn: () =>
      api.post<SearchResult[]>(`/api/v1/ai/knowledge/graphs/${kgId}/search`, {
        query: searchQuery,
        topK: 10,
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

function LogsTab({ kgId }: { kgId: number }) {
  const { data, isLoading } = useQuery({
    queryKey: ["ai-kg-logs", kgId],
    queryFn: () => api.get<LogItem[]>(`/api/v1/ai/knowledge/graphs/${kgId}/logs?limit=50`),
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

function SettingsTab({ kg }: { kg: KnowledgeAsset }) {
  const queryClient = useQueryClient()
  const [editOpen, setEditOpen] = useState(false)

  const deleteMutation = useMutation({
    mutationFn: () => api.delete(`/api/v1/ai/knowledge/graphs/${kg.id}`),
    onSuccess: () => {
      toast.success("知识图谱已删除")
      window.history.back()
    },
    onError: (err) => toast.error(err.message),
  })

  return (
    <div className="space-y-6 max-w-2xl">
      <div>
        <Button size="sm" variant="outline" onClick={() => setEditOpen(true)}>编辑知识图谱</Button>
      </div>

      <div className="rounded-lg border border-destructive/30 p-4 space-y-3">
        <h3 className="text-sm font-semibold text-destructive">危险区域</h3>
        <p className="text-sm text-muted-foreground">删除知识图谱将清除所有节点和关系数据，此操作不可撤销。</p>
        <AlertDialog>
          <AlertDialogTrigger asChild>
            <Button variant="destructive" size="sm">删除知识图谱</Button>
          </AlertDialogTrigger>
          <AlertDialogContent>
            <AlertDialogHeader>
              <AlertDialogTitle>确认删除</AlertDialogTitle>
              <AlertDialogDescription>
                确定要删除「{kg.name}」吗？所有节点和关系将被永久删除。
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

      <KgFormSheet
        open={editOpen}
        onOpenChange={(open) => {
          setEditOpen(open)
          if (!open) queryClient.invalidateQueries({ queryKey: ["ai-kg-detail", kg.id] })
        }}
        knowledgeGraph={kg}
      />
    </div>
  )
}

// ── Main Page ────────────────────────────────────────────────────────────────

export function Component() {
  const { id } = useParams<{ id: string }>()
  const kgId = Number(id)

  const { data: kg, isLoading } = useQuery({
    queryKey: ["ai-kg-detail", kgId],
    queryFn: () => api.get<KnowledgeAsset>(`/api/v1/ai/knowledge/graphs/${kgId}`),
    enabled: !Number.isNaN(kgId),
  })

  const progress = useBuildProgress(kgId, kg?.status ?? "idle")

  if (isLoading) {
    return (
      <div className="flex items-center justify-center h-48 text-sm text-muted-foreground">
        加载中...
      </div>
    )
  }

  if (!kg) {
    return (
      <div className="flex items-center justify-center h-48 text-sm text-muted-foreground">
        知识图谱不存在
      </div>
    )
  }

  return (
    <div className="space-y-4">
      <div className="flex items-center gap-3">
        <Button variant="ghost" size="sm" asChild>
          <Link to="/ai/knowledge/graphs">
            <ArrowLeft className="h-4 w-4" />
          </Link>
        </Button>
        <div className="flex-1 min-w-0">
          <div className="flex items-center gap-2">
            <h2 className="text-lg font-semibold truncate">{kg.name}</h2>
            <AssetStatusBadge status={kg.status} />
          </div>
          {kg.description && (
            <p className="text-sm text-muted-foreground truncate">{kg.description}</p>
          )}
        </div>
      </div>

      <Tabs defaultValue="overview">
        <TabsList>
          <TabsTrigger value="overview">概览</TabsTrigger>
          <TabsTrigger value="sources">素材</TabsTrigger>
          <TabsTrigger value="graph">图谱</TabsTrigger>
          <TabsTrigger value="nodes">节点</TabsTrigger>
          <TabsTrigger value="search">检索测试</TabsTrigger>
          <TabsTrigger value="logs">日志</TabsTrigger>
          <TabsTrigger value="settings">设置</TabsTrigger>
        </TabsList>
        <TabsContent value="overview" className="mt-4">
          <OverviewTab kg={kg} progress={progress} />
        </TabsContent>
        <TabsContent value="sources" className="mt-4">
          <SourcesTab kgId={kgId} />
        </TabsContent>
        <TabsContent value="graph" className="mt-4">
          <GraphTab kgId={kgId} />
        </TabsContent>
        <TabsContent value="nodes" className="mt-4">
          <NodesTab kgId={kgId} />
        </TabsContent>
        <TabsContent value="search" className="mt-4">
          <SearchTab kgId={kgId} />
        </TabsContent>
        <TabsContent value="logs" className="mt-4">
          <LogsTab kgId={kgId} />
        </TabsContent>
        <TabsContent value="settings" className="mt-4">
          <SettingsTab kg={kg} />
        </TabsContent>
      </Tabs>
    </div>
  )
}
