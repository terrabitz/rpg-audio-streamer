import { watchEffect } from 'vue'
import { useAudioStore } from '../stores/audio'

export function useAudioSync(fileName: string, audioElement: HTMLAudioElement) {
  const audioStore = useAudioStore()

  watchEffect(() => {
    const state = audioStore.tracks[fileName]
    if (!state) return

    // Sync volume
    if (audioElement.volume !== state.volume / 100) {
      audioElement.volume = state.volume / 100
    }

    // Sync loop state
    if (audioElement.loop !== state.isRepeating) {
      audioElement.loop = state.isRepeating
    }

    // Sync play/pause state
    if (state.isPlaying && audioElement.paused) {
      audioElement.play()
    } else if (!state.isPlaying && !audioElement.paused) {
      audioElement.pause()
    }
  })
}
