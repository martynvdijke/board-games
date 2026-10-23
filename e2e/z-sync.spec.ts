import { expect, test } from '@playwright/test';
import { signIn } from './helpers';

test('manual BGG sync runs against the fake API and reports progress', async ({ page }) => {
  await signIn(page);
  await page.goto('/settings');
  await page.getByRole('button', { name: 'Sync now' }).click();
  await expect(page.locator('[data-sync-pill]')).toHaveText(/success/i, { timeout: 20_000 });
  await expect(page.locator('[data-sync-detail]')).toContainText('3 collection');
});
