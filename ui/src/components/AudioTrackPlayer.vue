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
      syncAll(videoElement)
    })
    hls.on(Hls.Events.LEVEL_LOADED, (_, data) => {
      // Once we figure out the total duration, update the store
      audioStore.updateTrackState(fileID, { duration: data.details.totalduration })
    })
  } else if (videoElement.canPlayType('application/vnd.apple.mpegurl')) {
    videoElement.src = `/api/v1/stream/${fileID}/index.m3u8`
    syncAll(videoElement)
  }

  // Watch state and sync to video element
  watch(() => audioStore.tracks[fileID].isPlaying, () => {
    syncIsPlaying(fileID, videoElement)
  })

  watch(() => audioStore.tracks[fileID].volume, () => {
    syncVolume(fileID, videoElement)
  })

  watch(() => audioStore.masterVolume, () => {
    if (fadeTimer) return

    syncVolumeImmediate(fileID, videoElement)
  })

  watch(() => audioStore.typeVolumes, () => {
    if (fadeTimer) return

    syncVolumeImmediate(fileID, videoElement)
  }, { deep: true })

  watch(() => audioStore.tracks[fileID].isRepeating, () => {
    syncRepeating(fileID, videoElement)
  })

  watch(() => audioStore.tracks[fileID].currentTime, () => {
    syncCurrentTime(fileID, videoElement)
  })
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

function getDesiredVolume(desiredState: AudioTrack) {
  if (!desiredState.isPlaying) {
    return 0
  }

  return desiredState.volume / 100
}

function getVolumeMultiplier() {
  const typeVolume = audioStore.getTypeVolume(audioStore.tracks[props.fileID].trackType) ?? 100
  return (audioStore.masterVolume / 100) * (typeVolume / 100)
}

function syncVolume(fileID: string, videoElement: HTMLVideoElement) {
  const desiredState = audioStore.tracks[fileID]
  let volumeMultiplier = getVolumeMultiplier()
  if (volumeMultiplier < 0.01) {
    volumeMultiplier = 0.01
  }

  const currentVolume = videoElement.volume / getVolumeMultiplier()
  let desiredVolume = getDesiredVolume(desiredState)

  if (videoElement.paused) {
    // If our video is paused, we don't need to fade anything
    setVolume(desiredVolume * volumeMultiplier)
    return
  }

  if (!isFadeable(fileID)) {
    // If we're not fadeable, just set the volume directly
    setVolume(desiredVolume * volumeMultiplier)
    return
  }

  if (Math.abs(currentVolume - desiredVolume) > MIN_VOLUME_SKEW) {
    // Only start a fade if the desired volume is sufficiently different

    // Clear any existing fade timers to start a new one
    stopFade()
    audioStore.setFading(props.fileID, true)

    // Start fade if volume is different
    let currentFadeStep = 0
    fadeTimer = setInterval(() => {
      currentFadeStep++
      if (currentFadeStep >= FADE_STEPS) {
        // We're done fading; stop the video if desired and clear the timer
        if (!desiredState.isPlaying) {
          videoElement.pause()
        }
        stopFade()
      }

      const fadePercent = currentFadeStep / FADE_STEPS
      const newVolume = (getDesiredVolume(desiredState) * fadePercent + currentVolume * (1 - fadePercent)) * getVolumeMultiplier()
      setVolume(newVolume)
    }, FADE_STEP_DURATION)
  }
}

function syncVolumeImmediate(fileID: string, videoElement: HTMLVideoElement) {
  const desiredState = audioStore.tracks[fileID]
  const desiredVolume = getDesiredVolume(desiredState) * getVolumeMultiplier()
  setVolume(desiredVolume)
}

function syncRepeating(fileID: string, videoElement: HTMLVideoElement) {
  const state = audioStore.tracks[fileID]
  videoElement.loop = state.isRepeating
}

function syncCurrentTime(fileID: string, videoElement: HTMLVideoElement) {
  const state = audioStore.tracks[fileID]

  // Only seek if difference is significant
  if (Math.abs(videoElement.currentTime - state.currentTime) > MIN_SEEK_SKEW) {
    videoElement.currentTime = state.currentTime
  }
}

function syncAll(videoElement: HTMLVideoElement) {
  syncIsPlaying(props.fileID, videoElement)
  // syncIsPlaying already calls syncVolume, so we don't need to call it again
  syncRepeating(props.fileID, videoElement)
  syncCurrentTime(props.fileID, videoElement)
}

function stopFade() {
  if (fadeTimer) {
    clearInterval(fadeTimer)
    fadeTimer = undefined
  }

  audioStore.setFading(props.fileID, false)
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

function isFadeable(trackID: string) {
  const track = audioStore.tracks[trackID]
  if (!track) {
    return false
  }

  // FIXME: This is a hack until we can better distinguish between fadeable and non-fadeable tracks
  return track.isRepeating
}

function setVolume(newVolume: number) {
  if (!videoElement.value) {
    return
  }

  if (newVolume < 0) {
    newVolume = 0;
  } else if (newVolume > 1) {
    newVolume = 1;
  }

  videoElement.value.volume = newVolume;
}
</script>
