import {
  getTablePlayback, patchTableMixVolumes, patchTablePlayback, putTableMix,
  type LiveMixSnapshot, type LivePlaybackPatch,
} from '@/client/apiClient'
import { defineStore } from 'pinia'

export const usePlaybackCommands = defineStore('playbackCommands', () => {
  const pending = new Map<string, Promise<void>>()

  function enqueue(tableID: string, command: () => Promise<unknown>) {
    const result = (pending.get(tableID) ?? Promise.resolve()).then(command)
    const settled = result.then(() => {}, () => {})
    pending.set(tableID, settled)
    void settled.then(() => {
      if (pending.get(tableID) === settled) pending.delete(tableID)
    })
    return result
  }

  async function snapshot(tableID: string, signal?: AbortSignal) {
    const { data } = await getTablePlayback<true>({ path: { tableID }, signal })
    return data
  }

  function patch(tableID: string, trackID: string, body: LivePlaybackPatch) {
    return enqueue(tableID, () => patchTablePlayback<true>({ path: { tableID, trackID }, body }))
  }

  function replaceMix(tableID: string, body: LiveMixSnapshot) {
    return enqueue(tableID, () => putTableMix<true>({ path: { tableID }, body }))
  }

  function mixVolumes(tableID: string, typeVolumes: Record<string, number>) {
    return enqueue(tableID, () => patchTableMixVolumes<true>({ path: { tableID }, body: { typeVolumes } }))
  }

  return { snapshot, patch, replaceMix, mixVolumes }
})
