"use client"

import { useTranslation } from "react-i18next"
import { Textarea } from "@/components/ui/textarea"
import { Label } from "@/components/ui/label"

interface SmartServiceConfigProps {
  collaborationSpec: string
  onCollaborationSpecChange: (v: string) => void
}

export function SmartServiceConfig({
  collaborationSpec,
  onCollaborationSpecChange,
}: SmartServiceConfigProps) {
  const { t } = useTranslation("itsm")

  return (
    <div className="space-y-1.5">
      <Label>{t("services.collaborationSpecLabel")}</Label>
      <Textarea
        rows={4}
        placeholder={t("smart.collaborationSpecPlaceholder")}
        value={collaborationSpec}
        onChange={(e) => onCollaborationSpecChange(e.target.value)}
      />
    </div>
  )
}
