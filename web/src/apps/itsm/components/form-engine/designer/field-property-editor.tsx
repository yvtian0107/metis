import { useTranslation } from "react-i18next"
import { Plus, Trash2 } from "lucide-react"
import { Input } from "@/components/ui/input"
import { Button } from "@/components/ui/button"
import { Switch } from "@/components/ui/switch"
import { Label } from "@/components/ui/label"
import {
  Select, SelectContent, SelectItem, SelectTrigger, SelectValue,
} from "@/components/ui/select"
import type { FieldOption, FormField, FieldType, TableColumn, ValidationRule, VisibilityCondition } from "../types"

export interface WorkflowNodeRef {
  id: string
  label: string
}

interface FieldPropertyEditorProps {
  field: FormField
  allFields: FormField[]
  onChange: (updated: FormField) => void
  workflowNodes?: WorkflowNodeRef[]
}

const NEEDS_OPTIONS: FieldType[] = ["select", "multi_select", "radio", "checkbox"]
const TABLE_COLUMN_TYPES: TableColumn["type"][] = [
  "text", "textarea", "number", "email", "url", "select", "multi_select",
  "radio", "checkbox", "switch", "date", "datetime", "date_range",
  "user_picker", "dept_picker",
]

const VALIDATION_RULE_TYPES = [
  "required", "minLength", "maxLength", "min", "max", "pattern", "email", "url",
] as const

const OPERATORS = [
  "equals", "not_equals", "in", "not_in", "is_empty", "is_not_empty",
] as const

function formatColumnOptions(options?: FieldOption[]) {
  return (options ?? []).map((option) => `${option.label}:${String(option.value)}`).join(", ")
}

function parseColumnOptions(input: string): FieldOption[] {
  return input
    .split(",")
    .map((item) => item.trim())
    .filter(Boolean)
    .map((item) => {
      const [label, ...rest] = item.split(":")
      const value = rest.join(":").trim() || label.trim()
      return { label: label.trim(), value }
    })
    .filter((option) => option.label && String(option.value))
}

