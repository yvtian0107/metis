"use client";

import { useCallback, useEffect, useMemo, useRef, useState } from "react"
import { useParams, useNavigate } from "react-router"
import { useTranslation } from "react-i18next"
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query"
import type { UIMessage } from "ai"
import { Brain, PanelLeft, PanelLeftClose, Trash2 } from "lucide-react"
import { sessionApi } from "@/lib/api"
import { toast } from "sonner"
import { Button } from "@/components/ui/button"
import { Badge } from "@/components/ui/badge"
import {
  AlertDialog, AlertDialogAction, AlertDialogCancel, AlertDialogContent,
  AlertDialogDescription, AlertDialogFooter, AlertDialogHeader, AlertDialogTitle, AlertDialogTrigger,
} from "@/components/ui/alert-dialog"
import {
  ChatWorkspace,
  createOptimisticUserMessage,
  hasUnmatchedPendingUserMessages,
  mergeTimelineMessages,
  SessionSidebar,
  sessionMessagesToUIMessages,
  useAiChat,
} from "@/components/chat-workspace"
import { WelcomeScreen } from "./components/welcome-screen"
import { MemoryPanel } from "./components/memory-panel"

const SIDEBAR_COLLAPSED_KEY = "ai-chat-sidebar-collapsed"

interface PendingImage {
  file: File
  preview: string
  uploading?: boolean
}

