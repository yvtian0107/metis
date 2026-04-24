import { test } from "@playwright/test"

import { AgenticITSMLiveWorld } from "./support/agentic-itsm-world"

test.describe("Feature: Agentic ITSM 真实开发环境登录", () => {
  test("Scenario: admin 成功登录系统", async ({ page }) => {
    const world = new AgenticITSMLiveWorld(page)

    await test.step("Given 隔离开发环境已完成 seed-dev 并启动", () =>
      world.givenIsolatedDevEnvironmentIsReady(),
    )

    await test.step("When 管理员使用 admin / password 登录", async () => {
      await world.whenAdminOpensLoginPage()
      await world.whenAdminSubmitsDevCredentials()
    })

    await test.step("Then 系统进入已登录态并展示 admin", () =>
      world.thenAdminCanSeeAuthenticatedShell(),
    )

    await test.step("And 前端没有运行时错误", () =>
      world.thenNoRuntimeErrorsAppeared(),
    )
  })
})
