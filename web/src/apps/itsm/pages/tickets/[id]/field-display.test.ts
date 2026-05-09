import { describe, expect, it } from "bun:test"

import {
  parseFieldDisplayMeta,
  parseFieldDisplaySections,
  resolveFieldDisplayModel,
  resolveFieldDisplayValue,
} from "./field-display"

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

  it("builds structured table models instead of json summaries", () => {
    const meta = parseFieldDisplayMeta({
      fields: [
        {
          key: "change_items",
          type: "table",
          label: "变更明细",
          props: {
            columns: [
              { key: "system", type: "text", label: "系统" },
              { key: "permission_level", type: "select", label: "权限级别", options: [{ label: "只读", value: "read" }] },
              { key: "effective_range", type: "date_range", label: "生效时段" },
            ],
          },
        },
      ],
    })

    const model = resolveFieldDisplayModel("change_items", [
      {
        system: "gateway",
        permission_level: "read",
        effective_range: { start: "2026-05-06 22:00:00", end: "2026-05-07 02:00:00" },
      },
      {
        system: "payment",
        permission_level: "read",
        effective_range: { start: "2026-05-06 23:00:00", end: "2026-05-07 01:00:00" },
      },
    ], meta, t, "zh-CN")

    expect(model.kind).toBe("table")
    if (model.kind !== "table") throw new Error("expected table model")
    expect(model.summaryLabel).toBe("2 条记录")
    expect(model.summary).toBe("gateway / 只读 / 2026/5/6 22:00 - 2026/5/7 02:00；payment / 只读 / 2026/5/6 23:00 - 2026/5/7 01:00")
    expect(model.columns.map((column) => column.label)).toEqual(["系统", "权限级别", "生效时段"])
    expect(model.rows[0]?.cells[1]?.value).toBe("只读")
  })

  it("marks short textarea values as non-expandable", () => {
    const meta = parseFieldDisplayMeta({
      fields: [
        { key: "impact_scope", type: "textarea", label: "影响范围" },
      ],
    })

    const model = resolveFieldDisplayModel("impact_scope", "支付链路", meta, t, "zh-CN")
    expect(model.kind).toBe("long_text")
    if (model.kind !== "long_text") throw new Error("expected long_text model")
    expect(model.expandable).toBe(false)
  })

  it("marks longer textarea values as expandable", () => {
    const meta = parseFieldDisplayMeta({
      fields: [
        { key: "impact_scope", type: "textarea", label: "影响范围" },
      ],
    })

    const model = resolveFieldDisplayModel(
      "impact_scope",
      "变更窗口内只读核对核心支付服务日志、订单状态与对账结果，并同步确认支付链路的路由表现是否符合预期，同时记录异常样本与回滚前后的观测结果。",
      meta,
      t,
      "zh-CN",
    )
    expect(model.kind).toBe("long_text")
    if (model.kind !== "long_text") throw new Error("expected long_text model")
    expect(model.expandable).toBe(true)
  })

  it("parses schema sections for request payload grouping", () => {
    const sections = parseFieldDisplaySections({
      layout: {
        sections: [
          { title: "基础信息", fields: ["subject", "change_window"] },
          { title: "影响与回滚", fields: ["impact_scope"] },
        ],
      },
    })

    expect(sections).toEqual([
      { title: "基础信息", description: undefined, collapsible: false, fields: ["subject", "change_window"] },
      { title: "影响与回滚", description: undefined, collapsible: false, fields: ["impact_scope"] },
    ])
  })

  it("falls back to start/end object formatting when schema is missing", () => {
    expect(resolveFieldDisplayValue("access_window", {
      start: "2026-04-29 13:00:00",
      end: "2026-04-29 15:00:00",
    }, {}, t, "zh-CN")).toBe("2026/4/29 13:00 - 2026/4/29 15:00")
  })
})