export function Component() {
  const { sid } = useParams<{ sid: string }>()
  const sessionId = Number(sid)
  const { t } = useTranslation(["ai", "common"])
  const navigate = useNavigate()
  const queryClient = useQueryClient()
  const [input, setInput] = useState("")
  const [memoryOpen, setMemoryOpen] = useState(false)
  const [sidebarCollapsed, setSidebarCollapsed] = useState(() => {
    const saved = localStorage.getItem(SIDEBAR_COLLAPSED_KEY)
    return saved ? saved === "true" : false
  })
  const [pendingImages, setPendingImages] = useState<PendingImage[]>([])
  const [pendingUserMessages, setPendingUserMessages] = useState<UIMessage[]>([])
  const [isAtBottom, setIsAtBottom] = useState(true)
  const scrollRef = useRef<HTMLDivElement>(null)
  const messagesEndRef = useRef<HTMLDivElement>(null)

  const { data: sessionData, isLoading } = useQuery({
    queryKey: ["ai-session", sessionId],
    queryFn: () => sessionApi.get(sessionId),
    enabled: !!sessionId,
  })

  const handleChatFinish = useCallback(() => {
    queryClient.invalidateQueries({ queryKey: ["ai-session", sessionId] })
  }, [queryClient, sessionId])

  const handleChatError = useCallback((err: Error) => {
    toast.error(err.message)
    queryClient.invalidateQueries({ queryKey: ["ai-session", sessionId] })
  }, [queryClient, sessionId])

  const chat = useAiChat(sessionId, sessionData?.messages, {
    onFinish: handleChatFinish,
    onError: handleChatError,
  })

  const chatBusy = chat.status === "streaming" || chat.status === "submitted"
  const serverMessages = useMemo(
    () => sessionMessagesToUIMessages(sessionData?.messages ?? []),
    [sessionData?.messages],
  )
  const baseVisibleMessages = useMemo(
    () => mergeTimelineMessages(serverMessages, chat.messages),
    [chat.messages, serverMessages],
  )
  const hasPendingUserMessage = useMemo(
    () => hasUnmatchedPendingUserMessages(baseVisibleMessages, pendingUserMessages),
    [baseVisibleMessages, pendingUserMessages],
  )
  const visibleMessages = useMemo(
    () => mergeTimelineMessages(serverMessages, chat.messages, pendingUserMessages),
    [chat.messages, pendingUserMessages, serverMessages],
  )
  const scrollToBottom = useCallback((instant?: boolean) => {
    messagesEndRef.current?.scrollIntoView({ behavior: instant ? "instant" : "smooth" })
  }, [])

  const updateAtBottom = useCallback(() => {
    const el = scrollRef.current
    if (!el) return
    setIsAtBottom(el.scrollHeight - el.scrollTop - el.clientHeight < 120)
  }, [])

  const uploadImageMutation = useMutation({
    mutationFn: (file: File) => sessionApi.uploadMessageImage(sessionId, file),
    onError: (err) => {
      setPendingUserMessages([])
      toast.error(err.message)
    },
  })

  const sendMutation = useMutation({
    mutationFn: async (text: string) => {
      const imageUrls: string[] = []
      for (const img of pendingImages) {
        if (!img.uploading) {
          const res = await uploadImageMutation.mutateAsync(img.file)
          imageUrls.push(res.url)
        }
      }
      chat.setPendingImageUrls(imageUrls)
      return text
    },
    onSuccess: (text) => {
      chat.sendMessage({ text })
      setInput("")
      setPendingImages([])
      setIsAtBottom(true)
      scrollToBottom()
    },
    onError: (err) => toast.error(err.message),
  })

  const cancelMutation = useMutation({
    mutationFn: async () => {
      await chat.stop()
      return sessionApi.cancel(sessionId)
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["ai-session", sessionId] })
    },
    onError: (err) => toast.error(err.message),
  })

  const continueMutation = useMutation({
    mutationFn: async () => {
      await sessionApi.continueGeneration(sessionId)
      chat.clearError()
      await chat.resumeStream()
    },
    onError: (err) => toast.error(err.message),
  })

  const deleteMutation = useMutation({
    mutationFn: () => sessionApi.delete(sessionId),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["ai-sessions"] })
      toast.success(t("ai:chat.sessionDeleted"))

      const currentData = queryClient.getQueryData<{ items: Array<{ id: number }> }>(["ai-sessions", agentId])
      const otherSession = currentData?.items.find((s) => s.id !== sessionId)

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
    onSuccess: (_, { mid, content }) => {
      const editedIndex = chat.messages.findIndex((m) => Number(m.id) === mid)
      if (editedIndex !== -1) {
        const newMessages = chat.messages.slice(0, editedIndex + 1)
        newMessages[editedIndex] = {
          ...newMessages[editedIndex],
          parts:
            newMessages[editedIndex].parts?.map((p) =>
              p.type === "text" ? { ...p, text: content } : p,
            ) || [{ type: "text", text: content }],
        }
        chat.setMessages(newMessages)
        chat.regenerate()
      } else {
        queryClient.invalidateQueries({ queryKey: ["ai-session", sessionId] })
      }
    },
    onError: (err) => toast.error(err.message),
  })

  const handleEditMessage = useCallback(
    (messageId: number, content: string) => {
      editMessageMutation.mutate({ mid: messageId, content })
    },
    [editMessageMutation],
  )

  // Follow new output only while the reader is already near the bottom.
  useEffect(() => {
    if (!isAtBottom && chat.status !== "submitted") return
    const instant = chat.status === "streaming" || chat.status === "submitted"
    scrollToBottom(instant)
  }, [chat.messages, chat.status, isAtBottom, scrollToBottom, visibleMessages])

  const isBusy = chatBusy || sendMutation.isPending || hasPendingUserMessage

  function handleSend(content?: string) {
    const text = (content ?? input).trim()
    if ((!text && pendingImages.length === 0) || isBusy || sendMutation.isPending) return
    setPendingUserMessages([
      createOptimisticUserMessage({
        text,
        images: pendingImages.map((image) => image.preview),
      }),
    ])
    sendMutation.mutate(text)
  }

  function addPendingImages(files: File[]) {
    for (const file of files) {
      const reader = new FileReader()
      reader.onload = (event) => {
        const preview = event.target?.result as string
        setPendingImages((prev) => [...prev, { file, preview }])
      }
      reader.readAsDataURL(file)
    }
  }

  function removePendingImage(index: number) {
    setPendingImages((prev) => {
      const newImages = [...prev]
      newImages.splice(index, 1)
      return newImages
    })
  }

  function handleRetry() {
    chat.clearError()
    chat.regenerate()
  }

  const toggleSidebar = useCallback(() => {
    setSidebarCollapsed((prev) => {
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
  const agentName = (session as unknown as Record<string, unknown>)?.agentName as string | undefined
  const hasMessages = visibleMessages.length > 0
  const showWelcome = !hasMessages && !isBusy
  const showJumpToBottom = !showWelcome && !isAtBottom

  return (
    <>
      <ChatWorkspace
        density="comfortable"
        messageWidth="standard"
        composerPlacement="docked"
        emptyStateTone="ai"
        sidebar={<SessionSidebar agentId={agentId} currentSessionId={sessionId} collapsed={sidebarCollapsed} />}
        leading={
          <Button variant="ghost" size="sm" className="h-8 w-8 p-0" onClick={toggleSidebar}>
            {sidebarCollapsed ? <PanelLeft className="h-4 w-4" /> : <PanelLeftClose className="h-4 w-4" />}
          </Button>
        }
        identity={{
          title: session?.title || t("ai:chat.newChat"),
          subtitle: agentName ? `当前智能体：${agentName}` : undefined,
          status: session?.status ? (
            <Badge variant="outline" className="text-xs">
              {t(`ai:chat.sessionStatus.${session.status}`)}
            </Badge>
          ) : undefined,
        }}
        actions={
          <>
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
          </>
        }
        loading={isLoading}
        emptyState={
          showWelcome ? (
            <WelcomeScreen
              agentName={agentName ?? session?.title}
              agentType={(session as unknown as Record<string, unknown>)?.agentType as string | undefined}
              onPromptClick={(prompt) => handleSend(prompt)}
            />
          ) : null
        }
        messages={visibleMessages}
        agentName={agentName}
        isBusy={isBusy}
        status={chat.status}
        error={chat.error}
        workspaceActions={{
          regenerate: () => chat.regenerate(),
          retry: handleRetry,
          continueGeneration: () => continueMutation.mutate(),
          cancel: () => cancelMutation.mutate(),
        }}
        onEditMessage={handleEditMessage}
        composer={{
          value: input,
          onChange: setInput,
          onSend: handleSend,
          onStop: () => cancelMutation.mutate(),
          onPasteImages: addPendingImages,
          onPickImages: addPendingImages,
          onRemoveImage: removePendingImage,
          images: pendingImages,
          placeholder: t("ai:chat.inputPlaceholder"),
          hint: t("ai:chat.inputHint"),
          disabled: isBusy,
          pending: sendMutation.isPending,
          isBusy,
          allowImages: true,
          variant: "compact",
          maxWidth: "standard",
          showToolbarHint: true,
          attachmentTone: "chat",
        }}
        messagesEndRef={messagesEndRef}
        scrollRef={scrollRef}
        onScroll={updateAtBottom}
        showJumpToBottom={showJumpToBottom}
        onJumpToBottom={() => {
          setIsAtBottom(true)
          scrollToBottom()
        }}
        getDoneMetrics={() => ({
          inputTokens: chat.lastUsage.promptTokens,
          outputTokens: chat.lastUsage.completionTokens,
        })}
      />
      {memoryOpen && agentId && <MemoryPanel agentId={agentId} onClose={() => setMemoryOpen(false)} />}
    </>
  )
}
