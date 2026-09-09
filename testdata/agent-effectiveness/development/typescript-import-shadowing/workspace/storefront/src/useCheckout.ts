import { saveOrder } from "./ordersApi.ts";
export function buildSubmitters(items, order) {
  return items.map(saveOrder => () => saveOrder(order));
}
