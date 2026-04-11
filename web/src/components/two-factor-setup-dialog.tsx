import { useState } from "react"
import { useTranslation } from "react-i18next"
import { useMutation, useQueryClient } from "@tanstack/react-query"
import QRCode from "react-qr-code"
import { api } from "@/lib/api"
import { toast } from "sonner"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog"

interface TwoFactorSetupDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  enabled: boolean
}

type Step = "idle" | "qr" | "backup"

interface SetupData {
  secret: string
  qrUri: string
}

export function TwoFactorSetupDialog({
  open,
  onOpenChange,
  enabled,
}: TwoFactorSetupDialogProps) {
  const queryClient = useQueryClient()
  const [step, setStep] = useState<Step>("idle")
  const [setupData, setSetupData] = useState<SetupData | null>(null)
  const [code, setCode] = useState("")
  const [backupCodes, setBackupCodes] = useState<string[]>([])
  const [error, setError] = useState("")
  const { t } = useTranslation("auth")

  const setupMutation = useMutation({
    mutationFn: () => api.post<SetupData>("/api/v1/auth/2fa/setup", {}),
    onSuccess: (data) => {
      setSetupData(data)
      setStep("qr")
      setError("")
    },
    onError: (err: Error) => setError(err.message),
  })

  const confirmMutation = useMutation({
    mutationFn: () => api.post<{ backupCodes: string[] }>("/api/v1/auth/2fa/confirm", { code }),
    onSuccess: (data) => {
      setBackupCodes(data.backupCodes)
      setStep("backup")
      setError("")
      queryClient.invalidateQueries({ queryKey: ["auth", "me"] })
    },
    onError: (err: Error) => setError(err.message),
  })

  const disableMutation = useMutation({
    mutationFn: () => api.delete("/api/v1/auth/2fa"),
    onSuccess: () => {
      toast.success(t("twoFactorSetup.disabled"))
      queryClient.invalidateQueries({ queryKey: ["auth", "me"] })
      handleClose()
    },
    onError: (err: Error) => toast.error(err.message),
  })

  function handleClose() {
    setStep("idle")
    setSetupData(null)
    setCode("")
    setBackupCodes([])
    setError("")
    onOpenChange(false)
  }

  return (
    <Dialog open={open} onOpenChange={handleClose}>
      <DialogContent className="max-w-md">
        <DialogHeader>
          <DialogTitle>{t("twoFactorSetup.title")}</DialogTitle>
          <DialogDescription>
            {enabled ? t("twoFactorSetup.descEnabled") : t("twoFactorSetup.descDisabled")}
          </DialogDescription>
        </DialogHeader>

        {/* Idle: show enable/disable */}
        {step === "idle" && (
          <div className="space-y-4">
            {enabled ? (
              <div className="space-y-3">
                <p className="text-sm text-muted-foreground">
                  {t("twoFactorSetup.enabledMessage")}
                </p>
                <Button
                  variant="destructive"
                  onClick={() => disableMutation.mutate()}
                  disabled={disableMutation.isPending}
                >
                  {disableMutation.isPending ? t("twoFactorSetup.disabling") : t("twoFactorSetup.disable")}
                </Button>
              </div>
            ) : (
              <div className="space-y-3">
                <p className="text-sm text-muted-foreground">
                  {t("twoFactorSetup.enableMessage")}
                </p>
                <Button
                  onClick={() => setupMutation.mutate()}
                  disabled={setupMutation.isPending}
                >
                  {setupMutation.isPending ? t("twoFactorSetup.generating") : t("twoFactorSetup.startSetup")}
                </Button>
              </div>
            )}
          </div>
        )}

        {/* QR code step */}
        {step === "qr" && setupData && (
          <div className="space-y-4">
            <p className="text-sm text-muted-foreground">
              {t("twoFactorSetup.scanQR")}
            </p>
            <div className="flex justify-center rounded-lg border bg-white p-4">
              <QRCode value={setupData.qrUri} size={200} />
            </div>
            <details className="text-xs text-muted-foreground">
              <summary className="cursor-pointer">{t("twoFactorSetup.cantScan")}</summary>
              <code className="mt-1 block break-all rounded bg-muted p-2 font-mono text-xs">
                {setupData.secret}
              </code>
            </details>
            <div className="space-y-2">
              <Label htmlFor="totp-code">{t("twoFactorSetup.enterCode")}</Label>
              <Input
                id="totp-code"
                placeholder={t("twoFactorSetup.codePlaceholder")}
                value={code}
                onChange={(e) => setCode(e.target.value)}
                autoComplete="one-time-code"
              />
            </div>
            {error && <p className="text-sm text-destructive">{error}</p>}
            <Button
              className="w-full"
              onClick={() => confirmMutation.mutate()}
              disabled={!code || confirmMutation.isPending}
            >
              {confirmMutation.isPending ? t("twoFactorSetup.confirming") : t("twoFactorSetup.confirm")}
            </Button>
          </div>
        )}

        {/* Backup codes step */}
        {step === "backup" && (
          <div className="space-y-4">
            <p className="text-sm font-medium text-green-600">
              {t("twoFactorSetup.enabledSuccess")}
            </p>
            <p className="text-sm text-muted-foreground">
              {t("twoFactorSetup.saveRecoveryCodes")}
            </p>
            <div className="grid grid-cols-2 gap-2 rounded-lg border bg-muted p-3">
              {backupCodes.map((c) => (
                <code key={c} className="font-mono text-sm">
                  {c}
                </code>
              ))}
            </div>
            <Button className="w-full" onClick={handleClose}>
              {t("twoFactorSetup.done")}
            </Button>
          </div>
        )}
      </DialogContent>
    </Dialog>
  )
}
