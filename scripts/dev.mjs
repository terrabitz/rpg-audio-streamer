import { spawn } from 'node:child_process'
import { argon2Sync, randomBytes } from 'node:crypto'
import { once } from 'node:events'
import { mkdtemp, readFile, rm } from 'node:fs/promises'
import { createRequire } from 'node:module'
import { createServer } from 'node:net'
import { tmpdir } from 'node:os'
import { join } from 'node:path'
import { setTimeout as delay } from 'node:timers/promises'
import { fileURLToPath } from 'node:url'
import { discordEnvironment } from './discord-env.mjs'

const root = fileURLToPath(new URL('../', import.meta.url))
// Public local-development credentials, never production credentials.
const username = 'gm'
const password = 'P@ssw0rd'
const children = new Set()
const controller = new AbortController()
let scratch
let failure

for (const signal of ['SIGINT', 'SIGTERM']) {
  process.on(signal, () => controller.abort())
}
controller.signal.addEventListener(
  'abort',
  () => {
    for (const child of children) stop(child)
  },
  { once: true },
)

try {
  await main()
} catch (error) {
  fail(error)
} finally {
  controller.abort()
  await cleanup()
}
process.exitCode = failure ? 1 : 0

async function main() {
  if (process.argv.length > 2) {
    throw new Error('Usage: node scripts/dev.mjs (no arguments)')
  }
  const require = createRequire(new URL('../ui/package.json', import.meta.url))
  require.resolve('vite')
  const discord = discordEnvironment(root, process.env.DEV_DISCORD === '1')
  controller.signal.throwIfAborted()
  scratch = await mkdtemp(join(tmpdir(), 'rpg-audio-dev-'))
  console.log(`Dev instance storage: ${scratch}`)
  const binary = join(scratch, 'server')
  await run('go', ['build', '-o', binary, '.'], root)
  const port = await availablePort()
  const apiURL = `http://127.0.0.1:${port}`
  const invite = randomBytes(16).toString('hex')
  const api = start(
    binary,
    [
      'serve',
      '--host',
      '127.0.0.1',
      '--port',
      String(port),
      '--dev-mode',
      '--root-username',
      username,
      '--root-password-hash',
      hashPassword(password),
      '--token-secret',
      randomBytes(32).toString('hex'),
      '--join-token',
      invite,
      '--token-duration',
      '24h',
      '--token-issuer',
      'rpg-audio-dev',
      '--token-audience',
      'rpg-audio-dev-client',
      '--db-path',
      join(scratch, 'app.db'),
      '--upload-dir',
      join(scratch, 'uploads'),
      '--log-format',
      'pretty',
      '--log-level',
      'warn',
      ...(discord.DISCORD_BOT_TOKEN ? ['--discord-worker', join(root, 'discord/stream.mjs')] : []),
    ],
    scratch,
    false,
    discord,
  )
  api.done.then(() => {
    if (!controller.signal.aborted) fail(new Error('Dev API exited unexpectedly'))
  }, fail)
  await waitForAPI(apiURL)
  await seed(apiURL)
  controller.signal.throwIfAborted()
  // Keep Vite in a child so its exit handlers and optimizer cannot race cleanup.
  const frontend = start(
    process.execPath,
    [join(root, 'scripts/dev-ui.mjs'), scratch, apiURL, String(await availablePort())],
    root,
    true,
  )
  frontend.done.then(() => {
    if (!controller.signal.aborted) fail(new Error('Dev UI exited unexpectedly'))
  }, fail)
  const [{ url }] = await once(frontend, 'message', {
    signal: controller.signal,
  })
  console.log(`\nFresh dev instance: ${url}\nGM login: ${username} / ${password}`)
  console.log(`Player invite: ${url}table/${invite}\nStop with Ctrl-C to remove this instance.`)
  console.log(
    `DEV_INSTANCE_READY ${JSON.stringify({
      url,
      apiURL,
      username,
      password,
      invite,
      scratch,
    })}`,
  )
  await new Promise((resolve) => {
    if (controller.signal.aborted) resolve()
    else controller.signal.addEventListener('abort', resolve, { once: true })
  })
}

