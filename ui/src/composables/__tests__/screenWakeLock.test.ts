import { flushPromises, mount, type VueWrapper } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import persistedState from 'pinia-plugin-persistedstate'
import { computed, createApp, defineComponent, ref } from 'vue'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { useClientPreferencesStore } from '@/stores/clientPreferences'
import { useScreenWakeLock } from '../screenWakeLock'

class ScreenLock extends EventTarget {
  released = false
  release = vi.fn(async () => {
    this.released = true
    this.dispatchEvent(new Event('release'))
  })
}

describe('screen wake lock', () => {
  let wrapper: VueWrapper | undefined
  let visibility: DocumentVisibilityState
  let lock: ScreenLock
  let request: ReturnType<typeof vi.fn>

  beforeEach(() => {
    visibility = 'visible'
    lock = new ScreenLock()
    request = vi.fn().mockResolvedValue(lock)
    vi.stubGlobal('navigator', { wakeLock: { request } })
    vi.spyOn(document, 'visibilityState', 'get').mockImplementation(() => visibility)
    vi.spyOn(console, 'error').mockImplementation(() => {})
    localStorage.clear()
  })

  afterEach(async () => {
    wrapper?.unmount()
    await flushPromises()
    vi.restoreAllMocks()
    vi.unstubAllGlobals()
    localStorage.clear()
  })

  function start(initial = false) {
    const enabled = ref(initial)
    let result!: ReturnType<typeof useScreenWakeLock>
    wrapper = mount(defineComponent({
      setup() {
        result = useScreenWakeLock(enabled)
        return () => null
      },
    }))
    return { enabled, get error() { return result.error.value } }
  }

  async function setVisibility(value: DocumentVisibilityState) {
    visibility = value
    document.dispatchEvent(new Event('visibilitychange'))
    await flushPromises()
  }

  it('only acquires when enabled, and releases when disabled or unmounted', async () => {
    const state = start()
    expect(request).not.toHaveBeenCalled()
    state.enabled.value = true
    await flushPromises()
    expect(request).toHaveBeenCalledWith('screen')
    state.enabled.value = false
    await flushPromises()
    expect(lock.released).toBe(true)
    const next = new ScreenLock()
    request.mockResolvedValue(next)
    state.enabled.value = true
    await flushPromises()
    wrapper!.unmount()
    await flushPromises()
    expect(next.released).toBe(true)
  })

  it('releases when hidden and reacquires on return, including a system release', async () => {
    start(true)
    await flushPromises()
    await setVisibility('hidden')
    expect(lock.released).toBe(true)
    const next = new ScreenLock()
    request.mockResolvedValue(next)
    await setVisibility('visible')
    expect(request).toHaveBeenCalledTimes(2)
    await next.release()
    request.mockResolvedValue(new ScreenLock())
    await setVisibility('hidden')
    await setVisibility('visible')
    expect(request).toHaveBeenCalledTimes(3)
  })

  it('waits until visible before acquiring a restored preference', async () => {
    visibility = 'hidden'
    start(true)
    await flushPromises()
    expect(request).not.toHaveBeenCalled()
    await setVisibility('visible')
    expect(request).toHaveBeenCalledWith('screen')
    await setVisibility('visible')
    expect(request).toHaveBeenCalledTimes(1)
  })

  it.each(['disabled', 'hidden', 'unmounted'])('releases an in-flight request after becoming %s', async (reason) => {
    let resolve!: (value: ScreenLock) => void
    request.mockReturnValue(new Promise<ScreenLock>(done => { resolve = done }))
    const state = start(true)
    await setVisibility('visible')
    expect(request).toHaveBeenCalledTimes(1)
    if (reason === 'disabled') state.enabled.value = false
    if (reason === 'hidden') await setVisibility('hidden')
    if (reason === 'unmounted') wrapper!.unmount()
    await flushPromises()
    resolve(lock)
    await flushPromises()
    expect(lock.released).toBe(true)
  })

  it('reports denied requests and can retry when returning to the app', async () => {
    request.mockRejectedValueOnce(new DOMException('Denied', 'NotAllowedError'))
    const state = start(true)
    await flushPromises()
    expect(state.error).toContain('Unable to keep the screen awake')
    expect(console.error).toHaveBeenCalled()
    await setVisibility('hidden')
    await setVisibility('visible')
    expect(state.error).toBe('')
    expect(request).toHaveBeenCalledTimes(2)
  })

  it('reports a release failure', async () => {
    const state = start(true)
    await flushPromises()
    lock.release.mockRejectedValueOnce(new Error('Release failed'))
    state.enabled.value = false
    await flushPromises()
    expect(state.error).toBe('Unable to release the screen wake lock.')
    expect(console.error).toHaveBeenCalled()
  })

  it('does not request a lock on unsupported browsers', async () => {
    vi.stubGlobal('navigator', {})
    const state = start(true)
    await flushPromises()
    expect(request).not.toHaveBeenCalled()
    expect(state.error).toBe('')
  })

  it('persists the preference locally and only holds a lock on mobile layouts', async () => {
    function preferences() {
      const pinia = createPinia().use(persistedState)
      createApp({}).use(pinia)
      setActivePinia(pinia)
      return useClientPreferencesStore()
    }
    const first = preferences()
    first.keepScreenAwake = true
    await flushPromises()
    const restored = preferences()
    expect(restored.keepScreenAwake).toBe(true)
    const mobile = ref(false)
    wrapper = mount(defineComponent({
      setup() {
        useScreenWakeLock(computed(() => mobile.value && restored.keepScreenAwake))
        return () => null
      },
    }))
    expect(request).not.toHaveBeenCalled()
    mobile.value = true
    await flushPromises()
    expect(request).toHaveBeenCalledWith('screen')
    mobile.value = false
    await flushPromises()
    expect(lock.released).toBe(true)
  })
})
