export async function removeOrder(orderId: string): Promise<void> {
  function authorizationHeaders(): Record<string, string> {
    return { Authorization: "Bearer runtime-token" };
  }

  await fetch(`/api/orders/${orderId}`, {
    method: "DELETE",
    headers: authorizationHeaders(),
  });
}
