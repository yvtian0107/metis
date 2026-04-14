import { useTranslation } from "react-i18next"
import { Clock } from "lucide-react"
import { Button } from "@/components/ui/button"

interface TimeRangePickerProps {
  value: string
  presets: ReadonlyArray<{ label: string; minutes: number }>
  onSelect: (label: string) => void
  onRefresh: () => void
}

export function TimeRangePicker({ value, presets, onSelect, onRefresh }: TimeRangePickerProps) {
  const { t } = useTranslation("apm")

  return (
    <div className="flex items-center gap-1">
      <Clock className="mr-1 h-4 w-4 text-muted-foreground" />
      {presets.map((p) => (
        <Button
          key={p.label}
          variant={value === p.label ? "default" : "ghost"}
          size="sm"
          className="h-7 px-2.5 text-xs"
          onClick={() => {
            if (value === p.label) {
              onRefresh()
            } else {
              onSelect(p.label)
            }
          }}
        >
          {t(`timeRange.${p.label}`)}
        </Button>
      ))}
    </div>
  )
}