function hashPassword(password) {
  const salt = randomBytes(16)
  const parameters = { memory: 65536, passes: 2, parallelism: 2, tagLength: 32 }
  const hash = argon2Sync('argon2id', { ...parameters, message: password, nonce: salt })
  const base64 = (value) => value.toString('base64').replace(/=+$/, '')
  return `$argon2id$v=19$m=${parameters.memory},t=${parameters.passes},p=${parameters.parallelism}$${base64(salt)}$${base64(hash)}`
}

async function seed(apiURL) {
  const login = await request(`${apiURL}/api/v1/login`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ username, password }),
  })
  const cookie = login.headers.get('set-cookie')?.split(';')[0]
  if (!cookie) throw new Error('Dev login did not return an authentication cookie')
  const table = await (
    await request(`${apiURL}/api/v1/tables/current`, {
      headers: { Cookie: cookie },
    })
  ).json()
  const types = await (
    await request(`${apiURL}/api/v1/trackTypes`, {
      headers: { Cookie: cookie },
    })
  ).json()
  const samples = [
    {
      type: 'Ambiance',
      name: 'Forest breeze',
      source: "anoisesrc=color=pink:amplitude=0.08:seed=1,volume='0.65+0.35*sin(2*PI*t/9)':eval=frame",
      duration: 12,
    },
    {
      type: 'Ambiance',
      name: 'Rainfall',
      source: "anoisesrc=color=white:amplitude=0.03:seed=2,volume='0.65+0.35*sin(2*PI*t/7+1)':eval=frame",
      duration: 12,
    },
    {
      type: 'Ambiance',
      name: 'Distant rumble',
      source: "anoisesrc=color=brown:amplitude=0.08:seed=3,volume='0.65+0.35*sin(2*PI*t/11+2)':eval=frame",
      duration: 12,
    },
    {
      type: 'Music',
      name: 'Gentle melody',
      source: melodySource([262, 330, 392, 440, 392, 330, 294, 220]),
      duration: 12,
    },
    {
      type: 'Music',
      name: 'Tavern tune',
      source: melodySource([196, 247, 294, 392, 330, 294, 247, 220]),
      duration: 12,
    },
    {
      type: 'Music',
      name: 'Adventure theme',
      source: melodySource([262, 294, 330, 392, 440, 392, 330, 294]),
      duration: 12,
    },
    {
      type: 'One-Shot',
      name: 'Bell',
      source: 'sine=frequency=880',
      duration: 2,
    },
    {
      type: 'One-Shot',
      name: 'Chime',
      source: 'sine=frequency=1320',
      duration: 1,
    },
    {
      type: 'One-Shot',
      name: 'Drumbeat',
      source: 'sine=frequency=110',
      duration: 1.5,
    },
  ]
  for (const [index, sample] of samples.entries()) {
    const type = types.find((type) => type.name === sample.type)
    if (!type) throw new Error(`Missing seeded track type: ${sample.type}`)
    const file = join(scratch, `sample-${index}.wav`)
    await run(
      'ffmpeg',
      [
        '-hide_banner',
        '-loglevel',
        'error',
        '-f',
        'lavfi',
        '-i',
        sample.source,
        '-t',
        String(sample.duration),
        '-af',
        'afade=t=in:d=0.1,afade=t=out:st=' + String(sample.duration - 0.5) + ':d=0.5',
        '-y',
        file,
      ],
      scratch,
    )
    const body = new FormData()
    body.set('files', new Blob([await readFile(file)], { type: 'audio/wav' }), 'sample.wav')
    body.set('name', sample.name)
    body.set('typeID', type.id)
    body.set('tableID', table.id)
    await request(`${apiURL}/api/v1/files`, {
      method: 'POST',
      headers: { Cookie: cookie },
      body,
    })
    await rm(file)
  }
}

