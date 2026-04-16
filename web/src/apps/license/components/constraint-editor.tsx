import { useRef, useState, useMemo } from "react"
import { useMutation, useQueryClient } from "@tanstack/react-query"
import { useTranslation } from "react-i18next"
import {
  Plus,
  Trash2,
  Save,
  ChevronDown,
  ChevronRight,
  Hash,
  List,
  ListChecks,
  Settings,
} from "lucide-react"
import { api } from "@/lib/api"
import { toast } from "sonner"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Badge } from "@/components/ui/badge"

interface ConstraintFeature {
  key: string
  label: string
  type: string
  min?: number
  max?: number
  default?: unknown
  options?: string[]
}

interface ConstraintModule {
  key: string
  label: string
  features: ConstraintFeature[]
}

interface ConstraintEditorProps {
  productId: number
  schema: ConstraintModule[] | null
  canEdit: boolean
}

const TYPE_ICONS: Record<string, typeof Hash> = {
  number: Hash,
  enum: List,
  multiSelect: ListChecks,
}

function useKeyCounter() {
  const [randPrefix] = useState(() => `${Date.now()}_${Math.random().toString(36).slice(2, 7)}`)
  const counterRef = useRef(0)
  return (prefix: string) => `${prefix}_${randPrefix}_${counterRef.current++}`
}

export function ConstraintEditor({ productId, schema, canEdit }: ConstraintEditorProps) {
  const { t } = useTranslation(["license", "common"])
  const queryClient = useQueryClient()
  const nextKey = useKeyCounter()
  const [modules, setModules] = useState<ConstraintModule[]>(() =>
    schema && Array.isArray(schema) ? structuredClone(schema) : [],
  )
  const initialJSON = useMemo(() => JSON.stringify(schema ?? []), [schema])

  const saveMutation = useMutation({
    mutationFn: () =>
      api.put(`/api/v1/license/products/${productId}/schema`, {
        constraintSchema: modules,
      }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["license-product", String(productId)] })
      toast.success(t("license:constraints.saved"))
    },
    onError: (err) => toast.error(err.message),
  })

  const hasChanges = useMemo(() => JSON.stringify(modules) !== initialJSON, [modules, initialJSON])

  function handleAddModule() {
    const key = nextKey("module")
    setModules([...modules, { key, label: "", features: [] }])
  }

  function handleRemoveModule(index: number) {
    setModules(modules.filter((_, i) => i !== index))
  }

  function handleUpdateModule(index: number, updated: ConstraintModule) {
    setModules(modules.map((m, i) => (i === index ? updated : m)))
  }

  function handleAddFeature(moduleIndex: number, type: "number" | "enum" | "multiSelect") {
    const mod = modules[moduleIndex]
    const key = nextKey("feat")
    let feature: ConstraintFeature
    if (type === "number") {
      feature = { key, label: "", type: "number", min: 0, max: 9999, default: 0 }
    } else if (type === "enum") {
      feature = { key, label: "", type: "enum", options: [] }
    } else {
      feature = { key, label: "", type: "multiSelect", options: [] }
    }
    handleUpdateModule(moduleIndex, { ...mod, features: [...mod.features, feature] })
  }

  function handleRemoveFeature(moduleIndex: number, featureIndex: number) {
    const mod = modules[moduleIndex]
    handleUpdateModule(moduleIndex, {
      ...mod,
      features: mod.features.filter((_, i) => i !== featureIndex),
    })
  }

  function handleUpdateFeature(moduleIndex: number, featureIndex: number, updated: ConstraintFeature) {
    const mod = modules[moduleIndex]
    handleUpdateModule(moduleIndex, {
      ...mod,
      features: mod.features.map((f, i) => (i === featureIndex ? updated : f)),
    })
  }

  return (
    <div className="space-y-4">
      {modules.length === 0 && (
        <div className="rounded-lg border border-dashed bg-muted/20 px-6 py-10 text-center">
          <p className="text-sm text-muted-foreground">
            {t("license:constraints.emptyHint")}
          </p>
        </div>
      )}

      {modules.map((mod, moduleIndex) => (
        <ModuleCard
          key={mod.key}
          module={mod}
          disabled={!canEdit}
          onUpdate={(updated) => handleUpdateModule(moduleIndex, updated)}
          onRemove={() => handleRemoveModule(moduleIndex)}
          onAddFeature={(type) => handleAddFeature(moduleIndex, type)}
          onRemoveFeature={(fi) => handleRemoveFeature(moduleIndex, fi)}
          onUpdateFeature={(fi, updated) => handleUpdateFeature(moduleIndex, fi, updated)}
        />
      ))}

      {canEdit && (
        <div className="flex items-center gap-2">
          <Button variant="outline" size="sm" className="rounded-lg" onClick={handleAddModule}>
            <Plus className="mr-1.5 h-4 w-4" />
            {t("license:constraints.addModule")}
          </Button>
        </div>
      )}

      {canEdit && hasChanges && (
        <div className="flex gap-2 border-t pt-3">
          <Button size="sm" className="rounded-lg" onClick={() => saveMutation.mutate()} disabled={saveMutation.isPending}>
            <Save className="mr-1.5 h-4 w-4" />
            {saveMutation.isPending ? t("common:saving") : t("common:save")}
          </Button>
          <Button
            variant="outline"
            size="sm"
            className="rounded-lg"
            onClick={() =>
              setModules(schema && Array.isArray(schema) ? structuredClone(schema) : [])
            }
          >
            {t("common:reset")}
          </Button>
        </div>
      )}
    </div>
  )
}

