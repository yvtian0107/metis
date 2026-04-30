import { z } from "zod"

interface PriorityFormMessages {
  nameRequired: string
  codeRequired: string
  valueRequired: string
  colorRequired: string
  colorInvalid: string
}

const HEX_COLOR_PATTERN = /^#[0-9a-fA-F]{6}$/

const defaultPriorityFormValues = {
  name: "",
  code: "",
  value: 1,
  color: "#5b6f8f",
  description: "",
  isActive: true,
}

function createPriorityFormSchema(messages: PriorityFormMessages) {
  return z.object({
    name: z.string().min(1, messages.nameRequired),
    code: z.string().min(1, messages.codeRequired),
    value: z.number({ error: messages.valueRequired }).int().min(1, messages.valueRequired),
    color: z.string().min(1, messages.colorRequired).regex(HEX_COLOR_PATTERN, messages.colorInvalid),
    description: z.string().optional(),
    isActive: z.boolean(),
  })
}

function parseIntegerInputValue(value: string) {
  if (value.trim() === "") return Number.NaN
  return Number(value)
}

function numberInputValue(value: number) {
  return Number.isNaN(value) ? "" : value
}

function isHexColor(value: string) {
  return HEX_COLOR_PATTERN.test(value)
}

export {
  createPriorityFormSchema,
  defaultPriorityFormValues,
  isHexColor,
  numberInputValue,
  parseIntegerInputValue,
}
export type { PriorityFormMessages }
