import { useMemo, useState, type ComponentType, type ReactNode } from "react"
import { useTranslation } from "react-i18next"
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import { Activity, ExternalLink, Pencil, Route, Save, ShieldAlert } from "lucide-react"
import { useNavigate } from "react-router"
import { toast } from "sonner"
import { Alert, AlertDescription } from "@/components/ui/alert"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetFooter,
  SheetHeader,
  SheetTitle,
} from "@/components/ui/sheet"
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select"
import { Slider } from "@/components/ui/slider"
import { Textarea } from "@/components/ui/textarea"
import { WorkspaceStatus, type WorkspaceStatusTone } from "@/components/workspace/primitives"
import {
  type EngineSettingsConfig,
  type EngineSettingsConfigUpdate,
  type EngineHealthItem,
  fetchModels,
  fetchProviders,
  fetchEngineSettingsConfig,
  fetchUsers,
  updateEngineSettingsConfig,
} from "../../api"
import { validateEngineSettingsRuntime } from "./engine-settings-validation"

type SectionStatus = "pass" | "warn" | "fail"

function statusFromHealth(item: EngineHealthItem | undefined): SectionStatus {
  return item?.status ?? "fail"
}

function EngineStatus({ status, label }: { status: SectionStatus; label?: string }) {
  const { t } = useTranslation("itsm")
  const content = label ?? t(`engineConfig.status.${status}`)
  const toneByStatus: Record<SectionStatus, WorkspaceStatusTone> = {
    pass: "success",
    warn: "warning",
    fail: "danger",
  }

  return <WorkspaceStatus tone={toneByStatus[status]} label={content} className="shrink-0 whitespace-nowrap py-0.5 text-[11px]" />
}

function EngineSettingGroup({
  title,
  description,
  children,
}: {
  title: string
  description: string
  children: ReactNode
}) {
  return (
    <section className="workspace-surface overflow-hidden rounded-[1.15rem]">
      <div className="border-b border-border/45 bg-muted/16 px-5 py-4">
        <h3 className="text-sm font-semibold text-foreground">{title}</h3>
        <p className="mt-1 max-w-3xl text-xs leading-5 text-muted-foreground">{description}</p>
      </div>
      <div className="divide-y divide-border/45">{children}</div>
    </section>
  )
}

function EngineSettingRow({
  icon,
  title,
  description,
  health,
  children,
}: {
  icon: ComponentType<{ className?: string }>
  title: string
  description: string
  health: EngineHealthItem | undefined
  children: ReactNode
}) {
  const Icon = icon

  return (
    <div className="grid gap-5 px-5 py-5 xl:grid-cols-[minmax(230px,310px)_1fr] xl:items-start">
      <div className="flex min-w-0 gap-3">
        <div className="flex h-9 w-9 shrink-0 items-center justify-center rounded-md border border-border/65 bg-background/45">
          <Icon className="h-4.5 w-4.5 text-foreground" />
        </div>
        <div className="min-w-0">
          <div className="flex min-w-0 flex-wrap items-center gap-2">
            <h4 className="text-[15px] font-semibold text-foreground">{title}</h4>
            <EngineStatus status={statusFromHealth(health)} />
          </div>
          <p className="mt-1 text-xs leading-5 text-muted-foreground">{description}</p>
          {health?.message ? <p className="mt-2 text-xs leading-5 text-muted-foreground/85">{health.message}</p> : null}
        </div>
      </div>
      <div className="min-w-0">{children}</div>
    </div>
  )
}

function ConfigErrorState({ message, onRetry }: { message: string; onRetry: () => void }) {
  const { t } = useTranslation(["itsm", "common"])
  return (
    <div className="flex items-center justify-center py-12">
      <div className="workspace-surface w-full max-w-xl rounded-lg p-6 text-center">
        <p className="text-sm font-medium text-foreground">{t("itsm:engineConfig.configLoadErrorTitle")}</p>
        <p className="mt-2 text-sm text-muted-foreground">{message}</p>
        <Button type="button" variant="outline" className="mt-4" onClick={onRetry}>
          {t("common:refresh")}
        </Button>
      </div>
    </div>
  )
}

