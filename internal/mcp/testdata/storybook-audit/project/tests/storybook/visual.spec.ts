import { test, expect } from '@playwright/test';
test('ProductCard screenshot', async ({ page }) => {
  await page.goto('/iframe.html?id=products-productcard--ready&viewMode=story');
  await expect(page).toHaveScreenshot('product-card.png');
});
