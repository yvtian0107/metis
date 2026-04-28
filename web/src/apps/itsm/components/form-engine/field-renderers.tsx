import { Input } from "@/components/ui/input"
import { Textarea } from "@/components/ui/textarea"
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select"
import { Checkbox } from "@/components/ui/checkbox"
import { Switch } from "@/components/ui/switch"
import { Button } from "@/components/ui/button"
import { Badge } from "@/components/ui/badge"
import {
  Popover,
  PopoverContent,
  PopoverTrigger,
} from "@/components/ui/popover"
import {
  Command,
  CommandEmpty,
  CommandGroup,
  CommandInput,
  CommandItem,
  CommandList,
} from "@/components/ui/command"
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table"
import { Check, ChevronsUpDown, Plus, Trash2 } from "lucide-react"
import type { ControllerRenderProps } from "react-hook-form"
import { cn } from "@/lib/utils"
import { defaultValueForField, tableColumns } from "./build-zod-schema"
import type { FormField, TableColumn } from "./types"
import { UserPicker } from "./user-picker"
import { DeptPicker } from "./dept-picker"

type FieldProps = {
  field: FormField
  value: unknown
  onChange: ControllerRenderProps["onChange"]
  onBlur: ControllerRenderProps["onBlur"]
  disabled: boolean
  readOnly: boolean
}

function renderText({ field, value, onChange, onBlur, disabled, readOnly }: FieldProps) {
  return (
    <Input
      type={field.type === "email" ? "email" : field.type === "url" ? "url" : "text"}
      placeholder={field.placeholder}
      value={(value as string) ?? ""}
      onChange={(e) => onChange(e.target.value)}
      onBlur={onBlur}
      disabled={disabled}
      readOnly={readOnly}
      maxLength={field.props?.maxLength as number | undefined}
    />
  )
}

function renderTextarea({ field, value, onChange, onBlur, disabled, readOnly }: FieldProps) {
  return (
    <Textarea
      placeholder={field.placeholder}
      value={(value as string) ?? ""}
      onChange={(e) => onChange(e.target.value)}
      onBlur={onBlur}
      disabled={disabled}
      readOnly={readOnly}
      rows={(field.props?.rows as number) ?? 3}
    />
  )
}

function renderNumber({ field, value, onChange, onBlur, disabled, readOnly }: FieldProps) {
  return (
    <Input
      type="number"
      placeholder={field.placeholder}
      value={value === undefined || value === null ? "" : String(value)}
      onChange={(e) => {
        const v = e.target.value
        onChange(v === "" ? undefined : Number(v))
      }}
      onBlur={onBlur}
      disabled={disabled}
      readOnly={readOnly}
      min={field.props?.min as number | undefined}
      max={field.props?.max as number | undefined}
      step={field.props?.step as number | undefined}
    />
  )
}

function renderSelect({ field, value, onChange, disabled, readOnly }: FieldProps) {
  if (readOnly) {
    const label = field.options?.find((o) => String(o.value) === String(value))?.label
    return <Input value={label ?? String(value ?? "")} readOnly disabled />
  }
  return (
    <Select
      value={value === undefined || value === null ? "" : String(value)}
      onValueChange={onChange}
      disabled={disabled}
    >
      <SelectTrigger>
        <SelectValue placeholder={field.placeholder ?? "请选择"} />
      </SelectTrigger>
      <SelectContent>
        {field.options?.map((opt) => (
          <SelectItem key={String(opt.value)} value={String(opt.value)}>
            {opt.label}
          </SelectItem>
        ))}
      </SelectContent>
    </Select>
  )
}

