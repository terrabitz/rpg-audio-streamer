import type { Locator, Page } from '@playwright/test'
import { expect, test } from './fixtures'
import { addExistingTrack, createTable, login, navigate, uploadTrack } from './helpers'

test('GM sees playback position and volume on track cards', async ({ page, recordFlow }, testInfo) => {
  await login(page)
  await uploadTrack(page, 'Card Music', 'Music', 60)
  await navigate(page, 'Tables')
  await createTable(page, 'Shared Volume')
  await page.getByRole('link', { name: 'Open Shared Volume', exact: true }).click()
  await addExistingTrack(page, 'Card Music')
  await navigate(page, 'Tables')
  await page.getByRole('link', { name: 'Open My Table', exact: true }).click()
  const secondTable = await page.context().newPage()
  await secondTable.goto('/tables')
  await secondTable.getByRole('link', { name: 'Open Shared Volume', exact: true }).click()
  const secondTile = secondTable.getByRole('group', { name: 'Card Music', exact: true })
  await secondTile.click()
  await expect.poll(() => secondTable.locator('[data-active="true"]').evaluate(
    (el: HTMLMediaElement) => !el.paused && el.volume === 1,
  )).toBe(true)
  const secondVolume = secondTable.getByRole('complementary', { name: 'Currently playing' })
    .getByRole('slider', { name: 'Volume for Card Music' })
  await expect(secondVolume).toHaveAttribute('aria-valuenow', '100')
  const tile = page.getByRole('group', { name: 'Card Music', exact: true })
  await tile.click()
  const media = page.locator('[data-active="true"]')
  await expect.poll(() => media.evaluate((el: HTMLMediaElement) => !el.paused && el.currentTime > 0)).toBe(true)
  const sidebar = page.getByRole('complementary', { name: 'Currently playing' })
  const position = sidebar.getByRole('slider', { name: 'Playback position for Card Music' })
  const seekTrack = sidebar.locator('.track-position .v-slider-track')
  const seekBounds = await seekTrack.boundingBox()
  if (!seekBounds) throw new Error('Seek track must be visible')
  await seekTrack.click({ position: {
    x: seekBounds.width * 36 / Number(await position.getAttribute('aria-valuemax')),
    y: seekBounds.height / 2,
  } })
  await expect.poll(() => media.evaluate((el: HTMLMediaElement) => el.currentTime)).toBeGreaterThanOrEqual(35)
  const volume = sidebar.getByRole('slider', { name: 'Volume for Card Music' })
  recordFlow.start()
  await volume.press('Home')
  await expect(secondVolume).toHaveAttribute('aria-valuenow', '0')
  await expect(tile.locator('.tile-volume')).toHaveCSS('width', '0px')
  await volume.press('End')
  await expect(secondVolume).toHaveAttribute('aria-valuenow', '100')
  await expect.poll(() => tile.evaluate(el => {
    const bar = el.querySelector('.tile-volume')!
    return Math.round(bar.getBoundingClientRect().width / bar.parentElement!.clientWidth * 100)
  })).toBe(100)
  await sidebar.locator('.track-volume .v-slider-track').click()
  await expect(volume).toHaveAttribute('aria-valuenow', '50')
  await expect(secondVolume).toHaveAttribute('aria-valuenow', '50')
  await expect.poll(() => tile.evaluate(el => {
    const bar = el.querySelector('.tile-volume')!
    return Math.round(bar.getBoundingClientRect().width / bar.parentElement!.clientWidth * 100)
  })).toBe(50)
  recordFlow.stop()
  await expect(tile.locator('.tile-volume')).toHaveCSS('height', '2px')
  await expect(tile.locator('.tile-volume')).toHaveCSS('background-color', 'rgb(255, 255, 255)')
  await expect.poll(() => tile.locator('.audio-control-tile').evaluate(el =>
    parseFloat((el as HTMLElement).style.getPropertyValue('--track-progress')),
  )).toBeGreaterThanOrEqual(58)
  await expect(tile.locator('.audio-control-tile')).toHaveCSS('background-image', /linear-gradient/)
  const playingColors = await cardColors(page, tile)
  expect(playingColors.contrast).toBeGreaterThan(0)
  expect(new Set(playingColors.left).size).toBeGreaterThan(1)
  await page.screenshot({ path: testInfo.outputPath('card-indicators.png'), fullPage: true })
  await tile.click()
  await page.mouse.move(600, 300)
  await expect(tile.locator('.audio-control-tile')).not.toHaveClass(/is-active/)
  await expect(tile.locator('.tile-volume')).toBeVisible()
  await expect.poll(() => media.evaluate((el: HTMLMediaElement) => el.paused)).toBe(true)
  await expect(tile.locator('.audio-control-tile')).toHaveCSS('background-image', /linear-gradient/)
  await expect.poll(() => tile.locator('.audio-control-tile').evaluate(el =>
    parseFloat((el as HTMLElement).style.getPropertyValue('--track-progress')),
  )).toBeGreaterThanOrEqual(58)
  const pausedColors = await cardColors(page, tile)
  expect(pausedColors.contrast).toBeGreaterThan(0)
  expect(pausedColors.contrast).toBeLessThan(playingColors.contrast)
  expect(pausedColors.chromaticPixels).toBe(0)
  await page.screenshot({ path: testInfo.outputPath('paused-card.png'), fullPage: true })
  await tile.getByRole('button', { name: 'Edit Card Music', exact: true }).click()
  const pausedPosition = page.getByRole('dialog').getByRole('slider', { name: 'Playback position for Card Music' })
  await pausedPosition.press('Home')
  await expect.poll(() => tile.locator('.audio-control-tile').evaluate(el =>
    parseFloat((el as HTMLElement).style.getPropertyValue('--track-progress')),
  )).toBe(0)
  const startColors = await cardColors(page, tile)
  expect(startColors.contrast).toBe(0)
  await pausedPosition.press('End')
  await expect.poll(() => tile.locator('.audio-control-tile').evaluate(el =>
    parseFloat((el as HTMLElement).style.getPropertyValue('--track-progress')),
  )).toBeGreaterThan(95)
  const endColors = await cardColors(page, tile)
  expect(endColors.contrast).toBe(0)
  expect(endColors.left).not.toEqual(startColors.left)
  await expect.poll(() => media.evaluate((el: HTMLMediaElement) => el.paused)).toBe(true)
})

