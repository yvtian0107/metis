import { useState, useMemo, useCallback, useRef, useEffect } from "react"
import { useNavigate } from "react-router"
import { useTranslation } from "react-i18next"
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query"
import { Plus, MessageSquare, MoreHorizontal, Pencil, Trash2 } from "lucide-react"
import { sessionApi, type AgentSession } from "@/lib/api"
import { toast } from "sonner"
import { Button } from "@/components/ui/button"
import { ScrollArea } from "@/components/ui/scroll-area"
import {
  DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu"
import {
  AlertDialog, AlertDialogAction, AlertDialogCancel, AlertDialogContent,
  AlertDialogDescription, AlertDialogFooter, AlertDialogHeader, AlertDialogTitle,
} from "@/components/ui/alert-dialog"
import { cn } from "@/lib/utils"

interface SessionSidebarProps {
  agentId?: number
  currentSessionId?: number
  collapsed?: boolean
  sessions?: AgentSession[]
  loading?: boolean
  title?: string
  emptyText?: string
  newLabel?: string
  onNew?: () => void
  onSelect?: (sessionId: number) => void
  showDateGroups?: boolean
  showItemActions?: boolean
}

type DateGroup = "today" | "yesterday" | "last7Days" | "last30Days" | "older"

function getDateGroup(dateStr: string): DateGroup {
  const date = new Date(dateStr)
  const now = new Date()
  const today = new Date(now.getFullYear(), now.getMonth(), now.getDate())
  const yesterday = new Date(today)
  yesterday.setDate(yesterday.getDate() - 1)
  const last7 = new Date(today)
  last7.setDate(last7.getDate() - 7)
  const last30 = new Date(today)
  last30.setDate(last30.getDate() - 30)

  if (date >= today) return "today"
  if (date >= yesterday) return "yesterday"
  if (date >= last7) return "last7Days"
  if (date >= last30) return "last30Days"
  return "older"
}

function groupSessionsByDate(sessions: AgentSession[]): Map<DateGroup, AgentSession[]> {
  const groups = new Map<DateGroup, AgentSession[]>()
  const order: DateGroup[] = ["today", "yesterday", "last7Days", "last30Days", "older"]
  for (const key of order) groups.set(key, [])

  for (const s of sessions) {
    const group = getDateGroup(s.createdAt)
    groups.get(group)!.push(s)
  }

  // Remove empty groups
  for (const key of order) {
    if (groups.get(key)!.length === 0) groups.delete(key)
  }
  return groups
}

// Inline rename input
function InlineRename({
  initialValue,
  onSave,
  onCancel,
}: {
  initialValue: string
  onSave: (value: string) => void
  onCancel: () => void
}) {
  const [value, setValue] = useState(initialValue)
  const inputRef = useRef<HTMLInputElement>(null)

  useEffect(() => {
    inputRef.current?.focus()
    inputRef.current?.select()
  }, [])

  return (
    <input
      ref={inputRef}
      type="text"
      value={value}
      onChange={(e) => setValue(e.target.value)}
      onKeyDown={(e) => {
        if (e.key === "Enter") {
          e.preventDefault()
          const trimmed = value.trim()
          if (trimmed) onSave(trimmed)
        } else if (e.key === "Escape") {
          onCancel()
        }
      }}
      onBlur={() => {
        const trimmed = value.trim()
        if (trimmed && trimmed !== initialValue) onSave(trimmed)
        else onCancel()
      }}
      className="flex-1 text-sm bg-transparent border-b border-primary focus:outline-none truncate"
    />
  )
}

export function SessionSidebar({
  agentId,
  currentSessionId,
  collapsed = false,
  sessions: controlledSessions,
  loading,
  title,
  emptyText,
  newLabel,
  onNew,
  onSelect,
  showDateGroups = true,
  showItemActions = true,
}: SessionSidebarProps) {
  const { t } = useTranslation(["ai", "common"])
  const navigate = useNavigate()
  const queryClient = useQueryClient()
  const [renamingId, setRenamingId] = useState<number | null>(null)
  const [deleteTarget, setDeleteTarget] = useState<AgentSession | null>(null)

  const { data, isLoading } = useQuery({
    queryKey: ["ai-sessions", agentId],
    queryFn: () => sessionApi.list({ agentId, pageSize: 50 }),
    enabled: !!agentId && !controlledSessions,
  })
  const sessions = useMemo(() => controlledSessions ?? data?.items ?? [], [controlledSessions, data])
  const isSidebarLoading = loading ?? isLoading

  const grouped = useMemo(() => groupSessionsByDate(sessions), [sessions])

  const createMutation = useMutation({
    mutationFn: () => sessionApi.create(agentId!),
    onSuccess: (session) => {
      queryClient.invalidateQueries({ queryKey: ["ai-sessions"] })
      navigate(`/ai/chat/${session.id}`)
    },
    onError: (err) => toast.error(err.message),
  })

  const deleteMutation = useMutation({
    mutationFn: (sid: number) => sessionApi.delete(sid),
    onSuccess: (_, sid) => {
      queryClient.invalidateQueries({ queryKey: ["ai-sessions"] })
      toast.success(t("ai:chat.sessionDeleted"))
      setDeleteTarget(null)
      if (sid === currentSessionId) {
        // 获取同 Agent 的其他会话（排除被删除的）
        const currentData = queryClient.getQueryData<{ items: AgentSession[] }>(
          ["ai-sessions", agentId]
        )
        const otherSession = currentData?.items.find(s => s.id !== sid)

        if (otherSession) {
          navigate(`/ai/chat/${otherSession.id}`)
        } else {
          navigate("/ai/chat")
        }
      }
    },
    onError: (err) => toast.error(err.message),
  })

  const renameMutation = useMutation({
    mutationFn: ({ sid, title }: { sid: number; title: string }) =>
      sessionApi.update(sid, { title }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["ai-sessions"] })
      setRenamingId(null)
    },
    onError: (err) => toast.error(err.message),
  })

  const handleRename = useCallback((sid: number, title: string) => {
    renameMutation.mutate({ sid, title })
  }, [renameMutation])

  const handleNew = useCallback(() => {
    if (onNew) {
      onNew()
      return
    }
    createMutation.mutate()
  }, [createMutation, onNew])

  const handleSelect = useCallback((sid: number) => {
    if (onSelect) {
      onSelect(sid)
      return
    }
    navigate(`/ai/chat/${sid}`)
  }, [navigate, onSelect])

  // Collapsed mode
  if (collapsed) {
    return (
      <div className="w-12 border-r flex flex-col shrink-0 transition-all duration-200">
        <div className="p-2 border-b">
          <Button
            variant="outline"
            size="icon"
            className="w-8 h-8"
            disabled={(!agentId && !onNew) || createMutation.isPending}
            onClick={handleNew}
          >
            <Plus className="h-4 w-4" />
          </Button>
        </div>
        <ScrollArea className="flex-1">
          <div className="p-1.5 space-y-1">
            {sessions.map((s: AgentSession) => (
              <button
                key={s.id}
                type="button"
                className={cn(
                  "w-full flex items-center justify-center h-8 rounded-md hover:bg-accent",
                  s.id === currentSessionId && "bg-accent"
                )}
                onClick={() => handleSelect(s.id)}
                title={s.title || `#${s.id}`}
              >
                <MessageSquare className={cn(
                  "h-4 w-4 shrink-0",
                  s.id === currentSessionId ? "text-foreground" : "text-muted-foreground"
                )} />
              </button>
            ))}
          </div>
        </ScrollArea>
      </div>
    )
  }

  // Expanded mode
  return (
    <div className="w-64 border-r flex flex-col shrink-0 transition-all duration-200 hidden md:flex">
      <div className="p-3 border-b">
        <Button
          variant="outline"
          size="sm"
          className="w-full"
          disabled={(!agentId && !onNew) || createMutation.isPending}
          onClick={handleNew}
        >
          <Plus className="mr-1.5 h-3.5 w-3.5" />
          {newLabel ?? t("ai:chat.newChat")}
        </Button>
      </div>
      <ScrollArea className="flex-1">
        <div className="p-2">
          {title && <div className="px-2.5 pb-2 pt-1 text-xs font-medium text-muted-foreground">{title}</div>}
          {isSidebarLoading ? (
            <p className="py-4 text-center text-xs text-muted-foreground">{t("common:loading")}</p>
          ) : sessions.length === 0 ? (
            <p className="text-xs text-muted-foreground text-center py-4">{emptyText ?? t("ai:chat.noSessions")}</p>
          ) : showDateGroups ? (
            Array.from(grouped.entries()).map(([groupKey, groupSessions]) => (
              <div key={groupKey} className="mb-3">
                <div className="px-2.5 py-1 text-[10px] font-medium text-muted-foreground/60 uppercase tracking-wider">
                  {t(`ai:chat.dateGroups.${groupKey}`)}
                </div>
                <div className="space-y-0.5">
                  {groupSessions.map((s: AgentSession) => (
                    <div
                      key={s.id}
                      className={cn(
                        "group flex items-center gap-2 rounded-md px-2.5 py-2 cursor-pointer hover:bg-accent text-sm",
                        s.id === currentSessionId && "bg-accent",
                      )}
                      onClick={() => handleSelect(s.id)}
                      onDoubleClick={(e) => {
                        e.stopPropagation()
                        setRenamingId(s.id)
                      }}
                    >
                      <MessageSquare className="h-3.5 w-3.5 text-muted-foreground shrink-0" />
                      {renamingId === s.id ? (
                        <InlineRename
                          initialValue={s.title || `#${s.id}`}
                          onSave={(title) => handleRename(s.id, title)}
                          onCancel={() => setRenamingId(null)}
                        />
                      ) : (
                        <span className="flex-1 truncate">{s.title || `#${s.id}`}</span>
                      )}
                      {showItemActions && (
                      <DropdownMenu>
                        <DropdownMenuTrigger asChild>
                          <button
                            type="button"
                            className="opacity-0 group-hover:opacity-100 shrink-0 p-0.5 rounded hover:bg-muted"
                            onClick={(e) => e.stopPropagation()}
                          >
                            <MoreHorizontal className="h-3.5 w-3.5 text-muted-foreground" />
                          </button>
                        </DropdownMenuTrigger>
                        <DropdownMenuContent align="end" className="w-36">
                          <DropdownMenuItem onClick={(e) => { e.stopPropagation(); setRenamingId(s.id) }}>
                            <Pencil className="h-3.5 w-3.5 mr-2" />
                            {t("ai:chat.rename")}
                          </DropdownMenuItem>
                          <DropdownMenuItem
                            className="text-destructive focus:text-destructive"
                            onClick={(e) => { e.stopPropagation(); setDeleteTarget(s) }}
                          >
                            <Trash2 className="h-3.5 w-3.5 mr-2" />
                            {t("common:delete")}
                          </DropdownMenuItem>
                        </DropdownMenuContent>
                      </DropdownMenu>
                      )}
                    </div>
                  ))}
                </div>
              </div>
            ))
          ) : (
            <div className="space-y-0.5">
              {sessions.map((s: AgentSession) => (
                <button
                  key={s.id}
                  type="button"
                  className={cn(
                    "group flex w-full flex-col rounded-md border border-transparent px-2.5 py-2 text-left text-sm hover:bg-accent",
                    s.id === currentSessionId && "border-primary/18 bg-primary/8 text-foreground",
                  )}
                  onClick={() => handleSelect(s.id)}
                >
                  <span className="line-clamp-2">{s.title || `#${s.id}`}</span>
                  <span className="mt-1 text-[11px] text-muted-foreground/75">
                    {new Date(s.updatedAt).toLocaleString("zh-CN", { month: "2-digit", day: "2-digit", hour: "2-digit", minute: "2-digit" })}
                  </span>
                </button>
              ))}
            </div>
          )}
        </div>
      </ScrollArea>

      {/* Delete confirmation dialog */}
      <AlertDialog open={!!deleteTarget} onOpenChange={(open) => { if (!open) setDeleteTarget(null) }}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>{t("ai:chat.deleteSession")}</AlertDialogTitle>
            <AlertDialogDescription>{t("ai:chat.deleteSessionDesc")}</AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>{t("common:cancel")}</AlertDialogCancel>
            <AlertDialogAction
              onClick={() => deleteTarget && deleteMutation.mutate(deleteTarget.id)}
              disabled={deleteMutation.isPending}
            >
              {t("common:delete")}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  )
}
