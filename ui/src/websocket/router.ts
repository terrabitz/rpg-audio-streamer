import type { Payloads } from './protocol'

export interface MessageMetadata {
  senderId?: string
}

export interface Message extends MessageMetadata {
  event: string
  payload: unknown
}

export type EventArgs = {
  [K in keyof Payloads]: [event: K, payload: Payloads[K]]
}[keyof Payloads]

type Handler<K extends keyof Payloads> =
  (payload: Payloads[K], metadata: MessageMetadata) => void | Promise<void>
type Subscription = (message: Message) => void | Promise<void>

export class EventHub {
  private handlers = new Map<string, Set<Subscription>>()

  readonly subscriptions = {
    onPing: (handler: Handler<'ping'>) => this.on('ping', handler),
    onPong: (handler: Handler<'pong'>) => this.on('pong', handler),
    onBroadcast: (handler: Handler<'broadcast'>) => this.on('broadcast', handler),
    onPlaybackSyncRequested: (handler: Handler<'playbackSyncRequested'>) => this.on('playbackSyncRequested', handler),
    onTrackPlaybackChanged: (handler: Handler<'trackPlaybackChanged'>) => this.on('trackPlaybackChanged', handler),
    onPlaybackStateChanged: (handler: Handler<'playbackStateChanged'>) => this.on('playbackStateChanged', handler),
    onTrackRemoved: (handler: Handler<'trackRemoved'>) => this.on('trackRemoved', handler),
    onTableDeleted: (handler: Handler<'tableDeleted'>) => this.on('tableDeleted', handler),
    onMixChanged: (handler: Handler<'mixChanged'>) => this.on('mixChanged', handler),
    onMixVolumesChanged: (handler: Handler<'mixVolumesChanged'>) => this.on('mixVolumesChanged', handler),
    onMixerReady: (handler: Handler<'mixerReady'>) => this.on('mixerReady', handler),
    onMixerApplied: (handler: Handler<'mixerApplied'>) => this.on('mixerApplied', handler),
    onMixerFailed: (handler: Handler<'mixerFailed'>) => this.on('mixerFailed', handler),
    onPlaybackFailed: (handler: Handler<'playbackFailed'>) => this.on('playbackFailed', handler),
    onTrackStatusChanged: (handler: Handler<'trackStatusChanged'>) => this.on('trackStatusChanged', handler),
    onDiscordStatusChanged: (handler: Handler<'discordStatusChanged'>) => this.on('discordStatusChanged', handler),
  }

  private on<K extends keyof Payloads>(event: K, handler: Handler<K>): () => void {
    let handlers = this.handlers.get(event)
    if (!handlers) {
      handlers = new Set()
      this.handlers.set(event, handlers)
    }
    const subscription: Subscription = message => handler(message.payload as Payloads[K], message)
    handlers.add(subscription)
    let subscribed = true
    return () => {
      if (!subscribed) return
      subscribed = false
      handlers.delete(subscription)
      if (handlers.size === 0) this.handlers.delete(event)
    }
  }

  dispatch(message: Message) {
    const handlers = this.handlers.get(message.event)
    if (!handlers) return
    for (const handler of [...handlers]) {
      try {
        void Promise.resolve(handler(message)).catch(error => {
          console.error(`WebSocket handler failed for ${message.event}:`, error)
        })
      } catch (error) {
        console.error(`WebSocket handler failed for ${message.event}:`, error)
      }
    }
  }
}

export function parseMessage(value: unknown): Message {
  if (value === null || typeof value !== 'object' ||
      !('event' in value) || typeof value.event !== 'string' || !('payload' in value)) {
    throw new Error('Invalid WebSocket message')
  }
  const senderId = 'senderId' in value ? value.senderId : undefined
  if (senderId !== undefined && typeof senderId !== 'string') {
    throw new Error('Invalid WebSocket senderId')
  }
  return { event: value.event, payload: value.payload, senderId }
}
