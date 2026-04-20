import { useState } from "react"
import { useMutation, useQueryClient } from "@tanstack/react-query"
import { Link } from "react-router"
import {
  Search, Upload, Globe, FileText, Trash2, ExternalLink,
} from "lucide-react"
import { useListPage } from "@/hooks/use-list-page"
import { api, ApiError } from "@/lib/api"
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
import { formatDateTime, formatBytes } from "@/lib/utils"
import { SourceUploadSheet } from "./components/source-upload-sheet"
import { SourceUrlSheet } from "./components/source-url-sheet"
import { SourceTextSheet } from "./components/source-text-sheet"

export interface SourceItem {
  id: number
  parentId: number | null
  title: string
  format: string
  sourceUrl: string | null
  crawlDepth: number
  crawlEnabled: boolean
  crawlSchedule: string
  lastCrawledAt: string | null
  fileName: string | null
  byteSize: number
  extractStatus: string
  errorMessage: string | null
  refCount: number
  createdAt: string
  updatedAt: string
}

const FORMAT_LABELS: Record<string, string> = {
  pdf: "PDF",
  markdown: "Markdown",
  md: "Markdown",
  txt: "文本",
  text: "文本",
  url: "URL",
  docx: "Word",
  xlsx: "Excel",
  pptx: "PPT",
}

function FormatBadge({ format }: { format: string }) {
  const label = FORMAT_LABELS[format?.toLowerCase()] ?? format?.toUpperCase() ?? "—"
  return <Badge variant="outline">{label}</Badge>
}

function ExtractStatusBadge({ status }: { status: string }) {
  if (status === "completed") {
    return (
      <Badge variant="outline" className="border-transparent bg-green-500/20 text-green-700 dark:bg-green-500/20 dark:text-green-400">
        已完成
      </Badge>
    )
  }
  if (status === "error" || status === "failed") {
    return <Badge variant="destructive">失败</Badge>
  }
  if (status === "processing") {
    return (
      <Badge variant="outline" className="border-transparent bg-blue-100 text-blue-700 dark:bg-blue-900/30 dark:text-blue-400 animate-pulse">
        处理中
      </Badge>
    )
  }
  return <Badge variant="secondary">待处理</Badge>
}

export function Component() {
  const queryClient = useQueryClient()
  const [uploadOpen, setUploadOpen] = useState(false)
  const [urlOpen, setUrlOpen] = useState(false)
  const [textOpen, setTextOpen] = useState(false)

  const {
    keyword, setKeyword, page, setPage,
    items, total, totalPages, isLoading, handleSearch,
  } = useListPage<SourceItem>({
    queryKey: "ai-knowledge-sources",
    endpoint: "/api/v1/ai/knowledge/sources",
  })

  const deleteMutation = useMutation({
    mutationFn: (id: number) => api.delete(`/api/v1/ai/knowledge/sources/${id}`),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["ai-knowledge-sources"] })
      toast.success("素材已删除")
    },
    onError: (err) => {
      if (err instanceof ApiError && err.status === 409) {
        toast.error("该素材正在被知识库引用，无法删除")
      } else {
        toast.error(err.message)
      }
    },
  })

  const colSpan = 7

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <h2 className="text-lg font-semibold">素材管理</h2>
        <div className="flex items-center gap-2">
          <Button size="sm" variant="outline" onClick={() => setUploadOpen(true)}>
            <Upload className="mr-1.5 h-4 w-4" />
            上传文件
          </Button>
          <Button size="sm" variant="outline" onClick={() => setUrlOpen(true)}>
            <Globe className="mr-1.5 h-4 w-4" />
            添加网址
          </Button>
          <Button size="sm" variant="outline" onClick={() => setTextOpen(true)}>
            <FileText className="mr-1.5 h-4 w-4" />
            添加文本
          </Button>
        </div>
      </div>

      <DataTableToolbar>
        <DataTableToolbarGroup>
          <form onSubmit={handleSearch} className="flex w-full flex-col gap-2 sm:flex-row sm:items-center">
            <div className="relative w-full sm:max-w-sm">
              <Search className="absolute left-2.5 top-2.5 h-4 w-4 text-muted-foreground" />
              <Input
                placeholder="搜索素材..."
                value={keyword}
                onChange={(e) => setKeyword(e.target.value)}
                className="pl-8"
              />
            </div>
            <Button type="submit" variant="outline">
              搜索
            </Button>
          </form>
        </DataTableToolbarGroup>
      </DataTableToolbar>

      <DataTableCard>
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead className="min-w-[200px]">标题</TableHead>
              <TableHead className="w-[100px]">格式</TableHead>
              <TableHead className="w-[100px]">状态</TableHead>
              <TableHead className="w-[100px]">大小</TableHead>
              <TableHead className="w-[80px]">引用数</TableHead>
              <TableHead className="w-[150px]">创建时间</TableHead>
              <DataTableActionsHead className="min-w-[120px]">操作</DataTableActionsHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {isLoading ? (
              <DataTableLoadingRow colSpan={colSpan} />
            ) : items.length === 0 ? (
              <DataTableEmptyRow
                colSpan={colSpan}
                icon={FileText}
                title="暂无素材"
                description="上传文件、添加网址或文本来创建素材"
              />
            ) : (
              items.map((item) => (
                <TableRow key={item.id}>
                  <TableCell className="font-medium">
                    <Link
                      to={`/ai/knowledge/sources/${item.id}`}
                      className="hover:underline text-primary"
                    >
                      {item.title}
                    </Link>
                  </TableCell>
                  <TableCell>
                    <FormatBadge format={item.format} />
                  </TableCell>
                  <TableCell>
                    <ExtractStatusBadge status={item.extractStatus} />
                  </TableCell>
                  <TableCell className="text-sm text-muted-foreground">
                    {formatBytes(item.byteSize)}
                  </TableCell>
                  <TableCell className="text-sm text-muted-foreground">
                    {item.refCount}
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
                        <Link to={`/ai/knowledge/sources/${item.id}`}>
                          <ExternalLink className="mr-1 h-3.5 w-3.5" />
                          查看
                        </Link>
                      </Button>
                      <AlertDialog>
                        <AlertDialogTrigger asChild>
                          <Button
                            variant="ghost"
                            size="sm"
                            className="px-2 text-destructive hover:text-destructive"
                          >
                            <Trash2 className="mr-1 h-3.5 w-3.5" />
                            删除
                          </Button>
                        </AlertDialogTrigger>
                        <AlertDialogContent>
                          <AlertDialogHeader>
                            <AlertDialogTitle>确认删除素材</AlertDialogTitle>
                            <AlertDialogDescription>
                              确定要删除「{item.title}」吗？{item.refCount > 0 && "该素材正在被知识库引用，删除可能失败。"}此操作不可撤销。
                            </AlertDialogDescription>
                          </AlertDialogHeader>
                          <AlertDialogFooter>
                            <AlertDialogCancel>取消</AlertDialogCancel>
                            <AlertDialogAction
                              onClick={() => deleteMutation.mutate(item.id)}
                              disabled={deleteMutation.isPending}
                            >
                              确认删除
                            </AlertDialogAction>
                          </AlertDialogFooter>
                        </AlertDialogContent>
                      </AlertDialog>
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

      <SourceUploadSheet
        open={uploadOpen}
        onOpenChange={setUploadOpen}
      />
      <SourceUrlSheet
        open={urlOpen}
        onOpenChange={setUrlOpen}
      />
      <SourceTextSheet
        open={textOpen}
        onOpenChange={setTextOpen}
      />
    </div>
  )
}
