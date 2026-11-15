import assert from 'node:assert/strict'
import { mkdirSync, mkdtempSync, writeFileSync, rmSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { join } from 'node:path'
import { test } from 'node:test'
import { discordEnvironment } from './discord-env.mjs'

test('ordinary development and browser tests never enable Discord from ambient credentials', () => {
  assert.deepEqual(discordEnvironment('/does-not-exist', false, {
    HOME: '/does-not-exist', DISCORD_STREAMING_BOT_TOKEN: 'test-only',
  }), {})
})
test('explicit Discord runs map test credentials and fail clearly on missing values', () => {
  assert.deepEqual(discordEnvironment('/does-not-exist', true, {
    HOME: '/does-not-exist',
    INTEGRATION_TEST_DISCORD_BOT_STREAMER_TOKEN: 'test-only',
    INTEGRATION_TEST_DISCORD_SERVER_ID: '1',
    INTEGRATION_TEST_DISCORD_VOICE_CHANNEL_ID: '2',
  }), { DISCORD_STREAMING_ENABLED: 'true', DISCORD_STREAMING_BOT_TOKEN: 'test-only' })
  assert.deepEqual(discordEnvironment('/does-not-exist', true, {
    HOME: '/does-not-exist', DISCORD_STREAMING_BOT_TOKEN: 'test-only',
  }), { DISCORD_STREAMING_ENABLED: 'true', DISCORD_STREAMING_BOT_TOKEN: 'test-only' })
  assert.throws(() => discordEnvironment('/does-not-exist', true, { HOME: '/does-not-exist' }), /Missing Discord configuration/)
})

test('Discord dev configuration reads user .env, lets project and process values override it', () => {
  const root = mkdtempSync(join(tmpdir(), 'discord-dev-env-'))
  const home = join(root, 'home')
  const configDir = join(home, '.config', 'rpg-audio-streamer')
  const project = join(root, 'project')
  const missing = join(root, 'missing')
  mkdirSync(configDir, { recursive: true })
  mkdirSync(project)
  try {
    writeFileSync(join(configDir, '.env'), 'DISCORD_STREAMING_BOT_TOKEN=user-token\n')
    assert.deepEqual(discordEnvironment(project, true, { HOME: home }), { DISCORD_STREAMING_ENABLED: 'true', DISCORD_STREAMING_BOT_TOKEN: 'user-token' })
    writeFileSync(join(project, '.env'), 'DISCORD_STREAMING_BOT_TOKEN=project-token\n')
    assert.deepEqual(discordEnvironment(project, true, { HOME: home }), { DISCORD_STREAMING_ENABLED: 'true', DISCORD_STREAMING_BOT_TOKEN: 'project-token' })
    assert.deepEqual(discordEnvironment(project, true, {
      HOME: home, DISCORD_STREAMING_BOT_TOKEN: 'process-token',
    }), { DISCORD_STREAMING_ENABLED: 'true', DISCORD_STREAMING_BOT_TOKEN: 'process-token' })
    assert.deepEqual(discordEnvironment(missing, true, { HOME: home }), { DISCORD_STREAMING_ENABLED: 'true', DISCORD_STREAMING_BOT_TOKEN: 'user-token' })
  } finally {
    rmSync(root, { recursive: true, force: true })
  }
})
