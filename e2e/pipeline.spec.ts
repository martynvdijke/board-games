import { expect, test } from '@playwright/test';
import { signIn } from './helpers';

test.beforeEach(async ({ page }) => {
  await signIn(page);
});

test('pipeline shows the five acquisition states', async ({ page }) => {
  await page.goto('/pipeline');
  await expect(page.getByRole('heading', { level: 1 })).toContainText('The pipeline');

  const headings = page.locator('.pipeline-bucket .section-title');
  await expect(headings).toHaveCount(5);
  for (const label of ['Wishlist', 'Want to play', 'Preordered', 'For trade', 'Owned, never played']) {
    await expect(headings.filter({ hasText: label })).toHaveCount(1);
  }

  await expect(page.locator('.site-nav a[aria-current="page"]')).toContainText('Pipeline');
});

test('game detail shows a price row once a price is recorded', async ({ page }) => {
  await page.goto('/');
  await page.locator('.game-card-link').filter({ hasText: 'Cascadia' }).first().click();
  await expect(page.getByRole('heading', { level: 1 })).toContainText('Cascadia');

  await page.getByLabel('Price paid (£)').fill('35.50');
  await page.getByRole('button', { name: 'Save shelf details' }).click();
  await expect(page.getByText('Shelf details saved')).toBeVisible();

  const specs = page.locator('.spec-grid');
  await expect(specs.getByText('Price paid', { exact: true })).toBeVisible();
  await expect(specs).toContainText('£35.50');
  await expect(specs.getByText('Cost per play', { exact: true })).toBeVisible();
  await expect(specs.getByText('Cost per hour', { exact: true })).toBeVisible();
});
