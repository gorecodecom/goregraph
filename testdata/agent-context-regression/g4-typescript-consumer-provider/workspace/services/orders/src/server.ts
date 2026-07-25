import express from "express";
import type { Request, Response } from "express";

import { orderService } from "./orderService";

const application = express();

application.delete("/api/orders/:orderId", removeOrder);
application.get("/api/orders", listOrders);

export async function removeOrder(
  request: Request,
  response: Response,
): Promise<void> {
  await orderService.remove(request.params.orderId);
  response.sendStatus(204);
}

export async function listOrders(
  request: Request,
  response: Response,
): Promise<void> {
  response.json([]);
}
