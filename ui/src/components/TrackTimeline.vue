<template>
  <div class="track-timeline" :class="{ 'is-disabled': unavailable }" @click.stop>
    <VolumeSlider v-if="volume !== undefined" :model-value="volume" :disabled="disabled" :aria-label="`Volume for ${name}`"
      thumb-size="18" track-size="6" class="timeline-volume ma-0"
      @update:model-value="$emit('volume', $event)" />
    <div ref="axis" class="timeline-axis" @pointerdown="startDrag($event, 'seek')"
      @pointermove="moveDrag" @pointerup="finishDrag" @pointercancel="cancelDrag">
      <div class="timeline-usable" :style="{ left: percent(modelValue.startTime), right: percent(duration - end) }" />
      <div class="timeline-progress" :style="{ left: percent(modelValue.startTime), width: percent(position - modelValue.startTime) }" />
      <button v-for="point in points" :key="point.key" type="button" role="slider"
        class="timeline-handle" :class="[
          `timeline-handle--${point.key}`,
          { 'timeline-handle--anchored': point.key === 'loop' && modelValue.loopBackTime === null },
        ]"
        :style="{ left: percent(point.value) }" :disabled="unavailable"
        :aria-label="`${point.label} for ${name}`" :aria-valuemin="point.min" :aria-valuemax="point.max"
        :aria-valuenow="point.value" :aria-valuetext="valueText(point.key, point.value)"
        :title="`${point.label}: ${valueText(point.key, point.value)}`" aria-orientation="horizontal"
        @pointerdown.stop="startDrag($event, point.key)" @pointermove.stop="moveDrag"
        @pointerup.stop="finishDrag" @pointercancel.stop="cancelDrag" @keydown="handleKey($event, point.key)">
        <span aria-hidden="true">{{ formatTime(point.value, true) }}</span>
      </button>
      <button type="button" role="slider" class="timeline-cursor" :style="{ left: percent(position) }"
        :disabled="unavailable" :aria-label="`Playback position for ${name}`"
        :aria-valuemin="modelValue.startTime" :aria-valuemax="end" :aria-valuenow="position"
        :aria-valuetext="`${formatTime(position)} of ${formatTime(duration)}`" aria-orientation="horizontal"
        @pointerdown.stop="startDrag($event, 'seek')" @pointermove.stop="moveDrag"
        @pointerup.stop="finishDrag" @pointercancel.stop="cancelDrag" @keydown="handleKey($event, 'seek')" />
    </div>
    <span class="position-time text-caption">{{ formatTime(currentTime) }} / {{ formatTime(duration) }}</span>
    <p v-if="unavailable && !disabled" class="text-caption mt-2" role="status">Loading audio timeline…</p>
  </div>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import type { PlaybackSettings } from '@/client/apiClient'
import { movePlaybackPoint, type PlaybackPoint } from '@/composables/playback'
import VolumeSlider from './VolumeSlider.vue'

type TimelinePoint = PlaybackPoint | 'seek'
const props = defineProps<{
  modelValue: PlaybackSettings
  duration: number
  currentTime: number
  name: string
  volume?: number
  isRepeating?: boolean
  disabled?: boolean
}>()
const emit = defineEmits<{
  'update:modelValue': [settings: PlaybackSettings]
  seek: [time: number]
  volume: [volume: number]
}>()
const axis = ref<HTMLElement | null>(null)
const drag = ref<{
  point: TimelinePoint
  pointerID: number
  offset: number
  settings: PlaybackSettings
  time: number
} | null>(null)
const unavailable = computed(() => props.disabled || !Number.isFinite(props.duration) || props.duration <= 0)
const end = computed(() => Math.min(props.modelValue.endTime ?? props.duration, props.duration))
const loop = computed(() => props.modelValue.loopBackTime ?? props.modelValue.startTime)
const position = computed(() => Math.max(props.modelValue.startTime, Math.min(props.currentTime, end.value)))
const gap = computed(() => Math.min(0.1, props.duration))
const points = computed(() => [
  { key: 'start' as const, label: 'Beginning', value: props.modelValue.startTime, min: 0, max: end.value - gap.value },
  { key: 'end' as const, label: 'End', value: end.value, min: props.modelValue.startTime + gap.value, max: props.duration },
  ...(props.isRepeating ? [
    { key: 'loop' as const, label: 'Loop-back point', value: loop.value, min: props.modelValue.startTime, max: end.value - gap.value },
  ] : []),
])

