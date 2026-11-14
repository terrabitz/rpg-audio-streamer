import { beforeEach, expect, it, vi } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import { useTagStore } from '../tags'
import { useFileStore } from '../files'

const api = vi.hoisted(() => ({
  getApiV1Tags: vi.fn(), postApiV1Tags: vi.fn(), putApiV1TagsByTagId: vi.fn(),
  deleteApiV1TagsByTagId: vi.fn(),
}))
vi.mock('@/client/apiClient', () => api)

const tag = { id: 'intensity', name: 'High intensity', color: '#ff0000' }
const track = { id: 'battle', name: 'Battle', path: '', typeID: 'music', createdAt: '', tags: [tag] }

beforeEach(() => {
  setActivePinia(createPinia())
  vi.resetAllMocks()
})

it('loads and creates reusable tags without changing track assignments', async () => {
  const tags = useTagStore()
  api.getApiV1Tags.mockResolvedValue({ data: [] })
  await tags.fetchTags()
  api.postApiV1Tags.mockResolvedValue({ data: tag })
  expect(await tags.saveTag({ name: tag.name, color: tag.color })).toEqual(tag)
  expect(tags.tags).toEqual([tag])
  expect(useFileStore().tracks).toEqual([])
})

it('shares renamed and recolored tags, and removes deleted assignments without deleting audio', async () => {
  const tags = useTagStore()
  const files = useFileStore()
  tags.tags = [tag]
  files.tracks = [track]
  const updated = { ...tag, name: 'Intense', color: '#123456' }
  api.putApiV1TagsByTagId.mockResolvedValue({ data: updated })
  await tags.saveTag(updated, tag.id)
  expect(files.tracks[0]?.tags).toEqual([updated])
  api.deleteApiV1TagsByTagId.mockResolvedValue({})
  await tags.deleteTag(tag.id)
  expect(files.tracks).toEqual([{ ...track, tags: [] }])
  expect(tags.tags).toEqual([])
})

it('propagates failures and keeps the previous tags and track assignments', async () => {
  const tags = useTagStore()
  const files = useFileStore()
  tags.tags = [tag]
  files.tracks = [track]
  for (const request of Object.values(api)) request.mockRejectedValue(new Error('offline'))
  await expect(tags.fetchTags()).rejects.toThrow('offline')
  await expect(tags.saveTag(tag)).rejects.toThrow('offline')
  await expect(tags.saveTag(tag, tag.id)).rejects.toThrow('offline')
  await expect(tags.deleteTag(tag.id)).rejects.toThrow('offline')
  expect(tags.tags).toEqual([tag])
  expect(files.tracks).toEqual([track])
})
