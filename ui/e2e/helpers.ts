import { execFileSync } from 'node:child_process'
import { type Locator, type Page } from '@playwright/test'
import { expect } from './fixtures'

const audioFixtures = new Map<string, Buffer>()

export async function expectEdgeToEdgeTable(page: Page, sidebar: Locator) {
  await expect.poll(async () => {
    const panel = await sidebar.boundingBox()
    const grid = await page.locator('.table-track-grid').boundingBox()
    const card = await page.locator('.file-tile').first().boundingBox()
    const banner = await page.getByRole('banner').boundingBox()
    const viewport = page.viewportSize()
    if (!panel || !grid || !card || !banner || !viewport) return false
    return grid.x === 0 && card.x === grid.x + 10 && card.y === grid.y + 10
      && panel.x === grid.x + grid.width
      && panel.x + panel.width === viewport.width
      && panel.y === banner.y + banner.height
      && panel.y + panel.height === viewport.height
  }).toBe(true)
  await expect(sidebar).toHaveCSS('border-left-width', '1px')
  await expect(sidebar.locator('.playing-sidebar-tracks')).toHaveCSS('overflow-y', 'auto')
  const background = await page.locator('.table-track-grid').evaluate(el => getComputedStyle(el).backgroundColor)
  await expect(sidebar).not.toHaveCSS('background-color', background)
  await expect.poll(() => page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)).toBe(true)
}

export async function expectTrackGridSpacing(page: Page, multipleRows = false) {
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

export async function expectCompactPlayingCard(card: Locator) {
  await expect.poll(async () => (await card.boundingBox())?.height ?? Infinity).toBeLessThanOrEqual(112)
  await expect(card.locator('.playing-track-name')).toHaveCSS('white-space', 'nowrap')
}

export async function expectAlignedTrackControls(tile: Locator, showPlay = true) {
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
    await expect(tile.locator(`${selector} .v-slider-thumb__surface`)).toHaveCSS('width', '18px')
    await expect(tile.locator(`${selector} .v-slider-thumb__surface`)).toHaveCSS('height', '18px')
    await expect(tile.locator(`${selector} .v-slider-track__background`)).toHaveCSS('height', '6px')
    await expect(tile.locator(`${selector} .v-slider-track__fill`)).toHaveCSS('height', '6px')
  }
}

export async function expectTimelineLayout(dialog: Locator) {
  const axis = await dialog.locator('.timeline-axis').boundingBox()
  const cursor = await dialog.locator('.timeline-cursor').boundingBox()
  const start = await dialog.locator('.timeline-handle--start').boundingBox()
  const end = await dialog.locator('.timeline-handle--end').boundingBox()
  const loop = await dialog.locator('.timeline-handle--loop').boundingBox()
  const time = await dialog.locator('.position-time').boundingBox()
  if (!axis || !cursor || !start || !end || !loop || !time) throw new Error('All timeline controls must be visible')
  const duration = Number(await dialog.locator('.timeline-handle--end').getAttribute('aria-valuemax'))
  const usable = {
    x: axis.x + Number(await dialog.locator('.timeline-handle--start').getAttribute('aria-valuenow')) / duration * axis.width,
    width: 0,
  }
  usable.width = axis.x + Number(await dialog.locator('.timeline-handle--end').getAttribute('aria-valuenow')) / duration
    * axis.width - usable.x
  expect(start.y + start.height).toBeLessThan(axis.y)
  expect(end.y + end.height).toBeLessThan(axis.y)
  expect(loop.y).toBeGreaterThan(axis.y + axis.height)
  expect(Math.abs(cursor.y + cursor.height / 2 - axis.y - axis.height / 2)).toBeLessThan(1)
  expect(Math.abs(start.x + start.width - usable.x)).toBeLessThan(1)
  expect(Math.abs(end.x - usable.x - usable.width)).toBeLessThan(1)
  expect(start.x + start.width).toBeLessThan(end.x)
  expect(time.x).toBeGreaterThan(axis.x + axis.width)
  expect(Math.abs(time.y + time.height / 2 - axis.y - axis.height / 2)).toBeLessThan(1)
  await expect(dialog.getByRole('button', { name: /^Restart / })).toHaveCount(0)
  await expect(dialog.getByText('Playback region', { exact: true })).toHaveCount(0)
  const volume = dialog.locator('.timeline-volume .v-slider-track')
  if (await volume.count()) {
    const bounds = await volume.boundingBox()
    if (!bounds) throw new Error('The volume slider must be visible')
    await expect(dialog.locator('.timeline-volume .v-input__prepend .v-icon')).toBeVisible()
    expect(Math.abs(bounds.x - axis.x)).toBeLessThan(1)
    expect(Math.abs(bounds.width - axis.width)).toBeLessThan(1)
    await expect(dialog.locator('.timeline-volume .v-slider-track__background')).toHaveCSS('height', '6px')
    await expect(dialog.locator('.timeline-volume .v-slider-track__fill')).toHaveCSS('height', '6px')
    await expect(dialog.locator('.timeline-volume .v-slider-thumb__surface')).toHaveCSS('width', `${cursor.width}px`)
    await expect(dialog.locator('.timeline-volume .v-slider-thumb__surface')).toHaveCSS('height', `${cursor.height}px`)
    await expect(dialog.locator('.timeline-volume .v-slider-thumb__ripple')).toHaveCSS('width', `${cursor.width}px`)
    await expect(dialog.locator('.timeline-volume .v-slider-thumb__ripple')).toHaveCSS('height', `${cursor.height}px`)
  }
  for (const layer of ['track', 'usable', 'progress']) {
    await expect(dialog.getByTestId(`waveform-${layer}`)).toBeVisible()
  }
  await expect(dialog.getByTestId('waveform-usable')).not.toHaveCSS(
    'fill', await dialog.getByTestId('waveform-track').evaluate(el => getComputedStyle(el).fill),
  )
}

