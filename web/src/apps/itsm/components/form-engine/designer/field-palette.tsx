import { useTranslation } from "react-i18next"
import {
  Type, AlignLeft, Hash, Mail, Link, ChevronDown, ListChecks,
  CircleDot, CheckSquare, ToggleLeft, Calendar, CalendarRange,
  User, Building2, FileText, Table2,
} from "lucide-react"
import { cn } from "@/lib/utils"
import type { FieldType } from "../types"

interface FieldTypePaletteProps {
  onAddField: (type: FieldType) => void
}

interface PaletteItem {
  type: FieldType
  icon: React.ElementType
}

const groups: { key: string; items: PaletteItem[] }[] = [
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

export function FieldTypePalette({ onAddField }: FieldTypePaletteProps) {
  const { t } = useTranslation("itsm")

  function handleDragStart(event: React.DragEvent, type: FieldType) {
    event.dataTransfer.setData("application/metis-form-field-type", type)
    event.dataTransfer.effectAllowed = "copy"
  }

  return (
    <div className="space-y-5">
      {groups.map((group) => (
        <div key={group.key}>
          <h4 className="mb-2 text-[11px] font-semibold uppercase tracking-[0.14em] text-muted-foreground/72">
            {t(`forms.fieldGroup.${group.key}`)}
          </h4>
          <div className="grid grid-cols-1 gap-1.5 xl:grid-cols-2">
            {group.items.map(({ type, icon: Icon }) => (
              <button
                key={type}
                type="button"
                draggable
                onDragStart={(event) => handleDragStart(event, type)}
                onClick={() => onAddField(type)}
                className={cn(
                  "flex min-h-9 cursor-grab items-center gap-2 rounded-lg border border-border/70 bg-white/72 px-2.5 py-2 text-[12px] active:cursor-grabbing",
                  "text-left transition-colors hover:border-primary/35 hover:bg-white hover:text-foreground",
                )}
              >
                <Icon className="h-3.5 w-3.5 shrink-0 text-muted-foreground" />
                <span className="min-w-0 leading-4">{t(`forms.type.${type}`)}</span>
              </button>
            ))}
          </div>
        </div>
      ))}
    </div>
  )
}
