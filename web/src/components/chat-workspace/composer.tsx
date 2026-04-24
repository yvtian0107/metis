"use client"

import { useEffect, useRef } from "react"
import { ImagePlus, Loader2, Send, Square, X } from "lucide-react"
import { Button } from "@/components/ui/button"
import { cn } from "@/lib/utils"

export interface ChatComposerImage {
  file: File
  preview: string
}

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
  compact?: boolean
  className?: string
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
  compact,
  className,
}: ChatComposerProps) {
  const textareaRef = useRef<HTMLTextAreaElement>(null)
  const fileInputRef = useRef<HTMLInputElement>(null)

  useEffect(() => {
    const textarea = textareaRef.current
    if (!textarea) return
    textarea.style.height = "auto"
    textarea.style.height = `${Math.min(textarea.scrollHeight, compact ? 160 : 200)}px`
  }, [value, compact])

  useEffect(() => {
    if (disabled || pending || isBusy) return
    const timer = window.setTimeout(() => textareaRef.current?.focus(), 0)
    return () => window.clearTimeout(timer)
  }, [disabled, pending, isBusy])

  const canSend = Boolean(value.trim() || images.length > 0) && !disabled && !pending && !isBusy

  return (
    <div className={cn("w-full", className)}>
      <div className="rounded-2xl border bg-background shadow-lg transition-colors focus-within:border-primary/30">
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
          rows={1}
          className={cn(
            "w-full max-h-[200px] resize-none bg-transparent px-4 pt-3 pb-1 text-base leading-relaxed placeholder:text-muted-foreground focus:outline-none read-only:cursor-text",
            compact ? "min-h-11" : "min-h-24",
          )}
          readOnly={disabled || pending || isBusy}
        />

        {images.length > 0 && (
          <div className="flex gap-2 overflow-x-auto px-4 pb-2">
            {images.map((image, index) => (
              <div key={`${image.preview}:${index}`} className="group relative shrink-0">
                <img src={image.preview} alt={`attachment-${index + 1}`} className="h-16 w-16 rounded-md border object-cover" />
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

        <div className="flex items-center justify-between px-3 pb-2">
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
                  className="h-8 w-8 text-muted-foreground/70"
                  disabled={disabled || pending || isBusy}
                  onClick={() => fileInputRef.current?.click()}
                >
                  <ImagePlus className="h-4 w-4" />
                </Button>
              </>
            )}
          </div>
          <Button
            type="button"
            size="icon"
            className="h-8 w-8 rounded-full"
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
