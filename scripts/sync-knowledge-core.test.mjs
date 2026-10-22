import assert from "node:assert/strict";
import { mkdtemp, mkdir, writeFile, readFile, rm } from "node:fs/promises";
import os from "node:os";
import path from "node:path";
import { test } from "node:test";
import {
  syncKnowledgeCore,
  verifyKnowledgeCore,
} from "./sync-knowledge-core.mjs";

test("同步独立源码形成可核验快照，不包含运行数据和个人路径", async () => {
  const root = await mkdtemp(path.join(os.tmpdir(), "knowledge-snapshot-"));
  try {
    const source = path.join(root, "source"),
      target = path.join(root, "snapshot");
    await mkdir(path.join(source, "core"), { recursive: true });
    await writeFile(
      path.join(source, "go.mod"),
      "module github.com/JieWaZi/neux-knowledge\n\ngo 1.25.8\n",
    );
    await writeFile(path.join(source, "core", "core.go"), "package core\n");
    await writeFile(path.join(source, "core", "secret.db"), "private");
    await writeFile(path.join(source, ".env"), "private");
    await syncKnowledgeCore(source, target);
    const manifest = await verifyKnowledgeCore(target);
    assert.deepEqual(
      manifest.files.map((file) => file.path),
      ["core/core.go", "go.mod"],
    );
    assert.equal(JSON.stringify(manifest).includes(root), false);
    await writeFile(path.join(target, "core", "core.go"), "changed");
    await assert.rejects(verifyKnowledgeCore(target), /快照内容不匹配/);
    await syncKnowledgeCore(source, target);
    assert.equal(
      await readFile(path.join(target, "core", "core.go"), "utf8"),
      "package core\n",
    );
    await writeFile(path.join(target, "unexpected.go"), "package core\n");
    await assert.rejects(verifyKnowledgeCore(target), /额外文件/);
    await rm(path.join(target, "unexpected.go"));
    await writeFile(path.join(target, ".env"), "private");
    await assert.rejects(verifyKnowledgeCore(target), /额外文件/);
  } finally {
    await rm(root, { recursive: true, force: true });
  }
});
