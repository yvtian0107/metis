import { useEffect, useMemo, useCallback } from "react"
import { useForm, Controller, useWatch } from "react-hook-form"
import { cn } from "@/lib/utils"
import { renderField } from "./field-renderers"
import { defaultValueForField } from "./build-zod-schema"
import { validateVisibleFormData } from "./form-submit"
import { useFieldVisibility } from "./use-visibility"
import type { FormRendererProps, FormField } from "./types"

/**
 * FormRenderer dynamically renders a form from a FormSchema definition.
 * Supports create/edit/view modes with per-node field permissions.
 */
export function FormRenderer({
  schema,
  data,
  mode,
  nodeId,
  onSubmit,
  onChange,
  disabled = false,
  footer,
}: FormRendererProps) {
  // Build default values from schema
  const defaultValues = useMemo(() => {
    const defaults: Record<string, unknown> = {}
    for (const field of schema.fields) {
      if (mode === "create") {
        defaults[field.key] = defaultValueForField(field)
      } else {
        defaults[field.key] = data?.[field.key] ?? defaultValueForField(field)
      }
    }
    return defaults
  }, [schema, data, mode])

  const form = useForm({
    defaultValues,
    resolver: undefined, // will set dynamically below
  })

  const watchValues = useWatch({ control: form.control }) as Record<string, unknown>

  // Compute field visibility
  const visibleFields = useFieldVisibility(schema, watchValues)

  // Reset form when data changes
  useEffect(() => {
    const vals: Record<string, unknown> = {}
    for (const field of schema.fields) {
      if (mode === "create") {
        vals[field.key] = defaultValueForField(field)
      } else {
        vals[field.key] = data?.[field.key] ?? defaultValueForField(field)
      }
    }
    form.reset(vals)
  }, [data, mode, schema, form])

  // onChange callback
  useEffect(() => {
    if (!onChange) return
    const values = form.getValues()
    const filtered: Record<string, unknown> = {}
    for (const key of Object.keys(values)) {
      if (visibleFields.has(key)) {
        filtered[key] = values[key]
      }
    }
    onChange(filtered)
  }, [form, onChange, visibleFields, watchValues])

  // Submit handler with manual Zod validation
  const handleSubmit = useCallback(async () => {
    const values = form.getValues()
    form.clearErrors()
    const result = validateVisibleFormData(schema, visibleFields, values)
    if (!result.success) {
      for (const error of result.errors) {
        form.setError(error.field, { message: error.message })
      }
      return
    }
    onSubmit?.(result.data)
  }, [form, schema, visibleFields, onSubmit])

  // Determine field permission for current node
  const getFieldState = (field: FormField) => {
    if (mode === "view") return "readonly" as const
    if (mode === "create") return "editable" as const
    // edit mode: check permissions
    if (nodeId && field.permissions?.[nodeId]) {
      return field.permissions[nodeId]
    }
    return "editable" as const
  }

  // Render fields in layout sections or flat list
  const renderFields = (fieldKeys: string[]) => {
    const columns = schema.layout?.columns ?? 1

    return (
      <div
        className={cn(
          "grid gap-4",
          columns === 2 && "grid-cols-2",
          columns === 3 && "grid-cols-3",
          columns === 1 && "grid-cols-1",
        )}
      >
        {fieldKeys.map((key) => {
          const field = schema.fields.find((f) => f.key === key)
          if (!field) return null
          if (!visibleFields.has(field.key)) return null

          const fieldState = getFieldState(field)
          if (fieldState === "hidden") return null

          const isReadOnly = fieldState === "readonly" || mode === "view"
          const isDisabled = disabled || field.disabled || false
          const colSpan = cn(
            columns === 2 && (field.width === "full" || !field.width) && "col-span-2",
            columns === 3 && (field.width === "full" || !field.width) && "col-span-3",
            columns === 3 && field.width === "half" && "col-span-2",
          )

          return (
            <div key={field.key} className={colSpan}>
              <Controller
                control={form.control}
                name={field.key}
                render={({ field: controllerField, fieldState: { error } }) => (
                  <div className="space-y-1.5" data-testid={`itsm-form-field-${field.key}`}>
                    <label className="text-sm font-medium leading-none">
                      {field.label}
                      {field.required && <span className="text-destructive ml-0.5">*</span>}
                    </label>
                    {renderField({
                      field,
                      value: controllerField.value,
                      onChange: controllerField.onChange,
                      onBlur: controllerField.onBlur,
                      disabled: isDisabled,
                      readOnly: isReadOnly,
                    })}
                    {field.description && !error && (
                      <p className="text-xs text-muted-foreground">{field.description}</p>
                    )}
                    {error && (
                      <p className="text-xs text-destructive">{error.message}</p>
                    )}
                  </div>
                )}
              />
            </div>
          )
        })}
      </div>
    )
  }

  const content = schema.layout?.sections && schema.layout.sections.length > 0 ? (
    <div className="space-y-6">
      {schema.layout.sections.map((section, i) => (
        <fieldset key={i} className="space-y-4">
          <div>
            <legend className="text-base font-semibold">{section.title}</legend>
            {section.description && (
              <p className="text-sm text-muted-foreground">{section.description}</p>
            )}
          </div>
          {renderFields(section.fields)}
        </fieldset>
      ))}
    </div>
  ) : (
    renderFields(schema.fields.map((f) => f.key))
  )

  if (mode === "view") {
    return <div className="space-y-4">{content}</div>
  }

  return (
    <form
      onSubmit={(e) => {
        e.preventDefault()
        handleSubmit()
      }}
      className="space-y-4"
    >
      {content}
      {footer}
    </form>
  )
}
