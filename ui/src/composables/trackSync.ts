import debounce from 'lodash.debounce'
import type { AudioTrack } from '@/stores/audio'

type TrackPatch = Partial<AudioTrack> & { fileID: string }

export function createTrackSync(send: (method: string, payload: TrackPatch) => void) {
  const pending = new Map<string, TrackPatch>()
  const flush = debounce(() => {
    const patches = [...pending.values()]
    pending.clear()
    for (const patch of patches) send('syncTrack', patch)
  }, 100)

  return {
    send(patch: TrackPatch) {
      pending.set(patch.fileID, { ...pending.get(patch.fileID), ...patch })
      flush()
    },
    flush() {
      flush.flush()
    },
    cancel() {
      flush.cancel()
      pending.clear()
    },
    remove(fileID: string) {
      pending.delete(fileID)
    },
  }
}
