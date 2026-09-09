import assert from "node:assert/strict";
import test from "node:test";
import { dispatchShipment } from "../workspace/storefront/src/shipmentClient.ts";
test("dispatch function resolves", async () => { const result=await dispatchShipment({request:async request=>request},"token","42"); assert.equal(result.headers.Authorization,"Bearer token"); });
