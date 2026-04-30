type RuntimeSectionKey = "pathBuilder" | "titleBuilder" | "healthChecker"

type RuntimeModelSelection = Record<RuntimeSectionKey, { modelId: number }>

const REQUIRED_MODEL_MESSAGES: Record<RuntimeSectionKey, string> = {
  pathBuilder: "参考路径生成必须选择模型",
  titleBuilder: "会话标题生成必须选择模型",
  healthChecker: "发布健康检查必须选择模型",
}

export function validateEngineSettingsRuntime(runtime: RuntimeModelSelection) {
  const errors: Partial<Record<RuntimeSectionKey, string>> = {}

  for (const key of Object.keys(REQUIRED_MODEL_MESSAGES) as RuntimeSectionKey[]) {
    if (runtime[key].modelId <= 0) {
      errors[key] = REQUIRED_MODEL_MESSAGES[key]
    }
  }

  return {
    valid: Object.keys(errors).length === 0,
    errors,
  }
}
