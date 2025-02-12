import { watchEffect } from 'vue'
import { useAudioStore } from '../stores/audio'

export function useAudioSync(fileName: string, audioElement: HTMLAudioElement) {
  const audioStore = useAudioStore()

  // Listen for metadata loaded to get duration
  audioElement.addEventListener('loadedmetadata', () => {
    audioStore.updateTrackState(fileName, { duration: audioElement.duration })
  })

  // Listen for track end
  audioElement.addEventListener('ended', () => {
    // Reset currentTime and update state
    audioElement.currentTime = 0
    audioStore.updateTrackState(fileName, {
      isPlaying: false,
      currentTime: 0
    })
  })

  // Listen for audio source ready state
  audioElement.addEventListener('canplay', () => {
    const state = audioStore.tracks[fileName]
    if (state?.isPlaying) {
      audioElement.play().catch(err => {
        console.warn('Failed to auto-play audio:', err)
        audioStore.updateTrackState(fileName, { isPlaying: false })
      })
    }
  })

  watchEffect(() => {
    const state = audioStore.tracks[fileName]
    if (!state) return

    // Sync volume and loop state regardless of ready state
    audioElement.volume = state.volume / 100
    audioElement.loop = state.isRepeating

    // Only attempt play/pause if the audio is ready
    if (audioElement.readyState >= HTMLMediaElement.HAVE_ENOUGH_DATA) {
      if (state.isPlaying && audioElement.paused) {
        audioElement.play().catch(err => {
          console.warn('Failed to play audio:', err)
          audioStore.updateTrackState(fileName, { isPlaying: false })
        })
      } else if (!state.isPlaying && !audioElement.paused) {
        audioElement.pause()
      }

      // Sync seeking only when ready and if difference is significant
      if (Math.abs(audioElement.currentTime - state.currentTime) > 0.5) {
        audioElement.currentTime = state.currentTime
      }
    }
  })
}
