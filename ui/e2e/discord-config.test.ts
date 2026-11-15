import assert from 'node:assert/strict'
import { mkdtempSync, rmSync, writeFileSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { join } from 'node:path'
import { test } from 'node:test'
import { discordTestConfig } from './discord-config.ts'

test('Discord test configuration requires distinct bots and valid destination', () => {
  const env = {
    HOME: '/does-not-exist',
    INTEGRATION_TEST_DISCORD_SERVER_ID: '1',
    INTEGRATION_TEST_DISCORD_VOICE_CHANNEL_ID: '2',
    INTEGRATION_TEST_DISCORD_BOT_STREAMER_CLIENT_ID: '3',
    INTEGRATION_TEST_DISCORD_BOT_STREAMER_TOKEN: 'streamer',
    INTEGRATION_TEST_DISCORD_BOT_LISTENER_TOKEN: 'listener',
  }
  assert.throws(() => discordTestConfig('/does-not-exist', { HOME: '/does-not-exist' }), /Missing Discord configuration/)
  assert.throws(() => discordTestConfig('/does-not-exist', {
    ...env, INTEGRATION_TEST_DISCORD_BOT_LISTENER_TOKEN: 'streamer',
  }), /distinct/)
  assert.throws(() => discordTestConfig('/does-not-exist', {
    ...env, INTEGRATION_TEST_DISCORD_SERVER_ID: 'invalid',
  }), /Invalid Discord test/)
})

test('Discord test configuration honors local, external, and environment precedence', () => {
  const root = mkdtempSync(join(tmpdir(), 'discord-env-test-'))
  try {
    writeFileSync(join(root, '.env.test'), 'INTEGRATION_TEST_DISCORD_SERVER_ID=1\nINTEGRATION_TEST_DISCORD_VOICE_CHANNEL_ID=2\n')
    const external = join(root, 'external.env')
    writeFileSync(external, 'INTEGRATION_TEST_DISCORD_SERVER_ID=3\nINTEGRATION_TEST_DISCORD_BOT_STREAMER_CLIENT_ID=4\nINTEGRATION_TEST_DISCORD_BOT_STREAMER_TOKEN=streamer\nINTEGRATION_TEST_DISCORD_BOT_LISTENER_TOKEN=listener\n')
    const config = discordTestConfig(root, {
      DISCORD_TEST_ENV_FILE: external,
      INTEGRATION_TEST_DISCORD_SERVER_ID: '5',
    })
    assert.equal(config.guildID, '5')
    assert.equal(config.channelID, '2')
    assert.equal(config.streamerID, '4')
    assert.throws(() => discordTestConfig(root, { DISCORD_TEST_ENV_FILE: join(root, 'missing') }), /ENOENT/)
  } finally {
    rmSync(root, { recursive: true, force: true })
  }
})
