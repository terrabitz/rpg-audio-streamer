import { expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import { nextTick } from 'vue'
import TrackTimeline from '../TrackTimeline.vue'
import { defaultPlayback } from '@/composables/playback'

it('separates the seek cursor and three region handles and exposes their bounds', async () => {
  const wrapper = await mountTimeline()
  expect(wrapper.findAll('[role="slider"]')).toHaveLength(4)
  expect(wrapper.get('[aria-label="Beginning for Forest"]').attributes('aria-valuenow')).toBe('0')
  expect(wrapper.get('[aria-label="End for Forest"]').attributes('aria-valuemax')).toBe('60')
  expect(wrapper.get('[aria-label="Loop-back point for Forest"]').attributes('aria-valuetext')).toBe('Beginning (0:00.0)')
  expect(wrapper.get('[aria-label="Beginning for Forest"]').text()).toBe('0:00.0')
  expect(wrapper.get('[aria-label="End for Forest"]').text()).toBe('1:00.0')
  expect(wrapper.get('[aria-label="Loop-back point for Forest"]').text()).toBe('0:00.0')
  expect(wrapper.find('.timeline-labels').exists()).toBe(false)
  expect(wrapper.find('.timeline-loop-label').exists()).toBe(false)
  expect(wrapper.find('.timeline-reset').exists()).toBe(false)
  expect(wrapper.find('.timeline-help').exists()).toBe(false)
  await wrapper.get('[aria-label="Beginning for Forest"]').trigger('keydown', { key: 'ArrowRight', shiftKey: true })
  await wrapper.get('[aria-label="Loop-back point for Forest"]').trigger('keydown', { key: 'ArrowRight' })
  expect(wrapper.props('modelValue')).toEqual({ startTime: 1, endTime: null, loopBackTime: 1.1 })
  expect(wrapper.get('[aria-label="Playback position for Forest"]').attributes('aria-valuemin')).toBe('1')
  expect(wrapper.get('.timeline-usable').attributes('style')).toContain('left: 1.666')
  expect(wrapper.emitted('seek')).toBeUndefined()
  wrapper.unmount()
})

it('keeps seeking inside the draft region and supports Home, End, and page keys', async () => {
  const wrapper = await mountTimeline()
  await wrapper.setProps({ modelValue: { startTime: 10, endTime: 25, loopBackTime: null }, currentTime: 15 })
  const cursor = wrapper.get('[aria-label="Playback position for Forest"]')
  await cursor.trigger('keydown', { key: 'Home' })
  await cursor.trigger('keydown', { key: 'ArrowRight' })
  await cursor.trigger('keydown', { key: 'End' })
  await cursor.trigger('keydown', { key: 'PageDown' })
  await cursor.trigger('keydown', { key: 'PageUp' })
  expect(wrapper.emitted('seek')).toEqual([[10], [11], [25], [20], [25]])
  expect(cursor.attributes('aria-valuetext')).toBe('0:25 of 1:00')
  wrapper.unmount()
})

it('shares volume changes without adding restart or playback buttons', async () => {
  const wrapper = mount(TrackTimeline, {
    props: { modelValue: defaultPlayback(), duration: 60, currentTime: 12, name: 'Forest', volume: 0, isRepeating: true },
    global: {
      stubs: {
        VolumeSlider: { name: 'VolumeSlider', props: { modelValue: Number, disabled: Boolean }, template: '<div />' },
      },
    },
  })
  const volume = wrapper.getComponent({ name: 'VolumeSlider' })
  expect(volume.props('modelValue')).toBe(0)
  expect(volume.attributes('track-size')).toBe('6')
  expect(volume.attributes('thumb-size')).toBe('18')
  await wrapper.setProps({ disabled: true })
  expect(volume.props('disabled')).toBe(true)
  await wrapper.setProps({ disabled: false })
  await volume.vm.$emit('update:modelValue', 25)
  expect(wrapper.emitted('volume')).toEqual([[25]])
  expect(wrapper.findAll('button')).toHaveLength(4)
  expect(wrapper.get('.position-time').text()).toBe('0:12 / 1:00')
  wrapper.unmount()
})

it('drags boundaries without seeking, cancels pointer edits, and clamps clicks in excluded areas', async () => {
  const wrapper = await mountTimeline()
  const axis = wrapper.get('.timeline-axis')
  vi.spyOn(axis.element, 'getBoundingClientRect').mockReturnValue(new DOMRect(0, 0, 300, 6))
  const start = wrapper.get('[aria-label="Beginning for Forest"]')
  Object.defineProperty(start.element, 'setPointerCapture', { value: vi.fn() })
  Object.defineProperty(axis.element, 'setPointerCapture', { value: vi.fn() })
  await pointerEvent(start.element, 'pointerdown', 1, 0)
  await pointerEvent(start.element, 'pointermove', 1, 50)
  expect(wrapper.props('modelValue').startTime).toBe(10)
  expect(wrapper.emitted('seek')).toBeUndefined()
  await pointerEvent(start.element, 'pointercancel', 1, 50)
  expect(wrapper.props('modelValue').startTime).toBe(0)
  await wrapper.setProps({ modelValue: { startTime: 10, endTime: 25, loopBackTime: 15 } })
  await pointerEvent(axis.element, 'pointerdown', 2, 0)
  expect(wrapper.props('currentTime')).toBe(10)
  await pointerEvent(axis.element, 'pointermove', 2, 300)
  expect(wrapper.props('currentTime')).toBe(25)
  await pointerEvent(axis.element, 'pointerup', 2, 300)
  wrapper.unmount()
})

it('reanchors the loop and resets boundaries through their handles without seeking', async () => {
  const wrapper = await mountTimeline()
  await wrapper.setProps({ modelValue: { startTime: 10, endTime: 25, loopBackTime: 15 } })
  const loop = wrapper.get('[aria-label="Loop-back point for Forest"]')
  const start = wrapper.get('[aria-label="Beginning for Forest"]')
  await loop.trigger('keydown', { key: 'Home' })
  expect(wrapper.props('modelValue')).toEqual({ startTime: 10, endTime: 25, loopBackTime: null })
  expect(loop.classes()).toContain('timeline-handle--anchored')
  await start.trigger('keydown', { key: 'ArrowRight', shiftKey: true })
  expect(loop.text()).toBe('0:11.0')
  await start.trigger('keydown', { key: 'Home' })
  await wrapper.get('[aria-label="End for Forest"]').trigger('keydown', { key: 'End' })
  expect(wrapper.props('modelValue')).toEqual(defaultPlayback())
  expect(wrapper.emitted('seek')).toBeUndefined()
  wrapper.unmount()
})

it('carries fractional timestamps into the next minute', async () => {
  const wrapper = await mountTimeline()
  await wrapper.setProps({ modelValue: { startTime: 59.96, endTime: null, loopBackTime: null }, duration: 120 })
  expect(wrapper.get('[aria-label="Beginning for Forest"]').text()).toBe('1:00.0')
  expect(wrapper.get('[aria-label="End for Forest"]').text()).toBe('2:00.0')
  wrapper.unmount()
})

it('hides loop-back controls for non-repeating types without discarding saved points', async () => {
  const wrapper = await mountTimeline()
  const playback = { startTime: 10, endTime: 25, loopBackTime: 15 }
  await wrapper.setProps({ modelValue: playback, isRepeating: false })
  expect(wrapper.findAll('[role="slider"]')).toHaveLength(3)
  expect(wrapper.find('.timeline-handle--loop').exists()).toBe(false)
  expect(wrapper.props('modelValue')).toEqual(playback)
  expect(wrapper.emitted('update:modelValue')).toBeUndefined()
  await wrapper.setProps({ isRepeating: true })
  expect(wrapper.get('[aria-label="Loop-back point for Forest"]').attributes('aria-valuenow')).toBe('15')
  wrapper.unmount()
})

it('disables all handles until metadata is ready and during save', async () => {
  const wrapper = await mountTimeline()
  await wrapper.setProps({ duration: 0 })
  expect(wrapper.get('[role="status"]').text()).toContain('Loading audio timeline')
  for (const handle of wrapper.findAll('[role="slider"]')) {
    expect(handle.attributes('disabled')).toBeDefined()
    await handle.trigger('keydown', { key: 'ArrowRight' })
  }
  expect(wrapper.emitted('update:modelValue')).toBeUndefined()
  await wrapper.setProps({ duration: 60, disabled: true })
  expect(wrapper.find('[role="status"]').exists()).toBe(false)
  for (const handle of wrapper.findAll('[role="slider"]')) expect(handle.attributes('disabled')).toBeDefined()
  wrapper.unmount()
})

async function mountTimeline() {
  const wrapper = mount(TrackTimeline, {
    props: { modelValue: defaultPlayback(), duration: 60, currentTime: 0, name: 'Forest', isRepeating: true },
  })
  await wrapper.setProps({
    'onUpdate:modelValue': settings => { void wrapper.setProps({ modelValue: settings }) },
    onSeek: time => { void wrapper.setProps({ currentTime: time }) },
  })
  return wrapper
}

async function pointerEvent(element: Element, type: string, pointerID: number, clientX: number) {
  const event = new MouseEvent(type, { button: 0, clientX, bubbles: true })
  Object.defineProperty(event, 'pointerId', { value: pointerID })
  element.dispatchEvent(event)
  await nextTick()
}