// --- Module card ---

function ModuleCard({
  module: mod,
  disabled,
  onUpdate,
  onRemove,
  onAddFeature,
  onRemoveFeature,
  onUpdateFeature,
}: {
  module: ConstraintModule
  disabled: boolean
  onUpdate: (m: ConstraintModule) => void
  onRemove: () => void
  onAddFeature: (type: "number" | "enum" | "multiSelect") => void
  onRemoveFeature: (index: number) => void
  onUpdateFeature: (index: number, f: ConstraintFeature) => void
}) {
  const { t } = useTranslation("license")
  const [expanded, setExpanded] = useState(true)
  const [showKey, setShowKey] = useState(false)

  return (
    <div className="rounded-lg border bg-muted/10">
      <div className="flex items-center gap-2 border-b bg-muted/20 px-4 py-3">
        <button
          type="button"
          className="rounded-md p-1 text-muted-foreground transition-colors hover:text-foreground"
          onClick={() => setExpanded(!expanded)}
        >
          {expanded ? <ChevronDown className="h-4 w-4" /> : <ChevronRight className="h-4 w-4" />}
        </button>
        <Input
          value={mod.label}
          onChange={(e) => onUpdate({ ...mod, label: e.target.value })}
          placeholder={t("constraints.modulePlaceholder")}
          className="h-8 max-w-[260px] flex-1 border-0 bg-transparent px-0 text-sm font-medium shadow-none focus-visible:ring-0"
          disabled={disabled}
        />
        {mod.features.length > 0 && (
          <Badge variant="outline" className="rounded-md border-0 bg-muted/60 text-[11px] text-muted-foreground">
            {t("constraints.itemCount", { count: mod.features.length })}
          </Badge>
        )}
        <span className="flex-1" />
        {!disabled && (
          <>
            <button
              type="button"
              className="rounded-md p-1 text-muted-foreground transition-colors hover:text-foreground"
              onClick={() => setShowKey(!showKey)}
              title={t("constraints.editKey")}
            >
              <Settings className="h-3.5 w-3.5" />
            </button>
            <Button
              variant="ghost"
              size="icon"
              onClick={onRemove}
              className="h-7 w-7 shrink-0 rounded-md text-muted-foreground hover:text-destructive"
            >
              <Trash2 className="h-3.5 w-3.5" />
            </Button>
          </>
        )}
      </div>

      {showKey && !disabled && (
        <div className="mx-4 mb-4 flex items-center gap-2 rounded-md bg-background/60 px-3 py-2">
          <Label className="text-xs text-muted-foreground shrink-0">{t("constraints.identifierKey")}</Label>
          <Input
            value={mod.key}
            onChange={(e) => onUpdate({ ...mod, key: e.target.value })}
            placeholder={t("constraints.moduleKeyPlaceholder")}
            className="h-8 max-w-[220px] font-mono text-xs"
          />
        </div>
      )}

      {expanded && (
        <div className="px-4 pb-4">
          {mod.features.length === 0 && (
            <p className="py-4 text-xs text-muted-foreground">
              {t("constraints.noFeatureHint")}
            </p>
          )}

          {mod.features.length > 0 && (
            <div className="space-y-1 pt-3">
              {mod.features.map((feature, featureIndex) => (
                <FeatureRow
                  key={feature.key}
                  feature={feature}
                  disabled={disabled}
                  onUpdate={(f) => onUpdateFeature(featureIndex, f)}
                  onRemove={() => onRemoveFeature(featureIndex)}
                />
              ))}
            </div>
          )}

          {!disabled && (
            <div className="mt-4 flex flex-wrap gap-1.5 border-t pt-3">
              <Button variant="outline" size="sm" className="h-7 rounded-md px-2 text-xs" onClick={() => onAddFeature("number")}>
                <Hash className="mr-1 h-3 w-3" />
                {t("constraints.number")}
              </Button>
              <Button variant="outline" size="sm" className="h-7 rounded-md px-2 text-xs" onClick={() => onAddFeature("enum")}>
                <List className="mr-1 h-3 w-3" />
                {t("constraints.enum")}
              </Button>
              <Button variant="outline" size="sm" className="h-7 rounded-md px-2 text-xs" onClick={() => onAddFeature("multiSelect")}>
                <ListChecks className="mr-1 h-3 w-3" />
                {t("constraints.multiSelect")}
              </Button>
            </div>
          )}
        </div>
      )}
    </div>
  )
}

