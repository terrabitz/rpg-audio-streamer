import { ref } from 'vue'

interface AudioState {
  isPlaying: boolean
  volume: number
  isRepeating: boolean
}

export function useAudioPlayer() {
  const audioPlayers = ref(new Map<string, HTMLAudioElement>())
  const audioStates = ref(new Map<string, AudioState>())

  function getInitialState(): AudioState {
    return {
      isPlaying: false,
      volume: 100,
      isRepeating: false,
    }
  }

  function getState(fileName: string): AudioState {
    if (!audioStates.value.has(fileName)) {
      audioStates.value.set(fileName, getInitialState())
    }
    return audioStates.value.get(fileName)!
  }

  function createAudioPlayer(fileName: string): HTMLAudioElement {
    const state = getState(fileName)
    const player = new Audio()
    player.src = `${import.meta.env.VITE_API_BASE_URL}/stream/${fileName}`
    player.volume = state.volume / 100
    player.loop = state.isRepeating
    player.onended = () => {
      if (!player.loop) {
        state.isPlaying = false
        audioPlayers.value.delete(fileName)
      }
    }
    return player
  }

  function togglePlay(fileName: string) {
    const state = getState(fileName)
    let player = audioPlayers.value.get(fileName)

    if (!player) {
      player = createAudioPlayer(fileName)
      audioPlayers.value.set(fileName, player)
    }

    if (state.isPlaying) {
      player.pause()
      state.isPlaying = false
    } else {
      player.play()
      state.isPlaying = true
    }
  }

  function toggleRepeat(fileName: string) {
    const state = getState(fileName)
    const player = audioPlayers.value.get(fileName)

    state.isRepeating = !state.isRepeating
    if (player) {
      player.loop = state.isRepeating
    }
  }

  function setVolume(fileName: string, volume: number) {
    const state = getState(fileName)
    const player = audioPlayers.value.get(fileName)

    state.volume = volume
    if (player) {
      player.volume = volume / 100
    }
  }

  function cleanup(fileName: string) {
    const player = audioPlayers.value.get(fileName)
    if (player) {
      player.pause()
      audioPlayers.value.delete(fileName)
    }
    audioStates.value.delete(fileName)
  }

  return {
    getState,
    togglePlay,
    toggleRepeat,
    setVolume,
    cleanup,
  }
}