export async function expectPlaceholderAt(page: Page, target: { x: number; y: number }) {
  await expect
    .poll(async () => {
      const placeholder = await page.locator('[data-dnd-placeholder] .file-tile').boundingBox()
      return placeholder ? Math.hypot(placeholder.x - target.x, placeholder.y - target.y) : Infinity
    })
    .toBeLessThan(2)
}

export async function dragTimelinePoint(page: Page, name: string, time: number, useTouch = false) {
  const dialog = page.getByRole('dialog')
  const handle = dialog.getByRole('slider', { name, exact: true })
  await expect(handle).toBeEnabled()
  if (useTouch) await handle.tap()
  else await handle.hover()
  const end = dialog.locator('.timeline-handle--end')
  const duration = Number(await end.getAttribute('aria-valuemax'))
  const current = Number(await handle.getAttribute('aria-valuenow'))
  const axis = await dialog.locator('.timeline-axis').boundingBox()
  const source = await handle.boundingBox()
  if (!axis || !source || duration <= 0) throw new Error('The audio timeline must be ready')
  const from = source.x + source.width / 2
  const to = from + (time - current) / duration * axis.width
  const y = source.y + source.height / 2
  if (useTouch) {
    const touch = await page.context().newCDPSession(page)
    try {
      await touch.send('Input.dispatchTouchEvent', { type: 'touchStart', touchPoints: [{ x: from, y }] })
      for (let step = 1; step <= 12; step++) {
        await touch.send('Input.dispatchTouchEvent', {
          type: 'touchMove', touchPoints: [{ x: from + (to - from) * step / 12, y }],
        })
      }
      await touch.send('Input.dispatchTouchEvent', { type: 'touchEnd', touchPoints: [] })
    } finally {
      await touch.detach()
    }
  } else {
    await page.mouse.move(from, y)
    await page.mouse.down()
    await page.mouse.move(to, y, { steps: 12 })
    await page.mouse.up()
  }
  await expect(handle).toHaveAttribute('aria-valuenow', time.toString())
}

export async function login(page: Page) {
  await page.goto('/login')
  await page.getByLabel('Username').fill('e2e-gm')
  await page.getByLabel('Password', { exact: true }).fill('e2e-password')
  await page.getByRole('button', { name: 'Login', exact: true }).click()
  await expect(page.getByRole('heading', { name: 'Tables', exact: true })).toBeVisible()
  await page.getByRole('link', { name: 'Open My Table', exact: true }).click()
  await expect(page.getByRole('button', { name: 'Table options', exact: true })).toBeVisible()
}

export async function createTable(page: Page, name: string) {
  await page.getByRole('button', { name: 'Create table', exact: true }).click()
  await page.getByLabel('Table name', { exact: true }).fill(name)
  await page.getByRole('button', { name: 'Save table', exact: true }).click()
  await expect(page.getByRole('link', { name: `Open ${name}`, exact: true })).toBeVisible()
}

export async function addExistingTrack(page: Page, name: string) {
  await page.getByRole('button', { name: 'Add Track', exact: true }).click()
  await page.getByRole('tab', { name: 'From library', exact: true }).click()
  await page.getByRole('combobox', { name: 'Library track', exact: true }).press('ArrowDown')
  await page.getByRole('option', { name, exact: true }).click()
  await page.getByRole('button', { name: 'Add to table', exact: true }).click()
  await expect(page.locator('.file-tile').filter({ hasText: name })).toBeVisible()
  await expect(page.getByRole('dialog')).toHaveCount(0)
  await expect(page.getByRole('combobox', { name: 'Library track', exact: true })).toHaveCount(0)
}

