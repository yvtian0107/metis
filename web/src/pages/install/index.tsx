import { useState, useCallback } from "react"
import { useNavigate } from "react-router"

import { AuthShell } from "@/components/auth/auth-shell"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"

// ─── Types ───────────────────────────────────────────────────────────────────

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

interface StepDef {
  id: string
  label: string
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
                className={`h-px w-8 sm:w-12 transition-colors duration-300 ${
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

// ─── Database Step ───────────────────────────────────────────────────────────

function DatabaseStep({
  config,
  onChange,
  onNext,
}: {
  config: DBConfig
  onChange: (c: DBConfig) => void
  onNext: () => void
}) {
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
          选择数据库
        </h2>
        <p className="mt-1 text-[13px] text-slate-400">
          选择用于存储数据的数据库类型
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
            <div className="text-sm font-medium text-slate-700">SQLite</div>
            <div className="text-[11px] text-slate-400">零配置，适合小型部署</div>
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
            <div className="text-sm font-medium text-slate-700">PostgreSQL</div>
            <div className="text-[11px] text-slate-400">高性能，适合生产环境</div>
          </div>
        </button>
      </div>

      {/* PostgreSQL connection form */}
      {isPostgres && (
        <div className="space-y-3 rounded-xl border border-slate-200/60 bg-white/60 p-4">
          <div className="grid grid-cols-3 gap-3">
            <div className="col-span-2">
              <Label className="mb-1.5 block text-[13px] font-medium text-slate-500">
                主机地址
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
                端口
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
                用户名
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
                密码
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
              数据库名
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
            {testing ? "测试中..." : "测试连接"}
          </Button>

          {testResult && (
            <div
              className={`rounded-xl px-3.5 py-2.5 text-[13px] leading-snug ${
                testResult.success
                  ? "bg-emerald-50 text-emerald-700"
                  : "bg-red-50 text-red-600"
              }`}
            >
              {testResult.success ? "连接成功" : testResult.error || "连接失败"}
            </div>
          )}
        </div>
      )}

      <Button
        className="h-[2.625rem] w-full rounded-xl border-0 bg-slate-900 text-sm font-medium tracking-[-0.01em] text-white shadow-[0_1px_2px_rgba(0,0,0,0.05),inset_0_1px_0_rgba(255,255,255,0.06)] hover:bg-slate-800 active:scale-[0.985]"
        onClick={onNext}
        disabled={!canProceed}
      >
        下一步
      </Button>
    </div>
  )
}

// ─── Site Info Step ──────────────────────────────────────────────────────────

function SiteInfoStep({
  config,
  onChange,
  onNext,
  onBack,
}: {
  config: SiteConfig
  onChange: (c: SiteConfig) => void
  onNext: () => void
  onBack: () => void
}) {
  return (
    <div className="space-y-5">
      <div className="text-center">
        <h2 className="text-lg font-semibold tracking-[-0.02em] text-slate-900">
          站点信息
        </h2>
        <p className="mt-1 text-[13px] text-slate-400">
          为你的站点起一个名字
        </p>
      </div>

      <div>
        <Label htmlFor="site-name" className="mb-1.5 block text-[13px] font-medium text-slate-500">
          站点名称
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
          显示在登录页和浏览器标题中
        </p>
      </div>

      <div className="flex gap-3">
        <Button
          type="button"
          variant="outline"
          className="h-[2.625rem] flex-1 rounded-xl border-slate-200/70 text-sm font-medium text-slate-600 hover:bg-slate-50"
          onClick={onBack}
        >
          上一步
        </Button>
        <Button
          className="h-[2.625rem] flex-[2] rounded-xl border-0 bg-slate-900 text-sm font-medium tracking-[-0.01em] text-white shadow-[0_1px_2px_rgba(0,0,0,0.05),inset_0_1px_0_rgba(255,255,255,0.06)] hover:bg-slate-800 active:scale-[0.985]"
          onClick={onNext}
          disabled={!config.siteName.trim()}
        >
          下一步
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
  const errors: Record<string, string> = {}
  if (config.username && config.username.length < 3) {
    errors.username = "用户名至少 3 个字符"
  }
  if (config.password && config.password.length < 8) {
    errors.password = "密码至少 8 个字符"
  }
  if (config.confirmPassword && config.password !== config.confirmPassword) {
    errors.confirmPassword = "两次密码不一致"
  }
  if (config.email && !/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(config.email)) {
    errors.email = "请输入有效的邮箱地址"
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
          管理员账号
        </h2>
        <p className="mt-1 text-[13px] text-slate-400">
          创建超级管理员账号
        </p>
      </div>

      <div className="space-y-3">
        <div>
          <Label htmlFor="admin-username" className="mb-1.5 block text-[13px] font-medium text-slate-500">
            用户名
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
            邮箱
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
            密码
          </Label>
          <Input
            id="admin-password"
            type="password"
            placeholder="至少 8 个字符"
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
            确认密码
          </Label>
          <Input
            id="admin-confirm"
            type="password"
            placeholder="再次输入密码"
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
          上一步
        </Button>
        <Button
          className="h-[2.625rem] flex-[2] rounded-xl border-0 bg-slate-900 text-sm font-medium tracking-[-0.01em] text-white shadow-[0_1px_2px_rgba(0,0,0,0.05),inset_0_1px_0_rgba(255,255,255,0.06)] hover:bg-slate-800 active:scale-[0.985]"
          onClick={onNext}
          disabled={!isValid}
        >
          下一步
        </Button>
      </div>
    </div>
  )
}

// ─── Completion Step ─────────────────────────────────────────────────────────

function CompleteStep({
  dbConfig,
  siteConfig,
  adminConfig,
  onBack,
}: {
  dbConfig: DBConfig
  siteConfig: SiteConfig
  adminConfig: AdminConfig
  onBack: () => void
}) {
  const navigate = useNavigate()
  const [installing, setInstalling] = useState(false)
  const [done, setDone] = useState(false)
  const [error, setError] = useState("")

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
        admin_username: adminConfig.username,
        admin_password: adminConfig.password,
        admin_email: adminConfig.email,
      })
      setDone(true)
    } catch (err) {
      setError(err instanceof Error ? err.message : "安装失败")
    } finally {
      setInstalling(false)
    }
  }, [dbConfig, siteConfig, adminConfig])

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
            安装完成
          </h2>
          <p className="mt-1 text-[13px] text-slate-400">
            {siteConfig.siteName} 已准备就绪
          </p>
        </div>
        <Button
          className="h-[2.625rem] w-full rounded-xl border-0 bg-slate-900 text-sm font-medium tracking-[-0.01em] text-white shadow-[0_1px_2px_rgba(0,0,0,0.05),inset_0_1px_0_rgba(255,255,255,0.06)] hover:bg-slate-800 active:scale-[0.985]"
          onClick={() => navigate("/login", { replace: true })}
        >
          进入系统
        </Button>
      </div>
    )
  }

  return (
    <div className="space-y-5">
      <div className="text-center">
        <h2 className="text-lg font-semibold tracking-[-0.02em] text-slate-900">
          确认安装
        </h2>
        <p className="mt-1 text-[13px] text-slate-400">
          请确认以下配置信息
        </p>
      </div>

      {/* Summary */}
      <div className="space-y-3 rounded-xl border border-slate-200/60 bg-white/60 p-4 text-[13px]">
        <div className="flex justify-between">
          <span className="text-slate-400">数据库</span>
          <span className="font-medium text-slate-700">
            {dbConfig.driver === "sqlite" ? "SQLite" : `PostgreSQL (${dbConfig.host}:${dbConfig.port || "5432"})`}
          </span>
        </div>
        <div className="border-t border-slate-100" />
        <div className="flex justify-between">
          <span className="text-slate-400">站点名称</span>
          <span className="font-medium text-slate-700">{siteConfig.siteName}</span>
        </div>
        <div className="border-t border-slate-100" />
        <div className="flex justify-between">
          <span className="text-slate-400">管理员</span>
          <span className="font-medium text-slate-700">{adminConfig.username}</span>
        </div>
        <div className="border-t border-slate-100" />
        <div className="flex justify-between">
          <span className="text-slate-400">邮箱</span>
          <span className="font-medium text-slate-700">{adminConfig.email}</span>
        </div>
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
          上一步
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
              安装中...
            </span>
          ) : (
            "开始安装"
          )}
        </Button>
      </div>
    </div>
  )
}

// ─── Main Install Page ───────────────────────────────────────────────────────

const STEPS_SQLITE: StepDef[] = [
  { id: "db", label: "数据库" },
  { id: "site", label: "站点信息" },
  { id: "admin", label: "管理员" },
  { id: "complete", label: "完成" },
]

const STEPS_POSTGRES: StepDef[] = STEPS_SQLITE

export default function InstallPage() {
  const [step, setStep] = useState(0)
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

  const steps = dbConfig.driver === "postgres" ? STEPS_POSTGRES : STEPS_SQLITE

  // Step order: 0=db, 1=site, 2=admin, 3=complete
  const stepContent = (() => {
    switch (step) {
      case 0:
        return (
          <DatabaseStep
            config={dbConfig}
            onChange={setDBConfig}
            onNext={() => setStep(1)}
          />
        )
      case 1:
        return (
          <SiteInfoStep
            config={siteConfig}
            onChange={setSiteConfig}
            onNext={() => setStep(2)}
            onBack={() => setStep(0)}
          />
        )
      case 2:
        return (
          <AdminStep
            config={adminConfig}
            onChange={setAdminConfig}
            onNext={() => setStep(3)}
            onBack={() => setStep(1)}
          />
        )
      case 3:
        return (
          <CompleteStep
            dbConfig={dbConfig}
            siteConfig={siteConfig}
            adminConfig={adminConfig}
            onBack={() => setStep(2)}
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
          Powered by Metis
        </p>
      </div>
    </AuthShell>
  )
}

export function Component() {
  return <InstallPage />
}
