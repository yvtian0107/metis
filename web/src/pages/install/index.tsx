import { useState, useCallback, useMemo } from "react"
import { useTranslation } from "react-i18next"
import { ChevronRight, Globe, Search } from "lucide-react"

import { supportedLocales, changeLocale } from "@/i18n"
import { AuthShell } from "@/components/auth/auth-shell"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Switch } from "@/components/ui/switch"

// ─── Types ───────────────────────────────────────────────────────────────────

interface LocaleConfig {
  locale: string
  timezone: string
}

interface DBConfig {
  driver: "sqlite" | "postgres"
  host: string
  port: string
  user: string
  password: string
  dbname: string
}

interface SiteConfig {
  siteName: string
}

interface AdminConfig {
  username: string
  password: string
  confirmPassword: string
  email: string
}

interface OTelConfig {
  enabled: boolean
  exporterEndpoint: string
  serviceName: string
  sampleRate: string
}

interface StepDef {
  id: string
  label: string
}

// ─── Common timezone list (IANA, grouped) ────────────────────────────────────

const TIMEZONE_LIST = [
  "UTC",
  "America/New_York",
  "America/Chicago",
  "America/Denver",
  "America/Los_Angeles",
  "America/Anchorage",
  "America/Sao_Paulo",
  "America/Argentina/Buenos_Aires",
  "America/Mexico_City",
  "America/Toronto",
  "America/Vancouver",
  "Europe/London",
  "Europe/Berlin",
  "Europe/Paris",
  "Europe/Moscow",
  "Europe/Istanbul",
  "Europe/Rome",
  "Europe/Madrid",
  "Europe/Amsterdam",
  "Asia/Shanghai",
  "Asia/Tokyo",
  "Asia/Seoul",
  "Asia/Singapore",
  "Asia/Hong_Kong",
  "Asia/Taipei",
  "Asia/Dubai",
  "Asia/Kolkata",
  "Asia/Bangkok",
  "Asia/Jakarta",
  "Australia/Sydney",
  "Australia/Melbourne",
  "Australia/Perth",
  "Pacific/Auckland",
  "Pacific/Honolulu",
  "Africa/Cairo",
  "Africa/Johannesburg",
  "Africa/Lagos",
]

function getBrowserTimezone(): string {
  try {
    return Intl.DateTimeFormat().resolvedOptions().timeZone
  } catch {
    return "UTC"
  }
}

// ─── API helpers (raw fetch, no auth needed) ─────────────────────────────────

async function apiPost<T>(url: string, data: unknown): Promise<T> {
  const res = await fetch(url, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(data),
  })
  const body = await res.json()
  if (!res.ok || body.code !== 0) {
    throw new Error(body.message || res.statusText)
  }
  return body.data as T
}

// ─── Step Indicator ──────────────────────────────────────────────────────────

function StepIndicator({ steps, current }: { steps: StepDef[]; current: number }) {
  return (
    <div className="flex items-center justify-center gap-0">
      {steps.map((step, i) => {
        const isCompleted = i < current
        const isCurrent = i === current
        return (
          <div key={step.id} className="flex items-center">
            {i > 0 && (
              <div
                className={`h-px w-6 sm:w-10 transition-colors duration-300 ${
                  isCompleted ? "bg-slate-900" : "bg-slate-200"
                }`}
              />
            )}
            <div className="flex flex-col items-center gap-1.5">
              <div
                className={`flex h-7 w-7 items-center justify-center rounded-full text-xs font-semibold transition-all duration-300 ${
                  isCompleted
                    ? "bg-slate-900 text-white"
                    : isCurrent
                      ? "border-2 border-slate-900 bg-white text-slate-900"
                      : "border border-slate-200 bg-white text-slate-400"
                }`}
              >
                {isCompleted ? (
                  <svg className="h-3.5 w-3.5" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="3" strokeLinecap="round" strokeLinejoin="round">
                    <polyline points="20 6 9 17 4 12" />
                  </svg>
                ) : (
                  i + 1
                )}
              </div>
              <span
                className={`text-[11px] font-medium whitespace-nowrap ${
                  isCurrent ? "text-slate-700" : isCompleted ? "text-slate-500" : "text-slate-400"
                }`}
              >
                {step.label}
              </span>
            </div>
          </div>
        )
      })}
    </div>
  )
}

