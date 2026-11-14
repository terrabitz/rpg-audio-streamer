import { expect, it } from 'vitest'
import { matchesLibrarySearch } from '../librarySearch'

const track = {
  id: 'forest', name: 'Forest breeze', path: '/forest', createdAt: '', typeID: 'ambiance',
  tags: [{ id: 'calm', name: 'Low intensity', color: '#abcdef' }],
}
const types = [{ id: 'ambiance', name: 'Ambiance', color: '#ffffff',
  isRepeating: true, allowSimultaneousPlay: true }]

it('finds tracks by name, type, and tags with case-insensitive search words', () => {
  expect(matchesLibrarySearch(track, ' FOREST  intensity ', types)).toBe(true)
  expect(matchesLibrarySearch(track, 'AMBIANCE', types)).toBe(true)
  expect(matchesLibrarySearch(track, '', types)).toBe(true)
  expect(matchesLibrarySearch(track, 'forest battle', types)).toBe(false)
  expect(matchesLibrarySearch({ ...track, tags: undefined }, 'intensity', [])).toBe(false)
})
