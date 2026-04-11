import { useState } from "react"
import { useTranslation } from "react-i18next"
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query"
import { useNavigate } from "react-router"
import { Clock, Play, Pause, Zap, Activity, CheckCircle, XCircle, Timer } from "lucide-react"
import { taskApi, type TaskInfo } from "@/lib/api"
import { usePermission } from "@/hooks/use-permission"
import { Button } from "@/components/ui/button"
import { Badge } from "@/components/ui/badge"
import { Card, CardContent } from "@/components/ui/card"
import {
  DataTableActions,
  DataTableCard,
  DataTableEmptyRow,
  DataTableLoadingRow,
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
import { addKernelNamespace } from "@/i18n"
import zhCNTasks from "@/i18n/locales/zh-CN/tasks.json"
import enTasks from "@/i18n/locales/en/tasks.json"

addKernelNamespace("tasks", zhCNTasks, enTasks)

function formatRelativeTime(dateStr: string, t: (key: string, opts?: Record<string, unknown>) => string) {
  const date = new Date(dateStr)
  const now = new Date()
  const diff = now.getTime() - date.getTime()
  const minutes = Math.floor(diff / 60000)
  if (minutes < 1) return t("tasks:relativeTime.justNow")
  if (minutes < 60) return t("tasks:relativeTime.minutesAgo", { count: minutes })
  const hours = Math.floor(minutes / 60)
  if (hours < 24) return t("tasks:relativeTime.hoursAgo", { count: hours })
  const days = Math.floor(hours / 24)
  return t("tasks:relativeTime.daysAgo", { count: days })
}

function StatusBadge({ status, t }: { status: string; t: (key: string) => string }) {
  const map: Record<string, { key: string; variant: "default" | "secondary" | "destructive" | "outline" }> = {
    active: { key: "tasks:status.active", variant: "default" },
    paused: { key: "tasks:status.paused", variant: "secondary" },
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

export function Component() {
  const { t } = useTranslation(["tasks", "common"])
  const [tab, setTab] = useState<"all" | "scheduled" | "async">("all")
  const queryClient = useQueryClient()
  const navigate = useNavigate()
  const canPause = usePermission("system:task:pause")
  const canResume = usePermission("system:task:resume")
  const canTrigger = usePermission("system:task:trigger")

  const { data: stats } = useQuery({
    queryKey: ["task-stats"],
    queryFn: () => taskApi.stats(),
    refetchInterval: 10000,
  })

  const typeFilter = tab === "all" ? undefined : tab
  const { data: tasks, isLoading } = useQuery({
    queryKey: ["tasks", typeFilter],
    queryFn: () => taskApi.list(typeFilter),
    refetchInterval: 10000,
  })

  const pauseMutation = useMutation({
    mutationFn: (name: string) => taskApi.pause(name),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["tasks"] })
      queryClient.invalidateQueries({ queryKey: ["task-stats"] })
    },
  })

  const resumeMutation = useMutation({
    mutationFn: (name: string) => taskApi.resume(name),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["tasks"] })
      queryClient.invalidateQueries({ queryKey: ["task-stats"] })
    },
  })

  const triggerMutation = useMutation({
    mutationFn: (name: string) => taskApi.trigger(name),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["tasks"] })
      queryClient.invalidateQueries({ queryKey: ["task-stats"] })
    },
  })

  const statCards = [
    { label: t("tasks:stats.totalTasks"), value: stats?.totalTasks ?? 0, icon: Clock, color: "text-blue-500" },
    { label: t("tasks:stats.running"), value: stats?.running ?? 0, icon: Activity, color: "text-green-500" },
    { label: t("tasks:stats.completedToday"), value: stats?.completedToday ?? 0, icon: CheckCircle, color: "text-emerald-500" },
    { label: t("tasks:stats.failedToday"), value: stats?.failedToday ?? 0, icon: XCircle, color: "text-red-500" },
  ]

  return (
    <div className="space-y-4">
      <h2 className="text-lg font-semibold">{t("tasks:title")}</h2>

      {/* Stats Cards */}
      <div className="grid grid-cols-2 gap-4 sm:grid-cols-4">
        {statCards.map((card) => (
          <Card key={card.label}>
            <CardContent className="flex items-center gap-3 p-4">
              <card.icon className={`h-8 w-8 ${card.color}`} />
              <div>
                <p className="text-2xl font-bold">{card.value}</p>
                <p className="text-xs text-muted-foreground">{card.label}</p>
              </div>
            </CardContent>
          </Card>
        ))}
      </div>

      {/* Tab Buttons */}
      <div className="flex gap-1 border-b pb-2">
        {([
          { key: "all", label: t("tasks:tabs.all") },
          { key: "scheduled", label: t("tasks:tabs.scheduled") },
          { key: "async", label: t("tasks:tabs.async") },
        ] as const).map((item) => (
          <Button
            key={item.key}
            variant={tab === item.key ? "default" : "ghost"}
            size="sm"
            onClick={() => setTab(item.key)}
          >
            {item.label}
          </Button>
        ))}
      </div>

      {/* Task Table */}
      <DataTableCard>
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead className="w-[220px]">{t("common:name")}</TableHead>
              <TableHead className="min-w-[220px]">{t("common:description")}</TableHead>
              <TableHead className="w-[140px]">{t("common:type")}</TableHead>
              <TableHead className="w-[120px]">{t("common:status")}</TableHead>
              <TableHead className="min-w-[180px]">{t("tasks:table.lastExecution")}</TableHead>
              <TableHead className="w-[180px] text-center">{t("common:actions")}</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {isLoading ? (
              <DataTableLoadingRow colSpan={6} />
            ) : !tasks || tasks.length === 0 ? (
              <DataTableEmptyRow colSpan={6} icon={Timer} title={t("tasks:table.emptyTitle")} />
            ) : (
              tasks.map((task: TaskInfo) => (
                <TableRow
                  key={task.name}
                  className="cursor-pointer"
                  onClick={() => navigate(`/tasks/${task.name}`)}
                  >
                    <TableCell className="font-mono text-sm font-medium">{task.name}</TableCell>
                    <TableCell className="max-w-[360px] text-sm text-muted-foreground">
                      <span className="block truncate" title={task.description || "-"}>
                        {task.description || "-"}
                      </span>
                    </TableCell>
                    <TableCell>
                      <Badge variant="outline">
                        {task.type === "scheduled" ? task.cronExpr || t("tasks:taskType.scheduled") : t("tasks:taskType.async")}
                    </Badge>
                  </TableCell>
                  <TableCell>
                    <StatusBadge status={task.status} t={t} />
                  </TableCell>
                    <TableCell className="text-sm text-muted-foreground">
                      {task.lastExecution ? (
                        <span className="flex items-center gap-1.5">
                        <StatusBadge status={task.lastExecution.status} t={t} />
                        <span>{formatRelativeTime(task.lastExecution.timestamp, t)}</span>
                      </span>
                    ) : (
                      "-"
                    )}
                    </TableCell>
                    <TableCell className="text-center" onClick={(e) => e.stopPropagation()}>
                      <DataTableActions className="justify-center">
                        {task.type === "scheduled" && task.status === "active" && canPause && (
                          <Button
                            variant="ghost"
                            size="sm"
                            className="px-2.5"
                            onClick={() => pauseMutation.mutate(task.name)}
                            disabled={pauseMutation.isPending}
                          >
                          <Pause className="mr-1 h-3.5 w-3.5" />
                          {t("tasks:actions.pause")}
                        </Button>
                      )}
                      {task.type === "scheduled" && task.status === "paused" && canResume && (
                          <Button
                            variant="ghost"
                            size="sm"
                            className="px-2.5"
                            onClick={() => resumeMutation.mutate(task.name)}
                            disabled={resumeMutation.isPending}
                          >
                          <Play className="mr-1 h-3.5 w-3.5" />
                          {t("tasks:actions.resume")}
                        </Button>
                      )}
                        {canTrigger && (
                          <AlertDialog>
                            <AlertDialogTrigger asChild>
                              <Button variant="ghost" size="sm" className="px-2.5">
                                <Zap className="mr-1 h-3.5 w-3.5" />
                                {t("tasks:actions.trigger")}
                              </Button>
                          </AlertDialogTrigger>
                          <AlertDialogContent>
                            <AlertDialogHeader>
                              <AlertDialogTitle>{t("tasks:triggerDialog.title")}</AlertDialogTitle>
                              <AlertDialogDescription>
                                {t("tasks:triggerDialog.description", { name: task.name })}
                              </AlertDialogDescription>
                            </AlertDialogHeader>
                            <AlertDialogFooter>
                              <AlertDialogCancel>{t("common:cancel")}</AlertDialogCancel>
                              <AlertDialogAction onClick={() => triggerMutation.mutate(task.name)}>
                                {t("tasks:actions.execute")}
                              </AlertDialogAction>
                            </AlertDialogFooter>
                          </AlertDialogContent>
                          </AlertDialog>
                        )}
                      </DataTableActions>
                    </TableCell>
                  </TableRow>
                ))
              )}
          </TableBody>
        </Table>
      </DataTableCard>
    </div>
  )
}
