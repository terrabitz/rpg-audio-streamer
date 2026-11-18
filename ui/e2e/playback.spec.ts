import { expect, test } from './fixtures'
import { expectEdgeToEdgeTable, expectTrackGridSpacing, expectCompactPlayingCard, dragTimelinePoint, login, createTable, addExistingTrack, deleteTable, deleteLibraryTrack, uploadTrack, navigate } from './helpers'

for (const surface of ['sidebar', 'timeline'] as const) {
  test(`GM previews a seek in the ${surface} and commits on release`, async ({ page, browser }, testInfo) => {
    await login(page)
    await uploadTrack(page, 'Seek Music', 'Music', 60)
    const tile = page.getByRole('group', { name: 'Seek Music', exact: true })
    await tile.click()
    const gmAudio = page.locator('[data-active="true"]')
    await expect.poll(() => gmAudio.evaluate((el: HTMLMediaElement) => !el.paused && el.currentTime > 0)).toBe(true)
    const playerContext = await browser.newContext()
    try {
      const player = await playerContext.newPage()
      await player.goto(page.url())
      await player.getByRole('button', { name: 'Connect Audio', exact: true }).click()
      const playerAudio = player.locator('[data-active="true"]')
      await expect.poll(() => playerAudio.evaluate((el: HTMLMediaElement) => !el.paused && el.currentTime > 0)).toBe(true)
      if (surface === 'timeline') await tile.getByRole('button', { name: 'Edit Seek Music', exact: true }).click()
      const controls = surface === 'timeline'
        ? page.getByRole('dialog')
        : page.getByRole('complementary', { name: 'Currently playing' })
      const slider = controls.getByRole('slider', { name: 'Playback position for Seek Music' })
      const track = controls.locator(surface === 'timeline' ? '.timeline-axis' : '.track-position .v-slider-track')
      await expect(slider).toBeEnabled()
      if (surface === 'timeline') await expect(controls.getByTestId('waveform-track')).toBeVisible()
      await track.hover()
      const bounds = await track.boundingBox()
      if (!bounds) throw new Error('Seek track must be visible')
      const y = bounds.y + bounds.height / 2
      const maximum = surface === 'timeline' ? 60 : Number(await slider.getAttribute('aria-valuemax'))
      await page.mouse.move(bounds.x + bounds.width * 36 / maximum, y)
      await page.mouse.down()
      await page.mouse.move(bounds.x + bounds.width * 45 / maximum, y, { steps: 8 })
      await expect(slider).toHaveAttribute('aria-valuenow', '45')
      await page.screenshot({ path: testInfo.outputPath('seek-preview.png') })
      for (const media of [gmAudio, playerAudio]) {
        await expect.poll(() => media.evaluate((el: HTMLMediaElement) => el.currentTime)).toBeLessThan(30)
        const before = await media.evaluate((el: HTMLMediaElement) => el.currentTime)
        await expect.poll(() => media.evaluate((el: HTMLMediaElement) => el.currentTime)).toBeGreaterThan(before + 1)
        await expect.poll(() => media.evaluate((el: HTMLMediaElement) => el.paused)).toBe(false)
      }
      await expect(slider).toHaveAttribute('aria-valuenow', '45')
      const observations = await Promise.all([page, player].map(view => view.evaluateHandle(() => ({
        result: new Promise<{ elapsed: number, outgoing: number, incoming: number, time: number }[]>((resolve, reject) => {
          const outgoing = document.querySelector<HTMLMediaElement>('[data-active="true"]')!
          const incoming = [...document.querySelectorAll<HTMLMediaElement>('video, audio')].find(el => el !== outgoing)!
          const deadline = performance.now() + 10_000
          const samples: { elapsed: number, outgoing: number, incoming: number, time: number }[] = []
          let start: number | undefined
          function sample() {
            if (performance.now() > deadline) {
              reject(new Error('Seek crossfade did not complete'))
              return
            }
            if (!incoming.paused && incoming.currentTime >= 43) {
              start ??= incoming.currentTime
              samples.push({ elapsed: incoming.currentTime - start, outgoing: outgoing.volume,
                incoming: incoming.volume, time: incoming.currentTime })
            }
            if (samples.length && outgoing.paused && incoming.volume === 1) resolve(samples)
            else requestAnimationFrame(sample)
          }
          sample()
        }),
      }))))
      const playhead = await slider.evaluateHandle(element => {
        const values: number[] = []
        const timer = setInterval(() => values.push(Number(element.getAttribute('aria-valuenow'))), 5)
        return { finish() { clearInterval(timer); return values } }
      })
      await page.mouse.up()
      for (const observation of observations) {
        const samples = await observation.evaluate(value => value.result)
        const middle = samples.find(sample => sample.incoming >= 0.4 && sample.incoming <= 0.6)
        expect(middle, 'both sources must be audible halfway through the seek').toBeDefined()
        expect(middle!.outgoing + middle!.incoming).toBeCloseTo(1, 1)
        expect(middle!.time).toBeGreaterThan(43.5)
        expect(middle!.time).toBeLessThan(44.5)
        expect(samples.at(-1)!.time).toBeGreaterThanOrEqual(45)
        expect(samples.at(-1)!.time).toBeLessThan(46)
        expect(samples.at(-1)!.elapsed).toBeGreaterThan(1.7)
        await observation.dispose()
      }
      const positions = await playhead.evaluate(value => value.finish())
      await playhead.dispose()
      expect(positions.length).toBeGreaterThan(0)
      expect(Math.min(...positions), 'released seek must never flash the outgoing position').toBeGreaterThanOrEqual(42.5)
      for (const media of [gmAudio, playerAudio]) {
        await expect.poll(() => media.evaluate((el: HTMLMediaElement) => el.currentTime)).toBeGreaterThanOrEqual(44)
        await expect.poll(() => media.evaluate((el: HTMLMediaElement) => el.currentTime)).toBeLessThan(50)
        await expect.poll(() => media.evaluate((el: HTMLMediaElement) => el.paused)).toBe(false)
      }
    } finally {
      await playerContext.close()
    }
  })
}