function renderMultiSelect({ field, value, onChange, disabled, readOnly }: FieldProps) {
  const selected = Array.isArray(value) ? (value as string[]) : []
  const selectedLabels = selected
    .map((v) => field.options?.find((o) => String(o.value) === v)?.label ?? v)

  if (readOnly) {
    return <Input value={selectedLabels.join(", ")} readOnly disabled />
  }

  return (
    <Popover>
      <PopoverTrigger asChild>
        <Button
          type="button"
          variant="outline"
          role="combobox"
          disabled={disabled}
          className="h-auto min-h-9 w-full justify-between px-3 py-1.5 font-normal"
        >
          <span className="flex min-w-0 flex-1 flex-wrap gap-1">
            {selectedLabels.length === 0 ? (
              <span className="text-muted-foreground">{field.placeholder ?? "请选择"}</span>
            ) : selectedLabels.length <= 3 ? (
              selectedLabels.map((label) => <Badge key={label} variant="secondary">{label}</Badge>)
            ) : (
              <span>{`已选择 ${selectedLabels.length} 项`}</span>
            )}
          </span>
          <ChevronsUpDown className="ml-2 size-4 shrink-0 opacity-50" />
        </Button>
      </PopoverTrigger>
      <PopoverContent className="w-[--radix-popover-trigger-width] p-0" align="start">
        <Command>
          <CommandInput placeholder="搜索选项" />
          <CommandList>
            <CommandEmpty>没有可选项</CommandEmpty>
            <CommandGroup>
              {field.options?.map((opt) => {
                const optionValue = String(opt.value)
                const checked = selected.includes(optionValue)
                return (
                  <CommandItem
                    key={optionValue}
                    value={`${opt.label} ${optionValue}`}
                    onSelect={() => {
                      const next = checked
                        ? selected.filter((v) => v !== optionValue)
                        : [...selected, optionValue]
                      onChange(next)
                    }}
                  >
                    <Check className={cn("size-4", checked ? "opacity-100" : "opacity-0")} />
                    <span>{opt.label}</span>
                  </CommandItem>
                )
              })}
            </CommandGroup>
          </CommandList>
        </Command>
      </PopoverContent>
    </Popover>
  )
}

function renderRadio({ field, value, onChange, disabled, readOnly }: FieldProps) {
  if (readOnly) {
    const label = field.options?.find((o) => String(o.value) === String(value))?.label
    return <Input value={label ?? String(value ?? "")} readOnly disabled />
  }
  return (
    <div className="flex flex-wrap gap-3">
      {field.options?.map((opt) => (
        <label key={String(opt.value)} className="flex items-center gap-1.5 text-sm cursor-pointer">
          <input
            type="radio"
            name={field.key}
            value={String(opt.value)}
            checked={String(value) === String(opt.value)}
            onChange={() => onChange(String(opt.value))}
            disabled={disabled}
            className="accent-primary"
          />
          {opt.label}
        </label>
      ))}
    </div>
  )
}

function renderCheckbox({ field, value, onChange, disabled, readOnly }: FieldProps) {
  // Single checkbox (no options) — boolean toggle
  if (!field.options || field.options.length === 0) {
    return (
      <div className="flex items-center gap-2">
        <Checkbox
          checked={!!value}
          disabled={disabled || readOnly}
          onCheckedChange={onChange}
        />
        {field.description && <span className="text-sm text-muted-foreground">{field.description}</span>}
      </div>
    )
  }
  // Multiple options use the same dropdown multiselect value contract.
  return renderMultiSelect({ field, value, onChange, onBlur: () => {}, disabled, readOnly })
}

function renderSwitch({ field, value, onChange, disabled, readOnly }: FieldProps) {
  return (
    <div className="flex items-center gap-2">
      <Switch
        checked={!!value}
        disabled={disabled || readOnly}
        onCheckedChange={onChange}
      />
      {field.description && <span className="text-sm text-muted-foreground">{field.description}</span>}
    </div>
  )
}

function renderDate({ field, value, onChange, onBlur, disabled, readOnly }: FieldProps) {
  return (
    <Input
      type="date"
      placeholder={field.placeholder}
      value={(value as string) ?? ""}
      onChange={(e) => onChange(e.target.value)}
      onBlur={onBlur}
      disabled={disabled}
      readOnly={readOnly}
    />
  )
}

function renderDatetime({ field, value, onChange, onBlur, disabled, readOnly }: FieldProps) {
  return (
    <Input
      type="datetime-local"
      placeholder={field.placeholder}
      value={(value as string) ?? ""}
      onChange={(e) => onChange(e.target.value)}
      onBlur={onBlur}
      disabled={disabled}
      readOnly={readOnly}
    />
  )
}

function usesDateTimeRange(field: FormField, range: { start?: string; end?: string }) {
  if (field.props?.withTime === true || field.props?.mode === "datetime") return true
  return hasTimeComponent(range.start) || hasTimeComponent(range.end)
}

function hasTimeComponent(value?: string) {
  return Boolean(value && /\d{2}:\d{2}/.test(value))
}

