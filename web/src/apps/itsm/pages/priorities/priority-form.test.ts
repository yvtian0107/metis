import { describe, expect, test } from "bun:test"
import { createPriorityFormSchema } from "./priority-form"

const schema = createPriorityFormSchema({
  nameRequired: "name required",
  codeRequired: "code required",
  valueRequired: "value required",
  colorRequired: "color required",
  colorInvalid: "color invalid",
})

const validPriority = {
  name: "紧急",
  code: "P0",
  value: 1,
  color: "#5b6f8f",
  description: "",
  isActive: true,
}

describe("priority form schema", () => {
  test("rejects value 0 because backend treats required int zero as missing", () => {
    expect(schema.safeParse({ ...validPriority, value: 0 }).success).toBe(false)
  })

  test("accepts value 1 as the first valid sort weight", () => {
    expect(schema.safeParse(validPriority).success).toBe(true)
  })

  test("requires six digit hex colors", () => {
    expect(schema.safeParse({ ...validPriority, color: "red" }).success).toBe(false)
    expect(schema.safeParse({ ...validPriority, color: "#fff" }).success).toBe(false)
    expect(schema.safeParse({ ...validPriority, color: "#5b6f8f" }).success).toBe(true)
  })
})
