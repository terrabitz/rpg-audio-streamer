import { execFileSync } from 'node:child_process'
import type { Page } from '@playwright/test'
import { expect, test } from './fixtures'

test('GM can recover from invalid credentials, keep a session, and log out', async ({ page }) => {
  await page.goto('/')
  await page.getByRole('link', { name: 'GM Login' }).click()
  await page.getByLabel('Username').fill('e2e-gm')
  await page.getByLabel('Password', { exact: true }).fill('wrong-password')
  await page.getByRole('button', { name: 'Login', exact: true }).click()
  await expect(page.getByText('Invalid credentials', { exact: true })).toBeVisible()
  await page.getByLabel('Password', { exact: true }).fill('e2e-password')
  await page.getByRole('button', { name: 'Login', exact: true }).click()
  await expect(page.getByRole('heading', { name: 'Tables', exact: true })).toBeVisible()
  await expect(page.getByRole('link', { name: 'Open My Table', exact: true })).toHaveText('My Table')
  await page.getByRole('link', { name: 'Open My Table', exact: true }).click()
  await expect(page.getByRole('button', { name: 'Add Track', exact: true })).toBeVisible()
  await page.reload()
  await expect(page.getByRole('button', { name: 'Add Track', exact: true })).toBeVisible()
  await page.getByRole('button', { name: 'Logout' }).click()
  await expect(page.getByRole('link', { name: 'GM Login' })).toBeVisible()
  await page.reload()
  await expect(page.getByRole('link', { name: 'GM Login' })).toBeVisible()
})

test('GM uploads, renames, persists, and deletes an audio track', async ({ page }) => {
  await login(page)
  await page.getByRole('link', { name: 'Library', exact: true }).click()
  await uploadTrack(page, 'Forest')
  const row = page.getByRole('row').filter({ hasText: 'Forest' })
  await expect(row).toBeVisible()
  await expect(page.getByRole('columnheader', { name: 'Name', exact: true })).toBeVisible()
  await expect(page.getByRole('columnheader', { name: 'Type', exact: true })).toBeVisible()
  await expect(page.getByRole('columnheader', { name: 'Added', exact: true })).toBeVisible()
  await expect(page.getByRole('columnheader', { name: 'Playback', exact: true })).toHaveCount(0)
  await expect(page.getByRole('heading', { name: 'Track library', exact: true })).toHaveCount(0)
  await expect(page.getByText('Tracks can be shared by multiple tables.', { exact: false })).toHaveCount(0)
  await expect(page.getByText('Click a row to preview a track locally.', { exact: false })).toHaveCount(0)
  await expect(row.getByText('Music', { exact: true })).toBeVisible()
  await expect(row.locator('time')).toHaveAttribute('datetime', /^\d{4}-\d{2}-\d{2}T/)
  await row.getByRole('button', { name: 'Edit Forest' }).click()
  await expect(page.locator('video')).toHaveCount(0)
  await page.getByLabel('Track Name', { exact: true }).fill('Rain')
  await page.getByRole('button', { name: 'Save Changes' }).click()
  await expect(page.getByRole('row').filter({ hasText: 'Rain' })).toBeVisible()
  await page.reload()
  const renamed = page.getByRole('row').filter({ hasText: 'Rain' })
  await expect(renamed).toBeVisible()
  await renamed.click()
  await expect.poll(() => page.locator('video').evaluate(
    (el: HTMLVideoElement) => !el.paused && el.currentTime > 0,
  )).toBe(true)
  await renamed.getByRole('button', { name: 'Edit Rain' }).click()
  await page.getByRole('button', { name: 'Delete track', exact: true }).click()
  await expect(renamed).toHaveCount(0)
  await expect(page.locator('video')).toHaveCount(0)
  await page.reload()
  await expect(page.getByText('No tracks in the library yet.', { exact: true })).toBeVisible()
})

