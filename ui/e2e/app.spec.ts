import { execFileSync } from 'node:child_process'
import { devices, type Locator, type Page } from '@playwright/test'
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

test('GM selects a required track type and receives a bottom upload toast on mobile', async ({ page }) => {
  await page.setViewportSize({ width: 375, height: 812 })
  await login(page)
  await page.getByRole('link', { name: 'Library', exact: true }).click()
  let uploads = 0
  page.on('request', request => {
    if (request.method() === 'POST' && new URL(request.url()).pathname === '/api/v1/files') uploads += 1
  })
  await uploadTrack(page, 'Mobile Forest')
  expect(uploads).toBe(1)
  await expect(page.getByRole('status').filter({ hasText: 'File uploaded successfully!' })).toBeHidden()
  await openUploadDialog(page)
  await expect(page.getByRole('combobox', { name: 'Track Type', exact: true })).toHaveValue('')
  await expect(page.getByText('Select a track type before uploading.', { exact: true })).toHaveCount(0)
  await expect(page.getByRole('button', { name: 'Upload', exact: true })).toBeDisabled()
  await page.getByRole('button', { name: 'Cancel', exact: true }).click()
  await deleteLibraryTrack(page, 'Mobile Forest', 0)
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
  await page.getByRole('combobox', { name: 'Track Type', exact: true }).press('ArrowDown')
  await page.getByRole('option', { name: 'One-Shot', exact: true }).click()
  await page.getByRole('button', { name: 'Save Changes' }).click()
  await expect(page.getByRole('row').filter({ hasText: 'Rain' })).toBeVisible()
  await page.reload()
  const renamed = page.getByRole('row').filter({ hasText: 'Rain' })
  await expect(renamed).toBeVisible()
  await expect(renamed.getByText('One-Shot', { exact: true })).toBeVisible()
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

test('GM keeps track ordering independent for each table', async ({ page }) => {
  test.slow()
  await login(page)
  await page.getByRole('link', { name: 'Library', exact: true }).click()
  await uploadTrack(page, 'Library only')
  await page.getByRole('link', { name: 'Tables', exact: true }).click()
  await page.getByRole('link', { name: 'Open My Table', exact: true }).click()
  await uploadTrack(page, 'Ordered Forest')
  await uploadTrack(page, 'Ordered Rain')
  await page.getByRole('link', { name: 'Tables', exact: true }).click()
  await createTable(page, 'Ordered table')
  await page.getByRole('link', { name: 'Open Ordered table', exact: true }).click()
  await addExistingTrack(page, 'Ordered Forest')
  await addExistingTrack(page, 'Ordered Rain')
  const headings = page.locator('.file-tile .track-heading > span')
  await page.getByRole('button', { name: 'Drag to reorder Ordered Rain', exact: true }).press('Alt+ArrowLeft')
  await expect(headings).toHaveText(['Ordered Rain', 'Ordered Forest'])
  await page.reload()
  await expect(headings).toHaveText(['Ordered Rain', 'Ordered Forest'])
  await page.getByRole('link', { name: 'Tables', exact: true }).click()
  await page.getByRole('link', { name: 'Open My Table', exact: true }).click()
  await expect(headings).toHaveText(['Ordered Forest', 'Ordered Rain'])
  await deleteLibraryTrack(page, 'Ordered Forest', 2)
  await deleteLibraryTrack(page, 'Ordered Rain', 2)
  await deleteLibraryTrack(page, 'Library only', 0)
  await page.getByRole('link', { name: 'Tables', exact: true }).click()
  await deleteTable(page, 'Ordered table')
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

test.describe('drag-handle reordering', () => {
  test('GM drags handles, keeps the order, and uses inline playback controls', async ({
    page,
  }, testInfo) => {
    await login(page)
    for (const name of ['Forest', 'Rain', 'Tavern']) await uploadTrack(page, name)
    const tiles = page.locator('.track-item:not([data-dnd-dragging]) > .file-tile')
    const forest = tiles.filter({ hasText: 'Forest' })
    const rain = tiles.filter({ hasText: 'Rain' })
    const tavern = tiles.filter({ hasText: 'Tavern' })
    const sidebar = page.getByRole('complementary', { name: 'Currently playing' })
    const playingForest = sidebar.getByRole('group', { name: 'Playing Forest', exact: true })
    const forestVolume = playingForest.getByRole('slider', { name: 'Volume for Forest' })
    const names = () => tiles.locator('.track-heading > span')
    await expect(names()).toHaveText(['Forest', 'Rain', 'Tavern'])
    await expect(page.getByRole('button', { name: /^Move .+ (earlier|later)/ })).toHaveCount(0)
    await expect(page.getByRole('button', { name: /^Drag to reorder/ })).toHaveCount(3)
    await expect(forest.locator('.audio-control-tile')).toHaveCSS(
      'border-top-color',
      'rgb(128, 222, 234)',
    )
    await expect(forest.locator('.v-chip')).toHaveCount(0)
    const inactiveColor = await forest.locator('.audio-control-tile').evaluate(el => getComputedStyle(el).backgroundColor)
    await page.mouse.move(1000, 600)
    await expect(sidebar).toContainText('No tracks playing.')
    const collapsedForest = await forest.boundingBox()
    if (!collapsedForest) throw new Error('The compact card must be visible')
    expect(collapsedForest.height).toBeLessThan(55)
    await forest.hover()
    await expect(forest.getByRole('slider')).toHaveCount(0)
    expect((await forest.boundingBox())?.height).toBe(collapsedForest.height)
    await forest.focus()
    await page.keyboard.press('Tab')
    await expect(forest.getByRole('button', { name: 'Drag to reorder Forest' })).toBeFocused()
    await page.keyboard.press('Tab')
    await expect(forest.getByRole('button', { name: 'Edit Forest' })).toBeFocused()
    await rain.getByRole('button', { name: 'Drag to reorder Rain' }).click()
    await expect(page.locator('[data-dnd-dragging]')).toHaveCount(0)
    for (const video of await page.locator('video').all()) {
      await expect.poll(() => video.evaluate((el: HTMLVideoElement) => el.paused)).toBe(true)
    }
    await forest.locator('.track-heading > span').click()
    await expect(playingForest).toBeVisible()
    await expect(forest.locator('.audio-control-tile')).toHaveClass(/is-active/)
    await expect(forest.locator('.audio-control-tile')).not.toHaveCSS('background-color', inactiveColor)
    await expect(rain.locator('.audio-control-tile')).not.toHaveClass(/is-active/)
    const forestAudio = page.locator('video').first()
    await expect
      .poll(() => forestAudio.evaluate((el: HTMLVideoElement) => !el.paused && el.currentTime > 0))
      .toBe(true)
    const beforeReorder = await forestAudio.evaluate((el: HTMLVideoElement) => el.currentTime)
    const source = await tavern
      .getByRole('button', { name: 'Drag to reorder Tavern' })
      .boundingBox()
    const target = await forest.boundingBox()
    if (!source || !target) throw new Error('Track cards must have visible bounds')
    await page.mouse.move(source.x + source.width / 2, source.y + source.height / 2)
    await page.mouse.down()
    await page.mouse.move(source.x + source.width / 2 - 8, source.y + source.height / 2, {
      steps: 2,
    })
    await expect(page.locator('[data-dnd-dragging]')).toBeVisible()
    await expect(page.locator('[data-dnd-placeholder] .audio-control-tile')).toBeHidden()
    await expect(page.locator('[data-dnd-placeholder] .track-drag-handle')).toBeHidden()
    await expect(page.locator('[data-dnd-placeholder] .file-tile')).toHaveCSS(
      'outline-style',
      'dashed',
    )
    await expect(
      page.locator(
        '.track-item:not([data-dnd-dragging]):not([data-dnd-placeholder]) .track-heading > span',
      ),
    ).toHaveText(['Forest', 'Rain'])
    await page.mouse.move(target.x + target.width / 4, target.y + 20, { steps: 12 })
    await expect(names()).toHaveText(['Tavern', 'Forest', 'Rain'])
    await expectPlaceholderAt(page, target)
    await page.screenshot({ path: testInfo.outputPath('gm-table-dragging.png'), fullPage: true })
    await page.mouse.up()
    await expect(page.locator('[data-dnd-dragging], [data-dnd-placeholder]')).toHaveCount(0)
    await expect(names()).toHaveText(['Tavern', 'Forest', 'Rain'])
    await expect
      .poll(() =>
        forestAudio.evaluate(
          (el: HTMLVideoElement, time) => !el.paused && el.currentTime >= time,
          beforeReorder,
        ),
      )
      .toBe(true)
    const rainHandle = rain.getByRole('button', { name: 'Drag to reorder Rain' })
    await rainHandle.focus()
    await page.keyboard.press('Alt+ArrowLeft')
    await expect(names()).toHaveText(['Tavern', 'Rain', 'Forest'])
    await expect(rainHandle).toBeFocused()
    await expect(page.getByText('Moved Rain to position 2 of 3.', { exact: true })).toHaveCount(1)
    await page.keyboard.press('Alt+ArrowRight')
    await expect(names()).toHaveText(['Tavern', 'Forest', 'Rain'])
    await page.keyboard.press('Alt+ArrowRight')
    await expect(names()).toHaveText(['Tavern', 'Forest', 'Rain'])
    const slider = playingForest.locator('.v-slider-track').first()
    const volume = await slider.boundingBox()
    if (!volume) throw new Error('The inline volume slider must be visible')
    const beforeVolume = await forestAudio.evaluate((el: HTMLVideoElement) => el.currentTime)
    await page.mouse.move(volume.x + volume.width / 2, volume.y + volume.height / 2)
    await page.mouse.down()
    await expect
      .poll(() => forestAudio.evaluate((el: HTMLVideoElement) => el.currentTime))
      .toBeGreaterThan(beforeVolume + 0.3)
    await expect(page.locator('[data-dnd-dragging]')).toHaveCount(0)
    await page.mouse.move(volume.x + volume.width * 0.7, volume.y + volume.height / 2)
    await page.mouse.up()
    await expect
      .poll(async () => Number(await forestVolume.getAttribute('aria-valuenow')))
      .toBeGreaterThan(50)
    await expect
      .poll(async () => Number(await forestVolume.getAttribute('aria-valuenow')))
      .toBeLessThan(80)
    await expect(names()).toHaveText(['Tavern', 'Forest', 'Rain'])
    await forestVolume.press('End')
    await playingForest.getByRole('button', { name: 'Pause Forest', exact: true }).click()
    await expect.poll(() => forestAudio.evaluate((el: HTMLVideoElement) => el.paused)).toBe(true)
    await expect(playingForest).toHaveCount(0)
    await expect(forest.locator('.audio-control-tile')).toHaveCSS('background-color', inactiveColor)
    await forest.getByRole('button', { name: 'Edit Forest' }).click()
    const settings = page.getByRole('dialog')
    const forestPosition = settings.getByRole('slider', { name: 'Playback position for Forest' })
    await expect(settings.getByRole('button', { name: /^(Play|Pause) Forest$/ })).toHaveCount(0)
    await expect(forestPosition).toBeEnabled()
    await settings.locator('.track-position .v-slider-track').click()
    await expectAlignedTrackControls(settings, false)
    await page.screenshot({ path: testInfo.outputPath('track-settings-aligned.png'), fullPage: true })
    const position = await settings.locator('.track-position .v-slider-track').boundingBox()
    if (!position) throw new Error('Track settings must expose the seek slider')
    await expect(forestPosition).toHaveAttribute('aria-valuenow', /^(14|15|16)$/)
    const selectedTime = Number(await forestPosition.getAttribute('aria-valuenow'))
    await expect
      .poll(() => forestAudio.evaluate((el: HTMLVideoElement) => el.currentTime))
      .toBeCloseTo(selectedTime, 0)
    await expect.poll(() => forestAudio.evaluate((el: HTMLVideoElement) => el.paused)).toBe(true)
    await expect(forestPosition).toHaveAttribute('aria-valuetext', `0:${selectedTime} of 0:30`)
    await forestPosition.hover()
    await page.mouse.down()
    await page.mouse.move(position.x + position.width * 0.75, position.y + position.height / 2, {
      steps: 4,
    })
    await page.mouse.up()
    await expect(forestPosition).toHaveAttribute('aria-valuenow', /^(22|23|24)$/)
    const draggedTime = Number(await forestPosition.getAttribute('aria-valuenow'))
    await expect
      .poll(() => forestAudio.evaluate((el: HTMLVideoElement) => el.currentTime))
      .toBeCloseTo(draggedTime, 0)
    await expect.poll(() => forestAudio.evaluate((el: HTMLVideoElement) => el.paused)).toBe(true)
    await forestPosition.press('Home')
    await forestPosition.press('ArrowRight')
    await expect.poll(() => forestAudio.evaluate((el: HTMLVideoElement) => el.currentTime)).toBe(1)
    await expect(settings.locator('.position-time')).toHaveText('0:01 / 0:30')
    await settings.getByRole('slider', { name: 'Volume for Forest' }).press('Home')
    await expect.poll(() => forestAudio.evaluate((el: HTMLVideoElement) => el.volume)).toBe(0)
    await settings.getByRole('slider', { name: 'Volume for Forest' }).press('End')
    await page.getByRole('button', { name: 'Close track controls' }).click()
    await expect(page.locator('[data-dnd-dragging]')).toHaveCount(0)
    await forest.focus()
    await page.keyboard.press('Enter')
    await expect
      .poll(() => forestAudio.evaluate((el: HTMLVideoElement) => !el.paused && el.volume > 0.3))
      .toBe(true)
    await page.keyboard.press('Space')
    await expect.poll(() => forestAudio.evaluate((el: HTMLVideoElement) => el.paused)).toBe(true)
    await page.reload()
    await expect(names()).toHaveText(['Tavern', 'Forest', 'Rain'])
    await uploadTrack(page, 'New Track')
    await expect(names()).toHaveText(['Tavern', 'Forest', 'Rain', 'New Track'])
    await tiles
      .filter({ hasText: 'New Track' })
      .getByRole('button', { name: 'Edit New Track' })
      .click()
    await page.getByRole('button', { name: 'Remove from table', exact: true }).click()
    await expect(names()).toHaveText(['Tavern', 'Forest', 'Rain'])
    await page.mouse.click(1000, 500)
    await expect(page.getByRole('dialog')).toHaveCount(0)
    await expect(page.getByText('File uploaded successfully!', { exact: true })).not.toBeVisible()
    await expect(page.locator('.file-tile .track-playback-controls')).toHaveCount(0)
    await expect(page.locator('video')).toHaveCount(3)
    for (const video of await page.locator('video').all()) {
      await expect.poll(() => video.evaluate((el: HTMLVideoElement) => el.paused)).toBe(true)
    }
    await page.screenshot({ path: testInfo.outputPath('gm-table-collapsed.png'), fullPage: true })
    await forest.locator('.track-heading > span').click()
    await expect(forestVolume).toBeVisible()
    await expect(playingForest.getByRole('slider', { name: 'Playback position for Forest' })).toBeVisible()
    await expectAlignedTrackControls(playingForest)
    await expectCompactPlayingCard(playingForest)
    await expect(sidebar).toHaveCSS('width', '260px')
    await page.screenshot({ path: testInfo.outputPath('gm-table-sidebar.png'), fullPage: true })
    await page.setViewportSize({ width: 390, height: 844 })
    await page.mouse.move(380, 800)
    await expect(sidebar).toHaveCount(0)
    await page.getByRole('button', { name: 'Now playing (1)', exact: true }).click()
    const drawer = page.getByRole('dialog', { name: 'Currently playing', exact: true })
    const mobileForest = drawer.getByRole('group', { name: 'Playing Forest', exact: true })
    const mobileVolume = mobileForest.getByRole('slider', { name: 'Volume for Forest' })
    await expect(mobileVolume).toBeVisible()
    await expect(drawer.locator('.now-playing-drawer')).toBeInViewport({ ratio: 1 })
    await expect(mobileForest.getByRole('button', { name: 'Restart Forest' })).toBeVisible()
    await expectAlignedTrackControls(mobileForest)
    await expect
      .poll(() => page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth))
      .toBe(true)
    await page.screenshot({
      path: testInfo.outputPath('gm-table-mobile-sidebar.png'),
      fullPage: true,
    })
    await mobileVolume.press('Home')
    await expect(mobileVolume).toHaveAttribute('aria-valuenow', '0')
    await mobileForest.getByRole('button', { name: 'Restart Forest' }).click()
    await mobileForest.getByRole('button', { name: 'Pause Forest' }).click()
    await expect(drawer).toContainText('No tracks playing.')
    await page.getByRole('button', { name: 'Close now playing' }).click()
    await expect(drawer).toBeHidden()
    for (const video of await page.locator('video').all()) {
      await expect.poll(() => video.evaluate((el: HTMLVideoElement) => el.paused)).toBe(true)
    }
    for (const name of ['Forest', 'Rain', 'Tavern']) {
      await tiles
        .filter({ hasText: name })
        .getByRole('button', { name: `Edit ${name}` })
        .click()
      await page.getByRole('button', { name: 'Remove from table', exact: true }).click()
    }
    await expect(tiles).toHaveCount(0)
    for (const name of ['Forest', 'Rain', 'Tavern', 'New Track']) {
      await deleteLibraryTrack(page, name, 0)
    }
  })
})

for (const device of ['Pixel 7', 'iPhone 13']) {
  test(`touch dragging reflows the grid, supports cancellation, and preserves scrolling (${device})`, async ({
    browser,
  }, testInfo) => {
    test.slow()
    const context = await browser.newContext({
      ...devices[device],
      viewport: { width: 390, height: 844 },
      isMobile: true,
      hasTouch: true,
      recordVideo: {
        dir: testInfo.outputPath('touch-recording'),
        size: { width: 390, height: 844 },
      },
    })
    const page = await context.newPage()
    const touch = await context.newCDPSession(page)
    const names = ['Track 1', 'Track 2', 'Track 3', 'Track 4', 'Track 5', 'Track 6']
    try {
      await login(page)
      for (const name of names) await uploadTrack(page, name)
      await expect(page.getByText('File uploaded successfully!', { exact: true })).not.toBeVisible()
      const tiles = page.locator('.track-item:not([data-dnd-dragging]) > .file-tile')
      const headings = tiles.locator('.track-heading > span')
      const first = tiles.filter({ hasText: 'Track 1' })
      const drawer = page.getByRole('dialog', { name: 'Currently playing', exact: true })
      await expect(first.getByRole('button', { name: 'Edit Track 1' })).toBeVisible()
      await expect(first.getByRole('slider')).toHaveCount(0)
      await page.setViewportSize({ width: 390, height: 400 })
      await page.screenshot({ path: testInfo.outputPath('gm-touch-collapsed.png'), fullPage: true })
      for (const video of await page.locator('video').all()) {
        await expect.poll(() => video.evaluate((el: HTMLVideoElement) => el.paused)).toBe(true)
      }
      await first.locator('.track-heading > span').tap()
      const firstAudio = page.locator('video').first()
      await expect
        .poll(() => firstAudio.evaluate((el: HTMLVideoElement) => !el.paused && el.volume > 0.3))
        .toBe(true)
      await page.getByRole('button', { name: 'Now playing (1)', exact: true }).tap()
      await expect(drawer.getByRole('group', { name: 'Playing Track 1', exact: true })).toBeVisible()
      await drawer.getByRole('button', { name: 'Pause Track 1', exact: true }).tap()
      await expect.poll(() => firstAudio.evaluate((el: HTMLVideoElement) => el.paused)).toBe(true)
      await page.getByRole('button', { name: 'Close now playing' }).tap()
      await expect(drawer).toBeHidden()
      await page.setViewportSize({ width: 390, height: 844 })
      await tiles.filter({ hasText: 'Track 3' }).getByRole('button', { name: 'Drag to reorder Track 3' })
        .tap({ trial: true })
      const source = await tiles
        .filter({ hasText: 'Track 3' })
        .getByRole('button', { name: 'Drag to reorder Track 3' })
        .boundingBox()
      const target = await tiles.filter({ hasText: 'Track 2' }).boundingBox()
      if (!source || !target) throw new Error('Touch drag cards must have visible bounds')
      const start = { x: source.x + source.width / 2, y: source.y + source.height / 2 }
      await touch.send('Input.dispatchTouchEvent', { type: 'touchStart', touchPoints: [start] })
      await touch.send('Input.dispatchTouchEvent', {
        type: 'touchMove',
        touchPoints: [{ x: start.x + 24, y: start.y }],
      })
      await expect(page.locator('[data-dnd-dragging]')).toBeVisible()
      await touch.send('Input.dispatchTouchEvent', {
        type: 'touchMove',
        touchPoints: [{ x: target.x + target.width / 4, y: target.y + 20 }],
      })
      await expect(headings).toHaveText([
        'Track 1',
        'Track 3',
        'Track 2',
        'Track 4',
        'Track 5',
        'Track 6',
      ])
      await expectPlaceholderAt(page, target)
      await page.screenshot({ path: testInfo.outputPath('gm-touch-dragging.png'), fullPage: true })
      await touch.send('Input.dispatchTouchEvent', { type: 'touchEnd', touchPoints: [] })
      await expect(page.locator('[data-dnd-dragging], [data-dnd-placeholder]')).toHaveCount(0)
      for (const video of await page.locator('video').all()) {
        await expect.poll(() => video.evaluate((el: HTMLVideoElement) => el.paused)).toBe(true)
      }
      await page.reload()
      const reordered = ['Track 1', 'Track 3', 'Track 2', 'Track 4', 'Track 5', 'Track 6']
      await expect(headings).toHaveText(reordered)
      const cancel = await tiles
        .first()
        .getByRole('button', { name: 'Drag to reorder Track 1' })
        .boundingBox()
      const cancelTarget = await tiles.last().boundingBox()
      if (!cancel || !cancelTarget) throw new Error('Cards must be available to cancel dragging')
      const cancelPoint = { x: cancel.x + cancel.width / 2, y: cancel.y + cancel.height / 2 }
      await touch.send('Input.dispatchTouchEvent', {
        type: 'touchStart',
        touchPoints: [cancelPoint],
      })
      await touch.send('Input.dispatchTouchEvent', {
        type: 'touchMove',
        touchPoints: [{ x: cancelPoint.x + 24, y: cancelPoint.y }],
      })
      await expect(page.locator('[data-dnd-dragging]')).toBeVisible()
      await touch.send('Input.dispatchTouchEvent', {
        type: 'touchMove',
        touchPoints: [{ x: cancelTarget.x + cancelTarget.width * 0.75, y: cancelTarget.y + 20 }],
      })
      await expect(headings).toHaveText([
        'Track 3',
        'Track 2',
        'Track 4',
        'Track 5',
        'Track 6',
        'Track 1',
      ])
      await expectPlaceholderAt(page, cancelTarget)
      await touch.send('Input.dispatchTouchEvent', { type: 'touchCancel', touchPoints: [] })
      await expect(page.locator('[data-dnd-dragging], [data-dnd-placeholder]')).toHaveCount(0)
      await expect(headings).toHaveText(reordered)
      await tiles.first().getByRole('button', { name: 'Edit Track 1' }).tap()
      await expect(page.getByRole('dialog')).toBeVisible()
      await page.getByRole('dialog').locator('.track-position .v-slider-track').tap()
      await expect
        .poll(() =>
          page
            .locator('video')
            .first()
            .evaluate((el: HTMLVideoElement) => el.currentTime),
        )
        .toBeGreaterThanOrEqual(14)
      await expect
        .poll(() =>
          page
            .locator('video')
            .first()
            .evaluate((el: HTMLVideoElement) => el.paused),
        )
        .toBe(true)
      await expect(page.locator('[data-dnd-dragging]')).toHaveCount(0)
      await page.getByRole('button', { name: 'Close track controls' }).tap()
      await expect(page.getByRole('dialog')).toBeHidden()
      await page.setViewportSize({ width: 390, height: 180 })
      await page.evaluate(() => window.scrollTo(0, 0))
      const swipe = await tiles.first().locator('.track-heading').boundingBox()
      if (!swipe) throw new Error('A card must be available for touch scrolling')
      const swipePoint = { x: swipe.x + swipe.width / 2, y: swipe.y + swipe.height / 2 }
      await touch.send('Input.dispatchTouchEvent', {
        type: 'touchStart',
        touchPoints: [swipePoint],
      })
      await touch.send('Input.dispatchTouchEvent', {
        type: 'touchMove',
        touchPoints: [{ x: swipePoint.x, y: swipePoint.y - 80 }],
      })
      await touch.send('Input.dispatchTouchEvent', { type: 'touchEnd', touchPoints: [] })
      await expect.poll(() => page.evaluate(() => window.scrollY)).toBeGreaterThan(0)
      await expect(page.locator('[data-dnd-dragging]')).toHaveCount(0)
      await expect(headings).toHaveText(reordered)
      for (const name of names) {
        const tile = tiles.filter({ hasText: name })
        await tile.getByRole('button', { name: `Edit ${name}` }).tap()
        await expect(page.getByRole('dialog')).toBeVisible()
        await page.getByRole('button', { name: 'Remove from table', exact: true }).tap()
      }
      await expect(tiles).toHaveCount(0)
      await page.setViewportSize({ width: 390, height: 844 })
      for (const name of names) await deleteLibraryTrack(page, name, 0)
    } finally {
      await context.close()
    }
  })
}

test('invited player receives live playback, late-join state, and pause', async ({
  page,
  browser,
}) => {
  await login(page)
  await uploadTrack(page, 'Session Music')
  const tile = page.locator('.file-tile').filter({ hasText: 'Session Music' })
  await tile.click()
  const sidebar = page.getByRole('complementary', { name: 'Currently playing' })
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
    const trackVolume = sidebar.getByRole('slider', { name: 'Volume for Session Music' })
    await trackVolume.press('Home')
    await expect.poll(() => gmAudio.evaluate((el: HTMLVideoElement) => el.volume)).toBe(0)
    await expect.poll(() => playerAudio.evaluate((el: HTMLVideoElement) => el.volume)).toBe(0)
    await expect.poll(() => gmAudio.evaluate((el: HTMLVideoElement) => el.paused)).toBe(false)
    await trackVolume.press('End')
    await expect
      .poll(() => playerAudio.evaluate((el: HTMLVideoElement) => el.volume))
      .toBeGreaterThan(0)
    await expect
      .poll(() => gmAudio.evaluate((el: HTMLVideoElement) => el.currentTime))
      .toBeGreaterThan(4)
    await expect
      .poll(() => playerAudio.evaluate((el: HTMLVideoElement) => el.currentTime))
      .toBeGreaterThan(4)
    const seek = await sidebar.locator('.track-position .v-slider-track').boundingBox()
    if (!seek) throw new Error('The playing track must expose a seek slider')
    await page.mouse.click(seek.x + seek.width / 2, seek.y + seek.height / 2)
    for (const media of [gmAudio, playerAudio]) {
      await expect
        .poll(() => media.evaluate((el: HTMLVideoElement) => el.currentTime))
        .toBeGreaterThanOrEqual(14)
      await expect
        .poll(() => media.evaluate((el: HTMLVideoElement) => el.currentTime))
        .toBeLessThan(18)
      await expect.poll(() => media.evaluate((el: HTMLVideoElement) => el.paused)).toBe(false)
    }
    await sidebar.getByRole('button', { name: 'Restart Session Music' }).click()
    await expect
      .poll(() => gmAudio.evaluate((el: HTMLVideoElement) => el.currentTime))
      .toBeLessThan(2)
    await expect
      .poll(() => playerAudio.evaluate((el: HTMLVideoElement) => el.currentTime))
      .toBeLessThan(2)
    await expect.poll(() => playerAudio.evaluate((el: HTMLVideoElement) => el.paused)).toBe(false)
    await sidebar.getByRole('button', { name: 'Pause Session Music', exact: true }).click()
    await expect.poll(() => playerAudio.evaluate((el: HTMLVideoElement) => el.paused)).toBe(true)
    await tile.getByRole('button', { name: 'Edit Session Music', exact: true }).click()
    await page.getByRole('dialog').getByRole('button', { name: 'Restart Session Music' }).click()
    await expect.poll(() => gmAudio.evaluate((el: HTMLVideoElement) => el.currentTime)).toBe(0)
    await expect.poll(() => gmAudio.evaluate((el: HTMLVideoElement) => el.paused)).toBe(true)
    await page.getByRole('button', { name: 'Close track controls' }).click()
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

test('GM manages simultaneous tracks in the currently-playing sidebar', async ({ page }, testInfo) => {
  test.slow()
  await login(page)
  await uploadTrack(page, 'Forest Ambiance', 'Ambiance')
  await uploadTrack(page, 'Bell', 'One-Shot')
  const forest = page.getByRole('group', { name: 'Forest Ambiance', exact: true })
  const bell = page.getByRole('group', { name: 'Bell', exact: true })
  const sidebar = page.getByRole('complementary', { name: 'Currently playing' })
  await forest.click()
  await bell.click()
  await expect(sidebar.getByRole('group')).toHaveCount(2)
  for (const card of await sidebar.getByRole('group').all()) await expectCompactPlayingCard(card)
  for (const media of await page.locator('video').all()) {
    await expect.poll(() => media.evaluate(
      (el: HTMLVideoElement) => !el.paused && el.currentTime > 0,
    )).toBe(true)
  }
  await sidebar.getByRole('slider', { name: 'Volume for Forest Ambiance' }).press('Home')
  await expect.poll(() => page.locator('video').first().evaluate(
    (el: HTMLVideoElement) => el.volume,
  )).toBe(0)
  await expect.poll(() => page.locator('video').last().evaluate(
    (el: HTMLVideoElement) => !el.paused && el.volume > 0,
  )).toBe(true)
  await sidebar.getByRole('slider', { name: 'Volume for Forest Ambiance' }).press('End')
  for (const width of [839, 840, 885, 959, 960, 1280, 1920, 840, 839]) {
    await page.setViewportSize({ width, height: 720 })
    if (width >= 840) {
      await expectEdgeToEdgeTable(page, sidebar)
      await expect(sidebar).toHaveCSS('width', '260px')
      await expect(page.getByRole('button', { name: /^Now playing \(/ })).toHaveCount(0)
    } else {
      await expect(sidebar).toHaveCount(0)
      await expect(page.getByRole('button', { name: 'Now playing (2)', exact: true })).toBeVisible()
      await expect(page.locator('.table-track-grid')).toHaveCSS('width', `${width}px`)
    }
    await expectTrackGridSpacing(page)
  }
  await page.setViewportSize({ width: 1280, height: 720 })
  await page.screenshot({ path: testInfo.outputPath('simultaneous-sidebar.png'), fullPage: true })
  await bell.getByRole('button', { name: 'Edit Bell', exact: true }).click()
  await page.getByRole('dialog').getByRole('slider', { name: 'Playback position for Bell' }).press('End')
  await page.getByRole('button', { name: 'Close track controls' }).click()
  await expect(sidebar.getByRole('group')).toHaveCount(1)
  await expect(sidebar.getByRole('group')).toHaveAccessibleName('Playing Forest Ambiance')
  await forest.getByRole('button', { name: 'Edit Forest Ambiance', exact: true }).click()
  await page.getByRole('button', { name: 'Remove from table', exact: true }).click()
  await expect(sidebar.getByRole('group')).toHaveCount(0)
  await expect(sidebar).toContainText('No tracks playing.')
  await expect(page.locator('video')).toHaveCount(1)
  await deleteLibraryTrack(page, 'Forest Ambiance', 0)
  await deleteLibraryTrack(page, 'Bell', 1)
})

test('GM opens compact now-playing controls from a mobile bottom drawer', async ({ page }, testInfo) => {
  test.slow()
  await page.setViewportSize({ width: 390, height: 667 })
  await login(page)
  const forestName = 'Forest at the edge of the ancient enchanted woodland'
  const names = [forestName, 'Rain', 'Wind', 'River', 'Tavern']
  for (const name of names) await uploadTrack(page, name, 'Ambiance')
  const drawer = page.getByRole('dialog', { name: 'Currently playing', exact: true })
  const toggle = page.getByRole('button', { name: /^Now playing \(\d+\)$/ })
  await expect(toggle).toHaveAccessibleName('Now playing (0)')
  await expect(toggle).toHaveAttribute('aria-expanded', 'false')
  await toggle.click()
  await expect(drawer).toContainText('No tracks playing.')
  await page.getByRole('button', { name: 'Close now playing' }).click()
  for (const name of names) {
    await page.getByRole('group', { name, exact: true }).locator('.track-heading > span').click()
  }
  await expect(toggle).toHaveAccessibleName('Now playing (5)')
  await expect(drawer).toBeHidden()
  await expect(page.getByRole('complementary', { name: 'Currently playing' })).toHaveCount(0)
  await expect.poll(async () => {
    const grid = await page.locator('.table-track-grid').boundingBox()
    const card = await page.locator('.file-tile').first().boundingBox()
    return grid?.x === 0 && grid.width === 390 && card?.x === 10
  }).toBe(true)
  await expectTrackGridSpacing(page, true)
  await expect.poll(async () => {
    const lastCard = await page.locator('.file-tile').nth(1).boundingBox()
    return lastCard ? 390 - lastCard.x - lastCard.width : Infinity
  }).toBeCloseTo(10, 1)
  const forestTile = page.getByRole('group', { name: forestName, exact: true })
  await expect(forestTile.locator('.track-heading > span')).toHaveCSS('white-space', 'nowrap')
  await expect(forestTile.locator('.track-heading > span')).toHaveCSS('text-overflow', 'ellipsis')
  await expect(forestTile.locator('.track-heading > span')).toHaveAttribute('title', forestName)
  await expect.poll(() => forestTile.locator('.track-heading > span').evaluate(
    el => el.scrollWidth > el.clientWidth,
  )).toBe(true)
  for (const tile of await page.locator('.file-tile').all()) {
    await expect.poll(async () => (await tile.boundingBox())?.height).toBe(44)
  }
  await expect.poll(async () => (await page.locator('.track-list').boundingBox())?.y ?? Infinity)
    .toBeLessThan(120)
  await expect(toggle).toHaveCSS('height', '44px')
  await expect.poll(async () => {
    const bounds = await toggle.boundingBox()
    return bounds ? Math.abs(bounds.y + bounds.height - 667) : Infinity
  }).toBeLessThan(1)
  await page.screenshot({ path: testInfo.outputPath('mobile-now-playing-collapsed.png'), fullPage: true })
  await toggle.click()
  await expect(drawer.getByRole('group')).toHaveCount(5)
  const forest = drawer.getByRole('group', { name: `Playing ${forestName}`, exact: true })
  await expectCompactPlayingCard(forest)
  await expect(forest.locator('.playing-track-name')).toHaveAttribute('title', forestName)
  await expect(drawer.locator('.now-playing-drawer')).toBeInViewport({ ratio: 1 })
  await expect.poll(async () => (await drawer.locator('.now-playing-drawer').boundingBox())?.height ?? Infinity)
    .toBeLessThanOrEqual(667 * 0.7 + 1)
  await expect.poll(() => drawer.locator('.now-playing-drawer-tracks').evaluate(
    el => el.scrollHeight > el.clientHeight,
  )).toBe(true)
  await forest.getByRole('slider', { name: `Volume for ${forestName}` }).press('Home')
  await expect.poll(() => page.locator('video').first().evaluate(
    (el: HTMLVideoElement) => el.volume === 0 && !el.paused,
  )).toBe(true)
  await forest.getByRole('slider', { name: `Playback position for ${forestName}` }).press('Home')
  await forest.getByRole('slider', { name: `Playback position for ${forestName}` }).press('ArrowRight')
  await expect.poll(() => page.locator('video').first().evaluate(
    (el: HTMLVideoElement) => el.currentTime < 4 && !el.paused,
  )).toBe(true)
  await page.screenshot({ path: testInfo.outputPath('mobile-now-playing-open.png'), fullPage: true })
  await drawer.getByRole('button', { name: 'Pause Tavern', exact: true }).click()
  await expect(drawer.getByRole('group')).toHaveCount(4)
  await page.getByRole('button', { name: 'Close now playing' }).click()
  await expect(drawer).toBeHidden()
  await expect(toggle).toHaveAccessibleName('Now playing (4)')
  await toggle.click()
  await expect(drawer.locator('.v-overlay__content')).toBeFocused()
  await page.keyboard.press('Escape')
  await expect(drawer).toBeHidden()
  await expect(toggle).toBeFocused()
  await toggle.click()
  await page.mouse.click(10, 80)
  await expect(drawer).toBeHidden()
  await expect.poll(() => page.locator('video').first().evaluate(
    (el: HTMLVideoElement) => !el.paused,
  )).toBe(true)
  await toggle.click()
  await page.setViewportSize({ width: 1280, height: 720 })
  const sidebar = page.getByRole('complementary', { name: 'Currently playing' })
  await expect(sidebar.getByRole('group')).toHaveCount(4)
  await expect(drawer).toHaveCount(0)
  await expect(toggle).toHaveCount(0)
  await page.setViewportSize({ width: 390, height: 667 })
  await expect(toggle).toHaveAttribute('aria-expanded', 'false')
  await expect.poll(() => page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)).toBe(true)
  for (const name of names) await deleteLibraryTrack(page, name, 1)
  await expect(toggle).toHaveCount(0)
})

test('starting another Music track stops the previous one', async ({ page }) => {
  await login(page)
  await uploadTrack(page, 'First Music')
  await uploadTrack(page, 'Second Music')
  const first = page.locator('.file-tile').filter({ hasText: 'First Music' })
  const second = page.locator('.file-tile').filter({ hasText: 'Second Music' })
  await first.locator('.track-heading').click()
  const sidebar = page.getByRole('complementary', { name: 'Currently playing' })
  await expect(sidebar.getByRole('group')).toHaveAccessibleName('Playing First Music')
  await expect
    .poll(() =>
      page
        .locator('video')
        .first()
        .evaluate((el: HTMLVideoElement) => !el.paused && el.currentTime > 0),
    )
    .toBe(true)
  await second.locator('.track-heading').click()
  await expect(sidebar.getByRole('group')).toHaveAccessibleName('Playing Second Music')
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

async function expectEdgeToEdgeTable(page: Page, sidebar: Locator) {
  await expect.poll(async () => {
    const panel = await sidebar.boundingBox()
    const grid = await page.locator('.table-track-grid').boundingBox()
    const card = await page.locator('.file-tile').first().boundingBox()
    const banner = await page.getByRole('banner').boundingBox()
    const viewport = page.viewportSize()
    if (!panel || !grid || !card || !banner || !viewport) return false
    return grid.x === 0 && card.x === 10 && card.y === grid.y + 10
      && panel.x === grid.width
      && panel.x + panel.width === viewport.width
      && panel.y === banner.y + banner.height
      && panel.y + panel.height === viewport.height
  }).toBe(true)
  await expect(sidebar).toHaveCSS('border-left-width', '1px')
  await expect(sidebar).toHaveCSS('overflow-y', 'auto')
  const background = await page.locator('.table-track-grid').evaluate(el => getComputedStyle(el).backgroundColor)
  await expect(sidebar).not.toHaveCSS('background-color', background)
  await expect.poll(() => page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)).toBe(true)
}

async function expectTrackGridSpacing(page: Page, multipleRows = false) {
  await expect.poll(async () => {
    const boxes = await page.locator('.track-list .file-tile').evaluateAll(tiles =>
      tiles.map(tile => {
        const { x, y, width, height } = tile.getBoundingClientRect()
        return { x, y, width, height }
      }),
    )
    const [first, second] = boxes
    if (!first || !second) return false
    const nextRow = boxes.find(box => box.y > first.y && Math.abs(box.x - first.x) < 0.1)
    return Math.abs(second.x - first.x - first.width - 3) < 0.1
      && (!multipleRows || !!nextRow && Math.abs(nextRow.y - first.y - first.height - 3) < 0.1)
  }).toBe(true)
}

async function expectCompactPlayingCard(card: Locator) {
  await expect.poll(async () => (await card.boundingBox())?.height ?? Infinity).toBeLessThanOrEqual(112)
  await expect(card.locator('.playing-track-name')).toHaveCSS('white-space', 'nowrap')
}

async function expectAlignedTrackControls(tile: Locator, showPlay = true) {
  const [volume, position, volumeIcon, playButton] = await Promise.all(
    [
      '.track-volume .v-slider-track',
      '.track-position .v-slider-track',
      '.track-volume .v-input__prepend .v-icon',
      showPlay ? '.v-btn[aria-label^="Play "], .v-btn[aria-label^="Pause "]' : '.v-btn[aria-label^="Restart "]',
    ].map(selector => tile.locator(selector).boundingBox()),
  )
  if (!volume || !position || !volumeIcon || !playButton)
    throw new Error('Both slider rows must have visible bounds')
  expect(Math.abs(volume.x - position.x)).toBeLessThan(1)
  expect(Math.abs(volume.width - position.width)).toBeLessThan(1)
  expect(volume.height).toBe(position.height)
  expect(
    Math.abs(volumeIcon.x + volumeIcon.width / 2 - playButton.x - playButton.width / 2),
  ).toBeLessThan(1)
  expect(
    Math.abs(position.y + position.height / 2 - playButton.y - playButton.height / 2),
  ).toBeLessThan(1)
  for (const selector of ['.track-volume', '.track-position']) {
    await expect(tile.locator(`${selector} .v-slider-thumb__surface`)).toHaveCSS('width', '8px')
    await expect(tile.locator(`${selector} .v-slider-track__background`)).toHaveCSS('height', '2px')
    await expect(tile.locator(`${selector} .v-slider-track__fill`)).toHaveCSS('height', '2px')
  }
}

async function expectPlaceholderAt(page: Page, target: { x: number; y: number }) {
  await expect
    .poll(async () => {
      const placeholder = await page.locator('[data-dnd-placeholder] .file-tile').boundingBox()
      return placeholder ? Math.hypot(placeholder.x - target.x, placeholder.y - target.y) : Infinity
    })
    .toBeLessThan(2)
}
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

async function uploadTrack(page: Page, name: string, typeName = 'Music') {
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
  const upload = page.getByRole('button', { name: 'Upload', exact: true })
  await expect(upload).toBeDisabled()
  await page
    .getByLabel('Select a file', { exact: true })
    .setInputFiles({ name: 'sample.mp3', mimeType: 'audio/mpeg', buffer: audio })
  await page.getByLabel('Track Name', { exact: true }).fill(name)
  await expect(upload).toBeDisabled()
  await expect(page.getByText('Select a track type before uploading.', { exact: true })).toHaveCount(0)
  await page.getByRole('combobox', { name: 'Track Type', exact: true }).press('ArrowDown')
  await page.getByRole('option', { name: typeName, exact: true }).click()
  await expect(page.getByText('Select a track type before uploading.', { exact: true })).toHaveCount(0)
  await expect(upload).toBeEnabled()
  await upload.click()
  const toast = page.getByRole('status').filter({ hasText: 'File uploaded successfully!' })
  await expect(toast).toBeVisible()
  await expect.poll(async () => {
    const bounds = await toast.boundingBox()
    const viewport = page.viewportSize()
    if (!bounds || !viewport) return false
    return bounds.y > viewport.height * 0.75
      && bounds.y + bounds.height <= viewport.height
      && bounds.x >= 0
      && bounds.x + bounds.width <= viewport.width
      && bounds.height <= 80
      && bounds.width <= 400
  }).toBe(true)
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
