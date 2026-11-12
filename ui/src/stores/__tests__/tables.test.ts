import { beforeEach, describe, expect, it, vi } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import { useTableStore } from '../tables'

const api = vi.hoisted(() => ({
  getApiV1Tables: vi.fn(),
  postApiV1Tables: vi.fn(),
  putApiV1TablesByTableId: vi.fn(),
  deleteApiV1TablesByTableId: vi.fn(),
  getApiV1TablesByTableId: vi.fn(),
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
})

describe('tables', () => {
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
