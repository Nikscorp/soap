import { expect, test, type Page } from '@playwright/test';

const counterRegex = /^(\d+)\s+of\s+(\d+)$/;

async function openLost(page: Page) {
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
  await expect(page.getByRole('slider')).toBeVisible({ timeout: 15_000 });
}

async function readSliderTotal(page: Page): Promise<number> {
  const counter = page.getByText(counterRegex).first();
  const text = (await counter.textContent()) ?? '';
  const match = counterRegex.exec(text.trim());
  expect(match, `slider counter "${text}" did not match "X of Y"`).not.toBeNull();
  return Number(match![2]);
}

test('search → select → see best episodes → slider trims the list', async ({ page }) => {
  await openLost(page);
  const slider = page.getByRole('slider');

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

test('season selector filters episodes and restores on "All"', async ({ page }) => {
  await openLost(page);

  const originalTotal = await readSliderTotal(page);
  expect(originalTotal).toBeGreaterThan(1);

  const group = page.getByRole('group', { name: /filter seasons/i });
  await expect(group).toBeVisible({ timeout: 5_000 });

  // Click the first non-"All" chip; in all-mode this selects ONLY that season.
  const seasonChips = group.getByRole('button').filter({ hasText: /^S\d+$/ });
  await expect(seasonChips.first()).toBeVisible({ timeout: 5_000 });
  const firstSeasonLabel = ((await seasonChips.first().textContent()) ?? '').trim();
  const seasonMatch = /^S(\d+)$/.exec(firstSeasonLabel);
  expect(seasonMatch, `chip label "${firstSeasonLabel}" was not "S<n>"`).not.toBeNull();
  const selectedSeason = Number(seasonMatch![1]);

  await seasonChips.first().click();
  await expect(seasonChips.first()).toHaveAttribute('aria-pressed', 'true');
  await expect(page).toHaveURL(new RegExp(`seasons=${selectedSeason}(?:&|$)`));

  // After refetch, the slider's "Y" should drop because totalEpisodes is
  // recomputed over the filtered subset.
  await expect
    .poll(() => readSliderTotal(page), { timeout: 15_000 })
    .toBeLessThan(originalTotal);

  // Every rendered episode row should belong to the selected season.
  const list = page.getByRole('list').last();
  const items = list.getByRole('listitem');
  await expect(items.first()).toBeVisible({ timeout: 5_000 });
  const codes = await items.allInnerTexts();
  expect(codes.length).toBeGreaterThan(0);
  const selectedCodeRe = new RegExp(`\\bS${selectedSeason}E\\d+\\b`);
  for (const code of codes) {
    expect(code, `episode row was not from selected season ${selectedSeason}`).toMatch(
      selectedCodeRe,
    );
  }

  // Click "All" — original total returns, seasons param drops, "All" chip pressed.
  const allChip = group.getByRole('button', { name: /^all$/i });
  await allChip.click();
  await expect(allChip).toHaveAttribute('aria-pressed', 'true');
  await expect(page).not.toHaveURL(/seasons=/);

  await expect
    .poll(() => readSliderTotal(page), { timeout: 15_000 })
    .toBe(originalTotal);
});
