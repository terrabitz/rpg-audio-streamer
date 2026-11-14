import assert from 'node:assert/strict'
import { mkdirSync, mkdtempSync, writeFileSync, rmSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { join } from 'node:path'
import { test } from 'node:test'
import { discordEnvironment, discordTestConfig, discordTestDestination } from './discord-env.mjs'

test('ordinary development and browser tests never enable Discord from ambient credentials', () => {
  assert.deepEqual(discordEnvironment('/does-not-exist', false, {
    HOME: '/does-not-exist', DISCORD_BOT_TOKEN: 'test-only',
  }), {})
})

test('explicit Discord runs map test credentials and fail clearly on missing values', () => {
  assert.deepEqual(discordEnvironment('/does-not-exist', true, {
    HOME: '/does-not-exist',
    INTEGRATION_TEST_DISCORD_BOT_STREAMER_TOKEN: 'test-only',
    INTEGRATION_TEST_DISCORD_SERVER_ID: '1',
    INTEGRATION_TEST_DISCORD_VOICE_CHANNEL_ID: '2',
  }), { DISCORD_BOT_TOKEN: 'test-only' })
  assert.deepEqual(discordEnvironment('/does-not-exist', true, {
    HOME: '/does-not-exist', DISCORD_BOT_TOKEN: 'test-only',
  }), {
    DISCORD_BOT_TOKEN: 'test-only',
  })
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
    writeFileSync(join(configDir, '.env'), 'DISCORD_BOT_TOKEN=user-token\n')
    assert.deepEqual(discordEnvironment(project, true, { HOME: home }), { DISCORD_BOT_TOKEN: 'user-token' })
    writeFileSync(join(project, '.env'), 'DISCORD_BOT_TOKEN=project-token\n')
    assert.deepEqual(discordEnvironment(project, true, { HOME: home }), { DISCORD_BOT_TOKEN: 'project-token' })
    assert.deepEqual(discordEnvironment(project, true, {
      HOME: home, DISCORD_BOT_TOKEN: 'process-token',
    }), { DISCORD_BOT_TOKEN: 'process-token' })
    assert.deepEqual(discordEnvironment(missing, true, { HOME: home }), { DISCORD_BOT_TOKEN: 'user-token' })
  } finally {
    rmSync(root, { recursive: true, force: true })
  }
})

test('live browser fixtures explicitly select the isolated test destination', () => {
  const environment = {
    HOME: '/does-not-exist',
    INTEGRATION_TEST_DISCORD_SERVER_ID: '1',
    INTEGRATION_TEST_DISCORD_VOICE_CHANNEL_ID: '2',
    INTEGRATION_TEST_DISCORD_BOT_STREAMER_CLIENT_ID: '3',
    INTEGRATION_TEST_DISCORD_BOT_STREAMER_TOKEN: 'streamer',
    INTEGRATION_TEST_DISCORD_BOT_LISTENER_TOKEN: 'listener',
  }
  assert.deepEqual(discordTestDestination('/does-not-exist', environment), { guildID: '1', channelID: '2' })
  assert.throws(() => discordTestConfig('/does-not-exist', { HOME: '/does-not-exist' }), /Missing Discord configuration/)
  assert.throws(() => discordTestConfig('/does-not-exist', {
    ...environment, INTEGRATION_TEST_DISCORD_BOT_LISTENER_TOKEN: 'streamer',
  }), /distinct/)
})

test('live configuration merges local and explicit files with environment precedence', () => {
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
