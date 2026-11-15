import { expect, test } from './fixtures'
import { expectAlignedTrackControls, expectTimelineLayout, dragTimelinePoint, login, deleteLibraryTrack, uploadTrack, navigate } from './helpers'

test('GM configures and persists track loop points', async ({ page }) => {
  test.slow()
  await login(page)
  await navigate(page, 'Library')
  await uploadTrack(page, 'Looping Forest')
  const row = page.getByRole('row').filter({ hasText: 'Looping Forest' })
  await row.getByRole('button', { name: 'Edit Looping Forest' }).click()
  await dragTimelinePoint(page, 'Beginning for Looping Forest', 10)
  await dragTimelinePoint(page, 'End for Looping Forest', 25)
  await dragTimelinePoint(page, 'Loop-back point for Looping Forest', 15)
  const timeline = page.getByRole('dialog').locator('.track-timeline')
  await expect(timeline.getByRole('slider', { name: 'Beginning for Looping Forest' })).toHaveText('0:10.0')
  await expect(timeline.getByRole('slider', { name: 'End for Looping Forest' })).toHaveText('0:25.0')
  await expect(timeline.getByRole('slider', { name: 'Loop-back point for Looping Forest' })).toHaveText('0:15.0')
  await expect(timeline.getByRole('slider', { name: 'Playback position for Looping Forest' }))
    .toHaveAttribute('aria-valuemin', '10')
  await expect(timeline.getByRole('slider', { name: 'Playback position for Looping Forest' }))
    .toHaveAttribute('aria-valuemax', '25')
  await expectTimelineLayout(page.getByRole('dialog'))
  await page.getByRole('button', { name: 'Save Changes', exact: true }).click()
  await expect(page.getByRole('dialog')).toHaveCount(0)
  await page.reload()
  const persistedRow = page.getByRole('row').filter({ hasText: 'Looping Forest' })
  await persistedRow.getByRole('button', { name: 'Edit Looping Forest' }).click()
  await expect(page.getByRole('slider', { name: 'Beginning for Looping Forest' })).toHaveAttribute('aria-valuenow', '10')
  await expect(page.getByRole('slider', { name: 'End for Looping Forest' })).toHaveAttribute('aria-valuenow', '25')
  await expect(page.getByRole('slider', { name: 'Loop-back point for Looping Forest' })).toHaveAttribute('aria-valuenow', '15')
  await page.getByRole('button', { name: 'Close track controls', exact: true }).click()
  await deleteLibraryTrack(page, 'Looping Forest', 0)
})

test('GM reads playback time relative to the trimmed track', async ({ page }) => {
  await login(page)
  await uploadTrack(page, 'Trimmed Timer')
  const tile = page.locator('.file-tile').filter({ hasText: 'Trimmed Timer' })
  await tile.getByRole('button', { name: 'Edit Trimmed Timer' }).click()
  await dragTimelinePoint(page, 'Beginning for Trimmed Timer', 10)
  await dragTimelinePoint(page, 'End for Trimmed Timer', 25)
  await page.getByRole('button', { name: 'Save Changes', exact: true }).click()
  await tile.locator('.track-heading').click()
  const playing = page.getByRole('group', { name: 'Playing Trimmed Timer', exact: true })
  const position = playing.getByRole('slider', { name: 'Playback position for Trimmed Timer' })
  await expect(playing.locator('.position-time')).toHaveText('0:00 / 0:15')
  await expect(position).toHaveAttribute('aria-valuetext', '0:00 of 0:15')
  await position.press('Home')
  await position.press('ArrowRight')
  const timer = await playing.evaluate(el => {
    const slider = el.querySelector('[aria-label="Playback position for Trimmed Timer"]')
    return {
      sourceTime: Number(slider?.getAttribute('aria-valuenow')),
      valueText: slider?.getAttribute('aria-valuetext'),
      text: el.querySelector('.position-time')?.textContent,
    }
  })
  const elapsed = Math.floor(timer.sourceTime - 10)
  expect(elapsed).toBeGreaterThanOrEqual(1)
  expect(elapsed).toBeLessThan(5)
  expect(timer.text).toBe(`0:0${elapsed} / 0:15`)
  expect(timer.valueText).toBe(`0:0${elapsed} of 0:15`)
  await expect.poll(() => page.locator('video').evaluate((media: HTMLVideoElement) => media.currentTime))
    .toBeGreaterThanOrEqual(11)
  await tile.getByRole('button', { name: 'Edit Trimmed Timer' }).click()
  await page.getByRole('button', { name: 'Remove from table', exact: true }).click()
  await deleteLibraryTrack(page, 'Trimmed Timer', 0)
})

