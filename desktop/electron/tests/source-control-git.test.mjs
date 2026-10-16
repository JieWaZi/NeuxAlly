import { test } from "node:test";
import assert from "node:assert/strict";
import { execFile } from "node:child_process";
import { promisify } from "node:util";
import { mkdtemp, open, readFile, rm, writeFile } from "node:fs/promises";
import { tmpdir } from "node:os";
import path from "node:path";

import { loadModule } from "./load-module.mjs";

const run = promisify(execFile);
const gitModule = loadModule("main/source-control-git.ts");

function statusEntry(
  path,
  xy,
  stagedAddedLines,
  stagedDeletedLines,
  unstagedAddedLines,
  unstagedDeletedLines,
) {
  return {
    path,
    xy,
    stagedAddedLines,
    stagedDeletedLines,
    unstagedAddedLines,
    unstagedDeletedLines,
  };
}

test("Git 解析保留特殊文件名与重命名的当前路径", () => {
  assert.deepEqual(
    gitModule.parsePorcelainZ(
      " M 报告 #1%.md\0R  新名字.md\0旧名字.md\0?? a b.txt\0",
    ),
    [
      { path: "报告 #1%.md", xy: " M" },
      { path: "新名字.md", xy: "R " },
      { path: "a b.txt", xy: "??" },
    ],
  );
});

test("Git 状态分别统计暂存、未暂存和未跟踪文件的增删行", async (t) => {
  const root = await mkdtemp(path.join(tmpdir(), "ally-git-lines-"));
  t.after(() => rm(root, { recursive: true, force: true }));
  const git = async (...args) => run("git", ["-C", root, ...args]);
  await git("init", "-q");
  await git("config", "user.name", "NeuxAlly Test");
  await git("config", "user.email", "ally-test@example.invalid");
  await writeFile(path.join(root, "tracked.txt"), "before\n");
  await git("add", "tracked.txt");
  await git("commit", "-qm", "initial");
  await writeFile(path.join(root, "tracked.txt"), "staged\nmore\n");
  await git("add", "tracked.txt");
  await writeFile(path.join(root, "tracked.txt"), "staged\nworking\n");
  await writeFile(path.join(root, "new file.txt"), "one\ntwo\nthree");

  const result = await gitModule.status(root);
  assert.deepEqual(result.entries, [
    {
      path: "tracked.txt",
      xy: "MM",
      stagedAddedLines: 2,
      stagedDeletedLines: 1,
      unstagedAddedLines: 1,
      unstagedDeletedLines: 1,
    },
    {
      path: "new file.txt",
      xy: "??",
      stagedAddedLines: 0,
      stagedDeletedLines: 0,
      unstagedAddedLines: 3,
      unstagedDeletedLines: 0,
    },
  ]);
  assert.match(await gitModule.diff(root, "new file.txt", false), /\+three/);
});

test("大型未跟踪文件的状态不扫描内容且标明行数不可用", async (t) => {
  const root = await mkdtemp(path.join(tmpdir(), "ally-git-large-"));
  t.after(() => rm(root, { recursive: true, force: true }));
  await run("git", ["-C", root, "init", "-q"]);
  const file = await open(path.join(root, "large.bin"), "w");
  try {
    await file.truncate(64 * 1024 * 1024);
  } finally {
    await file.close();
  }
  const result = await gitModule.status(root);
  assert.deepEqual(result.entries, [
    {
      ...statusEntry("large.bin", "??", 0, 0, 0, 0),
      lineCountUnavailable: true,
    },
  ]);
});

test("Git 状态、差异、暂存、撤销暂存、提交与历史共用真实仓库", async (t) => {
  const root = await mkdtemp(path.join(tmpdir(), "ally-git-"));
  t.after(() => rm(root, { recursive: true, force: true }));
  const git = async (...args) => run("git", ["-C", root, ...args]);
  await git("init", "-q");
  await git("config", "user.name", "NeuxAlly Test");
  await git("config", "user.email", "ally-test@example.invalid");
  const name = "报告 #1%.md";
  await writeFile(path.join(root, name), "before\n");
  await git("add", "--", name);
  await git("commit", "-qm", "initial");
  await writeFile(path.join(root, name), "after\n");
  let status = await gitModule.status(root);
  assert.deepEqual(status.entries, [statusEntry(name, " M", 0, 0, 1, 1)]);
  assert.match(await gitModule.diff(root, name, false), /\+after/);
  await gitModule.stage(root, name);
  status = await gitModule.status(root);
  assert.deepEqual(status.entries, [statusEntry(name, "M ", 1, 1, 0, 0)]);
  assert.match(await gitModule.diff(root, name, true), /\+after/);
  await gitModule.unstage(root, name);
  assert.deepEqual((await gitModule.status(root)).entries, [
    statusEntry(name, " M", 0, 0, 1, 1),
  ]);
  await gitModule.stage(root, name);
  await gitModule.commit(root, "updated");
  assert.deepEqual((await gitModule.status(root)).entries, []);
  const history = await gitModule.log(root, 2);
  assert.equal(history[0].subject, "updated");
  assert.equal(await gitModule.show(root, "HEAD", name), "after\n");
  await writeFile(path.join(root, name), "discard me\n");
  await gitModule.discard(root, name);
  assert.equal(await readFile(path.join(root, name), "utf8"), "after\n");
});