// ─── Language & Timezone Step ────────────────────────────────────────────────

function LanguageStep({
  config,
  onChange,
  onNext,
}: {
  config: LocaleConfig
  onChange: (c: LocaleConfig) => void
  onNext: () => void
}) {
  const { t } = useTranslation("install")
  const [tzSearch, setTzSearch] = useState("")

  const filteredTimezones = useMemo(() => {
    if (!tzSearch) return TIMEZONE_LIST
    const q = tzSearch.toLowerCase()
    return TIMEZONE_LIST.filter((tz) => tz.toLowerCase().includes(q))
  }, [tzSearch])

  const handleLocaleChange = (locale: string) => {
    onChange({ ...config, locale })
    changeLocale(locale)
  }

  return (
    <div className="space-y-5">
      <div className="text-center">
        <h2 className="text-lg font-semibold tracking-[-0.02em] text-slate-900">
          {t("language.title")}
        </h2>
        <p className="mt-1 text-[13px] text-slate-400">
          {t("language.description")}
        </p>
      </div>

      {/* Language selection */}
      <div>
        <Label className="mb-1.5 block text-[13px] font-medium text-slate-500">
          {t("language.selectLanguage")}
        </Label>
        <div className="grid grid-cols-2 gap-3">
          {supportedLocales.map((loc) => (
            <button
              key={loc.code}
              type="button"
              onClick={() => handleLocaleChange(loc.code)}
              className={`flex items-center gap-2.5 rounded-xl border p-3.5 transition-all ${
                config.locale === loc.code
                  ? "border-slate-900 bg-slate-900/[0.03] shadow-[0_0_0_1px_rgba(15,23,42,0.08)]"
                  : "border-slate-200/70 bg-white hover:border-slate-300"
              }`}
            >
              <Globe className="h-4 w-4 text-slate-500" />
              <span className="text-sm font-medium text-slate-700">{loc.name}</span>
            </button>
          ))}
        </div>
      </div>

      {/* Timezone selection */}
      <div>
        <Label className="mb-1.5 block text-[13px] font-medium text-slate-500">
          {t("language.selectTimezone")}
        </Label>
        <div className="relative mb-2">
          <Search className="absolute left-3 top-1/2 h-3.5 w-3.5 -translate-y-1/2 text-slate-400" />
          <Input
            placeholder={t("language.timezoneSearch")}
            value={tzSearch}
            onChange={(e) => setTzSearch(e.target.value)}
            className="auth-input pl-8"
          />
        </div>
        <div className="max-h-48 overflow-y-auto rounded-xl border border-slate-200/60 bg-white/60">
          {filteredTimezones.map((tz) => (
            <button
              key={tz}
              type="button"
              onClick={() => onChange({ ...config, timezone: tz })}
              className={`w-full px-3.5 py-2 text-left text-[13px] transition-colors ${
                config.timezone === tz
                  ? "bg-slate-900/[0.05] font-medium text-slate-900"
                  : "text-slate-600 hover:bg-slate-50"
              }`}
            >
              {tz}
            </button>
          ))}
        </div>
      </div>

      <Button
        className="h-[2.625rem] w-full rounded-xl border-0 bg-slate-900 text-sm font-medium tracking-[-0.01em] text-white shadow-[0_1px_2px_rgba(0,0,0,0.05),inset_0_1px_0_rgba(255,255,255,0.06)] hover:bg-slate-800 active:scale-[0.985]"
        onClick={onNext}
      >
        {t("language.next")}
      </Button>
    </div>
  )
}

// ─── Database Step ───────────────────────────────────────────────────────────

