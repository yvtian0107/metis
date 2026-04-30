import { describe, expect, test } from "bun:test"

import { validateVisibleFormData } from "./form-submit"
import type { FormSchema } from "./types"

describe("form submit validation", () => {
  test("blocks invalid visible fields before submitting a draft", () => {
    const schema: FormSchema = {
      version: 1,
      fields: [
        { key: "title", type: "text", label: "标题", required: true },
        { key: "hidden", type: "text", label: "隐藏字段", required: true },
      ],
    }

    const result = validateVisibleFormData(schema, new Set(["title"]), { title: "", hidden: "" })

    expect(result.success).toBe(false)
    if (!result.success) {
      expect(result.errors).toEqual([{ field: "title", message: "此字段为必填项" }])
    }
  })

  test("returns only validated visible data for draft submission", () => {
    const schema: FormSchema = {
      version: 1,
      fields: [
        { key: "title", type: "text", label: "标题", required: true },
        { key: "hidden", type: "text", label: "隐藏字段", required: true },
      ],
    }

    const result = validateVisibleFormData(schema, new Set(["title"]), { title: "VPN", hidden: "" })

    expect(result).toEqual({ success: true, data: { title: "VPN" } })
  })
})
