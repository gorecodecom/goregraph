import assert from "node:assert/strict";
import { access } from "node:fs/promises";
import { spawnSync } from "node:child_process";
import test from "node:test";
test("maintained TypeScript source remains discoverable", async () => {
  await access("src/vendorClient.ts");
  const result=spawnSync("git",["check-ignore","--no-index","src/vendorClient.ts"],{encoding:"utf8"});
  assert.equal(result.status,1,"maintained source is ignored");
});