test('GM plays and pauses a track from its settings', async ({ page }) => {
  await login(page)
  await uploadTrack(page, 'Settings Preview')
  const tile = page.locator('.file-tile').filter({ hasText: 'Settings Preview' })
  await tile.getByRole('button', { name: 'Edit Settings Preview' }).click()
  const dialog = page.getByRole('dialog')
  await expect(dialog.locator('.timeline-axis.has-waveform')).toBeVisible()
  await dialog.getByRole('button', { name: 'Play Settings Preview', exact: true }).click()
  const pause = dialog.getByRole('button', { name: 'Pause Settings Preview', exact: true })
  await expect(pause).toHaveAttribute('aria-pressed', 'true')
  await expect(page.getByRole('group', { name: 'Playing Settings Preview', exact: true })).toBeVisible()
  await expect.poll(() => page.locator('video').evaluate((media: HTMLVideoElement) => media.currentTime))
    .toBeGreaterThan(0.5)
  await pause.click()
  await expect(dialog.getByRole('button', { name: 'Play Settings Preview', exact: true }))
    .toHaveAttribute('aria-pressed', 'false')
  await expect.poll(() => page.locator('video').evaluate((media: HTMLVideoElement) => media.paused)).toBe(true)
  await dialog.getByRole('button', { name: 'Remove from table', exact: true }).click()
  await deleteLibraryTrack(page, 'Settings Preview', 0)
})

test('GM previews loop edits live for listeners before saving them', async ({ page, browser }) => {
  test.slow()
  await login(page)
  await uploadTrack(page, 'Live Loop')
  const tile = page.locator('.file-tile').filter({ hasText: 'Live Loop' })
  await tile.locator('.track-heading').click()
  const playerContext = await browser.newContext()
  try {
    const player = await playerContext.newPage()
    await player.goto(page.url())
    await player.getByRole('button', { name: 'Connect Audio', exact: true }).click()
    await expect.poll(() => player.locator('[data-active="true"]').evaluate(
      (media: HTMLMediaElement) => !media.paused && media.currentTime > 0,
    )).toBe(true)
    await tile.getByRole('button', { name: 'Edit Live Loop', exact: true }).click()
    await dragTimelinePoint(page, 'Beginning for Live Loop', 10)
    await dragTimelinePoint(page, 'End for Live Loop', 20)
    await dragTimelinePoint(page, 'Loop-back point for Live Loop', 12)
    for (const browserPage of [page, player]) {
      await expect.poll(() => browserPage.locator('[data-active="true"]').evaluate(
        (media: HTMLMediaElement) => !media.paused && media.currentTime >= 10,
      )).toBe(true)
    }
    const savedPlayback = async () => {
      const response = await page.request.get('/api/v1/files')
      expect(response.ok()).toBe(true)
      const tracks = await response.json()
      return tracks.find((track: { name: string }) => track.name === 'Live Loop').playback
    }
    expect(await savedPlayback()).toEqual({ startTime: 0, endTime: null, loopBackTime: null })
    const seek = page.getByRole('dialog').getByRole('slider', { name: 'Playback position for Live Loop' })
    await seek.press('End')
    for (const browserPage of [page, player]) {
      await expect.poll(() => browserPage.locator('[data-active="true"]').evaluate(
        (media: HTMLMediaElement) => !media.paused && media.currentTime >= 12 && media.currentTime < 18,
      )).toBe(true)
    }
    await page.getByRole('button', { name: 'Close track controls', exact: true }).click()
    const sidebar = page.getByRole('group', { name: 'Playing Live Loop', exact: true })
    await expect(sidebar.getByRole('slider', { name: 'Playback position for Live Loop' }))
      .toHaveAttribute('aria-valuemax', /^30(?:\.\d+)?$/)
    await expect(sidebar.locator('.v-slider-track__tick')).toHaveCount(0)
    expect(await savedPlayback()).toEqual({ startTime: 0, endTime: null, loopBackTime: null })
    await tile.getByRole('button', { name: 'Edit Live Loop', exact: true }).click()
    await dragTimelinePoint(page, 'Beginning for Live Loop', 10)
    await dragTimelinePoint(page, 'End for Live Loop', 20)
    await dragTimelinePoint(page, 'Loop-back point for Live Loop', 12)
    await page.getByRole('button', { name: 'Save Changes', exact: true }).click()
    await expect(page.getByRole('dialog')).toHaveCount(0)
    expect(await savedPlayback()).toEqual({ startTime: 10, endTime: 20, loopBackTime: 12 })
  } finally {
    await playerContext.close()
  }
  await deleteLibraryTrack(page, 'Live Loop', 1)
})

