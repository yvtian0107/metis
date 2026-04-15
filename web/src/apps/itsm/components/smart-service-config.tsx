"use client"

import { useTranslation } from "react-i18next"
import { useQuery } from "@tanstack/react-query"
import { Info } from "lucide-react"
import { Textarea } from "@/components/ui/textarea"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Slider } from "@/components/ui/slider"
import {
  Select, SelectContent, SelectItem, SelectTrigger, SelectValue,
} from "@/components/ui/select"
import { Alert, AlertDescription } from "@/components/ui/alert"
import { fetchAgents } from "../api"

interface SmartServiceConfigProps {
  collaborationSpec: string
  onCollaborationSpecChange: (v: string) => void
  agentId: number | null
  onAgentIdChange: (v: number | null) => void
  confidenceThreshold: number
  onConfidenceThresholdChange: (v: number) => void
  decisionTimeout: number
  onDecisionTimeoutChange: (v: number) => void
}

export function SmartServiceConfig({
  collaborationSpec,
  onCollaborationSpecChange,
  agentId,
  onAgentIdChange,
  confidenceThreshold,
  onConfidenceThresholdChange,
  decisionTimeout,
  onDecisionTimeoutChange,
}: SmartServiceConfigProps) {
  const { t } = useTranslation("itsm")

  const { data: agents = [], isError: agentsError } = useQuery({
    queryKey: ["ai-agents"],
    queryFn: () => fetchAgents(),
  })

  if (agentsError) {
    return (
      <Alert>
        <Info className="h-4 w-4" />
        <AlertDescription>{t("smart.aiUnavailable")}</AlertDescription>
      </Alert>
    )
  }

  return (
    <div className="space-y-4 rounded-md border p-4">
      <h4 className="text-sm font-medium">{t("smart.configTitle")}</h4>

      {/* Agent Select */}
      <div className="space-y-1.5">
        <Label>{t("smart.agent")}</Label>
        <Select
          value={agentId ? String(agentId) : ""}
          onValueChange={(v) => onAgentIdChange(v ? Number(v) : null)}
        >
          <SelectTrigger>
            <SelectValue placeholder={t("smart.agentPlaceholder")} />
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

      {/* Collaboration Spec */}
      <div className="space-y-1.5">
        <Label>{t("smart.collaborationSpec")}</Label>
        <Textarea
          rows={6}
          placeholder={t("smart.collaborationSpecPlaceholder")}
          value={collaborationSpec}
          onChange={(e) => onCollaborationSpecChange(e.target.value)}
        />
      </div>

      {/* Confidence Threshold */}
      <div className="space-y-1.5">
        <div className="flex items-center justify-between">
          <Label>{t("smart.confidenceThreshold")}</Label>
          <span className="text-xs text-muted-foreground">{confidenceThreshold.toFixed(2)}</span>
        </div>
        <Slider
          min={0}
          max={1}
          step={0.05}
          value={[confidenceThreshold]}
          onValueChange={([v]) => onConfidenceThresholdChange(v)}
        />
      </div>

      {/* Decision Timeout */}
      <div className="space-y-1.5">
        <Label>{t("smart.decisionTimeout")}</Label>
        <div className="flex items-center gap-2">
          <Input
            type="number"
            min={10}
            max={120}
            value={decisionTimeout}
            onChange={(e) => onDecisionTimeoutChange(Number(e.target.value))}
            className="w-24"
          />
          <span className="text-xs text-muted-foreground">{t("smart.seconds")}</span>
        </div>
      </div>
    </div>
  )
}
