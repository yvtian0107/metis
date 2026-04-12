import { useState, useCallback, useMemo } from "react"
import { useTranslation } from "react-i18next"
import { ChevronDown, ChevronRight, Wrench, Copy, Check, RotateCcw, ThumbsUp, ThumbsDown, Pencil, Search } from "lucide-react"
import { type SessionMessage } from "@/lib/api"
import { Button } from "@/components/ui/button"
import { toast } from "sonner"
import ReactMarkdown from "react-markdown"
import { Prism as SyntaxHighlighter } from "react-syntax-highlighter"
import { vscDarkPlus } from "react-syntax-highlighter/dist/esm/styles/prism"
import remarkGfm from "remark-gfm"

interface QAPairProps {
  userMessage: SessionMessage
  aiMessage?: SessionMessage
  tools?: SessionMessage[]
  agentName?: string
  isStreaming?: boolean
  streamingContent?: string
  onRegenerate?: () => void
  onEditMessage?: (messageId: number, content: string) => void
  doneMetrics?: { durationMs?: number; inputTokens?: number; outputTokens?: number }
}

// Code block component with copy button
function CodeBlock({ language, code }: { language: string; code: string }) {
  const [copied, setCopied] = useState(false)

  const handleCopy = useCallback(async () => {
    try {
      await navigator.clipboard.writeText(code)
      setCopied(true)
      setTimeout(() => setCopied(false), 2000)
    } catch {
      toast.error("Copy failed")
    }
  }, [code])

  return (
    <div className="relative group rounded-lg overflow-hidden my-4">
      <div className="flex items-center justify-between px-3 py-1.5 bg-zinc-900 border-b border-zinc-800">
        <span className="text-xs text-zinc-400 font-mono">{language || "text"}</span>
        <button
          type="button"
          onClick={handleCopy}
          className="text-xs text-zinc-400 hover:text-zinc-200 transition-colors"
        >
          {copied ? "Copied" : "Copy"}
        </button>
      </div>
      <SyntaxHighlighter
        language={language || "text"}
        style={vscDarkPlus}
        customStyle={{ margin: 0, padding: "1rem", fontSize: "0.875rem", background: "#18181b" }}
      >
        {code}
      </SyntaxHighlighter>
    </div>
  )
}

// Markdown components configuration
const markdownComponents = {
  h1: ({ children }: { children?: React.ReactNode }) => <h1 className="text-2xl font-semibold mt-8 mb-4">{children}</h1>,
  h2: ({ children }: { children?: React.ReactNode }) => <h2 className="text-xl font-semibold mt-6 mb-3">{children}</h2>,
  h3: ({ children }: { children?: React.ReactNode }) => <h3 className="text-lg font-semibold mt-5 mb-2">{children}</h3>,
  p: ({ children }: { children?: React.ReactNode }) => <p className="leading-7 mb-4 last:mb-0">{children}</p>,
  ul: ({ children }: { children?: React.ReactNode }) => <ul className="list-disc pl-6 space-y-1 mb-4">{children}</ul>,
  ol: ({ children }: { children?: React.ReactNode }) => <ol className="list-decimal pl-6 space-y-1 mb-4">{children}</ol>,
  li: ({ children }: { children?: React.ReactNode }) => <li className="leading-7">{children}</li>,
  blockquote: ({ children }: { children?: React.ReactNode }) => (
    <blockquote className="border-l-2 border-border pl-4 italic text-muted-foreground my-4">{children}</blockquote>
  ),
  code: ({ className, children }: { className?: string; children?: React.ReactNode }) => {
    const language = className?.replace("language-", "") ?? ""
    const code = String(children).replace(/\n$/, "")
    if (!language && !code.includes("\n")) {
      return <code className="bg-muted px-1.5 py-0.5 rounded text-sm font-mono">{children}</code>
    }
    return <CodeBlock language={language} code={code} />
  },
  table: ({ children }: { children?: React.ReactNode }) => (
    <div className="overflow-x-auto my-4"><table className="w-full border-collapse">{children}</table></div>
  ),
  thead: ({ children }: { children?: React.ReactNode }) => <thead className="bg-muted">{children}</thead>,
  th: ({ children }: { children?: React.ReactNode }) => (
    <th className="border border-border px-3 py-2 text-left font-semibold text-sm">{children}</th>
  ),
  td: ({ children }: { children?: React.ReactNode }) => (
    <td className="border border-border px-3 py-2 text-sm">{children}</td>
  ),
  tr: ({ children }: { children?: React.ReactNode }) => <tr className="even:bg-muted/50">{children}</tr>,
  a: ({ children, href }: { children?: React.ReactNode; href?: string }) => (
    <a href={href} className="text-primary hover:underline" target="_blank" rel="noopener noreferrer">{children}</a>
  ),
  hr: () => <hr className="my-6 border-border" />,
}

