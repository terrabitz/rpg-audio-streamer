<template>
  <video :ref="el => videoElement = el as HTMLVideoElement" style="display: none;" @ended="handleEnded"
    @timeupdate="handleTimeUpdate" @loadedmetadata="handleLoadedMetadata" />
</template>

<script setup lang="ts">
import Hls from 'hls.js';
import { onBeforeUnmount, ref, watch } from 'vue';
import { useAudioStore, type AudioTrack } from '../stores/audio';

const props = defineProps<{ fileName: string }>()
const audioStore = useAudioStore()
const videoElement = ref<HTMLVideoElement | null>(null)

watch(videoElement, (el) => {
  if (el) {
    console.log("registering video element", props.fileName)
    startAudioSync(props.fileName, el)
  }
})

onBeforeUnmount(() => {
  if (videoElement.value) {
    videoElement.value.pause()
    videoElement.value.src = ''
  }
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

function handleEnded(evt: Event) {
  const videoElement = evt.target as HTMLVideoElement
  audioStore.updateTrackState(props.fileName, { isPlaying: false })
  videoElement.pause()
  setTimeout(() => {
    videoElement.currentTime = 0
    audioStore.updateTrackState(props.fileName, { currentTime: 0 })
  }, 0)
}

function handleTimeUpdate(evt: Event) {
  const videoElement = evt.target as HTMLVideoElement
  audioStore.updateTrackState(props.fileName, { currentTime: videoElement.currentTime })
}

function handleLoadedMetadata(evt: Event) {
  const videoElement = evt.target as HTMLVideoElement
  audioStore.updateTrackState(props.fileName, { duration: videoElement.duration })
}
</script>
