import { useMemo } from "react"
import type { FormSchema, VisibilityCondition } from "./types"

/**
 * Evaluates field visibility conditions in real-time.
 * Returns a Set of visible field keys.
 */
export function useFieldVisibility(
  schema: FormSchema,
  watchValues: Record<string, unknown>,
): Set<string> {
  return useMemo(() => getVisibleFields(schema, watchValues), [schema, watchValues])
}

export function getVisibleFields(
  schema: FormSchema,
  values: Record<string, unknown>,
): Set<string> {
  const visible = new Set<string>()

  for (const field of schema.fields) {
    if (!field.visibility || field.visibility.conditions.length === 0) {
      visible.add(field.key)
      continue
    }

    const { conditions, logic = "and" } = field.visibility
    const results = conditions.map((c) => evaluateCondition(c, values))

    const isVisible =
      logic === "or"
        ? results.some(Boolean)
        : results.every(Boolean)

    if (isVisible) {
      visible.add(field.key)
    }
  }

  return visible
}

function evaluateCondition(
  condition: VisibilityCondition,
  values: Record<string, unknown>,
): boolean {
  const fieldValue = values[condition.field]

  switch (condition.operator) {
    case "equals":
      return String(fieldValue) === String(condition.value)
    case "not_equals":
      return String(fieldValue) !== String(condition.value)
    case "in": {
      const arr = Array.isArray(condition.value) ? condition.value : []
      return arr.some((v) => String(v) === String(fieldValue))
    }
    case "not_in": {
      const arr = Array.isArray(condition.value) ? condition.value : []
      return !arr.some((v) => String(v) === String(fieldValue))
    }
    case "is_empty":
      return fieldValue === undefined || fieldValue === null || fieldValue === ""
    case "is_not_empty":
      return fieldValue !== undefined && fieldValue !== null && fieldValue !== ""
    default:
      return true
  }
}
