"use client"

import { useCallback, useEffect, useMemo, useRef, useState, type ComponentType } from "react"
import { Link, useNavigate } from "react-router"
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import type { UIMessage } from "ai"
import {
  AlertTriangle,
  ArrowUpRight,
  Bot,
  GitPullRequestArrow,
  History,
  Loader2,
  MessageSquare,
  PanelRight,
  Plus,
  RotateCw,
  SearchCheck,
  Send,
  ShieldAlert,
  Square,
} from "lucide-react"
import { toast } from "sonner"

import { QAPair } from "@/apps/ai/pages/chat/components/message-item"
import { useAiChat } from "@/apps/ai/pages/chat/hooks/use-ai-chat"
import { sessionApi, type AgentSession } from "@/lib/api"
import { cn } from "@/lib/utils"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Textarea } from "@/components/ui/textarea"
import {
  fetchEngineConfig,
  fetchMyTickets,
  fetchServiceDeskSessionState,
  type ServiceDeskSessionState,
  type TicketItem,
} from "../../api"

type AgentSeatStatus = "online" | "pending" | "offline"

interface AgentSeat {
  key: string
  name: string
  role: string
  status: AgentSeatStatus
  icon: ComponentType<{ className?: string }>
}

function groupUIMessagesIntoPairs(messages: UIMessage[]): Array<{ userMessage: UIMessage; aiMessages: UIMessage[] }> {
  const pairs: Array<{ userMessage: UIMessage; aiMessages: UIMessage[] }> = []
  for (const msg of messages) {
    if (msg.role === "user") {
      pairs.push({ userMessage: msg, aiMessages: [] })
      continue
    }
    if (pairs.length > 0) {
      pairs[pairs.length - 1].aiMessages.push(msg)
    }
  }
  return pairs
}

function formatSessionTime(value: string) {
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return ""
  return date.toLocaleString("zh-CN", { month: "2-digit", day: "2-digit", hour: "2-digit", minute: "2-digit" })
}

function stageView(stage?: string) {
  switch (stage) {
    case "candidates_ready":
      return { label: "匹配服务", tone: "bg-sky-500/10 text-sky-700 border-sky-500/20" }
    case "service_selected":
      return { label: "已选服务", tone: "bg-cyan-500/10 text-cyan-700 border-cyan-500/20" }
    case "service_loaded":
      return { label: "加载工件", tone: "bg-amber-500/10 text-amber-700 border-amber-500/20" }
    case "awaiting_confirmation":
      return { label: "等待确认", tone: "bg-violet-500/10 text-violet-700 border-violet-500/20" }
    case "confirmed":
      return { label: "已确认", tone: "bg-emerald-500/10 text-emerald-700 border-emerald-500/20" }
    default:
      return { label: "待接入", tone: "bg-muted text-muted-foreground border-border" }
  }
}

function expectedActionLabel(action?: string) {
  switch (action) {
    case "itsm.service_match":
      return "识别诉求"
    case "itsm.service_confirm":
      return "确认服务"
    case "itsm.service_load":
      return "装载服务"
    case "itsm.draft_prepare":
      return "生成草稿"
    case "itsm.draft_confirm":
      return "确认草稿"
    case "itsm.validate_participants":
      return "校验参与者"
    default:
      return action || "等待输入"
  }
}

function objectEntries(value?: Record<string, unknown>) {
  return Object.entries(value ?? {}).filter(([, v]) => v !== undefined && v !== null && String(v) !== "")
}

function compactValue(value: unknown) {
  if (typeof value === "string") return value
  if (typeof value === "number" || typeof value === "boolean") return String(value)
  return JSON.stringify(value)
}

