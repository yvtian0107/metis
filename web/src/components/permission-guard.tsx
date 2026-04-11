import { Outlet } from "react-router"
import { useTranslation } from "react-i18next"
import { useMenuStore } from "@/stores/menu"

interface PermissionGuardProps {
  permission: string
  children?: React.ReactNode
}

export function PermissionGuard({ permission, children }: PermissionGuardProps) {
  const { t } = useTranslation()
  const hasPermission = useMenuStore((s) => s.permissions.includes(permission))

  if (!hasPermission) {
    return (
      <div className="flex h-64 items-center justify-center text-muted-foreground">
        {t("noPermission")}
      </div>
    )
  }

  return children ?? <Outlet />
}
