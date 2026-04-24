"use client"

import { useCallback, useMemo, useState } from "react"
import { useMutation, useQuery } from "@tanstack/react-query"
import { toast } from "sonner"
import { sessionApi, type AgentSession, type SessionMessage } from "@/lib/api"
import { groupUIMessagesIntoPairs } from "./utils"
import { useAiChat, type UseAiChatOptions } from "./use-ai-chat"
import type { ChatComposerImage } from "./composer"

function previewImage(file: File) {
  return new Promise<string>((resolve, reject) => {
    const reader = new FileReader()
    reader.onload = (event) => resolve(String(event.target?.result ?? ""))
    reader.onerror = () => reject(new Error("图片预览生成失败"))
    reader.readAsDataURL(file)
  })
}

export interface UseChatWorkspaceOptions extends UseAiChatOptions {
  sessionId: number
  initialSessionMessages?: SessionMessage[]
  loadSession?: boolean
  onCancelSuccess?: () => void
}

export function useChatWorkspace({
  sessionId,
  initialSessionMessages,
  loadSession,
  onFinish,
  onError,
  onCancelSuccess,
}: UseChatWorkspaceOptions) {
  const [input, setInput] = useState("")
  const [pendingImages, setPendingImages] = useState<ChatComposerImage[]>([])

  const sessionQuery = useQuery({
    queryKey: ["ai-session", sessionId],
    queryFn: () => sessionApi.get(sessionId),
    enabled: Boolean(loadSession && sessionId),
  })

  const chat = useAiChat(sessionId, initialSessionMessages ?? sessionQuery.data?.messages, {
    onFinish,
    onError,
  })

  const isBusy = chat.status === "streaming" || chat.status === "submitted"
  const qaPairs = useMemo(() => groupUIMessagesIntoPairs(chat.messages), [chat.messages])

  const addImages = useCallback(async (files: File[]) => {
    const images = await Promise.all(files.map(async (file) => ({ file, preview: await previewImage(file) })))
    setPendingImages((prev) => [...prev, ...images])
  }, [])

  const removeImage = useCallback((index: number) => {
    setPendingImages((prev) => prev.filter((_, i) => i !== index))
  }, [])

  const uploadImageMutation = useMutation({
    mutationFn: (file: File) => sessionApi.uploadMessageImage(sessionId, file),
    onError: (err) => toast.error(err.message),
  })

  const sendMutation = useMutation({
    mutationFn: async (text: string) => {
      const imageUrls: string[] = []
      for (const image of pendingImages) {
        const res = await uploadImageMutation.mutateAsync(image.file)
        imageUrls.push(res.url)
      }
      chat.setPendingImageUrls(imageUrls)
      return text
    },
    onSuccess: (text) => {
      chat.sendMessage({ text })
      setInput("")
      setPendingImages([])
    },
    onError: (err) => toast.error(err.message),
  })

  const cancelMutation = useMutation({
    mutationFn: async () => {
      await chat.stop()
      return sessionApi.cancel(sessionId)
    },
    onSuccess: onCancelSuccess,
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

  const send = useCallback((content?: string) => {
    const text = (content ?? input).trim()
    if ((!text && pendingImages.length === 0) || isBusy || sendMutation.isPending) return
    sendMutation.mutate(text)
  }, [input, isBusy, pendingImages.length, sendMutation])

  return {
    chat,
    session: sessionQuery.data?.session as AgentSession | undefined,
    messagesLoading: loadSession ? sessionQuery.isLoading : false,
    input,
    setInput,
    pendingImages,
    addImages,
    removeImage,
    send,
    sendMutation,
    cancelMutation,
    continueMutation,
    isBusy,
    qaPairs,
  }
}
