export interface FieldDisplayMeta {
  label?: string
  type?: string
  props?: Record<string, unknown>
  valueLabels: Record<string, string>
}

export function compactValue(value: unknown) {
  if (value == null || value === "") return "—"
  if (typeof value === "string" || typeof value === "number" || typeof value === "boolean") return String(value)
  try {
    return JSON.stringify(value)
  } catch {
    return String(value)
  }
}

export function toRecord(value: unknown) {
  return value != null && typeof value === "object" && !Array.isArray(value)
    ? value as Record<string, unknown>
    : null
}

function asTrimmedString(value: unknown) {
  return typeof value === "string" ? value.trim() : ""
}

function hasCJK(value: string) {
  return /[\u3400-\u9fff]/.test(value)
}

function resolveI18nValue(
  t: (key: string, options?: Record<string, unknown>) => string,
  path: string,
) {
  const value = t(path, { defaultValue: "" })
  if (!value || value === path) return ""
  return value
}

export function parseFieldDisplayMeta(schema: unknown) {
  const root = toRecord(schema)
  const rawFields = Array.isArray(root?.fields) ? root.fields : []
  const meta: Record<string, FieldDisplayMeta> = {}
  for (const rawField of rawFields) {
    const field = toRecord(rawField)
    if (!field) continue
    const key = asTrimmedString(field.key)
    if (!key) continue
    const label = asTrimmedString(field.label)
    const type = asTrimmedString(field.type)
    const props = toRecord(field.props) ?? undefined
    const valueLabels: Record<string, string> = {}
    const rawOptions = Array.isArray(field.options) ? field.options : []
    for (const rawOption of rawOptions) {
      const option = toRecord(rawOption)
      if (option) {
        const optionLabel = asTrimmedString(option.label)
        const optionValue = option.value
        if (optionLabel && optionValue != null) valueLabels[String(optionValue)] = optionLabel
        continue
      }
      if (typeof rawOption === "string" || typeof rawOption === "number" || typeof rawOption === "boolean") {
        valueLabels[String(rawOption)] = String(rawOption)
      }
    }
    meta[key] = { label: label || undefined, type: type || undefined, props, valueLabels }
  }
  return meta
}

export function resolveFieldLabel(
  key: string,
  fieldMeta: Record<string, FieldDisplayMeta>,
  t: (key: string, options?: Record<string, unknown>) => string,
  locale: string,
) {
  const schemaLabel = fieldMeta[key]?.label || ""
  const i18nLabel = resolveI18nValue(t, `itsm:tickets.fieldLabels.${key}`)
  if (locale.startsWith("zh")) return schemaLabel || i18nLabel || key
  if (i18nLabel) return i18nLabel
  if (schemaLabel && !hasCJK(schemaLabel)) return schemaLabel
  return schemaLabel || key
}

function resolveFieldOptionLabel(
  fieldKey: string,
  rawValue: unknown,
  fieldMeta: Record<string, FieldDisplayMeta>,
  t: (key: string, options?: Record<string, unknown>) => string,
  locale: string,
) {
  const valueKey = String(rawValue)
  const schemaLabel = fieldMeta[fieldKey]?.valueLabels[valueKey] || ""
  const i18nLabel = resolveI18nValue(t, `itsm:tickets.fieldValueLabels.${fieldKey}.${valueKey}`)
  if (locale.startsWith("zh")) return schemaLabel || i18nLabel || valueKey
  if (i18nLabel) return i18nLabel
  if (schemaLabel && !hasCJK(schemaLabel)) return schemaLabel
  return schemaLabel || valueKey
}

function parseLooseDate(value?: string | null) {
  if (!value) return null
  const trimmed = value.trim()
  if (!trimmed) return null
  const plainMatch = trimmed.match(/^(\d{4})-(\d{2})-(\d{2})(?:[ T](\d{2}):(\d{2})(?::\d{2})?)?/)
  if (plainMatch) {
    return {
      year: Number(plainMatch[1]),
      month: Number(plainMatch[2]),
      day: Number(plainMatch[3]),
      hour: plainMatch[4] ? Number(plainMatch[4]) : null,
      minute: plainMatch[5] ? Number(plainMatch[5]) : null,
    }
  }
  const parsed = new Date(trimmed)
  if (Number.isNaN(parsed.getTime())) return null
  return {
    year: parsed.getFullYear(),
    month: parsed.getMonth() + 1,
    day: parsed.getDate(),
    hour: parsed.getHours(),
    minute: parsed.getMinutes(),
  }
}