// --- Feature row ---

function featureSummary(feature: ConstraintFeature, t: (key: string, opts?: object) => string): string {
  if (feature.type === "number") {
    const parts: string[] = []
    if (feature.min != null) parts.push(`${feature.min}`)
    if (feature.max != null) parts.push(`${feature.max}`)
    if (parts.length === 2) return `${parts[0]} ~ ${parts[1]}`
    if (feature.default != null) return t("constraints.defaultValue", { value: feature.default })
    return ""
  }
  if (feature.type === "enum" || feature.type === "multiSelect") {
    const count = (feature.options ?? []).length
    return count > 0 ? t("constraints.optionCount", { count }) : ""
  }
  return ""
}

function FeatureRow({
  feature,
  disabled,
  onUpdate,
  onRemove,
}: {
  feature: ConstraintFeature
  disabled: boolean
  onUpdate: (f: ConstraintFeature) => void
  onRemove: () => void
}) {
  const { t } = useTranslation("license")
  const [showAdvanced, setShowAdvanced] = useState(false)
  const Icon = TYPE_ICONS[feature.type] ?? Hash

  return (
    <div className="rounded-md bg-background/60 px-3 py-3">
      <div className="flex items-center gap-2">
        <div className="flex h-7 w-7 shrink-0 items-center justify-center rounded-md bg-muted/50 text-muted-foreground">
          <Icon className="h-3.5 w-3.5" />
        </div>
        <Input
          value={feature.label}
          onChange={(e) => onUpdate({ ...feature, label: e.target.value })}
          placeholder={t("constraints.featurePlaceholder")}
          className="h-8 max-w-[220px] flex-1 border-0 bg-transparent px-0 text-sm font-medium shadow-none focus-visible:ring-0"
          disabled={disabled}
        />
        <Badge variant="secondary" className="shrink-0 rounded-md border-0 bg-muted/60 text-[10px]">
          {t(`constraints.${feature.type}` as const) ?? feature.type}
        </Badge>
        <span className="flex-1 truncate text-xs text-muted-foreground">
          {featureSummary(feature, t as (key: string, opts?: object) => string)}
        </span>
        {!disabled && (
          <>
            <button
              type="button"
              className="rounded-md p-1 text-muted-foreground transition-colors hover:text-foreground"
              onClick={() => setShowAdvanced(!showAdvanced)}
              title={t("constraints.advancedSettings")}
            >
              <Settings className="h-3.5 w-3.5" />
            </button>
            <Button
              variant="ghost"
              size="icon"
              onClick={onRemove}
              className="h-7 w-7 rounded-md text-muted-foreground hover:text-destructive"
            >
              <Trash2 className="h-3.5 w-3.5" />
            </Button>
          </>
        )}
      </div>

      {showAdvanced && !disabled && (
        <div className="mt-3 space-y-3 rounded-md bg-muted/30 px-3 py-3">
          <div className="flex items-center gap-2">
            <Label className="text-xs text-muted-foreground shrink-0 w-16">{t("constraints.identifierKey")}</Label>
            <Input
              value={feature.key}
              onChange={(e) => onUpdate({ ...feature, key: e.target.value })}
              className="h-8 max-w-[220px] font-mono text-xs"
            />
          </div>
          {feature.type === "number" && (
            <NumberFeatureEditor feature={feature} onChange={onUpdate} disabled={disabled} />
          )}
          {(feature.type === "enum" || feature.type === "multiSelect") && (
            <OptionsFeatureEditor feature={feature} onChange={onUpdate} disabled={disabled} />
          )}
        </div>
      )}
    </div>
  )
}

