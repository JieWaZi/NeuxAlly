import { createRequire } from "node:module";
const requireDesktop = createRequire(
  new URL("../desktop/package.json", import.meta.url),
);
import { test } from "node:test";
import assert from "node:assert/strict";
import { mkdtemp, readFile, rm } from "node:fs/promises";
import os from "node:os";
import path from "node:path";
import {
  loadLock,
  verifyDigest,
  prepareCatalog,
  selectAsset,
} from "./plugin-resources.mjs";

test("fixed source rejects missing provenance and tampered bytes", async () => {
  assert.throws(
    () => verifyDigest(Buffer.from("tampered"), "a".repeat(64)),
    /摘要/,
  );
  await assert.rejects(
    () => loadLock({ schemaVersion: 1, source: {} }),
    /来源/,
  );
});
test("target chooses only exact fixed native asset", async () => {
  const lock = await loadLock();
  assert.equal(
    selectAsset(lock, "darwin-arm64").file,
    "lark-cli-1.0.98-darwin-arm64.tar.gz",
  );
  assert.equal(
    selectAsset(lock, "win32-x64").file,
    "lark-cli-1.0.98-windows-amd64.zip",
  );
  assert.throws(() => selectAsset(lock, "unknown"), /目标/);
});
test("independent Git bundle builds catalog completely offline", async () => {
  const output = await mkdtemp(path.join(os.tmpdir(), "plugin-catalog-"));
  try {
    const catalog = await prepareCatalog(output);
    assert.equal(catalog.entries.length, 1);
    const entry = catalog.entries[0];
    assert.equal(entry.key, "feishu-office");
    const files = requireDesktop("fflate").unzipSync(
      new Uint8Array(await readFile(path.join(output, entry.directory))),
    );
    const text = (name) => Buffer.from(files[name]).toString("utf8");
    const manifest = JSON.parse(text("plugin.json"));
    assert.equal(manifest.extensions["com.ally"].provider, "feishu");
    for (const skill of [
      "lark-doc",
      "lark-base",
      "lark-calendar",
      "lark-task",
      "lark-shared",
    ])
      assert.match(text(`skills/${skill}/SKILL.md`), /name:/);
    assert.match(text("skills/lark-doc/references/lark-doc-fetch.md"), /fetch/);
    assert.equal(
      JSON.parse(text("provenance.json")).upstream.commit,
      "ac81f5056ab88f37d581d25916f3dab85abfca76",
    );
    assert.match(text("LICENSE.larksuite-cli"), /MIT License/);
  } finally {
    await rm(output, { recursive: true, force: true });
  }
});
