import assert from 'node:assert/strict'
import { test } from 'node:test'
import { discordEnvironment, discordTestDestination } from './discord-env.mjs'

test('ordinary development and browser tests never enable Discord from ambient credentials', () => {
  assert.deepEqual(discordEnvironment('/does-not-exist', false, { DISCORD_BOT_TOKEN: 'test-only' }), {})
})

test('explicit Discord runs map test credentials and fail clearly on missing values', () => {
  assert.deepEqual(discordEnvironment('/does-not-exist', true, {
    INTEGRATION_TEST_DISCORD_BOT_STREAMER_TOKEN: 'test-only',
    INTEGRATION_TEST_DISCORD_SERVER_ID: '1',
    INTEGRATION_TEST_DISCORD_VOICE_CHANNEL_ID: '2',
  }), { DISCORD_BOT_TOKEN: 'test-only' })
  assert.deepEqual(discordEnvironment('/does-not-exist', true, { DISCORD_BOT_TOKEN: 'test-only' }), {
    DISCORD_BOT_TOKEN: 'test-only',
  })
  assert.throws(() => discordEnvironment('/does-not-exist', true, {}), /Missing Discord configuration/)
})

test('live browser fixtures explicitly select the isolated test destination', () => {
  assert.deepEqual(discordTestDestination('/does-not-exist', {
    INTEGRATION_TEST_DISCORD_SERVER_ID: '1',
    INTEGRATION_TEST_DISCORD_VOICE_CHANNEL_ID: '2',
  }), { guildID: '1', channelID: '2' })
  assert.throws(() => discordTestDestination('/does-not-exist', {}), /Missing Discord test destination/)
})
