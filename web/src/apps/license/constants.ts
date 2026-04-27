import {
  Archive,
  EyeOff,
  Rocket,
  RotateCcw,
} from "lucide-react"

export const STATUS_STYLES: Record<
  string,
  { variant: "default" | "secondary" | "outline" | "destructive"; className: string }
> = {
  unpublished: {
    variant: "secondary",
    className: "bg-amber-100 text-amber-700 border-amber-200 dark:bg-amber-950 dark:text-amber-300 dark:border-amber-900",
  },
  published: {
    variant: "default",
    className: "bg-emerald-100 text-emerald-700 border-emerald-200 dark:bg-emerald-950 dark:text-emerald-300 dark:border-emerald-900",
  },
  archived: {
    variant: "outline",
    className: "bg-slate-100 text-slate-600 border-slate-200 dark:bg-slate-900 dark:text-slate-400 dark:border-slate-800",
  },
}

export type StatusActionConfig = {
  status: string
  labelKey: string
  variant: "default" | "secondary" | "outline" | "destructive"
  icon: React.ElementType
}

export const STATUS_ACTION_CONFIG: Record<string, StatusActionConfig[]> = {
  unpublished: [
    { status: "published", labelKey: "status.publish", variant: "default", icon: Rocket },
    { status: "archived", labelKey: "status.archiveAction", variant: "outline", icon: Archive },
  ],
  published: [
    { status: "unpublished", labelKey: "status.unpublish", variant: "secondary", icon: EyeOff },
    { status: "archived", labelKey: "status.archiveAction", variant: "outline", icon: Archive },
  ],
  archived: [
    { status: "unpublished", labelKey: "status.restoreAction", variant: "default", icon: RotateCcw },
  ],
}