test('GM creates, renames, opens, and deletes tables', async ({ page }) => {
  await login(page)
  await page.getByRole('link', { name: 'Tables', exact: true }).click()
  await createTable(page, 'Friday')
  const options = page.getByRole('button', { name: 'Options for Friday', exact: true })
  await options.focus()
  await options.press('Enter')
  await expect(page.getByRole('menuitem', { name: 'Rename Friday', exact: true })).toBeVisible()
  await expect(page).toHaveURL(/\/tables$/)
  await page.keyboard.press('Escape')
  await expect(page.getByRole('menuitem', { name: 'Rename Friday', exact: true })).toHaveCount(0)
  await options.click()
  await expect(page).toHaveURL(/\/tables$/)
  await page.getByRole('menuitem', { name: 'Rename Friday', exact: true }).click()
  await expect(page).toHaveURL(/\/tables$/)
  await page.getByLabel('Table name', { exact: true }).fill('Friday group')
  await page.getByRole('button', { name: 'Save table', exact: true }).click()
  await page.reload()
  await page.getByRole('link', { name: 'Open Friday group', exact: true }).click()
  await expect(page.getByRole('banner')).toContainText('Friday group')
  await expect(page.getByRole('heading', { name: 'Friday group', exact: true })).toHaveCount(0)
  await expect(page.locator('.file-tile')).toHaveCount(0)
  await page.getByRole('link', { name: 'Tables', exact: true }).click()
  await deleteTable(page, 'Friday group')
  await page.reload()
  await expect(page.getByRole('link', { name: 'Open Friday group', exact: true })).toHaveCount(0)
  await expect(page.getByRole('link', { name: 'Open My Table', exact: true })).toBeVisible()
})

test('GM adds library tracks or new uploads to a table from one dialog', async ({ page }) => {
  test.slow()
  await login(page)
  await page.getByRole('link', { name: 'Library', exact: true }).click()
  await uploadTrack(page, 'Shared Forest')
  await page.getByRole('link', { name: 'Tables', exact: true }).click()
  await createTable(page, 'Friday')
  await page.getByRole('link', { name: 'Open Friday', exact: true }).click()
  await expect(page.getByRole('combobox', { name: 'Library track', exact: true })).toHaveCount(0)
  await page.getByRole('button', { name: 'Add Track', exact: true }).click()
  await expect(page.getByRole('tab', { name: 'From library', exact: true })).toBeVisible()
  await expect(page.getByRole('tab', { name: 'Upload new', exact: true })).toBeVisible()
  await page.getByRole('combobox', { name: 'Library track', exact: true }).press('ArrowDown')
  await page.getByRole('option', { name: 'Shared Forest', exact: true }).click()
  await page.getByRole('button', { name: 'Cancel', exact: true }).click()
  await expect(page.locator('.file-tile')).toHaveCount(0)
  await addExistingTrack(page, 'Shared Forest')
  await page.getByRole('button', { name: 'Add Track', exact: true }).click()
  await expect(page.getByText('No unused tracks in the library. Upload a new track instead.')).toBeVisible()
  await expect(page.getByRole('button', { name: 'Add to table', exact: true })).toBeDisabled()
  await page.getByRole('button', { name: 'Cancel', exact: true }).click()
  await uploadTrack(page, 'Friday Only')
  await page.reload()
  await expect(page.locator('.file-tile')).toHaveCount(2)
  await page.getByRole('link', { name: 'Library', exact: true }).click()
  await expect(page.getByRole('row').filter({ hasText: 'Friday Only' })).toBeVisible()
  await expect(page.getByRole('row').filter({ hasText: 'Shared Forest' })).toBeVisible()
  await page.getByRole('link', { name: 'Tables', exact: true }).click()
  await deleteTable(page, 'Friday')
  await deleteLibraryTrack(page, 'Shared Forest', 0)
  await deleteLibraryTrack(page, 'Friday Only', 0)
  await expect(page.getByText('No tracks in the library yet.', { exact: true })).toBeVisible()
})

