import AxeBuilder from '@axe-core/playwright';
import { expect, test } from '@playwright/test';
import { signIn } from './helpers';

const year = new Date().getFullYear();
const axeTags = ['wcag2a', 'wcag2aa', 'wcag21a', 'wcag21aa'];

function seriousViolations(results: { violations: { impact?: string | null; id: string; help: string }[] }) {
  return results.violations
    .filter((v) => v.impact === 'serious' || v.impact === 'critical')
    .map((v) => `${v.id}: ${v.help}`);
}

test('the current year recap is reachable from stats and shows its totals', async ({ page }) => {
  await signIn(page);

  await page.goto('/stats');
  await page.getByRole('link', { name: 'Year in review' }).click();

  await expect(page).toHaveURL(new RegExp(`/year/${year}$`));
  await expect(page.getByRole('heading', { level: 1 })).toContainText(`${year} in review`);
  await expect(page.getByText('The year in numbers')).toBeVisible();
  await expect(page.locator('.year-figure-value').first()).toHaveText(/^\d+$/);
  await expect(page.getByText('hours at the table')).toBeVisible();
  await expect(page.getByText('Most played')).toBeVisible();

  const results = await new AxeBuilder({ page }).withTags(axeTags).analyze();
  expect(seriousViolations(results)).toEqual([]);
});

// The data layer answers not found for a year outside the user's range (after
// the current year, or before their first play), so 2001 must produce the
// app's 404 page rather than an empty recap. The sparse year-in-range variant
// needs a gap year in the play log, which the seed data does not contain.
test('a year with no plays answers not found instead of an empty recap', async ({ page }) => {
  await signIn(page);
  const response = await page.goto('/year/2001');

  expect(response?.status()).toBe(404);
  await expect(page.getByRole('heading', { level: 1 })).toContainText("That year isn't here");
  await expect(page.locator('.year-letter')).toHaveCount(0);
});

test('a public link excludes player names, and revoking it makes it vanish', async ({ page, browser }) => {
  await signIn(page);

  // The owner's own page names the nemesis, so the public check below is real.
  await page.goto(`/year/${year}`);
  await expect(page.locator('.year-letter')).toContainText('Alex');

  await page.goto('/settings');
  const panel = page.locator('#year-review');
  await expect(panel.getByRole('heading', { name: 'Year in review' })).toBeVisible();

  await panel.locator('select[name="year"]').selectOption(String(year));
  const names = panel.locator('input[name="include_players"]');
  await expect(names).not.toBeChecked();
  await expect(panel.getByText('Include player names')).toBeVisible();
  await panel.getByRole('button', { name: 'Preview the page' }).click();

  await expect(page).toHaveURL(new RegExp(`/settings/year/preview\\?year=${year}`));
  await expect(page.getByText('Nothing here is public yet')).toBeVisible();
  await expect(page.locator('.year-letter')).not.toContainText('Alex');
  await expect(page.locator('.year-letter')).toContainText('One opponent');

  await page.getByRole('button', { name: 'Create the public link' }).click();
  await expect(page).toHaveURL(/\/settings\?flash=Public\+link\+ready/);
  await expect(page.getByText('Public link ready')).toBeVisible();

  const shareUrl = await panel.locator('input[data-share-url]').first().inputValue();
  expect(shareUrl).toContain('/s/');

  // A visitor with no cookies at all.
  const guestContext = await browser.newContext();
  const guest = await guestContext.newPage();
  const served = await guest.goto(shareUrl);
  expect(served?.status()).toBe(200);
  await expect(guest.getByRole('heading', { level: 1 })).toContainText(`${year} in review`);
  await expect(guest.locator('body')).not.toContainText('Alex');
  await expect(guest.getByText('One opponent')).toBeVisible();
  // No navigation into the private app, no session, no index, no third party.
  await expect(guest.locator('.site-header')).toHaveCount(0);
  await expect(guest.locator('.site-nav')).toHaveCount(0);
  await expect(guest.locator('.mobile-nav')).toHaveCount(0);
  await expect(guest.locator('a[href^="/plays"], a[href^="/stats"], a[href^="/settings"], a[href^="/pipeline"], a[href^="/loans"]')).toHaveCount(0);
  await expect(guest.locator('meta[name="robots"][content="noindex"]')).toHaveCount(1);
  await expect(guest.locator('meta[name="csrf-token"]')).toHaveCount(0);
  await expect(guest.locator('script[src*="umami"]')).toHaveCount(0);
  expect(await guestContext.cookies()).toEqual([]);

  const results = await new AxeBuilder({ page: guest }).withTags(axeTags).analyze();
  expect(seriousViolations(results)).toEqual([]);

  // Revoke from the signed-in page and check the same URL is gone.
  page.once('dialog', (dialog) => dialog.accept());
  await panel.locator('.share-row').first().getByRole('button', { name: 'Revoke' }).click();
  await expect(page.getByText('Link revoked')).toBeVisible();
  await expect(panel.locator('input[data-share-url]')).toHaveCount(0);

  const gone = await guest.goto(shareUrl);
  expect(gone?.status()).toBe(404);
  await expect(guest.getByRole('heading', { level: 1 })).toContainText("That recap isn't here");

  await guestContext.close();
});
