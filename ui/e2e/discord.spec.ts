import { fileURLToPath } from 'node:url'
import { type Page } from '@playwright/test'
import { expect, recording, test } from './fixtures'
import { discordTestConfig } from './discord-config'
import { startTestListener, type ListenerAudio } from './discord-listener'
import { login, createTable, uploadTrack } from './helpers'

test('GM controls real Discord audio and reconnects the saved channel', { tag: '@discord' }, async ({ page }) => {
  if (recording) throw new Error('The Discord journey cannot run in recording mode')
  test.slow()
  const config = discordTestConfig(fileURLToPath(new URL('../../', import.meta.url)))
  await login(page)
  await page.goto('/tables')
  await createTable(page, 'Discord audio')
  await page.getByRole('link', { name: 'Open Discord audio', exact: true }).click()
  await expect(page).toHaveURL(/\/tables\/[A-Za-z0-9_-]+$/)
  const tableID = new URL(page.url()).pathname.split('/').pop()!
  const observer = await page.context().newPage()
  let observerStatusRequests = 0
  observer.on('request', request => {
    if (request.method() === 'GET' &&
        new URL(request.url()).pathname === `/api/v1/tables/${tableID}/discord`) {
      observerStatusRequests++
    }
  })
  let listener: Awaited<ReturnType<typeof startTestListener>> | undefined
  try {
    await observer.goto(page.url())
    await expect(observer.getByRole('button', { name: 'Connect', exact: true })).toBeVisible()
    await uploadTrack(page, 'Discord Low', 'Ambiance', 440, 120)
    await uploadTrack(page, 'Discord High', 'Music', 880, 120)
    await uploadTrack(page, 'Discord Replacement', 'Music', 440, 120)
    await chooseDiscordDestination(page)
    await expect(page.getByRole('button', { name: 'Connected', exact: true })).toBeVisible()
    await expect(observer.getByRole('button', { name: 'Connected', exact: true })).toBeVisible()
    listener = await startTestListener(config)
    const current = () => {
      const audio = listener!.latest()
      if (audio.error) throw new Error(audio.error)
      return audio
    }
    const waitAudio = async (check: (audio: ListenerAudio) => boolean) => {
      try {
        await expect.poll(() => check(current()), { timeout: 45_000 }).toBe(true)
      } catch (error) {
        throw new Error(`Discord audio did not match: ${JSON.stringify(current())}`, { cause: error })
      }
    }
    await waitAudio(audio => audio.present)
    const lowTile = page.getByRole('group', { name: 'Discord Low', exact: true })
    await lowTile.getByRole('button', { name: 'Edit Discord Low', exact: true }).click()
    const volume = page.getByRole('dialog').getByRole('slider', { name: 'Volume for Discord Low' })
    await volume.press('Home')
    for (let step = 0; step < 25; step++) await volume.press('ArrowRight')
    await expect(volume).toHaveAttribute('aria-valuenow', '25')
    await page.getByRole('button', { name: 'Close track controls', exact: true }).click()
    const startingFrames = current().frames
    await page.getByRole('group', { name: 'Discord Low', exact: true }).locator('.track-heading > span').click()
    await page.getByRole('group', { name: 'Discord High', exact: true }).locator('.track-heading > span').click()
    await expect(page.getByRole('group', { name: 'Playing Discord Low', exact: true })).toBeVisible()
    await expect(page.getByRole('group', { name: 'Playing Discord High', exact: true })).toBeVisible()
    await waitAudio(audio => audio.frames >= startingFrames + 96000 &&
      audio.low > 0.01 && audio.low < 0.04 && audio.high > 0.05)
    const low = page.getByRole('group', { name: 'Playing Discord Low', exact: true })
    const high = page.getByRole('group', { name: 'Playing Discord High', exact: true })
    await low.getByRole('slider', { name: 'Volume for Discord Low' }).press('End')
    await waitAudio(audio => audio.low > 0.05 && audio.high > 0.05)
    await low.getByRole('slider', { name: 'Volume for Discord Low' }).press('Home')
    let frames = current().frames
    await waitAudio(audio => audio.frames >= frames + 96000 && audio.low < 0.003 && audio.high > 0.05)
    await high.getByRole('button', { name: 'Pause Discord High', exact: true }).click()
    frames = current().frames
    await waitAudio(audio => audio.present && (
      (audio.frames >= frames + 96000 && audio.rms < 0.003) ||
      (audio.last > 0 && Date.now() - audio.last > 2000)
    ))
    await page.getByRole('group', { name: 'Discord High', exact: true }).locator('.track-heading > span').click()
    frames = current().frames
    await waitAudio(audio => audio.frames >= frames + 96000 && audio.high > 0.05)
    await page.getByRole('group', { name: 'Discord Replacement', exact: true }).locator('.track-heading > span').click()
    frames = current().frames
    await waitAudio(audio => audio.frames >= frames + 96000 && audio.low > 0.05 && audio.high < 0.003)
    await expect(high).toHaveCount(0)
    await page.getByRole('button', { name: 'Connected', exact: true }).click()
    await expect(observer.getByRole('button', { name: 'Connect', exact: true })).toBeVisible()
    await waitAudio(audio => !audio.present)
    await chooseDiscordDestination(page, true)
    await expect(observer.getByRole('button', { name: 'Connected', exact: true })).toBeVisible()
    await waitAudio(audio => audio.present)
    frames = current().frames
    await waitAudio(audio => audio.frames >= frames + 96000 && audio.low > 0.05 && audio.high < 0.003)
    expect(observerStatusRequests).toBe(1)
    await observer.close()
    await page.goto('/tables')
    await expect.poll(async () => (await (await page.request.get(`/api/v1/tables/${tableID}/discord`)).json()).state)
      .toBe('disconnected')
    await waitAudio(audio => !audio.present)
    await page.getByRole('link', { name: 'Open Discord audio', exact: true }).click()
    await expect(page.getByRole('button', { name: 'Connect', exact: true })).toBeVisible()
    await expect.poll(async () => (await (await page.request.get(`/api/v1/tables/${tableID}/discord`)).json()).state)
      .toBe('disconnected')
    await chooseDiscordDestination(page, true)
    await expect(page.getByRole('button', { name: 'Connected', exact: true })).toBeVisible()
    await waitAudio(audio => audio.present)
    frames = current().frames
    await waitAudio(audio => audio.frames >= frames + 96000 && audio.low > 0.05 && audio.high < 0.003)
  } finally {
    await observer.close()
    listener?.close()
    const cleanup = await page.request.delete(`/api/v1/tables/${tableID}`)
    expect(cleanup.ok(), await cleanup.text()).toBe(true)
    const response = await page.request.get('/api/v1/files')
    expect(response.ok(), await response.text()).toBe(true)
    const tracks: { id: string, name: string }[] = await response.json()
    for (const track of tracks.filter(item =>
      ['Discord Low', 'Discord High', 'Discord Replacement'].includes(item.name))) {
      const deleted = await page.request.delete(`/api/v1/files/${track.id}`)
      expect(deleted.ok(), await deleted.text()).toBe(true)
    }
  }
})

