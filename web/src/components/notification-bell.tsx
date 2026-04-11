import { useState } from "react"
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query"
import { useTranslation } from "react-i18next"
import { Bell, Megaphone, Check } from "lucide-react"
import { api } from "@/lib/api"
import { formatDateTime } from "@/lib/utils"
import { Button } from "@/components/ui/button"
import { Popover, PopoverContent, PopoverTrigger } from "@/components/ui/popover"
import { ScrollArea } from "@/components/ui/scroll-area"
import { cn } from "@/lib/utils"

interface NotificationItem {
  id: number
  type: string
  source: string
  title: string
  content: string
  createdAt: string
  isRead: boolean
}

interface NotificationListResponse {
  items: NotificationItem[]
  total: number
  page: number
  pageSize: number
}

function formatRelativeTime(dateStr: string, t: (key: string, opts?: Record<string, unknown>) => string): string {
  const now = Date.now()
  const date = new Date(dateStr).getTime()
  const diff = Math.floor((now - date) / 1000)

  if (diff < 60) return t("layout:notifications.justNow")
  if (diff < 3600) return t("layout:notifications.minutesAgo", { count: Math.floor(diff / 60) })
  if (diff < 86400) return t("layout:notifications.hoursAgo", { count: Math.floor(diff / 3600) })
  if (diff < 604800) return t("layout:notifications.daysAgo", { count: Math.floor(diff / 86400) })
  return formatDateTime(dateStr, { dateOnly: true })
}

function NotificationIcon({ type }: { type: string }) {
  switch (type) {
    case "announcement":
      return <Megaphone className="h-4 w-4 shrink-0 text-blue-500" />
    default:
      return <Bell className="h-4 w-4 shrink-0 text-muted-foreground" />
  }
}

export function NotificationBell() {
  const { t } = useTranslation("layout")
  const queryClient = useQueryClient()
  const [open, setOpen] = useState(false)

  const { data: countData } = useQuery({
    queryKey: ["notifications-unread-count"],
    queryFn: () => api.get<{ count: number }>("/api/v1/notifications/unread-count"),
    refetchInterval: 30_000,
    refetchIntervalInBackground: false,
  })

  const { data: listData, isLoading } = useQuery({
    queryKey: ["notifications-list"],
    queryFn: () =>
      api.get<NotificationListResponse>("/api/v1/notifications?page=1&pageSize=50"),
    enabled: open,
  })

  const markReadMutation = useMutation({
    mutationFn: (id: number) => api.put(`/api/v1/notifications/${id}/read`, {}),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["notifications-unread-count"] })
      queryClient.invalidateQueries({ queryKey: ["notifications-list"] })
    },
  })

  const markAllReadMutation = useMutation({
    mutationFn: () => api.put("/api/v1/notifications/read-all", {}),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["notifications-unread-count"] })
      queryClient.invalidateQueries({ queryKey: ["notifications-list"] })
    },
  })

  const unreadCount = countData?.count ?? 0
  const items = listData?.items ?? []
  const hasUnread = unreadCount > 0

  function handleItemClick(item: NotificationItem) {
    if (!item.isRead) {
      markReadMutation.mutate(item.id)
    }
  }

  return (
    <Popover open={open} onOpenChange={setOpen}>
      <PopoverTrigger asChild>
        <button className="relative flex items-center justify-center rounded-md p-1.5 text-muted-foreground transition-colors hover:bg-accent hover:text-accent-foreground">
          <Bell className="h-4 w-4" />
          {hasUnread && (
            <span className="absolute -right-0.5 -top-0.5 flex h-4 min-w-4 items-center justify-center rounded-full bg-destructive px-1 text-[10px] font-medium text-destructive-foreground">
              {unreadCount > 99 ? "99+" : unreadCount}
            </span>
          )}
        </button>
      </PopoverTrigger>
      <PopoverContent align="end" className="w-96 p-0">
        <div className="flex items-center justify-between border-b px-4 py-3">
          <h4 className="text-sm font-semibold">{t("notifications.title")}</h4>
          {hasUnread && (
            <Button
              variant="ghost"
              size="sm"
              className="h-7 text-xs"
              onClick={() => markAllReadMutation.mutate()}
              disabled={markAllReadMutation.isPending}
            >
              <Check className="mr-1 h-3 w-3" />
              {t("notifications.markAllRead")}
            </Button>
          )}
        </div>

        <ScrollArea className="max-h-96">
          {isLoading ? (
            <div className="flex items-center justify-center py-8 text-sm text-muted-foreground">
              {t("common:loading")}
            </div>
          ) : items.length === 0 ? (
            <div className="flex flex-col items-center justify-center gap-2 py-12 text-muted-foreground">
              <Bell className="h-8 w-8 stroke-1" />
              <span className="text-sm">{t("notifications.noNotifications")}</span>
            </div>
          ) : (
            <div className="divide-y">
              {items.map((item) => (
                <button
                  key={item.id}
                  className={cn(
                    "flex w-full items-start gap-3 px-4 py-3 text-left transition-colors hover:bg-accent/50",
                    !item.isRead && "bg-accent/30",
                  )}
                  onClick={() => handleItemClick(item)}
                >
                  <div className="mt-0.5 flex items-center gap-2">
                    {!item.isRead && (
                      <span className="h-2 w-2 shrink-0 rounded-full bg-blue-500" />
                    )}
                    {item.isRead && <span className="h-2 w-2 shrink-0" />}
                    <NotificationIcon type={item.type} />
                  </div>
                  <div className="min-w-0 flex-1">
                    <p className="truncate text-sm font-medium">{item.title}</p>
                    {item.content && (
                      <p className="mt-0.5 line-clamp-2 text-xs text-muted-foreground">
                        {item.content}
                      </p>
                    )}
                    <p className="mt-1 text-xs text-muted-foreground/60">
                      {formatRelativeTime(item.createdAt, t)}
                    </p>
                  </div>
                </button>
              ))}
            </div>
          )}
        </ScrollArea>
      </PopoverContent>
    </Popover>
  )
}
