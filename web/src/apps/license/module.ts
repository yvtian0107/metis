import { registerApp } from "@/apps/registry"
import { registerTranslations } from "@/i18n"
import zhCN from "./locales/zh-CN.json"
import en from "./locales/en.json"

registerTranslations("license", { "zh-CN": zhCN, en })

registerApp({
  name: "license",
  routes: [
    {
      path: "license/products",
      children: [
        {
          index: true,
          lazy: () => import("./pages/products/index"),
        },
        {
          path: ":id",
          lazy: () => import("./pages/products/[id]"),
        },
      ],
    },
    {
      path: "license/licensees",
      children: [
        {
          index: true,
          lazy: () => import("./pages/licensees/index"),
        },
      ],
    },
    {
      path: "license/licenses",
      children: [
        {
          index: true,
          lazy: () => import("./pages/licenses/index"),
        },
        {
          path: ":id",
          lazy: () => import("./pages/licenses/[id]"),
        },
      ],
    },
  ],
})
