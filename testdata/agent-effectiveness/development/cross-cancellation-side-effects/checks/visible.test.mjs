import assert from "node:assert/strict";
import { mkdtemp, mkdir, readFile, rm, writeFile } from "node:fs/promises";
import { tmpdir } from "node:os";
import path from "node:path";
import { spawnSync } from "node:child_process";
import test from "node:test";
async function runHarness(source) {
  const root = await mkdtemp(path.join(tmpdir(), "cancel-visible-"));
  try {
    const pkg = path.join(root, "example/orders");
    await mkdir(pkg, { recursive: true });
    await writeFile(path.join(pkg, "OrderService.java"), source);
    await writeFile(path.join(pkg, "Visible.java"), `package example.orders;
public final class Visible {
  public static void main(String[] args) {
    var calls = new java.util.ArrayList<String>();
    OrderService.Repository repository = id -> calls.add("cancel:" + id);
    OrderService.Publisher publisher = event -> calls.add("publish:" + event.orderId());
    OrderService.TransactionHooks transactions = callback -> {};
    new OrderService(repository, publisher, transactions).cancel("42");
    if (!calls.contains("cancel:42")) throw new AssertionError(calls);
  }
}`);
    const out = path.join(root, "out");
    await mkdir(out);
    let result = spawnSync("javac", ["-d", out, path.join(pkg, "OrderService.java"), path.join(pkg, "Visible.java")], { encoding: "utf8" });
    assert.equal(result.status, 0, result.stderr);
    result = spawnSync("java", ["-cp", out, "example.orders.Visible"], { encoding: "utf8" });
    assert.equal(result.status, 0, result.stderr);
  } finally {
    await rm(root, { recursive: true, force: true });
  }
}
test("repository cancellation is performed", async () => runHarness(await readFile("workspace/orders-service/src/example/orders/OrderService.java", "utf8")));