test('deleting a shared library track confirms affected tables in a follow-up dialog', async ({
  page, context,
}) => {
  test.slow()
  await login(page)
  await page.getByRole('link', { name: 'Library', exact: true }).click()
  await uploadTrack(page, 'Shared Forest')
  await page.getByRole('link', { name: 'Tables', exact: true }).click()
  await createTable(page, 'Friday')
  await createTable(page, 'Sunday')
  await page.getByRole('link', { name: 'Open Friday', exact: true }).click()
  await addExistingTrack(page, 'Shared Forest')
  const sundayGM = await context.newPage()
  try {
    await sundayGM.goto('/tables')
    await sundayGM.getByRole('link', { name: 'Open Sunday', exact: true }).click()

    await page.getByRole('link', { name: 'Library', exact: true }).click()
    const row = page.getByRole('row').filter({ hasText: 'Shared Forest' })
    await row.getByRole('button', { name: 'Edit Shared Forest', exact: true }).click()
    const editor = page.getByRole('dialog').filter({ has: page.getByLabel('Track Name', { exact: true }) })
    const confirmation = page.getByRole('dialog', { name: 'Delete track?', exact: true })
    await page.getByRole('button', { name: 'Delete track', exact: true }).click()
    await expect(confirmation).toContainText('Deleting Shared Forest will remove it from 1 table.')
    await expect(editor).toBeVisible()

    await addExistingTrack(sundayGM, 'Shared Forest')
    await confirmation.getByRole('button', { name: 'Confirm delete track', exact: true }).click()
    await expect(confirmation).toHaveCount(0)
    await expect(editor).toContainText('Table usage changed.')
    await expect(row).toBeVisible()

    await page.getByRole('button', { name: 'Delete track', exact: true }).click()
    await expect(confirmation).toContainText('Deleting Shared Forest will remove it from 2 tables.')
    await confirmation.getByRole('button', { name: 'Cancel', exact: true }).click()
    await expect(confirmation).toHaveCount(0)
    await expect(editor).toBeVisible()
    await expect(row).toBeVisible()

    await page.getByRole('button', { name: 'Delete track', exact: true }).click()
    await confirmation.getByRole('button', { name: 'Confirm delete track', exact: true }).click()
    await expect(page.getByRole('dialog')).toHaveCount(0)
    await expect(row).toHaveCount(0)
    await expect(sundayGM.locator('.file-tile')).toHaveCount(0)
  } finally {
    await sundayGM.close()
  }
  await page.getByRole('link', { name: 'Tables', exact: true }).click()
  await deleteTable(page, 'Friday')
  await deleteTable(page, 'Sunday')
})

