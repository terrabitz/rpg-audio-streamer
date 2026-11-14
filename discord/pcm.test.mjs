import assert from 'node:assert/strict'
import { test } from 'node:test'
import { Readable, Writable } from 'node:stream'
import { pipeline } from 'node:stream/promises'
import { PCMTransform } from './pcm.mjs'

test('converts stereo float PCM to signed 16-bit without losing split samples', async () => {
  const input = floats(0.5, -0.5, 1, -1, 0, 0)
  const output = await convert([input.subarray(0, 3), input.subarray(3, 9), input.subarray(9)])
  assert.deepEqual(
    Array.from({ length: 6 }, (_, i) => output.readInt16LE(i * 2)),
    [16384, -16384, 32767, -32768, 0, 0],
  )
})

test('rejects corrupt PCM and incomplete stereo frames instead of playing garbage', async () => {
  for (const input of [floats(NaN, 0), floats(Infinity, 0), floats(1.5, 0), floats(0)]) {
    await assert.rejects(convert([input]), /PCM/)
  }
})

function floats(...samples) {
  const data = Buffer.alloc(samples.length * 4)
  samples.forEach((sample, i) => data.writeFloatLE(sample, i * 4))
  return data
}

async function convert(chunks) {
  const output = []
  await pipeline(Readable.from(chunks), new PCMTransform(), new Writable({
    write(chunk, _, callback) { output.push(chunk); callback() },
  }))
  return Buffer.concat(output)
}
