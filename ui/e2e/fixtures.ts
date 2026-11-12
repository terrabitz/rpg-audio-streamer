import { mkdir, writeFile } from 'node:fs/promises'
import path from 'node:path'
import { test as base, type BrowserContext, type Page } from '@playwright/test'

export { expect } from '@playwright/test'

export const recording = !!process.env.E2E_RECORD
export const recordingViewport = { width: 1280, height: 720 }
export const recordingManifestDir = path.resolve('test-results', 'recordings')

type TrackedPage = { page: Page; label: string; offsetMs: number }

// In recording mode every page, including player pages from browser.newContext(),
// is captured with its start offset so record.mjs can compose one time-aligned GIF per test.
export const test = base.extend<{ recorder: void }>({
  recorder: [
    async ({ browser, context }, use, testInfo) => {
      if (!recording) {
        await use()
        return
      }
      const started = Date.now()
      const pages: TrackedPage[] = []
      const track = (ctx: BrowserContext, label: string) => {
        let count = 0
        ctx.on('page', (page: Page) => {
          count += 1
          const pageLabel = count > 1 ? `${label} (tab ${count})` : label
          const offsetMs = Date.now() - started
          void page.addInitScript(showInteractions, pageLabel)
          pages.push({ page, label: pageLabel, offsetMs })
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

      const durationMs = Date.now() - started
      // Playwright discards temporary videos during context teardown, so finalize them here.
      const recorded = await Promise.all(pages.map(async ({ page, label, offsetMs }, index) => {
        await page.close()
        const video = testInfo.outputPath(`recording-${index}.webm`)
        await page.video()!.saveAs(video)
        return { video, label, offsetMs }
      }))
      const manifest = { title: testInfo.title, status: testInfo.status, durationMs, pages: recorded }
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

// Playwright videos omit the pointer, so recordings draw one plus a page label.
function showInteractions(label: string) {
  const install = () => {
    const badge = document.createElement('div')
    badge.textContent = label
    badge.setAttribute('aria-hidden', 'true')
    Object.assign(badge.style, {
      position: 'fixed', left: '8px', bottom: '8px', zIndex: '2147483647', pointerEvents: 'none',
      padding: '4px 12px', borderRadius: '12px', background: 'rgba(0, 0, 0, 0.75)', color: '#fff',
      font: '600 16px system-ui, sans-serif',
    })
    const cursor = document.createElement('div')
    cursor.setAttribute('aria-hidden', 'true')
    Object.assign(cursor.style, {
      position: 'fixed', left: '-100px', top: '-100px', width: '22px', height: '22px',
      zIndex: '2147483647', pointerEvents: 'none', borderRadius: '50%',
      border: '3px solid #ff3d3d', background: 'rgba(255, 61, 61, 0.35)',
      transform: 'translate(-50%, -50%)', transition: 'left 0.3s, top 0.3s, transform 0.15s',
    })
    document.addEventListener('mousemove', event => {
      cursor.style.left = `${event.clientX}px`
      cursor.style.top = `${event.clientY}px`
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
}
