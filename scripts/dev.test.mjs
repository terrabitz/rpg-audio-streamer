import assert from 'node:assert/strict'
import { spawn } from 'node:child_process'
import { once } from 'node:events'
import { access, mkdtemp, rm, symlink } from 'node:fs/promises'
import { tmpdir } from 'node:os'
import { join } from 'node:path'
import { createInterface } from 'node:readline'
import { test } from 'node:test'
import { fileURLToPath } from 'node:url'

const root = fileURLToPath(new URL('../', import.meta.url))

test(
  'real dev instances seed playable data, isolate changes, and clean up',
  { timeout: 180_000 },
  async (t) => {
    const first = launch(t, {
      ROOT_USERNAME: 'must-not-be-used',
      ROOT_PASSWORD_HASH: 'must-not-be-used',
      TOKEN_DURATION: 'invalid',
      API_PROXY_TARGET: 'http://127.0.0.1:1',
      VITE_API_BASE_URL: 'https://unused.invalid/',
    })
    const second = launch(t)
    const [a, b] = await Promise.all([first.ready, second.ready])
    assert.notEqual(a.scratch, b.scratch)
    assert.notEqual(a.url, b.url)
    assert.notEqual(a.apiURL, b.apiURL)
    assert.equal(a.username, 'gm')
    assert.equal(a.password, 'P@ssw0rd')
    assert.equal(a.username, b.username)
    assert.equal(a.password, b.password)
    const cookie = await login(a)
    const otherCookie = await login(b)
    const types = await json(a, '/api/v1/trackTypes', cookie)
    assert.deepEqual(types.map((type) => type.name).sort(), ['Ambiance', 'Music', 'One-Shot'])
    const tracks = await json(a, '/api/v1/files', cookie)
    assert.equal(tracks.length, 9)
    assert.equal(new Set(tracks.map((track) => track.name)).size, 9)
    for (const type of types) {
      assert.equal(tracks.filter((track) => track.typeID === type.id).length, 3, type.name)
    }
    assert.deepEqual(
      new Set(tracks.map((track) => track.typeID)),
      new Set(types.map((type) => type.id)),
    )
    const table = (await json(a, '/api/v1/tables', cookie))[0]
    assert.equal(Buffer.from(table.externalID, 'base64url').byteLength, 32)
    const settingsPath = `/api/v1/tables/${table.externalID}/settings`
    const defaults = { masterVolume: 100, typeVolumes: {}, trackVolumes: {}, trackOrder: [] }
    assert.deepEqual(await json(a, settingsPath, cookie), defaults)
    await response(a, settingsPath, cookie, {
      method: 'PATCH',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ masterVolume: 0, trackVolumes: { [tracks[0].id]: 35 }, trackOrder: [tracks[0].id] }),
    })
    assert.equal((await json(a, settingsPath, cookie)).masterVolume, 0)
    const otherTable = (await json(b, '/api/v1/tables', otherCookie))[0]
    assert.deepEqual(await json(b, `/api/v1/tables/${otherTable.externalID}/settings`, otherCookie), defaults)
    const tableTracks = await json(a, `/api/v1/tables/${table.externalID}/tracks`, cookie)
    assert.deepEqual(
      new Set(tableTracks.map((track) => track.id)),
      new Set(tracks.map((track) => track.id)),
    )
    for (const track of tracks) {
      assert.ok(!track.name.startsWith('Dev -'), track.name)
      assert.ok(track.path.startsWith(join(a.scratch, 'uploads')))
      const playlist = await response(a, `/api/v1/tables/${table.externalID}/stream/${track.id}/index.m3u8`)
      const text = await playlist.text()
      assert.match(text, /^#EXTM3U/)
      const segment = text.split('\n').find((line) => line.endsWith('.ts'))
      assert.ok(segment, 'playlist references a real segment')
      const media = await response(a, `/api/v1/tables/${table.externalID}/stream/${track.id}/${segment}`)
      assert.ok((await media.arrayBuffer()).byteLength > 0)
    }
    for (const type of types.filter((type) => type.isRepeating)) {
      for (const track of tracks.filter((track) => track.typeID === type.id)) {
        const audio = await decodeAudio(join(track.path, 'index.m3u8'))
        assert.ok(Math.abs(audio.length / 8000 - 12) < 0.2, `${track.name}: duration stays 12 seconds`)
        if (type.name === 'Music') {
          const firstNote = frequency(audio, 0.5, 1)
          const secondNote = frequency(audio, 2, 2.5)
          assert.ok(firstNote > 150 && firstNote < 500, `${track.name}: audible first note`)
          assert.ok(Math.abs(firstNote - secondNote) > 20, `${track.name}: melody changes pitch`)
          const finalNote = frequency(audio, 11, 11.4)
          assert.ok(Math.abs(firstNote - finalNote) > 20, `${track.name}: loop boundary changes pitch`)
        } else {
          const levels = Array.from({ length: 10 }, (_, index) => rms(audio, index + 0.5, index + 1))
          assert.ok(Math.min(...levels) > 0.001, `${track.name}: swells do not fall silent`)
          assert.ok(Math.max(...levels) / Math.min(...levels) > 1.5, `${track.name}: ambiance swells over time`)
        }
      }
    }
    const playerPlaylist = await response(a, `/api/v1/tables/${table.externalID}/stream/${tracks[0].id}/index.m3u8`)
    assert.match(await playerPlaylist.text(), /^#EXTM3U/)
    const ui = await fetch(a.url)
    assert.equal(ui.status, 200)
    assert.match(await ui.text(), /src\/main.ts/)
    const source = await fetch(new URL('/src/main.ts', a.url))
    assert.equal(source.status, 200, 'Vite compiles the real application')
    const clientConfig = await fetch(new URL('/src/plugins/api.ts', a.url))
    assert.equal(clientConfig.status, 200)
    assert.doesNotMatch(await clientConfig.text(), /https:\/\/unused\.invalid/)
    const rejected = await fetch(new URL('/api/v1/files', b.url), {
      headers: { Cookie: cookie },
    })
    assert.equal(rejected.status, 401, 'signing keys differ between instances')
    const publicTable = await json(a, `/api/v1/tables/${table.externalID}`)
    assert.equal(publicTable.id, table.id)
    assert.equal(publicTable.externalID, table.externalID)
    const publicTracks = await json(a, `/api/v1/tables/${table.externalID}/tracks`)
    assert.equal(publicTracks.length, tracks.length)
    await response(a, `/api/v1/files/${tracks[0].id}?expectedTableCount=1`, cookie, {
      method: 'DELETE',
    })
    assert.equal((await json(a, '/api/v1/files', cookie)).length, 8)
    assert.equal((await json(a, `/api/v1/tables/${table.externalID}/tracks`, cookie)).length, 8)
    assert.equal((await json(b, '/api/v1/files', otherCookie)).length, 9)
    await first.stop('SIGINT')
    await assertCleaned(a)
    assert.equal((await json(b, '/api/v1/files', otherCookie)).length, 9)
    await second.stop('SIGTERM')
    await assertCleaned(b)
    const restarted = launch(t)
    const fresh = await restarted.ready
    const freshCookie = await login(fresh)
    const freshTracks = await json(fresh, '/api/v1/files', freshCookie)
    assert.equal(freshTracks.length, 9)
    assert.ok(freshTracks.every((track) => !tracks.some((old) => old.id === track.id)))
    const freshTable = (await json(fresh, '/api/v1/tables', freshCookie))[0]
    assert.deepEqual(await json(fresh, `/api/v1/tables/${freshTable.externalID}/settings`, freshCookie), defaults)
    await restarted.stop()
    await assertCleaned(fresh)
  },
)

