const calls = [];
export function saveOrder(order) { calls.push(order); return { saved: true, order }; }
export function resetCalls() { calls.length = 0; }
export function savedOrders() { return [...calls]; }
