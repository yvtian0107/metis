import { expect, type Page } from "@playwright/test"

export class AgenticITSMLiveWorld {
  private readonly runtimeErrors: string[] = []

  constructor(private readonly page: Page) {
    this.page.on("pageerror", (error) => {
      this.runtimeErrors.push(error.message)
    })
    this.page.on("console", (message) => {
      if (message.type() === "error") {
        this.runtimeErrors.push(message.text())
      }
    })
  }

  async givenIsolatedDevEnvironmentIsReady() {
    await this.page.goto("/login")
    await expect(this.page.locator("#username")).toBeVisible()
    await expect(this.page.locator("#password")).toBeVisible()
  }

  async whenAdminOpensLoginPage() {
    await expect(this.page).toHaveURL(/\/login$/)
  }

  async whenAdminSubmitsDevCredentials() {
    await this.page.locator("#username").fill("admin")
    await this.page.locator("#password").fill("password")

    const [loginResponse] = await Promise.all([
      this.page.waitForResponse((response) => {
        const url = new URL(response.url())
        return url.pathname === "/api/v1/auth/login" && response.request().method() === "POST"
      }),
      this.page.locator('button[type="submit"]').click(),
    ])

    expect(loginResponse.ok()).toBeTruthy()
    const body = await loginResponse.json()
    expect(body.code).toBe(0)
  }

  async thenAdminCanSeeAuthenticatedShell() {
    await expect(this.page).not.toHaveURL(/\/login$/)
    await expect(this.page.getByRole("button", { name: /admin/ })).toBeVisible()
  }

  async thenNoRuntimeErrorsAppeared() {
    expect(this.runtimeErrors).toEqual([])
  }
}
