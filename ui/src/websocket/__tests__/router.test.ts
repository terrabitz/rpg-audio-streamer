import { afterEach, expect, it, vi } from 'vitest'
import { EventHub, parseMessage } from '../router'
import type { EventArgs } from '../router'

afterEach(() => vi.restoreAllMocks())

it('routes only the selected event, with independent and idempotent subscriptions', () => {
  const router = new EventHub()
  const removed = vi.fn()
  const deleted = vi.fn()
  const first = router.subscriptions.onTrackRemoved(removed)
  const second = router.subscriptions.onTrackRemoved(removed)
  router.subscriptions.onTableDeleted(deleted)
  const message = parseMessage({ event: 'trackRemoved', senderId: 'server', payload: { fileID: 'rain' } })
  router.dispatch(message)
  expect(removed).toHaveBeenCalledTimes(2)
  expect(removed).toHaveBeenLastCalledWith({ fileID: 'rain' }, message)
  expect(deleted).not.toHaveBeenCalled()
  first()
  router.dispatch(message)
  expect(removed).toHaveBeenCalledTimes(3)
  second()
  const third = router.subscriptions.onTrackRemoved(removed)
  first()
  second()
  router.dispatch(message)
  expect(removed).toHaveBeenCalledTimes(4)
  third()
  router.dispatch(message)
  expect(removed).toHaveBeenCalledTimes(4)
  router.dispatch(parseMessage({ event: 'pong', payload: null }))
})

it('isolates throwing and rejecting subscribers from other handlers', async () => {
  const error = vi.spyOn(console, 'error').mockImplementation(() => {})
  const router = new EventHub()
  router.subscriptions.onTableDeleted(() => { throw new Error('sync failure') })
  router.subscriptions.onTableDeleted(async () => { throw new Error('async failure') })
  const healthy = vi.fn()
  router.subscriptions.onTableDeleted(healthy)
  router.dispatch(parseMessage({ event: 'tableDeleted', payload: {} }))
  expect(healthy).toHaveBeenCalledOnce()
  await Promise.resolve()
  expect(error).toHaveBeenCalledTimes(2)
})

it.each([
  null,
  [],
  { method: 'tableDeleted', payload: {} },
  { event: 3, payload: {} },
  { event: 'tableDeleted' },
  { event: 'trackRemoved', senderId: 3, payload: { fileID: 'rain' } },
])('rejects an invalid message envelope %j', value => {
  expect(() => parseMessage(value)).toThrow()
})

it('keeps event names coupled to their payloads at compile time', () => {
  const valid: EventArgs = ['trackPlaybackChanged', { fileID: 'rain', volume: 0 }]
  // @ts-expect-error trackPlaybackChanged requires a track identifier.
  const missingID: EventArgs = ['trackPlaybackChanged', { volume: 0 }]
  // @ts-expect-error a event cannot use another event's payload.
  const wrongPayload: EventArgs = ['mixVolumesChanged', { fileID: 'rain' }]
  expect([valid, missingID, wrongPayload]).toHaveLength(3)
})