function DatabaseStep({
  config,
  onChange,
  onNext,
  onBack,
}: {
  config: DBConfig
  onChange: (c: DBConfig) => void
  onNext: () => void
  onBack: () => void
}) {
  const { t } = useTranslation("install")
  const [testing, setTesting] = useState(false)
  const [testResult, setTestResult] = useState<{ success: boolean; error?: string } | null>(null)

  const isPostgres = config.driver === "postgres"

  const handleTestConnection = useCallback(async () => {
    setTesting(true)
    setTestResult(null)
    try {
      const data = await apiPost<{ success: boolean; error?: string }>("/api/v1/install/check-db", {
        driver: "postgres",
        host: config.host,
        port: parseInt(config.port, 10) || 5432,
        user: config.user,
        password: config.password,
        dbname: config.dbname,
      })
      setTestResult(data)
    } catch (err) {
      setTestResult({ success: false, error: err instanceof Error ? err.message : "Connection failed" })
    } finally {
      setTesting(false)
    }
  }, [config])

  const canProceed = !isPostgres || (config.host && config.user && config.dbname)

  return (
    <div className="space-y-5">
      <div className="text-center">
        <h2 className="text-lg font-semibold tracking-[-0.02em] text-slate-900">
          {t("database.title")}
        </h2>
        <p className="mt-1 text-[13px] text-slate-400">
          {t("database.description")}
        </p>
      </div>

      {/* Driver toggle */}
      <div className="grid grid-cols-2 gap-3">
        <button
          type="button"
          onClick={() => onChange({ ...config, driver: "sqlite" })}
          className={`flex flex-col items-center gap-2 rounded-xl border p-4 transition-all ${
            !isPostgres
              ? "border-slate-900 bg-slate-900/[0.03] shadow-[0_0_0_1px_rgba(15,23,42,0.08)]"
              : "border-slate-200/70 bg-white hover:border-slate-300"
          }`}
        >
          <svg className="h-7 w-7 text-slate-600" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
            <ellipse cx="12" cy="5" rx="9" ry="3" />
            <path d="M3 5V19A9 3 0 0 0 21 19V5" />
            <path d="M3 12A9 3 0 0 0 21 12" />
          </svg>
          <div>
            <div className="text-sm font-medium text-slate-700">{t("database.sqlite")}</div>
            <div className="text-[11px] text-slate-400">{t("database.sqliteDesc")}</div>
          </div>
        </button>
        <button
          type="button"
          onClick={() => onChange({ ...config, driver: "postgres" })}
          className={`flex flex-col items-center gap-2 rounded-xl border p-4 transition-all ${
            isPostgres
              ? "border-slate-900 bg-slate-900/[0.03] shadow-[0_0_0_1px_rgba(15,23,42,0.08)]"
              : "border-slate-200/70 bg-white hover:border-slate-300"
          }`}
        >
          <svg className="h-7 w-7 text-slate-600" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
            <ellipse cx="12" cy="5" rx="9" ry="3" />
            <path d="M3 5V19A9 3 0 0 0 21 19V5" />
            <path d="M3 12A9 3 0 0 0 21 12" />
            <path d="M12 12v7" />
          </svg>
          <div>
            <div className="text-sm font-medium text-slate-700">{t("database.postgres")}</div>
            <div className="text-[11px] text-slate-400">{t("database.postgresDesc")}</div>
          </div>
        </button>
      </div>

      {/* PostgreSQL connection form */}
      {isPostgres && (
        <div className="space-y-3 rounded-xl border border-slate-200/60 bg-white/60 p-4">
          <div className="grid grid-cols-3 gap-3">
            <div className="col-span-2">
              <Label className="mb-1.5 block text-[13px] font-medium text-slate-500">
                {t("database.host")}
              </Label>
              <Input
                placeholder="localhost"
                value={config.host}
                onChange={(e) => onChange({ ...config, host: e.target.value })}
                className="auth-input"
              />
            </div>
            <div>
              <Label className="mb-1.5 block text-[13px] font-medium text-slate-500">
                {t("database.port")}
              </Label>
              <Input
                placeholder="5432"
                value={config.port}
                onChange={(e) => onChange({ ...config, port: e.target.value })}
                className="auth-input"
              />
            </div>
          </div>
          <div className="grid grid-cols-2 gap-3">
            <div>
              <Label className="mb-1.5 block text-[13px] font-medium text-slate-500">
                {t("database.username")}
              </Label>
              <Input
                placeholder="metis"
                value={config.user}
                onChange={(e) => onChange({ ...config, user: e.target.value })}
                className="auth-input"
              />
            </div>
            <div>
              <Label className="mb-1.5 block text-[13px] font-medium text-slate-500">
                {t("database.password")}
              </Label>
              <Input
                type="password"
                placeholder="password"
                value={config.password}
                onChange={(e) => onChange({ ...config, password: e.target.value })}
                className="auth-input"
              />
            </div>
          </div>
          <div>
            <Label className="mb-1.5 block text-[13px] font-medium text-slate-500">
              {t("database.dbName")}
            </Label>
            <Input
              placeholder="metis"
              value={config.dbname}
              onChange={(e) => onChange({ ...config, dbname: e.target.value })}
              className="auth-input"
            />
          </div>

          <Button
            type="button"
            variant="outline"
            className="h-9 w-full rounded-xl border-slate-200/70 text-[13px] font-medium text-slate-600 hover:bg-slate-50"
            onClick={handleTestConnection}
            disabled={testing || !config.host || !config.user || !config.dbname}
          >
            {testing ? t("database.testing") : t("database.testConnection")}
          </Button>

          {testResult && (
            <div
              className={`rounded-xl px-3.5 py-2.5 text-[13px] leading-snug ${
                testResult.success
                  ? "bg-emerald-50 text-emerald-700"
                  : "bg-red-50 text-red-600"
              }`}
            >
              {testResult.success ? t("database.connectionSuccess") : testResult.error || t("database.connectionFailed")}
            </div>
          )}
        </div>
      )}

      <div className="flex gap-3">
        <Button
          type="button"
          variant="outline"
          className="h-[2.625rem] flex-1 rounded-xl border-slate-200/70 text-sm font-medium text-slate-600 hover:bg-slate-50"
          onClick={onBack}
        >
          {t("site.prev")}
        </Button>
        <Button
          className="h-[2.625rem] flex-[2] rounded-xl border-0 bg-slate-900 text-sm font-medium tracking-[-0.01em] text-white shadow-[0_1px_2px_rgba(0,0,0,0.05),inset_0_1px_0_rgba(255,255,255,0.06)] hover:bg-slate-800 active:scale-[0.985]"
          onClick={onNext}
          disabled={!canProceed}
        >
          {t("database.next")}
        </Button>
      </div>
    </div>
  )
}

