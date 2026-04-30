"use client"

import { useTranslation } from "react-i18next"
import { CollaborationSpecField } from "./collaboration-spec-field"

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
    <CollaborationSpecField
      label={t("services.collaborationSpecLabel")}
      placeholder={t("smart.collaborationSpecPlaceholder")}
      assistText={t("smart.collaborationSpecAssist")}
      value={collaborationSpec}
      onChange={onCollaborationSpecChange}
    />
  )
}