test('library row previews stay local while a table is streaming', async ({ page, context, browser }) => {
  test.slow()
  await login(page)
  await uploadTrack(page, 'Live Music')
  await page.locator('.file-tile').filter({ hasText: 'Live Music' }).click()
  await expect.poll(() => page.locator('video').evaluate(
    (el: HTMLVideoElement) => !el.paused && el.currentTime > 0,
  )).toBe(true)

  const playerContext = await browser.newContext()
  const library = await context.newPage()
  const previewSockets: string[] = []
  library.on('websocket', socket => previewSockets.push(socket.url()))
  try {
    const player = await playerContext.newPage()
    await player.goto('/table/e2e-invite')
    await player.getByRole('button', { name: 'Connect Audio', exact: true }).click()
    await expect.poll(() => player.locator('video').evaluate(
      (el: HTMLVideoElement) => !el.paused && el.currentTime > 0,
    )).toBe(true)
    const playerSource = await player.locator('video').evaluate((el: HTMLVideoElement) => el.src)
    await library.goto('/library')
    await uploadTrack(library, 'Private Preview')
    const row = library.getByRole('row').filter({ hasText: 'Private Preview' })
    await row.click()
    await expect.poll(() => library.locator('video').evaluate(
      (el: HTMLVideoElement) => !el.paused && el.currentTime > 0,
    )).toBe(true)
    await expect(row.getByRole('button', { name: 'Pause preview Private Preview', exact: true })).toBeVisible()
    await row.getByRole('button', { name: 'Edit Private Preview', exact: true }).click()
    await expect(library.getByRole('dialog')).toBeVisible()
    await expect(row.getByRole('button', { name: 'Pause preview Private Preview', exact: true })).toBeVisible()
    await library.getByRole('button', { name: 'Close track controls', exact: true }).click()
    await row.focus()
    await row.press('Enter')
    await expect.poll(() => library.locator('video').evaluate(
      (el: HTMLVideoElement) => el.paused,
    )).toBe(true)
    await row.press('Space')
    await expect.poll(() => library.locator('video').evaluate(
      (el: HTMLVideoElement) => !el.paused && el.currentTime > 0,
    )).toBe(true)
    await library.getByRole('row').filter({ hasText: 'Live Music' }).click()
    await expect(library.locator('video')).toHaveCount(1)
    await expect(library.getByRole('button', { name: 'Preview Private Preview', exact: true })).toBeVisible()
    await expect.poll(() => library.locator('video').evaluate(
      (el: HTMLVideoElement) => !el.paused && el.currentTime > 0,
    )).toBe(true)
    expect(previewSockets).toEqual([])
    await expect(player.locator('video')).toHaveCount(1)
    await expect(player.locator('video')).toHaveAttribute('src', playerSource)
    await expect.poll(() => player.locator('video').evaluate(
      (el: HTMLVideoElement) => !el.paused && el.currentTime > 0,
    )).toBe(true)
    await library.getByRole('link', { name: 'Tables', exact: true }).click()
    await expect(library.locator('video')).toHaveCount(0)
    await library.getByRole('link', { name: 'Open My Table', exact: true }).click()
    await expect(library.locator('.file-tile')).toHaveCount(1)
    await expect(library.getByText('Private Preview', { exact: true })).toHaveCount(0)
    await expect.poll(() => library.locator('video').evaluate(
      (el: HTMLVideoElement) => el.paused,
    )).toBe(true)
  } finally {
    await library.close()
    await playerContext.close()
  }
  await deleteLibraryTrack(page, 'Private Preview', 0)
  await deleteLibraryTrack(page, 'Live Music', 1)
})

test('failed audio conversion is shown as an upload error', async ({ page }) => {
  await login(page)
  await openUploadDialog(page)
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
    await expect(player.getByRole('button', { name: 'Add Track', exact: true })).toHaveCount(0)
    await expect(player.getByRole('banner')).toContainText('My Table')
    await expect(player.getByRole('heading', { name: 'My Table', exact: true })).toHaveCount(0)
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
  await deleteLibraryTrack(page, 'Session Music', 1)
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
    await deleteLibraryTrack(page, name, 1)
    await expect(tile).toHaveCount(0)
  }
  await expect(page.locator('video')).toHaveCount(0)
})

