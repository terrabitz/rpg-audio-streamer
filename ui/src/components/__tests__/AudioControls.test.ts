import { afterEach, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import AudioControls from '../AudioControls.vue'
import { TrackUsageChangedError, useFileStore } from '@/stores/files'
import { useTrackTypeStore } from '@/stores/trackTypes'
import { useAudioStore } from '@/stores/audio'
import { useTagStore } from '@/stores/tags'

const EDITOR = '[role="dialog"]:not([aria-label])'
const CONFIRMATION = '[role="dialog"][aria-label="Delete track?"]'

afterEach(() => vi.restoreAllMocks())

it('wires the timeline, waveform, and play toggle without a restart button', async () => {
  const { wrapper, fetchWaveform } = mountControls(false)
  useAudioStore().initTrack('forest', 'Forest', 'Music')
  await wrapper.get('[aria-label="Edit Forest"]').trigger('click')
  const controls = wrapper.getComponent({ name: 'TrackTimeline' })
  expect(controls.props('currentTime')).toBe(0)
  expect(controls.props('volume')).toBe(100)
  expect(controls.props('isRepeating')).toBe(true)
  expect(wrapper.text()).not.toContain('Playback region')
  expect(wrapper.find('[aria-label="Restart Forest"]').exists()).toBe(false)
  expect(controls.props('showPlay')).toBe(true)
  expect(controls.props('isPlaying')).toBe(false)
  await flushPromises()
  expect(fetchWaveform).toHaveBeenCalledWith('forest')
  expect(controls.props('waveform')).toEqual([0.2, 1])
  await controls.vm.$emit('volume', 25)
  await controls.vm.$emit('seek', 12)
  await controls.vm.$emit('play')
  expect(wrapper.emitted('volume')).toEqual([[25]])
  expect(wrapper.emitted('seek')).toEqual([[12]])
  expect(wrapper.emitted('play')).toEqual([[]])
  useAudioStore().updateTrackState('forest', { isPlaying: true })
  await flushPromises()
  expect(controls.props('isPlaying')).toBe(true)
  await wrapper.get('[aria-label="Close track controls"]').trigger('click')
  await wrapper.get('[aria-label="Edit Forest"]').trigger('click')
  expect(fetchWaveform).toHaveBeenCalledTimes(1)
  wrapper.unmount()
})

it('keeps the plain timeline when the waveform cannot load', async () => {
  const { wrapper, fetchWaveform } = mountControls()
  const error = vi.spyOn(console, 'error').mockImplementation(() => {})
  fetchWaveform.mockRejectedValue(new Error('offline'))
  await wrapper.get('[aria-label="Edit Forest"]').trigger('click')
  await flushPromises()
  expect(wrapper.getComponent({ name: 'TrackTimeline' }).props('waveform')).toBeUndefined()
  expect(error).toHaveBeenCalledWith('Failed to load track waveform:', expect.any(Error))
  wrapper.unmount()
})

it('uses the draft track type to show or hide loop-back controls immediately', async () => {
  const { wrapper } = mountControls()
  useTrackTypeStore().trackTypes.push({
    id: 'one-shot', name: 'One-Shot', color: '#123456', isRepeating: false, allowSimultaneousPlay: true,
  })
  await wrapper.get('[aria-label="Edit Forest"]').trigger('click')
  const timeline = wrapper.getComponent({ name: 'TrackTimeline' })
  await wrapper.getComponent({ name: 'TrackTypeSelector' }).vm.$emit('update:modelValue', 'one-shot')
  await flushPromises()
  expect(timeline.props('isRepeating')).toBe(false)
  expect(wrapper.find('.timeline-handle--loop').exists()).toBe(false)
  await wrapper.getComponent({ name: 'TrackTypeSelector' }).vm.$emit('update:modelValue', 'music')
  await flushPromises()
  expect(timeline.props('isRepeating')).toBe(true)
  expect(wrapper.find('.timeline-handle--loop').exists()).toBe(true)
  wrapper.unmount()
})

it('previews point edits immediately, restores them on cancel, and persists only on save', async () => {
  const { wrapper } = mountControls()
  const audio = useAudioStore()
  const files = useFileStore()
  const update = vi.spyOn(files, 'updateTrack').mockImplementation(async (id, settings) => ({
    ...files.tracks[0]!, ...settings, id,
  }))
  await wrapper.get('[aria-label="Edit Forest"]').trigger('click')
  expect(audio.tracks.forest?.isPlaying).toBe(false)
  audio.updateTrackState('forest', { duration: 60 })
  await flushPromises()
  const timeline = wrapper.getComponent({ name: 'TrackTimeline' })
  await timeline.vm.$emit('update:modelValue', { startTime: 10, endTime: 50, loopBackTime: 20 })
  expect(wrapper.emitted('settings')).toEqual([[{ startTime: 10, endTime: 50, loopBackTime: 20 }]])
  expect(files.tracks[0]?.playback).toBeUndefined()
  expect(update).not.toHaveBeenCalled()
  await wrapper.get('[aria-label="Close track controls"]').trigger('click')
  expect(wrapper.emitted('settings')?.at(-1)).toEqual([{ startTime: 0, endTime: null, loopBackTime: null }])
  await wrapper.get('[aria-label="Edit Forest"]').trigger('click')
  expect(wrapper.getComponent({ name: 'TrackTimeline' }).props('modelValue').startTime).toBe(0)
  await wrapper.getComponent({ name: 'TrackTimeline' }).vm.$emit('update:modelValue', {
    startTime: 10, endTime: 50, loopBackTime: 20,
  })
  await wrapper.findAll('button').find(button => button.text() === 'Save Changes')!.trigger('click')
  await flushPromises()
  expect(update).toHaveBeenCalledWith('forest', { playback: { startTime: 10, endTime: 50, loopBackTime: 20 } })
  expect(wrapper.emitted('settings')).toHaveLength(4)
  expect(wrapper.emitted('settings')?.at(-1)).toEqual([{ startTime: 10, endTime: 50, loopBackTime: 20 }])
  wrapper.unmount()
})

it('rejects invalid previews and restores saved settings after a failed save', async () => {
  const { wrapper } = mountControls()
  vi.spyOn(console, 'error').mockImplementation(() => {})
  const update = vi.spyOn(useFileStore(), 'updateTrack').mockRejectedValue(new Error('offline'))
  await wrapper.get('[aria-label="Edit Forest"]').trigger('click')
  useAudioStore().updateTrackState('forest', { duration: 60 })
  const timeline = wrapper.getComponent({ name: 'TrackTimeline' })
  await timeline.vm.$emit('update:modelValue', { startTime: 50, endTime: 10, loopBackTime: null })
  expect(wrapper.emitted('settings')).toBeUndefined()
  await timeline.vm.$emit('update:modelValue', { startTime: 10, endTime: 50, loopBackTime: null })
  await wrapper.findAll('button').find(button => button.text() === 'Save Changes')!.trigger('click')
  await flushPromises()
  expect(update).toHaveBeenCalledOnce()
  expect(wrapper.text()).toContain('Failed to update track.')
  expect(wrapper.emitted('settings')).toEqual([[{ startTime: 10, endTime: 50, loopBackTime: null }]])
  const settings = vi.fn()
  await wrapper.setProps({ onSettings: settings })
  wrapper.unmount()
  expect(settings).toHaveBeenCalledExactlyOnceWith({ startTime: 0, endTime: null, loopBackTime: null })
})

it.each([
  [1, 'table'],
  [2, 'tables'],
])('confirms deletion from %i %s in a follow-up dialog and allows cancellation', async (count, noun) => {
  const { wrapper, countTables, deleteTrack } = mountControls()
  countTables.mockResolvedValue(count)
  await wrapper.get('[aria-label="Edit Forest"]').trigger('click')
  await wrapper.get('[aria-label="Delete track"]').trigger('click')
  await flushPromises()
  const confirmation = wrapper.get(CONFIRMATION)
  expect(confirmation.get('[role="alert"]').text()).toContain(`Deleting Forest will remove it from ${count} ${noun}`)
  expect(wrapper.get(EDITOR).find('[role="alert"]').exists()).toBe(false)
  expect(deleteTrack).not.toHaveBeenCalled()
  await confirmation.get('button:not([aria-label])').trigger('click')
  expect(wrapper.find(CONFIRMATION).exists()).toBe(false)
  expect(wrapper.find(EDITOR).exists()).toBe(true)
  expect(deleteTrack).not.toHaveBeenCalled()
  await wrapper.get('[aria-label="Delete track"]').trigger('click')
  await flushPromises()
  await wrapper.get('[aria-label="Confirm delete track"]').trigger('click')
  await flushPromises()
  expect(deleteTrack).toHaveBeenCalledExactlyOnceWith(count)
  expect(wrapper.find('[role="dialog"]').exists()).toBe(false)
  wrapper.unmount()
})

it('deletes an unused library track without confirmation', async () => {
  const { wrapper, countTables, deleteTrack } = mountControls()
  countTables.mockResolvedValue(0)
  await wrapper.get('[aria-label="Edit Forest"]').trigger('click')
  await wrapper.get('[aria-label="Delete track"]').trigger('click')
  await flushPromises()
  expect(deleteTrack).toHaveBeenCalledExactlyOnceWith(0)
  expect(wrapper.find('[role="dialog"]').exists()).toBe(false)
  wrapper.unmount()
})

it('blocks repeat clicks while checking usage and keeps the track on lookup failure', async () => {
  const { wrapper, countTables, deleteTrack } = mountControls()
  vi.spyOn(console, 'error').mockImplementation(() => {})
  let rejectCount: (error: Error) => void = () => {}
  countTables.mockReturnValue(new Promise((_, reject) => { rejectCount = reject }))
  await wrapper.get('[aria-label="Edit Forest"]').trigger('click')
  await wrapper.get('[aria-label="Delete track"]').trigger('click')
  expect(wrapper.get('[aria-label="Delete track"]').attributes('disabled')).toBeDefined()
  expect(wrapper.get('[aria-label="Close track controls"]').attributes('disabled')).toBeDefined()
  rejectCount(new Error('offline'))
  await flushPromises()
  expect(wrapper.get('[role="alert"]').text()).toContain('The track was not deleted.')
  expect(deleteTrack).not.toHaveBeenCalled()
  wrapper.unmount()
})

it('returns deletion failures to the track controls', async () => {
  const { wrapper, countTables, deleteTrack } = mountControls()
  vi.spyOn(console, 'error').mockImplementation(() => {})
  countTables.mockResolvedValue(1)
  deleteTrack.mockRejectedValue(new Error('offline'))
  await wrapper.get('[aria-label="Edit Forest"]').trigger('click')
  await wrapper.get('[aria-label="Delete track"]').trigger('click')
  await flushPromises()
  await wrapper.get('[aria-label="Confirm delete track"]').trigger('click')
  await flushPromises()
  expect(wrapper.find(CONFIRMATION).exists()).toBe(false)
  expect(wrapper.get(EDITOR).get('[role="alert"]').text()).toContain('Failed to delete track.')
  wrapper.unmount()
})

it('requires a fresh count if usage changes before confirmation', async () => {
  const { wrapper, countTables, deleteTrack } = mountControls()
  vi.spyOn(console, 'error').mockImplementation(() => {})
  countTables.mockResolvedValueOnce(1).mockResolvedValueOnce(2)
  deleteTrack.mockRejectedValueOnce(new TrackUsageChangedError())
  await wrapper.get('[aria-label="Edit Forest"]').trigger('click')
  await wrapper.get('[aria-label="Delete track"]').trigger('click')
  await flushPromises()
  await wrapper.get('[aria-label="Confirm delete track"]').trigger('click')
  await flushPromises()
  expect(wrapper.get(EDITOR).text()).toContain('Table usage changed.')
  expect(wrapper.find(CONFIRMATION).exists()).toBe(false)
  await wrapper.get('[aria-label="Delete track"]').trigger('click')
  await flushPromises()
  expect(wrapper.get(CONFIRMATION).text()).toContain('remove it from 2 tables')
  await wrapper.get('[aria-label="Confirm delete track"]').trigger('click')
  await flushPromises()
  expect(deleteTrack.mock.calls).toEqual([[1], [2]])
  wrapper.unmount()
})

it('removes a table association without checking library usage', async () => {
  const { wrapper, countTables, deleteTrack } = mountControls(false)
  await wrapper.get('[aria-label="Edit Forest"]').trigger('click')
  await wrapper.get('[aria-label="Remove from table"]').trigger('click')
  await flushPromises()
  expect(countTables).not.toHaveBeenCalled()
  expect(deleteTrack).toHaveBeenCalledExactlyOnceWith(null)
  expect(wrapper.find('[role="dialog"]').exists()).toBe(false)
  wrapper.unmount()
})

it('shows table-removal failures without closing the controls', async () => {
  const { wrapper, countTables, deleteTrack } = mountControls(false)
  vi.spyOn(console, 'error').mockImplementation(() => {})
  deleteTrack.mockRejectedValue(new Error('offline'))
  await wrapper.get('[aria-label="Edit Forest"]').trigger('click')
  await wrapper.get('[aria-label="Remove from table"]').trigger('click')
  await flushPromises()
  expect(wrapper.text()).toContain('Failed to remove track from table.')
  expect(wrapper.find(EDITOR).exists()).toBe(true)
  expect(countTables).not.toHaveBeenCalled()
  wrapper.unmount()
})

it('keeps tag assignments as a draft until save and supports clearing them', async () => {
  const { wrapper } = mountControls()
  const files = useFileStore()
  const tag = { id: 'intensity', name: 'High intensity', color: '#123456' }
  files.tracks[0]!.tags = [tag]
  useTagStore().tags = [tag]
  const update = vi.spyOn(files, 'updateTrack').mockResolvedValue({ ...files.tracks[0]!, tags: [] })
  await wrapper.get('[aria-label="Edit Forest"]').trigger('click')
  await flushPromises()
  const selector = wrapper.getComponent({ name: 'VAutocomplete' })
  await selector.vm.$emit('update:modelValue', [])
  expect(files.tracks[0]?.tags).toEqual([tag])
  await wrapper.get('[aria-label="Close track controls"]').trigger('click')
  expect(update).not.toHaveBeenCalled()
  await wrapper.get('[aria-label="Edit Forest"]').trigger('click')
  await flushPromises()
  await wrapper.getComponent({ name: 'VAutocomplete' }).vm.$emit('update:modelValue', [])
  await wrapper.findAll('button').find(button => button.text() === 'Save Changes')!.trigger('click')
  await flushPromises()
  expect(update).toHaveBeenCalledExactlyOnceWith('forest', { tagIDs: [] })
  wrapper.unmount()
})

function mountControls(library = true) {
  const pinia = createPinia()
  setActivePinia(pinia)
  const files = useFileStore()
  files.tracks = [{
    id: 'forest', name: 'Forest', typeID: 'music', path: '/uploads/forest', createdAt: '2026-01-01T00:00:00Z',
  }]
  useTrackTypeStore().trackTypes = [{
    id: 'music', name: 'Music', color: '#123456', isRepeating: true, allowSimultaneousPlay: false,
  }]
  const countTables = vi.spyOn(files, 'countTrackTables')
  const fetchWaveform = vi.spyOn(files, 'fetchWaveform').mockResolvedValue([0.2, 1])
  vi.spyOn(useTagStore(), 'fetchTags').mockResolvedValue(undefined)
  const deleteTrack = vi.fn().mockResolvedValue(undefined)
  const passthrough = { template: '<div><slot /></div>' }
  const wrapper = mount(AudioControls, {
    props: { fileID: 'forest', fileName: 'Forest', library, deleteTrack },
    global: {
      plugins: [pinia],
      stubs: {
        VBtn: {
          props: ['disabled', 'loading'],
          emits: ['click'],
          template: '<button :disabled="disabled" @click="$emit(\'click\', $event)"><slot /></button>',
        },
        VDialog: {
          props: ['modelValue', 'persistent'],
          template: '<div v-if="modelValue" role="dialog"><slot /></div>',
        },
        VAlert: { template: '<div role="alert"><slot /></div>' },
        VCard: passthrough,
        VCardTitle: passthrough,
        VCardText: passthrough,
        VCardActions: passthrough,
        VChip: passthrough,
        VDivider: true,
        VSpacer: true,
        VTextField: true,
        VIcon: true,
        TrackTypeSelector: true,
        TagManager: true,
        VAutocomplete: true,
        VolumeSlider: true,
      },
    },
  })
  return { wrapper, countTables, deleteTrack, fetchWaveform }
}
