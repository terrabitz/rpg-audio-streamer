import type { LivePlaybackSnapshot } from '@/client/apiClient'
import { usePlaybackCommands } from '@/stores/playbackCommands'

export function usePlaybackSnapshot(apply: (snapshot: LivePlaybackSnapshot) => void, fail: () => void) {
  const commands = usePlaybackCommands()
  let latestRevision = -1
  let request: AbortController | undefined

  function accept(snapshot: LivePlaybackSnapshot) {
    if (snapshot.revision <= latestRevision) return
    latestRevision = snapshot.revision
    apply(snapshot)
  }

  async function refresh(tableID: string) {
    request?.abort()
    const controller = new AbortController()
    request = controller
    try {
      const snapshot = await commands.snapshot(tableID, controller.signal)
      if (!controller.signal.aborted) accept(snapshot)
    } catch (error) {
      if (controller.signal.aborted) return
      console.error('Unable to load playback snapshot:', error)
      if (latestRevision < 0) fail()
    } finally {
      if (request === controller) request = undefined
    }
  }

  function reset() {
    request?.abort()
    latestRevision = -1
  }

  return { accept, refresh, reset }
}
