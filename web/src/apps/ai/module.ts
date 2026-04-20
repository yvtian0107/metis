import { registerApp } from "@/apps/registry"
import { registerTranslations } from "@/i18n"
import zhCN from "./locales/zh-CN.json"
import en from "./locales/en.json"

registerTranslations("ai", { "zh-CN": zhCN, en })

registerApp({
  name: "ai",
  navigation: [
    {
      label: "agents",
      items: [{ permission: "ai:agent:list" }],
    },
    {
      label: "knowledge",
      items: [{ permission: "ai:knowledge:list" }],
    },
    {
      label: "tools",
      items: [
        { permission: "ai:tool:list" },
        { permission: "ai:mcp:list" },
        { permission: "ai:skill:list" },
      ],
    },
    {
      label: "modelAccess",
      items: [{ permission: "ai:provider:list" }],
    },
  ],
  routes: [
    {
      path: "ai/providers",
      children: [
        {
          index: true,
          lazy: () => import("./pages/providers/index"),
        },
        {
          path: ":id",
          lazy: () => import("./pages/providers/[id]"),
        },
      ],
    },
    {
      path: "ai/knowledge",
      children: [
        {
          index: true,
          lazy: () => import("./pages/knowledge/index"),
        },
        {
          path: ":id",
          lazy: () => import("./pages/knowledge/[id]"),
        },
      ],
    },
    {
      path: "ai/tools",
      children: [
        {
          index: true,
          lazy: async () => {
            const { Navigate } = await import("react-router")
            return { Component: () => Navigate({ to: "/ai/tools/builtin", replace: true }) }
          },
        },
        {
          path: "builtin",
          lazy: () => import("./pages/tools/builtin"),
        },
        {
          path: "mcp",
          lazy: () => import("./pages/tools/mcp"),
        },
        {
          path: "skills",
          lazy: () => import("./pages/tools/skills"),
        },
      ],
    },
    {
      path: "ai/agents",
      children: [
        {
          index: true,
          lazy: () => import("./pages/agents/index"),
        },
        {
          path: "create",
          lazy: () => import("./pages/agents/create"),
        },
        {
          path: ":id",
          lazy: () => import("./pages/agents/[id]"),
        },
        {
          path: ":id/edit",
          lazy: () => import("./pages/agents/[id]/edit"),
        },
      ],
    },
    {
      path: "ai/chat",
      children: [
        {
          path: ":sid",
          lazy: () => import("./pages/chat/[sid]"),
        },
      ],
    },
  ],
})
