import AxeBuilder from '@axe-core/playwright';
import { expect, test } from '@playwright/test';
import { signIn } from './helpers';

test.beforeEach(async ({ page }) => {
  await signIn(page);
});

test('loans page opens on an empty state that points at the shelf', async ({ page }) => {
  await page.goto('/loans');

  await expect(page.getByRole('heading', { level: 1 })).toContainText('Loans');
  await expect(page.locator('.site-nav a[aria-current="page"]')).toContainText('Loans');
  await expect(page.getByText('Nothing is lent out')).toBeVisible();
  await expect(page.getByRole('link', { name: 'Browse the shelf' })).toHaveAttribute('href', '/');
});

test('lending a game shows it on the shelf and the loans page, and returning files it', async ({ page }) => {
  // Discovery only: seeded game ids are odd and non-contiguous, so the test
  // takes whatever game the owned shelf offers first.
  await page.goto('/?status=owned');
  const card = page.locator('.game-card-link').first();
  const name = (await card.locator('.game-name').innerText()).trim();
  await card.click();
  await expect(page.getByRole('heading', { level: 1 })).toContainText(name);

  const form = page.locator('form.loan-form');
  await form.getByLabel('Borrower').fill('Sam Rivera');
  await form.getByLabel('Contact (optional)').fill('sam@example.com');
  await form.getByRole('button', { name: 'Lend this game' }).click();

  await expect(page.getByText('Lent to Sam Rivera')).toBeVisible();
  await expect(page.locator('.status-badge.is-out')).toContainText('On loan');
  await expect(page.locator('.loan-out-line')).toContainText('Sam Rivera');
  await expect(page.locator('form.loan-form')).toHaveCount(0);

  await page.goto('/loans');
  const openRow = page.locator('.loan-row').filter({ hasText: name });
  await expect(openRow).toBeVisible();
  await expect(openRow).toContainText('Sam Rivera');
  await expect(openRow).toContainText('No due date');

  await page.goto('/?status=owned&view=list');
  const listRow = page.locator('.game-row').filter({ hasText: name });
  await expect(listRow.locator('.game-row-loan')).toContainText('Sam Rivera');

  await page.goto('/?status=owned&view=grid');
  const gridCard = page.locator('.game-card').filter({ hasText: name });
  await expect(gridCard.locator('.loan-badge')).toBeVisible();

  await page.goto('/loans');
  await page.locator('.loan-row').filter({ hasText: name }).getByRole('button', { name: /Mark .* returned/ }).click();
  await expect(page.getByText(`${name} is back on the shelf`)).toBeVisible();
  await expect(page.getByText('Everything is back on the shelf.')).toBeVisible();

  const history = page.locator('.loan-history');
  await expect(history).toContainText(name);
  await expect(history).toContainText('Sam Rivera');
});

test('loans page has no serious accessibility violations', async ({ page }) => {
  await page.goto('/loans');
  const results = await new AxeBuilder({ page })
    .withTags(['wcag2a', 'wcag2aa', 'wcag21a', 'wcag21aa'])
    .analyze();
  const serious = results.violations.filter((v) => v.impact === 'serious' || v.impact === 'critical');
  expect(serious.map((v) => `${v.id}: ${v.help}`)).toEqual([]);
});
