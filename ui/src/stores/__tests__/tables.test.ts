import { beforeEach, describe, expect, it, vi } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import { useTableStore } from '../tables'

const api = vi.hoisted(() => ({
  getApiV1Tables: vi.fn(),
  postApiV1Tables: vi.fn(),
  putApiV1TablesByTableId: vi.fn(),
  deleteApiV1TablesByTableId: vi.fn(),
  getApiV1TablesByTableId: vi.fn(),
  getApiV1TablesByTableIdSettings: vi.fn(),
  patchApiV1TablesByTableIdSettings: vi.fn(),
  getApiV1TablesCurrent: vi.fn(),
  getApiV1TablesByTableIdTracks: vi.fn(),
  putApiV1TablesByTableIdTracksByTrackId: vi.fn(),
  deleteApiV1TablesByTableIdTracksByTrackId: vi.fn(),
}))
vi.mock('@/client/apiClient', () => api)

const friday = { id: 'friday', name: 'Friday', inviteToken: 'friday-invite' }
const sunday = { id: 'sunday', name: 'Sunday', inviteToken: 'sunday-invite' }
const track = {
  id: 'forest', name: 'Forest', typeID: 'ambiance',
  path: '/uploads/forest', createdAt: '2026-01-01T00:00:00Z',
}

beforeEach(() => {
  setActivePinia(createPinia())
  vi.resetAllMocks()
  api.patchApiV1TablesByTableIdSettings.mockResolvedValue({})
})

