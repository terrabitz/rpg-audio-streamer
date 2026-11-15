import { readFileSync } from 'node:fs'
import { homedir } from 'node:os'
import { join } from 'node:path'
import { parseEnv } from 'node:util'

export function discordTestConfig(root: string, environment: NodeJS.ProcessEnv = process.env) {
  const override = environment.DISCORD_TEST_ENV_FILE
  const paths = [
    join(root, '.env.test'),
    override || join(environment.HOME || homedir(), '.config', 'rpg-audio-streamer', 'discord-test.env'),
  ]
  let source: NodeJS.ProcessEnv = {}
  for (const path of paths) {
    try {
      source = { ...source, ...parseEnv(readFileSync(path, 'utf8')) }
    } catch (error) {
      if ((error as NodeJS.ErrnoException).code !== 'ENOENT' || (override && path === override)) throw error
    }
  }
  source = { ...source, ...Object.fromEntries(Object.entries(environment).filter(([, value]) => value)) }
  const names = [
    'INTEGRATION_TEST_DISCORD_SERVER_ID',
    'INTEGRATION_TEST_DISCORD_VOICE_CHANNEL_ID',
    'INTEGRATION_TEST_DISCORD_BOT_STREAMER_TOKEN',
    'INTEGRATION_TEST_DISCORD_BOT_STREAMER_CLIENT_ID',
    'INTEGRATION_TEST_DISCORD_BOT_LISTENER_TOKEN',
  ] as const
  const missing = names.filter(name => !source[name])
  if (missing.length) throw new Error(`Missing Discord configuration: ${missing.join(', ')}`)
  const guildID = source.INTEGRATION_TEST_DISCORD_SERVER_ID!
  const channelID = source.INTEGRATION_TEST_DISCORD_VOICE_CHANNEL_ID!
  const streamerID = source.INTEGRATION_TEST_DISCORD_BOT_STREAMER_CLIENT_ID!
  if (![guildID, channelID, streamerID].every(id =>
    /^[1-9][0-9]{0,19}$/.test(id) && BigInt(id) <= 18446744073709551615n)) {
    throw new Error('Invalid Discord test server, channel, or streamer ID')
  }
  const streamerToken = source.INTEGRATION_TEST_DISCORD_BOT_STREAMER_TOKEN!
  const listenerToken = source.INTEGRATION_TEST_DISCORD_BOT_LISTENER_TOKEN!
  if (streamerToken === listenerToken) throw new Error('Discord tests require distinct bot tokens')
  return { guildID, channelID, streamerID, streamerToken, listenerToken }
}
