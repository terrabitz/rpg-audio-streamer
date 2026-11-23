import { setTimeout as delay } from 'node:timers/promises'
import { fileURLToPath } from 'node:url'
import type { Locator, Page } from '@playwright/test'
import { expect, recording, test } from './fixtures'
import { login, uploadTrack } from './helpers'

test('GM switches music while a player listens', async ({ page, browser, recordFlow }) => {
  const screenshotOptions = process.env.E2E_README_DEMO === '1' ? {
    maxDiffPixelRatio: readDiffRatio(),
    stylePath: fileURLToPath(new URL('./readme-demo.css', import.meta.url)),
  } : undefined
  const first = 'Quiet Tavern'
  const second = 'Into the Catacombs'
  const effect = 'Dungeon Bell'
  await login(page)
  await uploadTrack(page, first, 'Music', 440, 30)
  await uploadTrack(page, second, 'Music', 880, 45)
  await uploadTrack(page, effect, 'One-Shot', 660, 12)
  await expect(page.getByRole('status').filter({ hasText: 'Track added' })).toBeHidden()

  const playerContext = await browser.newContext()
  try {
    const player = await playerContext.newPage()
    await player.goto(page.url())
    await player.getByRole('button', { name: 'Connect', exact: true }).click()
    await player.mouse.move(1200, 680)
    const status = player.getByRole('status')
    await expect(status).toHaveText('Nothing is playing')
    await expect.poll(() => player.getByRole('button', {
      name: 'Connected, disconnect audio',
      exact: true,
    }).evaluate(button => button.matches(':hover'))).toBe(false)
    const sidebar = page.getByRole('complementary', { name: 'Currently playing' })
    const playerMedia = player.locator('[data-active="true"]')
    const playingDurations = () => playerMedia.evaluateAll(elements => elements
      .filter((el): el is HTMLMediaElement => el instanceof HTMLMediaElement && !el.paused && el.currentTime > 0)
      .map(el => Math.round(el.duration)).sort((a, b) => a - b))

    await parkDemoCursor(page)
    recordFlow.start()
    await holdDemoBeat()
    await clickDemoControl(page.getByRole('group', { name: first, exact: true }))
    await expect(sidebar.getByRole('group', { name: `Playing ${first}`, exact: true })).toBeVisible()
    await expect.poll(playingDurations).toEqual([30])
    await expect(status).toHaveText('Now Playing')
    await holdDemoBeat()
    await clickDemoControl(page.getByRole('group', { name: second, exact: true }))
    await expect(sidebar.getByRole('group', { name: `Playing ${second}`, exact: true })).toBeVisible()
    await expect(sidebar.getByRole('group', { name: `Playing ${first}`, exact: true })).toHaveCount(0)
    await expect.poll(playingDurations).toEqual([45])
    await holdDemoBeat()
    await clickDemoControl(page.getByRole('group', { name: effect, exact: true }))
    await expect(sidebar.getByRole('group', { name: `Playing ${effect}`, exact: true })).toBeVisible()
    await expect.poll(playingDurations).toEqual([12, 45])
    await holdDemoBeat()
    await clickDemoControl(sidebar.getByRole('button', { name: `Pause ${effect}`, exact: true }))
    await clickDemoControl(sidebar.getByRole('button', { name: `Pause ${second}`, exact: true }))
    await expect(sidebar.getByText('No tracks playing.')).toBeVisible()
    await expect.poll(playingDurations).toEqual([])
    await expect(status).toHaveText('Nothing is playing')
    await parkDemoCursor(page)
    recordFlow.stop()

    if (screenshotOptions) {
      await expect(page).toHaveScreenshot(['readme-demo', 'gm.png'], screenshotOptions)
      await expect(player).toHaveScreenshot(['readme-demo', 'player.png'], screenshotOptions)
    }
  } finally {
    await playerContext.close()
  }
})

async function clickDemoControl(control: Locator) {
  if (recording) {
    await control.hover()
    await settleDemoCursor()
  }
  await control.click()
}

async function parkDemoCursor(page: Page) {
  if (!recording) return
  await page.mouse.move(1200, 680)
  await settleDemoCursor()
}

async function settleDemoCursor() {
  // Leave a visible beat after the recording overlay's 300 ms glide.
  await delay(500)
}

async function holdDemoBeat() {
  if (recording) await delay(2000)
}

function readDiffRatio() {
  const input = process.env.README_DEMO_DIFF_RATIO ?? '0.002'
  const ratio = Number(input)
  if (!input.trim() || !Number.isFinite(ratio) || ratio < 0 || ratio > 1) {
    throw new Error('README_DEMO_DIFF_RATIO must be a number between 0 and 1')
  }
  return ratio
}
