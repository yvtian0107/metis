import type { ReactNode } from "react"

import { cn } from "@/lib/utils"
import { LanguageSwitcher } from "@/components/language-switcher"

interface AuthShellProps {
  aside?: ReactNode
  children: ReactNode
  className?: string
}

export function AuthShell({ aside, children, className }: AuthShellProps) {
  return (
    <div className="auth-shell-bg relative min-h-screen overflow-hidden">
      <div className="auth-grid pointer-events-none absolute inset-0 opacity-60" />
      <div className="auth-orb-primary pointer-events-none absolute left-[-10rem] top-[-8rem] h-80 w-80 rounded-full blur-3xl" />
      <div className="auth-orb-secondary pointer-events-none absolute bottom-[-10rem] right-[-8rem] h-96 w-96 rounded-full blur-3xl" />

      <div
        className={cn(
          "relative mx-auto grid min-h-screen w-full max-w-[1600px] grid-cols-1",
          aside && "lg:grid-cols-[minmax(0,1.12fr)_minmax(420px,520px)]",
          className
        )}
      >
        {aside ? (
          <aside className="hidden min-h-screen px-8 py-8 lg:flex lg:items-stretch lg:px-10 xl:px-14">
            {aside}
          </aside>
        ) : null}

        <main className="flex min-h-screen items-center justify-center px-4 py-6 sm:px-6 lg:px-8">
          {children}
        </main>
      </div>

      {/* Language switcher */}
      <div className="absolute bottom-4 right-4">
        <LanguageSwitcher />
      </div>
    </div>
  )
}
