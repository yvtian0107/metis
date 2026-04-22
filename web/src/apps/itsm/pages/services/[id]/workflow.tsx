"use client"

import { useMemo } from "react"
import { useTranslation } from "react-i18next"
import { useParams, useNavigate } from "react-router"
import { useQuery, useMutation } from "@tanstack/react-query"
import { ArrowLeft } from "lucide-react"
import { toast } from "sonner"
import { Button } from "@/components/ui/button"
import { type Node, type Edge } from "@xyflow/react"
import { WorkflowEditor } from "../../../components/workflow"
import { fetchServiceDef, updateServiceDef } from "../../../api"

export function Component() {
  const { t } = useTranslation("itsm")
  const { id } = useParams<{ id: string }>()
  const navigate = useNavigate()
  const serviceId = Number(id)

  const { data: service, isLoading } = useQuery({
    queryKey: ["itsm-service", serviceId],
    queryFn: () => fetchServiceDef(serviceId),
    enabled: !!serviceId,
  })

  const initialData = useMemo(() => {
    if (!service?.workflowJson) return undefined
    try {
      const wf = typeof service.workflowJson === "string"
        ? JSON.parse(service.workflowJson)
        : service.workflowJson
      return { nodes: wf.nodes ?? [], edges: wf.edges ?? [] }
    } catch {
      return undefined
    }
  }, [service])

  const saveMut = useMutation({
    mutationFn: (data: { nodes: Node[]; edges: Edge[] }) =>
      updateServiceDef(serviceId, { workflowJson: data } as Record<string, unknown>),
    onSuccess: () => toast.success(t("workflow.saveSuccess")),
    onError: (err) => toast.error(err.message),
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

  return (
    <div className="flex h-[calc(100vh-120px)] flex-col overflow-hidden rounded-[1.15rem] border border-border/55 bg-background/60">
      <div className="flex min-h-14 items-center justify-between gap-3 border-b border-border/55 bg-white/52 px-4">
        <div className="flex min-w-0 items-center gap-3">
          <Button variant="ghost" size="icon-sm" onClick={() => navigate(-1)}>
            <ArrowLeft className="h-4 w-4" />
          </Button>
          <div className="min-w-0">
            <h2 className="truncate text-sm font-semibold">{t("workflow.editorTitle", { name: service?.name ?? "" })}</h2>
            <p className="text-xs text-muted-foreground">{t("workflow.editorSubtitle")}</p>
          </div>
        </div>
        <Button variant="outline" size="sm" onClick={() => navigate(`/itsm/services/${serviceId}`)}>
          {t("workflow.back")}
        </Button>
      </div>
      <div className="flex-1">
        <WorkflowEditor
          initialData={initialData}
          onSave={(data) => saveMut.mutate(data)}
          saving={saveMut.isPending}
          serviceId={serviceId}
        />
      </div>
    </div>
  )
}
