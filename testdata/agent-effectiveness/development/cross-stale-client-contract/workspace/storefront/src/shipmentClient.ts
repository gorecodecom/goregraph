export async function dispatchShipment(transport, token, id) {
  return transport.request({ method: "POST", path: `/shipments/${id}/send`, headers: { Authorization: `Bearer ${token}` } });
}
