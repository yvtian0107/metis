import { useState, useEffect } from "react"
import { useTranslation } from "react-i18next"
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query"
import { Settings, Save, ExternalLink } from "lucide-react"
import { toast } from "sonner"
import { useNavigate } from "react-router"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select"
import { Slider } from "@/components/ui/slider"
import { Alert, AlertDescription } from "@/components/ui/alert"
import {
  type EngineConfigUpdate,
  fetchEngineConfig,
  updateEngineConfig,
  fetchProviders,
  fetchModels,
} from "../../api"

// Provider → Model → Temperature fields (no Card wrapper)
function LLMFields({
  providerId,
  modelId,
  temperature,
  onProviderChange,
  onModelChange,
  onTemperatureChange,
}: {
  providerId: number
  modelId: number
  temperature: number
  onProviderChange: (id: number) => void
  onModelChange: (id: number) => void
  onTemperatureChange: (v: number) => void
}) {
  const { t } = useTranslation("itsm")
  const navigate = useNavigate()

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
        <AlertDescription className="flex items-center justify-between">
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
    <>
      <div className="grid grid-cols-2 gap-4">
        <div className="space-y-1.5">
          <Label>{t("engineConfig.provider")}</Label>
          <Select
            value={providerId ? String(providerId) : ""}
            onValueChange={(v) => {
              onProviderChange(Number(v))
              onModelChange(0)
            }}
          >
            <SelectTrigger>
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
          <Select
            value={modelId ? String(modelId) : ""}
            onValueChange={(v) => onModelChange(Number(v))}
            disabled={!providerId}
          >
            <SelectTrigger>
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
        </div>
      </div>
      <div className="space-y-1.5">
        <div className="flex items-center justify-between">
          <Label>{t("engineConfig.temperature")}</Label>
          <span className="text-xs text-muted-foreground">{temperature.toFixed(2)}</span>
        </div>
        <Slider
          min={0}
          max={1}
          step={0.05}
          value={[temperature]}
          onValueChange={([v]) => onTemperatureChange(v)}
        />
      </div>
    </>
  )
}

