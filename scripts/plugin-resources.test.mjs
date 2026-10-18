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
  syncPluginGuides,
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
    assert.equal(catalog.entries.length, 2);
    await syncPluginGuides(catalog, true);
    for (const item of catalog.entries) {
      assert.match(item.package.readme, /### 可以做什么/);
      assert.match(item.package.readme, /### 典型用途/);
      assert.ok(item.package.readme.length < 600);
      assert.doesNotMatch(
        item.package.readme,
        /源码提交|钥匙串|工具链|隔离补丁|NeuxAlly|\bAlly\b|伙伴|安装|扫码|开始使用/i,
      );
    }
    const github = catalog.entries.find((entry) => entry.key === "github");
    assert.ok(github);
    assert.equal(
      github.package.servers[0].url,
      "https://api.githubcopilot.com/mcp/",
    );
    assert.equal(github.package.skills.length, 0);
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

test("both providers use pinned assets for all six desktop targets", async () => {
  const lock = await loadLock();
  for (const target of [
    "darwin-amd64",
    "darwin-arm64",
    "windows-amd64",
    "windows-arm64",
    "linux-amd64",
    "linux-arm64",
  ]) {
    const asset = selectAsset(lock, target, "github");
    assert.equal(asset.target, target);
    assert.equal(
      asset.file,
      "github-cli-fc4b137cdef0a6bd28fd461b7cf9c84a5812a8cd.tar.gz",
    );
    assert.equal(
      asset.url,
      "https://github.com/cli/cli/archive/fc4b137cdef0a6bd28fd461b7cf9c84a5812a8cd.tar.gz",
    );
  }
  const incomplete = structuredClone(lock);
  incomplete.providers = incomplete.providers.filter(
    (p) => p.name !== "github",
  );
  await assert.rejects(() => loadLock(incomplete), /来源/);
});
