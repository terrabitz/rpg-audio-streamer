import { afterEach, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { postApiV1Files } from '@/client/apiClient'
import { useFileStore } from '@/stores/files'
import { useTrackTypeStore } from '@/stores/trackTypes'
import AudioUploader from '../AudioUploader.vue'
import TrackTypeSelector from '../TrackTypeSelector.vue'

vi.mock('@/client/apiClient', () => ({
  postApiV1Files: vi.fn(),
  getApiV1TrackTypes: vi.fn(),
}))

afterEach(() => vi.restoreAllMocks())

it.each(['', ' ', 'missing-type'])('blocks uploads with invalid track type %j', async (typeID) => {
  const { wrapper, upload } = await mountUploader()
  await wrapper.getComponent(TrackTypeSelector).vm.$emit('update:modelValue', typeID)
  await flushPromises()
  expect(upload().attributes('disabled')).toBeDefined()
  expect(postApiV1Files).not.toHaveBeenCalled()
  wrapper.unmount()
})

it('requires a file and a nonblank name even with a valid type', async () => {
  const { wrapper, upload } = await mountUploader()
  await wrapper.getComponent(TrackTypeSelector).vm.$emit('update:modelValue', 'music')
  await wrapper.getComponent({ name: 'VTextField' }).vm.$emit('update:modelValue', ' ')
  await flushPromises()
  expect(upload().attributes('disabled')).toBeDefined()
  await wrapper.getComponent({ name: 'VTextField' }).vm.$emit('update:modelValue', 'Forest')
  await wrapper.getComponent({ name: 'VFileInput' }).vm.$emit('update:modelValue', null)
  await flushPromises()
  expect(upload().attributes('disabled')).toBeDefined()
  wrapper.unmount()
})

it('blocks uploading if the selected type is no longer available', async () => {
  const { wrapper, upload } = await mountUploader()
  await wrapper.getComponent(TrackTypeSelector).vm.$emit('update:modelValue', 'music')
  await flushPromises()
  expect(upload().attributes('disabled')).toBeUndefined()
  useTrackTypeStore().trackTypes = []
  await flushPromises()
  expect(upload().attributes('disabled')).toBeDefined()
  expect(postApiV1Files).not.toHaveBeenCalled()
  wrapper.unmount()
})

it('uploads a valid type, shows a bottom toast, and requires a fresh selection afterward', async () => {
  const { wrapper, upload, file } = await mountUploader()
  vi.mocked(postApiV1Files).mockResolvedValue({ data: undefined, request: new Request('http://localhost'), response: new Response() })
  await wrapper.getComponent(TrackTypeSelector).vm.$emit('update:modelValue', 'music')
  await flushPromises()
  expect(upload().attributes('disabled')).toBeUndefined()
  await upload().trigger('click')
  await flushPromises()
  expect(postApiV1Files).toHaveBeenCalledExactlyOnceWith({
    body: { files: file, name: 'Forest', typeID: 'music', tableID: undefined },
  })
  const toast = wrapper.getComponent({ name: 'VSnackbar' })
  expect(toast.props()).toMatchObject({
    modelValue: true, attach: 'body', location: 'bottom center', timeout: 4000,
  })
  expect(toast.text()).toContain('File uploaded successfully!')
  await wrapper.get('button').trigger('click')
  await flushPromises()
  expect(upload().attributes('disabled')).toBeDefined()
  expect(wrapper.getComponent(TrackTypeSelector).props('modelValue')).toBe('')
  wrapper.unmount()
})

it('keeps upload failures in the dialog instead of showing a success toast', async () => {
  const { wrapper, upload } = await mountUploader()
  vi.spyOn(console, 'error').mockImplementation(() => {})
  vi.mocked(postApiV1Files).mockRejectedValue(new Error('offline'))
  await wrapper.getComponent(TrackTypeSelector).vm.$emit('update:modelValue', 'music')
  await flushPromises()
  await upload().trigger('click')
  await flushPromises()
  expect(wrapper.get('[role="alert"]').text()).toContain('Failed to upload file')
  expect(wrapper.getComponent({ name: 'VSnackbar' }).props('modelValue')).toBe(false)
  wrapper.unmount()
})

async function mountUploader() {
  vi.mocked(postApiV1Files).mockReset()
  const pinia = createPinia()
  setActivePinia(pinia)
  const trackTypes = useTrackTypeStore()
  trackTypes.trackTypes = [{
    id: 'music', name: 'Music', color: '#123456', isRepeating: true, allowSimultaneousPlay: false,
  }]
  vi.spyOn(trackTypes, 'fetchTrackTypes').mockResolvedValue(undefined)
  vi.spyOn(useFileStore(), 'fetchFiles').mockResolvedValue(undefined)
  const field = (name: string) => ({
    name,
    props: ['modelValue'],
    emits: ['update:modelValue'],
    template: '<div />',
  })
  const passthrough = { template: '<div><slot /></div>' }
  const wrapper = mount(AudioUploader, {
    global: {
      plugins: [pinia],
      stubs: {
        VDialog: {
          props: ['modelValue'],
          emits: ['update:modelValue'],
          template: '<div><slot name="activator" :props="{ onClick: () => $emit(\'update:modelValue\', true) }" /><div v-if="modelValue" role="dialog"><slot /></div></div>',
        },
        VBtn: {
          props: ['disabled', 'loading'],
          template: '<button :disabled="disabled"><slot /></button>',
        },
        VForm: {
          emits: ['update:modelValue'],
          mounted() { this.$emit('update:modelValue', true) },
          template: '<form><slot /></form>',
        },
        VFileInput: field('VFileInput'),
        VTextField: field('VTextField'),
        VSelect: field('VSelect'),
        VSnackbar: {
          name: 'VSnackbar',
          props: ['modelValue', 'attach', 'location', 'timeout'],
          template: '<div v-if="modelValue" role="status"><slot /></div>',
        },
        VAlert: { template: '<div role="alert"><slot /></div>' },
        VCard: passthrough,
        VCardTitle: passthrough,
        VCardText: passthrough,
        VCardActions: passthrough,
        VSpacer: true,
      },
    },
  })
  await wrapper.get('button').trigger('click')
  await flushPromises()
  const file = new File(['audio'], 'Forest.mp3', { type: 'audio/mpeg' })
  await wrapper.getComponent({ name: 'VFileInput' }).vm.$emit('update:modelValue', file)
  await flushPromises()
  const upload = () => wrapper.findAll('button').find(button => button.text() === 'Upload')!
  return { wrapper, upload, file }
}
