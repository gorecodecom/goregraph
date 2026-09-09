import assert from "node:assert/strict";
import { mkdtemp, mkdir, readFile, rm, writeFile } from "node:fs/promises";
import { tmpdir } from "node:os";
import path from "node:path";
import { spawnSync } from "node:child_process";
import test from "node:test";
test("configured token is sent as a header and not disclosed", async () => {
  const root = await mkdtemp(path.join(tmpdir(), "inventory-acceptance-"));
  try {
    const pkg = path.join(root, "example/inventory");
    await mkdir(pkg, { recursive: true });
    for (const file of ["InventoryClient.java", "InventoryProperties.java"]) {
      await writeFile(path.join(pkg, file), await readFile(`workspace/inventory-client/src/example/inventory/${file}`, "utf8"));
    }
    await writeFile(path.join(pkg, "Acceptance.java"), `package example.inventory;
public final class Acceptance {
  static final class Request implements InventoryClient.Request {
    String uri; String headerName; String headerValue;
    public InventoryClient.Request uri(String template, String value) { uri = template.replace("{sku}", value); return this; }
    public InventoryClient.Request header(String name, String value) { headerName = name; headerValue = value; return this; }
    public String retrieve() { return "ok"; }
  }
  public static void main(String[] args) {
    Request request = new Request();
    var output = new java.io.ByteArrayOutputStream();
    var previous = System.out;
    System.setOut(new java.io.PrintStream(output));
    try {
      String result = new InventoryClient(() -> request, new InventoryProperties("https://inventory", "secret-value")).inventory("A-1");
      if (!"ok".equals(result) || !"https://inventory/inventory/A-1".equals(request.uri) || !"X-Service-Token".equals(request.headerName) || !"secret-value".equals(request.headerValue)) throw new AssertionError();
    } finally { System.setOut(previous); }
    if (output.toString().contains("secret-value")) throw new AssertionError("token disclosed");
  }
}`);
    const out = path.join(root, "out");
    await mkdir(out);
    let result = spawnSync("javac", ["-d", out, path.join(pkg, "InventoryClient.java"), path.join(pkg, "InventoryProperties.java"), path.join(pkg, "Acceptance.java")], { encoding: "utf8" });
    assert.equal(result.status, 0, result.stderr);
    result = spawnSync("java", ["-cp", out, "example.inventory.Acceptance"], { encoding: "utf8" });
    assert.equal(result.status, 0, result.stderr);
  } finally {
    await rm(root, { recursive: true, force: true });
  }
});
