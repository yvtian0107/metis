#!/usr/bin/env bun
import { spawn } from "node:child_process"
import { randomBytes } from "node:crypto"
import { mkdtemp, readFile, rm, writeFile } from "node:fs/promises"
import { createServer } from "node:net"
import { tmpdir } from "node:os"
import path from "node:path"
import { fileURLToPath } from "node:url"

const scriptPath = fileURLToPath(import.meta.url)
const repoRoot = path.resolve(path.dirname(scriptPath), "..")
const webRoot = path.join(repoRoot, "web")
const bunBin = process.env.BUN || "bun"
const timeoutMs = 120_000
const children = []

function log(message) {
  console.log(`[agentic-itsm-e2e] ${message}`)
}

function secret() {
  return randomBytes(32).toString("hex")
}

async function freePort() {
  return await new Promise((resolve, reject) => {
    const server = createServer()
    server.unref()
    server.on("error", reject)
    server.listen(0, "127.0.0.1", () => {
      const address = server.address()
      server.close(() => {
        if (!address || typeof address === "string") {
          reject(new Error("failed to allocate free port"))
          return
        }
        resolve(address.port)
      })
    })
  })
}

async function portPair() {
  const apiPort = await freePort()
  let webPort = await freePort()
  while (webPort === apiPort) {
    webPort = await freePort()
  }
  return { apiPort, webPort }
}

function prefixOutput(child, name) {
  child.stdout?.on("data", (chunk) => {
    for (const line of chunk.toString().split(/\r?\n/).filter(Boolean)) {
      console.log(`[${name}] ${line}`)
    }
  })
  child.stderr?.on("data", (chunk) => {
    for (const line of chunk.toString().split(/\r?\n/).filter(Boolean)) {
      console.error(`[${name}] ${line}`)
    }
  })
}

function spawnManaged(name, command, args, options = {}) {
  const child = spawn(command, args, {
    cwd: repoRoot,
    env: process.env,
    stdio: ["ignore", "pipe", "pipe"],
    detached: process.platform !== "win32",
    ...options,
  })
  child.name = name
  prefixOutput(child, name)
  children.push(child)
  return child
}

async function run(name, command, args, options = {}) {
  log(`${name}: ${command} ${args.join(" ")}`)
  await new Promise((resolve, reject) => {
    const child = spawn(command, args, {
      cwd: repoRoot,
      env: process.env,
      stdio: "inherit",
      ...options,
    })
    child.on("error", reject)
    child.on("exit", (code, signal) => {
      if (code === 0) {
        resolve()
        return
      }
      reject(new Error(`${name} failed with ${signal ?? `exit code ${code}`}`))
    })
  })
}

function ensureRunning(child) {
  if (child.exitCode !== null) {
    throw new Error(`${child.name} exited before the environment became ready`)
  }
}

async function waitFor(url, predicate, label, processes) {
  const started = Date.now()
  let lastError = null
  while (Date.now() - started < timeoutMs) {
    for (const child of processes) ensureRunning(child)
    try {
      const response = await fetch(url)
      if (await predicate(response)) {
        log(`${label} ready: ${url}`)
        return
      }
      lastError = new Error(`${label} returned HTTP ${response.status}`)
    } catch (error) {
      lastError = error
    }
    await new Promise((resolve) => setTimeout(resolve, 500))
  }
  throw new Error(`${label} did not become ready in ${timeoutMs}ms: ${lastError?.message ?? "unknown error"}`)
}

async function waitForAPI(apiURL, processes) {
  await waitFor(
    `${apiURL}/api/v1/install/status`,
    async (response) => {
      if (!response.ok) return false
      const body = await response.json()
      return body?.data?.installed === true
    },
    "api",
    processes,
  )
}

async function waitForWeb(webURL, processes) {
  await waitFor(
    webURL,
    async (response) => response.ok,
    "web",
    processes,
  )
}

function stopChildren() {
  for (const child of [...children].reverse()) {
    if (child.exitCode !== null) continue
    try {
      if (process.platform === "win32") {
        child.kill("SIGTERM")
      } else {
        process.kill(-child.pid, "SIGTERM")
      }
    } catch {
      try {
        child.kill("SIGTERM")
      } catch {
        // already stopped
      }
    }
  }
}

async function main() {
  const envSource = path.join(repoRoot, ".env.dev")
  const tmpRoot = await mkdtemp(path.join(tmpdir(), "metis-agentic-itsm-e2e-"))
  const configPath = path.join(tmpRoot, "config.yml")
  const envPath = path.join(tmpRoot, ".env.dev")
  const dbPath = path.join(tmpRoot, "metis-e2e.db")
  const envContent = await readFile(envSource, "utf8")
  const { apiPort, webPort } = await portPair()
  const apiURL = `http://127.0.0.1:${apiPort}`
  const webURL = `http://127.0.0.1:${webPort}`

  try {
    log(`workspace: ${tmpRoot}`)
    await writeFile(envPath, envContent, { mode: 0o600 })
    await writeFile(configPath, [
      "# Metis Agentic ITSM E2E configuration",
      "db_driver: sqlite",
      `db_dsn: ${JSON.stringify(`${dbPath}?_pragma=foreign_keys(1)&_pragma=journal_mode(WAL)`)}`,
      `secret_key: ${secret()}`,
      `jwt_secret: ${secret()}`,
      `license_key_secret: ${secret()}`,
      "",
    ].join("\n"), { mode: 0o600 })

    await run("seed-dev", "go", [
      "run",
      "-tags",
      "dev",
      "./cmd/server",
      "seed-dev",
      "-config",
      configPath,
      "-env",
      envPath,
    ])

    const server = spawnManaged("server", "go", [
      "run",
      "-tags",
      "dev",
      "./cmd/server",
      "-config",
      configPath,
      "-dev-env",
      envPath,
      "-host",
      "127.0.0.1",
      "-port",
      String(apiPort),
    ])
    const web = spawnManaged("web", bunBin, [
      "run",
      "dev",
      "--",
      "--host",
      "127.0.0.1",
      "--port",
      String(webPort),
      "--strictPort",
    ], {
      cwd: webRoot,
      env: {
        ...process.env,
        VITE_API_TARGET: apiURL,
      },
    })

    await waitForAPI(apiURL, [server, web])
    await waitForWeb(webURL, [server, web])

    await run("playwright", bunBin, ["run", "test:e2e:agentic-itsm"], {
      cwd: webRoot,
      env: {
        ...process.env,
        E2E_BASE_URL: webURL,
      },
    })
  } finally {
    stopChildren()
    await new Promise((resolve) => setTimeout(resolve, 1200))
    await rm(tmpRoot, { recursive: true, force: true })
  }
}

process.on("SIGINT", () => {
  stopChildren()
  process.exit(130)
})
process.on("SIGTERM", () => {
  stopChildren()
  process.exit(143)
})

main().catch((error) => {
  stopChildren()
  console.error(error)
  process.exit(1)
})
