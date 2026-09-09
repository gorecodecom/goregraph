export interface Transport { delete(path: string): Promise<void>; }
export function ordersApi(transport: Transport) {
  return { cancel: (id: string) => transport.delete(`/api/orders/${id}`) };
}
