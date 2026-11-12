import { execFileSync } from 'node:child_process'
import { devices, expect, test, type Locator, type Page } from '@playwright/test'

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
  await page.getByRole('combobox', { name: 'Track Type', exact: true }).press('ArrowDown')
  await page.getByRole('option', { name: 'One-Shot', exact: true }).click()
  await page.getByRole('button', { name: 'Save Changes' }).click()
  await expect(page.locator('.file-tile').filter({ hasText: 'Rain' })).toBeVisible()
  await page.reload()
  const renamed = page.locator('.file-tile').filter({ hasText: 'Rain' })
  await expect(renamed).toBeVisible()
  await expect(renamed.locator('.audio-control-tile')).toHaveCSS(
    'border-top-color',
    'rgb(165, 214, 167)',
  )
  await expect(renamed.locator('.v-chip')).toHaveCount(0)
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
    const forestVolume = forest.getByRole('slider', { name: 'Volume for Forest' })
    const forestPosition = forest.getByRole('slider', { name: 'Playback position for Forest' })
    const names = () => tiles.locator('.track-heading > span')
    await expect(names()).toHaveText(['Forest', 'Rain', 'Tavern'])
    await expect(page.getByRole('button', { name: /^Move .+ (earlier|later)/ })).toHaveCount(0)
    await expect(page.getByRole('button', { name: /^Drag to reorder/ })).toHaveCount(3)
    await expect(forest.locator('.audio-control-tile')).toHaveCSS(
      'border-top-color',
      'rgb(128, 222, 234)',
    )
    await expect(forest.locator('.v-chip')).toHaveCount(0)
    await expectAlignedTrackControls(forest)
    await rain.getByRole('button', { name: 'Drag to reorder Rain' }).click()
    await expect(page.locator('[data-dnd-dragging]')).toHaveCount(0)
    for (const video of await page.locator('video').all()) {
      await expect.poll(() => video.evaluate((el: HTMLVideoElement) => el.paused)).toBe(true)
    }
    await forest.getByRole('button', { name: 'Play Forest', exact: true }).click()
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
    const slider = forest.locator('.v-slider-track').first()
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
    await forest.locator('.track-heading').click()
    await expect.poll(() => forestAudio.evaluate((el: HTMLVideoElement) => el.paused)).toBe(true)
    await expect(forestPosition).toBeEnabled()
    const position = await forest.locator('.track-position .v-slider-track').boundingBox()
    const playButton = await forest
      .getByRole('button', { name: 'Play Forest', exact: true })
      .boundingBox()
    const restartButton = await forest.getByRole('button', { name: 'Restart Forest' }).boundingBox()
    if (!position || !playButton || !restartButton)
      throw new Error('Inline playback controls must be visible')
    expect(position.y).toBeGreaterThan(volume.y)
    expect(playButton.x + playButton.width).toBeLessThan(position.x)
    expect(restartButton.x).toBeGreaterThan(position.x + position.width)
    await page.mouse.click(position.x + position.width / 2, position.y + position.height / 2)
    await expect(forestPosition).toHaveAttribute('aria-valuenow', /^(14|15|16)$/)
    const selectedTime = Number(await forestPosition.getAttribute('aria-valuenow'))
    await expect
      .poll(() => forestAudio.evaluate((el: HTMLVideoElement) => el.currentTime))
      .toBe(selectedTime)
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
      .toBe(draggedTime)
    await expect.poll(() => forestAudio.evaluate((el: HTMLVideoElement) => el.paused)).toBe(true)
    await forestPosition.press('Home')
    await forestPosition.press('ArrowRight')
    await expect.poll(() => forestAudio.evaluate((el: HTMLVideoElement) => el.currentTime)).toBe(1)
    await expect(forest.locator('.position-time')).toHaveText('0:01 / 0:30')
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
    await page.getByRole('button', { name: 'Delete track', exact: true }).click()
    await expect(names()).toHaveText(['Tavern', 'Forest', 'Rain'])
    await page.mouse.click(1000, 500)
    await expect(page.getByRole('dialog')).toHaveCount(0)
    await expect(page.getByText('File uploaded successfully!', { exact: true })).not.toBeVisible()
    for (const tile of [forest, rain, tavern]) {
      await expect(tile.getByRole('slider', { name: /^Volume for/ })).toBeVisible()
      await expect(tile.getByRole('slider', { name: /^Playback position for/ })).toBeVisible()
    }
    await expect(page.locator('video')).toHaveCount(3)
    for (const video of await page.locator('video').all()) {
      await expect.poll(() => video.evaluate((el: HTMLVideoElement) => el.paused)).toBe(true)
    }
    await page.screenshot({ path: testInfo.outputPath('gm-table-after.png'), fullPage: true })
    await page.setViewportSize({ width: 390, height: 844 })
    await expect(forestVolume).toBeVisible()
    await expect(forestPosition).toBeVisible()
    await expect(forest.getByRole('button', { name: 'Drag to reorder Forest' })).toBeVisible()
    await expect(forest.getByRole('button', { name: 'Restart Forest' })).toBeVisible()
    await expectAlignedTrackControls(forest)
    await expect
      .poll(() => page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth))
      .toBe(true)
    await page.screenshot({
      path: testInfo.outputPath('gm-table-mobile-after.png'),
      fullPage: true,
    })
    await forestVolume.press('Home')
    await expect(forestVolume).toHaveAttribute('aria-valuenow', '0')
    await forest.getByRole('button', { name: 'Restart Forest' }).click()
    for (const video of await page.locator('video').all()) {
      await expect.poll(() => video.evaluate((el: HTMLVideoElement) => el.paused)).toBe(true)
    }
    for (const name of ['Forest', 'Rain', 'Tavern']) {
      await tiles
        .filter({ hasText: name })
        .getByRole('button', { name: `Edit ${name}` })
        .click()
      await page.getByRole('button', { name: 'Delete track', exact: true }).click()
    }
    await expect(tiles).toHaveCount(0)
  })
})

