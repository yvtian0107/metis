"use client"

import * as React from "react"
import type { LucideIcon } from "lucide-react"
import { ChevronLeft, ChevronRight } from "lucide-react"
import { useTranslation } from "react-i18next"

import { cn } from "@/lib/utils"

import { Button } from "./button"
import { TableCell, TableHead, TableRow } from "./table"

function DataTableCard({ className, ...props }: React.ComponentProps<"div">) {
  return (
    <div
      className={cn(
        "overflow-hidden rounded-xl border bg-card",
        className
      )}
      {...props}
    />
  )
}

function DataTableToolbar({ className, ...props }: React.ComponentProps<"div">) {
  return (
    <div
      className={cn(
        "flex flex-col gap-3 lg:flex-row lg:items-center lg:justify-between",
        className
      )}
      {...props}
    />
  )
}

function DataTableToolbarGroup({
  className,
  ...props
}: React.ComponentProps<"div">) {
  return (
    <div
      className={cn(
        "flex min-w-0 flex-1 flex-col gap-2 sm:flex-row sm:items-center",
        className
      )}
      {...props}
    />
  )
}

function DataTableActions({ className, ...props }: React.ComponentProps<"div">) {
  return (
    <div
      className={cn("flex items-center justify-end gap-1.5", className)}
      {...props}
    />
  )
}

function DataTableActionsHead({
  className,
  ...props
}: React.ComponentProps<typeof TableHead>) {
  return (
    <TableHead
      className={cn(
        "w-[1%] border-l border-border/50 pl-6 text-center whitespace-nowrap",
        className
      )}
      {...props}
    />
  )
}

function DataTableActionsCell({
  className,
  ...props
}: React.ComponentProps<typeof TableCell>) {
  return (
    <TableCell
      className={cn(
        "border-l border-border/50 pl-6 whitespace-nowrap",
        className
      )}
      {...props}
    />
  )
}

function DataTableLoadingRow({
  colSpan,
  label,
}: {
  colSpan: number
  label?: React.ReactNode
}) {
  const { t } = useTranslation()
  return (
    <TableRow>
      <TableCell colSpan={colSpan} className="h-28 text-center text-sm text-muted-foreground">
        {label ?? t("loading")}
      </TableCell>
    </TableRow>
  )
}

function DataTableEmptyRow({
  colSpan,
  icon: Icon,
  title,
  description,
}: {
  colSpan: number
  icon: LucideIcon
  title: React.ReactNode
  description?: React.ReactNode
}) {
  return (
    <TableRow>
      <TableCell colSpan={colSpan} className="h-44 text-center">
        <div className="flex flex-col items-center gap-2 text-muted-foreground">
          <Icon className="h-10 w-10 stroke-1" />
          <p className="text-sm font-medium">{title}</p>
          {description ? <p className="text-xs">{description}</p> : null}
        </div>
      </TableCell>
    </TableRow>
  )
}

function DataTablePagination({
  total,
  page,
  totalPages,
  onPageChange,
  className,
}: {
  total: number
  page: number
  totalPages: number
  onPageChange: (page: number) => void
  className?: string
}) {
  const { t } = useTranslation()
  if (totalPages <= 1) return null

  return (
    <div
      className={cn(
        "flex flex-col gap-3 pt-4 text-sm text-muted-foreground sm:flex-row sm:items-center sm:justify-between",
        className
      )}
    >
      <span>{t("pagination.total", { total })}</span>
      <div className="flex items-center gap-2 self-end sm:self-auto">
        <Button
          variant="outline"
          size="sm"
          disabled={page <= 1}
          onClick={() => onPageChange(page - 1)}
        >
          <ChevronLeft className="h-4 w-4" />
          {t("pagination.prev")}
        </Button>
        <span className="min-w-16 text-center font-medium tabular-nums">
          {page} / {totalPages}
        </span>
        <Button
          variant="outline"
          size="sm"
          disabled={page >= totalPages}
          onClick={() => onPageChange(page + 1)}
        >
          {t("pagination.next")}
          <ChevronRight className="h-4 w-4" />
        </Button>
      </div>
    </div>
  )
}

export {
  DataTableActions,
  DataTableActionsCell,
  DataTableActionsHead,
  DataTableCard,
  DataTableEmptyRow,
  DataTableLoadingRow,
  DataTablePagination,
  DataTableToolbar,
  DataTableToolbarGroup,
}