async function cardColors(page: Page, card: Locator) {
  const png = await card.screenshot({ animations: 'disabled' })
  return page.evaluate(async data => {
    const image = new Image()
    image.src = `data:image/png;base64,${data}`
    await image.decode()
    const canvas = document.createElement('canvas')
    canvas.width = image.width
    canvas.height = image.height
    const context = canvas.getContext('2d')
    if (!context) throw new Error('Canvas must be available to inspect card colors')
    context.drawImage(image, 0, 0)
    const color = (x: number) => Array.from(context.getImageData(
      Math.floor(image.width * x), Math.floor(image.height * 0.16), 1, 1,
    ).data).slice(0, 3)
    const left = color(0.25)
    const right = color(0.85)
    const pixels = context.getImageData(5, 5, image.width - 10, image.height - 10).data
    let chromaticPixels = 0
    for (let i = 0; i < pixels.length; i += 4) {
      if (pixels[i] !== pixels[i + 1] || pixels[i] !== pixels[i + 2]) chromaticPixels++
    }
    return {
      left,
      chromaticPixels,
      contrast: left.reduce((sum, channel, index) => sum + Math.abs(channel - (right[index] ?? 0)), 0),
    }
  }, png.toString('base64'))
}

for (const touch of [false, true]) {
  test(`GM plays a card from start using ${touch ? 'long press' : 'right click'}`, async ({ page }, testInfo) => {
    if (touch) await page.setViewportSize({ width: 390, height: 667 })
    await login(page)
    await uploadTrack(page, 'Restart Music', 'Music', 60)
    const tile = page.getByRole('group', { name: 'Restart Music', exact: true })
    await tile.click()
    await tile.getByRole('button', { name: 'Edit Restart Music', exact: true }).click()
    const dialog = page.getByRole('dialog')
    await dialog.getByRole('slider', { name: 'Beginning for Restart Music', exact: true }).press('PageUp')
    await dialog.getByRole('slider', { name: 'Playback position for Restart Music', exact: true }).press('End')
    await page.getByRole('button', { name: 'Close track controls', exact: true }).click()
    await expect(dialog).not.toBeVisible()
    await tile.click()
    await expect(tile.locator('.audio-control-tile')).not.toHaveClass(/is-active/)
    if (touch) {
      const bounds = await tile.boundingBox()
      if (!bounds) throw new Error('Track card must be visible')
      const session = await page.context().newCDPSession(page)
      try {
        await session.send('Input.dispatchTouchEvent', {
          type: 'touchStart', touchPoints: [{ x: bounds.x + bounds.width / 2, y: bounds.y + bounds.height / 2 }],
        })
        await expect(page.getByRole('menuitem', { name: 'Play from start', exact: true })).toBeVisible()
        await session.send('Input.dispatchTouchEvent', { type: 'touchEnd', touchPoints: [] })
      } finally {
        await session.detach()
      }
    } else await tile.click({ button: 'right' })
    await expect(tile.locator('.audio-control-tile')).not.toHaveClass(/is-active/)
    await page.screenshot({ path: testInfo.outputPath('card-context-menu.png'), fullPage: true })
    await page.getByRole('menuitem', { name: 'Play from start', exact: true }).click()
    const media = page.locator('[data-active="true"]')
    await expect.poll(() => media.evaluate((el: HTMLMediaElement) =>
      !el.paused && el.currentTime >= 5 && el.currentTime < 15,
    )).toBe(true)
    await expect(tile.locator('.audio-control-tile')).toHaveClass(/is-active/)
  })
}
