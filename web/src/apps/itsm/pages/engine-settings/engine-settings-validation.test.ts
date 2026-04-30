import { describe, expect, test } from "bun:test"

import { validateEngineSettingsRuntime } from "./engine-settings-validation"

describe("engine settings validation", () => {
  const validRuntime = {
    pathBuilder: { modelId: 1 },
    titleBuilder: { modelId: 2 },
    healthChecker: { modelId: 3 },
  }

  test("rejects provider changes that leave any runtime model empty", () => {
    expect(validateEngineSettingsRuntime({ ...validRuntime, pathBuilder: { modelId: 0 } })).toEqual({
      valid: false,
      errors: { pathBuilder: "参考路径生成必须选择模型" },
    })

    expect(validateEngineSettingsRuntime({ ...validRuntime, titleBuilder: { modelId: 0 } })).toEqual({
      valid: false,
      errors: { titleBuilder: "会话标题生成必须选择模型" },
    })

    expect(validateEngineSettingsRuntime({ ...validRuntime, healthChecker: { modelId: 0 } })).toEqual({
      valid: false,
      errors: { healthChecker: "发布健康检查必须选择模型" },
    })
  })

  test("accepts engine settings only when all runtime models are selected", () => {
    expect(validateEngineSettingsRuntime(validRuntime)).toEqual({ valid: true, errors: {} })
  })
})
