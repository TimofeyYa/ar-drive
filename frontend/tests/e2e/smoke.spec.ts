import { test, expect } from '@playwright/test';

test('home redirects to /setup when not authenticated', async ({ page }) => {
  await page.goto('/');
  await expect(page).toHaveURL(/\/setup$/);
  await expect(page.getByText(/Cloud\.ru Artifact Registry|Artifact Registry/)).toBeVisible();
});

test('setup form rejects empty submit', async ({ page }) => {
  await page.goto('/setup');
  const submit = page.getByRole('button', { name: /Проверить|Verify/ });
  await expect(submit).toBeDisabled();
});