function PathBuilderFields({
  providerId,
  modelId,
  temperature,
  maxRetries,
  timeoutSeconds,
  systemPrompt,
  modelError,
  promptDrawerTitle,
  promptDrawerDescription,
  onProviderChange,
  onModelChange,
  onTemperatureChange,
  onMaxRetriesChange,
  onTimeoutSecondsChange,
  onApplySystemPrompt,
}: {
  providerId: number
  modelId: number
  temperature: number
  maxRetries: number
  timeoutSeconds: number
  systemPrompt: string
  modelError?: string
  promptDrawerTitle: string
  promptDrawerDescription: string
  onProviderChange: (id: number) => void
  onModelChange: (id: number) => void
  onTemperatureChange: (v: number) => void
  onMaxRetriesChange: (v: number) => void
  onTimeoutSecondsChange: (v: number) => void
  onApplySystemPrompt: (value: string) => Promise<boolean>
}) {
  const { t } = useTranslation("itsm")
  const navigate = useNavigate()
  const [promptEditorOpen, setPromptEditorOpen] = useState(false)
  const [promptDraft, setPromptDraft] = useState(systemPrompt)
  const [applyPending, setApplyPending] = useState(false)
  const promptPreview = useMemo(() => {
    const compact = systemPrompt.replace(/\s+/g, " ").trim()
    if (!compact) return t("engineConfig.systemPromptEmpty")
    if (compact.length <= 96) return compact
    return `${compact.slice(0, 96)}...`
  }, [systemPrompt, t])

  const { data: providers = [] } = useQuery({
    queryKey: ["ai-providers"],
    queryFn: fetchProviders,
  })

  const { data: models = [] } = useQuery({
    queryKey: ["ai-models", providerId],
    queryFn: () => fetchModels(providerId),
    enabled: providerId > 0,
  })

  if (providers.length === 0) {
    return (
      <Alert>
        <AlertDescription className="flex items-center justify-between gap-4">
          <span>{t("engineConfig.noProviders")}</span>
          <Button variant="link" size="sm" className="h-auto p-0" onClick={() => navigate("/ai/providers")}>
            {t("engineConfig.goToProviders")}
            <ExternalLink className="ml-1 h-3 w-3" />
          </Button>
        </AlertDescription>
      </Alert>
    )
  }

  return (
    <div className="grid gap-3.5 lg:grid-cols-2 2xl:grid-cols-[minmax(170px,210px)_minmax(210px,260px)_minmax(240px,1fr)_150px_170px] 2xl:items-start">
      <div className="space-y-1.5">
        <Label>{t("engineConfig.provider")}</Label>
        <Select
          value={providerId ? String(providerId) : ""}
          onValueChange={(v) => {
            onProviderChange(Number(v))
            onModelChange(0)
          }}
        >
          <SelectTrigger className="w-full">
            <SelectValue placeholder={t("engineConfig.providerPlaceholder")} />
          </SelectTrigger>
          <SelectContent>
            {providers.map((p) => (
              <SelectItem key={p.id} value={String(p.id)}>
                {p.name}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
      </div>
      <div className="space-y-1.5">
        <Label>{t("engineConfig.model")}</Label>
        <Select value={modelId ? String(modelId) : ""} onValueChange={(v) => onModelChange(Number(v))} disabled={!providerId}>
          <SelectTrigger className="w-full" aria-invalid={Boolean(modelError)}>
            <SelectValue placeholder={t("engineConfig.modelPlaceholder")} />
          </SelectTrigger>
          <SelectContent>
            {models.map((m) => (
              <SelectItem key={m.id} value={String(m.id)}>
                {m.displayName}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
        {modelError ? <p className="text-xs text-destructive">{modelError}</p> : null}
      </div>
      <div className="space-y-2">
        <div className="flex items-center justify-between gap-3">
          <Label>{t("engineConfig.temperature")}</Label>
          <span className="rounded-full border border-border/55 bg-background/35 px-2 py-0.5 font-mono text-[11px] text-muted-foreground">{temperature.toFixed(2)}</span>
        </div>
        <Slider min={0} max={1} step={0.05} value={[temperature]} onValueChange={([v]) => onTemperatureChange(v)} />
      </div>
      <div className="space-y-1.5">
        <Label>{t("engineConfig.maxRetries")}</Label>
        <Input type="number" min={0} max={10} value={maxRetries} onChange={(e) => onMaxRetriesChange(Number(e.target.value))} />
      </div>
      <div className="space-y-1.5">
        <Label>{t("engineConfig.timeoutSeconds")}</Label>
        <div className="flex items-center gap-2">
          <Input type="number" min={10} max={300} value={timeoutSeconds} onChange={(e) => onTimeoutSecondsChange(Number(e.target.value))} />
          <span className="text-xs text-muted-foreground">{t("engineConfig.seconds")}</span>
        </div>
      </div>
      <div className="lg:col-span-2 2xl:col-span-5">
        <div className="flex items-center justify-between gap-3 rounded-md border border-border/65 bg-muted/18 px-3 py-2.5">
          <div className="min-w-0">
            <div className="text-[11px] font-semibold uppercase tracking-[0.16em] text-muted-foreground/85">
              {t("engineConfig.systemPrompt")}
            </div>
            <p className="mt-1 line-clamp-2 text-xs leading-5 text-muted-foreground/90">{promptPreview}</p>
          </div>
          <Button
            type="button"
            variant="outline"
            size="icon-sm"
            className="shrink-0"
            onClick={() => {
              setPromptDraft(systemPrompt)
              setPromptEditorOpen(true)
            }}
            aria-label={t("engineConfig.openPromptEditor")}
            title={t("engineConfig.openPromptEditor")}
          >
            <Pencil className="size-3.5" />
          </Button>
        </div>
      </div>

      <Sheet open={promptEditorOpen} onOpenChange={setPromptEditorOpen}>
        <SheetContent className="flex h-full flex-col gap-0 p-0 sm:max-w-3xl">
          <SheetHeader className="border-b px-6 py-5">
            <SheetTitle>{promptDrawerTitle}</SheetTitle>
            <SheetDescription>{promptDrawerDescription}</SheetDescription>
          </SheetHeader>
          <div className="flex min-h-0 flex-1 flex-col space-y-2 overflow-hidden px-6 py-5">
            <Label>{t("engineConfig.systemPrompt")}</Label>
            <Textarea
              value={promptDraft}
              onChange={(e) => setPromptDraft(e.target.value)}
              placeholder={t("engineConfig.systemPromptPlaceholder")}
              className="h-full min-h-0 flex-1 resize-none overflow-y-auto font-mono text-xs leading-6"
            />
          </div>
          <SheetFooter className="border-t px-6 py-4">
            <Button
              type="button"
              variant="outline"
              disabled={applyPending}
              onClick={() => {
                setPromptDraft(systemPrompt)
                setPromptEditorOpen(false)
              }}
            >
              {t("common:cancel")}
            </Button>
            <Button
              type="button"
              disabled={applyPending}
              onClick={async () => {
                setApplyPending(true)
                try {
                  const ok = await onApplySystemPrompt(promptDraft)
                  if (ok) {
                    setPromptEditorOpen(false)
                  }
                } finally {
                  setApplyPending(false)
                }
              }}
            >
              {applyPending ? t("common:saving") : t("engineConfig.applyPrompt")}
            </Button>
          </SheetFooter>
        </SheetContent>
      </Sheet>
    </div>
  )
}

function configFormKey(config: EngineSettingsConfig) {
  return [
    config.runtime.pathBuilder.providerId,
    config.runtime.pathBuilder.modelId,
    config.runtime.pathBuilder.temperature,
    config.runtime.pathBuilder.maxRetries,
    config.runtime.pathBuilder.timeoutSeconds,
    config.runtime.pathBuilder.systemPrompt,
    config.runtime.titleBuilder.providerId,
    config.runtime.titleBuilder.modelId,
    config.runtime.titleBuilder.temperature,
    config.runtime.titleBuilder.maxRetries,
    config.runtime.titleBuilder.timeoutSeconds,
    config.runtime.titleBuilder.systemPrompt,
    config.runtime.healthChecker.providerId,
    config.runtime.healthChecker.modelId,
    config.runtime.healthChecker.temperature,
    config.runtime.healthChecker.maxRetries,
    config.runtime.healthChecker.timeoutSeconds,
    config.runtime.healthChecker.systemPrompt,
    config.runtime.guard.auditLevel,
    config.runtime.guard.fallbackAssignee,
  ].join(":")
}

function EngineSettingsForm({ config }: { config: EngineSettingsConfig }) {
  const { t } = useTranslation(["itsm", "common"])
  const queryClient = useQueryClient()

  const { data: fallbackUsers = [] } = useQuery({
    queryKey: ["users-for-engine-settings-fallback"],
    queryFn: () => fetchUsers(),
  })

  const [pathProviderId, setPathProviderId] = useState(config.runtime.pathBuilder.providerId)
  const [pathModelId, setPathModelId] = useState(config.runtime.pathBuilder.modelId)
  const [pathTemperature, setPathTemperature] = useState(config.runtime.pathBuilder.temperature)
  const [pathMaxRetries, setPathMaxRetries] = useState(config.runtime.pathBuilder.maxRetries)
  const [pathTimeoutSeconds, setPathTimeoutSeconds] = useState(config.runtime.pathBuilder.timeoutSeconds)
  const [pathSystemPrompt, setPathSystemPrompt] = useState(config.runtime.pathBuilder.systemPrompt)
  const [titleProviderId, setTitleProviderId] = useState(config.runtime.titleBuilder.providerId)
  const [titleModelId, setTitleModelId] = useState(config.runtime.titleBuilder.modelId)
  const [titleTemperature, setTitleTemperature] = useState(config.runtime.titleBuilder.temperature)
  const [titleMaxRetries, setTitleMaxRetries] = useState(config.runtime.titleBuilder.maxRetries)
  const [titleTimeoutSeconds, setTitleTimeoutSeconds] = useState(config.runtime.titleBuilder.timeoutSeconds)
  const [titleSystemPrompt, setTitleSystemPrompt] = useState(config.runtime.titleBuilder.systemPrompt)
  const [healthProviderId, setHealthProviderId] = useState(config.runtime.healthChecker.providerId)
  const [healthModelId, setHealthModelId] = useState(config.runtime.healthChecker.modelId)
  const [healthTemperature, setHealthTemperature] = useState(config.runtime.healthChecker.temperature)
  const [healthMaxRetries, setHealthMaxRetries] = useState(config.runtime.healthChecker.maxRetries)
  const [healthTimeoutSeconds, setHealthTimeoutSeconds] = useState(config.runtime.healthChecker.timeoutSeconds)
  const [healthSystemPrompt, setHealthSystemPrompt] = useState(config.runtime.healthChecker.systemPrompt)
  const [auditLevel, setAuditLevel] = useState(config.runtime.guard.auditLevel)
  const [fallbackAssignee, setFallbackAssignee] = useState(config.runtime.guard.fallbackAssignee)

  const healthByKey = useMemo(() => {
    const map = new Map<string, EngineHealthItem>()
    for (const item of config.health.items) {
      map.set(item.key, item)
    }
    return map
  }, [config.health.items])

  const fallbackUserKnown = fallbackAssignee === 0 || fallbackUsers.some((u) => u.id === fallbackAssignee)
  const runtimeValidation = useMemo(() => validateEngineSettingsRuntime({
    pathBuilder: { modelId: pathModelId },
    titleBuilder: { modelId: titleModelId },
    healthChecker: { modelId: healthModelId },
  }), [healthModelId, pathModelId, titleModelId])
  const validationMessage = Object.values(runtimeValidation.errors)[0] ?? "引擎设置未完成"

  const saveMut = useMutation({
    mutationFn: (data: EngineSettingsConfigUpdate) => updateEngineSettingsConfig(data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["itsm-engine-settings-config"] })
      toast.success(t("itsm:engineConfig.engineSaveSuccess"))
    },
    onError: (err) => toast.error(err.message),
  })

  function buildRuntimePayload(pathPrompt: string, titlePrompt: string, healthPrompt: string): EngineSettingsConfigUpdate {
    return {
      runtime: {
        pathBuilder: {
          modelId: pathModelId,
          temperature: pathTemperature,
          maxRetries: pathMaxRetries,
          timeoutSeconds: pathTimeoutSeconds,
          systemPrompt: pathPrompt,
        },
        titleBuilder: {
          modelId: titleModelId,
          temperature: titleTemperature,
          maxRetries: titleMaxRetries,
          timeoutSeconds: titleTimeoutSeconds,
          systemPrompt: titlePrompt,
        },
        healthChecker: {
          modelId: healthModelId,
          temperature: healthTemperature,
          maxRetries: healthMaxRetries,
          timeoutSeconds: healthTimeoutSeconds,
          systemPrompt: healthPrompt,
        },
        guard: { auditLevel, fallbackAssignee },
      },
    }
  }

  function handleSave() {
    if (!runtimeValidation.valid) {
      toast.error(validationMessage)
      return
    }
    saveMut.mutate(buildRuntimePayload(pathSystemPrompt, titleSystemPrompt, healthSystemPrompt))
  }

  async function applyPathPrompt(nextPrompt: string) {
    if (!runtimeValidation.valid) {
      toast.error(validationMessage)
      return false
    }
    try {
      await saveMut.mutateAsync(buildRuntimePayload(nextPrompt, titleSystemPrompt, healthSystemPrompt))
      setPathSystemPrompt(nextPrompt)
      return true
    } catch {
      return false
    }
  }

  async function applyTitlePrompt(nextPrompt: string) {
    if (!runtimeValidation.valid) {
      toast.error(validationMessage)
      return false
    }
    try {
      await saveMut.mutateAsync(buildRuntimePayload(pathSystemPrompt, nextPrompt, healthSystemPrompt))
      setTitleSystemPrompt(nextPrompt)
      return true
    } catch {
      return false
    }
  }

  async function applyHealthPrompt(nextPrompt: string) {
    if (!runtimeValidation.valid) {
      toast.error(validationMessage)
      return false
    }
    try {
      await saveMut.mutateAsync(buildRuntimePayload(pathSystemPrompt, titleSystemPrompt, nextPrompt))
      setHealthSystemPrompt(nextPrompt)
      return true
    } catch {
      return false
    }
  }

  return (
    <div className="workspace-page">
      <div className="workspace-page-header">
        <div className="min-w-0">
          <h2 className="workspace-page-title">{t("itsm:engineConfig.engineSettingsTitle")}</h2>
          <p className="workspace-page-description">{t("itsm:engineConfig.engineSettingsDesc")}</p>
        </div>
        <Button className="shrink-0" onClick={handleSave} disabled={saveMut.isPending || !runtimeValidation.valid}>
          <Save className="mr-1.5 h-4 w-4" />
          {saveMut.isPending ? t("common:saving") : t("common:save")}
        </Button>
      </div>

      <div className="space-y-5">
        {!runtimeValidation.valid && validationMessage ? (
          <Alert>
            <AlertDescription>{validationMessage}</AlertDescription>
          </Alert>
        ) : null}
        <EngineSettingGroup
          title={t("itsm:engineConfig.modelCapabilityGroupTitle")}
          description={t("itsm:engineConfig.modelCapabilityGroupDesc")}
        >
          <EngineSettingRow
            icon={Route}
            title={t("itsm:engineConfig.pathBuilderTitle")}
            description={t("itsm:engineConfig.pathBuilderDesc")}
            health={healthByKey.get("pathBuilder")}
          >
            <PathBuilderFields
              providerId={pathProviderId}
              modelId={pathModelId}
              temperature={pathTemperature}
              maxRetries={pathMaxRetries}
              timeoutSeconds={pathTimeoutSeconds}
              systemPrompt={pathSystemPrompt}
              modelError={runtimeValidation.errors.pathBuilder}
              promptDrawerTitle={t("engineConfig.pathPromptEditorTitle")}
              promptDrawerDescription={t("engineConfig.pathPromptEditorDesc")}
              onProviderChange={setPathProviderId}
              onModelChange={setPathModelId}
              onTemperatureChange={setPathTemperature}
              onMaxRetriesChange={setPathMaxRetries}
              onTimeoutSecondsChange={setPathTimeoutSeconds}
              onApplySystemPrompt={applyPathPrompt}
            />
          </EngineSettingRow>

          <EngineSettingRow
            icon={Route}
            title={t("itsm:engineConfig.titleBuilderTitle")}
            description={t("itsm:engineConfig.titleBuilderDesc")}
            health={healthByKey.get("titleBuilder")}
          >
            <PathBuilderFields
              providerId={titleProviderId}
              modelId={titleModelId}
              temperature={titleTemperature}
              maxRetries={titleMaxRetries}
              timeoutSeconds={titleTimeoutSeconds}
              systemPrompt={titleSystemPrompt}
              modelError={runtimeValidation.errors.titleBuilder}
              promptDrawerTitle={t("engineConfig.titlePromptEditorTitle")}
              promptDrawerDescription={t("engineConfig.titlePromptEditorDesc")}
              onProviderChange={setTitleProviderId}
              onModelChange={setTitleModelId}
              onTemperatureChange={setTitleTemperature}
              onMaxRetriesChange={setTitleMaxRetries}
              onTimeoutSecondsChange={setTitleTimeoutSeconds}
              onApplySystemPrompt={applyTitlePrompt}
            />
          </EngineSettingRow>

          <EngineSettingRow
            icon={Activity}
            title={t("itsm:engineConfig.healthCheckerTitle")}
            description={t("itsm:engineConfig.healthCheckerDesc")}
            health={healthByKey.get("healthChecker")}
          >
            <PathBuilderFields
              providerId={healthProviderId}
              modelId={healthModelId}
              temperature={healthTemperature}
              maxRetries={healthMaxRetries}
              timeoutSeconds={healthTimeoutSeconds}
              systemPrompt={healthSystemPrompt}
              modelError={runtimeValidation.errors.healthChecker}
              promptDrawerTitle={t("engineConfig.healthPromptEditorTitle")}
              promptDrawerDescription={t("engineConfig.healthPromptEditorDesc")}
              onProviderChange={setHealthProviderId}
              onModelChange={setHealthModelId}
              onTemperatureChange={setHealthTemperature}
              onMaxRetriesChange={setHealthMaxRetries}
              onTimeoutSecondsChange={setHealthTimeoutSeconds}
              onApplySystemPrompt={applyHealthPrompt}
            />
          </EngineSettingRow>
        </EngineSettingGroup>

        <EngineSettingGroup
          title={t("itsm:engineConfig.runtimeGuardGroupTitle")}
          description={t("itsm:engineConfig.runtimeGuardGroupDesc")}
        >
          <EngineSettingRow
            icon={ShieldAlert}
            title={t("itsm:engineConfig.guardTitle")}
            description={t("itsm:engineConfig.guardDesc")}
            health={healthByKey.get("guard")}
          >
            <div className="grid gap-4 md:grid-cols-2 xl:grid-cols-[220px_320px]">
              <div className="space-y-1.5">
                <Label>{t("engineConfig.auditLevel")}</Label>
                <Select value={auditLevel} onValueChange={setAuditLevel}>
                  <SelectTrigger className="w-full">
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="full">{t("engineConfig.logFull")}</SelectItem>
                    <SelectItem value="summary">{t("engineConfig.logSummary")}</SelectItem>
                    <SelectItem value="off">{t("engineConfig.logOff")}</SelectItem>
                  </SelectContent>
                </Select>
              </div>
              <div className="space-y-1.5">
                <Label>{t("engineConfig.fallbackAssignee")}</Label>
                <Select value={String(fallbackAssignee)} onValueChange={(v) => setFallbackAssignee(Number(v))}>
                  <SelectTrigger className="w-full">
                    <SelectValue placeholder={t("engineConfig.fallbackAssigneePlaceholder")} />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="0">{t("engineConfig.fallbackAssigneeNone")}</SelectItem>
                    {!fallbackUserKnown && (
                      <SelectItem value={String(fallbackAssignee)}>
                        {t("engineConfig.fallbackAssigneeUnknown", { id: fallbackAssignee })}
                      </SelectItem>
                    )}
                    {fallbackUsers.map((user) => (
                      <SelectItem key={user.id} value={String(user.id)}>
                        {user.username}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </div>
            </div>
          </EngineSettingRow>
        </EngineSettingGroup>
      </div>
    </div>
  )
}

export function Component() {
  const { t } = useTranslation("common")

  const { data: config, error, isError, isLoading, refetch } = useQuery({
    queryKey: ["itsm-engine-settings-config"],
    queryFn: fetchEngineSettingsConfig,
  })

  if (isError) {
    return <ConfigErrorState message={error.message} onRetry={() => { void refetch() }} />
  }

  if (isLoading || !config) {
    return (
      <div className="flex items-center justify-center py-12">
        <span className="text-muted-foreground">{t("loading")}</span>
      </div>
    )
  }

  return <EngineSettingsForm key={configFormKey(config)} config={config} />
}