test('GM adjusts waveform markers without interrupting playback for listeners', async ({ page, browser }) => {
  test.slow()
  await login(page)
  await uploadTrack(page, 'Smooth Markers')
  const tile = page.locator('.file-tile').filter({ hasText: 'Smooth Markers' })
  await tile.locator('.track-heading').click()
  const playerContext = await browser.newContext()
  try {
    const player = await playerContext.newPage()
    await player.goto(page.url())
    await player.getByRole('button', { name: 'Connect Audio', exact: true }).click()
    for (const browserPage of [page, player]) {
      await expect.poll(() => browserPage.locator('[data-active="true"]').evaluate(
        (media: HTMLMediaElement) => !media.paused && media.currentTime > 3,
      )).toBe(true)
    }
    await tile.getByRole('button', { name: 'Edit Smooth Markers' }).click()
    const observations = await Promise.all([page, player].map(browserPage =>
      browserPage.locator('[data-active="true"]').evaluateHandle((media: HTMLMediaElement) => {
        const observation = { seeks: 0 }
        media.addEventListener('seeking', () => { observation.seeks++ })
        return observation
      }),
    ))
    for (const [name, key] of [
      ['Beginning', 'ArrowRight'],
      ['End', 'ArrowLeft'],
      ['Loop-back point', 'ArrowRight'],
    ]) {
      const positions = await Promise.all([page, player].map(browserPage =>
        browserPage.locator('[data-active="true"]').evaluate((media: HTMLMediaElement) => media.currentTime),
      ))
      const handle = page.getByRole('dialog').getByRole('slider', { name: `${name} for Smooth Markers`, exact: true })
      for (let step = 0; step < 6; step++) await handle.press(key)
      for (const [index, browserPage] of [page, player].entries()) {
        await expect.poll(() => browserPage.locator('[data-active="true"]').evaluate(
          (media: HTMLMediaElement) => media.paused ? 0 : media.currentTime,
        )).toBeGreaterThan(positions[index]! + 0.5)
        expect(await observations[index]!.evaluate(observation => observation.seeks)).toBe(0)
      }
    }
    await page.getByRole('button', { name: 'Close track controls', exact: true }).click()
    await tile.getByRole('button', { name: 'Edit Smooth Markers' }).click()
    await page.getByRole('button', { name: 'Remove from table', exact: true }).click()
    await deleteLibraryTrack(page, 'Smooth Markers', 0)
    for (const observation of observations) await observation.dispose()
  } finally { await playerContext.close() }
})

