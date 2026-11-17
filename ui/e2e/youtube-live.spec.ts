import { expect, test } from './fixtures'
import { deleteLibraryTrack, login, navigate, openUploadDialog } from './helpers'
import { youtubeTestConfig } from './youtube-config'

test.use({ youtubeFixture: false })
test.skip(process.env.INTEGRATION_TEST_YT_DLP_LINK === undefined, 'Set INTEGRATION_TEST_YT_DLP_LINK to test real YouTube')

test('GM imports a real YouTube video and plays the processed audio', { tag: '@youtube' }, async ({ page }) => {
  const config = youtubeTestConfig()
  await login(page)
  await openUploadDialog(page)
  await page.getByRole('tab', { name: 'YouTube link', exact: true }).click()
  const scan = page.waitForResponse(response =>
    response.request().method() === 'POST' && response.url().endsWith('/api/v1/files/scan'), { timeout: 70_000 })
  await page.getByLabel('YouTube URL', { exact: true }).fill(config.link)
  const scanned = await scan
  expect(scanned.ok(), await scanned.text()).toBe(true)
  const name = page.getByLabel('Track Name', { exact: true })
  await expect(name).toHaveValue(/\S/)
  const trackName = await name.inputValue()
  const metadata = page.getByLabel('Video metadata', { exact: true }).locator('dd')
  await expect(metadata.nth(0)).not.toHaveText('0:00:00')
  await expect(metadata.nth(1)).toHaveText(/\S/)
  await page.getByRole('combobox', { name: 'Track Type', exact: true }).press('ArrowDown')
  await page.getByRole('option', { name: 'Music', exact: true }).click()
  const accepted = page.waitForResponse(response =>
    response.request().method() === 'POST' && response.url().endsWith('/api/v1/files/import'))
  await page.getByRole('button', { name: 'Import', exact: true }).click()
  const imported = await accepted
  expect(imported.status(), await imported.text()).toBe(202)
  await expect(page.getByRole('dialog')).toHaveCount(0)
  const tile = page.getByRole('group', { name: trackName, exact: true })
  await expect(tile).toBeVisible()
  await expect(tile).toHaveAttribute('aria-busy', 'false', { timeout: 15 * 60_000 + 10_000 })
  await expect(tile).not.toContainText('Processing failed')
  await page.reload()
  await expect(tile).toHaveAttribute('aria-busy', 'false')
  await tile.click()
  await expect.poll(() => page.locator('video').first().evaluate(
    (media: HTMLVideoElement) => !media.paused && media.currentTime > 0,
  ), { timeout: 30_000 }).toBe(true)
  await navigate(page, 'Library')
  await deleteLibraryTrack(page, trackName, 1)
})
