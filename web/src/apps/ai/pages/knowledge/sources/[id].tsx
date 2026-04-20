import { useParams, Link } from "react-router"
import { useQuery } from "@tanstack/react-query"
import { ArrowLeft, Globe, FileText, Clock, Link2 } from "lucide-react"
import { api } from "@/lib/api"
import { Button } from "@/components/ui/button"
import { Badge } from "@/components/ui/badge"
import {
  DataTableCard,
} from "@/components/ui/data-table"
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table"
import { formatDateTime, formatBytes } from "@/lib/utils"
import type { SourceItem } from "./index"

interface SourceContent {
  content: string
}

interface ReferenceAsset {
  id: number
  name: string
  type: string
}

function StatusBadge({ status }: { status: string }) {
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
  const { id } = useParams<{ id: string }>()
  const sourceId = Number(id)

  const { data: source, isLoading } = useQuery({
    queryKey: ["ai-knowledge-source", sourceId],
    queryFn: () => api.get<SourceItem>(`/api/v1/ai/knowledge/sources/${sourceId}`),
    enabled: sourceId > 0,
  })

  const { data: contentData } = useQuery({
    queryKey: ["ai-knowledge-source-content", sourceId],
    queryFn: () => api.get<SourceContent>(`/api/v1/ai/knowledge/sources/${sourceId}/content`),
    enabled: sourceId > 0,
  })

  const { data: references } = useQuery({
    queryKey: ["ai-knowledge-source-refs", sourceId],
    queryFn: () => api.get<ReferenceAsset[]>(`/api/v1/ai/knowledge/sources/${sourceId}/references`),
    enabled: sourceId > 0,
  })

  if (isLoading) {
    return (
      <div className="flex items-center justify-center h-48 text-muted-foreground text-sm">
        加载中...
      </div>
    )
  }

  if (!source) {
    return (
      <div className="flex items-center justify-center h-48 text-muted-foreground text-sm">
        素材不存在
      </div>
    )
  }

  const contentPreview = contentData?.content
    ? contentData.content.length > 5000
      ? contentData.content.slice(0, 5000) + "..."
      : contentData.content
    : null

  return (
    <div className="space-y-6">
      <div className="flex items-center gap-3">
        <Button variant="ghost" size="sm" asChild>
          <Link to="/ai/knowledge/sources">
            <ArrowLeft className="mr-1 h-4 w-4" />
            返回
          </Link>
        </Button>
        <h2 className="text-lg font-semibold">{source.title}</h2>
      </div>

      {/* Info card */}
      <DataTableCard>
        <div className="grid grid-cols-2 gap-4 p-4 sm:grid-cols-3">
          <div className="space-y-1">
            <p className="text-xs text-muted-foreground">格式</p>
            <Badge variant="outline">{source.format?.toUpperCase() ?? "—"}</Badge>
          </div>
          <div className="space-y-1">
            <p className="text-xs text-muted-foreground">状态</p>
            <StatusBadge status={source.extractStatus} />
          </div>
          <div className="space-y-1">
            <p className="text-xs text-muted-foreground">大小</p>
            <p className="text-sm">{formatBytes(source.byteSize)}</p>
          </div>
          {source.sourceUrl && (
            <div className="space-y-1 col-span-2 sm:col-span-3">
              <p className="text-xs text-muted-foreground flex items-center gap-1">
                <Globe className="h-3 w-3" /> 来源网址
              </p>
              <a
                href={source.sourceUrl}
                target="_blank"
                rel="noopener noreferrer"
                className="text-sm text-primary hover:underline break-all"
              >
                {source.sourceUrl}
              </a>
            </div>
          )}
          {source.errorMessage && (
            <div className="space-y-1 col-span-2 sm:col-span-3">
              <p className="text-xs text-destructive">错误信息</p>
              <p className="text-sm text-destructive">{source.errorMessage}</p>
            </div>
          )}
          <div className="space-y-1">
            <p className="text-xs text-muted-foreground flex items-center gap-1">
              <Clock className="h-3 w-3" /> 创建时间
            </p>
            <p className="text-sm">{formatDateTime(source.createdAt)}</p>
          </div>
          <div className="space-y-1">
            <p className="text-xs text-muted-foreground flex items-center gap-1">
              <Clock className="h-3 w-3" /> 更新时间
            </p>
            <p className="text-sm">{formatDateTime(source.updatedAt)}</p>
          </div>
          <div className="space-y-1">
            <p className="text-xs text-muted-foreground flex items-center gap-1">
              <Link2 className="h-3 w-3" /> 引用数
            </p>
            <p className="text-sm">{source.refCount}</p>
          </div>
        </div>
      </DataTableCard>

      {/* Content preview */}
      {contentPreview && (
        <div className="space-y-2">
          <h3 className="text-sm font-medium flex items-center gap-1.5">
            <FileText className="h-4 w-4" />
            内容预览
          </h3>
          <div className="rounded-lg border bg-muted/30 p-4">
            <pre className="whitespace-pre-wrap text-sm leading-relaxed max-h-[400px] overflow-y-auto">
              {contentPreview}
            </pre>
          </div>
        </div>
      )}

      {/* References */}
      {references && references.length > 0 && (
        <div className="space-y-2">
          <h3 className="text-sm font-medium flex items-center gap-1.5">
            <Link2 className="h-4 w-4" />
            引用此素材的知识库
          </h3>
          <DataTableCard>
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>名称</TableHead>
                  <TableHead className="w-[100px]">类型</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {references.map((ref) => (
                  <TableRow key={ref.id}>
                    <TableCell className="font-medium">{ref.name}</TableCell>
                    <TableCell>
                      <Badge variant="outline">{ref.type}</Badge>
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          </DataTableCard>
        </div>
      )}
    </div>
  )
}
