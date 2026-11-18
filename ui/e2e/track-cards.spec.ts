import { expect, test } from './fixtures'
import { login, uploadTrack } from './helpers'

test('GM sees playback position and volume on track cards', async ({ page }, testInfo) => {
  await login(page)
  await uploadTrack(page, 'Card Music', 'Music', 60)
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
  await volume.press('Home')
  await expect(tile.locator('.tile-volume')).toHaveCSS('width', '0px')
  await volume.press('End')
  await expect.poll(() => tile.evaluate(el => {
    const bar = el.querySelector('.tile-volume')!
    return Math.round(bar.getBoundingClientRect().width / bar.parentElement!.clientWidth * 100)
  })).toBe(100)
  await sidebar.locator('.track-volume .v-slider-track').click()
  await expect(volume).toHaveAttribute('aria-valuenow', '50')
  await expect.poll(() => tile.evaluate(el => {
    const bar = el.querySelector('.tile-volume')!
    return Math.round(bar.getBoundingClientRect().width / bar.parentElement!.clientWidth * 100)
  })).toBe(50)
  await expect(tile.locator('.tile-volume')).toHaveCSS('height', '2px')
  await expect(tile.locator('.tile-volume')).toHaveCSS('background-color', 'rgb(255, 255, 255)')
  await expect.poll(() => tile.locator('.audio-control-tile').evaluate(el =>
    parseFloat((el as HTMLElement).style.getPropertyValue('--track-progress')),
  )).toBeGreaterThanOrEqual(58)
  await expect(tile.locator('.audio-control-tile')).toHaveCSS('background-image', /linear-gradient/)
  await page.screenshot({ path: testInfo.outputPath('card-indicators.png'), fullPage: true })
  await tile.click()
  await expect(tile.locator('.audio-control-tile')).not.toHaveClass(/is-active/)
  await expect(tile.locator('.tile-volume')).toBeVisible()
})

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
