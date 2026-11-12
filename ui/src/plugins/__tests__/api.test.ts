import { expect, it } from 'vitest'
import { createClientConfig } from '../api'

it('throws HTTP errors at runtime, not only in generated SDK types', () => {
  expect(createClientConfig()).toMatchObject({
    baseUrl: import.meta.env.VITE_API_BASE_URL,
    throwOnError: true,
  })
})
