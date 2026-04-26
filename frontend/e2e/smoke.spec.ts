import { expect, test } from '@playwright/test';

test('search → select → see best episodes → slider trims the list', async ({ page }) => {
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

  const slider = page.getByRole('slider');
  await expect(slider).toBeVisible({ timeout: 15_000 });

  // Force the slider to its minimum and verify exactly one episode row remains.
  // We bypass the visual drag because Playwright's mouse drag on a range input
  // is flaky; setting `value` and dispatching the right events is enough for
  // React's onChange handler.
  await slider.evaluate((el) => {
    const input = el as HTMLInputElement;
    const setter = Object.getOwnPropertyDescriptor(
      window.HTMLInputElement.prototype,
      'value',
    )?.set;
    setter?.call(input, '1');
    input.dispatchEvent(new Event('input', { bubbles: true }));
    input.dispatchEvent(new Event('change', { bubbles: true }));
  });
  await expect(slider).toHaveAttribute('aria-valuenow', '1');

  const list = page.getByRole('list').last();
  await expect(list.getByRole('listitem')).toHaveCount(1, { timeout: 5_000 });
});
