import { useState } from "react"
import { useTranslation } from "react-i18next"
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query"
import { Wrench, BookOpen, Globe, Code, Save } from "lucide-react"
import { usePermission } from "@/hooks/use-permission"
import { api } from "@/lib/api"
import { toast } from "sonner"
import { Switch } from "@/components/ui/switch"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select"
import { Slider } from "@/components/ui/slider"
import {
  AlertDialog, AlertDialogAction, AlertDialogCancel, AlertDialogContent,
  AlertDialogDescription, AlertDialogFooter, AlertDialogHeader, AlertDialogTitle,
} from "@/components/ui/alert-dialog"
import {
  Sheet, SheetContent, SheetHeader, SheetTitle, SheetDescription,
} from "@/components/ui/sheet"
import { WorkspaceStatus, type WorkspaceStatusTone } from "@/components/workspace/primitives"

type AvailabilityStatus = "available" | "inactive" | "needs_config" | "unimplemented" | "risk_disabled"

interface ToolRuntimeConfig {
  modelId?: number
  temperature?: number
  maxTokens?: number
  timeoutSeconds?: number
}

interface ToolItem {
  id: number
  toolkit: string
  name: string
  displayName: string
  description: string
  parametersSchema: Record<string, unknown>
  runtimeConfigSchema?: Record<string, unknown>
  runtimeConfig?: ToolRuntimeConfig
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

const STATUS_TONES: Record<AvailabilityStatus, WorkspaceStatusTone> = {
  available: "success",
  inactive: "neutral",
  needs_config: "warning",
  unimplemented: "neutral",
  risk_disabled: "danger",
}

function canToggleTool(tool: ToolItem) {
  return tool.availabilityStatus === "available" || tool.availabilityStatus === "inactive" || tool.availabilityStatus === "needs_config"
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
      <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-3">
        {groups.map((group) => {
          const Icon = TOOLKIT_ICONS[group.toolkit] ?? Wrench
          const activeCount = group.tools.filter((tool) => tool.isActive).length
          const executableCount = group.tools.filter((tool) => tool.isExecutable).length
          const totalCount = group.tools.length

          return (
            <button
              key={group.toolkit}
              type="button"
              className="workspace-surface flex min-h-[128px] flex-col gap-3 rounded-lg p-4 text-left transition-colors hover:border-primary/35 hover:bg-accent/20 cursor-pointer"
              onClick={() => setOpenToolkit(group)}
            >
              <div className="flex items-center gap-3">
                <div className="flex h-9 w-9 items-center justify-center rounded-md border bg-muted/45">
                  <Icon className="h-4.5 w-4.5 text-foreground" />
                </div>
                <div className="flex-1 min-w-0">
                  <h3 className="text-sm font-semibold">
                    {t(`ai:tools.toolkits.${group.toolkit}.name`)}
                  </h3>
                  <p className="text-xs text-muted-foreground">
                    {executableCount}/{totalCount} {t("ai:tools.builtin.toolsAvailable")}
                  </p>
                </div>
                <Badge variant="outline" className="shrink-0 font-normal">
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
        <SheetContent className="overflow-y-auto sm:max-w-3xl">
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
                {tool.runtimeConfigSchema && (
                  <ToolRuntimeConfigForm tool={tool} canUpdate={canUpdate} />
                )}
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
  return <WorkspaceStatus tone={STATUS_TONES[tool.availabilityStatus]} label={t(`tools.builtin.availability.${tool.availabilityStatus}`)} className="mt-0.5 shrink-0" />
}

interface ProviderItem {
  id: number
  name: string
  type: string
  status: string
}

interface ModelItem {
  id: number
  displayName: string
  providerId: number
  status: string
}

function ToolRuntimeConfigForm({ tool, canUpdate }: { tool: ToolItem; canUpdate: boolean }) {
  const { t } = useTranslation(["ai", "common"])
  const queryClient = useQueryClient()
  const current = tool.runtimeConfig ?? {}
  const [providerId, setProviderId] = useState(0)
  const [modelId, setModelId] = useState(current.modelId ?? 0)
  const [temperature, setTemperature] = useState(current.temperature ?? 0.2)
  const [maxTokens, setMaxTokens] = useState(current.maxTokens ?? 1024)
  const [timeoutSeconds, setTimeoutSeconds] = useState(current.timeoutSeconds ?? 30)

  const { data: providers = [] } = useQuery({
    queryKey: ["ai-providers-for-tool-runtime"],
    queryFn: () => api.get<{ items: ProviderItem[] }>("/api/v1/ai/providers?pageSize=100").then((r) => r.items ?? []),
  })

  const { data: models = [] } = useQuery({
    queryKey: ["ai-llm-models-for-tool-runtime"],
    queryFn: () => api.get<{ items: ModelItem[] }>("/api/v1/ai/models?type=llm&pageSize=100").then((r) => r.items ?? []),
  })

  const selectedModel = models.find((m) => m.id === modelId)
  const effectiveProviderId = providerId || selectedModel?.providerId || 0
  const providerModels = models.filter((m) => m.providerId === effectiveProviderId)

  const runtimeMutation = useMutation({
    mutationFn: () => api.patch(`/api/v1/ai/tools/${tool.id}/runtime`, {
      runtimeConfig: { modelId, temperature, maxTokens, timeoutSeconds },
    }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["ai-tools"] })
      toast.success(t("ai:tools.builtin.runtimeSaveSuccess"))
    },
    onError: (err) => toast.error(err.message),
  })

  return (
    <div className="mt-4 border-t border-border/45 pt-4">
      <div className="mb-3 flex flex-wrap items-center justify-between gap-3">
        <div>
          <p className="text-sm font-medium">{t("ai:tools.builtin.runtimeTitle")}</p>
          <p className="mt-1 text-xs leading-5 text-muted-foreground">{t("ai:tools.builtin.runtimeDesc")}</p>
        </div>
        <Button
          size="sm"
          className="shrink-0"
          disabled={!canUpdate || runtimeMutation.isPending || modelId === 0}
          onClick={() => runtimeMutation.mutate()}
        >
          <Save className="mr-1.5 h-3.5 w-3.5" />
          {runtimeMutation.isPending ? t("common:saving") : t("common:save")}
        </Button>
      </div>
      <div className="grid gap-4 lg:grid-cols-2 2xl:grid-cols-[minmax(160px,200px)_minmax(200px,260px)_minmax(220px,1fr)_140px_150px] 2xl:items-start">
        <div className="space-y-1.5">
          <Label>{t("ai:tools.builtin.provider")}</Label>
          <Select
            value={effectiveProviderId ? String(effectiveProviderId) : ""}
            onValueChange={(v) => {
              setProviderId(Number(v))
              setModelId(0)
            }}
          >
            <SelectTrigger className="w-full">
              <SelectValue placeholder={t("ai:tools.builtin.providerPlaceholder")} />
            </SelectTrigger>
            <SelectContent>
              {providers.map((provider) => (
                <SelectItem key={provider.id} value={String(provider.id)}>
                  {provider.name}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>
        <div className="space-y-1.5">
          <Label>{t("ai:tools.builtin.model")}</Label>
          <Select value={modelId ? String(modelId) : ""} onValueChange={(v) => setModelId(Number(v))} disabled={!effectiveProviderId}>
            <SelectTrigger className="w-full">
              <SelectValue placeholder={t("ai:tools.builtin.modelPlaceholder")} />
            </SelectTrigger>
            <SelectContent>
              {providerModels.map((model) => (
                <SelectItem key={model.id} value={String(model.id)}>
                  {model.displayName}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>
        <div className="space-y-2">
          <div className="flex items-center justify-between gap-3">
            <Label>{t("ai:tools.builtin.temperature")}</Label>
            <span className="rounded-full border border-border/55 bg-background/35 px-2 py-0.5 font-mono text-[11px] text-muted-foreground">{temperature.toFixed(2)}</span>
          </div>
          <Slider min={0} max={1} step={0.05} value={[temperature]} onValueChange={([v]) => setTemperature(v)} />
        </div>
        <div className="space-y-1.5">
          <Label>{t("ai:tools.builtin.maxTokens")}</Label>
          <Input type="number" min={256} max={8192} value={maxTokens} onChange={(e) => setMaxTokens(Number(e.target.value))} />
        </div>
        <div className="space-y-1.5">
          <Label>{t("ai:tools.builtin.timeoutSeconds")}</Label>
          <div className="flex items-center gap-2">
            <Input type="number" min={5} max={300} value={timeoutSeconds} onChange={(e) => setTimeoutSeconds(Number(e.target.value))} />
            <span className="text-xs text-muted-foreground">{t("ai:tools.builtin.seconds")}</span>
          </div>
        </div>
      </div>
    </div>
  )
}
