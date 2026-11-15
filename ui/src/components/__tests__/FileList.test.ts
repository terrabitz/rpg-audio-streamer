import { flushPromises, mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { ref } from 'vue'
import { expect, it, vi } from 'vitest'
import FileList from '../FileList.vue'
import { useAudioStore } from '@/stores/audio'
import { useFileStore } from '@/stores/files'
import { useTrackTypeStore } from '@/stores/trackTypes'

vi.mock('vuetify', async (importOriginal) => ({
  ...await importOriginal<typeof import('vuetify')>(),
  useDisplay: () => ({ smAndDown: ref(false) }),
}))

it('preserves audio metadata when resuming a library preview', async () => {
  const pinia = createPinia()
  setActivePinia(pinia)
  const files = useFileStore()
  files.tracks = [
    { id: 'forest', name: 'Forest', typeID: 'music', path: '/forest', createdAt: '2026-01-01T00:00:00Z' },
    { id: 'rain', name: 'Rain', typeID: 'music', path: '/rain', createdAt: '2026-01-01T00:00:00Z' },
  ]
  const types = useTrackTypeStore()
  types.trackTypes = [{
    id: 'music', name: 'Music', color: '#123456', isRepeating: true, allowSimultaneousPlay: false,
  }]
  vi.spyOn(files, 'fetchFiles').mockResolvedValue()
  vi.spyOn(types, 'fetchTrackTypes').mockResolvedValue()
  const wrapper = mount(FileList, {
    props: { library: true },
    global: {
      plugins: [pinia],
      stubs: {
        VContainer: { template: '<div><slot /></div>' },
        VDataTable: {
          props: ['items'],
          template: '<div><template v-for="item in items" :key="item.id"><slot name="item.preview" :item="item" /></template></div>',
        },
        VBtn: {
          emits: ['click'],
          template: '<button @click="$emit(\'click\', $event)"><slot /></button>',
        },
        AudioPlayer: true,
        AudioControls: true,
      },
    },
  })
  await flushPromises()
  const audio = useAudioStore()
  await wrapper.get('[aria-label="Preview Forest"]').trigger('click')
  audio.updateTrackState('forest', {
    duration: 30, currentTime: 5, playback: { startTime: 5, endTime: 15, loopBackTime: null },
  })
  await wrapper.get('[aria-label="Pause preview Forest"]').trigger('click')
  await wrapper.get('[aria-label="Preview Forest"]').trigger('click')
  expect(audio.tracks.forest).toMatchObject({
    isPlaying: true, duration: 30, currentTime: 5,
    playback: { startTime: 5, endTime: 15, loopBackTime: null },
  })
  await wrapper.get('[aria-label="Preview Rain"]').trigger('click')
  expect(audio.tracks.rain).toMatchObject({ isPlaying: true, duration: 0 })
  expect(audio.tracks.forest).toBeUndefined()
  wrapper.unmount()
})
