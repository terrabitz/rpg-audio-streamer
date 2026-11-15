import { devices } from '@playwright/test'
import { expect, test } from './fixtures'
import { expectCompactPlayingCard, expectAlignedTrackControls, expectTimelineLayout, expectPlaceholderAt, login, deleteLibraryTrack, uploadTrack } from './helpers'

test.describe('drag-handle reordering', () => {
  test('GM drags handles, keeps the order, and uses inline playback controls', async ({
    page,
  }, testInfo) => {
    testInfo.setTimeout(Math.max(testInfo.timeout, 60_000))
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
    await expect(settings.getByRole('button', { name: 'Play Forest', exact: true })).toBeVisible()
    await expect(forestPosition).toBeEnabled()
    await settings.locator('.timeline-axis').click()
    await expectTimelineLayout(settings)
    await page.screenshot({ path: testInfo.outputPath('track-settings-aligned.png'), fullPage: true })
    const position = await settings.locator('.timeline-axis').boundingBox()
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
      await page.getByRole('dialog').locator('.timeline-axis').tap()
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
