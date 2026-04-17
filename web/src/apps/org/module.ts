import { registerApp } from "@/apps/registry"
import { registerTranslations } from "@/i18n"
import zhCN from "./locales/zh-CN.json"
import en from "./locales/en.json"

registerTranslations("org", { "zh-CN": zhCN, en })

registerApp({
  name: "org",
  routes: [
    {
      path: "org/departments",
      children: [
        {
          index: true,
          lazy: () => import("./pages/departments/index"),
        },
        {
          path: ":id",
          lazy: () => import("./pages/departments/[id]"),
        },
      ],
    },
    {
      path: "org/positions",
      children: [
        {
          index: true,
          lazy: () => import("./pages/positions/index"),
        },
      ],
    },
    {
      path: "org/assignments",
      lazy: async () => {
        const { redirect } = await import("react-router")
        return { loader: () => redirect("/org/departments") }
      },
    },
  ],
})
