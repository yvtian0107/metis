import { registerApp } from "@/apps/registry"
import { registerTranslations } from "@/i18n"
import zhCN from "./locales/zh-CN.json"
import en from "./locales/en.json"

registerTranslations("apm", { "zh-CN": zhCN, en })

registerApp({
  name: "apm",
  routes: [
    {
      path: "apm/traces",
      children: [
        {
          index: true,
          lazy: () => import("./pages/traces/index"),
        },
        {
          path: ":traceId",
          lazy: () => import("./pages/traces/[traceId]/index"),
        },
      ],
    },
    {
      path: "apm/services",
      children: [
        {
          index: true,
          lazy: () => import("./pages/services/index"),
        },
        {
          path: ":name",
          lazy: () => import("./pages/services/[name]/index"),
        },
      ],
    },
    {
      path: "apm/topology",
      lazy: () => import("./pages/topology/index"),
    },
  ],
})
