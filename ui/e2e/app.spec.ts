import { execFileSync } from 'node:child_process'
import { expect, test, type Page } from '@playwright/test'

test('GM can recover from invalid credentials, keep a session, and log out', async ({ page }) => {
  await page.goto('/')
  await page.getByRole('link', { name: 'GM Login' }).click()
  await page.getByLabel('Username').fill('e2e-gm')
  await page.getByLabel('Password', { exact: true }).fill('wrong-password')
  await page.getByRole('button', { name: 'Login', exact: true }).click()
  await expect(page.getByText('Invalid credentials', { exact: true })).toBeVisible()
  await page.getByLabel('Password', { exact: true }).fill('e2e-password')
  await page.getByRole('button', { name: 'Login', exact: true }).click()
  await expect(page.getByRole('button', { name: 'Upload Track' })).toBeVisible()
  await page.reload()
  await expect(page.getByRole('button', { name: 'Upload Track' })).toBeVisible()
  await page.getByRole('button', { name: 'Logout' }).click()
  await expect(page.getByRole('link', { name: 'GM Login' })).toBeVisible()
  await page.reload()
  await expect(page.getByRole('link', { name: 'GM Login' })).toBeVisible()
})

test('GM uploads, renames, persists, and deletes an audio track', async ({ page }) => {
  await login(page)
  await uploadTrack(page, 'Forest')
  const tile = page.locator('.file-tile').filter({ hasText: 'Forest' })
  await expect(tile).toBeVisible()
  await tile.getByRole('button', { name: 'Edit Forest' }).click()
  await page.getByLabel('Track Name', { exact: true }).fill('Rain')
  await page.getByRole('button', { name: 'Save Changes' }).click()
  await expect(page.locator('.file-tile').filter({ hasText: 'Rain' })).toBeVisible()
  await page.reload()
  const renamed = page.locator('.file-tile').filter({ hasText: 'Rain' })
  await expect(renamed).toBeVisible()
  await renamed.getByRole('button', { name: 'Edit Rain' }).click()
  await page.getByRole('button', { name: 'Delete track', exact: true }).click()
  await expect(renamed).toHaveCount(0)
  await expect(page.locator('video')).toHaveCount(0)
  await page.reload()
  await expect(page.locator('.file-tile')).toHaveCount(0)
})

test('failed audio conversion is shown as an upload error', async ({ page }) => {
  await login(page)
  await page.getByRole('button', { name: 'Upload Track' }).click()
  await page.getByLabel('Select a file', { exact: true }).setInputFiles({
    name: 'invalid.mp3',
    mimeType: 'audio/mpeg',
    buffer: Buffer.from('not an audio file'),
  })
  await page.getByRole('combobox', { name: 'Track Type', exact: true }).press('ArrowDown')
  await page.getByRole('option', { name: 'Music', exact: true }).click()
  await page.getByRole('button', { name: 'Upload', exact: true }).click()
  await expect(page.getByText('Failed to upload file', { exact: true })).toBeVisible()
  await expect(page.locator('.file-tile')).toHaveCount(0)
})

