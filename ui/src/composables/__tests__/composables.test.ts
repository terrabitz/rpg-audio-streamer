import { describe, expect, it } from 'vitest'
import { defineComponent } from 'vue'
import { patchObject } from '../util'
import { useAppBar } from '../useAppBar'
import { useBaseUrl } from '../useBaseUrl'

describe('shared UI state', () => {
  it('applies partial patches without mutating either input', () => {
    const original = { name: 'Forest', volume: 100 }
    const patch = { volume: 0 }
    expect(patchObject(original, patch)).toEqual({ name: 'Forest', volume: 0 })
    expect(original).toEqual({ name: 'Forest', volume: 100 })
    expect(patch).toEqual({ volume: 0 })
  })

  it('shares app bar titles and actions across callers', () => {
    const first = useAppBar()
    const second = useAppBar()
    const component = defineComponent({ template: '<button>Upload</button>' })
    first.setTitle('My Table')
    first.setActions([component])
    expect(second.title.value).toBe('My Table')
    expect(second.actions.value).toEqual([component])
    first.setActions([])
    first.setTitle('Skald Bot')
  })

  it('builds link origins from the current browser location', () => {
    expect(useBaseUrl().getBaseUrl()).toBe(window.location.origin)
  })
})
