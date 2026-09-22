import { expect, within, userEvent } from 'storybook/test';
import { ProductCard } from './ProductCard';
import { productFixture } from './productFixture';
export default { title: 'Products/ProductCard', component: ProductCard };
export const Ready = {
  args: productFixture(),
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    await userEvent.click(canvas.getByRole('button'));
    await expect(canvas.getByRole('button')).toBeVisible();
  },
};
