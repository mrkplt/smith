import { expect, test } from '@playwright/test';
import { mockEventSource, mockApiRoutes, emitLoopUpdates, loopsFixture } from './helpers.js';

async function setStateFilter(page, value) {
  const select = page.locator('select').first();
  if (await select.count()) {
    await select.selectOption(value);
    return;
  }

  await page.getByTestId('pods-state-filter-trigger').click();
  await page.getByRole('button', { name: 'Clear' }).click();
  await page.getByRole('menuitemcheckbox', { name: new RegExp(`^${value}$`, 'i') }).click();
  await page.keyboard.press('Escape');
}

test.describe('Modular Console', () => {
  test.beforeEach(async ({ page }) => {
    await mockEventSource(page);
    await mockApiRoutes(page);
    await page.goto('/pods');
    await emitLoopUpdates(page, loopsFixture);
    // Wait for seeded loops to render
    await expect(page.getByRole('button', { name: /loop-alpha/i })).toBeVisible();
  });

  test('should render loops and stats', async ({ page }) => {
    await expect(page.getByText('Total Pods')).toBeVisible();
    await expect(page.getByText('Active Loops')).toBeVisible();
    await expect(page.locator('.pods-summary').getByText('Flatline')).toBeVisible();
    await expect(page.getByRole('button', { name: /loop-alpha/i })).toBeVisible();
    await expect(page.getByRole('button', { name: /loop-beta/i })).toBeVisible();
  });

  test('should filter loops by state', async ({ page }) => {
    await setStateFilter(page, 'flatline');
    await expect(page.getByRole('button', { name: /loop-gamma/i })).toBeVisible();
    await expect(page.getByRole('button', { name: /loop-alpha/i })).toHaveCount(0);
  });

  test('should filter loops by search', async ({ page }) => {
    await page.getByPlaceholder(/Filter pods/i).fill('loop-beta');
    await expect(page.getByRole('button', { name: /loop-beta/i })).toBeVisible();
    await expect(page.getByRole('button', { name: /loop-alpha/i })).toHaveCount(0);
  });

  test('should navigate to documents page', async ({ page }) => {
    // Click Documents link in the top navbar (visible on desktop)
    await page.locator('a[href="/documents"]').first().click();
    await expect(page).toHaveURL(/\/documents/);
  });
});
