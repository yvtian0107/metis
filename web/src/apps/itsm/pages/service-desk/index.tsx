"use client"

import { useCallback, useEffect, useMemo, useRef, useState } from "react"
import { Link, useNavigate } from "react-router"
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import type { UIMessage } from "ai"
import {
  AlertTriangle,
  ArrowUpRight,
  Bot,
  FileStack,
  History,
  Loader2,
  PanelRight,
  Plus,
  RotateCw,
  Send,
  Sparkles,
  Square,
} from "lucide-react"
import { toast } from "sonner"

import { QAPair } from "@/apps/ai/pages/chat/components/message-item"
import { useAiChat } from "@/apps/ai/pages/chat/hooks/use-ai-chat"
import { sessionApi, type AgentSession } from "@/lib/api"
import { cn } from "@/lib/utils"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetHeader,
  SheetTitle,
} from "@/components/ui/sheet"
import { Textarea } from "@/components/ui/textarea"
import {
  fetchEngineConfig,
  fetchMyTickets,
  fetchServiceDeskSessionState,
  type ServiceDeskSessionState,
  type TicketItem,
} from "../../api"

const SUGGESTED_PROMPTS = [
  "我想申请 VPN，线上支持用",
  "电脑无法连接公司 Wi-Fi",
  "需要临时访问生产服务器",
  "帮我查一下我的工单进度",
]

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

function sessionTitle(session: AgentSession) {
  return session.title || `会话 #${session.id}`
}

