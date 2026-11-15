import { execFileSync } from 'node:child_process'
import { mkdtempSync, rmSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { join } from 'node:path'
import { fileURLToPath } from 'node:url'

export default function globalSetup() {
  const root = fileURLToPath(new URL('../..', import.meta.url))
  const ui = fileURLToPath(new URL('..', import.meta.url))
  const scratch = mkdtempSync(join(tmpdir(), 'rpg-audio-e2e-build-'))
  const binary = join(scratch, 'server')
  const env = { ...process.env }
  for (const name of Object.keys(env)) {
    if (name.startsWith('VITE_')) delete env[name]
  }
  try {
    execFileSync('go', ['build', '-o', binary, '.'], { cwd: root, stdio: 'inherit' })
    execFileSync('npm', ['run', 'build'], {
      cwd: ui,
      stdio: 'inherit',
      env: { ...env, E2E_ENV_DIR: scratch, VITE_API_BASE_URL: '/' },
    })
    process.env.E2E_SERVER_BINARY = binary
  } catch (error) {
    rmSync(scratch, { recursive: true, force: true })
    throw error
  }
  return () => {
    delete process.env.E2E_SERVER_BINARY
    rmSync(scratch, { recursive: true, force: true })
  }
}
