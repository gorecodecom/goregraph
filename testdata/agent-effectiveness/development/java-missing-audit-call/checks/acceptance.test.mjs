import assert from "node:assert/strict";
import { mkdtemp, mkdir, readFile, rm, writeFile } from "node:fs/promises";
import { tmpdir } from "node:os";
import path from "node:path";
import { spawnSync } from "node:child_process";
import test from "node:test";
test("delete, audit, and mail behavior is ordered and failure-safe", async () => {
  const root = await mkdtemp(path.join(tmpdir(), "account-acceptance-"));
  try {
    const pkg = path.join(root, "example/accounts");
    await mkdir(pkg, { recursive: true });
    await writeFile(path.join(pkg, "AccountService.java"), await readFile("workspace/account-service/src/example/accounts/AccountService.java", "utf8"));
    await writeFile(path.join(pkg, "Acceptance.java"), `package example.accounts;
public final class Acceptance {
  public static void main(String[] args) {
    var calls = new java.util.ArrayList<String>();
    AccountService.AccountRepository repo = account -> calls.add("delete");
    AccountService.AuditPublisher audit = (event, id) -> calls.add("audit:" + event + ":" + id);
    AccountService.AccountMailSender mail = id -> calls.add("mail:" + id);
    new AccountService(repo, audit, mail).remove(new AccountService.Account("42"));
    if (!calls.equals(java.util.List.of("delete", "audit:account.removed:42", "mail:42"))) throw new AssertionError(calls);
    calls.clear();
    AccountService failing = new AccountService(account -> { throw new IllegalStateException("delete failed"); }, audit, mail);
    try { failing.remove(new AccountService.Account("43")); } catch (IllegalStateException expected) {}
    if (!calls.isEmpty()) throw new AssertionError("side effects after failed delete " + calls);
  }
}`);
    const out = path.join(root, "out");
    await mkdir(out);
    let result = spawnSync("javac", ["-d", out, path.join(pkg, "AccountService.java"), path.join(pkg, "Acceptance.java")], { encoding: "utf8" });
    assert.equal(result.status, 0, result.stderr);
    result = spawnSync("java", ["-cp", out, "example.accounts.Acceptance"], { encoding: "utf8" });
    assert.equal(result.status, 0, result.stderr);
  } finally {
    await rm(root, { recursive: true, force: true });
  }
});