test('invited player receives live playback, late-join state, and pause', async ({
  page,
  browser,
}) => {
  await login(page)
  await uploadTrack(page, 'Session Music')
  const tile = page.locator('.file-tile').filter({ hasText: 'Session Music' })
  await tile.click()
  const gmAudio = page.locator('video')
  await expect
    .poll(() => gmAudio.evaluate((el: HTMLVideoElement) => !el.paused && el.currentTime > 0))
    .toBe(true)

  const playerContext = await browser.newContext()
  try {
    const player = await playerContext.newPage()
    await player.goto('/table/e2e-invite')
    await expect(player.getByText('Volume Mixer', { exact: true })).toBeVisible()
    await expect(player.getByRole('button', { name: 'Upload Track' })).toHaveCount(0)
    await player.getByRole('button', { name: 'Connect Audio', exact: true }).click()
    await expect(player.getByText('Connected', { exact: true })).toBeVisible()
    const playerAudio = player.locator('video')
    await expect(playerAudio).toHaveCount(1)
    await expect
      .poll(() => playerAudio.evaluate((el: HTMLVideoElement) => !el.paused && el.currentTime > 0))
      .toBe(true)
    await expect.poll(() => playerAudio.evaluate((el: HTMLVideoElement) => el.loop)).toBe(true)
    const master = player.locator('.mixer-row').filter({ hasText: 'Master' }).getByRole('slider')
    await master.press('Home')
    await expect.poll(() => playerAudio.evaluate((el: HTMLVideoElement) => el.volume)).toBe(0)
    await master.press('End')
    await expect
      .poll(() => playerAudio.evaluate((el: HTMLVideoElement) => el.volume))
      .toBeGreaterThan(0)
    const music = player.locator('.mixer-row').filter({ hasText: 'Music' }).getByRole('slider')
    await music.press('Home')
    await expect.poll(() => playerAudio.evaluate((el: HTMLVideoElement) => el.volume)).toBe(0)
    await music.press('End')
    await expect
      .poll(() => playerAudio.evaluate((el: HTMLVideoElement) => el.volume))
      .toBeGreaterThan(0)
    await tile.click()
    await expect.poll(() => playerAudio.evaluate((el: HTMLVideoElement) => el.paused)).toBe(true)
    await tile.click()
    await expect.poll(() => playerAudio.evaluate((el: HTMLVideoElement) => !el.paused)).toBe(true)
    await player.getByRole('button', { name: 'Disconnect Audio', exact: true }).click()
    await expect(player.getByRole('button', { name: 'Connect Audio', exact: true })).toBeVisible()
    await expect(player.locator('video')).toHaveCount(0)
  } finally {
    await playerContext.close()
  }
  await tile.getByRole('button', { name: 'Edit Session Music' }).click()
  await page.getByRole('button', { name: 'Delete track', exact: true }).click()
  await expect(tile).toHaveCount(0)
})

test('starting another Music track stops the previous one', async ({ page }) => {
  await login(page)
  await uploadTrack(page, 'First Music')
  await uploadTrack(page, 'Second Music')
  const first = page.locator('.file-tile').filter({ hasText: 'First Music' })
  const second = page.locator('.file-tile').filter({ hasText: 'Second Music' })
  await first.click()
  await expect
    .poll(() =>
      page
        .locator('video')
        .first()
        .evaluate((el: HTMLVideoElement) => !el.paused && el.currentTime > 0),
    )
    .toBe(true)
  await second.click()
  await expect
    .poll(() =>
      page
        .locator('video')
        .first()
        .evaluate((el: HTMLVideoElement) => el.paused),
    )
    .toBe(true)
  await expect
    .poll(() =>
      page
        .locator('video')
        .last()
        .evaluate((el: HTMLVideoElement) => !el.paused && el.currentTime > 0),
    )
    .toBe(true)
  for (const [tile, name] of [
    [first, 'First Music'],
    [second, 'Second Music'],
  ] as const) {
    await tile.getByRole('button', { name: `Edit ${name}` }).click()
    await page.getByRole('button', { name: 'Delete track', exact: true }).click()
    await expect(tile).toHaveCount(0)
  }
  await expect(page.locator('video')).toHaveCount(0)
})

async function login(page: Page) {
  await page.goto('/login')
  await page.getByLabel('Username').fill('e2e-gm')
  await page.getByLabel('Password', { exact: true }).fill('e2e-password')
  await page.getByRole('button', { name: 'Login', exact: true }).click()
  await expect(page.getByRole('button', { name: 'Upload Track' })).toBeVisible()
}

async function uploadTrack(page: Page, name: string) {
  const audio = execFileSync('ffmpeg', [
    '-v',
    'error',
    '-f',
    'lavfi',
    '-i',
    'sine=frequency=440:duration=30',
    '-f',
    'mp3',
    'pipe:1',
  ])
  await page.getByRole('button', { name: 'Upload Track' }).click()
  await page
    .getByLabel('Select a file', { exact: true })
    .setInputFiles({ name: 'sample.mp3', mimeType: 'audio/mpeg', buffer: audio })
  await page.getByLabel('Track Name', { exact: true }).fill(name)
  await page.getByRole('combobox', { name: 'Track Type', exact: true }).press('ArrowDown')
  await page.getByRole('option', { name: 'Music', exact: true }).click()
  await page.getByRole('button', { name: 'Upload', exact: true }).click()
  await expect(page.getByText('File uploaded successfully!', { exact: true })).toBeVisible()
}