function startDrag(event: PointerEvent, point: TimelinePoint) {
  if (unavailable.value || drag.value || event.button !== 0 || !(event.currentTarget instanceof HTMLElement)) return
  const time = pointerTime(event)
  if (time === null) return
  event.preventDefault()
  event.currentTarget.focus()
  event.currentTarget.setPointerCapture(event.pointerId)
  drag.value = {
    point, pointerID: event.pointerId, settings: { ...props.modelValue }, time: props.currentTime,
    offset: event.currentTarget === axis.value ? 0 : pointValue(point) - time,
  }
  moveDrag(event)
}

function moveDrag(event: PointerEvent) {
  if (!drag.value || drag.value.pointerID !== event.pointerId || unavailable.value) return
  const time = pointerTime(event)
  if (time !== null) movePoint(drag.value.point, time + drag.value.offset)
}

function finishDrag() {
  drag.value = null
}

function cancelDrag() {
  if (!drag.value) return
  if (drag.value.point === 'seek') emit('seek', drag.value.time)
  else emit('update:modelValue', drag.value.settings)
  drag.value = null
}

function handleKey(event: KeyboardEvent, point: TimelinePoint) {
  if (event.key === 'Escape' && drag.value) {
    event.preventDefault()
    cancelDrag()
    return
  }
  if (unavailable.value) return
  const bounds = point === 'end'
    ? { min: props.modelValue.startTime + gap.value, max: props.duration }
    : {
      min: point === 'start' ? 0 : props.modelValue.startTime,
      max: point === 'seek' ? end.value : end.value - gap.value,
    }
  const step = point === 'seek' || event.shiftKey ? 1 : 0.1
  const time = pointValue(point)
  let next: number
  switch (event.key) {
    case 'ArrowRight': case 'ArrowUp': next = time + step; break
    case 'ArrowLeft': case 'ArrowDown': next = time - step; break
    case 'PageUp': next = time + 5; break
    case 'PageDown': next = time - 5; break
    case 'Home': next = bounds.min; break
    case 'End': next = bounds.max; break
    default: return
  }
  event.preventDefault()
  movePoint(point, Math.max(bounds.min, Math.min(next, bounds.max)))
}

function movePoint(point: TimelinePoint, time: number) {
  if (point === 'seek') {
    const start = props.modelValue.startTime
    emit('seek', Math.max(start, Math.min(start + Math.round(time - start), end.value)))
    return
  }
  emit('update:modelValue', movePlaybackPoint(props.modelValue, props.duration, point, time))
}

function pointValue(point: TimelinePoint) {
  if (point === 'start') return props.modelValue.startTime
  if (point === 'end') return end.value
  if (point === 'loop') return loop.value
  return position.value
}

function pointerTime(event: PointerEvent) {
  const bounds = axis.value?.getBoundingClientRect()
  if (!bounds || bounds.width <= 0) return null
  return (event.clientX - bounds.left) / bounds.width * props.duration
}

function percent(time: number) {
  return `${props.duration > 0 ? Math.max(0, Math.min(100, time / props.duration * 100)) : 0}%`
}

function valueText(point: PlaybackPoint, time: number) {
  if (point === 'loop' && props.modelValue.loopBackTime === null) return `Beginning (${formatTime(time, true)})`
  return formatTime(time, true)
}

function formatTime(time: number, precise = false) {
  const seconds = precise ? Math.round(Math.max(0, time) * 10) / 10 : Math.floor(Math.max(0, time))
  const minutes = Math.floor(seconds / 60)
  const remainder = precise ? (seconds % 60).toFixed(1).padStart(4, '0') : Math.floor(seconds % 60).toString().padStart(2, '0')
  return `${minutes}:${remainder}`
}
</script>