// ─── Site Info Step ──────────────────────────────────────────────────────────

function SiteInfoStep({
  config,
  onChange,
  otelConfig,
  onOTelChange,
  onNext,
  onBack,
}: {
  config: SiteConfig
  onChange: (c: SiteConfig) => void
  otelConfig: OTelConfig
  onOTelChange: (c: OTelConfig) => void
  onNext: () => void
  onBack: () => void
}) {
  const { t } = useTranslation("install")
  const [showAdvanced, setShowAdvanced] = useState(false)

  return (
    <div className="space-y-5">
      <div className="text-center">
        <h2 className="text-lg font-semibold tracking-[-0.02em] text-slate-900">
          {t("site.title")}
        </h2>
        <p className="mt-1 text-[13px] text-slate-400">
          {t("site.description")}
        </p>
      </div>

      <div>
        <Label htmlFor="site-name" className="mb-1.5 block text-[13px] font-medium text-slate-500">
          {t("site.siteName")}
        </Label>
        <Input
          id="site-name"
          placeholder="Metis"
          value={config.siteName}
          onChange={(e) => onChange({ ...config, siteName: e.target.value })}
          autoFocus
          className="auth-input"
        />
        <p className="mt-1.5 text-[12px] text-slate-400">
          {t("site.siteNameHint")}
        </p>
      </div>

      {/* Advanced settings toggle */}
      <button
        type="button"
        onClick={() => setShowAdvanced(!showAdvanced)}
        className="flex w-full items-center gap-1.5 text-[13px] font-medium text-slate-400 transition hover:text-slate-600"
      >
        <ChevronRight
          className={`h-3.5 w-3.5 transition-transform duration-200 ${showAdvanced ? "rotate-90" : ""}`}
        />
        {t("site.advanced")}
      </button>

      {showAdvanced && (
        <div className="space-y-3 rounded-xl border border-slate-200/60 bg-white/60 p-4">
          <div className="mb-2 text-[12px] font-medium text-slate-400 uppercase tracking-wide">
            {t("site.otel")}
          </div>

          {/* OTel enabled toggle */}
          <div className="flex items-center justify-between">
            <div>
              <div className="text-[13px] font-medium text-slate-600">{t("site.enableTracing")}</div>
              <div className="text-[11px] text-slate-400">{t("site.enableTracingDesc")}</div>
            </div>
            <Switch
              checked={otelConfig.enabled}
              onCheckedChange={(checked) => onOTelChange({ ...otelConfig, enabled: checked })}
            />
          </div>

          {otelConfig.enabled && (
            <div className="space-y-3 pt-1">
              <div>
                <Label className="mb-1.5 block text-[13px] font-medium text-slate-500">
                  {t("site.exportEndpoint")}
                </Label>
                <Input
                  placeholder="http://localhost:4318"
                  value={otelConfig.exporterEndpoint}
                  onChange={(e) => onOTelChange({ ...otelConfig, exporterEndpoint: e.target.value })}
                  className="auth-input"
                />
              </div>
              <div className="grid grid-cols-2 gap-3">
                <div>
                  <Label className="mb-1.5 block text-[13px] font-medium text-slate-500">
                    {t("site.serviceName")}
                  </Label>
                  <Input
                    placeholder="metis"
                    value={otelConfig.serviceName}
                    onChange={(e) => onOTelChange({ ...otelConfig, serviceName: e.target.value })}
                    className="auth-input"
                  />
                </div>
                <div>
                  <Label className="mb-1.5 block text-[13px] font-medium text-slate-500">
                    {t("site.sampleRate")}
                  </Label>
                  <Input
                    placeholder="1.0"
                    value={otelConfig.sampleRate}
                    onChange={(e) => onOTelChange({ ...otelConfig, sampleRate: e.target.value })}
                    className="auth-input"
                  />
                </div>
              </div>
            </div>
          )}
        </div>
      )}

      <div className="flex gap-3">
        <Button
          type="button"
          variant="outline"
          className="h-[2.625rem] flex-1 rounded-xl border-slate-200/70 text-sm font-medium text-slate-600 hover:bg-slate-50"
          onClick={onBack}
        >
          {t("site.prev")}
        </Button>
        <Button
          className="h-[2.625rem] flex-[2] rounded-xl border-0 bg-slate-900 text-sm font-medium tracking-[-0.01em] text-white shadow-[0_1px_2px_rgba(0,0,0,0.05),inset_0_1px_0_rgba(255,255,255,0.06)] hover:bg-slate-800 active:scale-[0.985]"
          onClick={onNext}
          disabled={!config.siteName.trim()}
        >
          {t("site.next")}
        </Button>
      </div>
    </div>
  )
}

