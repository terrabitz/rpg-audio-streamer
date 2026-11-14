import assert from 'node:assert/strict'
import { test } from 'node:test'
import { analyzeAudio } from './test-listener.mjs'

test('listener distinguishes two tones and silence in stereo PCM', () => {
  const pcm = Buffer.alloc(48000 * 4)
  for (let i = 0; i < 48000; i++) {
    const sample = Math.round(5000 * Math.sin(2 * Math.PI * 440 * i / 48000)
      + 2500 * Math.sin(2 * Math.PI * 880 * i / 48000))
    pcm.writeInt16LE(sample, i * 4)
    pcm.writeInt16LE(sample, i * 4 + 2)
  }
  const audio = analyzeAudio(pcm)
  assert.ok(audio.low > 0.15 && audio.low < 0.16)
  assert.ok(audio.high > 0.075 && audio.high < 0.08)
  assert.ok(audio.rms > 0.1)
  assert.deepEqual(analyzeAudio(Buffer.alloc(48000 * 4)), { low: 0, high: 0, rms: 0 })
})