test('GM drags waveform markers continuously without stuttering for listeners', { tag: '@timing' }, async ({ page, browser }, testInfo) => {
  test.slow()
  await login(page)
  await uploadTrack(page, 'Continuous Drag')
  const tile = page.locator('.file-tile').filter({ hasText: 'Continuous Drag' })
  await tile.locator('.track-heading').click()
  const playerContext = await browser.newContext()
  try {
    const player = await playerContext.newPage()
    await player.goto(page.url())
    await player.getByRole('button', { name: 'Connect Audio', exact: true }).click()
    await tile.getByRole('button', { name: 'Edit Continuous Drag', exact: true }).click()
    for (const [point, toTime] of [
      ['Beginning', 2],
      ['End', 26],
      ['Loop-back point', 4],
    ] as const) {
      const seek = page.getByRole('dialog').getByRole('slider', { name: 'Playback position for Continuous Drag', exact: true })
      await seek.press('Home')
      await seek.press('PageUp')
      const target = Number(await page.getByRole('slider', { name: 'Beginning for Continuous Drag', exact: true })
        .getAttribute('aria-valuenow')) + 5
      for (const browserPage of [page, player]) {
        await expect.poll(() => browserPage.locator('[data-active="true"]').evaluate(
          (media: HTMLMediaElement, position: number) => !media.paused && !media.seeking &&
            media.readyState >= 3 && media.currentTime >= position && media.currentTime < position + 3,
          target,
        )).toBe(true)
      }
      const handle = page.getByRole('dialog').getByRole('slider', { name: `${point} for Continuous Drag`, exact: true })
      await handle.hover()
      const source = await handle.boundingBox()
      const axis = await page.getByRole('dialog').locator('.timeline-axis').boundingBox()
      const duration = Number(await page.getByRole('slider', { name: 'End for Continuous Drag', exact: true })
        .getAttribute('aria-valuemax'))
      const fromTime = Number(await handle.getAttribute('aria-valuenow'))
      if (!source || !axis) throw new Error('The waveform marker and axis must be visible')
      const start = source.x + source.width / 2
      const end = start + (toTime - fromTime) / duration * axis.width
      const y = source.y + source.height / 2
      await page.mouse.move(start, y)
      await page.mouse.down()
      const observations = await Promise.all([page, player].map(browserPage =>
        browserPage.locator('[data-active="true"]').evaluateHandle((media: HTMLMediaElement) => {
          const samples: {
            elapsed: number, position: number, paused: boolean, seeking: boolean,
            readyState: number, bufferedEnd: number, playbackRate: number,
            visibility: DocumentVisibilityState,
          }[] = []
          const started = performance.now()
          let seeks = 0
          const onSeeking = () => { seeks++ }
          media.addEventListener('seeking', onSeeking)
          const sample = () => samples.push({
            elapsed: (performance.now() - started) / 1000,
            position: media.currentTime, paused: media.paused, seeking: media.seeking,
            readyState: media.readyState,
            bufferedEnd: media.buffered.length ? media.buffered.end(media.buffered.length - 1) : 0,
            playbackRate: media.playbackRate,
            visibility: document.visibilityState,
          })
          sample()
          const timer = setInterval(sample, 50)
          return {
            finish() {
              clearInterval(timer)
              media.removeEventListener('seeking', onSeeking)
              sample()
              return { seeks, samples }
            },
          }
        }),
      ))
      try {
        const deadline = Date.now() + 6000
        let direction = true
        do {
          await page.mouse.move(direction ? end : start, y, { steps: 30 })
          direction = !direction
        } while (Date.now() < deadline)
      } finally { await page.mouse.up() }
      for (const [index, observation] of observations.entries()) {
        const result = await observation.evaluate(value => value.finish())
        await testInfo.attach(`${point}-${index === 0 ? 'GM' : 'listener'}-playback.json`, {
          body: JSON.stringify(result), contentType: 'application/json',
        })
        expect(result.seeks, `${point}: no seek events during an in-bounds marker drag`).toBe(0)
        for (const [sampleIndex, sample] of result.samples.entries()) {
          expect(sample.paused, `${point}: playback stays unpaused throughout the gesture`).toBe(false)
          expect(sample.seeking, `${point}: playback does not seek during the gesture`).toBe(false)
          const earlier = result.samples.slice(0, sampleIndex).findLast(previous => sample.elapsed - previous.elapsed >= 0.5)
          if (!earlier) continue
          expect(sample.position - earlier.position,
            `${point} ${index === 0 ? 'GM' : 'listener'}: every half-second window advances smoothly (${JSON.stringify({ earlier, sample })})`)
            .toBeGreaterThanOrEqual((sample.elapsed - earlier.elapsed) * 0.8)
        }
        await observation.dispose()
      }
    }
    await page.getByRole('button', { name: 'Remove from table', exact: true }).click()
  } finally { await playerContext.close() }
  await deleteLibraryTrack(page, 'Continuous Drag', 0)
})