// ─── Admin Account Step ──────────────────────────────────────────────────────

function AdminStep({
  config,
  onChange,
  onNext,
  onBack,
}: {
  config: AdminConfig
  onChange: (c: AdminConfig) => void
  onNext: () => void
  onBack: () => void
}) {
  const { t } = useTranslation("install")

  const errors: Record<string, string> = {}
  if (config.username && config.username.length < 3) {
    errors.username = t("admin.usernameMinLength")
  }
  if (config.password && config.password.length < 8) {
    errors.password = t("admin.passwordMinLength")
  }
  if (config.confirmPassword && config.password !== config.confirmPassword) {
    errors.confirmPassword = t("admin.passwordMismatch")
  }
  if (config.email && !/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(config.email)) {
    errors.email = t("admin.emailInvalid")
  }

  const isValid =
    config.username.length >= 3 &&
    config.password.length >= 8 &&
    config.password === config.confirmPassword &&
    /^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(config.email)

  return (
    <div className="space-y-5">
      <div className="text-center">
        <h2 className="text-lg font-semibold tracking-[-0.02em] text-slate-900">
          {t("admin.title")}
        </h2>
        <p className="mt-1 text-[13px] text-slate-400">
          {t("admin.description")}
        </p>
      </div>

      <div className="space-y-3">
        <div>
          <Label htmlFor="admin-username" className="mb-1.5 block text-[13px] font-medium text-slate-500">
            {t("admin.username")}
          </Label>
          <Input
            id="admin-username"
            placeholder="admin"
            value={config.username}
            onChange={(e) => onChange({ ...config, username: e.target.value })}
            autoFocus
            className="auth-input"
          />
          {errors.username && (
            <p className="mt-1 text-[12px] text-red-500">{errors.username}</p>
          )}
        </div>

        <div>
          <Label htmlFor="admin-email" className="mb-1.5 block text-[13px] font-medium text-slate-500">
            {t("admin.email")}
          </Label>
          <Input
            id="admin-email"
            type="email"
            placeholder="admin@example.com"
            value={config.email}
            onChange={(e) => onChange({ ...config, email: e.target.value })}
            className="auth-input"
          />
          {errors.email && (
            <p className="mt-1 text-[12px] text-red-500">{errors.email}</p>
          )}
        </div>

        <div>
          <Label htmlFor="admin-password" className="mb-1.5 block text-[13px] font-medium text-slate-500">
            {t("admin.password")}
          </Label>
          <Input
            id="admin-password"
            type="password"
            placeholder={t("admin.passwordHint")}
            value={config.password}
            onChange={(e) => onChange({ ...config, password: e.target.value })}
            className="auth-input"
          />
          {errors.password && (
            <p className="mt-1 text-[12px] text-red-500">{errors.password}</p>
          )}
        </div>

        <div>
          <Label htmlFor="admin-confirm" className="mb-1.5 block text-[13px] font-medium text-slate-500">
            {t("admin.confirmPassword")}
          </Label>
          <Input
            id="admin-confirm"
            type="password"
            placeholder={t("admin.confirmPasswordHint")}
            value={config.confirmPassword}
            onChange={(e) => onChange({ ...config, confirmPassword: e.target.value })}
            className="auth-input"
          />
          {errors.confirmPassword && (
            <p className="mt-1 text-[12px] text-red-500">{errors.confirmPassword}</p>
          )}
        </div>
      </div>

      <div className="flex gap-3">
        <Button
          type="button"
          variant="outline"
          className="h-[2.625rem] flex-1 rounded-xl border-slate-200/70 text-sm font-medium text-slate-600 hover:bg-slate-50"
          onClick={onBack}
        >
          {t("admin.prev")}
        </Button>
        <Button
          className="h-[2.625rem] flex-[2] rounded-xl border-0 bg-slate-900 text-sm font-medium tracking-[-0.01em] text-white shadow-[0_1px_2px_rgba(0,0,0,0.05),inset_0_1px_0_rgba(255,255,255,0.06)] hover:bg-slate-800 active:scale-[0.985]"
          onClick={onNext}
          disabled={!isValid}
        >
          {t("admin.next")}
        </Button>
      </div>
    </div>
  )
}

