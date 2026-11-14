import { Client, GatewayIntentBits } from 'discord.js'
import { EndBehaviorType, entersState, joinVoiceChannel, VoiceConnectionStatus } from '@discordjs/voice'
import prism from 'prism-media'

const sampleRate = 48000
const windowBytes = sampleRate * 4

export async function startTestListener({ guildID, channelID, streamerID, listenerToken }) {
  const client = new Client({ intents: [GatewayIntentBits.Guilds, GatewayIntentBits.GuildVoiceStates] })
  let connection
  let input
  let decoder
  let closed = false
  let phase = 'login'
  const states = []
  let pending = Buffer.alloc(0)
  let consecutiveDecodeErrors = 0
  let audio = { low: 0, high: 0, rms: 0, frames: 0, last: 0, present: false, error: '' }
  const fail = source => () => { audio = { ...audio, error: `Discord ${source} failed` } }
  const close = () => {
    closed = true
    input?.destroy()
    decoder?.destroy()
    if (connection && connection.state.status !== VoiceConnectionStatus.Destroyed) connection.destroy()
    client.destroy()
  }
  client.on('error', fail('client'))
  client.on('voiceStateUpdate', (_, state) => {
    if (state.id === streamerID && state.guild.id === guildID) {
      audio = { ...audio, present: state.channelId === channelID }
    }
  })
  try {
    await client.login(listenerToken)
    phase = 'server discovery'
    const guild = await client.guilds.fetch(guildID)
    await guild.members.fetchMe()
    phase = 'voice connection'
    connection = joinVoiceChannel({
      guildId: guildID,
      channelId: channelID,
      adapterCreator: guild.voiceAdapterCreator,
      selfDeaf: false,
      selfMute: true,
      daveEncryption: true,
    })
    connection.on('error', fail('voice connection'))
    connection.on('stateChange', (_, state) => { states.push(state.status) })
    await entersState(connection, VoiceConnectionStatus.Ready, 20_000)
    phase = 'audio subscription'
    audio.present = guild.voiceStates.cache.get(streamerID)?.channelId === channelID
    input = connection.receiver.subscribe(streamerID, { end: { behavior: EndBehaviorType.Manual } })
    input.on('error', fail('audio receiver'))
    input.on('data', () => { audio.last = Date.now() })
    const decodeNextPacket = () => {
      decoder = new prism.opus.Decoder({ rate: sampleRate, channels: 2, frameSize: 960 })
      decoder.on('data', chunk => {
        consecutiveDecodeErrors = 0
        pending = Buffer.concat([pending, chunk])
        while (pending.length >= windowBytes) {
          const signal = analyzeAudio(pending.subarray(0, windowBytes))
          pending = pending.subarray(windowBytes)
          audio = { ...audio, ...signal, frames: audio.frames + sampleRate }
        }
      })
      decoder.on('error', () => {
        input.unpipe(decoder)
        decoder.destroy()
        consecutiveDecodeErrors++
        if (consecutiveDecodeErrors >= 100) {
          audio = { ...audio, error: 'Repeated Discord Opus decode failures' }
          return
        }
        if (!closed) decodeNextPacket()
      })
      input.pipe(decoder)
    }
    decodeNextPacket()
    return { latest: () => ({ ...audio }), close }
  } catch {
    close()
    throw new Error(`Unable to start Discord listener during ${phase} (${states.join(', ')}); check bot installation, permissions, and network`)
  }
}

export function analyzeAudio(pcm) {
  const frames = pcm.length / 4
  let sum = 0
  const real = [0, 0]
  const imaginary = [0, 0]
  for (let i = 0; i < frames; i++) {
    const sample = (pcm.readInt16LE(i * 4) + pcm.readInt16LE(i * 4 + 2)) / 65536
    sum += sample * sample
    for (const [j, frequency] of [440, 880].entries()) {
      const angle = 2 * Math.PI * frequency * i / sampleRate
      real[j] += sample * Math.cos(angle)
      imaginary[j] += sample * Math.sin(angle)
    }
  }
  return {
    low: 2 * Math.hypot(real[0], imaginary[0]) / frames,
    high: 2 * Math.hypot(real[1], imaginary[1]) / frames,
    rms: Math.sqrt(sum / frames),
  }
}
