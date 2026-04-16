import { z, type ZodTypeAny } from "zod"
import type { FormSchema, FormField } from "./types"

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
    schema = z.boolean().optional()
    return schema
  }

  if (isArray) {
    let arrSchema = z.array(z.string())
    if (field.required) {
      arrSchema = arrSchema.min(1, requiredMessage(field))
    }
    return arrSchema
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

  const isRequired = field.required || field.validation?.some((r) => r.rule === "required")
  if (isRequired) {
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
          s = s.url(rule.message)
          break
      }
    }
  }

  if (!isRequired) {
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

  const isRequired = field.required || field.validation?.some((r) => r.rule === "required")
  if (!isRequired) {
    return n.optional()
  }
  return n
}

function requiredMessage(field: FormField): string {
  const rule = field.validation?.find((r) => r.rule === "required")
  return rule?.message ?? "此字段为必填项"
}
