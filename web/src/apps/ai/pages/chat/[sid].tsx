import { useCallback, useMemo, useRef, useState, useEffect } from "react"
import { useParams, useNavigate } from "react-router"
import { useTranslation } from "react-i18next"
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query"
import { Square, Trash2, Brain, PanelLeft, PanelLeftClose, Paperclip, AlertTriangle, RotateCcw, Play, X } from "lucide-react"
import { sessionApi, type SessionMessage as SessionMsg } from "@/lib/api"
import { toast } from "sonner"
import { Button } from "@/components/ui/button"
import { Badge } from "@/components/ui/badge"
import {
  AlertDialog, AlertDialogAction, AlertDialogCancel, AlertDialogContent,
  AlertDialogDescription, AlertDialogFooter, AlertDialogHeader, AlertDialogTitle, AlertDialogTrigger,
} from "@/components/ui/alert-dialog"
import { useChatStream, type ChatEvent } from "./hooks/use-chat-stream"
import { QAPair, AIResponse } from "./components/message-item"
import { ThinkingBlock } from "./components/thinking-block"
import { PlanProgress } from "./components/plan-progress"
import { WelcomeScreen } from "./components/welcome-screen"
import { SessionSidebar } from "./components/session-sidebar"
import { MemoryPanel } from "./components/memory-panel"

const SIDEBAR_COLLAPSED_KEY = "ai-chat-sidebar-collapsed"

interface StreamState {
  text: string
  thinkingText: string
  thinkingDone: boolean
  thinkingDurationMs: number
  planSteps: { description: string; durationMs?: number }[]
  planStepIndex: number
  doneMetrics: { durationMs?: number; inputTokens?: number; outputTokens?: number } | null
  cancelled: boolean
  error: string | null
}

const INITIAL_STREAM_STATE: StreamState = {
  text: "",
  thinkingText: "",
  thinkingDone: false,
  thinkingDurationMs: 0,
  planSteps: [],
  planStepIndex: -1,
  doneMetrics: null,
  cancelled: false,
  error: null,
}

interface PendingImage {
  file: File
  preview: string
  uploading?: boolean
}

