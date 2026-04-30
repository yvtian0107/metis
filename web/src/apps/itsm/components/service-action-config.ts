export type ServiceActionConfigParseResult =
  | { ok: true; value: unknown | null }
  | { ok: false; message: string }

export function parseServiceActionConfigInput(input?: string): ServiceActionConfigParseResult {
  const trimmed = input?.trim() ?? ""
  if (!trimmed) return { ok: true, value: null }
  try {
    return { ok: true, value: JSON.parse(trimmed) as unknown }
  } catch {
    return { ok: false, message: "JSON 格式不正确" }
  }
}
