import { registerApp } from "@/apps/registry"
import { registerTranslations } from "@/i18n"
import zhCN from "./locales/zh-CN.json"
import en from "./locales/en.json"

registerTranslations("itsm", { "zh-CN": zhCN, en })

registerApp({
  name: "itsm",
  routes: [
    {
      path: "itsm/catalogs",
      children: [
        {
          index: true,
          lazy: () => import("./pages/catalogs/index"),
        },
      ],
    },
    {
      path: "itsm/services",
      children: [
        {
          index: true,
          lazy: () => import("./pages/services/index"),
        },
      ],
    },
    {
      path: "itsm/services/:id",
      lazy: () => import("./pages/services/[id]/index"),
    },
    {
      path: "itsm/services/:id/actions",
      lazy: () => import("./pages/services/[id]/actions"),
    },
    {
      path: "itsm/services/:id/workflow",
      lazy: () => import("./pages/services/[id]/workflow"),
    },
    {
      path: "itsm/tickets",
      children: [
        {
          index: true,
          lazy: () => import("./pages/tickets/index"),
        },
      ],
    },
    {
      path: "itsm/tickets/create",
      lazy: () => import("./pages/tickets/create/index"),
    },
    {
      path: "itsm/tickets/mine",
      children: [
        {
          index: true,
          lazy: () => import("./pages/tickets/mine/index"),
        },
      ],
    },
    {
      path: "itsm/tickets/todo",
      children: [
        {
          index: true,
          lazy: () => import("./pages/tickets/todo/index"),
        },
      ],
    },
    {
      path: "itsm/tickets/history",
      children: [
        {
          index: true,
          lazy: () => import("./pages/tickets/history/index"),
        },
      ],
    },
    {
      path: "itsm/tickets/approvals",
      children: [
        {
          index: true,
          lazy: () => import("./pages/tickets/approvals/index"),
        },
      ],
    },
    {
      path: "itsm/tickets/:id",
      lazy: () => import("./pages/tickets/[id]/index"),
    },
    {
      path: "itsm/priorities",
      children: [
        {
          index: true,
          lazy: () => import("./pages/priorities/index"),
        },
      ],
    },
    {
      path: "itsm/sla",
      children: [
        {
          index: true,
          lazy: () => import("./pages/sla/index"),
        },
      ],
    },
  ],
})
