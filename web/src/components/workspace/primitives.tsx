"use client"

import type { LucideIcon } from "lucide-react"
import type { MouseEventHandler, ReactNode } from "react"
import { Search, X } from "lucide-react"

import { cn } from "@/lib/utils"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { AlertDialogTrigger } from "@/components/ui/alert-dialog"
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from "@/components/ui/tooltip"

type WorkspaceStatusTone = "success" | "neutral" | "warning" | "danger" | "info"

const statusToneClass: Record<WorkspaceStatusTone, string> = {
  success: "bg-emerald-500/78",
  neutral: "bg-muted-foreground/45",
  warning: "bg-amber-500/78",
  danger: "bg-red-500/78",
  info: "bg-sky-500/78",
}

function WorkspaceSearchField({
  value,
  onChange,
  placeholder,
  className,
}: {
  value: string
  onChange: (value: string) => void
  placeholder: string
  className?: string
}) {
  return (
    <div className={cn("relative w-full sm:w-72", className)}>
      <Search className="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground/58" />
      <Input
        value={value}
        onChange={(event) => onChange(event.target.value)}
        placeholder={placeholder}
        className="workspace-toolbar-input h-8 bg-transparent pl-9 pr-8 text-sm"
      />
      {value ? (
        <Button
          type="button"
          variant="ghost"
          size="icon-xs"
          className="absolute right-1 top-1/2 -translate-y-1/2 text-muted-foreground/70 hover:text-foreground"
          onClick={() => onChange("")}
        >
          <X className="h-3.5 w-3.5" />
          <span className="sr-only">Clear</span>
        </Button>
      ) : null}
    </div>
  )
}

function WorkspaceStatus({
  label,
  tone = "neutral",
  className,
}: {
  label: ReactNode
  tone?: WorkspaceStatusTone
  className?: string
}) {
  return (
    <span className={cn("inline-flex items-center gap-1.5 rounded-full border border-border/65 bg-background/35 px-2.5 py-1 text-xs font-medium text-foreground/72", className)}>
      <span className={cn("h-1.5 w-1.5 rounded-full", statusToneClass[tone])} />
      {label}
    </span>
  )
}

function WorkspaceBooleanStatus({
  active,
  activeLabel,
  inactiveLabel,
}: {
  active: boolean
  activeLabel: string
  inactiveLabel: string
}) {
  return (
    <WorkspaceStatus
      tone={active ? "success" : "neutral"}
      label={active ? activeLabel : inactiveLabel}
    />
  )
}

function WorkspaceIconAction({
  label,
  icon: Icon,
  className,
  onClick,
  disabled,
}: {
  label: string
  icon: LucideIcon
  className?: string
  onClick?: MouseEventHandler<HTMLButtonElement>
  disabled?: boolean
}) {
  return (
    <Tooltip>
      <TooltipTrigger asChild>
        <Button
          type="button"
          variant="ghost"
          size="icon-sm"
          className={cn("text-muted-foreground hover:text-foreground", className)}
          onClick={onClick}
          disabled={disabled}
        >
          <Icon className="h-3.5 w-3.5" />
          <span className="sr-only">{label}</span>
        </Button>
      </TooltipTrigger>
      <TooltipContent side="top" sideOffset={6}>
        {label}
      </TooltipContent>
    </Tooltip>
  )
}

function WorkspaceAlertIconAction({
  label,
  icon: Icon,
  className,
  disabled,
}: {
  label: string
  icon: LucideIcon
  className?: string
  disabled?: boolean
}) {
  return (
    <Tooltip>
      <TooltipTrigger asChild>
        <AlertDialogTrigger asChild>
          <Button
            type="button"
            variant="ghost"
            size="icon-sm"
            className={cn("text-muted-foreground hover:text-foreground", className)}
            disabled={disabled}
          >
            <Icon className="h-3.5 w-3.5" />
            <span className="sr-only">{label}</span>
          </Button>
        </AlertDialogTrigger>
      </TooltipTrigger>
      <TooltipContent side="top" sideOffset={6}>
        {label}
      </TooltipContent>
    </Tooltip>
  )
}

function WorkspaceFormSection({
  title,
  children,
  className,
}: {
  title: string
  children: ReactNode
  className?: string
}) {
  return (
    <section className={cn("space-y-3 border-t border-border/45 pt-4 first:border-t-0 first:pt-0", className)}>
      <h3 className="text-xs font-semibold tracking-[0.16em] text-muted-foreground/72 uppercase">
        {title}
      </h3>
      {children}
    </section>
  )
}

function WorkspaceColorSwatch({
  color,
  className,
}: {
  color: string
  className?: string
}) {
  return (
    <span className={cn("inline-flex h-9 w-9 shrink-0 items-center justify-center rounded-lg border border-border/60 bg-background/40", className)}>
      <span className="h-4 w-4 rounded-[0.35rem] border border-black/5" style={{ backgroundColor: color }} />
    </span>
  )
}

export {
  WorkspaceBooleanStatus,
  WorkspaceAlertIconAction,
  WorkspaceColorSwatch,
  WorkspaceFormSection,
  WorkspaceIconAction,
  WorkspaceSearchField,
  WorkspaceStatus,
}
export type { WorkspaceStatusTone }
