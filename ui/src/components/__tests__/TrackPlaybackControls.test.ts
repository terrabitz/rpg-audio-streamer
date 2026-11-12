import { expect, it } from 'vitest'
import { mount } from '@vue/test-utils'
import TrackPlaybackControls from '../TrackPlaybackControls.vue'
import type { AudioTrack } from '@/stores/audio'

const state: AudioTrack = {
  fileID: 'forest', name: 'Forest', isPlaying: false, volume: 0,
  currentTime: 65, duration: 120, isRepeating: true, trackType: 'Music',
}

it('allows volume, seeking, and restart without offering playback in settings', async () => {
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
  await seek.vm.$emit('update:modelValue', 90)
  await wrapper.getComponent({ name: 'VolumeSlider' }).vm.$emit('update:modelValue', 35)
  await wrapper.get('[aria-label="Restart Forest"]').trigger('click')
  expect(wrapper.emitted('seek')).toEqual([[90], [0]])
  expect(wrapper.emitted('volume')).toEqual([[35]])
  expect(wrapper.emitted('play')).toBeUndefined()
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
        VSlider: { name: 'VSlider', props: ['modelValue', 'disabled'], template: '<div />' },
        VBtn: { template: '<button><slot /></button>' },
        VProgressCircular: { name: 'VProgressCircular', template: '<span />' },
        VIcon: true,
      },
    },
  })
}
