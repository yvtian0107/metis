import { useState } from "react"
import { useTranslation } from "react-i18next"
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query"
import { Wrench, BookOpen, Globe, Code, CheckCircle2, CircleOff, Clock3, ShieldAlert } from "lucide-react"
import { usePermission } from "@/hooks/use-permission"
import { api } from "@/lib/api"
import { toast } from "sonner"
import { Switch } from "@/components/ui/switch"
import { Badge } from "@/components/ui/badge"
import {
  AlertDialog, AlertDialogAction, AlertDialogCancel, AlertDialogContent,
  AlertDialogDescription, AlertDialogFooter, AlertDialogHeader, AlertDialogTitle,
} from "@/components/ui/alert-dialog"
import {
  Sheet, SheetContent, SheetHeader, SheetTitle, SheetDescription,
} from "@/components/ui/sheet"
import { cn } from "@/lib/utils"

type AvailabilityStatus = "available" | "inactive" | "unimplemented" | "risk_disabled"

interface ToolItem {
  id: number
  toolkit: string
  name: string
  displayName: string
  description: string
  parametersSchema: Record<string, unknown>
  isActive: boolean
  isExecutable: boolean
  availabilityStatus: AvailabilityStatus
  availabilityReason?: string
  boundAgentCount: number
}

interface ToolkitGroup {
  toolkit: string
  tools: ToolItem[]
}

const TOOLKIT_ICONS: Record<string, React.ElementType> = {
  knowledge: BookOpen,
  network: Globe,
  code: Code,
}

const STATUS_ICONS: Record<AvailabilityStatus, React.ElementType> = {
  available: CheckCircle2,
  inactive: CircleOff,
  unimplemented: Clock3,
  risk_disabled: ShieldAlert,
}

function canToggleTool(tool: ToolItem) {
  return tool.availabilityStatus === "available" || tool.availabilityStatus === "inactive"
}

export function BuiltinToolsTab() {
  const { t } = useTranslation(["ai", "common"])
  const queryClient = useQueryClient()
  const canUpdate = usePermission("ai:tool:update")
  const [openToolkit, setOpenToolkit] = useState<ToolkitGroup | null>(null)
  const [confirmTool, setConfirmTool] = useState<ToolItem | null>(null)

  const { data, isLoading } = useQuery({
    queryKey: ["ai-tools"],
    queryFn: () => api.get<{ items: ToolkitGroup[] }>("/api/v1/ai/tools"),
  })
  const groups = data?.items ?? []

  const toggleMutation = useMutation({
    mutationFn: ({ id, isActive, confirmImpact }: { id: number; isActive: boolean; confirmImpact?: boolean }) =>
      api.put(`/api/v1/ai/tools/${id}`, { isActive, confirmImpact }),
    onSuccess: (_data, variables) => {
      queryClient.invalidateQueries({ queryKey: ["ai-tools"] })
      setConfirmTool(null)
      toast.success(
        variables.isActive
          ? t("ai:tools.builtin.enableSuccess")
          : t("ai:tools.builtin.disableSuccess"),
      )
    },
    onError: (err) => toast.error(err.message),
  })

  if (isLoading) {
    return (
      <div className="flex items-center justify-center py-12 text-sm text-muted-foreground">
        {t("common:loading")}
      </div>
    )
  }

  if (groups.length === 0) {
    return (
      <div className="flex flex-col items-center gap-2 py-12 text-center">
        <Wrench className="h-10 w-10 text-muted-foreground/50" />
        <p className="text-sm text-muted-foreground">{t("ai:tools.builtin.empty")}</p>
      </div>
    )
  }

  // Keep the drawer content in sync with latest query data
  const activeDrawerGroup = openToolkit
    ? groups.find((g) => g.toolkit === openToolkit.toolkit) ?? openToolkit
    : null

  return (
    <>
      <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
        {groups.map((group) => {
          const Icon = TOOLKIT_ICONS[group.toolkit] ?? Wrench
          const activeCount = group.tools.filter((tool) => tool.isActive).length
          const executableCount = group.tools.filter((tool) => tool.isExecutable).length
          const totalCount = group.tools.length

          return (
            <button
              key={group.toolkit}
              type="button"
              className="flex flex-col gap-3 rounded-lg border bg-card p-4 text-left transition-colors hover:border-primary/50 hover:bg-accent/30 cursor-pointer"
              onClick={() => setOpenToolkit(group)}
            >
              <div className="flex items-center gap-3">
                <div className="flex h-9 w-9 items-center justify-center rounded-md bg-primary/10">
                  <Icon className="h-5 w-5 text-primary" />
                </div>
                <div className="flex-1 min-w-0">
                  <h3 className="text-sm font-semibold">
                    {t(`ai:tools.toolkits.${group.toolkit}.name`)}
                  </h3>
                  <p className="text-xs text-muted-foreground">
                    {executableCount}/{totalCount} {t("ai:tools.builtin.toolsAvailable")}
                  </p>
                </div>
                <Badge variant={executableCount > 0 ? "default" : "secondary"} className="shrink-0">
                  {activeCount}/{totalCount} {t("ai:tools.builtin.toolsEnabled")}
                </Badge>
              </div>
              <p className="text-sm text-muted-foreground line-clamp-2">
                {t(`ai:tools.toolkits.${group.toolkit}.description`)}
              </p>
            </button>
          )
        })}
      </div>

      {/* Toolkit detail drawer */}
      <Sheet open={activeDrawerGroup !== null} onOpenChange={(v) => { if (!v) setOpenToolkit(null) }}>
        <SheetContent className="sm:max-w-lg overflow-y-auto">
          {activeDrawerGroup && (
            <ToolkitDetail
              group={activeDrawerGroup}
              canUpdate={canUpdate}
              toggleMutation={toggleMutation}
              onRequestDisable={(tool) => setConfirmTool(tool)}
            />
          )}
        </SheetContent>
      </Sheet>
      <AlertDialog open={confirmTool !== null} onOpenChange={(open) => { if (!open) setConfirmTool(null) }}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>{t("ai:tools.builtin.disableConfirmTitle")}</AlertDialogTitle>
            <AlertDialogDescription>
              {t("ai:tools.builtin.disableConfirmDesc", {
                name: confirmTool ? t(`ai:tools.toolDefs.${confirmTool.name}.name`, confirmTool.displayName) : "",
                count: confirmTool?.boundAgentCount ?? 0,
              })}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel disabled={toggleMutation.isPending}>{t("common:cancel")}</AlertDialogCancel>
            <AlertDialogAction
              disabled={!confirmTool || toggleMutation.isPending}
              onClick={() => {
                if (!confirmTool) return
                toggleMutation.mutate({ id: confirmTool.id, isActive: false, confirmImpact: true })
              }}
            >
              {t("common:confirm")}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </>
  )
}