function AgentDock({
  seats,
  activeKey,
}: {
  seats: AgentSeat[]
  activeKey: string
}) {
  return (
    <div className="flex min-h-24 items-center gap-3 overflow-x-auto border-b border-border/70 bg-background/80 px-5 py-4">
      {seats.map((seat) => {
        const Icon = seat.icon
        const active = seat.key === activeKey
        const online = seat.status === "online"
        return (
          <button
            key={seat.key}
            type="button"
            disabled={!online}
            className={cn(
              "group flex min-w-40 items-center gap-3 rounded-lg border px-3 py-2.5 text-left transition-colors",
              active
                ? "border-primary/40 bg-primary/8 text-foreground shadow-[0_16px_34px_-28px_hsl(var(--primary))]"
                : "border-border/70 bg-background hover:bg-accent/45",
              !online && "cursor-not-allowed opacity-55 hover:bg-background",
            )}
          >
            <span
              className={cn(
                "flex size-11 shrink-0 items-center justify-center rounded-full border",
                active ? "border-primary/35 bg-primary/12 text-primary" : "border-border bg-muted/55 text-muted-foreground",
              )}
            >
              <Icon className="size-5" />
            </span>
            <span className="min-w-0">
              <span className="block truncate text-sm font-medium">{seat.name}</span>
              <span className="mt-0.5 flex items-center gap-1.5 text-xs text-muted-foreground">
                <span
                  className={cn(
                    "size-1.5 rounded-full",
                    online ? "bg-emerald-500" : seat.status === "pending" ? "bg-amber-500" : "bg-muted-foreground/45",
                  )}
                />
                {seat.role}
              </span>
            </span>
          </button>
        )
      })}
    </div>
  )
}

function SessionStrip({
  sessions,
  activeSessionId,
  creating,
  onSelect,
  onCreate,
}: {
  sessions: AgentSession[]
  activeSessionId: number | null
  creating: boolean
  onSelect: (sessionId: number) => void
  onCreate: () => void
}) {
  return (
    <div className="flex items-center gap-2 border-b border-border/70 bg-muted/18 px-5 py-2.5">
      <div className="flex min-w-0 flex-1 items-center gap-2 overflow-x-auto">
        <History className="size-4 shrink-0 text-muted-foreground" />
        {sessions.slice(0, 8).map((session) => (
          <button
            key={session.id}
            type="button"
            onClick={() => onSelect(session.id)}
            className={cn(
              "h-8 max-w-52 shrink-0 rounded-md border px-3 text-left text-xs transition-colors",
              activeSessionId === session.id
                ? "border-primary/35 bg-primary/8 text-foreground"
                : "border-border/70 bg-background/70 text-muted-foreground hover:bg-accent/45 hover:text-foreground",
            )}
          >
            <span className="block truncate">{session.title || `会话 #${session.id}`}</span>
          </button>
        ))}
      </div>
      <Button type="button" size="sm" variant="outline" onClick={onCreate} disabled={creating}>
        {creating ? <Loader2 className="mr-1.5 size-3.5 animate-spin" /> : <Plus className="mr-1.5 size-3.5" />}
        新服务
      </Button>
    </div>
  )
}

function NotOnDutyState({ loading }: { loading: boolean }) {
  const navigate = useNavigate()
  return (
    <div className="flex flex-1 items-center justify-center p-8">
      <div className="w-full max-w-xl rounded-lg border border-dashed border-border bg-background p-8 text-center">
        {loading ? (
          <Loader2 className="mx-auto size-7 animate-spin text-muted-foreground" />
        ) : (
          <AlertTriangle className="mx-auto size-7 text-amber-600" />
        )}
        <h2 className="mt-4 text-lg font-semibold">服务台智能体未上岗</h2>
        <p className="mt-2 text-sm text-muted-foreground">
          需要在引擎配置中绑定 itsm.servicedesk 默认智能体。
        </p>
        <Button className="mt-5" onClick={() => navigate("/itsm/engine-config")}>
          前往引擎配置
        </Button>
      </div>
    </div>
  )
}

