import { createPinia, setActivePinia } from 'pinia'
import { expect, it, vi } from 'vitest'
import { usePlaybackCommands } from '../playbackCommands'

const api = vi.hoisted(() => ({ patch: vi.fn(), mix: vi.fn() }))
vi.mock('@/client/apiClient', () => ({
  patchTablePlayback: api.patch,
  putTableMix: api.mix,
}))

it('preserves same-table command order through errors without blocking another table', async () => {
  setActivePinia(createPinia())
  const commands = usePlaybackCommands()
  let rejectPlay!: (reason: Error) => void
  api.patch
    .mockImplementationOnce(() => new Promise((_, reject) => { rejectPlay = reject }))
    .mockResolvedValue({})
  api.mix.mockResolvedValue({})

  const play = commands.patch('friday', 'music', { isPlaying: true })
  const seek = commands.patch('friday', 'music', { currentTime: 0 })
  const mix = commands.replaceMix('friday', { tracks: [] })
  await commands.patch('sunday', 'rain', { isPlaying: false })
  expect(api.patch.mock.calls.map(([request]) => request.path.tableID)).toEqual(['friday', 'sunday'])
  expect(api.mix).not.toHaveBeenCalled()

  rejectPlay(new Error('offline'))
  await expect(play).rejects.toThrow('offline')
  await seek
  await mix
  expect(api.patch).toHaveBeenLastCalledWith({
    path: { tableID: 'friday', trackID: 'music' }, body: { currentTime: 0 },
  })
  expect(api.patch.mock.invocationCallOrder[2]).toBeLessThan(api.mix.mock.invocationCallOrder[0]!)
})