test('tables share a library but isolate invitations, playback, removal, and deletion', async ({
  page, browser, context,
}) => {
  test.slow()
  await context.grantPermissions(['clipboard-read', 'clipboard-write'])
  await login(page)
  await page.getByRole('link', { name: 'Library', exact: true }).click()
  await uploadTrack(page, 'Shared Forest')
  await page.getByRole('link', { name: 'Tables', exact: true }).click()
  await createTable(page, 'Friday')
  await createTable(page, 'Sunday')
  await page.getByRole('link', { name: 'Open Friday', exact: true }).click()
  await addExistingTrack(page, 'Shared Forest')
  await page.getByRole('button', { name: 'Copy invite link', exact: true }).click()
  await expect(page.getByRole('button', { name: 'Copied to clipboard', exact: true })).toBeVisible()
  const fridayInvite = await page.evaluate(() => navigator.clipboard.readText())

  const sundayGM = await context.newPage()
  await sundayGM.goto('/tables')
  await sundayGM.getByRole('link', { name: 'Open Sunday', exact: true }).click()
  await addExistingTrack(sundayGM, 'Shared Forest')
  await sundayGM.getByRole('button', { name: 'Copy invite link', exact: true }).click()
  await expect(sundayGM.getByRole('button', { name: 'Copied to clipboard', exact: true })).toBeVisible()
  const sundayInvite = await sundayGM.evaluate(() => navigator.clipboard.readText())
  expect(sundayInvite).not.toBe(fridayInvite)

  const fridayContext = await browser.newContext()
  const sundayContext = await browser.newContext()
  try {
    const fridayPlayer = await fridayContext.newPage()
    const sundayPlayer = await sundayContext.newPage()
    await fridayPlayer.goto(fridayInvite)
    await sundayPlayer.goto(sundayInvite)
    await expect(fridayPlayer.getByRole('banner')).toContainText('Friday')
    await expect(sundayPlayer.getByRole('banner')).toContainText('Sunday')
    await expect(fridayPlayer.getByRole('link', { name: 'Library', exact: true })).toHaveCount(0)
    await fridayPlayer.getByRole('button', { name: 'Connect Audio', exact: true }).click()
    await sundayPlayer.getByRole('button', { name: 'Connect Audio', exact: true }).click()
    await expect(fridayPlayer.getByText('Connected', { exact: true })).toBeVisible()
    await expect(sundayPlayer.getByText('Connected', { exact: true })).toBeVisible()
    const fridayTile = page.locator('.file-tile').filter({ hasText: 'Shared Forest' })
    const sundayTile = sundayGM.locator('.file-tile').filter({ hasText: 'Shared Forest' })
    await fridayTile.click()
    await expect(fridayPlayer.locator('video')).toHaveCount(1)
    await expect.poll(() => fridayPlayer.locator('video').evaluate(
      (el: HTMLVideoElement) => !el.paused && el.currentTime > 0,
    )).toBe(true)
    await expect(sundayPlayer.locator('video')).toHaveCount(0)
    await sundayTile.click()
    await expect(sundayPlayer.locator('video')).toHaveCount(1)
    await expect.poll(() => sundayPlayer.locator('video').evaluate(
      (el: HTMLVideoElement) => !el.paused && el.currentTime > 0,
    )).toBe(true)
    await fridayTile.click()
    await expect.poll(() => fridayPlayer.locator('video').evaluate(
      (el: HTMLVideoElement) => el.paused,
    )).toBe(true)
    await expect.poll(() => sundayPlayer.locator('video').evaluate(
      (el: HTMLVideoElement) => !el.paused,
    )).toBe(true)
    await fridayTile.getByRole('button', { name: 'Edit Shared Forest', exact: true }).click()
    await page.getByRole('button', { name: 'Remove from table', exact: true }).click()
    await expect(fridayTile).toHaveCount(0)
    await expect(fridayPlayer.locator('video')).toHaveCount(0)
    await expect(sundayTile).toBeVisible()
    await expect.poll(() => sundayPlayer.locator('video').evaluate(
      (el: HTMLVideoElement) => !el.paused,
    )).toBe(true)
    await page.getByRole('link', { name: 'Library', exact: true }).click()
    await deleteLibraryTrack(page, 'Shared Forest', 1)
    await expect(sundayTile).toHaveCount(0)
    await expect(sundayPlayer.locator('video')).toHaveCount(0)
    await page.getByRole('link', { name: 'Tables', exact: true }).click()
    await deleteTable(page, 'Friday')
    await expect(fridayPlayer.getByText('This table has been deleted.', { exact: true })).toBeVisible()
    await fridayPlayer.reload()
    await expect(fridayPlayer.getByText(
      'This invite link is invalid or the table has been deleted.', { exact: true },
    )).toBeVisible()
    await deleteTable(page, 'Sunday')
    await expect(sundayPlayer.getByText('This table has been deleted.', { exact: true })).toBeVisible()
  } finally {
    await fridayContext.close()
    await sundayContext.close()
    await sundayGM.close()
  }
})

