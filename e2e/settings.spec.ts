import { expect, test } from '@playwright/test';
import { signIn } from './helpers';

test.beforeEach(async ({ page }) => {
  await signIn(page);
});

test('admin can save and remove the instance BGG API token', async ({ page }) => {
  await page.goto('/settings');

  await expect(page.getByRole('heading', { name: 'BoardGameGeek API' })).toBeVisible();
  await expect(page.getByText('Not configured', { exact: true })).toBeVisible();

  await page.getByLabel('API token', { exact: true }).fill('e2e-instance-token');
  await page.getByRole('button', { name: 'Save token' }).click();

  await expect(page.getByText('BGG API token saved', { exact: false })).toBeVisible();
  await expect(page.getByText('Saved in this instance', { exact: true })).toBeVisible();
  await expect(page.getByRole('button', { name: 'Remove saved token' })).toBeVisible();

  await page.getByRole('button', { name: 'Remove saved token' }).click();

  await expect(page.getByText('BGG API token removed', { exact: true })).toBeVisible();
  await expect(page.getByText('Not configured', { exact: true })).toBeVisible();
});