function ToolkitDetail({
  group,
  canUpdate,
  toggleMutation,
  onRequestDisable,
}: {
  group: ToolkitGroup
  canUpdate: boolean
  toggleMutation: ReturnType<typeof useMutation<unknown, Error, { id: number; isActive: boolean; confirmImpact?: boolean }>>
  onRequestDisable: (tool: ToolItem) => void
}) {
  const { t } = useTranslation(["ai", "common"])
  const Icon = TOOLKIT_ICONS[group.toolkit] ?? Wrench

  return (
    <>
      <SheetHeader>
        <SheetTitle className="flex items-center gap-2">
          <Icon className="h-5 w-5 text-primary" />
          {t(`ai:tools.toolkits.${group.toolkit}.name`)}
        </SheetTitle>
        <SheetDescription>
          {t(`ai:tools.toolkits.${group.toolkit}.description`)}
        </SheetDescription>
      </SheetHeader>
      <div className="flex flex-col gap-3 px-4">
        <p className="text-xs text-muted-foreground">
          {t("ai:tools.builtin.drawerHint")}
        </p>
        {group.tools.map((tool) => (
          <div
            key={tool.id}
            className="rounded-lg border p-3 transition-colors"
          >
            <div className="flex items-start gap-3">
              <ToolStatusBadge tool={tool} />
              <div className="min-w-0 flex-1">
                <div className="flex flex-wrap items-center gap-2">
                  <p className="text-sm font-medium">{t(`ai:tools.toolDefs.${tool.name}.name`, tool.displayName)}</p>
                  {tool.boundAgentCount > 0 && (
                    <Badge variant="outline" className="text-[11px]">
                      {t("ai:tools.builtin.boundAgentCount", { count: tool.boundAgentCount })}
                    </Badge>
                  )}
                </div>
                <p className="mt-1 text-xs leading-5 text-muted-foreground">
                  {t(`ai:tools.toolDefs.${tool.name}.description`, tool.description)}
                </p>
                {tool.availabilityReason && (
                  <p className="mt-2 rounded-md border border-border/55 bg-muted/35 px-2 py-1.5 text-xs leading-5 text-muted-foreground">
                    {tool.availabilityReason}
                  </p>
                )}
                <details className="mt-2 text-xs text-muted-foreground">
                  <summary className="cursor-pointer select-none">{t("ai:tools.builtin.parameters")}</summary>
                  <pre className="mt-2 max-h-48 overflow-auto rounded-md bg-muted/45 p-2 text-[11px] leading-5">
                    {JSON.stringify(tool.parametersSchema ?? {}, null, 2)}
                  </pre>
                </details>
              </div>
              <Switch
                checked={tool.isActive}
                disabled={!canUpdate || toggleMutation.isPending || !canToggleTool(tool)}
                onCheckedChange={(checked) => {
                  if (!checked && tool.boundAgentCount > 0) {
                    onRequestDisable(tool)
                    return
                  }
                  toggleMutation.mutate({ id: tool.id, isActive: checked })
                }}
              />
            </div>
          </div>
        ))}
      </div>
    </>
  )
}

function ToolStatusBadge({ tool }: { tool: ToolItem }) {
  const { t } = useTranslation("ai")
  const Icon = STATUS_ICONS[tool.availabilityStatus] ?? CircleOff
  return (
    <Badge
      variant={tool.availabilityStatus === "available" ? "default" : "outline"}
      className={cn(
        "mt-0.5 shrink-0 gap-1.5",
        tool.availabilityStatus === "risk_disabled" && "border-amber-500/45 text-amber-700 dark:text-amber-300",
        tool.availabilityStatus === "unimplemented" && "border-muted-foreground/35 text-muted-foreground"
      )}
    >
      <Icon className="size-3" />
      {t(`tools.builtin.availability.${tool.availabilityStatus}`)}
    </Badge>
  )
}