describe('tables', () => {
  it('loads server ordering, serializes writes, and retries unsaved settings', async () => {
    const tables = useTableStore()
    tables.current = friday
    const settings = { masterVolume: 0, typeVolumes: { Music: 0 }, trackVolumes: {}, trackOrder: ['rain', 'forest'] }
    api.getApiV1TablesByTableIdSettings.mockResolvedValue({ data: settings })
    expect(await tables.loadSettings(friday.id)).toEqual(settings)
    expect(tables.trackOrders.friday).toEqual(['rain', 'forest'])
    let finish!: () => void
    api.patchApiV1TablesByTableIdSettings.mockImplementationOnce(() => new Promise(resolve => {
      finish = () => resolve({})
    }))
    const first = tables.saveSettings(friday.id, { masterVolume: 0 })
    const next = tables.saveSettings(friday.id, { masterVolume: 60 })
    await Promise.resolve()
    expect(api.patchApiV1TablesByTableIdSettings).toHaveBeenCalledOnce()
    finish()
    await Promise.all([first, next])
    expect(api.patchApiV1TablesByTableIdSettings).toHaveBeenLastCalledWith({
      path: { tableID: friday.id }, body: { masterVolume: 60, typeVolumes: {}, trackVolumes: {} }, keepalive: true,
    })
    vi.spyOn(console, 'error').mockImplementation(() => {})
    try {
      api.patchApiV1TablesByTableIdSettings.mockRejectedValueOnce(new Error('offline'))
      await tables.saveSettings(friday.id, { masterVolume: 0, typeVolumes: { Music: 0 }, trackOrder: ['forest'] })
      expect(tables.settingsError).toContain('not saved')
      tables.clearCurrent()
      tables.current = friday
      expect(await tables.loadSettings(friday.id)).toEqual({ ...settings, trackOrder: ['forest'] })
      expect(tables.settingsError).toContain('not saved')
      await tables.saveSettings(friday.id, { trackVolumes: { forest: 35 } })
      expect(api.patchApiV1TablesByTableIdSettings).toHaveBeenLastCalledWith({
        path: { tableID: friday.id },
        body: { masterVolume: 0, typeVolumes: { Music: 0 }, trackVolumes: { forest: 35 }, trackOrder: ['forest'] },
        keepalive: true,
      })
      expect(tables.settingsError).toBe('')
      expect(tables.failedSettings).toEqual({})
    } finally {
      vi.restoreAllMocks()
    }
    await tables.loadSettings(sunday.id)
    expect(tables.trackOrders.sunday).toBeUndefined()
  })

  it('moves tracks in either direction and keeps ordering scoped to each table', () => {
    const tables = useTableStore()
    const rain = { ...track, id: 'rain', name: 'Rain' }
    const tavern = { ...track, id: 'tavern', name: 'Tavern' }
    tables.current = friday
    tables.tracks = [track, rain, tavern]
    tables.moveTrack('forest', 'tavern')
    expect(tables.orderedTracks).toEqual([rain, tavern, track])
    tables.moveTrack('forest', 'rain')
    expect(tables.orderedTracks).toEqual([track, rain, tavern])
    tables.moveTrack('forest', 'forest')
    tables.moveTrack('missing', 'rain')
    tables.moveTrack('rain', 'missing')
    expect(tables.orderedTracks).toEqual([track, rain, tavern])
    tables.moveTrack('forest', 'tavern')
    tables.current = sunday
    expect(tables.orderedTracks).toEqual([track, rain, tavern])
    tables.moveTrack('rain', 'forest')
    expect(tables.orderedTracks).toEqual([rain, track, tavern])
    tables.current = friday
    expect(tables.orderedTracks).toEqual([rain, tavern, track])
    tables.clearCurrent()
    tables.moveTrack('forest', 'rain')
    expect(tables.orderedTracks).toEqual([])
    expect(tables.trackOrders.friday).toEqual(['rain', 'tavern', 'forest'])
  })

  it('preserves order on refresh, updates metadata, appends new tracks, and drops removed ones', async () => {
    const tables = useTableStore()
    const rain = { ...track, id: 'rain', name: 'Rain' }
    const tavern = { ...track, id: 'tavern', name: 'Tavern' }
    tables.current = friday
    tables.trackOrders.friday = ['rain', 'forest', 'deleted']
    const renamed = { ...track, name: 'New Forest' }
    api.getApiV1TablesByTableIdTracks.mockResolvedValue({ data: [renamed, tavern, rain] })
    await tables.fetchTracks(friday.id)
    expect(tables.orderedTracks).toEqual([rain, renamed, tavern])
    expect(tables.trackOrders.friday).toEqual(['rain', 'forest', 'tavern'])
    api.deleteApiV1TablesByTableIdTracksByTrackId.mockResolvedValue({})
    await tables.removeTrack(friday.id, 'rain')
    expect(tables.orderedTracks).toEqual([renamed, tavern])
    expect(tables.trackOrders.friday).toEqual(['forest', 'tavern'])
    api.getApiV1TablesByTableIdTracks.mockResolvedValue({ data: [renamed, rain, tavern] })
    await tables.fetchTracks(friday.id)
    expect(tables.orderedTracks).toEqual([renamed, tavern, rain])
    api.deleteApiV1TablesByTableId.mockResolvedValue({})
    await tables.deleteTable(friday.id)
    expect(tables.trackOrders.friday).toBeUndefined()
  })

  it('lists, creates, renames, and deletes tables without changing unrelated tables', async () => {
    const tables = useTableStore()
    api.getApiV1Tables.mockResolvedValue({ data: [friday] })
    await tables.fetchTables()
    expect(tables.tables).toEqual([friday])
    api.postApiV1Tables.mockResolvedValue({ data: sunday })
    expect(await tables.createTable('Sunday')).toEqual(sunday)
    expect(api.postApiV1Tables).toHaveBeenCalledWith({ body: { name: 'Sunday' } })
    api.putApiV1TablesByTableId.mockResolvedValue({ data: { ...friday, name: 'Friday group' } })
    tables.current = friday
    await tables.renameTable(friday.id, 'Friday group')
    expect(tables.tables.map(table => table.name)).toEqual(['Friday group', 'Sunday'])
    expect(tables.current?.name).toBe('Friday group')
    api.putApiV1TablesByTableId.mockResolvedValue({ data: { ...sunday, name: 'Sunday group' } })
    await tables.renameTable(sunday.id, 'Sunday group')
    expect(tables.current?.id).toBe(friday.id)
    api.deleteApiV1TablesByTableId.mockResolvedValue({})
    await tables.deleteTable(sunday.id)
    expect(tables.current?.id).toBe(friday.id)
    tables.tracks = [track]
    await tables.deleteTable(friday.id)
    expect(tables.tables).toEqual([])
    expect(tables.current).toBeNull()
    expect(tables.tracks).toEqual([])
  })

  it('resolves table IDs and invite links, clearing previous table tracks', async () => {
    const tables = useTableStore()
    tables.tracks = [track]
    api.getApiV1TablesByTableId.mockResolvedValue({ data: friday })
    await tables.loadTable(friday.id)
    expect(api.getApiV1TablesByTableId).toHaveBeenCalledWith({ path: { tableID: friday.id } })
    expect(tables.current).toEqual(friday)
    expect(tables.tracks).toEqual([])
    api.getApiV1TablesCurrent.mockResolvedValue({ data: sunday })
    await tables.loadTable(undefined, 'sunday-invite')
    expect(api.getApiV1TablesCurrent).toHaveBeenLastCalledWith({
      headers: { Authorization: 'Bearer sunday-invite' },
    })
    expect(tables.current).toEqual(sunday)
    await tables.loadTable()
    expect(api.getApiV1TablesCurrent).toHaveBeenLastCalledWith({ headers: undefined })
  })

  it('associates and removes tracks without deleting them from the library', async () => {
    const tables = useTableStore()
    tables.current = friday
    api.getApiV1TablesByTableIdTracks.mockResolvedValue({ data: [track] })
    await tables.fetchTracks(friday.id)
    expect(tables.tracks).toEqual([track])
    api.putApiV1TablesByTableIdTracksByTrackId.mockResolvedValue({})
    await tables.addTrack(friday.id, track.id)
    expect(api.putApiV1TablesByTableIdTracksByTrackId).toHaveBeenCalledWith({
      path: { tableID: friday.id, trackID: track.id },
    })
    tables.tracks.push({ ...track, id: 'rain' })
    api.deleteApiV1TablesByTableIdTracksByTrackId.mockResolvedValue({})
    await tables.removeTrack(friday.id, track.id)
    expect(tables.tracks.map(track => track.id)).toEqual(['rain'])
  })

  it('propagates failed writes without mutating table state', async () => {
    const tables = useTableStore()
    tables.tables = [friday]
    tables.current = friday
    tables.tracks = [track]
    const failure = new Error('offline')
    api.postApiV1Tables.mockRejectedValue(failure)
    api.putApiV1TablesByTableId.mockRejectedValue(failure)
    api.deleteApiV1TablesByTableId.mockRejectedValue(failure)
    api.putApiV1TablesByTableIdTracksByTrackId.mockRejectedValue(failure)
    api.deleteApiV1TablesByTableIdTracksByTrackId.mockRejectedValue(failure)
    await expect(tables.createTable('Sunday')).rejects.toThrow('offline')
    await expect(tables.renameTable(friday.id, 'Changed')).rejects.toThrow('offline')
    await expect(tables.deleteTable(friday.id)).rejects.toThrow('offline')
    await expect(tables.addTrack(friday.id, 'rain')).rejects.toThrow('offline')
    await expect(tables.removeTrack(friday.id, track.id)).rejects.toThrow('offline')
    expect(tables.tables).toEqual([friday])
    expect(tables.current).toEqual(friday)
    expect(tables.tracks).toEqual([track])
  })

  it('does not restore a previous table after failed or superseded navigation', async () => {
    const tables = useTableStore()
    tables.current = friday
    tables.tracks = [track]
    api.getApiV1TablesByTableId.mockRejectedValueOnce(new Error('deleted'))
    await expect(tables.loadTable('deleted')).rejects.toThrow('deleted')
    expect(tables.current).toBeNull()
    expect(tables.tracks).toEqual([])
    let finish!: (value: { data: typeof friday }) => void
    api.getApiV1TablesByTableId.mockImplementationOnce(() => new Promise(resolve => { finish = resolve }))
    const previous = tables.loadTable(friday.id)
    api.getApiV1TablesByTableId.mockResolvedValueOnce({ data: sunday })
    await tables.loadTable(sunday.id)
    finish({ data: friday })
    await previous
    expect(tables.current).toEqual(sunday)
    api.getApiV1TablesByTableIdTracks.mockResolvedValue({ data: [track] })
    await tables.fetchTracks(friday.id)
    expect(tables.tracks).toEqual([])
    tables.clearCurrent()
    await tables.fetchTracks(friday.id)
    expect(tables.tracks).toEqual([])
  })
})
