import { describe, expect, test } from "bun:test"
import {
  buildEdgeLabelDisplay,
  buildConditionDisplayDictFromNodes,
  conditionSummary,
  formatConditionForDisplay,
  type ConditionDisplayLocale,
  type ConditionGroup,
  type EdgeLabelLocale,
  type GatewayCondition,
} from "./types"

const zhLocale: ConditionDisplayLocale = {
  operatorLabels: {
    equals: "等于",
    not_equals: "不等于",
    contains_any: "属于",
    gt: "大于",
    lt: "小于",
    gte: "大于等于",
    lte: "小于等于",
    is_empty: "为空",
    is_not_empty: "非空",
  },
  logicLabels: { and: "且", or: "或" },
  valueSeparator: " / ",
}

const edgeLocale: EdgeLabelLocale = {
  defaultLabel: "默认路径",
  outcomeLabels: {
    submitted: "已提交",
    completed: "已完成",
    approved: "已通过",
    rejected: "已驳回",
  },
}

describe("workflow edge condition display", () => {
  test("maps field and option values from form schema for display", () => {
    const dict = buildConditionDisplayDictFromNodes([
      {
        data: {
          formSchema: {
            version: 1,
            fields: [
              {
                key: "request_kind",
                label: "访问原因",
                options: [
                  { label: "线上支持", value: "online_support" },
                  { label: "外部协作", value: "external_collaboration" },
                ],
              },
            ],
          },
        },
      },
    ])

    const cond: GatewayCondition = {
      field: "form.request_kind",
      operator: "contains_any",
      value: ["online_support", "external_collaboration"],
    }

    expect(formatConditionForDisplay(cond, dict, zhLocale)).toBe("访问原因 属于 线上支持 / 外部协作")
    expect(conditionSummary(cond)).toContain("request_kind ∈")
  })

  test("falls back to raw key/value for unknown mapping", () => {
    const cond: GatewayCondition = {
      field: "form.urgency",
      operator: "equals",
      value: "high",
    }
    const dict = buildConditionDisplayDictFromNodes([])
    expect(formatConditionForDisplay(cond, dict, zhLocale)).toBe("urgency 等于 high")
  })

  test("renders nested condition groups with localized logic labels", () => {
    const dict = buildConditionDisplayDictFromNodes([
      {
        data: {
          formSchema: {
            version: 1,
            fields: [
              { key: "request_kind", label: "访问原因", options: [{ label: "线上支持", value: "online_support" }] },
              { key: "urgency", label: "紧急程度", options: [{ label: "高", value: "high" }] },
            ],
          },
        },
      },
    ])
    const group: ConditionGroup = {
      logic: "and",
      conditions: [
        { field: "form.request_kind", operator: "contains_any", value: ["online_support"] },
        {
          logic: "or",
          conditions: [
            { field: "form.urgency", operator: "equals", value: "high" },
            { field: "form.urgency", operator: "is_empty", value: "" },
          ],
        },
      ],
    }
    expect(formatConditionForDisplay(group, dict, zhLocale)).toBe("访问原因 属于 线上支持 且 (紧急程度 等于 高 或 紧急程度 为空)")
  })

  test("maps known outcomes and keeps unknown outcomes raw", () => {
    const dict = buildConditionDisplayDictFromNodes([])
    const submitted = buildEdgeLabelDisplay({ outcome: "submitted" }, dict, zhLocale, edgeLocale)
    expect(submitted.primary).toBe("已提交")
    expect(submitted.raw).toBe("submitted")
    expect(submitted.isCompleted).toBe(true)

    const unknown = buildEdgeLabelDisplay({ outcome: "custom_outcome" }, dict, zhLocale, edgeLocale)
    expect(unknown.primary).toBe("custom_outcome")
    expect(unknown.raw).toBe("custom_outcome")
    expect(unknown.isCompleted).toBe(false)
  })
})
