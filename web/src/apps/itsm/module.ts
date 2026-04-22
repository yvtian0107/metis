import { registerApp } from "@/apps/registry"
import { registerTranslations } from "@/i18n"
import zhCN from "./locales/zh-CN.json"
import en from "./locales/en.json"

registerTranslations("itsm", { "zh-CN": zhCN, en })

registerApp({
  name: "itsm",
  menuGroups: [
    {
      label: "workspace",
      items: [
        "itsm:service-desk:use",
        "itsm:ticket:todo",
        "itsm:ticket:approvals",
        "itsm:ticket:mine",
        "itsm:ticket:list",
        "itsm:ticket:history",
      ],
    },
    { label: "serviceConfig", items: ["itsm:service:list"] },
    { label: "systemConfig", items: ["itsm:sla:list", "itsm:priority:list", "itsm:engine:config"] },
  ],
  routes: [
    {
      path: "itsm/service-desk",
      children: [
        {
          index: true,
          lazy: () => import("./pages/service-desk/index"),
        },
      ],
    },
    {
      path: "itsm/catalogs",
      lazy: async () => {
        const { Navigate } = await import("react-router")
        return { Component: () => Navigate({ to: "/itsm/services", replace: true }) }
      },
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
    {
      path: "itsm/engine-config",
      children: [
        {
          index: true,
          lazy: () => import("./pages/engine-config/index"),
        },
      ],
    },
  ],
})
