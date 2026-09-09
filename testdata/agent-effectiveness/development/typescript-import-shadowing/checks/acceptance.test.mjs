import assert from "node:assert/strict";
import test from "node:test";
import { resetCalls, savedOrders } from "../workspace/storefront/src/ordersApi.ts";
import { buildSubmitters } from "../workspace/storefront/src/useCheckout.ts";
test("module-level imported provider receives the order once", () => {
  resetCalls(); const order={id:"order-1"}; buildSubmitters(["line-a"],order)[0](); assert.deepEqual(savedOrders(),[order]);
});
