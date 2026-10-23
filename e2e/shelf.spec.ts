import { expect, test } from '@playwright/test';
import { openGame, signIn, today } from './helpers';

test.beforeEach(async ({ page }) => {
  await signIn(page);
});

test('overview greets the owner, shows stats and the daily pick', async ({ page }) => {
  await expect(page.getByRole('heading', { level: 1 })).toContainText('Your shelf');
  await expect(page.getByText('12 games', { exact: false }).first()).toBeVisible();
  await expect(page.getByText('On the table', { exact: false })).toBeVisible();
  await expect(page.locator('.game-card')).toHaveCount(12);
});

test('live search narrows the shelf and updates the URL', async ({ page }) => {
  const search = page.getByPlaceholder('Search games, designers, mechanics…');
  await search.fill('Wingspan');
  await expect(page.locator('#result-count')).toHaveText('1 game matching');
  await expect(page.locator('.game-card')).toHaveCount(1);
  await expect(page).toHaveURL(/q=Wingspan/);
  await expect(page.locator('.game-card-link').filter({ hasText: 'Wingspan' })).toBeVisible();
});

test('unplayed filter and list density work', async ({ page }) => {
  await page.getByRole('button', { name: 'Unplayed' }).click();
  await expect(page).toHaveURL(/status=unplayed/);
  await expect(page.locator('#result-count')).toContainText('matching');
  await page.getByRole('link', { name: 'List view' }).click();
  await expect(page.locator('.game-row').first()).toBeVisible();
});

test('sort menu orders the collection', async ({ page }) => {
  await page.locator('select[name="sort"]').selectOption('rating');
  await expect(page).toHaveURL(/sort=rating/);
  await expect(page.locator('.game-card').first()).toContainText('Ark Nova');
});

test('game detail shows metadata and saves shelf details', async ({ page }) => {
  await openGame(page, 'Wingspan');
  await expect(page.getByText('About the game')).toBeVisible();
  await page.getByLabel('Location').fill('Living room shelf');
  await page.getByLabel('Private notes').fill('The gateway game that stuck.');
  await page.getByRole('button', { name: 'Save shelf details' }).click();
  await expect(page.getByText('Shelf details saved')).toBeVisible();
  await page.reload();
  await expect(page.getByLabel('Location')).toHaveValue('Living room shelf');
  await expect(page.getByLabel('Private notes')).toHaveValue('The gateway game that stuck.');
});

test('log, edit, and delete a local play', async ({ page }) => {
  await page.goto('/plays/new');
  await page.getByLabel('Game').selectOption({ label: 'Wingspan' });
  await page.getByLabel('Date played').fill(today());
  await page.getByLabel('Location').fill('The kitchen table');
  await page.getByLabel('Duration (minutes)').fill('55');
  const firstPlayer = page.locator('[data-player-row]').first();
  await firstPlayer.getByPlaceholder('Player name').fill('Alex');
  await firstPlayer.getByPlaceholder('Score').fill('42');
  await firstPlayer.getByLabel('Won').check();
  await page.getByLabel('Notes').fill('A tidy first play.');
  await page.getByRole('button', { name: 'Log this play' }).click();

  await expect(page).toHaveURL(/\/plays/);
  const entry = page.locator('.play-entry', { hasText: 'A tidy first play.' });
  await expect(entry).toBeVisible();
  await entry.getByRole('link', { name: 'Edit play' }).click();
  await page.getByLabel('Notes').fill('A tidy first play, edited.');
  await page.getByRole('button', { name: 'Save changes' }).click();
  await expect(page.getByText('A tidy first play, edited.')).toBeVisible();

  page.once('dialog', (dialog) => dialog.accept());
  const edited = page.locator('.play-entry', { hasText: 'A tidy first play, edited.' });
  await edited.getByRole('button', { name: 'Delete play' }).click();
  await expect(page.getByText('Play deleted')).toBeVisible();
  await expect(page.locator('.play-entry', { hasText: 'A tidy first play, edited.' })).toHaveCount(0);
});

test('stats page renders charts and the shelf of shame', async ({ page }) => {
  await page.goto('/stats');
  await expect(page.getByRole('heading', { level: 1 })).toContainText('in review');
  await expect(page.getByText('Shelf of shame', { exact: true }).first()).toBeVisible();
  await expect(page.locator('.bar-column').first()).toBeVisible();
});

test('token creation exposes a working bearer token', async ({ page, request }) => {
  await page.goto('/settings');
  await page.getByLabel('Token name').fill('Playwright token');
  await page.getByLabel('Read').check();
  await page.getByRole('button', { name: 'Create token' }).click();
  const token = await page.locator('input[data-copy-value]').inputValue();
  expect(token).toMatch(/^bg_/);

  const me = await request.get('/api/v1/me', { headers: { Authorization: `Bearer ${token}` } });
  expect(me.ok()).toBeTruthy();
  const body = await me.json();
  expect(body.data.email).toBe('owner@example.com');

  const games = await request.get('/api/v1/games?limit=5', { headers: { Authorization: `Bearer ${token}` } });
  expect(games.ok()).toBeTruthy();
  const pageBody = await games.json();
  expect(pageBody.pagination.limit).toBe(5);
  expect(pageBody.data.length).toBe(5);
});

test('TRMNL preview is exactly 800x480 and feed is token scoped', async ({ page, request }) => {
  await page.goto('/trmnl/preview');
  const frame = page.locator('.trmnl-frame');
  await expect(frame).toBeVisible();
  const box = await frame.boundingBox();
  expect(Math.round(box!.width)).toBe(800);
  expect(Math.round(box!.height)).toBe(480);

  await page.goto('/settings');
  await page.getByLabel('Token name').fill('TRMNL device');
  await page.getByRole('checkbox', { name: 'TRMNL' }).check();
  await page.getByRole('button', { name: 'Create token' }).click();
  const token = await page.locator('input[data-copy-value]').inputValue();
  const feed = await request.get('/api/v1/trmnl', { headers: { Authorization: `Bearer ${token}` } });
  expect(feed.ok()).toBeTruthy();
  const payload = await feed.json();
  expect(payload.date).toMatch(/^\d{4}-\d{2}-\d{2}$/);
  expect(payload.pick.name).toBeTruthy();
  expect(payload.shelf_of_shame.count).toBeGreaterThanOrEqual(0);
});
