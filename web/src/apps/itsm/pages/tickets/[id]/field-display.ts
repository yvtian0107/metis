export interface FieldOptionLabelMap {
  [value: string]: string
}

export interface TableColumnMeta {
  key: string
  label?: string
  type?: string
  valueLabels: FieldOptionLabelMap
}

export interface FieldDisplayMeta {
  label?: string
  type?: string
  props?: Record<string, unknown>
  valueLabels: FieldOptionLabelMap
  tableColumns?: TableColumnMeta[]
}

export interface FieldDisplaySection {
  title?: string
  description?: string
  collapsible?: boolean
  fields: string[]
}

export interface FieldDisplayTextModel {
  kind: "text"
  value: string
}

export interface FieldDisplayTagsModel {
  kind: "tags"
  values: string[]
}

export interface FieldDisplayLongTextModel {
  kind: "long_text"
  value: string
  expandable: boolean
}

export interface FieldDisplayRangeModel {
  kind: "range"
  value: string
}

export interface FieldDisplayTableCell {
  key: string
  label: string
  value: string
}

export interface FieldDisplayTableRow {
  summary: string
  cells: FieldDisplayTableCell[]
}

export interface FieldDisplayTableModel {
  kind: "table"
  count: number
  summary: string
  summaryLabel: string
  columns: Array<{ key: string; label: string }>
  rows: FieldDisplayTableRow[]
}

export type FieldDisplayModel =
  | FieldDisplayTextModel
  | FieldDisplayTagsModel
  | FieldDisplayLongTextModel
  | FieldDisplayRangeModel
  | FieldDisplayTableModel

