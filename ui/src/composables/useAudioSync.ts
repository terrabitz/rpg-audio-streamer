import Hls from 'hls.js'
import { ref, watchEffect } from 'vue'
import { useAudioStore } from '../stores/audio'

export function useAudioSync(fileName: string, videoElement: HTMLVideoElement) {
  const audioStore = useAudioStore()
  const canPlay = ref(false)

  // Set up HLS.js if supported
  if (Hls.isSupported()) {
    const hls = new Hls()
    hls.loadSource(`/api/v1/stream/${fileName}/index.m3u8`)
    hls.attachMedia(videoElement)
    hls.on(Hls.Events.MANIFEST_PARSED, () => {
      canPlay.value = true
    })
    hls.on(Hls.Events.LEVEL_LOADED, (_, data) => {
      audioStore.updateTrackState(fileName, { duration: data.details.totalduration })
    })
  } else if (videoElement.canPlayType('application/vnd.apple.mpegurl')) {
    videoElement.src = `/api/v1/stream/${fileName}/index.m3u8`
    videoElement.addEventListener('loadedmetadata', () => {
      canPlay.value = true
      audioStore.updateTrackState(fileName, { duration: videoElement.duration })
    })
  }

  videoElement.addEventListener('ended', () => {
    // Update state and pause first
    audioStore.updateTrackState(fileName, { isPlaying: false })
    videoElement.pause()
    // Then reset position
    setTimeout(() => {
      videoElement.currentTime = 0
      audioStore.updateTrackState(fileName, { currentTime: 0 })
    }, 0)
  })

  videoElement.addEventListener('timeupdate', () => {
    audioStore.updateTrackState(fileName, { currentTime: videoElement.currentTime })
  })

  // Watch state and sync to video element
  watchEffect(() => {
    const state = audioStore.tracks[fileName]
    if (!state) return

    // Always sync these properties
    videoElement.volume = state.volume / 100
    videoElement.loop = state.isRepeating

    // Only sync playback state when ready
    if (canPlay.value && videoElement.readyState >= HTMLMediaElement.HAVE_ENOUGH_DATA) {
      if (state.isPlaying && videoElement.paused) {
        const playPromise = videoElement.play()
        if (playPromise !== undefined) {
          playPromise.catch(() => {
            audioStore.updateTrackState(fileName, { isPlaying: false })
          })
        }
      } else if (!state.isPlaying && !videoElement.paused) {
        videoElement.pause()
      }

      // Only seek if difference is significant
      if (Math.abs(videoElement.currentTime - state.currentTime) > 0.5) {
        videoElement.currentTime = state.currentTime
      }
    }
  })
}
