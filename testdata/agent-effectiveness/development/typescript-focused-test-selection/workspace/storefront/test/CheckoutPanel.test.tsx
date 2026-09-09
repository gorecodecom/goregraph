import { submitLine } from "../src/CheckoutPanel";
test("shows quantity validation error", () => { expect(submitLine([], { sku: "A", quantity: 0 }).error).toContain("quantity"); });
