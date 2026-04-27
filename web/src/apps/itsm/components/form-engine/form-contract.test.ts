import { describe, expect, test } from "bun:test"
import { buildZodSchema, defaultValueForField } from "./build-zod-schema"
import { getVisibleFields } from "./use-visibility"
import type { FormSchema } from "./types"

describe("form engine contract", () => {
  test("validates required, email, url, number, and hidden fields", () => {
    const schema: FormSchema = {
      version: 1,
      fields: [
        { key: "title", type: "text", label: "标题", required: true },
        { key: "email", type: "email", label: "邮箱", validation: [{ rule: "email", message: "邮箱错误" }] },
        { key: "site", type: "url", label: "主页", validation: [{ rule: "url", message: "URL错误" }] },
        { key: "amount", type: "number", label: "数量", validation: [{ rule: "min", value: 1, message: "数量太小" }] },
        { key: "hidden", type: "text", label: "隐藏", required: true },
      ],
    }

    const zodSchema = buildZodSchema(schema, new Set(["title", "email", "site", "amount"]))
    const invalid = zodSchema.safeParse({ title: "", email: "bad", site: "ftp://example.test", amount: 0 })

    expect(invalid.success).toBe(false)
    if (!invalid.success) {
      expect(invalid.error.issues.map((issue) => issue.message)).toEqual(expect.arrayContaining([
        "此字段为必填项",
        "邮箱错误",
        "URL错误",
        "数量太小",
      ]))
      expect(invalid.error.issues.some((issue) => issue.path[0] === "hidden")).toBe(false)
    }

    expect(zodSchema.safeParse({
      title: "VPN",
      email: "ops@example.test",
      site: "https://example.test",
      amount: 1,
    }).success).toBe(true)
  })

  test("handles multi value and boolean fields", () => {
    const schema: FormSchema = {
      version: 1,
      fields: [
        { key: "tags", type: "multi_select", label: "标签", required: true, options: [{ label: "VPN", value: "vpn" }] },
        { key: "agree", type: "checkbox", label: "同意" },
        { key: "systems", type: "checkbox", label: "系统", required: true, options: [{ label: "ERP", value: "erp" }] },
        { key: "switcher", type: "switch", label: "开关" },
      ],
    }

    const zodSchema = buildZodSchema(schema, new Set(["tags", "agree", "systems", "switcher"]))

    expect(zodSchema.safeParse({ tags: [], agree: true, systems: ["erp"], switcher: false }).success).toBe(false)
    expect(zodSchema.safeParse({ tags: ["bad"], agree: true, systems: ["erp"], switcher: false }).success).toBe(false)
    expect(zodSchema.safeParse({ tags: ["vpn"], agree: "true", systems: ["erp"], switcher: true }).success).toBe(false)
    expect(zodSchema.safeParse({ tags: ["vpn"], agree: false, systems: "erp", switcher: true }).success).toBe(false)
    expect(zodSchema.safeParse({ tags: ["vpn"], agree: false, systems: ["erp"], switcher: true }).success).toBe(true)
  })

  test("handles date range and table fields", () => {
    const schema: FormSchema = {
      version: 1,
      fields: [
        { key: "range", type: "date_range", label: "日期范围", required: true },
        {
          key: "items",
          type: "table",
          label: "明细",
          required: true,
          props: {
            columns: [
              { key: "name", type: "text", label: "名称", required: true },
              { key: "kind", type: "select", label: "类型", required: true, options: [{ label: "网络", value: "network" }] },
            ],
          },
        },
      ],
    }

    const zodSchema = buildZodSchema(schema, new Set(["range", "items"]))
    expect(zodSchema.safeParse({ range: {}, items: [] }).success).toBe(false)
    expect(zodSchema.safeParse({ range: { start: "2026-01-01", end: "" }, items: [{ name: "A", kind: "network" }] }).success).toBe(false)
    expect(zodSchema.safeParse({ range: { start: "2026-01-01", end: "2026-01-02" }, items: [{ kind: "network" }] }).success).toBe(false)
    expect(zodSchema.safeParse({ range: { start: "2026-01-01", end: "2026-01-02" }, items: [{ name: "A", kind: "bad" }] }).success).toBe(false)
    expect(zodSchema.safeParse({ range: { start: "2026-01-01", end: "2026-01-02" }, items: [{ name: "A", kind: "network" }] }).success).toBe(true)
  })

  test("builds stable default values by field type", () => {
    expect(defaultValueForField({ key: "tags", type: "multi_select", label: "标签" })).toEqual([])
    expect(defaultValueForField({ key: "range", type: "date_range", label: "日期范围" })).toEqual({ start: "", end: "" })
    expect(defaultValueForField({ key: "items", type: "table", label: "明细" })).toEqual([])
    expect(defaultValueForField({ key: "agree", type: "checkbox", label: "同意" })).toBe(false)
    expect(defaultValueForField({ key: "count", type: "number", label: "数量" })).toBeUndefined()
  })

  test("computes visibility with and/or logic", () => {
    const schema: FormSchema = {
      version: 1,
      fields: [
        { key: "kind", type: "select", label: "类型" },
        {
          key: "vpn_account",
          type: "text",
          label: "VPN账号",
          visibility: { conditions: [{ field: "kind", operator: "equals", value: "vpn" }] },
        },
        {
          key: "review_reason",
          type: "textarea",
          label: "复核原因",
          visibility: {
            logic: "or",
            conditions: [
              { field: "kind", operator: "equals", value: "security" },
              { field: "kind", operator: "equals", value: "audit" },
            ],
          },
        },
      ],
    }

    expect([...getVisibleFields(schema, { kind: "vpn" })].sort()).toEqual(["kind", "vpn_account"])
    expect([...getVisibleFields(schema, { kind: "audit" })].sort()).toEqual(["kind", "review_reason"])
    expect([...getVisibleFields(schema, { kind: "other" })].sort()).toEqual(["kind"])
  })
})
