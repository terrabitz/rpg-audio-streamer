import {
  deleteApiV1TablesByTableId,
  deleteApiV1TablesByTableIdTracksByTrackId,
  getApiV1Tables,
  getApiV1TablesByTableId,
  getApiV1TablesByTableIdSettings,
  patchApiV1TablesByTableIdSettings,
  getApiV1TablesByTableIdTracks,
  getApiV1TablesCurrent,
  postApiV1Tables,
  putApiV1TablesByTableId,
  putApiV1TablesByTableIdTracksByTrackId,
  type Table,
  type Track,
  type TableSettingsPatch,
} from '@/client/apiClient'
import { defineStore } from 'pinia'

export const useTableStore = defineStore('tables', {
  state: () => ({
    tables: [] as Table[],
    current: null as Table | null,
    tracks: [] as Track[],
    revision: 0,
    trackOrders: {} as Record<string, string[]>,
    settingsError: '',
    failedSettings: {} as Record<string, TableSettingsPatch>,
    saveQueue: Promise.resolve(),
  }),
  getters: {
    orderedTracks(state): Track[] {
      const order = state.current ? state.trackOrders[state.current.id] ?? [] : []
      const positions = new Map(order.map((id, index) => [id, index]))
      return [...state.tracks].sort((a, b) =>
        (positions.get(a.id) ?? order.length) - (positions.get(b.id) ?? order.length),
      )
    },
  },
  actions: {
    async loadSettings(tableID: string) {
      await this.saveQueue
      const { data } = await getApiV1TablesByTableIdSettings<true>({ path: { tableID } })
      const failed = this.failedSettings[tableID]
      const settings = {
        ...data,
        ...failed,
        typeVolumes: { ...data.typeVolumes, ...failed?.typeVolumes },
        trackVolumes: { ...data.trackVolumes, ...failed?.trackVolumes },
      }
      if (this.current?.id === tableID) {
        this.trackOrders[tableID] = settings.trackOrder
        if (failed) {
          this.settingsError = 'Unable to save table settings. Your changes are not saved.'
        }
      }
      return settings
    },
    saveSettings(tableID: string, patch: TableSettingsPatch): Promise<void> {
      // Serialize slider and reorder writes so an older request cannot win.
      this.saveQueue = this.saveQueue.then(async () => {
        const failed = this.failedSettings[tableID]
        const body = {
          ...failed, ...patch,
          typeVolumes: { ...failed?.typeVolumes, ...patch.typeVolumes },
          trackVolumes: { ...failed?.trackVolumes, ...patch.trackVolumes },
        }
        try {
          await patchApiV1TablesByTableIdSettings<true>({ path: { tableID }, body, keepalive: true })
          delete this.failedSettings[tableID]
          if (this.current?.id === tableID) this.settingsError = ''
        } catch (e) {
          console.error('Failed to save table settings:', e)
          this.failedSettings[tableID] = body
          if (this.current?.id === tableID) {
            this.settingsError = 'Unable to save table settings. Your changes are not saved.'
          }
        }
      })
      return this.saveQueue
    },
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
      delete this.trackOrders[tableID]
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
      if (this.current?.id !== tableID) return
      this.tracks = data
      this.trackOrders[tableID] = this.orderedTracks.map(track => track.id)
    },
    moveTrack(trackID: string, targetID: string) {
      if (!this.current) return
      const ordered = this.orderedTracks
      const sourceIndex = ordered.findIndex(track => track.id === trackID)
      const targetIndex = ordered.findIndex(track => track.id === targetID)
      if (sourceIndex < 0 || targetIndex < 0 || sourceIndex === targetIndex) return
      const [track] = ordered.splice(sourceIndex, 1)
      if (track) ordered.splice(targetIndex, 0, track)
      this.trackOrders[this.current.id] = ordered.map(track => track.id)
      void this.saveSettings(this.current.id, { trackOrder: this.trackOrders[this.current.id] })
    },
    async addTrack(tableID: string, trackID: string) {
      await putApiV1TablesByTableIdTracksByTrackId<true>({ path: { tableID, trackID } })
      await this.fetchTracks(tableID)
    },
    async removeTrack(tableID: string, trackID: string) {
      await deleteApiV1TablesByTableIdTracksByTrackId<true>({ path: { tableID, trackID } })
      this.tracks = this.tracks.filter(track => track.id !== trackID)
      if (this.trackOrders[tableID]) {
        this.trackOrders[tableID] = this.trackOrders[tableID].filter(id => id !== trackID)
      }
    },
    clearCurrent() {
      this.revision++
      this.current = null
      this.tracks = []
      this.settingsError = ''
    },
  },
})
