export interface OrdersApi { cancel(id: string): Promise<void>; }
export async function cancelFromPanel(api: OrdersApi, id: string): Promise<void> { await api.cancel(id); }