async function chooseDiscordDestination(page: Page, saved = false) {
  const target = discordTestConfig(fileURLToPath(new URL('../../', import.meta.url)))
  const setupResponse = await page.request.get('/api/v1/discord/setup')
  expect(setupResponse.ok(), 'Discord setup request failed').toBe(true)
  const setup = await setupResponse.json()
  const guild = setup.guilds.find((item: { id: string }) => item.id === target.guildID)
  const channelsResponse = await page.request.get(`/api/v1/discord/servers/${target.guildID}/channels`)
    .catch(() => { throw new Error('Discord channel request failed') })
  expect(channelsResponse.ok(), 'Discord channel request failed').toBe(true)
  const channels = await channelsResponse.json()
  const channel = channels.find((item: { id: string }) => item.id === target.channelID)
  if (!guild || !channel) throw new Error('Isolated test server and voice channel must be installed and visible to the bot')
  const connectButton = page.getByRole('button', { name: 'Connect', exact: true })
  await expect(connectButton).toHaveClass(/text-grey/)
  await connectButton.click()
  const dialog = page.getByRole('dialog', { name: 'Discord setup' })
  await expect(dialog).toBeVisible()
  const refresh = dialog.getByRole('button', { name: 'Refresh Discord servers', exact: true })
  await expect(refresh).toBeVisible()
  const serverSelect = dialog.getByRole('combobox', { name: 'Discord server', exact: true })
  await expect(serverSelect).toBeEnabled()
  await refresh.click()
  await expect(serverSelect).toBeEnabled()
  await serverSelect.press('ArrowDown')
  const addServerOption = page.getByRole('option', { name: '+ Add to server', exact: true })
  await expect(addServerOption).toBeVisible()
  await expect(addServerOption).toHaveClass(/text-primary/)
  expect((await page.getByRole('option').allTextContents()).at(-1)?.trim()).toBe('+ Add to server')
  await serverSelect.press('Escape')
  if (!saved) {
    await serverSelect.press('ArrowDown')
    await page.getByRole('option', { name: guild.name, exact: true }).click()
    await dialog.getByRole('combobox', { name: 'Voice channel', exact: true }).press('ArrowDown')
    await page.getByRole('option', { name: channel.name, exact: true }).click()
  }
  await expect(dialog.getByRole('button', { name: 'Connect', exact: true })).toBeEnabled()
  const [response] = await Promise.all([
    page.waitForResponse(response => response.request().method() === 'POST' &&
      new URL(response.url()).pathname === `/api/v1/tables/${new URL(page.url()).pathname.split('/').pop()}/discord`,
    { timeout: 30_000 }),
    dialog.getByRole('button', { name: 'Connect', exact: true }).click(),
  ])
  expect(response.status(), 'Discord connect HTTP status').toBe(200)
  await expect(dialog).toBeHidden()
  const disconnectButton = page.getByRole('button', { name: 'Connected', exact: true })
  await expect(disconnectButton).toBeVisible()
  await expect(disconnectButton).toHaveClass(/text-success/)
  await expect.poll(async () => {
    const sidebar = await page.getByRole('complementary', { name: 'Currently playing' }).boundingBox()
    const button = await disconnectButton.boundingBox()
    return sidebar && button && Math.abs(sidebar.y + sidebar.height - button.y - button.height - 8) < 1
  }).toBe(true)
  await expect(page.getByText(`${guild.name} / ${channel.name}`, { exact: true })).toHaveCount(0)
  await disconnectButton.hover()
  await expect(page.getByRole('tooltip')).toContainText(`${guild.name} / ${channel.name}`)
}
