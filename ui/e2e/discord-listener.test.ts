import assert from 'node:assert/strict'
import { test } from 'node:test'
import { analyzeAudio, createPlaintextStream } from './discord-listener.ts'

test('listener rejects undecrypted DAVE frames and reports persistent failure', async () => {
  const packets: Buffer[] = []
  let accepted = 0
  let failures = 0
  const stream = createPlaintextStream(() => { accepted++ }, () => { failures++ })
  stream.on('data', packet => packets.push(packet))
  const encrypted = Buffer.concat([Buffer.alloc(10), Buffer.from([0xfa, 0xfa])])
  const opus = Buffer.from([0xf8, 0xff, 0xfe])
  stream.write(encrypted)
  stream.write(opus)
  for (let i = 0; i < 250; i++) stream.write(encrypted)
  stream.end()
  await new Promise<void>((resolve, reject) => {
    stream.on('end', resolve)
    stream.on('error', reject)
  })
  assert.deepEqual(packets, [opus])
  assert.equal(accepted, 1)
  assert.equal(failures, 1)
})

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

test('listener recognizes tones across packet discontinuities', () => {
  const pcm = Buffer.alloc(48000 * 4)
  for (let i = 0; i < 48000; i++) {
    const phase = 2 * Math.PI * Math.floor(i / 4800) / 10
    const sample = Math.round(5000 * Math.sin(2 * Math.PI * 440 * i / 48000 + phase))
    pcm.writeInt16LE(sample, i * 4)
    pcm.writeInt16LE(sample, i * 4 + 2)
  }
  const audio = analyzeAudio(pcm)
  assert.ok(audio.low > 0.15 && audio.low < 0.16)
  assert.ok(audio.high < 0.001)
})
