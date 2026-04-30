"use client"

import { useTranslation } from "react-i18next"
import { Label } from "@/components/ui/label"
import { Textarea } from "@/components/ui/textarea"

interface CollaborationSpecFieldProps {
  label: string
  value: string
  placeholder: string
  assistText?: string
  onChange: (value: string) => void
}

export function CollaborationSpecField({
  label,
  value,
  placeholder,
  assistText,
  onChange,
}: CollaborationSpecFieldProps) {
  const { t } = useTranslation("itsm")
  const charCount = Array.from(value).length

  return (
    <section className="space-y-2">
      <div className="flex items-center justify-between gap-3">
        <Label className="text-sm font-medium text-foreground/90">{label}</Label>
        <span className="text-xs tabular-nums text-muted-foreground">
          {t("smart.collaborationSpecCount", { count: charCount })}
        </span>
      </div>

      <div className="rounded-lg border border-border/60 bg-background/70 px-3 py-2 shadow-[inset_0_1px_0_rgba(255,255,255,0.62)] transition-colors focus-within:border-ring/70 focus-within:ring-2 focus-within:ring-ring/25">
        <Textarea
          rows={8}
          className="min-h-[188px] resize-y border-none bg-transparent px-1 py-1.5 leading-6 shadow-none focus-visible:ring-0"
          placeholder={placeholder}
          value={value}
          onChange={(e) => onChange(e.target.value)}
        />
      </div>

      {assistText ? (
        <p className="px-1 text-xs leading-5 text-muted-foreground">{assistText}</p>
      ) : null}
    </section>
  )
}