// Memoized markdown content
const MarkdownContent = ({ content }: { content: string }) => {
  return (
    <ReactMarkdown remarkPlugins={[remarkGfm]} components={markdownComponents}>
      {content}
    </ReactMarkdown>
  )
}

// Tool call component with rich rendering
function ToolCall({ message }: { message: SessionMessage }) {
  const { t } = useTranslation(["ai"])
  const [expanded, setExpanded] = useState(false)
  const meta = message.metadata as { tool_name?: string; tool_args?: string; duration_ms?: number } | undefined
  const toolName = meta?.tool_name ?? "unknown"

  // Rich rendering for known tools
  const isKnowledgeSearch = toolName === "search_knowledge"
  let parsedArgs: Record<string, unknown> | null = null
  try {
    if (meta?.tool_args) parsedArgs = JSON.parse(meta.tool_args)
  } catch { /* ignore */ }

  return (
    <div className="py-2 my-2 rounded-lg border bg-muted/30 px-3">
      <button
        type="button"
        className="flex items-center gap-2 text-xs text-muted-foreground hover:text-foreground transition-colors w-full"
        onClick={() => setExpanded(!expanded)}
      >
        <div className="flex items-center justify-center h-5 w-5 rounded bg-amber-100 dark:bg-amber-900/30 shrink-0">
          {isKnowledgeSearch
            ? <Search className="h-3 w-3 text-amber-600 dark:text-amber-400" />
            : <Wrench className="h-3 w-3 text-amber-600 dark:text-amber-400" />}
        </div>
        {expanded ? <ChevronDown className="h-3 w-3 shrink-0" /> : <ChevronRight className="h-3 w-3 shrink-0" />}
        <span className="truncate">
          {isKnowledgeSearch && parsedArgs
            ? `${t("ai:tools.toolDefs.search_knowledge.name")}: "${parsedArgs.query ?? ""}"`
            : t("ai:chat.toolCall", { name: toolName })}
        </span>
        {meta?.duration_ms != null && (
          <span className="ml-auto text-[10px] text-muted-foreground/60 shrink-0">
            {(meta.duration_ms / 1000).toFixed(1)}s
          </span>
        )}
      </button>
      {expanded && meta?.tool_args && (
        <pre className="mt-2 text-xs bg-muted rounded-md p-3 overflow-auto max-h-48 font-mono">
          {meta.tool_args}
        </pre>
      )}
    </div>
  )
}

// Tool result component
function ToolResult({ message }: { message: SessionMessage }) {
  const { t } = useTranslation(["ai"])
  const [expanded, setExpanded] = useState(false)

  return (
    <div className="py-2 my-2 ml-4">
      <button
        type="button"
        className="flex items-center gap-2 text-xs text-muted-foreground hover:text-foreground transition-colors"
        onClick={() => setExpanded(!expanded)}
      >
        {expanded ? <ChevronDown className="h-3 w-3" /> : <ChevronRight className="h-3 w-3" />}
        <span>{t("ai:chat.toolResult")}</span>
      </button>
      {expanded && (
        <pre className="mt-2 text-xs bg-muted rounded-md p-3 overflow-auto max-h-48 font-mono">
          {message.content}
        </pre>
      )}
    </div>
  )
}