// --- Type-specific editors ---

function NumberFeatureEditor({
  feature,
  onChange,
  disabled,
}: {
  feature: ConstraintFeature
  onChange: (f: ConstraintFeature) => void
  disabled: boolean
}) {
  const { t } = useTranslation("license")
  return (
    <div className="grid gap-2 sm:grid-cols-3">
      <div className="rounded-md border bg-background/70 px-3 py-2">
        <Label className="text-[11px] text-muted-foreground">{t("constraints.min")}</Label>
        <Input
          type="number"
          value={feature.min ?? ""}
          onChange={(e) =>
            onChange({ ...feature, min: e.target.value ? Number(e.target.value) : undefined })
          }
          className="mt-1 h-8 border-0 bg-transparent px-0 text-sm shadow-none focus-visible:ring-0"
          disabled={disabled}
        />
      </div>
      <div className="rounded-md border bg-background/70 px-3 py-2">
        <Label className="text-[11px] text-muted-foreground">{t("constraints.max")}</Label>
        <Input
          type="number"
          value={feature.max ?? ""}
          onChange={(e) =>
            onChange({ ...feature, max: e.target.value ? Number(e.target.value) : undefined })
          }
          className="mt-1 h-8 border-0 bg-transparent px-0 text-sm shadow-none focus-visible:ring-0"
          disabled={disabled}
        />
      </div>
      <div className="rounded-md border bg-background/70 px-3 py-2">
        <Label className="text-[11px] text-muted-foreground">{t("constraints.default")}</Label>
        <Input
          type="number"
          value={feature.default != null ? String(feature.default) : ""}
          onChange={(e) =>
            onChange({ ...feature, default: e.target.value ? Number(e.target.value) : undefined })
          }
          className="mt-1 h-8 border-0 bg-transparent px-0 text-sm shadow-none focus-visible:ring-0"
          disabled={disabled}
        />
      </div>
    </div>
  )
}

function OptionsFeatureEditor({
  feature,
  onChange,
  disabled,
}: {
  feature: ConstraintFeature
  onChange: (f: ConstraintFeature) => void
  disabled: boolean
}) {
  const { t } = useTranslation("license")
  const [newOption, setNewOption] = useState("")
  const options = feature.options ?? []

  function addOption() {
    const trimmed = newOption.trim()
    if (!trimmed) return
    if (options.includes(trimmed)) return
    onChange({ ...feature, options: [...options, trimmed] })
    setNewOption("")
  }

  function removeOption(index: number) {
    onChange({ ...feature, options: options.filter((_, i) => i !== index) })
  }

  return (
    <div className="flex flex-wrap items-center gap-2">
      {options.map((opt, i) => (
        <Badge key={i} variant="secondary" className="gap-0.5 rounded-md border-0 bg-muted/60 text-xs font-normal">
          {opt}
          {!disabled && (
            <button
              type="button"
              className="ml-0.5 text-muted-foreground hover:text-destructive"
              onClick={() => removeOption(i)}
            >
              &times;
            </button>
          )}
        </Badge>
      ))}
      {!disabled && (
        <Input
          value={newOption}
          onChange={(e) => setNewOption(e.target.value)}
          placeholder={t("constraints.addOption")}
          className="h-8 w-32 rounded-md border-dashed bg-transparent text-xs"
          onKeyDown={(e) => {
            if (e.key === "Enter") {
              e.preventDefault()
              addOption()
            }
          }}
        />
      )}
    </div>
  )
}