for (const device of ['Pixel 7', 'iPhone 13']) {
  test(`touch dragging reflows the grid, supports cancellation, and preserves scrolling (${device})`, async ({
    browser,
  }, testInfo) => {
    test.setTimeout(60_000)
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
      await page.getByRole('button', { name: 'Close track controls' }).tap()
      await expect(page.getByRole('dialog')).toBeHidden()
      const seek = await tiles.first().locator('.track-position .v-slider-track').boundingBox()
      if (!seek) throw new Error('The playback slider must be visible on touch devices')
      await touch.send('Input.dispatchTouchEvent', {
        type: 'touchStart',
        touchPoints: [{ x: seek.x + seek.width / 2, y: seek.y + seek.height / 2 }],
      })
      await touch.send('Input.dispatchTouchEvent', { type: 'touchEnd', touchPoints: [] })
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
      await page.setViewportSize({ width: 390, height: 400 })
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
        await tiles
          .filter({ hasText: name })
          .getByRole('button', { name: `Edit ${name}` })
          .tap()
        await expect(page.getByRole('dialog')).toBeVisible()
        await page.getByRole('button', { name: 'Delete track', exact: true }).tap()
      }
      await expect(tiles).toHaveCount(0)
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
  await tile.getByRole('button', { name: 'Play Session Music', exact: true }).click()
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
    const trackVolume = tile.getByRole('slider', { name: 'Volume for Session Music' })
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
    const seek = await tile.locator('.track-position .v-slider-track').boundingBox()
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
    await tile.getByRole('button', { name: 'Restart Session Music' }).click()
    await expect
      .poll(() => gmAudio.evaluate((el: HTMLVideoElement) => el.currentTime))
      .toBeLessThan(2)
    await expect
      .poll(() => playerAudio.evaluate((el: HTMLVideoElement) => el.currentTime))
      .toBeLessThan(2)
    await expect.poll(() => playerAudio.evaluate((el: HTMLVideoElement) => el.paused)).toBe(false)
    await tile.getByRole('button', { name: 'Pause Session Music', exact: true }).click()
    await expect.poll(() => playerAudio.evaluate((el: HTMLVideoElement) => el.paused)).toBe(true)
    await tile.getByRole('button', { name: 'Restart Session Music' }).click()
    await expect.poll(() => gmAudio.evaluate((el: HTMLVideoElement) => el.currentTime)).toBe(0)
    await expect.poll(() => gmAudio.evaluate((el: HTMLVideoElement) => el.paused)).toBe(true)
    await tile.getByRole('button', { name: 'Play Session Music', exact: true }).click()
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
  await first.locator('.track-heading').click()
  await expect
    .poll(() =>
      page
        .locator('video')
        .first()
        .evaluate((el: HTMLVideoElement) => !el.paused && el.currentTime > 0),
    )
    .toBe(true)
  await second.locator('.track-heading').click()
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

async function expectAlignedTrackControls(tile: Locator) {
  const [volume, position, volumeIcon, playButton] = await Promise.all(
    [
      '.track-volume .v-slider-track',
      '.track-position .v-slider-track',
      '.track-volume .v-input__prepend .v-icon',
      '.v-btn[aria-label^="Play "], .v-btn[aria-label^="Pause "]',
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
