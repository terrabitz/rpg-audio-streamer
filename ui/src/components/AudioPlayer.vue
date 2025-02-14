<template>
  <div>
    <video v-for="track in audioStore.availableTracks" :key="track.fileName"
      :ref="el => videoElements[track.fileName] = el as HTMLVideoElement" style="display: none;"
      @ended="evt => handleEnded(track.fileName, evt)" @timeupdate="evt => handleTimeUpdate(track.fileName, evt)"
      @loadedmetadata="evt => handleLoadedMetadata(track.fileName, evt)" />
  </div>
</template>

<script setup lang="ts">
import Hls from 'hls.js'
import { onBeforeUnmount, ref, watch } from 'vue'
import { useAudioStore } from '../stores/audio'

const audioStore = useAudioStore()
const videoElements = ref<Record<string, HTMLVideoElement>>({})

// Set up audio sync for new elements
watch(videoElements, (elements) => {
  console.log('Setting up audio sync for new elements')
  Object.entries(elements).forEach(([fileName, video]) => {
    if (video) {
      useAudioSync(fileName, video)
    }
  })
}, { deep: true })

onBeforeUnmount(() => {
  Object.values(videoElements.value).forEach(video => {
    if (!video) return
    video.pause()
    video.src = ''
  })
})

function useAudioSync(fileName: string, videoElement: HTMLVideoElement) {
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
  }

  // Watch state and sync to video element
  watch(() => audioStore.tracks[fileName], (state) => {
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
  }, { deep: true })
}

function handleEnded(fileName: string, evt: Event) {
  const videoElement = evt.target as HTMLVideoElement
  audioStore.updateTrackState(fileName, { isPlaying: false })
  videoElement.pause()
  setTimeout(() => {
    videoElement.currentTime = 0
    audioStore.updateTrackState(fileName, { currentTime: 0 })
  }, 0)
}

function handleTimeUpdate(fileName: string, evt: Event) {
  const videoElement = evt.target as HTMLVideoElement
  audioStore.updateTrackState(fileName, { currentTime: videoElement.currentTime })
}

function handleLoadedMetadata(fileName: string, evt: Event) {
  const videoElement = evt.target as HTMLVideoElement
  audioStore.updateTrackState(fileName, { duration: videoElement.duration })
}
</script>
