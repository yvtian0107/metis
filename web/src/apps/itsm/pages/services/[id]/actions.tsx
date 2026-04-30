"use client"

import { useParams, useNavigate } from "react-router"
import { useTranslation } from "react-i18next"
import { useQuery } from "@tanstack/react-query"
import { ArrowLeft, Zap } from "lucide-react"
import { Button } from "@/components/ui/button"
import { fetchServiceDef } from "../../../api"
import { ServiceActionsSection } from "../../../components/service-actions-section"
import { itsmQueryKeys } from "../../../query-keys"

export function Component() {
  const { t } = useTranslation(["itsm", "common"])
  const { id } = useParams<{ id: string }>()
  const navigate = useNavigate()
  const serviceId = Number(id)

  const { data: service } = useQuery({
    queryKey: itsmQueryKeys.services.detail(serviceId),
    queryFn: () => fetchServiceDef(serviceId),
    enabled: serviceId > 0,
  })

  return (
    <div className="workspace-page">
      <div className="workspace-page-header">
        <div className="flex min-w-0 items-center gap-3">
          <Button variant="ghost" size="icon-sm" onClick={() => navigate("/itsm/services")}>
            <ArrowLeft className="h-4 w-4" />
          </Button>
          <div className="min-w-0">
            <h2 className="workspace-page-title">{t("itsm:actions.title")}</h2>
            {service && <p className="workspace-page-description">{service.name} ({service.code})</p>}
          </div>
        </div>
      </div>

      <ServiceActionsSection
        serviceId={serviceId}
        showHeader
        title={(
          <span className="inline-flex items-center gap-2">
            <Zap className="h-4 w-4" />
            {t("itsm:services.tabActions")}
          </span>
        )}
      />
    </div>
  )
}
