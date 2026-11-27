import type { Payloads } from './protocol'

export interface MessageMetadata {
  senderId?: string
}

export interface Message extends MessageMetadata {
  event: string
  payload: unknown
}

type Handler<K extends keyof Payloads> =
  (payload: Payloads[K], metadata: MessageMetadata) => void | Promise<void>
type Subscription = (message: Message) => void | Promise<void>

export class EventHub {
  private handlers = new Map<string, Set<Subscription>>()

  readonly subscriptions = {
    onPlaybackStateChanged: (handler: Handler<'playbackStateChanged'>) => this.on('playbackStateChanged', handler),
    onTrackRemoved: (handler: Handler<'trackRemoved'>) => this.on('trackRemoved', handler),
    onTableDeleted: (handler: Handler<'tableDeleted'>) => this.on('tableDeleted', handler),
    onMixerReady: (handler: Handler<'mixerReady'>) => this.on('mixerReady', handler),
    onMixerApplied: (handler: Handler<'mixerApplied'>) => this.on('mixerApplied', handler),
    onMixerFailed: (handler: Handler<'mixerFailed'>) => this.on('mixerFailed', handler),
    onTrackStatusChanged: (handler: Handler<'trackStatusChanged'>) => this.on('trackStatusChanged', handler),
    onTrackUpdated: (handler: Handler<'trackUpdated'>) => this.on('trackUpdated', handler),
    onTagUpdated: (handler: Handler<'tagUpdated'>) => this.on('tagUpdated', handler),
    onTagDeleted: (handler: Handler<'tagDeleted'>) => this.on('tagDeleted', handler),
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