test(
  'startup failure removes instance state and exits nonzero',
  { timeout: 120_000 },
  async (t) => {
    // Only Go is available to children, so sample generation must fail without FFmpeg.
    const path = await mkdtemp(join(tmpdir(), 'rpg-dev-test-path-'))
    t.after(() => rm(path, { recursive: true, force: true }))
    const go = spawn('which', ['go'])
    let executable = ''
    go.stdout.on('data', (data) => {
      executable += data
    })
    assert.equal((await once(go, 'exit'))[0], 0)
    await symlink(executable.trim(), join(path, 'go'))
    const instance = launch(t, { PATH: path })
    await assert.rejects(instance.ready, /Dev instance failed/)
    const [code] = await instance.exited
    assert.equal(code, 1)
    assert.match(instance.output(), /ffmpeg/)
    const scratch = instance.output().match(/Dev instance storage: (.+)/)?.[1]
    assert.ok(scratch)
    await assert.rejects(access(scratch), { code: 'ENOENT' })
  },
)

test('interrupting startup removes temporary storage', { timeout: 120_000 }, async (t) => {
  const instance = launch(t)
  await instance.storage
  await instance.stop()
  await assert.rejects(instance.ready)
  const scratch = instance.output().match(/Dev instance storage: (.+)/)?.[1]
  assert.ok(scratch)
  await assert.rejects(access(scratch), { code: 'ENOENT' })
})

