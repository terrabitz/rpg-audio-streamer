import { afterEach, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { postApiV1Files, postApiV1FilesImport, postApiV1FilesScan } from '@/client/apiClient'
import { useFileStore } from '@/stores/files'
import { useTrackTypeStore } from '@/stores/trackTypes'
import AudioUploader from '../AudioUploader.vue'
import TrackTypeSelector from '../TrackTypeSelector.vue'

vi.mock('@/client/apiClient', () => ({
  postApiV1Files: vi.fn(),
  postApiV1FilesImport: vi.fn(),
  postApiV1FilesScan: vi.fn(),
  getApiV1TrackTypes: vi.fn(),
}))

afterEach(() => {
  vi.restoreAllMocks()
  vi.useRealTimers()
})

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
  vi.mocked(postApiV1Files).mockResolvedValue({
    data: { id: 'uploaded', name: 'Forest', typeID: 'music', createdAt: '', path: '', status: 'processing' },
    request: new Request('http://localhost'), response: new Response(null, { status: 202 }),
  })
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
  expect(toast.text()).toContain('Track added')
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

it('requires a link, name, and valid type before importing', async () => {
  const { wrapper, submit } = await prepareImport()
  expect(submit().attributes('disabled')).toBeDefined()
  await wrapper.getComponent(TrackTypeSelector).vm.$emit('update:modelValue', 'music')
  const url = wrapper.findAllComponents({ name: 'VTextField' })[0]!
  const name = wrapper.findAllComponents({ name: 'VTextField' })[1]!
  await url.vm.$emit('update:modelValue', ' ')
  await flushPromises()
  expect(submit().attributes('disabled')).toBeDefined()
  await url.vm.$emit('update:modelValue', 'https://youtu.be/aaaaaaaaaaa')
  await name.vm.$emit('update:modelValue', ' ')
  await flushPromises()
  expect(submit().attributes('disabled')).toBeDefined()
  expect(postApiV1FilesImport).not.toHaveBeenCalled()
  wrapper.unmount()
})

it('closes on acceptance and adds a processing track without waiting for conversion', async () => {
  const { wrapper, submit } = await prepareImport()
  await wrapper.getComponent(TrackTypeSelector).vm.$emit('update:modelValue', 'music')
  let complete!: () => void
  const pending = new Promise<void>(resolve => { complete = resolve })
  vi.mocked(postApiV1FilesImport).mockImplementation(async () => {
    await pending
    return {
      data: { id: 'imported', name: 'Forest', typeID: 'music', createdAt: '', path: '', status: 'processing' },
      request: new Request('http://localhost'), response: new Response(null, { status: 202 }),
    }
  })
  await submit().trigger('click')
  await flushPromises()
  expect(submit().attributes('disabled')).toBeDefined()
  expect(wrapper.getComponent({ name: 'VTabs' }).props('disabled')).toBe(true)
  expect(postApiV1FilesImport).toHaveBeenCalledExactlyOnceWith({
    body: { url: 'https://youtu.be/aaaaaaaaaaa', name: 'Forest', typeID: 'music', tableID: undefined },
  })
  complete()
  await flushPromises()
  expect(wrapper.find('[role="dialog"]').exists()).toBe(false)
  expect(useFileStore().getTrackById('imported')?.status).toBe('processing')
  expect(wrapper.getComponent({ name: 'VSnackbar' }).text()).toContain('Track added')
  await wrapper.get('button').trigger('click')
  await flushPromises()
  expect(submit().attributes('disabled')).toBeDefined()
  expect(wrapper.findAllComponents({ name: 'VTextField' })[0]!.props('modelValue')).toBe('')
  wrapper.unmount()
})

it('auto-fills scanned metadata and puts limit errors on the URL field', async () => {
  const { wrapper, submit } = await prepareImport()
  expect(wrapper.findAllComponents({ name: 'VTextField' })[1]!.props('modelValue')).toBe('Forest')
  expect(wrapper.findAllComponents({ name: 'VTextField' })[1]!.props('disabled')).toBe(false)
  const metadata = wrapper.get('dl[aria-label="Video metadata"]')
  expect(metadata.findAll('dt').map(term => term.text())).toEqual(['Duration', 'Audio size'])
  expect(metadata.findAll('dd').map(value => value.text())).toEqual(['0:00:12', '1.0 MiB'])
  expect(wrapper.findAllComponents({ name: 'VTextField' })).toHaveLength(2)
  vi.spyOn(console, 'error').mockImplementation(() => {})
  vi.mocked(postApiV1FilesScan).mockRejectedValue('Video exceeds the 3-hour limit\n')
  await wrapper.findAllComponents({ name: 'VTextField' })[0]!.vm.$emit('update:modelValue', 'https://youtu.be/ggggggggggg')
  await vi.advanceTimersByTimeAsync(500)
  await flushPromises()
  expect(wrapper.findAllComponents({ name: 'VTextField' })[0]!.props('errorMessages')).toEqual(['Video exceeds the 3-hour limit'])
  expect(wrapper.findAllComponents({ name: 'VTextField' })[1]!.props('disabled')).toBe(true)
  expect(submit().attributes('disabled')).toBeDefined()
  expect(postApiV1FilesImport).not.toHaveBeenCalled()
  wrapper.unmount()
})

it('keeps the name disabled until scanning succeeds and disables it when the link is cleared', async () => {
  vi.useFakeTimers()
  const { wrapper } = await mountUploader()
  await wrapper.getComponent({ name: 'VTabs' }).vm.$emit('update:modelValue', 'youtube')
  await flushPromises()
  const [url, name] = wrapper.findAllComponents({ name: 'VTextField' })
  expect(name!.props('disabled')).toBe(true)
  let release!: () => void
  const pending = new Promise<void>(resolve => { release = resolve })
  vi.mocked(postApiV1FilesScan).mockImplementationOnce(async () => {
    await pending
    return {
      data: { url: 'https://youtu.be/aaaaaaaaaaa', name: 'Forest', duration: 12, sizeBytes: 1048576, sizeEstimated: false },
      request: new Request('http://localhost'), response: new Response(),
    }
  })
  await url!.vm.$emit('update:modelValue', 'https://youtu.be/aaaaaaaaaaa')
  expect(name!.props('disabled')).toBe(true)
  await vi.advanceTimersByTimeAsync(500)
  await flushPromises()
  expect(name!.props('disabled')).toBe(true)
  release()
  await flushPromises()
  expect(name!.props('disabled')).toBe(false)
  await url!.vm.$emit('update:modelValue', '')
  await flushPromises()
  expect(name!.props('disabled')).toBe(true)
  wrapper.unmount()
})

it('ignores stale scan results after the URL changes', async () => {
  const { wrapper, submit } = await prepareImport()
  await wrapper.getComponent(TrackTypeSelector).vm.$emit('update:modelValue', 'music')
  let release!: () => void
  const pending = new Promise<void>(resolve => { release = resolve })
  vi.mocked(postApiV1FilesScan).mockImplementationOnce(async () => {
    await pending
    return {
      data: { url: 'https://youtu.be/bbbbbbbbbbb', name: 'Stale video', duration: 30, sizeBytes: null, sizeEstimated: false },
      request: new Request('http://localhost'), response: new Response(),
    }
  })
  const url = wrapper.findAllComponents({ name: 'VTextField' })[0]!
  await url.vm.$emit('update:modelValue', 'https://youtu.be/bbbbbbbbbbb')
  await vi.advanceTimersByTimeAsync(500)
  await flushPromises()
  expect(submit().attributes('disabled')).toBeDefined()
  await url.vm.$emit('update:modelValue', 'https://youtu.be/aaaaaaaaaaa')
  await vi.advanceTimersByTimeAsync(500)
  await flushPromises()
  release()
  await flushPromises()
  expect(wrapper.findAllComponents({ name: 'VTextField' })[1]!.props('modelValue')).toBe('Forest')
  expect(submit().attributes('disabled')).toBeUndefined()
  wrapper.unmount()
})

it('preserves failed import fields and displays the server error inline', async () => {
  const { wrapper, submit } = await prepareImport()
  vi.spyOn(console, 'error').mockImplementation(() => {})
  vi.mocked(postApiV1FilesImport).mockRejectedValue('YouTube import requires yt-dlp on the server\n')
  await wrapper.getComponent(TrackTypeSelector).vm.$emit('update:modelValue', 'music')
  await submit().trigger('click')
  await flushPromises()
  expect(wrapper.get('[role="alert"]').text()).toContain('YouTube import requires yt-dlp on the server')
  expect(wrapper.findAllComponents({ name: 'VTextField' })[0]!.props('modelValue')).toBe(' https://youtu.be/aaaaaaaaaaa ')
  expect(submit().attributes('disabled')).toBeUndefined()
  expect(wrapper.getComponent({ name: 'VSnackbar' }).props('modelValue')).toBe(false)
  wrapper.unmount()
})

async function prepareImport() {
  vi.useFakeTimers()
  const result = await mountUploader()
  await result.wrapper.getComponent({ name: 'VTabs' }).vm.$emit('update:modelValue', 'youtube')
  await flushPromises()
  await result.wrapper.findAllComponents({ name: 'VTextField' })[0]!.vm.$emit(
    'update:modelValue', ' https://youtu.be/aaaaaaaaaaa ',
  )
  await vi.advanceTimersByTimeAsync(500)
  await flushPromises()
  const submit = () => result.wrapper.findAll('button').find(button => button.text() === 'Import')!
  return { ...result, submit }
}

async function mountUploader() {
  vi.mocked(postApiV1Files).mockReset()
  vi.mocked(postApiV1FilesImport).mockReset()
  vi.mocked(postApiV1FilesScan).mockReset()
  vi.mocked(postApiV1FilesScan).mockResolvedValue({
    data: { url: 'https://youtu.be/aaaaaaaaaaa', name: 'Forest', duration: 12, sizeBytes: 1048576, sizeEstimated: false },
    request: new Request('http://localhost'), response: new Response(),
  })
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
    props: ['modelValue', 'errorMessages', 'loading', 'disabled'],
    emits: ['update:modelValue'],
    template: '<div />',
  })
  const passthrough = { template: '<div><slot /></div>' }
  const wrapper = mount(AudioUploader, {
    global: {
      plugins: [pinia],
      stubs: {
        ErrorToast: { props: ['modelValue'], template: '<div v-if="modelValue" role="alert">{{ modelValue }}</div>' },
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
        VTabs: {
          name: 'VTabs',
          props: ['modelValue', 'disabled'],
          emits: ['update:modelValue'],
          template: '<div><slot /></div>',
        },
        VTab: passthrough,
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
        VTable: { template: '<table><slot /></table>' },
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

it('keeps only failed tracks and their edited metadata for retry after a partial upload', async () => {
  const { wrapper, file, upload } = await mountUploader()
  useTrackTypeStore().trackTypes.push({
    id: 'ambiance', name: 'Ambiance', color: '#234567', isRepeating: true, allowSimultaneousPlay: true,
  })
  const secondFile = new File(['audio'], 'Rain.ogg', { type: 'audio/ogg' })
  await wrapper.getComponent({ name: 'VFileInput' }).vm.$emit('update:modelValue', [file, secondFile])
  const [first, second] = wrapper.findAll('tbody tr')
  await first!.getComponent({ name: 'VSelect' }).vm.$emit('update:modelValue', 'music')
  await second!.getComponent({ name: 'VSelect' }).vm.$emit('update:modelValue', 'ambiance')
  await second!.getComponent({ name: 'VTextField' }).vm.$emit('update:modelValue', ' Night Rain ')
  const response = (id: string, name: string, typeID: string) => ({
    data: { id, name, typeID, createdAt: '', path: '', status: 'processing' as const },
    request: new Request('http://localhost'), response: new Response(null, { status: 202 }),
  })
  vi.spyOn(console, 'error').mockImplementation(() => {})
  vi.mocked(postApiV1Files)
    .mockResolvedValueOnce(response('forest', 'Forest', 'music'))
    .mockRejectedValueOnce(new Error('offline'))
    .mockResolvedValueOnce(response('rain', 'Night Rain', 'ambiance'))

  const batchUpload = wrapper.findAll('button').find(button => button.text() === 'Upload 2 tracks')!
  await batchUpload.trigger('click')
  await flushPromises()
  expect(wrapper.get('[role="alert"]').text()).toContain('1 of 2 tracks added.')
  expect(useFileStore().tracks.map(track => track.name)).toEqual(['Forest'])
  expect(wrapper.getComponent({ name: 'VFileInput' }).props('modelValue')).toEqual([secondFile])
  await upload().trigger('click')
  await flushPromises()

  expect(vi.mocked(postApiV1Files).mock.calls.map(([{ body }]) => [body?.name, body?.typeID]))
    .toEqual([['Forest', 'music'], ['Night Rain', 'ambiance'], ['Night Rain', 'ambiance']])
  expect(useFileStore().tracks.map(track => track.name)).toEqual(['Forest', 'Night Rain'])
  expect(wrapper.find('[role="dialog"]').exists()).toBe(false)
  wrapper.unmount()
})