for (const surface of ['sidebar', 'timeline'] as const) {
  test(`GM seeks near the end from the ${surface} and the playhead follows the loop`, async ({ page, browser }) => {
    await login(page)
    await uploadTrack(page, 'Loop Seek Music', 'Music', 30)
    const tile = page.getByRole('group', { name: 'Loop Seek Music', exact: true })
    await tile.click()
    const gmAudio = page.locator('[data-active="true"]')
    await expect.poll(() => gmAudio.evaluate((el: HTMLMediaElement) => !el.paused && el.currentTime > 0)).toBe(true)
    await tile.getByRole('button', { name: 'Edit Loop Seek Music', exact: true }).click()
    const dialog = page.getByRole('dialog')
    const loop = dialog.getByRole('slider', { name: 'Loop-back point for Loop Seek Music' })
    await loop.press('Home')
    await loop.press('PageUp')
    await expect(loop).toHaveAttribute('aria-valuenow', '5')
    const end = Number(await dialog.getByRole('slider', { name: 'End for Loop Seek Music' }).getAttribute('aria-valuenow'))
    await dialog.getByRole('button', { name: 'Close track controls' }).click()
    await expect(dialog).not.toBeVisible()
    if (surface === 'timeline') await tile.getByRole('button', { name: 'Edit Loop Seek Music', exact: true }).click()
    const controls = surface === 'timeline' ? dialog : page.getByRole('complementary', { name: 'Currently playing' })
    const slider = controls.getByRole('slider', { name: 'Playback position for Loop Seek Music' })
    await expect(slider).toHaveAttribute('aria-valuemax', String(end - 2))
    const context = await browser.newContext()
    try {
      const player = await context.newPage()
      await player.goto(page.url())
      await player.getByRole('button', { name: 'Connect Audio', exact: true }).click()
      const playerAudio = player.locator('[data-active="true"]')
      await expect.poll(() => playerAudio.evaluate((el: HTMLMediaElement) => !el.paused && el.currentTime > 0)).toBe(true)
      await slider.press('End')
      for (const media of [gmAudio, playerAudio]) {
        await expect.poll(() => media.evaluate((el: HTMLMediaElement) => el.currentTime)).toBeGreaterThanOrEqual(26)
        await expect.poll(() => media.evaluate((el: HTMLMediaElement) => el.currentTime)).toBeLessThan(30)
      }
      for (const media of [gmAudio, playerAudio]) {
        await expect.poll(() => media.evaluate((el: HTMLMediaElement) => el.currentTime)).toBeLessThan(12)
        await expect.poll(() => media.evaluate((el: HTMLMediaElement) => el.currentTime)).toBeGreaterThanOrEqual(5)
        await expect.poll(() => media.evaluate((el: HTMLMediaElement) => el.paused)).toBe(false)
      }
      const afterLoop = await gmAudio.evaluate((el: HTMLMediaElement) => el.currentTime)
      await expect.poll(async () => Number(await slider.getAttribute('aria-valuenow'))).toBeGreaterThan(afterLoop + 1)
      await expect.poll(async () => Math.abs(Number(await slider.getAttribute('aria-valuenow')) -
        await gmAudio.evaluate((el: HTMLMediaElement) => el.currentTime))).toBeLessThan(1)
    } finally { await context.close() }
  })
}

