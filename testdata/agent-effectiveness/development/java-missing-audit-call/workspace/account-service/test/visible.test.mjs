import assert from "node:assert/strict";
import { mkdtemp, mkdir, readFile, rm, writeFile } from "node:fs/promises";
import { tmpdir } from "node:os";
import path from "node:path";
import { spawnSync } from "node:child_process";
import test from "node:test";
test("remove performs repository deletion and mail notification", async () => {
  const root = await mkdtemp(path.join(tmpdir(), "account-visible-"));
  try {
    const pkg = path.join(root, "example/accounts");
    await mkdir(pkg, { recursive: true });
    await writeFile(path.join(pkg, "AccountService.java"), await readFile("src/example/accounts/AccountService.java", "utf8"));
    await writeFile(path.join(pkg, "Visible.java"), `package example.accounts;
public final class Visible {
  public static void main(String[] args) {
    var calls = new java.util.ArrayList<String>();
    new AccountService(account -> calls.add("delete"), (event, id) -> {}, id -> calls.add("mail"))
        .remove(new AccountService.Account("42"));
    if (!calls.equals(java.util.List.of("delete", "mail"))) throw new AssertionError(calls);
  }
}`);
    const out = path.join(root, "out");
    await mkdir(out);
    let result = spawnSync("javac", ["-d", out, path.join(pkg, "AccountService.java"), path.join(pkg, "Visible.java")], { encoding: "utf8" });
    assert.equal(result.status, 0, result.stderr);
    result = spawnSync("java", ["-cp", out, "example.accounts.Visible"], { encoding: "utf8" });
    assert.equal(result.status, 0, result.stderr);
  } finally {
    await rm(root, { recursive: true, force: true });
  }
});
