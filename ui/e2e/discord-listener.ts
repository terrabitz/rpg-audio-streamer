import { Client, GatewayIntentBits } from 'discord.js'
import { EndBehaviorType, entersState, joinVoiceChannel, VoiceConnectionStatus, type VoiceConnection, type AudioReceiveStream } from '@discordjs/voice'
import { Transform } from 'node:stream'
import prism from 'prism-media'

const sampleRate = 48000
const windowBytes = sampleRate * 4

export interface ListenerAudio {
  low: number
  high: number
  rms: number
  frames: number
  last: number
  present: boolean
  error: string
}

export async function startTestListener({ guildID, channelID, streamerID, listenerToken }: {
  guildID: string, channelID: string, streamerID: string, listenerToken: string
}) {
  const client = new Client({ intents: [GatewayIntentBits.Guilds, GatewayIntentBits.GuildVoiceStates] })
  let connection: VoiceConnection | undefined
  let input: AudioReceiveStream | undefined
  let plaintext: Transform | undefined
  let decoder: prism.opus.Decoder | undefined
  let closed = false
  let phase = 'login'
  const states: string[] = []
  let pending = Buffer.alloc(0)
  let consecutiveDecodeErrors = 0
  let audio: ListenerAudio = { low: 0, high: 0, rms: 0, frames: 0, last: 0, present: false, error: '' }
  const fail = (source: string) => () => { audio = { ...audio, error: `Discord ${source} failed` } }
  const close = () => {
    closed = true
    input?.destroy()
    plaintext?.destroy()
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
      guildId: guildID, channelId: channelID, adapterCreator: guild.voiceAdapterCreator,
      selfDeaf: false, selfMute: true, daveEncryption: true,
    })
    connection.on('error', fail('voice connection'))
    connection.on('stateChange', (_, state) => { states.push(state.status) })
    await entersState(connection, VoiceConnectionStatus.Ready, 20_000)
    phase = 'audio subscription'
    audio.present = guild.voiceStates.cache.get(streamerID)?.channelId === channelID
    input = connection.receiver.subscribe(streamerID, { end: { behavior: EndBehaviorType.Manual } })
    input.on('error', fail('audio receiver'))
    plaintext = createPlaintextStream(
      () => { audio.last = Date.now() },
      () => { audio = { ...audio, error: 'Discord listener cannot decrypt voice audio' } },
    )
    input.pipe(plaintext)
    const decodeNextPacket = () => {
      const current = new prism.opus.Decoder({ rate: sampleRate, channels: 2, frameSize: 960 })
      decoder = current
      current.on('data', (chunk: Buffer) => {
        consecutiveDecodeErrors = 0
        pending = Buffer.concat([pending, chunk])
        while (pending.length >= windowBytes) {
          const signal = analyzeAudio(pending.subarray(0, windowBytes))
          pending = pending.subarray(windowBytes)
          audio = { ...audio, ...signal, frames: audio.frames + sampleRate }
        }
      })
      current.on('error', () => {
        plaintext!.unpipe(current)
        current.destroy()
        consecutiveDecodeErrors++
        if (consecutiveDecodeErrors >= 100) {
          audio = { ...audio, error: 'Repeated Discord Opus decode failures' }
          return
        }
        if (!closed) decodeNextPacket()
      })
      plaintext!.pipe(current)
    }
    decodeNextPacket()
    return { latest: () => ({ ...audio }), close }
  } catch {
    close()
    throw new Error(`Unable to start Discord listener during ${phase} (${states.join(', ')}); check bot installation, permissions, and network`)
  }
}

export function createPlaintextStream(onPacket: () => void, onFailure: () => void) {
  let encryptedPackets = 0
  return new Transform({
    objectMode: true,
    transform(packet: Buffer, _, callback) {
      // The voice receiver passes ciphertext through while DAVE is unready.
      if (packet.length >= 11 && packet[packet.length - 2] === 0xfa && packet[packet.length - 1] === 0xfa) {
        encryptedPackets++
        if (encryptedPackets === 250) onFailure()
        callback()
        return
      }
      encryptedPackets = 0
      onPacket()
      callback(null, packet)
    },
  })
}

export function analyzeAudio(pcm: Buffer) {
  const frames = pcm.length / 4
  let sum = 0
  const magnitude = [0, 0]
  // Packet loss can shift phase; measure integer-cycle windows independently.
  const chunkFrames = sampleRate / 10
  for (let start = 0; start < frames; start += chunkFrames) {
    const real = [0, 0]
    const imaginary = [0, 0]
    const count = Math.min(chunkFrames, frames - start)
    for (let i = 0; i < count; i++) {
      const offset = (start + i) * 4
      const sample = (pcm.readInt16LE(offset) + pcm.readInt16LE(offset + 2)) / 65536
      sum += sample * sample
      for (const [j, frequency] of [440, 880].entries()) {
        const angle = 2 * Math.PI * frequency * i / sampleRate
        real[j]! += sample * Math.cos(angle)
        imaginary[j]! += sample * Math.sin(angle)
      }
    }
    for (let j = 0; j < 2; j++) magnitude[j]! += 2 * Math.hypot(real[j]!, imaginary[j]!)
  }
  return {
    low: magnitude[0]! / frames,
    high: magnitude[1]! / frames,
    rms: Math.sqrt(sum / frames),
  }
}
