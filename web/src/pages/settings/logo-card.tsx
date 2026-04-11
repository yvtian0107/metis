import { useRef, useState } from "react"
import { useMutation, useQueryClient } from "@tanstack/react-query"
import { useTranslation } from "react-i18next"
import { Upload, Trash2 } from "lucide-react"
import { api } from "@/lib/api"
import { Button } from "@/components/ui/button"
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card"

const MAX_SIZE = 2 * 1024 * 1024 // 2MB

export function LogoCard({ hasLogo }: { hasLogo: boolean }) {
  const { t } = useTranslation("settings")
  const queryClient = useQueryClient()
  const fileRef = useRef<HTMLInputElement>(null)
  const [logoKey, setLogoKey] = useState(0)

  const uploadMutation = useMutation({
    mutationFn: (dataUrl: string) => api.put("/api/v1/site-info/logo", { logo: dataUrl }),
    onSuccess: () => {
      setLogoKey((k) => k + 1)
      queryClient.invalidateQueries({ queryKey: ["site-info"] })
    },
  })

  const deleteMutation = useMutation({
    mutationFn: () => api.delete("/api/v1/site-info/logo"),
    onSuccess: () => {
      setLogoKey((k) => k + 1)
      queryClient.invalidateQueries({ queryKey: ["site-info"] })
    },
  })

  function handleFile(e: React.ChangeEvent<HTMLInputElement>) {
    const file = e.target.files?.[0]
    if (!file) return

    if (file.size > MAX_SIZE) {
      alert(t("logo.fileTooLarge"))
      return
    }

    if (!file.type.startsWith("image/")) {
      alert(t("logo.invalidFileType"))
      return
    }

    const reader = new FileReader()
    reader.onload = () => {
      uploadMutation.mutate(reader.result as string)
    }
    reader.readAsDataURL(file)

    // Reset so the same file can be re-selected
    e.target.value = ""
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle>{t("logo.title")}</CardTitle>
        <CardDescription>{t("logo.description")}</CardDescription>
      </CardHeader>
      <CardContent>
        <div className="flex items-center gap-6">
          <div className="flex h-20 w-20 shrink-0 items-center justify-center rounded-lg border border-dashed bg-muted/30">
            {hasLogo ? (
              <img
                src={`/api/v1/site-info/logo?t=${logoKey}`}
                alt="Logo"
                className="h-full w-full rounded-lg object-contain"
              />
            ) : (
              <Upload className="h-8 w-8 text-muted-foreground/50" />
            )}
          </div>

          <div className="flex gap-2">
            <Button
              variant="outline"
              size="sm"
              disabled={uploadMutation.isPending}
              onClick={() => fileRef.current?.click()}
            >
              <Upload className="mr-1.5 h-4 w-4" />
              {hasLogo ? t("logo.change") : t("logo.upload")}
            </Button>

            {hasLogo && (
              <Button
                variant="outline"
                size="sm"
                disabled={deleteMutation.isPending}
                onClick={() => deleteMutation.mutate()}
              >
                <Trash2 className="mr-1.5 h-4 w-4" />
                {t("logo.remove")}
              </Button>
            )}
          </div>

          <input
            ref={fileRef}
            type="file"
            accept="image/*"
            className="hidden"
            onChange={handleFile}
          />
        </div>
      </CardContent>
    </Card>
  )
}