// ─── Completion Step ─────────────────────────────────────────────────────────

function CompleteStep({
  localeConfig,
  dbConfig,
  siteConfig,
  adminConfig,
  otelConfig,
  onBack,
}: {
  localeConfig: LocaleConfig
  dbConfig: DBConfig
  siteConfig: SiteConfig
  adminConfig: AdminConfig
  otelConfig: OTelConfig
  onBack: () => void
}) {
  const { t } = useTranslation("install")
  const [installing, setInstalling] = useState(false)
  const [done, setDone] = useState(false)
  const [error, setError] = useState("")

  const localeName = supportedLocales.find((l) => l.code === localeConfig.locale)?.name || localeConfig.locale

  const handleInstall = useCallback(async () => {
    setInstalling(true)
    setError("")
    try {
      await apiPost("/api/v1/install/execute", {
        db_driver: dbConfig.driver,
        db_host: dbConfig.driver === "postgres" ? dbConfig.host : undefined,
        db_port: dbConfig.driver === "postgres" ? parseInt(dbConfig.port, 10) || 5432 : undefined,
        db_user: dbConfig.driver === "postgres" ? dbConfig.user : undefined,
        db_password: dbConfig.driver === "postgres" ? dbConfig.password : undefined,
        db_name: dbConfig.driver === "postgres" ? dbConfig.dbname : undefined,
        site_name: siteConfig.siteName,
        locale: localeConfig.locale,
        timezone: localeConfig.timezone,
        admin_username: adminConfig.username,
        admin_password: adminConfig.password,
        admin_email: adminConfig.email,
        otel_enabled: otelConfig.enabled,
        otel_exporter_endpoint: otelConfig.exporterEndpoint,
        otel_service_name: otelConfig.serviceName,
        otel_sample_rate: otelConfig.sampleRate,
      })
      setDone(true)
    } catch (err) {
      setError(err instanceof Error ? err.message : t("confirm.installFailed"))
    } finally {
      setInstalling(false)
    }
  }, [dbConfig, siteConfig, adminConfig, otelConfig, localeConfig, t])

  if (done) {
    return (
      <div className="space-y-5 text-center">
        <div className="mx-auto flex h-14 w-14 items-center justify-center rounded-full bg-emerald-50">
          <svg className="h-7 w-7 text-emerald-600" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
            <polyline points="20 6 9 17 4 12" />
          </svg>
        </div>
        <div>
          <h2 className="text-lg font-semibold tracking-[-0.02em] text-slate-900">
            {t("done.title")}
          </h2>
          <p className="mt-1 text-[13px] text-slate-400">
            {t("done.description", { name: siteConfig.siteName })}
          </p>
        </div>
        <Button
          className="h-[2.625rem] w-full rounded-xl border-0 bg-slate-900 text-sm font-medium tracking-[-0.01em] text-white shadow-[0_1px_2px_rgba(0,0,0,0.05),inset_0_1px_0_rgba(255,255,255,0.06)] hover:bg-slate-800 active:scale-[0.985]"
          onClick={() => { window.location.href = "/login" }}
        >
          {t("done.enter")}
        </Button>
      </div>
    )
  }

  return (
    <div className="space-y-5">
      <div className="text-center">
        <h2 className="text-lg font-semibold tracking-[-0.02em] text-slate-900">
          {t("confirm.title")}
        </h2>
        <p className="mt-1 text-[13px] text-slate-400">
          {t("confirm.description")}
        </p>
      </div>

      {/* Summary */}
      <div className="space-y-3 rounded-xl border border-slate-200/60 bg-white/60 p-4 text-[13px]">
        <div className="flex justify-between">
          <span className="text-slate-400">{t("confirm.language")}</span>
          <span className="font-medium text-slate-700">{localeName}</span>
        </div>
        <div className="border-t border-slate-100" />
        <div className="flex justify-between">
          <span className="text-slate-400">{t("confirm.timezone")}</span>
          <span className="font-medium text-slate-700">{localeConfig.timezone}</span>
        </div>
        <div className="border-t border-slate-100" />
        <div className="flex justify-between">
          <span className="text-slate-400">{t("confirm.database")}</span>
          <span className="font-medium text-slate-700">
            {dbConfig.driver === "sqlite" ? "SQLite" : `PostgreSQL (${dbConfig.host}:${dbConfig.port || "5432"})`}
          </span>
        </div>
        <div className="border-t border-slate-100" />
        <div className="flex justify-between">
          <span className="text-slate-400">{t("confirm.siteName")}</span>
          <span className="font-medium text-slate-700">{siteConfig.siteName}</span>
        </div>
        <div className="border-t border-slate-100" />
        <div className="flex justify-between">
          <span className="text-slate-400">{t("confirm.administrator")}</span>
          <span className="font-medium text-slate-700">{adminConfig.username}</span>
        </div>
        <div className="border-t border-slate-100" />
        <div className="flex justify-between">
          <span className="text-slate-400">{t("confirm.email")}</span>
          <span className="font-medium text-slate-700">{adminConfig.email}</span>
        </div>
        {otelConfig.enabled && (
          <>
            <div className="border-t border-slate-100" />
            <div className="flex justify-between">
              <span className="text-slate-400">OpenTelemetry</span>
              <span className="font-medium text-slate-700">{otelConfig.exporterEndpoint}</span>
            </div>
          </>
        )}
      </div>

      {error && (
        <div className="rounded-xl bg-red-50 px-3.5 py-2.5 text-[13px] leading-snug text-red-600">
          {error}
        </div>
      )}

      <div className="flex gap-3">
        <Button
          type="button"
          variant="outline"
          className="h-[2.625rem] flex-1 rounded-xl border-slate-200/70 text-sm font-medium text-slate-600 hover:bg-slate-50"
          onClick={onBack}
          disabled={installing}
        >
          {t("confirm.prev")}
        </Button>
        <Button
          className="h-[2.625rem] flex-[2] rounded-xl border-0 bg-slate-900 text-sm font-medium tracking-[-0.01em] text-white shadow-[0_1px_2px_rgba(0,0,0,0.05),inset_0_1px_0_rgba(255,255,255,0.06)] hover:bg-slate-800 active:scale-[0.985]"
          onClick={handleInstall}
          disabled={installing}
        >
          {installing ? (
            <span className="flex items-center gap-2">
              <svg className="h-4 w-4 animate-spin" viewBox="0 0 24 24" fill="none">
                <circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4" />
                <path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z" />
              </svg>
              {t("confirm.installing")}
            </span>
          ) : (
            t("confirm.install")
          )}
        </Button>
      </div>
    </div>
  )
}

