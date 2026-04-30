import { describe, expect, test } from "bun:test"
import { parseServiceActionConfigInput } from "./service-action-config"

describe("parseServiceActionConfigInput", () => {
  test("parses valid JSON objects", () => {
    expect(parseServiceActionConfigInput('{"url":"https://example.test"}')).toEqual({
      ok: true,
      value: { url: "https://example.test" },
    })
  })

  test("empty input is saved as null", () => {
    expect(parseServiceActionConfigInput("   ")).toEqual({ ok: true, value: null })
  })

  test("invalid JSON returns a field error instead of nulling the config", () => {
    const result = parseServiceActionConfigInput("{bad")

    expect(result.ok).toBe(false)
    if (!result.ok) {
      expect(result.message.length).toBeGreaterThan(0)
    }
  })
})
