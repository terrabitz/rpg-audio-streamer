import assert from 'node:assert/strict'
import { test } from 'node:test'
import { PassThrough } from 'node:stream'
import { setTimeout as delay } from 'node:timers/promises'
import { AudioPlayerStatus, createAudioResource, StreamType } from '@discordjs/voice'
import { createMixPlayer } from './player.mjs'

test('voice player survives a short input gap and connection transition', async () => {
  const player = createMixPlayer()
  const stream = new PassThrough({ objectMode: true })
  const packets = []
  const connection = {
    state: { status: 'ready' },
    prepareAudioPacket(packet) { packets.push(packet) },
    dispatchAudio() {},
    setSpeaking() {},
  }
  player.subscribe(connection)
  try {
    player.play(createAudioResource(stream, { inputType: StreamType.Opus }))
    stream.write(Buffer.from([1]))
    await waitFor(() => packets.some(packet => packet[0] === 1))
    await delay(250)
    stream.write(Buffer.from([2]))
    await waitFor(() => packets.some(packet => packet[0] === 2))
    connection.state.status = 'connecting'
    await waitFor(() => player.state.status === AudioPlayerStatus.AutoPaused)
    connection.state.status = 'ready'
    stream.write(Buffer.from([3]))
    await waitFor(() => packets.some(packet => packet[0] === 3))
  } finally {
    player.stop(true)
    stream.destroy()
  }
})

async function waitFor(condition) {
  const deadline = Date.now() + 3000
  while (!condition()) {
    assert.ok(Date.now() < deadline, 'audio player did not recover')
    await delay(10)
  }
}
