import AxeBuilder from '@axe-core/playwright';
import { expect, test } from '@playwright/test';
import { openGame, signIn } from './helpers';

const pages = ['/', '/plays', '/stats', '/settings', '/trmnl/preview'];

for (const path of pages) {
  test(`no serious accessibility violations on ${path}`, async ({ page }) => {
    await signIn(page);
    await page.goto(path);
    const results = await new AxeBuilder({ page })
      .withTags(['wcag2a', 'wcag2aa', 'wcag21a', 'wcag21aa'])
      .analyze();
    const serious = results.violations.filter((v) => v.impact === 'serious' || v.impact === 'critical');
    expect(serious.map((v) => `${v.id}: ${v.help}`)).toEqual([]);
  });
}

test('game detail has no serious accessibility violations', async ({ page }) => {
  await signIn(page);
  await openGame(page, 'Wingspan');
  const results = await new AxeBuilder({ page })
    .withTags(['wcag2a', 'wcag2aa', 'wcag21a', 'wcag21aa'])
    .analyze();
  const serious = results.violations.filter((v) => v.impact === 'serious' || v.impact === 'critical');
  expect(serious.map((v) => `${v.id}: ${v.help}`)).toEqual([]);
});
