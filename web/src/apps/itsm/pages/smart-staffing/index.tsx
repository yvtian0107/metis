import { useMemo, useState } from "react"
import type { ComponentType, ReactNode } from "react"
import { useTranslation } from "react-i18next"
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query"
import { Activity, Bot, ExternalLink, Save, ShieldCheck } from "lucide-react"
import { toast } from "sonner"
import { useNavigate } from "react-router"
import { Alert, AlertDescription } from "@/components/ui/alert"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardDescription, CardTitle } from "@/components/ui/card"
import { Label } from "@/components/ui/label"
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select"
import { WorkspaceStatus, type WorkspaceStatusTone } from "@/components/workspace/primitives"
import {
  type AgentItem,
  type EngineHealthItem,
  type SmartStaffingConfig,
  type SmartStaffingConfigUpdate,
  fetchAgents,
  fetchSmartStaffingConfig,
  updateSmartStaffingConfig,
} from "../../api"

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

function AgentPreview({ agent }: { agent: AgentItem | undefined }) {
  const { t } = useTranslation("itsm")
  if (!agent) return null
  const strategyLabel = agent.strategy === "plan_and_execute"
    ? t("engineConfig.strategyPlanAndExecute")
    : t("engineConfig.strategyReact")
  return (
    <p className="text-xs leading-5 text-muted-foreground">
      {t("engineConfig.previewStrategy")}: {strategyLabel} · {t("engineConfig.previewTemperature")}: {agent.temperature.toFixed(2)} · {t("engineConfig.previewMaxTurns")}: {agent.maxTurns}
    </p>
  )
}

