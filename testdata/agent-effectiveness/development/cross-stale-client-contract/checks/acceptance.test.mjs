import assert from "node:assert/strict";
import test from "node:test";
import { dispatchShipment } from "../workspace/storefront/src/shipmentClient.ts";
test("client matches live PATCH contract and retains auth", async () => { const request=await dispatchShipment({request:async value=>value},"secret","42"); assert.deepEqual(request,{method:"PATCH",path:"/shipments/42/dispatch",headers:{Authorization:"Bearer secret"}}); });
