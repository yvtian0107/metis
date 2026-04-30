"use client"

import { useTranslation } from "react-i18next"
import { useParams, useNavigate } from "react-router"
import { useQuery } from "@tanstack/react-query"
import { ArrowLeft } from "lucide-react"
import { Button } from "@/components/ui/button"
import { ClassicWorkflowWorkbench } from "./classic-workflow-workbench"
import { fetchCatalogTree, fetchServiceDef, fetchSLATemplates } from "../../../api"
import { itsmQueryKeys } from "../../../query-keys"

export function Component() {
  const { t } = useTranslation("itsm")
  const { id } = useParams<{ id: string }>()
  const navigate = useNavigate()
  const serviceId = Number(id)

  const { data: service, isLoading } = useQuery({
    queryKey: itsmQueryKeys.services.detail(serviceId),
    queryFn: () => fetchServiceDef(serviceId),
    enabled: !!serviceId,
  })

  const { data: catalogs } = useQuery({
    queryKey: itsmQueryKeys.catalogs.tree(),
    queryFn: () => fetchCatalogTree(),
  })

  const { data: slaTemplates } = useQuery({
    queryKey: itsmQueryKeys.sla.all,
    queryFn: () => fetchSLATemplates(),
  })

  if (isLoading) {
    return <div className="flex h-96 items-center justify-center text-muted-foreground">Loading...</div>
  }

  if (service?.engineType !== "classic") {
    return (
      <div className="flex h-96 flex-col items-center justify-center gap-2 text-muted-foreground">
        <p>{t("workflow.notClassic")}</p>
        <Button variant="outline" size="sm" onClick={() => navigate(-1)}>
          <ArrowLeft className="mr-1.5 h-3.5 w-3.5" />{t("workflow.back")}
        </Button>
      </div>
    )
  }

  return <ClassicWorkflowWorkbench service={service} catalogs={catalogs ?? []} slaTemplates={slaTemplates ?? []} />
}
