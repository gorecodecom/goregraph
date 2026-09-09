export type CartLine = { sku: string; quantity: number };
export function addLine(lines: CartLine[], line: CartLine): CartLine[] {
  if (line.quantity < 1) throw new RangeError("quantity must be at least one");
  return [...lines, line];
}
