<template>
  <video :ref="el => videoElement = el as HTMLVideoElement" style="display: none;" @ended="handleEnded"
    @timeupdate="handleTimeUpdate" @loadedmetadata="handleLoadedMetadata" />
</template>

<script setup lang="ts">
import Hls from 'hls.js';
import { computed, onBeforeUnmount, ref, watch } from 'vue';
import { useAudioStore, type AudioTrack } from '../stores/audio';

const props = defineProps<{ fileID: string }>()
const audioStore = useAudioStore()
const videoElement = ref<HTMLVideoElement | null>(null)

interface FadeState {
  startVolume: number
  targetVolume: number
  startTime: number
}

const FADE_DURATION = 2000 // 2 seconds
const fadeState = ref<FadeState | null>(null)

const currentAudioState = computed(() => audioStore.tracks[props.fileID])

watch(videoElement, (el) => {
  if (el) {
    console.log("registering video element", props.fileID)
    startAudioSync(props.fileID, el)
  }
})

onBeforeUnmount(() => {
  if (videoElement.value) {
    videoElement.value.pause()
    videoElement.value.src = ''
  }
})

// startAudioSync sets up the HLS.js player and watches state for syncing
function startAudioSync(fileID: string, videoElement: HTMLVideoElement) {
  // Set up HLS.js if supported
  if (Hls.isSupported()) {
    const hls = new Hls()
    hls.loadSource(`/api/v1/stream/${fileID}/index.m3u8`)
    hls.attachMedia(videoElement)
    hls.on(Hls.Events.MANIFEST_PARSED, () => {
      if (audioStore.tracks[fileID].isPlaying) {
        syncStateToVideoElement(audioStore.tracks[fileID], videoElement)
      }
    })
    hls.on(Hls.Events.LEVEL_LOADED, (_, data) => {
      audioStore.updateTrackState(fileID, { duration: data.details.totalduration })
    })
  } else if (videoElement.canPlayType('application/vnd.apple.mpegurl')) {
    videoElement.src = `/api/v1/stream/${fileID}/index.m3u8`
    syncStateToVideoElement(audioStore.tracks[fileID], videoElement)
  }

  // Watch state and sync to video element
  watch(() => audioStore.tracks[fileID], (state) => {
    syncStateToVideoElement(state, videoElement)
  }, { deep: true })
}

function startFade(startVolume: number, targetVolume: number) {
  fadeState.value = {
    startVolume,
    targetVolume,
    startTime: Date.now()
  }
}

function syncStateToVideoElement(newState: AudioTrack, videoElement: HTMLVideoElement) {
  if (fadeState.value) {
    const elapsed = Date.now() - fadeState.value.startTime
    const fadePercent = Math.min(1, elapsed / FADE_DURATION)

    const currentVolume = fadeState.value.startVolume +
      (fadeState.value.targetVolume - fadeState.value.startVolume) * fadePercent

    videoElement.volume = (currentVolume / 100) * (audioStore.masterVolume / 100)

    if (fadePercent === 1) {
      fadeState.value = null
      // If we faded to zero, actually pause the track
      if (currentVolume <= .01) {
        videoElement.pause()
      }
    }

    return
  }

  // Handle play/pause with fading
  if (newState.isPlaying && videoElement.paused) {
    // Start fade in when playing
    startFade(0, newState.volume)
    const playPromise = videoElement.play()
    if (playPromise !== undefined) {
      playPromise.catch(() => {
        audioStore.updateTrackState(newState.fileID, { isPlaying: false })
      })
    }
  } else if (!newState.isPlaying && !videoElement.paused) {
    // Start fade out when stopping
    startFade(newState.volume, 0)
  }

  videoElement.volume = (newState.volume / 100) * (audioStore.masterVolume / 100)

  // Only seek if difference is significant
  if (Math.abs(videoElement.currentTime - newState.currentTime) > 0.5) {
    videoElement.currentTime = newState.currentTime
  }
}

function handleEnded(evt: Event) {
  const videoElement = evt.target as HTMLVideoElement
  audioStore.updateTrackState(props.fileID, { isPlaying: false })
  videoElement.pause()
  setTimeout(() => {
    videoElement.currentTime = 0
    audioStore.updateTrackState(props.fileID, { currentTime: 0 })
  }, 0)
}

function handleTimeUpdate(evt: Event) {
  const videoElement = evt.target as HTMLVideoElement
  audioStore.updateTrackState(props.fileID, { currentTime: videoElement.currentTime })
}

function handleLoadedMetadata(evt: Event) {
  const videoElement = evt.target as HTMLVideoElement
  audioStore.updateTrackState(props.fileID, { duration: videoElement.duration })
}
</script>
