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
    <div className="flex h-[calc(100vh-120px)] flex-col">
      <div className="flex items-center gap-3 border-b px-4 py-2">
        <Button variant="ghost" size="sm" onClick={() => navigate(-1)}>
          <ArrowLeft className="mr-1 h-3.5 w-3.5" />{t("workflow.back")}
        </Button>
        <h2 className="text-sm font-semibold">{t("workflow.editorTitle", { name: service?.name ?? "" })}</h2>
      </div>
      <div className="flex-1">
        <WorkflowEditor
          initialData={initialData}
          onSave={(data) => saveMut.mutate(data)}
          saving={saveMut.isPending}
        />
      </div>
    </div>
  )
}
