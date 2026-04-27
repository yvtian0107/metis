import { z, type ZodTypeAny } from "zod"
import type { FormSchema, FormField, TableColumn } from "./types"

/**
 * Dynamically build a Zod validation schema from a FormSchema definition.
 * Hidden fields (not in visibleFields) are marked optional.
 */
export function buildZodSchema(
  schema: FormSchema,
  visibleFields: Set<string>,
): z.ZodObject<Record<string, ZodTypeAny>> {
  const shape: Record<string, ZodTypeAny> = {}

  for (const field of schema.fields) {
    const isVisible = visibleFields.has(field.key)
    shape[field.key] = isVisible
      ? buildFieldSchema(field)
      : z.any().optional()
  }

  return z.object(shape)
}

function buildFieldSchema(field: FormField): ZodTypeAny {
  const isNumeric = field.type === "number"
  const isBool = field.type === "switch" || (field.type === "checkbox" && (!field.options || field.options.length === 0))
  const isArray = field.type === "multi_select" || (field.type === "checkbox" && field.options && field.options.length > 0)

  let schema: ZodTypeAny

  if (isBool) {
    schema = z.boolean({ error: "请选择" })
    return isRequired(field) ? schema : schema.optional()
  }

  if (isArray) {
    let arrSchema = z.array(z.string())
    arrSchema = arrSchema.refine((values) => values.every((value) => optionValues(field).has(value)), `${field.label} 包含非法选项`)
    if (field.required) {
      arrSchema = arrSchema.min(1, requiredMessage(field))
    }
    return arrSchema
  }

  if (field.type === "date_range") {
    const rangeSchema = z.object({
      start: z.string().min(1, requiredMessage(field)),
      end: z.string().min(1, requiredMessage(field)),
    })
    if (!isRequired(field)) {
      return rangeSchema.optional().or(z.object({ start: z.literal(""), end: z.literal("") }))
    }
    return rangeSchema
  }

  if (field.type === "table") {
    let tableSchema = z.array(z.object(buildTableShape(tableColumns(field))))
    if (isRequired(field)) {
      tableSchema = tableSchema.min(1, requiredMessage(field))
    }
    return tableSchema
  }

  if (isNumeric) {
    schema = buildNumberSchema(field)
  } else {
    schema = buildStringSchema(field)
  }

  return schema
}

function buildStringSchema(field: FormField): ZodTypeAny {
  let s = z.string()

  if (isRequired(field)) {
    s = s.min(1, requiredMessage(field))
  }

  if (field.validation) {
    for (const rule of field.validation) {
      switch (rule.rule) {
        case "minLength":
          s = s.min(Number(rule.value), rule.message)
          break
        case "maxLength":
          s = s.max(Number(rule.value), rule.message)
          break
        case "pattern":
          s = s.regex(new RegExp(String(rule.value)), rule.message)
          break
        case "email":
          s = s.email(rule.message)
          break
        case "url":
          s = s.url(rule.message).refine((value) => value.startsWith("http://") || value.startsWith("https://"), rule.message)
          break
      }
    }
  }

  if (field.type === "select" || field.type === "radio") {
    s = s.refine((value) => value === "" || optionValues(field).has(value), `${field.label} 包含非法选项`)
  }

  if (!isRequired(field)) {
    return s.optional().or(z.literal(""))
  }

  return s
}

function buildNumberSchema(field: FormField): ZodTypeAny {
  let n = z.number({ error: "请输入数字" })

  if (field.validation) {
    for (const rule of field.validation) {
      switch (rule.rule) {
        case "min":
          n = n.min(Number(rule.value), rule.message)
          break
        case "max":
          n = n.max(Number(rule.value), rule.message)
          break
      }
    }
  }

  if (!isRequired(field)) {
    return n.optional()
  }
  return n
}

function buildTableShape(columns: TableColumn[]): Record<string, ZodTypeAny> {
  const shape: Record<string, ZodTypeAny> = {}
  for (const column of columns) {
    shape[column.key] = buildFieldSchema({
      key: column.key,
      type: column.type,
      label: column.label,
      placeholder: column.placeholder,
      required: column.required,
      validation: column.validation,
      options: column.options,
    })
  }
  return shape
}

function requiredMessage(field: FormField): string {
  const rule = field.validation?.find((r) => r.rule === "required")
  return rule?.message ?? "此字段为必填项"
}

function isRequired(field: Pick<FormField, "required" | "validation">): boolean {
  return !!field.required || !!field.validation?.some((r) => r.rule === "required")
}

function optionValues(field: Pick<FormField, "options">): Set<string> {
  return new Set((field.options ?? []).map((option) => String(option.value)))
}

export function tableColumns(field: Pick<FormField, "props">): TableColumn[] {
  const raw = field.props?.columns
  return Array.isArray(raw) ? raw as TableColumn[] : []
}

export function defaultValueForField(field: FormField): unknown {
  if (field.defaultValue !== undefined) return field.defaultValue
  if (field.type === "multi_select" || (field.type === "checkbox" && field.options && field.options.length > 0)) return []
  if (field.type === "date_range") return { start: "", end: "" }
  if (field.type === "table") return []
  if (field.type === "switch" || field.type === "checkbox") return false
  if (field.type === "number") return undefined
  return ""
}
