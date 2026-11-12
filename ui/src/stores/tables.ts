import {
  deleteApiV1TablesByTableId,
  deleteApiV1TablesByTableIdTracksByTrackId,
  getApiV1Tables,
  getApiV1TablesByTableId,
  getApiV1TablesByTableIdTracks,
  getApiV1TablesCurrent,
  postApiV1Tables,
  putApiV1TablesByTableId,
  putApiV1TablesByTableIdTracksByTrackId,
  type Table,
  type Track,
} from '@/client/apiClient'
import { defineStore } from 'pinia'

export const useTableStore = defineStore('tables', {
  state: () => ({
    tables: [] as Table[],
    current: null as Table | null,
    tracks: [] as Track[],
    revision: 0,
  }),
  actions: {
    async fetchTables() {
      const { data } = await getApiV1Tables<true>()
      this.tables = data
    },
    async createTable(name: string) {
      const { data } = await postApiV1Tables<true>({ body: { name } })
      this.tables.push(data)
      return data
    },
    async renameTable(tableID: string, name: string) {
      const { data } = await putApiV1TablesByTableId<true>({ path: { tableID }, body: { name } })
      this.tables = this.tables.map(table => table.id === tableID ? data : table)
      if (this.current?.id === tableID) this.current = data
    },
    async deleteTable(tableID: string) {
      await deleteApiV1TablesByTableId<true>({ path: { tableID } })
      this.tables = this.tables.filter(table => table.id !== tableID)
      if (this.current?.id === tableID) this.clearCurrent()
    },
    async loadTable(tableID?: string, token?: string) {
      this.clearCurrent()
      const revision = this.revision
      const { data } = tableID
        ? await getApiV1TablesByTableId<true>({ path: { tableID } })
        : await getApiV1TablesCurrent<true>({
          headers: token ? { Authorization: `Bearer ${token}` } : undefined,
        })
      if (revision === this.revision) this.current = data
    },
    async fetchTracks(tableID: string) {
      const { data } = await getApiV1TablesByTableIdTracks<true>({ path: { tableID } })
      if (this.current?.id === tableID) this.tracks = data
    },
    async addTrack(tableID: string, trackID: string) {
      await putApiV1TablesByTableIdTracksByTrackId<true>({ path: { tableID, trackID } })
      await this.fetchTracks(tableID)
    },
    async removeTrack(tableID: string, trackID: string) {
      await deleteApiV1TablesByTableIdTracksByTrackId<true>({ path: { tableID, trackID } })
      this.tracks = this.tracks.filter(track => track.id !== trackID)
    },
    clearCurrent() {
      this.revision++
      this.current = null
      this.tracks = []
    },
  },
})
