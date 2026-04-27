import { useCallback, useMemo, useState, type ElementType } from "react"
import { useTranslation } from "react-i18next"
import {
  closestCenter,
  DndContext,
  KeyboardSensor,
  PointerSensor,
  useSensor,
  useSensors,
  type DragEndEvent,
} from "@dnd-kit/core"
import {
  SortableContext,
  sortableKeyboardCoordinates,
  useSortable,
  verticalListSortingStrategy,
} from "@dnd-kit/sortable"
import { CSS } from "@dnd-kit/utilities"
import {
  AlignLeft,
  Building2,
  Calendar,
  CalendarRange,
  CheckSquare,
  ChevronDown,
  FileText,
  GripVertical,
  Hash,
  Link,
  ListChecks,
  Mail,
  Plus,
  Table2,
  Trash2,
  ToggleLeft,
  Type,
  User,
  CircleDot,
} from "lucide-react"
import { Button } from "@/components/ui/button"
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu"
import { Badge } from "@/components/ui/badge"
import { cn } from "@/lib/utils"
import type { FieldType, FormField, FormSchema } from "../types"
import { FieldPropertyEditor, type WorkflowNodeRef } from "./field-property-editor"

interface FormComposerProps {
  schema: FormSchema
  onChange: (schema: FormSchema) => void
  title?: string
  emptyHint?: string
  workflowNodes?: WorkflowNodeRef[]
}

const FIELD_GROUPS: { key: string; items: Array<{ type: FieldType; icon: ElementType }> }[] = [
  {
    key: "basic",
    items: [
      { type: "text", icon: Type },
      { type: "textarea", icon: AlignLeft },
      { type: "number", icon: Hash },
      { type: "email", icon: Mail },
      { type: "url", icon: Link },
    ],
  },
  {
    key: "selection",
    items: [
      { type: "select", icon: ChevronDown },
      { type: "multi_select", icon: ListChecks },
      { type: "radio", icon: CircleDot },
      { type: "checkbox", icon: CheckSquare },
      { type: "switch", icon: ToggleLeft },
    ],
  },
  {
    key: "datetime",
    items: [
      { type: "date", icon: Calendar },
      { type: "datetime", icon: Calendar },
      { type: "date_range", icon: CalendarRange },
    ],
  },
  {
    key: "advanced",
    items: [
      { type: "user_picker", icon: User },
      { type: "dept_picker", icon: Building2 },
      { type: "rich_text", icon: FileText },
      { type: "table", icon: Table2 },
    ],
  },
]

function generateFieldKey(type: FieldType, existing: FormField[]) {
  const keys = new Set(existing.map((field) => field.key))
  let index = existing.length + 1
  let key = `${type}_${index}`
  while (keys.has(key)) {
    index += 1
    key = `${type}_${index}`
  }
  return key
}

function normalizeSchema(schema: FormSchema): FormSchema {
  return {
    version: schema.version || 1,
    fields: Array.isArray(schema.fields) ? schema.fields : [],
    layout: schema.layout ?? null,
  }
}

