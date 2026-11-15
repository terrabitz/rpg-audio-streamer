import { access } from 'node:fs/promises'
import { request, type APIRequestContext } from '@playwright/test'
import { expect, test } from './fixtures'
import { startApp } from './instance'

test('stopping an app closes active WebSocket connections', { tag: '@infrastructure' }, async ({ browser }) => {
  const app = await startApp()
  const context = await browser.newContext({ baseURL: app.url })
  try {
    const page = await context.newPage()
    await page.goto('/login')
    const response = await page.request.post('/api/v1/login', {
      data: { username: 'e2e-gm', password: 'e2e-password' },
    })
    expect(response.ok()).toBe(true)
    await page.evaluate(async () => {
      const socket = new WebSocket(`ws://${location.host}/api/v1/ws`)
      await new Promise<void>((resolve, reject) => {
        socket.onopen = () => resolve()
        socket.onerror = () => reject(new Error('WebSocket connection failed'))
      })
    })
    let timer: ReturnType<typeof setTimeout> | undefined
    try {
      await Promise.race([
        app.stop(),
        new Promise<never>((_, reject) => { timer = setTimeout(() => reject(new Error('App shutdown stalled')), 5000) }),
      ])
    } finally { clearTimeout(timer) }
    await expect(access(app.scratch)).rejects.toMatchObject({ code: 'ENOENT' })
  } finally {
    await context.close()
    await app.stop()
  }
})

test('app instances isolate state and discard it after a failed attempt', { tag: '@infrastructure' }, async ({ app }) => {
  const first = await request.newContext({ baseURL: app.url })
  const secondApp = await startApp()
  const second = await request.newContext({ baseURL: secondApp.url })
  try {
    await login(first)
    await login(second)
    const created = await first.post('/api/v1/tables', { data: { name: 'Isolated table' } })
    expect(created.status()).toBe(201)
    expect(await tableNames(first)).toContain('Isolated table')
    expect(await tableNames(second)).toEqual(['My Table'])

    await expect((async () => {
      try {
        const created = await second.post('/api/v1/tables', { data: { name: 'Abandoned table' } })
        expect(created.status()).toBe(201)
        throw new Error('simulated test failure')
      } finally {
        await second.dispose()
        await secondApp.stop()
      }
    })()).rejects.toThrow('simulated test failure')
    await expect(access(secondApp.scratch)).rejects.toMatchObject({ code: 'ENOENT' })
    expect(await tableNames(first)).toContain('Isolated table')

    const freshApp = await startApp()
    const fresh = await request.newContext({ baseURL: freshApp.url })
    try {
      await login(fresh)
      expect(await tableNames(fresh)).toEqual(['My Table'])
    } finally {
      await fresh.dispose()
      await freshApp.stop()
    }
  } finally {
    await first.dispose()
    await second.dispose()
    await secondApp.stop()
  }
})

async function login(context: APIRequestContext) {
  const response = await context.post('/api/v1/login', {
    data: { username: 'e2e-gm', password: 'e2e-password' },
  })
  expect(response.ok()).toBe(true)
}

async function tableNames(context: APIRequestContext): Promise<string[]> {
  const response = await context.get('/api/v1/tables')
  expect(response.ok()).toBe(true)
  const tables: { name: string }[] = await response.json()
  return tables.map(table => table.name)
}
