import { useState, useCallback } from "react"
import { useTranslation } from "react-i18next"
import { ScrollArea } from "@/components/ui/scroll-area"
import type { FormSchema, FormField, FieldType } from "../types"
import type { WorkflowNodeRef } from "./field-property-editor"
import { FieldTypePalette } from "./field-palette"
import { DesignerCanvas } from "./designer-canvas"
import { FieldPropertyEditor } from "./field-property-editor"

interface FormDesignerProps {
  schema: FormSchema
  onChange: (schema: FormSchema) => void
  workflowNodes?: WorkflowNodeRef[]
}

function generateFieldKey(type: FieldType, existing: FormField[]): string {
  const keys = new Set(existing.map((f) => f.key))
  let i = 1
  let key = `${type}_${i}`
  while (keys.has(key)) {
    i++
    key = `${type}_${i}`
  }
  return key
}

export function FormDesigner({ schema, onChange, workflowNodes }: FormDesignerProps) {
  const { t } = useTranslation("itsm")
  const [selectedFieldKey, setSelectedFieldKey] = useState<string | null>(null)

  const fields = schema.fields
  const layout = schema.layout ?? null
  const columns = layout?.columns ?? 1
  const selectedField = selectedFieldKey
    ? fields.find((f) => f.key === selectedFieldKey) ?? null
    : null

  const updateFields = useCallback(
    (next: FormField[]) => {
      onChange({ ...schema, fields: next })
    },
    [schema, onChange],
  )

  const handleAddField = useCallback(
    (type: FieldType) => {
      const key = generateFieldKey(type, fields)
      const newField: FormField = {
        key,
        type,
        label: t(`forms.type.${type}`),
        required: false,
        width: columns === 1 ? "full" : columns === 2 ? "half" : "third",
      }
      if (type === "table") {
        newField.props = {
          columns: [
            { key: "name", label: "名称", type: "text", required: true },
            { key: "description", label: "说明", type: "text", required: false },
          ],
        }
      }
      const nextFields = [...fields, newField]
      const nextLayout = schema.layout?.sections.length
        ? {
            ...schema.layout,
            sections: schema.layout.sections.map((section, index) => index === 0
              ? { ...section, fields: [...section.fields, key] }
              : section),
          }
        : schema.layout
      onChange({ ...schema, fields: nextFields, layout: nextLayout })
      setSelectedFieldKey(key)
    },
    [columns, fields, schema, onChange, t],
  )

  const handleColumnsChange = useCallback(
    (nextColumns: 1 | 2 | 3) => {
      onChange({
        ...schema,
        layout: {
          columns: nextColumns,
          sections: schema.layout?.sections ?? [],
        },
      })
    },
    [schema, onChange],
  )

  const handleReorderFields = useCallback(
    (activeKey: string, overKey: string) => {
      const activeIndex = fields.findIndex((field) => field.key === activeKey)
      const overIndex = fields.findIndex((field) => field.key === overKey)
      if (activeIndex < 0 || overIndex < 0 || activeIndex === overIndex) return

      const next = [...fields]
      const [moved] = next.splice(activeIndex, 1)
      next.splice(overIndex, 0, moved)

      const nextLayout = schema.layout?.sections.length
        ? {
            ...schema.layout,
            sections: schema.layout.sections.map((section) => {
              const sectionActiveIndex = section.fields.indexOf(activeKey)
              const sectionOverIndex = section.fields.indexOf(overKey)
              if (sectionActiveIndex < 0 || sectionOverIndex < 0) return section
              const nextSectionFields = [...section.fields]
              const [movedField] = nextSectionFields.splice(sectionActiveIndex, 1)
              nextSectionFields.splice(sectionOverIndex, 0, movedField)
              return { ...section, fields: nextSectionFields }
            }),
          }
        : schema.layout

      onChange({ ...schema, fields: next, layout: nextLayout })
    },
    [fields, schema, onChange],
  )

  const handleDeleteField = useCallback(
    (key: string) => {
      updateFields(fields.filter((f) => f.key !== key))
      if (selectedFieldKey === key) setSelectedFieldKey(null)
    },
    [fields, updateFields, selectedFieldKey],
  )

  const handleMoveField = useCallback(
    (key: string, direction: "up" | "down") => {
      const idx = fields.findIndex((f) => f.key === key)
      if (idx < 0) return
      const target = direction === "up" ? idx - 1 : idx + 1
      if (target < 0 || target >= fields.length) return
      const next = [...fields]
      ;[next[idx], next[target]] = [next[target], next[idx]]
      updateFields(next)
    },
    [fields, updateFields],
  )

  const handleFieldChange = useCallback(
    (updated: FormField) => {
      updateFields(fields.map((f) => (f.key === updated.key ? updated : f)))
    },
    [fields, updateFields],
  )

  return (
    <div className="flex h-full min-h-[560px] overflow-hidden rounded-xl border border-border/60 bg-white/72 shadow-[0_24px_60px_-52px_rgba(15,23,42,0.45)]">
      <div className="flex w-[260px] shrink-0 flex-col border-r border-border/55 bg-white/70">
        <div className="flex h-11 shrink-0 items-center border-b border-border/55 px-4">
          <h3 className="text-xs font-semibold uppercase tracking-[0.16em] text-muted-foreground/75">
            {t("forms.fieldPalette")}
          </h3>
        </div>
        <ScrollArea className="min-h-0 flex-1">
          <div className="p-3">
            <FieldTypePalette onAddField={handleAddField} />
          </div>
        </ScrollArea>
      </div>

      <div className="flex min-w-0 flex-1 flex-col bg-slate-50/45">
        <div className="flex h-11 shrink-0 items-center justify-between gap-3 border-b border-border/55 px-4">
          <h3 className="text-xs font-semibold uppercase tracking-[0.16em] text-muted-foreground/75">
            {t("forms.canvas")}
          </h3>
          <div className="flex rounded-lg border border-border/65 bg-white/80 p-0.5">
            {([1, 2, 3] as const).map((count) => (
              <button
                key={count}
                type="button"
                onClick={() => handleColumnsChange(count)}
                className={`h-7 min-w-8 rounded-md px-2 text-xs font-medium transition-colors ${columns === count ? "bg-primary text-primary-foreground" : "text-muted-foreground hover:bg-muted/70 hover:text-foreground"}`}
              >
                {count}
              </button>
            ))}
          </div>
        </div>
        <ScrollArea className="min-h-0 flex-1">
          <div className="p-5">
            <DesignerCanvas
              fields={fields}
              layout={layout}
              layoutColumns={columns}
              selectedFieldKey={selectedFieldKey}
              onSelectField={setSelectedFieldKey}
              onAddField={handleAddField}
              onReorderFields={handleReorderFields}
              onDeleteField={handleDeleteField}
              onMoveField={handleMoveField}
            />
          </div>
        </ScrollArea>
      </div>

      <div className="flex w-[340px] shrink-0 flex-col border-l border-border/55 bg-white/76">
        <div className="flex h-11 shrink-0 items-center border-b border-border/55 px-4">
          <h3 className="text-xs font-semibold uppercase tracking-[0.16em] text-muted-foreground/75">
            {t("forms.properties")}
          </h3>
        </div>
        <ScrollArea className="min-h-0 flex-1">
          <div className="p-4">
            {selectedField ? (
              <FieldPropertyEditor
                field={selectedField}
                allFields={fields}
                onChange={handleFieldChange}
                workflowNodes={workflowNodes}
              />
            ) : (
              <p className="text-xs text-muted-foreground text-center py-8">
                {t("forms.selectFieldHint")}
              </p>
            )}
          </div>
        </ScrollArea>
      </div>
    </div>
  )
}