function melodySource(notes) {
  const noteDuration = 1.5
  const expressions = notes.map((frequency, index) => {
    const start = index * noteDuration
    const time = `(t-${start})`
    return `if(between(t,${start},${start + noteDuration}),0.12*sin(2*PI*${frequency}*${time})*sin(PI*${time}/${noteDuration}),0)`
  })
  return `aevalsrc='${expressions.join('+')}':s=44100`
}

async function waitForAPI(apiURL) {
  for (let attempt = 0; attempt < 150; attempt++) {
    controller.signal.throwIfAborted()
    try {
      await request(`${apiURL}/api/v1/auth/status`)
      return
    } catch (error) {
      if (!(error instanceof TypeError)) throw error
    }
    await delay(200, undefined, { signal: controller.signal })
  }
  throw new Error('Dev API did not become ready within 30 seconds')
}

async function request(url, options = {}) {
  const response = await fetch(url, {
    ...options,
    signal: AbortSignal.any([controller.signal, AbortSignal.timeout(30_000)]),
  })
  if (!response.ok) {
    throw new Error(
      `${options.method ?? 'GET'} ${url}: ${response.status} ${await response.text()}`,
    )
  }
  return response
}

async function availablePort() {
  const listener = createServer()
  listener.listen(0, '127.0.0.1')
  await once(listener, 'listening')
  const { port } = listener.address()
  await new Promise((resolve, reject) =>
    listener.close((error) => (error ? reject(error) : resolve())),
  )
  return port
}

async function run(command, args, cwd) {
  const child = start(command, args, cwd)
  const [code, signal] = await child.done
  controller.signal.throwIfAborted()
  if (code !== 0) throw new Error(`${command} failed (${signal ?? code})`)
}

function start(command, args, cwd, ipc = false, overrides = {}) {
  controller.signal.throwIfAborted()
  const env = {
    ...process.env,
    TMPDIR: scratch,
    TMP: scratch,
    TEMP: scratch,
    // Vite must not interpret a detached launcher's closed stdin as shutdown.
    ...(ipc ? { CI: 'true' } : {}),
  }
  // The CLI parses environment values even when explicit fixture flags override them.
  for (const name of [
    'HOST',
    'PORT',
    'CORS_ORIGINS',
    'LOG_FORMAT',
    'LOG_LEVEL',
    'UPLOAD_DIR',
    'ROOT_USERNAME',
    'ROOT_PASSWORD_HASH',
    'TOKEN_SECRET',
    'TOKEN_DURATION',
    'TOKEN_ISSUER',
    'TOKEN_AUDIENCE',
    'JOIN_TOKEN',
    'DEV_MODE',
    'DB_PATH',
    'API_PROXY_TARGET',
    'VITE_API_BASE_URL',
  ]) {
    delete env[name]
  }
  for (const name of Object.keys(env)) {
    if (name.startsWith('DISCORD_') || name.startsWith('INTEGRATION_TEST_DISCORD_')) delete env[name]
  }
  const child = spawn(command, args, {
    cwd,
    stdio: ipc ? ['ignore', 'inherit', 'inherit', 'ipc'] : 'inherit',
    env: { ...env, ...overrides },
  })
  children.add(child)
  child.done = once(child, 'exit').finally(() => children.delete(child))
  // Attach immediately so a spawn failure cannot become an unhandled rejection.
  child.done.catch(fail)
  return child
}

function fail(error) {
  if (controller.signal.aborted) return
  failure = error
  console.error('Dev instance failed:', error)
  controller.abort()
}

async function cleanup() {
  await Promise.all([...children].map(stop))
  if (scratch) {
    await rm(scratch, { recursive: true, force: true, maxRetries: 3 })
    console.log(`Removed dev instance storage: ${scratch}`)
  }
}

function stop(child) {
  if (child.stopping) return child.stopping
  child.kill('SIGTERM')
  const timeout = setTimeout(() => child.kill('SIGKILL'), 5_000)
  child.stopping = child.done
    .catch((error) => {
      console.error('Child process failed:', error)
      failure ??= error
    })
    .finally(() => clearTimeout(timeout))
  return child.stopping
}