test('GM continuously drags markers through short natural loops without audio stalls', { tag: '@timing' }, async ({ page, browser }, testInfo) => {
  test.slow()
  await login(page)
  await uploadTrack(page, 'Natural Loop Drag', 'Music', 12)
  const tile = page.locator('.file-tile').filter({ hasText: 'Natural Loop Drag' })
  await tile.getByRole('button', { name: 'Edit Natural Loop Drag', exact: true }).click()
  await dragTimelinePoint(page, 'Loop-back point for Natural Loop Drag', 9)
  await page.getByRole('dialog').getByRole('button', { name: 'Play Natural Loop Drag', exact: true }).click()
  const playerContext = await browser.newContext()
  try {
    const player = await playerContext.newPage()
    await player.goto(page.url())
    await player.getByRole('button', { name: 'Connect Audio', exact: true }).click()
    for (const browserPage of [page, player]) {
      await expect.poll(() => browserPage.locator('[data-active="true"]').evaluate(
        (media: HTMLMediaElement) => !media.paused && !media.seeking && media.currentTime > 9,
      ), { timeout: 20_000 }).toBe(true)
    }
    for (const [point, low, high] of [
      ['Beginning', 2, 11],
      ['End', 11.5, 12],
      ['Loop-back point', 9, 11],
    ] as const) {
      const handle = page.getByRole('dialog').getByRole('slider', { name: `${point} for Natural Loop Drag`, exact: true })
      await handle.hover()
      const source = await handle.boundingBox()
      const axis = await page.getByRole('dialog').locator('.timeline-axis').boundingBox()
      const duration = Number(await page.getByRole('slider', { name: 'End for Natural Loop Drag', exact: true })
        .getAttribute('aria-valuemax'))
      const current = Number(await handle.getAttribute('aria-valuenow'))
      if (!source || !axis) throw new Error('The waveform marker and axis must be visible')
      const start = source.x + source.width / 2
      const x = (time: number) => start + (time - current) / duration * axis.width
      const y = source.y + source.height / 2
      await page.mouse.move(start, y)
      await page.mouse.down()
      const observations = await Promise.all([page, player].map(browserPage =>
        browserPage.evaluateHandle(() => {
          const media = Array.from(document.querySelectorAll<HTMLMediaElement>('video, audio'))
          const read = () => media.map(source => ({
            position: source.currentTime, volume: source.volume, paused: source.paused,
            seeking: source.seeking, readyState: source.readyState,
          }))
          const started = performance.now()
          let lastTime = started
          let previous = read()
          let advanced = 0
          const samples: { elapsed: number, advanced: number, sources: ReturnType<typeof read> }[] = []
          const sample = () => {
            const now = performance.now()
            const elapsed = (now - lastTime) / 1000
            const sources = read()
            const progress = Math.max(0, ...sources.map((source, index) => {
              const before = previous[index]!
              if (source.paused || source.seeking || Math.max(source.volume, before.volume) < 0.05) return 0
              return Math.min(elapsed, Math.max(0, source.position - before.position))
            }))
            advanced += progress
            samples.push({ elapsed: (now - started) / 1000, advanced, sources })
            previous = sources
            lastTime = now
          }
          sample()
          const timer = setInterval(sample, 25)
          return {
            finish() { clearInterval(timer); sample(); return samples },
          }
        }),
      ))
      try {
        const deadline = Date.now() + 6000
        let direction = true
        do {
          await page.mouse.move(x(direction ? low : high), y, { steps: 30 })
          direction = !direction
        } while (Date.now() < deadline)
      } finally { await page.mouse.up() }
      for (const [index, observation] of observations.entries()) {
        const samples = await observation.evaluate(value => value.finish())
        await testInfo.attach(`${point}-${index === 0 ? 'GM' : 'listener'}-loop-progress.json`, {
          body: JSON.stringify(samples), contentType: 'application/json',
        })
        for (const [sampleIndex, sample] of samples.entries()) {
          const earlier = samples.slice(0, sampleIndex).findLast(previous => sample.elapsed - previous.elapsed >= 0.5)
          if (!earlier) continue
          expect(sample.advanced - earlier.advanced, `${point}: audible playback advances through every loop and drag`)
            .toBeGreaterThanOrEqual((sample.elapsed - earlier.elapsed) * 0.8)
        }
        await observation.dispose()
      }
    }
    await page.getByRole('button', { name: 'Remove from table', exact: true }).click()
  } finally { await playerContext.close() }
  await deleteLibraryTrack(page, 'Natural Loop Drag', 0)
})

for (const point of ['Loop-back point', 'End']) {
  test(`GM keeps looping while adjusting the ${point.toLowerCase()} marker`, async ({ page, browser }) => {
    test.slow()
    await login(page)
    await uploadTrack(page, 'Editing Loop')
    const tile = page.locator('.file-tile').filter({ hasText: 'Editing Loop' })
    await tile.getByRole('button', { name: 'Edit Editing Loop', exact: true }).click()
    await dragTimelinePoint(page, 'End for Editing Loop', 4)
    await dragTimelinePoint(page, 'Loop-back point for Editing Loop', 1)
    await page.getByRole('dialog').getByRole('button', { name: 'Play Editing Loop', exact: true }).click()
    const playerContext = await browser.newContext()
    try {
      const player = await playerContext.newPage()
      await player.goto(page.url())
      await player.getByRole('button', { name: 'Connect Audio', exact: true }).click()
      for (const browserPage of [page, player]) {
        await expect.poll(() => browserPage.locator('[data-active="true"]').evaluate(
          (media: HTMLMediaElement) => !media.paused && media.currentTime > 0,
        )).toBe(true)
      }
      const observations = await Promise.all([page, player].map(browserPage =>
        browserPage.evaluateHandle(() => {
          const observation = { handoffs: 0 }
          let active = document.querySelector('[data-active="true"]')
          const observer = new MutationObserver(() => {
            const next = document.querySelector('[data-active="true"]')
            if (next && next !== active) observation.handoffs++
            active = next
          })
          for (const media of document.querySelectorAll('video, audio')) {
            observer.observe(media, { attributes: true, attributeFilter: ['data-active'] })
          }
          return observation
        }),
      ))
      let edits = 0
      const handle = page.getByRole('dialog').getByRole('slider', { name: `${point} for Editing Loop`, exact: true })
      await expect.poll(async () => {
        await handle.press(edits++ % 2 === 0 ? 'ArrowRight' : 'ArrowLeft')
        return Math.min(...await Promise.all(observations.map(observation =>
          observation.evaluate(value => value.handoffs),
        )))
      }, { timeout: 30_000, intervals: [100] }).toBeGreaterThanOrEqual(4)
      for (const browserPage of [page, player]) {
        const position = await browserPage.locator('[data-active="true"]').evaluate(
          (media: HTMLMediaElement) => media.currentTime,
        )
        await expect.poll(() => browserPage.locator('[data-active="true"]').evaluate(
          (media: HTMLMediaElement, previous: number) => !media.paused && Math.abs(media.currentTime - previous) > 0.25,
          position,
        )).toBe(true)
      }
      for (const observation of observations) await observation.dispose()
      await page.getByRole('button', { name: 'Remove from table', exact: true }).click()
    } finally { await playerContext.close() }
    await deleteLibraryTrack(page, 'Editing Loop', 0)
  })
}

