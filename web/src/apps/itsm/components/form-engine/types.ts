import type { ReactNode } from "react"

// Form Schema v1 TypeScript types — mirrors Go form.FormSchema

export interface FormSchema {
  version: number
  fields: FormField[]
  layout?: FormLayout | null
}

export interface FormField {
  key: string
  type: FieldType
  label: string
  placeholder?: string
  description?: string
  defaultValue?: unknown
  required?: boolean
  disabled?: boolean
  validation?: ValidationRule[]
  options?: FieldOption[]
  visibility?: VisibilityRule
  binding?: string
  permissions?: Record<string, "editable" | "readonly" | "hidden">
  width?: "full" | "half" | "third"
  props?: Record<string, unknown>
}

export type FieldType =
  | "text"
  | "textarea"
  | "number"
  | "email"
  | "url"
  | "select"
  | "multi_select"
  | "radio"
  | "checkbox"
  | "switch"
  | "date"
  | "datetime"
  | "date_range"
  | "user_picker"
  | "dept_picker"
  | "rich_text"
  | "table"

export interface ValidationRule {
  rule: "required" | "minLength" | "maxLength" | "min" | "max" | "pattern" | "email" | "url"
  value?: unknown
  message: string
}

export interface FieldOption {
  label: string
  value: string | number | boolean
}

export interface TableColumn {
  key: string
  type: Exclude<FieldType, "table" | "rich_text">
  label: string
  placeholder?: string
  required?: boolean
  validation?: ValidationRule[]
  options?: FieldOption[]
}

export interface VisibilityRule {
  conditions: VisibilityCondition[]
  logic?: "and" | "or"
}

export interface VisibilityCondition {
  field: string
  operator: "equals" | "not_equals" | "in" | "not_in" | "is_empty" | "is_not_empty"
  value?: unknown
}

export interface FormLayout {
  columns?: 1 | 2 | 3
  sections: LayoutSection[]
}

export interface LayoutSection {
  title: string
  description?: string
  collapsible?: boolean
  fields: string[]
}

export type FormMode = "create" | "edit" | "view"

export interface FormRendererProps {
  schema: FormSchema
  data?: Record<string, unknown>
  mode: FormMode
  nodeId?: string
  onSubmit?: (data: Record<string, unknown>) => void
  onChange?: (data: Record<string, unknown>) => void
  disabled?: boolean
  footer?: ReactNode
}
