import type { Track, TrackType } from '@/client/apiClient'

export function matchesLibrarySearch(track: Track, query: string, types: TrackType[]) {
  const text = [track.name, types.find(type => type.id === track.typeID)?.name,
    ...track.tags?.map(tag => tag.name) ?? []].join(' ').toLocaleLowerCase()
  return query.trim().toLocaleLowerCase().split(/\s+/).every(word => text.includes(word))
}
