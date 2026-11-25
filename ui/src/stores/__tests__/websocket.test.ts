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
  it('connects once and disconnects cleanly', async () => {
    const ws = useWebSocketStore()
    await openSocket(ws)
    expect(ws.isConnected).toBe(true)
    expect(latestSocket().url).toBe(`ws://${window.location.host}/api/v1/ws`)
    await ws.connect()
    expect(TestSocket.instances).toHaveLength(1)
    ws.disconnect()
    expect(ws.isConnected).toBe(false)
    expect(latestSocket().close).toHaveBeenCalledWith(1000, 'Client disconnected')
    ws.disconnect()
  })

  it('uses secure sockets and encodes the public table ID', async () => {
    vi.stubEnv('VITE_API_BASE_URL', 'https://table.example/api/v1')
    const ws = useWebSocketStore()
    await openSocket(ws, 'table /?')
    expect(latestSocket().url).toBe('wss://table.example/api/v1/ws?tableID=table%20%2F%3F')
    ws.disconnect()
  })

  it('dispatches incoming messages and unregisters handlers', async () => {
    const ws = useWebSocketStore()
    const first = vi.fn()
    const second = vi.fn()
    const unsubscribe = ws.onTrackRemoved(first)
    ws.onTrackRemoved(second)
    await openSocket(ws)
    const message = { event: 'trackRemoved', senderId: 'gm', payload: { fileID: 'rain' } }
    latestSocket().onmessage?.(new MessageEvent('message', { data: JSON.stringify(message) }))
    expect(first).toHaveBeenCalledWith(message.payload, message)
    expect(second).toHaveBeenCalledWith(message.payload, message)
    unsubscribe()
    unsubscribe()
    latestSocket().onmessage?.(new MessageEvent('message', { data: JSON.stringify(message) }))
    expect(first).toHaveBeenCalledTimes(1)
    expect(second).toHaveBeenCalledTimes(2)
    latestSocket().onmessage?.(new MessageEvent('message', { data: 'invalid JSON' }))
    expect(second).toHaveBeenCalledTimes(2)
    ws.disconnect()
  })

  it('rejects connection failures', async () => {
    const ws = useWebSocketStore()
    const pending = ws.connect()
    const error = new Event('error')
    latestSocket().onerror?.(error)
    await expect(pending).rejects.toBe(error)
    expect(ws.isConnected).toBe(false)
  })

  it('rejects malformed message envelopes', async () => {
    const ws = useWebSocketStore()
    const handler = vi.fn()
    ws.onTrackRemoved(handler)
    await openSocket(ws)
    for (const message of [
      { event: 'trackRemoved' },
      { event: 42, payload: {} },
      { event: 'trackRemoved', senderId: 42, payload: { fileID: 'rain' } },
    ]) {
      latestSocket().onmessage?.(new MessageEvent('message', { data: JSON.stringify(message) }))
    }
    expect(handler).not.toHaveBeenCalled()
    expect(console.error).toHaveBeenCalledTimes(3)
    ws.disconnect()
  })

  it('reconnects abnormal closures after the retry interval with the same table ID', async () => {
    const ws = useWebSocketStore()
    await openSocket(ws, 'friday')
    latestSocket().readyState = 3
    latestSocket().onclose?.(new CloseEvent('close', { code: 1006, wasClean: false }))
    expect(ws.isConnected).toBe(false)
    await vi.advanceTimersByTimeAsync(2999)
    expect(TestSocket.instances).toHaveLength(1)
    vi.advanceTimersByTime(1)
    expect(TestSocket.instances).toHaveLength(2)
    expect(latestSocket().url).toContain('?tableID=friday')
    latestSocket().open()
    await Promise.resolve()
    await Promise.resolve()
    expect(ws.isConnected).toBe(true)
    ws.disconnect()
    await vi.advanceTimersByTimeAsync(3000)
    expect(TestSocket.instances).toHaveLength(2)
  })

  it('changes table scope and ignores stale sockets and pending reconnects', async () => {
    const ws = useWebSocketStore()
    const first = ws.connect('friday')
    const previous = latestSocket()
    previous.open()
    await first
    expect(previous.url).toContain('?tableID=friday')
    previous.onclose?.(new CloseEvent('close', { code: 1006, wasClean: false }))
    const next = ws.connect('sunday')
    latestSocket().open()
    await next
    expect(latestSocket().url).toContain('?tableID=sunday')
    const handler = vi.fn()
    ws.onTrackRemoved(handler)
    previous.onmessage?.(new MessageEvent('message', { data: '{"event":"trackRemoved","payload":{"fileID":"rain"}}' }))
    previous.onerror?.(new Event('error'))
    previous.onclose?.(new CloseEvent('close', { code: 1006, wasClean: false }))
    expect(handler).not.toHaveBeenCalled()
    expect(ws.isConnected).toBe(true)
    await vi.advanceTimersByTimeAsync(3000)
    expect(TestSocket.instances).toHaveLength(2)
    ws.disconnect()
  })

  it('cancels a scheduled reconnect when leaving the table', async () => {
    const ws = useWebSocketStore()
    await openSocket(ws, 'friday')
    latestSocket().onclose?.(new CloseEvent('close', { code: 1006, wasClean: false }))
    ws.disconnect()
    await vi.advanceTimersByTimeAsync(5000)
    expect(TestSocket.instances).toHaveLength(1)
  })

  it('settles an unfinished connection when navigation disconnects it', async () => {
    const ws = useWebSocketStore()
    const pending = ws.connect('friday')
    ws.disconnect()
    await expect(pending).rejects.toThrow('Connection superseded')
    expect(ws.isConnected).toBe(false)
    await vi.advanceTimersByTimeAsync(5000)
    expect(TestSocket.instances).toHaveLength(1)
  })
})

async function openSocket(ws: ReturnType<typeof useWebSocketStore>, tableID?: string) {
  const pending = ws.connect(tableID)
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

  close = vi.fn((code: number, reason: string) => {
    this.readyState = 3
    this.onclose?.(new CloseEvent('close', { code, reason, wasClean: true }))
  })
}
