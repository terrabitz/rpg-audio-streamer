import { ref, watchEffect } from 'vue'
import { useAudioStore } from '../stores/audio'

export function useAudioSync(fileName: string, audioElement: HTMLAudioElement) {
  const audioStore = useAudioStore()
  const canPlay = ref(false)

  // Set up one-time event listeners
  audioElement.addEventListener('loadedmetadata', () => {
    audioStore.updateTrackState(fileName, { duration: audioElement.duration })
  })

  audioElement.addEventListener('canplaythrough', () => {
    canPlay.value = true
  })

  audioElement.addEventListener('ended', () => {
    // Update state and pause first
    audioStore.updateTrackState(fileName, { isPlaying: false })
    audioElement.pause()
    // Then reset position
    setTimeout(() => {
      audioElement.currentTime = 0
      audioStore.updateTrackState(fileName, { currentTime: 0 })
    }, 0)
  })

  // Watch state and sync to audio element
  watchEffect(() => {
    const state = audioStore.tracks[fileName]
    if (!state) return

    // Always sync these properties
    audioElement.volume = state.volume / 100
    audioElement.loop = state.isRepeating

    // Only sync playback state when ready
    if (canPlay.value && audioElement.readyState >= HTMLMediaElement.HAVE_ENOUGH_DATA) {
      if (state.isPlaying && audioElement.paused) {
        const playPromise = audioElement.play()
        if (playPromise !== undefined) {
          playPromise.catch(() => {
            audioStore.updateTrackState(fileName, { isPlaying: false })
          })
        }
      } else if (!state.isPlaying && !audioElement.paused) {
        audioElement.pause()
      }

      // Only seek if difference is significant
      if (Math.abs(audioElement.currentTime - state.currentTime) > 0.5) {
        audioElement.currentTime = state.currentTime
      }
    }
  })
}
