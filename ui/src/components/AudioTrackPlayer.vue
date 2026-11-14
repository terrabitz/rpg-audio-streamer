<template>
  <video ref="primary" :data-active="active === 0"
    style="display: none;" @ended="handleEnded(0)" @timeupdate="handleTimeUpdate(0)"
    @loadedmetadata="handleMetadata(0)" />
  <audio ref="secondary" :data-active="active === 1"
    style="display: none;" @ended="handleEnded(1)" @timeupdate="handleTimeUpdate(1)"
    @loadedmetadata="handleMetadata(1)" />
</template>

<script setup lang="ts">
import Hls, { type HlsConfig } from 'hls.js'
import { onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { defaultPlayback, playbackError, playbackRegion } from '@/composables/playback'
import { useAudioStore, type AudioTrack } from '../stores/audio'

const props = defineProps<{ fileID: string, token?: string }>()
const emit = defineEmits<{ error: [message: string] }>()
const audioStore = useAudioStore()
const primary = ref<HTMLVideoElement | null>(null)
const secondary = ref<HTMLAudioElement | null>(null)
const active = ref(0)
const players: Hls[] = []
const ready = [false, false]
const bound = [false, false]
let timer: ReturnType<typeof setInterval> | undefined
let disposed = false
let overlap: { outgoing: number, incoming: number, start: number, end: number } | undefined
let envelope = 0
let fade: { from: number, to: number, start: number } | undefined
let mixedVolume = targetVolume()
let volumeFade: { from: number, to: number, start: number } | undefined

watch(() => audioStore.tracks[props.fileID]?.isPlaying, syncPlaying)
watch(() => audioStore.tracks[props.fileID]?.currentTime, () => {
  const state = audioStore.tracks[props.fileID]
  const media = element(active.value)
  if (!state || !media || !ready[active.value]) return
  if (Math.abs(media.currentTime - state.currentTime) > 0.5) {
    cancelOverlap()
    media.currentTime = state.currentTime
  }
})
watch([
  () => audioStore.tracks[props.fileID]?.playback?.startTime,
  () => audioStore.tracks[props.fileID]?.playback?.endTime,
  () => audioStore.tracks[props.fileID]?.playback?.loopBackTime,
], syncRegion)
watch(() => audioStore.tracks[props.fileID]?.isRepeating, (isRepeating) => {
  cancelOverlap()
  if (isRepeating && audioStore.tracks[props.fileID]?.isPlaying) bindHLS(1)
  tick()
})
watch(targetVolume, (target) => {
  const now = performance.now()
  advanceVolumeFade(now)
  volumeFade = mixedVolume !== target ? { from: mixedVolume, to: target, start: now } : undefined
})

onMounted(() => {
  if (!Hls.isSupported()) {
    reportPlaybackError(new Error('HLS.js is not supported in this browser.'))
    return
  }
  bindHLS(0)
  timer = setInterval(tick, 16)
})

onBeforeUnmount(() => {
  disposed = true
  clearInterval(timer)
  for (const player of players) player.destroy()
  for (const media of [primary.value, secondary.value]) {
    if (!media) continue
    media.pause()
    media.src = ''
  }
  if (audioStore.tracks[props.fileID]) audioStore.setFading(props.fileID, false)
})

function bindHLS(index: number) {
  if (bound[index]) return
  const media = element(index)
  if (!media) return
  bound[index] = true
  const options: Partial<HlsConfig> = props.token ? {
    xhrSetup(xhr) { xhr.setRequestHeader('Authorization', `Bearer ${props.token}`) },
  } : {}
  const player = new Hls(options)
  players.push(player)
  media.volume = 0
  player.loadSource(`/api/v1/stream/${props.fileID}/index.m3u8`)
  player.attachMedia(media)
  player.on(Hls.Events.MANIFEST_PARSED, () => {
    if (disposed) return
    ready[index] = true
    if (index === active.value) {
      syncPosition()
      syncPlaying()
    }
  })
  player.on(Hls.Events.LEVEL_LOADED, (_, data) => updateDuration(data.details.totalduration))
  player.on(Hls.Events.ERROR, (_, data) => {
    if (data.fatal) reportPlaybackError(data)
  })
}

function syncPlaying() {
  const state = audioStore.tracks[props.fileID]
  const media = element(active.value)
  if (!state || !media || !ready[active.value] || disposed) return
  if (state.isPlaying && state.isRepeating) bindHLS(1)
  if (!state.isPlaying) cancelOverlap()
  const target = state.isPlaying ? 1 : 0
  fade = state.isRepeating && envelope !== target
    ? { from: envelope, to: target, start: performance.now() } : undefined
  if (!fade) envelope = target
  if (state.isPlaying && media.paused) {
    syncPosition()
    void media.play().catch(reportPlaybackError)
  }
  tick()
}

function syncRegion() {
  const state = audioStore.tracks[props.fileID]
  const media = element(active.value)
  if (!state || !media) return
  const { start, end, loop } = playbackRegion(state.playback, state.duration)
  if (overlap) {
    // An in-flight handoff must survive edits, even if the outgoing source has ended.
    overlap.end = Math.min(overlap.end, end)
    const incoming = element(overlap.incoming)
    if (incoming) {
      const position = incoming.currentTime >= end ? loop : Math.max(start, incoming.currentTime)
      if (incoming.currentTime !== position) incoming.currentTime = position
    }
  }
  syncPosition(media.currentTime)
}

function syncPosition(currentTime?: number) {
  const state = audioStore.tracks[props.fileID]
  const media = element(active.value)
  if (!state || !media) return
  const { start, end } = playbackRegion(state.playback, state.duration)
  const time = Math.max(start, currentTime ?? state.currentTime)
  const position = end > start ? Math.min(time, end) : time
  if (media.currentTime !== position) media.currentTime = position
  updatePlaybackState({ currentTime: media.currentTime })
}

function tick() {
  const state = audioStore.tracks[props.fileID]
  const media = element(active.value)
  if (disposed || !state || !media || !ready[active.value]) return
  advanceVolumeFade(performance.now())
  if (fade) {
    const progress = Math.min(1, (performance.now() - fade.start) / 2000)
    envelope = fade.from + (fade.to - fade.from) * progress
    if (progress === 1) fade = undefined
  }
  if (!state.isPlaying) {
    if (!fade) media.pause()
    applyVolumes()
    audioStore.setFading(props.fileID, !!fade || !!volumeFade)
    return
  }
  const region = playbackRegion(state.playback, state.duration)
  if (region.end <= 0) return
  const error = playbackError(state.playback ?? defaultPlayback(), state.duration)
  if (error) {
    reportPlaybackError(new Error(error))
    return
  }
  if (state.isRepeating && !overlap && ready[1 - active.value] &&
      (media.ended || media.currentTime >= region.end ||
        !media.paused && media.currentTime >= region.end - region.fade)) {
    startOverlap(region.loop, region.end)
  }
  if (overlap && (media.ended || media.currentTime >= overlap.end)) finishOverlap()
  else if (!state.isRepeating && media.currentTime >= region.end) finishTrack()
  applyVolumes()
  if (audioStore.tracks[props.fileID]) audioStore.setFading(props.fileID, !!fade || !!overlap || !!volumeFade)
}

function advanceVolumeFade(now: number) {
  if (!volumeFade) return
  const progress = Math.min(1, (now - volumeFade.start) / 2000)
  mixedVolume = volumeFade.from + (volumeFade.to - volumeFade.from) * progress
  if (progress === 1) volumeFade = undefined
}

function targetVolume() {
  const state = audioStore.tracks[props.fileID]
  if (!state) return 0
  return Math.max(0, state.volume / 100 * audioStore.mixMasterVolume / 100 *
    audioStore.getTypeVolume(state.trackType) / 100)
}

function startOverlap(loop: number, end: number) {
  const outgoing = element(active.value)
  const incoming = element(1 - active.value)
  if (!outgoing || !incoming) return
  incoming.currentTime = loop
  incoming.volume = 0
  overlap = { outgoing: active.value, incoming: 1 - active.value, start: outgoing.currentTime, end }
  void incoming.play().catch(reportPlaybackError)
}

function finishOverlap() {
  if (!overlap) return
  const incoming = element(overlap.incoming)
  // Keep the outgoing element until the replacement has actually started.
  if (!incoming || incoming.paused || incoming.readyState < 2) return
  element(overlap.outgoing)?.pause()
  active.value = overlap.incoming
  overlap = undefined
  updatePlaybackState({ currentTime: incoming.currentTime })
}

function cancelOverlap() {
  if (!overlap) return
  element(overlap.incoming)?.pause()
  overlap = undefined
  applyVolumes()
}

function applyVolumes() {
  const state = audioStore.tracks[props.fileID]
  if (!state) return
  const volume = mixedVolume * envelope
  const outgoing = overlap ? element(overlap.outgoing) : undefined
  const incoming = overlap ? element(overlap.incoming) : undefined
  const progress = overlap && outgoing && incoming && !incoming.paused && incoming.readyState >= 2
    ? Math.min(1, Math.max(0, (outgoing.currentTime - overlap.start) / (overlap.end - overlap.start || 1))) : 0
  for (const index of [0, 1]) {
    const media = element(index)
    if (!media) continue
    const gain = overlap
      ? index === overlap.outgoing ? 1 - progress : progress
      : index === active.value ? 1 : 0
    media.volume = Math.min(1, Math.max(0, volume * gain))
  }
}

function handleEnded(index: number) {
  if (disposed || index !== active.value || !audioStore.tracks[props.fileID]) return
  if (audioStore.tracks[props.fileID]?.isRepeating) {
    tick()
    finishOverlap()
  } else finishTrack()
}

function finishTrack() {
  cancelOverlap()
  const media = element(active.value)
  media?.pause()
  const start = audioStore.tracks[props.fileID]?.playback?.startTime ?? 0
  if (media) media.currentTime = start
  envelope = 0
  fade = undefined
  updatePlaybackState({ isPlaying: false, currentTime: start })
}

function handleTimeUpdate(index: number) {
  if (index !== active.value || disposed) return
  tick()
  const media = element(active.value)
  if (media) updatePlaybackState({ currentTime: media.currentTime })
}

function handleMetadata(index: number) {
  const media = element(index)
  if (media) updateDuration(media.duration)
}

function updateDuration(duration: number) {
  if (Number.isFinite(duration) && duration > 0) updatePlaybackState({ duration })
}

function element(index: number): HTMLMediaElement | null {
  return index === 0 ? primary.value : secondary.value
}

function updatePlaybackState(updates: Partial<AudioTrack>) {
  if (disposed || !audioStore.tracks[props.fileID]) return
  const state = audioStore.tracks[props.fileID]
  if (Object.entries(updates).every(([key, value]) => state[key as keyof AudioTrack] === value)) return
  audioStore.updateTrackState(props.fileID, updates)
}

function reportPlaybackError(cause: unknown) {
  if (disposed || !audioStore.tracks[props.fileID]) return
  console.error(`Failed to play track ${props.fileID}:`, cause)
  cancelOverlap()
  fade = undefined
  envelope = 0
  primary.value?.pause()
  secondary.value?.pause()
  updatePlaybackState({ isPlaying: false })
  emit('error', 'Unable to play track. Try again or choose another track.')
}
</script>
