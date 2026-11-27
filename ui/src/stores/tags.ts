import { getApiV1Tags, postApiV1Tags, putApiV1TagsByTagId, type Tag, type TagRequest } from '@/client/apiClient'
import { defineStore } from 'pinia'

export const tagPalette = [
  { name: 'Red', color: '#E53935' },
  { name: 'Orange', color: '#FB8C00' },
  { name: 'Yellow', color: '#FDD835' },
  { name: 'Green', color: '#43A047' },
  { name: 'Teal', color: '#00897B' },
  { name: 'Blue', color: '#1E88E5' },
  { name: 'Indigo', color: '#3949AB' },
  { name: 'Purple', color: '#8E24AA' },
  { name: 'Pink', color: '#D81B60' },
  { name: 'Brown', color: '#6D4C41' },
] as const

export class TagNameTakenError extends Error {
  constructor() {
    super('A tag with this name already exists')
    this.name = 'TagNameTakenError'
  }
}

export const useTagStore = defineStore('tags', {
  state: () => ({
    tags: [] as Tag[]
  }),
  getters: {
    getTagById: (state) => (id: string) => state.tags.find(tag => tag.id === id),
    nextColor: (state) => tagPalette[state.tags.length % tagPalette.length]!.color,
  },
  actions: {
    async fetchTags() {
      const { data } = await getApiV1Tags<true>()
      this.tags = data
    },
    async createTag(body: TagRequest) {
      const result = await postApiV1Tags<false>({ body, throwOnError: false })
      return this.applyTag(tagResult(result))
    },
    async updateTag(tagID: string, body: TagRequest) {
      const result = await putApiV1TagsByTagId<false>({ path: { tagID }, body, throwOnError: false })
      return this.applyTag(tagResult(result))
    },
    removeTag(tagID: string) {
      this.tags = this.tags.filter(tag => tag.id !== tagID)
    },
    applyTag(tag: Tag) {
      this.tags = [...this.tags.filter(item => item.id !== tag.id), tag]
        .sort((a, b) => a.name.localeCompare(b.name, undefined, { sensitivity: 'base' }))
      return tag
    },
  }
})

function tagResult(result: { data?: Tag, error?: unknown, response?: Response }) {
  if (result.response?.status === 409) throw new TagNameTakenError()
  if (result.data === undefined) throw result.error ?? new Error('Failed to save tag')
  return result.data
}
