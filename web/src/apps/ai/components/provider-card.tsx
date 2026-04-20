import { useTranslation } from "react-i18next"
import { useNavigate } from "react-router"
import { Pencil, Trash2, Zap, MoreHorizontal, Plus } from "lucide-react"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu"
import { getProviderBrand } from "../lib/provider-brand"
import { StatusDot } from "./status-dot"
import type { ProviderItem } from "./provider-sheet"

const MODEL_TYPE_ORDER = ["llm", "embed", "rerank", "tts", "stt", "image"] as const

interface ProviderCardProps {
  provider: ProviderItem
  canUpdate: boolean
  canDelete: boolean
  canTest: boolean
  testingId: number | null
  onTest: (id: number) => void
  onDelete: (provider: ProviderItem) => void
}

function formatRelativeTime(dateStr: string | null): string | null {
  if (!dateStr) return null
  const diff = Date.now() - new Date(dateStr).getTime()
  const minutes = Math.floor(diff / 60000)
  if (minutes < 1) return "<1m"
  if (minutes < 60) return `${minutes}m`
  const hours = Math.floor(minutes / 60)
  if (hours < 24) return `${hours}h`
  const days = Math.floor(hours / 24)
  return `${days}d`
}

function ModelChips({ provider }: { provider: ProviderItem }) {
  const { t } = useTranslation("ai")
  const entries = MODEL_TYPE_ORDER
    .map((type) => ({ type, count: provider.modelTypeCounts?.[type] ?? 0 }))
    .filter((entry) => entry.count > 0)

  if (entries.length === 0) return null

  return (
    <div className="flex flex-wrap justify-end gap-1.5">
      {entries.map(({ type, count }) => (
        <Badge
          key={type}
          variant="outline"
          className="h-6 gap-1 rounded-full border-border/60 bg-muted/20 px-2 text-[11px] font-normal text-muted-foreground"
        >
          <span>{t(`ai:modelTypes.${type}`, type)}</span>
          <span className="rounded-full bg-background px-1.5 py-0.5 font-medium tabular-nums text-foreground">
            {count}
          </span>
        </Badge>
      ))}
    </div>
  )
}

export function ProviderCard({
  provider,
  canUpdate,
  canDelete,
  canTest,
  testingId,
  onTest,
  onDelete,
}: ProviderCardProps) {
  const { t } = useTranslation(["ai", "common"])
  const navigate = useNavigate()
  const brand = getProviderBrand(provider.type)
  const isTesting = testingId === provider.id
  const relTime = formatRelativeTime(provider.healthCheckedAt)

  function handleCardClick(e: React.MouseEvent) {
    // Don't navigate if clicking on action buttons or dropdown
    const target = e.target as HTMLElement
    if (target.closest("[data-action-zone]")) return
    navigate(`/ai/providers/${provider.id}`)
  }

  return (
    <div
      className="group relative flex min-h-[176px] cursor-pointer flex-col overflow-hidden rounded-xl border bg-card p-4 transition-all duration-200 hover:border-border/90 hover:shadow-sm"
      onClick={handleCardClick}
    >
      <div className="flex items-start gap-3">
        <div
          className="flex h-11 w-11 shrink-0 items-center justify-center rounded-xl border bg-muted/35 text-sm font-bold text-foreground/80"
        >
          {brand.avatarText}
        </div>
        <div className="min-w-0 flex-1">
          <div className="flex items-start justify-between gap-3">
            <div className="min-w-0">
              <div className="flex flex-wrap items-center gap-2">
                <h3 className="truncate text-base font-semibold leading-tight">{provider.name}</h3>
                <Badge variant="outline" className="shrink-0 rounded-full px-2 py-0.5 text-[11px] font-medium text-muted-foreground">
                  {t(`ai:types.${provider.type}`, provider.type)}
                </Badge>
              </div>
              <p className="mt-1.5 truncate text-sm leading-5 text-muted-foreground">{provider.baseUrl}</p>
            </div>
            {(canUpdate || canDelete) && (
              <div data-action-zone>
                <DropdownMenu>
                  <DropdownMenuTrigger asChild>
                    <Button
                      variant="ghost"
                      size="icon"
                      className="h-8 w-8 shrink-0 opacity-0 transition-opacity group-hover:opacity-100"
                    >
                      <MoreHorizontal className="h-4 w-4" />
                    </Button>
                  </DropdownMenuTrigger>
                  <DropdownMenuContent align="end">
                    {canUpdate && (
                      <DropdownMenuItem onClick={() => navigate(`/ai/providers/${provider.id}`)}>
                        <Pencil className="mr-2 h-3.5 w-3.5" />
                        {t("common:edit")}
                      </DropdownMenuItem>
                    )}
                    {canDelete && (
                      <>
                        <DropdownMenuSeparator />
                        <DropdownMenuItem
                          className="text-destructive focus:text-destructive"
                          onClick={() => onDelete(provider)}
                        >
                          <Trash2 className="mr-2 h-3.5 w-3.5" />
                          {t("common:delete")}
                        </DropdownMenuItem>
                      </>
                    )}
                  </DropdownMenuContent>
                </DropdownMenu>
              </div>
            )}
          </div>
        </div>
      </div>

      <div className="flex justify-end py-2.5">
        <ModelChips provider={provider} />
      </div>

      <div className="mt-auto flex items-center justify-between border-t pt-3">
        <div className="flex min-h-7 items-center gap-1.5 text-xs text-muted-foreground">
          <StatusDot status={provider.status} loading={isTesting} />
          <span>
            {t(`ai:statusLabels.${provider.status}`, provider.status)}
            {relTime && ` · ${relTime}`}
          </span>
        </div>
        <div className="flex min-h-7 items-center gap-1" data-action-zone>
          {canTest && (
            <Button
              variant="ghost"
              size="sm"
              className="h-8 px-2.5 text-xs font-medium"
              disabled={isTesting}
              onClick={() => onTest(provider.id)}
            >
              <Zap className="mr-1 h-3.5 w-3.5" />
              {isTesting ? t("ai:providers.testing") : t("ai:providers.testConnection")}
            </Button>
          )}
        </div>
      </div>
    </div>
  )
}

// ─── Guide card (add new provider) ──────────────────────────────────────────

interface GuideCardProps {
  onClick: () => void
}

export function ProviderGuideCard({ onClick }: GuideCardProps) {
  const { t } = useTranslation("ai")

  return (
    <button
      type="button"
      className="flex min-h-[164px] cursor-pointer flex-col items-center justify-center gap-3 rounded-xl border-2 border-dashed border-muted-foreground/20 bg-muted/15 p-4 transition-colors hover:border-border/80 hover:bg-muted/30"
      onClick={onClick}
    >
      <div className="flex h-11 w-11 items-center justify-center rounded-xl border bg-muted/35">
        <Plus className="h-4.5 w-4.5 text-muted-foreground" />
      </div>
      <span className="text-sm font-medium text-muted-foreground">{t("providers.create")}</span>
    </button>
  )
}
