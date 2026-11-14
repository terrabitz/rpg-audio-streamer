import { execFileSync, spawn } from 'node:child_process'
import { mkdtempSync, rmSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { join } from 'node:path'
import { fileURLToPath } from 'node:url'
import { discordEnvironment } from '../../scripts/discord-env.mjs'

const root = fileURLToPath(new URL('../..', import.meta.url))
const discord = discordEnvironment(root, process.env.E2E_DISCORD === '1')
const scratch = mkdtempSync(join(tmpdir(), 'rpg-audio-e2e-'))
const binary = join(scratch, 'server')

// Public fixture only: "e2e-password", never a real account credential.
const passwordHash =
  '$argon2id$v=19$m=65536,t=2,p=2$cnBnLWUyZS10ZXN0LXNhbHQ$TSxyC0hp4Wz6SM6WbZtuwgKY8UrQEWA3Dsy1Ay5IGlk'

try {
  execFileSync('go', ['build', '-o', binary, '.'], { cwd: root, stdio: 'inherit' })
} catch (error) {
  rmSync(scratch, { recursive: true, force: true })
  throw error
}

const server = spawn(
  binary,
  [
    'serve',
    '--port',
    process.env.E2E_API_PORT ?? '18080',
    '--dev-mode',
    ...(process.env.E2E_EXPERIMENTAL_MIXER === '1' ? ['--experimental-mixer'] : []),
    ...(discord.DISCORD_BOT_TOKEN ? ['--discord-worker', join(root, 'discord/stream.mjs')] : []),
    '--root-username',
    'e2e-gm',
    '--root-password-hash',
    passwordHash,
    '--token-secret',
    'public-e2e-signing-key-not-for-production',
    '--join-token',
    'e2e-invite',
    '--db-path',
    join(scratch, 'app.db'),
    '--upload-dir',
    join(scratch, 'uploads'),
  ],
  {
    cwd: scratch,
    stdio: 'inherit',
    env: {
      ...Object.fromEntries(Object.entries(process.env).filter(([name]) =>
        !name.startsWith('DISCORD_') && !name.startsWith('INTEGRATION_TEST_DISCORD_'))),
      ...discord,
    },
  },
)

server.on('error', (error) => {
  console.error(error)
  rmSync(scratch, { recursive: true, force: true })
  process.exit(1)
})
server.on('exit', (code, signal) => {
  rmSync(scratch, { recursive: true, force: true })
  process.exit(signal === 'SIGTERM' || signal === 'SIGINT' ? 0 : (code ?? 1))
})
process.on('SIGTERM', () => server.kill('SIGTERM'))
process.on('SIGINT', () => server.kill('SIGINT'))
