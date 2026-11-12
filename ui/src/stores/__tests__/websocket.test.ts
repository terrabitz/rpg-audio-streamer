import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import { useWebSocketStore } from '../websocket'

beforeEach(() => {
  setActivePinia(createPinia())
  vi.useFakeTimers()
  vi.stubGlobal('WebSocket', TestSocket)
  vi.stubEnv('VITE_API_BASE_URL', '/api/v1')
  vi.spyOn(console, 'log').mockImplementation(() => {})
  vi.spyOn(console, 'error').mockImplementation(() => {})
  TestSocket.instances = []
})

afterEach(() => {
  vi.clearAllTimers()
  vi.useRealTimers()
  vi.unstubAllGlobals()
  vi.unstubAllEnvs()
  vi.restoreAllMocks()
})

describe('WebSocket transport', () => {
  it('connects once, sends structured messages, and disconnects cleanly', async () => {
    const ws = useWebSocketStore()
    await openSocket(ws)
    expect(ws.isConnected).toBe(true)
    expect(latestSocket().url).toBe(`ws://${window.location.host}/api/v1/ws`)
    await ws.connect()
    expect(TestSocket.instances).toHaveLength(1)
    ws.sendMessage('ping', { nonce: 42 })
    expect(latestSocket().sent).toEqual([
      JSON.stringify({ method: 'ping', payload: { nonce: 42 } }),
    ])
    expect(ws.messageHistory).toEqual([
      expect.objectContaining({
        method: 'ping',
        payload: { nonce: 42 },
        direction: 'sent',
        timestamp: expect.any(Number),
      }),
    ])
    ws.disconnect()
    expect(ws.isConnected).toBe(false)
    expect(latestSocket().close).toHaveBeenCalledWith(1000, 'Client disconnected')
    ws.disconnect()
    expect(() => ws.sendMessage('ping', {})).toThrow('WebSocket is not connected')
  })

  it('uses secure sockets and encodes invite tokens', async () => {
    vi.stubEnv('VITE_API_BASE_URL', 'https://table.example/api/v1')
    const ws = useWebSocketStore()
    await openSocket(ws, 'invite /?')
    expect(latestSocket().url).toBe('wss://table.example/api/v1/ws?token=invite%20%2F%3F')
    ws.disconnect()
  })

  it('dispatches incoming messages, unregisters handlers, and clears history', async () => {
    const ws = useWebSocketStore()
    const first = vi.fn()
    const second = vi.fn()
    ws.addMessageHandler(first)
    ws.addMessageHandler(second)
    await openSocket(ws)
    const message = { method: 'syncTrack', senderId: 'gm', payload: { fileID: 'rain' } }
    latestSocket().onmessage?.(new MessageEvent('message', { data: JSON.stringify(message) }))
    expect(first).toHaveBeenCalledWith(message)
    expect(second).toHaveBeenCalledWith(message)
    expect(ws.messageHistory[0]).toMatchObject({ ...message, direction: 'received' })
    ws.removeMessageHandler(first)
    ws.removeMessageHandler(first)
    latestSocket().onmessage?.(new MessageEvent('message', { data: JSON.stringify(message) }))
    expect(first).toHaveBeenCalledTimes(1)
    expect(second).toHaveBeenCalledTimes(2)
    latestSocket().onmessage?.(new MessageEvent('message', { data: 'invalid JSON' }))
    expect(second).toHaveBeenCalledTimes(2)
    expect(ws.messageHistory).toHaveLength(2)
    ws.clearMessageHistory()
    expect(ws.messageHistory).toEqual([])
    ws.disconnect()
  })

  it('rejects connection failures and cannot send before connecting', async () => {
    const ws = useWebSocketStore()
    expect(() => ws.sendMessage('ping', {})).toThrow('WebSocket is not connected')
    const pending = ws.connect()
    const error = new Event('error')
    latestSocket().onerror?.(error)
    await expect(pending).rejects.toBe(error)
    expect(ws.isConnected).toBe(false)
  })

  it('reconnects abnormal closures after the retry interval with the same invite', async () => {
    const ws = useWebSocketStore()
    await openSocket(ws, 'invite')
    latestSocket().readyState = 3
    latestSocket().onclose?.(new CloseEvent('close', { code: 1006, wasClean: false }))
    expect(ws.isConnected).toBe(false)
    await vi.advanceTimersByTimeAsync(2999)
    expect(TestSocket.instances).toHaveLength(1)
    vi.advanceTimersByTime(1)
    expect(TestSocket.instances).toHaveLength(2)
    expect(latestSocket().url).toContain('?token=invite')
    latestSocket().open()
    await Promise.resolve()
    await Promise.resolve()
    expect(ws.isConnected).toBe(true)
    ws.disconnect()
    await vi.advanceTimersByTimeAsync(3000)
    expect(TestSocket.instances).toHaveLength(2)
  })
})

async function openSocket(ws: ReturnType<typeof useWebSocketStore>, token?: string) {
  const pending = ws.connect(token)
  latestSocket().open()
  await pending
}

function latestSocket() {
  const socket = TestSocket.instances.at(-1)
  if (!socket) throw new Error('No socket was created')
  return socket
}

class TestSocket {
  static OPEN = 1
  static instances: TestSocket[] = []
  readyState = 0
  sent: string[] = []
  onopen: (() => void) | null = null
  onmessage: ((event: MessageEvent) => void) | null = null
  onclose: ((event: CloseEvent) => void) | null = null
  onerror: ((event: Event) => void) | null = null

  constructor(public url: string) {
    TestSocket.instances.push(this)
  }

  open() {
    this.readyState = TestSocket.OPEN
    this.onopen?.()
  }

  send(message: string) {
    this.sent.push(message)
  }

  close = vi.fn((code: number, reason: string) => {
    this.readyState = 3
    this.onclose?.(new CloseEvent('close', { code, reason, wasClean: true }))
  })
}