test('GM seeks near the beginning while players listen', { tag: '@timing' }, async ({ page, browser }) => {
  await login(page)
  await uploadTrack(page, 'Beginning Seek Music', 'Music', 60)
  const tile = page.getByRole('group', { name: 'Beginning Seek Music', exact: true })
  await tile.click()
  const gmAudio = page.locator('[data-active="true"]')
  await expect.poll(() => gmAudio.evaluate((el: HTMLMediaElement) => !el.paused && el.currentTime > 5), { timeout: 20_000 }).toBe(true)
  const playerContext = await browser.newContext()
  try {
    const player = await playerContext.newPage()
    await player.goto(page.url())
    await player.getByRole('button', { name: 'Connect Audio', exact: true }).click()
    const playerAudio = player.locator('[data-active="true"]')
    await expect.poll(() => playerAudio.evaluate((el: HTMLMediaElement) => !el.paused && el.currentTime > 5), { timeout: 20_000 }).toBe(true)
    await expect.poll(() => playerAudio.evaluate((el: HTMLMediaElement) => el.volume)).toBe(1)
    await tile.getByRole('button', { name: 'Edit Beginning Seek Music', exact: true }).click()
    const dialog = page.getByRole('dialog')
    const slider = dialog.getByRole('slider', { name: 'Playback position for Beginning Seek Music' })
    const track = dialog.locator('.timeline-axis')
    await expect(dialog.getByTestId('waveform-track')).toBeVisible()
    await track.hover()
    const bounds = await track.boundingBox()
    if (!bounds) throw new Error('Seek track must be visible')
    const duration = Number(await dialog.getByRole('slider', { name: 'End for Beginning Seek Music' }).getAttribute('aria-valuenow'))
    const y = bounds.y + bounds.height / 2
    await page.mouse.move(bounds.x + bounds.width * 40 / duration, y)
    await page.mouse.down()
    await page.mouse.move(bounds.x + bounds.width / duration, y, { steps: 8 })
    await expect(slider).toHaveAttribute('aria-valuenow', '1')
    const observations = await Promise.all([page, player].map(view => view.evaluateHandle(() => ({
      result: new Promise<{ time: number, outgoing: number, incoming: number }[]>((resolve, reject) => {
        const outgoing = document.querySelector<HTMLMediaElement>('[data-active="true"]')!
        const incoming = [...document.querySelectorAll<HTMLMediaElement>('video, audio')].find(el => el !== outgoing)!
        const deadline = performance.now() + 10_000
        const samples: { time: number, outgoing: number, incoming: number }[] = []
        function sample() {
          if (performance.now() > deadline) {
            reject(new Error('Seek did not complete'))
            return
          }
          if (!incoming.paused && incoming.currentTime < 4) {
            samples.push({ time: incoming.currentTime, outgoing: outgoing.volume, incoming: incoming.volume })
          }
          if (samples.length && outgoing.paused && !incoming.paused) resolve(samples)
          else requestAnimationFrame(sample)
        }
        sample()
      }),
    }))))
    await page.mouse.up()
    for (const observation of observations) {
      const samples = await observation.evaluate(value => value.result)
      const middle = samples.find(sample => sample.incoming >= 0.4 && sample.incoming <= 0.6)
      expect(middle, 'both sources must be audible halfway through the opening').toBeDefined()
      expect(middle!.outgoing + middle!.incoming).toBeCloseTo(1, 1)
      expect(middle!.time).toBeGreaterThan(0.75)
      expect(middle!.time).toBeLessThan(1.25)
      expect(samples.at(-1)!.time).toBeGreaterThanOrEqual(2)
      expect(samples.at(-1)!.time).toBeLessThan(3)
      expect(samples.at(-1)!.time - samples[0]!.time).toBeGreaterThan(1.7)
      await observation.dispose()
    }
    for (const media of [gmAudio, playerAudio]) {
      await expect.poll(() => media.evaluate((el: HTMLMediaElement) => el.paused)).toBe(false)
    }
  } finally { await playerContext.close() }
})

