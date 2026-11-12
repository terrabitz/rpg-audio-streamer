<template>
  <video :ref="el => videoElement = el as HTMLVideoElement" style="display: none;" @ended="handleEnded"
    @timeupdate="handleTimeUpdate" @loadedmetadata="handleLoadedMetadata" />
</template>

<script setup lang="ts">
import Hls, { type HlsConfig } from 'hls.js';
import { onBeforeUnmount, ref, shallowRef, watch } from 'vue';
import { useAudioStore, type AudioTrack } from '../stores/audio';

const props = defineProps<{ fileID: string, token?: string }>()
const emit = defineEmits<{ error: [message: string] }>()
const audioStore = useAudioStore()
const videoElement = shallowRef<HTMLVideoElement | null>(null)

const MIN_SEEK_SKEW = 0.5

const MIN_VOLUME_SKEW = 0.01
const FADE_DURATION = 2000 // 2 seconds
const FADE_STEP_DURATION = 16 // 16ms per step
const FADE_STEPS = Math.ceil(FADE_DURATION / FADE_STEP_DURATION)
let fadeTimer: ReturnType<typeof setInterval> | undefined
let resetTimer: ReturnType<typeof setTimeout> | undefined
let hls: Hls | undefined
const hlsReady = ref(false)

watch(videoElement, async (el) => {
  if (!el) return

  console.log("registering video element", props.fileID)
  await startAudioSync(props.fileID, el)
})

watch(() => audioStore.tracks[props.fileID]?.isPlaying, () => {
  if (!hlsReady.value || !videoElement.value) return
  syncIsPlaying(props.fileID, videoElement.value)
})

watch(() => audioStore.tracks[props.fileID]?.volume, () => {
  if (!hlsReady.value || !videoElement.value) return
  syncVolume(props.fileID, videoElement.value)
})

watch(() => audioStore.masterVolume, () => {
  if (!hlsReady.value || fadeTimer) return
  syncVolumeImmediate(props.fileID)
})

watch(() => audioStore.typeVolumes, () => {
  if (!hlsReady.value || fadeTimer) return
  syncVolumeImmediate(props.fileID)
}, { deep: true })

watch(() => audioStore.tracks[props.fileID]?.isRepeating, () => {
  if (!hlsReady.value || !videoElement.value) return
  syncRepeating(props.fileID, videoElement.value)
})

watch(() => audioStore.tracks[props.fileID]?.currentTime, () => {
  if (!hlsReady.value || !videoElement.value) return
  syncCurrentTime(props.fileID, videoElement.value)
})

onBeforeUnmount(() => {
  if (fadeTimer !== undefined) {
    clearInterval(fadeTimer)
  }
  clearTimeout(resetTimer)
  hls?.destroy()
  if (videoElement.value) {
    videoElement.value.pause()
    videoElement.value.src = ''
  }
})

// startAudioSync sets up the HLS.js player
async function startAudioSync(fileID: string, videoElement: HTMLVideoElement) {
  const source = `/api/v1/stream/${fileID}/index.m3u8`
  if (!Hls.isSupported()) {
    reportPlaybackError(new Error('HLS.js is not supported in this browser.'))
    return
  }

  bindHLS(source, props.token, videoElement)
}

function bindHLS(source: string, token: string | undefined, videoElement: HTMLVideoElement): void {
  let options: Partial<HlsConfig> = {}
  if (token) {
    options = {
      xhrSetup: function (xhr) {
        xhr.setRequestHeader('Authorization', `Bearer ${token}`)
      }
    }
  }
  hls = new Hls(options)
  hls.loadSource(source)
  hls.attachMedia(videoElement)
  hls.on(Hls.Events.MANIFEST_PARSED, () => {
    syncAll(videoElement)
    hlsReady.value = true
  })
  hls.on(Hls.Events.LEVEL_LOADED, (_, data) => {
    // Once we figure out the total duration, update the store
    updateDuration(data.details.totalduration)
  })
  hls.on(Hls.Events.ERROR, (_, data) => {
    if (data.fatal) reportPlaybackError(data)
  })
}

function updateDuration(duration: number) {
  updatePlaybackState({ duration })
}

function syncIsPlaying(fileID: string, videoElement: HTMLVideoElement) {
  const desiredState = audioStore.tracks[fileID]

  if (desiredState?.isPlaying && videoElement.paused) {
    videoElement.volume = 0
    videoElement.play().catch((err) => {
      reportPlaybackError(err)
    })
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
  const track = audioStore.tracks[props.fileID]
  if (!track) {
    return 1.0
  }

  const typeVolume = audioStore.getTypeVolume(track.trackType) ?? 100
  return (audioStore.masterVolume / 100) * (typeVolume / 100)
}

function syncVolume(fileID: string, videoElement: HTMLVideoElement) {
  const desiredState = audioStore.tracks[fileID]
  if (!desiredState) {
    return
  }

  let volumeMultiplier = getVolumeMultiplier()
  if (volumeMultiplier < 0.01) {
    volumeMultiplier = 0.01
  }

  const currentVolume = videoElement.volume / getVolumeMultiplier()
  const desiredVolume = getDesiredVolume(desiredState)

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

function syncVolumeImmediate(fileID: string) {
  const desiredState = audioStore.tracks[fileID]
  if (!desiredState) {
    return
  }

  const desiredVolume = getDesiredVolume(desiredState) * getVolumeMultiplier()
  setVolume(desiredVolume)
}

function syncRepeating(fileID: string, videoElement: HTMLVideoElement) {
  const state = audioStore.tracks[fileID]
  if (!state) {
    return
  }

  videoElement.loop = state.isRepeating
}

function syncCurrentTime(fileID: string, videoElement: HTMLVideoElement) {
  const state = audioStore.tracks[fileID]
  if (!state) {
    return
  }

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
  updatePlaybackState({ isPlaying: false })
  videoElement.pause()
  resetTimer = setTimeout(() => {
    videoElement.currentTime = 0
    updatePlaybackState({ currentTime: 0 })
  }, 0)
}

function handleTimeUpdate(evt: Event) {
  const videoElement = evt.target as HTMLVideoElement
  updatePlaybackState({ currentTime: videoElement.currentTime })
}

function handleLoadedMetadata(evt: Event) {
  const videoElement = evt.target as HTMLVideoElement
  updatePlaybackState({ duration: videoElement.duration })
}

function updatePlaybackState(updates: Partial<AudioTrack>) {
  // Queued media events must not recreate a track after deletion.
  if (!audioStore.tracks[props.fileID]) return
  audioStore.updateTrackState(props.fileID, updates)
}

function reportPlaybackError(cause: unknown) {
  console.error(`Failed to play track ${props.fileID}:`, cause)
  updatePlaybackState({ isPlaying: false })
  emit('error', 'Unable to play track. Try again or choose another track.')
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
