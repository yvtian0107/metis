import { useState, useEffect, useRef, useCallback } from "react"
import { useTranslation } from "react-i18next"
import { ChevronRight, ChevronDown } from "lucide-react"

interface ThinkingBlockProps {
  content: string
  isStreaming: boolean
  durationMs?: number
}

export function ThinkingBlock({ content, isStreaming, durationMs }: ThinkingBlockProps) {
  const { t } = useTranslation(["ai"])
  const [expanded, setExpanded] = useState(true)
  const [elapsed, setElapsed] = useState(0)
  const startRef = useRef(0)

  const handleToggle = useCallback(() => {
    setExpanded(prev => !prev)
  }, [])

  // Elapsed timer during streaming (setState in interval callback is fine)
  useEffect(() => {
    if (!isStreaming) return
    startRef.current = Date.now()
    const interval = setInterval(() => {
      setElapsed(Math.round((Date.now() - startRef.current) / 100) / 10)
    }, 100)
    return () => clearInterval(interval)
  }, [isStreaming])

  const displayDuration = isStreaming ? elapsed.toFixed(1) : (durationMs ? (durationMs / 1000).toFixed(1) : elapsed.toFixed(1))

  if (!content && !isStreaming) return null

  return (
    <div className="mb-4">
      <button
        type="button"
        className="flex items-center gap-1.5 text-xs text-muted-foreground hover:text-foreground transition-colors"
        onClick={handleToggle}
      >
        {expanded ? <ChevronDown className="h-3.5 w-3.5" /> : <ChevronRight className="h-3.5 w-3.5" />}
        {isStreaming && (
          <span className="relative flex h-2 w-2">
            <span className="animate-ping absolute inline-flex h-full w-full rounded-full bg-primary/60" />
            <span className="relative inline-flex rounded-full h-2 w-2 bg-primary" />
          </span>
        )}
        <span>
          {isStreaming
            ? `${t("ai:chat.thinkingProcess")} ${displayDuration}s`
            : t("ai:chat.thinkingDuration", { duration: displayDuration })}
        </span>
      </button>
      {expanded && (
        <div className="mt-2 ml-5 pl-3 border-l-2 border-border/50 text-sm text-muted-foreground leading-relaxed whitespace-pre-wrap">
          {content}
          {isStreaming && (
            <span className="inline-block w-1.5 h-3.5 bg-muted-foreground/40 ml-0.5 animate-pulse" />
          )}
        </div>
      )}
    </div>
  )
}
