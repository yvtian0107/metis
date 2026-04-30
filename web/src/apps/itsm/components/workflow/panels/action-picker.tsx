import { useTranslation } from "react-i18next"
import { useQuery } from "@tanstack/react-query"
import { Label } from "@/components/ui/label"
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select"
import { Button } from "@/components/ui/button"
import { X, Zap } from "lucide-react"
import { fetchServiceActions } from "../../../api"
import { itsmQueryKeys } from "../../../query-keys"

interface ActionPickerProps {
  serviceId: number
  actionId?: number
  onChange: (actionId: number | undefined) => void
}

export function ActionPicker({ serviceId, actionId, onChange }: ActionPickerProps) {
  const { t } = useTranslation("itsm")

  const { data: actions } = useQuery({
    queryKey: itsmQueryKeys.services.actions(serviceId),
    queryFn: () => fetchServiceActions(serviceId),
    enabled: serviceId > 0,
    staleTime: 60_000,
  })

  const selected = actions?.find((a) => a.id === actionId)
  const config = selected?.configJson as { url?: string; method?: string } | undefined

  return (
    <div className="space-y-2">
      <Label className="text-xs">{t("workflow.prop.serviceAction")}</Label>
      <div className="flex items-center gap-1">
        <Select value={actionId ? String(actionId) : ""} onValueChange={(v) => onChange(v ? Number(v) : undefined)}>
          <SelectTrigger className="h-8 text-xs">
            <SelectValue placeholder={t("workflow.prop.selectAction")} />
          </SelectTrigger>
          <SelectContent>
            {(actions ?? []).map((a) => (
              <SelectItem key={a.id} value={String(a.id)}>
                <div className="flex items-center gap-1.5">
                  <Zap size={12} />
                  <span>{a.name}</span>
                </div>
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
        {actionId && (
          <Button variant="ghost" size="icon" className="h-8 w-8 shrink-0" onClick={() => onChange(undefined)}>
            <X size={14} />
          </Button>
        )}
      </div>
      {config?.url && (
        <div className="rounded border p-1.5 text-[10px]">
          <span className="font-medium text-primary">{config.method ?? "GET"}</span>
          <span className="ml-1.5 text-muted-foreground break-all">{config.url}</span>
        </div>
      )}
    </div>
  )
}