export function FieldPropertyEditor({ field, allFields, onChange, workflowNodes }: FieldPropertyEditorProps) {
  const { t } = useTranslation("itsm")

  function update(patch: Partial<FormField>) {
    onChange({ ...field, ...patch })
  }

  const columns = Array.isArray(field.props?.columns) ? field.props.columns as TableColumn[] : []

  function updateColumns(next: TableColumn[]) {
    update({ props: { ...(field.props ?? {}), columns: next } })
  }

  return (
    <div className="space-y-5">
      {/* Basic Properties */}
      <section className="space-y-3 rounded-xl border border-border/60 bg-white/70 p-3">
        <div>
          <Label className="text-xs">{t("forms.fieldKey")}</Label>
          <Input
            className="mt-1 h-9 text-sm"
            value={field.key}
            onChange={(e) => update({ key: e.target.value })}
          />
        </div>
        <div>
          <Label className="text-xs">{t("forms.fieldLabel")}</Label>
          <Input
            className="mt-1 h-9 text-sm"
            value={field.label}
            onChange={(e) => update({ label: e.target.value })}
          />
        </div>
        <div>
          <Label className="text-xs">{t("forms.fieldType")}</Label>
          <Input className="mt-1 h-9 text-sm" value={t(`forms.type.${field.type}`)} disabled />
        </div>
        <div>
          <Label className="text-xs">{t("forms.fieldPlaceholder")}</Label>
          <Input
            className="mt-1 h-9 text-sm"
            value={field.placeholder ?? ""}
            onChange={(e) => update({ placeholder: e.target.value || undefined })}
          />
        </div>
        <div>
          <Label className="text-xs">{t("forms.fieldDescription")}</Label>
          <Input
            className="mt-1 h-9 text-sm"
            value={field.description ?? ""}
            onChange={(e) => update({ description: e.target.value || undefined })}
          />
        </div>
        <div className="flex items-center justify-between">
          <Label className="text-xs">{t("forms.fieldRequired")}</Label>
          <Switch
            checked={!!field.required}
            onCheckedChange={(v) => update({ required: v })}
          />
        </div>
        <div>
          <Label className="text-xs">{t("forms.fieldWidth")}</Label>
          <Select value={field.width ?? "full"} onValueChange={(v) => update({ width: v as "full" | "half" | "third" })}>
            <SelectTrigger className="mt-1 h-9 text-sm"><SelectValue /></SelectTrigger>
            <SelectContent>
              <SelectItem value="full">{t("forms.fieldWidthFull")}</SelectItem>
              <SelectItem value="half">{t("forms.fieldWidthHalf")}</SelectItem>
              <SelectItem value="third">{t("forms.fieldWidthThird")}</SelectItem>
            </SelectContent>
          </Select>
        </div>
      </section>

      {/* Options (for select/multi_select/radio/checkbox) */}
      {NEEDS_OPTIONS.includes(field.type) && (
        <section className="space-y-2 rounded-xl border border-border/60 bg-white/70 p-3">
          <div className="flex items-center justify-between">
            <Label className="text-xs font-medium">{t("forms.fieldOptions")}</Label>
            <Button
              variant="ghost" size="sm" className="h-6 px-2 text-xs"
              onClick={() => {
                const opts = [...(field.options ?? []), { label: "", value: `opt_${Date.now()}` }]
                update({ options: opts })
              }}
            >
              <Plus className="h-3 w-3 mr-1" />{t("forms.addOption")}
            </Button>
          </div>
          {(field.options ?? []).map((opt, i) => (
            <div key={i} className="flex items-center gap-1.5">
              <Input
                className="flex-1 h-7 text-xs"
                placeholder={t("forms.optionLabel")}
                value={opt.label}
                onChange={(e) => {
                  const opts = [...(field.options ?? [])]
                  opts[i] = { ...opts[i], label: e.target.value }
                  update({ options: opts })
                }}
              />
              <Input
                className="w-24 h-7 text-xs"
                placeholder={t("forms.optionValue")}
                value={String(opt.value)}
                onChange={(e) => {
                  const opts = [...(field.options ?? [])]
                  opts[i] = { ...opts[i], value: e.target.value }
                  update({ options: opts })
                }}
              />
              <Button
                variant="ghost" size="icon" className="h-6 w-6 shrink-0 text-destructive"
                onClick={() => {
                  const opts = (field.options ?? []).filter((_, j) => j !== i)
                  update({ options: opts })
                }}
              >
                <Trash2 className="h-3 w-3" />
              </Button>
            </div>
          ))}
        </section>
      )}

      {field.type === "table" && (
        <section className="space-y-2 rounded-xl border border-border/60 bg-white/70 p-3">
          <div className="flex items-center justify-between">
            <Label className="text-xs font-medium">表格列</Label>
            <Button
              variant="ghost" size="sm" className="h-6 px-2 text-xs"
              onClick={() => {
                const key = `col_${Date.now()}`
                updateColumns([...columns, { key, label: "新列", type: "text", required: false }])
              }}
            >
              <Plus className="h-3 w-3 mr-1" />添加列
            </Button>
          </div>
          {columns.map((column, i) => (
            <div key={`${column.key}-${i}`} className="space-y-1.5 rounded-md border border-border/55 p-2">
              <div className="flex items-center gap-1.5">
                <Input
                  className="h-7 flex-1 text-xs"
                  placeholder="key"
                  value={column.key}
                  onChange={(e) => {
                    const next = [...columns]
                    next[i] = { ...column, key: e.target.value }
                    updateColumns(next)
                  }}
                />
                <Input
                  className="h-7 flex-1 text-xs"
                  placeholder="label"
                  value={column.label}
                  onChange={(e) => {
                    const next = [...columns]
                    next[i] = { ...column, label: e.target.value }
                    updateColumns(next)
                  }}
                />
                <Button
                  variant="ghost" size="icon" className="h-6 w-6 shrink-0 text-destructive"
                  onClick={() => updateColumns(columns.filter((_, j) => j !== i))}
                >
                  <Trash2 className="h-3 w-3" />
                </Button>
              </div>
              <div className="flex items-center gap-1.5">
                <Select
                  value={column.type}
                  onValueChange={(v) => {
                    const next = [...columns]
                    next[i] = { ...column, type: v as TableColumn["type"] }
                    updateColumns(next)
                  }}
                >
                  <SelectTrigger className="h-7 flex-1 text-xs"><SelectValue /></SelectTrigger>
                  <SelectContent>
                    {TABLE_COLUMN_TYPES.map((type) => (
                      <SelectItem key={type} value={type}>{t(`forms.type.${type}`)}</SelectItem>
                    ))}
                  </SelectContent>
                </Select>
                <Label className="flex items-center gap-1.5 text-xs">
                  <Switch
                    checked={!!column.required}
                    onCheckedChange={(checked) => {
                      const next = [...columns]
                      next[i] = { ...column, required: checked }
                      updateColumns(next)
                    }}
                  />
                  必填
                </Label>
              </div>
              {NEEDS_OPTIONS.includes(column.type) && (
                <Input
                  className="h-7 text-xs"
                  placeholder="选项，格式：显示名:value, 显示名:value"
                  value={formatColumnOptions(column.options)}
                  onChange={(e) => {
                    const next = [...columns]
                    next[i] = { ...column, options: parseColumnOptions(e.target.value) }
                    updateColumns(next)
                  }}
                />
              )}
            </div>
          ))}
        </section>
      )}

      {/* Validation Rules */}
      <section className="space-y-2 rounded-xl border border-border/60 bg-white/70 p-3">
        <div className="flex items-center justify-between">
          <Label className="text-xs font-medium">{t("forms.validationRules")}</Label>
          <Button
            variant="ghost" size="sm" className="h-6 px-2 text-xs"
            onClick={() => {
              const rules: ValidationRule[] = [...(field.validation ?? []), { rule: "required", message: "" }]
              update({ validation: rules })
            }}
          >
            <Plus className="h-3 w-3 mr-1" />{t("forms.addRule")}
          </Button>
        </div>
        {(field.validation ?? []).map((rule, i) => (
          <div key={i} className="flex items-center gap-1.5">
            <Select
              value={rule.rule}
              onValueChange={(v) => {
                const rules = [...(field.validation ?? [])]
                rules[i] = { ...rules[i], rule: v as ValidationRule["rule"] }
                update({ validation: rules })
              }}
            >
              <SelectTrigger className="w-28 h-7 text-xs"><SelectValue /></SelectTrigger>
              <SelectContent>
                {VALIDATION_RULE_TYPES.map((r) => (
                  <SelectItem key={r} value={r}>{r}</SelectItem>
                ))}
              </SelectContent>
            </Select>
            {!["required", "email", "url"].includes(rule.rule) && (
              <Input
                className="w-16 h-7 text-xs"
                placeholder="value"
                value={rule.value !== undefined ? String(rule.value) : ""}
                onChange={(e) => {
                  const rules = [...(field.validation ?? [])]
                  rules[i] = { ...rules[i], value: e.target.value }
                  update({ validation: rules })
                }}
              />
            )}
            <Input
              className="flex-1 h-7 text-xs"
              placeholder="message"
              value={rule.message}
              onChange={(e) => {
                const rules = [...(field.validation ?? [])]
                rules[i] = { ...rules[i], message: e.target.value }
                update({ validation: rules })
              }}
            />
            <Button
              variant="ghost" size="icon" className="h-6 w-6 shrink-0 text-destructive"
              onClick={() => {
                const rules = (field.validation ?? []).filter((_, j) => j !== i)
                update({ validation: rules })
              }}
            >
              <Trash2 className="h-3 w-3" />
            </Button>
          </div>
        ))}
      </section>

      {/* Visibility Conditions */}
      <section className="space-y-2 rounded-xl border border-border/60 bg-white/70 p-3">
        <div className="flex items-center justify-between">
          <Label className="text-xs font-medium">{t("forms.visibilityConditions")}</Label>
          <Button
            variant="ghost" size="sm" className="h-6 px-2 text-xs"
            onClick={() => {
              const conds: VisibilityCondition[] = [
                ...(field.visibility?.conditions ?? []),
                { field: "", operator: "equals", value: "" },
              ]
              update({
                visibility: {
                  conditions: conds,
                  logic: field.visibility?.logic ?? "and",
                },
              })
            }}
          >
            <Plus className="h-3 w-3 mr-1" />{t("forms.addCondition")}
          </Button>
        </div>
        {(field.visibility?.conditions ?? []).length > 1 && (
          <div className="flex items-center gap-2">
            <Label className="text-xs">{t("forms.conditionLogic")}</Label>
            <Select
              value={field.visibility?.logic ?? "and"}
              onValueChange={(v) => update({
                visibility: {
                  ...field.visibility!,
                  logic: v as "and" | "or",
                },
              })}
            >
              <SelectTrigger className="w-20 h-7 text-xs"><SelectValue /></SelectTrigger>
              <SelectContent>
                <SelectItem value="and">AND</SelectItem>
                <SelectItem value="or">OR</SelectItem>
              </SelectContent>
            </Select>
          </div>
        )}
        {(field.visibility?.conditions ?? []).map((cond, i) => (
          <div key={i} className="flex items-center gap-1.5">
            <Select
              value={cond.field}
              onValueChange={(v) => {
                const conds = [...(field.visibility?.conditions ?? [])]
                conds[i] = { ...conds[i], field: v }
                update({ visibility: { ...field.visibility!, conditions: conds } })
              }}
            >
              <SelectTrigger className="w-28 h-7 text-xs">
                <SelectValue placeholder={t("forms.conditionField")} />
              </SelectTrigger>
              <SelectContent>
                {allFields.filter((f) => f.key !== field.key).map((f) => (
                  <SelectItem key={f.key} value={f.key}>{f.label}</SelectItem>
                ))}
              </SelectContent>
            </Select>
            <Select
              value={cond.operator}
              onValueChange={(v) => {
                const conds = [...(field.visibility?.conditions ?? [])]
                conds[i] = { ...conds[i], operator: v as VisibilityCondition["operator"] }
                update({ visibility: { ...field.visibility!, conditions: conds } })
              }}
            >
              <SelectTrigger className="w-24 h-7 text-xs"><SelectValue /></SelectTrigger>
              <SelectContent>
                {OPERATORS.map((op) => (
                  <SelectItem key={op} value={op}>{op}</SelectItem>
                ))}
              </SelectContent>
            </Select>
            {!["is_empty", "is_not_empty"].includes(cond.operator) && (
              <Input
                className="flex-1 h-7 text-xs"
                placeholder={t("forms.conditionValue")}
                value={cond.value !== undefined ? String(cond.value) : ""}
                onChange={(e) => {
                  const conds = [...(field.visibility?.conditions ?? [])]
                  conds[i] = { ...conds[i], value: e.target.value }
                  update({ visibility: { ...field.visibility!, conditions: conds } })
                }}
              />
            )}
            <Button
              variant="ghost" size="icon" className="h-6 w-6 shrink-0 text-destructive"
              onClick={() => {
                const conds = (field.visibility?.conditions ?? []).filter((_, j) => j !== i)
                update({
                  visibility: conds.length > 0
                    ? { ...field.visibility!, conditions: conds }
                    : undefined,
                })
              }}
            >
              <Trash2 className="h-3 w-3" />
            </Button>
          </div>
        ))}
      </section>

      {/* Node Permissions — only shown when workflowNodes are provided */}
      {workflowNodes && workflowNodes.length > 0 && (
        <section className="space-y-2 rounded-xl border border-border/60 bg-white/70 p-3">
          <Label className="text-xs font-medium">{t("forms.nodePermissions", { defaultValue: "节点权限" })}</Label>
          <p className="text-[10px] text-muted-foreground">{t("forms.nodePermissionsHint", { defaultValue: "控制该字段在各流程节点的可见性和可编辑性" })}</p>
          {workflowNodes.map((wfNode) => {
            const perm = field.permissions?.[wfNode.id] ?? "editable"
            return (
              <div key={wfNode.id} className="flex items-center justify-between gap-2">
                <span className="min-w-0 truncate text-xs">{wfNode.label || wfNode.id}</span>
                <Select
                  value={perm}
                  onValueChange={(v) => {
                    const next: Record<string, "editable" | "readonly" | "hidden"> = { ...(field.permissions ?? {}) }
                    if (v === "editable") {
                      delete next[wfNode.id]
                    } else {
                      next[wfNode.id] = v as "readonly" | "hidden"
                    }
                    update({ permissions: Object.keys(next).length > 0 ? next : undefined })
                  }}
                >
                  <SelectTrigger className="w-28 h-7 text-xs"><SelectValue /></SelectTrigger>
                  <SelectContent>
                    <SelectItem value="editable">{t("forms.permEditable", { defaultValue: "可编辑" })}</SelectItem>
                    <SelectItem value="readonly">{t("forms.permReadonly", { defaultValue: "只读" })}</SelectItem>
                    <SelectItem value="hidden">{t("forms.permHidden", { defaultValue: "隐藏" })}</SelectItem>
                  </SelectContent>
                </Select>
              </div>
            )
          })}
        </section>
      )}
    </div>
  )
}