test('playback position follows the media clock without jumping backward @timing', async ({ page }) => {
  await login(page)
  await uploadTrack(page, 'Clock Music')
  await page.getByRole('group', { name: 'Clock Music', exact: true }).click()
  const media = page.locator('[data-active="true"]')
  await expect.poll(() => media.evaluate((el: HTMLMediaElement) => !el.paused && el.currentTime > 0)).toBe(true)
  const samples = await page.evaluate(async () => {
    const media = document.querySelector<HTMLMediaElement>('[data-active="true"]')!
    const slider = document.querySelector('[aria-label="Playback position for Clock Music"]')!
    const start = media.currentTime
    const deadline = performance.now() + 10_000
    const values: { media: number, slider: number }[] = []
    await new Promise<void>(resolve => {
      function sample() {
        values.push({ media: media.currentTime, slider: Number(slider.getAttribute('aria-valuenow')) })
        if (media.currentTime >= start + 4 || performance.now() >= deadline) resolve()
        else requestAnimationFrame(sample)
      }
      sample()
    })
    return values
  })
  expect(samples.at(-1)!.media - samples[0]!.media).toBeGreaterThanOrEqual(4)
  for (const [index, sample] of samples.entries()) {
    expect(Math.abs(sample.slider - sample.media)).toBeLessThan(1)
    if (index > 0) expect(sample.slider).toBeGreaterThanOrEqual(samples[index - 1]!.slider)
  }
})

