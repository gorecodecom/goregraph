import assert from "node:assert/strict";
import { mkdtemp, mkdir, readFile, rm, writeFile } from "node:fs/promises";
import { tmpdir } from "node:os";
import path from "node:path";
import { spawnSync } from "node:child_process";
import test from "node:test";
test("client calls the configured inventory URI", async () => {
  const root = await mkdtemp(path.join(tmpdir(), "inventory-visible-"));
  try {
    const pkg = path.join(root, "example/inventory");
    await mkdir(pkg, { recursive: true });
    for (const file of ["InventoryClient.java", "InventoryProperties.java"]) {
      await writeFile(path.join(pkg, file), await readFile(`src/example/inventory/${file}`, "utf8"));
    }
    await writeFile(path.join(pkg, "Visible.java"), `package example.inventory;
public final class Visible {
  static final class Request implements InventoryClient.Request {
    String uri;
    public InventoryClient.Request uri(String template, String value) { uri = template.replace("{sku}", value); return this; }
    public InventoryClient.Request header(String name, String value) { return this; }
    public String retrieve() { return "inventory"; }
  }
  public static void main(String[] args) {
    Request request = new Request();
    String value = new InventoryClient(() -> request, new InventoryProperties("https://inventory", "token")).inventory("A-1");
    if (!"inventory".equals(value) || !"https://inventory/inventory/A-1".equals(request.uri)) throw new AssertionError();
  }
}`);
    const out = path.join(root, "out");
    await mkdir(out);
    let result = spawnSync("javac", ["-d", out, path.join(pkg, "InventoryClient.java"), path.join(pkg, "InventoryProperties.java"), path.join(pkg, "Visible.java")], { encoding: "utf8" });
    assert.equal(result.status, 0, result.stderr);
    result = spawnSync("java", ["-cp", out, "example.inventory.Visible"], { encoding: "utf8" });
    assert.equal(result.status, 0, result.stderr);
  } finally {
    await rm(root, { recursive: true, force: true });
  }
});