test('GM auditions unsaved loop points in the library', async ({ page }) => {
  await login(page)
  await navigate(page, 'Library')
  await uploadTrack(page, 'Draft Preview')
  const row = page.getByRole('row').filter({ hasText: 'Draft Preview' })
  await row.getByRole('button', { name: 'Edit Draft Preview', exact: true }).click()
  await dragTimelinePoint(page, 'Beginning for Draft Preview', 5)
  await dragTimelinePoint(page, 'End for Draft Preview', 15)
  const dialog = page.getByRole('dialog')
  await dialog.getByRole('button', { name: 'Play Draft Preview', exact: true }).click()
  await expect.poll(() => page.locator('[data-active="true"]').evaluate(
    (media: HTMLMediaElement) => !media.paused && media.currentTime >= 5,
  )).toBe(true)
  await dialog.getByRole('button', { name: 'Pause Draft Preview', exact: true }).click()
  await dialog.getByRole('button', { name: 'Play Draft Preview', exact: true }).click()
  await expect.poll(() => page.locator('[data-active="true"]').evaluate(
    (media: HTMLMediaElement) => !media.paused && media.currentTime >= 5,
  )).toBe(true)
  await expect(dialog.getByRole('slider', { name: 'Beginning for Draft Preview' })).toHaveAttribute('aria-valuenow', '5')
  await expect(dialog.getByRole('slider', { name: 'End for Draft Preview' })).toHaveAttribute('aria-valuenow', '15')
  await dialog.getByRole('slider', { name: 'Playback position for Draft Preview' }).press('End')
  await expect.poll(() => page.locator('[data-active="true"]').evaluate(
    (media: HTMLMediaElement) => !media.paused && media.currentTime >= 5 && media.currentTime < 12,
  )).toBe(true)
  await dialog.getByRole('button', { name: 'Close track controls', exact: true }).click()
  await row.getByRole('button', { name: 'Edit Draft Preview', exact: true }).click()
  await expect(dialog.getByRole('slider', { name: 'Beginning for Draft Preview' })).toHaveAttribute('aria-valuenow', '0')
  await expect(dialog.getByRole('slider', { name: 'End for Draft Preview' })).toHaveAttribute('aria-valuenow', /^30(?:\.\d+)?$/)
  await dialog.getByRole('button', { name: 'Close track controls', exact: true }).click()
  await deleteLibraryTrack(page, 'Draft Preview', 0)
})

test('GM trims a one-shot without loop-back controls', async ({ page }) => {
  await login(page)
  await uploadTrack(page, 'Trimmed Bell', 'One-Shot')
  const tile = page.locator('.file-tile').filter({ hasText: 'Trimmed Bell' })
  await tile.getByRole('button', { name: 'Edit Trimmed Bell' }).click()
  const dialog = page.getByRole('dialog')
  await expect(dialog.getByRole('slider', { name: 'Loop-back point for Trimmed Bell' })).toHaveCount(0)
  await dragTimelinePoint(page, 'Beginning for Trimmed Bell', 1)
  await dragTimelinePoint(page, 'End for Trimmed Bell', 12)
  await dialog.getByRole('combobox', { name: 'Track Type', exact: true }).press('ArrowDown')
  await page.getByRole('option', { name: 'Music', exact: true }).click()
  await expect(dialog.getByRole('slider', { name: 'Loop-back point for Trimmed Bell' })).toBeVisible()
  await dialog.getByRole('combobox', { name: 'Track Type', exact: true }).press('ArrowDown')
  await page.getByRole('option', { name: 'One-Shot', exact: true }).click()
  await expect(dialog.getByRole('slider', { name: 'Loop-back point for Trimmed Bell' })).toHaveCount(0)
  await dialog.getByRole('button', { name: 'Save Changes', exact: true }).click()
  await tile.locator('.track-heading').click()
  await expect(page.locator('.playing-sidebar .v-slider-track__tick')).toHaveCount(0)
  await expect.poll(() => page.locator('video').evaluate((media: HTMLVideoElement) => media.currentTime))
    .toBeGreaterThanOrEqual(1)
  await tile.getByRole('button', { name: 'Edit Trimmed Bell' }).click()
  await dialog.getByRole('button', { name: 'Remove from table', exact: true }).click()
  await deleteLibraryTrack(page, 'Trimmed Bell', 0)
})