function ServiceDeskArtifacts({
  stateData,
  tickets,
  loading,
}: {
  stateData?: ServiceDeskSessionState
  tickets: TicketItem[]
  loading: boolean
}) {
  const state = stateData?.state
  const view = stageView(state?.stage)
  const draftEntries = objectEntries(state?.draft_form_data)
  const prefillEntries = objectEntries(state?.prefill_form_data)
  const hasDraft = Boolean(state?.draft_summary) || draftEntries.length > 0

  return (
    <aside className="hidden w-[340px] shrink-0 border-l border-border/70 bg-muted/12 xl:flex xl:flex-col">
      <div className="flex h-14 items-center justify-between border-b border-border/70 px-4">
        <div className="flex items-center gap-2">
          <PanelRight className="size-4 text-muted-foreground" />
          <span className="text-sm font-medium">ITSM 工件</span>
        </div>
        <Badge variant="outline" className={cn("border text-xs font-normal", view.tone)}>
          {view.label}
        </Badge>
      </div>

      <div className="min-h-0 flex-1 overflow-y-auto px-4 py-4">
        {loading && (
          <div className="flex items-center gap-2 rounded-md border border-border/70 bg-background/70 px-3 py-2 text-sm text-muted-foreground">
            <Loader2 className="size-4 animate-spin" />
            同步工件
          </div>
        )}

        <section className="rounded-lg border border-border/70 bg-background p-3">
          <div className="text-xs font-medium text-muted-foreground">当前状态</div>
          <div className="mt-2 text-sm font-medium">{expectedActionLabel(stateData?.nextExpectedAction)}</div>
          {state?.request_text && (
            <p className="mt-2 line-clamp-4 text-sm leading-6 text-muted-foreground">{state.request_text}</p>
          )}
          {(state?.loaded_service_id || state?.confirmed_service_id || state?.top_match_service_id) && (
            <div className="mt-3 flex flex-wrap gap-1.5">
              {state.loaded_service_id ? <Badge variant="secondary">服务 #{state.loaded_service_id}</Badge> : null}
              {state.confirmed_service_id ? <Badge variant="secondary">确认 #{state.confirmed_service_id}</Badge> : null}
              {state.top_match_service_id ? <Badge variant="secondary">匹配 #{state.top_match_service_id}</Badge> : null}
            </div>
          )}
        </section>

        <section className="mt-3 rounded-lg border border-border/70 bg-background p-3">
          <div className="flex items-center justify-between">
            <div className="text-xs font-medium text-muted-foreground">草稿</div>
            {state?.draft_version ? <Badge variant="outline">v{state.draft_version}</Badge> : null}
          </div>
          {hasDraft ? (
            <div className="mt-3 space-y-3">
              {state?.draft_summary && <p className="text-sm leading-6">{state.draft_summary}</p>}
              {draftEntries.length > 0 && (
                <div className="space-y-2">
                  {draftEntries.slice(0, 8).map(([key, value]) => (
                    <div key={key} className="rounded-md bg-muted/45 px-2.5 py-2">
                      <div className="text-[11px] text-muted-foreground">{key}</div>
                      <div className="mt-0.5 break-words text-xs">{compactValue(value)}</div>
                    </div>
                  ))}
                </div>
              )}
            </div>
          ) : (
            <div className="mt-3 text-sm text-muted-foreground">等待对话生成草稿</div>
          )}
        </section>

        {prefillEntries.length > 0 && (
          <section className="mt-3 rounded-lg border border-border/70 bg-background p-3">
            <div className="text-xs font-medium text-muted-foreground">预填字段</div>
            <div className="mt-3 space-y-2">
              {prefillEntries.slice(0, 6).map(([key, value]) => (
                <div key={key} className="flex items-start justify-between gap-3 text-xs">
                  <span className="text-muted-foreground">{key}</span>
                  <span className="min-w-0 flex-1 break-words text-right">{compactValue(value)}</span>
                </div>
              ))}
            </div>
          </section>
        )}

        <section className="mt-3 rounded-lg border border-border/70 bg-background p-3">
          <div className="text-xs font-medium text-muted-foreground">已创建工单</div>
          {tickets.length > 0 ? (
            <div className="mt-3 space-y-2">
              {tickets.map((ticket) => (
                <Link
                  key={ticket.id}
                  to={`/itsm/tickets/${ticket.id}`}
                  className="group block rounded-md border border-border/70 px-3 py-2 transition-colors hover:bg-accent/45"
                >
                  <div className="flex items-center justify-between gap-2">
                    <span className="truncate text-sm font-medium">{ticket.code}</span>
                    <ArrowUpRight className="size-3.5 text-muted-foreground group-hover:text-foreground" />
                  </div>
                  <div className="mt-1 line-clamp-2 text-xs text-muted-foreground">{ticket.title}</div>
                </Link>
              ))}
            </div>
          ) : (
            <div className="mt-3 text-sm text-muted-foreground">确认后沉淀为结构化工单</div>
          )}
        </section>
      </div>
    </aside>
  )
}

function ServiceDeskConversation({
  session,
  agentName,
}: {
  session: AgentSession
  agentName: string
}) {
  const queryClient = useQueryClient()
  const [input, setInput] = useState("")
  const scrollRef = useRef<HTMLDivElement>(null)
  const textareaRef = useRef<HTMLTextAreaElement>(null)
  const messagesEndRef = useRef<HTMLDivElement>(null)

  const { data: sessionData, isLoading } = useQuery({
    queryKey: ["ai-session", session.id],
    queryFn: () => sessionApi.get(session.id),
  })

  const invalidateWorkspace = useCallback(() => {
    queryClient.invalidateQueries({ queryKey: ["ai-session", session.id] })
    queryClient.invalidateQueries({ queryKey: ["itsm-service-desk-state", session.id] })
    queryClient.invalidateQueries({ queryKey: ["itsm-service-desk-tickets", session.id] })
    queryClient.invalidateQueries({ queryKey: ["ai-sessions"] })
  }, [queryClient, session.id])

  const chat = useAiChat(session.id, sessionData?.messages, {
    onFinish: invalidateWorkspace,
    onError: (err) => {
      toast.error(err.message)
      invalidateWorkspace()
    },
  })

  const isBusy = chat.status === "streaming" || chat.status === "submitted"
  const qaPairs = useMemo(() => groupUIMessagesIntoPairs(chat.messages), [chat.messages])

  const stateQuery = useQuery({
    queryKey: ["itsm-service-desk-state", session.id],
    queryFn: () => fetchServiceDeskSessionState(session.id),
    refetchInterval: isBusy ? 2500 : false,
  })

  const ticketsQuery = useQuery({
    queryKey: ["itsm-service-desk-tickets", session.id],
    queryFn: () => fetchMyTickets({ page: 1, pageSize: 50 }),
    select: (data) => data.items.filter((ticket) => ticket.agentSessionId === session.id),
    refetchInterval: isBusy ? 4000 : false,
  })

  useEffect(() => {
    messagesEndRef.current?.scrollIntoView({ behavior: isBusy ? "instant" : "smooth" })
  }, [chat.messages.length, isBusy])

  const sendMutation = useMutation({
    mutationFn: async (text: string) => text,
    onSuccess: (text) => {
      chat.sendMessage({ text })
      setInput("")
      requestAnimationFrame(() => textareaRef.current?.focus())
    },
    onError: (err) => toast.error(err.message),
  })

  const cancelMutation = useMutation({
    mutationFn: async () => {
      await chat.stop()
      return sessionApi.cancel(session.id)
    },
    onSuccess: invalidateWorkspace,
    onError: (err) => toast.error(err.message),
  })

  const handleSend = useCallback(() => {
    const text = input.trim()
    if (!text || isBusy || sendMutation.isPending) return
    sendMutation.mutate(text)
  }, [input, isBusy, sendMutation])

  const showEmpty = !isLoading && qaPairs.length === 0

  return (
    <div className="flex min-h-0 flex-1 bg-background">
      <main className="flex min-w-0 flex-1 flex-col">
        <div className="flex h-14 shrink-0 items-center justify-between border-b border-border/70 px-5">
          <div className="min-w-0">
            <div className="flex items-center gap-2">
              <MessageSquare className="size-4 text-primary" />
              <h1 className="truncate text-sm font-semibold">服务台</h1>
              <Badge variant="secondary" className="font-normal">{agentName}</Badge>
            </div>
            <div className="mt-0.5 text-xs text-muted-foreground">{formatSessionTime(session.updatedAt)}</div>
          </div>
          <div className="flex items-center gap-2">
            {isBusy ? (
              <Button size="sm" variant="outline" onClick={() => cancelMutation.mutate()} disabled={cancelMutation.isPending}>
                {cancelMutation.isPending ? <Loader2 className="mr-1.5 size-3.5 animate-spin" /> : <Square className="mr-1.5 size-3.5" />}
                停止
              </Button>
            ) : (
              <Button size="sm" variant="outline" onClick={() => chat.regenerate()} disabled={chat.messages.length === 0}>
                <RotateCw className="mr-1.5 size-3.5" />
                重试
              </Button>
            )}
          </div>
        </div>

        <div ref={scrollRef} className="min-h-0 flex-1 overflow-y-auto">
          {isLoading ? (
            <div className="flex h-full items-center justify-center text-sm text-muted-foreground">
              <Loader2 className="mr-2 size-4 animate-spin" />
              载入会话
            </div>
          ) : showEmpty ? (
            <div className="mx-auto flex h-full max-w-3xl flex-col justify-center px-6 py-12">
              <div className="flex size-14 items-center justify-center rounded-full border border-primary/25 bg-primary/10 text-primary">
                <Bot className="size-7" />
              </div>
              <h2 className="mt-5 text-2xl font-semibold">IT 服务台已上岗</h2>
              <p className="mt-2 max-w-xl text-sm leading-6 text-muted-foreground">
                直接描述诉求，智能体会澄清、填槽、生成草稿，并在你确认后沉淀为工单。
              </p>
            </div>
          ) : (
            <div className="mx-auto max-w-3xl px-4 pb-4">
              {qaPairs.map((pair, index) => {
                const isLastPair = index === qaPairs.length - 1
                return (
                  <QAPair
                    key={pair.userMessage.id}
                    userMessage={pair.userMessage}
                    aiMessages={pair.aiMessages}
                    agentName={agentName}
                    isStreaming={isLastPair && isBusy}
                    onRegenerate={isLastPair ? () => chat.regenerate() : undefined}
                    doneMetrics={
                      isLastPair && chat.status === "ready"
                        ? {
                            inputTokens: chat.lastUsage.promptTokens,
                            outputTokens: chat.lastUsage.completionTokens,
                          }
                        : undefined
                    }
                  />
                )
              })}
              {chat.error && !isBusy && (
                <div className="mb-6 rounded-lg border border-destructive/25 bg-destructive/5 p-3 text-sm text-destructive">
                  {chat.error.message}
                </div>
              )}
              <div ref={messagesEndRef} />
            </div>
          )}
        </div>

        <div className="shrink-0 border-t border-border/70 bg-background px-5 py-4">
          <div className="mx-auto flex max-w-3xl items-end gap-2 rounded-lg border border-border/80 bg-muted/20 p-2">
            <Textarea
              ref={textareaRef}
              value={input}
              onChange={(event) => setInput(event.target.value)}
              onKeyDown={(event) => {
                if (event.key === "Enter" && !event.shiftKey) {
                  event.preventDefault()
                  handleSend()
                }
              }}
              placeholder="描述你的 IT 诉求..."
              className="max-h-36 min-h-11 resize-none border-0 bg-transparent px-2 py-2 shadow-none focus-visible:ring-0"
              disabled={isBusy}
            />
            <Button type="button" size="icon" className="size-10 shrink-0" onClick={handleSend} disabled={!input.trim() || isBusy}>
              {isBusy || sendMutation.isPending ? <Loader2 className="size-4 animate-spin" /> : <Send className="size-4" />}
            </Button>
          </div>
        </div>
      </main>

      <ServiceDeskArtifacts
        stateData={stateQuery.data}
        tickets={ticketsQuery.data ?? []}
        loading={stateQuery.isLoading || ticketsQuery.isLoading}
      />
    </div>
  )
}

export function Component() {
  const queryClient = useQueryClient()
  const [selectedSessionId, setSelectedSessionId] = useState<number | null>(null)

  const { data: config, isLoading: configLoading } = useQuery({
    queryKey: ["itsm-engine-config"],
    queryFn: fetchEngineConfig,
  })

  const serviceDeskAgentId = config?.servicedesk?.agentId ?? 0
  const serviceDeskAgentName = config?.servicedesk?.agentName || "IT 服务台"

  const sessionsQuery = useQuery({
    queryKey: ["ai-sessions", serviceDeskAgentId],
    queryFn: () => sessionApi.list({ agentId: serviceDeskAgentId, page: 1, pageSize: 20 }),
    enabled: serviceDeskAgentId > 0,
  })

  const sessions = sessionsQuery.data?.items ?? []
  const activeSession = sessions.find((item) => item.id === selectedSessionId) ?? sessions[0] ?? null
  const activeSessionId = activeSession?.id ?? null

  const createSessionMutation = useMutation({
    mutationFn: () => sessionApi.create(serviceDeskAgentId),
    onSuccess: (session) => {
      setSelectedSessionId(session.id)
      queryClient.invalidateQueries({ queryKey: ["ai-sessions", serviceDeskAgentId] })
    },
    onError: (err) => toast.error(err.message),
  })

  useEffect(() => {
    if (serviceDeskAgentId <= 0) return
    if (sessionsQuery.isLoading || sessionsQuery.isFetching) return
    if (activeSessionId != null) return
    if (createSessionMutation.isPending) return
    createSessionMutation.mutate()
  }, [activeSessionId, createSessionMutation, serviceDeskAgentId, sessionsQuery.isFetching, sessionsQuery.isLoading])

  const seats = useMemo<AgentSeat[]>(() => [
    {
      key: "service-desk",
      name: "IT 服务台",
      role: serviceDeskAgentId > 0 ? "已上岗" : "未上岗",
      status: serviceDeskAgentId > 0 ? "online" : "offline",
      icon: Bot,
    },
    { key: "incident", name: "事件指挥官", role: "待上岗", status: "pending", icon: ShieldAlert },
    { key: "change", name: "变更协同官", role: "待上岗", status: "pending", icon: GitPullRequestArrow },
    { key: "problem", name: "问题分析师", role: "待上岗", status: "pending", icon: SearchCheck },
  ], [serviceDeskAgentId])

  return (
    <div className="flex h-[calc(100vh-3.5rem)] flex-col overflow-hidden bg-background">
      <AgentDock seats={seats} activeKey="service-desk" />
      {serviceDeskAgentId > 0 && (
        <SessionStrip
          sessions={sessions}
          activeSessionId={activeSessionId}
          creating={createSessionMutation.isPending}
          onSelect={setSelectedSessionId}
          onCreate={() => createSessionMutation.mutate()}
        />
      )}
      {serviceDeskAgentId <= 0 || configLoading ? (
        <NotOnDutyState loading={configLoading} />
      ) : activeSession ? (
        <ServiceDeskConversation
          key={activeSession.id}
          session={activeSession}
          agentName={serviceDeskAgentName}
        />
      ) : (
        <div className="flex flex-1 items-center justify-center text-sm text-muted-foreground">
          <Loader2 className="mr-2 size-4 animate-spin" />
          启动服务台
        </div>
      )}
    </div>
  )
}
