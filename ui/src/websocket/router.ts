import type { Payloads } from './protocol'

export interface MessageMetadata {
  senderId?: string
}

export interface Message extends MessageMetadata {
  method: string
  payload: unknown
}

export type CommandArgs = {
  [K in keyof Payloads]: [method: K, payload: Payloads[K]]
}[keyof Payloads]

type Handler<K extends keyof Payloads> =
  (payload: Payloads[K], metadata: MessageMetadata) => void | Promise<void>
type Subscription = (message: Message) => void | Promise<void>

export class CommandRouter {
  private handlers = new Map<string, Set<Subscription>>()

  on<K extends keyof Payloads>(command: K, handler: Handler<K>): () => void {
    let handlers = this.handlers.get(command)
    if (!handlers) {
      handlers = new Set()
      this.handlers.set(command, handlers)
    }
    const subscription: Subscription = message => handler(message.payload as Payloads[K], message)
    handlers.add(subscription)
    let subscribed = true
    return () => {
      if (!subscribed) return
      subscribed = false
      handlers.delete(subscription)
      if (handlers.size === 0) this.handlers.delete(command)
    }
  }

  dispatch(message: Message) {
    const handlers = this.handlers.get(message.method)
    if (!handlers) return
    for (const handler of [...handlers]) {
      try {
        void Promise.resolve(handler(message)).catch(error => {
          console.error(`WebSocket handler failed for ${message.method}:`, error)
        })
      } catch (error) {
        console.error(`WebSocket handler failed for ${message.method}:`, error)
      }
    }
  }
}

export function parseMessage(value: unknown): Message {
  if (value === null || typeof value !== 'object' ||
      !('method' in value) || typeof value.method !== 'string' || !('payload' in value)) {
    throw new Error('Invalid WebSocket message')
  }
  const senderId = 'senderId' in value ? value.senderId : undefined
  if (senderId !== undefined && typeof senderId !== 'string') {
    throw new Error('Invalid WebSocket senderId')
  }
  return { method: value.method, payload: value.payload, senderId }
}
