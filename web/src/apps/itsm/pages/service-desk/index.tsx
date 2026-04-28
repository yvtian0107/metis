"use client"

import { useCallback, useEffect, useMemo, useRef, useState } from "react"
import { useNavigate } from "react-router"
import { useInfiniteQuery, useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import { AssistantRuntimeProvider } from "@assistant-ui/react"
import type { UIMessage } from "ai"
import {
  AlertTriangle,
  Bot,
  CheckCircle2,
  FileCheck2,
  Loader2,
  MessageSquarePlus,
  Sparkles,
} from "lucide-react"
import { toast } from "sonner"

import {
  ChatComposer,
  ChatHeader,
  ChatStatusDot,
  ChatWorkspace,
  SessionSidebar,
  type ChatComposerImage,
  type ChatWorkspaceSurfaceRenderer,
} from "@/components/chat-workspace"
import { sessionApi, type AgentSession } from "@/lib/api"
import { Button } from "@/components/ui/button"
import { FormRenderer, type FormSchema } from "../../components/form-engine"
import {
  fetchSmartStaffingConfig,
  submitServiceDeskDraft,
  type AgenticUISurface,
  type ITSMDraftFormSurface,
  type ITSMDraftFormSurfacePayload,
} from "../../api"
import { useServiceDeskChat } from "./use-service-desk-chat"

const SUGGESTED_PROMPTS = [
  "我想申请 VPN，线上支持用",
  "电脑无法连接公司 Wi-Fi",
  "需要临时访问生产服务器",
  "帮我查一下我的工单进度",
]

function addImagePreviews(files: File[], onAdd: (image: ChatComposerImage) => void) {
  for (const file of files) {
    const reader = new FileReader()
    reader.onload = (event) => {
      onAdd({ file, preview: String(event.target?.result ?? "") })
    }
    reader.readAsDataURL(file)
  }
}

function toImageFileParts(urls: string[]) {
  return urls.map((url) => ({
    type: "file" as const,
    mediaType: "image/*",
    url,
  }))
}

function WelcomeStage({
  agentName,
  value,
  images,
  disabled,
  pending,
  onChange,
  onSend,
  onAddImages,
  onRemoveImage,
}: {
  agentName: string
  value: string
  images: ChatComposerImage[]
  disabled?: boolean
  pending?: boolean
  onChange: (value: string) => void
  onSend: () => void
  onAddImages: (files: File[]) => void
  onRemoveImage: (index: number) => void
}) {
  return (
    <div className="flex min-h-0 flex-1 flex-col items-center justify-start px-5 pb-8 pt-[10vh]">
      <div className="flex flex-col items-center text-center">
        <div className="flex size-16 items-center justify-center rounded-full border border-primary/20 bg-primary/8 text-primary shadow-[0_18px_44px_-34px_hsl(var(--primary))]">
          <Bot className="size-8" />
        </div>
        <div className="mt-4 flex items-center gap-2 text-sm text-muted-foreground">
          <ChatStatusDot />
          <span>{agentName}</span>
        </div>
        <h1 className="mt-3 text-3xl font-semibold tracking-normal text-foreground">IT 服务台</h1>
        <p className="mt-3 max-w-xl text-sm leading-6 text-muted-foreground">
          直接描述 IT 诉求，服务台会澄清信息、生成草稿，并在你确认后沉淀为工单。
        </p>
      </div>
      <div className="mt-9 flex w-full flex-col items-center">
        <ChatComposer
          value={value}
          onChange={onChange}
          onSend={onSend}
          onPasteImages={onAddImages}
          onPickImages={onAddImages}
          onRemoveImage={onRemoveImage}
          images={images}
          disabled={disabled}
          pending={pending}
          allowImages
          placeholder="描述你的 IT 诉求..."
          variant="stage"
          maxWidth="standard"
          minRows={3}
          showToolbarHint
          attachmentTone="service-desk"
        />
        <div className="mt-3 flex max-w-3xl flex-wrap justify-center gap-2">
          {SUGGESTED_PROMPTS.map((prompt) => (
            <button
              key={prompt}
              type="button"
              onClick={() => onChange(prompt)}
              className="rounded-full border border-border/70 bg-background/76 px-3 py-1.5 text-sm text-muted-foreground transition-colors hover:border-primary/25 hover:bg-primary/8 hover:text-foreground"
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
          需要在智能岗位中为服务受理岗安排智能体。
        </p>
        <Button className="mt-5" onClick={() => navigate("/itsm/smart-staffing")}>
          前往智能岗位
        </Button>
      </div>
    </div>
  )
}

function readDraftSurface(part: UIMessage["parts"][number]): ITSMDraftFormSurface | null {
  if (part.type !== "data-ui-surface") return null
  const data = (part as { data?: unknown }).data
  if (!data || typeof data !== "object") return null
  const surface = data as AgenticUISurface<ITSMDraftFormSurfacePayload>
  if (surface.surfaceType !== "itsm.draft_form") return null
  if (!surface.payload || typeof surface.payload !== "object") return null
  return surface
}

function isFormSchema(schema: unknown): schema is FormSchema {
  return Boolean(schema && typeof schema === "object" && Array.isArray((schema as FormSchema).fields))
}

function ServiceDeskDataPart({
  part,
  sessionId,
  onSubmitted,
}: {
  part: UIMessage["parts"][number]
  sessionId: number
  onSubmitted: () => void
}) {
  const surface = readDraftSurface(part)
  if (!surface) return null
  return (
    <ITSMDraftFormSurfaceCard
      key={`${surface.surfaceId}:${surface.payload.status}:${surface.payload.draftVersion ?? ""}`}
      surface={surface}
      sessionId={sessionId}
      onSubmitted={onSubmitted}
    />
  )
}

function ITSMDraftFormSurfaceCard({
  surface,
  sessionId,
  onSubmitted,
}: {
  surface: ITSMDraftFormSurface
  sessionId: number
  onSubmitted: () => void
}) {
  const payload = surface.payload
  const initialFormData = useMemo(() => payload.values ?? {}, [payload.values])
  const [formData, setFormData] = useState<Record<string, unknown>>(payload.values ?? {})
  const [submittedSurface, setSubmittedSurface] = useState<ITSMDraftFormSurface | null>(null)
  const [inlineError, setInlineError] = useState<string | null>(null)

  const submitMutation = useMutation({
    mutationFn: () => {
      if (!payload.draftVersion) {
        throw new Error("草稿版本缺失，请重新整理草稿")
      }
      return submitServiceDeskDraft(sessionId, {
        draftVersion: payload.draftVersion,
        summary: payload.summary ?? payload.title ?? "",
        formData,
      })
    },
    onSuccess: (result) => {
      if (!result.ok) {
        setInlineError(result.guidance || result.failureReason || result.message || "提交失败")
        return
      }
      if (result.surface?.surfaceType === "itsm.draft_form") {
        setSubmittedSurface(result.surface as ITSMDraftFormSurface)
      } else {
        setSubmittedSurface({
          surfaceId: `${surface.surfaceId}:submitted`,
          surfaceType: "itsm.draft_form",
          payload: {
            status: "submitted",
            title: payload.title,
            summary: payload.summary,
            values: formData,
            ticketId: result.ticketId,
            ticketCode: result.ticketCode,
            message: result.message,
          },
        })
      }
      onSubmitted()
    },
    onError: (err) => setInlineError(err.message),
  })

  const currentSurface = submittedSurface ?? surface
  const currentPayload = currentSurface.payload

  if (currentPayload.status === "loading") {
    return (
      <div data-testid="itsm-draft-form-surface" className="mb-5 max-w-[720px] rounded-2xl border border-border/65 bg-background/82 p-4 shadow-[0_18px_52px_-44px_hsl(var(--foreground))]">
        <div className="flex items-center gap-3">
          <div className="flex size-9 items-center justify-center rounded-full border border-primary/20 bg-primary/8 text-primary">
            <Loader2 className="size-4 animate-spin" />
          </div>
          <div>
            <div className="text-xs font-medium text-muted-foreground">服务申请草稿</div>
            <div className="mt-0.5 text-sm font-semibold text-foreground">{currentPayload.title || "正在整理草稿"}</div>
          </div>
        </div>
        <div className="mt-3 space-y-2">
          <div className="h-2 w-2/3 animate-pulse rounded bg-muted" />
          <div className="h-2 w-5/6 animate-pulse rounded bg-muted" />
          <div className="h-2 w-1/2 animate-pulse rounded bg-muted" />
        </div>
      </div>
    )
  }

  if (currentPayload.status === "submitted") {
    return (
      <div data-testid="itsm-draft-form-surface" className="mb-5 max-w-[720px] rounded-2xl border border-emerald-500/25 bg-emerald-500/6 p-4">
        <div className="flex items-start gap-3">
          <div className="flex size-9 shrink-0 items-center justify-center rounded-full border border-emerald-500/25 bg-background text-emerald-600 dark:text-emerald-300">
            <CheckCircle2 className="size-4" />
          </div>
          <div className="min-w-0">
            <div className="text-sm font-semibold text-emerald-700 dark:text-emerald-300">
              {currentPayload.message || "工单已提交"}
            </div>
            <div className="mt-1 text-xs leading-5 text-muted-foreground">
              服务台已保存申请内容，后续可在我的工单中跟进处理进度。
            </div>
          </div>
        </div>
        {currentPayload.ticketCode && (
          <div data-testid="itsm-submitted-ticket-code" className="mt-3 inline-flex items-center rounded-full border border-emerald-500/20 bg-background/70 px-3 py-1 text-sm text-foreground">
            工单编号：<span className="ml-1 font-semibold">{currentPayload.ticketCode}</span>
          </div>
        )}
      </div>
    )
  }

  if (!isFormSchema(currentPayload.schema)) {
    return (
      <div data-testid="itsm-draft-form-surface" className="mb-5 max-w-[720px] rounded-2xl border border-destructive/25 bg-destructive/5 px-4 py-3 text-sm text-destructive">
        申请确认表单未生成，请联系管理员先生成参考路径后再提交。
      </div>
    )
  }

  return (
    <div data-testid="itsm-draft-form-surface" className="mb-5 max-w-[720px] rounded-2xl border border-border/65 bg-background/94 p-4 shadow-[0_18px_52px_-44px_hsl(var(--foreground))]">
      <div className="mb-4 flex items-start gap-3">
        <div className="flex size-9 shrink-0 items-center justify-center rounded-full border border-primary/20 bg-primary/8 text-primary">
          <FileCheck2 className="size-4" />
        </div>
        <div className="min-w-0">
          <div className="text-xs font-medium text-muted-foreground">服务申请草稿</div>
          <div className="mt-1 text-base font-semibold text-foreground">{currentPayload.title || "请确认服务申请"}</div>
          {currentPayload.summary && (
            <div className="mt-1 text-sm leading-6 text-muted-foreground">{currentPayload.summary}</div>
          )}
        </div>
      </div>

      <FormRenderer
        schema={currentPayload.schema}
        data={initialFormData}
        mode="edit"
        onChange={setFormData}
        disabled={submitMutation.isPending}
      />

      {inlineError && (
        <div className="mt-4 rounded-lg border border-destructive/25 bg-destructive/5 px-3 py-2 text-sm text-destructive">
          {inlineError}
        </div>
      )}

      <div className="mt-4 flex justify-end">
        <Button
          data-testid="itsm-submit-draft-button"
          type="button"
          onClick={() => submitMutation.mutate()}
          disabled={submitMutation.isPending}
        >
          {submitMutation.isPending ? <Loader2 className="mr-1.5 size-4 animate-spin" /> : <CheckCircle2 className="mr-1.5 size-4" />}
          提交工单
        </Button>
      </div>
    </div>
  )
}

function ServiceDeskConversation({
  session,
  agentName,
  initialPrompt,
  initialImages,
  onInitialPromptSent,
}: {
  session: AgentSession
  agentName: string
  initialPrompt?: string
  initialImages?: ChatComposerImage[]
  onInitialPromptSent: () => void
}) {
  const queryClient = useQueryClient()
  const [input, setInput] = useState("")
  const [pendingImages, setPendingImages] = useState<ChatComposerImage[]>([])
  const scrollRef = useRef<HTMLDivElement>(null)
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

  const handleChatError = useCallback((err: Error) => {
    toast.error(err.message)
    invalidateWorkspace()
  }, [invalidateWorkspace])

  const { chat, runtime } = useServiceDeskChat(session.id, sessionData?.messages, {
    onFinish: invalidateWorkspace,
    onError: handleChatError,
  })

  const chatBusy = chat.status === "streaming" || chat.status === "submitted"
  const visibleMessages = chat.messages

  useEffect(() => {
    if (!initialPrompt || initialPromptSentRef.current || isLoading) return
    initialPromptSentRef.current = true
    ;(async () => {
      try {
        const imageUrls: string[] = []
        for (const image of initialImages ?? []) {
          const res = await sessionApi.uploadMessageImage(session.id, image.file)
          imageUrls.push(res.url)
        }
        await chat.sendMessage({
          text: initialPrompt,
          files: toImageFileParts(imageUrls),
        })
        onInitialPromptSent()
      } catch (err) {
        initialPromptSentRef.current = false
        toast.error(err instanceof Error ? err.message : "图片上传失败")
      }
    })()
  }, [chat, initialImages, initialPrompt, isLoading, onInitialPromptSent, session.id])

  useEffect(() => {
    const container = scrollRef.current
    if (!container) return
    container.scrollTo({ top: container.scrollHeight, behavior: chatBusy ? "instant" : "smooth" })
  }, [visibleMessages, chatBusy])

  const sendMutation = useMutation({
    mutationFn: async (text: string) => {
      const imageUrls: string[] = []
      for (const image of pendingImages) {
        const res = await sessionApi.uploadMessageImage(session.id, image.file)
        imageUrls.push(res.url)
      }
      return { text, imageUrls }
    },
    onSuccess: ({ text, imageUrls }) => {
      void chat.sendMessage({
        text,
        files: toImageFileParts(imageUrls),
      }).catch((err: Error) => {
        toast.error(err.message)
        invalidateWorkspace()
      })
      setInput("")
      setPendingImages([])
    },
    onError: (err) => {
      toast.error(err.message)
    },
  })

  const cancelMutation = useMutation({
    mutationFn: async () => {
      await chat.stop()
      return sessionApi.cancel(session.id)
    },
    onSuccess: invalidateWorkspace,
    onError: (err) => toast.error(err.message),
  })

  const isBusy = chatBusy || sendMutation.isPending

  const handleSend = useCallback(() => {
    const text = input.trim()
    if ((!text && pendingImages.length === 0) || isBusy || sendMutation.isPending) return
    sendMutation.mutate(text)
  }, [input, isBusy, pendingImages, sendMutation])

  const addPendingImages = useCallback((files: File[]) => {
    addImagePreviews(files, (image) => setPendingImages((prev) => [...prev, image]))
  }, [])

  const removePendingImage = useCallback((index: number) => {
    setPendingImages((prev) => prev.filter((_, i) => i !== index))
  }, [])

  const showEmpty = !isLoading && visibleMessages.length === 0 && !isBusy && !initialPrompt

  return (
    <AssistantRuntimeProvider runtime={runtime}>
    <ChatWorkspace
      density="workbench"
      messageWidth="standard"
      composerPlacement="docked"
      emptyStateTone="service-desk"
      headerClassName="pl-8 pr-5"
      identity={{
        title: "IT 服务台",
        subtitle: `当前智能体：${agentName}`,
        icon: <Bot className="size-4" />,
      }}
      loading={isLoading}
      emptyState={
        showEmpty ? (
            <div className="mx-auto flex h-full max-w-3xl flex-col justify-center px-6 py-12">
              <div className="flex size-14 items-center justify-center rounded-full border border-primary/25 bg-primary/10 text-primary">
                <Bot className="size-7" />
              </div>
              <h2 className="mt-5 text-2xl font-semibold">继续描述 IT 诉求</h2>
              <p className="mt-2 max-w-xl text-sm leading-6 text-muted-foreground">
                服务台会沿用当前会话上下文继续澄清、填槽和创建工单。
              </p>
            </div>
        ) : null
      }
      messages={visibleMessages}
      agentName={agentName}
      isBusy={isBusy}
      status={chat.status}
      error={chat.error}
      session={session}
      surfaces={[
        {
          surfaceType: "itsm.draft_form",
          suppressText: true,
          render: ({ part }) => (
            <ServiceDeskDataPart
              part={part}
              sessionId={session.id}
              onSubmitted={invalidateWorkspace}
            />
          ),
        } satisfies ChatWorkspaceSurfaceRenderer,
      ]}
      workspaceActions={{
        regenerate: () => chat.regenerate(),
        cancel: () => cancelMutation.mutate(),
      }}
      composer={{
        value: input,
        onChange: setInput,
        onSend: handleSend,
        onStop: () => cancelMutation.mutate(),
        onPasteImages: addPendingImages,
        onPickImages: addPendingImages,
        onRemoveImage: removePendingImage,
        images: pendingImages,
        disabled: isBusy,
        pending: sendMutation.isPending,
        isBusy,
        allowImages: true,
        placeholder: "描述你的 IT 诉求...",
        hint: "Enter 发送，Shift + Enter 换行",
        variant: "compact",
        maxWidth: "standard",
        showToolbarHint: true,
        attachmentTone: "service-desk",
      }}
      messagesEndRef={messagesEndRef}
      scrollRef={scrollRef}
    />
    </AssistantRuntimeProvider>
  )
}

export function Component() {
  const queryClient = useQueryClient()
  const [selectedSessionId, setSelectedSessionId] = useState<number | null>(null)
  const [createdSession, setCreatedSession] = useState<AgentSession | null>(null)
  const [landingInput, setLandingInput] = useState("")
  const [landingImages, setLandingImages] = useState<ChatComposerImage[]>([])
  const [pendingInitialPrompt, setPendingInitialPrompt] = useState<{ sessionId: number; text: string; images: ChatComposerImage[] } | null>(null)

  const { data: config, isLoading: configLoading } = useQuery({
    queryKey: ["itsm-smart-staffing-config"],
    queryFn: fetchSmartStaffingConfig,
  })

  const serviceDeskAgentId = config?.posts?.intake?.agentId ?? 0
  const serviceDeskAgentName = config?.posts?.intake?.agentName || "IT 服务台"

  const sessionsQuery = useInfiniteQuery({
    queryKey: ["ai-sessions", serviceDeskAgentId],
    initialPageParam: 1,
    queryFn: ({ pageParam }) => sessionApi.list({ agentId: serviceDeskAgentId, page: pageParam, pageSize: 30 }),
    enabled: serviceDeskAgentId > 0,
    getNextPageParam: (lastPage, allPages) => {
      const loaded = allPages.reduce((sum, page) => sum + page.items.length, 0)
      return loaded < lastPage.total ? allPages.length + 1 : undefined
    },
  })

  const sessions = useMemo(() => {
    const unique = new Map<number, AgentSession>()
    for (const page of sessionsQuery.data?.pages ?? []) {
      for (const item of page.items) {
        unique.set(item.id, item)
      }
    }
    return Array.from(unique.values())
  }, [sessionsQuery.data?.pages])
  const activeSession = selectedSessionId == null
    ? null
    : sessions.find((item) => item.id === selectedSessionId) ?? (createdSession?.id === selectedSessionId ? createdSession : null)

  const createSessionMutation = useMutation({
    mutationFn: async ({ text, images }: { text: string; images: ChatComposerImage[] }) => {
      const session = await sessionApi.create(serviceDeskAgentId)
      return { session, text, images }
    },
    onSuccess: ({ session, text, images }) => {
      setCreatedSession(session)
      setSelectedSessionId(session.id)
      setPendingInitialPrompt({ sessionId: session.id, text, images })
      setLandingInput("")
      setLandingImages([])
      queryClient.invalidateQueries({ queryKey: ["ai-sessions", serviceDeskAgentId] })
    },
    onError: (err) => toast.error(err.message),
  })

  const deleteSessionMutation = useMutation({
    mutationFn: (sid: number) => sessionApi.delete(sid),
    onSuccess: (_, sid) => {
      toast.success("会话已删除")
      if (selectedSessionId === sid) {
        setSelectedSessionId(null)
        setPendingInitialPrompt(null)
      }
      if (createdSession?.id === sid) {
        setCreatedSession(null)
      }
      queryClient.invalidateQueries({ queryKey: ["ai-sessions", serviceDeskAgentId] })
    },
    onError: (err) => toast.error(err.message),
  })

  const handleLandingSend = useCallback(() => {
    const text = landingInput.trim()
    if ((!text && landingImages.length === 0) || serviceDeskAgentId <= 0 || createSessionMutation.isPending) return
    createSessionMutation.mutate({ text, images: landingImages })
  }, [createSessionMutation, landingImages, landingInput, serviceDeskAgentId])

  const addLandingImages = useCallback((files: File[]) => {
    addImagePreviews(files, (image) => setLandingImages((prev) => [...prev, image]))
  }, [])

  const removeLandingImage = useCallback((index: number) => {
    setLandingImages((prev) => prev.filter((_, i) => i !== index))
  }, [])

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
    setLandingImages([])
  }, [])

  const clearPendingInitialPrompt = useCallback(() => {
    setPendingInitialPrompt(null)
  }, [])

  const handleDeleteSession = useCallback((session: AgentSession) => {
    deleteSessionMutation.mutate(session.id)
  }, [deleteSessionMutation])

  const handleLoadMoreSessions = useCallback(() => {
    if (!sessionsQuery.hasNextPage || sessionsQuery.isFetchingNextPage) return
    sessionsQuery.fetchNextPage()
  }, [sessionsQuery])

  return (
    <div className="grid h-full min-h-0 grid-cols-1 grid-rows-1 overflow-hidden bg-[linear-gradient(180deg,hsl(var(--background)),hsl(var(--muted)/0.18))] md:grid-cols-[240px_minmax(0,1fr)]">
      <SessionSidebar
        variant="service-desk"
        sessions={sessions}
        currentSessionId={activeSession?.id ?? undefined}
        loading={sessionsQuery.isLoading}
        title="服务台会话"
        emptyText="暂无历史会话"
        newLabel="新会话"
        showDateGroups={false}
        showItemActions
        onSelect={handleSelectSession}
        onNew={handleNewSession}
        onDeleteSession={handleDeleteSession}
        deletingSessionId={deleteSessionMutation.isPending ? deleteSessionMutation.variables ?? null : null}
        hasMore={Boolean(sessionsQuery.hasNextPage)}
        loadingMore={sessionsQuery.isFetchingNextPage}
        onLoadMore={handleLoadMoreSessions}
      />
      {serviceDeskAgentId <= 0 || configLoading ? (
        <main className="flex min-h-0 min-w-0 flex-1 flex-col overflow-hidden">
          <NotOnDutyState loading={configLoading} />
        </main>
      ) : activeSession ? (
        <main className="flex min-h-0 min-w-0 flex-1 flex-col overflow-hidden">
          <ServiceDeskConversation
            key={activeSession.id}
            session={activeSession}
            agentName={serviceDeskAgentName}
            initialPrompt={pendingInitialPrompt?.sessionId === activeSession.id ? pendingInitialPrompt.text : undefined}
            initialImages={pendingInitialPrompt?.sessionId === activeSession.id ? pendingInitialPrompt.images : undefined}
            onInitialPromptSent={clearPendingInitialPrompt}
          />
        </main>
      ) : (
        <main className="flex min-h-0 min-w-0 flex-1 flex-col overflow-hidden">
            <ChatHeader
              className="pl-8 pr-5"
              identity={{
                title: "IT 服务台",
                subtitle: `当前智能体：${serviceDeskAgentName}`,
                icon: <Sparkles className="size-4" />,
                status: <ChatStatusDot />,
              }}
              actions={
                <Button type="button" size="sm" variant="outline" className="md:hidden" onClick={handleNewSession}>
                <MessageSquarePlus className="mr-1.5 size-3.5" />
                新会话
                </Button>
              }
            />
            <WelcomeStage
              agentName={serviceDeskAgentName}
              value={landingInput}
              images={landingImages}
              onChange={setLandingInput}
              onSend={handleLandingSend}
              onAddImages={addLandingImages}
              onRemoveImage={removeLandingImage}
              disabled={createSessionMutation.isPending}
              pending={createSessionMutation.isPending}
            />
          </main>
      )}
    </div>
  )
}
