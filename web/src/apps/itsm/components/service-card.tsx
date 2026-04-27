import { useNavigate } from "react-router"
import { useTranslation } from "react-i18next"
import { MoreHorizontal, Pencil, Trash2, Plus, Sparkles, Workflow } from "lucide-react"
import { cn } from "@/lib/utils"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import {
  DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu"
import {
  AlertDialog, AlertDialogAction, AlertDialogCancel, AlertDialogContent,
  AlertDialogDescription, AlertDialogFooter, AlertDialogHeader, AlertDialogTitle,
} from "@/components/ui/alert-dialog"
import { useState } from "react"
import type { ServiceDefItem } from "../api"

function getInitials(name: string) {
  return name.slice(0, 2).toUpperCase()
}

function relativeTime(dateStr: string) {
  const diff = Date.now() - new Date(dateStr).getTime()
  const mins = Math.floor(diff / 60000)
  if (mins < 1) return "刚刚"
  if (mins < 60) return `${mins}分钟前`
  const hours = Math.floor(mins / 60)
  if (hours < 24) return `${hours}小时前`
  const days = Math.floor(hours / 24)
  if (days < 30) return `${days}天前`
  const months = Math.floor(days / 30)
  return `${months}个月前`
}

// ─── ServiceCard ─────────────────────────────────────────

interface ServiceCardProps {
  service: ServiceDefItem
  canUpdate: boolean
  canDelete: boolean
  onDelete: (id: number) => void
}

export function ServiceCard({ service, canUpdate, canDelete, onDelete }: ServiceCardProps) {
  const { t } = useTranslation("itsm")
  const navigate = useNavigate()
  const [deleteOpen, setDeleteOpen] = useState(false)
  const isSmart = service.engineType === "smart"
  const EngineIcon = isSmart ? Sparkles : Workflow

  return (
    <>
      <div
        data-testid={`itsm-service-card-${service.code}`}
        className={cn(
          "workspace-surface group relative flex min-h-[154px] cursor-pointer flex-col rounded-[1.25rem] p-4",
          "transition-colors duration-200 hover:border-border/80 hover:bg-white/50",
        )}
        onClick={() => navigate(`/itsm/services/${service.id}`)}
      >
        <div className="flex items-start gap-3">
          <div className="flex h-10 w-10 shrink-0 items-center justify-center rounded-xl border border-border/55 bg-background/45 text-[13px] font-semibold text-foreground/78">
            {getInitials(service.name)}
          </div>
          <div className="min-w-0 flex-1 space-y-1">
            <p className="truncate text-[15px] font-semibold leading-5 tracking-[-0.01em]">{service.name}</p>
            <div className="flex min-w-0 items-center gap-2 text-xs text-muted-foreground">
              <span className="truncate font-mono">{service.code}</span>
            </div>
          </div>
          {(canUpdate || canDelete) && (
            <div data-action-zone="" onClick={(e) => e.stopPropagation()}>
              <DropdownMenu>
                <DropdownMenuTrigger asChild>
                  <Button variant="ghost" size="icon-sm" className="opacity-0 transition-opacity group-hover:opacity-100">
                    <MoreHorizontal className="h-4 w-4" />
                  </Button>
                </DropdownMenuTrigger>
                <DropdownMenuContent align="end">
                  {canUpdate && (
                    <DropdownMenuItem onClick={() => navigate(`/itsm/services/${service.id}`)}>
                      <Pencil className="mr-2 h-3.5 w-3.5" />{t("edit")}
                    </DropdownMenuItem>
                  )}
                  {canDelete && (
                    <DropdownMenuItem className="text-destructive focus:text-destructive" onClick={() => setDeleteOpen(true)}>
                      <Trash2 className="mr-2 h-3.5 w-3.5" />{t("services.confirmDelete")}
                    </DropdownMenuItem>
                  )}
                </DropdownMenuContent>
              </DropdownMenu>
            </div>
          )}
        </div>

        <div className="mt-4 flex flex-wrap items-center gap-2">
          <Badge variant={isSmart ? "default" : "outline"} className="h-6 gap-1.5 px-2 text-[11px]">
            <EngineIcon className="h-3 w-3" />
            {isSmart ? t("services.engineSmart") : t("services.engineClassic")}
          </Badge>
          <Badge variant={service.isActive ? "secondary" : "outline"} className="h-6 gap-1.5 px-2 text-[11px]">
            <span className={cn("h-1.5 w-1.5 rounded-full", service.isActive ? "bg-emerald-500" : "bg-muted-foreground/45")} />
            {service.isActive ? t("services.active") : t("services.inactive")}
          </Badge>
        </div>

        <div className="mt-auto flex items-center border-t border-border/45 pt-3 text-xs text-muted-foreground">
          <span>{relativeTime(service.updatedAt)}</span>
          <span className="ml-auto opacity-0 transition-opacity group-hover:opacity-100">{t("services.detail")}</span>
        </div>
      </div>

      {/* Delete confirmation */}
      <AlertDialog open={deleteOpen} onOpenChange={setDeleteOpen}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>{t("services.deleteTitle")}</AlertDialogTitle>
            <AlertDialogDescription>{t("services.deleteDesc", { name: service.name })}</AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel size="sm">{t("common:cancel")}</AlertDialogCancel>
            <AlertDialogAction size="sm" onClick={() => onDelete(service.id)}>{t("services.confirmDelete")}</AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </>
  )
}

// ─── GuideCard ───────────────────────────────────────────

interface GuideCardProps {
  onClick: () => void
}

export function GuideCard({ onClick }: GuideCardProps) {
  const { t } = useTranslation("itsm")

  return (
    <button
      type="button"
      onClick={onClick}
      className={cn(
        "flex min-h-[140px] flex-col items-center justify-center gap-2 rounded-xl",
        "border-2 border-dashed border-muted-foreground/20 bg-muted/20",
        "transition-colors hover:border-primary/30 hover:bg-muted/40 cursor-pointer",
      )}
    >
      <Plus className="h-8 w-8 text-muted-foreground/40" />
      <span className="text-sm text-muted-foreground">{t("services.guideCardHint")}</span>
    </button>
  )
}