function toDateInputValue(value?: string) {
  if (!value) return ""
  return value.replace(" ", "T").slice(0, 10)
}

function toDateTimeLocalValue(value?: string) {
  if (!value) return ""
  const normalized = value.replace(" ", "T")
  const match = normalized.match(/^(\d{4}-\d{2}-\d{2})T(\d{2}:\d{2})/)
  if (!match) return ""
  return `${match[1]}T${match[2]}`
}

function fromDateTimeLocalValue(value: string) {
  if (!value) return ""
  return `${value}:00+08:00`
}

function renderDateRange({ field, value, onChange, disabled, readOnly }: FieldProps) {
  const range = (value as { start?: string; end?: string }) ?? {}
  const useDateTime = usesDateTimeRange(field, range)
  return (
    <div className="flex items-center gap-2">
      <Input
        type={useDateTime ? "datetime-local" : "date"}
        value={useDateTime ? toDateTimeLocalValue(range.start) : toDateInputValue(range.start)}
        onChange={(e) => onChange({ ...range, start: useDateTime ? fromDateTimeLocalValue(e.target.value) : e.target.value })}
        disabled={disabled}
        readOnly={readOnly}
        placeholder="开始日期"
      />
      <span className="text-muted-foreground">—</span>
      <Input
        type={useDateTime ? "datetime-local" : "date"}
        value={useDateTime ? toDateTimeLocalValue(range.end) : toDateInputValue(range.end)}
        onChange={(e) => onChange({ ...range, end: useDateTime ? fromDateTimeLocalValue(e.target.value) : e.target.value })}
        disabled={disabled}
        readOnly={readOnly}
        placeholder="结束日期"
      />
    </div>
  )
}

function renderUserPicker({ field, value, onChange, disabled, readOnly }: FieldProps) {
  return (
    <UserPicker
      value={(value as string) ?? ""}
      onChange={onChange}
      disabled={disabled}
      readOnly={readOnly}
      placeholder={field.placeholder ?? "选择用户"}
    />
  )
}

function renderDeptPicker({ field, value, onChange, disabled, readOnly }: FieldProps) {
  return (
    <DeptPicker
      value={(value as string) ?? ""}
      onChange={onChange}
      disabled={disabled}
      readOnly={readOnly}
      placeholder={field.placeholder ?? "选择部门"}
    />
  )
}

function renderRichText({ field, value, onChange, onBlur, disabled, readOnly }: FieldProps) {
  // Textarea + markdown preview deferred — use plain textarea for now
  return (
    <Textarea
      placeholder={field.placeholder ?? "支持 Markdown 格式"}
      value={(value as string) ?? ""}
      onChange={(e) => onChange(e.target.value)}
      onBlur={onBlur}
      disabled={disabled}
      readOnly={readOnly}
      rows={(field.props?.rows as number) ?? 6}
    />
  )
}

function renderTableField({ field, value, onChange, disabled, readOnly }: FieldProps) {
  const columns = tableColumns(field)
  const rows = Array.isArray(value) ? (value as Array<Record<string, unknown>>) : []

  const updateCell = (rowIndex: number, key: string, nextValue: unknown) => {
    const next = rows.map((row, index) => index === rowIndex ? { ...row, [key]: nextValue } : row)
    onChange(next)
  }

  const addRow = () => {
    const row: Record<string, unknown> = {}
    for (const column of columns) {
      row[column.key] = defaultValueForField(columnAsField(column))
    }
    onChange([...rows, row])
  }

  const removeRow = (rowIndex: number) => {
    onChange(rows.filter((_, index) => index !== rowIndex))
  }

  if (columns.length === 0) {
    return <p className="text-xs text-amber-500">表格字段缺少列配置</p>
  }

  return (
    <div className="space-y-2">
      <Table>
        <TableHeader>
          <TableRow>
            {columns.map((column) => (
              <TableHead key={column.key}>
                {column.label}
                {column.required && <span className="ml-0.5 text-destructive">*</span>}
              </TableHead>
            ))}
            {!readOnly && <TableHead className="w-12" />}
          </TableRow>
        </TableHeader>
        <TableBody>
          {rows.length === 0 ? (
            <TableRow>
              <TableCell colSpan={columns.length + (readOnly ? 0 : 1)} className="text-center text-muted-foreground">
                暂无数据
              </TableCell>
            </TableRow>
          ) : rows.map((row, rowIndex) => (
            <TableRow key={rowIndex}>
              {columns.map((column) => (
                <TableCell key={column.key} className="min-w-40">
                  {renderCompactField({
                    column,
                    value: row[column.key],
                    onChange: (nextValue) => updateCell(rowIndex, column.key, nextValue),
                    disabled,
                    readOnly,
                  })}
                </TableCell>
              ))}
              {!readOnly && (
                <TableCell>
                  <Button
                    type="button"
                    variant="ghost"
                    size="icon-sm"
                    onClick={() => removeRow(rowIndex)}
                    disabled={disabled}
                    aria-label="删除行"
                  >
                    <Trash2 className="size-4" />
                  </Button>
                </TableCell>
              )}
            </TableRow>
          ))}
        </TableBody>
      </Table>
      {!readOnly && (
        <Button type="button" variant="outline" size="sm" onClick={addRow} disabled={disabled}>
          <Plus className="size-4" />
          添加行
        </Button>
      )}
    </div>
  )
}