function formatLooseDate(value?: string | null, includeTime?: boolean) {
  const parsed = parseLooseDate(value)
  if (!parsed) return ""
  const datePart = `${parsed.year}/${parsed.month}/${parsed.day}`
  if (!includeTime || parsed.hour == null || parsed.minute == null) return datePart
  return `${datePart} ${String(parsed.hour).padStart(2, "0")}:${String(parsed.minute).padStart(2, "0")}`
}

function rangeUsesDateTime(meta: FieldDisplayMeta | undefined, range: { start?: string; end?: string }) {
  return meta?.props?.withTime === true
    || meta?.props?.mode === "datetime"
    || /\d{2}:\d{2}/.test(range.start ?? "")
    || /\d{2}:\d{2}/.test(range.end ?? "")
}

function formatDateRange(rawValue: unknown, meta: FieldDisplayMeta | undefined) {
  const range = toRecord(rawValue)
  if (!range) return ""
  const start = asTrimmedString(range.start)
  const end = asTrimmedString(range.end)
  if (!start && !end) return ""
  const includeTime = rangeUsesDateTime(meta, { start, end })
  const startText = formatLooseDate(start, includeTime) || start
  const endText = formatLooseDate(end, includeTime) || end
  if (startText && endText) return `${startText} - ${endText}`
  return startText || endText
}

function summarizeRecord(record: Record<string, unknown>) {
  const entries = Object.entries(record).filter(([, value]) => value != null && value !== "")
  if (entries.length === 0) return compactValue(record)
  return entries
    .slice(0, 2)
    .map(([key, value]) => `${key}: ${compactValue(value)}`)
    .join(", ")
}

function formatTableValue(rawValue: unknown, locale: string) {
  if (!Array.isArray(rawValue)) return compactValue(rawValue)
  if (rawValue.length === 0) return locale.startsWith("zh") ? "0 条记录" : "0 rows"
  const summary = toRecord(rawValue[0])
  const countLabel = locale.startsWith("zh") ? `${rawValue.length} 条记录` : `${rawValue.length} rows`
  if (!summary) return countLabel
  return `${countLabel}: ${summarizeRecord(summary)}`
}

export function resolveFieldDisplayValue(
  fieldKey: string,
  rawValue: unknown,
  fieldMeta: Record<string, FieldDisplayMeta>,
  t: (key: string, options?: Record<string, unknown>) => string,
  locale: string,
) {
  const meta = fieldMeta[fieldKey]
  const isRangeObject = meta?.type === "date_range"
    || (toRecord(rawValue) != null
      && "start" in (rawValue as Record<string, unknown>)
      && "end" in (rawValue as Record<string, unknown>))

  if (isRangeObject) {
    const formatted = formatDateRange(rawValue, meta)
    if (formatted) return formatted
  }

  if (meta?.type === "date") {
    const formatted = formatLooseDate(typeof rawValue === "string" ? rawValue : null, false)
    if (formatted) return formatted
  }

  if (meta?.type === "datetime") {
    const formatted = formatLooseDate(typeof rawValue === "string" ? rawValue : null, true)
    if (formatted) return formatted
  }

  if (Array.isArray(rawValue)) {
    if (meta?.type === "table") return formatTableValue(rawValue, locale)
    const separator = locale.startsWith("zh") ? "、" : ", "
    return rawValue
      .map((item) => {
        if (typeof item === "string" || typeof item === "number" || typeof item === "boolean") {
          return resolveFieldOptionLabel(fieldKey, item, fieldMeta, t, locale)
        }
        return compactValue(item)
      })
      .join(separator)
  }

  if (typeof rawValue === "string" || typeof rawValue === "number" || typeof rawValue === "boolean") {
    return resolveFieldOptionLabel(fieldKey, rawValue, fieldMeta, t, locale)
  }

  return compactValue(rawValue)
}
