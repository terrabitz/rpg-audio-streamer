import { mkdir, mkdtemp, rm, writeFile } from 'node:fs/promises'
import { tmpdir } from 'node:os'
import { join } from 'node:path'
import { expect, test } from './fixtures'
import { login, deleteLibraryTrack, uploadTrack, openUploadDialog, navigate, audioFixture, dropAudioPaths } from './helpers'

test('GM selects a required track type and receives a bottom upload toast on mobile', async ({ page }) => {
  await page.setViewportSize({ width: 375, height: 812 })
  await login(page)
  await navigate(page, 'Library')
  let uploads = 0
  page.on('request', request => {
    if (request.method() === 'POST' && new URL(request.url()).pathname === '/api/v1/files') uploads += 1
  })
  await uploadTrack(page, 'Mobile Forest')
  const toast = page.getByRole('status').filter({ hasText: 'Track added' })
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
  expect(uploads).toBe(1)
  await expect(page.getByRole('status').filter({ hasText: 'Track added' })).toBeHidden()
  await openUploadDialog(page)
  await expect(page.getByRole('combobox', { name: 'Track Type', exact: true })).toHaveValue('')
  await expect(page.getByText('Select a track type before uploading.', { exact: true })).toHaveCount(0)
  await expect(page.getByRole('button', { name: 'Upload', exact: true })).toBeDisabled()
  await page.getByRole('button', { name: 'Cancel', exact: true }).click()
  await deleteLibraryTrack(page, 'Mobile Forest', 0)
})

test('GM uploads, renames, persists, and deletes an audio track', async ({ page }) => {
  await login(page)
  await navigate(page, 'Library')
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
  await expect(page.locator('video')).toHaveCount(1)
  await expect.poll(() => page.locator('video').evaluate((media: HTMLVideoElement) => media.paused)).toBe(true)
  await page.getByLabel('Track Name', { exact: true }).fill('Rain')
  await page.getByRole('combobox', { name: 'Track Type', exact: true }).press('ArrowDown')
  await page.getByRole('option', { name: 'One-Shot', exact: true }).click()
  await expect(page.getByRole('row').filter({ hasText: 'Rain' }).getByText('One-Shot', { exact: true })).toBeVisible()
  await expect(page.getByRole('dialog')).toBeVisible()
  await expect(page.getByRole('button', { name: 'Save Changes' })).toHaveCount(0)
  await page.getByRole('button', { name: 'Close track controls' }).click()
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

test('GM drops a folder to import nested audio into the library', async ({ page }) => {
  const folder = await mkdtemp(join(tmpdir(), 'rpg-audio-folder-drop-'))
  try {
    const subfolder = join(folder, 'Nested')
    await mkdir(subfolder)
    await writeFile(join(folder, 'Campfire.mp3'), audioFixture(440, 2))
    await writeFile(join(subfolder, 'Night Rain.mp3'), audioFixture(880, 2))
    await writeFile(join(folder, 'notes.txt'), 'Not audio')
    await login(page)
    await navigate(page, 'Library')
    await expect(page.getByRole('button', { name: 'Upload Track', exact: true })).toBeVisible()
    await dropAudioPaths(page, page.getByRole('row', { name: 'No tracks in the library yet.' }),
      [folder], 'Drop audio to upload to the library')

    const dialog = page.getByRole('dialog')
    const table = dialog.getByRole('table', { name: 'Tracks to upload' })
    for (const name of ['Campfire.mp3', 'Night Rain.mp3']) {
      await expect(table.getByText(name, { exact: true })).toBeVisible()
    }
    await expect(dialog.getByText('Skipped 1 non-audio or unreadable item.')).toBeVisible()
    const upload = dialog.getByRole('button', { name: 'Upload 2 tracks', exact: true })
    await expect(upload).toBeDisabled()
    await table.getByRole('checkbox', { name: 'Select all tracks' }).check()
    const bulkActions = dialog.getByRole('group', { name: 'Bulk track actions' })
    await expect(bulkActions.getByRole('status')).toHaveText('2 selected')
    await bulkActions.getByRole('button', { name: 'Set track type' }).click()
    await page.getByRole('menuitem', { name: 'Music', exact: true }).click()
    await upload.click()
    await expect(page.getByRole('status').filter({ hasText: '2 tracks added' })).toBeVisible()
    for (const name of ['Campfire', 'Night Rain']) {
      const row = page.getByRole('row').filter({ hasText: name })
      await expect(row).toHaveAttribute('aria-busy', 'false')
      await expect(row.getByText('Music', { exact: true })).toBeVisible()
    }
  } finally {
    await rm(folder, { recursive: true, force: true })
  }
})
