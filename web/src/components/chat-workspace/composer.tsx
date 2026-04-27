"use client"

import { useEffect, useRef } from "react"
import { ImagePlus, Loader2, Send, Square, X } from "lucide-react"
import { Button } from "@/components/ui/button"
import { cn } from "@/lib/utils"

export interface ChatComposerImage {
  file: File
  preview: string
}

export type ChatComposerVariant = "default" | "stage" | "compact"
export type ChatComposerMaxWidth = "narrow" | "standard" | "wide"
export type ChatComposerAttachmentTone = "chat" | "service-desk"

export interface ChatComposerProps {
  value: string
  onChange: (value: string) => void
  onSend: () => void
  onStop?: () => void
  onPasteImages?: (files: File[]) => void
  onPickImages?: (files: File[]) => void
  onRemoveImage?: (index: number) => void
  images?: ChatComposerImage[]
  placeholder: string
  hint?: React.ReactNode
  disabled?: boolean
  pending?: boolean
  isBusy?: boolean
  allowImages?: boolean
  variant?: ChatComposerVariant
  maxWidth?: ChatComposerMaxWidth
  minRows?: number
  showToolbarHint?: boolean
  attachmentTone?: ChatComposerAttachmentTone
}

const composerWidthClass: Record<ChatComposerMaxWidth, string> = {
  narrow: "max-w-2xl",
  standard: "max-w-[720px]",
  wide: "max-w-4xl",
}

const composerShellClass: Record<ChatComposerVariant, string> = {
  default: "rounded-[1.35rem] border border-border/70 bg-background/96 shadow-[0_18px_52px_-42px_hsl(var(--foreground))]",
  stage: "rounded-[1.5rem] border border-primary/20 bg-background/96 shadow-[0_24px_70px_-48px_hsl(var(--primary))]",
  compact: "rounded-2xl border border-border/70 bg-background shadow-sm",
}

const composerTextareaClass: Record<ChatComposerVariant, string> = {
  default: "min-h-20 max-h-[200px] px-4 pb-1 pt-3 text-[15px]",
  stage: "min-h-28 max-h-[220px] px-5 pb-2 pt-4 text-[15px]",
  compact: "min-h-11 max-h-40 px-4 pb-1 pt-3 text-sm",
}

const composerToolbarClass: Record<ChatComposerVariant, string> = {
  default: "px-3 pb-2",
  stage: "px-4 pb-3",
  compact: "px-3 pb-2",
}

const composerHintText: Record<ChatComposerAttachmentTone, string> = {
  chat: "图片",
  "service-desk": "截图或图片",
}

export function ChatComposer({
  value,
  onChange,
  onSend,
  onStop,
  onPasteImages,
  onPickImages,
  onRemoveImage,
  images = [],
  placeholder,
  hint,
  disabled,
  pending,
  isBusy,
  allowImages = true,
  variant = "default",
  maxWidth = "standard",
  minRows,
  showToolbarHint,
  attachmentTone = "chat",
}: ChatComposerProps) {
  const textareaRef = useRef<HTMLTextAreaElement>(null)
  const fileInputRef = useRef<HTMLInputElement>(null)

  useEffect(() => {
    const textarea = textareaRef.current
    if (!textarea) return
    textarea.style.height = "auto"
    const maxHeight = variant === "stage" ? 220 : variant === "compact" ? 160 : 200
    textarea.style.height = `${Math.min(textarea.scrollHeight, maxHeight)}px`
  }, [value, variant])

  useEffect(() => {
    if (disabled || pending || isBusy) return
    const timer = window.setTimeout(() => textareaRef.current?.focus(), 0)
    return () => window.clearTimeout(timer)
  }, [disabled, pending, isBusy])

  const canSend = Boolean(value.trim() || images.length > 0) && !disabled && !pending && !isBusy

  return (
    <div className={cn("w-full", composerWidthClass[maxWidth])}>
      <div className={cn(
        "transition-colors focus-within:border-primary/35",
        composerShellClass[variant],
      )}>
        <textarea
          ref={textareaRef}
          value={value}
          onChange={(event) => onChange(event.target.value)}
          onKeyDown={(event) => {
            if (event.nativeEvent.isComposing) return
            if (event.key === "Enter" && !event.shiftKey) {
              event.preventDefault()
              if (canSend) onSend()
            }
          }}
          onPaste={(event) => {
            if (!allowImages || !onPasteImages) return
            const files = Array.from(event.clipboardData.items)
              .filter((item) => item.type.startsWith("image/"))
              .map((item) => item.getAsFile())
              .filter((file): file is File => Boolean(file))
            if (files.length === 0) return
            event.preventDefault()
            onPasteImages(files)
          }}
          placeholder={placeholder}
          rows={minRows ?? 1}
          className={cn(
            "w-full resize-none bg-transparent leading-relaxed placeholder:text-muted-foreground focus:outline-none read-only:cursor-text",
            composerTextareaClass[variant],
          )}
          readOnly={disabled || pending || isBusy}
        />

        {images.length > 0 && (
          <div className={cn("flex gap-2 overflow-x-auto pb-2", variant === "stage" ? "px-5" : "px-4")}>
            {images.map((image, index) => (
              <div key={`${image.preview}:${index}`} className="group relative shrink-0">
                <img src={image.preview} alt={`attachment-${index + 1}`} className="h-16 w-16 rounded-lg border border-border/70 object-cover" />
                <button
                  type="button"
                  onClick={() => onRemoveImage?.(index)}
                  className="absolute -right-1.5 -top-1.5 flex h-5 w-5 items-center justify-center rounded-full bg-destructive text-white opacity-0 transition-opacity group-hover:opacity-100"
                >
                  <X className="h-3 w-3" />
                </button>
              </div>
            ))}
          </div>
        )}

        <div className={cn("flex items-center justify-between", composerToolbarClass[variant])}>
          <div className="flex items-center gap-1">
            {allowImages && (
              <>
                <input
                  ref={fileInputRef}
                  type="file"
                  accept="image/*"
                  multiple
                  className="hidden"
                  onChange={(event) => {
                    const files = Array.from(event.target.files ?? [])
                    if (files.length > 0) onPickImages?.(files)
                    event.target.value = ""
                  }}
                />
                <Button
                  type="button"
                  variant="ghost"
                  size="icon"
                  className={cn(
                    "h-8 w-8 text-muted-foreground/70 hover:text-foreground",
                    attachmentTone === "service-desk" && "hover:bg-primary/8 hover:text-primary",
                  )}
                  disabled={disabled || pending || isBusy}
                  onClick={() => fileInputRef.current?.click()}
                >
                  <ImagePlus className="h-4 w-4" />
                </Button>
                {showToolbarHint && (
                  <span className="hidden text-xs text-muted-foreground/65 sm:inline">
                    {composerHintText[attachmentTone]}
                  </span>
                )}
              </>
            )}
          </div>
          <Button
            type="button"
            size="icon"
            className={cn(
              "h-8 w-8 rounded-full transition-transform active:scale-95",
              variant === "stage" && "h-9 w-9",
            )}
            onClick={() => {
              if (isBusy && onStop) {
                onStop()
                return
              }
              onSend()
            }}
            disabled={isBusy ? !onStop : !canSend}
          >
            {pending ? <Loader2 className="h-4 w-4 animate-spin" /> : isBusy ? <Square className="h-4 w-4" /> : <Send className="h-4 w-4" />}
          </Button>
        </div>
      </div>
      {hint && <div className="mt-1 text-center text-[10px] text-muted-foreground/50">{hint}</div>}
    </div>
  )
}