async function decodeAudio(path) {
  const child = spawn('ffmpeg', [
    '-v', 'error', '-i', path, '-f', 'f32le', '-ac', '1', '-ar', '8000', 'pipe:1',
  ])
  const chunks = []
  let stderr = ''
  child.stdout.on('data', (chunk) => chunks.push(chunk))
  child.stderr.on('data', (chunk) => { stderr += chunk })
  assert.equal((await once(child, 'exit'))[0], 0, stderr)
  const bytes = Buffer.concat(chunks)
  return Array.from({ length: bytes.length / 4 }, (_, index) => bytes.readFloatLE(index * 4))
}

function frequency(audio, start, end) {
  const samples = audio.slice(Math.round(start * 8000), Math.round(end * 8000))
  const crossings = samples.reduce((count, sample, index) =>
    count + (index > 0 && samples[index - 1] <= 0 && sample > 0 ? 1 : 0), 0)
  return crossings / (end - start)
}

function rms(audio, start, end) {
  const samples = audio.slice(Math.round(start * 8000), Math.round(end * 8000))
  return Math.sqrt(samples.reduce((sum, sample) => sum + sample * sample, 0) / samples.length)
}

function launch(t, env = {}) {
  const child = spawn(process.execPath, ['scripts/dev.mjs'], {
    cwd: root,
    env: { ...process.env, ...env },
    stdio: ['ignore', 'pipe', 'pipe'],
  })
  let output = ''
  child.stderr.on('data', (data) => {
    output += data
  })
  child.stdout.on('data', (data) => {
    output += data
  })
  const exited = once(child, 'exit')
  const lines = createInterface({ input: child.stdout })
  const ready = new Promise((resolve, reject) => {
    lines.on('line', (line) => {
      if (line.startsWith('DEV_INSTANCE_READY ')) {
        resolve(JSON.parse(line.slice('DEV_INSTANCE_READY '.length)))
      }
    })
    exited.then(() => reject(new Error(output)), reject)
  })
  ready.catch(() => {})
  const storage = new Promise((resolve) => {
    lines.on('line', (line) => {
      if (line.startsWith('Dev instance storage: ')) resolve()
    })
  })
  async function stop(signal = 'SIGTERM') {
    if (child.exitCode !== null || child.signalCode !== null) return
    child.kill(signal)
    const timeout = setTimeout(() => child.kill('SIGKILL'), 15_000)
    try {
      const [code] = await exited
      assert.equal(code, 0, output)
      assert.match(output, /Removed dev instance storage:/, output)
    } finally {
      clearTimeout(timeout)
    }
  }
  t.after(() => stop())
  return { ready, storage, exited, stop, output: () => output }
}

async function login(instance) {
  const result = await response(instance, '/api/v1/login', undefined, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({
      username: instance.username,
      password: instance.password,
    }),
  })
  const cookie = result.headers.get('set-cookie')?.split(';')[0]
  assert.ok(cookie)
  return cookie
}

async function json(instance, path, cookie) {
  return (await response(instance, path, cookie)).json()
}

async function response(instance, path, cookie, options = {}) {
  const result = await fetch(new URL(path, instance.url), {
    ...options,
    headers: { ...options.headers, ...(cookie ? { Cookie: cookie } : {}) },
    signal: AbortSignal.timeout(10_000),
  })
  assert.equal(result.status, 200, `${path}: ${await result.clone().text()}`)
  return result
}

async function assertCleaned(instance) {
  await assert.rejects(access(instance.scratch), { code: 'ENOENT' }, instance.scratch)
  for (const url of [instance.url, `${instance.apiURL}/api/v1/auth/status`]) {
    await assert.rejects(fetch(url, { signal: AbortSignal.timeout(2_000) }))
  }
}
