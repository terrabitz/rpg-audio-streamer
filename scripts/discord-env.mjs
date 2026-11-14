import { readFileSync } from 'node:fs'
import { join } from 'node:path'
import { parseEnv } from 'node:util'

export function discordEnvironment(root, enabled, environment = process.env) {
  if (!enabled) return {}
  const source = discordSource(root, environment)
  const token = source.DISCORD_BOT_TOKEN ?? source.INTEGRATION_TEST_DISCORD_BOT_STREAMER_TOKEN
  if (!token) throw new Error('Missing Discord configuration: DISCORD_BOT_TOKEN')
  return { DISCORD_BOT_TOKEN: token }
}

export function discordTestDestination(root, environment = process.env) {
  const source = discordSource(root, environment)
  const guildID = source.INTEGRATION_TEST_DISCORD_SERVER_ID
  const channelID = source.INTEGRATION_TEST_DISCORD_VOICE_CHANNEL_ID
  if (!/^[1-9][0-9]{0,19}$/.test(guildID ?? '') || !/^[1-9][0-9]{0,19}$/.test(channelID ?? '')) {
    throw new Error('Missing Discord test destination: INTEGRATION_TEST_DISCORD_SERVER_ID and INTEGRATION_TEST_DISCORD_VOICE_CHANNEL_ID')
  }
  return { guildID, channelID }
}

function discordSource(root, environment) {
  let file = {}
  try {
    file = parseEnv(readFileSync(join(root, '.env'), 'utf8'))
  } catch (error) {
    if (error.code !== 'ENOENT') throw error
  }
  return { ...file, ...environment }
}
