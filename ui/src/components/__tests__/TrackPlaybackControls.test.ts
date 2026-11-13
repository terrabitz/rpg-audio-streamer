import { expect, it } from 'vitest'
import { mount } from '@vue/test-utils'
import TrackPlaybackControls from '../TrackPlaybackControls.vue'
import type { AudioTrack } from '@/stores/audio'

const state: AudioTrack = {
  fileID: 'forest', name: 'Forest', isPlaying: false, volume: 0,
  currentTime: 65, duration: 120, isRepeating: true, trackType: 'Music',
}

it('allows volume, seeking, and restart through compact controls', async () => {
  const wrapper = mountControls()
  expect(wrapper.find('[aria-label="Play Forest"]').exists()).toBe(false)
  expect(wrapper.find('[aria-label="Pause Forest"]').exists()).toBe(false)
  expect(wrapper.get('.position-time').text()).toBe('1:05 / 2:00')
  expect(wrapper.get('[aria-label="Restart Forest"]').element.compareDocumentPosition(
    wrapper.get('.track-position').element,
  ) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy()
  const seek = wrapper.getComponent({ name: 'VSlider' })
  expect(seek.attributes('aria-valuetext')).toBe('1:05 of 2:00')
  expect(seek.props('disabled')).toBe(false)
  expect(seek.attributes('thumb-size')).toBe('18')
  expect(seek.attributes('track-size')).toBe('6')
  expect(wrapper.getComponent({ name: 'VolumeSlider' }).attributes('thumb-size')).toBe('18')
  expect(wrapper.getComponent({ name: 'VolumeSlider' }).attributes('track-size')).toBe('6')
  await seek.vm.$emit('update:modelValue', 90)
  await wrapper.getComponent({ name: 'VolumeSlider' }).vm.$emit('update:modelValue', 35)
  await wrapper.get('[aria-label="Restart Forest"]').trigger('click')
  expect(wrapper.emitted('seek')).toEqual([[90], [0]])
  expect(wrapper.emitted('volume')).toEqual([[35]])
  expect(wrapper.emitted('play')).toBeUndefined()
  wrapper.unmount()
})

it('marks only a distinct loop-back point for repeating tracks', async () => {
  const wrapper = mountControls(true)
  const seek = wrapper.getComponent({ name: 'VSlider' })
  expect(seek.props('ticks')).toEqual({})
  expect(seek.props('showTicks')).toBe(false)
  const playback = { startTime: 10, endTime: 50, loopBackTime: 20 }
  await wrapper.setProps({ state: { ...state, playback } })
  expect(seek.props('ticks')).toEqual({ 20: '' })
  expect(seek.props('showTicks')).toBe('always')
  await wrapper.setProps({ state: { ...state, playback, currentTime: 30 } })
  expect(seek.props('ticks')).toEqual({ 20: '' })
  await wrapper.setProps({ state: { ...state, playback: { ...playback, loopBackTime: null } } })
  expect(seek.props('ticks')).toEqual({})
  await wrapper.setProps({ state: { ...state, playback: { ...playback, loopBackTime: 10 } } })
  expect(seek.props('ticks')).toEqual({})
  await wrapper.setProps({ state: { ...state, playback, isRepeating: false } })
  expect(seek.props('ticks')).toEqual({})
  expect(seek.props('showTicks')).toBe(false)
  await wrapper.setProps({ state: { ...state, playback, duration: 0 } })
  expect(seek.props('ticks')).toEqual({})
  expect(seek.props('showTicks')).toBe(false)
  wrapper.unmount()
})

it.each([
  [10, 50, 10, 60, '0:00', '0:40'],
  [10, 50, 30, 60, '0:20', '0:40'],
  [10, 50, 50, 60, '0:40', '0:40'],
  [10, 50, 0, 60, '0:00', '0:40'],
  [10, 50, 55, 60, '0:40', '0:40'],
  [10, null, 20, 60, '0:10', '0:50'],
  [0, 50, 20, 60, '0:20', '0:50'],
  [10.5, 50.5, 11.5, 60, '0:01', '0:40'],
  [10, null, 10, 0, '0:00', '0:00'],
])('shows region-relative time without changing seek coordinates (%s–%s at %s)', async (
  startTime, endTime, currentTime, duration, elapsed, total,
) => {
  const wrapper = mountControls(true)
  await wrapper.setProps({
    state: { ...state, playback: { startTime, endTime, loopBackTime: null }, currentTime, duration },
  })
  expect(wrapper.get('.position-time').text()).toBe(`${elapsed} / ${total}`)
  const seek = wrapper.getComponent({ name: 'VSlider' })
  expect(seek.attributes('aria-valuetext')).toBe(`${elapsed} of ${total}`)
  expect(seek.props('modelValue')).toBe(currentTime)
  await seek.vm.$emit('update:modelValue', startTime + 1)
  await wrapper.get('[aria-label="Restart Forest"]').trigger('click')
  expect(wrapper.emitted('seek')).toEqual([[startTime + 1], [startTime]])
  wrapper.unmount()
})

it('exposes pause and fade status in the sidebar and disables seeking without duration', async () => {
  const wrapper = mountControls(true)
  await wrapper.setProps({ state: { ...state, isPlaying: true, duration: 0 }, fading: true })
  expect(wrapper.get('[aria-label="Pause Forest"]').attributes('aria-pressed')).toBe('true')
  expect(wrapper.get('.track-position').element.compareDocumentPosition(
    wrapper.get('[aria-label="Restart Forest"]').element,
  ) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy()
  expect(wrapper.findComponent({ name: 'VProgressCircular' }).exists()).toBe(true)
  expect(wrapper.getComponent({ name: 'VSlider' }).props('disabled')).toBe(true)
  await wrapper.get('[aria-label="Pause Forest"]').trigger('click')
  expect(wrapper.emitted('play')).toEqual([[]])
  wrapper.unmount()
})

function mountControls(showPlay = false) {
  return mount(TrackPlaybackControls, {
    props: { state, name: 'Forest', showPlay },
    global: {
      stubs: {
        VolumeSlider: { name: 'VolumeSlider', props: ['modelValue'], template: '<div />' },
        VSlider: { name: 'VSlider', props: ['modelValue', 'disabled', 'ticks', 'showTicks'], template: '<div />' },
        VBtn: { template: '<button><slot /></button>' },
        VProgressCircular: { name: 'VProgressCircular', template: '<span />' },
        VIcon: true,
      },
    },
  })
}