test('GM changes sidebar volume smoothly over two seconds', async ({ page }) => {
  await login(page)
  await uploadTrack(page, 'Smooth Volume')
  const tile = page.locator('.file-tile').filter({ hasText: 'Smooth Volume' })
  await tile.locator('.track-heading').click()
  const playing = page.getByRole('group', { name: 'Playing Smooth Volume', exact: true })
  await expectAlignedTrackControls(playing, true)
  await expect(playing.locator('.v-slider-track__tick')).toHaveCount(0)
  const media = page.locator('video')
  await expect.poll(() => media.evaluate((el: HTMLVideoElement) => !el.paused && el.volume === 1)).toBe(true)
  const [samples] = await Promise.all([
    media.evaluate((el: HTMLVideoElement) => new Promise<number[]>((resolve, reject) => {
      const deadline = performance.now() + 10000
      let start: number | undefined
      const values: number[] = []
      function sample(now: number) {
        if (now > deadline) {
          reject(new Error('The volume adjustment did not complete'))
          return
        }
        if (start === undefined && el.volume < 1) {
          start = now
          values.push(el.volume)
        }
        if (start !== undefined && values.length === 1 && now - start >= 1000) values.push(el.volume)
        if (start !== undefined && now - start >= 2100) {
          values.push(el.volume)
          resolve(values)
          return
        }
        requestAnimationFrame(sample)
      }
      requestAnimationFrame(sample)
    })),
    playing.getByRole('slider', { name: 'Volume for Smooth Volume' }).press('Home'),
  ])
  expect(samples[0]).toBeGreaterThan(0.9)
  expect(samples[0]).toBeLessThan(1)
  expect(samples[1]).toBeGreaterThan(0.4)
  expect(samples[1]).toBeLessThan(0.6)
  expect(samples[2]).toBe(0)
  await expect.poll(() => media.evaluate((el: HTMLVideoElement) => el.paused)).toBe(false)
  await tile.getByRole('button', { name: 'Edit Smooth Volume' }).click()
  await page.getByRole('button', { name: 'Remove from table', exact: true }).click()
  await deleteLibraryTrack(page, 'Smooth Volume', 0)
})

test.describe('touch loop controls', () => {
  test.use({ isMobile: true, hasTouch: true, viewport: { width: 390, height: 844 } })

  test('GM trims a track and adjusts loop-back on the shared timeline on mobile', async ({ page }) => {
    test.slow()
    await login(page)
    await uploadTrack(page, 'Mobile Loop')
    const tile = page.getByRole('group', { name: 'Mobile Loop', exact: true })
    await tile.getByRole('button', { name: 'Edit Mobile Loop' }).tap()
    await dragTimelinePoint(page, 'Beginning for Mobile Loop', 2, true)
    await dragTimelinePoint(page, 'End for Mobile Loop', 20, true)
    await dragTimelinePoint(page, 'Loop-back point for Mobile Loop', 5, true)
    const dialog = page.getByRole('dialog')
    const beginning = dialog.getByRole('slider', { name: 'Beginning for Mobile Loop' })
    await beginning.press('ArrowRight')
    await expect(beginning).toHaveAttribute('aria-valuenow', '2.1')
    await expectTimelineLayout(dialog)
    const cursor = dialog.getByRole('slider', { name: 'Playback position for Mobile Loop' })
    await cursor.press('Home')
    await expect.poll(() => page.locator('video').evaluate((media: HTMLVideoElement) => media.currentTime)).toBe(2.1)
    await cursor.press('End')
    await expect.poll(() => page.locator('video').evaluate((media: HTMLVideoElement) => media.currentTime)).toBe(20)
    await expect.poll(() => page.locator('video').evaluate((media: HTMLVideoElement) => media.paused)).toBe(true)
    await page.getByRole('button', { name: 'Save Changes', exact: true }).tap()
    await tile.getByRole('button', { name: 'Edit Mobile Loop' }).tap()
    await expect(beginning).toHaveAttribute('aria-valuenow', '2.1')
    await expect(dialog.getByRole('slider', { name: 'Loop-back point for Mobile Loop' })).toHaveAttribute('aria-valuenow', '5')
    await dragTimelinePoint(page, 'Loop-back point for Mobile Loop', 2.1, true)
    await expect(dialog.getByRole('slider', { name: 'Loop-back point for Mobile Loop' }))
      .toHaveAttribute('aria-valuetext', 'Beginning (0:02.1)')
    await beginning.press('Shift+ArrowRight')
    await expect(dialog.getByRole('slider', { name: 'Loop-back point for Mobile Loop' })).toHaveText('0:03.1')
    await page.getByRole('button', { name: 'Save Changes', exact: true }).tap()
    await tile.getByRole('button', { name: 'Edit Mobile Loop' }).tap()
    await expect(dialog.getByRole('slider', { name: 'Loop-back point for Mobile Loop' }))
      .toHaveAttribute('aria-valuetext', 'Beginning (0:03.1)')
    await beginning.press('Home')
    await dialog.getByRole('slider', { name: 'End for Mobile Loop' }).press('End')
    await expect(beginning).toHaveAttribute('aria-valuenow', '0')
    await expect(dialog.getByRole('slider', { name: 'Loop-back point for Mobile Loop' })).toHaveText('0:00.0')
    await expect(dialog.getByRole('button', { name: 'Use beginning', exact: true })).toHaveCount(0)
    await expect(dialog.getByRole('button', { name: 'Reset points', exact: true })).toHaveCount(0)
    await dialog.getByRole('button', { name: 'Close track controls' }).tap()
    await tile.getByRole('button', { name: 'Edit Mobile Loop' }).tap()
    await expect(beginning).toHaveAttribute('aria-valuenow', '3.1')
    await dialog.getByRole('button', { name: 'Remove from table', exact: true }).tap()
    await expect(tile).toHaveCount(0)
    await deleteLibraryTrack(page, 'Mobile Loop', 0)
  })
})