async function login(page: Page) {
  await page.goto('/login')
  await page.getByLabel('Username').fill('e2e-gm')
  await page.getByLabel('Password', { exact: true }).fill('e2e-password')
  await page.getByRole('button', { name: 'Login', exact: true }).click()
  await expect(page.getByRole('heading', { name: 'Tables', exact: true })).toBeVisible()
  await page.getByRole('link', { name: 'Open My Table', exact: true }).click()
  await expect(page.getByRole('button', { name: 'Add Track', exact: true })).toBeVisible()
}

async function createTable(page: Page, name: string) {
  await page.getByRole('button', { name: 'Create table', exact: true }).click()
  await page.getByLabel('Table name', { exact: true }).fill(name)
  await page.getByRole('button', { name: 'Save table', exact: true }).click()
  await expect(page.getByRole('link', { name: `Open ${name}`, exact: true })).toBeVisible()
}

async function addExistingTrack(page: Page, name: string) {
  await page.getByRole('button', { name: 'Add Track', exact: true }).click()
  await page.getByRole('tab', { name: 'From library', exact: true }).click()
  await page.getByRole('combobox', { name: 'Library track', exact: true }).press('ArrowDown')
  await page.getByRole('option', { name, exact: true }).click()
  await page.getByRole('button', { name: 'Add to table', exact: true }).click()
  await expect(page.locator('.file-tile').filter({ hasText: name })).toBeVisible()
  await expect(page.getByRole('dialog')).toHaveCount(0)
  await expect(page.getByRole('combobox', { name: 'Library track', exact: true })).toHaveCount(0)
}

async function deleteTable(page: Page, name: string) {
  await page.getByRole('button', { name: `Options for ${name}`, exact: true }).click()
  await expect(page).toHaveURL(/\/tables$/)
  await page.getByRole('menuitem', { name: `Delete ${name}`, exact: true }).click()
  await page.getByRole('button', { name: 'Confirm delete table', exact: true }).click()
  await expect(page.getByRole('link', { name: `Open ${name}`, exact: true })).toHaveCount(0)
}

async function deleteLibraryTrack(page: Page, name: string, tableCount: number) {
  await page.getByRole('link', { name: 'Library', exact: true }).click()
  const row = page.getByRole('row').filter({ hasText: name })
  await row.getByRole('button', { name: `Edit ${name}` }).click()
  await page.getByRole('button', { name: 'Delete track', exact: true }).click()
  if (tableCount > 0) {
    const noun = tableCount === 1 ? 'table' : 'tables'
    const confirmation = page.getByRole('dialog', { name: 'Delete track?', exact: true })
    await expect(confirmation).toContainText(`remove it from ${tableCount} ${noun}`)
    await confirmation.getByRole('button', { name: 'Confirm delete track', exact: true }).click()
  }
  await expect(row).toHaveCount(0)
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
  await openUploadDialog(page)
  await page
    .getByLabel('Select a file', { exact: true })
    .setInputFiles({ name: 'sample.mp3', mimeType: 'audio/mpeg', buffer: audio })
  await page.getByLabel('Track Name', { exact: true }).fill(name)
  await page.getByRole('combobox', { name: 'Track Type', exact: true }).press('ArrowDown')
  await page.getByRole('option', { name: 'Music', exact: true }).click()
  await page.getByRole('button', { name: 'Upload', exact: true }).click()
  await expect(page.getByText('File uploaded successfully!', { exact: true })).toBeVisible()
}

async function openUploadDialog(page: Page) {
  const addTrack = page.getByRole('button', { name: 'Add Track', exact: true })
  if (await addTrack.count()) {
    await addTrack.click()
    await page.getByRole('tab', { name: 'Upload new', exact: true }).click()
    return
  }
  await page.getByRole('button', { name: 'Upload Track', exact: true }).click()
  await expect(page.getByRole('tab', { name: 'From library', exact: true })).toHaveCount(0)
}
