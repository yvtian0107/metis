import { useState } from "react"
import { useTranslation } from "react-i18next"
import { useMutation, useQueryClient } from "@tanstack/react-query"
import { Plus, Globe, FileText, Loader2 } from "lucide-react"
import { api } from "@/lib/api"
import { toast } from "sonner"
import { Button } from "@/components/ui/button"
import { Badge } from "@/components/ui/badge"
import { Progress } from "@/components/ui/progress"
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table"
import { DataTableCard } from "@/components/ui/data-table"
import { formatDateTime, formatBytes } from "@/lib/utils"
import { ExtractStatusBadge } from "./status-badges"
import { SourceUpload } from "./source-upload"
import { UrlAddForm } from "./url-add-form"
import { useKbSources } from "../hooks/use-kb-sources"
import type { CompileProgress } from "../types"

interface SourcesTabProps {
  kbId: number
  canCreate: boolean
  progress?: CompileProgress
}

export function SourcesTab({ kbId, canCreate, progress }: SourcesTabProps) {
  const { t } = useTranslation(["ai", "common"])
  const queryClient = useQueryClient()
  const [uploadOpen, setUploadOpen] = useState(false)
  const [urlFormOpen, setUrlFormOpen] = useState(false)

  const { data, isLoading } = useKbSources(kbId)
  const sources = data?.items ?? []

  const deleteMutation = useMutation({
    mutationFn: (sourceId: number) =>
      api.delete(`/api/v1/ai/knowledge-bases/${kbId}/sources/${sourceId}`),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["ai-kb-sources", kbId] })
      queryClient.invalidateQueries({ queryKey: ["ai-kb-detail", kbId] })
      toast.success(t("ai:knowledge.sources.deleteSuccess"))
    },
    onError: (err) => toast.error(err.message),
  })

  // Calculate overall progress
  const isCompiling = progress && progress.stage !== "idle" && progress.stage !== "completed"
  const totalItems = (progress?.sources.total ?? 0) + (progress?.nodes.total ?? 0) + (progress?.embeddings.total ?? 0)
  const doneItems = (progress?.sources.done ?? 0) + (progress?.nodes.done ?? 0) + (progress?.embeddings.done ?? 0)
  const percent = totalItems > 0 ? Math.round((doneItems / totalItems) * 100) : 0

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between gap-4">
        {canCreate && (
          <div className="flex items-center gap-2">
            <Button size="sm" variant="outline" onClick={() => setUploadOpen(true)}>
              <Plus className="mr-1.5 h-4 w-4" />
              {t("ai:knowledge.sources.uploadFile")}
            </Button>
            <Button size="sm" variant="outline" onClick={() => setUrlFormOpen(true)}>
              <Globe className="mr-1.5 h-4 w-4" />
              {t("ai:knowledge.sources.addUrl")}
            </Button>
          </div>
        )}

        {isCompiling && (
          <div className="flex items-center gap-3 flex-1 max-w-xs">
            <div className="flex-1 min-w-0">
              <div className="flex items-center justify-between gap-2 text-xs text-muted-foreground mb-1">
                <span className="flex items-center gap-1.5">
                  <Loader2 className="h-3 w-3 animate-spin" />
                  {t(`ai:knowledge.compileStage.${progress.stage}`)}
                </span>
                <span>{doneItems}/{totalItems}</span>
              </div>
              <Progress value={percent} className="h-1" />
            </div>
          </div>
        )}
      </div>

      <DataTableCard>
        {!isLoading && data && data.total > sources.length && (
          <div className="px-4 pt-3 text-xs text-muted-foreground">
            {t("ai:knowledge.sources.showingFirst", { count: sources.length, total: data.total })}
          </div>
        )}
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead className="min-w-[200px]">{t("ai:knowledge.sources.title")}</TableHead>
              <TableHead className="w-[100px]">{t("ai:knowledge.sources.format")}</TableHead>
              <TableHead className="w-[120px]">{t("ai:knowledge.sources.extractStatus")}</TableHead>
              <TableHead className="w-[100px]">{t("ai:knowledge.sources.size")}</TableHead>
              <TableHead className="w-[150px]">{t("common:createdAt")}</TableHead>
              <TableHead className="w-[80px] text-right">{t("common:actions")}</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {isLoading ? (
              <TableRow>
                <TableCell colSpan={6} className="h-28 text-center text-sm text-muted-foreground">
                  {t("common:loading")}
                </TableCell>
              </TableRow>
            ) : sources.length === 0 ? (
              <TableRow>
                <TableCell colSpan={6} className="h-44 text-center">
                  <div className="flex flex-col items-center gap-2 text-muted-foreground">
                    <FileText className="h-10 w-10 stroke-1" />
                    <p className="text-sm font-medium">{t("ai:knowledge.sources.empty")}</p>
                  </div>
                </TableCell>
              </TableRow>
            ) : (
              sources.map((src) => (
                <TableRow key={src.id}>
                  <TableCell className="font-medium">{src.title}</TableCell>
                  <TableCell>
                    <Badge variant="outline">{src.format || src.sourceType}</Badge>
                  </TableCell>
                  <TableCell>
                    <ExtractStatusBadge status={src.extractStatus} />
                  </TableCell>
                  <TableCell className="text-sm text-muted-foreground">
                    {formatBytes(src.byteSize)}
                  </TableCell>
                  <TableCell className="text-sm text-muted-foreground whitespace-nowrap">
                    {formatDateTime(src.createdAt)}
                  </TableCell>
                  <TableCell className="text-right">
                    <Button
                      variant="ghost"
                      size="sm"
                      className="px-2 text-destructive hover:text-destructive"
                      disabled={deleteMutation.isPending}
                      onClick={() => deleteMutation.mutate(src.id)}
                    >
                      {t("common:delete")}
                    </Button>
                  </TableCell>
                </TableRow>
              ))
            )}
          </TableBody>
        </Table>
      </DataTableCard>

      <SourceUpload
        open={uploadOpen}
        onOpenChange={setUploadOpen}
        kbId={kbId}
        onSuccess={() => {
          queryClient.invalidateQueries({ queryKey: ["ai-kb-sources", kbId] })
          queryClient.invalidateQueries({ queryKey: ["ai-kb-detail", kbId] })
        }}
      />
      <UrlAddForm
        open={urlFormOpen}
        onOpenChange={setUrlFormOpen}
        kbId={kbId}
        onSuccess={() => {
          queryClient.invalidateQueries({ queryKey: ["ai-kb-sources", kbId] })
          queryClient.invalidateQueries({ queryKey: ["ai-kb-detail", kbId] })
        }}
      />
    </div>
  )
}
