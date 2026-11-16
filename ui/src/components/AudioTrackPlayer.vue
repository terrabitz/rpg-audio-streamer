<template>
  <video ref="primary" :data-active="active === 0"
    style="display: none;" @ended="handleEnded(0)" @timeupdate="handleTimeUpdate(0)"
    @loadedmetadata="handleMetadata(0)" @playing="handlePlaying(0)" @pause="handlePlaying(0)" />
  <audio ref="secondary" :data-active="active === 1"
    style="display: none;" @ended="handleEnded(1)" @timeupdate="handleTimeUpdate(1)"
    @loadedmetadata="handleMetadata(1)" @playing="handlePlaying(1)" @pause="handlePlaying(1)" />
</template>

<script setup lang="ts">
import Hls from 'hls.js'
import { onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { defaultPlayback, playbackError, playbackRegion, seekLimit } from '@/composables/playback'
import { useAudioStore, type AudioTrack } from '../stores/audio'
import { useWebSocketStore } from '../stores/websocket'

const props = defineProps<{ fileID: string, tableID?: string }>()
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
let overlap: { outgoing: number, incoming: number, start: number, end: number, seekStart?: number } | undefined
let pendingSeek: number | undefined
let envelope = 0
let fade: { from: number, to: number, start: number } | undefined
let mixedVolume = targetVolume()
let volumeFade: { from: number, to: number, start: number } | undefined
let positionVersion = audioStore.tracks[props.fileID]?.positionVersion
let seekVersion = audioStore.tracks[props.fileID]?.seekVersion

watch(() => audioStore.tracks[props.fileID]?.isPlaying, syncPlaying)
watch([
  () => audioStore.tracks[props.fileID]?.currentTime,
  () => audioStore.tracks[props.fileID]?.positionVersion,
  () => audioStore.tracks[props.fileID]?.seekVersion,
], () => {
  const state = audioStore.tracks[props.fileID]
  if (!state) return
  const commanded = state.positionVersion !== positionVersion
  const seek = state.seekVersion !== undefined && state.seekVersion > 0 && state.seekVersion !== seekVersion
  positionVersion = state.positionVersion
  seekVersion = state.seekVersion
  const media = element(active.value)
  if (!media || !ready[active.value]) return
  if (seek && state.isPlaying && !media.paused) {
    cancelOverlap(overlapProgress() > 0)
    pendingSeek = state.seekTarget ?? state.currentTime
    bindHLS(1 - active.value)
    tick()
    return
  }
  // Running media owns its handoff timing; only an explicit command may interrupt it.
  if (!commanded && !seek && (state.positionVersion !== undefined || overlap?.seekStart !== undefined || pendingSeek !== undefined) &&
      state.isPlaying && !media.paused) return
  const time = seek
    ? Math.max(state.playback?.startTime ?? 0, Math.min(state.seekTarget ?? state.currentTime, seekLimit(state.playback, state.duration)))
    : state.currentTime
  if (seek || Math.abs(media.currentTime - time) > 0.5) {
    cancelOverlap()
    const current = element(active.value)
    if (current) current.currentTime = time
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
  audioStore.setLocalPlayback(props.fileID, false)
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
  const player = new Hls()
  players.push(player)
  media.volume = 0
  const streamURL = props.tableID
    ? `/api/v1/tables/${props.tableID}/stream/${props.fileID}/index.m3u8`
    : `/api/v1/stream/${props.fileID}/index.m3u8`
  player.loadSource(streamURL)
  player.attachMedia(media)
  player.on(Hls.Events.MANIFEST_PARSED, () => {
    if (disposed) return
    ready[index] = true
    if (index === active.value) {
      syncPosition()
      syncPlaying()
    }
    tick()
  })
  player.on(Hls.Events.LEVEL_LOADED, (_, data) => updateDuration(data.details.totalduration))
  player.on(Hls.Events.ERROR, (_, data) => {
    if (data.fatal) reportPlaybackError(data)
  })
}

function syncPlaying() {
  const state = audioStore.tracks[props.fileID]
  if (!state?.isPlaying) {
    const interruptedSeek = pendingSeek !== undefined || overlap?.seekStart !== undefined
    cancelOverlap()
    if (interruptedSeek) updatePlaybackState({ seekTarget: null, seekLanding: null })
  }
  const media = element(active.value)
  if (!state || !media || !ready[active.value] || disposed) return
  if (state.isPlaying) bindHLS(1 - active.value)
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
  if (end > start && state.seekTarget != null &&
      (state.seekTarget < start || state.seekTarget > seekLimit(state.playback, state.duration) ||
        state.seekLanding != null && state.seekLanding > end)) {
    updatePlaybackState({ seekTarget: null, seekLanding: null })
  }
  if (overlap) {
    // An in-flight handoff must survive edits, even if the outgoing source has ended.
    overlap.end = Math.min(overlap.end, end)
    const incoming = element(overlap.incoming)
    if (incoming) {
      const position = incoming.currentTime >= end ? loop : Math.max(start, incoming.currentTime)
      if (incoming.currentTime !== position) {
        if (overlap.seekStart !== undefined) overlap.seekStart += position - incoming.currentTime
        incoming.currentTime = position
      }
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
  updatePlaybackState({ currentTime: playbackElement()?.currentTime ?? media.currentTime })
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
  if (pendingSeek !== undefined && ready[1 - active.value]) {
    const target = Math.max(region.start, Math.min(pendingSeek, seekLimit(state.playback, state.duration)))
    pendingSeek = undefined
    const duration = Math.min(2, region.end - media.currentTime, region.end - region.start)
    if (duration > 0 && !media.paused) {
      const incoming = target - region.start <= 2 ? region.start : target - duration
      startOverlap(incoming, media.currentTime + duration, true)
    }
    else {
      media.currentTime = target
      updatePlaybackState({ currentTime: target })
    }
  }
  if (state.isRepeating && !overlap && ready[1 - active.value] &&
      (media.ended || media.currentTime >= region.end ||
        !media.paused && media.currentTime >= region.end - region.fade)) {
    startOverlap(region.loop, region.end)
  }
  if (overlap && (media.ended || media.currentTime >= region.end || overlapProgress() >= 1)) finishOverlap()
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

function startOverlap(time: number, end: number, seek = false) {
  const outgoing = element(active.value)
  const incoming = element(1 - active.value)
  if (!outgoing || !incoming) return
  incoming.currentTime = time
  incoming.volume = 0
  overlap = { outgoing: active.value, incoming: 1 - active.value, start: outgoing.currentTime, end,
    seekStart: seek ? time : undefined }
  void incoming.play().catch(reportPlaybackError)
}

function finishOverlap() {
  if (!overlap) return
  const incoming = element(overlap.incoming)
  // Keep the outgoing element until the replacement has actually started.
  if (!incoming || incoming.paused || incoming.seeking || incoming.readyState < 2) return
  const completedSeek = overlap.seekStart !== undefined
  element(overlap.outgoing)?.pause()
  active.value = overlap.incoming
  overlap = undefined
  updatePlaybackState({
    currentTime: incoming.currentTime, ...(completedSeek ? { seekTarget: null, seekLanding: null } : {}),
  })
}

function cancelOverlap(adoptSeek = true) {
  pendingSeek = undefined
  if (!overlap) return
  if (adoptSeek && overlap.seekStart !== undefined) {
    element(overlap.outgoing)?.pause()
    active.value = overlap.incoming
  } else element(overlap.incoming)?.pause()
  overlap = undefined
  applyVolumes()
}

function applyVolumes() {
  const state = audioStore.tracks[props.fileID]
  if (!state) return
  const volume = mixedVolume * envelope
  const progress = overlapProgress()
  for (const index of [0, 1]) {
    const media = element(index)
    if (!media) continue
    const gain = overlap
      ? index === overlap.outgoing ? 1 - progress : progress
      : index === active.value ? 1 : 0
    media.volume = Math.min(1, Math.max(0, volume * gain))
  }
}

function overlapProgress() {
  const outgoing = overlap ? element(overlap.outgoing) : undefined
  const incoming = overlap ? element(overlap.incoming) : undefined
  if (!overlap || !outgoing || !incoming || incoming.paused || incoming.seeking || incoming.readyState < 2) return 0
  const elapsed = overlap.seekStart !== undefined
    ? incoming.currentTime - overlap.seekStart : outgoing.currentTime - overlap.start
  return Math.min(1, Math.max(0, elapsed / (overlap.end - overlap.start || 1)))
}

function handleEnded(index: number) {
  if (disposed || index !== active.value || !audioStore.tracks[props.fileID]) return
  if (overlap || audioStore.tracks[props.fileID]?.isRepeating) {
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
  if (disposed) return
  if (index !== active.value && index !== overlap?.incoming) return
  tick()
  const media = playbackElement()
  if (media && pendingSeek === undefined) updatePlaybackState({ currentTime: media.currentTime })
}

function handlePlaying(index: number) {
  const media = element(index)
  if (index !== active.value || disposed || !media) return
  audioStore.setLocalPlayback(props.fileID, !media.paused)
}

function handleMetadata(index: number) {
  const media = element(index)
  if (media) updateDuration(media.duration)
}

function updateDuration(duration: number) {
  if (!Number.isFinite(duration) || duration <= 0 || duration === audioStore.tracks[props.fileID]?.duration) return
  updatePlaybackState({ duration })
  const ws = useWebSocketStore()
  if (audioStore.tableMix && audioStore.tracks[props.fileID]?.isPlaying && ws.isConnected) {
    ws.sendMessage('syncTrack', { fileID: props.fileID, duration })
  }
}

function element(index: number): HTMLMediaElement | null {
  return index === 0 ? primary.value : secondary.value
}

function playbackElement(): HTMLMediaElement | null {
  return element(overlap?.seekStart !== undefined ? overlap.incoming : active.value)
}

function updatePlaybackState(updates: Partial<AudioTrack>) {
  if (disposed || !audioStore.tracks[props.fileID]) return
  const state = audioStore.tracks[props.fileID]
  if (state.seekTarget != null && pendingSeek === undefined && overlap?.seekStart === undefined &&
      updates.currentTime !== undefined && updates.currentTime >= (state.seekLanding ?? state.seekTarget)) {
    updates = { ...updates, seekTarget: null, seekLanding: null }
  }
  if (Object.entries(updates).every(([key, value]) => state[key as keyof AudioTrack] === value)) return
  audioStore.updateTrackState(props.fileID, updates)
}

function reportPlaybackError(cause: unknown) {
  if (disposed || !audioStore.tracks[props.fileID]) return
  audioStore.setLocalPlayback(props.fileID, false)
  console.error(`Failed to play track ${props.fileID}:`, cause)
  cancelOverlap()
  fade = undefined
  envelope = 0
  primary.value?.pause()
  secondary.value?.pause()
  if (!audioStore.tableMix) updatePlaybackState({ isPlaying: false })
  emit('error', 'Unable to play track. Try again or choose another track.')
}
</script>
