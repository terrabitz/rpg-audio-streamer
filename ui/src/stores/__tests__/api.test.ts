import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import { useAuthStore } from '../auth'
import { TrackUsageChangedError, useFileStore } from '../files'
import { useTrackTypeStore } from '../trackTypes'

const api = vi.hoisted(() => ({
  getApiV1AuthStatus: vi.fn(),
  postApiV1Login: vi.fn(),
  postApiV1AuthLogout: vi.fn(),
  getApiV1Files: vi.fn(),
  getApiV1FilesByTrackIdUsage: vi.fn(),
  getApiV1FilesByTrackIdWaveform: vi.fn(),
  postApiV1Files: vi.fn(),
  deleteApiV1FilesByTrackId: vi.fn(),
  putApiV1FilesByTrackId: vi.fn(),
  getApiV1TrackTypes: vi.fn(),
}))
vi.mock('@/client/apiClient', () => api)

const track = {
  id: 'forest',
  name: 'Forest',
  typeID: 'ambiance',
  path: '/uploads/forest',
  createdAt: '2026-01-01T00:00:00Z',
}
const trackType = {
  id: 'ambiance',
  name: 'Ambiance',
  color: '#B39DDB',
  isRepeating: true,
  allowSimultaneousPlay: true,
}

beforeEach(() => {
  setActivePinia(createPinia())
  vi.resetAllMocks()
  vi.spyOn(console, 'error').mockImplementation(() => {})
})
afterEach(() => vi.restoreAllMocks())

describe('authentication', () => {
  it('logs in, checks the cookie session, and logs out', async () => {
    api.postApiV1Login.mockResolvedValue({ data: { success: true } })
    api.getApiV1AuthStatus.mockResolvedValue({ data: { authenticated: true, role: 'gm' } })
    api.postApiV1AuthLogout.mockResolvedValue({})
    const auth = useAuthStore()
    const login = auth.login('gm', 'password')
    expect(auth.loading).toBe(true)
    await login
    expect(api.postApiV1Login).toHaveBeenCalledWith({
      body: { username: 'gm', password: 'password' },
    })
    expect(auth.authenticated).toBe(true)
    expect(auth.role).toBe('gm')
    expect(auth.loading).toBe(false)
    await auth.logout()
    expect(auth.authenticated).toBe(false)
    expect(auth.role).toBeNull()
  })

  it('clears unauthenticated or failed status', async () => {
    const auth = useAuthStore()
    api.getApiV1AuthStatus.mockResolvedValue({ data: { authenticated: false } })
    await auth.checkAuthStatus()
    expect(auth.authenticated).toBe(false)
    expect(auth.role).toBeNull()
    api.getApiV1AuthStatus.mockRejectedValue(new Error('offline'))
    await auth.checkAuthStatus()
    expect(auth.authenticated).toBe(false)
    expect(auth.loading).toBe(false)
  })

  it.each([undefined, 'Invalid credentials'])('rejects unsuccessful login (%s)', async (error) => {
    api.postApiV1Login.mockResolvedValue({ data: { success: false, error } })
    const auth = useAuthStore()
    await expect(auth.login('gm', 'wrong')).rejects.toThrow(error ?? 'Login failed')
    expect(auth.loading).toBe(false)
    expect(auth.authenticated).toBe(false)
  })

  it('propagates login and logout transport failures without erasing a valid session', async () => {
    const auth = useAuthStore()
    api.postApiV1Login.mockRejectedValue(new Error('offline'))
    await expect(auth.login('gm', 'password')).rejects.toThrow('offline')
    expect(auth.loading).toBe(false)
    auth.authenticated = true
    auth.role = 'gm'
    api.postApiV1AuthLogout.mockRejectedValue(new Error('offline'))
    await expect(auth.logout()).rejects.toThrow('offline')
    expect(auth.authenticated).toBe(true)
    expect(auth.role).toBe('gm')
  })
})

