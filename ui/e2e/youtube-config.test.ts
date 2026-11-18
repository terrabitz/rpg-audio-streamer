import assert from 'node:assert/strict'
import { test } from 'node:test'
import { youtubeTestConfig } from './youtube-config.ts'

test('YouTube configuration requires an environment-supplied video URL', () => {
  for (const value of [undefined, '', ' ', 'not a URL', 'file:///video', 'https://example.com/video']) {
    assert.throws(() => youtubeTestConfig({ INTEGRATION_TEST_YT_DLP_LINK: value }), /INTEGRATION_TEST_YT_DLP_LINK/)
  }
  const link = 'https://www.youtube.com/watch?v=aaaaaaaaaaa'
  assert.deepEqual(youtubeTestConfig({ INTEGRATION_TEST_YT_DLP_LINK: ` ${link} ` }), { link })
})
