import { mkdir, writeFile } from 'node:fs/promises'
import path from 'node:path'
import { test as base, type BrowserContext, type Page } from '@playwright/test'
import { startApp, type AppInstance } from './instance'

export { expect } from '@playwright/test'

export const recording = !!process.env.E2E_RECORD
export const recordingViewport = { width: 1280, height: 720 }
export const recordingManifestDir = path.resolve('test-results', 'recordings')

type TrackedPage = { page: Page; label: string; offsetMs: number }
type RecordingWindow = { startMs?: number; endMs?: number }
type RecordFlow = { start: () => void; stop: () => void }

// In recording mode every page, including player pages from browser.newContext(),
// is captured with its start offset so record.mjs can compose one time-aligned GIF per test.
export const test = base.extend<{
  app: AppInstance
  recordFlow: RecordFlow
  recorder: void
  recordingWindow: RecordingWindow
  youtubeFixture: boolean
  firstRun: boolean
  recordingCursorSeed: number | null
}>({
  youtubeFixture: [false, { option: true }],
  firstRun: [false, { option: true }],
  recordingCursorSeed: [null, { option: true }],
  app: [async ({ browser, youtubeFixture, firstRun }, use, testInfo) => {
    const existing = new Set(browser.contexts())
    const liveDiscord = testInfo.tags.includes('@discord')
    const app = await startApp(liveDiscord, youtubeFixture, firstRun)
    try {
      await use(app)
    } finally {
      try {
        await Promise.all(browser.contexts().filter(context => !existing.has(context)).map(context => context.close()))
      } finally {
        await app.stop()
        if (!liveDiscord && testInfo.status !== testInfo.expectedStatus && app.logs()) {
          await testInfo.attach('app.log', { body: app.logs(), contentType: 'text/plain' })
        }
      }
    }
  }, { timeout: 120_000 }],
  baseURL: async ({ app }, use) => {
    await use(app.url)
  },
  recordingWindow: async ({}, use) => {
    await use({})
  },
  recordFlow: async ({ recordingWindow }, use) => {
    await use({
      start() {
        if (!recording) return
        if (recordingWindow.startMs !== undefined) throw new Error('Record only one flow per test')
        recordingWindow.startMs = performance.now()
      },
      stop() {
        if (!recording) return
        if (recordingWindow.startMs === undefined || recordingWindow.endMs !== undefined) {
          throw new Error('Start the recording window before stopping it')
        }
        recordingWindow.endMs = performance.now()
      },
    })
  },
  recorder: [
    async ({ browser, context, recordingWindow, recordingCursorSeed }, use, testInfo) => {
      if (!recording) {
        await use()
        return
      }
      const started = performance.now()
      const pages: TrackedPage[] = []
      const track = (ctx: BrowserContext, label: string) => {
        let count = 0
        ctx.on('page', (page: Page) => {
          count += 1
          const pageLabel = count > 1 ? `${label} (tab ${count})` : label
          const offsetMs = performance.now() - started
          void page.addInitScript(showInteractions, { label: pageLabel, cursorSeed: recordingCursorSeed })
          const tracked = { page, label: pageLabel, offsetMs }
          page.on('framenavigated', frame => {
            if (frame === page.mainFrame() && isGMPath(new URL(frame.url()).pathname)) {
              tracked.label = gmLabel(pageLabel)
            }
          })
          pages.push(tracked)
        })
      }
      track(context, 'GM')

      const newContext = browser.newContext.bind(browser)
      let players = 0
      browser.newContext = async (options = {}) => {
        const playerContext = await newContext({
          ...options,
          viewport: recordingViewport,
          recordVideo: { dir: testInfo.outputPath('videos'), size: recordingViewport },
        })
        players += 1
        track(playerContext, `Player ${players}`)
        return playerContext
      }
      try {
        await use()
      } finally {
        browser.newContext = newContext
      }

      if (testInfo.status === 'passed' && recordingWindow.startMs !== undefined && recordingWindow.endMs === undefined) {
        throw new Error('Stop the recording window before the end of the test')
      }
      const startMs = (recordingWindow.startMs ?? started) - started
      const endMs = (recordingWindow.endMs ?? performance.now()) - started
      // Playwright discards temporary videos during context teardown, so finalize them here.
      const recorded = await Promise.all(pages.filter(({ offsetMs }) => offsetMs < endMs)
        .map(async ({ page, label, offsetMs }, index) => {
          await page.close()
          const video = testInfo.outputPath(`recording-${index}.webm`)
          await page.video()!.saveAs(video)
          return { video, label, offsetMs }
        }))
      const manifest = { title: testInfo.title, status: testInfo.status, startMs, endMs, pages: recorded }
      await mkdir(recordingManifestDir, { recursive: true })
      await writeFile(
        path.join(recordingManifestDir, `${slugify(testInfo.title)}.json`),
        JSON.stringify(manifest, null, 2),
      )
    },
    { auto: true },
  ],
})

