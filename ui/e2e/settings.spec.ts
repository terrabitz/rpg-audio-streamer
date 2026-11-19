import { expect, test } from './fixtures'
import { login, navigate } from './helpers'
import type { Page } from '@playwright/test'

async function claim(page: Page, code: string) {
  await page.goto('/')
  await expect(page.getByRole('heading', { name: 'Claim your instance' })).toBeVisible()
  await page.getByLabel('Setup code').fill(code)
  await page.getByLabel('Username', { exact: true }).fill('owner')
  await page.getByLabel('New password', { exact: true }).fill('a secure owner password')
  await page.getByLabel('Confirm password', { exact: true }).fill('a secure owner password')
  await page.getByRole('button', { name: 'Create owner account' }).click()
  await expect(page.getByText('Account created.', { exact: false })).toBeVisible()
  await page.getByRole('link', { name: 'Sign in', exact: true }).click()
  await expect(page.locator('.v-toolbar-title')).toHaveText('Login')
  await page.getByLabel('Username', { exact: true }).fill('owner')
  await page.getByLabel('Password', { exact: true }).fill('a secure owner password')
  await page.getByRole('button', { name: 'Login', exact: true }).click()
  await expect(page.getByRole('heading', { name: 'Tables', exact: true })).toBeVisible()
  expect((await page.context().cookies()).find(cookie => cookie.name === 'auth_token')?.secure).toBe(true)
}

test.describe('fresh instance', () => {
  test.use({ firstRun: true })

  test('Owner claims a fresh instance with a one-time setup code', async ({ page, app, recordFlow }) => {
    await page.goto('/')
    await expect(page.getByRole('heading', { name: 'Claim your instance' })).toBeVisible()
    await page.getByLabel('Setup code').fill('incorrect-code')
    await page.getByLabel('New password', { exact: true }).fill('a secure owner password')
    await page.getByLabel('Confirm password', { exact: true }).fill('a secure owner password')
    await page.getByRole('button', { name: 'Create owner account' }).click()
    await expect(page.getByText('invalid or expired setup code', { exact: false })).toBeVisible()
    recordFlow.start()
    await claim(page, app.setupCode())
    recordFlow.stop()
    await page.reload()
    await expect(page.getByRole('heading', { name: 'Tables', exact: true })).toBeVisible()
    await page.goto('/setup')
    await expect(page).toHaveURL(/\/login$/)
  })

  test('Owner changes their password from profile settings', async ({ page, app, browser, recordFlow }) => {
    await claim(page, app.setupCode())
    await navigate(page, 'Settings')
    recordFlow.start()
    await page.getByLabel('Current password').fill('incorrect-password')
    await page.getByLabel('New password', { exact: true }).fill('a changed owner password')
    await page.getByLabel('Confirm password', { exact: true }).fill('a changed owner password')
    await page.getByRole('button', { name: 'Change password', exact: true }).click()
    await expect(page.getByText('current password is incorrect', { exact: false })).toBeVisible()
    await page.getByLabel('Current password').fill('a secure owner password')
    await page.getByRole('button', { name: 'Change password', exact: true }).click()
    await expect(page.getByText('Password changed.', { exact: false })).toBeVisible()
    recordFlow.stop()
    await page.reload()
    await expect(page.getByLabel('Current password')).toBeVisible()
    await expect(page).toHaveURL(/\/settings$/)

    const otherContext = await browser.newContext()
    try {
      const other = await otherContext.newPage()
      await other.goto(`${app.url}/login`)
      await other.getByLabel('Username', { exact: true }).fill('owner')
      await other.getByLabel('Password', { exact: true }).fill('a secure owner password')
      await other.getByRole('button', { name: 'Login', exact: true }).click()
      await expect(other.getByText('Invalid credentials', { exact: true })).toBeVisible()
      await other.getByLabel('Password', { exact: true }).fill('a changed owner password')
      await other.getByRole('button', { name: 'Login', exact: true }).click()
      await expect(other.getByRole('heading', { name: 'Tables', exact: true })).toBeVisible()
    } finally {
      await otherContext.close()
    }
  })
})

test('A bootstrapped admin can change their password in Settings', async ({ page, recordFlow }) => {
  await login(page)
  await navigate(page, 'Settings')
  recordFlow.start()
  await page.getByLabel('Current password').fill('e2e-password')
  await page.getByLabel('New password', { exact: true }).fill('a changed bootstrap password')
  await page.getByLabel('Confirm password', { exact: true }).fill('a changed bootstrap password')
  await page.getByRole('button', { name: 'Change password', exact: true }).click()
  await expect(page.getByText('Password changed.', { exact: false })).toBeVisible()
  recordFlow.stop()
  await page.reload()
  await expect(page.getByLabel('Current password')).toBeVisible()
})
