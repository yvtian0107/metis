import { describe, expect, it } from "bun:test"

import { parseFieldDisplayMeta, resolveFieldDisplayValue } from "./field-display"

const t = (key: string) => key

describe("ticket field display", () => {
  it("formats datetime date_range values as readable ranges", () => {
    const meta = parseFieldDisplayMeta({
      fields: [
        {
          key: "access_window",
          type: "date_range",
          label: "访问时段",
          props: { withTime: true, mode: "datetime" },
        },
      ],
    })

    expect(resolveFieldDisplayValue("access_window", {
      start: "2026-04-29 13:00:00",
      end: "2026-04-29 15:00:00",
    }, meta, t, "zh-CN")).toBe("2026/4/29 13:00 - 2026/4/29 15:00")
  })

  it("formats multi_select values using option labels", () => {
    const meta = parseFieldDisplayMeta({
      fields: [
        {
          key: "request_kind",
          type: "multi_select",
          label: "访问原因",
          options: [
            { label: "线上支持", value: "online_support" },
            { label: "外部协作", value: "external_collaboration" },
          ],
        },
      ],
    })

    expect(resolveFieldDisplayValue("request_kind", ["online_support", "external_collaboration"], meta, t, "zh-CN"))
      .toBe("线上支持、外部协作")
  })

  it("summarizes table values instead of dumping raw json", () => {
    const meta = parseFieldDisplayMeta({
      fields: [
        {
          key: "change_items",
          type: "table",
          label: "变更明细",
        },
      ],
    })

    expect(resolveFieldDisplayValue("change_items", [
      { system: "gateway", permission_level: "read" },
      { system: "payment", permission_level: "write" },
    ], meta, t, "zh-CN")).toBe("2 条记录: system: gateway, permission_level: read")
  })

  it("falls back to start/end object formatting when schema is missing", () => {
    expect(resolveFieldDisplayValue("access_window", {
      start: "2026-04-29 13:00:00",
      end: "2026-04-29 15:00:00",
    }, {}, t, "zh-CN")).toBe("2026/4/29 13:00 - 2026/4/29 15:00")
  })
})