test('player receives live playback, late-join state, and pause', async ({
  page,
  browser,
}) => {
  await login(page)
  const defaultTableURL = new URL(page.url()).pathname
  await uploadTrack(page, 'Session Music')
  const tile = page.locator('.file-tile').filter({ hasText: 'Session Music' })
  await tile.click()
  const sidebar = page.getByRole('complementary', { name: 'Currently playing' })
  const gmAudio = page.locator('[data-active="true"]')
  await expect
    .poll(() => gmAudio.evaluate((el: HTMLVideoElement) => !el.paused && el.currentTime > 0))
    .toBe(true)

  const playerContext = await browser.newContext()
  try {
    const player = await playerContext.newPage()
    await player.goto(defaultTableURL)
    await expect(player.getByText('Volume Mixer', { exact: true })).toBeVisible()
    await expect(player.getByRole('button', { name: 'Toggle navigation' })).toHaveCount(0)
    await expect(player.getByRole('navigation', { name: 'Navigation' })).toHaveCount(0)
    await expect(player.getByRole('banner').getByRole('link', { name: 'Login', exact: true })).toHaveCount(0)
    await expect(player.getByRole('button', { name: 'Add Track', exact: true })).toHaveCount(0)
    await expect(player.getByRole('banner')).toContainText('My Table')
    await expect(player.getByRole('heading', { name: 'My Table', exact: true })).toHaveCount(0)
    await player.getByRole('button', { name: 'Connect Audio', exact: true }).click()
    await expect(player.getByText('Connected', { exact: true })).toBeVisible()
    const playerAudio = player.locator('[data-active="true"]')
    await expect(playerAudio).toHaveCount(1)
    await expect
      .poll(() => playerAudio.evaluate((el: HTMLVideoElement) => !el.paused && el.currentTime > 0))
      .toBe(true)
    await expect.poll(() => playerAudio.evaluate((el: HTMLMediaElement) => el.loop)).toBe(false)
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
      .toBeLessThan(4)
    await expect
      .poll(() => playerAudio.evaluate((el: HTMLVideoElement) => el.currentTime))
      .toBeLessThan(4)
    await expect.poll(() => playerAudio.evaluate((el: HTMLVideoElement) => el.paused)).toBe(false)
    await sidebar.getByRole('button', { name: 'Pause Session Music', exact: true }).click()
    await expect.poll(() => playerAudio.evaluate((el: HTMLVideoElement) => el.paused)).toBe(true)
    await tile.getByRole('button', { name: 'Edit Session Music', exact: true }).click()
    await expect(page.getByRole('dialog').getByRole('button', { name: 'Restart Session Music' })).toHaveCount(0)
    await page.getByRole('dialog').getByRole('slider', { name: 'Playback position for Session Music' }).press('Home')
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

test('GM manages now-playing tracks in a tablet drawer', async ({ page }, testInfo) => {
  test.slow()
  await page.setViewportSize({ width: 768, height: 900 })
  await login(page)
  for (const name of ['Tablet Forest', 'Tablet Rain', 'Tablet Wind']) {
    await uploadTrack(page, name, 'Ambiance', 60)
    await page.getByRole('group', { name, exact: true }).click()
  }
  await page.getByRole('button', { name: 'Now playing (3)', exact: true }).click()
  const drawer = page.getByRole('dialog', { name: 'Currently playing', exact: true })
  await expect(drawer.getByRole('group')).toHaveCount(3)
  const cards = drawer.getByRole('group')
  for (const width of [600, 768, 839]) {
    await page.setViewportSize({ width, height: 900 })
    await expect.poll(async () => {
      const boxes = await cards.evaluateAll(elements => elements.map(element => {
        const { x, y, width } = element.getBoundingClientRect()
        return { x, y, width }
      }))
      const [first, second] = boxes
      return !!first && !!second && first.y === second.y && second.x > first.x
        && boxes.every(box => box.width >= 240 && box.width <= 320)
    }).toBe(true)
    for (const card of await cards.all()) await expectCompactPlayingCard(card)
    await expect.poll(() => page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true)
  }
  await page.setViewportSize({ width: 768, height: 900 })
  await page.screenshot({ path: testInfo.outputPath('tablet-now-playing.png'), fullPage: true, animations: 'disabled' })
  await drawer.getByRole('slider', { name: 'Volume for Tablet Rain', exact: true }).press('Home')
  await expect.poll(() => page.locator('video').nth(1).evaluate(
    (el: HTMLVideoElement) => el.volume === 0 && !el.paused,
  )).toBe(true)
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
  await expect.poll(() => page.locator('[data-active="true"]').first().evaluate(
    (el: HTMLMediaElement) => el.volume === 0 && !el.paused,
  )).toBe(true)
  await forest.getByRole('slider', { name: `Playback position for ${forestName}` }).press('Home')
  await forest.getByRole('slider', { name: `Playback position for ${forestName}` }).press('ArrowRight')
  await expect.poll(() => page.locator('[data-active="true"]').first().evaluate(
    (el: HTMLMediaElement) => el.currentTime < 4 && !el.paused,
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
  await expect.poll(() => page.locator('[data-active="true"]').first().evaluate(
    (el: HTMLMediaElement) => !el.paused,
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

test('tables share a library but isolate playback, removal, and deletion', async ({
  page, browser, context,
}) => {
  test.slow()
  await context.grantPermissions(['clipboard-read', 'clipboard-write'])
  await login(page)
  await navigate(page, 'Library')
  await uploadTrack(page, 'Shared Forest')
  await navigate(page, 'Tables')
  await createTable(page, 'Friday')
  await createTable(page, 'Sunday')
  await page.getByRole('link', { name: 'Open Friday', exact: true }).click()
  await addExistingTrack(page, 'Shared Forest')
  await navigate(page, 'Tables')
  await page.getByRole('button', { name: 'Options for Friday', exact: true }).click()
  await page.getByRole('menuitem', { name: 'Copy invite code for Friday', exact: true }).click()
  await expect(page.getByText('Invite code copied to clipboard.', { exact: true })).toBeVisible()
  const fridayLink = await page.evaluate(() => navigator.clipboard.readText())
  await page.getByRole('link', { name: 'Open Friday', exact: true }).click()

  const sundayGM = await context.newPage()
  await sundayGM.goto('/tables')
  await sundayGM.getByRole('link', { name: 'Open Sunday', exact: true }).click()
  await addExistingTrack(sundayGM, 'Shared Forest')
  await navigate(sundayGM, 'Tables')
  await sundayGM.getByRole('button', { name: 'Options for Sunday', exact: true }).click()
  await sundayGM.getByRole('menuitem', { name: 'Copy invite code for Sunday', exact: true }).click()
  await expect(sundayGM.getByText('Invite code copied to clipboard.', { exact: true })).toBeVisible()
  const sundayLink = await sundayGM.evaluate(() => navigator.clipboard.readText())
  expect(sundayLink).not.toBe(fridayLink)
  await sundayGM.getByRole('link', { name: 'Open Sunday', exact: true }).click()

  const playerContext = await browser.newContext()
  try {
    const fridayPlayer = await playerContext.newPage()
    const sundayPlayer = await playerContext.newPage()
    await fridayPlayer.goto(fridayLink)
    await sundayPlayer.goto(sundayLink)
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
    await navigate(page, 'Library')
    await deleteLibraryTrack(page, 'Shared Forest', 1)
    await expect(sundayTile).toHaveCount(0)
    await expect(sundayPlayer.locator('video')).toHaveCount(0)
    await navigate(page, 'Tables')
    await deleteTable(page, 'Friday')
    await expect(fridayPlayer.getByText('This table has been deleted.', { exact: true })).toBeVisible()
    await fridayPlayer.reload()
    await expect(fridayPlayer.getByText(
      'This table is unavailable or has been deleted.', { exact: true },
    )).toBeVisible()
    await deleteTable(page, 'Sunday')
    await expect(sundayPlayer.getByText('This table has been deleted.', { exact: true })).toBeVisible()
  } finally {
    await playerContext.close()
    await sundayGM.close()
  }
})

test('GM refresh and other tabs preserve track settings while music plays', async ({ page, context }, testInfo) => {
  test.slow()
  await login(page)
  await uploadTrack(page, 'Shared Settings', 'Music', 60)
  const tableURL = page.url()
  const tile = page.getByRole('group', { name: 'Shared Settings', exact: true })
  await tile.getByRole('button', { name: 'Edit Shared Settings', exact: true }).click()
  await dragTimelinePoint(page, 'Beginning for Shared Settings', 10)
  await dragTimelinePoint(page, 'End for Shared Settings', 50)
  await dragTimelinePoint(page, 'Loop-back point for Shared Settings', 20)
  await page.getByRole('button', { name: 'Close track controls', exact: true }).click()
  await tile.click()
  await expect.poll(() => page.locator('[data-active="true"]').evaluate(
    (media: HTMLMediaElement) => !media.paused && media.currentTime >= 10,
  )).toBe(true)
  const otherGM = await context.newPage()
  await otherGM.goto(tableURL)
  const otherTile = otherGM.getByRole('group', { name: 'Shared Settings', exact: true })
  await expect(otherTile).toBeVisible()
  await otherTile.getByRole('button', { name: 'Edit Shared Settings', exact: true }).click()
  await expect(otherGM.getByRole('slider', { name: 'Beginning for Shared Settings', exact: true }))
    .toHaveAttribute('aria-valuenow', '10')
  // Keep the second editor open while the first GM changes shared metadata.
  await tile.getByRole('button', { name: 'Edit Shared Settings', exact: true }).click()
  await dragTimelinePoint(page, 'Beginning for Shared Settings', 12)
  await page.getByRole('button', { name: 'Close track controls', exact: true }).click()
  await expect(otherGM.getByRole('slider', { name: 'Beginning for Shared Settings', exact: true }))
    .toHaveAttribute('aria-valuenow', '12')
  await otherGM.getByRole('textbox', { name: 'Track Name', exact: true }).fill('Renamed Settings')
  await otherGM.getByRole('button', { name: 'Close track controls', exact: true }).click()
  await expect(page.getByRole('group', { name: 'Renamed Settings', exact: true })).toBeVisible()
  await otherGM.reload()
  await expect(otherGM.getByRole('group', { name: 'Renamed Settings', exact: true })).toBeVisible()
  await navigate(otherGM, 'Tables')
  await otherGM.goto(tableURL)
  await page.reload()
  const enableAudio = page.getByRole('button', { name: 'Enable local audio', exact: true })
  await expect(page.getByRole('group', { name: 'Renamed Settings', exact: true })).toBeVisible()
  if (await enableAudio.isVisible()) await enableAudio.click()
  await page.getByRole('button', { name: 'Edit Renamed Settings', exact: true }).click()
  for (const [point, time] of [['Beginning', '12'], ['End', '50'], ['Loop-back point', '20']] as const) {
    await expect(page.getByRole('slider', { name: `${point} for Renamed Settings`, exact: true }))
      .toHaveAttribute('aria-valuenow', time)
  }
  await page.getByRole('button', { name: 'Close track controls', exact: true }).click()
  await expect.poll(() => page.locator('[data-active="true"]').evaluate(
    (media: HTMLMediaElement) => !media.paused && media.currentTime >= 12 && media.currentTime < 50,
  )).toBe(true)
  await expect(page.getByText('Unable to play track.', { exact: false })).not.toBeVisible()
  await page.screenshot({ path: testInfo.outputPath('gm-refresh-settings.png') })
  await otherGM.close()
})
