import { expect, it } from 'vitest'
import { mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import PlayingTrackList from '../PlayingTrackList.vue'
import { useAudioStore } from '@/stores/audio'
import { useTrackTypeStore } from '@/stores/trackTypes'

it('shows an empty state without playback controls', () => {
  const { wrapper } = mountTracks()
  expect(wrapper.text()).toContain('No tracks playing. Select a track to start.')
  expect(wrapper.findComponent({ name: 'TrackPlaybackControls' }).exists()).toBe(false)
  wrapper.unmount()
})

it('keeps full names accessible and routes controls to the matching track', async () => {
  const { wrapper, audio } = mountTracks()
  audio.initTrack('forest', 'Forest', 'Ambiance')
  const state = audio.tracks.forest
  if (!state) throw new Error('Missing fixture track')
  audio.setFading('forest', true)
  await wrapper.setProps({
    tracks: [{
      id: 'forest', name: 'Forest', typeID: 'ambiance', path: '/forest', createdAt: '2026-01-01T00:00:00Z',
      state,
    }],
  })
  expect(wrapper.get('[role="group"]').attributes('aria-label')).toBe('Playing Forest')
  expect(wrapper.get('h3').attributes('title')).toBe('Forest')
  const controls = wrapper.getComponent({ name: 'TrackPlaybackControls' })
  expect(controls.props('showPlay')).toBe(true)
  expect(controls.props('fading')).toBe(true)
  await controls.vm.$emit('play')
  await controls.vm.$emit('volume', 0)
  await controls.vm.$emit('seek', 12)
  expect(wrapper.emitted('play')).toEqual([['forest']])
  expect(wrapper.emitted('volume')).toEqual([['forest', 0]])
  expect(wrapper.emitted('seek')).toEqual([['forest', 12]])
  await wrapper.setProps({ tracks: [] })
  expect(wrapper.find('[role="group"]').exists()).toBe(false)
  wrapper.unmount()
})

function mountTracks() {
  const pinia = createPinia()
  setActivePinia(pinia)
  const audio = useAudioStore()
  useTrackTypeStore().trackTypes = [{
    id: 'ambiance', name: 'Ambiance', color: '#b39ddb', isRepeating: true, allowSimultaneousPlay: true,
  }]
  const wrapper = mount(PlayingTrackList, {
    props: { tracks: [] },
    global: {
      plugins: [pinia],
      stubs: {
        VCard: { template: '<div><slot /></div>' },
        TrackPlaybackControls: true,
      },
    },
  })
  return { wrapper, audio }
}
