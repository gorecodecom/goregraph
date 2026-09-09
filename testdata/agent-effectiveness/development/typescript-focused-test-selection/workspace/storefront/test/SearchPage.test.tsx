import { filterProducts } from "../src/SearchPage";
test("filters search results", () => { expect(filterProducts(["apple", "pear"], "app")).toEqual(["apple"]); });
