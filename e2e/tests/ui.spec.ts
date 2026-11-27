import { test, expect, type Page } from '@playwright/test';

// App runs with DEV_MODE=true: auth bypassed, Deluge/Plex mocked (cmd/webui-be/dev_mocks.go).

async function openDashboard(page: Page) {
  await page.goto('/');
  await expect(page.locator('#torrent-list table')).toBeVisible();
}

async function selectAndProcess(page: Page, ids: string[]) {
  for (const id of ids) {
    await page.locator(`#torrent-list input[type=checkbox][value="${id}"]`).check();
  }
  const processBtn = page.getByRole('button', { name: /Process with FileBot/ });
  await expect(processBtn).toContainText(`(${ids.length})`);
  await processBtn.click();
  await expect(page.locator('#filebot-slot').getByText(`${ids.length} torrent(s)`)).toBeVisible();
}

// Typewriter demo animates via JS timers.
const volatile = (page: Page) => [page.locator('.term-box')];

test('login page', async ({ page }) => {
  await page.goto('/login');
  await expect(page).toHaveScreenshot('login.png', { fullPage: true });
});

test('dashboard lists completed torrents', async ({ page }) => {
  await openDashboard(page);
  await expect(page.locator('#torrent-list')).toMatchAriaSnapshot({ name: 'torrent-list.aria.yml' });
  await expect(page).toHaveScreenshot('dashboard.png', { fullPage: true, mask: volatile(page) });
});

test('selecting torrents opens the FileBot form', async ({ page }) => {
  await openDashboard(page);
  await selectAndProcess(page, ['abc123', 'ghi789']);

  const slot = page.locator('#filebot-slot');
  await expect(slot).toMatchAriaSnapshot({ name: 'filebot-form.aria.yml' });
  await expect(page).toHaveScreenshot('filebot-form.png', { fullPage: true, mask: volatile(page) });
});

test('executing a missing source renders the failure result', async ({ page }) => {
  await openDashboard(page);
  await selectAndProcess(page, ['abc123']);

  await page.getByRole('button', { name: /Execute/ }).click();

  const slot = page.locator('#filebot-slot');
  await expect(slot.getByText(/Torrent outcomes/i)).toBeVisible();
  const toast = page.locator('#toast-region .toast');
  await expect(toast).toContainText('FileBot failed');
  await toast.getByRole('button', { name: 'Dismiss' }).click();
  await expect(toast).toHaveCount(0);

  await expect(slot).toMatchAriaSnapshot({ name: 'filebot-result.aria.yml' });
  await expect(page).toHaveScreenshot('filebot-result.png', { fullPage: true, mask: volatile(page) });
});