// User query — right-aligned pill (ChatGPT style)
function UserQuery({
  content,
  images,
  messageId,
  onEdit,
}: {
  content: string
  images?: string[]
  messageId?: number
  onEdit?: (messageId: number, content: string) => void
}) {
  const { t } = useTranslation(["ai"])
  const [editing, setEditing] = useState(false)
  const [editContent, setEditContent] = useState(content)

  const handleSave = useCallback(() => {
    const trimmed = editContent.trim()
    if (!trimmed || !messageId || !onEdit) return
    onEdit(messageId, trimmed)
    setEditing(false)
  }, [editContent, messageId, onEdit])

  const handleCancel = useCallback(() => {
    setEditContent(content)
    setEditing(false)
  }, [content])

  if (editing) {
    return (
      <div className="flex justify-end mb-6">
        <div className="max-w-[70%] w-full">
          <div className="rounded-3xl bg-secondary p-4">
            <textarea
              value={editContent}
              onChange={(e) => setEditContent(e.target.value)}
              className="w-full min-h-[60px] max-h-[200px] resize-none bg-transparent text-sm leading-relaxed focus:outline-none"
              autoFocus
            />
            <div className="flex items-center gap-2 mt-3">
              <Button size="sm" onClick={handleSave} disabled={!editContent.trim()}>
                {t("ai:chat.saveAndRegenerate")}
              </Button>
              <Button size="sm" variant="ghost" onClick={handleCancel}>
                {t("ai:chat.cancelEdit")}
              </Button>
            </div>
          </div>
        </div>
      </div>
    )
  }

  return (
    <div className="group flex justify-end mb-6">
      <div className="flex items-start gap-1.5 max-w-[70%]">
        {/* Edit button — appears on hover to the left of the pill */}
        {onEdit && messageId && (
          <Button
            variant="ghost"
            size="icon"
            className="h-7 w-7 shrink-0 mt-1 text-muted-foreground/0 group-hover:text-muted-foreground hover:!text-foreground transition-colors"
            onClick={() => setEditing(true)}
          >
            <Pencil className="h-3.5 w-3.5" />
          </Button>
        )}
        <div className="rounded-3xl bg-secondary px-5 py-2.5">
          {images && images.length > 0 && (
            <div className="flex flex-wrap gap-2 mb-2">
              {images.map((src, idx) => (
                <img
                  key={idx}
                  src={src}
                  alt={`image-${idx}`}
                  className="max-h-48 max-w-xs rounded-xl object-cover"
                />
              ))}
            </div>
          )}
          {content && <div className="text-[15px] leading-relaxed whitespace-pre-wrap">{content}</div>}
        </div>
      </div>
    </div>
  )
}

