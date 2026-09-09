declare function rpc(route: string): MethodDecorator;
declare function routeFromManifest(key: string): string;
export class RpcOrders {
  @rpc(routeFromManifest("orders.cancel"))
  cancel(id: string): { accepted: string } { return { accepted: id }; }
}
