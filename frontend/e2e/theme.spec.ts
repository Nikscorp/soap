import { expect, test } from '@playwright/test';

test.beforeEach(async ({ page }) => {
  await page.route('**/featured?*', (route) =>
    route.fulfill({
      json: { language: 'en', series: [] },
    }),
  );
  await page.route('**/meta', (route) =>
    route.fulfill({
      json: { ratingsSource: 'tmdb' },
    }),
  );
});

test('defaults to system and follows live appearance changes', async ({ page }) => {
  await page.emulateMedia({ colorScheme: 'dark' });
  await page.goto('/');
  await expect(page.getByRole('button', { name: 'System', exact: true })).toHaveAttribute(
    'aria-pressed',
    'true',
  );
  await expect(page.locator('body')).toHaveCSS('background-color', 'rgb(17, 19, 24)');
  await expect(page.locator('section').first()).toHaveCSS('background-color', 'rgb(28, 32, 39)');
  await expect(page.getByRole('combobox')).toHaveCSS('color', 'rgb(241, 245, 249)');

  await page.emulateMedia({ colorScheme: 'light' });
  await expect(page.locator('body')).toHaveCSS('background-color', 'rgb(241, 245, 249)');
  await expect(page.locator('section').first()).toHaveCSS('background-color', 'rgb(255, 255, 255)');
  await expect(page.locator('meta[name="theme-color"]')).toHaveAttribute('content', '#f1f5f9');
});

test('explicit themes persist, override system, and can return to system', async ({ page }) => {
  await page.emulateMedia({ colorScheme: 'dark' });
  await page.goto('/');
  await page.getByRole('button', { name: 'Light', exact: true }).click();
  await expect(page.locator('body')).toHaveCSS('background-color', 'rgb(241, 245, 249)');
  await page.reload();
  await expect(page.getByRole('button', { name: 'Light', exact: true })).toHaveAttribute(
    'aria-pressed',
    'true',
  );
  await expect(page.locator('body')).toHaveCSS('background-color', 'rgb(241, 245, 249)');

  await page.getByRole('button', { name: 'Dark', exact: true }).click();
  await page.emulateMedia({ colorScheme: 'light' });
  await expect(page.locator('body')).toHaveCSS('background-color', 'rgb(17, 19, 24)');
  await page.reload();
  await expect(page.getByRole('button', { name: 'Dark', exact: true })).toHaveAttribute(
    'aria-pressed',
    'true',
  );
  await page.getByRole('button', { name: 'System', exact: true }).click();
  await expect(page.locator('body')).toHaveCSS('background-color', 'rgb(241, 245, 249)');
  await page.reload();
  await expect(page.getByRole('button', { name: 'System', exact: true })).toHaveAttribute(
    'aria-pressed',
    'true',
  );
});

test('saved theme is applied before React starts', async ({ page }) => {
  await page.emulateMedia({ colorScheme: 'light' });
  await page.addInitScript(() => localStorage.setItem('lazysoap-theme', 'dark'));
  await page.route('**/assets/*.js', (route) => route.abort());
  await page.goto('/');
  await expect(page.locator('html')).toHaveAttribute('data-theme', 'dark');
  await expect(page.locator('body')).toHaveCSS('background-color', 'rgb(17, 19, 24)');
});

test('unavailable storage does not prevent changing themes', async ({ page }) => {
  await page.emulateMedia({ colorScheme: 'light' });
  await page.addInitScript(() => {
    Object.defineProperty(window, 'localStorage', {
      get() {
        throw new Error('Storage unavailable');
      },
    });
  });
  await page.goto('/');
  await expect(page.getByRole('button', { name: 'System', exact: true })).toHaveAttribute(
    'aria-pressed',
    'true',
  );
  await page.getByRole('button', { name: 'Dark', exact: true }).click();
  await expect(page.locator('body')).toHaveCSS('background-color', 'rgb(17, 19, 24)');
});