export function FormComposer({ schema, onChange, title, emptyHint, workflowNodes }: FormComposerProps) {
  const { t } = useTranslation("itsm")
  const [selectedFieldKey, setSelectedFieldKey] = useState<string | null>(() => schema.fields[0]?.key ?? null)
  const current = normalizeSchema(schema)
  const selectedField = selectedFieldKey
    ? current.fields.find((field) => field.key === selectedFieldKey) ?? null
    : null

  const sensors = useSensors(
    useSensor(PointerSensor, { activationConstraint: { distance: 6 } }),
    useSensor(KeyboardSensor, { coordinateGetter: sortableKeyboardCoordinates }),
  )

  const fieldKeys = useMemo(() => current.fields.map((field) => field.key), [current.fields])

  const updateFields = useCallback(
    (fields: FormField[]) => {
      const validKeys = new Set(fields.map((field) => field.key))
      const layout = current.layout
        ? {
            ...current.layout,
            sections: current.layout.sections.map((section) => ({
              ...section,
              fields: section.fields.filter((key) => validKeys.has(key)),
            })),
          }
        : current.layout
      onChange({ ...current, fields, layout })
    },
    [current, onChange],
  )

  function addField(type: FieldType) {
    const key = generateFieldKey(type, current.fields)
    const nextField: FormField = {
      key,
      type,
      label: t(`forms.type.${type}`),
      required: false,
      width: "full",
    }
    if (type === "table") {
      nextField.props = {
        columns: [
          { key: "name", label: "名称", type: "text", required: true },
          { key: "description", label: "说明", type: "text", required: false },
        ],
      }
    }
    updateFields([...current.fields, nextField])
    setSelectedFieldKey(key)
  }

  function handleDragEnd(event: DragEndEvent) {
    const { active, over } = event
    if (!over || active.id === over.id) return
    const activeIndex = current.fields.findIndex((field) => field.key === active.id)
    const overIndex = current.fields.findIndex((field) => field.key === over.id)
    if (activeIndex < 0 || overIndex < 0) return
    const next = [...current.fields]
    const [moved] = next.splice(activeIndex, 1)
    next.splice(overIndex, 0, moved)
    updateFields(next)
  }

  function updateSelectedField(updated: FormField) {
    const originalKey = selectedFieldKey
    const next = current.fields.map((field) => (field.key === originalKey ? updated : field))
    updateFields(next)
    if (originalKey !== updated.key) setSelectedFieldKey(updated.key)
  }

  function deleteField(key: string) {
    const next = current.fields.filter((field) => field.key !== key)
    updateFields(next)
    if (selectedFieldKey === key) setSelectedFieldKey(next[0]?.key ?? null)
  }

  return (
    <div className="flex min-h-0 flex-1 flex-col gap-3">
      <div className="flex shrink-0 items-center justify-between gap-3">
        <div className="min-w-0">
          {title ? <div className="truncate text-sm font-semibold">{title}</div> : null}
          <div className="text-xs text-muted-foreground">{current.fields.length} 个字段</div>
        </div>
        <DropdownMenu>
          <DropdownMenuTrigger asChild>
            <Button type="button" variant="outline" size="sm" className="h-8 shrink-0">
              <Plus className="mr-1.5 h-3.5 w-3.5" />
              字段
            </Button>
          </DropdownMenuTrigger>
          <DropdownMenuContent align="end" className="w-56">
            {FIELD_GROUPS.map((group, index) => (
              <div key={group.key}>
                {index > 0 ? <DropdownMenuSeparator /> : null}
                <DropdownMenuLabel className="text-[11px] uppercase tracking-[0.14em] text-muted-foreground">
                  {t(`forms.fieldGroup.${group.key}`)}
                </DropdownMenuLabel>
                {group.items.map(({ type, icon: Icon }) => (
                  <DropdownMenuItem key={type} onClick={() => addField(type)}>
                    <Icon className="h-3.5 w-3.5" />
                    {t(`forms.type.${type}`)}
                  </DropdownMenuItem>
                ))}
              </div>
            ))}
          </DropdownMenuContent>
        </DropdownMenu>
      </div>

      {current.fields.length === 0 ? (
        <div className="flex min-h-32 shrink-0 flex-col items-center justify-center rounded-lg border border-dashed border-border/70 bg-background/32 px-4 py-6 text-center">
          <p className="text-sm font-medium text-foreground/80">{t("forms.empty")}</p>
          <p className="mt-1 text-xs text-muted-foreground">{emptyHint ?? "暂无字段"}</p>
        </div>
      ) : (
        <DndContext sensors={sensors} collisionDetection={closestCenter} onDragEnd={handleDragEnd}>
          <SortableContext items={fieldKeys} strategy={verticalListSortingStrategy}>
            <div className="shrink-0 space-y-1.5">
              {current.fields.map((field) => (
                <ComposerFieldRow
                  key={field.key}
                  field={field}
                  selected={selectedFieldKey === field.key}
                  onSelect={() => setSelectedFieldKey(field.key)}
                  onDelete={() => deleteField(field.key)}
                />
              ))}
            </div>
          </SortableContext>
        </DndContext>
      )}

      <div className="min-h-0 flex-1 overflow-y-auto rounded-lg border border-border/55 bg-background/28 p-3">
        {selectedField ? (
          <FieldPropertyEditor
            field={selectedField}
            allFields={current.fields}
            onChange={updateSelectedField}
            workflowNodes={workflowNodes}
          />
        ) : (
          <div className="py-8 text-center text-xs text-muted-foreground">未选择字段</div>
        )}
      </div>
    </div>
  )
}

function ComposerFieldRow({
  field,
  selected,
  onSelect,
  onDelete,
}: {
  field: FormField
  selected: boolean
  onSelect: () => void
  onDelete: () => void
}) {
  const { t } = useTranslation("itsm")
  const { attributes, listeners, setNodeRef, transform, transition, isDragging } = useSortable({ id: field.key })

  return (
    <button
      ref={setNodeRef}
      type="button"
      style={{ transform: CSS.Transform.toString(transform), transition }}
      className={cn(
        "group flex w-full items-center gap-2 rounded-lg border px-2.5 py-2 text-left transition-colors",
        selected
          ? "border-primary/45 bg-primary/6 text-foreground"
          : "border-border/58 bg-white/50 hover:border-primary/28 hover:bg-white/72",
        isDragging && "z-10 opacity-70",
      )}
      onClick={onSelect}
    >
      <span
        className="flex size-6 shrink-0 cursor-grab items-center justify-center rounded-md text-muted-foreground hover:bg-muted/65 active:cursor-grabbing"
        onClick={(event) => event.stopPropagation()}
        {...attributes}
        {...listeners}
      >
        <GripVertical className="h-3.5 w-3.5" />
      </span>
      <span className="min-w-0 flex-1">
        <span className="flex min-w-0 items-center gap-1.5">
          <span className="truncate text-xs font-medium">{field.label}</span>
          {field.required ? <span className="text-xs text-destructive">*</span> : null}
        </span>
        <span className="mt-0.5 flex min-w-0 items-center gap-1.5">
          <span className="truncate font-mono text-[10px] text-muted-foreground">{field.key}</span>
          <Badge variant="outline" className="h-4 px-1.5 text-[10px]">
            {t(`forms.type.${field.type}`)}
          </Badge>
        </span>
      </span>
      <span
        className="flex size-6 shrink-0 items-center justify-center rounded-md text-muted-foreground opacity-0 transition group-hover:opacity-100 hover:bg-destructive/8 hover:text-destructive"
        onClick={(event) => {
          event.stopPropagation()
          onDelete()
        }}
      >
        <Trash2 className="h-3.5 w-3.5" />
      </span>
    </button>
  )
}
