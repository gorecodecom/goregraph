import assert from "node:assert/strict";
import { spawnSync } from "node:child_process";
import test from "node:test";
test("gitignore excludes only generated bundle", () => {
  const cwd="workspace/storefront";
  const generated=spawnSync("git",["check-ignore","--no-index","public/vendor.generated.js"],{cwd,encoding:"utf8"});
  const maintained=spawnSync("git",["check-ignore","--no-index","src/vendorClient.ts"],{cwd,encoding:"utf8"});
  assert.equal(generated.status,0,"generated bundle is not ignored");
  assert.equal(maintained.status,1,"maintained source is also ignored");
});
