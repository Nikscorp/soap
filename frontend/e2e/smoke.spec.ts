import { expect, test } from '@playwright/test';

test('search → select → see best episodes', async ({ page }) => {
  await page.goto('/');

  await expect(page.getByRole('heading', { name: /lazy soap/i })).toBeVisible();

  const input = page.getByRole('combobox');
  await input.fill('lost');

  // The Vite preview build proxies via Caddy/Go; in CI we run against the Go
  // backend which serves static assets and the API on the same origin.
  const firstOption = page.getByRole('option').first();
  await expect(firstOption).toBeVisible({ timeout: 10_000 });
  await firstOption.click();

  await expect(page.getByText(/best of/i)).toBeVisible({ timeout: 15_000 });
});
