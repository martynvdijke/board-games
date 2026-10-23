import { expect, type Page } from '@playwright/test';

export async function signIn(page: Page) {
  await page.goto('/auth/login');
  await expect(page).toHaveURL(/\/$/);
  await expect(page.getByRole('heading', { level: 1 })).toContainText('Your shelf');
}

export async function openGame(page: Page, name: string) {
  await page.locator('.game-card-link').filter({ hasText: name }).first().click();
  await expect(page.getByRole('heading', { level: 1 })).toContainText(name);
}

export function today(): string {
  return new Date().toISOString().slice(0, 10);
}
