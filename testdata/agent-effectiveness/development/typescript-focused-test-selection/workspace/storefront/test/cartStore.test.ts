import { addLine } from "../src/cartStore";
test("rejects quantity below one", () => { expect(() => addLine([], { sku: "A", quantity: 0 })).toThrow(); });