describe('file library', () => {
  it('fetches a current table count and rejects invalid or failed responses', async () => {
    const files = useFileStore()
    api.getApiV1FilesByTrackIdUsage.mockResolvedValue({ data: { tableCount: 2 } })
    expect(await files.countTrackTables('forest')).toBe(2)
    expect(api.getApiV1FilesByTrackIdUsage).toHaveBeenCalledWith({ path: { trackID: 'forest' } })
    for (const tableCount of [-1, 0.5, Number.NaN, Number.MAX_SAFE_INTEGER + 1, undefined]) {
      api.getApiV1FilesByTrackIdUsage.mockResolvedValueOnce({ data: { tableCount } })
      await expect(files.countTrackTables('forest')).rejects.toThrow('Invalid track usage count')
    }
    api.getApiV1FilesByTrackIdUsage.mockRejectedValue(new Error('offline'))
    await expect(files.countTrackTables('forest')).rejects.toThrow('offline')
  })

  it('fetches a track waveform and rejects malformed responses', async () => {
    const files = useFileStore()
    api.getApiV1FilesByTrackIdWaveform.mockResolvedValue({ data: { peaks: [0, 0.5, 1] } })
    expect(await files.fetchWaveform('forest')).toEqual([0, 0.5, 1])
    expect(api.getApiV1FilesByTrackIdWaveform).toHaveBeenCalledWith({ path: { trackID: 'forest' } })
    api.getApiV1FilesByTrackIdWaveform.mockResolvedValue({ data: {} })
    await expect(files.fetchWaveform('forest')).rejects.toThrow('Invalid track waveform')
  })

  it('fetches, finds, updates, and deletes tracks', async () => {
    const files = useFileStore()
    api.getApiV1Files.mockResolvedValue({ data: [track] })
    await files.fetchFiles()
    expect(files.getTrackById('forest')).toEqual(track)
    expect(files.getTrackById('missing')).toBeUndefined()
    const updated = { ...track, name: 'Rain', typeID: 'music' }
    api.putApiV1FilesByTrackId.mockResolvedValue({ data: updated })
    expect(await files.updateTrack('forest', { name: 'Rain', typeID: 'music' })).toEqual(updated)
    expect(api.putApiV1FilesByTrackId).toHaveBeenCalledWith({
      path: { trackID: 'forest' },
      body: { id: 'forest', name: 'Rain', typeID: 'music' },
    })
    expect(files.tracks).toEqual([updated])
    await files.updateTrack('absent', {})
    expect(files.tracks).toEqual([updated])
    api.deleteApiV1FilesByTrackId.mockResolvedValue({ response: { ok: true, status: 200 } })
    api.getApiV1Files.mockResolvedValue({ data: [] })
    await files.deleteFile('forest', 1)
    expect(api.deleteApiV1FilesByTrackId).toHaveBeenCalledWith({
      path: { trackID: 'forest' },
      query: { expectedTableCount: 1 },
      throwOnError: false,
    })
    expect(files.tracks).toEqual([])
  })

  it('preserves the track if its usage changed or deletion fails', async () => {
    const files = useFileStore()
    files.tracks = [track]
    api.deleteApiV1FilesByTrackId.mockResolvedValueOnce({
      error: 'usage changed', response: { ok: false, status: 409 },
    })
    await expect(files.deleteFile('forest', 0)).rejects.toBeInstanceOf(TrackUsageChangedError)
    api.deleteApiV1FilesByTrackId.mockResolvedValueOnce({
      error: 'database failure', response: { ok: false, status: 500 },
    })
    await expect(files.deleteFile('forest', 1)).rejects.toBe('database failure')
    api.deleteApiV1FilesByTrackId.mockResolvedValueOnce({})
    await expect(files.deleteFile('forest', 1)).rejects.toThrow('Failed to delete track')
    expect(files.tracks).toEqual([track])
  })

  it('uploads metadata and refreshes the library', async () => {
    api.postApiV1Files.mockResolvedValue({})
    api.getApiV1Files.mockResolvedValue({ data: [track] })
    const files = useFileStore()
    const form = uploadForm()
    await files.uploadFile(form)
    expect(api.postApiV1Files).toHaveBeenCalledWith({
      body: { files: form.get('files'), name: 'Forest', typeID: 'ambiance' },
    })
    expect(files.tracks).toEqual([track])
  })

  it.each(['files', 'name', 'typeID'])('rejects an upload missing %s', async (field) => {
    const form = uploadForm()
    form.delete(field)
    await expect(useFileStore().uploadFile(form)).rejects.toThrow('Failed to upload file')
    expect(api.postApiV1Files).not.toHaveBeenCalled()
  })

  it('preserves cached files on fetch failure and propagates write failures', async () => {
    const files = useFileStore()
    files.tracks = [track]
    api.getApiV1Files.mockRejectedValue(new Error('offline'))
    await expect(files.fetchFiles()).rejects.toThrow('offline')
    expect(files.tracks).toEqual([track])
    api.deleteApiV1FilesByTrackId.mockRejectedValue(new Error('delete failed'))
    await expect(files.deleteFile('forest', 1)).rejects.toThrow('delete failed')
    api.putApiV1FilesByTrackId.mockRejectedValue(new Error('update failed'))
    await expect(files.updateTrack('forest', { name: 'Rain' })).rejects.toThrow('update failed')
    api.postApiV1Files.mockRejectedValue(new Error('upload failed'))
    await expect(files.uploadFile(uploadForm())).rejects.toThrow('Failed to upload file')
    expect(files.tracks).toEqual([track])
  })
})

describe('track types', () => {
  it('loads track types without a player session', async () => {
    const types = useTrackTypeStore()
    api.getApiV1TrackTypes.mockResolvedValue({ data: [trackType] })
    await types.fetchTrackTypes()
    expect(api.getApiV1TrackTypes).toHaveBeenLastCalledWith()
    expect(types.getTypeById('ambiance')).toEqual(trackType)
    expect(types.getTypeById('missing')).toBeUndefined()
    api.getApiV1TrackTypes.mockRejectedValue(new Error('offline'))
    await expect(types.fetchTrackTypes()).rejects.toThrow('offline')
    expect(types.trackTypes).toEqual([trackType])
  })
})

function uploadForm() {
  const form = new FormData()
  form.set('files', new File(['audio'], 'forest.mp3', { type: 'audio/mpeg' }))
  form.set('name', 'Forest')
  form.set('typeID', 'ambiance')
  return form
}