function stageView(stage?: string) {
  switch (stage) {
    case "candidates_ready":
      return { label: "匹配服务", tone: "bg-sky-500", badge: "bg-sky-500/10 text-sky-700 border-sky-500/20" }
    case "service_selected":
      return { label: "已选服务", tone: "bg-cyan-500", badge: "bg-cyan-500/10 text-cyan-700 border-cyan-500/20" }
    case "service_loaded":
      return { label: "加载工件", tone: "bg-amber-500", badge: "bg-amber-500/10 text-amber-700 border-amber-500/20" }
    case "awaiting_confirmation":
      return { label: "等待确认", tone: "bg-violet-500", badge: "bg-violet-500/10 text-violet-700 border-violet-500/20" }
    case "confirmed":
      return { label: "已确认", tone: "bg-emerald-500", badge: "bg-emerald-500/10 text-emerald-700 border-emerald-500/20" }
    default:
      return { label: "待接入", tone: "bg-muted-foreground/40", badge: "bg-muted text-muted-foreground border-border" }
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

function hasArtifacts(stateData?: ServiceDeskSessionState, tickets?: TicketItem[]) {
  const state = stateData?.state
  return Boolean(
    (state && state.stage !== "idle") ||
    state?.draft_summary ||
    objectEntries(state?.draft_form_data).length > 0 ||
    objectEntries(state?.prefill_form_data).length > 0 ||
    (tickets && tickets.length > 0),
  )
}

function StatusDot({ className }: { className?: string }) {
  return (
    <span className={cn("relative flex size-2.5", className)}>
      <span className="absolute inline-flex size-full animate-ping rounded-full bg-emerald-400 opacity-45" />
      <span className="relative inline-flex size-2.5 rounded-full bg-emerald-500" />
    </span>
  )
}

function ServiceDeskSidebar({
  sessions,
  activeSessionId,
  loading,
  onSelect,
  onNew,
}: {
  sessions: AgentSession[]
  activeSessionId: number | null
  loading: boolean
  onSelect: (sessionId: number) => void
  onNew: () => void
}) {
  return (
    <aside className="hidden w-60 shrink-0 border-r border-border/70 bg-muted/12 md:flex md:flex-col">
      <div className="flex h-14 items-center justify-between px-4">
        <div className="flex items-center gap-2 text-sm font-medium">
          <History className="size-4 text-muted-foreground" />
          会话
        </div>
        <Button type="button" size="icon" variant="ghost" className="size-8" onClick={onNew}>
          <Plus className="size-4" />
        </Button>
      </div>
      <div className="min-h-0 flex-1 overflow-y-auto px-2 pb-3">
        {loading ? (
          <div className="flex items-center gap-2 px-3 py-3 text-xs text-muted-foreground">
            <Loader2 className="size-3.5 animate-spin" />
            载入会话
          </div>
        ) : sessions.length === 0 ? (
          <div className="px-3 py-3 text-xs leading-5 text-muted-foreground">暂无历史会话</div>
        ) : (
          <div className="space-y-1">
            {sessions.map((session) => {
              const active = session.id === activeSessionId
              return (
                <button
                  key={session.id}
                  type="button"
                  onClick={() => onSelect(session.id)}
                  className={cn(
                    "group flex w-full flex-col rounded-md px-3 py-2 text-left transition-colors",
                    active ? "bg-background text-foreground shadow-[0_10px_30px_-24px_rgba(15,23,42,0.45)]" : "text-muted-foreground hover:bg-background/70 hover:text-foreground",
                  )}
                >
                  <span className="line-clamp-2 text-sm leading-5">{sessionTitle(session)}</span>
                  <span className="mt-1 text-[11px] text-muted-foreground/75">{formatSessionTime(session.updatedAt)}</span>
                </button>
              )
            })}
          </div>
        )}
      </div>
    </aside>
  )
}

function ServiceDeskComposer({
  value,
  disabled,
  pending,
  placeholder,
  onChange,
  onSend,
  compact,
}: {
  value: string
  disabled?: boolean
  pending?: boolean
  placeholder: string
  onChange: (value: string) => void
  onSend: () => void
  compact?: boolean
}) {
  return (
    <div
      className={cn(
        "flex w-full items-end gap-2 rounded-xl border border-border/80 bg-background/92 p-2 shadow-[0_22px_55px_-46px_rgba(15,23,42,0.75)]",
        compact ? "max-w-3xl" : "max-w-[720px]",
      )}
    >
      <Textarea
        value={value}
        onChange={(event) => onChange(event.target.value)}
        onKeyDown={(event) => {
          if (event.key === "Enter" && !event.shiftKey) {
            event.preventDefault()
            onSend()
          }
        }}
        placeholder={placeholder}
        className={cn(
          "max-h-40 resize-none border-0 bg-transparent px-3 py-2.5 shadow-none focus-visible:ring-0",
          compact ? "min-h-11" : "min-h-28 text-base",
        )}
        disabled={disabled}
      />
      <Button type="button" size="icon" className="size-10 shrink-0" onClick={onSend} disabled={!value.trim() || disabled || pending}>
        {pending ? <Loader2 className="size-4 animate-spin" /> : <Send className="size-4" />}
      </Button>
    </div>
  )
}

function WelcomeStage({
  agentName,
  value,
  disabled,
  pending,
  onChange,
  onSend,
}: {
  agentName: string
  value: string
  disabled?: boolean
  pending?: boolean
  onChange: (value: string) => void
  onSend: () => void
}) {
  return (
    <div className="flex min-h-0 flex-1 flex-col items-center justify-center px-5 py-8">
      <div className="flex flex-col items-center text-center">
        <div className="flex size-16 items-center justify-center rounded-full border border-primary/20 bg-primary/8 text-primary shadow-[0_18px_44px_-34px_hsl(var(--primary))]">
          <Bot className="size-8" />
        </div>
        <div className="mt-4 flex items-center gap-2 text-sm text-muted-foreground">
          <StatusDot />
          <span>{agentName}</span>
        </div>
        <h1 className="mt-3 text-3xl font-semibold tracking-normal text-foreground">IT 服务台</h1>
        <p className="mt-3 max-w-xl text-sm leading-6 text-muted-foreground">
          直接描述 IT 诉求，服务台会澄清信息、生成草稿，并在你确认后沉淀为工单。
        </p>
      </div>
      <div className="mt-9 flex w-full flex-col items-center">
        <ServiceDeskComposer
          value={value}
          onChange={onChange}
          onSend={onSend}
          disabled={disabled}
          pending={pending}
          placeholder="描述你的 IT 诉求..."
        />
        <div className="mt-4 flex max-w-3xl flex-wrap justify-center gap-2">
          {SUGGESTED_PROMPTS.map((prompt) => (
            <button
              key={prompt}
              type="button"
              onClick={() => onChange(prompt)}
              className="rounded-full border border-border/80 bg-background/76 px-3 py-1.5 text-sm text-muted-foreground transition-colors hover:bg-accent/45 hover:text-foreground"
              disabled={disabled}
            >
              {prompt}
            </button>
          ))}
        </div>
      </div>
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
        <h2 className="mt-4 text-lg font-semibold">服务台智能体未配置</h2>
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

function ArtifactPanelContent({
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
    <div className="min-h-0 flex-1 overflow-y-auto px-4 pb-4">
      {loading && (
        <div className="mb-3 flex items-center gap-2 rounded-md border border-border/70 bg-background/70 px-3 py-2 text-sm text-muted-foreground">
          <Loader2 className="size-4 animate-spin" />
          同步工件
        </div>
      )}

      <section className="rounded-lg border border-border/70 bg-background p-3">
        <div className="flex items-center justify-between">
          <div className="text-xs font-medium text-muted-foreground">当前状态</div>
          <Badge variant="outline" className={cn("border text-xs font-normal", view.badge)}>
            {view.label}
          </Badge>
        </div>
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
  )
}

function ArtifactRail({
  stateData,
  tickets,
  loading,
}: {
  stateData?: ServiceDeskSessionState
  tickets: TicketItem[]
  loading: boolean
}) {
  const [open, setOpen] = useState(false)
  const view = stageView(stateData?.state?.stage)
  const active = hasArtifacts(stateData, tickets)

  return (
    <aside className="pointer-events-none fixed bottom-24 right-4 z-30 flex shrink-0 md:pointer-events-auto md:static md:w-14 md:border-l md:border-border/70 md:bg-muted/12 md:py-3">
      <button
        type="button"
        onClick={() => setOpen(true)}
        className="pointer-events-auto group flex w-10 flex-col items-center gap-2 rounded-lg border border-border/70 bg-background/88 px-2 py-3 text-muted-foreground shadow-[0_18px_45px_-32px_rgba(15,23,42,0.85)] backdrop-blur transition-colors hover:bg-background hover:text-foreground"
      >
        <span className="relative">
          <FileStack className="size-4" />
          {active && <span className={cn("absolute -right-1 -top-1 size-2 rounded-full", view.tone)} />}
        </span>
        <span className="text-[11px] [writing-mode:vertical-rl]">工件</span>
      </button>
      {loading && <Loader2 className="mt-3 size-3.5 animate-spin text-muted-foreground" />}
      <Sheet open={open} onOpenChange={setOpen}>
        <SheetContent
          side="bottom"
          className="h-[78vh] gap-0 rounded-t-2xl md:inset-y-0 md:left-auto md:right-0 md:bottom-auto md:h-full md:w-[390px] md:max-w-none md:rounded-none md:border-l md:border-t-0"
        >
          <SheetHeader className="border-b border-border/70 px-4 py-4">
            <SheetTitle className="flex items-center gap-2 text-base">
              <PanelRight className="size-4 text-muted-foreground" />
              ITSM 工件
            </SheetTitle>
            <SheetDescription>{expectedActionLabel(stateData?.nextExpectedAction)}</SheetDescription>
          </SheetHeader>
          <ArtifactPanelContent stateData={stateData} tickets={tickets} loading={loading} />
        </SheetContent>
      </Sheet>
    </aside>
  )
}

function ServiceDeskConversation({
  session,
  agentName,
  initialPrompt,
  onInitialPromptSent,
}: {
  session: AgentSession
  agentName: string
  initialPrompt?: string
  onInitialPromptSent: () => void
}) {
  const queryClient = useQueryClient()
  const [input, setInput] = useState("")
  const scrollRef = useRef<HTMLDivElement>(null)
  const textareaRef = useRef<HTMLTextAreaElement>(null)
  const messagesEndRef = useRef<HTMLDivElement>(null)
  const initialPromptSentRef = useRef(false)

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
    if (!initialPrompt || initialPromptSentRef.current || isLoading) return
    initialPromptSentRef.current = true
    chat.sendMessage({ text: initialPrompt })
    onInitialPromptSent()
  }, [chat, initialPrompt, isLoading, onInitialPromptSent])

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

  const showEmpty = !isLoading && qaPairs.length === 0 && !initialPrompt

  return (
    <>
      <main className="flex min-w-0 flex-1 flex-col bg-background">
        <div className="flex h-14 shrink-0 items-center justify-between border-b border-border/70 px-5">
          <div className="flex min-w-0 items-center gap-3">
            <div className="flex size-8 shrink-0 items-center justify-center rounded-full border border-primary/20 bg-primary/8 text-primary">
              <Bot className="size-4" />
            </div>
            <div className="min-w-0">
              <div className="flex items-center gap-2">
                <h1 className="truncate text-sm font-semibold">IT 服务台</h1>
                <StatusDot />
              </div>
              <div className="mt-0.5 truncate text-xs text-muted-foreground">{agentName} · {formatSessionTime(session.updatedAt)}</div>
            </div>
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
              <h2 className="mt-5 text-2xl font-semibold">继续描述 IT 诉求</h2>
              <p className="mt-2 max-w-xl text-sm leading-6 text-muted-foreground">
                服务台会沿用当前会话上下文继续澄清、填槽和创建工单。
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
          <div className="mx-auto max-w-3xl">
            <ServiceDeskComposer
              value={input}
              onChange={setInput}
              onSend={handleSend}
              disabled={isBusy}
              pending={sendMutation.isPending}
              placeholder="描述你的 IT 诉求..."
              compact
            />
          </div>
        </div>
      </main>

      <ArtifactRail
        stateData={stateQuery.data}
        tickets={ticketsQuery.data ?? []}
        loading={stateQuery.isLoading || ticketsQuery.isLoading}
      />
    </>
  )
}

export function Component() {
  const queryClient = useQueryClient()
  const [selectedSessionId, setSelectedSessionId] = useState<number | null>(null)
  const [createdSession, setCreatedSession] = useState<AgentSession | null>(null)
  const [landingInput, setLandingInput] = useState("")
  const [pendingInitialPrompt, setPendingInitialPrompt] = useState<{ sessionId: number; text: string } | null>(null)

  const { data: config, isLoading: configLoading } = useQuery({
    queryKey: ["itsm-engine-config"],
    queryFn: fetchEngineConfig,
  })

  const serviceDeskAgentId = config?.servicedesk?.agentId ?? 0
  const serviceDeskAgentName = config?.servicedesk?.agentName || "IT 服务台"

  const sessionsQuery = useQuery({
    queryKey: ["ai-sessions", serviceDeskAgentId],
    queryFn: () => sessionApi.list({ agentId: serviceDeskAgentId, page: 1, pageSize: 30 }),
    enabled: serviceDeskAgentId > 0,
  })

  const sessions = sessionsQuery.data?.items ?? []
  const activeSession = selectedSessionId == null
    ? null
    : sessions.find((item) => item.id === selectedSessionId) ?? (createdSession?.id === selectedSessionId ? createdSession : null)

  const createSessionMutation = useMutation({
    mutationFn: async (text: string) => {
      const session = await sessionApi.create(serviceDeskAgentId)
      return { session, text }
    },
    onSuccess: ({ session, text }) => {
      setCreatedSession(session)
      setSelectedSessionId(session.id)
      setPendingInitialPrompt({ sessionId: session.id, text })
      setLandingInput("")
      queryClient.invalidateQueries({ queryKey: ["ai-sessions", serviceDeskAgentId] })
    },
    onError: (err) => toast.error(err.message),
  })

  const handleLandingSend = useCallback(() => {
    const text = landingInput.trim()
    if (!text || serviceDeskAgentId <= 0 || createSessionMutation.isPending) return
    createSessionMutation.mutate(text)
  }, [createSessionMutation, landingInput, serviceDeskAgentId])

  const handleSelectSession = useCallback((sessionId: number) => {
    setSelectedSessionId(sessionId)
    setCreatedSession(null)
    setPendingInitialPrompt(null)
  }, [])

  const handleNewSession = useCallback(() => {
    setSelectedSessionId(null)
    setCreatedSession(null)
    setPendingInitialPrompt(null)
    setLandingInput("")
  }, [])

  const clearPendingInitialPrompt = useCallback(() => {
    setPendingInitialPrompt(null)
  }, [])

  return (
    <div className="grid h-[calc(100vh-3.5rem)] grid-cols-1 overflow-hidden bg-[linear-gradient(180deg,hsl(var(--background)),hsl(var(--muted)/0.18))] md:grid-cols-[240px_minmax(0,1fr)_56px]">
      <ServiceDeskSidebar
        sessions={sessions}
        activeSessionId={activeSession?.id ?? null}
        loading={sessionsQuery.isLoading}
        onSelect={handleSelectSession}
        onNew={handleNewSession}
      />

      {serviceDeskAgentId <= 0 || configLoading ? (
        <main className="flex min-w-0 flex-1 flex-col">
          <NotOnDutyState loading={configLoading} />
        </main>
      ) : activeSession ? (
        <ServiceDeskConversation
          key={activeSession.id}
          session={activeSession}
          agentName={serviceDeskAgentName}
          initialPrompt={pendingInitialPrompt?.sessionId === activeSession.id ? pendingInitialPrompt.text : undefined}
          onInitialPromptSent={clearPendingInitialPrompt}
        />
      ) : (
        <>
          <main className="flex min-w-0 flex-1 flex-col">
            <div className="flex h-14 shrink-0 items-center justify-between border-b border-border/70 px-5">
              <div className="flex min-w-0 items-center gap-3">
                <div className="flex size-8 shrink-0 items-center justify-center rounded-full border border-primary/20 bg-primary/8 text-primary">
                  <Sparkles className="size-4" />
                </div>
                <div className="min-w-0">
                  <div className="flex items-center gap-2">
                    <h1 className="truncate text-sm font-semibold">IT 服务台</h1>
                    <StatusDot />
                  </div>
                  <div className="mt-0.5 truncate text-xs text-muted-foreground">{serviceDeskAgentName}</div>
                </div>
              </div>
              <Button type="button" size="sm" variant="outline" className="md:hidden" onClick={handleNewSession}>
                <Plus className="mr-1.5 size-3.5" />
                新会话
              </Button>
            </div>
            <WelcomeStage
              agentName={serviceDeskAgentName}
              value={landingInput}
              onChange={setLandingInput}
              onSend={handleLandingSend}
              disabled={createSessionMutation.isPending}
              pending={createSessionMutation.isPending}
            />
          </main>
          <aside className="hidden w-14 shrink-0 border-l border-border/70 bg-muted/12 md:flex md:flex-col md:items-center md:py-3">
            <div className="flex w-10 flex-col items-center gap-2 rounded-lg border border-border/60 bg-background/45 px-2 py-3 text-muted-foreground/50">
              <FileStack className="size-4" />
              <span className="text-[11px] [writing-mode:vertical-rl]">工件</span>
            </div>
          </aside>
        </>
      )}
    </div>
  )
}
