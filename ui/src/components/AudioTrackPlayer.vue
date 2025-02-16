<template>
  <video :ref="el => videoElement = el as HTMLVideoElement" style="display: none;" @ended="handleEnded"
    @timeupdate="handleTimeUpdate" @loadedmetadata="handleLoadedMetadata" />
</template>

<script setup lang="ts">
import Hls from 'hls.js';
import { onBeforeUnmount, ref, watch } from 'vue';
import { useAudioStore, type AudioTrack } from '../stores/audio';

const props = defineProps<{ fileID: string }>()
const audioStore = useAudioStore()
const videoElement = ref<HTMLVideoElement | null>(null)

const MIN_SEEK_SKEW = 0.5

const MIN_VOLUME_SKEW = 0.01
const FADE_DURATION = 2000 // 2 seconds
const FADE_STEP_DURATION = 16 // 16ms per step
const FADE_STEPS = Math.ceil(FADE_DURATION / FADE_STEP_DURATION)
let fadeTimer: number | undefined = undefined

watch(videoElement, (el) => {
  if (el) {
    console.log("registering video element", props.fileID)
    startAudioSync(props.fileID, el)
  }
})

onBeforeUnmount(() => {
  if (fadeTimer !== null) {
    clearInterval(fadeTimer)
  }
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
  watch(() => audioStore.tracks[fileID].isRepeating, () => {
    syncRepeating(fileID, videoElement)
  })

  watch(() => audioStore.tracks[fileID].currentTime, () => {
    syncCurrentTime(fileID, videoElement)
  })

  watch(() => audioStore.tracks[fileID].isPlaying, () => {
    syncIsPlaying(fileID, videoElement)
  })

  watch(() => audioStore.tracks[fileID].volume, () => {
    syncVolume(fileID, videoElement)
  })
}

function syncCurrentTime(fileID: string, videoElement: HTMLVideoElement) {
  const state = audioStore.tracks[fileID]

  // Only seek if difference is significant
  if (Math.abs(videoElement.currentTime - state.currentTime) > MIN_SEEK_SKEW) {
    videoElement.currentTime = state.currentTime
  }
}

function syncRepeating(fileID: string, videoElement: HTMLVideoElement) {
  const state = audioStore.tracks[fileID]
  videoElement.loop = state.isRepeating
}

function syncVolume(fileID: string, videoElement: HTMLVideoElement) {
  const desiredState = audioStore.tracks[fileID]
  const currentVolume = videoElement.volume
  let desiredVolume = (desiredState.volume / 100) * (audioStore.masterVolume / 100)
  if (!desiredState.isPlaying && !videoElement.paused) {
    desiredVolume = 0
  }

  if (videoElement.paused) {
    // If our video is paused, we don't need to fade anything
    videoElement.volume = desiredVolume
    return
  }

  if (Math.abs(currentVolume - desiredVolume) > MIN_VOLUME_SKEW) {
    audioStore.setFading(props.fileID, true)

    // Clear any existing fade timers to start a new one
    if (fadeTimer !== undefined) {
      clearInterval(fadeTimer)
    }

    // Start fade if volume is different
    let currentFadeStep = 0
    fadeTimer = setInterval(() => {
      currentFadeStep++
      if (currentFadeStep >= FADE_STEPS) {
        // We're done fading; stop the video if desired and clear the timer
        if (!desiredState.isPlaying) {
          videoElement.pause()
        }
        // Stop the loop once we've reached the desired volume
        clearInterval(fadeTimer)
        fadeTimer = undefined
        audioStore.setFading(props.fileID, false)
      }

      const fadePercent = currentFadeStep / FADE_STEPS
      const newVolume = desiredVolume * fadePercent + currentVolume * (1 - fadePercent)
      videoElement.volume = newVolume
    }, FADE_STEP_DURATION)
  }
}

function syncIsPlaying(fileID: string, videoElement: HTMLVideoElement) {
  const desiredState = audioStore.tracks[fileID]

  if (desiredState.isPlaying && videoElement.paused) {
    videoElement.volume = 0
    videoElement.play()
  }

  // Sync volume, since we may need to update it if we're fading out to a pause
  syncVolume(props.fileID, videoElement)
}

function syncStateToVideoElement(desiredState: AudioTrack, videoElement: HTMLVideoElement) {
  syncIsPlaying(props.fileID, videoElement)
  syncRepeating(props.fileID, videoElement)
  syncCurrentTime(props.fileID, videoElement)
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
