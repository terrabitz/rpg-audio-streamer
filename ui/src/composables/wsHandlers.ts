import { onMounted, onUnmounted, type Ref } from 'vue'
import { useAudioStore } from '../stores/audio'
import { useWebSocketStore } from '../stores/websocket'

interface Track {
  fileName: string
  isPlaying: boolean
  currentTime: number
  volume: number
  isRepeating: boolean
}

interface SyncAllPayload {
  tracks: Track[]
}

interface SyncTrackPayload {
  fileName: string
  isPlaying?: boolean
  currentTime?: number
  volume?: number
  isRepeating?: boolean
}

export function wsHandlers(audioElements: Ref<Record<string, HTMLAudioElement>>) {
  const wsStore = useWebSocketStore()
  const audioStore = useAudioStore()

  function handleSyncAll(message: { method: string, payload: SyncAllPayload }) {
    if (message.method === 'syncAll' && message.payload.tracks) {
      // Get current track names before sync
      const previousTracks = new Set(Object.keys(audioStore.tracks))

      // Update store state
      audioStore.syncTracks(message.payload.tracks)

      // Get new track names after sync
      const newTracks = new Set(message.payload.tracks.map((t: Track) => t.fileName))

      // Stop and cleanup removed audio elements
      previousTracks.forEach(fileName => {
        if (!newTracks.has(fileName)) {
          const audio = audioElements.value[fileName]
          if (audio) {
            audio.pause()
            audio.src = ''
            delete audioElements.value[fileName]
          }
        }
      })

      // Then ensure all tracks that should be playing are playing
      message.payload.tracks.forEach((track: Track) => {
        if (track.fileName && track.isPlaying) {
          audioStore.updateTrackState(track.fileName, {
            isPlaying: true,
            currentTime: track.currentTime || 0,
            volume: track.volume || 100,
            isRepeating: track.isRepeating || false
          })
        }
      })
    }
  }

  function handleSyncTrack(message: { method: string, payload: SyncTrackPayload }) {
    if (message.method === 'syncTrack' && message.payload.fileName) {
      console.log(message)
      const { fileName, ...updates } = message.payload
      audioStore.updateTrackState(fileName, updates)

      if (updates.isPlaying === false) {
        audioStore.removeTrack(fileName)
        const audio = audioElements.value[fileName]
        if (audio) {
          audio.pause()
          audio.src = ''
          delete audioElements.value[fileName]
        }
      }
    }
  }

  onMounted(() => {
    wsStore.addMessageHandler(handleSyncAll)
    wsStore.addMessageHandler(handleSyncTrack)
  })

  onUnmounted(() => {
    wsStore.removeMessageHandler(handleSyncAll)
    wsStore.removeMessageHandler(handleSyncTrack)
  })
}