export function slugify(title: string) {
  return title.toLowerCase().replace(/[^a-z0-9]+/g, '-').replace(/^-|-$/g, '')
}

function isGMPath(pathname: string) {
  // The individual table URL is shared by both roles.
  return /^\/(login|tables|library)\/?$/.test(pathname)
}

function gmLabel(label: string) {
  return label.replace(/^Player (\d+)/, 'GM (device $1)')
}

// Playwright videos omit the pointer, so recordings draw one plus a page label.
function showInteractions({ label, cursorSeed }: { label: string; cursorSeed: number | null }) {
  const install = () => {
    const badge = document.createElement('div')
    badge.dataset.recordingOverlay = ''
    badge.textContent = /^\/(login|tables|library)\/?$/.test(location.pathname)
      ? label.replace(/^Player (\d+)/, 'GM (device $1)') : label
    badge.setAttribute('aria-hidden', 'true')
    Object.assign(badge.style, {
      position: 'fixed', left: '8px', bottom: '8px', zIndex: '2147483647', pointerEvents: 'none',
      padding: '4px 12px', borderRadius: '12px', background: 'rgba(0, 0, 0, 0.75)', color: '#fff',
      font: '600 16px system-ui, sans-serif',
    })
    const cursor = document.createElement('div')
    cursor.dataset.recordingOverlay = ''
    cursor.setAttribute('aria-hidden', 'true')
    Object.assign(cursor.style, {
      position: 'fixed', left: '-100px', top: '-100px', width: '22px', height: '22px',
      zIndex: '2147483647', pointerEvents: 'none', borderRadius: '50%',
      border: '3px solid #ff3d3d', background: 'rgba(255, 61, 61, 0.35)',
      transform: 'translate(-50%, -50%)',
      transition: cursorSeed === null ? 'left 0.3s, top 0.3s, transform 0.15s' : 'transform 0.15s',
    })
    let previous: { x: number; y: number } | undefined
    let motion: Animation | undefined
    document.addEventListener('mousemove', ({ clientX: x, clientY: y }) => {
      if (previous?.x === x && previous.y === y) return
      cursor.style.left = `${x}px`
      cursor.style.top = `${y}px`
      if (cursorSeed !== null && previous) {
        motion?.cancel()
        motion = cursor.animate(cursorPath(previous, { x, y }, cursorSeed), 500)
        motion.id = 'recording-cursor-motion'
      }
      previous = { x, y }
    }, true)
    document.addEventListener('mousedown', () => {
      cursor.style.transform = 'translate(-50%, -50%) scale(1.8)'
    }, true)
    document.addEventListener('mouseup', () => {
      cursor.style.transform = 'translate(-50%, -50%)'
    }, true)
    document.documentElement.append(badge, cursor)
  }
  if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', install)
    return
  }
  install()

  function cursorPath(from: { x: number; y: number }, to: { x: number; y: number }, seed: number) {
    // Hash the endpoints so setup events and browser timing cannot change the bend.
    let hash = seed
    for (const coordinate of [from.x, from.y, to.x, to.y]) {
      hash = Math.imul(hash ^ coordinate, 16777619)
    }
    hash ^= hash >>> 16
    const dx = to.x - from.x
    const dy = to.y - from.y
    const bend = (hash & 1 ? 1 : -1) * (0.12 + ((hash >>> 1) % 100) / 1000)
    return Array.from({ length: 21 }, (_, i) => {
      const t = i / 20
      const progress = t * t * (3 - 2 * t)
      const arc = 4 * progress * (1 - progress) * bend
      return {
        left: `${from.x + dx * progress - dy * arc}px`,
        top: `${from.y + dy * progress + dx * arc}px`,
      }
    })
  }
}
