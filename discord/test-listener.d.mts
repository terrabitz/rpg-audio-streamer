export interface ListenerAudio {
  low: number
  high: number
  rms: number
  frames: number
  last: number
  present: boolean
  error: string
}

export function startTestListener(config: {
  guildID: string, channelID: string, streamerID: string, listenerToken: string
}): Promise<{ latest(): ListenerAudio, close(): void }>
