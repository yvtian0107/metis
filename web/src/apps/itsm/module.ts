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
        "itsm:ticket:mine",
        "itsm:ticket:approval:pending",
        "itsm:ticket:approval:history",
      ],
    },
    {
      label: "serviceManagement",
      items: [
        "itsm:service:list",
        "itsm:priority:list",
        "itsm:sla:list",
        "itsm:ticket:list",
      ],
    },
    { label: "systemConfig", items: ["itsm:smart-staffing:config", "itsm:engine-settings:config"] },
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
      path: "itsm/tickets/mine",
      children: [
        {
          index: true,
          lazy: () => import("./pages/tickets/mine/index"),
        },
      ],
    },
    {
      path: "itsm/tickets/approvals/pending",
      children: [
        {
          index: true,
          lazy: () => import("./pages/tickets/approvals/pending"),
        },
      ],
    },
    {
      path: "itsm/tickets/approvals/history",
      children: [
        {
          index: true,
          lazy: () => import("./pages/tickets/approvals/history"),
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
      path: "itsm/smart-staffing",
      children: [
        {
          index: true,
          lazy: () => import("./pages/smart-staffing/index"),
        },
      ],
    },
    {
      path: "itsm/engine-settings",
      children: [
        {
          index: true,
          lazy: () => import("./pages/engine-settings/index"),
        },
      ],
    },
  ],
})
