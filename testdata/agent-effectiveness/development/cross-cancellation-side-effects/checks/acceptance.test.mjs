import assert from "node:assert/strict";
import { mkdtemp, mkdir, readFile, rm, writeFile } from "node:fs/promises";
import { tmpdir } from "node:os";
import path from "node:path";
import { spawnSync } from "node:child_process";
import test from "node:test";
test("event waits for commit and is absent on rollback", async () => {
  const root = await mkdtemp(path.join(tmpdir(), "cancel-acceptance-"));
  try {
    const pkg = path.join(root, "example/orders");
    await mkdir(pkg, { recursive: true });
    await writeFile(path.join(pkg, "OrderService.java"), await readFile("workspace/orders-service/src/example/orders/OrderService.java", "utf8"));
    await writeFile(path.join(pkg, "Acceptance.java"), `package example.orders;
public final class Acceptance {
  static final class Tx implements OrderService.TransactionHooks {
    Runnable callback;
    public void afterCommit(Runnable value) { callback = value; }
  }
  public static void main(String[] args) {
    var events = new java.util.ArrayList<String>();
    Tx commit = new Tx();
    new OrderService(id -> {}, event -> events.add(event.orderId()), commit).cancel("42");
    if (!events.isEmpty()) throw new AssertionError("published before commit " + events);
    if (commit.callback == null) throw new AssertionError("no post-commit callback");
    commit.callback.run();
    if (!events.equals(java.util.List.of("42"))) throw new AssertionError(events);
    events.clear();
    Tx rollback = new Tx();
    new OrderService(id -> {}, event -> events.add(event.orderId()), rollback).cancel("43");
    if (!events.isEmpty()) throw new AssertionError("published on rollback");
  }
}`);
    const out = path.join(root, "out");
    await mkdir(out);
    let result = spawnSync("javac", ["-d", out, path.join(pkg, "OrderService.java"), path.join(pkg, "Acceptance.java")], { encoding: "utf8" });
    assert.equal(result.status, 0, result.stderr);
    result = spawnSync("java", ["-cp", out, "example.orders.Acceptance"], { encoding: "utf8" });
    assert.equal(result.status, 0, result.stderr);
  } finally {
    await rm(root, { recursive: true, force: true });
  }
});