test('GM crossfades a repeating track at its configured loop point', async ({ page }) => {
  test.slow()
  await login(page)
  await uploadTrack(page, 'Boss Loop')
  const tile = page.locator('.file-tile').filter({ hasText: 'Boss Loop' })
  await tile.getByRole('button', { name: 'Edit Boss Loop', exact: true }).click()
  await dragTimelinePoint(page, 'Beginning for Boss Loop', 1)
  await dragTimelinePoint(page, 'End for Boss Loop', 6)
  await dragTimelinePoint(page, 'Loop-back point for Boss Loop', 2)
  await page.getByRole('button', { name: 'Save Changes', exact: true }).click()

  const video = page.locator('video')
  const audio = page.locator('audio')
  await tile.locator('.track-heading').click()
  const sidebarSeek = page.locator('.playing-sidebar .track-position')
  const loopTick = sidebarSeek.locator('.v-slider-track__tick')
  await expect(loopTick).toHaveCount(1)
  await expect(loopTick).toHaveCSS('width', '2px')
  await expect(loopTick).toHaveCSS('height', '8px')
  await expect(sidebarSeek.locator('.v-slider-track__tick-label')).toHaveCount(0)
  const warningColor = await loopTick.evaluate(el => getComputedStyle(el).getPropertyValue('--v-theme-warning').trim())
  await expect(loopTick).toHaveCSS('background-color', `rgb(${warningColor.split(',').map(value => value.trim()).join(', ')})`)
  await expect.poll(async () => {
    const rail = await sidebarSeek.locator('.v-slider-track__background').boundingBox()
    const tick = await loopTick.boundingBox()
    if (!rail || !tick) throw new Error('The seek rail and loop-back tick must be visible')
    return Math.abs(tick.x + tick.width / 2 - rail.x - rail.width * (2 - 1) / (6 - 1))
  }).toBeLessThan(1)
  await expect.poll(() => audio.evaluate((media: HTMLAudioElement) => !media.paused), { timeout: 20_000 }).toBe(true)
  await expect.poll(() => audio.evaluate((media: HTMLAudioElement) => media.currentTime))
    .toBeGreaterThan(2)
  await expect.poll(() => Promise.all([
    video.evaluate((media: HTMLVideoElement) => media.volume > 0),
    audio.evaluate((media: HTMLAudioElement) => media.volume > 0),
  ])).toEqual([true, true])
  await expect.poll(() => video.evaluate((media: HTMLVideoElement) => media.paused)).toBe(true)
  await expect.poll(() => audio.evaluate((media: HTMLAudioElement) => media.currentTime))
    .toBeGreaterThan(3)

  await tile.getByRole('button', { name: 'Edit Boss Loop', exact: true }).click()
  await page.getByRole('slider', { name: 'Loop-back point for Boss Loop' }).press('Home')
  await page.getByRole('button', { name: 'Save Changes', exact: true }).click()
  await expect(loopTick).toHaveCount(0)
  await tile.getByRole('button', { name: 'Edit Boss Loop', exact: true }).click()
  await page.getByRole('button', { name: 'Remove from table', exact: true }).click()
  await expect(tile).toHaveCount(0)
  await deleteLibraryTrack(page, 'Boss Loop', 0)
})
