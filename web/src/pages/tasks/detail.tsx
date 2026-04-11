import { useState } from "react"
import { useTranslation } from "react-i18next"
import { useParams, useNavigate } from "react-router"
import { useQuery } from "@tanstack/react-query"
import { ArrowLeft, Clock, RotateCcw, Timer, Zap } from "lucide-react"
import { taskApi, type TaskExecution } from "@/lib/api"
import { Button } from "@/components/ui/button"
import { Badge } from "@/components/ui/badge"
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"
import {
  DataTableCard,
  DataTableEmptyRow,
  DataTableLoadingRow,
  DataTablePagination,
} from "@/components/ui/data-table"
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table"
import { formatDateTime } from "@/lib/utils"

function ExecStatusBadge({ status, t }: { status: string; t: (key: string) => string }) {
  const map: Record<string, { key: string; variant: "default" | "secondary" | "destructive" | "outline" }> = {
    completed: { key: "tasks:status.completed", variant: "default" },
    failed: { key: "tasks:status.failed", variant: "destructive" },
    timeout: { key: "tasks:status.timeout", variant: "destructive" },
    pending: { key: "tasks:status.pending", variant: "outline" },
    running: { key: "tasks:status.running", variant: "default" },
    stale: { key: "tasks:status.stale", variant: "secondary" },
  }
  const info = map[status]
  if (!info) return <Badge variant="outline">{status}</Badge>
  return <Badge variant={info.variant}>{t(info.key)}</Badge>
}

function TriggerBadge({ trigger, t }: { trigger: string; t: (key: string) => string }) {
  const map: Record<string, { key: string; icon: typeof Clock }> = {
    cron: { key: "tasks:trigger.cron", icon: Clock },
    manual: { key: "tasks:trigger.manual", icon: Zap },
    api: { key: "tasks:trigger.api", icon: RotateCcw },
  }
  const info = map[trigger]
  if (!info) {
    return (
      <Badge variant="outline" className="gap-1">
        <Clock className="h-3 w-3" />
        {trigger}
      </Badge>
    )
  }
  return (
    <Badge variant="outline" className="gap-1">
      <info.icon className="h-3 w-3" />
      {t(info.key)}
    </Badge>
  )
}

function formatDuration(exec: TaskExecution): string {
  if (!exec.startedAt || !exec.finishedAt) return "-"
  const ms = new Date(exec.finishedAt).getTime() - new Date(exec.startedAt).getTime()
  if (ms < 1000) return `${ms}ms`
  return `${(ms / 1000).toFixed(1)}s`
}

export function Component() {
  const { t } = useTranslation(["tasks", "common"])
  const { name } = useParams<{ name: string }>()
  const navigate = useNavigate()
  const [page, setPage] = useState(1)
  const pageSize = 20

  const { data: detail } = useQuery({
    queryKey: ["task-detail", name],
    queryFn: () => taskApi.get(name!),
    enabled: !!name,
  })

  const { data: execData, isLoading: execLoading } = useQuery({
    queryKey: ["task-executions", name, page],
    queryFn: () => taskApi.executions(name!, page, pageSize),
    enabled: !!name,
    refetchInterval: 10000,
  })

  const task = detail?.task
  const executions = execData?.list ?? []
  const total = execData?.total ?? 0
  const totalPages = Math.ceil(total / pageSize)

  if (!task) {
    return (
      <div className="flex h-64 items-center justify-center text-muted-foreground">
        {t("tasks:detail.loading")}
      </div>
    )
  }

  return (
    <div className="space-y-4">
      <div className="flex items-center gap-2">
        <Button variant="ghost" size="sm" onClick={() => navigate("/tasks")}>
          <ArrowLeft className="mr-1 h-4 w-4" />
          {t("common:back")}
        </Button>
        <h2 className="text-lg font-semibold">{task.name}</h2>
      </div>

      {/* Task Config Card */}
      <Card>
        <CardHeader>
          <CardTitle className="text-base">{t("tasks:detail.taskConfig")}</CardTitle>
        </CardHeader>
        <CardContent>
          <div className="grid grid-cols-2 gap-4 text-sm sm:grid-cols-3">
            <div>
              <p className="text-muted-foreground">{t("common:name")}</p>
              <p className="font-mono font-medium">{task.name}</p>
            </div>
            <div>
              <p className="text-muted-foreground">{t("common:type")}</p>
              <Badge variant="outline">{task.type === "scheduled" ? t("tasks:detail.scheduledTask") : t("tasks:detail.asyncTask")}</Badge>
            </div>
            <div>
              <p className="text-muted-foreground">{t("common:status")}</p>
              <Badge variant={task.status === "active" ? "default" : "secondary"}>
                {task.status === "active" ? t("tasks:status.active") : t("tasks:status.paused")}
              </Badge>
            </div>
            <div>
              <p className="text-muted-foreground">{t("common:description")}</p>
              <p>{task.description || "-"}</p>
            </div>
            {task.cronExpr && (
              <div>
                <p className="text-muted-foreground">{t("tasks:detail.cronExpression")}</p>
                <p className="font-mono">{task.cronExpr}</p>
              </div>
            )}
            <div>
              <p className="text-muted-foreground">{t("tasks:detail.timeout")}</p>
              <p>{task.timeoutMs >= 1000 ? `${task.timeoutMs / 1000}s` : `${task.timeoutMs}ms`}</p>
            </div>
            <div>
              <p className="text-muted-foreground">{t("tasks:detail.maxRetries")}</p>
              <p>{task.maxRetries} {t("tasks:detail.retriesSuffix")}</p>
            </div>
          </div>
        </CardContent>
      </Card>

      {/* Execution History */}
      <div>
        <h3 className="mb-2 text-base font-semibold">{t("tasks:detail.executionHistory")}</h3>
        <DataTableCard>
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead className="w-[90px]">{t("tasks:execution.id")}</TableHead>
                <TableHead className="w-[120px]">{t("tasks:execution.triggerType")}</TableHead>
                <TableHead className="w-[120px]">{t("common:status")}</TableHead>
                <TableHead className="w-[100px]">{t("tasks:execution.duration")}</TableHead>
                <TableHead className="min-w-[240px]">{t("tasks:execution.error")}</TableHead>
                <TableHead className="w-[150px]">{t("tasks:execution.time")}</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {execLoading ? (
                <DataTableLoadingRow colSpan={6} />
              ) : executions.length === 0 ? (
                <DataTableEmptyRow colSpan={6} icon={Timer} title={t("tasks:detail.emptyExecutions")} />
              ) : (
                executions.map((exec: TaskExecution) => (
                  <TableRow key={exec.id}>
                    <TableCell className="font-mono text-sm">{exec.id}</TableCell>
                    <TableCell>
                      <TriggerBadge trigger={exec.trigger} t={t} />
                    </TableCell>
                    <TableCell>
                      <ExecStatusBadge status={exec.status} t={t} />
                    </TableCell>
                    <TableCell className="font-mono text-sm">{formatDuration(exec)}</TableCell>
                    <TableCell className="max-w-[200px] truncate text-sm text-muted-foreground" title={exec.error}>
                      {exec.error || "-"}
                    </TableCell>
                    <TableCell className="text-sm text-muted-foreground">
                      {formatDateTime(exec.createdAt)}
                    </TableCell>
                  </TableRow>
                ))
              )}
            </TableBody>
          </Table>
        </DataTableCard>

        <DataTablePagination
          className="mt-4"
          total={total}
          page={page}
          totalPages={totalPages}
          onPageChange={setPage}
        />
      </div>
    </div>
  )
}
