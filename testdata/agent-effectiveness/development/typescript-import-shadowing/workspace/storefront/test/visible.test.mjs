import assert from "node:assert/strict";
import test from "node:test";
import { buildSubmitters } from "../src/useCheckout.ts";
test("one submit callback is created per item", () => {
  assert.equal(buildSubmitters(["line-a"], { id: "order-1" }).length, 1);
});
