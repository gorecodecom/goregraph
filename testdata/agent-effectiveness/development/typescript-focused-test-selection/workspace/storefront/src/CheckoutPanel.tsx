import { addLine, type CartLine } from "./cartStore";
export function submitLine(lines: CartLine[], line: CartLine): { lines: CartLine[]; error?: string } {
  try { return { lines: addLine(lines, line) }; } catch (error) { return { lines, error: String(error) }; }
}