<style scoped>
.track-timeline {
  display: grid;
  grid-template-columns: 64px minmax(0, 1fr) max-content;
  column-gap: 12px;
  align-items: center;
}

.position-time {
  grid-column: 3;
  grid-row: 2;
  white-space: nowrap;
  font-variant-numeric: tabular-nums;
}

.timeline-volume {
  grid-column: 2;
  grid-row: 1;
  position: relative;
  min-width: 0;
}

.timeline-volume :deep(.v-input__prepend) {
  position: absolute;
  inset-inline-end: calc(100% + 12px);
  margin-inline-end: 0;
}

.timeline-volume :deep(.v-input__control) {
  grid-column: 1 / -1;
  margin-inline: 0;
}

.timeline-volume :deep(.v-slider-track__fill) {
  height: var(--v-slider-track-size);
}

.timeline-volume :deep(.v-slider-thumb__surface::after) {
  width: var(--v-slider-thumb-size);
}

.timeline-volume :deep(.v-slider-thumb__ripple) {
  inset: 0;
  width: 100%;
  height: 100%;
}

.timeline-axis {
  grid-column: 2;
  grid-row: 2;
  position: relative;
  height: 6px;
  margin-block: 48px;
  border-radius: 3px;
  background: rgba(var(--v-theme-on-surface), 0.12);
  touch-action: none;
  cursor: pointer;
}

.timeline-axis::before {
  content: '';
  position: absolute;
  inset: -12px 0;
}

.timeline-usable,
.timeline-progress {
  position: absolute;
  height: 100%;
  border-radius: inherit;
  pointer-events: none;
}

.timeline-usable {
  background: rgba(var(--v-theme-primary), 0.4);
}

.timeline-progress {
  background: rgb(var(--v-theme-primary));
}

.timeline-handle,
.timeline-cursor {
  position: absolute;
  transform: translateX(-50%);
  touch-action: none;
  cursor: ew-resize;
}

.timeline-handle {
  top: -44px;
  width: 64px;
  height: 32px;
  border: 1px solid rgb(var(--v-theme-primary));
  border-radius: 5px;
  background: rgb(var(--v-theme-surface));
  color: rgb(var(--v-theme-primary));
  font-size: 0.75rem;
  font-weight: 700;
  font-variant-numeric: tabular-nums;
  user-select: none;
}

.timeline-handle::after {
  content: '';
  position: absolute;
  left: calc(50% - 1px);
  top: 100%;
  width: 2px;
  height: 14px;
  background: currentColor;
  pointer-events: none;
}

.timeline-handle--start {
  transform: translateX(-100%);
}

.timeline-handle--start::after {
  left: calc(100% - 1px);
}

.timeline-handle--end {
  transform: none;
}

.timeline-handle--end::after {
  left: -1px;
}

.timeline-handle--loop {
  top: 24px;
  border-color: rgb(var(--v-theme-warning));
  color: rgb(var(--v-theme-warning));
}

.timeline-handle--loop::after {
  top: auto;
  bottom: 100%;
  height: 20px;
}

.timeline-handle--anchored {
  border-style: dashed;
}

.timeline-cursor {
  top: -6px;
  width: 18px;
  height: 18px;
  border: 3px solid rgb(var(--v-theme-surface));
  border-radius: 50%;
  background: rgb(var(--v-theme-primary));
  box-shadow: 0 0 0 1px rgb(var(--v-theme-primary));
}

.timeline-cursor::after {
  content: '';
  position: absolute;
  inset: -4px -12px;
}

.timeline-handle:focus-visible,
.timeline-cursor:focus-visible {
  outline: 2px solid rgb(var(--v-theme-on-surface));
  outline-offset: 3px;
  z-index: 1;
}

.position-time,
.is-disabled {
  opacity: 0.7;
}

[role="status"] {
  grid-column: 1 / -1;
}
</style>