export function Component() {
  const groupMessagesIntoPairs = useCallback((messages: SessionMsg[]): Array<{
    userMessage: SessionMsg
    aiMessage?: SessionMsg
    tools: SessionMsg[]
  }> => {
    const pairs: Array<{ userMessage: SessionMsg; aiMessage?: SessionMsg; tools: SessionMsg[] }> = []
    let currentPair: { userMessage?: SessionMsg; aiMessage?: SessionMsg; tools: SessionMsg[] } = { tools: [] }

    for (const msg of messages) {
      if (msg.role === "user") {
        if (currentPair.userMessage) {
          pairs.push({
            userMessage: currentPair.userMessage,
            aiMessage: currentPair.aiMessage,
            tools: currentPair.tools,
          })
        }
        currentPair = { userMessage: msg, tools: [] }
      } else if (msg.role === "assistant") {
        currentPair.aiMessage = msg
      } else if (msg.role === "tool_call" || msg.role === "tool_result") {
        currentPair.tools.push(msg)
      }
    }

    if (currentPair.userMessage) {
      pairs.push({
        userMessage: currentPair.userMessage,
        aiMessage: currentPair.aiMessage,
        tools: currentPair.tools,
      })
    }

    return pairs
  }, [])

  const { sid } = useParams<{ sid: string }>()
  const sessionId = Number(sid)
  const { t } = useTranslation(["ai", "common"])
  const navigate = useNavigate()
  const queryClient = useQueryClient()
  const [input, setInput] = useState("")
  const [pendingMessages, setPendingMessages] = useState<SessionMsg[]>([])
  const [stream, setStream] = useState<StreamState>(INITIAL_STREAM_STATE)
  const [memoryOpen, setMemoryOpen] = useState(false)
  const [sidebarCollapsed, setSidebarCollapsed] = useState(() => {
    const saved = localStorage.getItem(SIDEBAR_COLLAPSED_KEY)
    return saved ? saved === "true" : false
  })
  const [pendingImages, setPendingImages] = useState<PendingImage[]>([])
  const scrollRef = useRef<HTMLDivElement>(null)
  const textareaRef = useRef<HTMLTextAreaElement>(null)
  const messagesEndRef = useRef<HTMLDivElement>(null)

  const { data: sessionData, isLoading } = useQuery({
    queryKey: ["ai-session", sessionId],
    queryFn: () => sessionApi.get(sessionId),
    enabled: !!sessionId,
  })

  const messages = useMemo(() => {
    const base = sessionData?.messages ?? []
    const baseIds = new Set(base.map(m => m.id))
    const uniquePending = pendingMessages.filter(m => !baseIds.has(m.id))
    return [...base, ...uniquePending]
  }, [sessionData, pendingMessages])

  const qaPairs = useMemo(() => {
    return groupMessagesIntoPairs(messages)
  }, [messages, groupMessagesIntoPairs])

  const scrollToBottom = useCallback(() => {
    messagesEndRef.current?.scrollIntoView({ behavior: "smooth" })
  }, [])

  const handleEvent = useCallback((event: ChatEvent) => {
    if (event.type === "content_delta" && event.text) {
      setStream(prev => ({ ...prev, text: prev.text + event.text }))
    } else if (event.type === "thinking_delta" && event.text) {
      setStream(prev => ({ ...prev, thinkingText: prev.thinkingText + event.text }))
    } else if (event.type === "thinking_done") {
      setStream(prev => ({
        ...prev,
        thinkingDone: true,
        thinkingDurationMs: event.durationMs ?? 0,
      }))
    } else if (event.type === "plan_steps" && event.steps) {
      setStream(prev => ({
        ...prev,
        planSteps: event.steps!.map(s => ({ description: s.description })),
        planStepIndex: 0,
      }))
    } else if (event.type === "step_start" && event.stepIndex != null) {
      setStream(prev => ({ ...prev, planStepIndex: event.stepIndex! }))
    } else if (event.type === "step_done" && event.stepIndex != null) {
      setStream(prev => {
        const steps = [...prev.planSteps]
        if (steps[event.stepIndex!]) {
          steps[event.stepIndex!] = { ...steps[event.stepIndex!], durationMs: event.durationMs }
        }
        return { ...prev, planSteps: steps, planStepIndex: event.stepIndex! + 1 }
      })
    }
  }, [])

  const handleDone = useCallback((event: ChatEvent) => {
    setStream(prev => ({
      ...prev,
      doneMetrics: {
        durationMs: event.durationMs,
        inputTokens: event.inputTokens,
        outputTokens: event.outputTokens,
      },
    }))
    setPendingMessages([])
    queryClient.invalidateQueries({ queryKey: ["ai-session", sessionId] })
    // Reset stream state after query refresh, keeping metrics for last message
    setTimeout(() => {
      setStream(INITIAL_STREAM_STATE)
    }, 100)
  }, [queryClient, sessionId])

  const handleStreamError = useCallback((msg: string) => {
    setStream(prev => ({ ...prev, error: msg }))
    setPendingMessages([])
    queryClient.invalidateQueries({ queryKey: ["ai-session", sessionId] })
  }, [queryClient, sessionId])

  const { isStreaming, connect, disconnect } = useChatStream({
    onEvent: handleEvent,
    onDone: handleDone,
    onError: handleStreamError,
  })

  const uploadImageMutation = useMutation({
    mutationFn: (file: File) => sessionApi.uploadMessageImage(sessionId, file),
    onError: (err) => toast.error(err.message),
  })

  const sendMutation = useMutation({
    mutationFn: async (content: string) => {
      // Upload pending images first
      const imageUrls: string[] = []
      for (const img of pendingImages) {
        if (!img.uploading) {
          const res = await uploadImageMutation.mutateAsync(img.file)
          imageUrls.push(res.url)
        }
      }
      return sessionApi.sendMessage(sessionId, content, imageUrls)
    },
    onSuccess: (msg) => {
      setPendingMessages(prev => [...prev, msg])
      setInput("")
      setPendingImages([]) // Clear pending images after sending
      setStream(INITIAL_STREAM_STATE)
      connect(sessionId)
      scrollToBottom()
    },
    onError: (err) => toast.error(err.message),
  })

  const cancelMutation = useMutation({
    mutationFn: () => sessionApi.cancel(sessionId),
    onSuccess: () => {
      disconnect()
      setStream(prev => ({ ...prev, cancelled: true }))
      queryClient.invalidateQueries({ queryKey: ["ai-session", sessionId] })
    },
  })

  const deleteMutation = useMutation({
    mutationFn: () => sessionApi.delete(sessionId),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["ai-sessions"] })
      toast.success(t("ai:chat.sessionDeleted"))

      // 获取同 Agent 的其他会话（排除被删除的），导航到下一个
      const currentData = queryClient.getQueryData<{ items: Array<{ id: number }> }>(
        ["ai-sessions", agentId]
      )
      const otherSession = currentData?.items.find(s => s.id !== sessionId)

      if (otherSession) {
        navigate(`/ai/chat/${otherSession.id}`)
      } else {
        navigate("/ai/chat")
      }
    },
    onError: (err) => toast.error(err.message),
  })

  const editMessageMutation = useMutation({
    mutationFn: ({ mid, content }: { mid: number; content: string }) =>
      sessionApi.editMessage(sessionId, mid, content),
    onSuccess: () => {
      setPendingMessages([])
      setStream(INITIAL_STREAM_STATE)
      queryClient.invalidateQueries({ queryKey: ["ai-session", sessionId] })
      // Trigger new SSE stream after edit
      connect(sessionId)
    },
    onError: (err) => toast.error(err.message),
  })

  const handleEditMessage = useCallback((messageId: number, content: string) => {
    editMessageMutation.mutate({ mid: messageId, content })
  }, [editMessageMutation])

  // Auto-resize textarea
  useEffect(() => {
    const textarea = textareaRef.current
    if (!textarea) return
    textarea.style.height = "auto"
    const newHeight = Math.min(textarea.scrollHeight, 200)
    textarea.style.height = `${newHeight}px`
  }, [input])

  // Scroll to bottom when streaming
  useEffect(() => {
    if (isStreaming && (stream.text || stream.thinkingText)) {
      scrollToBottom()
    }
  }, [isStreaming, stream.text, stream.thinkingText, scrollToBottom])

  // Auto scroll on messages change
  useEffect(() => {
    scrollToBottom()
  }, [qaPairs, scrollToBottom])

  function handleSend(content?: string) {
    const text = (content ?? input).trim()
    // Allow sending if there's text or pending images
    if ((!text && pendingImages.length === 0) || isStreaming || sendMutation.isPending) return
    sendMutation.mutate(text)
  }

  function handleKeyDown(e: React.KeyboardEvent) {
    if (e.key === "Enter" && !e.shiftKey) {
      e.preventDefault()
      handleSend()
    }
  }

  function handlePaste(e: React.ClipboardEvent) {
    const items = e.clipboardData.items
    const imageFiles: File[] = []

    for (let i = 0; i < items.length; i++) {
      const item = items[i]
      if (item.type.startsWith("image/")) {
        const file = item.getAsFile()
        if (file) {
          imageFiles.push(file)
        }
      }
    }

    if (imageFiles.length > 0) {
      e.preventDefault()
      for (const file of imageFiles) {
        const reader = new FileReader()
        reader.onload = (event) => {
          const preview = event.target?.result as string
          setPendingImages(prev => [...prev, { file, preview }])
        }
        reader.readAsDataURL(file)
      }
    }
  }

  function removePendingImage(index: number) {
    setPendingImages(prev => {
      const newImages = [...prev]
      // Revoke object URL if created (for memory cleanup)
      // Note: data URLs don't need revocation, but blob URLs would
      newImages.splice(index, 1)
      return newImages
    })
  }

  function handleRetry() {
    setStream(INITIAL_STREAM_STATE)
    connect(sessionId)
  }

  const toggleSidebar = useCallback(() => {
    setSidebarCollapsed(prev => {
      const newValue = !prev
      localStorage.setItem(SIDEBAR_COLLAPSED_KEY, String(newValue))
      return newValue
    })
  }, [])

  if (isLoading) {
    return <div className="flex items-center justify-center h-full text-muted-foreground">{t("common:loading")}</div>
  }

  const session = sessionData?.session
  const agentId = session?.agentId
  const agentName = (session as Record<string, unknown>)?.agentName as string | undefined
  const hasMessages = messages.length > 0 || isStreaming
  const showWelcome = !hasMessages && !isStreaming

  return (
    <div className="flex h-full overflow-hidden">
      {/* Sidebar */}
      <SessionSidebar
        agentId={agentId}
        currentSessionId={sessionId}
        collapsed={sidebarCollapsed}
      />

      {/* Main chat area */}
      <div className="flex-1 flex flex-col min-w-0 bg-background h-full">
        {/* Header */}
        <div className="flex items-center justify-between border-b px-4 py-2 shrink-0 h-12">
          <div className="flex items-center gap-2">
            <Button variant="ghost" size="sm" className="h-8 w-8 p-0" onClick={toggleSidebar}>
              {sidebarCollapsed ? <PanelLeft className="h-4 w-4" /> : <PanelLeftClose className="h-4 w-4" />}
            </Button>
            <h3 className="font-medium truncate">{session?.title || t("ai:chat.newChat")}</h3>
            {session?.status && (
              <Badge variant="outline" className="text-xs">{t(`ai:chat.sessionStatus.${session.status}`)}</Badge>
            )}
          </div>
          <div className="flex items-center gap-1">
            {agentId && (
              <Button variant="ghost" size="sm" onClick={() => setMemoryOpen(!memoryOpen)}>
                <Brain className="h-4 w-4" />
              </Button>
            )}
            <AlertDialog>
              <AlertDialogTrigger asChild>
                <Button variant="ghost" size="sm" className="text-destructive hover:text-destructive">
                  <Trash2 className="h-4 w-4" />
                </Button>
              </AlertDialogTrigger>
              <AlertDialogContent>
                <AlertDialogHeader>
                  <AlertDialogTitle>{t("ai:chat.deleteSession")}</AlertDialogTitle>
                  <AlertDialogDescription>{t("ai:chat.deleteSessionDesc")}</AlertDialogDescription>
                </AlertDialogHeader>
                <AlertDialogFooter>
                  <AlertDialogCancel>{t("common:cancel")}</AlertDialogCancel>
                  <AlertDialogAction onClick={() => deleteMutation.mutate()} disabled={deleteMutation.isPending}>
                    {t("common:delete")}
                  </AlertDialogAction>
                </AlertDialogFooter>
              </AlertDialogContent>
            </AlertDialog>
          </div>
        </div>

        {/* Messages area */}
        <div ref={scrollRef} className="flex-1 overflow-y-auto overflow-x-hidden min-h-0">
          {showWelcome ? (
            <WelcomeScreen
              agentName={agentName ?? session?.title}
              agentType={(session as Record<string, unknown>)?.agentType as string | undefined}
              onPromptClick={(prompt) => handleSend(prompt)}
            />
          ) : (
            <div className="max-w-3xl mx-auto px-4 pb-4">
              {/* Rendered QA pairs */}
              {qaPairs.map((pair) => (
                <QAPair
                  key={pair.userMessage.id}
                  userMessage={pair.userMessage}
                  aiMessage={pair.aiMessage}
                  tools={pair.tools}
                  agentName={agentName}
                  onEditMessage={handleEditMessage}
                />
              ))}

              {/* Streaming area */}
              {isStreaming && (
                <div className="py-6">
                  {/* Thinking block */}
                  {stream.thinkingText && (
                    <ThinkingBlock
                      content={stream.thinkingText}
                      isStreaming={!stream.thinkingDone}
                      durationMs={stream.thinkingDurationMs}
                    />
                  )}

                  {/* Plan progress */}
                  {stream.planSteps.length > 0 && (
                    <PlanProgress
                      steps={stream.planSteps}
                      currentStepIndex={stream.planStepIndex}
                      isStreaming={isStreaming}
                    />
                  )}

                  {/* Streaming AI response */}
                  {stream.text && (
                    <AIResponse
                      content={stream.text}
                      agentName={agentName}
                      isStreaming
                    />
                  )}

                  {/* Initial "thinking" indicator before any text */}
                  {!stream.text && !stream.thinkingText && (
                    <div className="flex items-center gap-2 text-sm text-muted-foreground">
                      {agentName && <span className="text-xs font-medium">{agentName}</span>}
                      <span className="flex gap-1">
                        <span className="h-1.5 w-1.5 rounded-full bg-foreground/40 animate-bounce [animation-delay:0ms]" />
                        <span className="h-1.5 w-1.5 rounded-full bg-foreground/40 animate-bounce [animation-delay:150ms]" />
                        <span className="h-1.5 w-1.5 rounded-full bg-foreground/40 animate-bounce [animation-delay:300ms]" />
                      </span>
                    </div>
                  )}
                </div>
              )}

              {/* Cancelled state — preserved content */}
              {stream.cancelled && stream.text && !isStreaming && (
                <div className="py-6">
                  <AIResponse content={stream.text} agentName={agentName} />
                  <div className="flex items-center gap-3 mt-2 p-3 rounded-lg border border-amber-200 dark:border-amber-800 bg-amber-50 dark:bg-amber-950/30">
                    <AlertTriangle className="h-4 w-4 text-amber-500 shrink-0" />
                    <span className="text-sm text-amber-700 dark:text-amber-400">{t("ai:chat.stopped")}</span>
                    <div className="flex items-center gap-2 ml-auto">
                      <Button variant="outline" size="sm" onClick={() => { setStream(INITIAL_STREAM_STATE); connect(sessionId) }}>
                        <Play className="h-3.5 w-3.5 mr-1" />
                        {t("ai:chat.continueGenerating")}
                      </Button>
                      <Button variant="outline" size="sm" onClick={handleRetry}>
                        <RotateCcw className="h-3.5 w-3.5 mr-1" />
                        {t("ai:chat.regenerate")}
                      </Button>
                    </div>
                  </div>
                </div>
              )}

              {/* Inline error */}
              {stream.error && !isStreaming && (
                <div className="py-6">
                  <div className="flex items-center gap-3 p-3 rounded-lg border-l-4 border-destructive bg-destructive/5">
                    <AlertTriangle className="h-4 w-4 text-destructive shrink-0" />
                    <div className="flex-1">
                      <div className="text-sm font-medium text-destructive">{t("ai:chat.generationError")}</div>
                      <div className="text-xs text-muted-foreground mt-0.5">{stream.error}</div>
                    </div>
                    <Button variant="outline" size="sm" onClick={handleRetry}>
                      <RotateCcw className="h-3.5 w-3.5 mr-1" />
                      {t("ai:chat.retry")}
                    </Button>
                  </div>
                </div>
              )}

              <div ref={messagesEndRef} />
            </div>
          )}
        </div>

        {/* Centered stop button during streaming */}
        {isStreaming && (
          <div className="flex justify-center pb-2 shrink-0">
            <Button
              variant="outline"
              size="sm"
              className="rounded-full px-4"
              onClick={() => cancelMutation.mutate()}
            >
              <Square className="h-3.5 w-3.5 mr-1.5" />
              {t("ai:chat.cancel")}
            </Button>
          </div>
        )}

        {/* Input area — floating card */}
        <div className="px-4 pb-3 pt-1 shrink-0">
          <div className="max-w-3xl mx-auto">
            <div className="rounded-2xl bg-background shadow-lg border transition-colors focus-within:border-primary/30">
              <textarea
                ref={textareaRef}
                value={input}
                onChange={(e) => setInput(e.target.value)}
                onKeyDown={handleKeyDown}
                onPaste={handlePaste}
                placeholder={t("ai:chat.inputPlaceholder")}
                rows={1}
                className="w-full min-h-[44px] max-h-[200px] resize-none bg-transparent px-4 pt-3 pb-1 text-base leading-relaxed placeholder:text-muted-foreground focus:outline-none disabled:cursor-not-allowed disabled:opacity-50"
                disabled={isStreaming}
              />
              {/* Pending images preview */}
              {pendingImages.length > 0 && (
                <div className="flex gap-2 px-4 pb-2 overflow-x-auto">
                  {pendingImages.map((img, idx) => (
                    <div key={idx} className="relative group shrink-0">
                      <img
                        src={img.preview}
                        alt={`Pending ${idx}`}
                        className="h-16 w-16 object-cover rounded-md border"
                      />
                      <button
                        type="button"
                        onClick={() => removePendingImage(idx)}
                        className="absolute -top-1.5 -right-1.5 h-5 w-5 rounded-full bg-destructive text-white flex items-center justify-center opacity-0 group-hover:opacity-100 transition-opacity"
                      >
                        <X className="h-3 w-3" />
                      </button>
                    </div>
                  ))}
                </div>
              )}
              {/* Toolbar */}
              <div className="flex items-center justify-between px-3 pb-2">
                <div className="flex items-center gap-1">
                  <Button variant="ghost" size="icon" className="h-8 w-8 text-muted-foreground/40" disabled>
                    <Paperclip className="h-4 w-4" />
                  </Button>
                </div>
                <div className="flex items-center gap-2">
                  {!isStreaming && (
                    <Button
                      size="icon"
                      className="h-8 w-8 rounded-full"
                      onClick={() => handleSend()}
                      disabled={(!input.trim() && pendingImages.length === 0) || sendMutation.isPending}
                    >
                      <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" className="h-4 w-4">
                        <path d="m22 2-7 20-4-9-9-4Z" />
                        <path d="M22 2 11 13" />
                      </svg>
                    </Button>
                  )}
                </div>
              </div>
            </div>
            <p className="text-[10px] text-muted-foreground/50 text-center mt-1">
              {t("ai:chat.inputHint")}
            </p>
          </div>
        </div>
      </div>

      {/* Memory panel */}
      {memoryOpen && agentId && (
        <MemoryPanel agentId={agentId} onClose={() => setMemoryOpen(false)} />
      )}
    </div>
  )
}