export function compactValue(value: unknown) {
  if (value == null || value === "") return "-"
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

function parseValueLabels(rawOptions: unknown) {
  const valueLabels: FieldOptionLabelMap = {}
  const options = Array.isArray(rawOptions) ? rawOptions : []
  for (const rawOption of options) {
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
  return valueLabels
}

function parseTableColumns(rawProps: Record<string, unknown> | undefined) {
  const rawColumns = Array.isArray(rawProps?.columns) ? rawProps.columns : []
  const columns: TableColumnMeta[] = []
  for (const rawColumn of rawColumns) {
    const column = toRecord(rawColumn)
    if (!column) continue
    const key = asTrimmedString(column.key)
    if (!key) continue
    columns.push({
      key,
      label: asTrimmedString(column.label) || undefined,
      type: asTrimmedString(column.type) || undefined,
      valueLabels: parseValueLabels(column.options),
    })
  }
  return columns
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
    meta[key] = {
      label: label || undefined,
      type: type || undefined,
      props,
      valueLabels: parseValueLabels(field.options),
      tableColumns: parseTableColumns(props),
    }
  }
  return meta
}

export function parseFieldDisplaySections(schema: unknown) {
  const root = toRecord(schema)
  const layout = toRecord(root?.layout)
  const rawSections = Array.isArray(layout?.sections) ? layout.sections : []
  const sections: FieldDisplaySection[] = []
  for (const rawSection of rawSections) {
    const section = toRecord(rawSection)
    if (!section) continue
    const fields = Array.isArray(section.fields)
      ? section.fields.map((field) => asTrimmedString(field)).filter(Boolean)
      : []
    if (fields.length === 0) continue
    sections.push({
      title: asTrimmedString(section.title) || undefined,
      description: asTrimmedString(section.description) || undefined,
      collapsible: section.collapsible === true,
      fields,
    })
  }
  return sections
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

function resolveValueLabel(
  valueLabels: FieldOptionLabelMap | undefined,
  rawValue: unknown,
  i18nKey: string,
  t: (key: string, options?: Record<string, unknown>) => string,
  locale: string,
) {
  const valueKey = String(rawValue)
  const schemaLabel = valueLabels?.[valueKey] || ""
  const i18nLabel = resolveI18nValue(t, i18nKey)
  if (locale.startsWith("zh")) return schemaLabel || i18nLabel || valueKey
  if (i18nLabel) return i18nLabel
  if (schemaLabel && !hasCJK(schemaLabel)) return schemaLabel
  return schemaLabel || valueKey
}

function resolveFieldOptionLabel(
  fieldKey: string,
  rawValue: unknown,
  fieldMeta: Record<string, FieldDisplayMeta>,
  t: (key: string, options?: Record<string, unknown>) => string,
  locale: string,
) {
  return resolveValueLabel(
    fieldMeta[fieldKey]?.valueLabels,
    rawValue,
    `itsm:tickets.fieldValueLabels.${fieldKey}.${String(rawValue)}`,
    t,
    locale,
  )
}

function resolveTableColumnLabel(
  column: TableColumnMeta,
  tableKey: string,
  t: (key: string, options?: Record<string, unknown>) => string,
  locale: string,
) {
  const schemaLabel = column.label || ""
  const i18nLabel = resolveI18nValue(t, `itsm:tickets.fieldLabels.${tableKey}.${column.key}`)
  if (locale.startsWith("zh")) return schemaLabel || i18nLabel || column.key
  if (i18nLabel) return i18nLabel
  if (schemaLabel && !hasCJK(schemaLabel)) return schemaLabel
  return schemaLabel || column.key
}

function resolveTableCellValue(
  tableKey: string,
  column: TableColumnMeta,
  rawValue: unknown,
  t: (key: string, options?: Record<string, unknown>) => string,
  locale: string,
) {
  const columnMeta: FieldDisplayMeta = {
    label: column.label,
    type: column.type,
    valueLabels: column.valueLabels,
  }
  return resolveFieldDisplayValueInternal(
    `${tableKey}.${column.key}`,
    rawValue,
    { [tableKey]: columnMeta },
    t,
    locale,
    columnMeta,
  )
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

const LONG_TEXT_EXPAND_THRESHOLD = 60

function looksLikeLongText(fieldKey: string, meta: FieldDisplayMeta | undefined, rawValue: unknown) {
  if (meta?.type === "textarea" || meta?.type === "rich_text") return true
  if (typeof rawValue !== "string") return false
  if (rawValue.length > 80) return true
  return /(remark|description|comment|note|reason|scope|summary|detail|impact|remark)/i.test(fieldKey)
}

function shouldExpandLongText(rawValue: unknown) {
  if (typeof rawValue !== "string") return false
  const normalized = rawValue.trim()
  if (!normalized) return false
  return normalized.length > LONG_TEXT_EXPAND_THRESHOLD || normalized.includes("\n")
}

function buildTableSummary(
  rows: FieldDisplayTableRow[],
  count: number,
  locale: string,
) {
  const countLabel = locale.startsWith("zh") ? `${count} 条记录` : `${count} rows`
  const previews = rows
    .slice(0, 2)
    .map((row) => row.summary)
    .filter(Boolean)
  if (previews.length === 0) return countLabel
  return previews.join(locale.startsWith("zh") ? "；" : "; ")
}

function buildTableRow(
  tableKey: string,
  rowValue: unknown,
  meta: FieldDisplayMeta | undefined,
  t: (key: string, options?: Record<string, unknown>) => string,
  locale: string,
) {
  const record = toRecord(rowValue)
  if (!record) {
    return {
      summary: compactValue(rowValue),
      cells: [],
    } satisfies FieldDisplayTableRow
  }

  const columns = meta?.tableColumns?.length
    ? meta.tableColumns
    : Object.keys(record).map((key) => ({
      key,
      label: key,
      valueLabels: {},
    }))

  const cells = columns.map((column) => ({
    key: column.key,
    label: resolveTableColumnLabel(column, tableKey, t, locale),
    value: resolveTableCellValue(tableKey, column, record[column.key], t, locale),
  }))

  const summary = cells
    .map((cell) => cell.value)
    .filter((value) => value && value !== "-")
    .slice(0, 3)
    .join(" / ") || "-"

  return { summary, cells }
}

function buildTableModel(
  fieldKey: string,
  rawValue: unknown,
  meta: FieldDisplayMeta | undefined,
  t: (key: string, options?: Record<string, unknown>) => string,
  locale: string,
) {
  if (!Array.isArray(rawValue)) {
    return {
      kind: "text",
      value: compactValue(rawValue),
    } satisfies FieldDisplayTextModel
  }

  const rows = rawValue.map((rowValue) => buildTableRow(fieldKey, rowValue, meta, t, locale))
  const columns = meta?.tableColumns?.length
    ? meta.tableColumns.map((column) => ({
      key: column.key,
      label: resolveTableColumnLabel(column, fieldKey, t, locale),
    }))
    : rows[0]?.cells.map((cell) => ({ key: cell.key, label: cell.label })) ?? []

  return {
    kind: "table",
    count: rawValue.length,
    summary: buildTableSummary(rows, rawValue.length, locale),
    summaryLabel: locale.startsWith("zh") ? `${rawValue.length} 条记录` : `${rawValue.length} rows`,
    columns,
    rows,
  } satisfies FieldDisplayTableModel
}

function resolveFieldDisplayValueInternal(
  fieldKey: string,
  rawValue: unknown,
  fieldMeta: Record<string, FieldDisplayMeta>,
  t: (key: string, options?: Record<string, unknown>) => string,
  locale: string,
  overrideMeta?: FieldDisplayMeta,
) {
  const meta = overrideMeta ?? fieldMeta[fieldKey]
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
    const separator = locale.startsWith("zh") ? "、" : ", "
    return rawValue
      .map((item) => {
        if (typeof item === "string" || typeof item === "number" || typeof item === "boolean") {
          return resolveValueLabel(meta?.valueLabels, item, `itsm:tickets.fieldValueLabels.${fieldKey}.${String(item)}`, t, locale)
        }
        return compactValue(item)
      })
      .join(separator)
  }

  if (typeof rawValue === "string" || typeof rawValue === "number" || typeof rawValue === "boolean") {
    return resolveValueLabel(meta?.valueLabels, rawValue, `itsm:tickets.fieldValueLabels.${fieldKey}.${String(rawValue)}`, t, locale)
  }

  return compactValue(rawValue)
}

export function resolveFieldDisplayValue(
  fieldKey: string,
  rawValue: unknown,
  fieldMeta: Record<string, FieldDisplayMeta>,
  t: (key: string, options?: Record<string, unknown>) => string,
  locale: string,
) {
  return resolveFieldDisplayValueInternal(fieldKey, rawValue, fieldMeta, t, locale)
}

export function resolveFieldDisplayModel(
  fieldKey: string,
  rawValue: unknown,
  fieldMeta: Record<string, FieldDisplayMeta>,
  t: (key: string, options?: Record<string, unknown>) => string,
  locale: string,
): FieldDisplayModel {
  const meta = fieldMeta[fieldKey]

  if (meta?.type === "table") {
    return buildTableModel(fieldKey, rawValue, meta, t, locale)
  }

  const isRangeObject = meta?.type === "date_range"
    || (toRecord(rawValue) != null
      && "start" in (rawValue as Record<string, unknown>)
      && "end" in (rawValue as Record<string, unknown>))
  if (isRangeObject) {
    return {
      kind: "range",
      value: resolveFieldDisplayValueInternal(fieldKey, rawValue, fieldMeta, t, locale),
    }
  }

  if (Array.isArray(rawValue)) {
    return {
      kind: "tags",
      values: rawValue.map((item) => {
        if (typeof item === "string" || typeof item === "number" || typeof item === "boolean") {
          return resolveFieldOptionLabel(fieldKey, item, fieldMeta, t, locale)
        }
        return compactValue(item)
      }),
    }
  }

  const value = resolveFieldDisplayValueInternal(fieldKey, rawValue, fieldMeta, t, locale)
  if (looksLikeLongText(fieldKey, meta, rawValue)) {
    return { kind: "long_text", value, expandable: shouldExpandLongText(rawValue) }
  }

  return { kind: "text", value }
}
