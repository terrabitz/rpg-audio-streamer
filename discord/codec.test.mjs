import assert from 'node:assert/strict'
import { test } from 'node:test'
import { Readable, Writable } from 'node:stream'
import { pipeline } from 'node:stream/promises'
import prism from 'prism-media'
import { PCMTransform } from './pcm.mjs'

test('the Discord Opus encoder preserves both tones from the stereo mix', async () => {
  const frames = 48000
  const input = Buffer.alloc(frames * 8)
  for (let i = 0; i < frames; i++) {
    input.writeFloatLE(0.25 * Math.sin(2 * Math.PI * 440 * i / 48000), i * 8)
    input.writeFloatLE(0.125 * Math.sin(2 * Math.PI * 880 * i / 48000), i * 8 + 4)
  }
  const output = []
  await pipeline(
    Readable.from([input]),
    new PCMTransform(),
    new prism.opus.Encoder({ rate: 48000, channels: 2, frameSize: 960 }),
    new prism.opus.Decoder({ rate: 48000, channels: 2, frameSize: 960 }),
    new Writable({ write(chunk, _, done) { output.push(chunk); done() } }),
  )
  const decoded = Buffer.concat(output)
  assert.equal(decoded.length, frames * 4)
  // Skip codec startup delay and allow lossy coding rather than comparing bytes.
  const left = tone(decoded.subarray(4800 * 4), 440, 0)
  const right = tone(decoded.subarray(4800 * 4), 880, 1)
  assert.ok(left > 0.2 && left < 0.3, `left tone amplitude ${left}`)
  assert.ok(right > 0.09 && right < 0.16, `right tone amplitude ${right}`)
})

function tone(pcm, frequency, channel) {
  let real = 0, imaginary = 0
  const frames = pcm.length / 4
  for (let i = 0; i < frames; i++) {
    const sample = pcm.readInt16LE(i * 4 + channel * 2) / 32768
    const angle = 2 * Math.PI * frequency * i / 48000
    real += sample * Math.cos(angle)
    imaginary += sample * Math.sin(angle)
  }
  return 2 * Math.hypot(real, imaginary) / frames
}
