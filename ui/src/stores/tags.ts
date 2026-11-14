import {
  deleteApiV1TagsByTagId, getApiV1Tags, postApiV1Tags, putApiV1TagsByTagId,
  type Tag, type TagInput,
} from '@/client/apiClient'
import { defineStore } from 'pinia'
import { useFileStore } from './files'

export const useTagStore = defineStore('tags', {
  state: () => ({ tags: [] as Tag[] }),
  actions: {
    async fetchTags() {
      const { data } = await getApiV1Tags<true>()
      this.tags = data
    },
    async saveTag(input: TagInput, id?: string) {
      const { data } = id
        ? await putApiV1TagsByTagId<true>({ path: { tagID: id }, body: input })
        : await postApiV1Tags<true>({ body: input })
      this.tags = [...this.tags.filter(tag => tag.id !== data.id), data]
        .sort((a, b) => a.name.localeCompare(b.name))
      const files = useFileStore()
      for (const track of files.tracks) {
        track.tags = track.tags?.map(tag => tag.id === data.id ? data : tag)
      }
      return data
    },
    async deleteTag(id: string) {
      await deleteApiV1TagsByTagId<true>({ path: { tagID: id } })
      this.tags = this.tags.filter(tag => tag.id !== id)
      for (const track of useFileStore().tracks) {
        track.tags = track.tags?.filter(tag => tag.id !== id)
      }
    },
  },
})