// ─── Main Install Page ───────────────────────────────────────────────────────

export default function InstallPage() {
  const { t } = useTranslation("install")

  const steps: StepDef[] = [
    { id: "language", label: t("steps.language") },
    { id: "db", label: t("steps.database") },
    { id: "site", label: t("steps.site") },
    { id: "admin", label: t("steps.admin") },
    { id: "complete", label: t("steps.complete") },
  ]

  const [step, setStep] = useState(0)
  const [localeConfig, setLocaleConfig] = useState<LocaleConfig>({
    locale: supportedLocales.some((l) => l.code === navigator.language) ? navigator.language : "zh-CN",
    timezone: getBrowserTimezone(),
  })
  const [dbConfig, setDBConfig] = useState<DBConfig>({
    driver: "sqlite",
    host: "localhost",
    port: "5432",
    user: "",
    password: "",
    dbname: "metis",
  })
  const [siteConfig, setSiteConfig] = useState<SiteConfig>({ siteName: "Metis" })
  const [adminConfig, setAdminConfig] = useState<AdminConfig>({
    username: "",
    password: "",
    confirmPassword: "",
    email: "",
  })
  const [otelConfig, setOTelConfig] = useState<OTelConfig>({
    enabled: false,
    exporterEndpoint: "http://localhost:4318",
    serviceName: "metis",
    sampleRate: "1.0",
  })

  // Step order: 0=language, 1=db, 2=site, 3=admin, 4=complete
  const stepContent = (() => {
    switch (step) {
      case 0:
        return (
          <LanguageStep
            config={localeConfig}
            onChange={setLocaleConfig}
            onNext={() => setStep(1)}
          />
        )
      case 1:
        return (
          <DatabaseStep
            config={dbConfig}
            onChange={setDBConfig}
            onNext={() => setStep(2)}
            onBack={() => setStep(0)}
          />
        )
      case 2:
        return (
          <SiteInfoStep
            config={siteConfig}
            onChange={setSiteConfig}
            otelConfig={otelConfig}
            onOTelChange={setOTelConfig}
            onNext={() => setStep(3)}
            onBack={() => setStep(1)}
          />
        )
      case 3:
        return (
          <AdminStep
            config={adminConfig}
            onChange={setAdminConfig}
            onNext={() => setStep(4)}
            onBack={() => setStep(2)}
          />
        )
      case 4:
        return (
          <CompleteStep
            localeConfig={localeConfig}
            dbConfig={dbConfig}
            siteConfig={siteConfig}
            adminConfig={adminConfig}
            otelConfig={otelConfig}
            onBack={() => setStep(3)}
          />
        )
      default:
        return null
    }
  })()

  return (
    <AuthShell>
      <div className="w-full max-w-[28rem]">
        <div className="auth-panel-glass rounded-3xl px-8 py-8 sm:px-10 sm:py-9">
          <div className="mb-7">
            <StepIndicator steps={steps} current={step} />
          </div>
          {stepContent}
        </div>
        <p className="mt-4 text-center text-[11px] text-slate-300">
          {t("poweredBy")}
        </p>
      </div>
    </AuthShell>
  )
}

export function Component() {
  return <InstallPage />
}
