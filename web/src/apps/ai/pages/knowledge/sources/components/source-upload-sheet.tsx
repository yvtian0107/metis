import { useRef, useState } from "react"
import { Upload, X, FileText, Loader2 } from "lucide-react"
import { useMutation, useQueryClient } from "@tanstack/react-query"
import { api } from "@/lib/api"
import { toast } from "sonner"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import {
  Sheet,
  SheetContent,
  SheetHeader,
  SheetTitle,
  SheetDescription,
  SheetFooter,
} from "@/components/ui/sheet"
import { formatBytes } from "@/lib/utils"

const ACCEPTED_EXTENSIONS = [".md", ".txt", ".pdf", ".docx", ".xlsx", ".pptx"]
const ACCEPTED_MIME = [
  "text/markdown",
  "text/plain",
  "application/pdf",
  "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
  "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
  "application/vnd.openxmlformats-officedocument.presentationml.presentation",
].join(",")

interface SourceUploadSheetProps {
  open: boolean
  onOpenChange: (open: boolean) => void
}

export function SourceUploadSheet({ open, onOpenChange }: SourceUploadSheetProps) {
  const queryClient = useQueryClient()
  const inputRef = useRef<HTMLInputElement>(null)
  const [file, setFile] = useState<File | null>(null)
  const [title, setTitle] = useState("")
  const [dragOver, setDragOver] = useState(false)

  const uploadMutation = useMutation({
    mutationFn: () => {
      if (!file) throw new Error("请选择文件")
      const formData = new FormData()
      formData.append("file", file)
      if (title.trim()) {
        formData.append("title", title.trim())
      }
      return api.upload("/api/v1/ai/knowledge/sources/upload", formData)
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["ai-knowledge-sources"] })
      toast.success("文件上传成功")
      handleClose()
    },
    onError: (err) => toast.error(err.message),
  })

  function handleFileChange(e: React.ChangeEvent<HTMLInputElement>) {
    const selected = e.target.files?.[0]
    if (selected) {
      setFile(selected)
      if (!title.trim()) {
        setTitle(selected.name.replace(/\.[^.]+$/, ""))
      }
    }
    if (inputRef.current) inputRef.current.value = ""
  }

  function handleDrop(e: React.DragEvent) {
    e.preventDefault()
    setDragOver(false)
    const dropped = Array.from(e.dataTransfer.files).find((f) => {
      const ext = "." + f.name.split(".").pop()?.toLowerCase()
      return ACCEPTED_EXTENSIONS.includes(ext)
    })
    if (dropped) {
      setFile(dropped)
      if (!title.trim()) {
        setTitle(dropped.name.replace(/\.[^.]+$/, ""))
      }
    }
  }

  function handleClose() {
    setFile(null)
    setTitle("")
    onOpenChange(false)
  }

  function handleOpenChange(val: boolean) {
    if (!uploadMutation.isPending) {
      if (!val) handleClose()
      else onOpenChange(val)
    }
  }

  return (
    <Sheet open={open} onOpenChange={handleOpenChange}>
      <SheetContent className="sm:max-w-lg overflow-y-auto">
        <SheetHeader>
          <SheetTitle>上传文件</SheetTitle>
          <SheetDescription className="sr-only">上传文件</SheetDescription>
        </SheetHeader>
        <div className="flex flex-1 flex-col gap-5 px-4">
          <div
            className={`relative flex flex-col items-center justify-center gap-3 rounded-lg border-2 border-dashed p-8 transition-colors cursor-pointer
              ${dragOver ? "border-primary bg-primary/5" : "border-muted-foreground/25 hover:border-muted-foreground/50"}`}
            onDragOver={(e) => { e.preventDefault(); setDragOver(true) }}
            onDragLeave={() => setDragOver(false)}
            onDrop={handleDrop}
            onClick={() => inputRef.current?.click()}
          >
            <Upload className="h-8 w-8 text-muted-foreground" />
            <div className="text-center">
              <p className="text-sm font-medium">拖拽文件到此处，或点击选择</p>
              <p className="text-xs text-muted-foreground mt-1">
                {ACCEPTED_EXTENSIONS.join(", ")}
              </p>
            </div>
            <input
              ref={inputRef}
              type="file"
              accept={ACCEPTED_MIME}
              className="hidden"
              onChange={handleFileChange}
            />
          </div>

          {file && (
            <div className="flex items-center gap-2 rounded-md border bg-muted/30 px-3 py-2">
              <FileText className="h-4 w-4 text-muted-foreground shrink-0" />
              <span className="flex-1 truncate text-sm">{file.name}</span>
              <span className="text-xs text-muted-foreground shrink-0">{formatBytes(file.size)}</span>
              <button
                type="button"
                className="rounded p-0.5 hover:bg-accent text-muted-foreground"
                onClick={() => setFile(null)}
              >
                <X className="h-3.5 w-3.5" />
              </button>
            </div>
          )}

          <div className="space-y-2">
            <Label>标题（可选，默认使用文件名）</Label>
            <Input
              placeholder="输入素材标题"
              value={title}
              onChange={(e) => setTitle(e.target.value)}
            />
          </div>
        </div>
        <SheetFooter className="px-4">
          <Button
            size="sm"
            disabled={!file || uploadMutation.isPending}
            onClick={() => uploadMutation.mutate()}
          >
            {uploadMutation.isPending ? (
              <>
                <Loader2 className="mr-1.5 h-4 w-4 animate-spin" />
                上传中...
              </>
            ) : (
              "上传"
            )}
          </Button>
        </SheetFooter>
      </SheetContent>
    </Sheet>
  )
}