export async function deleteTable(page: Page, name: string) {
  await page.getByRole('button', { name: `Options for ${name}`, exact: true }).click()
  await expect(page).toHaveURL(/\/tables$/)
  await page.getByRole('menuitem', { name: `Delete ${name}`, exact: true }).click()
  await page.getByRole('button', { name: 'Confirm delete table', exact: true }).click()
  await expect(page.getByRole('link', { name: `Open ${name}`, exact: true })).toHaveCount(0)
}

export async function deleteLibraryTrack(page: Page, name: string, tableCount: number) {
  await navigate(page, 'Library')
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

export async function uploadTrack(page: Page, name: string, typeName = 'Music', frequencyOrDuration = 30, duration?: number) {
  const frequency = duration === undefined ? 440 : frequencyOrDuration
  const trackDuration = duration ?? frequencyOrDuration
  const audio = audioFixture(frequency, trackDuration)
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
  const toast = page.getByRole('status').filter({ hasText: 'Track added' })
  await expect(toast).toBeVisible()
  const item = new URL(page.url()).pathname === '/library'
    ? page.getByRole('row').filter({ hasText: name })
    : page.locator('.file-tile').filter({ hasText: name })
  await expect(item).toHaveAttribute('aria-busy', 'false')
}

export async function dropAudioPaths(page: Page, target: Locator, files: string[], hint: string) {
  await expect(target).toBeVisible()
  const bounds = await target.boundingBox()
  if (!bounds) throw new Error('Drop target is not visible')
  const x = Math.round(bounds.x + bounds.width / 2)
  const y = Math.round(bounds.y + bounds.height / 2)
  await expect.poll(() => target.evaluate((el, point) =>
    el.contains(document.elementFromPoint(point.x, point.y)), { x, y })).toBe(true)
  const data = { items: [], files, dragOperationsMask: 1 }
  const session = await page.context().newCDPSession(page)
  try {
    await page.mouse.move(x, y)
    await session.send('Input.dispatchDragEvent', { type: 'dragEnter', x, y, data })
    await session.send('Input.dispatchDragEvent', { type: 'dragOver', x, y, data })
    await expect(page.getByText(hint, { exact: true })).toBeVisible()
    await session.send('Input.dispatchDragEvent', { type: 'drop', x, y, data })
    await expect(page.getByText(hint, { exact: true })).toBeHidden()
  } finally {
    await session.detach()
  }
}

export function audioFixture(frequency: number, duration: number) {
  const key = `${frequency}:${duration}`
  let audio = audioFixtures.get(key)
  if (!audio) {
    audio = execFileSync('ffmpeg', [
      '-v', 'error', '-f', 'lavfi', '-i', `sine=frequency=${frequency}:duration=${duration}`,
      '-f', 'mp3', 'pipe:1',
    ])
    audioFixtures.set(key, audio)
  }
  return audio
}

export async function openUploadDialog(page: Page) {
  if (new URL(page.url()).pathname === '/library') {
    await page.getByRole('button', { name: 'Upload Track', exact: true }).click()
    await expect(page.getByRole('tab', { name: 'From library', exact: true })).toHaveCount(0)
    return
  }
  await expect(page.locator('.table-track-grid')).toBeVisible()
  const addTrack = page.getByRole('button', { name: 'Add Track', exact: true })
  if (await addTrack.isVisible()) {
    await addTrack.click()
    await page.getByRole('tab', { name: 'Upload new', exact: true }).click()
    return
  }
  await page.getByRole('button', { name: 'Table options', exact: true }).click()
  await page.getByRole('menuitem', { name: 'Add Track', exact: true }).click()
  await page.getByRole('tab', { name: 'Upload new', exact: true }).click()
}

export async function navigate(page: Page, destination: 'Library' | 'Tables' | 'Settings') {
  const toggle = page.getByRole('button', { name: 'Toggle navigation' })
  if (await toggle.getAttribute('aria-expanded') !== 'true') await toggle.click()
  await page.getByRole('navigation', { name: 'Navigation' }).getByRole('link', { name: destination, exact: true }).click()
  const paths = { Library: '/library', Tables: '/tables', Settings: '/settings' }
  await expect(page).toHaveURL(new RegExp(`${paths[destination]}$`))
  if (destination === 'Library') {
    await expect(page.getByRole('button', { name: 'Upload Track', exact: true })).toBeVisible()
  } else {
    await expect(page.getByRole('heading', { name: destination, exact: true })).toBeVisible()
  }
}