// AI Response display
export function AIResponse({
  content,
  agentName,
  isStreaming,
  onRegenerate,
  doneMetrics,
}: {
  content: string
  agentName?: string
  isStreaming?: boolean
  onRegenerate?: () => void
  doneMetrics?: { durationMs?: number; inputTokens?: number; outputTokens?: number }
}) {
  const { t } = useTranslation(["ai"])
  const [copied, setCopied] = useState(false)

  const handleCopy = useCallback(async () => {
    try {
      await navigator.clipboard.writeText(content)
      setCopied(true)
      setTimeout(() => setCopied(false), 2000)
    } catch {
      toast.error("Copy failed")
    }
  }, [content])

  const markdownContent = useMemo(() => {
    return <MarkdownContent content={content} />
  }, [content])

  // Format metrics
  const metricsText = useMemo(() => {
    if (!doneMetrics?.durationMs) return null
    const parts: string[] = []
    const durationSec = (doneMetrics.durationMs / 1000).toFixed(1)
    if (doneMetrics.outputTokens && doneMetrics.durationMs > 0) {
      const tokPerSec = Math.round((doneMetrics.outputTokens / doneMetrics.durationMs) * 1000)
      parts.push(`${tokPerSec} tok/s`)
    }
    parts.push(`${durationSec}s`)
    if (doneMetrics.outputTokens) {
      parts.push(`${doneMetrics.outputTokens} tokens`)
    }
    return parts.join(" · ")
  }, [doneMetrics])

  return (
    <div className="mb-6">
      {agentName && (
        <div className="text-xs font-medium text-muted-foreground mb-1.5">{agentName}</div>
      )}
      <div className="text-base leading-relaxed">
        {markdownContent}
        {isStreaming && (
          <span className="inline-block w-2 h-4 bg-foreground/40 ml-1 animate-pulse" />
        )}
      </div>

      {/* Action buttons — copy always visible, others on hover */}
      {!isStreaming && content && (
        <div className="flex items-center gap-1 mt-3">
          <Button
            variant="ghost"
            size="sm"
            className="h-7 px-2 text-xs text-muted-foreground hover:text-foreground"
            onClick={handleCopy}
          >
            {copied
              ? <><Check className="h-3.5 w-3.5 mr-1" />{t("ai:chat.copied")}</>
              : <><Copy className="h-3.5 w-3.5 mr-1" />{t("ai:chat.copy")}</>}
          </Button>
          {onRegenerate && (
            <Button
              variant="ghost"
              size="sm"
              className="h-7 px-2 text-xs text-muted-foreground hover:text-foreground"
              onClick={onRegenerate}
            >
              <RotateCcw className="h-3.5 w-3.5 mr-1" />
              {t("ai:chat.regenerate")}
            </Button>
          )}
          <Button variant="ghost" size="sm" className="h-7 w-7 p-0 text-muted-foreground hover:text-foreground">
            <ThumbsUp className="h-3.5 w-3.5" />
          </Button>
          <Button variant="ghost" size="sm" className="h-7 w-7 p-0 text-muted-foreground hover:text-foreground">
            <ThumbsDown className="h-3.5 w-3.5" />
          </Button>
          {/* Performance metrics — always visible on right */}
          {metricsText && (
            <span className="ml-auto text-[10px] text-muted-foreground/50">{metricsText}</span>
          )}
        </div>
      )}
    </div>
  )
}

// QA Pair component — document-flow layout
export function QAPair({
  userMessage,
  aiMessage,
  tools,
  agentName,
  isStreaming,
  streamingContent,
  onRegenerate,
  onEditMessage,
  doneMetrics,
}: QAPairProps) {
  const displayContent = isStreaming && streamingContent ? streamingContent : (aiMessage?.content ?? "")

  const userImages = (userMessage.metadata as { images?: string[] } | undefined)?.images

  return (
    <div className="py-6">
      <UserQuery
        content={userMessage.content}
        images={userImages}
        messageId={userMessage.id}
        onEdit={onEditMessage}
      />

      {/* Tool calls */}
      {tools?.map((tool) =>
        tool.role === "tool_call"
          ? <ToolCall key={tool.id} message={tool} />
          : <ToolResult key={tool.id} message={tool} />
      )}

      {/* AI response */}
      {(aiMessage || (isStreaming && displayContent)) && (
        <AIResponse
          content={displayContent}
          agentName={agentName}
          isStreaming={isStreaming}
          onRegenerate={onRegenerate}
          doneMetrics={doneMetrics}
        />
      )}
    </div>
  )
}

// Legacy single message item (for tool calls and simple display)
export function MessageItem({
  message,
  isStreaming,
  onRegenerate,
}: {
  message: SessionMessage
  isStreaming?: boolean
  onRegenerate?: () => void
}) {
  if (message.role === "tool_call") return <ToolCall message={message} />
  if (message.role === "tool_result") return <ToolResult message={message} />
  if (message.role === "user") {
    return <div className="py-4"><UserQuery content={message.content} /></div>
  }
  return (
    <div className="py-4">
      <AIResponse content={message.content} isStreaming={isStreaming} onRegenerate={onRegenerate} />
    </div>
  )
}

export type { QAPairProps }
