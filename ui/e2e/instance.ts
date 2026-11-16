import { spawn, type ChildProcess } from 'node:child_process'
import { createHmac, randomBytes } from 'node:crypto'
import { once } from 'node:events'
import { mkdtemp, rm } from 'node:fs/promises'
import { createServer } from 'node:net'
import { tmpdir } from 'node:os'
import { join } from 'node:path'
import { setTimeout as delay } from 'node:timers/promises'
import { fileURLToPath } from 'node:url'
import { discordTestConfig } from './discord-config'

const root = fileURLToPath(new URL('../..', import.meta.url))
const ui = fileURLToPath(new URL('..', import.meta.url))
const passwordHash =
  '$argon2id$v=19$m=65536,t=2,p=2$cnBnLWUyZS10ZXN0LXNhbHQ$TSxyC0hp4Wz6SM6WbZtuwgKY8UrQEWA3Dsy1Ay5IGlk'

export type AppInstance = {
  url: string
  scratch: string
  logs: () => string
  stop: () => Promise<void>
}

export async function startApp(liveDiscord = false): Promise<AppInstance> {
  const binary = process.env.E2E_SERVER_BINARY
  if (!binary) throw new Error('E2E server binary was not built by global setup')
  const scratch = await mkdtemp(join(tmpdir(), 'rpg-audio-e2e-'))
  let server: ChildProcess | undefined
  let done: Promise<void> | undefined
  let stopping: Promise<void> | undefined
  let output = ''
  const stop = () => {
    stopping ??= (async () => {
      try {
        if (server && done) {
          server.kill('SIGTERM')
          const timeout = setTimeout(() => server?.kill('SIGKILL'), 5000)
          try { await done } finally { clearTimeout(timeout) }
        }
      } finally {
        await rm(scratch, { recursive: true, force: true, maxRetries: 3 })
      }
    })()
    return stopping
  }
  try {
    const port = await availablePort()
    const apiURL = `http://127.0.0.1:${port}`
    const secret = randomBytes(32).toString('hex')
    const discord = liveDiscord ? discordTestConfig(root) : undefined
    const env = { ...process.env }
    for (const name of Object.keys(env)) {
      if (name.startsWith('DISCORD_') || name.startsWith('INTEGRATION_TEST_DISCORD_')) delete env[name]
    }
    for (const name of [
      'HOST', 'PORT', 'CORS_ORIGINS', 'LOG_FORMAT', 'LOG_LEVEL', 'UPLOAD_DIR', 'FRONTEND_DIR',
      'ROOT_USERNAME', 'ROOT_PASSWORD_HASH', 'TOKEN_SECRET', 'TOKEN_DURATION',
      'TOKEN_ISSUER', 'TOKEN_AUDIENCE', 'DEV_MODE', 'DB_PATH',
    ]) delete env[name]
    server = spawn(binary, [
      'serve', '--host', '127.0.0.1', '--port', String(port), '--dev-mode',
      '--root-username', 'e2e-gm', '--root-password-hash', passwordHash,
      '--db-path', join(scratch, 'app.db'), '--upload-dir', join(scratch, 'uploads'),
      '--frontend-dir', join(ui, 'dist'),
    ], {
      cwd: scratch,
      stdio: liveDiscord ? 'ignore' : ['ignore', 'pipe', 'pipe'],
      env: {
        ...env,
        TMPDIR: scratch,
        TMP: scratch,
        TEMP: scratch,
        TOKEN_SECRET: secret,
        DISCORD_STREAMING_ENABLED: discord ? 'true' : 'false',
        DISCORD_STREAMING_BOT_TOKEN: discord?.streamerToken,
      },
    })
    const collect = (chunk: Buffer) => {
      output = (output + chunk.toString()).replaceAll(secret, '[redacted]').slice(-100_000)
    }
    server.stdout?.on('data', collect)
    server.stderr?.on('data', collect)
    done = new Promise((resolve, reject) => {
      server!.once('error', reject)
      server!.once('exit', () => resolve())
    })
    void done.catch(() => {})
    await Promise.race([
      waitForAPI(apiURL, server),
      done.then(() => { throw new Error('E2E API exited before becoming ready') }),
    ])
    await verifyAPI(apiURL, secret)
    return { url: apiURL, scratch, logs: () => output, stop }
  } catch (error) {
    await stop()
    if (output) throw new Error(`E2E app startup failed:\n${output}`, { cause: error })
    throw error
  }
}

async function availablePort() {
  const listener = createServer()
  listener.listen(0, '127.0.0.1')
  await once(listener, 'listening')
  const address = listener.address()
  if (!address || typeof address === 'string') throw new Error('Could not allocate an E2E API port')
  await new Promise<void>((resolve, reject) => listener.close(error => error ? reject(error) : resolve()))
  return address.port
}

async function waitForAPI(url: string, server: ChildProcess) {
  for (let attempt = 0; attempt < 150; attempt++) {
    if (server.exitCode !== null || server.signalCode !== null) {
      throw new Error(`E2E API exited before becoming ready (${server.signalCode ?? server.exitCode})`)
    }
    try {
      const response = await fetch(`${url}/api/v1/auth/status`, { signal: AbortSignal.timeout(1000) })
      if (response.ok) return
      throw new Error(`E2E API readiness returned HTTP ${response.status}`)
    } catch (error) {
      if (!(error instanceof TypeError) && !(error instanceof DOMException && error.name === 'TimeoutError')) throw error
    }
    await delay(200)
  }
  throw new Error('E2E API did not become ready within 30 seconds')
}

async function verifyAPI(url: string, secret: string) {
  const response = await fetch(`${url}/api/v1/login`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ username: 'e2e-gm', password: 'e2e-password' }),
    signal: AbortSignal.timeout(10_000),
  })
  if (!response.ok) throw new Error(`E2E API identity check returned HTTP ${response.status}`)
  const cookie = response.headers.getSetCookie().find(value => value.startsWith('auth_token='))
  const token = cookie?.split(';', 1)[0]?.slice('auth_token='.length)
  const parts = token?.split('.')
  // The port can be taken between allocation and the API binding it.
  if (!parts || parts.length !== 3 ||
      createHmac('sha256', secret).update(`${parts[0]}.${parts[1]}`).digest('base64url') !== parts[2]) {
    throw new Error('E2E API identity check failed: the allocated port belongs to another server')
  }
}
