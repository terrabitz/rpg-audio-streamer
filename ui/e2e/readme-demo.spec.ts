import { fileURLToPath } from 'node:url'
import { expect, test } from './fixtures'
import { login, uploadTrack } from './helpers'

const screenshotOptions = {
  maxDiffPixelRatio: 0.002,
  stylePath: fileURLToPath(new URL('./readme-demo.css', import.meta.url)),
}

test('GM switches music while a player listens', async ({ page, browser, recordFlow }) => {
  const first = 'Quiet Tavern'
  const second = 'Into the Catacombs'
  await login(page)
  await uploadTrack(page, first, 'Music', 440, 30)
  await uploadTrack(page, second, 'Music', 880, 45)
  await expect(page.getByRole('status').filter({ hasText: 'Track added' })).toBeHidden()

  const playerContext = await browser.newContext()
  try {
    const player = await playerContext.newPage()
    await player.goto(page.url())
    await player.getByRole('button', { name: 'Connect', exact: true }).click()
    const status = player.getByRole('status')
    await expect(status).toHaveText('Nothing is playing')
    await player.getByRole('button', { name: 'Connected, disconnect audio', exact: true }).hover()
    const tooltip = player.getByRole('tooltip')
    await expect(tooltip).toHaveText('Nothing is playing')
    const sidebar = page.getByRole('complementary', { name: 'Currently playing' })
    const playerMedia = player.locator('[data-active="true"]')
    const playingDurations = () => playerMedia.evaluateAll(elements => elements
      .filter((el): el is HTMLMediaElement => el instanceof HTMLMediaElement && !el.paused && el.currentTime > 0)
      .map(el => Math.round(el.duration)))

    recordFlow.start()
    await page.getByRole('group', { name: first, exact: true }).click()
    await expect(sidebar.getByRole('group', { name: `Playing ${first}`, exact: true })).toBeVisible()
    await expect.poll(playingDurations).toEqual([30])
    await expect(tooltip).toHaveText('Now Playing')
    await page.getByRole('group', { name: second, exact: true }).click()
    await expect(sidebar.getByRole('group', { name: `Playing ${second}`, exact: true })).toBeVisible()
    await expect(sidebar.getByRole('group', { name: `Playing ${first}`, exact: true })).toHaveCount(0)
    await expect.poll(playingDurations).toEqual([45])
    await expect(status).toHaveText('Now Playing')
    recordFlow.stop()

    if (process.env.E2E_README_DEMO === '1') {
      await page.mouse.move(0, 0)
      await expect(page).toHaveScreenshot(['readme-demo', 'gm.png'], screenshotOptions)
      await expect(player).toHaveScreenshot(['readme-demo', 'player.png'], screenshotOptions)
    }
  } finally {
    await playerContext.close()
  }
})
