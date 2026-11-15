import { readFileSync } from 'node:fs'
import { homedir } from 'node:os'
import { join } from 'node:path'
import { parseEnv } from 'node:util'

export function discordEnvironment(root, enabled, environment = process.env) {
  if (!enabled) return {}
  const source = discordSource(root, environment)
  const token = source.DISCORD_STREAMING_BOT_TOKEN ?? source.INTEGRATION_TEST_DISCORD_BOT_STREAMER_TOKEN
  if (!token) throw new Error('Missing Discord configuration: DISCORD_STREAMING_BOT_TOKEN')
  return { DISCORD_STREAMING_ENABLED: 'true', DISCORD_STREAMING_BOT_TOKEN: token }
}

function discordSource(root, environment) {
  let file = {}
  const paths = [
    join(environment.HOME || homedir(), '.config', 'rpg-audio-streamer', '.env'),
    join(root, '.env'),
  ]
  for (const path of paths) {
    try {
      file = { ...file, ...parseEnv(readFileSync(path, 'utf8')) }
    } catch (error) {
      if (error.code !== 'ENOENT') throw error
    }
  }
  return { ...file, ...Object.fromEntries(Object.entries(environment).filter(([, value]) => value)) }
}
