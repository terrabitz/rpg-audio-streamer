import { afterEach, expect, it, vi } from 'vitest'
import { CommandRouter, parseMessage } from '../router'
import type { CommandArgs } from '../router'

afterEach(() => vi.restoreAllMocks())

it('routes only the selected command, with independent and idempotent subscriptions', () => {
  const router = new CommandRouter()
  const removed = vi.fn()
  const deleted = vi.fn()
  const first = router.on('removeTrack', removed)
  const second = router.on('removeTrack', removed)
  router.on('tableDeleted', deleted)
  const message = parseMessage({ method: 'removeTrack', senderId: 'server', payload: { fileID: 'rain' } })
  router.dispatch(message)
  expect(removed).toHaveBeenCalledTimes(2)
  expect(removed).toHaveBeenLastCalledWith({ fileID: 'rain' }, message)
  expect(deleted).not.toHaveBeenCalled()
  first()
  router.dispatch(message)
  expect(removed).toHaveBeenCalledTimes(3)
  second()
  const third = router.on('removeTrack', removed)
  first()
  second()
  router.dispatch(message)
  expect(removed).toHaveBeenCalledTimes(4)
  third()
  router.dispatch(message)
  expect(removed).toHaveBeenCalledTimes(4)
  router.dispatch(parseMessage({ method: 'pong', payload: null }))
})

it('isolates throwing and rejecting subscribers from other handlers', async () => {
  const error = vi.spyOn(console, 'error').mockImplementation(() => {})
  const router = new CommandRouter()
  router.on('tableDeleted', () => { throw new Error('sync failure') })
  router.on('tableDeleted', async () => { throw new Error('async failure') })
  const healthy = vi.fn()
  router.on('tableDeleted', healthy)
  router.dispatch(parseMessage({ method: 'tableDeleted', payload: {} }))
  expect(healthy).toHaveBeenCalledOnce()
  await Promise.resolve()
  expect(error).toHaveBeenCalledTimes(2)
})

it.each([
  null,
  [],
  { method: 3, payload: {} },
  { method: 'tableDeleted' },
  { method: 'removeTrack', senderId: 3, payload: { fileID: 'rain' } },
])('rejects an invalid message envelope %j', value => {
  expect(() => parseMessage(value)).toThrow()
})

it('keeps command names coupled to their payloads at compile time', () => {
  const valid: CommandArgs = ['syncTrack', { fileID: 'rain', volume: 0 }]
  // @ts-expect-error syncTrack requires a track identifier.
  const missingID: CommandArgs = ['syncTrack', { volume: 0 }]
  // @ts-expect-error a command cannot use another command's payload.
  const wrongPayload: CommandArgs = ['mixVolumes', { fileID: 'rain' }]
  expect([valid, missingID, wrongPayload]).toHaveLength(3)
})