export function Component() {
  const { t } = useTranslation(["itsm", "common"])
  const queryClient = useQueryClient()

  const { data: config, isLoading } = useQuery({
    queryKey: ["itsm-engine-config"],
    queryFn: fetchEngineConfig,
  })

  // Local form state — generator
  const [genProviderId, setGenProviderId] = useState(0)
  const [genModelId, setGenModelId] = useState(0)
  const [genTemp, setGenTemp] = useState(0.3)
  // servicedesk agent
  const [sdProviderId, setSdProviderId] = useState(0)
  const [sdModelId, setSdModelId] = useState(0)
  const [sdTemp, setSdTemp] = useState(0.7)
  // decision agent
  const [decProviderId, setDecProviderId] = useState(0)
  const [decModelId, setDecModelId] = useState(0)
  const [decTemp, setDecTemp] = useState(0.1)
  const [decisionMode, setDecisionMode] = useState("direct_first")
  // general
  const [maxRetries, setMaxRetries] = useState(3)
  const [timeoutSeconds, setTimeoutSeconds] = useState(30)
  const [reasoningLog, setReasoningLog] = useState("full")
  const [fallbackAssignee, setFallbackAssignee] = useState(0)

  useEffect(() => {
    if (!config) return
    setGenProviderId(config.generator.providerId)
    setGenModelId(config.generator.modelId)
    setGenTemp(config.generator.temperature)
    setSdProviderId(config.servicedesk.providerId)
    setSdModelId(config.servicedesk.modelId)
    setSdTemp(config.servicedesk.temperature)
    setDecProviderId(config.decision.providerId)
    setDecModelId(config.decision.modelId)
    setDecTemp(config.decision.temperature)
    setDecisionMode(config.decision.decisionMode || "direct_first")
    setMaxRetries(config.general.maxRetries)
    setTimeoutSeconds(config.general.timeoutSeconds)
    setReasoningLog(config.general.reasoningLog)
    setFallbackAssignee(config.general.fallbackAssignee)
  }, [config])

  const saveMut = useMutation({
    mutationFn: (data: EngineConfigUpdate) => updateEngineConfig(data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["itsm-engine-config"] })
      toast.success(t("itsm:engineConfig.saveSuccess"))
    },
    onError: (err) => toast.error(err.message),
  })

  function handleSave() {
    saveMut.mutate({
      generator: { modelId: genModelId, temperature: genTemp },
      servicedesk: { modelId: sdModelId, temperature: sdTemp },
      decision: { modelId: decModelId, temperature: decTemp, decisionMode },
      general: { maxRetries, timeoutSeconds, reasoningLog, fallbackAssignee },
    })
  }

  if (isLoading) {
    return (
      <div className="flex items-center justify-center py-12">
        <span className="text-muted-foreground">{t("common:loading")}</span>
      </div>
    )
  }

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-2">
          <Settings className="h-5 w-5" />
          <h2 className="text-lg font-semibold">{t("itsm:engineConfig.title")}</h2>
        </div>
        <Button onClick={handleSave} disabled={saveMut.isPending}>
          <Save className="mr-1.5 h-4 w-4" />
          {saveMut.isPending ? t("common:saving") : t("common:save")}
        </Button>
      </div>

      {/* Generator Engine */}
      <Card>
        <CardHeader>
          <CardTitle className="text-base">{t("itsm:engineConfig.generatorTitle")}</CardTitle>
          <CardDescription>{t("itsm:engineConfig.generatorDesc")}</CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          <LLMFields
            providerId={genProviderId}
            modelId={genModelId}
            temperature={genTemp}
            onProviderChange={setGenProviderId}
            onModelChange={setGenModelId}
            onTemperatureChange={setGenTemp}
          />
        </CardContent>
      </Card>

      {/* Servicedesk Agent */}
      <Card>
        <CardHeader>
          <CardTitle className="text-base">{t("itsm:engineConfig.servicedeskTitle")}</CardTitle>
          <CardDescription>{t("itsm:engineConfig.servicedeskDesc")}</CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          <LLMFields
            providerId={sdProviderId}
            modelId={sdModelId}
            temperature={sdTemp}
            onProviderChange={setSdProviderId}
            onModelChange={setSdModelId}
            onTemperatureChange={setSdTemp}
          />
        </CardContent>
      </Card>

      {/* Decision Agent */}
      <Card>
        <CardHeader>
          <CardTitle className="text-base">{t("itsm:engineConfig.decisionTitle")}</CardTitle>
          <CardDescription>{t("itsm:engineConfig.decisionDesc")}</CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          <LLMFields
            providerId={decProviderId}
            modelId={decModelId}
            temperature={decTemp}
            onProviderChange={setDecProviderId}
            onModelChange={setDecModelId}
            onTemperatureChange={setDecTemp}
          />
          <div className="space-y-1.5">
            <Label>{t("itsm:engineConfig.decisionMode")}</Label>
            <Select value={decisionMode} onValueChange={setDecisionMode}>
              <SelectTrigger>
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="direct_first">{t("itsm:engineConfig.modeDirectFirst")}</SelectItem>
                <SelectItem value="ai_only">{t("itsm:engineConfig.modeAiOnly")}</SelectItem>
              </SelectContent>
            </Select>
          </div>
        </CardContent>
      </Card>

      {/* General Settings */}
      <Card>
        <CardHeader>
          <CardTitle className="text-base">{t("itsm:engineConfig.generalTitle")}</CardTitle>
          <CardDescription>{t("itsm:engineConfig.generalDesc")}</CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="grid grid-cols-2 gap-4">
            <div className="space-y-1.5">
              <Label>{t("itsm:engineConfig.maxRetries")}</Label>
              <Input
                type="number"
                min={0}
                max={10}
                value={maxRetries}
                onChange={(e) => setMaxRetries(Number(e.target.value))}
              />
            </div>
            <div className="space-y-1.5">
              <Label>{t("itsm:engineConfig.timeoutSeconds")}</Label>
              <div className="flex items-center gap-2">
                <Input
                  type="number"
                  min={10}
                  max={300}
                  value={timeoutSeconds}
                  onChange={(e) => setTimeoutSeconds(Number(e.target.value))}
                />
                <span className="text-xs text-muted-foreground">{t("itsm:engineConfig.seconds")}</span>
              </div>
            </div>
          </div>
          <div className="space-y-1.5">
            <Label>{t("itsm:engineConfig.reasoningLog")}</Label>
            <Select value={reasoningLog} onValueChange={setReasoningLog}>
              <SelectTrigger>
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="full">{t("itsm:engineConfig.logFull")}</SelectItem>
                <SelectItem value="summary">{t("itsm:engineConfig.logSummary")}</SelectItem>
                <SelectItem value="off">{t("itsm:engineConfig.logOff")}</SelectItem>
              </SelectContent>
            </Select>
          </div>
        </CardContent>
      </Card>
    </div>
  )
}
