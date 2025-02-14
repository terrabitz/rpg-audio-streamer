<template>
  <div>
    <video v-for="track in audioStore.availableTracks" :key="track.fileName"
      :ref="el => registerVideoElement(track.fileName, el as HTMLVideoElement)" style="display: none;"
      @ended="evt => handleEnded(track.fileName, evt)" @timeupdate="evt => handleTimeUpdate(track.fileName, evt)"
      @loadedmetadata="evt => handleLoadedMetadata(track.fileName, evt)" />
  </div>
</template>

<script setup lang="ts">
import Hls from 'hls.js'
import { onBeforeUnmount, ref, watch } from 'vue'
import { type AudioTrack, useAudioStore } from '../stores/audio'

const audioStore = useAudioStore()
const videoElements = ref<Record<string, HTMLVideoElement>>({})
const initializedElements = ref<Set<string>>(new Set())

function registerVideoElement(fileName: string, videoElement: HTMLVideoElement) {
  if (!initializedElements.value.has(fileName)) {
    console.log('Registering video element:', fileName)
    videoElements.value[fileName] = videoElement
    startAudioSync(fileName, videoElement)
    initializedElements.value.add(fileName)
  }
}

onBeforeUnmount(() => {
  Object.values(videoElements.value).forEach(video => {
    if (!video) return
    video.pause()
    video.src = ''
  })
})

// startAudioSync sets up the HLS.js player and watches state for syncing
function startAudioSync(fileName: string, videoElement: HTMLVideoElement) {
  // Set up HLS.js if supported
  if (Hls.isSupported()) {
    const hls = new Hls()
    hls.loadSource(`/api/v1/stream/${fileName}/index.m3u8`)
    hls.attachMedia(videoElement)
    hls.on(Hls.Events.MANIFEST_PARSED, () => {
      if (audioStore.tracks[fileName].isPlaying) {
        syncStateToVideoElement(audioStore.tracks[fileName], videoElement)
      }
    })
    hls.on(Hls.Events.LEVEL_LOADED, (_, data) => {
      audioStore.updateTrackState(fileName, { duration: data.details.totalduration })
    })
  } else if (videoElement.canPlayType('application/vnd.apple.mpegurl')) {
    videoElement.src = `/api/v1/stream/${fileName}/index.m3u8`
    syncStateToVideoElement(audioStore.tracks[fileName], videoElement)
  }

  // Watch state and sync to video element
  watch(() => audioStore.tracks[fileName], (state) => {
    if (!state) return
    syncStateToVideoElement(state, videoElement)
  }, { deep: true })
}

function syncStateToVideoElement(state: AudioTrack, videoElement: HTMLVideoElement) {
  // Always sync these properties
  videoElement.volume = state.volume / 100
  videoElement.loop = state.isRepeating

  if (state.isPlaying && videoElement.paused) {
    const playPromise = videoElement.play()
    if (playPromise !== undefined) {
      playPromise.catch(() => {
        audioStore.updateTrackState(state.fileName, { isPlaying: false })
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
