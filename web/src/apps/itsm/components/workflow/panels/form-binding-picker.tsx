import { useTranslation } from "react-i18next"
import { useQuery } from "@tanstack/react-query"
import { Label } from "@/components/ui/label"
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select"
import { Button } from "@/components/ui/button"
import { X, FileText } from "lucide-react"
import { fetchFormDefs } from "../../../api"

interface FormBindingPickerProps {
  formDefinitionId?: number
  onChange: (formDefId: number | undefined, schema: unknown) => void
}

interface FormField {
  key: string
  type: string
  label: string
}

export function FormBindingPicker({ formDefinitionId, onChange }: FormBindingPickerProps) {
  const { t } = useTranslation("itsm")

  const { data: forms } = useQuery({
    queryKey: ["itsm-forms-active"],
    queryFn: () => fetchFormDefs({ isActive: true, pageSize: 100 }),
    staleTime: 60_000,
    select: (d) => d.items,
  })

  const selectedForm = forms?.find((f) => f.id === formDefinitionId)
  const fields = parseFields(selectedForm?.schema)

  function handleSelect(val: string) {
    if (val === "__clear__") {
      onChange(undefined, undefined)
      return
    }
    const id = Number(val)
    const form = forms?.find((f) => f.id === id)
    onChange(id, form?.schema)
  }

  return (
    <div className="space-y-2">
      <Label className="text-xs">{t("workflow.prop.formBinding")}</Label>
      <div className="flex items-center gap-1">
        <Select value={formDefinitionId ? String(formDefinitionId) : ""} onValueChange={handleSelect}>
          <SelectTrigger className="h-8 text-xs">
            <SelectValue placeholder={t("workflow.prop.selectForm")} />
          </SelectTrigger>
          <SelectContent>
            {(forms ?? []).map((f) => (
              <SelectItem key={f.id} value={String(f.id)}>
                <div className="flex items-center gap-1.5">
                  <FileText size={12} />
                  <span>{f.name}</span>
                </div>
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
        {formDefinitionId && (
          <Button variant="ghost" size="icon" className="h-8 w-8 shrink-0" onClick={() => onChange(undefined, undefined)}>
            <X size={14} />
          </Button>
        )}
      </div>
      {fields.length > 0 && (
        <div className="rounded border p-1.5">
          <div className="text-[10px] font-medium text-muted-foreground mb-1">{t("workflow.prop.formFields")} ({fields.length})</div>
          <div className="space-y-0.5">
            {fields.slice(0, 6).map((f) => (
              <div key={f.key} className="flex items-center justify-between text-[10px]">
                <span>{f.label || f.key}</span>
                <span className="text-muted-foreground">{f.type}</span>
              </div>
            ))}
            {fields.length > 6 && (
              <div className="text-[10px] text-muted-foreground">+{fields.length - 6} more</div>
            )}
          </div>
        </div>
      )}
    </div>
  )
}

function parseFields(schema: unknown): FormField[] {
  if (!schema || typeof schema !== "object") return []
  const s = schema as { fields?: FormField[] }
  return Array.isArray(s.fields) ? s.fields : []
}
