"use client"

import { useRef, useState, useEffect, useCallback } from "react"
import { useTranslation } from "react-i18next"
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query"
import { Upload, Trash2, FileText, Loader2 } from "lucide-react"
import { toast } from "sonner"
import { Button } from "@/components/ui/button"
import { Badge } from "@/components/ui/badge"
import {
  Table, TableBody, TableCell, TableHead, TableHeader, TableRow,
} from "@/components/ui/table"
import {
  DataTableCard, DataTableEmptyRow, DataTableLoadingRow,
  DataTableActions, DataTableActionsCell, DataTableActionsHead,
} from "@/components/ui/data-table"
import {
  AlertDialog, AlertDialogAction, AlertDialogCancel, AlertDialogContent,
  AlertDialogDescription, AlertDialogFooter, AlertDialogHeader,
  AlertDialogTitle, AlertDialogTrigger,
} from "@/components/ui/alert-dialog"
import {
  fetchKnowledgeDocs, uploadKnowledgeDoc, deleteKnowledgeDoc,
} from "../api"

function formatFileSize(bytes: number): string {
  if (bytes < 1024) return `${bytes} B`
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`
  return `${(bytes / (1024 * 1024)).toFixed(1)} MB`
}

function ParseStatusBadge({ status }: { status: string }) {
  const { t } = useTranslation("itsm")
  switch (status) {
    case "completed":
      return <Badge variant="default" className="bg-green-600">{t("knowledge.statusCompleted")}</Badge>
    case "processing":
      return <Badge variant="default" className="bg-blue-600"><Loader2 className="mr-1 h-3 w-3 animate-spin" />{t("knowledge.statusProcessing")}</Badge>
    case "failed":
      return <Badge variant="destructive">{t("knowledge.statusFailed")}</Badge>
    default:
      return <Badge variant="secondary">{t("knowledge.statusPending")}</Badge>
  }
}

export function ServiceKnowledgeCard({ serviceId }: { serviceId: number }) {
  const { t } = useTranslation(["itsm", "common"])
  const queryClient = useQueryClient()
  const fileInputRef = useRef<HTMLInputElement>(null)
  const [uploading, setUploading] = useState(false)

  const queryKey = ["itsm-knowledge-docs", serviceId]

  const { data: docs = [], isLoading } = useQuery({
    queryKey,
    queryFn: () => fetchKnowledgeDocs(serviceId),
    enabled: serviceId > 0,
  })

  // Auto-poll when any document is in processing state
  const hasProcessing = docs.some((d) => d.parseStatus === "processing")
  useEffect(() => {
    if (!hasProcessing || serviceId <= 0) return
    const timer = window.setTimeout(() => {
      queryClient.invalidateQueries({ queryKey })
    }, 2000)
    return () => window.clearTimeout(timer)
  }, [hasProcessing, serviceId, queryClient, queryKey])

  const handleUpload = useCallback(async (file: File) => {
    setUploading(true)
    try {
      await uploadKnowledgeDoc(serviceId, file)
      queryClient.invalidateQueries({ queryKey })
      toast.success(t("itsm:knowledge.uploadSuccess"))
    } catch (err) {
      toast.error((err as Error).message || t("itsm:knowledge.uploadError"))
    } finally {
      setUploading(false)
    }
  }, [serviceId, queryClient, queryKey, t])

  const deleteMut = useMutation({
    mutationFn: (docId: number) => deleteKnowledgeDoc(serviceId, docId),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey })
      toast.success(t("itsm:knowledge.deleteSuccess"))
    },
    onError: (err) => toast.error(err.message),
  })

  return (
    <div className="space-y-3">
      <div className="flex items-center justify-between">
        <div>
          <p className="text-xs text-muted-foreground">{t("itsm:knowledge.supportedFormats")}</p>
        </div>
        <Button
          size="sm"
          disabled={uploading}
          onClick={() => fileInputRef.current?.click()}
        >
          {uploading
            ? <><Loader2 className="mr-1.5 h-4 w-4 animate-spin" />{t("itsm:knowledge.uploading")}</>
            : <><Upload className="mr-1.5 h-4 w-4" />{t("itsm:knowledge.upload")}</>
          }
        </Button>
        <input
          ref={fileInputRef}
          type="file"
          className="hidden"
          accept=".pdf,.docx,.xlsx,.pptx,.txt,.md,.markdown"
          onChange={(e) => {
            const file = e.target.files?.[0]
            if (file) handleUpload(file)
            e.target.value = ""
          }}
        />
      </div>

      <DataTableCard>
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead className="min-w-[200px]">{t("itsm:knowledge.fileName")}</TableHead>
              <TableHead className="w-[100px]">{t("itsm:knowledge.fileSize")}</TableHead>
              <TableHead className="w-[120px]">{t("itsm:knowledge.parseStatus")}</TableHead>
              <TableHead className="w-[160px]">{t("itsm:knowledge.createdAt")}</TableHead>
              <DataTableActionsHead className="w-[80px]">{t("common:actions")}</DataTableActionsHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {isLoading ? (
              <DataTableLoadingRow colSpan={5} />
            ) : docs.length === 0 ? (
              <DataTableEmptyRow colSpan={5} icon={FileText} title={t("itsm:knowledge.empty")} description={t("itsm:knowledge.emptyHint")} />
            ) : (
              docs.map((doc) => (
                <TableRow key={doc.id}>
                  <TableCell className="font-medium">{doc.fileName}</TableCell>
                  <TableCell className="text-sm text-muted-foreground">{formatFileSize(doc.fileSize)}</TableCell>
                  <TableCell><ParseStatusBadge status={doc.parseStatus} /></TableCell>
                  <TableCell className="text-sm text-muted-foreground">{new Date(doc.createdAt).toLocaleString()}</TableCell>
                  <DataTableActionsCell>
                    <DataTableActions>
                      <AlertDialog>
                        <AlertDialogTrigger asChild>
                          <Button variant="ghost" size="sm" className="px-2.5 text-destructive hover:text-destructive">
                            <Trash2 className="mr-1 h-3.5 w-3.5" />{t("common:delete")}
                          </Button>
                        </AlertDialogTrigger>
                        <AlertDialogContent>
                          <AlertDialogHeader>
                            <AlertDialogTitle>{t("itsm:knowledge.deleteTitle")}</AlertDialogTitle>
                            <AlertDialogDescription>{t("itsm:knowledge.deleteDesc", { name: doc.fileName })}</AlertDialogDescription>
                          </AlertDialogHeader>
                          <AlertDialogFooter>
                            <AlertDialogCancel size="sm">{t("common:cancel")}</AlertDialogCancel>
                            <AlertDialogAction size="sm" onClick={() => deleteMut.mutate(doc.id)} disabled={deleteMut.isPending}>{t("itsm:knowledge.confirmDelete")}</AlertDialogAction>
                          </AlertDialogFooter>
                        </AlertDialogContent>
                      </AlertDialog>
                    </DataTableActions>
                  </DataTableActionsCell>
                </TableRow>
              ))
            )}
          </TableBody>
        </Table>
      </DataTableCard>
    </div>
  )
}
