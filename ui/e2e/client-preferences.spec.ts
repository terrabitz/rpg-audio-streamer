import { expect, test } from './fixtures'
import { login } from './helpers'

test('GM keeps the mobile screen awake with a browser-local preference', async ({ page, browser }, testInfo) => {
  await page.context().grantPermissions(['screen-wake-lock'])
  await page.setViewportSize({ width: 390, height: 740 })
  await login(page)
  const nativeLock = await page.evaluateHandle(() => {
    const observed: { lock?: WakeLockSentinel } = {}
    const request = navigator.wakeLock.request.bind(navigator.wakeLock)
    navigator.wakeLock.request = async type => {
      observed.lock = await request(type)
      return observed.lock
    }
    return observed
  })
  const options = page.getByRole('button', { name: 'Table options', exact: true })
  const toggle = page.getByRole('menuitemcheckbox', { name: 'Keep screen awake', exact: true })
  const rotate = page.getByRole('menuitem', { name: 'Rotate invite link', exact: true })
  await options.click()
  await expect(toggle).toHaveAttribute('aria-checked', 'false')
  await toggle.click()
  await expect(toggle).toBeHidden()
  await expect.poll(() => nativeLock.evaluate(value => value.lock?.released)).toBe(false)
  await options.click()
  await expect(toggle).toBeVisible()
  await expect(toggle).toHaveAttribute('aria-checked', 'true')
  await page.screenshot({ path: testInfo.outputPath('keep-screen-awake.png'), fullPage: true, animations: 'disabled' })
  await page.keyboard.press('Escape')
  await expect(toggle).toBeHidden()
  await page.setViewportSize({ width: 1280, height: 900 })
  await expect.poll(() => nativeLock.evaluate(value => value.lock?.released)).toBe(true)
  await nativeLock.dispose()
  await page.setViewportSize({ width: 390, height: 740 })
  await page.reload()
  await options.click()
  await expect(toggle).toBeVisible()
  await expect(toggle).toHaveAttribute('aria-checked', 'true')
  await page.keyboard.press('Escape')
  await expect(toggle).toBeHidden()

  const otherClient = await browser.newContext()
  try {
    const otherPage = await otherClient.newPage()
    await otherPage.setViewportSize({ width: 390, height: 740 })
    await login(otherPage)
    await otherPage.getByRole('button', { name: 'Table options', exact: true }).click()
    await expect(otherPage.getByRole('menuitemcheckbox', { name: 'Keep screen awake', exact: true }))
      .toHaveAttribute('aria-checked', 'false')
  } finally {
    await otherClient.close()
  }

  await page.setViewportSize({ width: 1280, height: 900 })
  await options.click()
  await expect(rotate).toBeVisible()
  await expect(toggle).toHaveCount(0)
  await page.keyboard.press('Escape')
  await expect(rotate).toBeHidden()
  await page.setViewportSize({ width: 390, height: 740 })
  await options.click()
  await toggle.click()
  await expect(toggle).toBeHidden()
  await options.click()
  await expect(toggle).toBeVisible()
  await expect(toggle).toHaveAttribute('aria-checked', 'false')
})
