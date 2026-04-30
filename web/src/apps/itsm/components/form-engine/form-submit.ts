import { buildZodSchema } from "./build-zod-schema"
import type { FormSchema } from "./types"

export type FormSubmitValidationResult =
  | { success: true; data: Record<string, unknown> }
  | { success: false; errors: Array<{ field: string; message: string }> }

export function validateVisibleFormData(
  schema: FormSchema,
  visibleFields: Set<string>,
  values: Record<string, unknown>,
): FormSubmitValidationResult {
  const result = buildZodSchema(schema, visibleFields).safeParse(values)
  if (!result.success) {
    return {
      success: false,
      errors: result.error.issues
        .map((issue) => ({
          field: String(issue.path[0] ?? ""),
          message: issue.message,
        }))
        .filter((error) => error.field),
    }
  }

  const data: Record<string, unknown> = {}
  for (const key of Object.keys(result.data as Record<string, unknown>)) {
    if (visibleFields.has(key)) {
      data[key] = (result.data as Record<string, unknown>)[key]
    }
  }
  return { success: true, data }
}
