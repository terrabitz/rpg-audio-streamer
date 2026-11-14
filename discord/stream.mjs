import { createAudioPlayer, createAudioResource, NoSubscriberBehavior, StreamType, VoiceConnectionStatus } from '@discordjs/voice'
import { PCMTransform } from './pcm.mjs'
import { connectVoice } from './voice.mjs'

let voice
let player
let ending = false

for (const signal of ['SIGTERM', 'SIGINT']) process.on(signal, () => stop())

try {
  const token = process.env.DISCORD_BOT_TOKEN
  const guildID = process.env.DISCORD_GUILD_ID
  const channelID = process.env.DISCORD_CHANNEL_ID
  if (!token || !guildID || !channelID) throw new Error('Missing Discord configuration')
  voice = await connectVoice({ token, guildID, channelID, onError: fail })
  if (ending) voice.close()
  else start()
} catch {
  fail('Unable to join Discord voice; check bot token, channel permissions, and network access')
}

function start() {
  voice.connection.on('error', () => fail('Discord voice connection failed'))
  voice.connection.on('stateChange', (_, next) => {
    if (next.status === VoiceConnectionStatus.Disconnected) fail('Discord voice disconnected')
  })
  const pcm = new PCMTransform()
  pcm.on('error', () => fail('Invalid PCM received by Discord worker'))
  process.stdin.on('error', () => fail('Discord audio input failed'))
  player = createAudioPlayer({ behaviors: { noSubscriber: NoSubscriberBehavior.Stop } })
  player.on('error', () => fail('Discord audio encoding failed'))
  const resource = createAudioResource(pcm, { inputType: StreamType.Raw })
  voice.connection.subscribe(player)
  player.play(resource)
  process.stdin.pipe(pcm)
  process.stdin.on('end', () => stop())
  report({ state: 'connected' })
}

function fail(message) {
  if (ending) return
  report({ state: 'error', message })
  process.exitCode = 1
  stop()
}

function stop() {
  if (ending) return
  ending = true
  player?.stop(true)
  voice?.close()
  process.stdin.destroy()
}

function report(message) {
  process.stdout.write(`${JSON.stringify(message)}\n`)
}
