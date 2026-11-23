import assert from 'node:assert/strict'
import { execFile } from 'node:child_process'
import { cp, mkdtemp, rm } from 'node:fs/promises'
import { tmpdir } from 'node:os'
import { join } from 'node:path'
import { test } from 'node:test'
import { setTimeout as sleep } from 'node:timers/promises'
import { fileURLToPath } from 'node:url'
import { promisify } from 'node:util'

const examples = fileURLToPath(new URL('./', import.meta.url))
const image = 'ghcr.io/terrabitz/rpg-audio-streamer:latest'
const container = 'rpg-audio-streamer'
const appURL = 'http://localhost:8080'
const exec = promisify(execFile)

test('quick start exposes a setup code that claims the instance', { timeout: 180_000 }, async (t) => {
  const dir = await workspace(t, 'quick-start.sh', removeContainerAndData)

  await runScript(dir, 'quick-start.sh')

  await claimWithLoggedCode(() => command(dir, 'docker', ['logs', container]))
})

test('docker compose installation exposes a setup code that claims the instance', { timeout: 180_000 }, async (t) => {
  const dir = await workspace(t, 'docker-compose', (dir) => command(dir, 'docker', ['compose', 'down', '--volumes']))

  await runScript(dir, 'start.sh')

  await claimWithLoggedCode(() => runScript(dir, 'logs.sh'))
})

test('plain docker installation exposes a setup code that claims the instance', { timeout: 180_000 }, async (t) => {
  const dir = await workspace(t, 'docker', removeContainerAndData)

  await runScript(dir, 'create-data-dir.sh')
  await runScript(dir, 'run.sh')

  await claimWithLoggedCode(() => runScript(dir, 'logs.sh'))
})

async function claimWithLoggedCode(readLogs) {
  const code = await poll('setup code in logs', async () => {
    return (await readLogs()).match(/\bsetup_code=(\S+)/)?.[1]
  })
  const before = await poll('setup status', async () => {
    const response = await fetch(`${appURL}/api/v1/setup`).catch(() => undefined)
    return response?.ok ? response.json() : undefined
  })
  assert.equal(before.required, true)

  const claim = await fetch(`${appURL}/api/v1/setup`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
    },
    body: JSON.stringify({
      code,
      username: 'example-gm',
      password: 'example-password',
    }),
  })
  assert.equal(claim.status, 204, await claim.text())

  const after = await (await fetch(`${appURL}/api/v1/setup`)).json()
  assert.equal(after.required, false)
}

async function workspace(t, example, teardown) {
  const dir = await mkdtemp(join(tmpdir(), 'rpg-audio-streamer-example-'))
  t.after(async () => {
    try {
      await teardown(dir)
    } finally {
      await rm(dir, { recursive: true, force: true })
    }
  })
  const destination = example.endsWith('.sh') ? join(dir, example) : dir
  await cp(join(examples, example), destination, { recursive: true })
  return dir
}

async function removeContainerAndData(dir) {
  await command(dir, 'docker', ['rm', '--force', '--volumes', container])
  // The container writes the bind-mounted data as root, so the host user cannot delete it directly.
  await command(dir, 'docker', ['run', '--rm', '--entrypoint', 'rm', '-v', `${dir}:/example`, image, '-rf', '/example/data'])
}

function runScript(dir, script) {
  return command(dir, 'bash', ['-e', script])
}

async function command(cwd, file, args) {
  const { stdout, stderr } = await exec(file, args, { cwd })
  return stdout + stderr
}

async function poll(description, attempt) {
  const deadline = Date.now() + 90_000
  while (Date.now() < deadline) {
    const result = await attempt()
    if (result !== undefined) {
      return result
    }
    await sleep(500)
  }
  throw new Error(`Timed out waiting for ${description}`)
}
