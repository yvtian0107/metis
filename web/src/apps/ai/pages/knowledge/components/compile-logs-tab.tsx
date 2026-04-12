import { useState } from "react"
import { useTranslation } from "react-i18next"
import { useQuery } from "@tanstack/react-query"
import { History, ChevronDown, ChevronRight, Sparkles } from "lucide-react"
import { api } from "@/lib/api"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table"
import { DataTableCard } from "@/components/ui/data-table"
import { formatDateTime } from "@/lib/utils"
import type { LogItem, CascadeDetail } from "../types"

function CascadeBadge({ count }: { count: number }) {
  if (count === 0) return <span className="text-sm text-muted-foreground">—</span>
  return (
    <span className="inline-flex items-center gap-1 text-sm text-primary">
      <Sparkles className="h-3 w-3" />
      {count}
    </span>
  )
}

function UpdateTypeBadge({ type }: { type: CascadeDetail["updateType"] }) {
  const variants: Record<string, string> = {
    content: "bg-blue-100 text-blue-700 dark:bg-blue-900 dark:text-blue-300",
    relationship: "bg-green-100 text-green-700 dark:bg-green-900 dark:text-green-300",
    contradiction: "bg-red-100 text-red-700 dark:bg-red-900 dark:text-red-300",
    merge: "bg-purple-100 text-purple-700 dark:bg-purple-900 dark:text-purple-300",
  }
  const labels: Record<string, string> = {
    content: "内容更新",
    relationship: "关系更新",
    contradiction: "矛盾标记",
    merge: "合并",
  }
  return (
    <span className={`text-xs px-1.5 py-0.5 rounded ${variants[type] || variants.content}`}>
      {labels[type] || type}
    </span>
  )
}

function CascadeDetailsRow({ log }: { log: LogItem }) {
  const { t } = useTranslation("ai")
  const [expanded, setExpanded] = useState(false)

  const cascade = log.cascadeDetails
  if (!cascade || cascade.cascadeUpdates.length === 0) return null

  return (
    <TableRow className="bg-muted/30">
      <TableCell colSpan={8} className="py-2">
        <div className="space-y-2">
          <Button
            variant="ghost"
            size="sm"
            className="h-6 px-2 text-xs"
            onClick={() => setExpanded(!expanded)}
          >
            {expanded ? <ChevronDown className="h-3 w-3 mr-1" /> : <ChevronRight className="h-3 w-3 mr-1" />}
            {expanded ? t("knowledge.logs.hideCascade") : t("knowledge.logs.showCascade", { count: cascade.cascadeUpdates.length })}
          </Button>

          {expanded && (
            <div className="pl-4 space-y-2">
              {/* Primary Nodes */}
              {cascade.primaryNodes.length > 0 && (
                <div>
                  <p className="text-xs text-muted-foreground mb-1">{t("knowledge.logs.primaryNodes")}:</p>
                  <div className="flex flex-wrap gap-1">
                    {cascade.primaryNodes.map((node) => (
                      <Badge key={node} variant="outline" className="text-xs">
                        {node}
                      </Badge>
                    ))}
                  </div>
                </div>
              )}

              {/* Cascade Updates */}
              <div>
                <p className="text-xs text-muted-foreground mb-1">{t("knowledge.logs.cascadeUpdates")}:</p>
                <div className="space-y-1">
                  {cascade.cascadeUpdates.map((update, idx) => (
                    <div key={idx} className="flex items-start gap-2 text-sm">
                      <UpdateTypeBadge type={update.updateType} />
                      <span className="font-medium">{update.nodeTitle}</span>
                      <span className="text-muted-foreground">—</span>
                      <span className="text-muted-foreground truncate">{update.reason}</span>
                    </div>
                  ))}
                </div>
              </div>
            </div>
          )}
        </div>
      </TableCell>
    </TableRow>
  )
}

export function CompileLogsTab({ kbId }: { kbId: number }) {
  const { t } = useTranslation(["ai", "common"])

  const { data, isLoading } = useQuery({
    queryKey: ["ai-kb-logs", kbId],
    queryFn: () => api.get<{ items: LogItem[]; total: number }>(
      `/api/v1/ai/knowledge-bases/${kbId}/logs?pageSize=50`,
    ),
  })
  const logs = data?.items ?? []

  return (
    <DataTableCard>
      <Table>
        <TableHeader>
          <TableRow>
            <TableHead className="w-[150px]">{t("common:createdAt")}</TableHead>
            <TableHead className="w-[100px]">{t("ai:knowledge.logs.action")}</TableHead>
            <TableHead className="w-[140px]">{t("ai:knowledge.logs.model")}</TableHead>
            <TableHead className="w-[80px] text-center">{t("ai:knowledge.logs.created")}</TableHead>
            <TableHead className="w-[80px] text-center">{t("ai:knowledge.logs.updated")}</TableHead>
            <TableHead className="w-[80px] text-center">{t("ai:knowledge.logs.edges")}</TableHead>
            <TableHead className="w-[80px] text-center">{t("ai:knowledge.logs.cascade")}</TableHead>
            <TableHead className="w-[80px] text-center">{t("ai:knowledge.logs.lint")}</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {isLoading ? (
            <TableRow>
              <TableCell colSpan={8} className="h-28 text-center text-sm text-muted-foreground">
                {t("common:loading")}
              </TableCell>
            </TableRow>
          ) : logs.length === 0 ? (
            <TableRow>
              <TableCell colSpan={8} className="h-44 text-center">
                <div className="flex flex-col items-center gap-2 text-muted-foreground">
                  <History className="h-10 w-10 stroke-1" />
                  <p className="text-sm font-medium">{t("ai:knowledge.logs.empty")}</p>
                </div>
              </TableCell>
            </TableRow>
          ) : (
            logs.map((log) => (
              <>
                <TableRow key={log.id} className={log.errorMessage ? "bg-destructive/5" : ""}>
                  <TableCell className="text-sm text-muted-foreground whitespace-nowrap">
                    {formatDateTime(log.createdAt)}
                  </TableCell>
                  <TableCell>
                    <Badge variant={log.action === "recompile" ? "secondary" : "outline"}>
                      {log.action}
                    </Badge>
                  </TableCell>
                  <TableCell className="text-sm text-muted-foreground truncate max-w-[140px]">
                    {log.modelId || "—"}
                  </TableCell>
                  <TableCell className="text-center text-sm">{log.nodesCreated}</TableCell>
                  <TableCell className="text-center text-sm">{log.nodesUpdated}</TableCell>
                  <TableCell className="text-center text-sm">{log.edgesCreated}</TableCell>
                  <TableCell className="text-center text-sm">
                    <CascadeBadge count={log.cascadeDetails?.cascadeUpdates?.length ?? 0} />
                  </TableCell>
                  <TableCell className="text-center text-sm">
                    {log.lintIssues > 0 ? (
                      <span className="text-amber-600 dark:text-amber-400">{log.lintIssues}</span>
                    ) : "0"}
                  </TableCell>
                </TableRow>
                <CascadeDetailsRow log={log} />
              </>
            ))
          )}
        </TableBody>
      </Table>
    </DataTableCard>
  )
}