function columnAsField(column: TableColumn): FormField {
  return {
    key: column.key,
    type: column.type,
    label: column.label,
    placeholder: column.placeholder,
    required: column.required,
    validation: column.validation,
    options: column.options,
  }
}

function renderCompactField({
  column,
  value,
  onChange,
  disabled,
  readOnly,
}: {
  column: TableColumn
  value: unknown
  onChange: (value: unknown) => void
  disabled: boolean
  readOnly: boolean
}) {
  const field = columnAsField(column)
  if (field.type === "select") {
    return renderSelect({ field, value, onChange, onBlur: () => {}, disabled, readOnly })
  }
  if (field.type === "radio") {
    return renderRadio({ field, value, onChange, onBlur: () => {}, disabled, readOnly })
  }
  if (field.type === "multi_select" || (field.type === "checkbox" && field.options && field.options.length > 0)) {
    return renderMultiSelect({ field, value, onChange, onBlur: () => {}, disabled, readOnly })
  }
  if (field.type === "checkbox") {
    return renderCheckbox({ field, value, onChange, onBlur: () => {}, disabled, readOnly })
  }
  if (field.type === "switch") {
    return renderSwitch({ field, value, onChange, onBlur: () => {}, disabled, readOnly })
  }
  if (field.type === "number") {
    return renderNumber({ field, value, onChange, onBlur: () => {}, disabled, readOnly })
  }
  if (field.type === "date") {
    return renderDate({ field, value, onChange, onBlur: () => {}, disabled, readOnly })
  }
  if (field.type === "datetime") {
    return renderDatetime({ field, value, onChange, onBlur: () => {}, disabled, readOnly })
  }
  if (field.type === "date_range") {
    return renderDateRange({ field, value, onChange, onBlur: () => {}, disabled, readOnly })
  }
  if (field.type === "textarea") {
    return renderTextarea({ field, value, onChange, onBlur: () => {}, disabled, readOnly })
  }
  return renderText({ field, value, onChange, onBlur: () => {}, disabled, readOnly })
}

function renderFallback({ field, value, onChange, onBlur, disabled, readOnly }: FieldProps) {
  return (
    <div>
      <Input
        placeholder={field.placeholder}
        value={(value as string) ?? ""}
        onChange={(e) => onChange(e.target.value)}
        onBlur={onBlur}
        disabled={disabled}
        readOnly={readOnly}
      />
      <p className="text-xs text-amber-500 mt-1">未知字段类型: {field.type}</p>
    </div>
  )
}

// Renderer lookup
const renderers: Record<string, (props: FieldProps) => React.ReactNode> = {
  text: renderText,
  email: renderText,
  url: renderText,
  textarea: renderTextarea,
  number: renderNumber,
  select: renderSelect,
  multi_select: renderMultiSelect,
  radio: renderRadio,
  checkbox: renderCheckbox,
  switch: renderSwitch,
  date: renderDate,
  datetime: renderDatetime,
  date_range: renderDateRange,
  user_picker: renderUserPicker,
  dept_picker: renderDeptPicker,
  rich_text: renderRichText,
  table: renderTableField,
}

export function renderField(props: FieldProps): React.ReactNode {
  const renderer = renderers[props.field.type] ?? renderFallback
  return renderer(props)
}