function EmptyAgentsAlert() {
  const { t } = useTranslation("itsm")
  const navigate = useNavigate()
  return (
    <Alert>
      <AlertDescription className="flex items-center justify-between gap-4">
        <span>{t("engineConfig.noAgents")}</span>
        <Button variant="link" size="sm" className="h-auto p-0" onClick={() => navigate("/ai/assistant-agents")}>
          {t("engineConfig.goToAgents")}
          <ExternalLink className="ml-1 h-3 w-3" />
        </Button>
      </AlertDescription>
    </Alert>
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

function AgentField({
  agentId,
  agents,
  onAgentChange,
}: {
  agentId: number
  agents: AgentItem[]
  onAgentChange: (id: number) => void
}) {
  const { t } = useTranslation("itsm")
  const selectedAgent = agentId ? agents.find((a) => a.id === agentId) : undefined

  if (agents.length === 0) {
    return <EmptyAgentsAlert />
  }

  return (
    <div className="space-y-2">
      <div className="space-y-1.5">
        <Label>{t("engineConfig.agent")}</Label>
        <Select value={agentId ? String(agentId) : ""} onValueChange={(v) => onAgentChange(Number(v))}>
          <SelectTrigger className="w-full">
            <SelectValue placeholder={t("engineConfig.agentPlaceholder")} />
          </SelectTrigger>
          <SelectContent>
            {agents.map((a) => (
              <SelectItem key={a.id} value={String(a.id)}>
                {a.name}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
      </div>
      <AgentPreview agent={selectedAgent} />
    </div>
  )
}

function StaffingSection({
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
    <Card className="min-w-0 gap-0 overflow-hidden py-0">
      <div className="border-b border-border/45 px-5 py-4">
        <div className="flex min-w-0 items-start gap-3">
          <div className="flex h-9 w-9 shrink-0 items-center justify-center rounded-md border bg-muted/45">
            <Icon className="h-4.5 w-4.5 text-foreground" />
          </div>
          <div className="min-w-0 flex-1">
            <div className="flex min-w-0 flex-wrap items-center gap-2">
              <CardTitle className="text-[15px]">{title}</CardTitle>
              <EngineStatus status={statusFromHealth(health)} />
            </div>
            <CardDescription className="mt-1 text-xs leading-5">{description}</CardDescription>
            {health?.message ? <p className="mt-2 text-xs leading-5 text-muted-foreground">{health.message}</p> : null}
          </div>
        </div>
      </div>
      <CardContent className="px-5 py-4">{children}</CardContent>
    </Card>
  )
}

function configFormKey(config: SmartStaffingConfig) {
  return [
    config.posts.intake.agentId,
    config.posts.decision.agentId,
    config.posts.decision.mode,
    config.posts.slaAssurance.agentId,
  ].join(":")
}

function SmartStaffingForm({ config }: { config: SmartStaffingConfig }) {
  const { t } = useTranslation(["itsm", "common"])
  const queryClient = useQueryClient()

  const { data: agents = [] } = useQuery({
    queryKey: ["ai-agents-for-smart-staffing"],
    queryFn: fetchAgents,
    select: (list) => list.filter((a) => a.type === "assistant" && a.isActive),
  })

  const healthByKey = useMemo(() => {
    const map = new Map<string, EngineHealthItem>()
    for (const item of config.health.items) {
      map.set(item.key, item)
    }
    return map
  }, [config.health.items])

  const [intakeAgentId, setIntakeAgentId] = useState(config.posts.intake.agentId)
  const [decisionAgentId, setDecisionAgentId] = useState(config.posts.decision.agentId)
  const [decisionMode, setDecisionMode] = useState(config.posts.decision.mode || "direct_first")
  const [slaAssuranceAgentId, setSlaAssuranceAgentId] = useState(config.posts.slaAssurance.agentId)

  const saveMut = useMutation({
    mutationFn: (data: SmartStaffingConfigUpdate) => updateSmartStaffingConfig(data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["itsm-smart-staffing-config"] })
      toast.success(t("itsm:engineConfig.saveSuccess"))
    },
    onError: (err) => toast.error(err.message),
  })

  function handleSave() {
    saveMut.mutate({
      posts: {
        intake: { agentId: intakeAgentId },
        decision: { agentId: decisionAgentId, mode: decisionMode },
        slaAssurance: { agentId: slaAssuranceAgentId },
      },
    })
  }

  return (
    <div className="workspace-page">
      <div className="workspace-page-header">
        <div className="min-w-0">
          <h2 className="workspace-page-title">{t("itsm:engineConfig.title")}</h2>
          <p className="workspace-page-description">{t("itsm:engineConfig.pageDesc")}</p>
        </div>
        <Button className="shrink-0" onClick={handleSave} disabled={saveMut.isPending}>
          <Save className="mr-1.5 h-4 w-4" />
          {saveMut.isPending ? t("common:saving") : t("common:save")}
        </Button>
      </div>

      <div className="grid w-full items-start gap-4 lg:grid-cols-2 xl:grid-cols-3">
        <StaffingSection
          icon={Bot}
          title={t("itsm:engineConfig.intakeTitle")}
          description={t("itsm:engineConfig.intakeDesc")}
          health={healthByKey.get("intake")}
        >
          <AgentField agentId={intakeAgentId} agents={agents} onAgentChange={setIntakeAgentId} />
        </StaffingSection>

        <StaffingSection
          icon={Activity}
          title={t("itsm:engineConfig.decisionTitle")}
          description={t("itsm:engineConfig.decisionDesc")}
          health={healthByKey.get("decision")}
        >
          <div className="grid gap-4">
            <AgentField agentId={decisionAgentId} agents={agents} onAgentChange={setDecisionAgentId} />
            <div className="space-y-1.5">
              <Label>{t("itsm:engineConfig.decisionMode")}</Label>
              <Select value={decisionMode} onValueChange={setDecisionMode}>
                <SelectTrigger className="w-full">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="direct_first">{t("itsm:engineConfig.modeDirectFirst")}</SelectItem>
                  <SelectItem value="ai_only">{t("itsm:engineConfig.modeAiOnly")}</SelectItem>
                </SelectContent>
              </Select>
            </div>
          </div>
        </StaffingSection>

        <StaffingSection
          icon={ShieldCheck}
          title={t("itsm:engineConfig.slaAssuranceTitle")}
          description={t("itsm:engineConfig.slaAssuranceDesc")}
          health={healthByKey.get("slaAssurance")}
        >
          <AgentField agentId={slaAssuranceAgentId} agents={agents} onAgentChange={setSlaAssuranceAgentId} />
        </StaffingSection>
      </div>
    </div>
  )
}

export function Component() {
  const { t } = useTranslation("common")

  const { data: config, error, isError, isLoading, refetch } = useQuery({
    queryKey: ["itsm-smart-staffing-config"],
    queryFn: fetchSmartStaffingConfig,
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

  return <SmartStaffingForm key={configFormKey(config)} config={config} />
}
