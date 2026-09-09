export interface Transport { post(path: string): Promise<Uint8Array>; }
export function exportReport(transport: Transport): Promise<Uint8Array> { return transport.post("/reports/export"); }
